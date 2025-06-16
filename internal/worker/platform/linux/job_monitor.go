//go:build linux

package linux

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"

	"job-worker/internal/worker/domain"
	"job-worker/internal/worker/interfaces"
	"job-worker/internal/worker/platform/linux/network"
	"job-worker/internal/worker/platform/linux/process"
	"job-worker/pkg/logger"
	osinterface "job-worker/pkg/os"
)

// JobMonitor handles job monitoring operations
type JobMonitor struct {
	worker         *Worker
	store          interfaces.Store
	processCleaner *process.Cleaner
	networkManager *network.Manager
	cgroup         interfaces.Resource
	osInterface    osinterface.OsInterface
	config         *Config
	logger         *logger.Logger
}

// MonitoringResult contains the result of job monitoring
type MonitoringResult struct {
	JobID         string
	FinalStatus   domain.JobStatus
	ExitCode      int32
	Duration      time.Duration
	Error         error
	CleanupErrors []error
}

// NewJobMonitor creates a new job monitor
func NewJobMonitor(worker *Worker, deps *Dependencies) *JobMonitor {
	return &JobMonitor{
		worker:         worker,
		store:          deps.Store,
		processCleaner: deps.ProcessCleaner,
		networkManager: deps.NetworkManager,
		cgroup:         deps.Cgroup,
		osInterface:    deps.OsInterface,
		config:         deps.Config,
		logger:         logger.New().WithField("component", "job-monitor"),
	}
}

// StartMonitoring starts monitoring a job process
func (jm *JobMonitor) StartMonitoring(ctx context.Context, cmd osinterface.Command, job *domain.Job) {
	go jm.monitorJob(ctx, cmd, job)
}

// monitorJob monitors a job until completion
func (jm *JobMonitor) monitorJob(ctx context.Context, cmd osinterface.Command, job *domain.Job) {
	log := jm.logger.WithField("jobId", job.Id)
	startTime := time.Now()

	log.Debug("starting job monitoring",
		"pid", job.Pid,
		"command", job.Command,
		"networkGroup", job.NetworkGroupID)

	// Setup cleanup defer function
	defer func() {
		jm.performPostCompletionCleanup(job, log)

		if r := recover(); r != nil {
			duration := time.Since(startTime)
			log.Error("panic in job monitoring", "panic", r, "duration", duration)
			jm.handlePanicRecovery(job, fmt.Sprintf("%v", r))
		}
	}()

	// Wait for the process to complete
	result := jm.waitForProcessCompletion(ctx, cmd, job, startTime)

	// Update job status based on result
	jm.updateJobWithResult(job, result, log)

	// Perform cleanup
	jm.performJobCleanup(job, log)

	log.Debug("job monitoring completed", "totalDuration", time.Since(startTime))
}

// waitForProcessCompletion waits for the process to complete and returns the result
func (jm *JobMonitor) waitForProcessCompletion(ctx context.Context, cmd osinterface.Command, job *domain.Job, startTime time.Time) *MonitoringResult {
	log := jm.logger.WithField("jobId", job.Id)

	// Wait for command completion
	err := cmd.Wait()
	duration := time.Since(startTime)

	result := &MonitoringResult{
		JobID:    job.Id,
		Duration: duration,
		Error:    err,
	}

	if err != nil {
		// Process completed with error
		var exitCode int32
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = int32(exitErr.ExitCode())
		} else {
			exitCode = -1
		}

		log.Info("job completed with error",
			"exitCode", exitCode,
			"duration", duration,
			"error", err)

		result.FinalStatus = domain.StatusFailed
		result.ExitCode = exitCode
	} else {
		// Process completed successfully
		log.Info("job completed successfully", "duration", duration)
		result.FinalStatus = domain.StatusCompleted
		result.ExitCode = 0
	}

	return result
}

