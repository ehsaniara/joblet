//go:build linux

package process

import (
	"context"
	"errors"
	"fmt"
	"io"
	"joblet/internal/joblet/core/upload"
	"joblet/internal/joblet/domain"
	"joblet/pkg/platform"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"joblet/pkg/logger"
)

const (
	GracefulShutdownTimeout = 100 * time.Millisecond
	StartTimeout            = 10 * time.Second
	SmallFileThreshold      = 1024 * 1024 // 1MB
)

// Manager handles all process-related operations including launching, cleanup, and validation
type Manager struct {
	platform      platform.Platform
	logger        *logger.Logger
	uploadManager *upload.Manager
}

// NewProcessManager creates a new unified process manager
func NewProcessManager(platform platform.Platform) *Manager {
	log := logger.New().WithField("component", "process-manager")
	return &Manager{
		platform:      platform,
		logger:        log,
		uploadManager: upload.NewManager(platform, log),
	}
}

// LaunchConfig contains all configuration for launching a process
type LaunchConfig struct {
	InitPath    string
	Environment []string
	SysProcAttr *syscall.SysProcAttr
	Stdout      io.Writer
	Stderr      io.Writer
	JobID       string
	Command     string
	Args        []string
}

// LaunchResult contains the result of a process launch
type LaunchResult struct {
	PID     int32
	Command platform.Command
	Error   error
}

// LaunchProcess launches a process with the given configuration
func (m *Manager) LaunchProcess(ctx context.Context, config *LaunchConfig) (*LaunchResult, error) {
	if config == nil {
		return nil, fmt.Errorf("launch config cannot be nil")
	}

	log := m.logger.WithFields("jobID", config.JobID, "command", config.Command)
	log.Debug("launching process")

	// Validate configuration
	if err := m.validateLaunchConfig(config); err != nil {
		return nil, fmt.Errorf("invalid launch config: %w", err)
	}

	// Use pre-fork namespace setup approach for network joining
	resultChan := make(chan *LaunchResult, 1)
	go m.launchInGoroutine(config, resultChan)

	// Wait for the goroutine to complete with timeout
	select {
	case result := <-resultChan:
		if result.Error != nil {
			log.Error("failed to start process in goroutine", "error", result.Error)
			return nil, fmt.Errorf("failed to start process: %w", result.Error)
		}
		log.Debug("process started successfully", "pid", result.PID)
		return result, nil
	case <-ctx.Done():
		log.Warn("context cancelled while starting process")
		return nil, ctx.Err()
	case <-time.After(StartTimeout):
		log.Error("timeout waiting for process to start")
		return nil, fmt.Errorf("timeout waiting for process to start")
	}
}

// launchInGoroutine launches the process in a separate goroutine with proper namespace handling
func (m *Manager) launchInGoroutine(config *LaunchConfig, resultChan chan<- *LaunchResult) {
	defer func() {
		if r := recover(); r != nil {
			resultChan <- &LaunchResult{
				Error: fmt.Errorf("panic in launch goroutine: %v", r),
			}
		}
	}()

	// Lock this goroutine to the OS thread for namespace operations
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Start the process (which will inherit the current namespace)
	cmd, err := m.createAndStartCommand(config)
	if err != nil {
		resultChan <- &LaunchResult{
			Error: fmt.Errorf("failed to start command: %w", err),
		}
		return
	}

	process := cmd.Process()
	if process == nil {
		resultChan <- &LaunchResult{
			Error: fmt.Errorf("process is nil after start"),
		}
		return
	}

	resultChan <- &LaunchResult{
		PID:     int32(process.Pid()),
		Command: cmd,
		Error:   nil,
	}
}

