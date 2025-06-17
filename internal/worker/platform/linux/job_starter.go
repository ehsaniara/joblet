//go:build linux

package linux

import (
	"context"
	"fmt"
	"net"
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

// JobStarter handles job starting operations with comprehensive validation,
// resource management, process lifecycle management, and automatic IP assignment.
type JobStarter struct {
	// Core dependencies for job starting functionality
	worker *Worker          // Parent worker for shared functionality
	store  interfaces.Store // Job persistence and state management

	// Platform-specific components
	networkManager   *network.Manager    // Network namespace and group management
	processLauncher  *process.Launcher   // Low-level process creation
	processValidator *process.Validator  // Security validation and checks
	cgroup           interfaces.Resource // Cgroup resource management

	// System interfaces
	osInterface osinterface.OsInterface // OS operations abstraction

	// Configuration and logging
	config *Config        // Platform-specific configuration
	logger *logger.Logger // Structured logging with context
}

// StartJobRequest contains all parameters needed to start a job.
type StartJobRequest struct {
	Command        string   // Executable command to run
	Args           []string // Command-line arguments
	MaxCPU         int32    // CPU limit in percentage (100 = 1 core)
	MaxMemory      int32    // Memory limit in megabytes
	MaxIOBPS       int32    // I/O bandwidth limit in bytes/second (0 = unlimited)
	NetworkGroupID string   // Network group for shared networking (empty = isolated)
}

// JobStartResult contains the result of starting a job process.
type JobStartResult struct {
	Job              *domain.Job        // Created job domain object
	NetworkGroupInfo *network.GroupInfo // Network configuration details
	ProcessPID       int32              // Process ID of started job
	InitPath         string             // Path to job-init binary used
	ResolvedCommand  string             // Full path to resolved command
	AssignedIP       string             // IP assigned by server (if any)
}

// ProcessStartResult contains the result of process creation.
type ProcessStartResult struct {
	PID     int32               // Process ID of the created process
	Command osinterface.Command // Command interface for process control
}

// NewJobStarter creates a new job starter with proper dependency injection.
func NewJobStarter(worker *Worker, deps *Dependencies) *JobStarter {
	return &JobStarter{
		// Core references
		worker: worker,
		store:  deps.Store,

		// Platform components
		networkManager:   deps.NetworkManager,
		processLauncher:  deps.ProcessLauncher,
		processValidator: deps.ProcessValidator,
		cgroup:           deps.Cgroup,

		// System interfaces
		osInterface: deps.OsInterface,

		// Configuration and logging
		config: deps.Config,
		logger: logger.New().WithField("component", "job-starter"),
	}
}

// StartJob starts a new job with comprehensive validation, automatic IP assignment, and setup.
func (js *JobStarter) StartJob(ctx context.Context, req *StartJobRequest) (*domain.Job, error) {
	// Early context cancellation check
	select {
	case <-ctx.Done():
		js.logger.Warn("start job cancelled by context before processing", "error", ctx.Err())
		return nil, ctx.Err()
	default:
		// Continue with job creation
	}

	// Generate unique job ID for this job
	jobID := js.worker.GetNextJobID()
	jobLogger := js.logger.WithField("jobId", jobID)

	jobLogger.Info("starting new job creation process",
		"command", req.Command,
		"args", req.Args,
		"requestedCPU", req.MaxCPU,
		"requestedMemory", req.MaxMemory,
		"requestedIOBPS", req.MaxIOBPS,
		"networkGroup", req.NetworkGroupID)

	// Phase 1: Request Validation
	if err := js.validateStartRequest(req); err != nil {
		jobLogger.Error("job creation failed during validation", "error", err)
		return nil, fmt.Errorf("invalid start request: %w", err)
	}
	jobLogger.Debug("request validation completed successfully")

	// Phase 2: Resource Defaults Application
	js.applyResourceDefaults(req)
	jobLogger.Debug("resource defaults applied",
		"finalCPU", req.MaxCPU,
		"finalMemory", req.MaxMemory,
		"finalIOBPS", req.MaxIOBPS)

	// Phase 3: Command Resolution and Security
	resolvedCommand, err := js.worker.ValidateAndResolveCommand(req.Command)
	if err != nil {
		jobLogger.Error("command validation/resolution failed", "error", err)
		return nil, err
	}
	jobLogger.Debug("command resolved successfully",
		"original", req.Command,
		"resolved", resolvedCommand)

	// Phase 4: Domain Object Creation
	job := js.worker.CreateJobDomain(req, jobID, resolvedCommand)
	jobLogger.Debug("job domain object created",
		"status", string(job.Status),
		"cgroupPath", job.CgroupPath,
		"interfaceName", job.InterfaceName)

	// Phase 5: Network Setup with Automatic IP Assignment
	networkGroupInfo, assignedIP, isNewNetworkGroup, err := js.setupJobNetworking(ctx, job, req)
	if err != nil {
		jobLogger.Error("failed to setup job networking", "error", err)
		js.worker.CleanupJobResources(jobID, req.NetworkGroupID, isNewNetworkGroup)
		return nil, err
	}

	// Update job with networking information
	if assignedIP != nil {
		job.AssignedIP = assignedIP.String()
		job.NetworkSubnet = networkGroupInfo.NetworkConfig.Subnet
		jobLogger.Info("server assigned IP to job",
			"assignedIP", job.AssignedIP,
			"subnet", job.NetworkSubnet,
			"interface", job.InterfaceName)
	}

	// Phase 6: Resource Setup (Cgroups)
	if err := js.setupJobResources(ctx, job, req); err != nil {
		jobLogger.Error("failed to setup job resources", "error", err)
		js.worker.CleanupJobResources(jobID, req.NetworkGroupID, isNewNetworkGroup)
		return nil, err
	}
	jobLogger.Debug("job resources setup completed")

	// Phase 7: Job Store Registration
	js.store.CreateNewJob(job)
	jobLogger.Debug("job registered in store", "status", string(job.Status))

	// Phase 8: Process Creation
	processResult, err := js.startJobProcess(ctx, job, networkGroupInfo, assignedIP)
	if err != nil {
		jobLogger.Error("failed to start job process", "error", err)

		// Perform comprehensive cleanup on process creation failure
		js.worker.CleanupJobResources(jobID, req.NetworkGroupID, isNewNetworkGroup)

		// Update job status to failed in store
		failedJob := job.DeepCopy()
		if failErr := failedJob.Fail(-1); failErr != nil {
			jobLogger.Warn("domain validation failed during failure handling", "error", failErr)
			failedJob.Status = domain.StatusFailed
			failedJob.ExitCode = -1
			now := time.Now()
			failedJob.EndTime = &now
		}
		js.store.UpdateJob(failedJob)

		return nil, err
	}
	jobLogger.Debug("job process created successfully", "pid", processResult.PID)

	// Phase 9: Job Status Update
	runningJob := job.DeepCopy()
	runningJob.Pid = processResult.PID

	if err := runningJob.MarkAsRunning(processResult.PID); err != nil {
		jobLogger.Warn("domain validation failed for running status transition", "error", err)
		runningJob.Status = domain.StatusRunning
		runningJob.Pid = processResult.PID
	}
	runningJob.StartTime = time.Now()
	js.store.UpdateJob(runningJob)

	jobLogger.Debug("job status updated to running", "pid", runningJob.Pid)

	// Phase 10: Namespace Artifact Creation
	if err := js.handleNamespaceCreation(processResult.PID, networkGroupInfo, jobID); err != nil {
		jobLogger.Error("failed to handle namespace creation", "error", err)
		js.worker.EmergencyCleanup(jobID, processResult.PID, req.NetworkGroupID)
		return nil, fmt.Errorf("failed to handle namespace creation: %w", err)
	}
	jobLogger.Debug("namespace artifacts created successfully")

	// Phase 11: Network Group Registration
	if req.NetworkGroupID != "" {
		if err := js.networkManager.IncrementJobCount(req.NetworkGroupID); err != nil {
			jobLogger.Warn("failed to increment network group job count", "error", err)
		} else {
			jobLogger.Debug("network group job count incremented")
		}
	}

	// Phase 12: Monitoring Initialization
	js.worker.StartMonitoring(ctx, processResult.Command, runningJob)
	jobLogger.Debug("job monitoring started")

	// Phase 13: Success Completion
	jobLogger.Info("job started successfully",
		"pid", runningJob.Pid,
		"status", string(runningJob.Status),
		"cgroupPath", runningJob.CgroupPath,
		"networkGroup", runningJob.NetworkGroupID,
		"assignedIP", runningJob.AssignedIP)

	return runningJob, nil
}

// setupJobNetworking sets up networking with automatic IP assignment for network groups
func (js *JobStarter) setupJobNetworking(ctx context.Context, job *domain.Job, req *StartJobRequest) (*network.GroupInfo, *net.IP, bool, error) {
	jobLogger := js.logger.WithField("jobId", job.Id)

	if req.NetworkGroupID != "" {
		// NETWORK GROUP: Server automatically assigns unique IP
		jobLogger.Info("setting up network group with automatic IP assignment",
			"groupID", req.NetworkGroupID)

		networkGroupInfo, assignedIP, err := js.networkManager.HandleNetworkGroupWithAutoIP(
			req.NetworkGroupID,
			job.Id,
		)
		if err != nil {
			jobLogger.Error("failed to setup network group with auto IP", "error", err)
			return nil, nil, false, fmt.Errorf("failed to setup network group: %w", err)
		}

		jobLogger.Info("network group setup complete",
			"groupID", req.NetworkGroupID,
			"assignedIP", assignedIP.String(),
			"isNewGroup", networkGroupInfo.IsNewGroup,
			"subnet", networkGroupInfo.NetworkConfig.Subnet)

		return networkGroupInfo, &assignedIP, networkGroupInfo.IsNewGroup, nil

	} else {
		// ISOLATED JOB: Server creates isolated namespace
		jobLogger.Info("setting up isolated job networking")

		networkGroupInfo := &network.GroupInfo{
			NamePath:           js.config.BuildNamespacePath(job.Id),
			SysProcAttr:        js.processLauncher.CreateSysProcAttr(true),
			IsNewGroup:         true,
			NeedsNamespaceJoin: false,
			NetworkConfig:      network.NewDefaultConfig(),
		}

		return networkGroupInfo, nil, true, nil
	}
}

// setupJobResources configures cgroup resources for the job
func (js *JobStarter) setupJobResources(ctx context.Context, job *domain.Job, req *StartJobRequest) error {
	jobLogger := js.logger.WithField("jobId", job.Id)

	// Create cgroup with resource limits
	jobLogger.Debug("creating cgroup for job", "cgroupPath", job.CgroupPath)

	err := js.cgroup.Create(
		job.CgroupPath,
		job.Limits.MaxCPU,
		job.Limits.MaxMemory,
		job.Limits.MaxIOBPS,
	)
	if err != nil {
		jobLogger.Error("failed to create cgroup", "error", err)
		return fmt.Errorf("failed to create cgroup: %w", err)
	}

	jobLogger.Debug("cgroup created successfully",
		"cpuLimit", job.Limits.MaxCPU,
		"memoryLimit", job.Limits.MaxMemory,
		"ioLimit", job.Limits.MaxIOBPS)

	return nil
}

// startJobProcess creates and launches the job process with automatic IP configuration
func (js *JobStarter) startJobProcess(ctx context.Context, job *domain.Job, networkGroupInfo *network.GroupInfo, assignedIP *net.IP) (*ProcessStartResult, error) {
	jobLogger := js.logger.WithField("jobId", job.Id)

	// Locate job-init binary
	initPath, err := js.getJobInitPath()
	if err != nil {
		jobLogger.Error("failed to locate job-init binary", "error", err)
		return nil, fmt.Errorf("failed to get job-init path: %w", err)
	}
	jobLogger.Debug("job-init binary located", "initPath", initPath)

	// Prepare environment with server networking decisions
	env := js.prepareJobEnvironmentWithServerDecisions(job, networkGroupInfo, assignedIP)
	jobLogger.Debug("job environment prepared", "totalEnvVars", len(env))

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

	jobLogger.Info("launching job-init process with networking configuration",
		"initPath", initPath,
		"targetCommand", job.Command,
		"targetArgs", job.Args,
		"networkGroup", job.NetworkGroupID,
		"assignedIP", job.AssignedIP,
		"needsNamespaceJoin", networkGroupInfo.NeedsNamespaceJoin)

	// Launch the process
	result, err := js.processLauncher.LaunchProcess(ctx, launchConfig)
	if err != nil {
		jobLogger.Error("failed to launch job-init process", "error", err)
		return nil, fmt.Errorf("failed to launch process: %w", err)
	}

	jobLogger.Info("job-init process launched successfully", "pid", result.PID)

	return &ProcessStartResult{
		PID:     result.PID,
		Command: result.Command,
	}, nil
}

// prepareJobEnvironmentWithServerDecisions builds the environment with server networking decisions
func (js *JobStarter) prepareJobEnvironmentWithServerDecisions(job *domain.Job, networkGroupInfo *network.GroupInfo, assignedIP *net.IP) []string {
	// Build base job environment
	baseEnv := js.osInterface.Environ()
	jobEnvVars := js.processLauncher.BuildJobEnvironment(
		job.Id,
		job.Command,
		job.CgroupPath,
		job.Args,
		nil,
	)

	if job.NetworkGroupID != "" {
		// Essential network environment variables only
		networkEnvVars := []string{
			fmt.Sprintf("NETWORK_GROUP_ID=%s", job.NetworkGroupID),
			fmt.Sprintf("INTERNAL_SUBNET=%s", networkGroupInfo.NetworkConfig.Subnet),
			fmt.Sprintf("INTERNAL_GATEWAY=%s", networkGroupInfo.NetworkConfig.Gateway),
		}

		if assignedIP != nil {
			// Enable automatic IP setup with minimal required variables
			networkEnvVars = append(networkEnvVars,
				fmt.Sprintf("JOB_ASSIGNED_IP=%s", assignedIP.String()),
				fmt.Sprintf("JOB_INTERFACE=%s", job.InterfaceName),
				"AUTO_SETUP_IP=true",
			)

			js.logger.Info("enabling automatic IP setup",
				"jobID", job.Id,
				"assignedIP", assignedIP.String(),
				"interface", job.InterfaceName)
		}

		jobEnvVars = append(jobEnvVars, networkEnvVars...)
	}

	return js.processLauncher.PrepareEnvironment(baseEnv, jobEnvVars)
}

// validateStartRequest performs comprehensive validation of job start parameters
func (js *JobStarter) validateStartRequest(req *StartJobRequest) error {
	if req == nil {
		return fmt.Errorf("start request cannot be nil")
	}

	// Command validation
	if err := js.processValidator.ValidateCommand(req.Command); err != nil {
		return fmt.Errorf("command validation failed: %w", err)
	}

	// Arguments validation
	if err := js.processValidator.ValidateArguments(req.Args); err != nil {
		return fmt.Errorf("arguments validation failed: %w", err)
	}

	// Resource limits validation
	if err := js.processValidator.ValidateResourceLimits(req.MaxCPU, req.MaxMemory, req.MaxIOBPS); err != nil {
		return fmt.Errorf("resource limits validation failed: %w", err)
	}

	// Network group validation
	if req.NetworkGroupID != "" {
		if err := js.processValidator.ValidateNetworkGroupID(req.NetworkGroupID); err != nil {
			return fmt.Errorf("network group ID validation failed: %w", err)
		}
	}

	js.logger.Debug("start request validation completed successfully")
	return nil
}

// applyResourceDefaults applies system default values for unspecified resource limits
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
			"originalCPU", originalCPU, "finalCPU", req.MaxCPU,
			"originalMemory", originalMemory, "finalMemory", req.MaxMemory,
			"originalIOBPS", originalIOBPS, "finalIOBPS", req.MaxIOBPS)
	}
}

