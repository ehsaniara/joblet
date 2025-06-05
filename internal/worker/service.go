package worker

import (
	"context"
	"errors"
	"fmt"
	"job-worker/internal/config"
	"job-worker/internal/worker/domain"
	"job-worker/internal/worker/executor"
	"job-worker/internal/worker/interfaces"
	"job-worker/internal/worker/resource"
	"job-worker/pkg/logger"
	"job-worker/pkg/os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"
)

var jobCounter int64

type worker struct {
	store      interfaces.Store
	cgroup     interfaces.Resource
	cmdFactory os.CommandFactory
	syscall    os.SyscallInterface
	os         os.OsInterface
	logger     *logger.Logger
}

func New(store interfaces.Store) interfaces.JobWorker {
	return &worker{
		store:      store,
		cgroup:     resource.New(),
		cmdFactory: &os.DefaultCommandFactory{},
		syscall:    &os.DefaultSyscall{},
		os:         &os.DefaultOs{},
		logger:     logger.WithField("component", "worker"),
	}
}

func (w *worker) StartJob(ctx context.Context, command string, args []string, maxCPU, maxMemory, maxIOBPS int32) (*domain.Job, error) {

	select {
	case <-ctx.Done():
		w.logger.Warn("start job cancelled by context", "error", ctx.Err())
		return nil, ctx.Err()
	default:
	}

	jobId := strconv.FormatInt(atomic.AddInt64(&jobCounter, 1), 10)

	jobLogger := w.logger.WithField("jobId", jobId)

	jobLogger.Info("starting new job", "command", command, "args", args, "requestedCPU", maxCPU, "requestedMemory", maxMemory, "requestedIOBPS", maxIOBPS)

	originalCPU, originalMemory, originalIOBPS := maxCPU, maxMemory, maxIOBPS
	if maxCPU <= 0 {
		maxCPU = config.DefaultCPULimitPercent
	}
	if maxMemory <= 0 {
		maxMemory = config.DefaultMemoryLimitMB
	}
	if maxIOBPS <= 0 {
		maxIOBPS = config.DefaultIOBPS
	}

	if originalCPU != maxCPU || originalMemory != maxMemory || originalIOBPS != maxIOBPS {
		jobLogger.Debug("applied resource defaults", "finalCPU", maxCPU, "finalMemory", maxMemory, "finalIOBPS", maxIOBPS)
	}

	cgroupJobDir := filepath.Join(config.CgroupsBaseDir, "job-"+jobId)

	jobLogger.Debug("creating cgroup", "cgroupPath", cgroupJobDir)

	// create cgroup first with limits
	if err := w.cgroup.Create(cgroupJobDir, maxCPU, maxMemory, maxIOBPS); err != nil {

		jobLogger.Error("failed to create cgroup", "cgroupPath", cgroupJobDir, "error", err)
		return nil, fmt.Errorf("failed to create cgroup for job %s: %w", jobId, err)
	}

	jobLogger.Debug("cgroup created successfully", "cgroupPath", cgroupJobDir, "cpuLimit", maxCPU, "memoryLimit", maxMemory, "ioLimit", maxIOBPS)

	// to prevent "command not found" errors
	resolvedCommand := command
	if !filepath.IsAbs(command) {

		jobLogger.Debug("resolving command path", "originalCommand", command)

		if path, err := exec.LookPath(command); err == nil {

			resolvedCommand = path
			jobLogger.Debug("command resolved", "originalCommand", command, "resolvedPath", resolvedCommand)
		} else {

			jobLogger.Error("command not found in PATH", "command", command, "error", err)
			w.cgroup.CleanupCgroup(jobId)
			return nil, fmt.Errorf("command not found in PATH: %s", command)
		}
	}

	limits := domain.ResourceLimits{
		MaxCPU:    maxCPU,
		MaxMemory: maxMemory,
		MaxIOBPS:  maxIOBPS,
	}

	job := &domain.Job{
		Id:         jobId,
		Command:    resolvedCommand,
		Args:       append([]string(nil), args...),
		Limits:     limits,
		Status:     domain.StatusInitializing,
		CgroupPath: cgroupJobDir,
		StartTime:  time.Now(),
	}
	w.store.CreateNewJob(job)

	jobLogger.Debug("job created in store", "status", string(job.Status), "resolvedCommand", resolvedCommand, "argsCount", len(job.Args))

	// get path to the job-init binary
	initPath, err := w.getJobInitPath()
	if err != nil {
		jobLogger.Error("failed to get job-init path", "error", err)

		w.cgroup.CleanupCgroup(job.Id)
		return nil, fmt.Errorf("failed to get job-init path: %w", err)
	}

	jobLogger.Debug("job-init binary located", "initPath", initPath)

	// prepare environment for init process
	env := w.os.Environ()
	jobEnvVars := []string{
		fmt.Sprintf("JOB_ID=%s", job.Id),
		fmt.Sprintf("JOB_COMMAND=%s", job.Command),
		fmt.Sprintf("JOB_CGROUP_PATH=%s", cgroupJobDir),
	}

	// add arguments as separate environment variables to handle spaces
	for i, arg := range job.Args {
		jobEnvVars = append(jobEnvVars, fmt.Sprintf("JOB_ARG_%d=%s", i, arg))
	}
	jobEnvVars = append(jobEnvVars, fmt.Sprintf("JOB_ARGS_COUNT=%d", len(job.Args)))
	env = append(env, jobEnvVars...)

	jobLogger.Debug("environment prepared for init process", "envVarsAdded", len(jobEnvVars), "totalEnvVars", len(env))

	// start the init process instead of the actual command
	cmd := w.cmdFactory.CreateCommand(initPath)
	cmd.SetEnv(env)

	jobLogger.Info("starting init process", "initPath", initPath, "targetCommand", job.Command, "targetArgs", job.Args)

	cmd.SetStdout(executor.New(w.store, job.Id))
	cmd.SetStderr(executor.New(w.store, job.Id))

	// create a process group for child process
	cmd.SetSysProcAttr(w.syscall.CreateProcessGroup())

	startTime := time.Now()
	if e := cmd.Start(); e != nil {
		duration := time.Since(startTime)

		jobLogger.Error("failed to start init process", "error", e, "duration", duration)

		failedJob := job.DeepCopy()
		if failErr := failedJob.Fail(-1); failErr != nil {

			jobLogger.Warn("domain validation failed for job failure", "domainError", failErr)

			failedJob.Status = domain.StatusFailed
			failedJob.ExitCode = -1
			now := time.Now()
			failedJob.EndTime = &now
		}
		w.store.UpdateJob(failedJob)

		w.cgroup.CleanupCgroup(job.Id)
		return nil, fmt.Errorf("start init process: %w", e)
	}

	startDuration := time.Since(startTime)

	process := cmd.Process()
	if process == nil {
		jobLogger.Error("process is nil after start", "startDuration", startDuration)
		w.cgroup.CleanupCgroup(job.Id)
		return nil, fmt.Errorf("process is nil after start")
	}

	job.Pid = int32(process.Pid())
	jobLogger.Info("init process started successfully", "pid", job.Pid, "startDuration", startDuration)

	runningJob := job.DeepCopy()
	if e := runningJob.MarkAsRunning(job.Pid); e != nil {

		jobLogger.Warn("domain validation failed for running status", "domainError", e)
		runningJob.Status = domain.StatusRunning
		runningJob.Pid = job.Pid
	}

	runningJob.StartTime = time.Now()
	w.store.UpdateJob(runningJob)

	jobLogger.Info("job marked as running", "pid", runningJob.Pid, "status", string(runningJob.Status))

	// Start monitoring
	go w.waitForCompletion(ctx, cmd, runningJob)

	return runningJob, nil
}

