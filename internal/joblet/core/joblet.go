//go:build linux

package core

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/ehsaniara/joblet/internal/joblet/adapters"
	"github.com/ehsaniara/joblet/internal/joblet/core/cleanup"
	"github.com/ehsaniara/joblet/internal/joblet/core/filesystem"
	"github.com/ehsaniara/joblet/internal/joblet/core/interfaces"
	"github.com/ehsaniara/joblet/internal/joblet/core/job"
	"github.com/ehsaniara/joblet/internal/joblet/core/process"
	"github.com/ehsaniara/joblet/internal/joblet/core/resource"
	"github.com/ehsaniara/joblet/internal/joblet/core/unprivileged"
	"github.com/ehsaniara/joblet/internal/joblet/core/upload"
	"github.com/ehsaniara/joblet/internal/joblet/domain"
	"github.com/ehsaniara/joblet/internal/joblet/gpu"
	metricsdomain "github.com/ehsaniara/joblet/internal/joblet/metrics/domain"
	"github.com/ehsaniara/joblet/internal/joblet/scheduler"
	"github.com/ehsaniara/joblet/pkg/config"
	"github.com/ehsaniara/joblet/pkg/logger"
	"github.com/ehsaniara/joblet/pkg/platform"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

// Joblet orchestrates job execution using specialized components.
// Main entry point for job management - coordinates validation, building,
// resource allocation, execution, and cleanup for all job types.
type Joblet struct {
	// Core dependencies
	store        adapters.JobStorer
	metricsStore *adapters.MetricsStoreAdapter
	config       *config.Config
	logger       *logger.Logger
	platform     platform.Platform

	// Specialized services
	jobBuilder      *job.Builder
	resourceManager *ResourceManager
	executionEngine *ExecutionEngineV2
	scheduler       *scheduler.Scheduler
	cleanup         *cleanup.Coordinator
}

// NewPlatformJoblet creates a new Linux platform joblet with specialized components.
// Initializes all core services, starts the scheduler, and begins periodic cleanup.
// Returns a fully configured joblet ready for job execution.
func NewPlatformJoblet(store adapters.JobStorer, metricsStore *adapters.MetricsStoreAdapter, cfg *config.Config, networkStoreAdapter adapters.NetworkStorer) interfaces.Joblet {
	platformInterface := platform.NewPlatform()
	jobletLogger := logger.New().WithField("component", "linux-joblet")

	// Initialize all specialized components (use adapter directly)
	c := initializeComponents(store, cfg, platformInterface, jobletLogger, networkStoreAdapter)

	// Create the joblet
	j := &Joblet{
		store:           store,
		metricsStore:    metricsStore,
		config:          cfg,
		logger:          jobletLogger,
		platform:        platformInterface,
		jobBuilder:      c.jobBuilder,
		resourceManager: c.resourceManager,
		executionEngine: c.executionEngine,
		cleanup:         c.cleanup,
	}

	// Create scheduler with simplified executor
	s := scheduler.New(&jobletExecutor{j})
	j.scheduler = s

	// Setup cgroup controllers
	if err := c.cgroup.EnsureControllers(); err != nil {
		j.logger.Fatal("cgroup controller setup failed", "error", err)
	}

	// Start the scheduler
	if err := j.scheduler.Start(); err != nil {
		j.logger.Fatal("scheduler start failed", "error", err)
	}

	// Start periodic cleanup
	go j.cleanup.SchedulePeriodicCleanup(
		context.Background(),
		5*time.Minute,
		j.getActiveJobIDs,
	)

	return j
}

