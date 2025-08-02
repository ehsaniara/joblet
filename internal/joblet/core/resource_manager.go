package core

import (
	"context"
	"fmt"
	"path/filepath"

	"joblet/internal/joblet/core/filesystem"
	"joblet/internal/joblet/core/resource"
	"joblet/internal/joblet/core/upload"
	"joblet/internal/joblet/domain"
	"joblet/pkg/config"
	"joblet/pkg/logger"
	"joblet/pkg/platform"
)

// ResourceManager handles all resource-related operations for jobs
type ResourceManager struct {
	cgroup     resource.Resource
	filesystem *filesystem.Isolator
	platform   platform.Platform
	config     *config.Config
	logger     *logger.Logger
	uploadMgr  *upload.Manager
}

// NewResourceManager creates a new resource manager
func NewResourceManager(
	cgroup resource.Resource,
	filesystem *filesystem.Isolator,
	platform platform.Platform,
	config *config.Config,
	uploadMgr *upload.Manager,
	logger *logger.Logger,
) *ResourceManager {
	return &ResourceManager{
		cgroup:     cgroup,
		filesystem: filesystem,
		platform:   platform,
		config:     config,
		logger:     logger.WithField("component", "resource-manager"),
		uploadMgr:  uploadMgr,
	}
}

// SetupJobResources sets up all resources for a job (cgroup, filesystem)
func (rm *ResourceManager) SetupJobResources(job *domain.Job) error {
	log := rm.logger.WithField("jobID", job.Id)
	log.Debug("setting up job resources")

	// Create workspace directory
	if err := rm.createWorkspace(job.Id); err != nil {
		return err
	}

	// Create cgroup with resource limits
	if err := rm.createCgroup(job); err != nil {
		rm.cleanupWorkspace(job.Id)
		return err
	}

	// Apply CPU core restrictions if specified
	if !job.Limits.CPUCores.IsEmpty() {
		if err := rm.applyCPUCoreRestrictions(job); err != nil {
			rm.cleanupAll(job.Id)
			return fmt.Errorf("CPU core setup failed: %w", err)
		}
	}

	log.Info("job resources setup completed")
	return nil
}

// PrepareScheduledJobUploads prepares uploads for a scheduled job
func (rm *ResourceManager) PrepareScheduledJobUploads(ctx context.Context, job *domain.Job, uploads []domain.FileUpload) error {
	if len(uploads) == 0 {
		return nil
	}

	log := rm.logger.WithField("jobID", job.Id)
	log.Debug("preparing uploads for scheduled job", "count", len(uploads))

	// Create workspace
	workspaceDir := rm.getWorkspaceDir(job.Id)
	if err := rm.platform.MkdirAll(workspaceDir, 0755); err != nil {
		return fmt.Errorf("failed to create workspace: %w", err)
	}

	// Setup cgroup early for resource limits during upload
	if err := rm.createCgroup(job); err != nil {
		_ = rm.platform.RemoveAll(filepath.Dir(workspaceDir))
		return fmt.Errorf("failed to create cgroup: %w", err)
	}

	// Process uploads
	streamConfig := &upload.StreamConfig{
		JobID:        job.Id,
		Uploads:      uploads,
		MemoryLimit:  job.Limits.Memory.Megabytes(),
		WorkspaceDir: workspaceDir,
	}

	if err := rm.uploadMgr.ProcessDirectUploads(ctx, streamConfig); err != nil {
		return fmt.Errorf("upload processing failed: %w", err)
	}

	log.Info("scheduled job uploads prepared")
	return nil
}

// Private helper methods

func (rm *ResourceManager) createWorkspace(jobID string) error {
	baseDir := filepath.Join(rm.config.Filesystem.BaseDir, jobID)
	if err := rm.platform.MkdirAll(baseDir, 0755); err != nil {
		return fmt.Errorf("failed to create workspace: %w", err)
	}
	return nil
}

func (rm *ResourceManager) createCgroup(job *domain.Job) error {
	if err := rm.cgroup.Create(
		job.CgroupPath,
		job.Limits.CPU.Value(),
		job.Limits.Memory.Megabytes(),
		int32(job.Limits.IOBandwidth.BytesPerSecond()),
	); err != nil {
		return fmt.Errorf("cgroup creation failed: %w", err)
	}
	return nil
}

func (rm *ResourceManager) applyCPUCoreRestrictions(job *domain.Job) error {
	log := rm.logger.WithFields("jobID", job.Id, "cores", job.Limits.CPUCores)
	log.Debug("applying CPU core restrictions")

	if err := rm.cgroup.SetCPUCores(job.CgroupPath, job.Limits.CPUCores.String()); err != nil {
		return fmt.Errorf("failed to set CPU cores: %w", err)
	}

	log.Info("CPU core restrictions applied")
	return nil
}

func (rm *ResourceManager) getWorkspaceDir(jobID string) string {
	return filepath.Join(rm.config.Filesystem.BaseDir, jobID, "work")
}

func (rm *ResourceManager) cleanupWorkspace(jobID string) {
	baseDir := filepath.Join(rm.config.Filesystem.BaseDir, jobID)
	if err := rm.platform.RemoveAll(baseDir); err != nil {
		rm.logger.Error("failed to cleanup workspace",
			"jobID", jobID, "error", err)
	}
}

func (rm *ResourceManager) cleanupAll(jobID string) {
	rm.cgroup.CleanupCgroup(jobID)
	rm.cleanupWorkspace(jobID)
}