func (w *worker) getJobInitPath() (string, error) {

	w.logger.Debug("searching for job-init binary")

	execPath, err := w.os.Executable()
	if err != nil {
		w.logger.Warn("failed to get executable path", "error", err)
	} else {

		initPath := filepath.Join(filepath.Dir(execPath), "job-init")

		w.logger.Debug("checking job-init in executable directory", "execPath", execPath, "initPath", initPath)

		if _, e := w.os.Stat(initPath); e == nil {

			w.logger.Info("job-init found in executable directory", "path", initPath)
			return initPath, nil
		}
	}

	w.logger.Error("job-init binary not found in any location")

	return "", fmt.Errorf("job-init binary not found in executable dir, /usr/local/bin, or PATH")
}

// waitForCompletion waits for a job to complete and updates its status
func (w *worker) waitForCompletion(ctx context.Context, cmd os.Command, job *domain.Job) {

	log := w.logger.WithField("jobId", job.Id)

	startTime := time.Now()

	log.Debug("starting job monitoring", "pid", job.Pid, "command", job.Command)

	defer func() {

		// ensure cleanup happens even if something panics
		if r := recover(); r != nil {
			duration := time.Since(startTime)

			log.Error("panic in job monitoring", "panic", r, "duration", duration)

			w.cleanup(job.Id, int(job.Pid))
		}
	}()

	err := cmd.Wait()
	duration := time.Since(startTime)

	if err != nil {
		var exitCode int32
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = int32(exitErr.ExitCode())
		} else {
			exitCode = -1
		}

		log.Info("job completed with error", "exitCode", exitCode, "duration", duration, "error", err)

		completedJob := job.DeepCopy()

		if failErr := completedJob.Fail(exitCode); failErr != nil {

			log.Warn("domain validation failed for job failure", "domainError", failErr, "exitCode", exitCode)
			completedJob.Status = domain.StatusFailed
			completedJob.ExitCode = exitCode
			now := time.Now()
			completedJob.EndTime = &now

		}

		w.store.UpdateJob(completedJob)

	} else {
		log.Info("job completed successfully", "duration", duration)

		completedJob := job.DeepCopy()
		if completeErr := completedJob.Complete(0); completeErr != nil {

			log.Warn("domain validation failed for job completion", "domainError", completeErr)
			completedJob.Status = domain.StatusCompleted
			completedJob.ExitCode = 0
			now := time.Now()
			completedJob.EndTime = &now
		}
		w.store.UpdateJob(completedJob)
	}

	if job.CgroupPath != "" {
		log.Debug("cleaning up cgroup", "cgroupPath", job.CgroupPath)
		w.cgroup.CleanupCgroup(job.Id)
	}

	log.Debug("job monitoring completed", "totalDuration", time.Since(startTime))
}