// StartJob validates and starts a job (immediate or scheduled).
// Main job entry point - validates request, builds job domain object,
// then routes to either immediate execution or scheduler based on schedule field.
func (j *Joblet) StartJob(ctx context.Context, req interfaces.StartJobRequest) (*domain.Job, error) {
	j.logger.Debug("CORE JOBLET StartJob called",
		"command", req.Command,
		"network", req.Network,
		"volumes", req.Volumes,
		"runtime", req.Runtime)
	j.logger.Debug("StartJob called",
		"command", req.Command,
		"network", req.Network,
		"args", req.Args)

	// Convert interface request to internal request using simplified approach
	limits := domain.NewResourceLimitsFromParams(
		req.Resources.MaxCPU,
		req.Resources.CPUCores,
		req.Resources.MaxMemory,
		int64(req.Resources.MaxIOBPS),
	)

	// Build internal request
	internalReq := job.BuildRequest{
		Command:           req.Command,
		Args:              req.Args,
		Limits:            *limits,
		Schedule:          req.Schedule,
		Uploads:           req.Uploads,
		Network:           req.Network,
		Volumes:           req.Volumes,
		Runtime:           req.Runtime,
		Environment:       req.Environment,
		SecretEnvironment: req.SecretEnvironment,
		JobType:           req.JobType,
		WorkflowUuid:      req.WorkflowUuid,
		WorkingDirectory:  req.WorkingDirectory,
		Dependencies:      req.Dependencies,
		GPUCount:          req.GPUCount,    // GPU requirements
		GPUMemoryMB:       req.GPUMemoryMB, // GPU memory requirement
	}

	log := j.logger.WithFields(
		"command", req.Command,
		"uploadCount", len(req.Uploads),
		"schedule", req.Schedule,
		"network", req.Network,
	)
	log.Debug("starting job")

	// Check context
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// 1. Basic request validation (simplified)
	if internalReq.Command == "" {
		return nil, fmt.Errorf("command cannot be empty")
	}

	// 2. Build the job
	jb, err := j.jobBuilder.Build(internalReq)
	if err != nil {
		return nil, fmt.Errorf("job creation failed: %w", err)
	}

	// 3. Route to appropriate handler
	if internalReq.Schedule != "" {
		return j.scheduleJob(ctx, jb, internalReq)
	}
	return j.executeJob(ctx, jb, internalReq)
}

// scheduleJob handles scheduled job execution by parsing the schedule time,
// preparing uploads, and queuing the job for future execution. Validates
// schedule format, pre-processes uploads, and registers with scheduler.
func (j *Joblet) scheduleJob(ctx context.Context, job *domain.Job, req job.BuildRequest) (*domain.Job, error) {
	log := j.logger.WithField("jobID", job.Uuid)

	// Parse and set scheduled time
	scheduledTime, err := time.Parse(time.RFC3339, req.Schedule)
	if err != nil {
		return nil, fmt.Errorf("invalid schedule format: %w", err)
	}

	job.ScheduledTime = &scheduledTime
	job.Status = domain.StatusScheduled

	log.Info("scheduling job", "scheduledTime", scheduledTime.Format(time.RFC3339))

	// Pre-process uploads for scheduled jobs
	if len(req.Uploads) > 0 {
		if err := j.resourceManager.PrepareScheduledJobUploads(ctx, job, req.Uploads); err != nil {
			return nil, fmt.Errorf("upload preparation failed: %w", err)
		}
	}

	// Register and schedule - Debug the field values before storage
	log.Debug("storing scheduled job with field values",
		"jobId", job.Uuid,
		"network", job.Network,
		"volumes", job.Volumes,
		"runtime", job.Runtime,
		"hasNetwork", job.Network != "",
		"volumeCount", len(job.Volumes),
		"hasRuntime", job.Runtime != "")

	j.store.CreateNewJob(job)

	if e := j.scheduler.AddJob(job); e != nil {
		// Skip cleanup for runtime build jobs even on scheduling failure
		if !job.Type.IsRuntimeBuild() {
			_ = j.cleanup.CleanupJob(job.Uuid)
		}
		return nil, fmt.Errorf("scheduling failed: %w", e)
	}

	return job, nil
}