// createAndStartCommand creates and starts the command with proper configuration
func (m *Manager) createAndStartCommand(config *LaunchConfig) (platform.Command, error) {
	// Create command
	cmd := m.platform.CreateCommand(config.InitPath)

	// Set environment
	if config.Environment != nil {
		cmd.SetEnv(config.Environment)
	}

	// Set stdout/stderr
	if config.Stdout != nil {
		cmd.SetStdout(config.Stdout)
	}
	if config.Stderr != nil {
		cmd.SetStderr(config.Stderr)
	}

	// Set system process attributes (namespaces, process group, etc.)
	if config.SysProcAttr != nil {
		cmd.SetSysProcAttr(config.SysProcAttr)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	return cmd, nil
}

// CleanupRequest contains information needed for cleanup
type CleanupRequest struct {
	JobID           string
	PID             int32
	CgroupPath      string
	NetworkGroupID  string
	NamespacePath   string
	ForceKill       bool
	GracefulTimeout time.Duration
}

// CleanupResult contains the result of a cleanup operation
type CleanupResult struct {
	JobID            string
	ProcessKilled    bool
	CgroupCleaned    bool
	NamespaceRemoved bool
	Method           string // "graceful", "forced", "already_dead"
	Duration         time.Duration
	Errors           []error
}

// CleanupProcess performs comprehensive cleanup of a job process and its resources
func (m *Manager) CleanupProcess(ctx context.Context, req *CleanupRequest) (*CleanupResult, error) {
	if req == nil {
		return nil, fmt.Errorf("cleanup request cannot be nil")
	}

	if err := m.validateCleanupRequest(req); err != nil {
		return nil, fmt.Errorf("invalid cleanup request: %w", err)
	}

	log := m.logger.WithFields("jobID", req.JobID, "pid", req.PID)
	log.Debug("starting process cleanup", "forceKill", req.ForceKill, "gracefulTimeout", req.GracefulTimeout)

	result := &CleanupResult{
		JobID:  req.JobID,
		Errors: make([]error, 0),
	}

	// Handle process termination
	if req.PID > 0 {
		processResult := m.cleanupProcessAndGroup(ctx, req)
		result.ProcessKilled = processResult.Killed
		result.Method = processResult.Method
		if processResult.Error != nil {
			result.Errors = append(result.Errors, processResult.Error)
		}
	}

	// Cleanup namespace if it's an isolated job
	if req.NamespacePath != "" {
		if err := m.cleanupNamespace(req.NamespacePath, false); err != nil {
			log.Warn("failed to cleanup namespace", "path", req.NamespacePath, "error", err)
			result.Errors = append(result.Errors, fmt.Errorf("namespace cleanup failed: %w", err))
		} else {
			result.NamespaceRemoved = true
		}
	}

	if len(result.Errors) > 0 {
		log.Warn("cleanup completed with errors", "duration", result.Duration, "errorCount", len(result.Errors))
	} else {
		log.Debug("cleanup completed successfully", "duration", result.Duration)
	}

	return result, nil
}

// processCleanupResult contains the result of process cleanup
type processCleanupResult struct {
	Killed bool
	Method string
	Error  error
}

// cleanupProcessAndGroup handles process and process group cleanup
func (m *Manager) cleanupProcessAndGroup(ctx context.Context, req *CleanupRequest) *processCleanupResult {
	log := m.logger.WithFields("jobID", req.JobID, "pid", req.PID)

	// Check if process is still alive
	if !m.isProcessAlive(req.PID) {
		log.Debug("process already dead, no cleanup needed")
		return &processCleanupResult{
			Killed: false,
			Method: "already_dead",
			Error:  nil,
		}
	}

	// If force kill is requested, skip graceful shutdown
	if req.ForceKill {
		return m.forceKillProcess(req.PID, req.JobID)
	}

	// Try graceful shutdown first
	gracefulResult := m.attemptGracefulShutdown(req.PID, req.GracefulTimeout, req.JobID)
	if gracefulResult.Killed {
		return gracefulResult
	}

	// If graceful shutdown failed, force kill
	log.Warn("graceful shutdown failed, attempting force kill")
	return m.forceKillProcess(req.PID, req.JobID)
}

// attemptGracefulShutdown attempts to gracefully shut down a process
func (m *Manager) attemptGracefulShutdown(pid int32, timeout time.Duration, jobID string) *processCleanupResult {
	log := m.logger.WithFields("jobID", jobID, "pid", pid)

	if timeout <= 0 {
		timeout = GracefulShutdownTimeout
	}

	log.Debug("attempting graceful shutdown", "timeout", timeout)

	// Send SIGTERM to process group first
	if err := m.platform.Kill(-int(pid), syscall.SIGTERM); err != nil {
		log.Warn("failed to send SIGTERM to process group", "error", err)
		// If killing the group failed, try killing just the main process
		if err := m.platform.Kill(int(pid), syscall.SIGTERM); err != nil {
			log.Warn("failed to send SIGTERM to main process", "error", err)
			return &processCleanupResult{
				Killed: false,
				Method: "graceful_failed",
				Error:  fmt.Errorf("failed to send SIGTERM: %w", err),
			}
		}
	}

	// Wait for graceful shutdown
	log.Debug("waiting for graceful shutdown", "timeout", timeout)
	time.Sleep(timeout)

	// Check if process is still alive
	if !m.isProcessAlive(pid) {
		log.Debug("process terminated gracefully")
		return &processCleanupResult{
			Killed: true,
			Method: "graceful",
			Error:  nil,
		}
	}

	log.Debug("process still alive after graceful shutdown attempt")
	return &processCleanupResult{
		Killed: false,
		Method: "graceful_timeout",
		Error:  nil,
	}
}

// forceKillProcess force kills a process and its group
func (m *Manager) forceKillProcess(pid int32, jobID string) *processCleanupResult {
	log := m.logger.WithFields("jobID", jobID, "pid", pid)
	log.Warn("force killing process")

	// Send SIGKILL to process group
	if err := m.platform.Kill(-int(pid), syscall.SIGKILL); err != nil {
		log.Warn("failed to send SIGKILL to process group", "error", err)
		// Try killing just the main process
		if err := m.platform.Kill(int(pid), syscall.SIGKILL); err != nil {
			log.Error("failed to kill process", "error", err)
			return &processCleanupResult{
				Killed: false,
				Method: "force_failed",
				Error:  fmt.Errorf("failed to kill process: %w", err),
			}
		}
	}

	// Give it a moment for the kill to take effect
	time.Sleep(50 * time.Millisecond)

	// Verify the process is dead
	if m.isProcessAlive(pid) {
		log.Error("process still alive after SIGKILL")
		return &processCleanupResult{
			Killed: false,
			Method: "force_failed",
			Error:  fmt.Errorf("process still alive after SIGKILL"),
		}
	}

	log.Debug("process force killed successfully")
	return &processCleanupResult{
		Killed: true,
		Method: "forced",
		Error:  nil,
	}
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string
	Value   interface{}
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation error for field '%s' (value: %v): %s",
		e.Field, e.Value, e.Message)
}

