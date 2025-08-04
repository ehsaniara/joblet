package monitoring

import (
	"context"
	"sync"
	"time"

	"joblet/internal/joblet/monitoring/cloud"
	"joblet/internal/joblet/monitoring/collectors"
	"joblet/internal/joblet/monitoring/domain"
	"joblet/pkg/config"
	"joblet/pkg/logger"
)

// Service is the main monitoring service coordinator
type Service struct {
	mu     sync.RWMutex
	config *domain.MonitoringConfig
	logger *logger.Logger

	// Current system metrics (latest snapshot)
	currentMetrics *domain.SystemMetrics

	// Collectors
	hostCollector    *collectors.HostCollector
	cpuCollector     *collectors.CPUCollector
	memoryCollector  *collectors.MemoryCollector
	diskCollector    *collectors.DiskCollector
	networkCollector *collectors.NetworkCollector
	ioCollector      *collectors.IOCollector
	processCollector *collectors.ProcessCollector

	// Cloud detection
	cloudDetector *cloud.Detector
	cloudInfo     *domain.CloudInfo

	// Control
	ctx     context.Context
	cancel  context.CancelFunc
	running bool
	wg      sync.WaitGroup
}

// NewService creates a new monitoring service with the specified configuration.
// It initializes all metric collectors (CPU, memory, disk, network, I/O, process),
// sets up cloud detection, and prepares the service for monitoring operations.
// If config is nil, it uses default configuration values.
// Returns a fully configured Service instance ready to be started.
func NewService(config *domain.MonitoringConfig) *Service {
	if config == nil {
		config = DefaultConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	service := &Service{
		config: config,
		logger: logger.WithField("component", "monitoring-service"),

		// Initialize collectors
		hostCollector:    collectors.NewHostCollector(),
		cpuCollector:     collectors.NewCPUCollector(),
		memoryCollector:  collectors.NewMemoryCollector(),
		diskCollector:    collectors.NewDiskCollector(),
		networkCollector: collectors.NewNetworkCollector(),
		ioCollector:      collectors.NewIOCollector(),
		processCollector: collectors.NewProcessCollector(),

		// Cloud detection
		cloudDetector: cloud.NewDetector(),

		ctx:    ctx,
		cancel: cancel,
	}

	return service
}

// NewServiceFromConfig creates a new monitoring service from configuration package types.
// This is a convenience constructor that converts config.MonitoringConfig to domain.MonitoringConfig
// and creates a new Service instance. This bridges the gap between the config package
// and the monitoring domain package types.
// Returns a new Service instance configured with the provided settings.
func NewServiceFromConfig(cfg *config.MonitoringConfig) *Service {
	domainConfig := &domain.MonitoringConfig{
		Enabled: cfg.Enabled,
		Collection: domain.CollectionConfig{
			SystemInterval:  cfg.SystemInterval,
			ProcessInterval: cfg.ProcessInterval,
			CloudDetection:  cfg.CloudDetection,
		},
	}
	return NewService(domainConfig)
}

// DefaultConfig returns a default monitoring configuration with sensible defaults.
// Sets system metrics collection interval to 10 seconds, process metrics to 60 seconds,
// enables cloud detection, and activates monitoring by default.
// Used as fallback configuration when no specific config is provided.
func DefaultConfig() *domain.MonitoringConfig {
	return &domain.MonitoringConfig{
		Enabled: true,
		Collection: domain.CollectionConfig{
			SystemInterval:  10 * time.Second,
			ProcessInterval: 60 * time.Second,
			CloudDetection:  true,
		},
	}
}

// Start initiates the monitoring service and begins metric collection.
// Creates background goroutines for:
//   - Cloud environment detection (if enabled)
//   - System metrics collection (CPU, memory, disk, network, I/O)
//   - Process metrics collection (running processes and statistics)
//
// The service runs continuously until Stop() is called.
// Returns error if service is already running or fails to start.
// Thread-safe and idempotent - multiple calls have no effect.
func (s *Service) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return nil
	}

	if !s.config.Enabled {
		s.logger.Info("monitoring service disabled")
		return nil
	}

	s.logger.Info("starting monitoring service")

	// Detect cloud environment if enabled
	if s.config.Collection.CloudDetection {
		s.wg.Add(1)
		go s.detectCloudEnvironment()
	}

	// Start system metrics collection
	s.wg.Add(1)
	go s.collectSystemMetrics()

	// Start process metrics collection (less frequent)
	s.wg.Add(1)
	go s.collectProcessMetrics()

	s.running = true
	s.logger.Info("monitoring service started")

	return nil
}

