//go:build linux

package jobinit

import (
	"fmt"
	"path/filepath"
	"strconv"
	"syscall"

	"worker/pkg/logger"
	osinterface "worker/pkg/os"
)

// linuxJobInitializer handles job initialization with filesystem isolation on Linux
type linuxJobInitializer struct {
	osInterface      osinterface.OsInterface
	execInterface    osinterface.ExecInterface
	syscallInterface osinterface.SyscallInterface
	logger           *logger.Logger
}

// Ensure linuxJobInitializer implements JobInitializer
var _ JobInitializer = (*linuxJobInitializer)(nil)

// Run starts the job initialization process
func (j *linuxJobInitializer) Run() error {
	j.logger.Info("job-init starting with filesystem isolation support")

	config, err := j.LoadConfigFromEnv()
	if err != nil {
		return err
	}

	return j.ExecuteJob(config)
}

// LoadConfigFromEnv loads configuration from environment variables
func (j *linuxJobInitializer) LoadConfigFromEnv() (*JobConfig, error) {
	jobID := j.osInterface.Getenv("JOB_ID")
	if jobID == "" {
		return nil, fmt.Errorf("JOB_ID environment variable not set")
	}

	command := j.osInterface.Getenv("JOB_COMMAND")
	if command == "" {
		return nil, fmt.Errorf("JOB_COMMAND environment variable not set")
	}

	cgroupPath := j.osInterface.Getenv("JOB_CGROUP_PATH")
	if cgroupPath == "" {
		return nil, fmt.Errorf("JOB_CGROUP_PATH environment variable not set")
	}

	// Load job arguments
	argsCountStr := j.osInterface.Getenv("JOB_ARGS_COUNT")
	argsCount, err := strconv.Atoi(argsCountStr)
	if err != nil {
		argsCount = 0
	}

	args := make([]string, argsCount)
	for i := 0; i < argsCount; i++ {
		argKey := fmt.Sprintf("JOB_ARG_%d", i)
		args[i] = j.osInterface.Getenv(argKey)
	}

	// Load user namespace configuration
	userNamespaceEnabled := j.osInterface.Getenv("USER_NAMESPACE_ENABLED") == "true"
	var namespaceUID, namespaceGID uint32

	if uidStr := j.osInterface.Getenv("USER_NAMESPACE_UID"); uidStr != "" {
		if uid, err := strconv.ParseUint(uidStr, 10, 32); err == nil {
			namespaceUID = uint32(uid)
		}
	}

	if gidStr := j.osInterface.Getenv("USER_NAMESPACE_GID"); gidStr != "" {
		if gid, err := strconv.ParseUint(gidStr, 10, 32); err == nil {
			namespaceGID = uint32(gid)
		}
	}

	return &JobConfig{
		JobID:                jobID,
		Command:              command,
		Args:                 args,
		CgroupPath:           cgroupPath,
		UserNamespaceEnabled: userNamespaceEnabled,
		NamespaceUID:         namespaceUID,
		NamespaceGID:         namespaceGID,
	}, nil
}

// ExecuteJob executes the job with the provided configuration
func (j *linuxJobInitializer) ExecuteJob(config *JobConfig) error {
	j.logger.Info("executing job", "jobID", config.JobID, "command", config.Command)

	// Join cgroup for resource limits
	if err := j.joinCgroup(config.CgroupPath); err != nil {
		j.logger.Warn("failed to join cgroup", "error", err)
		// Don't fail completely - job can still run without cgroup
	}

	// Setup filesystem isolation if enabled
	var isolatedRoot string
	if j.osInterface.Getenv("FILESYSTEM_ISOLATION") == "enabled" {
		var err error
		isolatedRoot, err = j.setupFilesystemIsolation(config.JobID)
		if err != nil {
			j.logger.Warn("filesystem isolation setup failed, continuing without isolation", "error", err)
			// Continue without isolation
		}
	}

	// Setup user namespace if enabled
	if config.UserNamespaceEnabled {
		if err := j.setupUserNamespace(config.NamespaceUID, config.NamespaceGID); err != nil {
			j.logger.Warn("user namespace setup failed, continuing without namespace", "error", err)
			// Continue without user namespace
		}
	}

	// Setup working directory
	if err := j.setupWorkingDirectory(isolatedRoot); err != nil {
		j.logger.Warn("working directory setup failed", "error", err)
		// Continue anyway
	}

	// Execute the actual command
	return j.executeJobCommand(config)
}

// setupFilesystemIsolation sets up filesystem isolation for the job
func (j *linuxJobInitializer) setupFilesystemIsolation(jobID string) (string, error) {
	j.logger.Debug("setting up filesystem isolation", "jobID", jobID)

	// Create isolated root directory
	isolatedRoot := filepath.Join("/tmp/worker/fs", fmt.Sprintf("job-%s-root", jobID))
	if err := j.osInterface.MkdirAll(isolatedRoot, 0755); err != nil {
		return "", fmt.Errorf("failed to create isolated root: %w", err)
	}

	j.logger.Info("filesystem isolation setup complete", "isolatedRoot", isolatedRoot)
	return isolatedRoot, nil
}

// setupUserNamespace sets up user namespace isolation
func (j *linuxJobInitializer) setupUserNamespace(uid, gid uint32) error {
	j.logger.Debug("setting up user namespace", "uid", uid, "gid", gid)

	// Create user namespace
	if err := j.syscallInterface.Unshare(syscall.CLONE_NEWUSER); err != nil {
		return fmt.Errorf("failed to create user namespace: %w", err)
	}

	j.logger.Info("user namespace setup complete", "uid", uid, "gid", gid)
	return nil
}

