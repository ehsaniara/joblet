package ipc

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/protobuf/proto"

	ipcpb "github.com/ehsaniara/joblet/internal/proto/gen/ipc"
	"github.com/ehsaniara/joblet/pkg/logger"
)

// Writer sends messages to joblet-persist via IPC
type Writer struct {
	socket    string
	conn      net.Conn
	connMu    sync.RWMutex
	connected atomic.Bool

	// Write channel (non-blocking)
	writeChan  chan *ipcpb.IPCMessage
	bufferSize int

	// Reconnection
	reconnect *reconnectManager

	// Metrics
	msgsSent    atomic.Uint64
	msgsDropped atomic.Uint64
	writeErrors atomic.Uint64

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	logger *logger.Logger
}

// Config for IPC writer
type Config struct {
	Socket         string
	BufferSize     int
	ReconnectDelay time.Duration
	MaxReconnects  int // 0 = infinite
}

// NewWriter creates a new IPC writer
func NewWriter(cfg *Config, log *logger.Logger) *Writer {
	ctx, cancel := context.WithCancel(context.Background())

	w := &Writer{
		socket:     cfg.Socket,
		writeChan:  make(chan *ipcpb.IPCMessage, cfg.BufferSize),
		bufferSize: cfg.BufferSize,
		reconnect:  newReconnectManager(cfg.ReconnectDelay, cfg.MaxReconnects),
		ctx:        ctx,
		cancel:     cancel,
		logger:     log.WithField("component", "ipc-writer"),
	}

	// Start background workers
	w.wg.Add(2)
	go w.writeLoop()
	go w.reconnectLoop()

	return w
}

// WriteLog sends a log line (non-blocking)
func (w *Writer) WriteLog(jobID string, stream ipcpb.StreamType, timestamp int64, sequence uint64, content []byte) error {
	// Create log line
	logLine := &ipcpb.LogLine{
		JobId:     jobID,
		Stream:    stream,
		Timestamp: timestamp,
		Sequence:  sequence,
		Content:   content,
	}

	// Marshal log line
	data, err := proto.Marshal(logLine)
	if err != nil {
		return fmt.Errorf("failed to marshal log line: %w", err)
	}

	// Create IPC message
	msg := &ipcpb.IPCMessage{
		Version:   1,
		Type:      ipcpb.MessageType_MESSAGE_TYPE_LOG,
		JobId:     jobID,
		Timestamp: timestamp,
		Sequence:  sequence,
		Data:      data,
	}

	return w.write(msg)
}

// WriteMetric sends a metric (non-blocking)
func (w *Writer) WriteMetric(jobID string, timestamp int64, sequence uint64, data *ipcpb.MetricData) error {
	// Create metric
	metric := &ipcpb.Metric{
		JobId:     jobID,
		Timestamp: timestamp,
		Sequence:  sequence,
		Data:      data,
	}

	// Marshal metric
	metricData, err := proto.Marshal(metric)
	if err != nil {
		return fmt.Errorf("failed to marshal metric: %w", err)
	}

	// Create IPC message
	msg := &ipcpb.IPCMessage{
		Version:   1,
		Type:      ipcpb.MessageType_MESSAGE_TYPE_METRIC,
		JobId:     jobID,
		Timestamp: timestamp,
		Sequence:  sequence,
		Data:      metricData,
	}

	return w.write(msg)
}

// write sends a message (non-blocking)
func (w *Writer) write(msg *ipcpb.IPCMessage) error {
	if !w.connected.Load() {
		w.msgsDropped.Add(1)
		return fmt.Errorf("not connected to persist service")
	}

	select {
	case w.writeChan <- msg:
		return nil
	default:
		// Channel full - drop message
		w.msgsDropped.Add(1)
		w.logger.Warn("IPC write channel full, dropping message", "jobID", msg.JobId)
		return fmt.Errorf("write channel full")
	}
}

// writeLoop processes the write queue
func (w *Writer) writeLoop() {
	defer w.wg.Done()

	lengthBuf := make([]byte, 4)

	for {
		select {
		case <-w.ctx.Done():
			return
		case msg := <-w.writeChan:
			if err := w.sendMessage(msg, lengthBuf); err != nil {
				w.writeErrors.Add(1)
				w.logger.Error("Failed to send IPC message", "error", err, "jobID", msg.JobId)

				// Mark as disconnected on write error
				w.connected.Store(false)
				w.closeConnection()
			} else {
				w.msgsSent.Add(1)
			}
		}
	}
}

