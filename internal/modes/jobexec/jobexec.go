package jobexec

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"

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

// LoadConfigFromEnv loads job configuration from environment variables
func LoadConfigFromEnv(logger *logger.Logger) (*JobConfig, error) {
	jobID := os.Getenv("JOB_ID")
	command := os.Getenv("JOB_COMMAND")
	cgroupPath := os.Getenv("JOB_CGROUP_PATH")
	argsCountStr := os.Getenv("JOB_ARGS_COUNT")

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
			args[i] = os.Getenv(argKey)
		}
	}

	logger.Debug("loaded job configuration",
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
	switch runtime.GOOS {
	case "linux":
		return executeLinux(config, logger)
	case "darwin":
		return executeDarwin(config, logger)
	default:
		return fmt.Errorf("unsupported platform for job execution: %s", runtime.GOOS)
	}
}

// executeLinux executes job on Linux using syscall.Exec
func executeLinux(config *JobConfig, logger *logger.Logger) error {
	logger.Debug("executing job on Linux", "command", config.Command, "args", config.Args)

	// Resolve command path
	commandPath, err := resolveCommandPath(config.Command, logger)
	if err != nil {
		return fmt.Errorf("command resolution failed: %w", err)
	}

	// Prepare arguments and environment
	execArgs := append([]string{config.Command}, config.Args...)
	envVars := os.Environ()

	logger.Debug("executing command with syscall.Exec",
		"commandPath", commandPath, "args", execArgs)

	// Use syscall.Exec to replace the current process
	if err := syscall.Exec(commandPath, execArgs, envVars); err != nil {
		return fmt.Errorf("syscall.Exec failed: %w", err)
	}

	// This line should never be reached
	return nil
}

// executeDarwin executes job on macOS using exec.Command
func executeDarwin(config *JobConfig, logger *logger.Logger) error {
	logger.Debug("executing job on macOS", "command", config.Command, "args", config.Args)

	// Resolve command path
	commandPath, err := resolveCommandPath(config.Command, logger)
	if err != nil {
		return fmt.Errorf("command resolution failed: %w", err)
	}

	// Use exec.Command for macOS
	cmd := exec.Command(commandPath, config.Args...)
	cmd.Env = os.Environ()

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command execution failed: %w", err)
	}

	logger.Debug("command completed successfully on macOS")
	return nil
}

// resolveCommandPath resolves a command to its full path
func resolveCommandPath(command string, logger *logger.Logger) (string, error) {
	if command == "" {
		return "", fmt.Errorf("command cannot be empty")
	}

	// If already absolute path, verify it exists
	if filepath.IsAbs(command) {
		if _, err := os.Stat(command); err != nil {
			return "", fmt.Errorf("command %s not found: %w", command, err)
		}
		return command, nil
	}

	// Try to find in PATH using platform abstraction
	p := platform.NewPlatform()
	if resolvedPath, err := p.LookPath(command); err == nil {
		logger.Debug("resolved command via PATH", "command", command, "resolved", resolvedPath)
		return resolvedPath, nil
	}

	// Check common paths
	commonPaths := getCommonPaths(command)

	for _, path := range commonPaths {
		if _, err := os.Stat(path); err == nil {
			logger.Debug("found command in common location", "command", command, "path", path)
			return path, nil
		}
	}

	return "", fmt.Errorf("command %s not found in PATH or common locations", command)
}

// getCommonPaths returns platform-specific common command paths
func getCommonPaths(command string) []string {
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
	switch runtime.GOOS {
	case "linux":
		// On Linux: This should never be reached since Execute calls syscall.Exec
		logger.Error("unexpected return from Execute - exec should have replaced process")
		os.Exit(1)
	case "darwin":
		// On macOS: This is expected since we use exec.Command.Run() instead of syscall.Exec
		logger.Debug("job execution completed successfully on macOS")
	default:
		logger.Debug("job execution completed", "platform", runtime.GOOS)
	}
}