// executeJob handles immediate job execution by setting up resources,
// coordinating with the execution engine, and starting monitoring.
// Manages complete lifecycle: resource setup → execution → monitoring.
func (j *Joblet) executeJob(ctx context.Context, job *domain.Job, req job.BuildRequest) (*domain.Job, error) {
	log := j.logger.WithField("jobID", job.Uuid)
	log.Debug("executing job immediately")

	// Setup resources
	if err := j.resourceManager.SetupJobResources(job); err != nil {
		return nil, fmt.Errorf("resource setup failed: %w", err)
	}

	// Register job - Debug the field values before storage
	log.Debug("storing job with field values",
		"jobId", job.Uuid,
		"network", job.Network,
		"volumes", job.Volumes,
		"runtime", job.Runtime,
		"hasNetwork", job.Network != "",
		"volumeCount", len(job.Volumes),
		"hasRuntime", job.Runtime != "")

	j.store.CreateNewJob(job)

	// Start execution
	log.Debug("calling execution engine with job volumes", "jobId", job.Uuid, "volumes", job.Volumes, "volumeCount", len(job.Volumes))
	cmd, err := j.executionEngine.StartProcessWithUploads(ctx, job, req.Uploads)
	if err != nil {
		j.handleExecutionFailure(job)
		return nil, fmt.Errorf("execution failed: %w", err)
	}

	// Update job state
	j.updateJobRunning(job, cmd)

	// Start metrics collection (always enabled for pubsub live streaming)
	// Metrics are sent to pubsub for real-time clients AND to persist via IPC
	if j.metricsStore != nil {
		sampleInterval := 5 * time.Second // Default sample interval

		// Get GPU indices from job if allocated
		var gpuIndices []int
		if len(job.GPUIndices) > 0 {
			gpuIndices = make([]int, len(job.GPUIndices))
			for i, idx := range job.GPUIndices {
				gpuIndices[i] = int(idx)
			}
		}

		// Convert job limits to metrics domain limits
		metricsLimits := &metricsdomain.ResourceLimits{
			CPU:    job.Limits.CPU.Value(),
			Memory: job.Limits.Memory.Bytes(),
			IO:     int32(job.Limits.IOBandwidth.BytesPerSecond()),
		}

		err := j.metricsStore.StartCollector(
			job.Uuid,
			job.CgroupPath,
			sampleInterval,
			metricsLimits,
			gpuIndices,
		)
		if err != nil {
			log.Warn("failed to start metrics collector", "error", err)
			// Don't fail the job if metrics collection fails
		} else {
			log.Debug("metrics collector started", "sampleInterval", sampleInterval)
		}
	}

	// Monitor asynchronously
	go j.monitorJob(ctx, cmd, job)

	log.Info("job started", "pid", job.Pid)
	return job, nil
}

// ExecuteScheduledJob implements the interfaces.Joblet interface for scheduled job execution.
// Called by external components that depend on the interface contract.
func (j *Joblet) ExecuteScheduledJob(ctx context.Context, req interfaces.ExecuteScheduledJobRequest) error {
	return j.executeScheduledJob(ctx, req.Job)
}

// executeScheduledJob implements the actual scheduled job execution logic.
// Used by both the interface method and scheduler.JobExecutor interface.
func (j *Joblet) executeScheduledJob(ctx context.Context, jobObj *domain.Job) error {
	log := j.logger.WithField("jobID", jobObj.Uuid)
	log.Info("executing scheduled job")

	// Get fresh job state from store to check for cancellation
	freshJob, exists := j.store.Job(jobObj.Uuid)
	if !exists {
		return fmt.Errorf("job not found: %s", jobObj.Uuid)
	}

	// Prevent execution of canceled jobs (defensive check against race conditions)
	if freshJob.Status == domain.StatusCanceled {
		log.Info("skipping execution of canceled job")
		return fmt.Errorf("job was canceled before execution: %s", jobObj.Uuid)
	}

	// Ensure job is still scheduled
	if freshJob.Status != domain.StatusScheduled {
		log.Warn("job is not in scheduled status", "currentStatus", freshJob.Status)
		return fmt.Errorf("job is not scheduled (status: %s)", freshJob.Status)
	}

	// Transition state
	freshJob.Status = domain.StatusInitializing
	j.store.UpdateJob(freshJob)

	// Execute (uploads already processed during scheduling)
	_, err := j.executeJob(ctx, freshJob, job.BuildRequest{})
	return err
}

