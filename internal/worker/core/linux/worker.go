//go:build linux

package linux

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"time"
	interfaces2 "worker/internal/worker/core/interfaces"
	"worker/internal/worker/core/linux/resource"
	"worker/internal/worker/core/linux/usernamespace"
	"worker/internal/worker/store"

	"worker/internal/worker/core/linux/process"
	"worker/internal/worker/domain"
	"worker/pkg/logger"
	osinterface "worker/pkg/os"
)

var jobCounter int64

// worker handles job execution with user namespace isolation
type worker struct {
	store                store.Store
	cgroup               resource.Resource
	userNamespaceManager usernamespace.UserNamespaceManager
	processLauncher      *process.Launcher
	processCleaner       *process.Cleaner
	processValidator     *process.Validator
	osInterface          osinterface.OsInterface
	config               *Config
	logger               *logger.Logger
}

// dependencies contains dependencies for worker
type dependencies struct {
	Store                store.Store
	Cgroup               resource.Resource
	UserNamespaceManager usernamespace.UserNamespaceManager
	ProcessLauncher      *process.Launcher
	ProcessCleaner       *process.Cleaner
	ProcessValidator     *process.Validator
	OsInterface          osinterface.OsInterface
	Config               *Config
}

// NewPlatformWorker creates a Linux worker with user namespace isolation
// Note: All platform requirements should be validated in main.go before calling this
func NewPlatformWorker(store store.Store) interfaces2.Worker {
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

	// Create user namespace manager
	userNSConfig := usernamespace.DefaultUserNamespaceConfig()
	userNSManager := usernamespace.NewUserNamespaceManager(userNSConfig, osInterface)

	// Create configuration
	config := DefaultConfigWithUserNamespaces()

	deps := &dependencies{
		Store:                store,
		Cgroup:               cgroupResource,
		UserNamespaceManager: userNSManager,
		ProcessLauncher:      processLauncher,
		ProcessCleaner:       processCleaner,
		ProcessValidator:     processValidator,
		OsInterface:          osInterface,
		Config:               config,
	}

	worker := &worker{
		store:                deps.Store,
		cgroup:               deps.Cgroup,
		userNamespaceManager: deps.UserNamespaceManager,
		processLauncher:      deps.ProcessLauncher,
		processCleaner:       deps.ProcessCleaner,
		processValidator:     deps.ProcessValidator,
		osInterface:          deps.OsInterface,
		config:               deps.Config,
		logger:               logger.New().WithField("component", "linux-worker"),
	}

	// Log successful initialization (validation was done in main)
	worker.logger.Info("Linux worker initialized with user namespace support",
		"userNamespacesEnabled", config.UserNamespaceEnabled,
		"cgroupNamespacesEnabled", true,
		"baseUID", config.UserNamespaceConfig.BaseUID,
		"maxJobs", config.UserNamespaceConfig.MaxJobs)

	return worker
}

// StartJob creates isolated job with user namespaces and cgroup limits
func (w *worker) StartJob(ctx context.Context, command string, args []string, maxCPU, maxMemory, maxIOBPS int32) (*domain.Job, error) {
	jobID := w.getNextJobID()
	log := w.logger.WithFields("jobID", jobID, "command", command)

	log.Info("starting job with user namespace isolation")

	// Early context check
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Validate input parameters before resource allocation
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

	// Create unique UID mapping for job isolation
	userMapping, err := w.userNamespaceManager.CreateUserMapping(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("failed to create user mapping: %w", err)
	}

	// Setup cgroup for resource limits (CPU/memory/IO)
	if e := w.cgroup.Create(
		job.CgroupPath,
		job.Limits.MaxCPU,
		job.Limits.MaxMemory,
		job.Limits.MaxIOBPS,
	); e != nil {
		// Cleanup on failure to prevent resource leaks
		w.userNamespaceManager.CleanupUserMapping(jobID)
		return nil, fmt.Errorf("cgroup setup failed: %w", e)
	}

	// Register job in store
	w.store.CreateNewJob(job)

	// Start the process with user namespace isolation
	cmd, err := w.startProcessWithUserNamespace(ctx, job, userMapping)
	if err != nil {
		w.cleanupFailedJob(job)
		return nil, fmt.Errorf("process start failed: %w", err)
	}

	// Update job with process info
	w.updateJobAsRunning(job, cmd)

	// Start monitoring
	go w.monitorJob(ctx, cmd, job)

	log.Info("job started successfully with user namespace isolation",
		"pid", job.Pid,
		"hostUID", userMapping.HostUID,
		"namespaceUID", userMapping.NamespaceUID)
	return job, nil
}

