//go:build linux

package linux

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"job-worker/internal/worker/domain"
	"job-worker/internal/worker/interfaces"
	"job-worker/internal/worker/platform/linux/network"
	"job-worker/internal/worker/platform/linux/process"
	"job-worker/pkg/logger"
	osinterface "job-worker/pkg/os"
)

// JobStopper handles job stopping operations
type JobStopper struct {
	worker         *Worker
	store          interfaces.Store
	processCleaner *process.Cleaner
	networkManager *network.Manager
	cgroup         interfaces.Resource
	osInterface    osinterface.OsInterface
	config         *Config
	logger         *logger.Logger
}

// StopJobRequest contains parameters for stopping a job
type StopJobRequest struct {
	JobID           string
	GracefulTimeout time.Duration
	ForceKill       bool
	CleanupNetwork  bool
}

// StopJobResult contains the result of stopping a job
type StopJobResult struct {
	JobID       string
	FinalStatus domain.JobStatus
	StopMethod  string // "graceful", "forced", "already_stopped"
	Duration    time.Duration
	ExitCode    int32
	Errors      []error
}

// NewJobStopper creates a new job stopper
func NewJobStopper(worker *Worker, deps *Dependencies) *JobStopper {
	return &JobStopper{
		worker:         worker,
		store:          deps.Store,
		processCleaner: deps.ProcessCleaner,
		networkManager: deps.NetworkManager,
		cgroup:         deps.Cgroup,
		osInterface:    deps.OsInterface,
		config:         deps.Config,
		logger:         logger.New().WithField("component", "job-stopper"),
	}
}

// StopJob stops a job with the given parameters
func (js *JobStopper) StopJob(ctx context.Context, req *StopJobRequest) error {
	// Validate request
	if err := js.validateStopRequest(req); err != nil {
		return fmt.Errorf("invalid stop request: %w", err)
	}

	log := js.logger.WithField("jobId", req.JobID)

	// Check for context cancellation
	select {
	case <-ctx.Done():
		log.Warn("stop job cancelled by context", "error", ctx.Err())
		return ctx.Err()
	default:
	}

	log.Info("stop job request received")

	// Get the job
	job, exists := js.store.GetJob(req.JobID)
	if !exists {
		log.Warn("job not found for stop operation")
		return fmt.Errorf("job not found: %s", req.JobID)
	}

	// Check if job can be stopped
	if err := js.validateJobCanBeStopped(job); err != nil {
		log.Warn("job cannot be stopped", "currentStatus", string(job.Status), "error", err)
		return err
	}

	log.Info("stopping running job", "pid", job.Pid, "status", string(job.Status))

	startTime := time.Now()

	// Perform the actual stop operation
	result, err := js.performStopOperation(ctx, job, req)
	if err != nil {
		log.Error("stop operation failed", "error", err, "duration", time.Since(startTime))
		return err
	}

	// Update job status
	if err := js.updateJobStatus(job, result); err != nil {
		log.Warn("failed to update job status", "error", err)
		// Don't return error here as the job was actually stopped
	}

	// Cleanup resources
	js.cleanupJobResources(job, req.CleanupNetwork)

	log.Info("job stopped successfully",
		"method", result.StopMethod,
		"finalStatus", string(result.FinalStatus),
		"exitCode", result.ExitCode,
		"totalDuration", result.Duration)

	return nil
}

// validateStopRequest validates the stop job request
func (js *JobStopper) validateStopRequest(req *StopJobRequest) error {
	if req == nil {
		return fmt.Errorf("stop request cannot be nil")
	}

	if req.JobID == "" {
		return fmt.Errorf("job ID cannot be empty")
	}

	if req.GracefulTimeout < 0 {
		return fmt.Errorf("graceful timeout cannot be negative")
	}

	// Set default timeout if not specified
	if req.GracefulTimeout == 0 {
		req.GracefulTimeout = GracefulShutdownTimeout
	}

	return nil
}

// validateJobCanBeStopped checks if a job can be stopped
func (js *JobStopper) validateJobCanBeStopped(job *domain.Job) error {
	if !job.IsRunning() {
		return fmt.Errorf("job is not running: %s (current status: %s)",
			job.Id, job.Status)
	}

	if job.Pid <= 0 {
		return fmt.Errorf("job has invalid PID: %d", job.Pid)
	}

	return nil
}