// StopJob stops a running or scheduled job.
// Handles both scheduled (removes from scheduler) and running jobs (terminates process).
// Special handling for runtime builds to preserve filesystem artifacts.
func (j *Joblet) StopJob(ctx context.Context, req interfaces.StopJobRequest) error {
	log := j.logger.WithField("jobID", req.JobID)
	log.Debug("stopping job", "force", req.Force, "reason", req.Reason)

	jb, exists := j.store.Job(req.JobID)
	if !exists {
		return fmt.Errorf("job not found: %s", req.JobID)
	}

	// Handle scheduled jobs
	if jb.IsScheduled() {
		if j.scheduler.RemoveJob(req.JobID) {
			jb.Status = domain.StatusCanceled
			j.store.UpdateJob(jb)
			// Skip cleanup for runtime build jobs even when stopped
			if !jb.Type.IsRuntimeBuild() {
				_ = j.cleanup.CleanupJob(req.JobID)
			}
			log.Info("scheduled job cancelled")
			return nil
		}
		return fmt.Errorf("failed to remove scheduled job")
	}

	// Handle running jobs
	if !jb.IsRunning() {
		return fmt.Errorf("job is not running: %s (status: %s)", req.JobID, jb.Status)
	}

	// Check if cleanup is already in progress (from monitor)
	if status, exists := j.cleanup.GetCleanupStatus(req.JobID); exists {
		log.Debug("cleanup already in progress", "started", status.StartTime)
		// Just update the job state
		jb.Status = domain.StatusStopped
		j.store.UpdateJob(jb)
		return nil
	}

	// Stop the process and cleanup - but handle runtime builds specially
	var err error
	if jb.Type.IsRuntimeBuild() {
		// For runtime builds: system cleanup only (cgroups, process) but preserve filesystem
		log.Info("stopping runtime build job with partial cleanup - preserving artifacts in /opt/joblet/runtimes")
		// Use a special cleanup path that preserves filesystem artifacts
		err = j.cleanup.CleanupJobWithProcessSystemOnly(ctx, req.JobID, jb.Pid)
	} else {
		// For regular jobs: do full cleanup
		err = j.cleanup.CleanupJobWithProcess(ctx, req.JobID, jb.Pid)
	}

	// Update state regardless of cleanup result
	jb.Status = domain.StatusStopped
	j.store.UpdateJob(jb)

	if err != nil {
		// If cleanup is already in progress, that's OK
		if err.Error() == fmt.Sprintf("cleanup already in progress for job %s", req.JobID) {
			log.Debug("cleanup initiated by monitor, stop command completed")
			return nil
		}
		return fmt.Errorf("cleanup failed: %w", err)
	}

	log.Info("job stopped")
	return nil
}

