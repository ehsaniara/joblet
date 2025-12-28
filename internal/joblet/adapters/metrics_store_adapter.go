package adapters

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ehsaniara/joblet/internal/joblet/metrics"
	"github.com/ehsaniara/joblet/internal/joblet/metrics/domain"
	"github.com/ehsaniara/joblet/internal/joblet/telemetry"
	pb "github.com/ehsaniara/joblet/internal/proto/gen/persist"
	"github.com/ehsaniara/joblet/pkg/logger"
)

// MetricsStoreAdapter manages metrics collection for jobs.
// Metrics are collected from cgroups and emitted to the unified telemetry collector.
type MetricsStoreAdapter struct {
	// Persist client for deleting historical metrics
	persistClient pb.PersistServiceClient

	// Telemetry collector for unified telemetry streaming
	telemetryCollector *telemetry.Collector

	// Active collectors per job
	collectors      map[string]*metrics.Collector
	collectorsMutex sync.RWMutex

	logger     *logger.Logger
	closed     bool
	closeMutex sync.RWMutex
}

// NewMetricsStoreAdapter creates a new metrics store adapter
func NewMetricsStoreAdapter(
	persistClient pb.PersistServiceClient,
	telemetryCollector *telemetry.Collector,
	log *logger.Logger,
) *MetricsStoreAdapter {
	if log == nil {
		log = logger.New().WithField("component", "metrics-store-adapter")
	}

	adapter := &MetricsStoreAdapter{
		persistClient:      persistClient,
		telemetryCollector: telemetryCollector,
		collectors:         make(map[string]*metrics.Collector),
		logger:             log,
	}

	if persistClient == nil {
		log.Warn("metrics store adapter created without persist client - historical metrics deletion will not work")
	}

	if telemetryCollector != nil {
		log.Info("telemetry collector attached for unified telemetry streaming")
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
	a.logger.Info("started metrics collector", "job_uuid", jobID, "interval", sampleInterval)

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
		a.logger.Warn("error stopping collector", "job_uuid", jobID, "error", err)
	}

	delete(a.collectors, jobID)
	a.logger.Info("stopped metrics collector", "job_uuid", jobID)

	return nil
}

// PublishMetrics implements the MetricsPublisher interface.
// This is called by the Collector to publish metrics samples to the telemetry collector.
func (a *MetricsStoreAdapter) PublishMetrics(ctx context.Context, sample *domain.JobMetricsSample) error {
	// Emit to telemetry collector (for StreamJobMetrics API)
	if a.telemetryCollector != nil {
		telemetryData := &telemetry.MetricsData{
			CPUPercent:     sample.CPU.UsagePercent,
			MemoryBytes:    int64(sample.Memory.Current),
			MemoryLimit:    int64(sample.Memory.Max),
			DiskReadBytes:  int64(sample.IO.TotalReadBytes),
			DiskWriteBytes: int64(sample.IO.TotalWriteBytes),
		}
		// Add network metrics if available
		if sample.Network != nil {
			telemetryData.NetRecvBytes = int64(sample.Network.TotalRxBytes)
			telemetryData.NetSentBytes = int64(sample.Network.TotalTxBytes)
		}
		// Add GPU metrics if available (use first GPU for summary)
		if len(sample.GPU) > 0 {
			telemetryData.GPUPercent = sample.GPU[0].Utilization
			telemetryData.GPUMemoryBytes = int64(sample.GPU[0].MemoryUsed)
		}
		a.telemetryCollector.EmitMetrics(sample.JobUUID, telemetryData)
	}

	return nil
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

	// Clear telemetry buffer for this job
	if a.telemetryCollector != nil {
		a.telemetryCollector.ClearJob(jobID)
	}

	// Metrics files are stored by persist - request deletion via persist gRPC service
	if a.persistClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		resp, err := a.persistClient.DeleteJob(ctx, &pb.DeleteJobRequest{
			JobUuid: jobID,
		})
		if err != nil {
			a.logger.Warn("failed to delete metrics from persist storage", "job_uuid", jobID, "error", err)
			return fmt.Errorf("failed to delete metrics from persist storage: %w", err)
		}

		if !resp.Success {
			a.logger.Warn("persist reported metrics deletion failure", "job_uuid", jobID, "message", resp.Message)
			return fmt.Errorf("persist metrics deletion failed: %s", resp.Message)
		}

		a.logger.Info("successfully deleted metrics from persist storage", "job_uuid", jobID)
	} else {
		a.logger.Warn("persist client not available - cannot delete historical metrics files", "job_uuid", jobID)
	}

	a.logger.Info("stopped metrics collector and cleared telemetry for job", "job_uuid", jobID)
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
			a.logger.Warn("error stopping collector during shutdown", "job_uuid", jobID, "error", err)
		}
	}
	a.collectors = make(map[string]*metrics.Collector)
	a.collectorsMutex.Unlock()

	a.logger.Info("metrics store adapter closed successfully")
	return nil
}
