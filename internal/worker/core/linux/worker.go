//go:build linux

package linux

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"time"
	"worker/internal/worker/core/interfaces"
	"worker/internal/worker/core/linux/process"
	"worker/internal/worker/core/linux/resource"
	"worker/internal/worker/core/linux/unprivileged"
	"worker/internal/worker/domain"
	"worker/internal/worker/state"
	"worker/pkg/config"
	"worker/pkg/logger"
	"worker/pkg/platform"
)

var jobCounter int64

// Worker handles job execution with configuration
type Worker struct {
	store          state.Store
	cgroup         resource.Resource
	processManager *process.Manager
	jobIsolation   *unprivileged.JobIsolation
	platform       platform.Platform
	config         *Config        // Linux-specific config
	globalConfig   *config.Config // Global application config
	logger         *logger.Logger
}

// NewPlatformWorker creates a new Linux platform worker
func NewPlatformWorker(store state.Store, cfg *config.Config) interfaces.Worker {
	platformInterface := platform.NewPlatform()
	processManager := process.NewProcessManager(platformInterface)
	cgroupResource := resource.New(cfg.Cgroup)
	jobIsolation := unprivileged.NewJobIsolation()

	// Convert global config to Linux-specific config
	linuxConfig := &Config{
		CgroupsBaseDir:          cfg.Cgroup.BaseDir,
		GracefulShutdownTimeout: cfg.Worker.CleanupTimeout,
		DefaultCPULimitPercent:  cfg.Worker.DefaultCPULimit,
		DefaultMemoryLimitMB:    cfg.Worker.DefaultMemoryLimit,
		DefaultIOBPS:            cfg.Worker.DefaultIOLimit,
		CgroupNamespaceMount:    cfg.Cgroup.NamespaceMount,
	}

	worker := &Worker{
		store:          store,
		cgroup:         cgroupResource,
		processManager: processManager,
		jobIsolation:   jobIsolation,
		platform:       platformInterface,
		config:         linuxConfig,
		globalConfig:   cfg,
		logger:         logger.New().WithField("component", "linux-worker"),
	}

	if err := worker.setupCgroupControllers(); err != nil {
		worker.logger.Fatal("cgroup controller setup failed", "error", err)
	}

	worker.logger.Debug("Linux worker initialized",
		"maxConcurrentJobs", cfg.Worker.MaxConcurrentJobs,
		"defaultCPU", cfg.Worker.DefaultCPULimit,
		"defaultMemory", cfg.Worker.DefaultMemoryLimit,
		"cgroupPath", cfg.Cgroup.BaseDir)

	return worker
}