// DeleteJob completely removes a job including logs and metadata.
// Prevents deletion of active jobs, delegates to job store for data removal,
// and performs final resource cleanup (preserves runtime build artifacts).
func (j *Joblet) DeleteJob(ctx context.Context, req interfaces.DeleteJobRequest) error {
	log := j.logger.WithField("jobID", req.JobID)
	log.Debug("deleting job", "reason", req.Reason)

	// Check if job exists
	jb, exists := j.store.Job(req.JobID)
	if !exists {
		return fmt.Errorf("job not found: %s", req.JobID)
	}

	// Prevent deletion of running jobs
	if jb.IsRunning() || jb.IsScheduled() {
		return fmt.Errorf("cannot delete job %s (status: %s) - stop the job first", req.JobID, jb.Status)
	}

	log.Info("deleting job completely", "status", jb.Status, "reason", req.Reason)

	// Use the job store adapter's DeleteJob method which handles:
	// 1. Task wrapper cleanup
	// 2. Buffer removal
	// 3. Log deletion via async system
	// 4. Job record removal
	// 5. Event publishing
	err := j.store.DeleteJob(req.JobID)
	if err != nil {
		log.Error("job deletion failed", "error", err)
		return fmt.Errorf("job deletion failed: %w", err)
	}

	// Delete metrics files if metrics system is enabled
	if j.metricsStore != nil {
		if err := j.metricsStore.DeleteJobMetrics(req.JobID); err != nil {
			log.Warn("failed to delete job metrics", "error", err)
			// Continue with deletion even if metrics cleanup fails
		}
	}

	// Cleanup any remaining resources - handle runtime builds specially
	if jb.Type.IsRuntimeBuild() {
		// For runtime builds: only clean system resources, preserve artifacts
		_ = j.cleanup.CleanupJobSystemResourcesOnly(req.JobID)
		log.Info("runtime build job deleted - system resources cleaned, artifacts preserved")
	} else {
		// For regular jobs: full cleanup
		_ = j.cleanup.CleanupJob(req.JobID)
	}

	log.Info("job deleted successfully")
	return nil
}

// DeleteAllJobs removes all non-running jobs from the system, including logs and metadata.
// Iterates through all jobs in the store, identifies non-running ones, and deletes them.
// Returns counts of deleted and skipped jobs. Skips running and scheduled jobs.
func (j *Joblet) DeleteAllJobs(ctx context.Context, req interfaces.DeleteAllJobsRequest) (*interfaces.DeleteAllJobsResponse, error) {
	log := j.logger.WithField("operation", "DeleteAllJobs")
	log.Info("bulk job deletion requested", "reason", req.Reason)

	// Get all jobs from the store
	allJobs := j.store.ListJobs()

	deletedCount := 0
	skippedCount := 0
	var errors []string

	for _, job := range allJobs {
		// Skip running and scheduled jobs
		if job.IsRunning() || job.IsScheduled() {
			skippedCount++
			log.Debug("skipping job", "jobID", job.Uuid, "status", job.Status)
			continue
		}

		// Delete the job using the existing delete logic
		deleteRequest := interfaces.DeleteJobRequest{
			JobID:  job.Uuid,
			Reason: req.Reason,
		}

		err := j.DeleteJob(ctx, deleteRequest)
		if err != nil {
			log.Error("failed to delete job", "jobID", job.Uuid, "error", err)
			errors = append(errors, fmt.Sprintf("job %s: %v", job.Uuid, err))
			continue
		}

		// Also delete logs for delete-all operations to match documented behavior
		err = j.store.DeleteJobLogs(job.Uuid)
		if err != nil {
			log.Warn("failed to delete logs for job", "jobID", job.Uuid, "error", err)
			// Continue with deletion even if log cleanup fails
		}

		// Delete metrics for delete-all operations
		if j.metricsStore != nil {
			err = j.metricsStore.DeleteJobMetrics(job.Uuid)
			if err != nil {
				log.Warn("failed to delete metrics for job", "jobID", job.Uuid, "error", err)
				// Continue with deletion even if metrics cleanup fails
			}
		}

		deletedCount++
		log.Debug("job deleted", "jobID", job.Uuid)
	}

	if len(errors) > 0 {
		log.Warn("some jobs failed to delete", "errors", len(errors))
		return nil, fmt.Errorf("failed to delete %d jobs: %s", len(errors), strings.Join(errors, "; "))
	}

	log.Info("bulk job deletion completed",
		"deletedCount", deletedCount,
		"skippedCount", skippedCount)

	return &interfaces.DeleteAllJobsResponse{
		DeletedCount: deletedCount,
		SkippedCount: skippedCount,
	}, nil
}