// ValidateCommand validates a command string
func (m *Manager) ValidateCommand(command string) error {
	return m.validateCommand(command)
}

// ValidateArguments validates command arguments
func (m *Manager) ValidateArguments(args []string) error {
	return m.validateArguments(args)
}

// ResolveCommand resolves a command to its full path
func (m *Manager) ResolveCommand(command string) (string, error) {
	if command == "" {
		return "", fmt.Errorf("command cannot be empty")
	}

	log := m.logger.WithField("command", command)

	// If command is already absolute, validate it exists
	if filepath.IsAbs(command) {
		if _, err := m.platform.Stat(command); err != nil {
			log.Error("absolute command path not found", "error", err)
			return "", fmt.Errorf("command %s not found: %w", command, err)
		}
		log.Debug("using absolute command path")
		return command, nil
	}

	// Try to resolve using PATH
	if resolvedPath, err := m.platform.LookPath(command); err == nil {
		log.Debug("resolved command via PATH", "resolved", resolvedPath)
		return resolvedPath, nil
	}

	// Try common paths
	commonPaths := []string{
		filepath.Join("/bin", command),
		filepath.Join("/usr/bin", command),
		filepath.Join("/usr/local/bin", command),
		filepath.Join("/sbin", command),
		filepath.Join("/usr/sbin", command),
	}

	log.Debug("checking common command locations", "paths", commonPaths)

	for _, path := range commonPaths {
		if _, err := m.platform.Stat(path); err == nil {
			log.Debug("found command in common location", "path", path)
			return path, nil
		}
	}

	log.Error("command not found anywhere", "searchedPaths", commonPaths)
	return "", fmt.Errorf("command %s not found in PATH or common locations", command)
}