func (w *Worker) StartJob(ctx context.Context, command string, args []string, maxCPU, maxMemory, maxIOBPS int32) (*domain.Job, error) {
	jobID := w.getNextJobID()
	log := w.logger.WithFields("jobID", jobID, "command", command)

	log.Debug("starting job with configuration",
		"requestedCPU", maxCPU,
		"requestedMemory", maxMemory,
		"requestedIO", maxIOBPS,
		"validateCommands", w.globalConfig.Worker.ValidateCommands)

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

func (w *Worker) StopJob(ctx context.Context, jobID string) error {
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
		GracefulTimeout: w.config.GracefulShutdownTimeout,
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
func (w *Worker) getNextJobID() string {
	nextID := atomic.AddInt64(&jobCounter, 1)
	return fmt.Sprintf("%d", nextID)
}

func (w *Worker) createJobDomain(jobID, resolvedCommand string, args []string, maxCPU, maxMemory, maxIOBPS int32) *domain.Job {
	// Apply defaults from configuration
	if maxCPU <= 0 {
		maxCPU = w.globalConfig.Worker.DefaultCPULimit
	}
	if maxMemory <= 0 {
		maxMemory = w.globalConfig.Worker.DefaultMemoryLimit
	}
	if maxIOBPS <= 0 {
		maxIOBPS = w.globalConfig.Worker.DefaultIOLimit
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
		CgroupPath: w.config.BuildCgroupPath(jobID),
		StartTime:  time.Now(),
	}
}

func (w *Worker) setupCgroupControllers() error {
	w.logger.Debug("setting up cgroup controllers for job isolation")

	if err := w.cgroup.EnsureControllers(); err != nil {
		return fmt.Errorf("failed to ensure controllers: %w", err)
	}

	w.logger.Debug("cgroup controllers setup completed successfully")
	return nil
}

// startProcessSingleBinary starts a job using the same binary in init mode
func (w *Worker) startProcessSingleBinary(ctx context.Context, job *domain.Job) (platform.Command, error) {
	// Get the current executable path (this same binary)
	execPath, err := w.platform.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get current executable path: %w", err)
	}

	// Prepare environment with job information and mode indicator
	env := w.buildJobEnvironmentSingleBinary(job, execPath)

	// Create isolation attributes
	sysProcAttr := w.jobIsolation.CreateIsolatedSysProcAttr()

	// Create launch configuration
	launchConfig := &process.LaunchConfig{
		InitPath:    execPath, // Use same binary
		Environment: env,
		SysProcAttr: sysProcAttr,
		Stdout:      New(w.store, job.Id),
		Stderr:      New(w.store, job.Id),
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

// buildJobEnvironmentSingleBinary builds environment for single binary mode
func (w *Worker) buildJobEnvironmentSingleBinary(job *domain.Job, execPath string) []string {
	baseEnv := w.platform.Environ()

	// Job-specific environment with mode indicator
	jobEnv := []string{
		"WORKER_MODE=init", // This tells the binary to run in init mode
		fmt.Sprintf("JOB_ID=%s", job.Id),
		fmt.Sprintf("JOB_COMMAND=%s", job.Command),
		fmt.Sprintf("JOB_CGROUP_PATH=%s", "/sys/fs/cgroup"),    // Namespace path
		fmt.Sprintf("JOB_CGROUP_HOST_PATH=%s", job.CgroupPath), // Host path for debugging
		fmt.Sprintf("JOB_ARGS_COUNT=%d", len(job.Args)),
		"HOST_NETWORKING=true",
		"CGROUP_NAMESPACE=true",
		"JOB_ISOLATION=enabled",
		"USER_NAMESPACE=true",
		fmt.Sprintf("WORKER_BINARY_PATH=%s", execPath), // For reference
		fmt.Sprintf("JOB_MAX_CPU=%d", job.Limits.MaxCPU),
		fmt.Sprintf("JOB_MAX_MEMORY=%d", job.Limits.MaxMemory),
		fmt.Sprintf("JOB_MAX_IOBPS=%d", job.Limits.MaxIOBPS),
	}

	// Add job arguments
	for i, arg := range job.Args {
		jobEnv = append(jobEnv, fmt.Sprintf("JOB_ARG_%d=%s", i, arg))
	}

	return append(baseEnv, jobEnv...)
}

// addProcessToCgroup moves a process to the specified cgroup
func (w *Worker) addProcessToCgroup(cgroupPath string, pid int32) error {
	procsFile := filepath.Join(cgroupPath, "cgroup.procs")
	pidBytes := []byte(fmt.Sprintf("%d", pid))
	return w.platform.WriteFile(procsFile, pidBytes, 0644)
}

func (w *Worker) updateJobAsRunning(job *domain.Job, processCmd platform.Command) {
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

func (w *Worker) monitorJob(ctx context.Context, cmd platform.Command, job *domain.Job) {
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

func (w *Worker) cleanupFailedJob(job *domain.Job) {
	failedJob := job.DeepCopy()
	if err := failedJob.Fail(-1); err != nil {
		failedJob.Status = domain.StatusFailed
		failedJob.ExitCode = -1
		now := time.Now()
		failedJob.EndTime = &now
	}
	w.store.UpdateJob(failedJob)
	w.cgroup.CleanupCgroup(job.Id)
}

func (w *Worker) updateJobStatus(job *domain.Job, result *process.CleanupResult) {
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