// performStopOperation performs the actual stop operation
func (js *JobStopper) performStopOperation(ctx context.Context, job *domain.Job, req *StopJobRequest) (*StopJobResult, error) {
	log := js.logger.WithField("jobId", job.Id)
	startTime := time.Now()

	result := &StopJobResult{
		JobID:  job.Id,
		Errors: make([]error, 0),
	}

	// Clean up isolated job namespace symlink immediately
	if job.NetworkGroupID == "" {
		nsPath := filepath.Join(js.config.NetnsPath, job.Id)
		if err := js.osInterface.Remove(nsPath); err != nil && !js.osInterface.IsNotExist(err) {
			log.Warn("failed to remove isolated job namespace", "path", nsPath, "error", err)
		}
	}

	// Create cleanup request
	cleanupReq := &process.CleanupRequest{
		JobID:           job.Id,
		PID:             job.Pid,
		CgroupPath:      job.CgroupPath,
		NetworkGroupID:  job.NetworkGroupID,
		NamespacePath:   js.buildNamespacePath(job),
		IsIsolatedJob:   job.NetworkGroupID == "",
		ForceKill:       req.ForceKill,
		GracefulTimeout: req.GracefulTimeout,
	}

	// Perform process cleanup
	cleanupResult, err := js.processCleaner.CleanupProcess(ctx, cleanupReq)
	if err != nil {
		log.Error("process cleanup failed", "error", err)
		result.Errors = append(result.Errors, fmt.Errorf("process cleanup failed: %w", err))
	}

	// Determine final status and exit code based on cleanup result
	if cleanupResult != nil {
		result.StopMethod = cleanupResult.Method
		result.Duration = cleanupResult.Duration

		switch cleanupResult.Method {
		case "graceful":
			result.FinalStatus = domain.StatusStopped
			result.ExitCode = 0
		case "forced", "force_failed":
			result.FinalStatus = domain.StatusStopped
			result.ExitCode = -1
		case "already_dead":
			result.FinalStatus = domain.StatusCompleted
			result.ExitCode = 0
		default:
			result.FinalStatus = domain.StatusFailed
			result.ExitCode = -1
		}

		// Collect any cleanup errors
		result.Errors = append(result.Errors, cleanupResult.Errors...)
	} else {
		// Fallback if cleanup result is nil
		result.FinalStatus = domain.StatusFailed
		result.ExitCode = -1
		result.StopMethod = "unknown"
		result.Duration = time.Since(startTime)
	}

	return result, nil
}

// updateJobStatus updates the job status in the store
func (js *JobStopper) updateJobStatus(job *domain.Job, result *StopJobResult) error {
	stoppedJob := job.DeepCopy()

	// Try to use domain methods first
	var domainErr error
	switch result.FinalStatus {
	case domain.StatusStopped:
		domainErr = stoppedJob.Stop()
	case domain.StatusCompleted:
		domainErr = stoppedJob.Complete(result.ExitCode)
	case domain.StatusFailed:
		domainErr = stoppedJob.Fail(result.ExitCode)
	}

	// If domain validation fails, update manually
	if domainErr != nil {
		js.logger.Warn("domain validation failed, updating manually",
			"error", domainErr,
			"targetStatus", string(result.FinalStatus))

		stoppedJob.Status = result.FinalStatus
		stoppedJob.ExitCode = result.ExitCode
		now := time.Now()
		stoppedJob.EndTime = &now
	}

	js.store.UpdateJob(stoppedJob)
	return nil
}

// cleanupJobResources cleans up job resources
func (js *JobStopper) cleanupJobResources(job *domain.Job, cleanupNetwork bool) {
	log := js.logger.WithField("jobId", job.Id)

	// Cleanup cgroup
	if job.CgroupPath != "" {
		log.Debug("cleaning up cgroup", "cgroupPath", job.CgroupPath)
		js.cgroup.CleanupCgroup(job.Id)
	}

	// Handle network group cleanup
	if job.NetworkGroupID != "" && cleanupNetwork {
		if err := js.networkManager.DecrementJobCount(job.NetworkGroupID); err != nil {
			log.Warn("failed to decrement network group job count",
				"groupID", job.NetworkGroupID, "error", err)
		}
	}
}

// buildNamespacePath builds the namespace path for a job
func (js *JobStopper) buildNamespacePath(job *domain.Job) string {
	if job.NetworkGroupID != "" {
		// Network group path
		return filepath.Join(js.config.VarRunNetns, job.NetworkGroupID)
	}
	// Isolated job path
	return filepath.Join(js.config.NetnsPath, job.Id)
}

// ForceStopJob forcefully stops a job without graceful shutdown
func (js *JobStopper) ForceStopJob(ctx context.Context, jobID string) error {
	req := &StopJobRequest{
		JobID:           jobID,
		GracefulTimeout: 0,
		ForceKill:       true,
		CleanupNetwork:  true,
	}

	return js.StopJob(ctx, req)
}

