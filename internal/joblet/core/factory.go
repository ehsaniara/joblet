package core

import (
	"path/filepath"

	"github.com/ehsaniara/joblet/internal/joblet/adapters"
	"github.com/ehsaniara/joblet/internal/joblet/core/interfaces"
	"github.com/ehsaniara/joblet/internal/joblet/core/volume"
	"github.com/ehsaniara/joblet/internal/joblet/events"
	"github.com/ehsaniara/joblet/internal/joblet/monitoring"
	"github.com/ehsaniara/joblet/internal/joblet/pubsub"
	"github.com/ehsaniara/joblet/internal/joblet/runtime"
	"github.com/ehsaniara/joblet/internal/joblet/workflow"
	"github.com/ehsaniara/joblet/pkg/client"
	"github.com/ehsaniara/joblet/pkg/config"
	"github.com/ehsaniara/joblet/pkg/logger"
	"github.com/ehsaniara/joblet/pkg/platform"
)

// ComponentFactory is like a smart assembler that knows how to build and connect
// all the different parts of our system. It keeps everything organized so components
// don't need to know about each other directly.
type ComponentFactory struct {
	config   *config.Config
	platform platform.Platform
	logger   *logger.Logger
}

// ServiceComponents is our collection of essential services that make the app work.
// Think of it as gathering all the key players in one place.
type ServiceComponents struct {
	Joblet            interfaces.Joblet
	WorkflowManager   *workflow.WorkflowManager
	VolumeManager     *volume.Manager
	RuntimeResolver   *runtime.Resolver
	MonitoringService *monitoring.Service
	JobStore          adapters.JobStorer
	NetworkStore      adapters.NetworkStorer
	VolumeStore       adapters.VolumeStorer
	MetricsStore      *adapters.MetricsStoreAdapter
	EventBus          events.EventBus
}

// NewComponentFactory sets up a new factory that can build all our system components.
// Just give it some config and platform info, and it'll handle the rest!
func NewComponentFactory(cfg *config.Config, platform platform.Platform) *ComponentFactory {
	factoryLogger := logger.New().WithField("component", "factory")
	return &ComponentFactory{
		config:   cfg,
		platform: platform,
		logger:   factoryLogger,
	}
}

// CreateServices builds all the services we need and wires them together nicely.
// It's like assembling a complex machine where each part knows how to talk to the others.
func (f *ComponentFactory) CreateServices() (*ServiceComponents, error) {
	f.logger.Debug("initializing application services")

	jobStore, err := f.createJobStore()
	if err != nil {
		return nil, err
	}

	networkStore, err := f.createNetworkStore()
	if err != nil {
		return nil, err
	}

	volumeStore, err := f.createVolumeStore()
	if err != nil {
		return nil, err
	}

	volumeManager := f.createVolumeManager(volumeStore)
	runtimeResolver := f.createRuntimeComponents()
	workflowManager := f.createWorkflowManager()
	monitoringService := f.createMonitoringService()

	f.configureVolumeMonitoring(monitoringService, volumeManager)

	metricsStore := f.createMetricsStore()

	eventBus := events.NewInMemoryEventBus()
	joblet := NewJoblet(jobStore, metricsStore, f.config, networkStore)

	f.logger.Info("all application services initialized successfully")

	return &ServiceComponents{
		Joblet:            joblet,
		WorkflowManager:   workflowManager,
		VolumeManager:     volumeManager,
		RuntimeResolver:   runtimeResolver,
		MonitoringService: monitoringService,
		JobStore:          jobStore,
		NetworkStore:      networkStore,
		VolumeStore:       volumeStore,
		MetricsStore:      metricsStore,
		EventBus:          eventBus,
	}, nil
}

// createJobStore sets up our job storage - this is where we keep track of all the jobs
// running in the system. Think of it as our job memory bank.
func (f *ComponentFactory) createJobStore() (adapters.JobStorer, error) {
	f.logger.Debug("creating job store adapter")

	// Create job store with buffer configuration for pub-sub and log persistence
	adapter := adapters.NewJobStore(f.config, f.config.IPC.Enabled, f.logger)

	f.logger.Info("job store adapter created successfully")
	return adapter, nil
}

