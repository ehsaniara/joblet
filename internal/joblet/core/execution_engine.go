//go:build linux

package core

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"joblet/internal/joblet/core/environment"
	"joblet/internal/joblet/network"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"joblet/internal/joblet/adapters"
	"joblet/internal/joblet/core/process"
	"joblet/internal/joblet/core/unprivileged"
	"joblet/internal/joblet/core/upload"
	"joblet/internal/joblet/domain"
	"joblet/pkg/config"
	"joblet/pkg/logger"
	"joblet/pkg/platform"
)

// ExecutionEngine handles job execution logic with consolidated environment management.
// This engine provides comprehensive job execution capabilities including:
//   - Process isolation using namespaces and cgroups
//   - Two-phase execution for upload processing and job execution
//   - Network setup and allocation management
//   - Resource limit enforcement and monitoring
//   - CI-compatible lightweight execution mode
//
// The engine supports both full isolation mode (production) and CI mode (development/testing).
// Thread-safe for concurrent job execution.
type ExecutionEngine struct {
	processManager *process.Manager
	uploadManager  *upload.Manager
	envBuilder     *environment.Builder
	platform       platform.Platform
	store          adapters.JobStoreAdapter
	config         *config.Config
	logger         *logger.Logger
	jobIsolation   *unprivileged.JobIsolation
	networkSetup   *network.NetworkSetup
	networkStore   adapters.NetworkStoreAdapter
}

// NewExecutionEngine creates a new execution engine with all required dependencies.
// Initializes the engine with process management, upload handling, network setup,
// and isolation capabilities. Returns a fully configured engine ready for job execution.
//
// Parameters:
//   - processManager: Handles process lifecycle and execution
//   - uploadManager: Manages file uploads and workspace preparation
//   - platform: Platform abstraction for system operations
//   - store: Job storage adapter for state management and logging
//   - config: Configuration settings for execution behavior
//   - logger: Structured logging interface
//   - jobIsolation: Provides security isolation capabilities
//   - networkStore: Network management and allocation storage
//
// Returns: Configured ExecutionEngine instance ready for job execution
func NewExecutionEngine(
	processManager *process.Manager,
	uploadManager *upload.Manager,
	platform platform.Platform,
	store adapters.JobStoreAdapter,
	config *config.Config,
	logger *logger.Logger,
	jobIsolation *unprivileged.JobIsolation,
	networkStore adapters.NetworkStoreAdapter,
) *ExecutionEngine {
	// Create environment builder with the correct parameters
	envBuilder := environment.NewBuilder(platform, uploadManager, logger)

	var networkSetup *network.NetworkSetup
	if networkStore != nil {
		// Create bridge to adapt NetworkStoreAdapter to NetworkStoreInterface
		networkStoreInterface := adapters.NewNetworkSetupBridge(networkStore)
		networkSetup = network.NewNetworkSetup(platform, networkStoreInterface)
	}

	return &ExecutionEngine{
		processManager: processManager,
		uploadManager:  uploadManager,
		envBuilder:     envBuilder,
		platform:       platform,
		store:          store,
		config:         config,
		logger:         logger.WithField("component", "execution-engine"),
		jobIsolation:   jobIsolation,
		networkSetup:   networkSetup,
		networkStore:   networkStore,
	}
}

// StartProcessOptions contains options for starting a process
type StartProcessOptions struct {
	Job               *domain.Job
	Uploads           []domain.FileUpload
	EnableStreaming   bool
	WorkspaceDir      string
	PreProcessUploads bool // For scheduled jobs that need uploads processed beforehand
}

