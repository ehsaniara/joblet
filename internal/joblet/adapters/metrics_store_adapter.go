package adapters

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ehsaniara/joblet/internal/joblet/metrics"
	"github.com/ehsaniara/joblet/internal/joblet/metrics/domain"
	"github.com/ehsaniara/joblet/internal/joblet/pubsub"
	"github.com/ehsaniara/joblet/pkg/logger"
)

// MetricsStoreAdapter implements metrics storage with pub-sub capabilities
// Metrics are published to pubsub for:
// 1. Real-time streaming to clients (StreamJobMetrics)
// 2. IPC forwarding to persist subprocess for disk storage
type MetricsStoreAdapter struct {
	// Pub-sub for real-time metrics streaming and IPC forwarding
	pubsub pubsub.PubSub[MetricsEvent]

	// Active collectors per job
	collectors      map[string]*metrics.Collector
	collectorsMutex sync.RWMutex

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
	log *logger.Logger,
) *MetricsStoreAdapter {
	if log == nil {
		log = logger.New().WithField("component", "metrics-store-adapter")
	}

	adapter := &MetricsStoreAdapter{
		pubsub:     pubsub,
		collectors: make(map[string]*metrics.Collector),
		logger:     log,
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
		sampleInterval = 5 * time.Second // Default to 5 seconds
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
// Metrics are published to pubsub for:
// 1. Real-time streaming to clients (StreamJobMetrics)
// 2. IPC forwarding to persist subprocess (MetricsSubscriber listens and forwards)
func (a *MetricsStoreAdapter) PublishMetrics(ctx context.Context, sample *domain.JobMetricsSample) error {
	// Publish to pub-sub for real-time streaming and IPC forwarding
	if a.pubsub != nil {
		event := MetricsEvent{
			Type:      "METRICS_SAMPLE",
			JobID:     sample.JobID,
			Sample:    sample,
			Timestamp: time.Now().Unix(),
		}

		// Publish to a single topic "metrics" for all jobs
		// Individual job filtering happens in StreamMetrics() by jobID
		topic := "metrics"
		if err := a.pubsub.Publish(ctx, topic, event); err != nil {
			a.logger.Warn("failed to publish metrics event", "jobId", sample.JobID, "error", err)
			// Don't return error - pubsub failure shouldn't fail the collector
		} else {
			a.logger.Info("published metrics sample to pubsub", "jobId", sample.JobID, "topic", topic, "timestamp", sample.Timestamp)
		}
	}

	return nil
}

// StreamMetrics streams real-time metrics for a job
// Historical metrics are retrieved via joblet-persist gRPC service (not implemented here)
func (a *MetricsStoreAdapter) StreamMetrics(
	ctx context.Context,
	jobID string,
	callback func(*domain.JobMetricsSample) error,
) error {
	a.logger.Debug("starting metrics stream", "jobId", jobID)

	// Check if there's an active collector for this job
	a.collectorsMutex.RLock()
	hasCollector := a.collectors[jobID] != nil
	a.collectorsMutex.RUnlock()

	// If no pubsub, we can't stream live metrics
	if a.pubsub == nil {
		a.logger.Warn("no pubsub available for live streaming", "jobId", jobID)
		return nil
	}

	if hasCollector {
		a.logger.Debug("streaming live metrics for running job", "jobId", jobID)
	} else {
		a.logger.Debug("no active collector - will stream any remaining metrics with drain timeout", "jobId", jobID)
	}

	// Subscribe to live metrics via pub-sub (single "metrics" topic for all jobs)
	topic := "metrics"
	a.logger.Debug("subscribing to live metrics stream", "jobId", jobID, "topic", topic)

	updates, unsubscribe, err := a.pubsub.Subscribe(ctx, topic)
	if err != nil {
		a.logger.Error("failed to subscribe to metrics", "jobId", jobID, "error", err)
		return fmt.Errorf("failed to subscribe to metrics: %w", err)
	}
	defer unsubscribe()

	a.logger.Debug("successfully subscribed to live metrics", "jobId", jobID)

	// Drain timeout: if no active collector, wait for final metrics samples
	// Metrics are sampled every ~5 seconds, so wait up to 6 seconds for any final samples
	var drainDeadline time.Time
	inDrainMode := !hasCollector
	if inDrainMode {
		drainDeadline = time.Now().Add(6 * time.Second)
		a.logger.Debug("entering drain mode for final metrics", "jobId", jobID, "drainDeadline", drainDeadline)
	}

	receivedAnySample := false

	for {
		// If in drain mode and deadline exceeded, terminate
		if inDrainMode && time.Now().After(drainDeadline) {
			if receivedAnySample {
				a.logger.Debug("drain deadline exceeded after receiving samples, ending stream", "jobId", jobID, "samplesReceived", receivedAnySample)
			} else {
				a.logger.Debug("drain deadline exceeded with no samples, ending stream", "jobId", jobID)
			}
			return nil
		}

		// Compute select timeout based on drain mode
		var selectTimeout <-chan time.Time
		if inDrainMode {
			// During drain, use short timeout to check deadline frequently
			selectTimeout = time.After(500 * time.Millisecond)
		} else {
			// Not in drain mode, check periodically if collector stopped
			selectTimeout = time.After(2 * time.Second)
		}

		select {
		case <-ctx.Done():
			a.logger.Debug("metrics stream context cancelled", "jobId", jobID)
			return ctx.Err()

		case <-selectTimeout:
			if !inDrainMode {
				// Check if collector has stopped (job completed)
				a.collectorsMutex.RLock()
				hasCollector = a.collectors[jobID] != nil
				a.collectorsMutex.RUnlock()

				if !hasCollector {
					// Collector stopped, enter drain mode
					inDrainMode = true
					drainDeadline = time.Now().Add(6 * time.Second)
					a.logger.Debug("collector stopped, entering drain mode", "jobId", jobID, "drainDeadline", drainDeadline)
				}
			}
			// Continue to check deadline at top of loop
			continue

		case msg, ok := <-updates:
			if !ok {
				a.logger.Debug("metrics updates channel closed", "jobId", jobID)
				return nil
			}

			event := msg.Payload
			// Filter for this specific job since all jobs use the same topic
			if event.Type == "METRICS_SAMPLE" && event.Sample != nil && event.JobID == jobID {
				receivedAnySample = true

				if err := callback(event.Sample); err != nil {
					a.logger.Warn("metrics callback error", "jobId", jobID, "error", err)
					return err
				}

				a.logger.Debug("streamed metrics sample", "jobId", jobID, "timestamp", event.Sample.Timestamp)
			}
		}
	}
}

// GetHistoricalMetrics reads historical metrics from joblet-persist service
// This is a placeholder - actual implementation should query persist gRPC service
func (a *MetricsStoreAdapter) GetHistoricalMetrics(
	jobID string,
	from time.Time,
	to time.Time,
) ([]*domain.JobMetricsSample, error) {
	return nil, fmt.Errorf("historical metrics should be queried from joblet-persist gRPC service")
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

	// Metrics files are stored by persist - deletion should be requested from persist gRPC service
	a.logger.Info("stopped metrics collector for job (files managed by persist)", "jobId", jobID)
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

	// Close pub-sub (optional)
	if a.pubsub != nil {
		if err := a.pubsub.Close(); err != nil {
			a.logger.Error("failed to close metrics pub-sub", "error", err)
		}
	}

	a.logger.Info("metrics store adapter closed successfully")
	return nil
}
