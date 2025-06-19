//go:build linux

package linux

import (
	"context"
	"errors"
	"fmt"
	"job-worker/internal/worker/jobworker/linux/resource"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"time"

	"job-worker/internal/worker/domain"
	"job-worker/internal/worker/interfaces"
	"job-worker/internal/worker/jobworker/linux/process"
	"job-worker/pkg/logger"
	osinterface "job-worker/pkg/os"
)

var jobCounter int64

// Worker handles job execution with host networking only
type Worker struct {
	store            interfaces.Store
	cgroup           interfaces.Resource
	processLauncher  *process.Launcher
	processCleaner   *process.Cleaner
	processValidator *process.Validator
	osInterface      osinterface.OsInterface
	config           *Config
	logger           *logger.Logger
}

// dependencies contains dependencies for Worker
type dependencies struct {
	Store            interfaces.Store
	Cgroup           interfaces.Resource
	ProcessLauncher  *process.Launcher
	ProcessCleaner   *process.Cleaner
	ProcessValidator *process.Validator
	OsInterface      osinterface.OsInterface
	Config           *Config
}

// NewPlatformWorker creates a simplified Linux worker
func NewPlatformWorker(store interfaces.Store) interfaces.JobWorker {
	// Create OS interfaces
	osInterface := &osinterface.DefaultOs{}
	syscallInterface := &osinterface.DefaultSyscall{}
	cmdFactory := &osinterface.DefaultCommandFactory{}
	execInterface := &osinterface.DefaultExec{}

	// Create process management components
	processValidator := process.NewValidator(osInterface, execInterface)
	processLauncher := process.NewLauncher(cmdFactory, syscallInterface, osInterface, processValidator)
	processCleaner := process.NewCleaner(syscallInterface, osInterface)

	// Create resource management
	cgroupResource := resource.New()

	// Create configuration with cgroup namespace as prerequisite
	config := DefaultConfigWithCgroupNamespace()

	deps := &dependencies{
		Store:            store,
		Cgroup:           cgroupResource,
		ProcessLauncher:  processLauncher,
		ProcessCleaner:   processCleaner,
		ProcessValidator: processValidator,
		OsInterface:      osInterface,
		Config:           config,
	}

	worker := &Worker{
		store:            deps.Store,
		cgroup:           deps.Cgroup,
		processLauncher:  deps.ProcessLauncher,
		processCleaner:   deps.ProcessCleaner,
		processValidator: deps.ProcessValidator,
		osInterface:      deps.OsInterface,
		config:           deps.Config,
		logger:           logger.New().WithField("component", "linux-worker"),
	}

	// Validate cgroup namespace support on startup
	if err := worker.validateCgroupNamespaceSupport(); err != nil {
		logger.Fatal("cgroup namespace support validation failed", "error", err)
	}

	return worker
}

// StartJob starts a job with host networking (no isolation)
func (w *Worker) StartJob(ctx context.Context, command string, args []string, maxCPU, maxMemory, maxIOBPS int32) (*domain.Job, error) {
	jobID := w.getNextJobID()
	log := w.logger.WithFields("jobID", jobID, "command", command)

	log.Info("starting job with host networking")

	// Early context check
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Validate command and arguments
	if err := w.processValidator.ValidateCommand(command); err != nil {
		return nil, fmt.Errorf("invalid command: %w", err)
	}

	if err := w.processValidator.ValidateArguments(args); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	// Resolve command path
	resolvedCommand, err := w.processValidator.ResolveCommand(command)
	if err != nil {
		return nil, fmt.Errorf("command resolution failed: %w", err)
	}

	// Create job domain object
	job := w.createJobDomain(jobID, resolvedCommand, args, maxCPU, maxMemory, maxIOBPS)

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

	// Start the process with host networking
	cmd, err := w.startProcess(ctx, job)
	if err != nil {
		w.cleanupFailedJob(job)
		return nil, fmt.Errorf("process start failed: %w", err)
	}

	// Update job with process info
	w.updateJobAsRunning(job, cmd)

	// Start monitoring
	go w.monitorJob(ctx, cmd, job)

	log.Info("job started successfully with host networking", "pid", job.Pid)
	return job, nil
}

// StopJob stops a running job
func (w *Worker) StopJob(ctx context.Context, jobID string) error {
	log := w.logger.WithField("jobID", jobID)
	log.Info("stopping job")

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
		IsIsolatedJob:   false,
		ForceKill:       false,
		GracefulTimeout: w.config.GracefulShutdownTimeout,
	}

	// Perform process cleanup
	result, err := w.processCleaner.CleanupProcess(ctx, cleanupReq)
	if err != nil {
		return fmt.Errorf("process cleanup failed: %w", err)
	}

	// Update job status
	w.updateJobStatus(job, result)

	// Cleanup cgroup
	w.cgroup.CleanupCgroup(jobID)

	log.Info("job stopped successfully", "method", result.Method)
	return nil
}

// Private helper methods
func (w *Worker) getNextJobID() string {
	nextID := atomic.AddInt64(&jobCounter, 1)
	return fmt.Sprintf("%d", nextID)
}