// createNetworkStore builds our network storage - this manages all the networking
// configurations for jobs so they can talk to each other and the outside world.
func (f *ComponentFactory) createNetworkStore() (adapters.NetworkStorer, error) {
	f.logger.Debug("creating network store adapter")

	// Keep it simple - just create what we need without any fancy patterns
	adapter := adapters.NewNetworkStore(f.logger)

	f.logger.Info("network store adapter created successfully")
	return adapter, nil
}

// createVolumeStore sets up storage for persistent volumes - this is where we keep
// data that needs to stick around even after jobs finish.
func (f *ComponentFactory) createVolumeStore() (adapters.VolumeStorer, error) {
	f.logger.Debug("creating volume store adapter")

	// Simple and direct - just what we need
	adapter := adapters.NewVolumeStore(f.logger)

	f.logger.Info("volume store adapter created successfully")
	return adapter, nil
}

// getVolumeBasePath returns the base path for volume storage.
// Uses configured filesystem base directory if available, otherwise defaults to /opt/joblet/volumes.
func (f *ComponentFactory) getVolumeBasePath() string {
	if f.config.Filesystem.BaseDir != "" {
		return filepath.Join(f.config.Filesystem.BaseDir, "volumes")
	}
	return "/opt/joblet/volumes"
}

// createVolumeManager builds the volume manager that actually deals with creating,
// mounting, and cleaning up volumes on the filesystem.
func (f *ComponentFactory) createVolumeManager(volumeStore adapters.VolumeStorer) *volume.Manager {
	f.logger.Debug("creating volume manager")

	basePath := f.getVolumeBasePath()
	manager := volume.NewManager(volumeStore, f.platform, basePath)

	f.logger.Info("volume manager created successfully")
	return manager
}

// createRuntimeComponents sets up the runtime resolver - this guy figures out
// which runtime environment (like Python, Node.js, etc.) to use for each job.
func (f *ComponentFactory) createRuntimeComponents() *runtime.Resolver {
	f.logger.Debug("creating runtime resolver", "basePath", f.config.Runtime.BasePath)
	return runtime.NewResolver(f.config.Runtime.BasePath, f.platform)
}

// createWorkflowManager builds the workflow manager that coordinates multiple jobs
// working together, making sure they run in the right order and dependencies are handled.
func (f *ComponentFactory) createWorkflowManager() *workflow.WorkflowManager {
	f.logger.Debug("creating workflow manager")
	return workflow.NewWorkflowManager()
}

// createMonitoringService sets up our monitoring service that keeps an eye on
// system health and job performance - basically our system's health checker.
func (f *ComponentFactory) createMonitoringService() *monitoring.Service {
	f.logger.Debug("creating monitoring service")
	return monitoring.NewServiceFromConfig(&f.config.Monitoring)
}

// configureVolumeMonitoring connects our volume manager to the monitoring service
// so we can keep track of how much disk space our volumes are using.
func (f *ComponentFactory) configureVolumeMonitoring(monitoringService *monitoring.Service, volumeManager *volume.Manager) {
	f.logger.Debug("configuring volume monitoring integration")

	basePath := f.getVolumeBasePath()

	monitoringService.SetVolumeManager(volumeManager, basePath)
	f.logger.Info("volume monitoring integration configured", "volumeBasePath", basePath)
}

// createMetricsStore sets up our job metrics collection store that tracks
// resource usage (CPU, memory, I/O, network, GPU) for each job.
func (f *ComponentFactory) createMetricsStore() *adapters.MetricsStoreAdapter {
	f.logger.Debug("creating metrics store adapter")

	// Create persist client for historical metrics deletion
	persistSocketPath := "/opt/joblet/run/persist-grpc.sock"
	persistClient, err := client.NewPersistClientUnix(persistSocketPath)
	if err != nil {
		f.logger.Warn("failed to connect to persist service - historical metrics deletion will not work",
			"socket", persistSocketPath, "error", err)
		persistClient = nil // Continue without persist client
	} else {
		f.logger.Info("connected to persist service for historical metrics deletion", "socket", persistSocketPath)
	}

	// Create a pub-sub for metrics events (live streaming + IPC forwarding)
	metricsPubSub := pubsub.NewPubSub[adapters.MetricsEvent]()

	metricsStore := adapters.NewMetricsStoreAdapter(
		metricsPubSub,
		persistClient,
		f.config.IPC.Enabled,
		logger.WithField("component", "metrics-store"),
	)

	f.logger.Info("metrics store adapter created successfully")
	return metricsStore
}
