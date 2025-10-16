package storage

import (
	"testing"

	ipcpb "github.com/ehsaniara/joblet/internal/proto/gen/ipc"
	"github.com/ehsaniara/joblet/persist/internal/config"
	"github.com/ehsaniara/joblet/pkg/logger"
)

func TestNewBackend_Local(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.StorageConfig{
		Type: "local",
		Local: config.LocalConfig{
			Logs: config.LogStorageConfig{
				Directory: tmpDir + "/logs",
			},
			Metrics: config.MetricStorageConfig{
				Directory: tmpDir + "/metrics",
			},
		},
	}

	log := logger.New()

	backend, err := NewBackend(cfg, "test-node", log)
	if err != nil {
		t.Fatalf("Failed to create local backend: %v", err)
	}

	if backend == nil {
		t.Fatal("Expected backend to be created, got nil")
	}

	defer backend.Close()

	// Verify it's a LocalBackend
	if _, ok := backend.(*LocalBackend); !ok {
		t.Error("Expected LocalBackend type")
	}
}

func TestNewBackend_CloudWatch_RequiresConfig(t *testing.T) {
	// CloudWatch backend is now implemented but requires proper configuration
	// This test verifies it initializes (though it may fail without AWS credentials)
	cfg := &config.StorageConfig{
		Type: "cloudwatch",
		CloudWatch: config.CloudWatchConfig{
			Region: "us-east-1", // Use specific region to avoid auto-detection
		},
	}

	log := logger.New()

	// Backend may fail due to missing AWS credentials, but should not return "not implemented"
	_, err := NewBackend(cfg, "test-node", log)
	if err != nil && err.Error() == "CloudWatch backend not implemented yet (v2.0)" {
		t.Error("CloudWatch backend should be implemented")
	}
	// Note: We don't fail on other errors as they may be due to missing AWS credentials in test environment
}

func TestNewBackend_S3_NotImplemented(t *testing.T) {
	cfg := &config.StorageConfig{
		Type: "s3",
	}

	log := logger.New()

	_, err := NewBackend(cfg, "test-node", log)
	if err == nil {
		t.Error("Expected error for S3 backend (not implemented)")
	}

	if err.Error() != "S3 backend not implemented yet (v2.0)" {
		t.Errorf("Expected S3 not implemented error, got: %v", err)
	}
}

func TestNewBackend_Unknown(t *testing.T) {
	cfg := &config.StorageConfig{
		Type: "unknown-backend",
	}

	log := logger.New()

	_, err := NewBackend(cfg, "test-node", log)
	if err == nil {
		t.Error("Expected error for unknown backend type")
	}

	expectedError := "unknown storage backend type: unknown-backend"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got: %v", expectedError, err)
	}
}

func TestLogQuery_Fields(t *testing.T) {
	startTime := int64(1000)
	endTime := int64(2000)

	query := &LogQuery{
		JobID:     "test-job",
		StartTime: &startTime,
		EndTime:   &endTime,
		Limit:     100,
		Offset:    10,
		Filter:    "error",
	}

	if query.JobID != "test-job" {
		t.Errorf("Expected JobID 'test-job', got '%s'", query.JobID)
	}

	if *query.StartTime != 1000 {
		t.Errorf("Expected StartTime 1000, got %d", *query.StartTime)
	}

	if *query.EndTime != 2000 {
		t.Errorf("Expected EndTime 2000, got %d", *query.EndTime)
	}

	if query.Limit != 100 {
		t.Errorf("Expected Limit 100, got %d", query.Limit)
	}

	if query.Offset != 10 {
		t.Errorf("Expected Offset 10, got %d", query.Offset)
	}

	if query.Filter != "error" {
		t.Errorf("Expected Filter 'error', got '%s'", query.Filter)
	}
}

func TestMetricQuery_Fields(t *testing.T) {
	startTime := int64(1000)
	endTime := int64(2000)

	query := &MetricQuery{
		JobID:       "test-job",
		StartTime:   &startTime,
		EndTime:     &endTime,
		Aggregation: "avg",
		Limit:       50,
		Offset:      5,
	}

	if query.JobID != "test-job" {
		t.Errorf("Expected JobID 'test-job', got '%s'", query.JobID)
	}

	if *query.StartTime != 1000 {
		t.Errorf("Expected StartTime 1000, got %d", *query.StartTime)
	}

	if *query.EndTime != 2000 {
		t.Errorf("Expected EndTime 2000, got %d", *query.EndTime)
	}

	if query.Aggregation != "avg" {
		t.Errorf("Expected Aggregation 'avg', got '%s'", query.Aggregation)
	}

	if query.Limit != 50 {
		t.Errorf("Expected Limit 50, got %d", query.Limit)
	}

	if query.Offset != 5 {
		t.Errorf("Expected Offset 5, got %d", query.Offset)
	}
}

func TestLogReader_Channels(t *testing.T) {
	reader := &LogReader{
		Channel: make(chan *ipcpb.LogLine, 10),
		Error:   make(chan error, 1),
		Done:    make(chan struct{}),
	}

	if reader.Channel == nil {
		t.Error("Expected Channel to be initialized")
	}

	if reader.Error == nil {
		t.Error("Expected Error channel to be initialized")
	}

	if reader.Done == nil {
		t.Error("Expected Done channel to be initialized")
	}
}

func TestMetricReader_Channels(t *testing.T) {
	reader := &MetricReader{
		Channel: make(chan *ipcpb.Metric, 10),
		Error:   make(chan error, 1),
		Done:    make(chan struct{}),
	}

	if reader.Channel == nil {
		t.Error("Expected Channel to be initialized")
	}

	if reader.Error == nil {
		t.Error("Expected Error channel to be initialized")
	}

	if reader.Done == nil {
		t.Error("Expected Done channel to be initialized")
	}
}
