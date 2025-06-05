package jobinit

import (
	"fmt"
	"job-worker/pkg/logger"
	"job-worker/pkg/os"
	"path/filepath"
	"strconv"
	"time"
)

type JobConfig struct {
	JobID      string
	Command    string
	Args       []string
	CgroupPath string
}

type JobInitializer struct {
	osInterface      os.OsInterface
	syscallInterface os.SyscallInterface
	execInterface    os.ExecInterface
	logger           *logger.Logger
}

func NewJobInitializer() *JobInitializer {
	return &JobInitializer{
		osInterface:      &os.DefaultOs{},
		syscallInterface: &os.DefaultSyscall{},
		execInterface:    &os.DefaultExec{},
		logger:           logger.New(),
	}
}

// LoadConfigFromEnv loads job configuration from environment variables
func (j *JobInitializer) LoadConfigFromEnv() (*JobConfig, error) {

	jobID := j.osInterface.Getenv("JOB_ID")
	command := j.osInterface.Getenv("JOB_COMMAND")
	cgroupPath := j.osInterface.Getenv("JOB_CGROUP_PATH")
	argsCountStr := j.osInterface.Getenv("JOB_ARGS_COUNT")

	jobLogger := j.logger.WithField("jobId", jobID)

	if jobID == "" || command == "" || cgroupPath == "" {

		jobLogger.Error("missing required environment variables",
			"jobId", jobID,
			"command", command,
			"cgroupPath", cgroupPath)

		return nil, fmt.Errorf("missing required environment variables (JOB_ID=%s, JOB_COMMAND=%s, JOB_CGROUP_PATH=%s)",
			jobID, command, cgroupPath)
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

	jobLogger.Debug("loaded job configuration",
		"command", command,
		"cgroupPath", cgroupPath,
		"argsCount", len(args))

	return &JobConfig{
		JobID:      jobID,
		Command:    command,
		Args:       args,
		CgroupPath: cgroupPath,
	}, nil
}

// ExecuteJob sets up cgroup and executes the job command
func (j *JobInitializer) ExecuteJob(config *JobConfig) error {
	jobLogger := j.logger.WithField("jobId", config.JobID)

	// Validate config
	if config == nil {
		j.logger.Error("job config cannot be nil")
		return fmt.Errorf("job config cannot be nil")
	}

	if config.JobID == "" || config.Command == "" || config.CgroupPath == "" {

		jobLogger.Error("invalid job config", "command", config.Command, "cgroupPath", config.CgroupPath)

		return fmt.Errorf("invalid job config: jobID=%s, command=%s, cgroupPath=%s",
			config.JobID, config.Command, config.CgroupPath)
	}

	jobLogger.Info("executing job", "command", config.Command, "args", config.Args)

	// Add ourselves to the cgroup BEFORE executing the real command
	if err := j.JoinCgroup(config.CgroupPath); err != nil {

		jobLogger.Error("failed to join cgroup", "cgroupPath", config.CgroupPath, "error", err)

		return fmt.Errorf("failed to join cgroup %s: %w", config.CgroupPath, err)
	}

	// Resolve command path if it's not absolute
	commandPath, err := j.resolveCommandPath(config.Command)
	if err != nil {
		jobLogger.Error("command resolution failed", "command", config.Command, "error", err)
		return fmt.Errorf("command not found: %w", err)
	}

	if commandPath != config.Command {
		jobLogger.Debug("resolved command path", "original", config.Command, "resolved", commandPath)
	}

	// Prepare arguments for executing - first arg should be the command name
	execArgs := append([]string{config.Command}, config.Args...)

	jobLogger.Debug("executing command",
		"commandPath", commandPath,
		"execArgs", execArgs,
		"envCount", len(j.osInterface.Environ()))

	// Now exec the real command - this replaces our process
	// The exec process will inherit our cgroup membership
	if e := j.syscallInterface.Exec(commandPath, execArgs, j.osInterface.Environ()); e != nil {

		jobLogger.Error("exec failed", "commandPath", commandPath, "command", config.Command, "error", e)

		return fmt.Errorf("failed to exec command %s (resolved to %s): %w",
			config.Command, commandPath, e)
	}

	// This should never be reached since exec replaces the process
	return nil
}

// resolveCommandPath finds the full path to a command
func (j *JobInitializer) resolveCommandPath(command string) (string, error) {
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

	// First try to find the command in PATH
	if resolvedPath, err := j.execInterface.LookPath(command); err == nil {
		j.logger.Debug("resolved command via PATH", "command", command, "resolved", resolvedPath)
		return resolvedPath, nil
	}

	// common locations for system commands
	commonPaths := []string{
		filepath.Join("/bin", command),
		filepath.Join("/usr/bin", command),
		filepath.Join("/usr/local/bin", command),
		filepath.Join("/sbin", command),
		filepath.Join("/usr/sbin", command),
	}

	j.logger.Debug("checking common command locations", "command", command, "paths", commonPaths)

	for _, path := range commonPaths {
		if _, err := j.osInterface.Stat(path); err == nil {
			j.logger.Debug("found command in common location", "command", command, "path", path)
			return path, nil
		}
	}

	j.logger.Error("command not found anywhere",
		"command", command,
		"searchedPaths", commonPaths,
		"searchedPATH", true)

	return "", fmt.Errorf("command %s not found in PATH or common locations", command)
}

// ValidateEnvironment checks if all required environment variables are set
func (j *JobInitializer) ValidateEnvironment() error {
	requiredVars := []string{"JOB_ID", "JOB_COMMAND", "JOB_CGROUP_PATH"}

	for _, varName := range requiredVars {
		if j.osInterface.Getenv(varName) == "" {

			j.logger.Error("required environment variable not set", "variable", varName)

			return fmt.Errorf("required environment variable %s is not set", varName)
		}
	}

	j.logger.Debug("environment validation passed", "requiredVars", requiredVars)
	return nil
}

// Run is the main entry point
func (j *JobInitializer) Run() error {
	j.logger.Info("job-init starting")

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

// JoinCgroup adds the current process to the specified cgroup
func (j *JobInitializer) JoinCgroup(cgroupPath string) error {

	procsFile := filepath.Join(cgroupPath, "cgroup.procs")

	pid := j.osInterface.Getpid()
	pidBytes := []byte(strconv.Itoa(pid))

	log := j.logger.WithFields("pid", pid, "cgroupPath", cgroupPath)

	log.Debug("joining cgroup", "procsFile", procsFile)

	// Retry logic in case cgroup is still being set up
	var lastErr error
	for i := 0; i < 10; i++ {
		if err := j.osInterface.WriteFile(procsFile, pidBytes, 0644); err != nil {
			lastErr = err

			// Exponential backoff: 1ms, 2ms, 4ms, 8ms, etc.
			backoff := time.Duration(1<<uint(i)) * time.Millisecond
			log.Debug("cgroup join attempt failed, retrying",
				"attempt", i+1,
				"error", err,
				"backoff", backoff)

			time.Sleep(backoff)
			continue
		}

		log.Info("successfully joined cgroup", "attempts", i+1)

		return nil
	}

	log.Error("failed to join cgroup after retries", "attempts", 10, "lastError", lastErr)

	return fmt.Errorf("failed to join cgroup after 10 retries: %w", lastErr)
}

// Run is entry point for the main function
func Run() error {
	ji := NewJobInitializer()
	return ji.Run()
}