// StartProcess starts a job process with proper isolation and phased execution.
// Implements a two-phase execution model:
//  1. Upload phase: Processes file uploads within resource constraints
//  2. Execution phase: Runs the actual job command with full isolation
//
// Supports both full isolation mode (production) and CI mode (development/testing).
// Handles network setup, resource limits, and proper cleanup on failures.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control
//   - opts: Process options including job details, uploads, and configuration
//
// Returns: Command interface for process control, or error if startup fails
func (ee *ExecutionEngine) StartProcess(ctx context.Context, opts *StartProcessOptions) (platform.Command, error) {
	log := ee.logger.WithField("jobID", opts.Job.Id)
	log.Debug("starting job process", "hasUploads", len(opts.Uploads) > 0)

	// Check if we're in CI mode - if so, use lightweight isolation
	if ee.platform.Getenv("JOBLET_CI_MODE") == "true" {
		return ee.executeCICommand(ctx, opts)
	}

	// Get executable path
	execPath, err := ee.platform.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get executable path: %w", err)
	}

	// Create job base directory
	jobDir := filepath.Join(ee.config.Filesystem.BaseDir, opts.Job.Id)
	if e := ee.platform.MkdirAll(filepath.Join(jobDir, "sbin"), 0755); e != nil {
		return nil, fmt.Errorf("failed to create job directory: %w", e)
	}

	// Copy joblet binary to job directory
	isolatedInitPath := filepath.Join(jobDir, "sbin", "init")
	if e := ee.copyInitBinary(execPath, isolatedInitPath); e != nil {
		return nil, fmt.Errorf("failed to prepare init binary: %w", e)
	}

	// CHANGED: Use two-phase execution for uploads
	if len(opts.Uploads) > 0 {
		log.Debug("executing two-phase job with uploads")

		// Phase 1: Upload processing within isolation
		if err := ee.executeUploadPhase(ctx, opts, isolatedInitPath); err != nil {
			// we Don't cleanup here - let the caller handle it
			return nil, fmt.Errorf("upload phase failed: %w", err)
		}

		log.Debug("upload phase completed successfully")
	}

	// Phase 2: Job execution (with or without uploads)
	return ee.executeJobPhase(ctx, opts, isolatedInitPath)
}

// executeUploadPhase runs the upload phase in full isolation.
// Processes file uploads within cgroup resource limits to prevent resource exhaustion.
// Uses base64 encoding to safely pass upload data through environment variables.
// Runs with full namespace isolation for security.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control
//   - opts: Process options containing uploads to process
//   - initPath: Path to the isolated init binary
//
// Returns: Error if upload processing fails, nil on success
func (ee *ExecutionEngine) executeUploadPhase(ctx context.Context, opts *StartProcessOptions, initPath string) error {
	log := ee.logger.WithField("jobID", opts.Job.Id).WithField("phase", "upload")

	// Serialize uploads to pass via environment
	uploadsJSON, err := json.Marshal(opts.Uploads)
	if err != nil {
		return fmt.Errorf("failed to serialize uploads: %w", err)
	}

	// Encode to base64 to avoid issues with special characters
	uploadsB64 := base64.StdEncoding.EncodeToString(uploadsJSON)

	// Build environment for upload phase
	env := ee.buildPhaseEnvironment(opts.Job, "upload")
	env = append(env, fmt.Sprintf("JOB_UPLOADS_DATA=%s", uploadsB64))
	env = append(env, fmt.Sprintf("JOB_UPLOADS_COUNT=%d", len(opts.Uploads)))

	// Create output writer for upload phase logs
	uploadOutput := NewWrite(ee.store, opts.Job.Id)

	// Launch upload phase process with full isolation
	launchConfig := &process.LaunchConfig{
		InitPath:    initPath,
		Environment: env,
		SysProcAttr: ee.createIsolatedSysProcAttr(), // Full isolation!
		Stdout:      uploadOutput,
		Stderr:      uploadOutput,
		JobID:       opts.Job.Id,
		Command:     "upload-phase", // Internal marker
		Args:        []string{},
	}

	result, err := ee.processManager.LaunchProcess(ctx, launchConfig)
	if err != nil {
		return fmt.Errorf("failed to launch upload phase: %w", err)
	}

	// Wait for upload phase to complete
	cmd := result.Command

	// Create a channel to wait for process completion
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	// Wait with timeout
	select {
	case e := <-done:
		if e != nil {
			var exitError *exec.ExitError
			if errors.As(e, &exitError) {
				log.Error("upload phase failed",
					"exitCode", exitError.ExitCode(),
					"error", e)
				return fmt.Errorf("upload phase exited with code %d", exitError.ExitCode())
			}
			return fmt.Errorf("upload phase failed: %w", e)
		}
		return nil // Success

	case <-ctx.Done():
		cmd.Kill()
		return ctx.Err()

	case <-time.After(ee.config.Buffers.DefaultConfig.UploadTimeout): // Upload timeout from config
		cmd.Kill()
		return fmt.Errorf("upload phase timeout")
	}
}