// CreateSysProcAttr creates syscall process attributes for namespace isolation
func (m *Manager) CreateSysProcAttr(enableNetworkNS bool) *syscall.SysProcAttr {
	sysProcAttr := m.platform.CreateProcessGroup()

	// Base namespaces that are always enabled
	sysProcAttr.Cloneflags = syscall.CLONE_NEWPID | // PID namespace ALWAYS isolated
		syscall.CLONE_NEWNS | // Mount namespace ALWAYS isolated
		syscall.CLONE_NEWIPC | // IPC namespace ALWAYS isolated
		syscall.CLONE_NEWUTS | // UTS namespace ALWAYS isolated
		syscall.CLONE_NEWCGROUP // Cgroup namespace MANDATORY

	// Conditionally add network namespace
	if enableNetworkNS {
		sysProcAttr.Cloneflags |= syscall.CLONE_NEWNET
	}

	m.logger.Debug("created process attributes",
		"flags", fmt.Sprintf("0x%x", sysProcAttr.Cloneflags),
		"networkNS", enableNetworkNS)

	return sysProcAttr
}

// BuildJobEnvironment builds environment variables for a specific job
func (m *Manager) BuildJobEnvironment(job *domain.Job, execPath string) []string {
	baseEnv := m.platform.Environ()

	// Job-specific environment with mode indicator
	jobEnv := []string{
		"JOBLET_MODE=init", // This tells the binary to run in init mode
		fmt.Sprintf("JOB_ID=%s", job.Id),
		fmt.Sprintf("JOB_COMMAND=%s", job.Command),
		fmt.Sprintf("JOB_CGROUP_PATH=%s", "/sys/fs/cgroup"),    // Namespace view
		fmt.Sprintf("JOB_CGROUP_HOST_PATH=%s", job.CgroupPath), // Host view - CRITICAL ADDITION
		fmt.Sprintf("JOB_ARGS_COUNT=%d", len(job.Args)),
		fmt.Sprintf("JOBLET_BINARY_PATH=%s", execPath),
		fmt.Sprintf("JOB_MAX_CPU=%d", job.Limits.MaxCPU),
		fmt.Sprintf("JOB_MAX_MEMORY=%d", job.Limits.MaxMemory),
		fmt.Sprintf("JOB_MAX_IOBPS=%d", job.Limits.MaxIOBPS),
	}

	// Add job arguments
	for i, arg := range job.Args {
		jobEnv = append(jobEnv, fmt.Sprintf("JOB_ARG_%d=%s", i, arg))
	}

	return append(baseEnv, jobEnv...)
}

// PrepareEnvironment prepares the environment variables for a job
func (m *Manager) PrepareEnvironment(baseEnv []string, jobEnvVars []string) []string {
	if baseEnv == nil {
		baseEnv = m.platform.Environ()
	}
	return append(baseEnv, jobEnvVars...)
}

// IsProcessAlive checks if a process is still alive
func (m *Manager) IsProcessAlive(pid int32) bool {
	if pid <= 0 {
		return false
	}
	return m.isProcessAlive(pid)
}

// KillProcess kills a process with the specified signal
func (m *Manager) KillProcess(pid int32, signal syscall.Signal) error {
	if err := m.validatePID(pid); err != nil {
		return fmt.Errorf("invalid PID: %w", err)
	}

	log := m.logger.WithFields("pid", pid, "signal", signal)
	log.Debug("killing process")

	if err := m.platform.Kill(int(pid), signal); err != nil {
		return fmt.Errorf("failed to kill process %d with signal %v: %w", pid, signal, err)
	}

	log.Debug("process killed successfully")
	return nil
}

// KillProcessGroup kills a process group with the specified signal
func (m *Manager) KillProcessGroup(pid int32, signal syscall.Signal) error {
	if err := m.validatePID(pid); err != nil {
		return fmt.Errorf("invalid PID: %w", err)
	}

	log := m.logger.WithFields("processGroup", pid, "signal", signal)
	log.Debug("killing process group")

	// Use negative PID to target the process group
	if err := m.platform.Kill(-int(pid), signal); err != nil {
		return fmt.Errorf("failed to kill process group %d with signal %v: %w", pid, signal, err)
	}

	log.Debug("process group killed successfully")
	return nil
}

