//go:build linux

package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"joblet/internal/joblet/adapters"
	"joblet/internal/joblet/core/environment"
	"joblet/internal/joblet/core/execution"
	"joblet/internal/joblet/core/process"
	"joblet/internal/joblet/core/unprivileged"
	"joblet/internal/joblet/core/upload"
	"joblet/internal/joblet/domain"
	"joblet/internal/joblet/network"
	"joblet/pkg/config"
	"joblet/pkg/logger"
	"joblet/pkg/platform"
)

// ExecutionEngineV2 is the main job execution engine using the coordinator pattern
// for managing job lifecycle, isolation, networking, and process execution
type ExecutionEngineV2 struct {
	coordinator execution.JobExecutor
	platform    platform.Platform
	config      *config.Config
	store       JobStore
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
	store JobStore,
	config *config.Config,
	logger *logger.Logger,
	jobIsolation *unprivileged.JobIsolation,
	networkStore NetworkStore,
) *ExecutionEngineV2 {
	// Create environment builder
	envBuilder := environment.NewBuilder(platform, uploadManager, logger)

	// Create environment service (runtime functionality now handled by filesystem isolator)
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

	// Create execution coordinator
	coordinator := execution.NewExecutionCoordinator(
		envService,
		netService,
		processService,
		isolationService,
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
	log := ee.logger.WithField("jobID", opts.Job.Uuid)
	log.Debug("starting job process", "hasUploads", len(opts.Uploads) > 0)

	// Check if we're in CI mode - if so, use lightweight isolation
	if ee.platform.Getenv("JOBLET_CI_MODE") == "true" {
		return ee.executeCICommand(ctx, opts)
	}

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

// executeCICommand executes a job in CI mode with minimal isolation
func (ee *ExecutionEngineV2) executeCICommand(ctx context.Context, opts *StartProcessOptions) (platform.Command, error) {
	log := ee.logger.WithField("jobID", opts.Job.Uuid).WithField("mode", "ci-isolated")

	// Create job directory for workspace
	jobDir := filepath.Join(ee.config.Filesystem.BaseDir, opts.Job.Uuid)
	workDir := filepath.Join(jobDir, "work")
	if err := ee.platform.MkdirAll(workDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create work directory: %w", err)
	}

	// Process uploads if any
	if len(opts.Uploads) > 0 {
		for _, upload := range opts.Uploads {
			fullPath := filepath.Join(workDir, upload.Path)
			if upload.IsDirectory {
				if err := os.MkdirAll(fullPath, os.FileMode(upload.Mode)); err != nil {
					return nil, fmt.Errorf("failed to create directory %s: %w", upload.Path, err)
				}
			} else {
				if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
					return nil, fmt.Errorf("failed to create parent directory for %s: %w", upload.Path, err)
				}
				if err := os.WriteFile(fullPath, upload.Content, os.FileMode(upload.Mode)); err != nil {
					return nil, fmt.Errorf("failed to write file %s: %w", upload.Path, err)
				}
			}
		}
		log.Debug("processed uploads for CI mode", "count", len(opts.Uploads))
	}

	// Build environment
	environment := ee.buildEnvironmentForCI(opts.Job)

	outputWriter := NewWrite(ee.store, opts.Job.Uuid)

	// Create command directly (no isolation)
	cmd := ee.platform.CreateCommand(opts.Job.Command, opts.Job.Args...)
	cmd.SetEnv(environment)
	cmd.SetDir(workDir)
	cmd.SetStdout(outputWriter)
	cmd.SetStderr(outputWriter)

	log.Info("starting CI command", "command", opts.Job.Command, "args", opts.Job.Args)

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start CI command: %w", err)
	}

	log.Info("CI command started successfully")
	return cmd, nil
}

// buildEnvironmentForCI creates a simplified environment for CI execution
func (ee *ExecutionEngineV2) buildEnvironmentForCI(job *domain.Job) []string {
	// Get base environment from platform
	baseEnv := ee.platform.Environ()

	// Create job-specific environment
	jobEnv := make([]string, 0, len(baseEnv)+len(job.Environment)+len(job.SecretEnvironment))

	// Add base environment
	jobEnv = append(jobEnv, baseEnv...)

	// Add job environment variables
	for key, value := range job.Environment {
		jobEnv = append(jobEnv, fmt.Sprintf("%s=%s", key, value))
	}

	// Add secret environment variables
	for key, value := range job.SecretEnvironment {
		jobEnv = append(jobEnv, fmt.Sprintf("%s=%s", key, value))
	}

	return jobEnv
}

// Adapter implementations to bridge between the new interfaces and existing implementations

// networkStoreAdapter adapts NetworkStore to execution.NetworkStoreInterface
type networkStoreAdapter struct {
	store NetworkStore
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
		JobID:       allocation.JobID,
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
		JobID:       alloc.JobID,
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
	store     JobStore
	logger    *logger.Logger
	isolation *unprivileged.JobIsolation
}

func (pma *processManagerAdapter) LaunchProcess(ctx context.Context, config *execution.LaunchConfig) (*execution.ProcessResult, error) {
	// Convert to process.LaunchConfig
	outputWriter := NewWrite(pma.store, config.JobID)

	// Use the job isolation's proper namespace isolation setup based on job type
	// Runtime build jobs disable network isolation for internet access
	// Production jobs get full isolation including network namespace
	pma.logger.Info("ABOUT TO CREATE ISOLATION WITH JOB TYPE", "jobType", config.JobType)
	sysProcAttr := pma.isolation.CreateIsolatedSysProcAttrForJobType(config.JobType)

	// Debug: Log namespace isolation configuration
	pma.logger.Info("configuring namespace isolation for job",
		"jobID", config.JobID,
		"cloneflags", fmt.Sprintf("0x%x", sysProcAttr.Cloneflags),
		"component", "process-manager-adapter")

	procConfig := &process.LaunchConfig{
		InitPath:    config.InitPath,
		Environment: config.Environment,
		Stdout:      outputWriter,
		Stderr:      outputWriter,
		JobID:       config.JobID,
		JobType:     config.JobType, // Pass job type for logging and validation
		Command:     config.Command,
		Args:        config.Args,
		SysProcAttr: sysProcAttr, // Isolation configured based on job type
	}

	result, err := pma.manager.LaunchProcess(ctx, procConfig)
	if err != nil {
		pma.logger.Error("failed to launch process with namespace isolation",
			"jobID", config.JobID,
			"error", err,
			"component", "process-manager-adapter")
		return nil, err
	}

	pma.logger.Info("process launched successfully with namespace isolation",
		"jobID", config.JobID,
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
		JobID:        jobID,
		Namespace:    "job-" + jobID,
		CgroupPath:   "/sys/fs/cgroup/joblet/" + jobID,
		WorkspaceDir: ima.config.Filesystem.BaseDir + "/" + jobID,
	}, nil
}

func (ima *isolationManagerAdapter) CreateBuilderEnvironment(jobID string) (*execution.IsolationContext, error) {
	// Create builder environment for runtime builds
	// Similar to regular isolated environment but with builder flag
	ima.logger.Debug("creating builder environment", "jobID", jobID)

	return &execution.IsolationContext{
		JobID:        jobID,
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
