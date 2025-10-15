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
		JobId:     "test-job-123",
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
		JobId: "test-job-123",
		Type:  ipcpb.MessageType_MESSAGE_TYPE_LOG,
		Data:  logData,
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

	// Wait for message to be processed
	time.Sleep(200 * time.Millisecond)

	// Verify backend was called
	if backend.WriteLogsCallCount() == 0 {
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
		JobId:     "test-job-456",
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
		JobId: "test-job-456",
		Type:  ipcpb.MessageType_MESSAGE_TYPE_METRIC,
		Data:  metricData,
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

	time.Sleep(200 * time.Millisecond)

	// Verify backend was called
	if backend.WriteMetricsCallCount() == 0 {
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

func TestServerGetStats(t *testing.T) {
	cfg := &config.IPCConfig{
		Socket:         "/tmp/test.sock",
		ReadBuffer:     262144,
		MaxMessageSize: 10485760,
	}
	backend := &storagefakes.FakeBackend{}
	log := logger.New()

	server := NewServer(cfg, backend, log)

	// Initial stats should be zero
	stats := server.GetStats()
	if stats.MessagesReceived != 0 {
		t.Errorf("Expected 0 messages received, got %d", stats.MessagesReceived)
	}
	if stats.BytesReceived != 0 {
		t.Errorf("Expected 0 bytes received, got %d", stats.BytesReceived)
	}
	if stats.WriteErrors != 0 {
		t.Errorf("Expected 0 write errors, got %d", stats.WriteErrors)
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
			JobId:     "batch-job",
			Stream:    ipcpb.StreamType_STREAM_TYPE_STDOUT,
			Timestamp: time.Now().UnixNano(),
			Sequence:  uint64(i),
			Content:   []byte("Batch log message"),
		}

		logData, _ := proto.Marshal(logLine)
		ipcMsg := &ipcpb.IPCMessage{
			JobId: "batch-job",
			Type:  ipcpb.MessageType_MESSAGE_TYPE_LOG,
			Data:  logData,
		}

		msgData, _ := proto.Marshal(ipcMsg)
		lengthBuf := make([]byte, 4)
		binary.BigEndian.PutUint32(lengthBuf, uint32(len(msgData)))

		conn.Write(lengthBuf)
		conn.Write(msgData)
	}

	// Wait for batch processing
	time.Sleep(500 * time.Millisecond)

	// Verify batching occurred (should be fewer calls than messages due to batching)
	callCount := backend.WriteLogsCallCount()
	if callCount == 0 {
		t.Error("Expected at least one batch write")
	}

	// Verify all logs were written
	totalLogs := 0
	for i := 0; i < callCount; i++ {
		_, logs := backend.WriteLogsArgsForCall(i)
		totalLogs += len(logs)
	}

	if totalLogs != 10 {
		t.Errorf("Expected 10 total logs, got %d", totalLogs)
	}
}
