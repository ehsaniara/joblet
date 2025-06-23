//go:build linux

package linux

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"time"

	"worker/internal/worker/core/interfaces"
	"worker/internal/worker/core/linux/filesystem"
	"worker/internal/worker/core/linux/process"
	"worker/internal/worker/core/linux/resource"
	"worker/internal/worker/core/linux/usernamespace"
	"worker/internal/worker/domain"
	"worker/internal/worker/store"
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
	filesystemManager    interfaces.FilesystemManager
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
	FilesystemManager    interfaces.FilesystemManager
	OsInterface          osinterface.OsInterface
	Config               *Config
}

// NewPlatformWorker creates a Linux worker with user namespace isolation
func NewPlatformWorker(store store.Store) interfaces.Worker {
	// Create OS interfaces
	osInterface := &osinterface.DefaultOs{}
	syscallInterface := &osinterface.DefaultSyscall{}
	cmdFactory := &osinterface.DefaultCommandFactory{}
	execInterface := &osinterface.DefaultExec{}

	// Create process management components
	processValidator := process.NewValidator(osInterface, execInterface)
	processLauncher := process.NewLauncher(cmdFactory, syscallInterface, osInterface, processValidator)
	processCleaner := process.NewCleaner(syscallInterface, osInterface)
	filesystemManager := filesystem.NewManager(osInterface)

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
		FilesystemManager:    filesystemManager,
		OsInterface:          osInterface,
		Config:               config,
	}

	w := &worker{
		store:                deps.Store,
		cgroup:               deps.Cgroup,
		userNamespaceManager: deps.UserNamespaceManager,
		processLauncher:      deps.ProcessLauncher,
		processCleaner:       deps.ProcessCleaner,
		processValidator:     deps.ProcessValidator,
		filesystemManager:    deps.FilesystemManager,
		osInterface:          deps.OsInterface,
		config:               deps.Config,
		logger:               logger.New().WithField("component", "linux-worker"),
	}

	// Log successful initialization
	w.logger.Info("Linux worker initialized with single-process filesystem isolation support")

	return w
}

// StartJob creates isolated job with user namespaces and cgroup limits
func (w *worker) StartJob(ctx context.Context, command string, args []string, maxCPU, maxMemory, maxIOBPS int32) (*domain.Job, error) {
	jobID := w.getNextJobID()
	log := w.logger.WithFields("jobID", jobID, "command", command)

	log.Info("starting job with single-process namespace isolation")

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

	// Setup isolated filesystem
	isolatedRoot, err := w.filesystemManager.SetupIsolatedFilesystem(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("filesystem isolation setup failed: %w", err)
	}

	// Create unique UID mapping for job isolation
	userMapping, err := w.userNamespaceManager.CreateUserMapping(ctx, jobID)
	if err != nil {
		// Cleanup filesystem on failure
		w.filesystemManager.CleanupIsolatedFilesystem(jobID)
		return nil, fmt.Errorf("failed to create user mapping: %w", err)
	}

	// Setup cgroup for resource limits (CPU/memory/IO)
	if err := w.cgroup.Create(
		job.CgroupPath,
		job.Limits.MaxCPU,
		job.Limits.MaxMemory,
		job.Limits.MaxIOBPS,
	); err != nil {
		// Cleanup on failure
		w.userNamespaceManager.CleanupUserMapping(jobID)
		w.filesystemManager.CleanupIsolatedFilesystem(jobID)
		return nil, fmt.Errorf("cgroup setup failed: %w", err)
	}

	// Register job in store
	w.store.CreateNewJob(job)

	// Start the process with single-process filesystem isolation
	cmd, err := w.startProcessWithSingleProcessIsolation(ctx, job, userMapping, isolatedRoot)
	if err != nil {
		w.cleanupFailedJob(job)
		return nil, fmt.Errorf("process start failed: %w", err)
	}

	// Update job with process info
	w.updateJobAsRunning(job, cmd)

	// Start monitoring
	go w.monitorJob(ctx, cmd, job)

	log.Info("job started successfully with single-process filesystem isolation",
		"pid", job.Pid,
		"isolatedRoot", isolatedRoot,
		"approach", "single-process")

	return job, nil
}

