//go:build linux

package process

import (
	"context"
	"fmt"
	"io"
	"runtime"
	"syscall"
	"time"

	"job-worker/pkg/logger"
	osinterface "job-worker/pkg/os"
)

const (
	ProcessStartTimeout = 10 * time.Second
)

// Launcher handles process launching with namespace support
type Launcher struct {
	cmdFactory  osinterface.CommandFactory
	syscall     osinterface.SyscallInterface
	osInterface osinterface.OsInterface
	validator   *Validator
	logger      *logger.Logger
}

// LaunchConfig contains all configuration for launching a process
type LaunchConfig struct {
	InitPath      string
	Environment   []string
	SysProcAttr   *syscall.SysProcAttr
	Stdout        io.Writer
	Stderr        io.Writer
	NamespacePath string
	NeedsNSJoin   bool
	JobID         string
	Command       string
	Args          []string
}

// LaunchResult contains the result of a process launch
type LaunchResult struct {
	PID     int32
	Command osinterface.Command
	Error   error
}

// NamespaceJoiner defines the interface for joining network namespaces
type NamespaceJoiner interface {
	JoinNetworkNamespace(nsPath string) error
}

// NewLauncher creates a new process launcher
func NewLauncher(
	cmdFactory osinterface.CommandFactory,
	syscall osinterface.SyscallInterface,
	osInterface osinterface.OsInterface,
	validator *Validator,
) *Launcher {
	return &Launcher{
		cmdFactory:  cmdFactory,
		syscall:     syscall,
		osInterface: osInterface,
		validator:   validator,
		logger:      logger.New().WithField("component", "process-launcher"),
	}
}