// monitorJob monitors a running job until completion asynchronously.
// Waits for process completion, determines exit code, updates job status,
// and triggers cleanup (special handling for runtime builds to preserve artifacts).
func (j *Joblet) monitorJob(ctx context.Context, cmd platform.Command, job *domain.Job) {
	log := j.logger.WithField("jobID", job.Uuid)
	log.Debug("starting job monitoring")

	// Wait for completion
	err := cmd.Wait()

	// Give a brief moment for final log chunks to be written and published
	// cmd.Wait() ensures pipes are closed, but async pubsub publishes might still be in flight
	time.Sleep(300 * time.Millisecond)

	// Determine final status
	var exitCode int32
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = int32(exitErr.ExitCode())
		} else {
			exitCode = -1
		}
		job.Status = domain.StatusFailed
		job.ExitCode = exitCode
		job.EndTime = &[]time.Time{time.Now()}[0]
	} else {
		exitCode = 0
		job.Status = domain.StatusCompleted
		job.ExitCode = exitCode
		job.EndTime = &[]time.Time{time.Now()}[0]
	}

	// Update state
	j.store.UpdateJob(job)

	// Stop metrics collection if enabled
	if j.metricsStore != nil {
		if err := j.metricsStore.StopCollector(job.Uuid); err != nil {
			log.Warn("failed to stop metrics collector", "error", err)
		} else {
			log.Debug("metrics collector stopped")
		}
	}

	// Cleanup resources - but handle runtime build jobs specially
	if job.Type.IsRuntimeBuild() {
		// For runtime builds: clean system resources but preserve filesystem artifacts
		if err := j.cleanup.CleanupJobSystemResourcesOnly(job.Uuid); err != nil {
			log.Error("system resource cleanup failed for runtime build job", "error", err)
		} else {
			log.Info("runtime build job completed - system resources cleaned, artifacts preserved",
				"jobType", job.Type, "runtimesPath", "/opt/joblet/runtimes")
		}
	} else {
		// For regular jobs: full cleanup
		if err := j.cleanup.CleanupJob(job.Uuid); err != nil {
			log.Error("cleanup failed during monitoring", "error", err)
		}
	}

	log.Info("job completed", "exitCode", exitCode)
}

// Helper methods

// updateJobRunning transitions job to running state and captures process PID.
// Called after successful process start to record execution details.
func (j *Joblet) updateJobRunning(job *domain.Job, cmd platform.Command) {
	if proc := cmd.Process(); proc != nil {
		job.Pid = int32(proc.Pid())
	}
	job.Status = domain.StatusRunning
	j.store.UpdateJob(job)
}

// handleExecutionFailure handles job execution failures by updating status,
// setting failure exit code, and triggering appropriate cleanup based on job type.
func (j *Joblet) handleExecutionFailure(job *domain.Job) {
	job.Status = domain.StatusFailed
	job.ExitCode = -1
	job.EndTime = &[]time.Time{time.Now()}[0]
	j.store.UpdateJob(job)

	// Handle cleanup for failed jobs - runtime builds get partial cleanup
	if job.Type.IsRuntimeBuild() {
		// For failed runtime builds: clean system resources but preserve partial artifacts
		if err := j.cleanup.CleanupJobSystemResourcesOnly(job.Uuid); err != nil {
			j.logger.Error("system resource cleanup failed for failed runtime build job", "error", err)
		} else {
			j.logger.Info("failed runtime build job - system resources cleaned, partial artifacts preserved",
				"jobType", job.Type, "jobID", job.Uuid)
		}
	} else {
		if err := j.cleanup.CleanupJob(job.Uuid); err != nil {
			j.logger.Error("cleanup failed after execution failure",
				"jobID", job.Uuid, "error", err)
		}
	}
}

