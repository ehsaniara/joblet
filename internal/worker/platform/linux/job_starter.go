//go:build linux

package linux

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"job-worker/internal/config"
	"job-worker/internal/worker/domain"
	"job-worker/internal/worker/executor"
	"job-worker/internal/worker/interfaces"
	"job-worker/internal/worker/platform/linux/network"
	"job-worker/internal/worker/platform/linux/process"
	"job-worker/pkg/logger"
	osinterface "job-worker/pkg/os"
)

// JobStarter handles job starting operations
type JobStarter struct {
	worker           *Worker
	store            interfaces.Store
	networkManager   *network.Manager
	processLauncher  *process.Launcher
	processValidator *process.Validator
	cgroup           interfaces.Resource
	osInterface      osinterface.OsInterface
	config           *Config
	logger           *logger.Logger
}

// StartJobRequest contains all parameters needed to start a job
type StartJobRequest struct {
	Command        string
	Args           []string
	MaxCPU         int32
	MaxMemory      int32
	MaxIOBPS       int32
	NetworkGroupID string
}

// JobStartResult contains the result of starting a job
type JobStartResult struct {
	Job              *domain.Job
	NetworkGroupInfo *network.GroupInfo
	ProcessPID       int32
	InitPath         string
	ResolvedCommand  string
}

// NewJobStarter creates a new job starter
func NewJobStarter(worker *Worker, deps *Dependencies) *JobStarter {
	return &JobStarter{
		worker:           worker,
		store:            deps.Store,
		networkManager:   deps.NetworkManager,
		processLauncher:  deps.ProcessLauncher,
		processValidator: deps.ProcessValidator,
		cgroup:           deps.Cgroup,
		osInterface:      deps.OsInterface,
		config:           deps.Config,
		logger:           logger.New().WithField("component", "job-starter"),
	}
}

// StartJob starts a new job with the given parameters
func (js *JobStarter) StartJob(ctx context.Context, req *StartJobRequest) (*domain.Job, error) {
	// Check context cancellation
	select {
	case <-ctx.Done():
		js.logger.Warn("start job cancelled by context", "error", ctx.Err())
		return nil, ctx.Err()
	default:
	}

	// Generate job ID
	jobID := js.worker.GetNextJobID()
	jobLogger := js.logger.WithField("jobId", jobID)

	jobLogger.Info("starting new job",
		"command", req.Command,
		"args", req.Args,
		"requestedCPU", req.MaxCPU,
		"requestedMemory", req.MaxMemory,
		"requestedIOBPS", req.MaxIOBPS,
		"networkGroup", req.NetworkGroupID)

	// Validate request
	if err := js.validateStartRequest(req); err != nil {
		jobLogger.Error("invalid start request", "error", err)
		return nil, fmt.Errorf("invalid start request: %w", err)
	}

	// Apply resource defaults
	js.applyResourceDefaults(req)

	// Validate and resolve command
	resolvedCommand, err := js.worker.ValidateAndResolveCommand(req.Command)
	if err != nil {
		jobLogger.Error("command validation/resolution failed", "error", err)
		return nil, err
	}

	// Create job domain object
	job := js.worker.CreateJobDomain(req, jobID, resolvedCommand)

	// Setup resources with cleanup on failure
	networkGroupInfo, err := js.setupJobResources(ctx, job, req)
	if err != nil {
		jobLogger.Error("failed to setup job resources", "error", err)
		js.worker.CleanupJobResources(jobID, req.NetworkGroupID, true)
		return nil, err
	}

	// Store the job
	js.store.CreateNewJob(job)
	jobLogger.Debug("job created in store", "status", string(job.Status))

	// Start the job process
	processResult, err := js.startJobProcess(ctx, job, networkGroupInfo)
	if err != nil {
		jobLogger.Error("failed to start job process", "error", err)
		js.worker.CleanupJobResources(jobID, req.NetworkGroupID, networkGroupInfo.IsNewGroup)

		// Mark job as failed
		failedJob := job.DeepCopy()
		if failErr := failedJob.Fail(-1); failErr != nil {
			jobLogger.Warn("domain validation failed for job failure", "error", failErr)
			failedJob.Status = domain.StatusFailed
			failedJob.ExitCode = -1
			now := time.Now()
			failedJob.EndTime = &now
		}
		js.store.UpdateJob(failedJob)

		return nil, err
	}

	// Update job with PID and mark as running
	runningJob := job.DeepCopy()
	runningJob.Pid = processResult.PID
	if err := runningJob.MarkAsRunning(processResult.PID); err != nil {
		jobLogger.Warn("domain validation failed for running status", "error", err)
		runningJob.Status = domain.StatusRunning
		runningJob.Pid = processResult.PID
	}
	runningJob.StartTime = time.Now()
	js.store.UpdateJob(runningJob)

	// Handle namespace creation after process start
	if err := js.handleNamespaceCreation(processResult.PID, networkGroupInfo, jobID); err != nil {
		jobLogger.Error("failed to handle namespace creation", "error", err)
		js.worker.EmergencyCleanup(jobID, processResult.PID, req.NetworkGroupID)
		return nil, fmt.Errorf("failed to handle namespace creation: %w", err)
	}

	// Update network group job count
	if req.NetworkGroupID != "" {
		if err := js.networkManager.IncrementJobCount(req.NetworkGroupID); err != nil {
			jobLogger.Warn("failed to increment network group job count", "error", err)
		}
	}

	// Start monitoring the job
	js.worker.StartMonitoring(ctx, processResult.Command, runningJob)

	jobLogger.Info("job started successfully", "pid", runningJob.Pid, "status", string(runningJob.Status))
	return runningJob, nil
}

