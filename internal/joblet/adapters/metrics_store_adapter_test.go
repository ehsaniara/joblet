package adapters

import (
	"context"
	"testing"
	"time"

	metricsdomain "github.com/ehsaniara/joblet/internal/joblet/metrics/domain"
	"github.com/ehsaniara/joblet/internal/joblet/pubsub"
	"github.com/ehsaniara/joblet/pkg/logger"
	"github.com/stretchr/testify/assert"
)

// TestPublishMetrics_PersistEnabled verifies that metrics ARE buffered when persist is enabled
func TestPublishMetrics_PersistEnabled(t *testing.T) {
	// Setup
	log := logger.New()
	ps := pubsub.NewPubSub[MetricsEvent]()

	adapter := NewMetricsStoreAdapter(ps, nil, true, log) // persistEnabled = true

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
		},
	}

	err := adapter.PublishMetrics(context.Background(), sample)
	assert.NoError(t, err, "PublishMetrics should not return error")

	// Verify: Sample should be in buffer (persist enabled)
	bufferedSamples := adapter.buffer.GetRecent(jobID, 10)
	assert.Equal(t, 1, len(bufferedSamples), "Buffer should contain 1 sample when persist enabled")
	assert.Equal(t, sample.CPU.UsagePercent, bufferedSamples[0].CPU.UsagePercent)
	assert.Equal(t, sample.Memory.Current, bufferedSamples[0].Memory.Current)
}

// TestPublishMetrics_PersistDisabled verifies that metrics are NOT buffered when persist is disabled
func TestPublishMetrics_PersistDisabled(t *testing.T) {
	// Setup
	log := logger.New()
	ps := pubsub.NewPubSub[MetricsEvent]()

	adapter := NewMetricsStoreAdapter(ps, nil, false, log) // persistEnabled = false

	// Test: Publish a metrics sample
	jobID := "test-job-456"
	sample := &metricsdomain.JobMetricsSample{
		JobID:     jobID,
		Timestamp: time.Now(),
		CPU: metricsdomain.CPUMetrics{
			UsagePercent: 75.3,
		},
		Memory: metricsdomain.MemoryMetrics{
			Current: 2048,
		},
	}

	err := adapter.PublishMetrics(context.Background(), sample)
	assert.NoError(t, err, "PublishMetrics should not return error")

	// Verify: Sample should NOT be in buffer (persist disabled)
	bufferedSamples := adapter.buffer.GetRecent(jobID, 10)
	assert.Equal(t, 0, len(bufferedSamples), "Buffer should be empty when persist disabled (no buffering)")
}

// TestPublishMetrics_MultipleSamples_PersistEnabled verifies multiple samples are buffered
func TestPublishMetrics_MultipleSamples_PersistEnabled(t *testing.T) {
	// Setup
	log := logger.New()
	ps := pubsub.NewPubSub[MetricsEvent]()

	adapter := NewMetricsStoreAdapter(ps, nil, true, log) // persistEnabled = true

	// Test: Publish multiple samples
	jobID := "test-job-789"
	samples := []*metricsdomain.JobMetricsSample{
		{
			JobID:     jobID,
			Timestamp: time.Now(),
			CPU:       metricsdomain.CPUMetrics{UsagePercent: 10.0},
			Memory:    metricsdomain.MemoryMetrics{Current: 100},
		},
		{
			JobID:     jobID,
			Timestamp: time.Now().Add(5 * time.Second),
			CPU:       metricsdomain.CPUMetrics{UsagePercent: 20.0},
			Memory:    metricsdomain.MemoryMetrics{Current: 200},
		},
		{
			JobID:     jobID,
			Timestamp: time.Now().Add(10 * time.Second),
			CPU:       metricsdomain.CPUMetrics{UsagePercent: 30.0},
			Memory:    metricsdomain.MemoryMetrics{Current: 300},
		},
	}

	ctx := context.Background()
	for _, sample := range samples {
		err := adapter.PublishMetrics(ctx, sample)
		assert.NoError(t, err)
	}

	// Verify: All samples should be in buffer
	bufferedSamples := adapter.buffer.GetRecent(jobID, 10)
	assert.Equal(t, 3, len(bufferedSamples), "Buffer should contain 3 samples when persist enabled")
	assert.Equal(t, 10.0, bufferedSamples[0].CPU.UsagePercent)
	assert.Equal(t, 20.0, bufferedSamples[1].CPU.UsagePercent)
	assert.Equal(t, 30.0, bufferedSamples[2].CPU.UsagePercent)
}

