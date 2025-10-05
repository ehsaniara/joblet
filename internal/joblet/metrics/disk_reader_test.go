package metrics

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"joblet/internal/joblet/metrics/domain"
	"joblet/pkg/logger"
)

func TestDiskReader_ReadJobMetrics(t *testing.T) {
	// Create a test directory
	testDir := filepath.Join("/tmp", "test-metrics-reader")
	defer os.RemoveAll(testDir)

	config := &domain.MetricsConfig{
		Enabled:           true,
		DefaultSampleRate: 5 * time.Second,
		Storage: domain.StorageConfig{
			Directory: testDir,
			Retention: domain.RetentionConfig{
				Days: 7,
			},
		},
	}

	// Create async metrics system (includes writer and reader)
	system := NewAsyncMetricsSystem(config, logger.New())

	// Write some test samples
	jobID := "test-job-read-123"
	now := time.Now()

	samples := []*domain.JobMetricsSample{
		{
			JobID:          jobID,
			Timestamp:      now.Add(-10 * time.Second),
			SampleInterval: 5 * time.Second,
			CPU: domain.CPUMetrics{
				UsagePercent: 50.0,
			},
		},
		{
			JobID:          jobID,
			Timestamp:      now.Add(-5 * time.Second),
			SampleInterval: 5 * time.Second,
			CPU: domain.CPUMetrics{
				UsagePercent: 75.0,
			},
		},
		{
			JobID:          jobID,
			Timestamp:      now,
			SampleInterval: 5 * time.Second,
			CPU: domain.CPUMetrics{
				UsagePercent: 90.0,
			},
		},
	}

	// Write samples
	for _, sample := range samples {
		system.WriteMetrics(sample)
	}

	// Wait for async write to complete and flush to disk
	time.Sleep(6 * time.Second) // Wait for flush interval (5 seconds) + buffer

	// Close the system to ensure all files are flushed and closed
	system.Close()

	// Create a new reader to read the closed files
	reader := NewMetricsDiskReader(testDir, logger.New())

	// Read all metrics
	readSamples, err := reader.ReadJobMetrics(jobID, time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("failed to read metrics: %v", err)
	}

	if len(readSamples) != len(samples) {
		t.Errorf("expected %d samples, got %d", len(samples), len(readSamples))
	}

	// Verify samples are sorted by timestamp
	for i := 1; i < len(readSamples); i++ {
		if readSamples[i].Timestamp.Before(readSamples[i-1].Timestamp) {
			t.Error("samples not sorted by timestamp")
		}
	}

	// Verify CPU values
	expectedCPU := []float64{50.0, 75.0, 90.0}
	for i, sample := range readSamples {
		if sample.CPU.UsagePercent != expectedCPU[i] {
			t.Errorf("sample %d: expected CPU %.1f, got %.1f", i, expectedCPU[i], sample.CPU.UsagePercent)
		}
	}
}

func TestDiskReader_ReadJobMetrics_TimeRange(t *testing.T) {
	// Create a test directory
	testDir := filepath.Join("/tmp", "test-metrics-reader-range")
	defer os.RemoveAll(testDir)

	config := &domain.MetricsConfig{
		Enabled:           true,
		DefaultSampleRate: 5 * time.Second,
		Storage: domain.StorageConfig{
			Directory: testDir,
			Retention: domain.RetentionConfig{
				Days: 7,
			},
		},
	}

	// Create async metrics system
	system := NewAsyncMetricsSystem(config, logger.New())

	// Write samples across a time range
	jobID := "test-job-range-456"
	baseTime := time.Now().Add(-30 * time.Second)

	for i := 0; i < 6; i++ {
		sample := &domain.JobMetricsSample{
			JobID:          jobID,
			Timestamp:      baseTime.Add(time.Duration(i*5) * time.Second),
			SampleInterval: 5 * time.Second,
			CPU: domain.CPUMetrics{
				UsagePercent: float64(10 * (i + 1)),
			},
		}
		system.WriteMetrics(sample)
	}

	// Wait for async write and flush to disk
	time.Sleep(6 * time.Second) // Wait for flush interval (5 seconds) + buffer

	// Close the system to ensure all files are flushed and closed
	system.Close()

	// Create a new reader to read the closed files
	reader := NewMetricsDiskReader(testDir, logger.New())

	// Read metrics in the middle range (samples 2-4)
	fromTime := baseTime.Add(10 * time.Second)
	toTime := baseTime.Add(20 * time.Second)

	readSamples, err := reader.ReadJobMetrics(jobID, fromTime, toTime)
	if err != nil {
		t.Fatalf("failed to read metrics: %v", err)
	}

	// Should get samples at 10s, 15s, 20s (3 samples)
	if len(readSamples) != 3 {
		t.Errorf("expected 3 samples in range, got %d", len(readSamples))
	}

	// Verify CPU values are from middle samples
	expectedCPU := []float64{30.0, 40.0, 50.0}
	for i, sample := range readSamples {
		if sample.CPU.UsagePercent != expectedCPU[i] {
			t.Errorf("sample %d: expected CPU %.1f, got %.1f", i, expectedCPU[i], sample.CPU.UsagePercent)
		}
	}
}

func TestDiskReader_ReadJobMetrics_NotFound(t *testing.T) {
	testDir := filepath.Join("/tmp", "test-metrics-reader-notfound")
	defer os.RemoveAll(testDir)

	reader := NewMetricsDiskReader(testDir, logger.New())

	_, err := reader.ReadJobMetrics("nonexistent-job", time.Time{}, time.Time{})
	if err == nil {
		t.Error("expected error for nonexistent job, got nil")
	}
}
