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

// Writer sends messages to persist via IPC
type Writer struct {
	socket    string
	conn      net.Conn
	connMu    sync.RWMutex
	connected atomic.Bool

	// Write channel with backpressure
	writeChan    chan *ipcpb.IPCMessage
	bufferSize   int
	writeTimeout time.Duration

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
	WriteTimeout   time.Duration // Backpressure timeout (0 = 5s default)
	ReconnectDelay time.Duration
	MaxReconnects  int // 0 = infinite
}

// NewWriter creates a new IPC writer
func NewWriter(cfg *Config, log *logger.Logger) *Writer {
	ctx, cancel := context.WithCancel(context.Background())

	writeTimeout := cfg.WriteTimeout
	if writeTimeout == 0 {
		writeTimeout = 5 * time.Second // Default 5s backpressure timeout
	}

	w := &Writer{
		socket:       cfg.Socket,
		writeChan:    make(chan *ipcpb.IPCMessage, cfg.BufferSize),
		bufferSize:   cfg.BufferSize,
		writeTimeout: writeTimeout,
		reconnect:    newReconnectManager(cfg.ReconnectDelay, cfg.MaxReconnects),
		ctx:          ctx,
		cancel:       cancel,
		logger:       log.WithField("component", "ipc-writer"),
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
		JobUuid:   jobID,
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
		JobUuid:   jobID,
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
		JobUuid:   jobID,
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
		JobUuid:   jobID,
		Timestamp: timestamp,
		Sequence:  sequence,
		Data:      metricData,
	}

	return w.write(msg)
}

// WriteExecEvent sends an eBPF process execution event (non-blocking)
func (w *Writer) WriteExecEvent(event *ipcpb.ExecEvent) error {
	// Marshal exec event
	data, err := proto.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal exec event: %w", err)
	}

	// Create IPC message
	msg := &ipcpb.IPCMessage{
		Version:   1,
		Type:      ipcpb.MessageType_MESSAGE_TYPE_EXEC_EVENT,
		JobUuid:   event.JobUuid,
		Timestamp: event.Timestamp,
		Sequence:  event.Sequence,
		Data:      data,
	}

	return w.write(msg)
}

// WriteConnectEvent sends an eBPF network connection event (non-blocking)
func (w *Writer) WriteConnectEvent(event *ipcpb.ConnectEvent) error {
	// Marshal connect event
	data, err := proto.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal connect event: %w", err)
	}

	// Create IPC message
	msg := &ipcpb.IPCMessage{
		Version:   1,
		Type:      ipcpb.MessageType_MESSAGE_TYPE_CONNECT_EVENT,
		JobUuid:   event.JobUuid,
		Timestamp: event.Timestamp,
		Sequence:  event.Sequence,
		Data:      data,
	}

	return w.write(msg)
}

// WriteAcceptEvent sends an eBPF incoming connection accept event (non-blocking)
func (w *Writer) WriteAcceptEvent(event *ipcpb.AcceptEvent) error {
	data, err := proto.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal accept event: %w", err)
	}

	msg := &ipcpb.IPCMessage{
		Version:   1,
		Type:      ipcpb.MessageType_MESSAGE_TYPE_ACCEPT_EVENT,
		JobUuid:   event.JobUuid,
		Timestamp: event.Timestamp,
		Sequence:  event.Sequence,
		Data:      data,
	}

	return w.write(msg)
}

// WriteSocketDataEvent sends an eBPF sendto/recvfrom event (non-blocking)
func (w *Writer) WriteSocketDataEvent(event *ipcpb.SocketDataEvent) error {
	data, err := proto.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal socket data event: %w", err)
	}

	msg := &ipcpb.IPCMessage{
		Version:   1,
		Type:      ipcpb.MessageType_MESSAGE_TYPE_SOCKET_DATA_EVENT,
		JobUuid:   event.JobUuid,
		Timestamp: event.Timestamp,
		Sequence:  event.Sequence,
		Data:      data,
	}

	return w.write(msg)
}

