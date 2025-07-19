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
	"joblet/internal/joblet/core/upload"
	"joblet/internal/joblet/domain"
	"joblet/internal/joblet/scheduler"
	"joblet/internal/joblet/state"
	"joblet/pkg/config"
	"joblet/pkg/logger"
	"joblet/pkg/platform"
	"os"
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
	uploadManager  *upload.Manager
	scheduler      *scheduler.Scheduler
}

// NewPlatformJoblet creates a new Linux platform joblet
func NewPlatformJoblet(store state.Store, cfg *config.Config) interfaces.Joblet {
	platformInterface := platform.NewPlatform()
	processManager := process.NewProcessManager(platformInterface)
	cgroupResource := resource.New(cfg.Cgroup)
	jobIsolation := unprivileged.NewJobIsolation()
	filesystemIsolator := filesystem.NewIsolator(cfg.Filesystem, platformInterface)

	jobletLogger := logger.New().WithField("component", "linux-joblet")
	uploadManager := upload.NewManager(platformInterface, jobletLogger)

	w := &Joblet{
		store:          store,
		cgroup:         cgroupResource,
		processManager: processManager,
		jobIsolation:   jobIsolation,
		filesystem:     filesystemIsolator,
		platform:       platformInterface,
		config:         cfg,
		logger:         jobletLogger,
		uploadManager:  uploadManager,
		scheduler:      nil, // Will be set below
	}

	if err := w.setupCgroupControllers(); err != nil {
		w.logger.Fatal("cgroup controller setup failed", "error", err)
	}

	w.scheduler = scheduler.New(w)

	// Start the scheduler
	if err := w.scheduler.Start(); err != nil {
		w.logger.Fatal("scheduler start failed", "error", err)
	}

	return w
}

// StartJob now immediately starts the isolated job with uploads happening inside cgroups
func (w *Joblet) StartJob(ctx context.Context, command string, args []string, maxCPU, maxMemory, maxIOBPS int32, cpuCores string, uploads []domain.FileUpload, schedule string) (*domain.Job, error) {
	jobID := w.getNextJobID()
	log := w.logger.WithFields("jobID", jobID, "command", command, "uploadCount", len(uploads), "schedule", schedule)

	log.Debug("starting job with isolated file upload",
		"requestedCPU", maxCPU,
		"requestedMemory", maxMemory,
		"requestedIO", maxIOBPS,
		"uploads", len(uploads))

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

	// Handle scheduling if specified
	if schedule != "" {
		scheduledTime, er := time.Parse("2006-01-02T15:04:05Z07:00", schedule)
		if er != nil {
			return nil, fmt.Errorf("invalid schedule format from client: %w", er)
		}

		// Validate the scheduled time is reasonable
		now := time.Now()
		if scheduledTime.Before(now.Add(-1 * time.Minute)) {
			return nil, fmt.Errorf("scheduled time is in the past: %s", scheduledTime.Format(time.RFC3339))
		}

		maxFuture := now.Add(365 * 24 * time.Hour)
		if scheduledTime.After(maxFuture) {
			return nil, fmt.Errorf("scheduled time is too far in the future (max 1 year): %s", scheduledTime.Format(time.RFC3339))
		}

		// Set scheduling fields
		job.ScheduledTime = &scheduledTime
		job.Status = domain.StatusScheduled

		log.Info("job scheduled for future execution", "scheduledTime", scheduledTime.Format(time.RFC3339))

		// Process file uploads immediately (even for scheduled jobs)
		if len(uploads) > 0 {
			if e := w.processUploadsForScheduledJob(ctx, job, uploads); e != nil {
				return nil, fmt.Errorf("upload processing failed for scheduled job: %w", e)
			}
		}

		// Register job in store
		w.store.CreateNewJob(job)

		// Add to scheduler queue
		if e := w.scheduler.AddJob(job); e != nil {
			return nil, fmt.Errorf("failed to schedule job: %w", e)
		}

		return job, nil
	}

	// Immediate execution
	job.Status = domain.StatusInitializing
	return w.executeJobImmediately(ctx, job, uploads)
}

