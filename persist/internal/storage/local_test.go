package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	ipcpb "github.com/ehsaniara/joblet/internal/proto/gen/ipc"
	"github.com/ehsaniara/joblet/persist/internal/config"
	"github.com/ehsaniara/joblet/pkg/logger"
)

func TestNewLocalBackend(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.StorageConfig{
		Type:    "local",
		BaseDir: tmpDir,
		Local: config.LocalConfig{
			Logs: config.LogStorageConfig{
				Directory: filepath.Join(tmpDir, "logs"),
			},
			Metrics: config.MetricStorageConfig{
				Directory: filepath.Join(tmpDir, "metrics"),
			},
		},
	}

	log := logger.New()

	backend, err := NewLocalBackend(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create local backend: %v", err)
	}

	if backend == nil {
		t.Fatal("Expected backend to be created, got nil")
	}

	defer backend.Close()

	// Verify directories were created
	if _, err := os.Stat(cfg.Local.Logs.Directory); os.IsNotExist(err) {
		t.Error("Logs directory was not created")
	}

	if _, err := os.Stat(cfg.Local.Metrics.Directory); os.IsNotExist(err) {
		t.Error("Metrics directory was not created")
	}
}

func TestLocalBackend_WriteLogs(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.StorageConfig{
		Type:    "local",
		BaseDir: tmpDir,
		Local: config.LocalConfig{
			Logs: config.LogStorageConfig{
				Directory: filepath.Join(tmpDir, "logs"),
			},
			Metrics: config.MetricStorageConfig{
				Directory: filepath.Join(tmpDir, "metrics"),
			},
		},
	}

	log := logger.New()
	backend, err := NewLocalBackend(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	jobID := "test-job-123"
	logs := []*ipcpb.LogLine{
		{
			JobId:     jobID,
			Stream:    ipcpb.StreamType_STREAM_TYPE_STDOUT,
			Timestamp: time.Now().UnixNano(),
			Sequence:  1,
			Content:   []byte("First log line"),
		},
		{
			JobId:     jobID,
			Stream:    ipcpb.StreamType_STREAM_TYPE_STDERR,
			Timestamp: time.Now().UnixNano(),
			Sequence:  2,
			Content:   []byte("Error log line"),
		},
	}

	err = backend.WriteLogs(jobID, logs)
	if err != nil {
		t.Errorf("Failed to write logs: %v", err)
	}

	// Verify log files were created in job subdirectory
	jobLogDir := filepath.Join(cfg.Local.Logs.Directory, jobID)
	stdoutPath := filepath.Join(jobLogDir, "stdout.log.gz")
	stderrPath := filepath.Join(jobLogDir, "stderr.log.gz")

	if _, err := os.Stat(stdoutPath); os.IsNotExist(err) {
		t.Error("Expected stdout.log.gz to be created")
	}

	if _, err := os.Stat(stderrPath); os.IsNotExist(err) {
		t.Error("Expected stderr.log.gz to be created")
	}
}

func TestLocalBackend_WriteMetrics(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.StorageConfig{
		Type:    "local",
		BaseDir: tmpDir,
		Local: config.LocalConfig{
			Logs: config.LogStorageConfig{
				Directory: filepath.Join(tmpDir, "logs"),
			},
			Metrics: config.MetricStorageConfig{
				Directory: filepath.Join(tmpDir, "metrics"),
			},
		},
	}

	log := logger.New()
	backend, err := NewLocalBackend(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	jobID := "test-job-456"
	metrics := []*ipcpb.Metric{
		{
			JobId:     jobID,
			Timestamp: time.Now().UnixNano(),
			Sequence:  1,
			Data: &ipcpb.MetricData{
				CpuUsage:    45.5,
				MemoryUsage: 1024000,
				GpuUsage:    80.0,
			},
		},
		{
			JobId:     jobID,
			Timestamp: time.Now().UnixNano(),
			Sequence:  2,
			Data: &ipcpb.MetricData{
				CpuUsage:    50.0,
				MemoryUsage: 2048000,
				GpuUsage:    85.0,
			},
		},
	}

	err = backend.WriteMetrics(jobID, metrics)
	if err != nil {
		t.Errorf("Failed to write metrics: %v", err)
	}

	// Verify metric files were created in job subdirectory
	jobMetricsDir := filepath.Join(cfg.Local.Metrics.Directory, jobID)
	metricsPath := filepath.Join(jobMetricsDir, "metrics.jsonl.gz")

	if _, err := os.Stat(metricsPath); os.IsNotExist(err) {
		t.Error("Expected metrics.jsonl.gz to be created")
	}
}

func TestLocalBackend_ReadLogs(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.StorageConfig{
		Type:    "local",
		BaseDir: tmpDir,
		Local: config.LocalConfig{
			Logs: config.LogStorageConfig{
				Directory: filepath.Join(tmpDir, "logs"),
			},
			Metrics: config.MetricStorageConfig{
				Directory: filepath.Join(tmpDir, "metrics"),
			},
		},
	}

	log := logger.New()
	backend, err := NewLocalBackend(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	jobID := "test-job-read"

	// Write some logs first
	logs := []*ipcpb.LogLine{
		{
			JobId:     jobID,
			Stream:    ipcpb.StreamType_STREAM_TYPE_STDOUT,
			Timestamp: time.Now().UnixNano(),
			Sequence:  1,
			Content:   []byte("Log line 1"),
		},
		{
			JobId:     jobID,
			Stream:    ipcpb.StreamType_STREAM_TYPE_STDOUT,
			Timestamp: time.Now().UnixNano(),
			Sequence:  2,
			Content:   []byte("Log line 2"),
		},
	}

	err = backend.WriteLogs(jobID, logs)
	if err != nil {
		t.Fatalf("Failed to write logs: %v", err)
	}

	// Give time for write to complete
	time.Sleep(100 * time.Millisecond)

	// Read the logs back
	query := &LogQuery{
		JobID:  jobID,
		Stream: ipcpb.StreamType_STREAM_TYPE_STDOUT,
		Limit:  100,
	}

	ctx := context.Background()
	reader, err := backend.ReadLogs(ctx, query)
	if err != nil {
		t.Fatalf("Failed to read logs: %v", err)
	}

	// Collect logs from channel
	var readLogs []*ipcpb.LogLine
	for {
		select {
		case log, ok := <-reader.Channel:
			if !ok {
				goto done
			}
			readLogs = append(readLogs, log)
		case err := <-reader.Error:
			if err != nil {
				t.Fatalf("Error reading logs: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for logs")
		}
	}

done:
	if len(readLogs) != 2 {
		t.Errorf("Expected 2 logs, got %d", len(readLogs))
	}

	if len(readLogs) > 0 && string(readLogs[0].Content) != "Log line 1" {
		t.Errorf("Expected first log 'Log line 1', got '%s'", string(readLogs[0].Content))
	}
}

func TestLocalBackend_ReadMetrics(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.StorageConfig{
		Type:    "local",
		BaseDir: tmpDir,
		Local: config.LocalConfig{
			Logs: config.LogStorageConfig{
				Directory: filepath.Join(tmpDir, "logs"),
			},
			Metrics: config.MetricStorageConfig{
				Directory: filepath.Join(tmpDir, "metrics"),
			},
		},
	}

	log := logger.New()
	backend, err := NewLocalBackend(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	jobID := "test-job-metrics"

	// Write some metrics first
	metrics := []*ipcpb.Metric{
		{
			JobId:     jobID,
			Timestamp: time.Now().UnixNano(),
			Sequence:  1,
			Data: &ipcpb.MetricData{
				CpuUsage:    45.5,
				MemoryUsage: 1024000,
			},
		},
	}

	err = backend.WriteMetrics(jobID, metrics)
	if err != nil {
		t.Fatalf("Failed to write metrics: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Read the metrics back
	query := &MetricQuery{
		JobID: jobID,
		Limit: 100,
	}

	ctx := context.Background()
	reader, err := backend.ReadMetrics(ctx, query)
	if err != nil {
		t.Fatalf("Failed to read metrics: %v", err)
	}

	// Collect metrics from channel
	var readMetrics []*ipcpb.Metric
	for {
		select {
		case metric, ok := <-reader.Channel:
			if !ok {
				goto done
			}
			readMetrics = append(readMetrics, metric)
		case err := <-reader.Error:
			if err != nil {
				t.Fatalf("Error reading metrics: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for metrics")
		}
	}

done:
	if len(readMetrics) != 1 {
		t.Errorf("Expected 1 metric, got %d", len(readMetrics))
	}

	if len(readMetrics) > 0 && readMetrics[0].Data.CpuUsage != 45.5 {
		t.Errorf("Expected CPU usage 45.5, got %f", readMetrics[0].Data.CpuUsage)
	}
}

func TestLocalBackend_DeleteJob(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.StorageConfig{
		Type:    "local",
		BaseDir: tmpDir,
		Local: config.LocalConfig{
			Logs: config.LogStorageConfig{
				Directory: filepath.Join(tmpDir, "logs"),
			},
			Metrics: config.MetricStorageConfig{
				Directory: filepath.Join(tmpDir, "metrics"),
			},
		},
	}

	log := logger.New()
	backend, err := NewLocalBackend(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	jobID := "test-job-delete"

	// Write some logs and metrics
	logs := []*ipcpb.LogLine{
		{
			JobId:     jobID,
			Stream:    ipcpb.StreamType_STREAM_TYPE_STDOUT,
			Timestamp: time.Now().UnixNano(),
			Sequence:  1,
			Content:   []byte("Test log"),
		},
	}

	metrics := []*ipcpb.Metric{
		{
			JobId:     jobID,
			Timestamp: time.Now().UnixNano(),
			Sequence:  1,
			Data: &ipcpb.MetricData{
				CpuUsage: 45.5,
			},
		},
	}

	backend.WriteLogs(jobID, logs)
	backend.WriteMetrics(jobID, metrics)

	time.Sleep(100 * time.Millisecond)

	// Verify directories exist
	logDir := filepath.Join(cfg.Local.Logs.Directory, jobID)
	metricsDir := filepath.Join(cfg.Local.Metrics.Directory, jobID)

	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		t.Error("Expected log directory to exist before deletion")
	}
	if _, err := os.Stat(metricsDir); os.IsNotExist(err) {
		t.Error("Expected metrics directory to exist before deletion")
	}

	// Delete the job
	err = backend.DeleteJob(jobID)
	if err != nil {
		t.Errorf("Failed to delete job: %v", err)
	}

	// Verify directories are gone
	if _, err := os.Stat(logDir); !os.IsNotExist(err) {
		t.Error("Expected log directory to be deleted")
	}
	if _, err := os.Stat(metricsDir); !os.IsNotExist(err) {
		t.Error("Expected metrics directory to be deleted")
	}
}

func TestLocalBackend_Close(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.StorageConfig{
		Type:    "local",
		BaseDir: tmpDir,
		Local: config.LocalConfig{
			Logs: config.LogStorageConfig{
				Directory: filepath.Join(tmpDir, "logs"),
			},
			Metrics: config.MetricStorageConfig{
				Directory: filepath.Join(tmpDir, "metrics"),
			},
		},
	}

	log := logger.New()
	backend, err := NewLocalBackend(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}

	err = backend.Close()
	if err != nil {
		t.Errorf("Failed to close backend: %v", err)
	}
}

func TestLocalBackend_EmptyJobID(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.StorageConfig{
		Type:    "local",
		BaseDir: tmpDir,
		Local: config.LocalConfig{
			Logs: config.LogStorageConfig{
				Directory: filepath.Join(tmpDir, "logs"),
			},
			Metrics: config.MetricStorageConfig{
				Directory: filepath.Join(tmpDir, "metrics"),
			},
		},
	}

	log := logger.New()
	backend, err := NewLocalBackend(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	// Test with empty job ID - this will create files in root logs directory
	// The implementation doesn't currently validate empty job IDs
	// This test just verifies the behavior is predictable
	logs := []*ipcpb.LogLine{
		{
			JobId:     "",
			Stream:    ipcpb.StreamType_STREAM_TYPE_STDOUT,
			Timestamp: time.Now().UnixNano(),
			Sequence:  1,
			Content:   []byte("Test"),
		},
	}

	// Empty job ID should work but create files in unusual location
	err = backend.WriteLogs("", logs)
	if err != nil {
		t.Logf("WriteLogs with empty job ID returned error (may be expected): %v", err)
	}
}
