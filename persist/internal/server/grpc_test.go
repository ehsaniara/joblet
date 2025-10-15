package server

import (
	"context"
	"errors"
	"testing"
	"time"

	persistpb "github.com/ehsaniara/joblet-proto/v2/gen"
	"github.com/ehsaniara/joblet/internal/joblet/auth/authfakes"
	ipcpb "github.com/ehsaniara/joblet/internal/proto/gen/ipc"
	"github.com/ehsaniara/joblet/persist/internal/config"
	"github.com/ehsaniara/joblet/persist/internal/storage"
	"github.com/ehsaniara/joblet/persist/internal/storage/storagefakes"
	"github.com/ehsaniara/joblet/pkg/logger"
)

var errUnauthorized = errors.New("unauthorized")

func TestNewGRPCServer(t *testing.T) {
	cfg := &config.ServerConfig{
		GRPCAddress:    ":50053",
		MaxConnections: 100,
	}
	backend := &storagefakes.FakeBackend{}
	log := logger.New()
	authorization := &authfakes.FakeGRPCAuthorization{}
	security := &config.SecurityConfig{}

	server := NewGRPCServer(cfg, backend, log, authorization, security)

	if server == nil {
		t.Fatal("Expected server to be created, got nil")
	}

	if server.config != cfg {
		t.Error("Server config not set correctly")
	}

	if server.backend != backend {
		t.Error("Server backend not set correctly")
	}
}

func TestStreamTypeConversion(t *testing.T) {
	tests := []struct {
		name     string
		ipcType  ipcpb.StreamType
		genType  persistpb.StreamType
		expected string
	}{
		{
			name:     "stdout",
			ipcType:  ipcpb.StreamType_STREAM_TYPE_STDOUT,
			genType:  persistpb.StreamType_STREAM_TYPE_STDOUT,
			expected: "stdout",
		},
		{
			name:     "stderr",
			ipcType:  ipcpb.StreamType_STREAM_TYPE_STDERR,
			genType:  persistpb.StreamType_STREAM_TYPE_STDERR,
			expected: "stderr",
		},
		{
			name:     "unspecified",
			ipcType:  ipcpb.StreamType_STREAM_TYPE_UNSPECIFIED,
			genType:  persistpb.StreamType_STREAM_TYPE_UNSPECIFIED,
			expected: "unspecified",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test IPC to Gen conversion
			converted := streamTypeIPCToGen(tt.ipcType)
			if converted != tt.genType {
				t.Errorf("streamTypeIPCToGen(%v) = %v, want %v", tt.ipcType, converted, tt.genType)
			}

			// Test Gen to IPC conversion
			convertedBack := streamTypeGenToIPC(tt.genType)
			if convertedBack != tt.ipcType {
				t.Errorf("streamTypeGenToIPC(%v) = %v, want %v", tt.genType, convertedBack, tt.ipcType)
			}
		})
	}
}

func TestLogLineConversion(t *testing.T) {
	now := time.Now().UnixNano()
	ipcLog := &ipcpb.LogLine{
		JobId:     "test-job",
		Stream:    ipcpb.StreamType_STREAM_TYPE_STDOUT,
		Timestamp: now,
		Sequence:  42,
		Content:   []byte("test content"),
	}

	genLog := logLineIPCToGen(ipcLog)

	if genLog.JobId != ipcLog.JobId {
		t.Errorf("JobId mismatch: got %s, want %s", genLog.JobId, ipcLog.JobId)
	}

	if genLog.Timestamp != ipcLog.Timestamp {
		t.Errorf("Timestamp mismatch: got %d, want %d", genLog.Timestamp, ipcLog.Timestamp)
	}

	if genLog.Sequence != ipcLog.Sequence {
		t.Errorf("Sequence mismatch: got %d, want %d", genLog.Sequence, ipcLog.Sequence)
	}

	if string(genLog.Content) != string(ipcLog.Content) {
		t.Errorf("Content mismatch: got %s, want %s", string(genLog.Content), string(ipcLog.Content))
	}

	// Test nil handling
	nilLog := logLineIPCToGen(nil)
	if nilLog != nil {
		t.Error("Expected nil for nil input")
	}
}

