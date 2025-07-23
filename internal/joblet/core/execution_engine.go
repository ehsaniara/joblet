package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"joblet/internal/joblet/core/environment"
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
) *ExecutionEngine {
	envBuilder := environment.NewBuilder(platform, uploadManager, logger)

	return &ExecutionEngine{
		processManager: processManager,
		uploadManager:  uploadManager,
		envBuilder:     envBuilder,
		platform:       platform,
		store:          store,
		config:         config,
		logger:         logger.WithField("component", "execution-engine"),
		jobIsolation:   jobIsolation,
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

// StartProcess starts a job process with the given options
func (ee *ExecutionEngine) StartProcess(ctx context.Context, opts *StartProcessOptions) (platform.Command, error) {
	log := ee.logger.WithField("jobID", opts.Job.Id)

	// Get executable path
	execPath, err := ee.platform.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get executable path: %w", err)
	}

	// Create job base directory (simple directory, not full filesystem isolation)
	jobDir := filepath.Join(ee.config.Filesystem.BaseDir, opts.Job.Id)
	if err := ee.platform.MkdirAll(filepath.Join(jobDir, "sbin"), 0755); err != nil {
		return nil, fmt.Errorf("failed to create job directory: %w", err)
	}

	// Copy joblet binary to job directory with generic name
	isolatedInitPath := filepath.Join(jobDir, "sbin", "init")
	if err := ee.copyInitBinary(execPath, isolatedInitPath); err != nil {
		return nil, fmt.Errorf("failed to prepare init binary: %w", err)
	}

	// Handle pre-processing of uploads if needed
	if opts.PreProcessUploads && len(opts.Uploads) > 0 {
		if err := ee.preProcessUploads(ctx, opts); err != nil {
			return nil, fmt.Errorf("failed to pre-process uploads: %w", err)
		}
	}

	// Build environment with generic path
	envConfig := &environment.JobEnvironmentConfig{
		Job:         opts.Job,
		ExecutePath: "/sbin/init", // Generic path that will be visible after chroot
		Uploads:     opts.Uploads,
	}

	env, streamCtx := ee.envBuilder.BuildJobEnvironment(envConfig)

	// Add flag to indicate exec should be used
	env = append(env, "JOBLET_EXEC_AFTER_ISOLATION=true")

	// Start streaming if needed
	if streamCtx != nil && opts.EnableStreaming {
		streamCtx.SetManager(ee.uploadManager)
		if err := streamCtx.StartStreaming(); err != nil {
			log.Error("failed to start upload streaming", "error", err)
		}
	}

	// Create launch configuration
	launchConfig := &process.LaunchConfig{
		InitPath:    isolatedInitPath, // Use the copied binary
		Environment: env,
		SysProcAttr: ee.createIsolatedSysProcAttr(),
		Stdout:      NewWrite(ee.store, opts.Job.Id),
		Stderr:      NewWrite(ee.store, opts.Job.Id),
		JobID:       opts.Job.Id,
		Command:     opts.Job.Command,
		Args:        opts.Job.Args,
	}

	// Launch the process
	result, err := ee.processManager.LaunchProcess(ctx, launchConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to launch process: %w", err)
	}

	log.Debug("process launched successfully",
		"pid", result.PID,
		"isolatedPath", isolatedInitPath)

	return result.Command, nil
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

// preProcessUploads handles upload processing before job execution (for scheduled jobs)
func (ee *ExecutionEngine) preProcessUploads(ctx context.Context, opts *StartProcessOptions) error {
	if opts.WorkspaceDir == "" {
		opts.WorkspaceDir = filepath.Join(ee.config.Filesystem.BaseDir, opts.Job.Id, "work")
	}

	// Ensure workspace exists
	if err := ee.platform.MkdirAll(opts.WorkspaceDir, 0755); err != nil {
		return fmt.Errorf("failed to create workspace: %w", err)
	}

	// Process uploads directly
	streamConfig := &upload.StreamConfig{
		JobID:        opts.Job.Id,
		Uploads:      opts.Uploads,
		MemoryLimit:  opts.Job.Limits.MaxMemory,
		WorkspaceDir: opts.WorkspaceDir,
	}

	return ee.uploadManager.ProcessDirectUploads(ctx, streamConfig)
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

	// Create and execute command
	cmd := ee.platform.CreateCommand(commandPath, config.Args...)

	// Set standard streams to OS streams
	// In init mode, we want the command to inherit the current process's streams
	cmd.SetStdout(os.Stdout)
	cmd.SetStderr(os.Stderr)
	cmd.SetStdin(os.Stdin)

	// Change to workspace if uploads were processed
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
