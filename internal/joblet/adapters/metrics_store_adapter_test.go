package adapters

import (
	"context"
	"testing"
	"time"

	metricsdomain "github.com/ehsaniara/joblet/internal/joblet/metrics/domain"
	"github.com/ehsaniara/joblet/internal/joblet/telemetry"
	"github.com/ehsaniara/joblet/pkg/logger"
	"github.com/stretchr/testify/assert"
)

// TestPublishMetrics_WithTelemetryCollector verifies metrics are emitted to telemetry collector
func TestPublishMetrics_WithTelemetryCollector(t *testing.T) {
	// Setup
	log := logger.New()
	collector := telemetry.NewCollector(100)
	adapter := NewMetricsStoreAdapter(nil, collector, log)

	// Test: Publish a metrics sample
	jobID := "test-job-123"
	sample := &metricsdomain.JobMetricsSample{
		JobID:     jobID,
		Timestamp: time.Now(),
		CPU: metricsdomain.CPUMetrics{
			UsagePercent: 50.5,
		},
		Memory: metricsdomain.MemoryMetrics{
			Current: 1024,
			Max:     2048,
		},
		IO: metricsdomain.IOMetrics{
			TotalReadBytes:  1000,
			TotalWriteBytes: 2000,
		},
	}

	err := adapter.PublishMetrics(context.Background(), sample)
	assert.NoError(t, err, "PublishMetrics should not return error")

	// Verify: Sample should be in telemetry collector
	events := collector.GetBufferedEvents(jobID, nil, time.Time{}, time.Time{}, 10)
	assert.Equal(t, 1, len(events), "Telemetry collector should contain 1 event")
	assert.Equal(t, telemetry.EventTypeMetrics, events[0].Type)

	metricsData := events[0].Data.(*telemetry.MetricsData)
	assert.Equal(t, sample.CPU.UsagePercent, metricsData.CPUPercent)
	assert.Equal(t, int64(sample.Memory.Current), metricsData.MemoryBytes)
	assert.Equal(t, int64(sample.Memory.Max), metricsData.MemoryLimit)
}

// TestPublishMetrics_WithoutTelemetryCollector verifies no error when collector is nil
func TestPublishMetrics_WithoutTelemetryCollector(t *testing.T) {
	// Setup
	log := logger.New()
	adapter := NewMetricsStoreAdapter(nil, nil, log)

	// Test: Publish a metrics sample
	sample := &metricsdomain.JobMetricsSample{
		JobID:     "test-job-456",
		Timestamp: time.Now(),
		CPU: metricsdomain.CPUMetrics{
			UsagePercent: 75.3,
		},
		Memory: metricsdomain.MemoryMetrics{
			Current: 512,
		},
	}

	err := adapter.PublishMetrics(context.Background(), sample)
	assert.NoError(t, err, "PublishMetrics should not return error even without collector")
}

// TestPublishMetrics_WithGPUMetrics verifies GPU metrics are included
func TestPublishMetrics_WithGPUMetrics(t *testing.T) {
	// Setup
	log := logger.New()
	collector := telemetry.NewCollector(100)
	adapter := NewMetricsStoreAdapter(nil, collector, log)

	// Test: Publish a metrics sample with GPU
	jobID := "test-job-gpu"
	sample := &metricsdomain.JobMetricsSample{
		JobID:     jobID,
		Timestamp: time.Now(),
		CPU: metricsdomain.CPUMetrics{
			UsagePercent: 25.0,
		},
		Memory: metricsdomain.MemoryMetrics{
			Current: 1024,
		},
		GPU: []metricsdomain.GPUMetrics{
			{
				Index:       0,
				Utilization: 80.5,
				MemoryUsed:  4096,
			},
		},
	}

	err := adapter.PublishMetrics(context.Background(), sample)
	assert.NoError(t, err)

	// Verify GPU metrics
	events := collector.GetBufferedEvents(jobID, nil, time.Time{}, time.Time{}, 10)
	assert.Equal(t, 1, len(events))

	metricsData := events[0].Data.(*telemetry.MetricsData)
	assert.Equal(t, 80.5, metricsData.GPUPercent)
	assert.Equal(t, int64(4096), metricsData.GPUMemoryBytes)
}

// TestPublishMetrics_WithNetworkMetrics verifies network metrics are included
func TestPublishMetrics_WithNetworkMetrics(t *testing.T) {
	// Setup
	log := logger.New()
	collector := telemetry.NewCollector(100)
	adapter := NewMetricsStoreAdapter(nil, collector, log)

	// Test: Publish a metrics sample with network
	jobID := "test-job-network"
	sample := &metricsdomain.JobMetricsSample{
		JobID:     jobID,
		Timestamp: time.Now(),
		CPU: metricsdomain.CPUMetrics{
			UsagePercent: 10.0,
		},
		Memory: metricsdomain.MemoryMetrics{
			Current: 512,
		},
		Network: &metricsdomain.NetworkMetrics{
			TotalRxBytes: 5000,
			TotalTxBytes: 3000,
		},
	}

	err := adapter.PublishMetrics(context.Background(), sample)
	assert.NoError(t, err)

	// Verify network metrics
	events := collector.GetBufferedEvents(jobID, nil, time.Time{}, time.Time{}, 10)
	assert.Equal(t, 1, len(events))

	metricsData := events[0].Data.(*telemetry.MetricsData)
	assert.Equal(t, int64(5000), metricsData.NetRecvBytes)
	assert.Equal(t, int64(3000), metricsData.NetSentBytes)
}

// TestDeleteJobMetrics verifies job cleanup
func TestDeleteJobMetrics(t *testing.T) {
	// Setup
	log := logger.New()
	collector := telemetry.NewCollector(100)
	adapter := NewMetricsStoreAdapter(nil, collector, log)

	// Add some metrics
	jobID := "test-job-delete"
	sample := &metricsdomain.JobMetricsSample{
		JobID:     jobID,
		Timestamp: time.Now(),
		CPU:       metricsdomain.CPUMetrics{UsagePercent: 50.0},
		Memory:    metricsdomain.MemoryMetrics{Current: 1024},
	}
	_ = adapter.PublishMetrics(context.Background(), sample)

	// Verify metrics exist
	events := collector.GetBufferedEvents(jobID, nil, time.Time{}, time.Time{}, 10)
	assert.Equal(t, 1, len(events))

	// Delete job metrics
	err := adapter.DeleteJobMetrics(jobID)
	assert.NoError(t, err)

	// Verify metrics cleared
	events = collector.GetBufferedEvents(jobID, nil, time.Time{}, time.Time{}, 10)
	assert.Equal(t, 0, len(events))
}

// TestClose verifies adapter shutdown
func TestClose(t *testing.T) {
	log := logger.New()
	adapter := NewMetricsStoreAdapter(nil, nil, log)

	err := adapter.Close()
	assert.NoError(t, err)

	// Second close should be no-op
	err = adapter.Close()
	assert.NoError(t, err)
}
