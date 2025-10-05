package adapters

import (
	"context"
	"testing"
	"time"

	"joblet/internal/joblet/metrics/domain"
	"joblet/pkg/logger"
)

func TestNewMetricsStoreAdapter(t *testing.T) {
	config := &domain.MetricsConfig{
		Enabled:           true,
		DefaultSampleRate: 5 * time.Second,
		Storage: domain.StorageConfig{
			Directory: "/tmp/test-metrics-adapter",
			Retention: domain.RetentionConfig{
				Days: 7,
			},
		},
	}

	adapter := NewMetricsStoreAdapter(nil, config, logger.New())

	if adapter == nil {
		t.Fatal("NewMetricsStoreAdapter returned nil")
	}

	if adapter.config != config {
		t.Error("config not set correctly")
	}

	if adapter.collectors == nil {
		t.Error("collectors map not initialized")
	}

	if config.Enabled && adapter.asyncMetricsSystem == nil {
		t.Error("async metrics system not initialized when enabled")
	}

	adapter.Close()
}

func TestMetricsStoreAdapter_PublishMetrics_NilPubSub(t *testing.T) {
	config := &domain.MetricsConfig{
		Enabled:           true,
		DefaultSampleRate: 5 * time.Second,
		Storage: domain.StorageConfig{
			Directory: "/tmp/test-metrics-publish",
			Retention: domain.RetentionConfig{
				Days: 7,
			},
		},
	}

	adapter := NewMetricsStoreAdapter(nil, config, logger.New())
	defer adapter.Close()

	sample := &domain.JobMetricsSample{
		JobID:     "test-job-456",
		Timestamp: time.Now(),
		CPU: domain.CPUMetrics{
			UsagePercent: 75.0,
		},
	}

	// Should not panic with nil pubsub
	err := adapter.PublishMetrics(context.Background(), sample)
	if err != nil {
		t.Errorf("PublishMetrics failed: %v", err)
	}
}

func TestMetricsStoreAdapter_Close(t *testing.T) {
	config := &domain.MetricsConfig{
		Enabled:           true,
		DefaultSampleRate: 5 * time.Second,
		Storage: domain.StorageConfig{
			Directory: "/tmp/test-metrics-close",
			Retention: domain.RetentionConfig{
				Days: 7,
			},
		},
	}

	adapter := NewMetricsStoreAdapter(nil, config, logger.New())

	err := adapter.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Verify close was successful (adapter should cleanup resources)
	if !adapter.closed {
		t.Error("Adapter not marked as closed")
	}
}