// sendMessage sends a single message to the socket
func (w *Writer) sendMessage(msg *ipcpb.IPCMessage, lengthBuf []byte) error {
	w.connMu.RLock()
	conn := w.conn
	w.connMu.RUnlock()

	if conn == nil {
		return fmt.Errorf("no connection")
	}

	// Marshal protobuf
	data, err := proto.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Write length prefix
	binary.BigEndian.PutUint32(lengthBuf, uint32(len(data)))
	if _, err := conn.Write(lengthBuf); err != nil {
		return fmt.Errorf("failed to write length: %w", err)
	}

	// Write message
	if _, err := conn.Write(data); err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	return nil
}

// reconnectLoop handles reconnection logic
func (w *Writer) reconnectLoop() {
	defer w.wg.Done()

	// Initial connection attempt
	if err := w.connect(); err != nil {
		w.logger.Warn("Initial connection to persist failed, will retry", "error", err)
	}

	ticker := time.NewTicker(w.reconnect.delay)
	defer ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			if !w.connected.Load() {
				if !w.reconnect.shouldRetry() {
					w.logger.Error("Max reconnection attempts reached, giving up")
					return
				}

				if err := w.connect(); err != nil {
					w.logger.Warn("Reconnection attempt failed",
						"error", err,
						"attempt", w.reconnect.attempts)
				} else {
					w.reconnect.reset()
				}
			}
		}
	}
}

// connect establishes connection to persist service
func (w *Writer) connect() error {
	w.connMu.Lock()
	defer w.connMu.Unlock()

	// Close existing connection
	if w.conn != nil {
		w.conn.Close()
		w.conn = nil
	}

	// Dial Unix socket
	conn, err := net.Dial("unix", w.socket)
	if err != nil {
		w.reconnect.recordAttempt()
		return fmt.Errorf("failed to connect to %s: %w", w.socket, err)
	}

	// Set socket buffer
	if uc, ok := conn.(*net.UnixConn); ok {
		if err := uc.SetWriteBuffer(8 * 1024 * 1024); err != nil {
			w.logger.Warn("Failed to set write buffer size", "error", err)
		}
	}

	w.conn = conn
	w.connected.Store(true)

	w.logger.Info("Connected to joblet-persist", "socket", w.socket)

	return nil
}

// closeConnection closes the current connection
func (w *Writer) closeConnection() {
	w.connMu.Lock()
	defer w.connMu.Unlock()

	if w.conn != nil {
		w.conn.Close()
		w.conn = nil
	}
}

// Close stops the writer
func (w *Writer) Close() error {
	w.logger.Info("Closing IPC writer")
	w.cancel()
	w.wg.Wait()
	w.closeConnection()
	close(w.writeChan)

	w.logger.Info("IPC writer closed",
		"msgsSent", w.msgsSent.Load(),
		"msgsDropped", w.msgsDropped.Load(),
		"writeErrors", w.writeErrors.Load())

	return nil
}

// Stats returns writer statistics
func (w *Writer) Stats() WriterStats {
	return WriterStats{
		Connected:   w.connected.Load(),
		MsgsSent:    w.msgsSent.Load(),
		MsgsDropped: w.msgsDropped.Load(),
		WriteErrors: w.writeErrors.Load(),
	}
}

// WriterStats represents writer statistics
type WriterStats struct {
	Connected   bool
	MsgsSent    uint64
	MsgsDropped uint64
	WriteErrors uint64
}

// reconnectManager handles reconnection logic
type reconnectManager struct {
	delay       time.Duration
	maxAttempts int
	attempts    int
	mu          sync.Mutex
}

func newReconnectManager(delay time.Duration, maxAttempts int) *reconnectManager {
	return &reconnectManager{
		delay:       delay,
		maxAttempts: maxAttempts,
	}
}

func (rm *reconnectManager) shouldRetry() bool {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if rm.maxAttempts == 0 {
		return true // Infinite retries
	}

	return rm.attempts < rm.maxAttempts
}

func (rm *reconnectManager) recordAttempt() {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.attempts++
}

func (rm *reconnectManager) reset() {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.attempts = 0
}