// StopJobGracefully stops a job with graceful shutdown
func (js *JobStopper) StopJobGracefully(ctx context.Context, jobID string, timeout time.Duration) error {
	if timeout <= 0 {
		timeout = GracefulShutdownTimeout
	}

	req := &StopJobRequest{
		JobID:           jobID,
		GracefulTimeout: timeout,
		ForceKill:       false,
		CleanupNetwork:  true,
	}

	return js.StopJob(ctx, req)
}

// StopAllJobs stops all running jobs
func (js *JobStopper) StopAllJobs(ctx context.Context, gracefulTimeout time.Duration) []error {
	js.logger.Info("stopping all running jobs")

	jobs := js.store.ListJobs()
	var errors []error

	for _, job := range jobs {
		if job.IsRunning() {
			js.logger.Debug("stopping job", "jobID", job.Id, "pid", job.Pid)

			req := &StopJobRequest{
				JobID:           job.Id,
				GracefulTimeout: gracefulTimeout,
				ForceKill:       false,
				CleanupNetwork:  true,
			}

			if err := js.StopJob(ctx, req); err != nil {
				js.logger.Warn("failed to stop job", "jobID", job.Id, "error", err)
				errors = append(errors, fmt.Errorf("failed to stop job %s: %w", job.Id, err))
			}
		}
	}

	js.logger.Info("finished stopping all jobs", "errors", len(errors))
	return errors
}

// GetStopJobStatus returns the status of a stop operation
func (js *JobStopper) GetStopJobStatus(jobID string) map[string]interface{} {
	job, exists := js.store.GetJob(jobID)
	if !exists {
		return map[string]interface{}{
			"job_id": jobID,
			"exists": false,
		}
	}

	status := map[string]interface{}{
		"job_id":       jobID,
		"exists":       true,
		"status":       string(job.Status),
		"is_running":   job.IsRunning(),
		"is_completed": job.IsCompleted(),
		"pid":          job.Pid,
		"exit_code":    job.ExitCode,
	}

	if job.EndTime != nil {
		status["end_time"] = job.EndTime.Format(time.RFC3339)
		status["duration"] = job.Duration().String()
	}

	// Check if process is actually running (for running jobs)
	if job.IsRunning() && job.Pid > 0 {
		processExists := js.processCleaner.IsProcessAlive(job.Pid)
		status["process_exists"] = processExists

		if !processExists {
			status["note"] = "job marked as running but process not found"
		}
	}

	return status
}

// ValidateStopRequest validates a stop request
func (js *JobStopper) ValidateStopRequest(req *StopJobRequest) error {
	return js.validateStopRequest(req)
}

// EstimateStopTime estimates how long it will take to stop a job
func (js *JobStopper) EstimateStopTime(jobID string, gracefulTimeout time.Duration) time.Duration {
	job, exists := js.store.GetJob(jobID)
	if !exists || !job.IsRunning() {
		return 0 // Job doesn't exist or isn't running
	}

	if gracefulTimeout <= 0 {
		gracefulTimeout = GracefulShutdownTimeout
	}

	// Estimate: graceful timeout + force kill time + cleanup time
	estimatedTime := gracefulTimeout + (100 * time.Millisecond) + (50 * time.Millisecond)

	return estimatedTime
}

// GetStopMetrics returns metrics about stop operations
func (js *JobStopper) GetStopMetrics() map[string]interface{} {
	jobs := js.store.ListJobs()

	var stoppedCount, failedCount, completedCount int
	for _, job := range jobs {
		switch job.Status {
		case domain.StatusStopped:
			stoppedCount++
		case domain.StatusFailed:
			failedCount++
		case domain.StatusCompleted:
			completedCount++
		}
	}

	return map[string]interface{}{
		"component":       "job-stopper",
		"stopped_jobs":    stoppedCount,
		"failed_jobs":     failedCount,
		"completed_jobs":  completedCount,
		"default_timeout": GracefulShutdownTimeout.String(),
	}
}

// IsJobStoppable checks if a job can be stopped
func (js *JobStopper) IsJobStoppable(jobID string) (bool, string) {
	job, exists := js.store.GetJob(jobID)
	if !exists {
		return false, "job not found"
	}

	if !job.IsRunning() {
		return false, fmt.Sprintf("job not running (status: %s)", job.Status)
	}

	if job.Pid <= 0 {
		return false, "invalid PID"
	}

	return true, "job can be stopped"
}

// PreflightStopCheck performs checks before stopping a job
func (js *JobStopper) PreflightStopCheck(jobID string) error {
	js.logger.Debug("performing preflight stop check", "jobID", jobID)

	stoppable, reason := js.IsJobStoppable(jobID)
	if !stoppable {
		return fmt.Errorf("preflight check failed: %s", reason)
	}

	js.logger.Debug("preflight stop check passed", "jobID", jobID)
	return nil
}
