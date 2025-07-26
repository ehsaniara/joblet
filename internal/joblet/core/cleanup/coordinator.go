package cleanup

import (
	"context"
	"fmt"
	"joblet/internal/joblet/network"
	"joblet/internal/joblet/state"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"joblet/internal/joblet/core/process"
	"joblet/internal/joblet/core/resource"
	"joblet/pkg/config"
	"joblet/pkg/logger"
	"joblet/pkg/platform"
)

// Coordinator coordinates all cleanup operations for jobs
type Coordinator struct {
	processManager *process.Manager
	cgroup         resource.Resource
	platform       platform.Platform
	config         *config.Config
	logger         *logger.Logger

	// Cleanup tracking
	activeCleanups sync.Map // jobID -> cleanup status

	networkSetup *network.NetworkSetup
	networkStore *state.NetworkStore
}

// CleanupStatus tracks the status of a cleanup operation
type CleanupStatus struct {
	JobID         string
	StartTime     time.Time
	ProcessKilled bool
	CgroupCleaned bool
	FilesCleaned  bool
	Errors        []error
	Completed     bool
}

// NewCoordinator creates a new cleanup coordinator
func NewCoordinator(
	processManager *process.Manager,
	cgroup resource.Resource,
	platform platform.Platform,
	config *config.Config,
	logger *logger.Logger,
	networkStore *state.NetworkStore,
) *Coordinator {
	return &Coordinator{
		processManager: processManager,
		cgroup:         cgroup,
		platform:       platform,
		config:         config,
		logger:         logger.WithField("component", "cleanup-coordinator"),
		networkStore:   networkStore,
		networkSetup:   network.NewNetworkSetup(platform, networkStore),
	}
}

// CleanupJob performs all cleanup operations for a job
func (c *Coordinator) CleanupJob(jobID string) error {
	log := c.logger.WithField("jobID", jobID)
	log.Debug("starting job cleanup")

	// Check if cleanup is already in progress
	if _, exists := c.activeCleanups.Load(jobID); exists {
		log.Warn("cleanup already in progress for job")
		return fmt.Errorf("cleanup already in progress for job %s", jobID)
	}

	// Track cleanup status
	status := &CleanupStatus{
		JobID:     jobID,
		StartTime: time.Now(),
		Errors:    make([]error, 0),
	}
	c.activeCleanups.Store(jobID, status)
	defer c.activeCleanups.Delete(jobID)

	// Perform cleanup operations in order
	// Continue even if individual operations fail

	// 1. Clean up cgroup (releases resources)
	c.cleanupCgroup(jobID)
	status.CgroupCleaned = true

	// 2. Clean up filesystem (removes job artifacts)
	if err := c.cleanupFilesystem(jobID); err != nil {
		log.Error("filesystem cleanup failed", "error", err)
		status.Errors = append(status.Errors, fmt.Errorf("filesystem: %w", err))
	} else {
		status.FilesCleaned = true
	}

	// 3. Clean up any remaining resources
	if err := c.cleanupAdditionalResources(jobID); err != nil {
		log.Error("additional resource cleanup failed", "error", err)
		status.Errors = append(status.Errors, fmt.Errorf("additional: %w", err))
	}

	// Clean up network resources
	if alloc, err := c.networkStore.GetJobAllocation(jobID); err == nil {
		if err := c.networkSetup.CleanupJobNetwork(alloc); err != nil {
			c.logger.Warn("failed to cleanup network", "jobID", jobID, "error", err)
		}
	}

	// Release network allocation
	c.networkStore.ReleaseJob(jobID)

	status.Completed = true

	// Log summary
	duration := time.Since(status.StartTime)
	if len(status.Errors) > 0 {
		log.Error("job cleanup completed with errors",
			"duration", duration,
			"errors", len(status.Errors),
			"errorDetails", status.Errors)
		return fmt.Errorf("cleanup completed with %d errors", len(status.Errors))
	}

	log.Info("job cleanup completed successfully", "duration", duration)
	return nil
}

// CleanupJobWithProcess cleans up a job including its process
func (c *Coordinator) CleanupJobWithProcess(ctx context.Context, jobID string, pid int32) error {
	log := c.logger.WithField("jobID", jobID)
	log.Debug("starting job cleanup with process termination", "pid", pid)

	// First, stop the process
	if pid > 0 {
		cleanupReq := &process.CleanupRequest{
			JobID:           jobID,
			PID:             pid,
			ForceKill:       false,
			GracefulTimeout: c.config.Cgroup.CleanupTimeout,
		}

		result, err := c.processManager.CleanupProcess(ctx, cleanupReq)
		if err != nil {
			log.Error("process cleanup failed", "error", err)
			// Continue with other cleanup even if process cleanup fails
		} else {
			log.Debug("process cleanup completed", "method", result.Method)
		}
	}

	// Then perform regular cleanup
	return c.CleanupJob(jobID)
}

// cleanupCgroup removes cgroup resources
func (c *Coordinator) cleanupCgroup(jobID string) {
	log := c.logger.WithField("operation", "cgroup-cleanup")
	log.Debug("cleaning up cgroup", "jobID", jobID)

	// The cgroup cleanup is handled by the resource manager
	c.cgroup.CleanupCgroup(jobID)
}

