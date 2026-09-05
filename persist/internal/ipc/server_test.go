package ipc

import (
	"context"
	"encoding/binary"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	ipcpb "github.com/ehsaniara/joblet/internal/proto/gen/ipc"
	"github.com/ehsaniara/joblet/persist/internal/config"
	"github.com/ehsaniara/joblet/persist/internal/storage/storagefakes"
	"github.com/ehsaniara/joblet/pkg/logger"
)

// waitFor polls cond until it holds or timeout passes; async flush timing
// varies under load, so tests wait on outcomes instead of fixed sleeps
func waitFor(timeout time.Duration, cond func() bool) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return cond()
}

func TestNewServer(t *testing.T) {
	cfg := &config.IPCConfig{
		Socket:         "/tmp/test.sock",
		ReadBuffer:     262144,
		MaxMessageSize: 10485760,
	}
	backend := &storagefakes.FakeBackend{}
	log := logger.New()

	server := NewServer(cfg, backend, log)

	if server == nil {
		t.Fatal("Expected server to be created, got nil")
	}

	if server.config != cfg {
		t.Error("Server config not set correctly")
	}

	if server.backend != backend {
		t.Error("Server backend not set correctly")
	}

	if server.writePipe == nil {
		t.Error("Write pipe not initialized")
	}
}

func TestServerStartStop(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

	cfg := &config.IPCConfig{
		Socket:         socketPath,
		ReadBuffer:     262144,
		MaxMessageSize: 10485760,
	}
	backend := &storagefakes.FakeBackend{}
	log := logger.New()

	server := NewServer(cfg, backend, log)

	ctx := context.Background()
	err := server.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// Verify socket was created
	if _, err := os.Stat(socketPath); os.IsNotExist(err) {
		t.Error("Socket file was not created")
	}

	// Stop the server
	err = server.Stop()
	if err != nil {
		t.Errorf("Failed to stop server: %v", err)
	}
}

func TestServerReceiveLogMessage(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

	cfg := &config.IPCConfig{
		Socket:         socketPath,
		ReadBuffer:     262144,
		MaxMessageSize: 10485760,
	}
	backend := &storagefakes.FakeBackend{}
	log := logger.New()

	server := NewServer(cfg, backend, log)

	ctx := context.Background()
	err := server.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Connect to the server
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	// Create a test log message
	logLine := &ipcpb.LogLine{
		JobUuid:   "test-job-123",
		Stream:    ipcpb.StreamType_STREAM_TYPE_STDOUT,
		Timestamp: time.Now().UnixNano(),
		Sequence:  1,
		Content:   []byte("Test log message"),
	}

	logData, err := proto.Marshal(logLine)
	if err != nil {
		t.Fatalf("Failed to marshal log line: %v", err)
	}

	ipcMsg := &ipcpb.IPCMessage{
		JobUuid: "test-job-123",
		Type:    ipcpb.MessageType_MESSAGE_TYPE_LOG,
		Data:    logData,
	}

	msgData, err := proto.Marshal(ipcMsg)
	if err != nil {
		t.Fatalf("Failed to marshal IPC message: %v", err)
	}

	// Send length prefix + message
	lengthBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBuf, uint32(len(msgData)))

	if _, err := conn.Write(lengthBuf); err != nil {
		t.Fatalf("Failed to write length: %v", err)
	}

	if _, err := conn.Write(msgData); err != nil {
		t.Fatalf("Failed to write message: %v", err)
	}

	// Verify backend was called
	if !waitFor(5*time.Second, func() bool { return backend.WriteLogsCallCount() > 0 }) {
		t.Error("Expected WriteLogs to be called on backend")
	}

	if backend.WriteLogsCallCount() > 0 {
		jobID, logs := backend.WriteLogsArgsForCall(0)
		if jobID != "test-job-123" {
			t.Errorf("Expected job ID 'test-job-123', got '%s'", jobID)
		}
		if len(logs) != 1 {
			t.Errorf("Expected 1 log, got %d", len(logs))
		}
		if len(logs) > 0 && string(logs[0].Content) != "Test log message" {
			t.Errorf("Expected log content 'Test log message', got '%s'", string(logs[0].Content))
		}
	}
}