// updateJobWithResult updates the job status based on monitoring result
func (jm *JobMonitor) updateJobWithResult(job *domain.Job, result *MonitoringResult, log *logger.Logger) {
	completedJob := job.DeepCopy()

	// Try to use domain methods
	var domainErr error
	switch result.FinalStatus {
	case domain.StatusCompleted:
		domainErr = completedJob.Complete(result.ExitCode)
	case domain.StatusFailed:
		domainErr = completedJob.Fail(result.ExitCode)
	}

	// If domain validation fails, update manually
	if domainErr != nil {
		log.Warn("domain validation failed for job completion",
			"domainError", domainErr,
			"exitCode", result.ExitCode)

		completedJob.Status = result.FinalStatus
		completedJob.ExitCode = result.ExitCode
		now := time.Now()
		completedJob.EndTime = &now
	}

	jm.store.UpdateJob(completedJob)
}

// performJobCleanup performs cleanup after job completion
func (jm *JobMonitor) performJobCleanup(job *domain.Job, log *logger.Logger) {
	// Cleanup cgroup
	if job.CgroupPath != "" {
		log.Debug("cleaning up cgroup", "cgroupPath", job.CgroupPath)
		jm.cgroup.CleanupCgroup(job.Id)
	}

	// Handle network group cleanup, Keep namespace alive until group is empty
	if job.NetworkGroupID != "" {
		if err := jm.networkManager.DecrementJobCount(job.NetworkGroupID); err != nil {
			log.Warn("failed to decrement network group job count",
				"groupID", job.NetworkGroupID, "error", err)
		}
	}
}

// performPostCompletionCleanup performs cleanup that always needs to happen
func (jm *JobMonitor) performPostCompletionCleanup(job *domain.Job, log *logger.Logger) {
	// Cleanup network group reference
	if job.NetworkGroupID != "" {
		if err := jm.networkManager.DecrementJobCount(job.NetworkGroupID); err != nil {
			log.Warn("failed to decrement network group count in post-completion cleanup",
				"groupID", job.NetworkGroupID, "error", err)
		}
	}

	// Cleanup isolated job namespace
	if job.NetworkGroupID == "" {
		nsPath := jm.config.BuildNamespacePath(job.Id)
		if err := jm.osInterface.Remove(nsPath); err != nil && !jm.osInterface.IsNotExist(err) {
			log.Warn("failed to remove isolated job namespace", "path", nsPath, "error", err)
		}
	}
}

// handlePanicRecovery handles panic recovery during job monitoring
func (jm *JobMonitor) handlePanicRecovery(job *domain.Job, panicMsg string) {
	log := jm.logger.WithField("jobId", job.Id)
	log.Error("handling panic recovery", "panic", panicMsg)

	// Emergency cleanup
	jm.worker.EmergencyCleanup(job.Id, job.Pid, job.NetworkGroupID)
}

// MonitorJobHealth continuously monitors job health (optional background monitoring)
func (jm *JobMonitor) MonitorJobHealth(ctx context.Context, job *domain.Job, interval time.Duration) {
	if interval <= 0 {
		interval = 30 * time.Second // Default interval
	}

	log := jm.logger.WithFields("jobId", job.Id, "interval", interval)
	log.Debug("starting job health monitoring")

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Debug("job health monitoring cancelled")
			return
		case <-ticker.C:
			if !jm.checkJobHealth(job, log) {
				log.Warn("job health check failed, stopping monitoring")
				return
			}
		}
	}
}

// checkJobHealth performs a health check on a job
func (jm *JobMonitor) checkJobHealth(job *domain.Job, log *logger.Logger) bool {
	// Get current job state
	currentJob, exists := jm.store.GetJob(job.Id)
	if !exists {
		log.Warn("job no longer exists in store")
		return false
	}

	// If job is no longer running, stop monitoring
	if !currentJob.IsRunning() {
		log.Debug("job no longer running", "status", string(currentJob.Status))
		return false
	}

	// Check if process is still alive
	if currentJob.Pid > 0 {
		processExists := jm.isProcessAlive(currentJob.Pid)
		if !processExists {
			log.Warn("job marked as running but process not found, marking as failed")
			jm.handleDeadProcess(currentJob)
			return false
		}
	}

	// Check cgroup status if available
	if currentJob.CgroupPath != "" {
		if err := jm.checkCgroupHealth(currentJob.CgroupPath); err != nil {
			log.Warn("cgroup health check failed", "error", err)
			// Don't stop monitoring for cgroup issues, just log
		}
	}

	log.Debug("job health check passed")
	return true
}