// getJobInitPath locates the job-init binary required for job execution
func (js *JobStarter) getJobInitPath() (string, error) {
	js.logger.Debug("searching for job-init binary")

	// Check same directory as main executable
	execPath, err := js.osInterface.Executable()
	if err != nil {
		js.logger.Warn("failed to determine executable path", "error", err)
	} else {
		initPath := filepath.Join(filepath.Dir(execPath), "job-init")
		js.logger.Debug("checking job-init in executable directory", "initPath", initPath)

		if _, err := js.osInterface.Stat(initPath); err == nil {
			js.logger.Info("job-init found in executable directory", "path", initPath)
			return initPath, nil
		}
		js.logger.Debug("job-init not found in executable directory")
	}

	// Check standard system paths
	standardPaths := []string{
		"/usr/local/bin/job-init",
		"/usr/bin/job-init",
		"/opt/job-worker/bin/job-init",
	}

	for _, path := range standardPaths {
		js.logger.Debug("checking standard path", "path", path)
		if _, err := js.osInterface.Stat(path); err == nil {
			js.logger.Info("job-init found in standard location", "path", path)
			return path, nil
		}
	}

	js.logger.Error("job-init binary not found in any searched location",
		"searchedPaths", append([]string{"executable_directory"}, standardPaths...))
	return "", fmt.Errorf("job-init binary not found in executable directory or standard paths")
}

