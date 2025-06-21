package process

import (
	"context"
	"errors"
	"fmt"
	"syscall"
	"time"

	"job-worker/pkg/logger"
	osinterface "job-worker/pkg/os"
)

const (
	GracefulShutdownTimeout = 100 * time.Millisecond
)

// Cleaner handles process cleanup operations
type Cleaner struct {
	syscall     osinterface.SyscallInterface
	osInterface osinterface.OsInterface
	logger      *logger.Logger
}

// CleanupRequest contains information needed for cleanup
type CleanupRequest struct {
	JobID           string
	PID             int32
	CgroupPath      string
	NetworkGroupID  string
	NamespacePath   string
	IsIsolatedJob   bool
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

// NewCleaner creates a new process cleaner
func NewCleaner(
	syscall osinterface.SyscallInterface,
	osInterface osinterface.OsInterface,
) *Cleaner {
	return &Cleaner{
		syscall:     syscall,
		osInterface: osInterface,
		logger:      logger.New().WithField("component", "process-cleaner"),
	}
}

// CleanupProcess performs comprehensive cleanup of a job process and its resources
func (c *Cleaner) CleanupProcess(ctx context.Context, req *CleanupRequest) (*CleanupResult, error) {
	if req == nil {
		return nil, fmt.Errorf("cleanup request cannot be nil")
	}

	if err := c.validateCleanupRequest(req); err != nil {
		return nil, fmt.Errorf("invalid cleanup request: %w", err)
	}

	log := c.logger.WithFields("jobID", req.JobID, "pid", req.PID)
	log.Info("starting process cleanup",
		"forceKill", req.ForceKill,
		"gracefulTimeout", req.GracefulTimeout)

	startTime := time.Now()
	result := &CleanupResult{
		JobID:  req.JobID,
		Errors: make([]error, 0),
	}

	// Try graceful shutdown first (SIGTERM)
	if req.PID > 0 {
		processResult := c.cleanupProcessAndGroup(ctx, req)
		result.ProcessKilled = processResult.Killed
		result.Method = processResult.Method
		if processResult.Error != nil {
			result.Errors = append(result.Errors, processResult.Error)
		}
	}

	// Remove namespace files to prevent resource leaks
	if req.IsIsolatedJob && req.NamespacePath != "" {
		if err := c.cleanupNamespace(req.NamespacePath, false); err != nil {
			log.Warn("failed to cleanup namespace", "path", req.NamespacePath, "error", err)
			result.Errors = append(result.Errors, fmt.Errorf("namespace cleanup failed: %w", err))
		} else {
			result.NamespaceRemoved = true
		}
	}

	result.Duration = time.Since(startTime)

	if len(result.Errors) > 0 {
		log.Warn("cleanup completed with errors",
			"duration", result.Duration,
			"errorCount", len(result.Errors))
	} else {
		log.Info("cleanup completed successfully", "duration", result.Duration)
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
func (c *Cleaner) cleanupProcessAndGroup(ctx context.Context, req *CleanupRequest) *processCleanupResult {
	log := c.logger.WithFields("jobID", req.JobID, "pid", req.PID)

	// Check if process is still alive
	if !c.isProcessAlive(req.PID) {
		log.Debug("process already dead, no cleanup needed")
		return &processCleanupResult{
			Killed: false,
			Method: "already_dead",
			Error:  nil,
		}
	}

	// If force kill is requested, skip graceful shutdown
	if req.ForceKill {
		return c.forceKillProcess(req.PID, req.JobID)
	}

	// Try graceful shutdown first
	gracefulResult := c.attemptGracefulShutdown(req.PID, req.GracefulTimeout, req.JobID)
	if gracefulResult.Killed {
		return gracefulResult
	}

	// If graceful shutdown failed, force kill
	log.Warn("graceful shutdown failed, attempting force kill")
	return c.forceKillProcess(req.PID, req.JobID)
}

// attemptGracefulShutdown attempts to gracefully shut down a process
func (c *Cleaner) attemptGracefulShutdown(pid int32, timeout time.Duration, jobID string) *processCleanupResult {
	log := c.logger.WithFields("jobID", jobID, "pid", pid)

	if timeout <= 0 {
		timeout = GracefulShutdownTimeout
	}

	log.Debug("attempting graceful shutdown", "timeout", timeout)

	// Send SIGTERM to process group first
	if err := c.syscall.Kill(-int(pid), syscall.SIGTERM); err != nil {
		log.Warn("failed to send SIGTERM to process group", "error", err)

		// If killing the group failed, try killing just the main process
		if err := c.syscall.Kill(int(pid), syscall.SIGTERM); err != nil {
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
	if !c.isProcessAlive(pid) {
		log.Info("process terminated gracefully")
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
func (c *Cleaner) forceKillProcess(pid int32, jobID string) *processCleanupResult {
	log := c.logger.WithFields("jobID", jobID, "pid", pid)
	log.Warn("force killing process")

	// Send SIGKILL to process group
	if err := c.syscall.Kill(-int(pid), syscall.SIGKILL); err != nil {
		log.Warn("failed to send SIGKILL to process group", "error", err)

		// Try killing just the main process
		if err := c.syscall.Kill(int(pid), syscall.SIGKILL); err != nil {
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
	if c.isProcessAlive(pid) {
		log.Error("process still alive after SIGKILL")
		return &processCleanupResult{
			Killed: false,
			Method: "force_failed",
			Error:  fmt.Errorf("process still alive after SIGKILL"),
		}
	}

	log.Info("process force killed successfully")
	return &processCleanupResult{
		Killed: true,
		Method: "forced",
		Error:  nil,
	}
}

// isProcessAlive checks if a process is still alive
func (c *Cleaner) isProcessAlive(pid int32) bool {
	err := c.syscall.Kill(int(pid), 0)
	if err == nil {
		return true
	}

	if errors.Is(err, syscall.ESRCH) {
		return false // No such process
	}

	if errors.Is(err, syscall.EPERM) {
		return true // Permission denied means process exists
	}

	// For other errors, assume process doesn't exist
	c.logger.Debug("process exists check returned error, assuming dead",
		"pid", pid, "error", err)
	return false
}

// cleanupNamespace removes a namespace file or symlink
func (c *Cleaner) cleanupNamespace(nsPath string, isBound bool) error {
	log := c.logger.WithFields("nsPath", nsPath, "isBound", isBound)

	// Check if the namespace path exists
	if _, err := c.osInterface.Stat(nsPath); err != nil {
		if c.osInterface.IsNotExist(err) {
			log.Debug("namespace path does not exist, nothing to cleanup")
			return nil
		}
		return fmt.Errorf("failed to stat namespace path: %w", err)
	}

	if isBound {
		// Unmount bind mount first
		log.Debug("unmounting namespace bind mount")
		if err := c.syscall.Unmount(nsPath, 0); err != nil {
			log.Warn("failed to unmount namespace", "error", err)
			// Continue with removal attempt
		}
	}

	// Remove the file/symlink
	log.Debug("removing namespace file")
	if err := c.osInterface.Remove(nsPath); err != nil {
		return fmt.Errorf("failed to remove namespace file: %w", err)
	}

	log.Info("namespace cleaned up successfully")
	return nil
}

// validateCleanupRequest validates a cleanup request
func (c *Cleaner) validateCleanupRequest(req *CleanupRequest) error {
	if req.JobID == "" {
		return fmt.Errorf("job ID cannot be empty")
	}

	if req.GracefulTimeout < 0 {
		return fmt.Errorf("graceful timeout cannot be negative")
	}

	return nil
}
