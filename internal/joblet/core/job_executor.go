//go:build linux

package core

import (
	"context"
	"fmt"

	"github.com/ehsaniara/joblet/internal/joblet/adapters"
	"github.com/ehsaniara/joblet/internal/joblet/core/environment"
	"github.com/ehsaniara/joblet/internal/joblet/core/execution"
	"github.com/ehsaniara/joblet/internal/joblet/core/process"
	"github.com/ehsaniara/joblet/internal/joblet/core/unprivileged"
	"github.com/ehsaniara/joblet/internal/joblet/core/upload"
	"github.com/ehsaniara/joblet/internal/joblet/domain"
	"github.com/ehsaniara/joblet/internal/joblet/gpu"
	"github.com/ehsaniara/joblet/internal/joblet/network"
	"github.com/ehsaniara/joblet/pkg/config"
	"github.com/ehsaniara/joblet/pkg/logger"
	"github.com/ehsaniara/joblet/pkg/platform"
)

// ExecutionEngineV2 is the main job execution engine using the coordinator pattern
// for managing job lifecycle, isolation, networking, and process execution
type ExecutionEngineV2 struct {
	coordinator execution.JobExecutor
	platform    platform.Platform
	config      *config.Config
	store       adapters.JobStorer
	logger      *logger.Logger
}

// StartProcessOptions contains options for starting a process
type StartProcessOptions struct {
	Job               *domain.Job
	Uploads           []domain.FileUpload
	EnableStreaming   bool
	WorkspaceDir      string
	PreProcessUploads bool // For scheduled jobs that need uploads processed beforehand
}

// NewExecutionEngineV2 creates a new job execution engine with coordinated dependency management
func NewExecutionEngineV2(
	processManager *process.Manager,
	uploadManager *upload.Manager,
	platform platform.Platform,
	store adapters.JobStorer,
	config *config.Config,
	logger *logger.Logger,
	jobIsolation *unprivileged.JobIsolation,
	networkStore adapters.NetworkStorer,
	gpuManager gpu.GPUManagerInterface,
) *ExecutionEngineV2 {
	// Create environment builder
	envBuilder := environment.NewBuilder(platform, uploadManager, config, logger)

	// Create environment service (runtime functionality is handled by the filesystem isolator)
	envService := execution.NewEnvironmentService(
		envBuilder,
		uploadManager,
		platform,
		config,
		logger,
	)

	// Create network service
	var netService execution.NetworkManager
	if networkStore != nil {
		// NetworkStore already implements network.NetworkStoreInterface via adapter
		networkSetup := network.NewNetworkSetup(platform, networkStore)

		// Create network store adapter for the execution service
		networkStoreAdapter := &networkStoreAdapter{store: networkStore}
		netService = execution.NewNetworkService(networkSetup, networkStoreAdapter, logger)
	}

	// Create process service adapter
	processService := &processManagerAdapter{
		manager:   processManager,
		platform:  platform,
		store:     store,
		logger:    logger,
		isolation: jobIsolation,
	}

	// Create isolation service adapter
	isolationService := &isolationManagerAdapter{
		isolation: jobIsolation,
		config:    config,
		platform:  platform,
		logger:    logger,
	}

	// Create GPU service with CUDA verification
	cudaVerifier := gpu.NewCUDAVerifier()
	gpuService := execution.NewGPUService(gpuManager, cudaVerifier, logger)

	// Create execution coordinator
	coordinator := execution.NewExecutionCoordinator(
		envService,
		netService,
		processService,
		isolationService,
		gpuService,
		platform,
		logger,
	)

	return &ExecutionEngineV2{
		coordinator: coordinator,
		platform:    platform,
		config:      config,
		store:       store,
		logger:      logger.WithField("component", "execution-engine-v2"),
	}
}

// StartProcess initiates job execution with proper isolation and coordination
func (ee *ExecutionEngineV2) StartProcess(ctx context.Context, opts *StartProcessOptions) (platform.Command, error) {
	log := ee.logger.WithField("job_uuid", opts.Job.Uuid)
	log.Debug("starting job process", "hasUploads", len(opts.Uploads) > 0)

	// Use coordinator for full isolation
	execOpts := &execution.StartProcessOptions{
		Job:               opts.Job,
		Uploads:           opts.Uploads,
		EnableStreaming:   opts.EnableStreaming,
		WorkspaceDir:      opts.WorkspaceDir,
		PreProcessUploads: opts.PreProcessUploads,
	}

	log.Debug("delegating to coordinator")
	return ee.coordinator.StartJob(ctx, execOpts)
}

// StartProcessWithUploads executes a job with file uploads and streaming enabled
func (ee *ExecutionEngineV2) StartProcessWithUploads(ctx context.Context, job *domain.Job, uploads []domain.FileUpload) (platform.Command, error) {
	opts := &StartProcessOptions{
		Job:             job,
		Uploads:         uploads,
		EnableStreaming: true,
	}
	return ee.StartProcess(ctx, opts)
}

// Adapter implementations to bridge between the new interfaces and existing implementations

// networkStoreAdapter adapts NetworkStore to execution.NetworkStoreInterface
type networkStoreAdapter struct {
	store adapters.NetworkStorer
}

func (nsa *networkStoreAdapter) AllocateIP(networkName string) (string, error) {
	return nsa.store.AllocateIP(networkName)
}

