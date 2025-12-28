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
		Type: "local",
		Local: config.LocalConfig{
			Logs: config.LogStorageConfig{
				Directory: filepath.Join(tmpDir, "logs"),
			},
			Metrics: config.MetricStorageConfig{
				Directory: filepath.Join(tmpDir, "metrics"),
			},
			Events: config.EventStorageConfig{
				Directory: filepath.Join(tmpDir, "events"),
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
		Type: "local",
		Local: config.LocalConfig{
			Logs: config.LogStorageConfig{
				Directory: filepath.Join(tmpDir, "logs"),
			},
			Metrics: config.MetricStorageConfig{
				Directory: filepath.Join(tmpDir, "metrics"),
			},
			Events: config.EventStorageConfig{
				Directory: filepath.Join(tmpDir, "events"),
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
			JobUuid:   jobID,
			Stream:    ipcpb.StreamType_STREAM_TYPE_STDOUT,
			Timestamp: time.Now().UnixNano(),
			Sequence:  1,
			Content:   []byte("First log line"),
		},
		{
			JobUuid:   jobID,
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
		Type: "local",
		Local: config.LocalConfig{
			Logs: config.LogStorageConfig{
				Directory: filepath.Join(tmpDir, "logs"),
			},
			Metrics: config.MetricStorageConfig{
				Directory: filepath.Join(tmpDir, "metrics"),
			},
			Events: config.EventStorageConfig{
				Directory: filepath.Join(tmpDir, "events"),
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
			JobUuid:   jobID,
			Timestamp: time.Now().UnixNano(),
			Sequence:  1,
			Data: &ipcpb.MetricData{
				CpuUsage:    45.5,
				MemoryUsage: 1024000,
				GpuUsage:    80.0,
			},
		},
		{
			JobUuid:   jobID,
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
		Type: "local",
		Local: config.LocalConfig{
			Logs: config.LogStorageConfig{
				Directory: filepath.Join(tmpDir, "logs"),
			},
			Metrics: config.MetricStorageConfig{
				Directory: filepath.Join(tmpDir, "metrics"),
			},
			Events: config.EventStorageConfig{
				Directory: filepath.Join(tmpDir, "events"),
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
			JobUuid:   jobID,
			Stream:    ipcpb.StreamType_STREAM_TYPE_STDOUT,
			Timestamp: time.Now().UnixNano(),
			Sequence:  1,
			Content:   []byte("Log line 1"),
		},
		{
			JobUuid:   jobID,
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
		JobUUID: jobID,
		Stream:  ipcpb.StreamType_STREAM_TYPE_STDOUT,
		Limit:   100,
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
		Type: "local",
		Local: config.LocalConfig{
			Logs: config.LogStorageConfig{
				Directory: filepath.Join(tmpDir, "logs"),
			},
			Metrics: config.MetricStorageConfig{
				Directory: filepath.Join(tmpDir, "metrics"),
			},
			Events: config.EventStorageConfig{
				Directory: filepath.Join(tmpDir, "events"),
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
			JobUuid:   jobID,
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
		JobUUID: jobID,
		Limit:   100,
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
		Type: "local",
		Local: config.LocalConfig{
			Logs: config.LogStorageConfig{
				Directory: filepath.Join(tmpDir, "logs"),
			},
			Metrics: config.MetricStorageConfig{
				Directory: filepath.Join(tmpDir, "metrics"),
			},
			Events: config.EventStorageConfig{
				Directory: filepath.Join(tmpDir, "events"),
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
			JobUuid:   jobID,
			Stream:    ipcpb.StreamType_STREAM_TYPE_STDOUT,
			Timestamp: time.Now().UnixNano(),
			Sequence:  1,
			Content:   []byte("Test log"),
		},
	}

	metrics := []*ipcpb.Metric{
		{
			JobUuid:   jobID,
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
		Type: "local",
		Local: config.LocalConfig{
			Logs: config.LogStorageConfig{
				Directory: filepath.Join(tmpDir, "logs"),
			},
			Metrics: config.MetricStorageConfig{
				Directory: filepath.Join(tmpDir, "metrics"),
			},
			Events: config.EventStorageConfig{
				Directory: filepath.Join(tmpDir, "events"),
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
		Type: "local",
		Local: config.LocalConfig{
			Logs: config.LogStorageConfig{
				Directory: filepath.Join(tmpDir, "logs"),
			},
			Metrics: config.MetricStorageConfig{
				Directory: filepath.Join(tmpDir, "metrics"),
			},
			Events: config.EventStorageConfig{
				Directory: filepath.Join(tmpDir, "events"),
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
			JobUuid:   "",
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

func TestLocalBackend_WriteExecEvents(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.StorageConfig{
		Type: "local",
		Local: config.LocalConfig{
			Logs: config.LogStorageConfig{
				Directory: filepath.Join(tmpDir, "logs"),
			},
			Metrics: config.MetricStorageConfig{
				Directory: filepath.Join(tmpDir, "metrics"),
			},
			Events: config.EventStorageConfig{
				Directory: filepath.Join(tmpDir, "events"),
			},
		},
	}

	log := logger.New()
	backend, err := NewLocalBackend(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	jobID := "test-job-exec-events"
	events := []*ipcpb.ExecEvent{
		{
			JobUuid:   jobID,
			Timestamp: time.Now().UnixNano(),
			Sequence:  1,
			Pid:       1234,
			Ppid:      1,
			Filename:  "/bin/bash",
			Args:      []string{"-c", "echo hello"},
		},
		{
			JobUuid:   jobID,
			Timestamp: time.Now().UnixNano(),
			Sequence:  2,
			Pid:       1235,
			Ppid:      1234,
			Filename:  "/usr/bin/echo",
			Args:      []string{"hello"},
		},
	}

	err = backend.WriteExecEvents(jobID, events)
	if err != nil {
		t.Errorf("Failed to write exec events: %v", err)
	}

	// Verify exec events file was created
	jobEventsDir := filepath.Join(cfg.Local.Events.Directory, jobID)
	execEventsPath := filepath.Join(jobEventsDir, "exec_events.jsonl.gz")

	if _, err := os.Stat(execEventsPath); os.IsNotExist(err) {
		t.Error("Expected exec_events.jsonl.gz to be created")
	}
}

func TestLocalBackend_WriteExecEvents_Empty(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.StorageConfig{
		Type: "local",
		Local: config.LocalConfig{
			Logs: config.LogStorageConfig{
				Directory: filepath.Join(tmpDir, "logs"),
			},
			Metrics: config.MetricStorageConfig{
				Directory: filepath.Join(tmpDir, "metrics"),
			},
			Events: config.EventStorageConfig{
				Directory: filepath.Join(tmpDir, "events"),
			},
		},
	}

	log := logger.New()
	backend, err := NewLocalBackend(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	// Writing empty events should not error
	err = backend.WriteExecEvents("test-job", []*ipcpb.ExecEvent{})
	if err != nil {
		t.Errorf("WriteExecEvents with empty slice should not error: %v", err)
	}
}

func TestLocalBackend_WriteConnectEvents(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.StorageConfig{
		Type: "local",
		Local: config.LocalConfig{
			Logs: config.LogStorageConfig{
				Directory: filepath.Join(tmpDir, "logs"),
			},
			Metrics: config.MetricStorageConfig{
				Directory: filepath.Join(tmpDir, "metrics"),
			},
			Events: config.EventStorageConfig{
				Directory: filepath.Join(tmpDir, "events"),
			},
		},
	}

	log := logger.New()
	backend, err := NewLocalBackend(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	jobID := "test-job-connect-events"
	events := []*ipcpb.ConnectEvent{
		{
			JobUuid:   jobID,
			Timestamp: time.Now().UnixNano(),
			Sequence:  1,
			Pid:       5678,
			DstAddr:   "8.8.8.8",
			DstPort:   443,
			Protocol:  "tcp",
			SrcAddr:   "10.0.0.1",
			SrcPort:   54321,
		},
		{
			JobUuid:   jobID,
			Timestamp: time.Now().UnixNano(),
			Sequence:  2,
			Pid:       5678,
			DstAddr:   "1.1.1.1",
			DstPort:   80,
			Protocol:  "tcp",
		},
	}

	err = backend.WriteConnectEvents(jobID, events)
	if err != nil {
		t.Errorf("Failed to write connect events: %v", err)
	}

	// Verify connect events file was created
	jobEventsDir := filepath.Join(cfg.Local.Events.Directory, jobID)
	connectEventsPath := filepath.Join(jobEventsDir, "connect_events.jsonl.gz")

	if _, err := os.Stat(connectEventsPath); os.IsNotExist(err) {
		t.Error("Expected connect_events.jsonl.gz to be created")
	}
}

func TestLocalBackend_WriteConnectEvents_Empty(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.StorageConfig{
		Type: "local",
		Local: config.LocalConfig{
			Logs: config.LogStorageConfig{
				Directory: filepath.Join(tmpDir, "logs"),
			},
			Metrics: config.MetricStorageConfig{
				Directory: filepath.Join(tmpDir, "metrics"),
			},
			Events: config.EventStorageConfig{
				Directory: filepath.Join(tmpDir, "events"),
			},
		},
	}

	log := logger.New()
	backend, err := NewLocalBackend(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	// Writing empty events should not error
	err = backend.WriteConnectEvents("test-job", []*ipcpb.ConnectEvent{})
	if err != nil {
		t.Errorf("WriteConnectEvents with empty slice should not error: %v", err)
	}
}

func TestLocalBackend_WriteExecEvents_AppendMode(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.StorageConfig{
		Type: "local",
		Local: config.LocalConfig{
			Logs: config.LogStorageConfig{
				Directory: filepath.Join(tmpDir, "logs"),
			},
			Metrics: config.MetricStorageConfig{
				Directory: filepath.Join(tmpDir, "metrics"),
			},
			Events: config.EventStorageConfig{
				Directory: filepath.Join(tmpDir, "events"),
			},
		},
	}

	log := logger.New()
	backend, err := NewLocalBackend(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	jobID := "test-job-append"

	// Write first batch
	events1 := []*ipcpb.ExecEvent{
		{
			JobUuid:  jobID,
			Sequence: 1,
			Pid:      100,
			Filename: "/bin/first",
		},
	}
	err = backend.WriteExecEvents(jobID, events1)
	if err != nil {
		t.Fatalf("Failed to write first batch: %v", err)
	}

	// Write second batch
	events2 := []*ipcpb.ExecEvent{
		{
			JobUuid:  jobID,
			Sequence: 2,
			Pid:      200,
			Filename: "/bin/second",
		},
	}
	err = backend.WriteExecEvents(jobID, events2)
	if err != nil {
		t.Fatalf("Failed to write second batch: %v", err)
	}

	// Verify file was created and has content from both writes
	jobEventsDir := filepath.Join(cfg.Local.Events.Directory, jobID)
	execEventsPath := filepath.Join(jobEventsDir, "exec_events.jsonl.gz")

	info, err := os.Stat(execEventsPath)
	if err != nil {
		t.Fatalf("Failed to stat exec events file: %v", err)
	}

	// File should have content from both writes
	if info.Size() == 0 {
		t.Error("Expected exec events file to have content")
	}
}

// Tests for new eBPF event types

func TestLocalBackend_WriteFileEvents(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.StorageConfig{
		Type: "local",
		Local: config.LocalConfig{
			Logs: config.LogStorageConfig{
				Directory: filepath.Join(tmpDir, "logs"),
			},
			Metrics: config.MetricStorageConfig{
				Directory: filepath.Join(tmpDir, "metrics"),
			},
			Events: config.EventStorageConfig{
				Directory: filepath.Join(tmpDir, "events"),
			},
		},
	}

	log := logger.New()
	backend, err := NewLocalBackend(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	jobID := "test-job-file-events"
	events := []*ipcpb.FileEvent{
		{
			JobUuid:   jobID,
			Timestamp: time.Now().UnixNano(),
			Sequence:  1,
			Pid:       1234,
			Comm:      "cat",
			Path:      "/etc/passwd",
			Operation: "read",
			Bytes:     0,
		},
		{
			JobUuid:   jobID,
			Timestamp: time.Now().UnixNano(),
			Sequence:  2,
			Pid:       1234,
			Comm:      "bash",
			Path:      "/tmp/test.txt",
			Operation: "write",
			Bytes:     1024,
		},
	}

	err = backend.WriteFileEvents(jobID, events)
	if err != nil {
		t.Errorf("Failed to write file events: %v", err)
	}

	// Verify file events file was created
	jobEventsDir := filepath.Join(cfg.Local.Events.Directory, jobID)
	fileEventsPath := filepath.Join(jobEventsDir, "file_events.jsonl.gz")

	if _, err := os.Stat(fileEventsPath); os.IsNotExist(err) {
		t.Error("Expected file_events.jsonl.gz to be created")
	}
}

func TestLocalBackend_WriteFileEvents_Empty(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.StorageConfig{
		Type: "local",
		Local: config.LocalConfig{
			Logs: config.LogStorageConfig{
				Directory: filepath.Join(tmpDir, "logs"),
			},
			Metrics: config.MetricStorageConfig{
				Directory: filepath.Join(tmpDir, "metrics"),
			},
			Events: config.EventStorageConfig{
				Directory: filepath.Join(tmpDir, "events"),
			},
		},
	}

	log := logger.New()
	backend, err := NewLocalBackend(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	err = backend.WriteFileEvents("test-job", []*ipcpb.FileEvent{})
	if err != nil {
		t.Errorf("WriteFileEvents with empty slice should not error: %v", err)
	}
}

func TestLocalBackend_WriteAcceptEvents(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.StorageConfig{
		Type: "local",
		Local: config.LocalConfig{
			Logs: config.LogStorageConfig{
				Directory: filepath.Join(tmpDir, "logs"),
			},
			Metrics: config.MetricStorageConfig{
				Directory: filepath.Join(tmpDir, "metrics"),
			},
			Events: config.EventStorageConfig{
				Directory: filepath.Join(tmpDir, "events"),
			},
		},
	}

	log := logger.New()
	backend, err := NewLocalBackend(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	jobID := "test-job-accept-events"
	events := []*ipcpb.AcceptEvent{
		{
			JobUuid:   jobID,
			Timestamp: time.Now().UnixNano(),
			Sequence:  1,
			Pid:       5678,
			SrcAddr:   "192.168.1.100",
			SrcPort:   54321,
			DstAddr:   "0.0.0.0",
			DstPort:   8080,
			Protocol:  "tcp",
		},
		{
			JobUuid:   jobID,
			Timestamp: time.Now().UnixNano(),
			Sequence:  2,
			Pid:       5678,
			SrcAddr:   "10.0.0.5",
			SrcPort:   12345,
			DstAddr:   "0.0.0.0",
			DstPort:   8080,
			Protocol:  "tcp",
		},
	}

	err = backend.WriteAcceptEvents(jobID, events)
	if err != nil {
		t.Errorf("Failed to write accept events: %v", err)
	}

	// Verify accept events file was created
	jobEventsDir := filepath.Join(cfg.Local.Events.Directory, jobID)
	acceptEventsPath := filepath.Join(jobEventsDir, "accept_events.jsonl.gz")

	if _, err := os.Stat(acceptEventsPath); os.IsNotExist(err) {
		t.Error("Expected accept_events.jsonl.gz to be created")
	}
}

func TestLocalBackend_WriteAcceptEvents_Empty(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.StorageConfig{
		Type: "local",
		Local: config.LocalConfig{
			Logs: config.LogStorageConfig{
				Directory: filepath.Join(tmpDir, "logs"),
			},
			Metrics: config.MetricStorageConfig{
				Directory: filepath.Join(tmpDir, "metrics"),
			},
			Events: config.EventStorageConfig{
				Directory: filepath.Join(tmpDir, "events"),
			},
		},
	}

	log := logger.New()
	backend, err := NewLocalBackend(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	err = backend.WriteAcceptEvents("test-job", []*ipcpb.AcceptEvent{})
	if err != nil {
		t.Errorf("WriteAcceptEvents with empty slice should not error: %v", err)
	}
}

func TestLocalBackend_WriteSocketDataEvents(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.StorageConfig{
		Type: "local",
		Local: config.LocalConfig{
			Logs: config.LogStorageConfig{
				Directory: filepath.Join(tmpDir, "logs"),
			},
			Metrics: config.MetricStorageConfig{
				Directory: filepath.Join(tmpDir, "metrics"),
			},
			Events: config.EventStorageConfig{
				Directory: filepath.Join(tmpDir, "events"),
			},
		},
	}

	log := logger.New()
	backend, err := NewLocalBackend(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	jobID := "test-job-socket-data-events"
	events := []*ipcpb.SocketDataEvent{
		{
			JobUuid:   jobID,
			Timestamp: time.Now().UnixNano(),
			Sequence:  1,
			Pid:       9012,
			Comm:      "curl",
			Direction: "send",
			Addr:      "8.8.8.8",
			Port:      443,
			Protocol:  "TCP",
			Bytes:     1024,
		},
		{
			JobUuid:   jobID,
			Timestamp: time.Now().UnixNano(),
			Sequence:  2,
			Pid:       9012,
			Comm:      "curl",
			Direction: "recv",
			Addr:      "8.8.8.8",
			Port:      443,
			Protocol:  "TCP",
			Bytes:     2048,
		},
	}

	err = backend.WriteSocketDataEvents(jobID, events)
	if err != nil {
		t.Errorf("Failed to write socket data events: %v", err)
	}

	// Verify socket data events file was created
	jobEventsDir := filepath.Join(cfg.Local.Events.Directory, jobID)
	socketDataEventsPath := filepath.Join(jobEventsDir, "socket_data_events.jsonl.gz")

	if _, err := os.Stat(socketDataEventsPath); os.IsNotExist(err) {
		t.Error("Expected socket_data_events.jsonl.gz to be created")
	}
}

func TestLocalBackend_WriteSocketDataEvents_Empty(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.StorageConfig{
		Type: "local",
		Local: config.LocalConfig{
			Logs: config.LogStorageConfig{
				Directory: filepath.Join(tmpDir, "logs"),
			},
			Metrics: config.MetricStorageConfig{
				Directory: filepath.Join(tmpDir, "metrics"),
			},
			Events: config.EventStorageConfig{
				Directory: filepath.Join(tmpDir, "events"),
			},
		},
	}

	log := logger.New()
	backend, err := NewLocalBackend(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	err = backend.WriteSocketDataEvents("test-job", []*ipcpb.SocketDataEvent{})
	if err != nil {
		t.Errorf("WriteSocketDataEvents with empty slice should not error: %v", err)
	}
}

func TestLocalBackend_WriteMmapEvents(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.StorageConfig{
		Type: "local",
		Local: config.LocalConfig{
			Logs: config.LogStorageConfig{
				Directory: filepath.Join(tmpDir, "logs"),
			},
			Metrics: config.MetricStorageConfig{
				Directory: filepath.Join(tmpDir, "metrics"),
			},
			Events: config.EventStorageConfig{
				Directory: filepath.Join(tmpDir, "events"),
			},
		},
	}

	log := logger.New()
	backend, err := NewLocalBackend(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	jobID := "test-job-mmap-events"
	events := []*ipcpb.MmapEvent{
		{
			JobUuid:   jobID,
			Timestamp: time.Now().UnixNano(),
			Sequence:  1,
			Pid:       3456,
			Comm:      "ld-linux",
			Addr:      0x7f0000000000,
			Length:    4096,
			Prot:      0x3, // PROT_READ | PROT_WRITE
			Flags:     0x2, // MAP_PRIVATE
			Filename:  "/lib/libc.so.6",
		},
		{
			JobUuid:   jobID,
			Timestamp: time.Now().UnixNano(),
			Sequence:  2,
			Pid:       3456,
			Comm:      "ld-linux",
			Addr:      0x7f0000001000,
			Length:    8192,
			Prot:      0x5, // PROT_READ | PROT_EXEC
			Flags:     0x2, // MAP_PRIVATE
			Filename:  "/lib/libpthread.so.0",
		},
	}

	err = backend.WriteMmapEvents(jobID, events)
	if err != nil {
		t.Errorf("Failed to write mmap events: %v", err)
	}

	// Verify mmap events file was created
	jobEventsDir := filepath.Join(cfg.Local.Events.Directory, jobID)
	mmapEventsPath := filepath.Join(jobEventsDir, "mmap_events.jsonl.gz")

	if _, err := os.Stat(mmapEventsPath); os.IsNotExist(err) {
		t.Error("Expected mmap_events.jsonl.gz to be created")
	}
}

func TestLocalBackend_WriteMmapEvents_Empty(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.StorageConfig{
		Type: "local",
		Local: config.LocalConfig{
			Logs: config.LogStorageConfig{
				Directory: filepath.Join(tmpDir, "logs"),
			},
			Metrics: config.MetricStorageConfig{
				Directory: filepath.Join(tmpDir, "metrics"),
			},
			Events: config.EventStorageConfig{
				Directory: filepath.Join(tmpDir, "events"),
			},
		},
	}

	log := logger.New()
	backend, err := NewLocalBackend(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	err = backend.WriteMmapEvents("test-job", []*ipcpb.MmapEvent{})
	if err != nil {
		t.Errorf("WriteMmapEvents with empty slice should not error: %v", err)
	}
}

func TestLocalBackend_WriteMprotectEvents(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.StorageConfig{
		Type: "local",
		Local: config.LocalConfig{
			Logs: config.LogStorageConfig{
				Directory: filepath.Join(tmpDir, "logs"),
			},
			Metrics: config.MetricStorageConfig{
				Directory: filepath.Join(tmpDir, "metrics"),
			},
			Events: config.EventStorageConfig{
				Directory: filepath.Join(tmpDir, "events"),
			},
		},
	}

	log := logger.New()
	backend, err := NewLocalBackend(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	jobID := "test-job-mprotect-events"
	events := []*ipcpb.MprotectEvent{
		{
			JobUuid:   jobID,
			Timestamp: time.Now().UnixNano(),
			Sequence:  1,
			Pid:       7890,
			Comm:      "jit-compiler",
			Addr:      0x7f0000000000,
			Length:    4096,
			Prot:      0x7, // PROT_READ | PROT_WRITE | PROT_EXEC
		},
		{
			JobUuid:   jobID,
			Timestamp: time.Now().UnixNano(),
			Sequence:  2,
			Pid:       7890,
			Comm:      "jit-compiler",
			Addr:      0x7f0000001000,
			Length:    8192,
			Prot:      0x1, // PROT_READ
		},
	}

	err = backend.WriteMprotectEvents(jobID, events)
	if err != nil {
		t.Errorf("Failed to write mprotect events: %v", err)
	}

	// Verify mprotect events file was created
	jobEventsDir := filepath.Join(cfg.Local.Events.Directory, jobID)
	mprotectEventsPath := filepath.Join(jobEventsDir, "mprotect_events.jsonl.gz")

	if _, err := os.Stat(mprotectEventsPath); os.IsNotExist(err) {
		t.Error("Expected mprotect_events.jsonl.gz to be created")
	}
}

func TestLocalBackend_WriteMprotectEvents_Empty(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.StorageConfig{
		Type: "local",
		Local: config.LocalConfig{
			Logs: config.LogStorageConfig{
				Directory: filepath.Join(tmpDir, "logs"),
			},
			Metrics: config.MetricStorageConfig{
				Directory: filepath.Join(tmpDir, "metrics"),
			},
			Events: config.EventStorageConfig{
				Directory: filepath.Join(tmpDir, "events"),
			},
		},
	}

	log := logger.New()
	backend, err := NewLocalBackend(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	err = backend.WriteMprotectEvents("test-job", []*ipcpb.MprotectEvent{})
	if err != nil {
		t.Errorf("WriteMprotectEvents with empty slice should not error: %v", err)
	}
}

// Read tests for new eBPF event types

func TestLocalBackend_ReadFileEvents(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.StorageConfig{
		Type: "local",
		Local: config.LocalConfig{
			Logs: config.LogStorageConfig{
				Directory: filepath.Join(tmpDir, "logs"),
			},
			Metrics: config.MetricStorageConfig{
				Directory: filepath.Join(tmpDir, "metrics"),
			},
			Events: config.EventStorageConfig{
				Directory: filepath.Join(tmpDir, "events"),
			},
		},
	}

	log := logger.New()
	backend, err := NewLocalBackend(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	jobID := "test-job-read-file-events"

	// Write some events first
	events := []*ipcpb.FileEvent{
		{
			JobUuid:   jobID,
			Timestamp: time.Now().UnixNano(),
			Sequence:  1,
			Pid:       1111,
			Comm:      "cat",
			Path:      "/etc/hosts",
			Operation: "read",
		},
		{
			JobUuid:   jobID,
			Timestamp: time.Now().UnixNano(),
			Sequence:  2,
			Pid:       1111,
			Comm:      "bash",
			Path:      "/tmp/output.txt",
			Operation: "write",
		},
	}

	err = backend.WriteFileEvents(jobID, events)
	if err != nil {
		t.Fatalf("Failed to write file events: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Read the events back
	query := &TelemetryQuery{
		JobUUID: jobID,
		Limit:   100,
	}

	ctx := context.Background()
	reader, err := backend.ReadFileEvents(ctx, query)
	if err != nil {
		t.Fatalf("Failed to read file events: %v", err)
	}

	var readEvents []*ipcpb.FileEvent
	for {
		select {
		case event, ok := <-reader.Channel:
			if !ok {
				goto done
			}
			readEvents = append(readEvents, event)
		case err := <-reader.Error:
			if err != nil {
				t.Fatalf("Error reading file events: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for file events")
		}
	}

done:
	if len(readEvents) != 2 {
		t.Errorf("Expected 2 file events, got %d", len(readEvents))
	}

	if len(readEvents) > 0 && readEvents[0].Path != "/etc/hosts" {
		t.Errorf("Expected first path '/etc/hosts', got '%s'", readEvents[0].Path)
	}
}

func TestLocalBackend_ReadAcceptEvents(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.StorageConfig{
		Type: "local",
		Local: config.LocalConfig{
			Logs: config.LogStorageConfig{
				Directory: filepath.Join(tmpDir, "logs"),
			},
			Metrics: config.MetricStorageConfig{
				Directory: filepath.Join(tmpDir, "metrics"),
			},
			Events: config.EventStorageConfig{
				Directory: filepath.Join(tmpDir, "events"),
			},
		},
	}

	log := logger.New()
	backend, err := NewLocalBackend(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	jobID := "test-job-read-accept-events"

	events := []*ipcpb.AcceptEvent{
		{
			JobUuid:   jobID,
			Timestamp: time.Now().UnixNano(),
			Sequence:  1,
			Pid:       2222,
			SrcAddr:   "192.168.1.50",
			SrcPort:   12345,
			DstPort:   80,
		},
	}

	err = backend.WriteAcceptEvents(jobID, events)
	if err != nil {
		t.Fatalf("Failed to write accept events: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	query := &TelemetryQuery{
		JobUUID: jobID,
		Limit:   100,
	}

	ctx := context.Background()
	reader, err := backend.ReadAcceptEvents(ctx, query)
	if err != nil {
		t.Fatalf("Failed to read accept events: %v", err)
	}

	var readEvents []*ipcpb.AcceptEvent
	for {
		select {
		case event, ok := <-reader.Channel:
			if !ok {
				goto done
			}
			readEvents = append(readEvents, event)
		case err := <-reader.Error:
			if err != nil {
				t.Fatalf("Error reading accept events: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for accept events")
		}
	}

done:
	if len(readEvents) != 1 {
		t.Errorf("Expected 1 accept event, got %d", len(readEvents))
	}

	if len(readEvents) > 0 && readEvents[0].SrcAddr != "192.168.1.50" {
		t.Errorf("Expected SrcAddr '192.168.1.50', got '%s'", readEvents[0].SrcAddr)
	}
}

func TestLocalBackend_ReadSocketDataEvents(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.StorageConfig{
		Type: "local",
		Local: config.LocalConfig{
			Logs: config.LogStorageConfig{
				Directory: filepath.Join(tmpDir, "logs"),
			},
			Metrics: config.MetricStorageConfig{
				Directory: filepath.Join(tmpDir, "metrics"),
			},
			Events: config.EventStorageConfig{
				Directory: filepath.Join(tmpDir, "events"),
			},
		},
	}

	log := logger.New()
	backend, err := NewLocalBackend(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	jobID := "test-job-read-socket-data-events"

	events := []*ipcpb.SocketDataEvent{
		{
			JobUuid:   jobID,
			Timestamp: time.Now().UnixNano(),
			Sequence:  1,
			Pid:       3333,
			Comm:      "curl",
			Direction: "send",
			Addr:      "8.8.8.8",
			Port:      443,
			Protocol:  "TCP",
			Bytes:     512,
		},
	}

	err = backend.WriteSocketDataEvents(jobID, events)
	if err != nil {
		t.Fatalf("Failed to write socket data events: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	query := &TelemetryQuery{
		JobUUID: jobID,
		Limit:   100,
	}

	ctx := context.Background()
	reader, err := backend.ReadSocketDataEvents(ctx, query)
	if err != nil {
		t.Fatalf("Failed to read socket data events: %v", err)
	}

	var readEvents []*ipcpb.SocketDataEvent
	for {
		select {
		case event, ok := <-reader.Channel:
			if !ok {
				goto done
			}
			readEvents = append(readEvents, event)
		case err := <-reader.Error:
			if err != nil {
				t.Fatalf("Error reading socket data events: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for socket data events")
		}
	}

done:
	if len(readEvents) != 1 {
		t.Errorf("Expected 1 socket data event, got %d", len(readEvents))
	}

	if len(readEvents) > 0 && readEvents[0].Direction != "send" {
		t.Errorf("Expected direction 'send', got '%s'", readEvents[0].Direction)
	}
}

func TestLocalBackend_ReadMmapEvents(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.StorageConfig{
		Type: "local",
		Local: config.LocalConfig{
			Logs: config.LogStorageConfig{
				Directory: filepath.Join(tmpDir, "logs"),
			},
			Metrics: config.MetricStorageConfig{
				Directory: filepath.Join(tmpDir, "metrics"),
			},
			Events: config.EventStorageConfig{
				Directory: filepath.Join(tmpDir, "events"),
			},
		},
	}

	log := logger.New()
	backend, err := NewLocalBackend(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	jobID := "test-job-read-mmap-events"

	events := []*ipcpb.MmapEvent{
		{
			JobUuid:   jobID,
			Timestamp: time.Now().UnixNano(),
			Sequence:  1,
			Pid:       4444,
			Comm:      "ld-linux",
			Addr:      0x7f0000000000,
			Length:    4096,
			Filename:  "/lib/test.so",
		},
	}

	err = backend.WriteMmapEvents(jobID, events)
	if err != nil {
		t.Fatalf("Failed to write mmap events: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	query := &TelemetryQuery{
		JobUUID: jobID,
		Limit:   100,
	}

	ctx := context.Background()
	reader, err := backend.ReadMmapEvents(ctx, query)
	if err != nil {
		t.Fatalf("Failed to read mmap events: %v", err)
	}

	var readEvents []*ipcpb.MmapEvent
	for {
		select {
		case event, ok := <-reader.Channel:
			if !ok {
				goto done
			}
			readEvents = append(readEvents, event)
		case err := <-reader.Error:
			if err != nil {
				t.Fatalf("Error reading mmap events: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for mmap events")
		}
	}

done:
	if len(readEvents) != 1 {
		t.Errorf("Expected 1 mmap event, got %d", len(readEvents))
	}

	if len(readEvents) > 0 && readEvents[0].Filename != "/lib/test.so" {
		t.Errorf("Expected filename '/lib/test.so', got '%s'", readEvents[0].Filename)
	}
}

func TestLocalBackend_ReadMprotectEvents(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.StorageConfig{
		Type: "local",
		Local: config.LocalConfig{
			Logs: config.LogStorageConfig{
				Directory: filepath.Join(tmpDir, "logs"),
			},
			Metrics: config.MetricStorageConfig{
				Directory: filepath.Join(tmpDir, "metrics"),
			},
			Events: config.EventStorageConfig{
				Directory: filepath.Join(tmpDir, "events"),
			},
		},
	}

	log := logger.New()
	backend, err := NewLocalBackend(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	jobID := "test-job-read-mprotect-events"

	events := []*ipcpb.MprotectEvent{
		{
			JobUuid:   jobID,
			Timestamp: time.Now().UnixNano(),
			Sequence:  1,
			Pid:       5555,
			Comm:      "jit-compiler",
			Addr:      0x7f0000000000,
			Length:    8192,
			Prot:      0x5, // PROT_READ | PROT_EXEC
		},
	}

	err = backend.WriteMprotectEvents(jobID, events)
	if err != nil {
		t.Fatalf("Failed to write mprotect events: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	query := &TelemetryQuery{
		JobUUID: jobID,
		Limit:   100,
	}

	ctx := context.Background()
	reader, err := backend.ReadMprotectEvents(ctx, query)
	if err != nil {
		t.Fatalf("Failed to read mprotect events: %v", err)
	}

	var readEvents []*ipcpb.MprotectEvent
	for {
		select {
		case event, ok := <-reader.Channel:
			if !ok {
				goto done
			}
			readEvents = append(readEvents, event)
		case err := <-reader.Error:
			if err != nil {
				t.Fatalf("Error reading mprotect events: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for mprotect events")
		}
	}

done:
	if len(readEvents) != 1 {
		t.Errorf("Expected 1 mprotect event, got %d", len(readEvents))
	}

	if len(readEvents) > 0 && readEvents[0].Prot != 0x5 {
		t.Errorf("Expected prot 0x5, got 0x%x", readEvents[0].Prot)
	}
}

func TestLocalBackend_DeleteJob_IncludesNewEventTypes(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.StorageConfig{
		Type: "local",
		Local: config.LocalConfig{
			Logs: config.LogStorageConfig{
				Directory: filepath.Join(tmpDir, "logs"),
			},
			Metrics: config.MetricStorageConfig{
				Directory: filepath.Join(tmpDir, "metrics"),
			},
			Events: config.EventStorageConfig{
				Directory: filepath.Join(tmpDir, "events"),
			},
		},
	}

	log := logger.New()
	backend, err := NewLocalBackend(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	jobID := "test-job-delete-all-events"

	// Write all event types
	backend.WriteFileEvents(jobID, []*ipcpb.FileEvent{{JobUuid: jobID, Sequence: 1, Path: "/test"}})
	backend.WriteAcceptEvents(jobID, []*ipcpb.AcceptEvent{{JobUuid: jobID, Sequence: 1, SrcAddr: "1.2.3.4"}})
	backend.WriteSocketDataEvents(jobID, []*ipcpb.SocketDataEvent{{JobUuid: jobID, Sequence: 1, Bytes: 100}})
	backend.WriteMmapEvents(jobID, []*ipcpb.MmapEvent{{JobUuid: jobID, Sequence: 1, Addr: 0x1000}})
	backend.WriteMprotectEvents(jobID, []*ipcpb.MprotectEvent{{JobUuid: jobID, Sequence: 1, Addr: 0x2000}})

	time.Sleep(100 * time.Millisecond)

	// Verify files were created
	jobEventsDir := filepath.Join(cfg.Local.Events.Directory, jobID)
	files := []string{
		"file_events.jsonl.gz",
		"accept_events.jsonl.gz",
		"socket_data_events.jsonl.gz",
		"mmap_events.jsonl.gz",
		"mprotect_events.jsonl.gz",
	}

	for _, file := range files {
		path := filepath.Join(jobEventsDir, file)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Expected %s to exist before deletion", file)
		}
	}

	// Delete the job
	err = backend.DeleteJob(jobID)
	if err != nil {
		t.Errorf("Failed to delete job: %v", err)
	}

	// Verify directory is gone (all files deleted)
	if _, err := os.Stat(jobEventsDir); !os.IsNotExist(err) {
		t.Error("Expected job events directory to be deleted")
	}
}

func TestLocalBackend_ReadFileEvents_NotFound(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.StorageConfig{
		Type: "local",
		Local: config.LocalConfig{
			Logs: config.LogStorageConfig{
				Directory: filepath.Join(tmpDir, "logs"),
			},
			Metrics: config.MetricStorageConfig{
				Directory: filepath.Join(tmpDir, "metrics"),
			},
			Events: config.EventStorageConfig{
				Directory: filepath.Join(tmpDir, "events"),
			},
		},
	}

	log := logger.New()
	backend, err := NewLocalBackend(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	query := &TelemetryQuery{
		JobUUID: "non-existent-job",
		Limit:   100,
	}

	ctx := context.Background()
	reader, err := backend.ReadFileEvents(ctx, query)
	if err != nil {
		t.Fatalf("ReadFileEvents should not error for missing job: %v", err)
	}

	// Should return empty channel
	var readEvents []*ipcpb.FileEvent
	for {
		select {
		case event, ok := <-reader.Channel:
			if !ok {
				goto done
			}
			readEvents = append(readEvents, event)
		case <-time.After(500 * time.Millisecond):
			goto done
		}
	}

done:
	if len(readEvents) != 0 {
		t.Errorf("Expected 0 events for non-existent job, got %d", len(readEvents))
	}
}

// TestLocalBackend_ReadMetrics_MultipleGzipStreams tests that ReadMetrics can read
// metrics from multiple gzip streams (each WriteMetrics call creates a new stream)
func TestLocalBackend_ReadMetrics_MultipleGzipStreams(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.StorageConfig{
		Type: "local",
		Local: config.LocalConfig{
			Logs: config.LogStorageConfig{
				Directory: filepath.Join(tmpDir, "logs"),
			},
			Metrics: config.MetricStorageConfig{
				Directory: filepath.Join(tmpDir, "metrics"),
			},
			Events: config.EventStorageConfig{
				Directory: filepath.Join(tmpDir, "events"),
			},
		},
	}

	log := logger.New()
	backend, err := NewLocalBackend(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	jobID := "test-job-multi-stream"

	// Write metrics in 5 separate batches (each creates a new gzip stream)
	totalMetrics := 0
	for batch := 0; batch < 5; batch++ {
		metrics := []*ipcpb.Metric{
			{
				JobUuid:   jobID,
				Timestamp: time.Now().UnixNano() + int64(batch*1000),
				Sequence:  uint64(batch*2 + 1),
				Data: &ipcpb.MetricData{
					CpuUsage:    float64(batch*10 + 1),
					MemoryUsage: int64(batch*1000 + 100),
				},
			},
			{
				JobUuid:   jobID,
				Timestamp: time.Now().UnixNano() + int64(batch*1000+500),
				Sequence:  uint64(batch*2 + 2),
				Data: &ipcpb.MetricData{
					CpuUsage:    float64(batch*10 + 2),
					MemoryUsage: int64(batch*1000 + 200),
				},
			},
		}

		err = backend.WriteMetrics(jobID, metrics)
		if err != nil {
			t.Fatalf("Failed to write metrics batch %d: %v", batch, err)
		}
		totalMetrics += len(metrics)
	}

	time.Sleep(100 * time.Millisecond)

	// Read all metrics back - should get all 10, not just the first 2
	query := &MetricQuery{
		JobUUID: jobID,
		Limit:   100,
	}

	ctx := context.Background()
	reader, err := backend.ReadMetrics(ctx, query)
	if err != nil {
		t.Fatalf("Failed to read metrics: %v", err)
	}

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
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for metrics")
		}
	}

done:
	if len(readMetrics) != totalMetrics {
		t.Errorf("Expected %d metrics (from 5 batches), got %d - multi-stream gzip reading may be broken", totalMetrics, len(readMetrics))
	}

	// Verify we got metrics from different batches (check CPU values)
	seenCpuValues := make(map[float64]bool)
	for _, m := range readMetrics {
		seenCpuValues[m.Data.CpuUsage] = true
	}

	// Should have metrics with CPU values like 1, 2, 11, 12, 21, 22, 31, 32, 41, 42
	if len(seenCpuValues) != totalMetrics {
		t.Errorf("Expected %d unique CPU values, got %d - suggests some batches weren't read", totalMetrics, len(seenCpuValues))
	}
}

// TestLocalBackend_ReadExecEvents_MultipleGzipStreams tests multi-stream reading for exec events
func TestLocalBackend_ReadExecEvents_MultipleGzipStreams(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.StorageConfig{
		Type: "local",
		Local: config.LocalConfig{
			Logs: config.LogStorageConfig{
				Directory: filepath.Join(tmpDir, "logs"),
			},
			Metrics: config.MetricStorageConfig{
				Directory: filepath.Join(tmpDir, "metrics"),
			},
			Events: config.EventStorageConfig{
				Directory: filepath.Join(tmpDir, "events"),
			},
		},
	}

	log := logger.New()
	backend, err := NewLocalBackend(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	jobID := "test-job-exec-multi-stream"

	// Write exec events in 3 separate batches
	totalEvents := 0
	for batch := 0; batch < 3; batch++ {
		events := []*ipcpb.ExecEvent{
			{
				JobUuid:   jobID,
				Timestamp: time.Now().UnixNano() + int64(batch*1000),
				Sequence:  uint64(batch*2 + 1),
				Pid:       uint32(1000 + batch*10 + 1),
				Filename:  "/bin/bash",
				Args:      []string{"-c", "echo", "batch", string(rune('0' + batch))},
			},
			{
				JobUuid:   jobID,
				Timestamp: time.Now().UnixNano() + int64(batch*1000+500),
				Sequence:  uint64(batch*2 + 2),
				Pid:       uint32(1000 + batch*10 + 2),
				Filename:  "/usr/bin/cat",
				Args:      []string{"/tmp/file"},
			},
		}

		err = backend.WriteExecEvents(jobID, events)
		if err != nil {
			t.Fatalf("Failed to write exec events batch %d: %v", batch, err)
		}
		totalEvents += len(events)
	}

	time.Sleep(100 * time.Millisecond)

	// Read all events back
	query := &TelemetryQuery{
		JobUUID: jobID,
		Limit:   100,
	}

	ctx := context.Background()
	reader, err := backend.ReadExecEvents(ctx, query)
	if err != nil {
		t.Fatalf("Failed to read exec events: %v", err)
	}

	var readEvents []*ipcpb.ExecEvent
	for {
		select {
		case event, ok := <-reader.Channel:
			if !ok {
				goto done
			}
			readEvents = append(readEvents, event)
		case err := <-reader.Error:
			if err != nil {
				t.Fatalf("Error reading exec events: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for exec events")
		}
	}

done:
	if len(readEvents) != totalEvents {
		t.Errorf("Expected %d exec events (from 3 batches), got %d - multi-stream gzip reading may be broken", totalEvents, len(readEvents))
	}

	// Verify we got events from different batches (check PID values)
	seenPids := make(map[uint32]bool)
	for _, e := range readEvents {
		seenPids[e.Pid] = true
	}

	if len(seenPids) != totalEvents {
		t.Errorf("Expected %d unique PIDs, got %d - suggests some batches weren't read", totalEvents, len(seenPids))
	}
}

// TestLocalBackend_ReadConnectEvents_MultipleGzipStreams tests multi-stream reading for connect events
func TestLocalBackend_ReadConnectEvents_MultipleGzipStreams(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.StorageConfig{
		Type: "local",
		Local: config.LocalConfig{
			Logs: config.LogStorageConfig{
				Directory: filepath.Join(tmpDir, "logs"),
			},
			Metrics: config.MetricStorageConfig{
				Directory: filepath.Join(tmpDir, "metrics"),
			},
			Events: config.EventStorageConfig{
				Directory: filepath.Join(tmpDir, "events"),
			},
		},
	}

	log := logger.New()
	backend, err := NewLocalBackend(cfg, log)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	jobID := "test-job-connect-multi-stream"

	// Write connect events in 4 separate batches
	totalEvents := 0
	for batch := 0; batch < 4; batch++ {
		events := []*ipcpb.ConnectEvent{
			{
				JobUuid:   jobID,
				Timestamp: time.Now().UnixNano() + int64(batch*1000),
				Sequence:  uint64(batch + 1),
				Pid:       uint32(2000 + batch),
				DstAddr:   "192.168.1." + string(rune('1'+batch)),
				DstPort:   uint32(8000 + batch),
			},
		}

		err = backend.WriteConnectEvents(jobID, events)
		if err != nil {
			t.Fatalf("Failed to write connect events batch %d: %v", batch, err)
		}
		totalEvents += len(events)
	}

	time.Sleep(100 * time.Millisecond)

	// Read all events back
	query := &TelemetryQuery{
		JobUUID: jobID,
		Limit:   100,
	}

	ctx := context.Background()
	reader, err := backend.ReadConnectEvents(ctx, query)
	if err != nil {
		t.Fatalf("Failed to read connect events: %v", err)
	}

	var readEvents []*ipcpb.ConnectEvent
	for {
		select {
		case event, ok := <-reader.Channel:
			if !ok {
				goto done
			}
			readEvents = append(readEvents, event)
		case err := <-reader.Error:
			if err != nil {
				t.Fatalf("Error reading connect events: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for connect events")
		}
	}

done:
	if len(readEvents) != totalEvents {
		t.Errorf("Expected %d connect events (from 4 batches), got %d - multi-stream gzip reading may be broken", totalEvents, len(readEvents))
	}

	// Verify we got events from different batches (check port values)
	seenPorts := make(map[uint32]bool)
	for _, e := range readEvents {
		seenPorts[e.DstPort] = true
	}

	if len(seenPorts) != totalEvents {
		t.Errorf("Expected %d unique ports, got %d - suggests some batches weren't read", totalEvents, len(seenPorts))
	}
}
