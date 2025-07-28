package jobexec

import (
	"context"
	"fmt"
	"joblet/internal/joblet/core/environment"
	"joblet/internal/joblet/core/upload"
	"joblet/pkg/logger"
	"joblet/pkg/platform"
	"path/filepath"
)

// JobExecutor handles job execution in init mode with consolidated environment handling
type JobExecutor struct {
	platform      platform.Platform
	logger        *logger.Logger
	envBuilder    *environment.Builder
	uploadManager *upload.Manager
}

// NewJobExecutor creates a new job executor
func NewJobExecutor(platform platform.Platform, logger *logger.Logger) *JobExecutor {
	// Create upload manager
	uploadManager := upload.NewManager(platform, logger)

	// Create upload factory for creating stream contexts
	uploadFactory := upload.NewFactory(logger)

	// Create environment builder with all dependencies
	envBuilder := environment.NewBuilder(platform, uploadManager, uploadFactory, logger)

	return &JobExecutor{
		platform:      platform,
		logger:        logger.WithField("component", "job-executor"),
		envBuilder:    envBuilder,
		uploadManager: uploadManager, // Store for direct access if needed
	}
}

// ExecuteInInitMode executes a job in init mode
func (je *JobExecutor) ExecuteInInitMode() error {
	// Load job configuration from environment
	config, err := je.envBuilder.LoadJobConfigFromEnvironment()
	if err != nil {
		return fmt.Errorf("failed to load job config: %w", err)
	}

	log := je.logger.WithField("jobID", config.JobID).
		WithField("totalFiles", config.TotalFiles)

	log.Debug("executing job in init mode",
		"command", config.Command,
		"args", config.Args,
		"hasUploads", config.HasUploadSession)

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
	executor := NewJobExecutor(p, logger)
	return executor.ExecuteJob()
}

func (je *JobExecutor) ExecuteJob() error {
	// Load configuration from environment
	config, err := je.envBuilder.LoadJobConfigFromEnvironment()
	if err != nil {
		return fmt.Errorf("failed to load job configuration: %w", err)
	}

	log := je.logger.WithField("jobID", config.JobID)

	// Check which phase we're in
	phase := je.platform.Getenv("JOB_PHASE")

	switch phase {
	case "upload":
		// Upload phase is handled in server.go
		return fmt.Errorf("upload phase should be handled by server.go")

	case "execute", "":
		// Execute phase - just run the command
		log.Debug("executing job in init mode", "command", config.Command, "args", config.Args,
			"hasUploads", je.platform.Getenv("JOB_HAS_UPLOADS") == "true")

		return je.executeCommand(config)

	default:
		return fmt.Errorf("unknown job phase: %s", phase)
	}
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

// executeCommand remains mostly the same but simpler
func (je *JobExecutor) executeCommand(config *environment.JobConfig) error {
	// Resolve command path
	commandPath, err := je.resolveCommandPath(config.Command)
	if err != nil {
		return fmt.Errorf("failed to resolve command: %w", err)
	}

	// Create command
	cmd := je.platform.CreateCommand(commandPath, config.Args...)

	// Change to workspace if uploads were processed
	if je.platform.Getenv("JOB_HAS_UPLOADS") == "true" {
		workDir := "/work"
		if _, err := je.platform.Stat(workDir); err == nil {
			cmd.SetDir(workDir)
			je.logger.Debug("changed working directory to /work")
		}
	}

	je.logger.Info("executing job command", "command", commandPath, "args", config.Args)
	// Execute
	return cmd.Run()
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
