package adapters

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"joblet/internal/joblet/metrics"
	"joblet/internal/joblet/metrics/domain"
	"joblet/internal/joblet/pubsub"
	"joblet/pkg/logger"
)

// MetricsStoreAdapter implements metrics storage with pub-sub capabilities
// Follows the same pattern as JobStoreAdapter but for time-series metrics
type MetricsStoreAdapter struct {
	// Pub-sub for real-time metrics streaming
	pubsub pubsub.PubSub[MetricsEvent]

	// Async metrics system for rate-decoupled persistence
	asyncMetricsSystem *metrics.AsyncMetricsSystem

	// Active collectors per job
	collectors      map[string]*metrics.Collector
	collectorsMutex sync.RWMutex

	// Configuration
	config *domain.MetricsConfig

	logger     *logger.Logger
	closed     bool
	closeMutex sync.RWMutex
}

// MetricsEvent represents events published about job metrics
type MetricsEvent struct {
	Type      string                   `json:"type"` // METRICS_SAMPLE
	JobID     string                   `json:"job_id"`
	Sample    *domain.JobMetricsSample `json:"sample,omitempty"`
	Timestamp int64                    `json:"timestamp"`
}

// NewMetricsStoreAdapter creates a new metrics store adapter
func NewMetricsStoreAdapter(
	pubsub pubsub.PubSub[MetricsEvent],
	config *domain.MetricsConfig,
	log *logger.Logger,
) *MetricsStoreAdapter {
	if log == nil {
		log = logger.New().WithField("component", "metrics-store-adapter")
	}

	adapter := &MetricsStoreAdapter{
		pubsub:     pubsub,
		config:     config,
		collectors: make(map[string]*metrics.Collector),
		logger:     log,
	}

	// Initialize async metrics system for persistence
	if config != nil && config.Enabled {
		adapter.asyncMetricsSystem = metrics.NewAsyncMetricsSystem(config, log)
	}

	// Debug log to verify pubsub is set
	if pubsub == nil {
		log.Warn("metrics store adapter created with nil pubsub - live streaming will not work")
	} else {
		log.Info("metrics store adapter created with pubsub - live streaming enabled")
	}

	return adapter
}

// StartCollector starts metrics collection for a job
func (a *MetricsStoreAdapter) StartCollector(
	jobID string,
	cgroupPath string,
	sampleInterval time.Duration,
	limits *domain.ResourceLimits,
	gpuIndices []int,
) error {
	a.closeMutex.RLock()
	if a.closed {
		a.closeMutex.RUnlock()
		return fmt.Errorf("metrics store is closed")
	}
	a.closeMutex.RUnlock()

	// Check if collector already exists
	a.collectorsMutex.Lock()
	defer a.collectorsMutex.Unlock()

	if _, exists := a.collectors[jobID]; exists {
		return fmt.Errorf("collector already exists for job %s", jobID)
	}

	// Use default sample interval if not specified
	if sampleInterval == 0 {
		sampleInterval = 5 * time.Second
		if a.config != nil && a.config.DefaultSampleRate > 0 {
			sampleInterval = a.config.DefaultSampleRate
		}
	}

	// Create collector with this adapter as the publisher
	collector := metrics.NewCollector(
		jobID,
		cgroupPath,
		sampleInterval,
		limits,
		gpuIndices,
		a, // MetricsStoreAdapter implements MetricsPublisher
	)

	// Start the collector
	if err := collector.Start(); err != nil {
		return fmt.Errorf("failed to start collector: %w", err)
	}

	a.collectors[jobID] = collector
	a.logger.Info("started metrics collector", "jobId", jobID, "interval", sampleInterval)

	return nil
}

// StopCollector stops metrics collection for a job
func (a *MetricsStoreAdapter) StopCollector(jobID string) error {
	a.collectorsMutex.Lock()
	defer a.collectorsMutex.Unlock()

	collector, exists := a.collectors[jobID]
	if !exists {
		return fmt.Errorf("no collector found for job %s", jobID)
	}

	if err := collector.Stop(); err != nil {
		a.logger.Warn("error stopping collector", "jobId", jobID, "error", err)
	}

	delete(a.collectors, jobID)
	a.logger.Info("stopped metrics collector", "jobId", jobID)

	return nil
}

// PublishMetrics implements the MetricsPublisher interface
// This is called by the Collector to publish metrics samples
func (a *MetricsStoreAdapter) PublishMetrics(ctx context.Context, sample *domain.JobMetricsSample) error {
	// Write to async metrics system for persistence
	if a.asyncMetricsSystem != nil {
		a.asyncMetricsSystem.WriteMetrics(sample)
	}

	// Publish to pub-sub for real-time streaming (optional)
	if a.pubsub != nil {
		event := MetricsEvent{
			Type:      "METRICS_SAMPLE",
			JobID:     sample.JobID,
			Sample:    sample,
			Timestamp: time.Now().Unix(),
		}

		topic := fmt.Sprintf("metrics.job.%s", sample.JobID)
		if err := a.pubsub.Publish(ctx, topic, event); err != nil {
			a.logger.Warn("failed to publish metrics event", "jobId", sample.JobID, "error", err)
			// Don't return error - pubsub is optional
		} else {
			a.logger.Debug("published metrics sample", "jobId", sample.JobID, "timestamp", sample.Timestamp)
		}
	}

	return nil
}