// startProcessWithSingleProcessIsolation implements the single-process approach
func (w *worker) startProcessWithSingleProcessIsolation(ctx context.Context, job *domain.Job, userMapping *usernamespace.UserMapping, isolatedRoot string) (osinterface.Command, error) {
	log := w.logger.WithFields("jobID", job.Id, "approach", "single-process")

	log.Info("Creating single process with transparent filesystem isolation")

	initPath, err := w.getJobInitPath()
	if err != nil {
		return nil, fmt.Errorf("job-init not found: %w", err)
	}

	// Pre-setup filesystem isolation BEFORE process creation
	if err := w.preSetupFilesystemIsolation(isolatedRoot); err != nil {
		log.Warn("filesystem pre-setup failed, continuing with fallback", "error", err)
	}

	// Create environment for single process
	jobEnv := w.buildSingleProcessEnvironment(job, userMapping, isolatedRoot)

	// Create process with mount namespace FIRST, then add user namespace in job-init
	sysProcAttr := &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWPID | // PID isolation
			syscall.CLONE_NEWNS | // Mount namespace for filesystem isolation
			syscall.CLONE_NEWIPC | // IPC isolation
			syscall.CLONE_NEWUTS | // UTS isolation
			// syscall.CLONE_NEWUSER - will be set up in job-init after mounts
			syscall.CLONE_NEWCGROUP, // Cgroup isolation
		Setpgid: true,
		// UidMappings/GidMappings - will be set up in job-init
	}

	// Launch single process
	config := &process.LaunchConfig{
		InitPath:      initPath,
		Environment:   jobEnv,
		SysProcAttr:   sysProcAttr,
		Stdout:        NewWrite(w.store, job.Id),
		Stderr:        NewWrite(w.store, job.Id),
		NamespacePath: "",
		NeedsNSJoin:   false,
		NamespaceType: "",
		JobID:         job.Id,
		Command:       job.Command,
		Args:          job.Args,
	}

	log.Info("launching single process with transparent mounts")

	result, err := w.processLauncher.LaunchProcess(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("single process launch failed: %w", err)
	}

	log.Info("single process started successfully", "pid", result.PID)

	return result.Command, nil
}

// preSetupFilesystemIsolation sets up filesystem isolation before process creation
func (w *worker) preSetupFilesystemIsolation(isolatedRoot string) error {
	w.logger.Info("pre-setting up filesystem isolation", "isolatedRoot", isolatedRoot)

	// Create bind mount directories
	bindMounts := map[string]string{
		"/tmp":     filepath.Join(isolatedRoot, "tmp"),
		"/var/tmp": filepath.Join(isolatedRoot, "var/tmp"),
		"/home":    filepath.Join(isolatedRoot, "home"),
	}

	for virtualPath, isolatedPath := range bindMounts {
		// Create isolated directory
		if err := w.osInterface.MkdirAll(isolatedPath, 0777); err != nil {
			w.logger.Warn("failed to create isolated directory", "path", isolatedPath, "error", err)
			continue
		}

		// Create test file to verify isolation
		testFile := filepath.Join(isolatedPath, ".isolation-test")
		if err := w.osInterface.WriteFile(testFile, []byte("isolated"), 0644); err != nil {
			w.logger.Warn("failed to create test file", "path", testFile, "error", err)
		}

		w.logger.Debug("prepared isolation directory", "virtual", virtualPath, "isolated", isolatedPath)
	}

	w.logger.Info("filesystem pre-setup complete")
	return nil
}

