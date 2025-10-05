package metrics

import (
	"testing"
	"time"

	"joblet/internal/joblet/metrics/domain"
	"joblet/pkg/logger"
)

func TestNewAsyncMetricsSystem(t *testing.T) {
	config := &domain.MetricsConfig{
		Enabled:           true,
		DefaultSampleRate: 5 * time.Second,
		Storage: domain.StorageConfig{
			Directory: "/tmp/test-metrics",
			Retention: domain.RetentionConfig{
				Days: 7,
			},
		},
	}

	system := NewAsyncMetricsSystem(config, logger.New())

	if system == nil {
		t.Fatal("NewAsyncMetricsSystem returned nil")
	}

	if system.metricsQueue == nil {
		t.Error("metrics queue not initialized")
	}

	if system.diskWriter == nil {
		t.Error("disk writer not initialized")
	}

	system.Close()
}

func TestAsyncMetricsSystem_WriteMetrics(t *testing.T) {
	config := &domain.MetricsConfig{
		Enabled:           true,
		DefaultSampleRate: 5 * time.Second,
		Storage: domain.StorageConfig{
			Directory: "/tmp/test-metrics-write",
			Retention: domain.RetentionConfig{
				Days: 7,
			},
		},
	}

	system := NewAsyncMetricsSystem(config, logger.New())
	defer system.Close()

	sample := &domain.JobMetricsSample{
		JobID:     "test-job-123",
		Timestamp: time.Now(),
		CPU: domain.CPUMetrics{
			UsagePercent: 50.0,
		},
		Memory: domain.MemoryMetrics{
			Current:      1024 * 1024 * 100, // 100MB
			UsagePercent: 25.0,
		},
	}

	// Should not block
	system.WriteMetrics(sample)

	// Give it a moment to process
	time.Sleep(100 * time.Millisecond)
}