// StopJob stops a running job (updated to cleanup user namespaces)
func (w *worker) StopJob(ctx context.Context, jobID string) error {
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
		IsIsolatedJob:   true, // User namespace isolation
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

	// Cleanup user namespace mapping
	if cleanupErr := w.userNamespaceManager.CleanupUserMapping(jobID); cleanupErr != nil {
		log.Warn("failed to cleanup user namespace mapping", "error", cleanupErr)
	}

	log.Info("job stopped successfully", "method", result.Method)
	return nil
}

// startProcessWithUserNamespace starts a process with user namespace isolation
func (w *worker) startProcessWithUserNamespace(ctx context.Context, job *domain.Job, userMapping *usernamespace.UserMapping) (osinterface.Command, error) {
	// Get job-init binary path
	initPath, err := w.getJobInitPath()
	if err != nil {
		return nil, fmt.Errorf("job-init not found: %w", err)
	}

	// Prepare environment with user namespace info
	env := w.buildJobEnvironmentWithUserNS(job, userMapping)

	// Create process attributes WITH user namespace isolation
	sysProcAttr := w.createUserNamespaceSysProcAttr()

	// Configure user namespace mappings
	sysProcAttr = w.userNamespaceManager.ConfigureSysProcAttr(sysProcAttr, userMapping)

	// Create launch configuration
	launchConfig := &process.LaunchConfig{
		InitPath:      initPath,
		Environment:   env,
		SysProcAttr:   sysProcAttr,
		Stdout:        NewWrite(w.store, job.Id),
		Stderr:        NewWrite(w.store, job.Id),
		NamespacePath: "",    // No separate network namespace needed
		NeedsNSJoin:   false, // User namespace is created, not joined
		JobID:         job.Id,
		Command:       job.Command,
		Args:          job.Args,
	}

	// Launch the process
	result, err := w.processLauncher.LaunchProcess(ctx, launchConfig)
	if err != nil {
		return nil, err
	}

	w.logger.Info("process launched with user namespace isolation",
		"jobID", job.Id,
		"pid", result.PID,
		"hostUID", userMapping.HostUID,
		"namespaceUID", userMapping.NamespaceUID)
	return result.Command, nil
}

// buildJobEnvironmentWithUserNS builds environment with user namespace info
func (w *worker) buildJobEnvironmentWithUserNS(job *domain.Job, userMapping *usernamespace.UserMapping) []string {
	baseEnv := w.osInterface.Environ()

	// In cgroup namespace, the job's cgroup always appears at root
	namespaceCgroupPath := "/sys/fs/cgroup"

	// Job-specific environment with user namespace info
	jobEnv := []string{
		fmt.Sprintf("JOB_ID=%s", job.Id),
		fmt.Sprintf("JOB_COMMAND=%s", job.Command),
		fmt.Sprintf("JOB_CGROUP_PATH=%s", namespaceCgroupPath),
		fmt.Sprintf("JOB_CGROUP_HOST_PATH=%s", job.CgroupPath),
		fmt.Sprintf("JOB_ARGS_COUNT=%d", len(job.Args)),
		"HOST_NETWORKING=true",
		"CGROUP_NAMESPACE=true",
		"USER_NAMESPACE=true",
		fmt.Sprintf("USER_NAMESPACE_UID=%d", userMapping.NamespaceUID),
		fmt.Sprintf("USER_NAMESPACE_GID=%d", userMapping.NamespaceGID),
		fmt.Sprintf("USER_HOST_UID=%d", userMapping.HostUID),
		fmt.Sprintf("USER_HOST_GID=%d", userMapping.HostGID),
	}

	// Add job arguments
	for i, arg := range job.Args {
		jobEnv = append(jobEnv, fmt.Sprintf("JOB_ARG_%d=%s", i, arg))
	}

	return append(baseEnv, jobEnv...)
}

