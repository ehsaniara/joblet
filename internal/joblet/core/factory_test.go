package core

import (
	"testing"
	"time"

	"github.com/ehsaniara/joblet/pkg/config"
	"github.com/ehsaniara/joblet/pkg/platform/platformfakes"
)

func TestNewComponentFactory(t *testing.T) {
	cfg := &config.Config{
		Version: "3.0",
		Monitoring: config.MonitoringConfig{
			Enabled:        true,
			SystemInterval: 10 * time.Second,
			CloudDetection: true,
		},
	}
	fakePlatform := &platformfakes.FakePlatform{}

	factory := NewComponentFactory(cfg, fakePlatform)

	if factory == nil {
		t.Fatal("Expected non-nil factory")
	}

	if factory.config != cfg {
		t.Error("Config not properly stored")
	}

	if factory.platform != fakePlatform {
		t.Error("Platform not properly stored")
	}

	if factory.logger == nil {
		t.Error("Logger should be initialized")
	}
}

func TestGetVolumeBasePath_DefaultPath(t *testing.T) {
	cfg := &config.Config{
		Version: "3.0",
		Filesystem: config.FilesystemConfig{
			BaseDir: "", // Empty - should use default
		},
	}
	fakePlatform := &platformfakes.FakePlatform{}
	factory := NewComponentFactory(cfg, fakePlatform)

	basePath := factory.getVolumeBasePath()

	expected := "/opt/joblet/volumes"
	if basePath != expected {
		t.Errorf("Expected default path %s, got %s", expected, basePath)
	}
}

func TestGetVolumeBasePath_CustomPath(t *testing.T) {
	cfg := &config.Config{
		Version: "3.0",
		Filesystem: config.FilesystemConfig{
			BaseDir: "/custom/base",
		},
	}
	fakePlatform := &platformfakes.FakePlatform{}
	factory := NewComponentFactory(cfg, fakePlatform)

	basePath := factory.getVolumeBasePath()

	expected := "/custom/base/volumes"
	if basePath != expected {
		t.Errorf("Expected custom path %s, got %s", expected, basePath)
	}
}

func TestCreateMonitoringService_UsesConfig(t *testing.T) {
	// Test that monitoring service uses config values
	cfg := &config.Config{
		Version: "3.0",
		Monitoring: config.MonitoringConfig{
			Enabled:        false,
			SystemInterval: 30 * time.Second,
			CloudDetection: false,
		},
		Buffers: config.BuffersConfig{
			PubsubBufferSize: 1000,
			ChunkSize:        1024,
		},
		Volumes: config.VolumesConfig{
			BasePath:              "/tmp/test-volumes",
			DefaultDiskQuotaBytes: 1048576,
		},
		Runtime: config.RuntimeConfig{
			BasePath: "/tmp/test-runtimes",
		},
		IPC: config.IPCConfig{
			Enabled: false,
		},
	}
	fakePlatform := &platformfakes.FakePlatform{}

	factory := NewComponentFactory(cfg, fakePlatform)
	monitoringService := factory.createMonitoringService()

	if monitoringService == nil {
		t.Fatal("Expected non-nil monitoring service")
	}

	// Verify the config was used (check internal config)
	// Since we can't directly access internal config, we check behavior
	// by verifying it respects the Enabled flag
	if err := monitoringService.Start(); err != nil {
		t.Errorf("Start should not error: %v", err)
	}

	// If monitoring is disabled (Enabled: false), it should not be running
	if monitoringService.IsRunning() {
		t.Error("Monitoring service should not be running when Enabled is false in config")
	}

	// Clean up
	_ = monitoringService.Stop()
}

