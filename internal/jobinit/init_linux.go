//go:build linux

package jobinit

import (
	"fmt"
	"job-worker/pkg/logger"
	"job-worker/pkg/os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

type jobInitializer struct {
	osInterface      os.OsInterface
	syscallInterface os.SyscallInterface
	execInterface    os.ExecInterface
	logger           *logger.Logger
}

// NewJobInitializer creates a job initializer
func NewJobInitializer() JobInitializer {
	return &jobInitializer{
		osInterface:      &os.DefaultOs{},
		syscallInterface: &os.DefaultSyscall{},
		execInterface:    &os.DefaultExec{},
		logger:           logger.New(),
	}
}

func (j *jobInitializer) LoadConfigFromEnv() (*JobConfig, error) {
	jobID := j.osInterface.Getenv("JOB_ID")
	command := j.osInterface.Getenv("JOB_COMMAND")
	cgroupPath := j.osInterface.Getenv("JOB_CGROUP_PATH")
	argsCountStr := j.osInterface.Getenv("JOB_ARGS_COUNT")
	hostNetworking := j.osInterface.Getenv("HOST_NETWORKING")

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
			args[i] = j.osInterface.Getenv(argKey)
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
		// No network fields
	}, nil
}

func (j *jobInitializer) ExecuteJob(config *JobConfig) error {
	jobLogger := j.logger.WithField("jobId", config.JobID)

	if config == nil {
		return fmt.Errorf("job config cannot be nil")
	}

	jobLogger.Info("executing job with host networking", "command", config.Command)

	// Setup basic namespace environment (no network namespaces)
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

	if err := j.syscallInterface.Exec(commandPath, execArgs, j.osInterface.Environ()); err != nil {
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
	pid := j.osInterface.Getpid()
	j.logger.Debug("setting up basic environment", "pid", pid)

	// Validate we're in a cgroup namespace
	if err := j.validateCgroupNamespace(); err != nil {
		return fmt.Errorf("cgroup namespace validation failed: %w", err)
	}

	// Only remount /proc if we're PID 1
	if pid == 1 {
		j.logger.Info("detected PID 1 - remounting /proc for namespace isolation")

		if err := j.syscallInterface.Mount("", "/", "", syscall.MS_PRIVATE|syscall.MS_REC, ""); err != nil {
			j.logger.Warn("failed to make mounts private", "error", err)
		}

		if err := j.remountProc(); err != nil {
			return fmt.Errorf("failed to remount /proc: %w", err)
		}

		// setup cgroup namespace mount
		if err := j.setupCgroupNamespace(); err != nil {
			return fmt.Errorf("failed to setup cgroup namespace: %w", err)
		}
	}

	return nil
}

// cgroup namespace validation
func (j *jobInitializer) validateCgroupNamespace() error {
	// Check if we're in a different cgroup namespace than init
	initNS, err := j.osInterface.ReadFile("/proc/1/ns/cgroup")
	if err != nil {
		return fmt.Errorf("cannot read init cgroup namespace: %w", err)
	}

	selfNS, err := j.osInterface.ReadFile("/proc/self/ns/cgroup")
	if err != nil {
		return fmt.Errorf("cannot read self cgroup namespace: %w", err)
	}

	if string(initNS) == string(selfNS) {
		return fmt.Errorf("not running in cgroup namespace (required for this project)")
	}

	j.logger.Info("cgroup namespace validation passed")
	return nil
}

// Setup cgroup namespace mount
func (j *jobInitializer) setupCgroupNamespace() error {
	j.logger.Info("setting up cgroup namespace mount")

	// Mount cgroup2 filesystem in the namespace
	// This gives us a clean view of just our cgroup subtree
	if err := j.syscallInterface.Mount("cgroup2", "/sys/fs/cgroup", "cgroup2", 0, ""); err != nil {
		return fmt.Errorf("failed to mount cgroup2 in namespace: %w", err)
	}

	// Verify the mount worked
	if _, err := j.osInterface.Stat("/sys/fs/cgroup/cgroup.procs"); err != nil {
		return fmt.Errorf("cgroup namespace mount verification failed: %w", err)
	}

	j.logger.Info("cgroup namespace mount setup complete")
	return nil
}

func (j *jobInitializer) remountProc() error {
	if err := j.syscallInterface.Mount("proc", "/proc", "proc", 0, ""); err != nil {
		if unmountErr := j.syscallInterface.Unmount("/proc", syscall.MNT_DETACH); unmountErr != nil {
			j.logger.Debug("/proc unmount failed", "error", unmountErr)
		}

		if err := j.syscallInterface.Mount("proc", "/proc", "proc", 0, ""); err != nil {
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

	pid := j.osInterface.Getpid()
	pidBytes := []byte(strconv.Itoa(pid))

	log := j.logger.WithFields(
		"pid", pid,
		"hostCgroupPath", cgroupPath,
		"namespaceCgroupPath", namespaceCgroupPath)

	for i := 0; i < 5; i++ {
		if err := j.osInterface.WriteFile(procsFile, pidBytes, 0644); err != nil {
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
		if _, err := j.osInterface.Stat(command); err != nil {
			return "", fmt.Errorf("command %s not found: %w", command, err)
		}
		return command, nil
	}

	if resolvedPath, err := j.execInterface.LookPath(command); err == nil {
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
		if _, err := j.osInterface.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("command %s not found in PATH or common locations", command)
}
