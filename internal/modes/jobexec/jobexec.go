package jobexec

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strconv"

	"worker/pkg/logger"
	"worker/pkg/platform"
)

// JobConfig represents job configuration
type JobConfig struct {
	JobID      string
	Command    string
	Args       []string
	CgroupPath string
}

// JobExecutor handles job execution using platform abstraction
type JobExecutor struct {
	platform platform.Platform
	logger   *logger.Logger
}

// NewJobExecutor creates a new job executor with the given platform
func NewJobExecutor(p platform.Platform, logger *logger.Logger) *JobExecutor {
	return &JobExecutor{
		platform: p,
		logger:   logger.WithField("component", "jobexec"),
	}
}

// LoadConfigFromEnv loads job configuration from environment variables using platform abstraction
func LoadConfigFromEnv(logger *logger.Logger) (*JobConfig, error) {
	p := platform.NewPlatform()
	executor := NewJobExecutor(p, logger)
	return executor.LoadConfigFromEnv()
}

// LoadConfigFromEnv loads job configuration from environment variables
func (je *JobExecutor) LoadConfigFromEnv() (*JobConfig, error) {
	jobID := je.platform.Getenv("JOB_ID")
	command := je.platform.Getenv("JOB_COMMAND")
	cgroupPath := je.platform.Getenv("JOB_CGROUP_PATH")
	argsCountStr := je.platform.Getenv("JOB_ARGS_COUNT")

	if jobID == "" || command == "" {
		return nil, fmt.Errorf("missing required environment variables (JOB_ID=%s, JOB_COMMAND=%s)",
			jobID, command)
	}

	var args []string
	if argsCountStr != "" {
		argsCount, err := strconv.Atoi(argsCountStr)
		if err != nil {
			return nil, fmt.Errorf("invalid JOB_ARGS_COUNT: %v", err)
		}

		args = make([]string, argsCount)
		for i := 0; i < argsCount; i++ {
			argKey := fmt.Sprintf("JOB_ARG_%d", i)
			args[i] = je.platform.Getenv(argKey)
		}
	}

	je.logger.Info("loaded job configuration",
		"jobId", jobID,
		"command", command,
		"argsCount", len(args),
		"cgroupPath", cgroupPath)

	return &JobConfig{
		JobID:      jobID,
		Command:    command,
		Args:       args,
		CgroupPath: cgroupPath,
	}, nil
}

// Execute executes the job based on platform
func Execute(config *JobConfig, logger *logger.Logger) error {
	p := platform.NewPlatform()
	executor := NewJobExecutor(p, logger)
	return executor.Execute(config)
}

// Execute executes the job based on platform using platform abstraction
func (je *JobExecutor) Execute(config *JobConfig) error {
	switch runtime.GOOS {
	case "linux":
		return je.executeLinux(config)
	case "darwin":
		return je.executeDarwin(config)
	default:
		return fmt.Errorf("unsupported platform for job execution: %s", runtime.GOOS)
	}
}

// executeLinux executes job on Linux using platform abstraction
func (je *JobExecutor) executeLinux(config *JobConfig) error {
	je.logger.Info("executing job on Linux", "command", config.Command, "args", config.Args)

	// Resolve command path using platform abstraction
	commandPath, err := je.resolveCommandPath(config.Command)
	if err != nil {
		return fmt.Errorf("command resolution failed: %w", err)
	}

	// Prepare arguments and environment using platform abstraction
	execArgs := append([]string{config.Command}, config.Args...)
	envVars := je.platform.Environ()

	je.logger.Info("executing command with platform exec",
		"commandPath", commandPath, "args", execArgs)

	// Use platform abstraction for exec
	if err := je.platform.Exec(commandPath, execArgs, envVars); err != nil {
		return fmt.Errorf("platform exec failed: %w", err)
	}

	// This line should never be reached on successful exec
	return nil
}

// executeDarwin executes job on macOS using platform abstraction
func (je *JobExecutor) executeDarwin(config *JobConfig) error {
	je.logger.Info("executing job on macOS", "command", config.Command, "args", config.Args)

	// Resolve command path using platform abstraction
	commandPath, err := je.resolveCommandPath(config.Command)
	if err != nil {
		return fmt.Errorf("command resolution failed: %w", err)
	}

	// Use platform abstraction to create and run command
	cmd := je.platform.CreateCommand(commandPath, config.Args...)
	cmd.SetEnv(je.platform.Environ())

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("command start failed: %w", err)
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("command execution failed: %w", err)
	}

	je.logger.Info("command completed successfully on macOS")
	return nil
}

// resolveCommandPath resolves a command to its full path using platform abstraction
func (je *JobExecutor) resolveCommandPath(command string) (string, error) {
	if command == "" {
		return "", fmt.Errorf("command cannot be empty")
	}

	// If already absolute path, verify it exists using platform abstraction
	if filepath.IsAbs(command) {
		if _, err := je.platform.Stat(command); err != nil {
			return "", fmt.Errorf("command %s not found: %w", command, err)
		}
		return command, nil
	}

	// Try to find in PATH using platform abstraction
	if resolvedPath, err := je.platform.LookPath(command); err == nil {
		je.logger.Debug("resolved command via PATH", "command", command, "resolved", resolvedPath)
		return resolvedPath, nil
	}

	// Check common paths using platform abstraction
	commonPaths := je.getCommonPaths(command)

	for _, path := range commonPaths {
		if _, err := je.platform.Stat(path); err == nil {
			je.logger.Debug("found command in common location", "command", command, "path", path)
			return path, nil
		}
	}

	return "", fmt.Errorf("command %s not found in PATH or common locations", command)
}

// getCommonPaths returns platform-specific common command paths
func (je *JobExecutor) getCommonPaths(command string) []string {
	commonPaths := []string{
		filepath.Join("/bin", command),
		filepath.Join("/usr/bin", command),
		filepath.Join("/usr/local/bin", command),
		filepath.Join("/sbin", command),
		filepath.Join("/usr/sbin", command),
	}

	// Add platform-specific paths
	switch runtime.GOOS {
	case "darwin":
		commonPaths = append(commonPaths,
			filepath.Join("/opt/homebrew/bin", command),
			filepath.Join("/usr/local/Cellar", command))
	case "linux":
		commonPaths = append(commonPaths,
			filepath.Join("/usr/local/sbin", command))
	}

	return commonPaths
}

// HandleCompletion handles platform-specific completion logic
func HandleCompletion(logger *logger.Logger) {
	p := platform.NewPlatform()
	executor := NewJobExecutor(p, logger)
	executor.HandleCompletion()
}

// HandleCompletion handles platform-specific completion logic using platform abstraction
func (je *JobExecutor) HandleCompletion() {
	switch runtime.GOOS {
	case "linux":
		// On Linux: This should never be reached since Execute calls platform.Exec
		je.logger.Error("unexpected return from Execute - exec should have replaced process")
		je.platform.Exit(1)
	case "darwin":
		// On macOS: This is expected since we use command execution instead of exec
		je.logger.Info("job execution completed successfully on macOS")
	default:
		je.logger.Info("job execution completed", "platform", runtime.GOOS)
	}
}