// validateStartRequest validates the start job request
func (js *JobStarter) validateStartRequest(req *StartJobRequest) error {
	if req == nil {
		return fmt.Errorf("start request cannot be nil")
	}

	// Validate command
	if err := js.processValidator.ValidateCommand(req.Command); err != nil {
		return fmt.Errorf("command validation failed: %w", err)
	}

	// Validate arguments
	if err := js.processValidator.ValidateArguments(req.Args); err != nil {
		return fmt.Errorf("arguments validation failed: %w", err)
	}

	// Validate resource limits
	if err := js.processValidator.ValidateResourceLimits(req.MaxCPU, req.MaxMemory, req.MaxIOBPS); err != nil {
		return fmt.Errorf("resource limits validation failed: %w", err)
	}

	// Validate network group ID if provided
	if req.NetworkGroupID != "" {
		if err := js.processValidator.ValidateNetworkGroupID(req.NetworkGroupID); err != nil {
			return fmt.Errorf("network group ID validation failed: %w", err)
		}
	}

	return nil
}

// applyResourceDefaults applies default values for resource limits
func (js *JobStarter) applyResourceDefaults(req *StartJobRequest) {
	originalCPU, originalMemory, originalIOBPS := req.MaxCPU, req.MaxMemory, req.MaxIOBPS

	if req.MaxCPU <= 0 {
		req.MaxCPU = config.DefaultCPULimitPercent
	}
	if req.MaxMemory <= 0 {
		req.MaxMemory = config.DefaultMemoryLimitMB
	}
	if req.MaxIOBPS <= 0 {
		req.MaxIOBPS = config.DefaultIOBPS
	}

	if originalCPU != req.MaxCPU || originalMemory != req.MaxMemory || originalIOBPS != req.MaxIOBPS {
		js.logger.Debug("applied resource defaults",
			"finalCPU", req.MaxCPU,
			"finalMemory", req.MaxMemory,
			"finalIOBPS", req.MaxIOBPS)
	}
}