func TestCreateMonitoringService_EnabledConfig(t *testing.T) {
	cfg := &config.Config{
		Version: "3.0",
		Monitoring: config.MonitoringConfig{
			Enabled:        true,
			SystemInterval: 10 * time.Second,
			CloudDetection: false,
		},
		Buffers: config.BuffersConfig{
			PubsubBufferSize: 1000,
			ChunkSize:        1024,
		},
		Volumes: config.VolumesConfig{
			BasePath:              "/tmp/test-volumes",
			DefaultDiskQuotaBytes: 1048576,
		},
		Runtime: config.RuntimeConfig{
			BasePath: "/tmp/test-runtimes",
		},
		IPC: config.IPCConfig{
			Enabled: false,
		},
	}
	fakePlatform := &platformfakes.FakePlatform{}

	factory := NewComponentFactory(cfg, fakePlatform)
	monitoringService := factory.createMonitoringService()

	if monitoringService == nil {
		t.Fatal("Expected non-nil monitoring service")
	}

	// Start the service
	if err := monitoringService.Start(); err != nil {
		t.Fatalf("Failed to start monitoring service: %v", err)
	}
	defer func() { _ = monitoringService.Stop() }()

	// When Enabled is true, the service should be running
	if !monitoringService.IsRunning() {
		t.Error("Monitoring service should be running when Enabled is true in config")
	}
}

func TestCreateMonitoringService_CustomInterval(t *testing.T) {
	customInterval := 25 * time.Second
	cfg := &config.Config{
		Version: "3.0",
		Monitoring: config.MonitoringConfig{
			Enabled:        true,
			SystemInterval: customInterval,
			CloudDetection: false,
		},
		Buffers: config.BuffersConfig{
			PubsubBufferSize: 1000,
			ChunkSize:        1024,
		},
		Volumes: config.VolumesConfig{
			BasePath:              "/tmp/test-volumes",
			DefaultDiskQuotaBytes: 1048576,
		},
		Runtime: config.RuntimeConfig{
			BasePath: "/tmp/test-runtimes",
		},
		IPC: config.IPCConfig{
			Enabled: false,
		},
	}
	fakePlatform := &platformfakes.FakePlatform{}

	factory := NewComponentFactory(cfg, fakePlatform)
	monitoringService := factory.createMonitoringService()

	if monitoringService == nil {
		t.Fatal("Expected non-nil monitoring service")
	}

	// We can't directly verify the interval, but we can ensure the service
	// was created without error and can be started/stopped
	if err := monitoringService.Start(); err != nil {
		t.Fatalf("Failed to start monitoring service: %v", err)
	}

	if !monitoringService.IsRunning() {
		t.Error("Monitoring service should be running")
	}

	if err := monitoringService.Stop(); err != nil {
		t.Fatalf("Failed to stop monitoring service: %v", err)
	}

	if monitoringService.IsRunning() {
		t.Error("Monitoring service should not be running after stop")
	}
}

func TestCreateJobStore(t *testing.T) {
	cfg := &config.Config{
		Version: "3.0",
		Buffers: config.BuffersConfig{
			PubsubBufferSize: 1000,
			ChunkSize:        1024,
		},
		IPC: config.IPCConfig{
			Enabled: false,
		},
	}
	fakePlatform := &platformfakes.FakePlatform{}

	factory := NewComponentFactory(cfg, fakePlatform)
	jobStore, err := factory.createJobStore()

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if jobStore == nil {
		t.Fatal("Expected non-nil job store")
	}
}

func TestCreateNetworkStore(t *testing.T) {
	cfg := &config.Config{
		Version: "3.0",
	}
	fakePlatform := &platformfakes.FakePlatform{}

	factory := NewComponentFactory(cfg, fakePlatform)
	networkStore, err := factory.createNetworkStore()

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if networkStore == nil {
		t.Fatal("Expected non-nil network store")
	}
}

func TestCreateVolumeStore(t *testing.T) {
	cfg := &config.Config{
		Version: "3.0",
	}
	fakePlatform := &platformfakes.FakePlatform{}

	factory := NewComponentFactory(cfg, fakePlatform)
	volumeStore, err := factory.createVolumeStore()

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if volumeStore == nil {
		t.Fatal("Expected non-nil volume store")
	}
}

