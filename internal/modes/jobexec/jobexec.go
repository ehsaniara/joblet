//go:build linux

package jobexec

import (
	"context"
	"fmt"
	"github.com/ehsaniara/joblet/internal/joblet/core/environment"
	"github.com/ehsaniara/joblet/internal/joblet/core/upload"
	"github.com/ehsaniara/joblet/pkg/config"
	"github.com/ehsaniara/joblet/pkg/errors"
	"github.com/ehsaniara/joblet/pkg/logger"
	"github.com/ehsaniara/joblet/pkg/platform"
	"os"
	"path/filepath"
)

// JobExecutor handles job execution in init mode with consolidated environment handling
type JobExecutor struct {
	platform      platform.Platform
	logger        *logger.Logger
	envBuilder    *environment.Builder
	uploadManager *upload.Manager
	config        *config.Config
}

// NewJobExecutor creates a new job executor
func NewJobExecutor(platform platform.Platform, logger *logger.Logger, cfg *config.Config) *JobExecutor {
	// Create upload manager
	uploadManager := upload.NewManager(platform, logger)

	// Create environment builder with the correct parameters
	envBuilder := environment.NewBuilder(platform, uploadManager, logger)

	return &JobExecutor{
		platform:      platform,
		logger:        logger.WithField("component", "job-executor"),
		envBuilder:    envBuilder,
		uploadManager: uploadManager, // Store for direct access if needed
		config:        cfg,
	}
}

// ExecuteInInitMode executes a job in init mode
func (je *JobExecutor) ExecuteInInitMode() error {
	// Load job configuration from environment
	config, err := je.envBuilder.LoadJobConfigFromEnvironment()
	if err != nil {
		return errors.WrapConfigError("job", "config", err)
	}

	log := je.logger.WithField("jobID", config.JobID).
		WithField("totalFiles", config.TotalFiles)

	// Executing job with provided configuration

	// Process uploads if present
	if config.HasUploadSession && config.UploadPipePath != "" {
		if e := je.processUploads(config); e != nil {
			log.Error("failed to process uploads", "error", e)
			// Continue execution even if upload fails
		}
	}

	// Execute the command
	return je.executeCommand(config)
}

// Execute executes the job using consolidated environment handling
func Execute(logger *logger.Logger) error {
	p := platform.NewPlatform()
	// Load configuration - this is needed for workspace directory
	cfg, _, err := config.LoadConfig()
	if err != nil {
		return errors.WrapConfigError("joblet", "config", err)
	}
	executor := NewJobExecutor(p, logger, cfg)
	return executor.ExecuteJob()
}

func (je *JobExecutor) ExecuteJob() error {
	// Load configuration from environment
	config, err := je.envBuilder.LoadJobConfigFromEnvironment()
	if err != nil {
		return errors.WrapConfigError("job", "configuration", err)
	}

	// Check which phase we're in
	phase := je.platform.Getenv("JOB_PHASE")

	switch phase {
	case "upload":
		// Upload phase is handled in server.go
		return fmt.Errorf("%w: upload phase should be handled by server.go", errors.ErrInvalidConfig)

	case "execute", "":
		// Execute phase - just run the command
		// Executing job command

		return je.executeCommand(config)

	default:
		return errors.WrapConfigError("job", "phase", fmt.Errorf("unknown phase: %s", phase))
	}
}

// processUploads handles upload processing from the pipe
func (je *JobExecutor) processUploads(config *environment.JobConfig) error {
	workspaceDir := je.config.Filesystem.WorkspaceDir
	if workspaceDir == "" {
		return errors.WrapConfigError("job", "workspace", fmt.Errorf("directory not configured"))
	}
	// Processing uploads from pipe

	// Create workspace
	if err := je.platform.MkdirAll(workspaceDir, 0755); err != nil {
		return errors.WrapFilesystemError(workspaceDir, "create", err)
	}

	// Create receiver and process files
	receiver := upload.NewReceiver(je.platform, je.logger)
	if err := receiver.ProcessAllFiles(config.UploadPipePath, workspaceDir); err != nil {
		return errors.WrapFilesystemError("", "process_upload", err)
	}

	// Upload processing completed
	return nil
}

// executeCommand uses fork to create a child process while keeping init as PID 1
func (je *JobExecutor) executeCommand(config *environment.JobConfig) error {
	// Resolve command path
	commandPath, err := je.resolveCommandPath(config.Command)
	if err != nil {
		return errors.WrapConfigError("job", "command", err)
	}

	// Change to workspace if uploads were processed (use os.Chdir since we're in isolated namespace)
	if je.platform.Getenv("JOB_HAS_UPLOADS") == "true" {
		workDir := je.config.Filesystem.WorkspaceDir
		if workDir == "" {
			return errors.WrapConfigError("job", "workspace", fmt.Errorf("directory not configured"))
		}
		if _, err := je.platform.Stat(workDir); err == nil {
			if err := os.Chdir(workDir); err != nil {
				return errors.WrapFilesystemError(workDir, "chdir", err)
			}
			// Changed to workspace directory
		}
	}

	// Get current environment (already set up by parent process)
	envv := je.platform.Environ()

	// Executing job command
	// About to exec to replace init process with job command

	// Prepare arguments for exec - argv[0] should be the command name
	argv := append([]string{commandPath}, config.Args...)

	// About to exec to replace init process

	// Use exec to replace the current process (init) with the job command
	// This makes the job command become PID 1 in the namespace, providing proper isolation
	err = je.platform.Exec(commandPath, argv, envv)
	// If we reach this point, exec failed
	je.logger.Error("exec failed - job will not appear as PID 1", "error", err)
	return fmt.Errorf("execution failed: %w", err)
}

// resolveCommandPath resolves the full path for a command
func (je *JobExecutor) resolveCommandPath(command string) (string, error) {
	// Check if absolute path
	if filepath.IsAbs(command) {
		return command, nil
	}

	// Try common locations first - check /usr/local/bin first for runtime binaries
	// We check these first because PATH may not be set correctly in the chroot environment
	commonPaths := []string{
		filepath.Join("/usr/local/bin", command), // Check runtime location first
		filepath.Join("/usr/bin", command),
		filepath.Join("/bin", command),
		filepath.Join("/sbin", command),
		filepath.Join("/usr/sbin", command),
	}

	for _, path := range commonPaths {
		if _, err := je.platform.Stat(path); err == nil {
			// Resolved command at path
			return path, nil
		}
	}

	// Fall back to PATH lookup if not found in common locations
	if path, err := je.platform.LookPath(command); err == nil {
		return path, nil
	}

	// Log what we checked for debugging
	je.logger.Debug("command not found in any location", "command", command, "checked", commonPaths)
	return "", fmt.Errorf("%w: %s", errors.ErrRuntimeNotFound, command)
}

// SetupCgroup sets up cgroup constraints (called before executing)
func (je *JobExecutor) SetupCgroup(cgroupPath string) error {
	// This is typically called from the joblet before switching to init mode
	// The init process will already be in the correct cgroup
	// Cgroup setup requested
	return nil
}

// HandleSignals sets up signal handling for graceful shutdown
func (je *JobExecutor) HandleSignals(ctx context.Context) {
	// Signal handling can be added here if needed
	// Signal handling setup
}