func (w *worker) StopJob(ctx context.Context, jobId string) error {
	log := w.logger.WithField("jobId", jobId)

	select {
	case <-ctx.Done():
		log.Warn("stop job cancelled by context", "error", ctx.Err())
		return ctx.Err()
	default:
	}

	log.Info("stop job request received")

	job, exists := w.store.GetJob(jobId)
	if !exists {
		log.Warn("job not found for stop operation")
		return fmt.Errorf("job not found: %s", jobId)
	}

	if !job.IsRunning() {
		log.Warn("attempted to stop non-running job",
			"currentStatus", string(job.Status))
		return fmt.Errorf("job is not running: %s (current status: %s)", jobId, job.Status)
	}

	log.Info("stopping running job",
		"pid", job.Pid,
		"status", string(job.Status))

	stopStartTime := time.Now()

	// send to process group
	log.Debug("sending SIGTERM to process group", "processGroup", -int(job.Pid))
	if err := w.syscall.Kill(-int(job.Pid), syscall.SIGTERM); err != nil {

		log.Warn("failed to send SIGTERM to process group", "processGroup", job.Pid, "error", err)

		// if killing the group failed, try killing just the main process
		if errIn := w.syscall.Kill(int(job.Pid), syscall.SIGTERM); errIn != nil {
			log.Warn("failed to send SIGTERM to main process", "pid", job.Pid, "error", errIn)
		}
	}

	// for graceful shutdown
	gracefulWaitTime := 100 * time.Millisecond
	log.Debug("waiting for graceful shutdown", "timeout", gracefulWaitTime)
	time.Sleep(gracefulWaitTime)

	// check if process is still alive
	processAlive := w.processExists(int(job.Pid))
	gracefulDuration := time.Since(stopStartTime)

	if !processAlive {
		log.Info("process terminated gracefully", "pid", job.Pid, "duration", gracefulDuration)

		stoppedJob := job.DeepCopy()
		if stopErr := stoppedJob.Stop(); stopErr != nil {

			// if domain validation fails
			log.Warn("domain validation failed for graceful stop", "domainError", stopErr)
			stoppedJob.Status = domain.StatusStopped
			stoppedJob.ExitCode = 0

			now := time.Now()
			stoppedJob.EndTime = &now

		} else {
			stoppedJob.ExitCode = 0
		}
		w.store.UpdateJob(stoppedJob)

		if job.CgroupPath != "" {
			log.Debug("cleaning up cgroup after graceful stop", "cgroupPath", job.CgroupPath)
			w.cgroup.CleanupCgroup(job.Id)
		}
		return nil
	}

	// send to process group first
	log.Warn("process still alive after graceful shutdown, force killing", "pid", job.Pid, "gracefulDuration", gracefulDuration)

	if err := w.syscall.Kill(-int(job.Pid), syscall.SIGKILL); err != nil {
		log.Warn("failed to send SIGKILL to process group", "processGroup", job.Pid, "error", err)

		// then try killing just the main process
		if errIn := w.syscall.Kill(int(job.Pid), syscall.SIGKILL); errIn != nil {

			log.Error("failed to kill process", "pid", job.Pid, "error", errIn)
			return fmt.Errorf("failed to kill process: %v", errIn)

		}
	}

	stoppedJob := job.DeepCopy()
	if stopErr := stoppedJob.Stop(); stopErr != nil {

		// if domain validation fails
		log.Warn("domain validation failed for forced stop", "domainError", stopErr)
		stoppedJob.Status = domain.StatusStopped

		// forced kill
		stoppedJob.ExitCode = -1
		now := time.Now()
		stoppedJob.EndTime = &now
	}
	w.store.UpdateJob(stoppedJob)

	if job.CgroupPath != "" {
		log.Debug("cleaning up cgroup after forced stop", "cgroupPath", job.CgroupPath)
		w.cgroup.CleanupCgroup(job.Id)
	}

	totalDuration := time.Since(stopStartTime)
	log.Info("job stopped successfully", "method", "forced", "totalDuration", totalDuration)
	return nil
}

