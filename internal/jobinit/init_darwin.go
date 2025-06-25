//go:build darwin

package jobinit

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"worker/pkg/logger"
	"worker/pkg/os"
)

type darwinJobInitializer struct {
	osInterface   os.OsInterface
	execInterface os.ExecInterface
	logger        *logger.Logger
}

// NewJobInitializer creates a macOS-specific job initializer
func NewJobInitializer() JobInitializer {
	return &darwinJobInitializer{
		osInterface:   &os.DefaultOs{},
		execInterface: &os.DefaultExec{},
		logger:        logger.New(),
	}
}

// Ensure darwinJobInitializer implements JobInitializer
var _ JobInitializer = (*darwinJobInitializer)(nil)

func (j *darwinJobInitializer) LoadConfigFromEnv() (*JobConfig, error) {
	jobID := j.osInterface.Getenv("JOB_ID")
	command := j.osInterface.Getenv("JOB_COMMAND")
	cgroupPath := j.osInterface.Getenv("JOB_CGROUP_PATH")
	argsCountStr := j.osInterface.Getenv("JOB_ARGS_COUNT")

	jobLogger := j.logger.WithField("jobId", jobID)

	if jobID == "" || command == "" {
		jobLogger.Error("missing required environment variables (macOS)",
			"jobId", jobID,
			"command", command)

		return nil, fmt.Errorf("missing required environment variables (JOB_ID=%s, JOB_COMMAND=%s)",
			jobID, command)
	}

	var args []string

	// Parse arguments from individual environment variables
	if argsCountStr != "" {
		argsCount, err := strconv.Atoi(argsCountStr)
		if err != nil {
			jobLogger.Error("invalid JOB_ARGS_COUNT", "argsCount", argsCountStr, "error", err)
			return nil, fmt.Errorf("invalid JOB_ARGS_COUNT: %v", err)
		}

		args = make([]string, argsCount)
		for i := 0; i < argsCount; i++ {
			argKey := fmt.Sprintf("JOB_ARG_%d", i)
			args[i] = j.osInterface.Getenv(argKey)
		}

		jobLogger.Debug("loaded job arguments", "argsCount", argsCount, "args", args)
	}

	jobLogger.Debug("loaded job configuration on macOS",
		"command", command,
		"cgroupPath", cgroupPath,
		"argsCount", len(args))

	return &JobConfig{
		JobID:      jobID,
		Command:    command,
		Args:       args,
		CgroupPath: cgroupPath, // Will be ignored on macOS
	}, nil
}

func (j *darwinJobInitializer) ExecuteJob(config *JobConfig) error {
	jobLogger := j.logger.WithField("jobId", config.JobID)

	// Validate config
	if config == nil {
		j.logger.Error("job config cannot be nil")
		return fmt.Errorf("job config cannot be nil")
	}

	if config.JobID == "" || config.Command == "" {
		jobLogger.Error("invalid job config (macOS)", "command", config.Command)
		return fmt.Errorf("invalid job config: jobID=%s, command=%s",
			config.JobID, config.Command)
	}

	jobLogger.Info("executing job on macOS (real exec)", "command", config.Command, "args", config.Args)

	// Resolve command path if it's not absolute
	commandPath, err := j.resolveCommandPath(config.Command)
	if err != nil {
		jobLogger.Error("command resolution failed", "command", config.Command, "error", err)
		return fmt.Errorf("command not found: %w", err)
	}

	if commandPath != config.Command {
		jobLogger.Debug("resolved command path", "original", config.Command, "resolved", commandPath)
	}

	// On macOS, we can actually exec the real command (no cgroups, but real execution)
	execArgs := append([]string{config.Command}, config.Args...)

	jobLogger.Debug("executing real command on macOS",
		"commandPath", commandPath,
		"execArgs", execArgs,
		"envCount", len(j.osInterface.Environ()))

	// Use standard exec.Command instead of syscall.Exec for macOS
	cmd := exec.Command(commandPath, config.Args...)
	cmd.Env = j.osInterface.Environ()

	// Replace current process with the command
	if err := cmd.Run(); err != nil {
		jobLogger.Error("command execution failed", "commandPath", commandPath, "error", err)
		return fmt.Errorf("failed to execute command %s: %w", commandPath, err)
	}

	jobLogger.Info("command completed successfully on macOS")
	return nil
}

func (j *darwinJobInitializer) Run() error {
	j.logger.Info("job-init starting on macOS")

	if err := j.ValidateEnvironment(); err != nil {
		j.logger.Error("environment validation failed", "error", err)
		return fmt.Errorf("environment validation failed: %w", err)
	}

	config, err := j.LoadConfigFromEnv()
	if err != nil {
		j.logger.Error("failed to load config", "error", err)
		return fmt.Errorf("failed to load config: %w", err)
	}

	if e := j.ExecuteJob(config); e != nil {
		j.logger.Error("failed to execute job", "error", e)
		return fmt.Errorf("failed to execute job: %w", e)
	}

	return nil
}

func (j *darwinJobInitializer) ValidateEnvironment() error {
	// On macOS, we only require JOB_ID and JOB_COMMAND (no cgroup)
	requiredVars := []string{"JOB_ID", "JOB_COMMAND"}

	for _, varName := range requiredVars {
		if j.osInterface.Getenv(varName) == "" {
			j.logger.Error("required environment variable not set", "variable", varName)
			return fmt.Errorf("required environment variable %s is not set", varName)
		}
	}

	j.logger.Debug("environment validation passed (macOS)", "requiredVars", requiredVars)
	return nil
}

func (j *darwinJobInitializer) resolveCommandPath(command string) (string, error) {
	if command == "" {
		return "", fmt.Errorf("command cannot be empty")
	}

	// If already absolute path, verify it exists
	if filepath.IsAbs(command) {
		if _, err := j.osInterface.Stat(command); err != nil {
			j.logger.Error("absolute command path not found", "command", command, "error", err)
			return "", fmt.Errorf("command %s not found: %w", command, err)
		}
		j.logger.Debug("using absolute command path", "command", command)
		return command, nil
	}

	// Try to find the command in PATH
	if resolvedPath, err := j.execInterface.LookPath(command); err == nil {
		j.logger.Debug("resolved command via PATH", "command", command, "resolved", resolvedPath)
		return resolvedPath, nil
	}

	// Common locations for macOS
	commonPaths := []string{
		filepath.Join("/bin", command),
		filepath.Join("/usr/bin", command),
		filepath.Join("/usr/local/bin", command),
		filepath.Join("/sbin", command),
		filepath.Join("/usr/sbin", command),
		filepath.Join("/opt/homebrew/bin", command), // Homebrew on Apple Silicon
		filepath.Join("/usr/local/Cellar", command), // Homebrew on Intel
	}

	j.logger.Debug("checking common command locations (macOS)", "command", command, "paths", commonPaths)

	for _, path := range commonPaths {
		if _, err := j.osInterface.Stat(path); err == nil {
			j.logger.Debug("found command in common location", "command", command, "path", path)
			return path, nil
		}
	}

	j.logger.Error("command not found anywhere (macOS)",
		"command", command,
		"searchedPaths", commonPaths,
		"searchedPATH", true)

	return "", fmt.Errorf("command %s not found in PATH or common locations", command)
}
