//go:build linux

package core

import (
	"context"
	"errors"
	"fmt"
	"joblet/internal/joblet/core/filesystem"
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
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

var jobCounter int64

// Joblet handles job execution with configuration
type Joblet struct {
	store          state.Store
	cgroup         resource.Resource
	processManager *process.Manager
	jobIsolation   *unprivileged.JobIsolation
	filesystem     *filesystem.Isolator
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
	filesystemIsolator := filesystem.NewIsolator(cfg.Filesystem, platformInterface)

	w := &Joblet{
		store:          store,
		cgroup:         cgroupResource,
		processManager: processManager,
		jobIsolation:   jobIsolation,
		filesystem:     filesystemIsolator,
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

func (w *Joblet) StartJob(ctx context.Context, command string, args []string, maxCPU, maxMemory, maxIOBPS int32, cpuCores string) (*domain.Job, error) {
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
	job := w.createJobDomain(jobID, resolvedCommand, args, maxCPU, maxMemory, maxIOBPS, cpuCores)

	log.Debug("creating cgroup with strict resource limit enforcement",
		"limits", fmt.Sprintf("CPU:%d%%, Memory:%dMB, IO:%d BPS, Cores:%s",
			job.Limits.MaxCPU, job.Limits.MaxMemory, job.Limits.MaxIOBPS, job.Limits.CPUCores))

	// Setup cgroup resources
	if e := w.cgroup.Create(
		job.CgroupPath,
		job.Limits.MaxCPU,
		job.Limits.MaxMemory,
		job.Limits.MaxIOBPS,
	); e != nil {
		return nil, fmt.Errorf("resource limit enforcement failed: %w", e)
	}

	// Set CPU core restrictions if specified
	if job.Limits.CPUCores != "" {
		if e := w.setupCPUCoreRestrictions(job); e != nil {
			w.cgroup.CleanupCgroup(job.Id)
			return nil, fmt.Errorf("CPU core enforcement failed: %w", e)
		}
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

func (w *Joblet) setupCPUCoreRestrictions(job *domain.Job) error {
	log := w.logger.WithFields("jobID", job.Id, "cpuCores", job.Limits.CPUCores)
	log.Debug("enforcing CPU core restrictions")

	// Validate core specification format
	if err := w.validateCoreSpecification(job.Limits.CPUCores); err != nil {
		return fmt.Errorf("invalid CPU core specification '%s': %w", job.Limits.CPUCores, err)
	}

	// Check if requested cores are available
	if err := w.validateCoreAvailability(job.Limits.CPUCores); err != nil {
		return fmt.Errorf("requested CPU cores not available: %w", err)
	}

	// STRICT: CPU cores MUST be set successfully
	if err := w.cgroup.SetCPUCores(job.CgroupPath, job.Limits.CPUCores); err != nil {
		return fmt.Errorf("failed to enforce CPU core restriction '%s': %w",
			job.Limits.CPUCores, err)
	}

	log.Info("CPU core restrictions enforced successfully",
		"cores", job.Limits.CPUCores,
		"coreCount", w.parseCoreCount(job.Limits.CPUCores))

	return nil
}

func (w *Joblet) validateCoreSpecification(cores string) error {
	if cores == "" {
		return fmt.Errorf("empty core specification")
	}

	// Validate format: "0-3", "1,3,5", "2", etc.
	if strings.Contains(cores, "-") {
		// Range format: "0-3"
		parts := strings.Split(cores, "-")
		if len(parts) != 2 {
			return fmt.Errorf("invalid range format, expected 'start-end'")
		}

		start, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
		end, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))

		if err1 != nil || err2 != nil {
			return fmt.Errorf("invalid range numbers")
		}

		if start < 0 || end < 0 || start > end {
			return fmt.Errorf("invalid range: start=%d, end=%d", start, end)
		}
	} else {
		// List format: "1,3,5" or single core "2"
		coreList := strings.Split(cores, ",")
		for _, coreStr := range coreList {
			coreNum, err := strconv.Atoi(strings.TrimSpace(coreStr))
			if err != nil || coreNum < 0 {
				return fmt.Errorf("invalid core number: %s", coreStr)
			}
		}
	}

	return nil
}

func (w *Joblet) validateCoreAvailability(cores string) error {
	availableCores := w.getSystemCoreCount()
	requestedCores := w.expandCoreSpecification(cores)

	for _, coreNum := range requestedCores {
		if coreNum >= availableCores {
			return fmt.Errorf("core %d not available (system has %d cores)", coreNum, availableCores)
		}
	}

	return nil
}

func (w *Joblet) getSystemCoreCount() int {
	// Read from /proc/cpuinfo or use runtime
	return runtime.NumCPU()
}

func (w *Joblet) expandCoreSpecification(cores string) []int {
	var result []int

	if strings.Contains(cores, "-") {
		// Range: "0-3" -> [0,1,2,3]
		parts := strings.Split(cores, "-")
		start, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
		end, _ := strconv.Atoi(strings.TrimSpace(parts[1]))

		for i := start; i <= end; i++ {
			result = append(result, i)
		}
	} else {
		// List: "1,3,5" -> [1,3,5]
		coreList := strings.Split(cores, ",")
		for _, coreStr := range coreList {
			if coreNum, err := strconv.Atoi(strings.TrimSpace(coreStr)); err == nil {
				result = append(result, coreNum)
			}
		}
	}

	return result
}

func (w *Joblet) parseCoreCount(cores string) int {
	return len(w.expandCoreSpecification(cores))
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

	// Cleanup all job resources (cgroup + filesystem)
	w.cleanupJobResources(jobID)

	log.Debug("job stopped successfully", "method", result.Method)
	return nil
}

// Helper methods (keeping existing implementations)
func (w *Joblet) getNextJobID() string {
	nextID := atomic.AddInt64(&jobCounter, 1)
	return fmt.Sprintf("%d", nextID)
}

func (w *Joblet) createJobDomain(jobID, resolvedCommand string, args []string, maxCPU, maxMemory, maxIOBPS int32, cpuCores string) *domain.Job {
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
			CPUCores:  cpuCores,
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

	// environment with job information and mode indicator
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

	w.logger.Debug("process launched using two-stage init with self-cgroup-assignment",
		"jobID", job.Id, "pid", result.PID)

	return result.Command, nil
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

	// Cleanup all job resources (cgroup + filesystem)
	w.cleanupJobResources(job.Id)

	log.Debug("job monitoring completed",
		"finalStatus", finalStatus,
		"exitCode", exitCode,
		"duration", duration)
}

// cleanupJobResources handles both cgroup and filesystem
func (w *Joblet) cleanupJobResources(jobID string) {
	log := w.logger.WithField("jobID", jobID)
	log.Debug("cleaning up job resources")

	w.cgroup.CleanupCgroup(jobID)

	jobRootDir := filepath.Join(w.config.Filesystem.BaseDir, jobID)
	jobTmpDir := strings.Replace(w.config.Filesystem.TmpDir, "{JOB_ID}", jobID, -1)

	if err := w.platform.RemoveAll(jobRootDir); err != nil {
		log.Warn("failed to remove job root directory", "path", jobRootDir, "error", err)
	} else {
		log.Debug("removed job root directory", "path", jobRootDir)
	}

	if err := w.platform.RemoveAll(jobTmpDir); err != nil {
		log.Warn("failed to remove job tmp directory", "path", jobTmpDir, "error", err)
	} else {
		log.Debug("removed job tmp directory", "path", jobTmpDir)
	}

	log.Debug("job resources cleanup completed")
}

func (w *Joblet) cleanupFailedJob(job *domain.Job) {
	failedJob := job.DeepCopy()
	failedJob.Fail(-1)
	w.store.UpdateJob(failedJob)
	w.cleanupJobResources(job.Id)
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