// isProcessAlive checks if a process is still alive
func (jm *JobMonitor) isProcessAlive(pid int32) bool {
	// Use the process cleaner's method for consistency
	return jm.processCleaner.IsProcessAlive(pid)
}

// handleDeadProcess handles a process that died unexpectedly
func (jm *JobMonitor) handleDeadProcess(job *domain.Job) {
	log := jm.logger.WithField("jobId", job.Id)
	log.Info("handling unexpectedly dead process", "pid", job.Pid)

	// Mark job as failed
	failedJob := job.DeepCopy()
	if err := failedJob.Fail(-1); err != nil {
		log.Warn("domain validation failed for dead process", "error", err)
		failedJob.Status = domain.StatusFailed
		failedJob.ExitCode = -1
		now := time.Now()
		failedJob.EndTime = &now
	}
	jm.store.UpdateJob(failedJob)

	// Perform cleanup
	jm.performJobCleanup(job, log)
}

// checkCgroupHealth checks the health of a job's cgroup
func (jm *JobMonitor) checkCgroupHealth(cgroupPath string) error {
	if cgroupPath == "" {
		return nil
	}

	// Check if cgroup directory exists
	if _, err := jm.osInterface.Stat(cgroupPath); err != nil {
		if jm.osInterface.IsNotExist(err) {
			return fmt.Errorf("cgroup directory missing: %s", cgroupPath)
		}
		return fmt.Errorf("failed to stat cgroup directory: %w", err)
	}

	// Could add more sophisticated cgroup health checks here
	// like checking memory usage, CPU throttling, etc.

	return nil
}

// GetJobStatus returns detailed status information about a job
func (jm *JobMonitor) GetJobStatus(jobID string) map[string]interface{} {
	job, exists := jm.store.GetJob(jobID)
	if !exists {
		return map[string]interface{}{
			"job_id": jobID,
			"exists": false,
		}
	}

	status := map[string]interface{}{
		"job_id":        jobID,
		"exists":        true,
		"status":        string(job.Status),
		"command":       job.Command,
		"args":          job.Args,
		"pid":           job.Pid,
		"exit_code":     job.ExitCode,
		"start_time":    job.StartTime.Format(time.RFC3339),
		"is_running":    job.IsRunning(),
		"is_completed":  job.IsCompleted(),
		"network_group": job.NetworkGroupID,
		"cgroup_path":   job.CgroupPath,
	}

	// Add duration information
	if job.EndTime != nil {
		status["end_time"] = job.EndTime.Format(time.RFC3339)
		status["duration"] = job.Duration().String()
		status["duration_seconds"] = job.Duration().Seconds()
	} else if job.IsRunning() {
		status["current_duration"] = job.Duration().String()
		status["current_duration_seconds"] = job.Duration().Seconds()
	}

	// Add resource limits
	status["resource_limits"] = map[string]interface{}{
		"max_cpu":    job.Limits.MaxCPU,
		"max_memory": job.Limits.MaxMemory,
		"max_io_bps": job.Limits.MaxIOBPS,
	}

	// Check process status for running jobs
	if job.IsRunning() && job.Pid > 0 {
		processExists := jm.isProcessAlive(job.Pid)
		status["process_exists"] = processExists

		if !processExists {
			status["warning"] = "job marked as running but process not found"
		}
	}

	// Check cgroup status
	if job.CgroupPath != "" {
		cgroupExists := false
		if _, err := jm.osInterface.Stat(job.CgroupPath); err == nil {
			cgroupExists = true
		}
		status["cgroup_exists"] = cgroupExists
	}

	return status
}

// ListJobsByStatus returns jobs filtered by status
func (jm *JobMonitor) ListJobsByStatus(status domain.JobStatus) []*domain.Job {
	allJobs := jm.store.ListJobs()
	var filteredJobs []*domain.Job

	for _, job := range allJobs {
		if job.Status == status {
			filteredJobs = append(filteredJobs, job)
		}
	}

	return filteredJobs
}

// GetRunningJobs returns all currently running jobs
func (jm *JobMonitor) GetRunningJobs() []*domain.Job {
	return jm.ListJobsByStatus(domain.StatusRunning)
}