// Stop gracefully shuts down the monitoring service.
// Cancels the context to signal all goroutines to stop,
// waits for all background collection routines to finish,
// and marks the service as stopped.
// Thread-safe and idempotent - multiple calls have no effect.
// Returns error (currently always nil for interface compatibility).
func (s *Service) Stop() error {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return nil
	}

	s.logger.Info("stopping monitoring service")

	// Cancel context to stop all goroutines
	s.cancel()

	// Release the lock before waiting to avoid deadlock
	s.mu.Unlock()

	// Wait for all goroutines to finish
	s.wg.Wait()

	// Acquire lock again to set running state
	s.mu.Lock()
	s.running = false
	s.mu.Unlock()

	s.logger.Info("monitoring service stopped")

	return nil
}

// IsRunning returns the current running state of the monitoring service.
// Thread-safe method that checks if the service is actively collecting metrics.
// Returns true if Start() has been called and Stop() has not been called,
// false otherwise.
func (s *Service) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// GetLatestMetrics returns the most recently collected system metrics snapshot.
// Provides current readings for CPU, memory, disk, network, I/O, and process metrics.
// Returns nil if no metrics have been collected yet (service not started or just started).
// Thread-safe method that returns a pointer to the internal metrics structure.
func (s *Service) GetLatestMetrics() *domain.SystemMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentMetrics
}

// GetCloudInfo returns information about the detected cloud environment.
// Provides details about the cloud provider (AWS, GCP, Azure, etc.) if detected.
// Returns nil if cloud detection is disabled, failed, or no cloud environment detected.
// Thread-safe method that returns cached cloud detection results.
func (s *Service) GetCloudInfo() *domain.CloudInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cloudInfo
}

// GetSystemStatus returns a comprehensive view of the current system state.
// Combines latest metrics with cloud information into a single status object.
// Includes availability flag indicating if metrics collection is working.
// Returns SystemStatus with Available=false if no metrics have been collected yet.
// Thread-safe method suitable for health checks and monitoring dashboards.
func (s *Service) GetSystemStatus() *SystemStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.currentMetrics == nil {
		return &SystemStatus{
			Timestamp: time.Now(),
			Available: false,
		}
	}

	return &SystemStatus{
		Timestamp: s.currentMetrics.Timestamp,
		Available: true,
		Host:      s.currentMetrics.Host,
		CPU:       s.currentMetrics.CPU,
		Memory:    s.currentMetrics.Memory,
		Disk:      s.currentMetrics.Disk,
		Network:   s.currentMetrics.Network,
		IO:        s.currentMetrics.IO,
		Processes: s.currentMetrics.Processes,
		Cloud:     s.cloudInfo,
	}
}

// SystemStatus represents the current system status
type SystemStatus struct {
	Timestamp time.Time               `json:"timestamp"`
	Available bool                    `json:"available"`
	Host      domain.HostInfo         `json:"host"`
	CPU       domain.CPUMetrics       `json:"cpu"`
	Memory    domain.MemoryMetrics    `json:"memory"`
	Disk      []domain.DiskMetrics    `json:"disk"`
	Network   []domain.NetworkMetrics `json:"network"`
	IO        domain.IOMetrics        `json:"io"`
	Processes domain.ProcessMetrics   `json:"processes"`
	Cloud     *domain.CloudInfo       `json:"cloud,omitempty"`
}

// detectCloudEnvironment performs cloud provider detection in a background goroutine.
// Attempts to identify the cloud environment (AWS, GCP, Azure, etc.) by checking
// metadata services and environment indicators with a 5-second timeout.
// Updates the service's cloudInfo field with detection results.
// Handles context cancellation gracefully and logs detection results.
// Runs once at service startup if cloud detection is enabled.
func (s *Service) detectCloudEnvironment() {
	defer s.wg.Done()

	s.logger.Debug("detecting cloud environment")

	// Check if context is already cancelled
	select {
	case <-s.ctx.Done():
		s.logger.Debug("cloud detection cancelled before starting")
		return
	default:
	}

	// Run detection with timeout
	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()

	cloudInfo, err := s.cloudDetector.DetectCloudEnvironment(ctx)
	if err != nil {
		s.logger.Debug("cloud detection failed", "error", err)
		return
	}

	// Check if context was cancelled while detecting
	select {
	case <-s.ctx.Done():
		s.logger.Debug("cloud detection cancelled after completion")
		return
	default:
	}

	s.mu.Lock()
	s.cloudInfo = cloudInfo
	s.mu.Unlock()

	if cloudInfo != nil {
		s.logger.Info("detected cloud environment", "provider", cloudInfo.Provider)
	} else {
		s.logger.Debug("no cloud environment detected")
	}
}