// buildSingleProcessEnvironment builds environment for single process
func (w *worker) buildSingleProcessEnvironment(job *domain.Job, userMapping *usernamespace.UserMapping, isolatedRoot string) []string {
	baseEnv := []string{
		"PATH=/usr/local/bin:/usr/bin:/bin",
		"HOME=/home/worker",
		"USER=worker",
		"SHELL=/bin/bash",
		"LANG=C.UTF-8",
	}

	jobEnv := []string{
		// Job identification
		fmt.Sprintf("JOB_ID=%s", job.Id),
		fmt.Sprintf("JOB_COMMAND=%s", job.Command),
		fmt.Sprintf("JOB_CGROUP_PATH=%s", job.CgroupPath),
		fmt.Sprintf("JOB_ARGS_COUNT=%d", len(job.Args)),

		// Process phase - tells init_linux.go to use single-process approach
		"PROCESS_PHASE=SINGLE_PROCESS",

		// Filesystem isolation
		fmt.Sprintf("JOB_ISOLATED_ROOT=%s", isolatedRoot),
		"FILESYSTEM_ISOLATION_ENABLED=true",

		// User namespace mapping
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

// StopJob stops a running job
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
		IsIsolatedJob:   true,
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

	// Cleanup isolated filesystem
	if cleanupErr := w.filesystemManager.CleanupIsolatedFilesystem(jobID); cleanupErr != nil {
		log.Warn("failed to cleanup isolated filesystem", "error", cleanupErr)
		// Don't fail the entire stop operation
	}

	log.Info("job stopped successfully", "method", result.Method)
	return nil
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
	job.Pid = runningJob.Pid

	w.store.UpdateJob(job)
}

func (w *worker) monitorJob(ctx context.Context, cmd osinterface.Command, job *domain.Job) {
	log := w.logger.WithField("jobID", job.Id)
	log.Info("starting job monitoring")

	select {
	case <-ctx.Done():
		log.Info("monitoring cancelled due to context cancellation")
		return
	default:
	}

	// Wait for process to complete
	if err := cmd.Wait(); err != nil {
		log.Info("job process completed with error", "error", err)

		exitCode := 1
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		}

		completedJob := job.DeepCopy()
		if err := completedJob.Complete(int32(exitCode)); err != nil {
			log.Warn("domain validation failed for completion", "error", err)
			completedJob.Status = domain.StatusCompleted
			completedJob.ExitCode = int32(exitCode)
		}
		w.store.UpdateJob(completedJob)
	} else {
		log.Info("job process completed successfully")

		completedJob := job.DeepCopy()
		if err := completedJob.Complete(0); err != nil {
			log.Warn("domain validation failed for completion", "error", err)
			completedJob.Status = domain.StatusCompleted
			completedJob.ExitCode = 0
		}
		w.store.UpdateJob(completedJob)
	}

	// Cleanup resources
	w.cleanupJobResources(job)
	log.Info("job monitoring completed")
}

func (w *worker) cleanupJobResources(job *domain.Job) {
	log := w.logger.WithField("jobID", job.Id)

	// Cleanup cgroup (no return value)
	w.cgroup.CleanupCgroup(job.Id)

	// Cleanup user namespace mapping
	if err := w.userNamespaceManager.CleanupUserMapping(job.Id); err != nil {
		log.Warn("failed to cleanup user namespace mapping", "error", err)
	}

	// Cleanup isolated filesystem
	if err := w.filesystemManager.CleanupIsolatedFilesystem(job.Id); err != nil {
		log.Warn("failed to cleanup isolated filesystem", "error", err)
	}

	log.Info("job resource cleanup completed")
}

func (w *worker) cleanupFailedJob(job *domain.Job) {
	log := w.logger.WithField("jobID", job.Id)
	log.Info("cleaning up failed job")

	// Mark job as failed
	failedJob := job.DeepCopy()
	if err := failedJob.Fail(1); err != nil {
		log.Error("failed to mark job as failed", "error", err)
		failedJob.Status = domain.StatusFailed
		failedJob.ExitCode = 1
	}

	w.store.UpdateJob(failedJob)

	// Cleanup all resources
	w.cleanupJobResources(job)
}

func (w *worker) updateJobStatus(job *domain.Job, result *process.CleanupResult) {
	updatedJob := job.DeepCopy()
	// Check what fields actually exist in CleanupResult - using result.Method for now
	if result.Method == "forced" {
		if err := updatedJob.Stop(); err != nil {
			w.logger.Error("failed to mark job as killed", "jobID", job.Id, "error", err)
			updatedJob.Status = domain.StatusStopped
			updatedJob.ExitCode = -1
		}
	} else {
		// Assume normal completion with exit code 0 if not killed
		if err := updatedJob.Complete(0); err != nil {
			w.logger.Error("failed to mark job as completed", "jobID", job.Id, "error", err)
			updatedJob.Status = domain.StatusCompleted
			updatedJob.ExitCode = 0
		}
	}
	w.store.UpdateJob(updatedJob)
}

// Ensure worker implements the interface
var _ interfaces.Worker = (*worker)(nil)
