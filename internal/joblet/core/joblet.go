//go:build linux

package core

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"

	"joblet/internal/joblet/core/cleanup"
	"joblet/internal/joblet/core/filesystem"
	"joblet/internal/joblet/core/interfaces"
	"joblet/internal/joblet/core/job"
	"joblet/internal/joblet/core/process"
	"joblet/internal/joblet/core/resource"
	"joblet/internal/joblet/core/unprivileged"
	"joblet/internal/joblet/core/upload"
	"joblet/internal/joblet/core/validation"
	"joblet/internal/joblet/domain"
	"joblet/internal/joblet/scheduler"
	"joblet/internal/joblet/state"
	"joblet/pkg/config"
	"joblet/pkg/logger"
	"joblet/pkg/platform"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

// Joblet orchestrates job execution using specialized components
type Joblet struct {
	// Core dependencies
	store    state.Store
	config   *config.Config
	logger   *logger.Logger
	platform platform.Platform

	// Specialized services
	validator       *validation.Service
	jobBuilder      *job.Builder
	resourceManager *ResourceManager
	executionEngine *ExecutionEngine
	scheduler       *scheduler.Scheduler
	cleanup         *cleanup.Coordinator
}

// StartJobRequest contains all parameters for starting a job
type StartJobRequest struct {
	Command  string
	Args     []string
	Limits   domain.ResourceLimits
	Uploads  []domain.FileUpload
	Schedule string
	Network  string
	Volumes  []string
}

func (r StartJobRequest) GetCommand() string               { return r.Command }
func (r StartJobRequest) GetArgs() []string                { return r.Args }
func (r StartJobRequest) GetSchedule() string              { return r.Schedule }
func (r StartJobRequest) GetLimits() domain.ResourceLimits { return r.Limits }
func (r StartJobRequest) GetNetwork() string               { return r.Network }
func (r StartJobRequest) GetVolumes() []string             { return r.Volumes }

