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
	"time"

	"google.golang.org/protobuf/proto"

	ipcpb "github.com/ehsaniara/joblet/internal/proto/gen/ipc"
	"github.com/ehsaniara/joblet/persist/internal/config"
	"github.com/ehsaniara/joblet/persist/internal/storage"
	"github.com/ehsaniara/joblet/pkg/logger"
)

const (
	// defaultFlushInterval is the periodic flush interval for batched writes
	defaultFlushInterval = 5 * time.Second
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
	msgsReceived       atomic.Uint64
	bytesReceived      atomic.Uint64
	writeErrors        atomic.Uint64
	backpressureEvents atomic.Uint64 // Count of backpressure events (block or drop)
	msgsDropped        atomic.Uint64 // Count of dropped messages (only in drop mode)

	// Computed config values
	backpressureTimeout time.Duration
	backpressureMode    string // "block" or "drop"
	batchSize           int

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewServer creates a new IPC server
func NewServer(cfg *config.IPCConfig, backend storage.Backend, log *logger.Logger) *Server {
	ctx, cancel := context.WithCancel(context.Background())

	// Apply defaults if not configured
	bufferSize := cfg.BufferSize
	if bufferSize <= 0 {
		bufferSize = 100000
	}

	backpressureTimeout := time.Duration(cfg.BackpressureTimeout) * time.Second
	if backpressureTimeout <= 0 {
		backpressureTimeout = 5 * time.Second
	}

	backpressureMode := cfg.BackpressureMode
	if backpressureMode == "" {
		backpressureMode = "block" // Default to block mode (prevents data loss)
	}

	batchSize := cfg.BatchSize
	if batchSize <= 0 {
		batchSize = 100
	}

	return &Server{
		config:              cfg,
		backend:             backend,
		logger:              log.WithField("component", "ipc-server"),
		writePipe:           make(chan *ipcpb.IPCMessage, bufferSize),
		backpressureTimeout: backpressureTimeout,
		backpressureMode:    backpressureMode,
		batchSize:           batchSize,
		ctx:                 ctx,
		cancel:              cancel,
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
	workerCount := s.config.WorkerCount
	if workerCount <= 0 {
		workerCount = 4
	}
	for i := 0; i < workerCount; i++ {
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

	// Close write pipeline first so workers can exit
	close(s.writePipe)

	// Wait for all goroutines
	s.wg.Wait()

	s.logger.Info("IPC server stopped",
		"msgsReceived", s.msgsReceived.Load(),
		"bytesReceived", s.bytesReceived.Load(),
		"backpressureEvents", s.backpressureEvents.Load(),
		"msgsDropped", s.msgsDropped.Load(),
		"writeErrors", s.writeErrors.Load())

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
		// Set read deadline to allow graceful shutdown
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))

		// Read length prefix
		if _, err := io.ReadFull(conn, lengthBuf); err != nil {
			// Check if context was cancelled during read
			select {
			case <-s.ctx.Done():
				return
			default:
			}

			// Check for timeout (expected during shutdown check)
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue // Retry read with new deadline
			}

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

		// Send to write pipeline with backpressure
		// Try non-blocking first for fast path
		select {
		case s.writePipe <- &msg:
			// Queued successfully
		default:
			// Pipeline full - apply backpressure based on configured mode
			s.backpressureEvents.Add(1)

			if s.backpressureMode == "drop" {
				// Drop mode: wait with timeout, then drop if still full
				timer := time.NewTimer(s.backpressureTimeout)
				select {
				case s.writePipe <- &msg:
					timer.Stop()
				case <-timer.C:
					s.msgsDropped.Add(1)
					s.logger.Warn("Write pipeline backpressure timeout, dropping message",
						"job_uuid", msg.JobUuid,
						"timeout", s.backpressureTimeout,
						"queueSize", len(s.writePipe),
						"totalDropped", s.msgsDropped.Load())
				case <-s.ctx.Done():
					timer.Stop()
					return
				}
			} else {
				// Block mode (default): block until space available (prevents data loss)
				// Log periodically to indicate backpressure is occurring
				s.logger.Debug("Write pipeline full, blocking sender",
					"job_uuid", msg.JobUuid,
					"queueSize", len(s.writePipe),
					"mode", "block")

				select {
				case s.writePipe <- &msg:
					// Successfully queued after blocking
				case <-s.ctx.Done():
					return
				}
			}
		}
	}
}