// LaunchProcess launches a process with the given configuration
func (l *Launcher) LaunchProcess(ctx context.Context, config *LaunchConfig) (*LaunchResult, error) {
	if config == nil {
		return nil, fmt.Errorf("launch config cannot be nil")
	}

	log := l.logger.WithFields("jobID", config.JobID, "command", config.Command)
	log.Info("launching process",
		"needsNSJoin", config.NeedsNSJoin,
		"namespacePath", config.NamespacePath)

	// Validate configuration
	if err := l.validateLaunchConfig(config); err != nil {
		return nil, fmt.Errorf("invalid launch config: %w", err)
	}

	// Use pre-fork namespace setup approach for network joining
	resultChan := make(chan *LaunchResult, 1)

	go l.launchInGoroutine(config, resultChan)

	// Wait for the goroutine to complete with timeout
	select {
	case result := <-resultChan:
		if result.Error != nil {
			log.Error("failed to start process in goroutine", "error", result.Error)
			return nil, fmt.Errorf("failed to start process: %w", result.Error)
		}

		log.Info("process started successfully", "pid", result.PID)
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
func (l *Launcher) launchInGoroutine(config *LaunchConfig, resultChan chan<- *LaunchResult) {
	defer func() {
		if r := recover(); r != nil {
			resultChan <- &LaunchResult{
				Error: fmt.Errorf("panic in launch goroutine: %v", r),
			}
		}
	}()

	log := l.logger.WithField("jobID", config.JobID)

	// Lock this goroutine to the OS thread for namespace operations
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Join network namespace if needed (before forking)
	if config.NeedsNSJoin && config.NamespacePath != "" {
		log.Debug("joining network namespace before fork", "nsPath", config.NamespacePath)
		if err := l.joinNetworkNamespace(config.NamespacePath); err != nil {
			resultChan <- &LaunchResult{
				Error: fmt.Errorf("failed to join namespace: %w", err),
			}
			return
		}
		log.Debug("successfully joined network namespace")
	}

	// Start the process (which will inherit the current namespace)
	startTime := time.Now()
	cmd, err := l.createAndStartCommand(config)
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
func (l *Launcher) createAndStartCommand(config *LaunchConfig) (osinterface.Command, error) {
	// Create command
	cmd := l.cmdFactory.CreateCommand(config.InitPath)

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

// joinNetworkNamespace joins an existing network namespace using setns syscall
func (l *Launcher) joinNetworkNamespace(nsPath string) error {
	if nsPath == "" {
		return fmt.Errorf("namespace path cannot be empty")
	}

	// Check if namespace file exists
	if _, err := l.osInterface.Stat(nsPath); err != nil {
		return fmt.Errorf("namespace file does not exist: %s (%w)", nsPath, err)
	}

	l.logger.Debug("opening namespace file", "nsPath", nsPath)

	// Open the namespace file
	fd, err := syscall.Open(nsPath, syscall.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf("failed to open namespace file %s: %w", nsPath, err)
	}
	defer func() {
		if closeErr := syscall.Close(fd); closeErr != nil {
			l.logger.Warn("failed to close namespace file descriptor", "error", closeErr)
		}
	}()

	l.logger.Debug("calling setns syscall", "fd", fd, "nsPath", nsPath)

	// Call setns system call (x86_64 syscall number for setns)
	const SysSetnsX86_64 = 308
	_, _, errno := syscall.Syscall(SysSetnsX86_64, uintptr(fd), syscall.CLONE_NEWNET, 0)
	if errno != 0 {
		return fmt.Errorf("setns syscall failed for %s: %v", nsPath, errno)
	}

	l.logger.Debug("successfully joined network namespace", "nsPath", nsPath)
	return nil
}

// validateLaunchConfig validates the launch configuration
func (l *Launcher) validateLaunchConfig(config *LaunchConfig) error {
	if config.InitPath == "" {
		return fmt.Errorf("init path cannot be empty")
	}

	if config.JobID == "" {
		return fmt.Errorf("job ID cannot be empty")
	}

	// Validate init path exists and is executable
	if err := l.validator.ValidateInitPath(config.InitPath); err != nil {
		return fmt.Errorf("invalid init path: %w", err)
	}

	// Validate environment if provided
	if config.Environment != nil {
		if err := l.validator.ValidateEnvironment(config.Environment); err != nil {
			return fmt.Errorf("invalid environment: %w", err)
		}
	}

	// Validate namespace path if namespace join is needed
	if config.NeedsNSJoin {
		if config.NamespacePath == "" {
			return fmt.Errorf("namespace path required when NeedsNSJoin is true")
		}

		// Check if namespace file exists
		if _, err := l.osInterface.Stat(config.NamespacePath); err != nil {
			return fmt.Errorf("namespace file validation failed: %w", err)
		}
	}

	return nil
}

// PrepareEnvironment prepares the environment variables for a job
func (l *Launcher) PrepareEnvironment(baseEnv []string, jobEnvVars []string) []string {
	if baseEnv == nil {
		baseEnv = l.osInterface.Environ()
	}

	// Combine base environment with job-specific variables
	return append(baseEnv, jobEnvVars...)
}

// BuildJobEnvironment builds environment variables for a specific job
func (l *Launcher) BuildJobEnvironment(jobID, command, cgroupPath string, args []string, networkEnvVars []string) []string {
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

// CreateSysProcAttr creates syscall process attributes for namespace isolation
func (l *Launcher) CreateSysProcAttr(enableNetworkNS bool) *syscall.SysProcAttr {
	sysProcAttr := l.syscall.CreateProcessGroup()

	// Base namespaces that are always enabled
	sysProcAttr.Cloneflags = syscall.CLONE_NEWPID | // PID namespace - ALWAYS isolated
		syscall.CLONE_NEWNS | // Mount namespace - ALWAYS isolated
		syscall.CLONE_NEWIPC | // IPC namespace - ALWAYS isolated
		syscall.CLONE_NEWUTS // UTS namespace - ALWAYS isolated

	// Conditionally add network namespace
	if enableNetworkNS {
		sysProcAttr.Cloneflags |= syscall.CLONE_NEWNET
	}

	l.logger.Debug("created process attributes",
		"flags", fmt.Sprintf("0x%x", sysProcAttr.Cloneflags),
		"networkNS", enableNetworkNS)

	return sysProcAttr
}

// WaitForProcess waits for a process to complete with timeout
func (l *Launcher) WaitForProcess(ctx context.Context, cmd osinterface.Command, timeout time.Duration) error {
	if cmd == nil {
		return fmt.Errorf("command cannot be nil")
	}

	if timeout <= 0 {
		// No timeout, wait indefinitely
		return cmd.Wait()
	}

	// Wait with timeout
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

// GetProcessInfo returns information about a running process
func (l *Launcher) GetProcessInfo(cmd osinterface.Command) (*Info, error) {
	if cmd == nil {
		return nil, fmt.Errorf("command cannot be nil")
	}

	process := cmd.Process()
	if process == nil {
		return nil, fmt.Errorf("process is nil")
	}

	return &Info{
		PID:    int32(process.Pid()),
		Status: "running", // We can enhance this with actual process status checking
	}, nil
}

// Info contains information about a running process
type Info struct {
	PID    int32
	Status string
}

// IsProcessRunning checks if a process is still running
func (l *Launcher) IsProcessRunning(pid int32) bool {
	if pid <= 0 {
		return false
	}

	// Use kill(pid, 0) to check if process exists
	err := l.syscall.Kill(int(pid), 0)
	return err == nil
}

// KillProcess kills a process with the specified signal
func (l *Launcher) KillProcess(pid int32, signal syscall.Signal) error {
	if err := l.validator.ValidatePID(pid); err != nil {
		return fmt.Errorf("invalid PID: %w", err)
	}

	log := l.logger.WithFields("pid", pid, "signal", signal)
	log.Debug("killing process")

	if err := l.syscall.Kill(int(pid), signal); err != nil {
		return fmt.Errorf("failed to kill process %d with signal %v: %w", pid, signal, err)
	}

	log.Info("process killed successfully")
	return nil
}

// KillProcessGroup kills a process group with the specified signal
func (l *Launcher) KillProcessGroup(pid int32, signal syscall.Signal) error {
	if err := l.validator.ValidatePID(pid); err != nil {
		return fmt.Errorf("invalid PID: %w", err)
	}

	log := l.logger.WithFields("processGroup", pid, "signal", signal)
	log.Debug("killing process group")

	// Use negative PID to target the process group
	if err := l.syscall.Kill(-int(pid), signal); err != nil {
		return fmt.Errorf("failed to kill process group %d with signal %v: %w", pid, signal, err)
	}

	log.Info("process group killed successfully")
	return nil
}