func TestServerReceiveMetricMessage(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

	cfg := &config.IPCConfig{
		Socket:         socketPath,
		ReadBuffer:     262144,
		MaxMessageSize: 10485760,
	}
	backend := &storagefakes.FakeBackend{}
	log := logger.New()

	server := NewServer(cfg, backend, log)

	ctx := context.Background()
	err := server.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	// Create a test metric message
	metric := &ipcpb.Metric{
		JobUuid:   "test-job-456",
		Timestamp: time.Now().UnixNano(),
		Sequence:  1,
		Data: &ipcpb.MetricData{
			CpuUsage:    50.5,
			MemoryUsage: 1024000,
			GpuUsage:    75.0,
		},
	}

	metricData, err := proto.Marshal(metric)
	if err != nil {
		t.Fatalf("Failed to marshal metric: %v", err)
	}

	ipcMsg := &ipcpb.IPCMessage{
		JobUuid: "test-job-456",
		Type:    ipcpb.MessageType_MESSAGE_TYPE_METRIC,
		Data:    metricData,
	}

	msgData, err := proto.Marshal(ipcMsg)
	if err != nil {
		t.Fatalf("Failed to marshal IPC message: %v", err)
	}

	lengthBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBuf, uint32(len(msgData)))

	if _, err := conn.Write(lengthBuf); err != nil {
		t.Fatalf("Failed to write length: %v", err)
	}

	if _, err := conn.Write(msgData); err != nil {
		t.Fatalf("Failed to write message: %v", err)
	}

	// Verify backend was called
	if !waitFor(5*time.Second, func() bool { return backend.WriteMetricsCallCount() > 0 }) {
		t.Error("Expected WriteMetrics to be called on backend")
	}

	if backend.WriteMetricsCallCount() > 0 {
		jobID, metrics := backend.WriteMetricsArgsForCall(0)
		if jobID != "test-job-456" {
			t.Errorf("Expected job ID 'test-job-456', got '%s'", jobID)
		}
		if len(metrics) != 1 {
			t.Errorf("Expected 1 metric, got %d", len(metrics))
		}
		if len(metrics) > 0 && metrics[0].Data.CpuUsage != 50.5 {
			t.Errorf("Expected CPU usage 50.5, got %f", metrics[0].Data.CpuUsage)
		}
	}
}