// GetCompletedJobs returns all completed jobs
func (jm *JobMonitor) GetCompletedJobs() []*domain.Job {
	return jm.ListJobsByStatus(domain.StatusCompleted)
}

// GetFailedJobs returns all failed jobs
func (jm *JobMonitor) GetFailedJobs() []*domain.Job {
	return jm.ListJobsByStatus(domain.StatusFailed)
}

// GetJobMetrics returns metrics about job execution
func (jm *JobMonitor) GetJobMetrics() map[string]interface{} {
	jobs := jm.store.ListJobs()

	metrics := map[string]interface{}{
		"component":  "job-monitor",
		"total_jobs": len(jobs),
	}

	// Count jobs by status
	statusCounts := make(map[string]int)
	var totalDuration time.Duration
	var completedJobs int

	for _, job := range jobs {
		statusCounts[string(job.Status)]++

		if job.IsCompleted() {
			totalDuration += job.Duration()
			completedJobs++
		}
	}

	metrics["jobs_by_status"] = statusCounts

	// Calculate average duration
	if completedJobs > 0 {
		avgDuration := totalDuration / time.Duration(completedJobs)
		metrics["average_duration_seconds"] = avgDuration.Seconds()
		metrics["average_duration"] = avgDuration.String()
	}

	// Add network group metrics if available
	if jm.networkManager != nil {
		networkStats := jm.networkManager.GetStats()
		for k, v := range networkStats {
			metrics["network_"+k] = v
		}
	}

	return metrics
}

// PerformHealthCheck performs a comprehensive health check of all jobs
func (jm *JobMonitor) PerformHealthCheck() map[string]interface{} {
	jm.logger.Debug("performing comprehensive health check")

	jobs := jm.store.ListJobs()
	result := map[string]interface{}{
		"timestamp":      time.Now().Format(time.RFC3339),
		"total_jobs":     len(jobs),
		"healthy_jobs":   0,
		"unhealthy_jobs": 0,
		"issues":         make([]string, 0),
	}

	var healthyCount, unhealthyCount int
	var issues []string

	for _, job := range jobs {
		if job.IsRunning() {
			if job.Pid <= 0 {
				issues = append(issues, fmt.Sprintf("job %s has invalid PID", job.Id))
				unhealthyCount++
				continue
			}

			if !jm.isProcessAlive(job.Pid) {
				issues = append(issues, fmt.Sprintf("job %s marked as running but process not found", job.Id))
				unhealthyCount++
				continue
			}

			// Check cgroup if available
			if job.CgroupPath != "" {
				if err := jm.checkCgroupHealth(job.CgroupPath); err != nil {
					issues = append(issues, fmt.Sprintf("job %s cgroup issue: %v", job.Id, err))
					// Don't count as unhealthy for cgroup issues
				}
			}

			healthyCount++
		} else {
			// Non-running jobs are considered healthy
			healthyCount++
		}
	}

	result["healthy_jobs"] = healthyCount
	result["unhealthy_jobs"] = unhealthyCount
	result["issues"] = issues

	return result
}

// WatchJob provides real-time monitoring of a specific job
func (jm *JobMonitor) WatchJob(ctx context.Context, jobID string, callback func(status map[string]interface{})) error {
	if callback == nil {
		return fmt.Errorf("callback function cannot be nil")
	}

	log := jm.logger.WithField("jobId", jobID)
	log.Debug("starting job watch")

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Debug("job watch cancelled")
			return ctx.Err()
		case <-ticker.C:
			status := jm.GetJobStatus(jobID)
			callback(status)

			// Stop watching if job no longer exists or is completed
			if exists, ok := status["exists"].(bool); !ok || !exists {
				log.Debug("job no longer exists, stopping watch")
				return nil
			}

			if completed, ok := status["is_completed"].(bool); ok && completed {
				log.Debug("job completed, stopping watch")
				return nil
			}
		}
	}
}

// GetMonitoringStats returns statistics about the monitoring component
func (jm *JobMonitor) GetMonitoringStats() map[string]interface{} {
	return map[string]interface{}{
		"component":       "job-monitor",
		"monitoring_jobs": len(jm.GetRunningJobs()),
		"total_jobs":      len(jm.store.ListJobs()),
	}
}
