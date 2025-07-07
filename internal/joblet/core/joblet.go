//go:build linux

package core

import (
	"context"
	"errors"
	"fmt"
	"joblet/internal/joblet/core/interfaces"
	"joblet/internal/joblet/core/process"
	"joblet/internal/joblet/core/resource"
	"joblet/internal/joblet/core/unprivileged"
	"joblet/internal/joblet/domain"
	"joblet/internal/joblet/state"
	"joblet/pkg/config"
	"joblet/pkg/logger"
	"joblet/pkg/platform"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"time"
)

var jobCounter int64

// Joblet handles job execution with configuration
type Joblet struct {
	store          state.Store
	cgroup         resource.Resource
	processManager *process.Manager
	jobIsolation   *unprivileged.JobIsolation
	platform       platform.Platform
	config         *config.Config
	logger         *logger.Logger
}

// NewPlatformJoblet creates a new Linux platform joblet
func NewPlatformJoblet(store state.Store, cfg *config.Config) interfaces.Joblet {
	platformInterface := platform.NewPlatform()
	processManager := process.NewProcessManager(platformInterface)
	cgroupResource := resource.New(cfg.Cgroup)
	jobIsolation := unprivileged.NewJobIsolation()

	w := &Joblet{
		store:          store,
		cgroup:         cgroupResource,
		processManager: processManager,
		jobIsolation:   jobIsolation,
		platform:       platformInterface,
		config:         cfg,
		logger:         logger.New().WithField("component", "linux-joblet"),
	}

	if err := w.setupCgroupControllers(); err != nil {
		w.logger.Fatal("cgroup controller setup failed", "error", err)
	}

	w.logger.Debug("Linux joblet initialized",
		"maxConcurrentJobs", cfg.Joblet.MaxConcurrentJobs,
		"defaultCPU", cfg.Joblet.DefaultCPULimit,
		"defaultMemory", cfg.Joblet.DefaultMemoryLimit,
		"cgroupPath", cfg.Cgroup.BaseDir)

	return w
}

func (w *Joblet) StartJob(ctx context.Context, command string, args []string, maxCPU, maxMemory, maxIOBPS int32) (*domain.Job, error) {
	jobID := w.getNextJobID()
	log := w.logger.WithFields("jobID", jobID, "command", command)

	log.Debug("starting job with configuration",
		"requestedCPU", maxCPU,
		"requestedMemory", maxMemory,
		"requestedIO", maxIOBPS,
		"validateCommands", w.config.Joblet.ValidateCommands)

	// Early context check
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Validate command and arguments
	if err := w.processManager.ValidateCommand(command); err != nil {
		return nil, fmt.Errorf("invalid command: %w", err)
	}

	if err := w.processManager.ValidateArguments(args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	// Resolve command path
	resolvedCommand, err := w.processManager.ResolveCommand(command)
	if err != nil {
		return nil, fmt.Errorf("command resolution failed: %w", err)
	}

	// Create job domain object
	job := w.createJobDomain(jobID, resolvedCommand, args, maxCPU, maxMemory, maxIOBPS)

	log.Debug("creating cgroup for job with resource limits",
		"limits", fmt.Sprintf("CPU:%d, Memory:%dMB, IO:%d",
			job.Limits.MaxCPU, job.Limits.MaxMemory, job.Limits.MaxIOBPS))

	// Setup cgroup resources
	if e := w.cgroup.Create(
		job.CgroupPath,
		job.Limits.MaxCPU,
		job.Limits.MaxMemory,
		job.Limits.MaxIOBPS,
	); e != nil {
		return nil, fmt.Errorf("cgroup setup failed: %w", e)
	}

	// Register job in store
	w.store.CreateNewJob(job)

	// Start the process using single binary approach
	cmd, err := w.startProcessSingleBinary(ctx, job)
	if err != nil {
		w.cleanupFailedJob(job)
		return nil, fmt.Errorf("process start failed: %w", err)
	}

	// Update job with process info
	w.updateJobAsRunning(job, cmd)

	// Start monitoring
	go w.monitorJob(ctx, cmd, job)

	log.Debug("job started successfully", "pid", job.Pid)
	return job, nil
}

func (w *Joblet) StopJob(ctx context.Context, jobID string) error {
	log := w.logger.WithField("jobID", jobID)
	log.Debug("stopping job")

	job, exists := w.store.GetJob(jobID)
	if !exists {
		return fmt.Errorf("job not found: %s", jobID)
	}

	if !job.IsRunning() {
		return fmt.Errorf("job is not running: %s (status: %s)", jobID, job.Status)
	}

	// Create cleanup request
	cleanupReq := &process.CleanupRequest{
		JobID:           jobID,
		PID:             job.Pid,
		CgroupPath:      job.CgroupPath,
		ForceKill:       false,
		GracefulTimeout: w.config.Cgroup.CleanupTimeout,
	}

	// Perform process cleanup
	result, err := w.processManager.CleanupProcess(ctx, cleanupReq)
	if err != nil {
		return fmt.Errorf("process cleanup failed: %w", err)
	}

	// Update job status
	w.updateJobStatus(job, result)

	// Cleanup cgroup
	w.cgroup.CleanupCgroup(jobID)

	log.Debug("job stopped successfully", "method", result.Method)
	return nil
}

// Helper methods (keeping existing implementations)
func (w *Joblet) getNextJobID() string {
	nextID := atomic.AddInt64(&jobCounter, 1)
	return fmt.Sprintf("%d", nextID)
}

func (w *Joblet) createJobDomain(jobID, resolvedCommand string, args []string, maxCPU, maxMemory, maxIOBPS int32) *domain.Job {
	// Apply defaults from configuration
	if maxCPU <= 0 {
		maxCPU = w.config.Joblet.DefaultCPULimit
	}
	if maxMemory <= 0 {
		maxMemory = w.config.Joblet.DefaultMemoryLimit
	}
	if maxIOBPS <= 0 {
		maxIOBPS = w.config.Joblet.DefaultIOLimit
	}

	w.logger.Debug("job resource limits applied",
		"jobID", jobID,
		"maxCPU", maxCPU,
		"maxMemory", maxMemory,
		"maxIOBPS", maxIOBPS,
		"source", "client-specified or defaults")

	return &domain.Job{
		Id:      jobID,
		Command: resolvedCommand,
		Args:    append([]string(nil), args...),
		Limits: domain.ResourceLimits{
			MaxCPU:    maxCPU,
			MaxMemory: maxMemory,
			MaxIOBPS:  maxIOBPS,
		},
		Status:     domain.StatusInitializing,
		CgroupPath: filepath.Join(w.config.Cgroup.BaseDir, "job-"+jobID),
		StartTime:  time.Now(),
	}
}

func (w *Joblet) setupCgroupControllers() error {
	w.logger.Debug("setting up cgroup controllers for job isolation")

	if err := w.cgroup.EnsureControllers(); err != nil {
		return fmt.Errorf("failed to ensure controllers: %w", err)
	}

	w.logger.Debug("cgroup controllers setup completed successfully")
	return nil
}

// startProcessSingleBinary starts a job using the same binary in init mode
func (w *Joblet) startProcessSingleBinary(ctx context.Context, job *domain.Job) (platform.Command, error) {
	// Get the current executable path
	execPath, err := w.platform.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get current executable path: %w", err)
	}

	// Prepare environment with job information and mode indicator
	env := w.processManager.BuildJobEnvironment(job, execPath)

	// Create isolation attributes
	sysProcAttr := w.jobIsolation.CreateIsolatedSysProcAttr()

	// Create launch configuration
	launchConfig := &process.LaunchConfig{
		InitPath:    execPath,
		Environment: env,
		SysProcAttr: sysProcAttr,
		Stdout:      NewWrite(w.store, job.Id),
		Stderr:      NewWrite(w.store, job.Id),
		JobID:       job.Id,
		Command:     job.Command,
		Args:        job.Args,
	}

	// Launch the process
	result, err := w.processManager.LaunchProcess(ctx, launchConfig)
	if err != nil {
		return nil, err
	}

	// Move process to cgroup
	if e := w.addProcessToCgroup(job.CgroupPath, result.PID); e != nil {
		w.logger.Warn("failed to add process to cgroup", "error", e)
	}

	w.logger.Debug("process launched using single binary", "jobID", job.Id, "pid", result.PID)
	return result.Command, nil
}

