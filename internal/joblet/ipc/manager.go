package ipc

import (
	"fmt"
	"time"

	"github.com/ehsaniara/joblet/internal/joblet/adapters"
	"github.com/ehsaniara/joblet/internal/joblet/pubsub"
	"github.com/ehsaniara/joblet/pkg/logger"
)

// Manager coordinates IPC writer and subscribers (logs and metrics)
type Manager struct {
	writer            *Writer
	logSubscriber     *Subscriber
	metricsSubscriber *MetricsSubscriber
	logger            *logger.Logger
}

// ManagerConfig configures the IPC manager
type ManagerConfig struct {
	Enabled        bool
	Socket         string
	BufferSize     int
	ReconnectDelay time.Duration
	MaxReconnects  int
}

// NewManager creates a new IPC manager with both log and metrics subscribers
func NewManager(
	cfg *ManagerConfig,
	logPubSub pubsub.PubSub[adapters.JobEvent],
	metricsPubSub pubsub.PubSub[adapters.MetricsEvent],
	log *logger.Logger,
) (*Manager, error) {
	if !cfg.Enabled {
		log.Info("IPC disabled in configuration")
		return &Manager{logger: log}, nil
	}

	// Create writer
	writerCfg := &Config{
		Socket:         cfg.Socket,
		BufferSize:     cfg.BufferSize,
		ReconnectDelay: cfg.ReconnectDelay,
		MaxReconnects:  cfg.MaxReconnects,
	}

	writer := NewWriter(writerCfg, log)

	// Create log subscriber
	logSubscriber := NewSubscriber(writer, logPubSub, log)

	// Create metrics subscriber
	metricsSubscriber := NewMetricsSubscriber(writer, metricsPubSub, log)

	return &Manager{
		writer:            writer,
		logSubscriber:     logSubscriber,
		metricsSubscriber: metricsSubscriber,
		logger:            log.WithField("component", "ipc-manager"),
	}, nil
}

// Start starts the IPC manager and all subscribers
func (m *Manager) Start() error {
	if m.writer == nil {
		m.logger.Debug("IPC not enabled, skipping start")
		return nil
	}

	// Start log subscriber (writer is already started in constructor)
	if err := m.logSubscriber.Start(); err != nil {
		return fmt.Errorf("failed to start log IPC subscriber: %w", err)
	}

	// Start metrics subscriber
	if err := m.metricsSubscriber.Start(); err != nil {
		m.logSubscriber.Stop() // Clean up log subscriber
		return fmt.Errorf("failed to start metrics IPC subscriber: %w", err)
	}

	m.logger.Info("IPC manager started (logs and metrics)")
	return nil
}

// Stop stops the IPC manager and all subscribers
func (m *Manager) Stop() error {
	if m.writer == nil {
		return nil
	}

	m.logger.Info("Stopping IPC manager")

	// Stop both subscribers first
	if m.logSubscriber != nil {
		m.logSubscriber.Stop()
	}

	if m.metricsSubscriber != nil {
		m.metricsSubscriber.Stop()
	}

	// Stop writer
	if m.writer != nil {
		m.writer.Close()
	}

	m.logger.Info("IPC manager stopped (logs and metrics)")
	return nil
}

// Stats returns combined statistics for logs and metrics
func (m *Manager) Stats() *Stats {
	if m.writer == nil {
		return &Stats{}
	}

	writerStats := m.writer.Stats()
	logStats := m.logSubscriber.Stats()
	metricsStats := m.metricsSubscriber.Stats()

	return &Stats{
		Connected:             writerStats.Connected,
		MsgsSent:              writerStats.MsgsSent,
		MsgsDropped:           writerStats.MsgsDropped,
		WriteErrors:           writerStats.WriteErrors,
		LogEventsProcessed:    logStats.EventsProcessed,
		LogsSent:              logStats.LogsSent,
		MetricEventsProcessed: metricsStats.EventsProcessed,
		MetricsSent:           metricsStats.MetricsSent,
	}
}

// Stats represents combined IPC statistics for logs and metrics
type Stats struct {
	Connected             bool
	MsgsSent              uint64
	MsgsDropped           uint64
	WriteErrors           uint64
	LogEventsProcessed    uint64
	LogsSent              uint64
	MetricEventsProcessed uint64
	MetricsSent           uint64
}
