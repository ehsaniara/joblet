package ipc

import (
	"context"
	"sync/atomic"

	"github.com/ehsaniara/joblet/internal/joblet/adapters"
	"github.com/ehsaniara/joblet/internal/joblet/metrics/domain"
	"github.com/ehsaniara/joblet/internal/joblet/pubsub"
	ipcpb "github.com/ehsaniara/joblet/internal/proto/gen/ipc"
	"github.com/ehsaniara/joblet/pkg/logger"
)

// MetricsSubscriber subscribes to metrics pub/sub and forwards to IPC writer
type MetricsSubscriber struct {
	writer *Writer
	pubsub pubsub.PubSub[adapters.MetricsEvent]
	logger *logger.Logger

	// Metrics
	eventsProcessed atomic.Uint64
	metricsSent     atomic.Uint64
	errors          atomic.Uint64

	// Lifecycle
	ctx         context.Context
	cancel      context.CancelFunc
	unsubscribe func()
}

// NewMetricsSubscriber creates a new metrics IPC subscriber
func NewMetricsSubscriber(writer *Writer, ps pubsub.PubSub[adapters.MetricsEvent], log *logger.Logger) *MetricsSubscriber {
	ctx, cancel := context.WithCancel(context.Background())

	return &MetricsSubscriber{
		writer: writer,
		pubsub: ps,
		logger: log.WithField("component", "ipc-metrics-subscriber"),
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start begins subscribing to metrics events
func (s *MetricsSubscriber) Start() error {
	// Subscribe to the single "metrics" topic (all jobs publish here)
	updates, unsubscribe, err := s.pubsub.Subscribe(s.ctx, "metrics")
	if err != nil {
		return err
	}

	s.unsubscribe = unsubscribe
	s.logger.Info("Metrics IPC subscriber started, listening to metrics events")

	// Process events in background
	go s.processEvents(updates)

	return nil
}

// processEvents handles incoming metrics pub/sub events
func (s *MetricsSubscriber) processEvents(updates <-chan pubsub.Message[adapters.MetricsEvent]) {
	sequence := make(map[string]uint64) // jobID -> sequence number

	for {
		select {
		case <-s.ctx.Done():
			return
		case msg, ok := <-updates:
			if !ok {
				s.logger.Info("Metrics pub/sub channel closed")
				return
			}

			s.eventsProcessed.Add(1)
			event := msg.Payload

			// Only process METRICS_SAMPLE events
			if event.Type != "METRICS_SAMPLE" {
				continue
			}

			if event.Sample == nil {
				continue
			}

			// Get or initialize sequence for this job
			jobID := event.JobID
			seq := sequence[jobID]
			sequence[jobID] = seq + 1

			// Convert domain metrics to IPC proto format
			metric := s.convertToIPCMetric(event.Sample, seq)

			// Send to IPC writer
			if err := s.writer.WriteMetric(jobID, event.Timestamp, seq, metric); err != nil {
				s.errors.Add(1)
				s.logger.Warn("Failed to write metrics to IPC",
					"jobID", jobID,
					"error", err)
			} else {
				s.metricsSent.Add(1)
				s.logger.Info("Forwarded metrics sample to IPC persist",
					"jobID", jobID,
					"sequence", seq,
					"timestamp", event.Sample.Timestamp)
			}
		}
	}
}

// convertToIPCMetric converts domain.JobMetricsSample to ipcpb.MetricData
func (s *MetricsSubscriber) convertToIPCMetric(sample *domain.JobMetricsSample, sequence uint64) *ipcpb.MetricData {
	// Calculate GPU usage average if GPUs present
	gpuUsage := 0.0
	if len(sample.GPU) > 0 {
		totalUtil := 0.0
		for _, gpu := range sample.GPU {
			totalUtil += gpu.Utilization
		}
		gpuUsage = totalUtil / float64(len(sample.GPU)) / 100.0 // Convert to 0.0-1.0 range
	}

	// Create network IO (may be nil)
	var networkIO *ipcpb.NetworkIO
	if sample.Network != nil {
		networkIO = &ipcpb.NetworkIO{
			RxBytes:   int64(sample.Network.TotalRxBytes),
			TxBytes:   int64(sample.Network.TotalTxBytes),
			RxPackets: int64(sample.Network.TotalRxPackets),
			TxPackets: int64(sample.Network.TotalTxPackets),
		}
	}

	metric := &ipcpb.MetricData{
		CpuUsage:    sample.CPU.UsagePercent / 100.0, // Convert to cores usage (0.0 - N.0)
		MemoryUsage: int64(sample.Memory.Current),
		GpuUsage:    gpuUsage,
		DiskIo: &ipcpb.DiskIO{
			ReadBytes:  int64(sample.IO.TotalReadBytes),
			WriteBytes: int64(sample.IO.TotalWriteBytes),
			ReadOps:    int64(sample.IO.TotalReadOps),
			WriteOps:   int64(sample.IO.TotalWriteOps),
		},
		NetworkIo: networkIO,
	}

	return metric
}

// Stop stops the metrics subscriber
func (s *MetricsSubscriber) Stop() {
	s.logger.Info("Stopping metrics IPC subscriber")
	s.cancel()

	if s.unsubscribe != nil {
		s.unsubscribe()
	}

	s.logger.Info("Metrics IPC subscriber stopped",
		"eventsProcessed", s.eventsProcessed.Load(),
		"metricsSent", s.metricsSent.Load(),
		"errors", s.errors.Load())
}

// Stats returns metrics subscriber statistics
func (s *MetricsSubscriber) Stats() MetricsSubscriberStats {
	return MetricsSubscriberStats{
		EventsProcessed: s.eventsProcessed.Load(),
		MetricsSent:     s.metricsSent.Load(),
		Errors:          s.errors.Load(),
	}
}

// MetricsSubscriberStats represents metrics subscriber statistics
type MetricsSubscriberStats struct {
	EventsProcessed uint64
	MetricsSent     uint64
	Errors          uint64
}