// collectSystemMetrics runs the primary metrics collection loop in a background goroutine.
// Continuously collects system metrics (CPU, memory, disk, network, I/O) at the
// configured system interval (default 10 seconds).
// Performs initial collection immediately, then runs on ticker schedule.
// Handles context cancellation for graceful shutdown.
// Updates the service's currentMetrics field with fresh data on each collection cycle.
func (s *Service) collectSystemMetrics() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.config.Collection.SystemInterval)
	defer ticker.Stop()

	s.logger.Info("started system metrics collection", "interval", s.config.Collection.SystemInterval)

	// Collect initial metrics immediately
	s.collectAndStoreSystemMetrics()

	for {
		select {
		case <-ticker.C:
			s.collectAndStoreSystemMetrics()
		case <-s.ctx.Done():
			s.logger.Debug("stopping system metrics collection")
			return
		}
	}
}

// collectProcessMetrics runs the process-specific metrics collection loop.
// Currently placeholder for future process-specific collection at different intervals.
// Process metrics are collected as part of system metrics collection for now.
// Could be extended to collect detailed per-process statistics less frequently
// than system metrics to reduce overhead.
// Handles context cancellation for graceful shutdown.
func (s *Service) collectProcessMetrics() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.config.Collection.ProcessInterval)
	defer ticker.Stop()

	s.logger.Info("started process metrics collection", "interval", s.config.Collection.ProcessInterval)

	for {
		select {
		case <-ticker.C:
			// Process metrics are collected as part of system metrics
			// This could be separated if needed for different intervals
		case <-s.ctx.Done():
			s.logger.Debug("stopping process metrics collection")
			return
		}
	}
}

// collectAndStoreSystemMetrics performs a complete system metrics collection cycle.
// Orchestrates collection from all individual metric collectors:
//   - Host information (hostname, uptime, OS details)
//   - CPU metrics (usage, load averages, core count)
//   - Memory metrics (total, used, available, swap)
//   - Disk metrics (usage, I/O statistics for each disk)
//   - Network metrics (interface statistics, traffic counters)
//   - I/O metrics (read/write operations and bytes)
//   - Process metrics (count, top processes by CPU/memory)
//
// Handles collection errors gracefully with fallback empty structs.
// Updates the service's currentMetrics atomically for thread-safe access.
// Respects context cancellation to avoid work during shutdown.
func (s *Service) collectAndStoreSystemMetrics() {
	// Check if we should stop before doing any work
	select {
	case <-s.ctx.Done():
		return
	default:
	}

	timestamp := time.Now()

	// Collect host information
	hostInfo, err := s.hostCollector.Collect()
	if err != nil {
		s.logger.Warn("failed to collect host info", "error", err)
		hostInfo = &domain.HostInfo{} // Use empty struct as fallback
	}

	// Collect CPU metrics
	cpuMetrics, err := s.cpuCollector.Collect()
	if err != nil {
		s.logger.Warn("failed to collect CPU metrics", "error", err)
		cpuMetrics = &domain.CPUMetrics{}
	}

	// Collect memory metrics
	memoryMetrics, err := s.memoryCollector.Collect()
	if err != nil {
		s.logger.Warn("failed to collect memory metrics", "error", err)
		memoryMetrics = &domain.MemoryMetrics{}
	}

	// Collect disk metrics
	diskMetrics, err := s.diskCollector.Collect()
	if err != nil {
		s.logger.Warn("failed to collect disk metrics", "error", err)
		diskMetrics = []domain.DiskMetrics{}
	}

	// Collect network metrics
	networkMetrics, err := s.networkCollector.Collect()
	if err != nil {
		s.logger.Warn("failed to collect network metrics", "error", err)
		networkMetrics = []domain.NetworkMetrics{}
	}

	// Collect I/O metrics
	ioMetrics, err := s.ioCollector.Collect()
	if err != nil {
		s.logger.Warn("failed to collect I/O metrics", "error", err)
		ioMetrics = &domain.IOMetrics{}
	}

	// Collect process metrics (less frequently)
	processMetrics, err := s.processCollector.Collect()
	if err != nil {
		s.logger.Warn("failed to collect process metrics", "error", err)
		processMetrics = &domain.ProcessMetrics{}
	}

	// Create system metrics snapshot
	systemMetrics := &domain.SystemMetrics{
		Timestamp: timestamp,
		Host:      *hostInfo,
		CPU:       *cpuMetrics,
		Memory:    *memoryMetrics,
		Disk:      diskMetrics,
		Network:   networkMetrics,
		IO:        *ioMetrics,
		Processes: *processMetrics,
		Cloud:     s.cloudInfo,
	}

	// Check if we should stop before updating metrics
	select {
	case <-s.ctx.Done():
		return
	default:
	}

	// Update current metrics
	s.mu.Lock()
	s.currentMetrics = systemMetrics
	s.mu.Unlock()

}