func (nsa *networkStoreAdapter) ReleaseIP(networkName, ipAddress string) error {
	return nsa.store.ReleaseIP(networkName, ipAddress)
}

func (nsa *networkStoreAdapter) AssignJobToNetwork(jobID, networkName string, allocation *execution.JobNetworkAllocation) error {
	// Convert execution.JobNetworkAllocation to adapters.JobNetworkAllocation
	adapterAlloc := &adapters.JobNetworkAllocation{
		JobUUID:     allocation.JobUUID,
		NetworkName: allocation.NetworkName,
		IPAddress:   allocation.IPAddress,
		Hostname:    allocation.Hostname,
		AssignedAt:  allocation.AssignedAt,
	}
	return nsa.store.AssignJobToNetwork(jobID, networkName, adapterAlloc)
}

func (nsa *networkStoreAdapter) RemoveJobFromNetwork(jobID string) error {
	return nsa.store.RemoveJobFromNetwork(jobID)
}

func (nsa *networkStoreAdapter) GetJobAllocation(jobID string) (*execution.JobNetworkAllocation, error) {
	// Get the job allocation from the store
	alloc, found := nsa.store.JobNetworkAllocation(jobID)
	if !found {
		return nil, fmt.Errorf("job network allocation not found: %s", jobID)
	}

	// Convert to execution package format
	return &execution.JobNetworkAllocation{
		JobUUID:     alloc.JobUUID,
		NetworkName: alloc.NetworkName,
		IPAddress:   alloc.IPAddress,
		Hostname:    alloc.Hostname,
		AssignedAt:  alloc.AssignedAt,
	}, nil
}

// processManagerAdapter adapts process.Manager to execution.ProcessManager
type processManagerAdapter struct {
	manager   *process.Manager
	platform  platform.Platform
	store     adapters.JobStorer
	logger    *logger.Logger
	isolation *unprivileged.JobIsolation
}

func (pma *processManagerAdapter) LaunchProcess(ctx context.Context, config *execution.LaunchConfig) (*execution.ProcessResult, error) {
	// Convert to process.LaunchConfig
	outputWriter := NewWrite(pma.store, config.JobUUID)

	// Use the job isolation's proper namespace isolation setup based on job type
	// Runtime build jobs disable network isolation for internet access
	// Production jobs get full isolation including network namespace
	pma.logger.Info("ABOUT TO CREATE ISOLATION WITH JOB TYPE", "jobType", config.JobType)
	sysProcAttr := pma.isolation.CreateIsolatedSysProcAttrForJobType(config.JobType)

	// Debug: Log namespace isolation configuration
	pma.logger.Info("configuring namespace isolation for job",
		"job_uuid", config.JobUUID,
		"cloneflags", fmt.Sprintf("0x%x", sysProcAttr.Cloneflags),
		"component", "process-manager-adapter")

	procConfig := &process.LaunchConfig{
		InitPath:    config.InitPath,
		Environment: config.Environment,
		Stdout:      outputWriter,
		Stderr:      outputWriter,
		JobUUID:     config.JobUUID,
		JobType:     config.JobType, // Pass job type for logging and validation
		Command:     config.Command,
		Args:        config.Args,
		SysProcAttr: sysProcAttr, // Isolation configured based on job type
	}

	result, err := pma.manager.LaunchProcess(ctx, procConfig)
	if err != nil {
		pma.logger.Error("failed to launch process with namespace isolation",
			"job_uuid", config.JobUUID,
			"error", err,
			"component", "process-manager-adapter")
		return nil, err
	}

	pma.logger.Info("process launched successfully with namespace isolation",
		"job_uuid", config.JobUUID,
		"pid", result.PID,
		"component", "process-manager-adapter")

	return &execution.ProcessResult{
		Command: result.Command,
		PID:     int(result.PID),
	}, nil
}

func (pma *processManagerAdapter) KillProcess(pid int) error {
	// Implementation would depend on how process killing is handled
	return nil
}

// isolationManagerAdapter adapts unprivileged.JobIsolation to execution.IsolationManager
type isolationManagerAdapter struct {
	isolation *unprivileged.JobIsolation
	config    *config.Config
	platform  platform.Platform
	logger    *logger.Logger
}

func (ima *isolationManagerAdapter) CreateIsolatedEnvironment(jobID string) (*execution.IsolationContext, error) {
	// Create job directory and other isolation setup
	// This is a simplified implementation
	return &execution.IsolationContext{
		JobUUID:      jobID,
		Namespace:    "job-" + jobID,
		CgroupPath:   "/sys/fs/cgroup/joblet/" + jobID,
		WorkspaceDir: ima.config.Filesystem.BaseDir + "/" + jobID,
	}, nil
}

func (ima *isolationManagerAdapter) CreateBuilderEnvironment(jobID string) (*execution.IsolationContext, error) {
	// Create builder environment for runtime builds
	// Similar to regular isolated environment but with builder flag
	ima.logger.Debug("creating builder environment", "job_uuid", jobID)

	return &execution.IsolationContext{
		JobUUID:      jobID,
		Namespace:    "builder-" + jobID,
		CgroupPath:   "/sys/fs/cgroup/joblet/" + jobID,
		WorkspaceDir: ima.config.Filesystem.BaseDir + "/" + jobID,
		IsBuilder:    true, // Mark as builder environment
	}, nil
}

func (ima *isolationManagerAdapter) DestroyIsolatedEnvironment(jobID string) error {
	// Cleanup isolation environment
	return nil
}