func TestMetricConversion(t *testing.T) {
	now := time.Now().UnixNano()
	ipcMetric := &ipcpb.Metric{
		JobId:     "test-job",
		Timestamp: now,
		Sequence:  100,
		Data: &ipcpb.MetricData{
			CpuUsage:    45.5,
			MemoryUsage: 2048000,
			GpuUsage:    80.0,
			DiskIo: &ipcpb.DiskIO{
				ReadBytes:  1024,
				WriteBytes: 2048,
				ReadOps:    10,
				WriteOps:   20,
			},
			NetworkIo: &ipcpb.NetworkIO{
				RxBytes:   512,
				TxBytes:   1024,
				RxPackets: 5,
				TxPackets: 10,
			},
		},
	}

	genMetric := metricIPCToGen(ipcMetric)

	if genMetric.JobId != ipcMetric.JobId {
		t.Errorf("JobId mismatch: got %s, want %s", genMetric.JobId, ipcMetric.JobId)
	}

	if genMetric.Timestamp != ipcMetric.Timestamp {
		t.Errorf("Timestamp mismatch: got %d, want %d", genMetric.Timestamp, ipcMetric.Timestamp)
	}

	if genMetric.Data.CpuUsage != ipcMetric.Data.CpuUsage {
		t.Errorf("CPU usage mismatch: got %f, want %f", genMetric.Data.CpuUsage, ipcMetric.Data.CpuUsage)
	}

	if genMetric.Data.MemoryUsage != ipcMetric.Data.MemoryUsage {
		t.Errorf("Memory usage mismatch: got %d, want %d", genMetric.Data.MemoryUsage, ipcMetric.Data.MemoryUsage)
	}

	if genMetric.Data.DiskIo.ReadBytes != ipcMetric.Data.DiskIo.ReadBytes {
		t.Errorf("Disk IO read bytes mismatch: got %d, want %d", genMetric.Data.DiskIo.ReadBytes, ipcMetric.Data.DiskIo.ReadBytes)
	}

	if genMetric.Data.NetworkIo.TxPackets != ipcMetric.Data.NetworkIo.TxPackets {
		t.Errorf("Network IO tx packets mismatch: got %d, want %d", genMetric.Data.NetworkIo.TxPackets, ipcMetric.Data.NetworkIo.TxPackets)
	}

	// Test nil handling
	nilMetric := metricIPCToGen(nil)
	if nilMetric != nil {
		t.Error("Expected nil for nil input")
	}
}

type mockQueryLogsServer struct {
	persistpb.PersistService_QueryLogsServer
	ctx      context.Context
	sentLogs []*persistpb.LogLine
}

func (m *mockQueryLogsServer) Context() context.Context {
	return m.ctx
}

func (m *mockQueryLogsServer) Send(log *persistpb.LogLine) error {
	m.sentLogs = append(m.sentLogs, log)
	return nil
}

func TestQueryLogsAuthorization(t *testing.T) {
	backend := &storagefakes.FakeBackend{}
	log := logger.New()
	authorization := &authfakes.FakeGRPCAuthorization{}

	// Configure authorization to deny
	authorization.AuthorizedReturns(errUnauthorized)

	cfg := &config.ServerConfig{
		GRPCAddress: ":50053",
	}
	security := &config.SecurityConfig{}

	server := NewGRPCServer(cfg, backend, log, authorization, security)

	req := &persistpb.QueryLogsRequest{
		JobId: "test-job",
	}

	mockStream := &mockQueryLogsServer{
		ctx: context.Background(),
	}

	err := server.QueryLogs(req, mockStream)
	if err == nil {
		t.Error("Expected unauthorized error, got nil")
	}

	// Verify authorization was checked
	if authorization.AuthorizedCallCount() != 1 {
		t.Error("Expected Authorized to be called once")
	}
}

func TestQueryLogsSuccess(t *testing.T) {
	backend := &storagefakes.FakeBackend{}
	log := logger.New()
	authorization := &authfakes.FakeGRPCAuthorization{}

	// Configure authorization to allow
	authorization.AuthorizedReturns(nil)

	// Configure backend to return test logs
	reader := &storage.LogReader{
		Channel: make(chan *ipcpb.LogLine, 10),
		Error:   make(chan error, 1),
		Done:    make(chan struct{}),
	}

	backend.ReadLogsReturns(reader, nil)

	// Send test log and close channel
	go func() {
		reader.Channel <- &ipcpb.LogLine{
			JobId:     "test-job",
			Stream:    ipcpb.StreamType_STREAM_TYPE_STDOUT,
			Timestamp: time.Now().UnixNano(),
			Sequence:  1,
			Content:   []byte("test log"),
		}
		close(reader.Channel)
	}()

	cfg := &config.ServerConfig{
		GRPCAddress: ":50053",
	}
	security := &config.SecurityConfig{}

	server := NewGRPCServer(cfg, backend, log, authorization, security)

	req := &persistpb.QueryLogsRequest{
		JobId:  "test-job",
		Stream: persistpb.StreamType_STREAM_TYPE_STDOUT,
		Limit:  100,
	}

	mockStream := &mockQueryLogsServer{
		ctx:      context.Background(),
		sentLogs: make([]*persistpb.LogLine, 0),
	}

	err := server.QueryLogs(req, mockStream)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Verify logs were sent
	if len(mockStream.sentLogs) != 1 {
		t.Errorf("Expected 1 log to be sent, got %d", len(mockStream.sentLogs))
	}

	if len(mockStream.sentLogs) > 0 {
		sentLog := mockStream.sentLogs[0]
		if sentLog.JobId != "test-job" {
			t.Errorf("Expected job ID 'test-job', got '%s'", sentLog.JobId)
		}
		if string(sentLog.Content) != "test log" {
			t.Errorf("Expected content 'test log', got '%s'", string(sentLog.Content))
		}
	}
}