// StreamMetrics streams historical metrics first, then real-time metrics for a job
// This allows users to see the complete time-series from start to current
func (a *MetricsStoreAdapter) StreamMetrics(
	ctx context.Context,
	jobID string,
	callback func(*domain.JobMetricsSample) error,
) error {
	a.logger.Debug("starting metrics stream", "jobId", jobID)

	// First, send all historical metrics from disk
	if a.asyncMetricsSystem != nil {
		reader := a.asyncMetricsSystem.GetReader()
		if reader != nil {
			samples, err := reader.ReadJobMetrics(jobID, time.Time{}, time.Time{})
			if err != nil {
				// If no historical metrics found, that's okay - job might be just starting
				if !strings.Contains(err.Error(), "no metrics") {
					a.logger.Warn("failed to read historical metrics", "jobId", jobID, "error", err)
				}
			} else {
				a.logger.Debug("sending historical metrics", "jobId", jobID, "count", len(samples))
				for _, sample := range samples {
					if err := callback(sample); err != nil {
						a.logger.Warn("failed to send historical sample", "jobId", jobID, "error", err)
						return err
					}
				}
			}
		}
	}

	// Check if there's an active collector for this job
	a.collectorsMutex.RLock()
	_, hasCollector := a.collectors[jobID]
	a.collectorsMutex.RUnlock()

	// If no active collector, job is completed - return after historical metrics
	if !hasCollector {
		a.logger.Debug("no active collector for job, returning historical metrics only", "jobId", jobID)
		return nil
	}

	// If no pubsub, we can't stream live metrics
	if a.pubsub == nil {
		a.logger.Warn("no pubsub available for live streaming, returning historical metrics only", "jobId", jobID)
		return nil
	}

	a.logger.Debug("streaming live metrics for running job", "jobId", jobID)

	// Now stream live metrics via pub-sub
	topic := fmt.Sprintf("metrics.job.%s", jobID)
	a.logger.Debug("subscribing to live metrics stream", "jobId", jobID, "topic", topic)

	updates, unsubscribe, err := a.pubsub.Subscribe(ctx, topic)
	if err != nil {
		a.logger.Error("failed to subscribe to metrics", "jobId", jobID, "error", err)
		return fmt.Errorf("failed to subscribe to metrics: %w", err)
	}
	defer unsubscribe()

	a.logger.Debug("successfully subscribed to live metrics", "jobId", jobID)

	for {
		select {
		case <-ctx.Done():
			a.logger.Debug("metrics stream context cancelled", "jobId", jobID)
			return ctx.Err()

		case msg, ok := <-updates:
			if !ok {
				a.logger.Debug("metrics updates channel closed", "jobId", jobID)
				return nil
			}

			event := msg.Payload
			if event.Type == "METRICS_SAMPLE" && event.Sample != nil {
				if err := callback(event.Sample); err != nil {
					a.logger.Warn("metrics callback error", "jobId", jobID, "error", err)
					return err
				}
			}
		}
	}
}

// GetHistoricalMetrics reads historical metrics from disk
func (a *MetricsStoreAdapter) GetHistoricalMetrics(
	jobID string,
	from time.Time,
	to time.Time,
) ([]*domain.JobMetricsSample, error) {
	if a.asyncMetricsSystem == nil {
		return nil, fmt.Errorf("metrics system not initialized")
	}

	reader := a.asyncMetricsSystem.GetReader()
	if reader == nil {
		return nil, fmt.Errorf("metrics reader not available")
	}

	samples, err := reader.ReadJobMetrics(jobID, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to read historical metrics: %w", err)
	}

	return samples, nil
}

// DeleteJobMetrics deletes all metrics for a specific job
func (a *MetricsStoreAdapter) DeleteJobMetrics(jobID string) error {
	// Stop collector if running
	a.collectorsMutex.Lock()
	if collector, exists := a.collectors[jobID]; exists {
		_ = collector.Stop()
		delete(a.collectors, jobID)
	}
	a.collectorsMutex.Unlock()

	// Delete metrics files
	if a.asyncMetricsSystem != nil {
		if err := a.asyncMetricsSystem.DeleteJobMetricsFiles(jobID); err != nil {
			a.logger.Warn("failed to delete metrics files", "jobId", jobID, "error", err)
			return err
		}
	}

	a.logger.Info("deleted job metrics", "jobId", jobID)
	return nil
}

// GetSystemMetrics returns metrics about the metrics system itself
func (a *MetricsStoreAdapter) GetSystemMetrics() *metrics.MetricsSystemMetrics {
	if a.asyncMetricsSystem != nil {
		return a.asyncMetricsSystem.GetMetrics()
	}
	return nil
}

// Close gracefully shuts down the metrics store adapter
func (a *MetricsStoreAdapter) Close() error {
	a.closeMutex.Lock()
	defer a.closeMutex.Unlock()

	if a.closed {
		return nil
	}
	a.closed = true

	// Stop all collectors
	a.collectorsMutex.Lock()
	for jobID, collector := range a.collectors {
		if err := collector.Stop(); err != nil {
			a.logger.Warn("error stopping collector during shutdown", "jobId", jobID, "error", err)
		}
	}
	a.collectors = make(map[string]*metrics.Collector)
	a.collectorsMutex.Unlock()

	// Close async metrics system
	if a.asyncMetricsSystem != nil {
		if err := a.asyncMetricsSystem.Close(); err != nil {
			a.logger.Error("failed to close async metrics system", "error", err)
		}
	}

	// Close pub-sub (optional)
	if a.pubsub != nil {
		if err := a.pubsub.Close(); err != nil {
			a.logger.Error("failed to close metrics pub-sub", "error", err)
		}
	}

	a.logger.Info("metrics store adapter closed successfully")
	return nil
}
