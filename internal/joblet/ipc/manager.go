package ipc

import (
	"fmt"
	"time"

	"github.com/ehsaniara/joblet/internal/joblet/adapters"
	"github.com/ehsaniara/joblet/internal/joblet/pubsub"
	"github.com/ehsaniara/joblet/pkg/logger"
)

// Manager coordinates IPC writer and log subscriber
type Manager struct {
	writer        *Writer
	logSubscriber *Subscriber
	logger        *logger.Logger
}

// ManagerConfig configures the IPC manager
type ManagerConfig struct {
	Enabled        bool
	Socket         string
	BufferSize     int
	ReconnectDelay time.Duration
	MaxReconnects  int
}

// NewManager creates a new IPC manager with log subscriber
// Metrics are written directly via GetWriter()
func NewManager(
	cfg *ManagerConfig,
	logPubSub pubsub.PubSub[adapters.JobEvent],
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

	return &Manager{
		writer:        writer,
		logSubscriber: logSubscriber,
		logger:        log.WithField("component", "ipc-manager"),
	}, nil
}

// GetWriter returns the IPC writer for direct metric writing
func (m *Manager) GetWriter() *Writer {
	return m.writer
}

// Start starts the IPC manager and log subscriber
func (m *Manager) Start() error {
	if m.writer == nil {
		m.logger.Debug("IPC not enabled, skipping start")
		return nil
	}

	// Start log subscriber
	if err := m.logSubscriber.Start(); err != nil {
		return fmt.Errorf("failed to start log IPC subscriber: %w", err)
	}

	m.logger.Info("IPC manager started (logs)")
	return nil
}

// Stop stops the IPC manager and log subscriber
func (m *Manager) Stop() error {
	if m.writer == nil {
		return nil
	}

	m.logger.Info("Stopping IPC manager")

	// Stop log subscriber
	if m.logSubscriber != nil {
		m.logSubscriber.Stop()
	}

	// Stop writer
	if m.writer != nil {
		m.writer.Close()
	}

	m.logger.Info("IPC manager stopped")
	return nil
}