// TestPublishMetrics_MultipleSamples_PersistDisabled verifies multiple samples skip buffering
func TestPublishMetrics_MultipleSamples_PersistDisabled(t *testing.T) {
	// Setup
	log := logger.New()
	ps := pubsub.NewPubSub[MetricsEvent]()

	adapter := NewMetricsStoreAdapter(ps, nil, false, log) // persistEnabled = false

	// Test: Publish multiple samples
	jobID := "test-job-000"
	samples := []*metricsdomain.JobMetricsSample{
		{JobID: jobID, Timestamp: time.Now(), CPU: metricsdomain.CPUMetrics{UsagePercent: 10.0}},
		{JobID: jobID, Timestamp: time.Now(), CPU: metricsdomain.CPUMetrics{UsagePercent: 20.0}},
		{JobID: jobID, Timestamp: time.Now(), CPU: metricsdomain.CPUMetrics{UsagePercent: 30.0}},
	}

	ctx := context.Background()
	for _, sample := range samples {
		err := adapter.PublishMetrics(ctx, sample)
		assert.NoError(t, err)
	}

	// Verify: Buffer should remain empty (all writes skipped)
	bufferedSamples := adapter.buffer.GetRecent(jobID, 10)
	assert.Equal(t, 0, len(bufferedSamples), "Buffer should remain empty when persist disabled (no buffering)")
}

// TestMetricsBuffer_CircularBehavior verifies the circular buffer still works when persist enabled
func TestMetricsBuffer_CircularBehavior_PersistEnabled(t *testing.T) {
	// Setup - buffer capacity is 100 samples
	log := logger.New()
	ps := pubsub.NewPubSub[MetricsEvent]()
	adapter := NewMetricsStoreAdapter(ps, nil, true, log) // persistEnabled = true

	jobID := "test-job-circular"
	ctx := context.Background()

	// Test: Add 110 samples (exceeds capacity of 100)
	for i := 0; i < 110; i++ {
		sample := &metricsdomain.JobMetricsSample{
			JobID:     jobID,
			Timestamp: time.Now().Add(time.Duration(i) * time.Second),
			CPU:       metricsdomain.CPUMetrics{UsagePercent: float64(i)},
			Memory:    metricsdomain.MemoryMetrics{Current: uint64(i * 10)},
		}
		err := adapter.PublishMetrics(ctx, sample)
		assert.NoError(t, err)
	}

	// Verify: Buffer should contain only last 100 samples (circular behavior)
	bufferedSamples := adapter.buffer.GetRecent(jobID, 200) // Ask for more than capacity
	assert.LessOrEqual(t, len(bufferedSamples), 100, "Buffer should not exceed capacity of 100")

	// Oldest sample should be sample #10 (0-9 overwritten)
	if len(bufferedSamples) == 100 {
		assert.Equal(t, 10.0, bufferedSamples[0].CPU.UsagePercent, "Oldest sample should be #10 (0-9 overwritten)")
		assert.Equal(t, 109.0, bufferedSamples[99].CPU.UsagePercent, "Newest sample should be #109")
	}
}

// TestMetricsBuffer_NoPersist_NoOverflowRisk verifies no overflow when persist disabled
func TestMetricsBuffer_NoPersist_NoOverflowRisk(t *testing.T) {
	// Setup
	log := logger.New()
	ps := pubsub.NewPubSub[MetricsEvent]()
	adapter := NewMetricsStoreAdapter(ps, nil, false, log) // persistEnabled = false

	jobID := "test-job-no-persist"
	ctx := context.Background()

	// Test: Add many samples (would overflow if buffered)
	for i := 0; i < 1000; i++ {
		sample := &metricsdomain.JobMetricsSample{
			JobID:     jobID,
			Timestamp: time.Now(),
			CPU:       metricsdomain.CPUMetrics{UsagePercent: float64(i)},
		}
		err := adapter.PublishMetrics(ctx, sample)
		assert.NoError(t, err)
	}

	// Verify: Buffer should remain empty (no overflow risk)
	bufferedSamples := adapter.buffer.GetRecent(jobID, 10)
	assert.Equal(t, 0, len(bufferedSamples), "Buffer should remain empty when persist disabled")
}

// TestMetricsBuffer_PerJobIsolation verifies buffer isolation between jobs
func TestMetricsBuffer_PerJobIsolation_PersistEnabled(t *testing.T) {
	// Setup
	log := logger.New()
	ps := pubsub.NewPubSub[MetricsEvent]()
	adapter := NewMetricsStoreAdapter(ps, nil, true, log) // persistEnabled = true

	ctx := context.Background()

	// Test: Publish samples for two different jobs
	job1 := "job-1"
	job2 := "job-2"

	err := adapter.PublishMetrics(ctx, &metricsdomain.JobMetricsSample{
		JobID:     job1,
		Timestamp: time.Now(),
		CPU:       metricsdomain.CPUMetrics{UsagePercent: 10.0},
	})
	assert.NoError(t, err)

	err = adapter.PublishMetrics(ctx, &metricsdomain.JobMetricsSample{
		JobID:     job2,
		Timestamp: time.Now(),
		CPU:       metricsdomain.CPUMetrics{UsagePercent: 20.0},
	})
	assert.NoError(t, err)

	// Verify: Each job has its own buffer
	samples1 := adapter.buffer.GetRecent(job1, 10)
	samples2 := adapter.buffer.GetRecent(job2, 10)

	assert.Equal(t, 1, len(samples1), "Job 1 should have 1 sample")
	assert.Equal(t, 1, len(samples2), "Job 2 should have 1 sample")
	assert.Equal(t, 10.0, samples1[0].CPU.UsagePercent, "Job 1 sample should have correct CPU usage")
	assert.Equal(t, 20.0, samples2[0].CPU.UsagePercent, "Job 2 sample should have correct CPU usage")
}