// executeJobPhase runs the main job execution phase.
// Launches the actual job command with full isolation, network setup, and resource limits.
// Handles network allocation, synchronization with child process, and proper cleanup.
// Returns the running command for monitoring and control.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control
//   - opts: Process options containing job command and configuration
//   - initPath: Path to the isolated init binary
//
// Returns: Running command interface, or error if execution setup fails
func (ee *ExecutionEngine) executeJobPhase(ctx context.Context, opts *StartProcessOptions, initPath string) (platform.Command, error) {
	log := ee.logger.WithField("jobID", opts.Job.Id).WithField("phase", "execute")

	// Build environment for execution phase
	env := ee.buildPhaseEnvironment(opts.Job, "execute")

	// Add command and args to environment
	env = append(env, fmt.Sprintf("JOB_COMMAND=%s", opts.Job.Command))
	env = append(env, fmt.Sprintf("JOB_ARGS_COUNT=%d", len(opts.Job.Args)))
	for i, arg := range opts.Job.Args {
		env = append(env, fmt.Sprintf("JOB_ARG_%d=%s", i, arg))
	}

	// Indicate if uploads were processed
	env = append(env, fmt.Sprintf("JOB_HAS_UPLOADS=%t", len(opts.Uploads) > 0))

	// Setup network synchronization if needed
	var networkReadyR, networkReadyW *os.File
	var extraFiles []*os.File

	if ee.networkStore != nil && opts.Job.Network != "" {
		r, w, err := os.Pipe()
		if err != nil {
			return nil, fmt.Errorf("failed to create network sync pipe: %w", err)
		}
		networkReadyR, networkReadyW = r, w
		env = append(env, "NETWORK_READY_FD=3")
		extraFiles = []*os.File{networkReadyR}
	}

	// Create output writer
	outputWriter := NewWrite(ee.store, opts.Job.Id)

	// Launch execution phase process
	launchConfig := &process.LaunchConfig{
		InitPath:    initPath,
		Environment: env,
		SysProcAttr: ee.createIsolatedSysProcAttr(),
		Stdout:      outputWriter,
		Stderr:      outputWriter,
		JobID:       opts.Job.Id,
		Command:     opts.Job.Command,
		Args:        opts.Job.Args,
		ExtraFiles:  extraFiles,
	}

	result, err := ee.processManager.LaunchProcess(ctx, launchConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to launch execution phase: %w", err)
	}

	// Handle network setup after process launch
	if networkReadyW != nil {
		defer networkReadyW.Close()

		// Handle network allocation and setup
		var alloc *network.JobAllocation
		var allocErr error

		if opts.Job.Network == "isolated" {
			// Create minimal allocation for isolated network
			alloc = &network.JobAllocation{
				JobID:   opts.Job.Id,
				Network: "isolated",
			}
		} else if ee.networkStore != nil {
			// Regular network allocation using adapter
			// First allocate IP address from the network
			ipAddress, ipErr := ee.networkStore.AllocateIP(opts.Job.Network)
			if ipErr != nil {
				result.Command.Kill()
				return nil, fmt.Errorf("failed to allocate IP: %w", ipErr)
			}

			hostname := fmt.Sprintf("job_%s", opts.Job.Id)
			adapterAlloc := &adapters.JobNetworkAllocation{
				JobID:       opts.Job.Id,
				NetworkName: opts.Job.Network,
				IPAddress:   ipAddress,
				Hostname:    hostname,
				AssignedAt:  time.Now().Unix(),
			}
			allocErr = ee.networkStore.AssignJobToNetwork(opts.Job.Id, opts.Job.Network, adapterAlloc)
			if allocErr != nil {
				// Release the allocated IP on error
				if releaseErr := ee.networkStore.ReleaseIP(opts.Job.Network, ipAddress); releaseErr != nil {
					log.Warn("failed to release IP during cleanup", "ip", ipAddress, "error", releaseErr)
				}
				result.Command.Kill()
				return nil, fmt.Errorf("failed to assign network: %w", allocErr)
			}

			// Convert adapter allocation to network allocation for consistency
			// Generate veth names based on PID for bridge networks
			pid := int(result.PID)
			ip := net.ParseIP(ipAddress)
			if ip == nil {
				result.Command.Kill()
				// Release the allocated IP on error
				if releaseErr := ee.networkStore.ReleaseIP(opts.Job.Network, ipAddress); releaseErr != nil {
					log.Warn("failed to release IP during cleanup", "ip", ipAddress, "error", releaseErr)
				}
				return nil, fmt.Errorf("failed to parse IP address '%s'", ipAddress)
			}
			alloc = &network.JobAllocation{
				JobID:    adapterAlloc.JobID,
				Network:  adapterAlloc.NetworkName,
				IP:       ip,
				Hostname: adapterAlloc.Hostname,
				VethHost: fmt.Sprintf("vjob%d", pid%10000),
				VethPeer: fmt.Sprintf("vjob%dp", pid%10000),
			}
		} else {
			// No network store available - create basic allocation
			// Generate veth names based on PID for consistency
			pid := int(result.PID)
			alloc = &network.JobAllocation{
				JobID:    opts.Job.Id,
				Network:  opts.Job.Network,
				VethHost: fmt.Sprintf("vjob%d", pid%10000),
				VethPeer: fmt.Sprintf("vjob%dp", pid%10000),
			}
		}

		// Setup network in namespace
		if ee.networkSetup != nil {
			if setupErr := ee.networkSetup.SetupJobNetwork(alloc, int(result.PID)); setupErr != nil {
				if opts.Job.Network != "isolated" && ee.networkStore != nil {
					if removeErr := ee.networkStore.RemoveJobFromNetwork(opts.Job.Id); removeErr != nil {
						// Log the error but continue with cleanup
						ee.logger.Warn("failed to remove job from network during cleanup",
							"JobId", opts.Job.Id,
							"error", removeErr)
					}
				}
				result.Command.Kill()
				return nil, fmt.Errorf("failed to setup network: %w", setupErr)
			}
		}

		// Signal that network is ready - INLINE instead of calling setupJobNetwork
		if _, writeErr := networkReadyW.Write([]byte{1}); writeErr != nil {
			log.Warn("failed to signal network ready", "error", writeErr)
			// Don't fail the job for this - the process might still work
		}
	}

	// Close read end in parent
	if networkReadyR != nil {
		networkReadyR.Close()
	}

	log.Debug("execution phase launched successfully", "pid", result.PID)
	return result.Command, nil
}