func TestServerMessageTooLarge(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

	cfg := &config.IPCConfig{
		Socket:         socketPath,
		ReadBuffer:     262144,
		MaxMessageSize: 1024, // Small max size for testing
	}
	backend := &storagefakes.FakeBackend{}
	log := logger.New()

	server := NewServer(cfg, backend, log)

	ctx := context.Background()
	err := server.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	// Send a message size that exceeds the limit
	lengthBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBuf, 2048) // Larger than MaxMessageSize

	if _, err := conn.Write(lengthBuf); err != nil {
		t.Fatalf("Failed to write length: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Connection should be closed by server
	testBuf := make([]byte, 1)
	conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, err = conn.Read(testBuf)
	if err == nil {
		t.Error("Expected connection to be closed by server")
	}
}

func TestServerBatchProcessing(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

	cfg := &config.IPCConfig{
		Socket:         socketPath,
		ReadBuffer:     262144,
		MaxMessageSize: 10485760,
	}
	backend := &storagefakes.FakeBackend{}
	log := logger.New()

	server := NewServer(cfg, backend, log)

	ctx := context.Background()
	err := server.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	// Send multiple messages rapidly
	for i := 0; i < 10; i++ {
		logLine := &ipcpb.LogLine{
			JobUuid:   "batch-job",
			Stream:    ipcpb.StreamType_STREAM_TYPE_STDOUT,
			Timestamp: time.Now().UnixNano(),
			Sequence:  uint64(i),
			Content:   []byte("Batch log message"),
		}

		logData, _ := proto.Marshal(logLine)
		ipcMsg := &ipcpb.IPCMessage{
			JobUuid: "batch-job",
			Type:    ipcpb.MessageType_MESSAGE_TYPE_LOG,
			Data:    logData,
		}

		msgData, _ := proto.Marshal(ipcMsg)
		lengthBuf := make([]byte, 4)
		binary.BigEndian.PutUint32(lengthBuf, uint32(len(msgData)))

		conn.Write(lengthBuf)
		conn.Write(msgData)
	}

	// The 10 logs may arrive across several flushes; wait on the total
	countLogs := func() int {
		total := 0
		for i := 0; i < backend.WriteLogsCallCount(); i++ {
			_, logs := backend.WriteLogsArgsForCall(i)
			total += len(logs)
		}
		return total
	}
	waitFor(5*time.Second, func() bool { return countLogs() == 10 })

	// Verify batching occurred (should be fewer calls than messages due to batching)
	if backend.WriteLogsCallCount() == 0 {
		t.Error("Expected at least one batch write")
	}

	// Verify all logs were written
	if totalLogs := countLogs(); totalLogs != 10 {
		t.Errorf("Expected 10 total logs, got %d", totalLogs)
	}
}

func TestServerConfigurablePipeline(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

	cfg := &config.IPCConfig{
		Socket:              socketPath,
		ReadBuffer:          262144,
		MaxMessageSize:      10485760,
		BufferSize:          50,      // Small buffer for testing
		WorkerCount:         2,       // 2 workers
		BatchSize:           5,       // Small batch size
		BackpressureTimeout: 1,       // 1 second timeout
		BackpressureMode:    "block", // Block mode
	}
	backend := &storagefakes.FakeBackend{}
	log := logger.New()

	server := NewServer(cfg, backend, log)

	// Verify server was created with custom settings
	if server.batchSize != 5 {
		t.Errorf("Expected batchSize 5, got %d", server.batchSize)
	}

	if server.backpressureMode != "block" {
		t.Errorf("Expected backpressureMode 'block', got '%s'", server.backpressureMode)
	}

	if server.backpressureTimeout != time.Second {
		t.Errorf("Expected backpressureTimeout 1s, got %v", server.backpressureTimeout)
	}

	// Verify write pipe capacity
	if cap(server.writePipe) != 50 {
		t.Errorf("Expected writePipe capacity 50, got %d", cap(server.writePipe))
	}
}

func TestServerDefaultPipelineSettings(t *testing.T) {
	cfg := &config.IPCConfig{
		Socket:         "/tmp/test.sock",
		ReadBuffer:     262144,
		MaxMessageSize: 10485760,
		// Pipeline settings not specified - should use defaults
	}
	backend := &storagefakes.FakeBackend{}
	log := logger.New()

	server := NewServer(cfg, backend, log)

	// Verify defaults are applied
	if server.batchSize != 100 {
		t.Errorf("Expected default batchSize 100, got %d", server.batchSize)
	}

	if server.backpressureMode != "block" {
		t.Errorf("Expected default backpressureMode 'block', got '%s'", server.backpressureMode)
	}

	if server.backpressureTimeout != 5*time.Second {
		t.Errorf("Expected default backpressureTimeout 5s, got %v", server.backpressureTimeout)
	}

	if cap(server.writePipe) != 100000 {
		t.Errorf("Expected default writePipe capacity 100000, got %d", cap(server.writePipe))
	}
}

func TestServerBackpressureBlockMode(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

	cfg := &config.IPCConfig{
		Socket:              socketPath,
		ReadBuffer:          262144,
		MaxMessageSize:      10485760,
		BufferSize:          5,   // Very small buffer to trigger backpressure
		WorkerCount:         1,   // Single worker
		BatchSize:           100, // Large batch size so messages accumulate
		BackpressureTimeout: 1,
		BackpressureMode:    "block",
	}

	// Create a slow backend that takes time to process
	backend := &storagefakes.FakeBackend{}
	backend.WriteLogsStub = func(jobID string, logs []*ipcpb.LogLine) error {
		time.Sleep(50 * time.Millisecond) // Simulate slow write
		return nil
	}
	log := logger.New()

	server := NewServer(cfg, backend, log)

	ctx := context.Background()
	err := server.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	// Send messages - in block mode, they should all eventually be processed
	messageCount := 10
	for i := 0; i < messageCount; i++ {
		logLine := &ipcpb.LogLine{
			JobUuid:   "block-test-job",
			Stream:    ipcpb.StreamType_STREAM_TYPE_STDOUT,
			Timestamp: time.Now().UnixNano(),
			Sequence:  uint64(i),
			Content:   []byte("Block mode test"),
		}

		logData, _ := proto.Marshal(logLine)
		ipcMsg := &ipcpb.IPCMessage{
			JobUuid: "block-test-job",
			Type:    ipcpb.MessageType_MESSAGE_TYPE_LOG,
			Data:    logData,
		}

		msgData, _ := proto.Marshal(ipcMsg)
		lengthBuf := make([]byte, 4)
		binary.BigEndian.PutUint32(lengthBuf, uint32(len(msgData)))

		conn.Write(lengthBuf)
		conn.Write(msgData)
	}

	// Wait for processing
	time.Sleep(1 * time.Second)

	// In block mode, no messages should be dropped
	if server.msgsDropped.Load() != 0 {
		t.Errorf("Expected 0 dropped messages in block mode, got %d", server.msgsDropped.Load())
	}

	// All messages should be received
	if server.msgsReceived.Load() != uint64(messageCount) {
		t.Errorf("Expected %d messages received, got %d", messageCount, server.msgsReceived.Load())
	}
}

func TestServerBackpressureDropMode(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

	cfg := &config.IPCConfig{
		Socket:              socketPath,
		ReadBuffer:          262144,
		MaxMessageSize:      10485760,
		BufferSize:          2, // Very small buffer
		WorkerCount:         1,
		BatchSize:           100,    // Large batch so messages accumulate
		BackpressureTimeout: 1,      // 1 second timeout before drop
		BackpressureMode:    "drop", // Drop mode
	}

	// Create a very slow backend
	backend := &storagefakes.FakeBackend{}
	backend.WriteLogsStub = func(jobID string, logs []*ipcpb.LogLine) error {
		time.Sleep(2 * time.Second) // Very slow - will cause backpressure
		return nil
	}
	log := logger.New()

	server := NewServer(cfg, backend, log)

	// Verify drop mode is set
	if server.backpressureMode != "drop" {
		t.Errorf("Expected backpressureMode 'drop', got '%s'", server.backpressureMode)
	}
}

func TestServerMetricsTracking(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

	cfg := &config.IPCConfig{
		Socket:         socketPath,
		ReadBuffer:     262144,
		MaxMessageSize: 10485760,
	}
	backend := &storagefakes.FakeBackend{}
	log := logger.New()

	server := NewServer(cfg, backend, log)

	ctx := context.Background()
	err := server.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	// Send a test message
	logLine := &ipcpb.LogLine{
		JobUuid:   "metrics-test-job",
		Stream:    ipcpb.StreamType_STREAM_TYPE_STDOUT,
		Timestamp: time.Now().UnixNano(),
		Sequence:  1,
		Content:   []byte("Metrics test message"),
	}

	logData, _ := proto.Marshal(logLine)
	ipcMsg := &ipcpb.IPCMessage{
		JobUuid: "metrics-test-job",
		Type:    ipcpb.MessageType_MESSAGE_TYPE_LOG,
		Data:    logData,
	}

	msgData, _ := proto.Marshal(ipcMsg)
	lengthBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBuf, uint32(len(msgData)))

	conn.Write(lengthBuf)
	conn.Write(msgData)

	time.Sleep(200 * time.Millisecond)

	// Verify metrics are tracked
	if server.msgsReceived.Load() != 1 {
		t.Errorf("Expected 1 message received, got %d", server.msgsReceived.Load())
	}

	if server.bytesReceived.Load() == 0 {
		t.Error("Expected bytes received to be tracked")
	}

	// No backpressure should have occurred with default buffer
	if server.backpressureEvents.Load() != 0 {
		t.Errorf("Expected 0 backpressure events, got %d", server.backpressureEvents.Load())
	}

	if server.msgsDropped.Load() != 0 {
		t.Errorf("Expected 0 dropped messages, got %d", server.msgsDropped.Load())
	}
}