// addProcessToCgroup moves a process to the specified cgroup
func (w *Joblet) addProcessToCgroup(cgroupPath string, pid int32) error {
	procsFile := filepath.Join(cgroupPath, "cgroup.procs")
	pidBytes := []byte(fmt.Sprintf("%d", pid))
	return w.platform.WriteFile(procsFile, pidBytes, 0644)
}

func (w *Joblet) updateJobAsRunning(job *domain.Job, processCmd platform.Command) {
	cmd := processCmd.Process()
	if cmd == nil {
		w.logger.Warn("process is nil after start", "jobID", job.Id)
		return
	}

	runningJob := job.DeepCopy()
	runningJob.Pid = int32(cmd.Pid())

	if err := runningJob.MarkAsRunning(runningJob.Pid); err != nil {
		w.logger.Warn("domain validation failed for running status", "error", err)
		runningJob.Status = domain.StatusRunning
		runningJob.Pid = int32(cmd.Pid())
	}

	job.Status = domain.StatusRunning
	runningJob.StartTime = time.Now()
	w.store.UpdateJob(runningJob)
}

func (w *Joblet) monitorJob(ctx context.Context, cmd platform.Command, job *domain.Job) {
	log := w.logger.WithField("jobID", job.Id)
	startTime := time.Now()

	// Wait for process completion
	err := cmd.Wait()
	duration := time.Since(startTime)

	// Determine final status and exit code
	var finalStatus domain.JobStatus
	var exitCode int32

	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = int32(exitErr.ExitCode())
			finalStatus = domain.StatusFailed
		}
	} else {
		exitCode = 0
		finalStatus = domain.StatusCompleted
	}

	// Update job status
	completedJob := job.DeepCopy()
	switch finalStatus {
	case domain.StatusCompleted:
		completedJob.Complete(exitCode)
	case domain.StatusFailed:
		completedJob.Fail(exitCode)
	}

	w.store.UpdateJob(completedJob)

	// Cleanup cgroup
	w.cgroup.CleanupCgroup(job.Id)

	log.Debug("job monitoring completed",
		"finalStatus", finalStatus,
		"exitCode", exitCode,
		"duration", duration)
}

func (w *Joblet) cleanupFailedJob(job *domain.Job) {
	failedJob := job.DeepCopy()
	failedJob.Fail(-1)
	w.store.UpdateJob(failedJob)
	w.cgroup.CleanupCgroup(job.Id)
}

func (w *Joblet) updateJobStatus(job *domain.Job, result *process.CleanupResult) {
	stoppedJob := job.DeepCopy()

	switch result.Method {
	case "graceful":
		stoppedJob.Stop()
	case "forced", "force_failed":
		stoppedJob.Fail(-1)
	case "already_dead":
		stoppedJob.Complete(0)
	default:
		stoppedJob.Fail(-1)
	}

	w.store.UpdateJob(stoppedJob)
}