// buildPhaseEnvironment builds common environment variables for both execution phases.
// Creates environment with job metadata, resource limits, cgroup paths, and volume information.
// Combines base environment with job-specific variables for isolation setup.
//
// Parameters:
//   - job: Job domain object containing limits and configuration
//   - phase: Execution phase ("upload" or "execute")
//
// Returns: Complete environment variable slice for process execution
func (ee *ExecutionEngine) buildPhaseEnvironment(job *domain.Job, phase string) []string {
	baseEnv := ee.platform.Environ()

	jobEnv := []string{
		"JOBLET_MODE=init",
		fmt.Sprintf("JOB_PHASE=%s", phase),
		fmt.Sprintf("JOB_ID=%s", job.Id),
		fmt.Sprintf("JOB_CGROUP_PATH=%s", "/sys/fs/cgroup"),
		fmt.Sprintf("JOB_CGROUP_HOST_PATH=%s", job.CgroupPath),
		fmt.Sprintf("JOB_MAX_CPU=%d", job.Limits.CPU.Value()),
		fmt.Sprintf("JOB_MAX_MEMORY=%d", job.Limits.Memory.Megabytes()),
		fmt.Sprintf("JOB_MAX_IOBPS=%d", job.Limits.IOBandwidth.BytesPerSecond()),
	}

	if !job.Limits.CPUCores.IsEmpty() {
		jobEnv = append(jobEnv, fmt.Sprintf("JOB_CPU_CORES=%s", job.Limits.CPUCores.String()))
	}

	// Add volume information
	if len(job.Volumes) > 0 {
		jobEnv = append(jobEnv, fmt.Sprintf("JOB_VOLUMES_COUNT=%d", len(job.Volumes)))
		for i, volume := range job.Volumes {
			jobEnv = append(jobEnv, fmt.Sprintf("JOB_VOLUME_%d=%s", i, volume))
		}
	}

	return append(baseEnv, jobEnv...)
}

// copyInitBinary copies the joblet binary to the job's isolated directory.
// Ensures the init binary is available within the job's filesystem namespace.
// Creates necessary directories and sets proper execute permissions.
//
// Parameters:
//   - source: Path to the original joblet binary
//   - dest: Destination path within the job directory
//
// Returns: Error if copy operation fails, nil on success
func (ee *ExecutionEngine) copyInitBinary(source, dest string) error {
	// Ensure destination directory exists
	destDir := filepath.Dir(dest)
	if err := ee.platform.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	// Copy the binary
	input, err := ee.platform.ReadFile(source)
	if err != nil {
		return err
	}

	// Write with execute permissions
	return ee.platform.WriteFile(dest, input, 0755)
}