// createUserNamespaceSysProcAttr creates syscall attributes with user namespaces
func (w *worker) createUserNamespaceSysProcAttr() *syscall.SysProcAttr {
	sysProcAttr := &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWPID | // PID namespace isolation
			syscall.CLONE_NEWNS | // Mount namespace isolation
			syscall.CLONE_NEWIPC | // IPC namespace isolation
			syscall.CLONE_NEWUTS | // UTS namespace isolation
			syscall.CLONE_NEWCGROUP | // Cgroup namespace isolation (mandatory)
			syscall.CLONE_NEWUSER, // User namespace isolation
		Setpgid: true,
	}

	w.logger.Debug("created user namespace process attributes",
		"flags", fmt.Sprintf("0x%x", sysProcAttr.Cloneflags),
		"userNamespace", true)

	return sysProcAttr
}

// Helper methods
func (w *worker) getNextJobID() string {
	nextID := atomic.AddInt64(&jobCounter, 1)
	return fmt.Sprintf("%d", nextID)
}

func (w *worker) createJobDomain(jobID, resolvedCommand string, args []string, maxCPU, maxMemory, maxIOBPS int32) *domain.Job {
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
	}
}

func (w *worker) getJobInitPath() (string, error) {
	// Check same directory as main executable
	if execPath, err := w.osInterface.Executable(); err == nil {
		initPath := filepath.Join(filepath.Dir(execPath), "job-init")
		if _, err := w.osInterface.Stat(initPath); err == nil {
			return initPath, nil
		}
	}

	// Check standard paths
	standardPaths := []string{
		"/opt/worker/job-init",
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

func (w *worker) updateJobAsRunning(job *domain.Job, processCmd osinterface.Command) {
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

func (w *worker) monitorJob(ctx context.Context, cmd osinterface.Command, job *domain.Job) {
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

	// Cleanup user namespace mapping
	if cleanupErr := w.userNamespaceManager.CleanupUserMapping(job.Id); cleanupErr != nil {
		log.Warn("failed to cleanup user namespace mapping during monitoring", "error", cleanupErr)
	}

	log.Info("job monitoring completed",
		"finalStatus", finalStatus,
		"exitCode", exitCode,
		"duration", duration)
}

func (w *worker) cleanupFailedJob(job *domain.Job) {
	failedJob := job.DeepCopy()
	if err := failedJob.Fail(-1); err != nil {
		failedJob.Status = domain.StatusFailed
		failedJob.ExitCode = -1
		now := time.Now()
		failedJob.EndTime = &now
	}
	w.store.UpdateJob(failedJob)
	w.cgroup.CleanupCgroup(job.Id)

	// Cleanup user namespace mapping for failed job
	if cleanupErr := w.userNamespaceManager.CleanupUserMapping(job.Id); cleanupErr != nil {
		w.logger.Warn("failed to cleanup user namespace mapping for failed job", "jobID", job.Id, "error", cleanupErr)
	}
}

func (w *worker) updateJobStatus(job *domain.Job, result *process.CleanupResult) {
	stoppedJob := job.DeepCopy()

	switch result.Method {
	case "graceful":
		// Process shut down gracefully with SIGTERM
		stoppedJob.Stop()
	case "forced":
		// Process was force killed with SIGKILL
		stoppedJob.Stop()
	case "already_dead":
		// Process was already dead - mark as completed
		stoppedJob.Complete(0)
	case "graceful_failed", "graceful_timeout", "force_failed":
		// These represent failures in the stop process itself
		stoppedJob.Fail(-1)
	default:
		// Unknown cleanup method - treat as failure
		w.logger.Warn("unknown cleanup method", "method", result.Method)
		stoppedJob.Fail(-1)
	}

	w.store.UpdateJob(stoppedJob)
}