// writeWorker processes messages from the write pipeline
func (s *Server) writeWorker(id int) {
	defer s.wg.Done()

	workerLog := s.logger.WithField("worker", id)
	workerLog.Debug("Write worker started", "batchSize", s.batchSize)

	batch := make([]*ipcpb.IPCMessage, 0, s.batchSize)

	// Add time-based flush to ensure metrics are written periodically
	// even if batch size threshold isn't reached
	flushTicker := time.NewTicker(defaultFlushInterval)
	defer flushTicker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			// Context cancelled, flush remaining batch and exit
			if len(batch) > 0 {
				s.processBatch(batch, workerLog)
			}
			workerLog.Debug("Write worker stopped (context cancelled)")
			return

		case msg, ok := <-s.writePipe:
			if !ok {
				// Channel closed, flush remaining batch and exit
				if len(batch) > 0 {
					s.processBatch(batch, workerLog)
				}
				workerLog.Debug("Write worker stopped")
				return
			}

			batch = append(batch, msg)

			// Flush batch when full or channel empty
			if len(batch) >= s.batchSize {
				s.processBatch(batch, workerLog)
				batch = batch[:0]
			} else if len(s.writePipe) == 0 && len(batch) > 0 {
				s.processBatch(batch, workerLog)
				batch = batch[:0]
			}

		case <-flushTicker.C:
			// Periodic flush to ensure timely writes
			if len(batch) > 0 {
				workerLog.Debug("periodic flush", "batchSize", len(batch))
				s.processBatch(batch, workerLog)
				batch = batch[:0]
			}
		}
	}
}

