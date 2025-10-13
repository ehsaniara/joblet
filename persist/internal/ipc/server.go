package ipc

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"sync/atomic"

	"google.golang.org/protobuf/proto"

	ipcpb "github.com/ehsaniara/joblet/internal/proto/gen/ipc"
	"github.com/ehsaniara/joblet/persist/internal/config"
	"github.com/ehsaniara/joblet/persist/internal/storage"
	"github.com/ehsaniara/joblet/persist/pkg/logger"
)

// Server is the IPC server that receives messages from joblet-core
type Server struct {
	config   *config.IPCConfig
	backend  storage.Backend
	logger   *logger.Logger
	listener net.Listener

	// Write pipeline
	writePipe chan *ipcpb.IPCMessage

	// Connection management
	connections sync.Map // conn_id -> net.Conn

	// Metrics
	msgsReceived  atomic.Uint64
	bytesReceived atomic.Uint64
	writeErrors   atomic.Uint64

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewServer creates a new IPC server
func NewServer(cfg *config.IPCConfig, backend storage.Backend, log *logger.Logger) *Server {
	ctx, cancel := context.WithCancel(context.Background())

	return &Server{
		config:    cfg,
		backend:   backend,
		logger:    log.WithField("component", "ipc-server"),
		writePipe: make(chan *ipcpb.IPCMessage, 10000), // 10k message buffer
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Start starts the IPC server
func (s *Server) Start(ctx context.Context) error {
	// Remove existing socket
	if err := os.Remove(s.config.Socket); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove existing socket: %w", err)
	}

	// Create Unix socket listener
	listener, err := net.Listen("unix", s.config.Socket)
	if err != nil {
		return fmt.Errorf("failed to create Unix socket: %w", err)
	}

	// Set permissions (joblet user only)
	if err := os.Chmod(s.config.Socket, 0600); err != nil {
		listener.Close()
		return fmt.Errorf("failed to set socket permissions: %w", err)
	}

	s.listener = listener
	s.logger.Info("IPC server listening", "socket", s.config.Socket)

	// Start write pipeline workers
	for i := 0; i < 4; i++ { // 4 workers
		s.wg.Add(1)
		go s.writeWorker(i)
	}

	// Start accept loop
	s.wg.Add(1)
	go s.acceptLoop()

	return nil
}

// Stop stops the IPC server
func (s *Server) Stop() error {
	s.logger.Info("Stopping IPC server")
	s.cancel()

	if s.listener != nil {
		s.listener.Close()
	}

	// Wait for all goroutines
	s.wg.Wait()

	// Close write pipeline
	close(s.writePipe)

	s.logger.Info("IPC server stopped",
		"msgsReceived", s.msgsReceived.Load(),
		"bytesReceived", s.bytesReceived.Load())

	return nil
}

// acceptLoop accepts incoming connections
func (s *Server) acceptLoop() {
	defer s.wg.Done()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				return
			default:
				s.logger.Error("Accept error", "error", err)
				continue
			}
		}

		// Configure Unix socket
		if uc, ok := conn.(*net.UnixConn); ok {
			uc.SetReadBuffer(s.config.ReadBuffer)
		}

		// Handle connection in goroutine
		s.wg.Add(1)
		go s.handleConnection(conn)
	}
}

// handleConnection handles a single IPC connection
func (s *Server) handleConnection(conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()

	connID := fmt.Sprintf("%p", conn)
	s.connections.Store(connID, conn)
	defer s.connections.Delete(connID)

	s.logger.Info("New IPC connection", "connID", connID)

	lengthBuf := make([]byte, 4)

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		// Read length prefix
		if _, err := io.ReadFull(conn, lengthBuf); err != nil {
			if err != io.EOF {
				s.logger.Debug("Connection closed", "connID", connID, "error", err)
			}
			return
		}

		length := binary.BigEndian.Uint32(lengthBuf)
		if length > uint32(s.config.MaxMessageSize) {
			s.logger.Error("Message too large", "length", length, "max", s.config.MaxMessageSize)
			return
		}

		// Read message
		msgBuf := make([]byte, length)
		if _, err := io.ReadFull(conn, msgBuf); err != nil {
			s.logger.Error("Failed to read message", "error", err)
			return
		}

		// Decode protobuf message
		var msg ipcpb.IPCMessage
		if err := proto.Unmarshal(msgBuf, &msg); err != nil {
			s.logger.Error("Failed to unmarshal message", "error", err)
			continue
		}

		s.msgsReceived.Add(1)
		s.bytesReceived.Add(uint64(length))

		// Send to write pipeline (non-blocking)
		select {
		case s.writePipe <- &msg:
			// Queued successfully
		default:
			s.logger.Warn("Write pipeline full, dropping message", "jobID", msg.JobId)
			s.writeErrors.Add(1)
		}
	}
}

