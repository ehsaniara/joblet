//go:build linux

package jobinit

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"worker/pkg/logger"
	"worker/pkg/os"
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

	jobLogger := j.logger.WithField("jobId", jobID)

	// Debug: Log all environment variables we're checking
	jobLogger.Info("checking required environment variables",
		"JOB_ID", jobID,
		"JOB_COMMAND", command,
		"JOB_CGROUP_PATH", cgroupPath,
		"JOB_ISOLATED_ROOT", j.osInterface.Getenv("JOB_ISOLATED_ROOT"))

	// Improved error checking with specific missing variables
	var missingVars []string
	if jobID == "" {
		missingVars = append(missingVars, "JOB_ID")
	}
	if command == "" {
		missingVars = append(missingVars, "JOB_COMMAND")
	}
	if cgroupPath == "" {
		missingVars = append(missingVars, "JOB_CGROUP_PATH")
	}

	if len(missingVars) > 0 {
		jobLogger.Error("missing required environment variables", "missing", missingVars)
		return nil, fmt.Errorf("missing required environment variables: %v", missingVars)
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

	jobLogger.Info("loaded job configuration with filesystem isolation",
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

func (j *jobInitializer) ExecuteJob(config *JobConfig) error {
	log := j.logger.WithField("jobId", config.JobID)

	if config == nil {
		return fmt.Errorf("job config cannot be nil")
	}

	log.Info("executing job with filesystem isolation", "command", config.Command)

	// Setup basic namespace environment with filesystem isolation
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

	// Execute the command (in isolated filesystem)
	execArgs := append([]string{config.Command}, config.Args...)
	log.Info("executing command in isolated environment", "commandPath", commandPath)

	if e := j.syscallInterface.Exec(commandPath, execArgs, j.osInterface.Environ()); e != nil {
		return fmt.Errorf("failed to exec command: %w", e)
	}

	return nil
}

func (j *jobInitializer) Run() error {
	j.logger.Info("job-init starting with filesystem isolation")

	if err := j.setupBasicEnvironment(); err != nil {
		return err
	}

	config, err := j.LoadConfigFromEnv()
	if err != nil {
		return err
	}

	return j.ExecuteJob(config)
}

// Just change working directory and setup environment
func (j *jobInitializer) setupBasicEnvironment() error {
	pid := j.osInterface.Getpid()
	j.logger.Info("setting up basic environment with filesystem isolation", "pid", pid)

	// Setup simple filesystem isolation (working directory + environment)
	if err := j.setupFilesystemIsolation(); err != nil {
		return fmt.Errorf("filesystem isolation failed: %w", err)
	}

	j.logger.Info("basic environment with filesystem isolation complete")
	return nil
}

func (j *jobInitializer) setupFilesystemIsolation() error {
	jobID := j.osInterface.Getenv("JOB_ID")
	isolatedRoot := j.osInterface.Getenv("JOB_ISOLATED_ROOT")

	if isolatedRoot == "" {
		return fmt.Errorf("JOB_ISOLATED_ROOT environment variable not set")
	}

	j.logger.Info("setting up conservative filesystem isolation", "jobID", jobID, "root", isolatedRoot)

	// Simply change to the isolated working directory
	workDir := filepath.Join(isolatedRoot, "work")
	if err := j.osInterface.Chdir(workDir); err != nil {
		return fmt.Errorf("failed to change to isolated work directory %s: %w", workDir, err)
	}

	// Update PATH to prioritize isolated binaries
	if err := j.setupIsolatedPath(isolatedRoot); err != nil {
		return fmt.Errorf("failed to setup isolated PATH: %w", err)
	}

	// Set up isolated environment variables with helpful paths
	j.setupConservativeIsolatedEnvironment(isolatedRoot)

	j.logger.Info("conservative filesystem isolation complete", "workDir", workDir)
	return nil
}

func (j *jobInitializer) setupConservativeIsolatedEnvironment(isolatedRoot string) {
	jobID := j.osInterface.Getenv("JOB_ID")
	workDir := filepath.Join(isolatedRoot, "work")

	// Set environment variables for isolated environment
	j.osInterface.Setenv("HOME", workDir)
	j.osInterface.Setenv("TMPDIR", filepath.Join(isolatedRoot, "tmp"))
	j.osInterface.Setenv("TMP", filepath.Join(isolatedRoot, "tmp"))
	j.osInterface.Setenv("TEMP", filepath.Join(isolatedRoot, "tmp"))

	// Set PWD to current working directory
	j.osInterface.Setenv("PWD", workDir)
	j.osInterface.Setenv("OLDPWD", "/")

	// Provide work directory paths for scripts that need absolute paths
	j.osInterface.Setenv("WORK_DIR", workDir)
	j.osInterface.Setenv("JOB_WORK_DIR", workDir)
	j.osInterface.Setenv("WORKSPACE", workDir)

	// Mark as isolated environment
	j.osInterface.Setenv("WORKER_ISOLATED", "true")
	j.osInterface.Setenv("WORKER_ISOLATED_ROOT", isolatedRoot)
	j.osInterface.Setenv("WORKER_JOB_ID", jobID)
	j.osInterface.Setenv("FILESYSTEM_VIRTUALIZED", "false")

	j.logger.Info("conservative isolated environment variables set")
}

// Setup additional bind mounts for common directories
func (j *jobInitializer) setupAdditionalBindMounts(isolatedRoot string) error {
	bindMounts := map[string]string{
		// Virtual path -> Isolated path
		"/tmp":     filepath.Join(isolatedRoot, "tmp"),
		"/var/tmp": filepath.Join(isolatedRoot, "var/tmp"),
		"/home":    filepath.Join(isolatedRoot, "home"),
	}

	for virtualPath, isolatedPath := range bindMounts {
		// Create virtual directory
		if err := j.osInterface.MkdirAll(virtualPath, 0755); err != nil {
			j.logger.Debug("failed to create virtual directory", "path", virtualPath, "error", err)
			continue
		}

		// Bind mount isolated path to virtual path
		if err := j.syscallInterface.Mount(isolatedPath, virtualPath, "", syscall.MS_BIND, ""); err != nil {
			j.logger.Debug("failed to bind mount", "virtual", virtualPath, "isolated", isolatedPath, "error", err)
			continue
		}

		// Make mount private
		if err := j.syscallInterface.Mount("", virtualPath, "", syscall.MS_PRIVATE, ""); err != nil {
			j.logger.Debug("failed to make mount private", "path", virtualPath, "error", err)
		}

		j.logger.Debug("bind mount created", "virtual", virtualPath, "isolated", isolatedPath)
	}

	return nil
}

// Setup isolated PATH
func (j *jobInitializer) setupIsolatedPath(isolatedRoot string) error {
	// Build new PATH with isolated directories first
	isolatedPaths := []string{
		filepath.Join(isolatedRoot, "bin"),
		filepath.Join(isolatedRoot, "usr/bin"),
	}

	// Get current PATH
	currentPath := j.osInterface.Getenv("PATH")

	// Build new PATH: isolated paths + current path
	var pathComponents []string
	for _, isolatedPath := range isolatedPaths {
		// Only add if directory exists
		if _, err := j.osInterface.Stat(isolatedPath); err == nil {
			pathComponents = append(pathComponents, isolatedPath)
		}
	}

	// Add original PATH as fallback
	if currentPath != "" {
		pathComponents = append(pathComponents, currentPath)
	}

	newPath := strings.Join(pathComponents, ":")
	j.osInterface.Setenv("PATH", newPath)

	j.logger.Info("PATH updated for isolation", "newPath", newPath)
	return nil
}

// Setup isolated environment variables
func (j *jobInitializer) setupIsolatedEnvironment(isolatedRoot string) {
	jobID := j.osInterface.Getenv("JOB_ID")

	// Set environment variables for the virtualized filesystem
	j.osInterface.Setenv("HOME", "/home")  // Virtualized home
	j.osInterface.Setenv("TMPDIR", "/tmp") // Virtualized tmp
	j.osInterface.Setenv("TMP", "/tmp")    // Virtualized tmp
	j.osInterface.Setenv("TEMP", "/tmp")   // Virtualized tmp

	// Set PWD to virtualized work directory
	j.osInterface.Setenv("PWD", "/work") // Virtualized work
	j.osInterface.Setenv("OLDPWD", "/")

	// Provide both virtualized and real paths for flexibility
	j.osInterface.Setenv("WORK_DIR", "/work")                                  // Virtualized
	j.osInterface.Setenv("REAL_WORK_DIR", filepath.Join(isolatedRoot, "work")) // Real path

	// Mark as isolated environment
	j.osInterface.Setenv("WORKER_ISOLATED", "true")
	j.osInterface.Setenv("WORKER_ISOLATED_ROOT", isolatedRoot)
	j.osInterface.Setenv("WORKER_JOB_ID", jobID)
	j.osInterface.Setenv("FILESYSTEM_VIRTUALIZED", "true")

	j.logger.Info("virtualized environment variables set")
}

// Setup cgroup namespace mount
func (j *jobInitializer) setupCgroupNamespace() error {
	j.logger.Info("verifying cgroup namespace setup")

	// Verify cgroup filesystem is accessible
	if _, err := j.osInterface.Stat("/sys/fs/cgroup/cgroup.procs"); err != nil {
		j.logger.Warn("cgroup.procs not immediately available, will be handled by joinCgroup", "error", err)
		// Don't fail here - joinCgroup will handle this later
	} else {
		j.logger.Info("cgroup filesystem is accessible in namespace")
	}

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
	// Check if we're in a user namespace
	inUserNS := j.osInterface.Getenv("USER_NAMESPACE") == "true"

	if inUserNS {
		j.logger.Info("user namespace detected - skipping manual cgroup join (parent process handles cgroup assignment)")
		return nil
	}

	// Original cgroup join logic for non-user-namespace jobs
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