// StartProcessWithUploads starts a job process with upload support (compatibility method).
// Provides backward compatibility for existing code that uses the simpler interface.
// Creates StartProcessOptions internally and delegates to StartProcess.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control
//   - job: Job domain object containing command and configuration
//   - uploads: File uploads to process before job execution
//
// Returns: Command interface for process control, or error if startup fails
func (ee *ExecutionEngine) StartProcessWithUploads(ctx context.Context, job *domain.Job, uploads []domain.FileUpload) (platform.Command, error) {
	opts := &StartProcessOptions{
		Job:             job,
		Uploads:         uploads,
		EnableStreaming: true,
		WorkspaceDir:    filepath.Join(ee.config.Filesystem.BaseDir, job.Id, "work"),
	}
	return ee.StartProcess(ctx, opts)
}

// createIsolatedSysProcAttr creates system process attributes for isolation.
// Delegates to the job isolation component to create appropriate namespace
// and security attributes for process isolation.
//
// Returns: System process attributes configured for isolation
func (ee *ExecutionEngine) createIsolatedSysProcAttr() *syscall.SysProcAttr {
	return ee.jobIsolation.CreateIsolatedSysProcAttr()
}

// ExecuteInitMode executes a job in init mode (inside the isolated environment).
// Called when the process is running as PID 1 inside an isolated namespace.
// Loads job configuration from environment, processes uploads if present,
// and executes the actual job command using exec to replace the init process.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control
//
// Returns: Error if initialization or execution fails
func (ee *ExecutionEngine) ExecuteInitMode(ctx context.Context) error {
	// Load configuration from environment
	config, err := ee.envBuilder.LoadJobConfigFromEnvironment()
	if err != nil {
		return fmt.Errorf("failed to load job config: %w", err)
	}

	log := ee.logger.WithField("jobID", config.JobID)
	log.Debug("executing in init mode", "command", config.Command)

	// Process uploads if present
	if config.HasUploadSession && config.UploadPipePath != "" {
		workspaceDir := ee.config.Filesystem.WorkspaceDir
		if workspaceDir == "" {
			return fmt.Errorf("workspace directory not configured")
		}
		receiver := upload.NewReceiver(ee.platform, ee.logger)

		if err := ee.platform.MkdirAll(workspaceDir, 0755); err != nil {
			return fmt.Errorf("failed to create workspace: %w", err)
		}

		if err := receiver.ProcessAllFiles(config.UploadPipePath, workspaceDir); err != nil {
			log.Error("failed to process uploads", "error", err)
			// Continue execution even if upload processing fails
		}
	}

	// Execute the actual command
	return ee.executeCommand(config)
}

// executeCommand executes the actual job command using exec to replace the init process.
// Resolves the command path, changes to workspace directory if needed,
// and uses exec syscall to replace the current process with the job command.
// This makes the job command become PID 1 in the namespace for proper isolation.
//
// Parameters:
//   - config: Job configuration loaded from environment variables
//
// Returns: Error if command resolution or exec fails (exec should not return on success)
func (ee *ExecutionEngine) executeCommand(config *environment.JobConfig) error {
	// Resolve command path
	commandPath, err := ee.resolveCommandPath(config.Command)
	if err != nil {
		return fmt.Errorf("failed to resolve command: %w", err)
	}

	// Change to workspace if needed (safe to use os.Chdir since we're in isolated namespace)
	if config.HasUploadSession {
		workspaceDir := ee.config.Filesystem.WorkspaceDir
		if workspaceDir == "" {
			return fmt.Errorf("workspace directory not configured")
		}
		if err := os.Chdir(workspaceDir); err != nil {
			return fmt.Errorf("failed to change to workspace directory: %w", err)
		}
	}

	// Prepare arguments for exec - argv[0] should be the command name
	argv := append([]string{commandPath}, config.Args...)

	// Get current environment (already set up by parent process)
	envv := ee.platform.Environ()

	// Use exec to replace the current process (init) with the job command
	// This makes the job command become PID 1 in the namespace, providing proper isolation
	log := ee.logger.WithField("command", commandPath).WithField("args", config.Args)
	log.Debug("about to exec to replace init process")

	err = ee.platform.Exec(commandPath, argv, envv)
	// If we reach this point, exec failed
	log.Error("exec failed - job will not appear as PID 1", "error", err)
	return fmt.Errorf("exec failed: %w", err)
}