// NewPlatformJoblet creates a new Linux platform joblet with specialized components
func NewPlatformJoblet(store state.Store, cfg *config.Config, networkStore *state.NetworkStore) interfaces.Joblet {
	platformInterface := platform.NewPlatform()
	jobletLogger := logger.New().WithField("component", "linux-joblet")

	// Initialize all specialized c
	c := initializeComponents(store, cfg, platformInterface, jobletLogger, networkStore)

	// Create the joblet
	j := &Joblet{
		store:           store,
		config:          cfg,
		logger:          jobletLogger,
		platform:        platformInterface,
		validator:       c.validator,
		jobBuilder:      c.jobBuilder,
		resourceManager: c.resourceManager,
		executionEngine: c.executionEngine,
		cleanup:         c.cleanup,
	}

	// Create scheduler with adapter
	s := scheduler.New(&schedulerAdapter{joblet: j})
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

// StartJob validates and starts a job (immediate or scheduled)
func (j *Joblet) StartJob(ctx context.Context, req interfaces.StartJobRequest) (*domain.Job, error) {
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
	internalReq := StartJobRequest{
		Command:  req.Command,
		Args:     req.Args,
		Limits:   *limits,
		Uploads:  req.Uploads,
		Schedule: req.Schedule,
		Network:  req.Network,
		Volumes:  req.Volumes,
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

	// 1. Validate the request
	if err := j.validator.ValidateJobRequest(internalReq.Command, internalReq.Args, internalReq.Schedule, internalReq.Limits); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
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

// scheduleJob handles scheduled job execution
func (j *Joblet) scheduleJob(ctx context.Context, job *domain.Job, req StartJobRequest) (*domain.Job, error) {
	log := j.logger.WithField("jobID", job.Id)

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

	// Register and schedule
	j.store.CreateNewJob(job)

	if e := j.scheduler.AddJob(job); e != nil {
		_ = j.cleanup.CleanupJob(job.Id)
		return nil, fmt.Errorf("scheduling failed: %w", e)
	}

	return job, nil
}

// executeJob handles immediate job execution
func (j *Joblet) executeJob(ctx context.Context, job *domain.Job, req StartJobRequest) (*domain.Job, error) {
	log := j.logger.WithField("jobID", job.Id)
	log.Debug("executing job immediately")

	// Setup resources
	if err := j.resourceManager.SetupJobResources(job); err != nil {
		return nil, fmt.Errorf("resource setup failed: %w", err)
	}

	// Register job
	j.store.CreateNewJob(job)

	// Start execution
	cmd, err := j.executionEngine.StartProcessWithUploads(ctx, job, req.Uploads)
	if err != nil {
		j.handleExecutionFailure(job)
		return nil, fmt.Errorf("execution failed: %w", err)
	}

	// Update job state
	j.updateJobRunning(job, cmd)

	// Monitor asynchronously
	go j.monitorJob(ctx, cmd, job)

	log.Info("job started", "pid", job.Pid)
	return job, nil
}

// ExecuteScheduledJob implements the interfaces.Joblet interface for scheduled job execution
func (j *Joblet) ExecuteScheduledJob(ctx context.Context, req interfaces.ExecuteScheduledJobRequest) error {
	jobObj := req.Job
	log := j.logger.WithField("jobID", jobObj.Id)
	log.Info("executing scheduled job")

	// Transition state
	jobObj.Status = domain.StatusInitializing
	j.store.UpdateJob(jobObj)

	// Execute (uploads already processed during scheduling)
	_, err := j.executeJob(ctx, jobObj, StartJobRequest{})
	return err
}

// StopJob stops a running or scheduled job
func (j *Joblet) StopJob(ctx context.Context, req interfaces.StopJobRequest) error {
	log := j.logger.WithField("jobID", req.JobID)
	log.Debug("stopping job", "force", req.Force, "reason", req.Reason)

	jb, exists := j.store.GetJob(req.JobID)
	if !exists {
		return fmt.Errorf("job not found: %s", req.JobID)
	}

	// Handle scheduled jobs
	if jb.IsScheduled() {
		if j.scheduler.RemoveJob(req.JobID) {
			jb.Status = domain.StatusStopped
			j.store.UpdateJob(jb)
			_ = j.cleanup.CleanupJob(req.JobID)
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

	// Stop the process and cleanup
	err := j.cleanup.CleanupJobWithProcess(ctx, req.JobID, jb.Pid)

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

// monitorJob monitors a running job until completion
func (j *Joblet) monitorJob(ctx context.Context, cmd platform.Command, job *domain.Job) {
	log := j.logger.WithField("jobID", job.Id)
	log.Debug("starting job monitoring")

	// Wait for completion
	err := cmd.Wait()

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

	// Cleanup resources
	if err := j.cleanup.CleanupJob(job.Id); err != nil {
		log.Error("cleanup failed during monitoring", "error", err)
	}

	log.Info("job completed", "exitCode", exitCode)
}

// Helper methods

func (j *Joblet) updateJobRunning(job *domain.Job, cmd platform.Command) {
	if proc := cmd.Process(); proc != nil {
		job.Pid = int32(proc.Pid())
	}
	job.Status = domain.StatusRunning
	j.store.UpdateJob(job)
}

func (j *Joblet) handleExecutionFailure(job *domain.Job) {
	job.Status = domain.StatusFailed
	job.ExitCode = -1
	job.EndTime = &[]time.Time{time.Now()}[0]
	j.store.UpdateJob(job)
	if err := j.cleanup.CleanupJob(job.Id); err != nil {
		j.logger.Error("cleanup failed after execution failure",
			"jobID", job.Id, "error", err)
	}
}

func (j *Joblet) getActiveJobIDs() map[string]bool {
	jobs := j.store.ListJobs()

	activeIDs := make(map[string]bool)
	for _, jb := range jobs {
		activeIDs[jb.Id] = true
	}
	return activeIDs
}

// initializeComponents creates all the specialized components
func initializeComponents(store state.Store, cfg *config.Config, platform platform.Platform, logger *logger.Logger, networkStore *state.NetworkStore) *components {
	// Create core resources
	cgroupResource := resource.New(cfg.Cgroup)
	filesystemIsolator := filesystem.NewIsolator(cfg.Filesystem, platform)
	jobIsolation := unprivileged.NewJobIsolation()

	// Create managers
	processManager := process.NewProcessManager(platform)
	uploadManager := upload.NewManager(platform, logger)

	// Create validation service
	validator := validation.NewService(
		validation.NewCommandValidator(platform),
		validation.NewScheduleValidator(),
		validation.NewResourceValidator(),
	)

	// Create job builder
	idGenerator := job.NewIDGenerator("job", "node")
	jobBuilder := job.NewBuilder(cfg, idGenerator, validator.ResourceValidator())

	// Create resource manager
	resourceManager := &ResourceManager{
		cgroup:     cgroupResource,
		filesystem: filesystemIsolator,
		platform:   platform,
		config:     cfg,
		logger:     logger.WithField("component", "resource-manager"),
		uploadMgr:  uploadManager,
	}

	// Create execution engine
	executionEngine := NewExecutionEngine(
		processManager,
		uploadManager,
		platform,
		store,
		cfg,
		logger,
		jobIsolation,
		networkStore,
	)

	// Create cleanup coordinator
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
		validator:       validator,
		jobBuilder:      jobBuilder,
		resourceManager: resourceManager,
		executionEngine: executionEngine,
		cleanup:         c,
	}
}

// components holds all initialized components
type components struct {
	cgroup          resource.Resource
	validator       *validation.Service
	jobBuilder      *job.Builder
	resourceManager *ResourceManager
	executionEngine *ExecutionEngine
	cleanup         *cleanup.Coordinator
}

// schedulerAdapter adapts the Joblet to the scheduler.JobExecutor interface
type schedulerAdapter struct {
	joblet *Joblet
}

// ExecuteScheduledJob adapts the old scheduler interface to the new one
func (sa *schedulerAdapter) ExecuteScheduledJob(ctx context.Context, job *domain.Job) error {
	req := interfaces.ExecuteScheduledJobRequest{
		Job: job,
	}
	return sa.joblet.ExecuteScheduledJob(ctx, req)
}