// cleanup is for when things go wrong
func (w *worker) cleanup(jobId string, pid int) {

	log := w.logger.WithFields("jobId", jobId, "pid", pid)
	log.Warn("starting emergency cleanup")

	// force kill the process group
	if err := w.syscall.Kill(-pid, syscall.SIGKILL); err != nil {

		log.Warn("failed to emergency kill process group", "processGroup", -pid, "error", err)

		// then try individual process
		if e := w.syscall.Kill(pid, syscall.SIGKILL); e != nil {
			log.Error("failed to emergency kill individual process", "pid", pid, "error", e)
		}
	}

	log.Debug("cleaning up cgroup in emergency cleanup")

	w.cgroup.CleanupCgroup(jobId)

	// Update job status using domain method
	if job, exists := w.store.GetJob(jobId); exists {
		failedJob := job.DeepCopy()

		if failErr := failedJob.Fail(-1); failErr != nil {
			// when domain validation fails
			log.Warn("domain validation failed in emergency cleanup", "domainError", failErr)
			failedJob.Status = domain.StatusFailed
			failedJob.ExitCode = -1
			now := time.Now()
			failedJob.EndTime = &now
		}

		w.store.UpdateJob(failedJob)
		log.Info("job marked as failed in emergency cleanup")
	} else {
		log.Warn("job not found during emergency cleanup")
	}

	log.Info("emergency cleanup completed")
}

// processExists checks if a process with the given PID exists
func (w *worker) processExists(pid int) bool {
	err := w.syscall.Kill(pid, 0)
	if err == nil {
		w.logger.Debug("process exists check: process found", "pid", pid)
		return true
	}

	if errors.Is(err, syscall.ESRCH) {
		w.logger.Debug("process exists check: no such process", "pid", pid)
		return false
	}

	if errors.Is(err, syscall.EPERM) {
		w.logger.Debug("process exists check: permission denied (process exists)", "pid", pid)
		return true
	}

	w.logger.Debug("process exists check: other error, assuming process doesn't exist", "pid", pid, "error", err)
	return false
}
