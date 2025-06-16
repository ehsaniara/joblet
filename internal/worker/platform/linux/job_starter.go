//go:build linux

// Package linux provides job starting functionality for the Linux platform worker.
// This package handles the complete job initialization process including validation,
// resource setup, process creation, and monitoring initialization.
//
// Job Starting Process Flow:
//
//	┌─────────────────────────────────────────────────────────────────┐
//	│                     Job Start Process                           │
//	└─────────────────────────────────────────────────────────────────┘
//	│
//	├── 1. Request Validation
//	│   ├── Command validation (security checks)
//	│   ├── Argument validation (size, content)
//	│   ├── Resource limit validation
//	│   └── Network group validation
//	│
//	├── 2. Resource Setup
//	│   ├── Apply default resource limits
//	│   ├── Create cgroup with limits
//	│   ├── Setup network namespace/group
//	│   └── Generate unique job ID
//	│
//	├── 3. Process Creation
//	│   ├── Resolve command path
//	│   ├── Prepare environment variables
//	│   ├── Launch job-init process
//	│   └── Create namespace artifacts
//	│
//	├── 4. Job Registration
//	│   ├── Store job in persistent store
//	│   ├── Update job status to running
//	│   ├── Start monitoring
//	│   └── Return job object
//	│
//	└── 5. Error Handling
//	    ├── Cleanup resources on failure
//	    ├── Update job status to failed
//	    └── Log detailed error information
//
// Security Features:
// - Command validation prevents shell injection
// - Resource limits prevent system abuse
// - Network isolation protects system and other jobs
// - Process namespaces provide comprehensive isolation
// - Argument and environment size limits prevent DoS attacks
//
// Thread Safety: All public methods are thread-safe and can be called
// concurrently from multiple goroutines.
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

// JobStarter handles job starting operations with comprehensive validation,
// resource management, and process lifecycle management.
//
// The JobStarter is responsible for:
//
// Validation and Security:
// - Command and argument validation
// - Resource limit validation and application
// - Network group security checks
// - Path traversal prevention
//
// Resource Management:
// - Cgroup creation and configuration
// - Network namespace setup
// - Environment preparation
// - File descriptor management
//
// Process Management:
// - job-init binary location and execution
// - Namespace isolation setup
// - Process monitoring initialization
// - Error recovery and cleanup
//
// The component implements a robust error handling strategy with automatic
// cleanup on failure to prevent resource leaks.
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
// This structure normalizes job creation parameters and provides
// a structured interface for internal processing.
//
// Field Validation:
// - Command: Must be non-empty, safe for execution
// - Args: Limited in count and individual size
// - Resource limits: Must be non-negative, within system limits
// - NetworkGroupID: Must be valid identifier format if specified
//
// Security Considerations:
// - All string fields are validated for dangerous content
// - Resource limits are bounded to prevent system abuse
// - Network group IDs are sanitized for filesystem safety
type StartJobRequest struct {
	Command        string   // Executable command to run
	Args           []string // Command-line arguments
	MaxCPU         int32    // CPU limit in percentage (100 = 1 core)
	MaxMemory      int32    // Memory limit in megabytes
	MaxIOBPS       int32    // I/O bandwidth limit in bytes/second (0 = unlimited)
	NetworkGroupID string   // Network group for shared networking (empty = isolated)
}

// JobStartResult contains the result of starting a job process.
// This structure provides comprehensive information about the job
// creation process for monitoring and debugging.
//
// The result includes both success information and any warnings
// or partial failures that occurred during startup.
type JobStartResult struct {
	Job              *domain.Job        // Created job domain object
	NetworkGroupInfo *network.GroupInfo // Network configuration details
	ProcessPID       int32              // Process ID of started job
	InitPath         string             // Path to job-init binary used
	ResolvedCommand  string             // Full path to resolved command
}