// resolveCommandPath resolves the full path for a command.
// Checks if command is already absolute, searches PATH, and tries common locations.
// Ensures the command exists and is executable before returning the path.
//
// Parameters:
//   - command: Command name or path to resolve
//
// Returns: Full path to executable command, or error if not found
func (ee *ExecutionEngine) resolveCommandPath(command string) (string, error) {
	// Check if it's already an absolute path
	if filepath.IsAbs(command) {
		return command, nil
	}

	// Try to find in PATH
	if path, err := ee.platform.LookPath(command); err == nil {
		return path, nil
	}

	// Try common locations
	commonPaths := []string{
		filepath.Join("/bin", command),
		filepath.Join("/usr/bin", command),
		filepath.Join("/usr/local/bin", command),
		filepath.Join("/sbin", command),
		filepath.Join("/usr/sbin", command),
	}

	for _, path := range commonPaths {
		if _, err := ee.platform.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("command %s not found", command)
}

// executeCICommand executes jobs with lightweight isolation in CI mode.
// Provides basic security isolation while maintaining CI compatibility.
// Processes uploads directly without full namespace isolation,
// uses process groups for basic isolation, and sets minimal environment.
// Designed to work in containerized CI environments with limited privileges.
//
// Parameters:
//   - ctx: Context (unused but maintained for interface compatibility)
//   - opts: Process options containing job and upload configuration
//
// Returns: Command interface for CI-compatible job execution, or error if setup fails
func (ee *ExecutionEngine) executeCICommand(_ context.Context, opts *StartProcessOptions) (platform.Command, error) {
	log := ee.logger.WithField("jobID", opts.Job.Id).WithField("mode", "ci-isolated")

	// Create job directory for workspace
	jobDir := filepath.Join(ee.config.Filesystem.BaseDir, opts.Job.Id)
	workDir := filepath.Join(jobDir, "work")

	if err := ee.platform.MkdirAll(workDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create work directory: %w", err)
	}

	// Process uploads directly if any
	if len(opts.Uploads) > 0 {
		for _, upload := range opts.Uploads {
			uploadPath := filepath.Join(workDir, upload.Path)

			if upload.IsDirectory {
				if err := ee.platform.MkdirAll(uploadPath, os.FileMode(upload.Mode)); err != nil {
					return nil, fmt.Errorf("failed to create upload directory %s: %w", upload.Path, err)
				}
			} else {
				parentDir := filepath.Dir(uploadPath)
				if err := ee.platform.MkdirAll(parentDir, 0755); err != nil {
					return nil, fmt.Errorf("failed to create parent directory for %s: %w", upload.Path, err)
				}

				if err := ee.platform.WriteFile(uploadPath, upload.Content, os.FileMode(upload.Mode)); err != nil {
					return nil, fmt.Errorf("failed to write upload file %s: %w", upload.Path, err)
				}
			}
		}
	}

	// Resolve command path
	commandPath, err := ee.resolveCommandPath(opts.Job.Command)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve command: %w", err)
	}

	// Create command with lightweight isolation
	cmd := ee.platform.CreateCommand(commandPath, opts.Job.Args...)

	// Apply CI-safe isolation - use platform's process group creation
	// This provides basic process isolation without complex namespace setup
	ciSysProcAttr := ee.platform.CreateProcessGroup()
	cmd.SetSysProcAttr(ciSysProcAttr)

	// Set working directory to workspace if uploads were processed
	if len(opts.Uploads) > 0 {
		cmd.SetDir(workDir)
	}

	// Set up output capture
	outputWriter := NewWrite(ee.store, opts.Job.Id)
	cmd.SetStdout(outputWriter)
	cmd.SetStderr(outputWriter)

	// Set clean environment with minimal host information
	env := []string{
		"PATH=/usr/local/bin:/usr/bin:/bin",
		"HOME=/tmp",
		"USER=joblet",
		fmt.Sprintf("JOB_ID=%s", opts.Job.Id),
		"JOBLET_CI_MODE=true",
	}
	cmd.SetEnv(env)

	log.Debug("starting CI job with lightweight isolation",
		"command", commandPath,
		"isolation", "pid-namespace-only",
		"workspace", workDir)

	// Start the command
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start CI command: %w", err)
	}

	return cmd, nil
}
