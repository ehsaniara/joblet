package core

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"joblet/internal/joblet/core/environment"
	"joblet/internal/joblet/network"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"joblet/internal/joblet/core/process"
	"joblet/internal/joblet/core/unprivileged"
	"joblet/internal/joblet/core/upload"
	"joblet/internal/joblet/domain"
	"joblet/internal/joblet/state"
	"joblet/pkg/config"
	"joblet/pkg/logger"
	"joblet/pkg/platform"
)

// ExecutionEngine handles job execution logic with consolidated environment management
type ExecutionEngine struct {
	processManager *process.Manager
	uploadManager  *upload.Manager
	envBuilder     *environment.Builder
	platform       platform.Platform
	store          state.Store
	config         *config.Config
	logger         *logger.Logger
	jobIsolation   *unprivileged.JobIsolation
	networkSetup   *network.NetworkSetup
	networkStore   *state.NetworkStore
}

// NewExecutionEngine creates a new execution engine
func NewExecutionEngine(
	processManager *process.Manager,
	uploadManager *upload.Manager,
	platform platform.Platform,
	store state.Store,
	config *config.Config,
	logger *logger.Logger,
	jobIsolation *unprivileged.JobIsolation,
	networkStore *state.NetworkStore,
) *ExecutionEngine {
	uploadFactory := upload.NewFactory(logger)
	envBuilder := environment.NewBuilder(platform, uploadManager, uploadFactory, logger)

	return &ExecutionEngine{
		processManager: processManager,
		uploadManager:  uploadManager,
		envBuilder:     envBuilder,
		platform:       platform,
		store:          store,
		config:         config,
		logger:         logger.WithField("component", "execution-engine"),
		jobIsolation:   jobIsolation,
		networkSetup:   network.NewNetworkSetup(platform, networkStore),
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

// StartProcess starts a job process with proper isolation and phased execution
func (ee *ExecutionEngine) StartProcess(ctx context.Context, opts *StartProcessOptions) (platform.Command, error) {
	log := ee.logger.WithField("jobID", opts.Job.Id)
	log.Debug("starting job process", "hasUploads", len(opts.Uploads) > 0)

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
		log.Info("executing two-phase job with uploads")

		// Phase 1: Upload processing within isolation
		if err := ee.executeUploadPhase(ctx, opts, isolatedInitPath); err != nil {
			// we Don't cleanup here - let the caller handle it
			return nil, fmt.Errorf("upload phase failed: %w", err)
		}

		log.Info("upload phase completed successfully")
	}

	// Phase 2: Job execution (with or without uploads)
	return ee.executeJobPhase(ctx, opts, isolatedInitPath)
}

// executeUploadPhase runs the upload phase in full isolation
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

	case <-time.After(5 * time.Minute): // Upload timeout
		cmd.Kill()
		return fmt.Errorf("upload phase timeout")
	}
}

// executeJobPhase runs the main job execution phase
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
		} else {
			// Regular network allocation
			hostname := fmt.Sprintf("job_%s", opts.Job.Id)
			alloc, allocErr = ee.networkStore.AssignJobToNetwork(opts.Job.Id, opts.Job.Network, hostname)
			if allocErr != nil {
				result.Command.Kill()
				return nil, fmt.Errorf("failed to assign network: %w", allocErr)
			}
		}

		// Setup network in namespace
		if setupErr := ee.networkSetup.SetupJobNetwork(alloc, int(result.PID)); setupErr != nil {
			if opts.Job.Network != "isolated" {
				ee.networkStore.ReleaseJob(opts.Job.Id)
			}
			result.Command.Kill()
			return nil, fmt.Errorf("failed to setup network: %w", setupErr)
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

// buildPhaseEnvironment builds common environment for both phases
func (ee *ExecutionEngine) buildPhaseEnvironment(job *domain.Job, phase string) []string {
	baseEnv := ee.platform.Environ()

	jobEnv := []string{
		"JOBLET_MODE=init",
		fmt.Sprintf("JOB_PHASE=%s", phase),
		fmt.Sprintf("JOB_ID=%s", job.Id),
		fmt.Sprintf("JOB_CGROUP_PATH=%s", "/sys/fs/cgroup"),
		fmt.Sprintf("JOB_CGROUP_HOST_PATH=%s", job.CgroupPath),
		fmt.Sprintf("JOB_MAX_CPU=%d", job.Limits.MaxCPU),
		fmt.Sprintf("JOB_MAX_MEMORY=%d", job.Limits.MaxMemory),
		fmt.Sprintf("JOB_MAX_IOBPS=%d", job.Limits.MaxIOBPS),
	}

	if job.Limits.CPUCores != "" {
		jobEnv = append(jobEnv, fmt.Sprintf("JOB_CPU_CORES=%s", job.Limits.CPUCores))
	}

	return append(baseEnv, jobEnv...)
}

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

// StartProcessWithUploads starts a job process with upload support (compatibility method)
func (ee *ExecutionEngine) StartProcessWithUploads(ctx context.Context, job *domain.Job, uploads []domain.FileUpload) (platform.Command, error) {
	opts := &StartProcessOptions{
		Job:             job,
		Uploads:         uploads,
		EnableStreaming: true,
		WorkspaceDir:    filepath.Join(ee.config.Filesystem.BaseDir, job.Id, "work"),
	}
	return ee.StartProcess(ctx, opts)
}

// createIsolatedSysProcAttr creates system process attributes for isolation
func (ee *ExecutionEngine) createIsolatedSysProcAttr() *syscall.SysProcAttr {
	return ee.jobIsolation.CreateIsolatedSysProcAttr()
}

// ExecuteInitMode executes a job in init mode (inside the isolated environment)
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
		workspaceDir := "/work"
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

// executeCommand executes the actual job command
func (ee *ExecutionEngine) executeCommand(config *environment.JobConfig) error {
	// Resolve command path
	commandPath, err := ee.resolveCommandPath(config.Command)
	if err != nil {
		return fmt.Errorf("failed to resolve command: %w", err)
	}

	// Create command
	cmd := ee.platform.CreateCommand(commandPath, config.Args...)

	// DON'T set stdout/stderr - inherit from parent
	// The parent already set them to NewWrite(store, jobId)

	// Change to workspace if needed
	if config.HasUploadSession {
		cmd.SetDir("/work")
	}

	return cmd.Run()
}

// resolveCommandPath resolves the full path for a command
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