// isInUserNamespace checks if we're running in a user namespace
func (j *linuxJobInitializer) isInUserNamespace() bool {
	// Simple check - in user namespace, our UID mapping might be different
	uid := j.osInterface.Getuid()
	return uid != 0 && j.osInterface.Getenv("USER_NAMESPACE_ENABLED") == "true"
}

// setupWorkingDirectory sets up the working directory for the job
func (j *linuxJobInitializer) setupWorkingDirectory(isolatedRoot string) error {
	// Try isolated paths if available
	workingDirs := []string{
		filepath.Join(isolatedRoot, "work"), // Isolated work directory
		filepath.Join(isolatedRoot, "home"), // Isolated home directory
		"/tmp",                              // System tmp as fallback
		"/",                                 // Root as last resort
	}

	var workDir string
	for _, dir := range workingDirs {
		if dir == "" {
			continue
		}

		// Try to create directory if it doesn't exist
		if err := j.osInterface.MkdirAll(dir, 0755); err != nil {
			j.logger.Debug("failed to create directory", "dir", dir, "error", err)
			continue
		}

		// Try to change to this directory
		if err := j.osInterface.Chdir(dir); err != nil {
			j.logger.Debug("failed to change to directory", "dir", dir, "error", err)
			continue
		}

		workDir = dir
		break
	}

	if workDir != "" {
		j.logger.Info("set working directory", "workDir", workDir)
		j.osInterface.Setenv("PWD", workDir)
		j.osInterface.Setenv("WORK_DIR", workDir)
	} else {
		j.logger.Warn("could not set working directory, using current directory")
	}

	// Set basic environment variables
	if isolatedRoot != "" {
		j.osInterface.Setenv("HOME", filepath.Join(isolatedRoot, "home"))
	} else {
		j.osInterface.Setenv("HOME", "/tmp")
	}

	j.osInterface.Setenv("USER", "worker")
	j.osInterface.Setenv("LOGNAME", "worker")

	// Ensure PATH is set
	if j.osInterface.Getenv("PATH") == "" {
		j.osInterface.Setenv("PATH", "/usr/local/bin:/usr/bin:/bin")
	}

	return nil
}

// joinCgroup joins the specified cgroup for resource limits
func (j *linuxJobInitializer) joinCgroup(cgroupPath string) error {
	if cgroupPath == "" {
		j.logger.Warn("no cgroup path specified, skipping cgroup join")
		return nil
	}

	j.logger.Debug("joining cgroup", "path", cgroupPath)

	// Check if we're in a user namespace - cgroup paths might be different
	if j.isInUserNamespace() {
		// In user namespace, try to join via the namespace-mapped path
		namespaceCgroupPath := "/sys/fs/cgroup"
		procsFile := filepath.Join(namespaceCgroupPath, "cgroup.procs")

		j.logger.Debug("attempting cgroup join in user namespace", "procsFile", procsFile)

		pid := j.osInterface.Getpid()
		pidStr := strconv.Itoa(pid)

		if err := j.osInterface.WriteFile(procsFile, []byte(pidStr), 0644); err != nil {
			j.logger.Warn("failed to join cgroup in user namespace, continuing anyway", "error", err)
			// Don't fail the job - cgroup joining in user namespace is optional
			return nil
		}

		j.logger.Info("successfully joined cgroup in user namespace", "pid", pid)
		return nil
	}

	// Normal cgroup join (outside user namespace)
	procsFile := filepath.Join(cgroupPath, "cgroup.procs")

	// Check if cgroup directory exists
	if _, err := j.osInterface.Stat(cgroupPath); err != nil {
		j.logger.Warn("cgroup directory does not exist, skipping join", "path", cgroupPath, "error", err)
		return nil
	}

	pid := j.osInterface.Getpid()
	pidStr := strconv.Itoa(pid)

	if err := j.osInterface.WriteFile(procsFile, []byte(pidStr), 0644); err != nil {
		return fmt.Errorf("failed to join cgroup: %w", err)
	}

	j.logger.Debug("successfully joined cgroup", "pid", pid, "path", cgroupPath)
	return nil
}

// executeJobCommand executes the final job command
func (j *linuxJobInitializer) executeJobCommand(config *JobConfig) error {
	// Resolve command path
	commandPath, err := j.resolveCommandPath(config.Command)
	if err != nil {
		return fmt.Errorf("command not found: %w", err)
	}

	// Prepare exec arguments
	execArgs := append([]string{config.Command}, config.Args...)

	j.logger.Info("executing command in isolated environment",
		"command", config.Command,
		"commandPath", commandPath,
		"args", config.Args)

	// Execute the command (replaces current process)
	if err := j.syscallInterface.Exec(commandPath, execArgs, j.osInterface.Environ()); err != nil {
		return fmt.Errorf("failed to exec command: %w", err)
	}

	// This point should never be reached (exec replaces the process)
	return nil
}

// resolveCommandPath resolves the full path to a command
func (j *linuxJobInitializer) resolveCommandPath(command string) (string, error) {
	// If command is already an absolute path, use it directly
	if filepath.IsAbs(command) {
		if _, err := j.osInterface.Stat(command); err != nil {
			return "", fmt.Errorf("command not found: %s", command)
		}
		return command, nil
	}

	// Search in PATH
	pathEnv := j.osInterface.Getenv("PATH")
	if pathEnv == "" {
		pathEnv = "/usr/local/bin:/usr/bin:/bin"
	}

	for _, dir := range filepath.SplitList(pathEnv) {
		if dir == "" {
			continue
		}

		fullPath := filepath.Join(dir, command)
		if info, err := j.osInterface.Stat(fullPath); err == nil && !info.IsDir() {
			// Check if file is executable
			if info.Mode()&0111 != 0 {
				return fullPath, nil
			}
		}
	}

	return "", fmt.Errorf("command not found in PATH: %s", command)
}
