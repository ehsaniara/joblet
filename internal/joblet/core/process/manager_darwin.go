//go:build darwin

package process

import (
	"context"
	"fmt"
	"io"
	"joblet/pkg/logger"
	"joblet/pkg/platform"
	"syscall"
	"time"
)

// Manager handles all process-related operations including launching, cleanup, and validation
type Manager struct {
	platform platform.Platform
	logger   *logger.Logger
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

// NewProcessManager creates a new unified process manager
func NewProcessManager(platform platform.Platform) *Manager {
	log := logger.New().WithField("component", "process-manager-darwin")
	return &Manager{
		platform: platform,
		logger:   log,
	}
}

// Stub implementations for Darwin
func (m *Manager) LaunchProcess(ctx context.Context, config *LaunchConfig) (*LaunchResult, error) {
	return nil, fmt.Errorf("process management not supported on Darwin")
}

func (m *Manager) CleanupProcess(ctx context.Context, req *CleanupRequest) (*CleanupResult, error) {
	return nil, fmt.Errorf("process cleanup not supported on Darwin")
}

func (m *Manager) ValidateCommand(command string) error {
	return nil // Basic validation only
}

func (m *Manager) ValidateArguments(args []string) error {
	return nil // Basic validation only
}

func (m *Manager) ResolveCommand(command string) (string, error) {
	return command, nil // Simple passthrough
}