func TestCreateWorkflowManager(t *testing.T) {
	cfg := &config.Config{
		Version: "3.0",
	}
	fakePlatform := &platformfakes.FakePlatform{}

	factory := NewComponentFactory(cfg, fakePlatform)
	workflowManager := factory.createWorkflowManager()

	if workflowManager == nil {
		t.Fatal("Expected non-nil workflow manager")
	}
}

func TestCreateRuntimeComponents(t *testing.T) {
	cfg := &config.Config{
		Version: "3.0",
		Runtime: config.RuntimeConfig{
			BasePath: "/tmp/test-runtimes",
		},
	}
	fakePlatform := &platformfakes.FakePlatform{}

	factory := NewComponentFactory(cfg, fakePlatform)
	runtimeResolver := factory.createRuntimeComponents()

	if runtimeResolver == nil {
		t.Fatal("Expected non-nil runtime resolver")
	}
}

// TestCreateServices_Integration tests the full service creation workflow
func TestCreateServices_Integration(t *testing.T) {
	cfg := &config.Config{
		Version: "3.0",
		Monitoring: config.MonitoringConfig{
			Enabled:        true,
			SystemInterval: 10 * time.Second,
			CloudDetection: false,
		},
		Buffers: config.BuffersConfig{
			PubsubBufferSize: 1000,
			ChunkSize:        1024,
		},
		Volumes: config.VolumesConfig{
			BasePath:              "/tmp/test-volumes",
			DefaultDiskQuotaBytes: 1048576,
		},
		Runtime: config.RuntimeConfig{
			BasePath: "/tmp/test-runtimes",
		},
		Filesystem: config.FilesystemConfig{
			BaseDir: "/tmp/test-jobs",
		},
		Cgroup: config.CgroupConfig{
			BaseDir:           "/tmp/test-cgroup", // Prevent creating cgroup dirs in source tree
			NamespaceMount:    "/sys/fs/cgroup",
			EnableControllers: []string{},
			CleanupTimeout:    5 * time.Second,
		},
		IPC: config.IPCConfig{
			Enabled: false,
		},
		Network: config.NetworkConfig{
			Enabled: false,
		},
	}
	fakePlatform := &platformfakes.FakePlatform{}

	factory := NewComponentFactory(cfg, fakePlatform)
	components, err := factory.CreateServices()

	if err != nil {
		t.Fatalf("Expected no error creating services, got: %v", err)
	}

	if components == nil {
		t.Fatal("Expected non-nil service components")
	}

	// Verify all components are initialized
	if components.Joblet == nil {
		t.Error("Expected Joblet to be initialized")
	}

	if components.WorkflowManager == nil {
		t.Error("Expected WorkflowManager to be initialized")
	}

	if components.VolumeManager == nil {
		t.Error("Expected VolumeManager to be initialized")
	}

	if components.RuntimeResolver == nil {
		t.Error("Expected RuntimeResolver to be initialized")
	}

	if components.MonitoringService == nil {
		t.Error("Expected MonitoringService to be initialized")
	}

	if components.JobStore == nil {
		t.Error("Expected JobStore to be initialized")
	}

	if components.NetworkStore == nil {
		t.Error("Expected NetworkStore to be initialized")
	}

	if components.VolumeStore == nil {
		t.Error("Expected VolumeStore to be initialized")
	}

	if components.MetricsStore == nil {
		t.Error("Expected MetricsStore to be initialized")
	}

	if components.EventBus == nil {
		t.Error("Expected EventBus to be initialized")
	}

	// Test that monitoring service was configured with the right settings
	if err := components.MonitoringService.Start(); err != nil {
		t.Errorf("Failed to start monitoring service: %v", err)
	}
	defer func() { _ = components.MonitoringService.Stop() }()

	if !components.MonitoringService.IsRunning() {
		t.Error("Monitoring service should be running with enabled config")
	}
}