// NewJobStarter creates a new job starter with proper dependency injection.
// This factory function initializes the job starter with all required
// components and establishes logging context.
//
// Parameters:
//
//	worker: Parent worker providing shared functionality
//	deps:   Complete dependency structure
//
// Returns: Fully initialized JobStarter ready for job creation
//
// The starter inherits configuration and shared components from the
// parent worker while maintaining its own specialized functionality.
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

// StartJob starts a new job with comprehensive validation and setup.
// This is the main entry point for job creation and handles the complete
// job initialization process from validation through monitoring setup.
//
// Process Overview:
//  1. Context validation and cancellation checking
//  2. Request validation and sanitization
//  3. Resource limit application and validation
//  4. Command resolution and security verification
//  5. Resource setup (cgroups, networking)
//  6. Process creation with proper isolation
//  7. Monitoring initialization
//  8. Error handling and cleanup
//
// Parameters:
//
//	ctx: Context for cancellation and timeout control
//	req: Complete job creation request
//
// Returns:
//
//	job:   Successfully created and running job
//	error: Detailed error information on failure
//
// Error Handling: On any failure, the method performs comprehensive cleanup
// to prevent resource leaks. This includes cgroup cleanup, network group
// management, and job store updates.
//
// Context Handling: Respects context cancellation at all stages. If the
// context is cancelled during startup, cleanup is performed automatically.
func (js *JobStarter) StartJob(ctx context.Context, req *StartJobRequest) (*domain.Job, error) {
	// Early context cancellation check
	// This prevents unnecessary work if the operation is already cancelled
	select {
	case <-ctx.Done():
		js.logger.Warn("start job cancelled by context before processing", "error", ctx.Err())
		return nil, ctx.Err()
	default:
		// Continue with job creation
	}

	// Generate unique job ID for this job
	// This ID will be used throughout the system for identification
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
	// Comprehensive validation of all request parameters
	if err := js.validateStartRequest(req); err != nil {
		jobLogger.Error("job creation failed during validation", "error", err)
		return nil, fmt.Errorf("invalid start request: %w", err)
	}
	jobLogger.Debug("request validation completed successfully")

	// Phase 2: Resource Defaults Application
	// Apply system defaults for unspecified resource limits
	js.applyResourceDefaults(req)
	jobLogger.Debug("resource defaults applied",
		"finalCPU", req.MaxCPU,
		"finalMemory", req.MaxMemory,
		"finalIOBPS", req.MaxIOBPS)

	// Phase 3: Command Resolution and Security
	// Validate command security and resolve to full path
	resolvedCommand, err := js.worker.ValidateAndResolveCommand(req.Command)
	if err != nil {
		jobLogger.Error("command validation/resolution failed", "error", err)
		return nil, err
	}
	jobLogger.Debug("command resolved successfully",
		"original", req.Command,
		"resolved", resolvedCommand)

	// Phase 4: Domain Object Creation
	// Create job domain object with validated parameters
	job := js.worker.CreateJobDomain(req, jobID, resolvedCommand)
	jobLogger.Debug("job domain object created",
		"status", string(job.Status),
		"cgroupPath", job.CgroupPath)

	// Phase 5: Resource Setup
	// Setup all required resources with cleanup on failure
	networkGroupInfo, err := js.setupJobResources(ctx, job, req)
	if err != nil {
		jobLogger.Error("failed to setup job resources", "error", err)
		// Cleanup any partially created resources
		js.worker.CleanupJobResources(jobID, req.NetworkGroupID, true)
		return nil, err
	}
	jobLogger.Debug("job resources setup completed",
		"networkGroup", req.NetworkGroupID,
		"isNewGroup", networkGroupInfo.IsNewGroup)

	// Phase 6: Job Store Registration
	// Store the job before process creation for tracking
	js.store.CreateNewJob(job)
	jobLogger.Debug("job registered in store", "status", string(job.Status))

	// Phase 7: Process Creation
	// Launch the actual job process with proper isolation
	processResult, err := js.startJobProcess(ctx, job, networkGroupInfo)
	if err != nil {
		jobLogger.Error("failed to start job process", "error", err)

		// Perform comprehensive cleanup on process creation failure
		js.worker.CleanupJobResources(jobID, req.NetworkGroupID, networkGroupInfo.IsNewGroup)

		// Update job status to failed in store
		failedJob := job.DeepCopy()
		if failErr := failedJob.Fail(-1); failErr != nil {
			// Handle domain validation failure gracefully
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

	// Phase 8: Job Status Update
	// Update job with process information and mark as running
	runningJob := job.DeepCopy()
	runningJob.Pid = processResult.PID

	// Use domain validation for status transition
	if err := runningJob.MarkAsRunning(processResult.PID); err != nil {
		// Handle validation failure but continue (log and update manually)
		jobLogger.Warn("domain validation failed for running status transition", "error", err)
		runningJob.Status = domain.StatusRunning
		runningJob.Pid = processResult.PID
	}
	runningJob.StartTime = time.Now()
	js.store.UpdateJob(runningJob)

	jobLogger.Debug("job status updated to running", "pid", runningJob.Pid)

	// Phase 9: Namespace Artifact Creation
	// Create filesystem artifacts for namespace access
	if err := js.handleNamespaceCreation(processResult.PID, networkGroupInfo, jobID); err != nil {
		jobLogger.Error("failed to handle namespace creation", "error", err)
		// Perform emergency cleanup and fail the job
		js.worker.EmergencyCleanup(jobID, processResult.PID, req.NetworkGroupID)
		return nil, fmt.Errorf("failed to handle namespace creation: %w", err)
	}
	jobLogger.Debug("namespace artifacts created successfully")

	// Phase 10: Network Group Registration
	// Update network group membership if applicable
	if req.NetworkGroupID != "" {
		if err := js.networkManager.IncrementJobCount(req.NetworkGroupID); err != nil {
			jobLogger.Warn("failed to increment network group job count", "error", err)
			// This is not fatal, continue with job startup
		} else {
			jobLogger.Debug("network group job count incremented")
		}
	}

	// Phase 11: Monitoring Initialization
	// Start continuous monitoring of the job process
	js.worker.StartMonitoring(ctx, processResult.Command, runningJob)
	jobLogger.Debug("job monitoring started")

	// Phase 12: Success Completion
	jobLogger.Info("job started successfully",
		"pid", runningJob.Pid,
		"status", string(runningJob.Status),
		"cgroupPath", runningJob.CgroupPath,
		"networkGroup", runningJob.NetworkGroupID)

	return runningJob, nil
}

// validateStartRequest performs comprehensive validation of job start parameters.
// This method implements multiple layers of security and sanity checks to ensure
// safe job execution.
//
// Validation Layers:
//  1. Null check validation
//  2. Command security validation
//  3. Argument safety validation
//  4. Resource limit validation
//  5. Network group ID validation
//
// Parameters:
//
//	req: Job start request to validate
//
// Returns: nil if request is valid, detailed error describing issues
//
// Security Focus: This method is critical for preventing various attack vectors
// including command injection, resource exhaustion, and filesystem attacks.
func (js *JobStarter) validateStartRequest(req *StartJobRequest) error {
	// Basic structure validation
	if req == nil {
		return fmt.Errorf("start request cannot be nil")
	}

	// Command validation - prevents injection attacks
	if err := js.processValidator.ValidateCommand(req.Command); err != nil {
		return fmt.Errorf("command validation failed: %w", err)
	}

	// Arguments validation - prevents oversized or malicious arguments
	if err := js.processValidator.ValidateArguments(req.Args); err != nil {
		return fmt.Errorf("arguments validation failed: %w", err)
	}

	// Resource limits validation - prevents system abuse
	if err := js.processValidator.ValidateResourceLimits(req.MaxCPU, req.MaxMemory, req.MaxIOBPS); err != nil {
		return fmt.Errorf("resource limits validation failed: %w", err)
	}

	// Network group validation - ensures safe filesystem operations
	if req.NetworkGroupID != "" {
		if err := js.processValidator.ValidateNetworkGroupID(req.NetworkGroupID); err != nil {
			return fmt.Errorf("network group ID validation failed: %w", err)
		}
	}

	js.logger.Debug("start request validation completed successfully")
	return nil
}

// applyResourceDefaults applies system default values for unspecified resource limits.
// This method ensures all jobs have defined resource constraints while allowing
// users to rely on sensible defaults.
//
// Default Application Strategy:
// - Only apply defaults for zero/unspecified values
// - Use configuration-defined defaults
// - Log when defaults are applied for transparency
//
// Parameters:
//
//	req: Job request to modify with defaults (modified in-place)
//
// The method modifies the request in-place to apply defaults, ensuring
// all resource limits are properly specified before job creation.
func (js *JobStarter) applyResourceDefaults(req *StartJobRequest) {
	// Capture original values for comparison and logging
	originalCPU, originalMemory, originalIOBPS := req.MaxCPU, req.MaxMemory, req.MaxIOBPS

	// Apply CPU default if not specified
	// Zero or negative values indicate no user specification
	if req.MaxCPU <= 0 {
		req.MaxCPU = config.DefaultCPULimitPercent
	}

	// Apply memory default if not specified
	if req.MaxMemory <= 0 {
		req.MaxMemory = config.DefaultMemoryLimitMB
	}

	// Apply I/O default if not specified
	if req.MaxIOBPS <= 0 {
		req.MaxIOBPS = config.DefaultIOBPS
	}

	// Log default application for transparency
	if originalCPU != req.MaxCPU || originalMemory != req.MaxMemory || originalIOBPS != req.MaxIOBPS {
		js.logger.Debug("applied resource defaults",
			"originalCPU", originalCPU, "finalCPU", req.MaxCPU,
			"originalMemory", originalMemory, "finalMemory", req.MaxMemory,
			"originalIOBPS", originalIOBPS, "finalIOBPS", req.MaxIOBPS)
	}
}

// setupJobResources configures all resources needed for job execution.
// This method coordinates cgroup creation, network setup, and resource
// allocation in the correct order with proper error handling.
//
// Resource Setup Process:
//  1. Create cgroup with resource limits
//  2. Configure network isolation (group or isolated)
//  3. Validate resource accessibility
//  4. Prepare for process creation
//
// Parameters:
//
//	ctx: Context for cancellation and timeout
//	job: Job domain object with resource specifications
//	req: Original job request for network configuration
//
// Returns:
//
//	networkGroupInfo: Network configuration for process creation
//	error: nil on success, detailed error on failure
//
// Error Handling: On failure, this method does NOT perform cleanup.
// The caller is responsible for cleanup using CleanupJobResources.
func (js *JobStarter) setupJobResources(ctx context.Context, job *domain.Job, req *StartJobRequest) (*network.GroupInfo, error) {
	jobLogger := js.logger.WithField("jobId", job.Id)

	// Phase 1: Cgroup Creation
	// Create and configure cgroup with resource limits
	jobLogger.Debug("creating cgroup for job", "cgroupPath", job.CgroupPath)

	err := js.cgroup.Create(
		job.CgroupPath,
		job.Limits.MaxCPU,
		job.Limits.MaxMemory,
		job.Limits.MaxIOBPS,
	)
	if err != nil {
		jobLogger.Error("failed to create cgroup", "error", err)
		return nil, fmt.Errorf("failed to create cgroup: %w", err)
	}

	jobLogger.Debug("cgroup created successfully",
		"cpuLimit", job.Limits.MaxCPU,
		"memoryLimit", job.Limits.MaxMemory,
		"ioLimit", job.Limits.MaxIOBPS)

	// Phase 2: Network Configuration
	// Setup network isolation based on job requirements
	var networkGroupInfo *network.GroupInfo

	if req.NetworkGroupID != "" {
		// Handle network group membership (shared networking)
		networkGroupInfo, err = js.networkManager.HandleNetworkGroup(req.NetworkGroupID, job.Id)
		if err != nil {
			jobLogger.Error("failed to handle network group setup", "error", err)
			return nil, fmt.Errorf("failed to handle network group: %w", err)
		}

		jobLogger.Info("network group handling completed",
			"groupID", req.NetworkGroupID,
			"isNewGroup", networkGroupInfo.IsNewGroup,
			"needsJoin", networkGroupInfo.NeedsNamespaceJoin)
	} else {
		// Setup isolated networking (private namespace)
		networkGroupInfo = &network.GroupInfo{
			NamePath:           js.config.BuildNamespacePath(job.Id),
			SysProcAttr:        js.processLauncher.CreateSysProcAttr(true), // Enable network namespace
			IsNewGroup:         true,
			NeedsNamespaceJoin: false, // New namespace, no join needed
			NetworkConfig:      network.NewDefaultConfig(),
		}

		jobLogger.Debug("isolated network configuration prepared",
			"namespacePath", networkGroupInfo.NamePath)
	}

	jobLogger.Debug("job resources setup completed successfully")
	return networkGroupInfo, nil
}

// startJobProcess creates and launches the job process with proper isolation.
// This method handles the complex process of creating a properly isolated
// job process using the job-init wrapper.
//
// Process Creation Strategy:
//  1. Locate job-init binary
//  2. Prepare job environment variables
//  3. Configure process launch parameters
//  4. Launch job-init with namespace isolation
//  5. Verify process startup success
//
// Parameters:
//
//	ctx:              Context for process creation timeout
//	job:              Job domain object with process details
//	networkGroupInfo: Network configuration for the process
//
// Returns:
//
//	processResult: Information about the created process
//	error:         nil on success, detailed error on failure
//
// The job-init process is responsible for:
// - Joining the correct cgroup
// - Setting up network configuration
// - Executing the actual user command
// - Handling process lifecycle within isolation
func (js *JobStarter) startJobProcess(ctx context.Context, job *domain.Job, networkGroupInfo *network.GroupInfo) (*ProcessStartResult, error) {
	jobLogger := js.logger.WithField("jobId", job.Id)

	// Phase 1: Locate job-init Binary
	// Find the job-init helper binary required for process setup
	initPath, err := js.getJobInitPath()
	if err != nil {
		jobLogger.Error("failed to locate job-init binary", "error", err)
		return nil, fmt.Errorf("failed to get job-init path: %w", err)
	}
	jobLogger.Debug("job-init binary located", "initPath", initPath)

	// Phase 2: Environment Preparation
	// Build environment variables for job execution
	env := js.prepareJobEnvironment(job, networkGroupInfo)
	jobLogger.Debug("job environment prepared", "totalEnvVars", len(env))

	// Phase 3: Process Launch Configuration
	// Configure all parameters for process creation
	launchConfig := &process.LaunchConfig{
		InitPath:      initPath,                            // job-init binary path
		Environment:   env,                                 // Complete environment
		SysProcAttr:   networkGroupInfo.SysProcAttr,        // Namespace configuration
		Stdout:        executor.New(js.store, job.Id),      // Output capture
		Stderr:        executor.New(js.store, job.Id),      // Error capture
		NamespacePath: networkGroupInfo.NamePath,           // Namespace reference
		NeedsNSJoin:   networkGroupInfo.NeedsNamespaceJoin, // Join existing namespace
		JobID:         job.Id,                              // Job identification
		Command:       job.Command,                         // Target command
		Args:          job.Args,                            // Command arguments
	}

	jobLogger.Info("launching job-init process with full isolation",
		"initPath", initPath,
		"targetCommand", job.Command,
		"targetArgs", job.Args,
		"isolationNamespaces", "pid,mount,ipc,uts,net",
		"networkGroup", job.NetworkGroupID,
		"needsNamespaceJoin", networkGroupInfo.NeedsNamespaceJoin)

	// Phase 4: Process Launch
	// Create the actual process with all configured isolation
	result, err := js.processLauncher.LaunchProcess(ctx, launchConfig)
	if err != nil {
		jobLogger.Error("failed to launch job-init process", "error", err)
		return nil, fmt.Errorf("failed to launch process: %w", err)
	}

	jobLogger.Info("job-init process launched successfully",
		"pid", result.PID,
		"command", job.Command)

	// Phase 5: Result Preparation
	// Package process information for return
	return &ProcessStartResult{
		PID:     result.PID,
		Command: result.Command,
	}, nil
}

// ProcessStartResult contains the result of process creation.
// This structure provides essential information about the created
// process for monitoring and management.
type ProcessStartResult struct {
	PID     int32               // Process ID of the created process
	Command osinterface.Command // Command interface for process control
}

// getJobInitPath locates the job-init binary required for job execution.
// This method implements a search strategy to find the helper binary
// in standard locations.
//
// Search Strategy:
//  1. Check same directory as main worker executable
//  2. Search standard system binary paths
//  3. Return error if not found anywhere
//
// Returns:
//
//	path:  Absolute path to job-init binary
//	error: Detailed error if binary cannot be located
//
// The job-init binary is essential for job execution as it handles
// the process setup within the isolated environment.
func (js *JobStarter) getJobInitPath() (string, error) {
	js.logger.Debug("searching for job-init binary")

	// Strategy 1: Check same directory as main executable
	// This is the most common deployment scenario
	execPath, err := js.osInterface.Executable()
	if err != nil {
		js.logger.Warn("failed to determine executable path", "error", err)
	} else {
		// Look for job-init in same directory as worker
		initPath := filepath.Join(filepath.Dir(execPath), "job-init")
		js.logger.Debug("checking job-init in executable directory", "initPath", initPath)

		if _, err := js.osInterface.Stat(initPath); err == nil {
			js.logger.Info("job-init found in executable directory", "path", initPath)
			return initPath, nil
		}
		js.logger.Debug("job-init not found in executable directory")
	}

	// Strategy 2: Check standard system paths
	// This handles system-wide installations
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

	// All search strategies failed
	js.logger.Error("job-init binary not found in any searched location",
		"searchedPaths", append([]string{"executable_directory"}, standardPaths...))
	return "", fmt.Errorf("job-init binary not found in executable directory or standard paths")
}

// prepareJobEnvironment builds the complete environment for job execution.
// This method combines system environment, job-specific variables, and
// network configuration into a complete environment setup.
//
// Environment Categories:
//  1. Job identification and metadata
//  2. Command and argument information
//  3. Resource and cgroup configuration
//  4. Network group configuration
//  5. System environment inheritance
//
// Parameters:
//
//	job:              Job domain object with metadata
//	networkGroupInfo: Network configuration details
//
// Returns: Complete environment variable array ready for process execution
//
// The environment provides job-init with all information needed to
// properly configure the job execution environment.
func (js *JobStarter) prepareJobEnvironment(job *domain.Job, networkGroupInfo *network.GroupInfo) []string {
	// Phase 1: Build Job-Specific Environment
	// Create variables specific to this job
	jobEnvVars := js.processLauncher.BuildJobEnvironment(
		job.Id,         // Job identification
		job.Command,    // Target command
		job.CgroupPath, // Cgroup location
		job.Args,       // Command arguments
		nil,            // Network vars added separately
	)

	js.logger.Debug("job-specific environment variables created",
		"count", len(jobEnvVars))

	// Phase 2: Add Network Environment
	// Include network group configuration if applicable
	if job.NetworkGroupID != "" {
		networkEnvVars := js.networkManager.PrepareEnvironment(
			job.NetworkGroupID,
			networkGroupInfo.IsNewGroup,
		)
		jobEnvVars = append(jobEnvVars, networkEnvVars...)

		js.logger.Debug("network environment variables added",
			"networkGroup", job.NetworkGroupID,
			"isNewGroup", networkGroupInfo.IsNewGroup,
			"additionalVars", len(networkEnvVars))
	}

	// Phase 3: Combine with System Environment
	// Merge job environment with inherited system environment
	baseEnv := js.osInterface.Environ()
	completeEnv := js.processLauncher.PrepareEnvironment(baseEnv, jobEnvVars)

	js.logger.Debug("complete environment prepared",
		"baseEnvVars", len(baseEnv),
		"jobEnvVars", len(jobEnvVars),
		"totalEnvVars", len(completeEnv))

	return completeEnv
}

// handleNamespaceCreation creates filesystem artifacts for namespace access.
// This method creates the necessary filesystem references (symlinks or bind mounts)
// that allow access to the job's network namespace.
//
// Namespace Artifact Types:
//  1. Symlinks: For isolated jobs (temporary, auto-cleanup)
//  2. Bind mounts: For network groups (persistent, manual cleanup)
//
// Parameters:
//
//	pid:              Process ID for namespace reference
//	networkGroupInfo: Network configuration details
//	jobID:            Job identifier for logging
//
// Returns: nil on success, error on failure
//
// This step is critical for enabling other processes to join the same
// network namespace if needed (e.g., for network group functionality).
func (js *JobStarter) handleNamespaceCreation(pid int32, networkGroupInfo *network.GroupInfo, jobID string) error {
	// Only create artifacts for new namespaces
	// Existing namespaces already have the necessary artifacts
	if !networkGroupInfo.IsNewGroup {
		js.logger.Debug("skipping namespace artifact creation for existing group")
		return nil
	}

	jobLogger := js.logger.WithField("jobId", jobID)
	jobLogger.Debug("creating namespace artifacts", "pid", pid)

	// Phase 1: Ensure Directory Structure
	// Create parent directories for namespace artifacts
	if err := js.ensureNamespaceDir(networkGroupInfo.NamePath); err != nil {
		return fmt.Errorf("failed to ensure namespace directory: %w", err)
	}

	// Phase 2: Create Appropriate Artifact Type
	// Choose between symlink (isolated) or bind mount (group)
	if jobID != "" && networkGroupInfo.NamePath != "" {
		isNetworkGroup := js.isNetworkGroupPath(networkGroupInfo.NamePath)

		// Delegate to network manager for consistent artifact creation
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

// isNetworkGroupPath determines if a namespace path is for a network group.
// This method distinguishes between persistent (network group) and temporary
// (isolated job) namespace paths based on their location.
//
// Path Classification:
//   - /var/run/netns/* → Network group (persistent)
//   - /tmp/shared-netns/* → Isolated job (temporary)
//
// Parameters:
//
//	nsPath: Namespace path to classify
//
// Returns: true if path is for network group, false for isolated job
func (js *JobStarter) isNetworkGroupPath(nsPath string) bool {
	// Network groups use the standard Linux netns location
	return filepath.HasPrefix(nsPath, js.config.VarRunNetns)
}

// ensureNamespaceDir creates parent directories for namespace artifacts.
// This method ensures the filesystem structure exists for namespace
// file creation.
//
// Parameters:
//
//	nsPath: Full path to namespace file (directory will be created)
//
// Returns: nil on success, error if directory creation fails
func (js *JobStarter) ensureNamespaceDir(nsPath string) error {
	dir := filepath.Dir(nsPath)

	// Check if directory already exists
	if _, err := js.osInterface.Stat(dir); err != nil {
		if js.osInterface.IsNotExist(err) {
			// Create directory with appropriate permissions
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

// Advanced Methods for Testing and Monitoring

// ValidateJobStartRequest validates a job start request thoroughly.
// This method provides comprehensive validation for external callers
// without creating a job.
//
// Parameters:
//
//	req: Job start request to validate
//
// Returns: nil if request is valid, detailed error describing issues
//
// This method is useful for:
// - Pre-flight validation before job creation
// - API request validation
// - Testing and development
func (js *JobStarter) ValidateJobStartRequest(req *StartJobRequest) error {
	if req == nil {
		return fmt.Errorf("start request cannot be nil")
	}

	// Create a process launch request for comprehensive validation
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

// EstimateResourceUsage estimates resource requirements for a job.
// This method helps with capacity planning and resource allocation decisions.
//
// Parameters:
//
//	req: Job start request to estimate
//
// Returns: Map containing resource usage estimates
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

// GetStartJobMetrics returns metrics about job starting operations.
// This method provides operational metrics for monitoring and debugging.
//
// Returns: Map containing job starter metrics and configuration
func (js *JobStarter) GetStartJobMetrics() map[string]interface{} {
	return map[string]interface{}{
		"component":            "job-starter",
		"default_cpu_limit":    config.DefaultCPULimitPercent,
		"default_memory_limit": config.DefaultMemoryLimitMB,
		"default_io_limit":     config.DefaultIOBPS,
	}
}

// PreflightCheck performs comprehensive pre-flight checks before job creation.
// This method verifies system readiness for job execution without creating
// any resources.
//
// Preflight Checks:
//  1. Command existence and accessibility
//  2. job-init binary availability
//  3. Network group validation (if specified)
//  4. System resource availability
//  5. Configuration validity
//
// Parameters:
//
//	req: Job start request to check
//
// Returns: nil if system is ready, descriptive error if issues found
//
// This method is useful for:
// - System health verification
// - Capacity planning
// - Troubleshooting job creation issues
func (js *JobStarter) PreflightCheck(req *StartJobRequest) error {
	js.logger.Debug("performing comprehensive preflight check", "command", req.Command)

	// Check 1: Command Validation and Resolution
	if _, err := js.worker.ValidateAndResolveCommand(req.Command); err != nil {
		return fmt.Errorf("preflight check failed - command issue: %w", err)
	}
	js.logger.Debug("command validation passed")

	// Check 2: job-init Binary Availability
	if _, err := js.getJobInitPath(); err != nil {
		return fmt.Errorf("preflight check failed - job-init not found: %w", err)
	}
	js.logger.Debug("job-init binary availability confirmed")

	// Check 3: Network Group Validation
	if req.NetworkGroupID != "" {
		if err := js.networkManager.ValidateGroupExists(req.NetworkGroupID); err != nil {
			// Group doesn't exist yet, which is acceptable for new groups
			js.logger.Debug("network group doesn't exist yet, will be created",
				"groupID", req.NetworkGroupID)
		} else {
			js.logger.Debug("network group exists and is accessible",
				"groupID", req.NetworkGroupID)
		}
	}

	// Check 4: System Resource Availability
	// This could be enhanced with actual system resource checking
	// For now, we perform basic configuration validation
	if err := js.config.Validate(); err != nil {
		return fmt.Errorf("preflight check failed - configuration invalid: %w", err)
	}
	js.logger.Debug("configuration validation passed")

	js.logger.Debug("preflight check completed successfully")
	return nil
}

// BuildJobEnvironmentPreview builds a preview of job environment variables.
// This method provides insight into the environment that will be created
// for a job without actually creating the job.
//
// Parameters:
//
//	jobID:          Job identifier
//	command:        Command to execute
//	cgroupPath:     Cgroup path for the job
//	args:           Command arguments
//	networkGroupID: Network group identifier
//
// Returns: Map of environment variable names to values
//
// This method is useful for:
// - Debugging environment issues
// - Documentation and training
// - Testing environment configuration
func (js *JobStarter) BuildJobEnvironmentPreview(jobID, command, cgroupPath string, args []string, networkGroupID string) map[string]string {
	envMap := make(map[string]string)

	// Basic job environment variables
	envMap["JOB_ID"] = jobID
	envMap["JOB_COMMAND"] = command
	envMap["JOB_CGROUP_PATH"] = cgroupPath
	envMap["JOB_ARGS_COUNT"] = fmt.Sprintf("%d", len(args))

	// Individual argument variables
	for i, arg := range args {
		envMap[fmt.Sprintf("JOB_ARG_%d", i)] = arg
	}

	// Network environment if applicable
	if networkGroupID != "" {
		envMap["NETWORK_GROUP_ID"] = networkGroupID

		// Try to get network configuration if group exists
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