// writeWorker processes messages from the write pipeline
func (s *Server) writeWorker(id int) {
	defer s.wg.Done()

	workerLog := s.logger.WithField("worker", id)
	workerLog.Debug("Write worker started")

	batch := make([]*ipcpb.IPCMessage, 0, 100)

	for msg := range s.writePipe {
		batch = append(batch, msg)

		// Flush batch when full or channel empty
		if len(batch) >= 100 {
			s.processBatch(batch, workerLog)
			batch = batch[:0]
		} else if len(s.writePipe) == 0 && len(batch) > 0 {
			s.processBatch(batch, workerLog)
			batch = batch[:0]
		}
	}

	// Flush remaining batch
	if len(batch) > 0 {
		s.processBatch(batch, workerLog)
	}

	workerLog.Debug("Write worker stopped")
}

// processBatch processes a batch of messages
func (s *Server) processBatch(batch []*ipcpb.IPCMessage, log *logger.Logger) {
	// Group by job ID for efficient writing
	jobBatches := make(map[string]*JobBatch)

	for _, msg := range batch {
		if _, exists := jobBatches[msg.JobId]; !exists {
			jobBatches[msg.JobId] = &JobBatch{
				JobID:   msg.JobId,
				Logs:    make([]*ipcpb.LogLine, 0),
				Metrics: make([]*ipcpb.Metric, 0),
			}
		}

		batch := jobBatches[msg.JobId]

		switch msg.Type {
		case ipcpb.MessageType_MESSAGE_TYPE_LOG:
			var logLine ipcpb.LogLine
			if err := proto.Unmarshal(msg.Data, &logLine); err != nil {
				log.Error("Failed to unmarshal log", "error", err)
				continue
			}
			batch.Logs = append(batch.Logs, &logLine)

		case ipcpb.MessageType_MESSAGE_TYPE_METRIC:
			var metric ipcpb.Metric
			if err := proto.Unmarshal(msg.Data, &metric); err != nil {
				log.Error("Failed to unmarshal metric", "error", err)
				continue
			}
			batch.Metrics = append(batch.Metrics, &metric)
		}
	}

	// Write each job's batch
	for jobID, jobBatch := range jobBatches {
		if len(jobBatch.Logs) > 0 {
			if err := s.backend.WriteLogs(jobID, jobBatch.Logs); err != nil {
				log.Error("Failed to write logs", "jobID", jobID, "error", err)
				s.writeErrors.Add(1)
			} else {
				log.Info("Wrote logs", "jobID", jobID, "count", len(jobBatch.Logs))
			}
		}

		if len(jobBatch.Metrics) > 0 {
			if err := s.backend.WriteMetrics(jobID, jobBatch.Metrics); err != nil {
				log.Error("Failed to write metrics", "jobID", jobID, "error", err)
				s.writeErrors.Add(1)
			} else {
				log.Info("Wrote metrics", "jobID", jobID, "count", len(jobBatch.Metrics))
			}
		}
	}
}

// JobBatch groups messages by job
type JobBatch struct {
	JobID   string
	Logs    []*ipcpb.LogLine
	Metrics []*ipcpb.Metric
}

// GetStats returns server statistics
func (s *Server) GetStats() Stats {
	return Stats{
		MessagesReceived: s.msgsReceived.Load(),
		BytesReceived:    s.bytesReceived.Load(),
		WriteErrors:      s.writeErrors.Load(),
	}
}

// Stats represents server statistics
type Stats struct {
	MessagesReceived uint64
	BytesReceived    uint64
	WriteErrors      uint64
}