// getActiveJobIDs returns a map of all active job IDs for cleanup coordination.
// Used by periodic cleanup to avoid cleaning up jobs that are still active.
func (j *Joblet) getActiveJobIDs() map[string]bool {
	jobs := j.store.ListJobs()

	activeIDs := make(map[string]bool)
	for _, jb := range jobs {
		activeIDs[jb.Uuid] = true
	}
	return activeIDs
}

// initializeComponents creates all specialized components for job execution.
// Sets up validation, job building, resource management, execution engine,
// and cleanup coordinator with proper dependencies and configuration.
func initializeComponents(store adapters.JobStorer, cfg *config.Config, platform platform.Platform, logger *logger.Logger, networkStore adapters.NetworkStorer) *components {
	// Create core resources
	cgroupResource := resource.New(cfg.Cgroup)
	filesystemIsolator := filesystem.NewIsolator(cfg, platform)
	jobIsolation := unprivileged.NewJobIsolation()

	// Create managers
	processManager := process.NewProcessManager(platform, cfg)
	uploadManager := upload.NewManager(platform, logger)

	// Create GPU manager
	gpuManager := createGPUManager(cfg.GPU, platform, logger)

	// Simplified validation - removed complex validation service

	// Create UUID generator for job identification
	uuidGenerator := job.NewUUIDGenerator("job", "node")
	jobBuilder := job.NewBuilder(cfg, uuidGenerator)

	// Create resource manager
	resourceManager := &ResourceManager{
		cgroup:     cgroupResource,
		filesystem: filesystemIsolator,
		platform:   platform,
		config:     cfg,
		logger:     logger.WithField("component", "resource-manager"),
		uploadMgr:  uploadManager,
	}

	// Create execution engine using the coordinator pattern
	executionEngine := NewExecutionEngineV2(
		processManager,
		uploadManager,
		platform,
		store,
		cfg,
		logger,
		jobIsolation,
		networkStore,
		gpuManager,
	)

	// Create cleanup coordinator with network store adapter
	c := cleanup.NewCoordinator(
		processManager,
		cgroupResource,
		platform,
		cfg,
		logger,
		networkStore,
	)

	return &components{
		cgroup:          cgroupResource,
		jobBuilder:      jobBuilder,
		resourceManager: resourceManager,
		executionEngine: executionEngine,
		cleanup:         c,
	}
}

// components holds all initialized components.
// Temporary struct to organize component initialization and dependency injection
// before final joblet assembly.
type components struct {
	cgroup          resource.Resource
	jobBuilder      *job.Builder
	resourceManager *ResourceManager
	executionEngine *ExecutionEngineV2
	cleanup         *cleanup.Coordinator
}

// jobletExecutor adapts joblet to scheduler.JobExecutor interface
type jobletExecutor struct {
	joblet *Joblet
}

func (je *jobletExecutor) ExecuteScheduledJob(ctx context.Context, job *domain.Job) error {
	return je.joblet.executeScheduledJob(ctx, job)
}

// createGPUManager creates and initializes a GPU manager based on configuration
func createGPUManager(gpuConfig config.GPUConfig, platform platform.Platform, logger *logger.Logger) gpu.GPUManagerInterface {
	// Create GPU discovery service
	gpuDiscovery := gpu.NewNvidiaDiscovery(platform)

	// Create CUDA detector
	cudaDetector := gpu.NewCUDADetector(platform)

	// Create and initialize GPU manager
	gpuManager := gpu.NewManager(gpuConfig, gpuDiscovery, cudaDetector)

	// Initialize GPU manager (discover GPUs)
	if err := gpuManager.Initialize(); err != nil {
		if gpuConfig.Enabled {
			logger.Error("GPU manager initialization failed", "error", err)
			// Continue without GPU support rather than failing completely
		} else {
			logger.Debug("GPU manager initialization skipped (GPU support disabled)")
		}
	}

	return gpuManager
}