// executeJobImmediately extract existing logic
func (w *Joblet) executeJobImmediately(ctx context.Context, job *domain.Job, uploads []domain.FileUpload) (*domain.Job, error) {
	log := w.logger.WithFields("job", job)

	// Create minimal base workspace directory (for cgroup setup only)
	baseWorkspaceDir := filepath.Join(w.config.Filesystem.BaseDir, job.Id)
	if err := w.platform.MkdirAll(baseWorkspaceDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base workspace: %w", err)
	}

	if e := w.cgroup.Create(
		job.CgroupPath,
		job.Limits.MaxCPU,
		job.Limits.MaxMemory,
		job.Limits.MaxIOBPS,
	); e != nil {
		_ = w.platform.RemoveAll(baseWorkspaceDir)
		return nil, fmt.Errorf("resource limit enforcement failed: %w", e)
	}

	// Set CPU core restrictions if specified
	if job.Limits.CPUCores != "" {
		if e := w.setupCPUCoreRestrictions(job); e != nil {
			w.cgroup.CleanupCgroup(job.Id)
			_ = w.platform.RemoveAll(baseWorkspaceDir)
			return nil, fmt.Errorf("CPU core enforcement failed: %w", e)
		}
	}

	// Register job in store
	w.store.CreateNewJob(job)

	// Start the isolated process with uploads embedded in environment
	cmd, err := w.startProcessWithEmbeddedUploads(ctx, job, uploads)
	if err != nil {
		w.cleanupFailedJob(job)
		_ = w.platform.RemoveAll(baseWorkspaceDir)
		return nil, fmt.Errorf("process start failed: %w", err)
	}

	// Update job with process info
	w.updateJobAsRunning(job, cmd)

	// Start monitoring
	go w.monitorJob(ctx, cmd, job)

	log.Debug("job started successfully with isolated file upload", "pid", job.Pid, "uploadFiles", len(uploads))
	return job, nil
}

// processUploadsForScheduledJob
func (w *Joblet) processUploadsForScheduledJob(ctx context.Context, job *domain.Job, uploads []domain.FileUpload) error {
	if len(uploads) == 0 {
		return nil
	}

	log := w.logger.WithField("jobID", job.Id)
	log.Debug("processing uploads for scheduled job", "uploadCount", len(uploads))

	// Create workspace directory for the scheduled job
	workspaceDir := filepath.Join(w.config.Filesystem.BaseDir, job.Id, "work")
	if err := w.platform.MkdirAll(workspaceDir, 0755); err != nil {
		return fmt.Errorf("failed to create workspace for scheduled job: %w", err)
	}

	// Create and setup cgroups early (for resource limits during upload processing)
	if err := w.cgroup.Create(job.CgroupPath, job.Limits.MaxCPU, job.Limits.MaxMemory, job.Limits.MaxIOBPS); err != nil {
		_ = w.platform.RemoveAll(filepath.Dir(workspaceDir))
		return fmt.Errorf("resource limit setup failed for scheduled job: %w", err)
	}

	// Process uploads with proper isolation
	for _, ul := range uploads {
		targetPath := filepath.Join(workspaceDir, ul.Path)

		if ul.IsDirectory {
			if err := w.platform.MkdirAll(targetPath, os.FileMode(ul.Mode)); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", ul.Path, err)
			}
		} else {
			// Ensure parent directory exists
			if err := w.platform.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("failed to create parent directory for %s: %w", ul.Path, err)
			}

			// Write file content
			if err := w.platform.WriteFile(targetPath, ul.Content, os.FileMode(ul.Mode)); err != nil {
				return fmt.Errorf("failed to write file %s: %w", ul.Path, err)
			}
		}
	}

	log.Debug("uploads processed successfully for scheduled job")
	return nil
}

