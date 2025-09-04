package core

import (
	"path/filepath"

	"joblet/internal/joblet/adapters"
	"joblet/internal/joblet/core/interfaces"
	"joblet/internal/joblet/core/volume"
	"joblet/internal/joblet/events"
	"joblet/internal/joblet/monitoring"
	"joblet/internal/joblet/runtime"
	"joblet/internal/joblet/workflow"
	"joblet/pkg/config"
	"joblet/pkg/logger"
	"joblet/pkg/platform"
)

// ComponentFactory handles creation and wiring of all system components
// This centralizes dependency injection and reduces coupling
type ComponentFactory struct {
	config   *config.Config
	platform platform.Platform
	logger   *logger.Logger
}

// ServiceComponents contains all the core services needed by the application
type ServiceComponents struct {
	Joblet            interfaces.Joblet
	WorkflowManager   *workflow.WorkflowManager
	VolumeManager     *volume.Manager
	RuntimeResolver   *runtime.Resolver
	MonitoringService *monitoring.Service
	JobStore          JobStore
	NetworkStore      adapters.NetworkStorer
	VolumeStore       VolumeStore
	EventBus          events.EventBus
}

// NewComponentFactory creates a new component factory for dependency injection
func NewComponentFactory(cfg *config.Config, platform platform.Platform) *ComponentFactory {
	factoryLogger := logger.New().WithField("component", "factory")
	return &ComponentFactory{
		config:   cfg,
		platform: platform,
		logger:   factoryLogger,
	}
}

// CreateServices creates and wires all application services with proper dependency injection
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

	eventBus := events.NewInMemoryEventBus()
	joblet := NewJoblet(jobStore, f.config, networkStore)

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
		EventBus:          eventBus,
	}, nil
}

// createJobStore creates and configures the job storage adapter with in-memory backend
func (f *ComponentFactory) createJobStore() (JobStore, error) {
	f.logger.Debug("creating job store adapter")

	// Use simple constructor instead of complex factory pattern
	adapter := adapters.NewJobStore(f.logger)

	f.logger.Info("job store adapter created successfully")
	return adapter, nil
}

// createNetworkStore creates and configures the network storage adapter for job networking
func (f *ComponentFactory) createNetworkStore() (adapters.NetworkStorer, error) {
	f.logger.Debug("creating network store adapter")

	// Use simple constructor instead of complex factory pattern
	adapter := adapters.NewNetworkStore(f.logger)

	f.logger.Info("network store adapter created successfully")
	return adapter, nil
}

// createVolumeStore creates and configures the volume storage adapter for persistent data
func (f *ComponentFactory) createVolumeStore() (VolumeStore, error) {
	f.logger.Debug("creating volume store adapter")

	// Use simple constructor instead of complex factory pattern
	adapter := adapters.NewVolumeStore(f.logger)

	f.logger.Info("volume store adapter created successfully")
	return adapter, nil
}

// createVolumeManager creates a volume manager to handle persistent volume operations
func (f *ComponentFactory) createVolumeManager(volumeStore VolumeStore) *volume.Manager {
	f.logger.Debug("creating volume manager")

	basePath := "/opt/joblet/volumes"
	if f.config.Filesystem.BaseDir != "" {
		basePath = filepath.Join(f.config.Filesystem.BaseDir, "volumes")
	}
	manager := volume.NewManager(volumeStore, f.platform, basePath)

	f.logger.Info("volume manager created successfully")
	return manager
}

// createRuntimeComponents creates a runtime resolver to manage execution environments
func (f *ComponentFactory) createRuntimeComponents() *runtime.Resolver {
	f.logger.Debug("creating runtime resolver", "basePath", f.config.Runtime.BasePath)
	return runtime.NewResolver(f.config.Runtime.BasePath, f.platform)
}

// createWorkflowManager creates a workflow manager to handle multi-job orchestration
func (f *ComponentFactory) createWorkflowManager() *workflow.WorkflowManager {
	f.logger.Debug("creating workflow manager")
	return workflow.NewWorkflowManager()
}

// createMonitoringService creates a monitoring service for system and job metrics collection
func (f *ComponentFactory) createMonitoringService() *monitoring.Service {
	f.logger.Debug("creating monitoring service")
	return monitoring.NewService(nil)
}

// configureVolumeMonitoring wires volume manager with monitoring service for disk usage tracking
func (f *ComponentFactory) configureVolumeMonitoring(monitoringService *monitoring.Service, volumeManager *volume.Manager) {
	f.logger.Debug("configuring volume monitoring integration")

	basePath := "/opt/joblet/volumes"
	if f.config.Filesystem.BaseDir != "" {
		basePath = filepath.Join(f.config.Filesystem.BaseDir, "volumes")
	}

	monitoringService.SetVolumeManager(volumeManager, basePath)
	f.logger.Info("volume monitoring integration configured", "volumeBasePath", basePath)
}