// cleanupFilesystem removes all filesystem resources for a job
func (c *Coordinator) cleanupFilesystem(jobID string) error {
	log := c.logger.WithField("operation", "filesystem-cleanup")
	log.Debug("cleaning up filesystem", "jobID", jobID)

	errors := make([]error, 0)

	// 1. Clean up main job directory
	jobRootDir := filepath.Join(c.config.Filesystem.BaseDir, jobID)
	if err := c.removeDirectory(jobRootDir, "job root"); err != nil {
		errors = append(errors, err)
	}

	// 2. Clean up temporary directory
	jobTmpDir := strings.Replace(c.config.Filesystem.TmpDir, "{JOB_ID}", jobID, -1)
	if jobTmpDir != c.config.Filesystem.TmpDir { // Ensure substitution happened
		if err := c.removeDirectory(jobTmpDir, "job tmp"); err != nil {
			errors = append(errors, err)
		}
	}

	// 3. Clean up pipes directory
	pipesDir := filepath.Join(c.config.Filesystem.BaseDir, jobID, "pipes")
	if err := c.removeDirectory(pipesDir, "pipes"); err != nil {
		// This might already be removed with job root, so just log
		log.Debug("pipes directory cleanup", "error", err)
	}

	// 4. Clean up any workspace directories
	workspaceDir := filepath.Join(c.config.Filesystem.BaseDir, jobID, "work")
	if err := c.removeDirectory(workspaceDir, "workspace"); err != nil {
		log.Debug("workspace directory cleanup", "error", err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("filesystem cleanup had %d errors: %v", len(errors), errors)
	}

	return nil
}

// cleanupAdditionalResources cleans up any additional resources
func (c *Coordinator) cleanupAdditionalResources(jobID string) error {
	log := c.logger.WithField("operation", "additional-cleanup")
	log.Debug("cleaning up additional resources", "jobID", jobID)

	// Clean up any network namespaces (if applicable)
	// Clean up any IPC resources
	// Clean up any other job-specific resources

	// For now, this is a placeholder for future resource types
	return nil
}

// removeDirectory safely removes a directory
func (c *Coordinator) removeDirectory(path string, description string) error {
	log := c.logger.WithFields("path", path, "type", description)

	// Check if directory exists
	if _, err := c.platform.Stat(path); err != nil {
		if c.platform.IsNotExist(err) {
			log.Debug("directory does not exist, nothing to remove")
			return nil
		}
		return fmt.Errorf("failed to stat %s directory: %w", description, err)
	}

	// Remove the directory
	if err := c.platform.RemoveAll(path); err != nil {
		log.Error("failed to remove directory", "error", err)
		return fmt.Errorf("failed to remove %s directory: %w", description, err)
	}

	log.Debug("directory removed successfully")
	return nil
}

// GetCleanupStatus returns the current cleanup status for a job
func (c *Coordinator) GetCleanupStatus(jobID string) (*CleanupStatus, bool) {
	if status, exists := c.activeCleanups.Load(jobID); exists {
		return status.(*CleanupStatus), true
	}
	return nil, false
}

// CleanupOrphanedResources cleans up resources for jobs that no longer exist
func (c *Coordinator) CleanupOrphanedResources(activeJobIDs map[string]bool) error {
	log := c.logger.WithField("operation", "orphaned-cleanup")
	log.Debug("starting orphaned resource cleanup")

	errors := make([]error, 0)
	cleanedCount := 0

	// Check job directories
	entries, err := c.platform.ReadDir(c.config.Filesystem.BaseDir)
	if err != nil {
		return fmt.Errorf("failed to read job base directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		jobID := entry.Name()

		// Skip if job is active
		if activeJobIDs[jobID] {
			continue
		}

		// Skip if cleanup is in progress
		if _, cleaning := c.activeCleanups.Load(jobID); cleaning {
			continue
		}

		log.Debug("found orphaned job resources", "jobID", jobID)

		// Clean up orphaned resources
		if err := c.CleanupJob(jobID); err != nil {
			log.Error("failed to clean orphaned job", "jobID", jobID, "error", err)
			errors = append(errors, fmt.Errorf("job %s: %w", jobID, err))
		} else {
			cleanedCount++
		}
	}

	log.Info("orphaned resource cleanup completed",
		"cleaned", cleanedCount,
		"errors", len(errors))

	if len(errors) > 0 {
		return fmt.Errorf("cleaned %d orphaned jobs with %d errors", cleanedCount, len(errors))
	}

	return nil
}

// SchedulePeriodicCleanup starts a periodic cleanup routine
func (c *Coordinator) SchedulePeriodicCleanup(ctx context.Context, interval time.Duration, getActiveJobs func() map[string]bool) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("periodic cleanup stopped")
			return
		case <-ticker.C:
			activeJobs := getActiveJobs()
			if err := c.CleanupOrphanedResources(activeJobs); err != nil {
				c.logger.Error("periodic cleanup failed", "error", err)
			}
		}
	}
}