// setupJobResources sets up all resources needed for the job
func (js *JobStarter) setupJobResources(ctx context.Context, job *domain.Job, req *StartJobRequest) (*network.GroupInfo, error) {
	jobLogger := js.logger.WithField("jobId", job.Id)

	// Create cgroup
	jobLogger.Debug("creating cgroup", "cgroupPath", job.CgroupPath)
	if err := js.cgroup.Create(job.CgroupPath, job.Limits.MaxCPU, job.Limits.MaxMemory, job.Limits.MaxIOBPS); err != nil {
		jobLogger.Error("failed to create cgroup", "error", err)
		return nil, fmt.Errorf("failed to create cgroup: %w", err)
	}

	jobLogger.Debug("cgroup created successfully",
		"cpuLimit", job.Limits.MaxCPU,
		"memoryLimit", job.Limits.MaxMemory,
		"ioLimit", job.Limits.MaxIOBPS)

	// Handle network group setup
	var networkGroupInfo *network.GroupInfo
	var err error

	if req.NetworkGroupID != "" {
		// Join existing or create new network group
		networkGroupInfo, err = js.networkManager.HandleNetworkGroup(req.NetworkGroupID, job.Id)
		if err != nil {
			jobLogger.Error("failed to handle network group", "error", err)
			return nil, fmt.Errorf("failed to handle network group: %w", err)
		}
		jobLogger.Info("network group handling completed",
			"groupID", req.NetworkGroupID,
			"isNew", networkGroupInfo.IsNewGroup,
			"needsJoin", networkGroupInfo.NeedsNamespaceJoin)
	} else {
		// Create isolated namespace
		networkGroupInfo = &network.GroupInfo{
			NamePath:           js.config.BuildNamespacePath(job.Id),
			SysProcAttr:        js.processLauncher.CreateSysProcAttr(true), // Enable network namespace
			IsNewGroup:         true,
			NeedsNamespaceJoin: false,
			NetworkConfig:      network.NewDefaultConfig(),
		}
	}

	return networkGroupInfo, nil
}

// startJobProcess starts the actual job process
func (js *JobStarter) startJobProcess(ctx context.Context, job *domain.Job, networkGroupInfo *network.GroupInfo) (*ProcessStartResult, error) {
	jobLogger := js.logger.WithField("jobId", job.Id)

	// Get job-init path
	initPath, err := js.getJobInitPath()
	if err != nil {
		jobLogger.Error("failed to get job-init path", "error", err)
		return nil, fmt.Errorf("failed to get job-init path: %w", err)
	}

	jobLogger.Debug("job-init binary located", "initPath", initPath)

	// Prepare environment
	env := js.prepareJobEnvironment(job, networkGroupInfo)
	jobLogger.Debug("environment prepared", "totalEnvVars", len(env))

	// Configure process launch
	launchConfig := &process.LaunchConfig{
		InitPath:      initPath,
		Environment:   env,
		SysProcAttr:   networkGroupInfo.SysProcAttr,
		Stdout:        executor.New(js.store, job.Id),
		Stderr:        executor.New(js.store, job.Id),
		NamespacePath: networkGroupInfo.NamePath,
		NeedsNSJoin:   networkGroupInfo.NeedsNamespaceJoin,
		JobID:         job.Id,
		Command:       job.Command,
		Args:          job.Args,
	}

	jobLogger.Info("starting init process with namespace isolation",
		"initPath", initPath,
		"targetCommand", job.Command,
		"targetArgs", job.Args,
		"namespaces", "pid,mount,ipc,uts,net",
		"networkGroup", job.NetworkGroupID,
		"needsNamespaceJoin", networkGroupInfo.NeedsNamespaceJoin)

	// Launch the process
	result, err := js.processLauncher.LaunchProcess(ctx, launchConfig)
	if err != nil {
		jobLogger.Error("failed to launch process", "error", err)
		return nil, fmt.Errorf("failed to launch process: %w", err)
	}

	jobLogger.Info("init process started successfully", "pid", result.PID)

	return &ProcessStartResult{
		PID:     result.PID,
		Command: result.Command,
	}, nil
}

// ProcessStartResult contains the result of starting a process
type ProcessStartResult struct {
	PID     int32
	Command osinterface.Command
}

// getJobInitPath returns the path to the job-init binary
func (js *JobStarter) getJobInitPath() (string, error) {
	js.logger.Debug("searching for job-init binary")

	execPath, err := js.osInterface.Executable()
	if err != nil {
		js.logger.Warn("failed to get executable path", "error", err)
	} else {
		initPath := filepath.Join(filepath.Dir(execPath), "job-init")
		js.logger.Debug("checking job-init in executable directory", "initPath", initPath)
		if _, err := js.osInterface.Stat(initPath); err == nil {
			js.logger.Info("job-init found in executable directory", "path", initPath)
			return initPath, nil
		}
	}

	js.logger.Error("job-init binary not found in any location")
	return "", fmt.Errorf("job-init binary not found in executable dir")
}