// WriteMmapEvent sends an eBPF memory mapping event (non-blocking)
func (w *Writer) WriteMmapEvent(event *ipcpb.MmapEvent) error {
	data, err := proto.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal mmap event: %w", err)
	}

	msg := &ipcpb.IPCMessage{
		Version:   1,
		Type:      ipcpb.MessageType_MESSAGE_TYPE_MMAP_EVENT,
		JobUuid:   event.JobUuid,
		Timestamp: event.Timestamp,
		Sequence:  event.Sequence,
		Data:      data,
	}

	return w.write(msg)
}

// WriteMprotectEvent sends an eBPF memory protection change event (non-blocking)
func (w *Writer) WriteMprotectEvent(event *ipcpb.MprotectEvent) error {
	data, err := proto.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal mprotect event: %w", err)
	}

	msg := &ipcpb.IPCMessage{
		Version:   1,
		Type:      ipcpb.MessageType_MESSAGE_TYPE_MPROTECT_EVENT,
		JobUuid:   event.JobUuid,
		Timestamp: event.Timestamp,
		Sequence:  event.Sequence,
		Data:      data,
	}

	return w.write(msg)
}

// WriteFileEvent sends an eBPF file access event (non-blocking)
func (w *Writer) WriteFileEvent(event *ipcpb.FileEvent) error {
	data, err := proto.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal file event: %w", err)
	}

	msg := &ipcpb.IPCMessage{
		Version:   1,
		Type:      ipcpb.MessageType_MESSAGE_TYPE_FILE_EVENT,
		JobUuid:   event.JobUuid,
		Timestamp: event.Timestamp,
		Sequence:  event.Sequence,
		Data:      data,
	}

	return w.write(msg)
}

// write sends a message with backpressure (blocks up to writeTimeout).
// Messages are queued even while disconnected: the channel acts as a bounded
// buffer during the persist warmup window and across reconnections, and
// writeLoop delivers once the connection is up. Without this, everything a
// job produces in the first seconds after service start is silently lost.
func (w *Writer) write(msg *ipcpb.IPCMessage) error {
	// Try non-blocking first for fast path
	select {
	case w.writeChan <- msg:
		return nil
	default:
		// Channel full - apply backpressure with timeout
	}

	// Blocking write with timeout (backpressure)
	timer := time.NewTimer(w.writeTimeout)
	defer timer.Stop()

	select {
	case w.writeChan <- msg:
		return nil
	case <-timer.C:
		// Timeout - drop message after waiting
		w.msgsDropped.Add(1)
		w.logger.Warn("IPC write timeout (backpressure), dropping message",
			"job_uuid", msg.JobUuid,
			"timeout", w.writeTimeout,
			"queueSize", len(w.writeChan))
		return fmt.Errorf("write timeout after %v", w.writeTimeout)
	case <-w.ctx.Done():
		return w.ctx.Err()
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
			// Hold the message until the connection is up (bounded wait) so
			// queued messages survive the initial persist warmup and brief
			// reconnections instead of being dropped
			if !w.waitForConnection() {
				w.msgsDropped.Add(1)
				w.logger.Warn("Dropping message, persist unavailable past wait cap", "job_uuid", msg.JobUuid)
				continue
			}
			if err := w.sendMessage(msg, lengthBuf); err != nil {
				w.writeErrors.Add(1)
				w.logger.Error("Failed to send IPC message", "error", err, "job_uuid", msg.JobUuid)

				// Mark as disconnected on write error
				w.connected.Store(false)
				w.closeConnection()
			} else {
				w.msgsSent.Add(1)
			}
		}
	}
}

// waitForConnection blocks until the writer is connected, a wait cap elapses,
// or the writer shuts down. Returns true when connected.
func (w *Writer) waitForConnection() bool {
	if w.connected.Load() {
		return true
	}

	const connectionWaitCap = 30 * time.Second
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.NewTimer(connectionWaitCap)
	defer deadline.Stop()

	for {
		select {
		case <-w.ctx.Done():
			return false
		case <-deadline.C:
			return false
		case <-ticker.C:
			if w.connected.Load() {
				return true
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

	w.logger.Info("Connected to persist", "socket", w.socket)

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