// handleNamespaceCreation creates filesystem artifacts for namespace access
func (js *JobStarter) handleNamespaceCreation(pid int32, networkGroupInfo *network.GroupInfo, jobID string) error {
	if !networkGroupInfo.IsNewGroup {
		js.logger.Debug("skipping namespace artifact creation for existing group")
		return nil
	}

	jobLogger := js.logger.WithField("jobId", jobID)
	jobLogger.Debug("creating namespace artifacts", "pid", pid)

	if err := js.ensureNamespaceDir(networkGroupInfo.NamePath); err != nil {
		return fmt.Errorf("failed to ensure namespace directory: %w", err)
	}

	if jobID != "" && networkGroupInfo.NamePath != "" {
		isNetworkGroup := js.isNetworkGroupPath(networkGroupInfo.NamePath)

		err := js.networkManager.CreateNamespaceForJob(
			jobID,
			pid,
			networkGroupInfo.NamePath,
			isNetworkGroup,
		)
		if err != nil {
			jobLogger.Error("failed to create namespace artifact", "error", err)
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

// isNetworkGroupPath determines if a namespace path is for a network group
func (js *JobStarter) isNetworkGroupPath(nsPath string) bool {
	return filepath.HasPrefix(nsPath, js.config.VarRunNetns)
}

// ensureNamespaceDir creates parent directories for namespace artifacts
func (js *JobStarter) ensureNamespaceDir(nsPath string) error {
	dir := filepath.Dir(nsPath)

	if _, err := js.osInterface.Stat(dir); err != nil {
		if js.osInterface.IsNotExist(err) {
			if mkdirErr := js.osInterface.MkdirAll(dir, 0755); mkdirErr != nil {
				return fmt.Errorf("failed to create directory %s: %w", dir, mkdirErr)
			}
			js.logger.Debug("created directory", "path", dir)
		} else {
			return fmt.Errorf("failed to stat directory %s: %w", dir, err)
		}
	}

	return nil
}

// Additional helper methods for testing and monitoring

// ValidateJobStartRequest validates a job start request without creating a job
func (js *JobStarter) ValidateJobStartRequest(req *StartJobRequest) error {
	return js.validateStartRequest(req)
}

// EstimateResourceUsage estimates resource requirements for a job
func (js *JobStarter) EstimateResourceUsage(req *StartJobRequest) map[string]interface{} {
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

// PreflightCheck performs comprehensive pre-flight checks before job creation
func (js *JobStarter) PreflightCheck(req *StartJobRequest) error {
	js.logger.Debug("performing comprehensive preflight check", "command", req.Command)

	// Command validation and resolution
	if _, err := js.worker.ValidateAndResolveCommand(req.Command); err != nil {
		return fmt.Errorf("preflight check failed - command issue: %w", err)
	}

	// job-init binary availability
	if _, err := js.getJobInitPath(); err != nil {
		return fmt.Errorf("preflight check failed - job-init not found: %w", err)
	}

	// Network group validation if specified
	if req.NetworkGroupID != "" {
		if err := js.networkManager.ValidateGroupExists(req.NetworkGroupID); err != nil {
			js.logger.Debug("network group doesn't exist yet, will be created",
				"groupID", req.NetworkGroupID)
		}
	}

	// Configuration validation
	if err := js.config.Validate(); err != nil {
		return fmt.Errorf("preflight check failed - configuration invalid: %w", err)
	}

	js.logger.Debug("preflight check completed successfully")
	return nil
}
