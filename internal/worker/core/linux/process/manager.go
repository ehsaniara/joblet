//go:build linux

package process

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
	"worker/pkg/platform"

	"worker/pkg/logger"
)

const (
	GracefulShutdownTimeout = 100 * time.Millisecond
	ProcessStartTimeout     = 10 * time.Second
	MaxJobArgs              = 100
	MaxJobArgLength         = 1024
)

// Manager handles all process-related operations including launching, cleanup, and validation
type Manager struct {
	platform platform.Platform
	logger   *logger.Logger
}

// NewProcessManager creates a new unified process manager
func NewProcessManager(platform platform.Platform) *Manager {
	return &Manager{
		platform: platform,
		logger:   logger.New().WithField("component", "process-manager"),
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
func (pm *Manager) LaunchProcess(ctx context.Context, config *LaunchConfig) (*LaunchResult, error) {
	if config == nil {
		return nil, fmt.Errorf("launch config cannot be nil")
	}

	log := pm.logger.WithFields("jobID", config.JobID, "command", config.Command)
	log.Debug("launching process")

	// Validate configuration
	if err := pm.validateLaunchConfig(config); err != nil {
		return nil, fmt.Errorf("invalid launch config: %w", err)
	}

	// Use pre-fork namespace setup approach for network joining
	resultChan := make(chan *LaunchResult, 1)
	go pm.launchInGoroutine(config, resultChan)

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
	case <-time.After(ProcessStartTimeout):
		log.Error("timeout waiting for process to start")
		return nil, fmt.Errorf("timeout waiting for process to start")
	}
}

// launchInGoroutine launches the process in a separate goroutine with proper namespace handling
func (pm *Manager) launchInGoroutine(config *LaunchConfig, resultChan chan<- *LaunchResult) {
	defer func() {
		if r := recover(); r != nil {
			resultChan <- &LaunchResult{
				Error: fmt.Errorf("panic in launch goroutine: %v", r),
			}
		}
	}()

	log := pm.logger.WithField("jobID", config.JobID)

	// Lock this goroutine to the OS thread for namespace operations
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Start the process (which will inherit the current namespace)
	startTime := time.Now()
	cmd, err := pm.createAndStartCommand(config)
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

	duration := time.Since(startTime)
	log.Debug("process started in goroutine", "pid", process.Pid(), "duration", duration)

	resultChan <- &LaunchResult{
		PID:     int32(process.Pid()),
		Command: cmd,
		Error:   nil,
	}
}

// createAndStartCommand creates and starts the command with proper configuration
func (pm *Manager) createAndStartCommand(config *LaunchConfig) (platform.Command, error) {
	// Create command
	cmd := pm.platform.CreateCommand(config.InitPath)

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
func (pm *Manager) CleanupProcess(ctx context.Context, req *CleanupRequest) (*CleanupResult, error) {
	if req == nil {
		return nil, fmt.Errorf("cleanup request cannot be nil")
	}

	if err := pm.validateCleanupRequest(req); err != nil {
		return nil, fmt.Errorf("invalid cleanup request: %w", err)
	}

	log := pm.logger.WithFields("jobID", req.JobID, "pid", req.PID)
	log.Debug("starting process cleanup", "forceKill", req.ForceKill, "gracefulTimeout", req.GracefulTimeout)

	startTime := time.Now()
	result := &CleanupResult{
		JobID:  req.JobID,
		Errors: make([]error, 0),
	}

	// Handle process termination
	if req.PID > 0 {
		processResult := pm.cleanupProcessAndGroup(ctx, req)
		result.ProcessKilled = processResult.Killed
		result.Method = processResult.Method
		if processResult.Error != nil {
			result.Errors = append(result.Errors, processResult.Error)
		}
	}

	// Cleanup namespace if it's an isolated job
	if req.NamespacePath != "" {
		if err := pm.cleanupNamespace(req.NamespacePath, false); err != nil {
			log.Warn("failed to cleanup namespace", "path", req.NamespacePath, "error", err)
			result.Errors = append(result.Errors, fmt.Errorf("namespace cleanup failed: %w", err))
		} else {
			result.NamespaceRemoved = true
		}
	}

	result.Duration = time.Since(startTime)

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
func (pm *Manager) cleanupProcessAndGroup(ctx context.Context, req *CleanupRequest) *processCleanupResult {
	log := pm.logger.WithFields("jobID", req.JobID, "pid", req.PID)

	// Check if process is still alive
	if !pm.isProcessAlive(req.PID) {
		log.Debug("process already dead, no cleanup needed")
		return &processCleanupResult{
			Killed: false,
			Method: "already_dead",
			Error:  nil,
		}
	}

	// If force kill is requested, skip graceful shutdown
	if req.ForceKill {
		return pm.forceKillProcess(req.PID, req.JobID)
	}

	// Try graceful shutdown first
	gracefulResult := pm.attemptGracefulShutdown(req.PID, req.GracefulTimeout, req.JobID)
	if gracefulResult.Killed {
		return gracefulResult
	}

	// If graceful shutdown failed, force kill
	log.Warn("graceful shutdown failed, attempting force kill")
	return pm.forceKillProcess(req.PID, req.JobID)
}

// attemptGracefulShutdown attempts to gracefully shut down a process
func (pm *Manager) attemptGracefulShutdown(pid int32, timeout time.Duration, jobID string) *processCleanupResult {
	log := pm.logger.WithFields("jobID", jobID, "pid", pid)

	if timeout <= 0 {
		timeout = GracefulShutdownTimeout
	}

	log.Debug("attempting graceful shutdown", "timeout", timeout)

	// Send SIGTERM to process group first
	if err := pm.platform.Kill(-int(pid), syscall.SIGTERM); err != nil {
		log.Warn("failed to send SIGTERM to process group", "error", err)
		// If killing the group failed, try killing just the main process
		if err := pm.platform.Kill(int(pid), syscall.SIGTERM); err != nil {
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
	if !pm.isProcessAlive(pid) {
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
func (pm *Manager) forceKillProcess(pid int32, jobID string) *processCleanupResult {
	log := pm.logger.WithFields("jobID", jobID, "pid", pid)
	log.Warn("force killing process")

	// Send SIGKILL to process group
	if err := pm.platform.Kill(-int(pid), syscall.SIGKILL); err != nil {
		log.Warn("failed to send SIGKILL to process group", "error", err)
		// Try killing just the main process
		if err := pm.platform.Kill(int(pid), syscall.SIGKILL); err != nil {
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
	if pm.isProcessAlive(pid) {
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
func (pm *Manager) ValidateCommand(command string) error {
	return pm.validateCommand(command)
}

// ValidateArguments validates command arguments
func (pm *Manager) ValidateArguments(args []string) error {
	return pm.validateArguments(args)
}

// ResolveCommand resolves a command to its full path
func (pm *Manager) ResolveCommand(command string) (string, error) {
	if command == "" {
		return "", fmt.Errorf("command cannot be empty")
	}

	log := pm.logger.WithField("command", command)

	// If command is already absolute, validate it exists
	if filepath.IsAbs(command) {
		if _, err := pm.platform.Stat(command); err != nil {
			log.Error("absolute command path not found", "error", err)
			return "", fmt.Errorf("command %s not found: %w", command, err)
		}
		log.Debug("using absolute command path")
		return command, nil
	}

	// Try to resolve using PATH
	if resolvedPath, err := pm.platform.LookPath(command); err == nil {
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
		if _, err := pm.platform.Stat(path); err == nil {
			log.Debug("found command in common location", "path", path)
			return path, nil
		}
	}

	log.Error("command not found anywhere", "searchedPaths", commonPaths)
	return "", fmt.Errorf("command %s not found in PATH or common locations", command)
}

// CreateSysProcAttr creates syscall process attributes for namespace isolation
func (pm *Manager) CreateSysProcAttr(enableNetworkNS bool) *syscall.SysProcAttr {
	sysProcAttr := pm.platform.CreateProcessGroup()

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

	pm.logger.Debug("created process attributes",
		"flags", fmt.Sprintf("0x%x", sysProcAttr.Cloneflags),
		"networkNS", enableNetworkNS)

	return sysProcAttr
}

// BuildJobEnvironment builds environment variables for a specific job
func (pm *Manager) BuildJobEnvironment(jobID, command, cgroupPath string, args []string, networkEnvVars []string) []string {
	jobEnvVars := []string{
		fmt.Sprintf("JOB_ID=%s", jobID),
		fmt.Sprintf("JOB_COMMAND=%s", command),
		fmt.Sprintf("JOB_CGROUP_PATH=%s", cgroupPath),
	}

	// Add job arguments
	for i, arg := range args {
		jobEnvVars = append(jobEnvVars, fmt.Sprintf("JOB_ARG_%d=%s", i, arg))
	}
	jobEnvVars = append(jobEnvVars, fmt.Sprintf("JOB_ARGS_COUNT=%d", len(args)))

	// Add network environment variables if provided
	if networkEnvVars != nil {
		jobEnvVars = append(jobEnvVars, networkEnvVars...)
	}

	return jobEnvVars
}

// PrepareEnvironment prepares the environment variables for a job
func (pm *Manager) PrepareEnvironment(baseEnv []string, jobEnvVars []string) []string {
	if baseEnv == nil {
		baseEnv = pm.platform.Environ()
	}
	return append(baseEnv, jobEnvVars...)
}

// IsProcessAlive checks if a process is still alive
func (pm *Manager) IsProcessAlive(pid int32) bool {
	if pid <= 0 {
		return false
	}
	return pm.isProcessAlive(pid)
}

// KillProcess kills a process with the specified signal
func (pm *Manager) KillProcess(pid int32, signal syscall.Signal) error {
	if err := pm.validatePID(pid); err != nil {
		return fmt.Errorf("invalid PID: %w", err)
	}

	log := pm.logger.WithFields("pid", pid, "signal", signal)
	log.Debug("killing process")

	if err := pm.platform.Kill(int(pid), signal); err != nil {
		return fmt.Errorf("failed to kill process %d with signal %v: %w", pid, signal, err)
	}

	log.Debug("process killed successfully")
	return nil
}

// KillProcessGroup kills a process group with the specified signal
func (pm *Manager) KillProcessGroup(pid int32, signal syscall.Signal) error {
	if err := pm.validatePID(pid); err != nil {
		return fmt.Errorf("invalid PID: %w", err)
	}

	log := pm.logger.WithFields("processGroup", pid, "signal", signal)
	log.Debug("killing process group")

	// Use negative PID to target the process group
	if err := pm.platform.Kill(-int(pid), signal); err != nil {
		return fmt.Errorf("failed to kill process group %d with signal %v: %w", pid, signal, err)
	}

	log.Debug("process group killed successfully")
	return nil
}

// WaitForProcess waits for a process to complete with timeout
func (pm *Manager) WaitForProcess(ctx context.Context, cmd platform.Command, timeout time.Duration) error {
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
func (pm *Manager) GetProcessExitCode(cmd platform.Command) (int32, error) {
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
func (pm *Manager) isProcessAlive(pid int32) bool {
	err := pm.platform.Kill(int(pid), 0)
	if err == nil {
		return true
	}

	if errors.Is(err, syscall.ESRCH) {
		return false // No such process
	}

	if errors.Is(err, syscall.EPERM) {
		return true // Permission denied means process exists
	}

	pm.logger.Debug("process exists check returned error, assuming dead", "pid", pid, "error", err)
	return false
}

// cleanupNamespace removes a namespace file or symlink
func (pm *Manager) cleanupNamespace(nsPath string, isBound bool) error {
	log := pm.logger.WithFields("nsPath", nsPath, "isBound", isBound)

	if _, err := pm.platform.Stat(nsPath); err != nil {
		if pm.platform.IsNotExist(err) {
			log.Debug("namespace path does not exist, nothing to cleanup")
			return nil
		}
		return fmt.Errorf("failed to stat namespace path: %w", err)
	}

	if isBound {
		log.Debug("unmounting namespace bind mount")
		if err := pm.platform.Unmount(nsPath, 0); err != nil {
			log.Warn("failed to unmount namespace", "error", err)
		}
	}

	log.Debug("removing namespace file")
	if err := pm.platform.Remove(nsPath); err != nil {
		return fmt.Errorf("failed to remove namespace file: %w", err)
	}

	log.Debug("namespace cleaned up successfully")
	return nil
}

// Validation helper methods
func (pm *Manager) validateCommand(command string) error {
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

func (pm *Manager) validateArguments(args []string) error {
	if len(args) > MaxJobArgs {
		return ValidationError{Field: "args", Value: len(args), Message: fmt.Sprintf("too many arguments (max %d)", MaxJobArgs)}
	}
	for i, arg := range args {
		if len(arg) > MaxJobArgLength {
			return ValidationError{Field: "args", Value: fmt.Sprintf("arg[%d]", i), Message: fmt.Sprintf("argument too long (max %d characters)", MaxJobArgLength)}
		}
		if strings.Contains(arg, "\x00") {
			return ValidationError{Field: "args", Value: fmt.Sprintf("arg[%d]", i), Message: "argument contains null bytes"}
		}
	}
	return nil
}

func (pm *Manager) validateResourceLimits(maxCPU, maxMemory, maxIOBPS int32) error {
	if maxCPU < 0 {
		return ValidationError{Field: "maxCPU", Value: maxCPU, Message: "CPU limit cannot be negative"}
	}
	if maxCPU > 10000 {
		return ValidationError{Field: "maxCPU", Value: maxCPU, Message: "CPU limit too high (max 10000%)"}
	}
	if maxMemory < 0 {
		return ValidationError{Field: "maxMemory", Value: maxMemory, Message: "memory limit cannot be negative"}
	}
	if maxMemory > 1024*1024 {
		return ValidationError{Field: "maxMemory", Value: maxMemory, Message: "memory limit too high (max 1TB)"}
	}
	if maxIOBPS < 0 {
		return ValidationError{Field: "maxIOBPS", Value: maxIOBPS, Message: "IO limit cannot be negative"}
	}
	if maxIOBPS > 10*1024*1024 {
		return ValidationError{Field: "maxIOBPS", Value: maxIOBPS, Message: "IO limit too high (max 10GB/s)"}
	}
	return nil
}

func (pm *Manager) validateJobID(jobID string) error {
	if jobID == "" {
		return ValidationError{Field: "jobID", Value: jobID, Message: "job ID cannot be empty"}
	}
	if len(jobID) > 64 {
		return ValidationError{Field: "jobID", Value: jobID, Message: "job ID too long (max 64 characters)"}
	}
	for _, char := range jobID {
		if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '-' || char == '_') {
			return ValidationError{Field: "jobID", Value: jobID, Message: "job ID contains invalid characters (only alphanumeric, dash, underscore allowed)"}
		}
	}
	return nil
}

func (pm *Manager) validatePID(pid int32) error {
	if pid <= 0 {
		return ValidationError{Field: "pid", Value: pid, Message: "PID must be positive"}
	}
	if pid > 4194304 {
		return ValidationError{Field: "pid", Value: pid, Message: "PID too large"}
	}
	return nil
}

func (pm *Manager) validateLaunchConfig(config *LaunchConfig) error {
	if config.InitPath == "" {
		return fmt.Errorf("init path cannot be empty")
	}
	if config.JobID == "" {
		return fmt.Errorf("job ID cannot be empty")
	}
	if err := pm.validateInitPath(config.InitPath); err != nil {
		return fmt.Errorf("invalid init path: %w", err)
	}
	if config.Environment != nil {
		if err := pm.validateEnvironment(config.Environment); err != nil {
			return fmt.Errorf("invalid environment: %w", err)
		}
	}
	return nil
}

func (pm *Manager) validateCleanupRequest(req *CleanupRequest) error {
	if req.JobID == "" {
		return fmt.Errorf("job ID cannot be empty")
	}
	if req.GracefulTimeout < 0 {
		return fmt.Errorf("graceful timeout cannot be negative")
	}
	return nil
}

func (pm *Manager) validateInitPath(initPath string) error {
	if !filepath.IsAbs(initPath) {
		return ValidationError{Field: "initPath", Value: initPath, Message: "init path must be absolute"}
	}
	fileInfo, err := pm.platform.Stat(initPath)
	if err != nil {
		if pm.platform.IsNotExist(err) {
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

func (pm *Manager) validateCgroupPath(cgroupPath string) error {
	if !filepath.IsAbs(cgroupPath) {
		return ValidationError{Field: "cgroupPath", Value: cgroupPath, Message: "cgroup path must be absolute"}
	}
	cleanPath := filepath.Clean(cgroupPath)
	if cleanPath != cgroupPath {
		return ValidationError{Field: "cgroupPath", Value: cgroupPath, Message: "cgroup path contains path traversal attempts"}
	}
	if _, err := pm.platform.Stat(cgroupPath); err != nil {
		if pm.platform.IsNotExist(err) {
			return ValidationError{Field: "cgroupPath", Value: cgroupPath, Message: "cgroup directory does not exist"}
		}
		return ValidationError{Field: "cgroupPath", Value: cgroupPath, Message: fmt.Sprintf("failed to stat cgroup directory: %v", err)}
	}
	return nil
}

func (pm *Manager) validateEnvironment(env []string) error {

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