// processBatch processes a batch of messages
func (s *Server) processBatch(batch []*ipcpb.IPCMessage, log *logger.Logger) {
	// Group by job ID for efficient writing
	jobBatches := make(map[string]*JobBatch)

	for _, msg := range batch {
		if _, exists := jobBatches[msg.JobUuid]; !exists {
			jobBatches[msg.JobUuid] = &JobBatch{
				JobUUID:          msg.JobUuid,
				Logs:             make([]*ipcpb.LogLine, 0),
				Metrics:          make([]*ipcpb.Metric, 0),
				ExecEvents:       make([]*ipcpb.ExecEvent, 0),
				ConnectEvents:    make([]*ipcpb.ConnectEvent, 0),
				FileEvents:       make([]*ipcpb.FileEvent, 0),
				AcceptEvents:     make([]*ipcpb.AcceptEvent, 0),
				SocketDataEvents: make([]*ipcpb.SocketDataEvent, 0),
				MmapEvents:       make([]*ipcpb.MmapEvent, 0),
				MprotectEvents:   make([]*ipcpb.MprotectEvent, 0),
			}
		}

		batch := jobBatches[msg.JobUuid]

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

		case ipcpb.MessageType_MESSAGE_TYPE_EXEC_EVENT:
			var execEvent ipcpb.ExecEvent
			if err := proto.Unmarshal(msg.Data, &execEvent); err != nil {
				log.Error("Failed to unmarshal exec event", "error", err)
				continue
			}
			batch.ExecEvents = append(batch.ExecEvents, &execEvent)

		case ipcpb.MessageType_MESSAGE_TYPE_CONNECT_EVENT:
			var connectEvent ipcpb.ConnectEvent
			if err := proto.Unmarshal(msg.Data, &connectEvent); err != nil {
				log.Error("Failed to unmarshal connect event", "error", err)
				continue
			}
			batch.ConnectEvents = append(batch.ConnectEvents, &connectEvent)

		case ipcpb.MessageType_MESSAGE_TYPE_FILE_EVENT:
			var fileEvent ipcpb.FileEvent
			if err := proto.Unmarshal(msg.Data, &fileEvent); err != nil {
				log.Error("Failed to unmarshal file event", "error", err)
				continue
			}
			batch.FileEvents = append(batch.FileEvents, &fileEvent)

		case ipcpb.MessageType_MESSAGE_TYPE_ACCEPT_EVENT:
			var acceptEvent ipcpb.AcceptEvent
			if err := proto.Unmarshal(msg.Data, &acceptEvent); err != nil {
				log.Error("Failed to unmarshal accept event", "error", err)
				continue
			}
			batch.AcceptEvents = append(batch.AcceptEvents, &acceptEvent)

		case ipcpb.MessageType_MESSAGE_TYPE_SOCKET_DATA_EVENT:
			var socketDataEvent ipcpb.SocketDataEvent
			if err := proto.Unmarshal(msg.Data, &socketDataEvent); err != nil {
				log.Error("Failed to unmarshal socket data event", "error", err)
				continue
			}
			batch.SocketDataEvents = append(batch.SocketDataEvents, &socketDataEvent)

		case ipcpb.MessageType_MESSAGE_TYPE_MMAP_EVENT:
			var mmapEvent ipcpb.MmapEvent
			if err := proto.Unmarshal(msg.Data, &mmapEvent); err != nil {
				log.Error("Failed to unmarshal mmap event", "error", err)
				continue
			}
			batch.MmapEvents = append(batch.MmapEvents, &mmapEvent)

		case ipcpb.MessageType_MESSAGE_TYPE_MPROTECT_EVENT:
			var mprotectEvent ipcpb.MprotectEvent
			if err := proto.Unmarshal(msg.Data, &mprotectEvent); err != nil {
				log.Error("Failed to unmarshal mprotect event", "error", err)
				continue
			}
			batch.MprotectEvents = append(batch.MprotectEvents, &mprotectEvent)
		}
	}

	// Write each job's batch
	for jobID, jobBatch := range jobBatches {
		if len(jobBatch.Logs) > 0 {
			if err := s.backend.WriteLogs(jobID, jobBatch.Logs); err != nil {
				log.Error("Failed to write logs", "job_uuid", jobID, "error", err)
				s.writeErrors.Add(1)
			} else {
				log.Info("Wrote logs", "job_uuid", jobID, "count", len(jobBatch.Logs))
			}
		}

		if len(jobBatch.Metrics) > 0 {
			if err := s.backend.WriteMetrics(jobID, jobBatch.Metrics); err != nil {
				log.Error("Failed to write metrics", "job_uuid", jobID, "error", err)
				s.writeErrors.Add(1)
			} else {
				log.Info("Wrote metrics", "job_uuid", jobID, "count", len(jobBatch.Metrics))
			}
		}

		if len(jobBatch.ExecEvents) > 0 {
			if err := s.backend.WriteExecEvents(jobID, jobBatch.ExecEvents); err != nil {
				log.Error("Failed to write exec events", "job_uuid", jobID, "error", err)
				s.writeErrors.Add(1)
			} else {
				log.Info("Wrote exec events", "job_uuid", jobID, "count", len(jobBatch.ExecEvents))
			}
		}

		if len(jobBatch.ConnectEvents) > 0 {
			if err := s.backend.WriteConnectEvents(jobID, jobBatch.ConnectEvents); err != nil {
				log.Error("Failed to write connect events", "job_uuid", jobID, "error", err)
				s.writeErrors.Add(1)
			} else {
				log.Info("Wrote connect events", "job_uuid", jobID, "count", len(jobBatch.ConnectEvents))
			}
		}

		if len(jobBatch.FileEvents) > 0 {
			if err := s.backend.WriteFileEvents(jobID, jobBatch.FileEvents); err != nil {
				log.Error("Failed to write file events", "job_uuid", jobID, "error", err)
				s.writeErrors.Add(1)
			} else {
				log.Info("Wrote file events", "job_uuid", jobID, "count", len(jobBatch.FileEvents))
			}
		}

		if len(jobBatch.AcceptEvents) > 0 {
			if err := s.backend.WriteAcceptEvents(jobID, jobBatch.AcceptEvents); err != nil {
				log.Error("Failed to write accept events", "job_uuid", jobID, "error", err)
				s.writeErrors.Add(1)
			} else {
				log.Info("Wrote accept events", "job_uuid", jobID, "count", len(jobBatch.AcceptEvents))
			}
		}

		if len(jobBatch.SocketDataEvents) > 0 {
			if err := s.backend.WriteSocketDataEvents(jobID, jobBatch.SocketDataEvents); err != nil {
				log.Error("Failed to write socket data events", "job_uuid", jobID, "error", err)
				s.writeErrors.Add(1)
			} else {
				log.Info("Wrote socket data events", "job_uuid", jobID, "count", len(jobBatch.SocketDataEvents))
			}
		}

		if len(jobBatch.MmapEvents) > 0 {
			if err := s.backend.WriteMmapEvents(jobID, jobBatch.MmapEvents); err != nil {
				log.Error("Failed to write mmap events", "job_uuid", jobID, "error", err)
				s.writeErrors.Add(1)
			} else {
				log.Info("Wrote mmap events", "job_uuid", jobID, "count", len(jobBatch.MmapEvents))
			}
		}

		if len(jobBatch.MprotectEvents) > 0 {
			if err := s.backend.WriteMprotectEvents(jobID, jobBatch.MprotectEvents); err != nil {
				log.Error("Failed to write mprotect events", "job_uuid", jobID, "error", err)
				s.writeErrors.Add(1)
			} else {
				log.Info("Wrote mprotect events", "job_uuid", jobID, "count", len(jobBatch.MprotectEvents))
			}
		}
	}
}

// JobBatch groups messages by job
type JobBatch struct {
	JobUUID          string
	Logs             []*ipcpb.LogLine
	Metrics          []*ipcpb.Metric
	ExecEvents       []*ipcpb.ExecEvent
	ConnectEvents    []*ipcpb.ConnectEvent
	FileEvents       []*ipcpb.FileEvent
	AcceptEvents     []*ipcpb.AcceptEvent
	SocketDataEvents []*ipcpb.SocketDataEvent
	MmapEvents       []*ipcpb.MmapEvent
	MprotectEvents   []*ipcpb.MprotectEvent
}