func (w *Worker) createJobDomain(jobID, resolvedCommand string, args []string, maxCPU, maxMemory, maxIOBPS int32) *domain.Job {
	// Apply defaults
	if maxCPU <= 0 {
		maxCPU = 100 // 1 CPU core
	}
	if maxMemory <= 0 {
		maxMemory = 512 // 512 MB
	}
	if maxIOBPS <= 0 {
		maxIOBPS = 0 // Unlimited
	}

	return &domain.Job{
		Id:      jobID,
		Command: resolvedCommand,
		Args:    append([]string(nil), args...), // Deep copy
		Limits: domain.ResourceLimits{
			MaxCPU:    maxCPU,
			MaxMemory: maxMemory,
			MaxIOBPS:  maxIOBPS,
		},
		Status:     domain.StatusInitializing,
		CgroupPath: w.config.BuildCgroupPath(jobID),
		StartTime:  time.Now(),
		// No network fields
	}
}

func (w *Worker) startProcess(ctx context.Context, job *domain.Job) (osinterface.Command, error) {
	// Get job-init binary path
	initPath, err := w.getJobInitPath()
	if err != nil {
		return nil, fmt.Errorf("job-init not found: %w", err)
	}

	// Prepare environment (no network environment)
	env := w.buildJobEnvironment(job)

	// Create process attributes with NO network namespace isolation
	sysProcAttr := &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWPID | // PID namespace isolation
			syscall.CLONE_NEWNS | // Mount namespace isolation
			syscall.CLONE_NEWIPC | // IPC namespace isolation
			syscall.CLONE_NEWUTS, // UTS namespace isolation
		Setpgid: true,
	}

	// Create launch configuration
	launchConfig := &process.LaunchConfig{
		InitPath:      initPath,
		Environment:   env,
		SysProcAttr:   sysProcAttr,
		Stdout:        New(w.store, job.Id),
		Stderr:        New(w.store, job.Id),
		NamespacePath: "",    // No namespace path needed
		NeedsNSJoin:   false, // No namespace joining
		JobID:         job.Id,
		Command:       job.Command,
		Args:          job.Args,
	}

	// Launch the process
	result, err := w.processLauncher.LaunchProcess(ctx, launchConfig)
	if err != nil {
		return nil, err
	}

	w.logger.Info("process launched with host networking", "jobID", job.Id, "pid", result.PID)
	return result.Command, nil
}

func (w *Worker) buildJobEnvironment(job *domain.Job) []string {
	baseEnv := w.osInterface.Environ()

	// In cgroup namespace, the job's cgroup always appears at root
	namespaceCgroupPath := "/sys/fs/cgroup"

	// Job-specific environment
	jobEnv := []string{
		fmt.Sprintf("JOB_ID=%s", job.Id),
		fmt.Sprintf("JOB_COMMAND=%s", job.Command),
		fmt.Sprintf("JOB_CGROUP_PATH=%s", namespaceCgroupPath), // Namespace path
		fmt.Sprintf("JOB_CGROUP_HOST_PATH=%s", job.CgroupPath), // Host path for debugging
		fmt.Sprintf("JOB_ARGS_COUNT=%d", len(job.Args)),
		"HOST_NETWORKING=true",
		"CGROUP_NAMESPACE=true", // cgroup namespace is active
	}

	// Add job arguments
	for i, arg := range job.Args {
		jobEnv = append(jobEnv, fmt.Sprintf("JOB_ARG_%d=%s", i, arg))
	}

	return append(baseEnv, jobEnv...)
}

// Validate cgroup namespace support at startup
func (w *Worker) validateCgroupNamespaceSupport() error {
	// Check kernel support
	if _, err := w.osInterface.Stat("/proc/self/ns/cgroup"); err != nil {
		return fmt.Errorf("cgroup namespaces not supported by kernel: %w", err)
	}

	// Check cgroup v2 support
	if _, err := w.osInterface.Stat("/sys/fs/cgroup/cgroup.controllers"); err != nil {
		return fmt.Errorf("cgroups v2 not available: %w", err)
	}

	w.logger.Info("cgroup namespace support validated")
	return nil
}

func (w *Worker) getJobInitPath() (string, error) {
	// Check same directory as main executable
	if execPath, err := w.osInterface.Executable(); err == nil {
		initPath := filepath.Join(filepath.Dir(execPath), "job-init")
		if _, err := w.osInterface.Stat(initPath); err == nil {
			return initPath, nil
		}
	}

	// Check standard paths
	standardPaths := []string{
		"/opt/job-worker/job-init",
		"/usr/local/bin/job-init",
		"/usr/bin/job-init",
	}

	for _, path := range standardPaths {
		if _, err := w.osInterface.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("job-init binary not found")
}

func (w *Worker) updateJobAsRunning(job *domain.Job, processCmd osinterface.Command) {
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

	//update the reference
	job.Status = domain.StatusRunning

	runningJob.StartTime = time.Now()
	w.store.UpdateJob(runningJob)
}

func (w *Worker) monitorJob(ctx context.Context, cmd osinterface.Command, job *domain.Job) {
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

	log.Info("job monitoring completed",
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
