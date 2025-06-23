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

// NewLinuxJobInitializer creates a new Linux job initializer
func NewLinuxJobInitializer() JobInitializer {
	osInterface := &osinterface.DefaultOs{}
	syscallInterface := &osinterface.DefaultSyscall{}
	execInterface := &osinterface.DefaultExec{}

	return &linuxJobInitializer{
		osInterface:      osInterface,
		execInterface:    execInterface,
		syscallInterface: syscallInterface,
		logger:           logger.New().WithField("component", "job-init-linux"),
	}
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

// ExecuteJob executes a job with filesystem isolation
func (j *linuxJobInitializer) ExecuteJob(config *JobConfig) error {
	j.logger.Info("STARTING JOB EXECUTION WITH FILESYSTEM ISOLATION",
		"jobID", config.JobID,
		"command", config.Command)

	// Check for process phase to determine execution mode
	processPhase := j.osInterface.Getenv("PROCESS_PHASE")

	switch processPhase {
	case "SINGLE_PROCESS":
		return j.executeSingleProcessJob(config)
	case "MOUNT_SETUP":
		return j.executeMountSetupJob(config)
	case "JOB_EXECUTION":
		return j.executeJobExecutionJob(config)
	default:
		// Fallback to standard approach
		j.logger.Info("using standard job execution approach")
		return j.executeStandardJob(config)
	}
}

// executeSingleProcessJob handles single-process approach with transparent mounts
func (j *linuxJobInitializer) executeSingleProcessJob(config *JobConfig) error {
	j.logger.Info("EXECUTING SINGLE-PROCESS JOB WITH TRANSPARENT ISOLATION")

	// Step 1: Setup filesystem isolation FIRST (while we have capabilities)
	if err := j.setupTransparentFilesystemIsolation(); err != nil {
		j.logger.Warn("transparent filesystem isolation failed, continuing anyway", "error", err)
	}

	// Step 2: Create user namespace for security (after mounts are set up)
	if err := j.setupUserNamespace(); err != nil {
		j.logger.Warn("user namespace setup failed, continuing without user isolation", "error", err)
	}

	// Step 3: Setup basic environment
	if err := j.setupBasicEnvironment(); err != nil {
		return fmt.Errorf("failed to setup basic environment: %w", err)
	}

	// Step 4: Join cgroup for resource limits
	if err := j.joinCgroup(config.CgroupPath); err != nil {
		return fmt.Errorf("failed to join cgroup: %w", err)
	}

	// Step 5: Execute the job command
	j.logger.Info("EXECUTING JOB COMMAND")
	return j.executeJobCommand(config)
}

// setupTransparentFilesystemIsolation sets up bind mounts so /tmp appears isolated to applications
func (j *linuxJobInitializer) setupTransparentFilesystemIsolation() error {
	j.logger.Info("setting up transparent filesystem isolation")

	isolatedRoot := j.osInterface.Getenv("JOB_ISOLATED_ROOT")
	if isolatedRoot == "" {
		return fmt.Errorf("JOB_ISOLATED_ROOT not set")
	}

	// Make root filesystem private to prevent mount propagation
	if err := j.makeRootPrivate(); err != nil {
		j.logger.Warn("failed to make root private", "error", err)
		// Continue anyway - this is just best practice
	}

	// Setup bind mounts for transparent isolation
	bindMounts := map[string]string{
		"/tmp":     filepath.Join(isolatedRoot, "tmp"),
		"/var/tmp": filepath.Join(isolatedRoot, "var/tmp"),
		"/home":    filepath.Join(isolatedRoot, "home"),
	}

	successCount := 0
	for virtualPath, isolatedPath := range bindMounts {
		if err := j.setupBindMount(virtualPath, isolatedPath); err != nil {
			j.logger.Error("bind mount failed", "virtual", virtualPath, "isolated", isolatedPath, "error", err)
			continue
		}
		successCount++
		j.logger.Info("mounted isolated directory", "virtual", virtualPath, "isolated", isolatedPath)
	}

	if successCount > 0 {
		j.osInterface.Setenv("ISOLATION_METHOD", "bind_mount")
		j.osInterface.Setenv("FILESYSTEM_VIRTUALIZED", "true")
		j.logger.Info("TRANSPARENT FILESYSTEM ISOLATION ACTIVE", "mounts", successCount)
		return nil
	}

	// If no mounts succeeded, fall back to environment isolation
	j.logger.Warn("all bind mounts failed, falling back to environment isolation")
	return j.setupFallbackIsolation(isolatedRoot)
}

// setupUserNamespace creates user namespace after mounts are set up
func (j *linuxJobInitializer) setupUserNamespace() error {
	userNSUID := j.osInterface.Getenv("USER_NAMESPACE_UID")
	userNSGID := j.osInterface.Getenv("USER_NAMESPACE_GID")
	hostUID := j.osInterface.Getenv("USER_HOST_UID")
	hostGID := j.osInterface.Getenv("USER_HOST_GID")

	if userNSUID == "" || hostUID == "" {
		j.logger.Info("user namespace mapping not provided, skipping user namespace setup")
		return nil
	}

	j.logger.Info("setting up user namespace", "nsUID", userNSUID, "hostUID", hostUID)

	// Unshare user namespace
	if err := j.syscallInterface.Unshare(syscall.CLONE_NEWUSER); err != nil {
		return fmt.Errorf("failed to unshare user namespace: %w", err)
	}

	// Write UID mapping
	uidMapContent := fmt.Sprintf("%s %s 1", userNSUID, hostUID)
	if err := j.osInterface.WriteFile("/proc/self/uid_map", []byte(uidMapContent), 0644); err != nil {
		return fmt.Errorf("failed to write uid_map: %w", err)
	}

	// Deny setgroups (required for GID mapping)
	if err := j.osInterface.WriteFile("/proc/self/setgroups", []byte("deny"), 0644); err != nil {
		j.logger.Warn("failed to deny setgroups", "error", err)
	}

	// Write GID mapping
	if userNSGID != "" && hostGID != "" {
		gidMapContent := fmt.Sprintf("%s %s 1", userNSGID, hostGID)
		if err := j.osInterface.WriteFile("/proc/self/gid_map", []byte(gidMapContent), 0644); err != nil {
			return fmt.Errorf("failed to write gid_map: %w", err)
		}
	}

	j.logger.Info("user namespace configured", "nsUID", userNSUID, "nsGID", userNSGID)
	return nil
}

// executeStandardJob implements standard job execution
func (j *linuxJobInitializer) executeStandardJob(config *JobConfig) error {
	j.logger.Info("EXECUTING STANDARD JOB")

	// Setup basic environment
	if err := j.setupBasicEnvironment(); err != nil {
		return fmt.Errorf("failed to setup basic environment: %w", err)
	}

	// Join cgroup
	if err := j.joinCgroup(config.CgroupPath); err != nil {
		return fmt.Errorf("failed to join cgroup: %w", err)
	}

	// Execute command
	return j.executeJobCommand(config)
}

// executeMountSetupJob handles mount setup phase (for two-process approach)
func (j *linuxJobInitializer) executeMountSetupJob(config *JobConfig) error {
	j.logger.Info("EXECUTING MOUNT SETUP JOB")
	// Implementation would go here for two-process approach
	// For now, just setup filesystem isolation and keep alive

	if err := j.setupFilesystemIsolation(); err != nil {
		return fmt.Errorf("mount setup failed: %w", err)
	}

	// Keep process alive for namespace sharing
	j.logger.Info("mount setup complete, keeping process alive")
	select {} // Block forever
}

// executeJobExecutionJob handles job execution phase (for two-process approach)
func (j *linuxJobInitializer) executeJobExecutionJob(config *JobConfig) error {
	j.logger.Info("EXECUTING JOB EXECUTION JOB")

	// Setup basic environment
	if err := j.setupBasicEnvironment(); err != nil {
		return fmt.Errorf("failed to setup basic environment: %w", err)
	}

	// Join cgroup
	if err := j.joinCgroup(config.CgroupPath); err != nil {
		return fmt.Errorf("failed to join cgroup: %w", err)
	}

	// Execute command
	return j.executeJobCommand(config)
}

// setupFilesystemIsolation sets up bind mounts for filesystem isolation
func (j *linuxJobInitializer) setupFilesystemIsolation() error {
	j.logger.Info("setting up filesystem isolation")

	isolatedRoot := j.osInterface.Getenv("JOB_ISOLATED_ROOT")
	if isolatedRoot == "" {
		return fmt.Errorf("JOB_ISOLATED_ROOT not set")
	}

	// Check if we're in a user namespace (which limits mount capabilities)
	if j.isInUserNamespace() {
		j.logger.Info("detected user namespace - using fallback isolation approach")
		return j.setupFallbackIsolation(isolatedRoot)
	}

	// Try full mount isolation if we have capabilities
	return j.setupMountIsolation(isolatedRoot)
}

// isInUserNamespace checks if we're running in a user namespace
func (j *linuxJobInitializer) isInUserNamespace() bool {
	// Check if we have USER_NAMESPACE environment variable set
	if j.osInterface.Getenv("USER_NAMESPACE_UID") != "" {
		return true
	}

	// Check if our UID is mapped (simple heuristic)
	if j.osInterface.Getuid() == 0 {
		// We're root - check if it's a mapped root in user namespace
		userNS := j.osInterface.Getenv("USER_NAMESPACE_ENABLED")
		return userNS == "true"
	}

	return false
}

// setupFallbackIsolation sets up isolation without mount operations
func (j *linuxJobInitializer) setupFallbackIsolation(isolatedRoot string) error {
	j.logger.Info("setting up fallback filesystem isolation", "isolatedRoot", isolatedRoot)

	// Set environment variables to use isolated paths
	isolatedPaths := map[string]string{
		"TMPDIR": filepath.Join(isolatedRoot, "tmp"),
		"TMP":    filepath.Join(isolatedRoot, "tmp"),
		"TEMP":   filepath.Join(isolatedRoot, "tmp"),
		"HOME":   filepath.Join(isolatedRoot, "home"),
	}

	for envVar, isolatedPath := range isolatedPaths {
		// Ensure directory exists
		if err := j.osInterface.MkdirAll(isolatedPath, 0777); err != nil {
			j.logger.Warn("failed to create isolated directory", "path", isolatedPath, "error", err)
			continue
		}

		// Set environment variable
		j.osInterface.Setenv(envVar, isolatedPath)
		j.logger.Debug("set isolated environment", "var", envVar, "path", isolatedPath)
	}

	// Set working directory to isolated location
	workDir := filepath.Join(isolatedRoot, "work")
	if err := j.osInterface.MkdirAll(workDir, 0755); err == nil {
		j.osInterface.Setenv("PWD", workDir)
		j.osInterface.Setenv("WORK_DIR", workDir)
	}

	j.osInterface.Setenv("ISOLATION_METHOD", "environment")
	j.osInterface.Setenv("FILESYSTEM_VIRTUALIZED", "partial")
	j.logger.Info("FALLBACK ISOLATION ACTIVE (environment variables)")

	return nil
}

// setupMountIsolation sets up full mount-based isolation
func (j *linuxJobInitializer) setupMountIsolation(isolatedRoot string) error {
	j.logger.Info("setting up mount-based filesystem isolation", "isolatedRoot", isolatedRoot)

	// Make root filesystem private to prevent mount propagation
	if err := j.makeRootPrivate(); err != nil {
		j.logger.Warn("failed to make root private, continuing anyway", "error", err)
	}

	// Setup bind mounts for key directories
	bindMounts := map[string]string{
		"/tmp":     filepath.Join(isolatedRoot, "tmp"),
		"/var/tmp": filepath.Join(isolatedRoot, "var/tmp"),
		"/home":    filepath.Join(isolatedRoot, "home"),
	}

	successCount := 0
	for virtualPath, isolatedPath := range bindMounts {
		if err := j.setupBindMount(virtualPath, isolatedPath); err != nil {
			j.logger.Error("bind mount failed", "virtual", virtualPath, "isolated", isolatedPath, "error", err)
			continue
		}
		successCount++
	}

	j.logger.Info("bind mount setup complete",
		"successful", successCount,
		"total", len(bindMounts))

	if successCount > 0 {
		j.osInterface.Setenv("ISOLATION_METHOD", "bind_mount")
		j.osInterface.Setenv("FILESYSTEM_VIRTUALIZED", "true")
		j.logger.Info("MOUNT ISOLATION ACTIVE (bind mounts)")
		return nil
	}

	// If bind mounts failed, fall back to environment-based isolation
	j.logger.Warn("bind mounts failed, falling back to environment isolation")
	return j.setupFallbackIsolation(isolatedRoot)
}

// makeRootPrivate makes the root filesystem mount private
func (j *linuxJobInitializer) makeRootPrivate() error {
	j.logger.Debug("making root filesystem private")

	if err := j.syscallInterface.Mount("", "/", "", syscall.MS_PRIVATE|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("failed to make root private: %w", err)
	}

	return nil
}

// setupBindMount sets up a single bind mount
func (j *linuxJobInitializer) setupBindMount(virtualPath, isolatedPath string) error {
	// Ensure isolated source directory exists
	if err := j.osInterface.MkdirAll(isolatedPath, 0777); err != nil {
		return fmt.Errorf("failed to create isolated directory %s: %w", isolatedPath, err)
	}

	// Ensure virtual target directory exists
	if err := j.osInterface.MkdirAll(virtualPath, 0755); err != nil {
		return fmt.Errorf("failed to create virtual directory %s: %w", virtualPath, err)
	}

	// Perform the bind mount
	if err := j.syscallInterface.Mount(isolatedPath, virtualPath, "", syscall.MS_BIND, ""); err != nil {
		return fmt.Errorf("bind mount operation failed: %w", err)
	}

	// Make the mount private
	if err := j.syscallInterface.Mount("", virtualPath, "", syscall.MS_PRIVATE, ""); err != nil {
		j.logger.Warn("failed to make mount private", "path", virtualPath, "error", err)
	}

	j.logger.Debug("bind mount successful", "virtual", virtualPath, "isolated", isolatedPath)
	return nil
}

// setupBasicEnvironment configures the basic process environment
func (j *linuxJobInitializer) setupBasicEnvironment() error {
	j.logger.Debug("setting up basic environment")

	// Get isolated root for proper path setup
	isolatedRoot := j.osInterface.Getenv("JOB_ISOLATED_ROOT")

	// Try to set working directory - use isolated paths if available
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

// NewDarwinJobInitializer creates a Darwin job initializer (placeholder)
func NewDarwinJobInitializer() JobInitializer {
	// Return the existing Darwin implementation
	// This should be defined in init_darwin.go
	panic("Darwin implementation should be in init_darwin.go")
}