// prepareJobEnvironment prepares environment variables for the job
func (js *JobStarter) prepareJobEnvironment(job *domain.Job, networkGroupInfo *network.GroupInfo) []string {
	// Build job-specific environment variables
	jobEnvVars := js.processLauncher.BuildJobEnvironment(
		job.Id,
		job.Command,
		job.CgroupPath,
		job.Args,
		nil, // Network env vars will be added separately
	)

	// Add network environment variables
	if job.NetworkGroupID != "" {
		networkEnvVars := js.networkManager.PrepareEnvironment(
			job.NetworkGroupID,
			networkGroupInfo.IsNewGroup,
		)
		jobEnvVars = append(jobEnvVars, networkEnvVars...)
	}

	// Combine with base environment
	baseEnv := js.osInterface.Environ()
	return js.processLauncher.PrepareEnvironment(baseEnv, jobEnvVars)
}

// handleNamespaceCreation handles namespace creation after process start
func (js *JobStarter) handleNamespaceCreation(pid int32, networkGroupInfo *network.GroupInfo, jobID string) error {
	// Only create namespace artifacts for new groups or isolated jobs
	if !networkGroupInfo.IsNewGroup {
		return nil
	}

	jobLogger := js.logger.WithField("jobId", jobID)
	jobLogger.Debug("handling namespace creation", "pid", pid)

	// Ensure directory exists
	if err := js.ensureNamespaceDir(networkGroupInfo.NamePath); err != nil {
		return fmt.Errorf("failed to ensure namespace directory: %w", err)
	}

	// Create namespace artifact based on type
	if jobID != "" && networkGroupInfo.NamePath != "" {
		// For network groups, create bind mount; for isolated jobs, create symlink
		isNetworkGroup := js.isNetworkGroupPath(networkGroupInfo.NamePath)

		// Use the network manager's helper method for creating namespaces
		if err := js.networkManager.CreateNamespaceForJob(jobID, pid, networkGroupInfo.NamePath, isNetworkGroup); err != nil {
			jobLogger.Error("failed to create namespace for job", "error", err)
			return fmt.Errorf("failed to create namespace for job: %w", err)
		}

		if isNetworkGroup {
			jobLogger.Debug("namespace bind mount created successfully")
		} else {
			jobLogger.Debug("namespace symlink created successfully")
		}
	}

	return nil
}

// isNetworkGroupPath determines if a path is for a network group (persistent) or isolated job
func (js *JobStarter) isNetworkGroupPath(nsPath string) bool {
	// Network groups use /var/run/netns/, isolated jobs use /tmp/shared-netns/
	return filepath.HasPrefix(nsPath, js.config.VarRunNetns)
}

// ensureNamespaceDir ensures the directory for a namespace path exists
func (js *JobStarter) ensureNamespaceDir(nsPath string) error {
	dir := filepath.Dir(nsPath)

	// Check if directory exists
	if _, err := js.osInterface.Stat(dir); err != nil {
		if js.osInterface.IsNotExist(err) {
			// Directory doesn't exist, create it
			if mkdirErr := js.osInterface.MkdirAll(dir, 0755); mkdirErr != nil {
				return fmt.Errorf("failed to create directory %s: %w", dir, mkdirErr)
			}
			js.logger.Debug("created namespace directory", "path", dir)
		} else {
			return fmt.Errorf("failed to stat directory %s: %w", dir, err)
		}
	}

	return nil
}

// getNamespaceOperations gets the namespace operations from the network manager
func (js *JobStarter) getNamespaceOperations() *network.NamespaceOperations {
	// This assumes the network manager exposes its namespace operations
	// You might need to add a getter method to the network manager
	return js.networkManager.GetNamespaceOperations()
}

