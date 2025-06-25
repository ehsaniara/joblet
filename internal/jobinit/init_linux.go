//go:build linux

package jobinit

import (
	"fmt"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
	"worker/pkg/logger"
	"worker/pkg/platform"
)

type jobInitializer struct {
	platform platform.Platform
	logger   *logger.Logger
}

// NewJobInitializer creates a job initializer
func NewJobInitializer() JobInitializer {
	platformInterface := platform.NewPlatform()
	return &jobInitializer{
		platform: platformInterface,
		logger:   logger.New(),
	}
}

func (j *jobInitializer) LoadConfigFromEnv() (*JobConfig, error) {
	jobID := j.platform.Getenv("JOB_ID")
	command := j.platform.Getenv("JOB_COMMAND")
	cgroupPath := j.platform.Getenv("JOB_CGROUP_PATH")
	argsCountStr := j.platform.Getenv("JOB_ARGS_COUNT")
	hostNetworking := j.platform.Getenv("HOST_NETWORKING")

	jobLogger := j.logger.WithField("jobId", jobID)

	if jobID == "" || command == "" || cgroupPath == "" {
		jobLogger.Error("missing required environment variables")
		return nil, fmt.Errorf("missing required environment variables")
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
			args[i] = j.platform.Getenv(argKey)
		}
	}

	jobLogger.Info("loaded job configuration (host networking)",
		"command", command,
		"cgroupPath", cgroupPath,
		"argsCount", len(args),
		"hostNetworking", hostNetworking)

	return &JobConfig{
		JobID:      jobID,
		Command:    command,
		Args:       args,
		CgroupPath: cgroupPath,
	}, nil
}

func (j *jobInitializer) ExecuteJob(config *JobConfig) error {
	jobLogger := j.logger.WithField("jobId", config.JobID)

	if config == nil {
		return fmt.Errorf("job config cannot be nil")
	}

	jobLogger.Info("executing job with host networking", "command", config.Command)

	// Setup basic namespace environment
	if err := j.setupBasicEnvironment(); err != nil {
		return fmt.Errorf("failed to setup basic environment: %w", err)
	}

	// Join cgroup
	if err := j.joinCgroup(config.CgroupPath); err != nil {
		return fmt.Errorf("failed to join cgroup: %w", err)
	}

	// Resolve command path
	commandPath, err := j.resolveCommandPath(config.Command)
	if err != nil {
		return fmt.Errorf("command not found: %w", err)
	}

	// Execute the command (inherits host networking)
	execArgs := append([]string{config.Command}, config.Args...)
	jobLogger.Info("executing command with host networking", "commandPath", commandPath)

	if err := j.platform.Exec(commandPath, execArgs, j.platform.Environ()); err != nil {
		return fmt.Errorf("failed to exec command: %w", err)
	}

	return nil
}

func (j *jobInitializer) Run() error {
	j.logger.Info("job-init starting with host networking")

	if err := j.setupBasicEnvironment(); err != nil {
		return err
	}

	config, err := j.LoadConfigFromEnv()
	if err != nil {
		return err
	}

	return j.ExecuteJob(config)
}

func (j *jobInitializer) setupBasicEnvironment() error {
	pid := j.platform.Getpid()
	j.logger.Debug("setting up basic environment", "pid", pid)

	// Only remount /proc if we're PID 1
	if pid == 1 {
		j.logger.Info("detected PID 1 - remounting /proc for namespace isolation")

		if err := j.platform.Mount("", "/", "", syscall.MS_PRIVATE|syscall.MS_REC, ""); err != nil {
			j.logger.Warn("failed to make mounts private", "error", err)
		}

		if err := j.remountProc(); err != nil {
			return fmt.Errorf("failed to remount /proc: %w", err)
		}

		j.logger.Info("cgroup namespace already configured by launcher")
	}

	j.logger.Info("basic environment setup completed")
	return nil
}

// Setup cgroup namespace mount
func (j *jobInitializer) setupCgroupNamespace() error {
	j.logger.Info("verifying cgroup namespace setup")

	// Verify cgroup filesystem is accessible
	if _, err := j.platform.Stat("/sys/fs/cgroup/cgroup.procs"); err != nil {
		j.logger.Warn("cgroup.procs not immediately available, will be handled by joinCgroup", "error", err)
		// Don't fail here - joinCgroup will handle this later
	} else {
		j.logger.Info("cgroup filesystem is accessible in namespace")
	}

	return nil
}

func (j *jobInitializer) remountProc() error {
	if err := j.platform.Mount("proc", "/proc", "proc", 0, ""); err != nil {
		if unmountErr := j.platform.Unmount("/proc", syscall.MNT_DETACH); unmountErr != nil {
			j.logger.Debug("/proc unmount failed", "error", unmountErr)
		}

		if err := j.platform.Mount("proc", "/proc", "proc", 0, ""); err != nil {
			return fmt.Errorf("failed to remount /proc: %w", err)
		}
	}

	j.logger.Info("/proc remounted for namespace isolation")
	return nil
}

func (j *jobInitializer) joinCgroup(cgroupPath string) error {
	// In cgroup namespace, our cgroup appears at the root
	namespaceCgroupPath := "/sys/fs/cgroup"
	procsFile := filepath.Join(namespaceCgroupPath, "cgroup.procs")

	pid := j.platform.Getpid()
	pidBytes := []byte(strconv.Itoa(pid))

	log := j.logger.WithFields(
		"pid", pid,
		"hostCgroupPath", cgroupPath,
		"namespaceCgroupPath", namespaceCgroupPath)

	for i := 0; i < 5; i++ {
		if err := j.platform.WriteFile(procsFile, pidBytes, 0644); err != nil {
			backoff := time.Duration(1<<uint(i)) * time.Millisecond
			log.Debug("cgroup join attempt failed, retrying", "attempt", i+1, "backoff", backoff)
			time.Sleep(backoff)
			continue
		}
		log.Info("successfully joined cgroup in namespace")
		return nil
	}

	return fmt.Errorf("failed to join cgroup after 5 retries")
}

func (j *jobInitializer) resolveCommandPath(command string) (string, error) {
	if filepath.IsAbs(command) {
		if _, err := j.platform.Stat(command); err != nil {
			return "", fmt.Errorf("command %s not found: %w", command, err)
		}
		return command, nil
	}

	if resolvedPath, err := j.platform.LookPath(command); err == nil {
		return resolvedPath, nil
	}

	commonPaths := []string{
		filepath.Join("/bin", command),
		filepath.Join("/usr/bin", command),
		filepath.Join("/usr/local/bin", command),
		filepath.Join("/sbin", command),
		filepath.Join("/usr/sbin", command),
	}

	for _, path := range commonPaths {
		if _, err := j.platform.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("command %s not found in PATH or common locations", command)
}