// WaitForProcess waits for a process to complete with timeout
func (m *Manager) WaitForProcess(ctx context.Context, cmd platform.Command, timeout time.Duration) error {
	if cmd == nil {
		return fmt.Errorf("command cannot be nil")
	}

	if timeout <= 0 {
		return cmd.Wait()
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(timeout):
		return fmt.Errorf("process wait timeout after %v", timeout)
	}
}

// GetProcessExitCode attempts to get the exit code of a completed process
func (m *Manager) GetProcessExitCode(cmd platform.Command) (int32, error) {
	if cmd == nil {
		return -1, fmt.Errorf("command cannot be nil")
	}

	err := cmd.Wait()
	if err == nil {
		return 0, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return int32(exitErr.ExitCode()), nil
	}

	return -1, err
}

// isProcessAlive checks if a process is still alive
func (m *Manager) isProcessAlive(pid int32) bool {
	err := m.platform.Kill(int(pid), 0)
	if err == nil {
		return true
	}

	if errors.Is(err, syscall.ESRCH) {
		return false // No such process
	}

	if errors.Is(err, syscall.EPERM) {
		return true // Permission denied means process exists
	}

	m.logger.Debug("process exists check returned error, assuming dead", "pid", pid, "error", err)
	return false
}

// cleanupNamespace removes a namespace file or symlink
func (m *Manager) cleanupNamespace(nsPath string, isBound bool) error {
	log := m.logger.WithFields("nsPath", nsPath, "isBound", isBound)

	if _, err := m.platform.Stat(nsPath); err != nil {
		if m.platform.IsNotExist(err) {
			log.Debug("namespace path does not exist, nothing to cleanup")
			return nil
		}
		return fmt.Errorf("failed to stat namespace path: %w", err)
	}

	if isBound {
		log.Debug("unmounting namespace bind mount")
		if err := m.platform.Unmount(nsPath, 0); err != nil {
			log.Warn("failed to unmount namespace", "error", err)
		}
	}

	log.Debug("removing namespace file")
	if err := m.platform.Remove(nsPath); err != nil {
		return fmt.Errorf("failed to remove namespace file: %w", err)
	}

	log.Debug("namespace cleaned up successfully")
	return nil
}

// Validation helper methods
func (m *Manager) validateCommand(command string) error {
	if command == "" {
		return ValidationError{Field: "command", Value: command, Message: "command cannot be empty"}
	}
	if strings.ContainsAny(command, ";&|`$()") {
		return ValidationError{Field: "command", Value: command, Message: "command contains dangerous characters"}
	}
	if len(command) > 1024 {
		return ValidationError{Field: "command", Value: command, Message: "command too long (max 1024 characters)"}
	}
	return nil
}

func (m *Manager) validateArguments(args []string) error {
	for i, arg := range args {
		if strings.Contains(arg, "\x00") {
			return ValidationError{Field: "args", Value: fmt.Sprintf("arg[%d]", i), Message: "argument contains null bytes"}
		}
	}
	return nil
}

func (m *Manager) validatePID(pid int32) error {
	if pid <= 0 {
		return ValidationError{Field: "pid", Value: pid, Message: "PID must be positive"}
	}
	if pid > 4194304 {
		return ValidationError{Field: "pid", Value: pid, Message: "PID too large"}
	}
	return nil
}

func (m *Manager) validateLaunchConfig(config *LaunchConfig) error {
	if config.InitPath == "" {
		return fmt.Errorf("init path cannot be empty")
	}
	if config.JobID == "" {
		return fmt.Errorf("job ID cannot be empty")
	}
	if err := m.validateInitPath(config.InitPath); err != nil {
		return fmt.Errorf("invalid init path: %w", err)
	}
	if config.Environment != nil {
		if err := m.validateEnvironment(config.Environment); err != nil {
			return fmt.Errorf("invalid environment: %w", err)
		}
	}
	return nil
}

func (m *Manager) validateCleanupRequest(req *CleanupRequest) error {
	if req.JobID == "" {
		return fmt.Errorf("job ID cannot be empty")
	}
	if req.GracefulTimeout < 0 {
		return fmt.Errorf("graceful timeout cannot be negative")
	}
	return nil
}

