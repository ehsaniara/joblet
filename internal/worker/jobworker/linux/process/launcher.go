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

// LaunchProcess starts process with namespace isolation and proper cleanup on failure
func (l *Launcher) LaunchProcess(ctx context.Context, config *LaunchConfig) (*LaunchResult, error) {
	if config == nil {
		return nil, fmt.Errorf("launch mapping.go cannot be nil")
	}

	log := l.logger.WithFields("jobID", config.JobID, "command", config.Command)
	log.Info("launching process",
		"needsNSJoin", config.NeedsNSJoin,
		"namespacePath", config.NamespacePath)

	// Validate configuration
	if err := l.validateLaunchConfig(config); err != nil {
		return nil, fmt.Errorf("invalid launch mapping.go: %w", err)
	}

	// Use pre-fork namespace setup approach for network joining
	resultChan := make(chan *LaunchResult, 1)

	go l.launchInGoroutine(config, resultChan)

	// Wait with timeout to prevent hanging on process start failures
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