// ValidateJobStartRequest validates a job start request thoroughly
func (js *JobStarter) ValidateJobStartRequest(req *StartJobRequest) error {
	if req == nil {
		return fmt.Errorf("start request cannot be nil")
	}

	// Create a process launch request for validation
	launchReq := &process.LaunchRequest{
		Command:        req.Command,
		Args:           req.Args,
		MaxCPU:         req.MaxCPU,
		MaxMemory:      req.MaxMemory,
		MaxIOBPS:       req.MaxIOBPS,
		NetworkGroupID: req.NetworkGroupID,
		JobID:          "validation", // Dummy job ID for validation
	}

	return js.processValidator.ValidateLaunchRequest(launchReq)
}

// EstimateResourceUsage estimates the resource usage for a job
func (js *JobStarter) EstimateResourceUsage(req *StartJobRequest) map[string]interface{} {
	// Apply defaults for estimation
	tempReq := *req
	js.applyResourceDefaults(&tempReq)

	return map[string]interface{}{
		"estimated_cpu_percent":  tempReq.MaxCPU,
		"estimated_memory_mb":    tempReq.MaxMemory,
		"estimated_io_bps":       tempReq.MaxIOBPS,
		"network_isolation":      req.NetworkGroupID != "",
		"network_group_id":       req.NetworkGroupID,
		"requires_new_namespace": req.NetworkGroupID == "",
	}
}

// GetStartJobMetrics returns metrics about job starting operations
func (js *JobStarter) GetStartJobMetrics() map[string]interface{} {
	return map[string]interface{}{
		"component":            "job-starter",
		"default_cpu_limit":    config.DefaultCPULimitPercent,
		"default_memory_limit": config.DefaultMemoryLimitMB,
		"default_io_limit":     config.DefaultIOBPS,
	}
}

// PreflightCheck performs preflight checks before starting a job
func (js *JobStarter) PreflightCheck(req *StartJobRequest) error {
	js.logger.Debug("performing preflight check", "command", req.Command)

	// Check if command exists and is executable
	if _, err := js.worker.ValidateAndResolveCommand(req.Command); err != nil {
		return fmt.Errorf("preflight check failed - command issue: %w", err)
	}

	// Check if job-init binary exists
	if _, err := js.getJobInitPath(); err != nil {
		return fmt.Errorf("preflight check failed - job-init not found: %w", err)
	}

	// Validate network group if specified
	if req.NetworkGroupID != "" {
		if err := js.networkManager.ValidateGroupExists(req.NetworkGroupID); err != nil {
			// Group doesn't exist yet, which is fine for new groups
			js.logger.Debug("network group doesn't exist yet, will be created",
				"groupID", req.NetworkGroupID)
		}
	}

	// Check system resources (basic check)
	// This could be enhanced with actual system resource checking
	js.logger.Debug("preflight check passed")
	return nil
}

// BuildJobEnvironmentPreview builds a preview of job environment variables
func (js *JobStarter) BuildJobEnvironmentPreview(jobID, command, cgroupPath string, args []string, networkGroupID string) map[string]string {
	envMap := make(map[string]string)

	// Basic job environment
	envMap["JOB_ID"] = jobID
	envMap["JOB_COMMAND"] = command
	envMap["JOB_CGROUP_PATH"] = cgroupPath
	envMap["JOB_ARGS_COUNT"] = fmt.Sprintf("%d", len(args))

	for i, arg := range args {
		envMap[fmt.Sprintf("JOB_ARG_%d", i)] = arg
	}

	// Network environment if applicable
	if networkGroupID != "" {
		envMap["NETWORK_GROUP_ID"] = networkGroupID

		// Try to get network config if group exists
		if group, exists := js.networkManager.GetGroup(networkGroupID); exists {
			envMap["INTERNAL_SUBNET"] = group.NetworkConfig.Subnet
			envMap["INTERNAL_GATEWAY"] = group.NetworkConfig.Gateway
			envMap["INTERNAL_INTERFACE"] = group.NetworkConfig.Interface
			envMap["IS_NEW_NETWORK_GROUP"] = "false"
		} else {
			envMap["IS_NEW_NETWORK_GROUP"] = "true"
		}
	}

	return envMap
}