func (m *Manager) validateInitPath(initPath string) error {
	if !filepath.IsAbs(initPath) {
		return ValidationError{Field: "initPath", Value: initPath, Message: "init path must be absolute"}
	}
	fileInfo, err := m.platform.Stat(initPath)
	if err != nil {
		if m.platform.IsNotExist(err) {
			return ValidationError{Field: "initPath", Value: initPath, Message: "init binary does not exist"}
		}
		return ValidationError{Field: "initPath", Value: initPath, Message: fmt.Sprintf("failed to stat init binary: %v", err)}
	}
	if !fileInfo.Mode().IsRegular() {
		return ValidationError{Field: "initPath", Value: initPath, Message: "init path is not a regular file"}
	}
	if fileInfo.Mode().Perm()&0111 == 0 {
		return ValidationError{Field: "initPath", Value: initPath, Message: "init binary is not executable"}
	}
	return nil
}

func (m *Manager) validateEnvironment(env []string) error {

	for i, envVar := range env {
		if strings.Contains(envVar, "\x00") {
			return ValidationError{Field: "environment", Value: fmt.Sprintf("env[%d]", i), Message: "environment variable contains null bytes"}
		}
		if !strings.Contains(envVar, "=") {
			return ValidationError{Field: "environment", Value: fmt.Sprintf("env[%d]", i), Message: "environment variable missing '=' separator"}
		}
	}
	return nil
}

// BuildJobEnvironmentWithUploads creates job environment with unified streaming upload support
func (m *Manager) BuildJobEnvironmentWithUploads(job *domain.Job, execPath string, uploads []domain.FileUpload, uploadManager *upload.Manager) []string {
	baseEnv := m.platform.Environ()

	// Prepare upload session for streaming
	session, err := uploadManager.PrepareUploadSession(job.Id, uploads, job.Limits.MaxMemory)
	if err != nil {
		m.logger.Error("failed to prepare upload session", "error", err)
		// Fallback to basic environment without uploads
		return m.BuildJobEnvironment(job, execPath)
	}

	// Job-specific environment with streaming support
	jobEnv := []string{
		"JOBLET_MODE=init",
		fmt.Sprintf("JOB_ID=%s", job.Id),
		fmt.Sprintf("JOB_COMMAND=%s", job.Command),
		fmt.Sprintf("JOB_CGROUP_PATH=%s", "/sys/fs/cgroup"),
		fmt.Sprintf("JOB_CGROUP_HOST_PATH=%s", job.CgroupPath),
		fmt.Sprintf("JOB_ARGS_COUNT=%d", len(job.Args)),
		fmt.Sprintf("JOBLET_BINARY_PATH=%s", execPath),
		fmt.Sprintf("JOB_MAX_CPU=%d", job.Limits.MaxCPU),
		fmt.Sprintf("JOB_MAX_MEMORY=%d", job.Limits.MaxMemory),
		fmt.Sprintf("JOB_MAX_IOBPS=%d", job.Limits.MaxIOBPS),

		// Upload session information
		fmt.Sprintf("JOB_UPLOAD_SESSION=%t", len(uploads) > 0),
		fmt.Sprintf("JOB_UPLOAD_TOTAL_FILES=%d", session.TotalFiles),
	}

	// Add job arguments
	for i, arg := range job.Args {
		jobEnv = append(jobEnv, fmt.Sprintf("JOB_ARG_%d=%s", i, arg))
	}

	// Single streaming path for all files
	if len(session.Files) > 0 {
		pipePath, err := uploadManager.CreateUploadPipe(job.Id)
		if err != nil {
			m.logger.Error("failed to create upload pipe", "error", err)
		} else {
			jobEnv = append(jobEnv, fmt.Sprintf("JOB_UPLOAD_PIPE=%s", pipePath))

			// Stream all files in background
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), upload.UploadTimeout)
				defer cancel()

				if err := uploadManager.StreamAllFiles(ctx, session, pipePath); err != nil {
					m.logger.Error("failed to stream files", "error", err)
				}

				_ = uploadManager.CleanupPipe(pipePath)
			}()
		}
	}

	return append(baseEnv, jobEnv...)
}

// UploadData represents serializable upload information
type UploadData struct {
	Path        string `json:"path"`
	Content     string `json:"content"` // Base64 encoded
	Mode        uint32 `json:"mode"`
	IsDirectory bool   `json:"isDirectory"`
}