type mockQueryMetricsServer struct {
	persistpb.PersistService_QueryMetricsServer
	ctx         context.Context
	sentMetrics []*persistpb.Metric
}

func (m *mockQueryMetricsServer) Context() context.Context {
	return m.ctx
}

func (m *mockQueryMetricsServer) Send(metric *persistpb.Metric) error {
	m.sentMetrics = append(m.sentMetrics, metric)
	return nil
}

func TestQueryMetricsSuccess(t *testing.T) {
	backend := &storagefakes.FakeBackend{}
	log := logger.New()
	authorization := &authfakes.FakeGRPCAuthorization{}

	authorization.AuthorizedReturns(nil)

	reader := &storage.MetricReader{
		Channel: make(chan *ipcpb.Metric, 10),
		Error:   make(chan error, 1),
		Done:    make(chan struct{}),
	}

	backend.ReadMetricsReturns(reader, nil)

	go func() {
		reader.Channel <- &ipcpb.Metric{
			JobId:     "test-job",
			Timestamp: time.Now().UnixNano(),
			Sequence:  1,
			Data: &ipcpb.MetricData{
				CpuUsage:    50.0,
				MemoryUsage: 1024,
			},
		}
		close(reader.Channel)
	}()

	cfg := &config.ServerConfig{
		GRPCAddress: ":50053",
	}
	security := &config.SecurityConfig{}

	server := NewGRPCServer(cfg, backend, log, authorization, security)

	req := &persistpb.QueryMetricsRequest{
		JobId: "test-job",
		Limit: 100,
	}

	mockStream := &mockQueryMetricsServer{
		ctx:         context.Background(),
		sentMetrics: make([]*persistpb.Metric, 0),
	}

	err := server.QueryMetrics(req, mockStream)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(mockStream.sentMetrics) != 1 {
		t.Errorf("Expected 1 metric to be sent, got %d", len(mockStream.sentMetrics))
	}
}

func TestDeleteJobSuccess(t *testing.T) {
	backend := &storagefakes.FakeBackend{}
	log := logger.New()
	authorization := &authfakes.FakeGRPCAuthorization{}

	authorization.AuthorizedReturns(nil)
	backend.DeleteJobReturns(nil)

	cfg := &config.ServerConfig{
		GRPCAddress: ":50053",
	}
	security := &config.SecurityConfig{}

	server := NewGRPCServer(cfg, backend, log, authorization, security)

	req := &persistpb.DeleteJobRequest{
		JobId: "test-job",
	}

	resp, err := server.DeleteJob(context.Background(), req)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if !resp.Success {
		t.Error("Expected success to be true")
	}

	if resp.Message != "Job deleted successfully" {
		t.Errorf("Expected success message, got '%s'", resp.Message)
	}

	// Verify backend was called
	if backend.DeleteJobCallCount() != 1 {
		t.Error("Expected DeleteJob to be called once")
	}

	if backend.DeleteJobCallCount() > 0 {
		jobID := backend.DeleteJobArgsForCall(0)
		if jobID != "test-job" {
			t.Errorf("Expected job ID 'test-job', got '%s'", jobID)
		}
	}
}

func TestDeleteJobEmptyID(t *testing.T) {
	backend := &storagefakes.FakeBackend{}
	log := logger.New()
	authorization := &authfakes.FakeGRPCAuthorization{}

	authorization.AuthorizedReturns(nil)

	cfg := &config.ServerConfig{
		GRPCAddress: ":50053",
	}
	security := &config.SecurityConfig{}

	server := NewGRPCServer(cfg, backend, log, authorization, security)

	req := &persistpb.DeleteJobRequest{
		JobId: "",
	}

	resp, err := server.DeleteJob(context.Background(), req)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if resp.Success {
		t.Error("Expected success to be false for empty job ID")
	}

	if resp.Message != "Job ID cannot be empty" {
		t.Errorf("Expected empty ID error message, got '%s'", resp.Message)
	}
}