// ExecuteScheduledJob (implements scheduler.JobExecutor interface)
func (w *Joblet) ExecuteScheduledJob(ctx context.Context, job *domain.Job) error {
	log := w.logger.WithField("jobID", job.Id)
	log.Info("executing scheduled job", "originalScheduledTime", job.ScheduledTime.Format(time.RFC3339))

	// Transition from SCHEDULED to INITIALIZING
	if err := job.MarkAsInitializing(); err != nil {
		log.Error("failed to transition job to initializing", "error", err)
		return fmt.Errorf("invalid job state transition: %w", err)
	}

	// Update job status in store
	w.store.UpdateJob(job)

	// Execute the job (uploads were already processed during scheduling)
	_, err := w.executeJobImmediately(ctx, job, nil) // No uploads - already processed
	return err
}

// startProcessWithEmbeddedUploads starts the job process with file uploads serialized in environment
func (w *Joblet) startProcessWithEmbeddedUploads(ctx context.Context, job *domain.Job, uploads []domain.FileUpload) (platform.Command, error) {
	// Get the current executable path
	execPath, err := w.platform.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get current executable path: %w", err)
	}

	// Prepare workspace directory first
	workspaceDir := filepath.Join(w.config.Filesystem.BaseDir, job.Id, "work")
	if err := w.platform.MkdirAll(workspaceDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create workspace directory: %w", err)
	}

	// Process uploads using the new streaming approach
	if len(uploads) > 0 {
		if err := w.processUploadsWithStreaming(ctx, job, uploads, workspaceDir); err != nil {
			return nil, fmt.Errorf("failed to process uploads: %w", err)
		}
	}

	// Build environment with streaming support
	env := w.processManager.BuildJobEnvironmentWithUploads(job, execPath, uploads)

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

	w.logger.Debug("process launched with streaming upload support",
		"jobID", job.Id, "pid", result.PID, "uploadCount", len(uploads))

	return result.Command, nil
}

func (w *Joblet) processUploadsWithStreaming(ctx context.Context, job *domain.Job, uploads []domain.FileUpload, workspaceDir string) error {
	log := w.logger.WithField("operation", "process-uploads-streaming")

	// Prepare upload session
	session, err := w.uploadManager.PrepareUploadSession(job.Id, uploads, job.Limits.MaxMemory)
	if err != nil {
		return fmt.Errorf("failed to prepare upload session: %w", err)
	}

	log.Debug("processing uploads with streaming",
		"totalFiles", session.TotalFiles,
		"smallFiles", len(session.SmallFiles),
		"largeFiles", len(session.LargeFiles),
		"totalSize", session.TotalSize)

	// Process small files directly
	if len(session.SmallFiles) > 0 {
		if err := w.uploadManager.ProcessSmallFiles(session, workspaceDir); err != nil {
			return fmt.Errorf("failed to process small files: %w", err)
		}
	}

	// For large files, the streaming will be handled by the process manager
	// during environment setup - they will be streamed to the job process
	// through named pipes

	log.Debug("upload processing completed",
		"smallFilesProcessed", len(session.SmallFiles),
		"largeFilesQueued", len(session.LargeFiles))

	return nil
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

	// CPU cores MUST be set successfully
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

	if job.IsScheduled() {
		if w.scheduler.RemoveJob(jobID) {
			job.Stop()
			w.store.UpdateJob(job)
			log.Info("scheduled job removed from queue")
			return nil
		}
		return fmt.Errorf("failed to remove scheduled job from queue")
	}

	if !job.IsRunning() {
		return fmt.Errorf("job is not running: %s (status: %s)", jobID, job.Status)
	}

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

	// Wait for process completion
	err := cmd.Wait()

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

	log.Debug("job monitoring completed", "finalStatus", finalStatus, "exitCode", exitCode)
}

// cleanupJobResources handles both cgroup and filesystem
func (w *Joblet) cleanupJobResources(jobID string) {
	log := w.logger.WithField("jobID", jobID)
	log.Debug("cleaning up job resources including upload pipes")

	// Cleanup cgroup
	w.cgroup.CleanupCgroup(jobID)

	// Cleanup filesystem
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

	// Cleanup upload pipes directory
	pipesDir := filepath.Join(w.config.Filesystem.BaseDir, jobID, "pipes")
	if err := w.platform.RemoveAll(pipesDir); err != nil {
		log.Warn("failed to remove pipes directory", "path", pipesDir, "error", err)
	} else {
		log.Debug("removed pipes directory", "path", pipesDir)
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
