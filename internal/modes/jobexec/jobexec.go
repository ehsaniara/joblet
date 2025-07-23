package jobexec

import (
	"context"
	"fmt"
	"joblet/internal/joblet/core/environment"
	"joblet/internal/joblet/core/upload"
	"joblet/pkg/logger"
	"joblet/pkg/platform"
	"os"
	"path/filepath"
	"strings"
)

// JobExecutor handles job execution in init mode with consolidated environment handling
type JobExecutor struct {
	platform   platform.Platform
	logger     *logger.Logger
	envBuilder *environment.Builder
}

// NewJobExecutor creates a new job executor
func NewJobExecutor(platform platform.Platform, logger *logger.Logger) *JobExecutor {
	uploadManager := upload.NewManager(platform, logger)
	envBuilder := environment.NewBuilder(platform, uploadManager, logger)

	return &JobExecutor{
		platform:   platform,
		logger:     logger.WithField("component", "job-executor"),
		envBuilder: envBuilder,
	}
}

// Execute executes the job using consolidated environment handling
func Execute(logger *logger.Logger) error {
	p := platform.NewPlatform()
	executor := NewJobExecutor(p, logger)
	return executor.ExecuteJob()
}

// ExecuteJob executes the job with consolidated upload and environment handling
func (je *JobExecutor) ExecuteJob() error {
	// Load configuration from environment
	config, err := je.envBuilder.LoadJobConfigFromEnvironment()
	if err != nil {
		return fmt.Errorf("failed to load job configuration: %w", err)
	}

	log := je.logger.WithField("jobID", config.JobID)
	log.Debug("executing job in init mode",
		"command", config.Command,
		"args", config.Args,
		"hasUploads", config.HasUploadSession)

	// Process uploads if present
	if config.HasUploadSession && config.UploadPipePath != "" {
		if err := je.processUploads(config); err != nil {
			log.Error("failed to process uploads", "error", err)
			// Continue execution even if upload fails
		}
	}

	// Execute the command
	return je.executeCommand(config)
}

// processUploads handles upload processing from the pipe
func (je *JobExecutor) processUploads(config *environment.JobConfig) error {
	log := je.logger.WithField("operation", "process-uploads")

	workspaceDir := "/work"
	log.Debug("processing uploads from pipe",
		"pipePath", config.UploadPipePath,
		"workspace", workspaceDir)

	// Create workspace
	if err := je.platform.MkdirAll(workspaceDir, 0755); err != nil {
		return fmt.Errorf("failed to create workspace: %w", err)
	}

	// Create receiver and process files
	receiver := upload.NewReceiver(je.platform, je.logger)
	if err := receiver.ProcessAllFiles(config.UploadPipePath, workspaceDir); err != nil {
		return fmt.Errorf("failed to process files from pipe: %w", err)
	}

	log.Debug("upload processing completed")
	return nil
}

// executeCommand executes the actual command
func (je *JobExecutor) executeCommand(config *environment.JobConfig) error {
	// Check if we should exec (replace process)
	shouldExec := je.platform.Getenv("JOBLET_EXEC_AFTER_ISOLATION") == "true"

	// Resolve command path
	commandPath, err := je.resolveCommandPath(config.Command)
	if err != nil {
		return fmt.Errorf("failed to resolve command: %w", err)
	}

	if shouldExec {
		// Use exec to completely replace the joblet process
		// This makes the user command become PID 1 with no trace of joblet
		je.logger.Debug("executing command with exec (process replacement)",
			"command", commandPath,
			"args", config.Args)

		// Build argv array (command is argv[0])
		argv := append([]string{commandPath}, config.Args...)

		// Get clean environment (remove joblet-specific vars)
		envv := je.cleanEnvironment()

		// This replaces the current process entirely
		// After this, the user command IS PID 1
		return je.platform.Exec(commandPath, argv, envv)
	}

	// Fallback: run as child process (original behavior)
	cmd := je.platform.CreateCommand(commandPath, config.Args...)
	cmd.SetStdout(os.Stdout)
	cmd.SetStderr(os.Stderr)
	cmd.SetStdin(os.Stdin)

	if config.HasUploadSession {
		cmd.SetDir("/work")
	}

	return cmd.Run()
}

// cleanEnvironment removes joblet-specific environment variables
func (je *JobExecutor) cleanEnvironment() []string {
	current := je.platform.Environ()
	cleaned := make([]string, 0, len(current))

	// List of prefixes to remove
	removePrefix := []string{
		"JOBLET_",
		"JOB_ID=",
		"JOB_COMMAND=",
		"JOB_CGROUP_",
		"JOB_ARG_",
		"JOB_MAX_",
		"JOB_UPLOAD_",
	}

	for _, env := range current {
		keep := true
		for _, prefix := range removePrefix {
			if strings.HasPrefix(env, prefix) {
				keep = false
				break
			}
		}
		if keep {
			cleaned = append(cleaned, env)
		}
	}

	// Add minimal required environment
	cleaned = append(cleaned,
		"PATH=/usr/local/bin:/usr/bin:/bin:/usr/local/sbin:/usr/sbin:/sbin",
		"HOME=/tmp",
		"USER=nobody",
		"TERM=xterm",
	)

	return cleaned
}

// resolveCommandPath resolves the full path for a command
func (je *JobExecutor) resolveCommandPath(command string) (string, error) {
	// Check if absolute path
	if filepath.IsAbs(command) {
		return command, nil
	}

	// Try PATH lookup
	if path, err := je.platform.LookPath(command); err == nil {
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
		if _, err := je.platform.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("command %s not found", command)
}

// SetupCgroup sets up cgroup constraints (called before executing)
func (je *JobExecutor) SetupCgroup(cgroupPath string) error {
	// This is typically called from the joblet before switching to init mode
	// The init process will already be in the correct cgroup
	je.logger.Debug("cgroup setup requested", "path", cgroupPath)
	return nil
}

// HandleSignals sets up signal handling for graceful shutdown
func (je *JobExecutor) HandleSignals(ctx context.Context) {
	// Signal handling can be added here if needed
	je.logger.Debug("signal handling setup")
}
