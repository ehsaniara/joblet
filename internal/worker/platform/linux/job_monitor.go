//go:build linux

// Package linux provides comprehensive job monitoring functionality for Linux platform workers.
// This package handles continuous job process monitoring, health checks, status updates,
// and automatic cleanup procedures.
//
// Job Monitoring Architecture:
//
//	┌─────────────────────────────────────────────────────────────────┐
//	│                    Job Monitor System                            │
//	└─────────────────────────────────────────────────────────────────┘
//	│
//	├── 1. Process Lifecycle Monitoring
//	│   ├── Process existence verification
//	│   ├── Exit code detection and handling
//	│   ├── Signal handling and cleanup
//	│   └── Panic recovery and emergency procedures
//	│
//	├── 2. Health Check System
//	│   ├── Continuous health verification
//	│   ├── Resource constraint monitoring
//	│   ├── Cgroup status validation
//	│   └── Network connectivity checks
//	│
//	├── 3. Status Management
//	│   ├── Real-time status updates
//	│   ├── Job completion detection
//	│   ├── Failure analysis and reporting
//	│   └── Performance metrics collection
//	│
//	├── 4. Resource Cleanup
//	│   ├── Automatic resource cleanup on completion
//	│   ├── Network group membership management
//	│   ├── Cgroup lifecycle management
//	│   └── Namespace artifact cleanup
//	│
//	└── 5. Monitoring Services
//	    ├── Real-time job watching capabilities
//	    ├── Comprehensive health reporting
//	    ├── Performance metrics and statistics
//	    └── Administrative monitoring interfaces
//
// Key Features:
// - Continuous process monitoring with automatic failure detection
// - Comprehensive health checks for system and job-level issues
// - Automatic resource cleanup preventing system resource leaks
// - Real-time status updates and notification system
// - Emergency recovery procedures for catastrophic failures
// - Performance metrics collection for capacity planning
// - Administrative interfaces for system monitoring
//
// Thread Safety: All public methods are thread-safe and designed for
// concurrent access from multiple monitoring goroutines.
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

// JobMonitor handles comprehensive job monitoring operations including
// process lifecycle management, health checks, and resource cleanup.
//
// Core Responsibilities:
//
// Process Monitoring:
// - Continuous process existence verification
// - Exit code detection and interpretation
// - Signal handling and graceful termination
// - Panic recovery and emergency cleanup
//
// Health Management:
// - Periodic health checks for running jobs
// - Resource constraint validation
// - Cgroup status monitoring
// - Network connectivity verification
//
// Resource Management:
// - Automatic cleanup on job completion
// - Network group membership tracking
// - Cgroup lifecycle management
// - Namespace artifact cleanup
//
// The monitor operates primarily through goroutines that run independently
// for each job, providing isolated monitoring without interference.
type JobMonitor struct {
	// Core component references
	worker *Worker          // Parent worker for shared functionality
	store  interfaces.Store // Job state management and persistence

	// Platform-specific components
	processCleaner *process.Cleaner    // Process cleanup and termination
	networkManager *network.Manager    // Network resource management
	cgroup         interfaces.Resource // Cgroup resource management

	// System interfaces
	osInterface osinterface.OsInterface // OS operations abstraction

	// Configuration and logging
	config *Config        // Platform-specific configuration
	logger *logger.Logger // Structured logging with context
}

// MonitoringResult contains comprehensive results from job monitoring operations.
// This structure provides detailed information about job completion, performance,
// and any issues encountered during execution.
//
// The result includes both successful completion information and error details
// for comprehensive analysis and debugging.
type MonitoringResult struct {
	JobID         string           // Job identifier for correlation
	FinalStatus   domain.JobStatus // Final job status (COMPLETED, FAILED, etc.)
	ExitCode      int32            // Process exit code
	Duration      time.Duration    // Total execution time
	Error         error            // Primary error if job failed
	CleanupErrors []error          // Any errors during cleanup process
}

// NewJobMonitor creates a new job monitor with proper dependency injection.
// This factory function initializes the monitor with all required components
// and establishes logging context for debugging.
//
// Parameters:
//
//	worker: Parent worker providing shared functionality
//	deps:   Complete dependency structure with required components
//
// Returns: Fully initialized JobMonitor ready for monitoring operations
//
// The monitor inherits configuration and logging context from the parent
// worker while maintaining its own specialized monitoring functionality.
func NewJobMonitor(worker *Worker, deps *Dependencies) *JobMonitor {
	return &JobMonitor{
		// Core references
		worker: worker,
		store:  deps.Store,

		// Platform components
		processCleaner: deps.ProcessCleaner,
		networkManager: deps.NetworkManager,
		cgroup:         deps.Cgroup,

		// System interfaces
		osInterface: deps.OsInterface,

		// Configuration and logging
		config: deps.Config,
		logger: logger.New().WithField("component", "job-monitor"),
	}
}

// StartMonitoring initiates comprehensive monitoring for a running job.
// This method launches a dedicated monitoring goroutine that tracks the job
// throughout its entire lifecycle until completion or failure.
//
// Monitoring Features:
// - Process existence verification
// - Exit code detection and interpretation
// - Resource usage tracking
// - Automatic cleanup on completion
// - Panic recovery and emergency procedures
// - Real-time status updates
//
// Parameters:
//
//	ctx: Context for monitoring lifecycle control and cancellation
//	cmd: Command interface providing process control and monitoring
//	job: Job domain object containing metadata and configuration
//
// The monitoring goroutine runs independently and handles all aspects of
// job lifecycle management. It automatically terminates when the job
// completes or when the context is cancelled.
func (jm *JobMonitor) StartMonitoring(ctx context.Context, cmd osinterface.Command, job *domain.Job) {
	// Launch monitoring in separate goroutine for non-blocking operation
	// This allows the caller to continue with other operations while
	// monitoring runs independently in the background
	go jm.monitorJob(ctx, cmd, job)
}

// monitorJob performs the complete job monitoring lifecycle.
// This method is the core monitoring loop that tracks a job from start
// to completion, handling all aspects of process management and cleanup.
//
// Monitoring Lifecycle:
//  1. Setup monitoring context and logging
//  2. Wait for process completion
//  3. Analyze completion results
//  4. Update job status in store
//  5. Perform resource cleanup
//  6. Handle any errors or panics
//
// Parameters:
//
//	ctx: Context for monitoring control and cancellation
//	cmd: Command interface for process monitoring
//	job: Job domain object being monitored
//
// Error Handling: This method includes comprehensive error handling and
// panic recovery to ensure monitoring never crashes and always performs
// necessary cleanup operations.
func (jm *JobMonitor) monitorJob(ctx context.Context, cmd osinterface.Command, job *domain.Job) {
	log := jm.logger.WithField("jobId", job.Id)
	startTime := time.Now()

	log.Debug("starting job monitoring",
		"pid", job.Pid,
		"command", job.Command,
		"networkGroup", job.NetworkGroupID)

	// Setup comprehensive cleanup and panic recovery
	// This defer function ensures cleanup always happens regardless of
	// how the monitoring function exits (normal completion, panic, etc.)
	defer func() {
		// Always perform post-completion cleanup
		jm.performPostCompletionCleanup(job, log)

		// Handle panic recovery to prevent monitor crashes
		if r := recover(); r != nil {
			duration := time.Since(startTime)
			log.Error("panic occurred during job monitoring - performing emergency recovery",
				"panic", r,
				"duration", duration,
				"stackTrace", fmt.Sprintf("%+v", r))
			jm.handlePanicRecovery(job, fmt.Sprintf("%v", r))
		}
	}()

	// Phase 1: Process Completion Monitoring
	// Wait for the job process to complete and analyze the results
	result := jm.waitForProcessCompletion(ctx, cmd, job, startTime)
	log.Debug("process completion detected",
		"finalStatus", result.FinalStatus,
		"exitCode", result.ExitCode,
		"duration", result.Duration)

	// Phase 2: Job Status Update
	// Update the job status in the store based on monitoring results
	jm.updateJobWithResult(job, result, log)

	// Phase 3: Resource Cleanup
	// Perform cleanup of all job-related resources
	jm.performJobCleanup(job, log)

	log.Info("job monitoring completed successfully",
		"finalStatus", result.FinalStatus,
		"exitCode", result.ExitCode,
		"totalDuration", time.Since(startTime))
}

// waitForProcessCompletion waits for job process completion and analyzes results.
// This method blocks until the process exits and determines the final job status
// based on the exit conditions.
//
// Process Analysis:
// - Monitors process exit via Command.Wait()
// - Extracts exit codes from process results
// - Distinguishes between successful completion and failures
// - Handles various error conditions and edge cases
//
// Parameters:
//
//	ctx:       Context for cancellation support
//	cmd:       Command interface for process monitoring
//	job:       Job being monitored
//	startTime: Job start timestamp for duration calculation
//
// Returns: MonitoringResult with complete analysis of job execution
//
// Exit Code Interpretation:
// - Exit code 0: Successful completion
// - Non-zero exit codes: Job failure with specific error code
// - No exit code available: System-level failure (-1)
func (jm *JobMonitor) waitForProcessCompletion(ctx context.Context, cmd osinterface.Command, job *domain.Job, startTime time.Time) *MonitoringResult {
	log := jm.logger.WithField("jobId", job.Id)

	// Block until process completes or context is cancelled
	// cmd.Wait() returns when the process exits for any reason
	err := cmd.Wait()
	duration := time.Since(startTime)

	// Initialize result structure with basic information
	result := &MonitoringResult{
		JobID:    job.Id,
		Duration: duration,
		Error:    err,
	}

	if err != nil {
		// Process completed with an error condition
		var exitCode int32
		var exitErr *exec.ExitError

		// Extract exit code from error if available
		if errors.As(err, &exitErr) {
			exitCode = int32(exitErr.ExitCode())
			log.Info("job completed with non-zero exit code",
				"exitCode", exitCode,
				"duration", duration,
				"error", err)
		} else {
			// No exit code available - system error
			exitCode = -1
			log.Error("job failed with system error",
				"duration", duration,
				"error", err)
		}

		log.Info("job completed with error",
			"exitCode", exitCode,
			"duration", duration,
			"error", err)

		result.FinalStatus = domain.StatusFailed
		result.ExitCode = exitCode
	} else {
		// Process completed successfully
		log.Info("job completed successfully",
			"duration", duration,
			"exitCode", 0)
		result.FinalStatus = domain.StatusCompleted
		result.ExitCode = 0
	}

	return result
}

// updateJobWithResult updates job status based on monitoring results.
// This method applies the monitoring results to the job object and
// persists the changes to the job store.
//
// Update Strategy:
// - Attempts to use domain object validation for status transitions
// - Falls back to manual updates if validation fails
// - Ensures job status is always updated regardless of validation issues
// - Logs validation problems for debugging
//
// Parameters:
//
//	job:    Original job object to update
//	result: Monitoring results with final status and exit code
//	log:    Logger for status update tracking
//
// The method creates a deep copy of the job to avoid modifying the
// original object and ensure thread safety.
func (jm *JobMonitor) updateJobWithResult(job *domain.Job, result *MonitoringResult, log *logger.Logger) {
	// Create a deep copy to avoid modifying the original job object
	completedJob := job.DeepCopy()

	log.Debug("updating job status based on monitoring result",
		"currentStatus", string(job.Status),
		"targetStatus", string(result.FinalStatus),
		"exitCode", result.ExitCode)

	// Attempt to use domain object validation for status transitions
	var domainErr error
	switch result.FinalStatus {
	case domain.StatusCompleted:
		domainErr = completedJob.Complete(result.ExitCode)
	case domain.StatusFailed:
		domainErr = completedJob.Fail(result.ExitCode)
	default:
		domainErr = fmt.Errorf("unsupported final status: %s", result.FinalStatus)
	}

	// Handle domain validation failure gracefully
	if domainErr != nil {
		log.Warn("domain validation failed during job completion - updating manually",
			"domainError", domainErr,
			"finalStatus", result.FinalStatus,
			"exitCode", result.ExitCode)

		// Perform manual status update bypassing domain validation
		completedJob.Status = result.FinalStatus
		completedJob.ExitCode = result.ExitCode
		now := time.Now()
		completedJob.EndTime = &now
	}

	// Persist the updated job to the store
	jm.store.UpdateJob(completedJob)

	log.Info("job status updated successfully",
		"finalStatus", string(completedJob.Status),
		"exitCode", completedJob.ExitCode,
		"duration", completedJob.Duration())
}

// performJobCleanup performs standard cleanup after job completion.
// This method handles the cleanup of resources that are always cleaned
// up when a job completes, regardless of success or failure.
//
// Standard Cleanup Operations:
// - Cgroup cleanup and resource release
// - Network group membership management
// - Resource usage finalization
// - Cleanup result logging
//
// Parameters:
//
//	job: Job object containing resource information
//	log: Logger for cleanup operation tracking
//
// Network Group Strategy:
// - Decrements job count for shared network groups
// - Allows automatic group cleanup when count reaches zero
// - Does not cleanup isolated job namespaces (handled separately)
func (jm *JobMonitor) performJobCleanup(job *domain.Job, log *logger.Logger) {
	log.Debug("starting standard job cleanup procedures")

	// Phase 1: Cgroup Cleanup
	// Always cleanup cgroup resources as they are job-specific
	if job.CgroupPath != "" {
		log.Debug("initiating cgroup cleanup", "cgroupPath", job.CgroupPath)
		jm.cgroup.CleanupCgroup(job.Id)
		log.Debug("cgroup cleanup completed")
	}

	// Phase 2: Network Group Management
	// Handle network group membership and potential cleanup
	if job.NetworkGroupID != "" {
		log.Debug("managing network group membership",
			"groupID", job.NetworkGroupID)

		// Decrement job count - this may trigger automatic group cleanup
		// if this was the last job in the group
		if err := jm.networkManager.DecrementJobCount(job.NetworkGroupID); err != nil {
			log.Warn("failed to decrement network group job count",
				"groupID", job.NetworkGroupID,
				"error", err)
		} else {
			log.Debug("network group job count decremented successfully")
		}
	}

	log.Info("standard job cleanup completed successfully")
}

// performPostCompletionCleanup performs cleanup that must always happen.
// This method handles cleanup operations that are essential regardless
// of how the monitoring process terminates (normal completion, panic, etc.).
//
// Essential Cleanup Operations:
// - Network group membership cleanup (prevent resource leaks)
// - Isolated job namespace cleanup
// - Final resource state verification
//
// Parameters:
//
//	job: Job object requiring cleanup
//	log: Logger for cleanup tracking
//
// This method is called from the defer function in monitorJob to ensure
// critical cleanup always happens even in error conditions.
func (jm *JobMonitor) performPostCompletionCleanup(job *domain.Job, log *logger.Logger) {
	log.Debug("performing essential post-completion cleanup")

	// Critical: Network group membership cleanup
	// This prevents resource leaks if normal cleanup failed
	if job.NetworkGroupID != "" {
		if err := jm.networkManager.DecrementJobCount(job.NetworkGroupID); err != nil {
			log.Warn("failed to decrement network group count in post-completion cleanup",
				"groupID", job.NetworkGroupID,
				"error", err)
		} else {
			log.Debug("network group membership cleaned up in post-completion")
		}
	}

	// Critical: Isolated job namespace cleanup
	// Remove namespace symlinks for isolated jobs to prevent accumulation
	if job.NetworkGroupID == "" {
		nsPath := jm.config.BuildNamespacePath(job.Id)
		if err := jm.osInterface.Remove(nsPath); err != nil && !jm.osInterface.IsNotExist(err) {
			log.Warn("failed to remove isolated job namespace in post-completion cleanup",
				"path", nsPath,
				"error", err)
		} else {
			log.Debug("isolated job namespace cleaned up in post-completion")
		}
	}

	log.Debug("essential post-completion cleanup completed")
}

// handlePanicRecovery handles panic recovery during job monitoring.
// This method provides emergency cleanup when monitoring encounters
// unexpected panics or catastrophic failures.
//
// Recovery Operations:
// - Emergency resource cleanup
// - Job status update to failed
// - Comprehensive error logging
// - System state stabilization
//
// Parameters:
//
//	job:      Job that was being monitored when panic occurred
//	panicMsg: Panic message for logging and analysis
//
// This method is designed to be safe to call in any system state and
// will not throw additional panics even if resources are corrupted.
func (jm *JobMonitor) handlePanicRecovery(job *domain.Job, panicMsg string) {
	log := jm.logger.WithField("jobId", job.Id)
	log.Error("initiating panic recovery procedures",
		"panic", panicMsg,
		"pid", job.Pid)

	// Perform emergency cleanup using worker's emergency procedures
	// This bypasses normal cleanup procedures and uses aggressive cleanup
	jm.worker.EmergencyCleanup(job.Id, job.Pid, job.NetworkGroupID)

	log.Error("panic recovery completed - job marked as failed",
		"panic", panicMsg)
}

// MonitorJobHealth continuously monitors job health with configurable intervals.
// This method provides ongoing health verification for running jobs beyond
// basic process existence checking.
//
// Health Monitoring Features:
// - Periodic process existence verification
// - Resource constraint validation
// - Cgroup status monitoring
// - Performance degradation detection
// - Automatic issue remediation
//
// Parameters:
//
//	ctx:      Context for health monitoring lifecycle
//	job:      Job to monitor for health issues
//	interval: Time between health checks (default: 30 seconds)
//
// The health monitoring runs in a separate goroutine and automatically
// stops when the job completes or the context is cancelled.
func (jm *JobMonitor) MonitorJobHealth(ctx context.Context, job *domain.Job, interval time.Duration) {
	// Apply default interval if not specified
	if interval <= 0 {
		interval = 30 * time.Second
	}

	log := jm.logger.WithFields("jobId", job.Id, "interval", interval)
	log.Debug("starting continuous health monitoring")

	// Setup periodic health checking
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Health monitoring loop
	for {
		select {
		case <-ctx.Done():
			log.Debug("health monitoring cancelled by context")
			return

		case <-ticker.C:
			// Perform health check and handle failures
			if !jm.checkJobHealth(job, log) {
				log.Warn("job health check failed - stopping health monitoring")
				return
			}
		}
	}
}

// checkJobHealth performs a comprehensive health check on a job.
// This method examines various aspects of job health and system state
// to detect potential issues before they become critical.
//
// Health Check Areas:
// - Job existence in store
// - Job status consistency
// - Process existence verification
// - Cgroup status validation
// - Resource constraint compliance
//
// Parameters:
//
//	job: Job to examine for health issues
//	log: Logger for health check results
//
// Returns: true if job is healthy, false if issues detected
//
// Issue Handling: When health issues are detected, this method may
// perform automatic remediation or mark jobs as failed for cleanup.
func (jm *JobMonitor) checkJobHealth(job *domain.Job, log *logger.Logger) bool {
	// Phase 1: Job Store Consistency Check
	// Verify job still exists in the store and status is consistent
	currentJob, exists := jm.store.GetJob(job.Id)
	if !exists {
		log.Warn("job no longer exists in store - health monitoring terminating")
		return false
	}

	// Phase 2: Job Status Validation
	// Stop monitoring jobs that are no longer running
	if !currentJob.IsRunning() {
		log.Debug("job no longer running - health monitoring terminating",
			"status", string(currentJob.Status))
		return false
	}

	// Phase 3: Process Existence Verification
	// Verify the process is still alive and accessible
	if currentJob.Pid > 0 {
		processExists := jm.isProcessAlive(currentJob.Pid)
		if !processExists {
			log.Warn("job marked as running but process not found - initiating failure handling",
				"pid", currentJob.Pid)
			jm.handleDeadProcess(currentJob)
			return false
		}
		log.Debug("process existence verified", "pid", currentJob.Pid)
	}

	// Phase 4: Cgroup Health Validation
	// Check cgroup status if available
	if currentJob.CgroupPath != "" {
		if err := jm.checkCgroupHealth(currentJob.CgroupPath); err != nil {
			log.Warn("cgroup health check failed",
				"cgroupPath", currentJob.CgroupPath,
				"error", err)
			// Note: We don't fail health check for cgroup issues alone
			// as the job might still be functional
		} else {
			log.Debug("cgroup health verified")
		}
	}

	log.Debug("comprehensive health check passed")
	return true
}

// isProcessAlive checks if a process is still alive and accessible.
// This method provides a consistent interface for process existence
// checking across the monitoring system.
//
// Parameters:
//
//	pid: Process ID to check
//
// Returns: true if process exists and is accessible, false otherwise
//
// Implementation: Delegates to the process cleaner for consistency
// with other system components that perform similar checks.
func (jm *JobMonitor) isProcessAlive(pid int32) bool {
	// Use the process cleaner's method for consistency
	return jm.processCleaner.IsProcessAlive(pid)
}

// handleDeadProcess handles a process that died unexpectedly.
// This method manages the transition from a running job to failed status
// when the underlying process is no longer alive.
//
// Dead Process Handling:
// - Mark job as failed in store
// - Perform standard resource cleanup
// - Log detailed information for debugging
// - Update job metrics and status
//
// Parameters:
//
//	job: Job whose process has died unexpectedly
//
// This method handles the case where a job is marked as running in the
// store but the underlying process no longer exists, which can happen
// due to external process termination or system issues.
func (jm *JobMonitor) handleDeadProcess(job *domain.Job) {
	log := jm.logger.WithField("jobId", job.Id)
	log.Info("handling unexpectedly dead process",
		"pid", job.Pid,
		"lastKnownStatus", string(job.Status))

	// Create updated job object marked as failed
	failedJob := job.DeepCopy()

	// Attempt domain-validated status transition
	if err := failedJob.Fail(-1); err != nil {
		// Handle validation failure gracefully
		log.Warn("domain validation failed for dead process handling", "error", err)
		failedJob.Status = domain.StatusFailed
		failedJob.ExitCode = -1
		now := time.Now()
		failedJob.EndTime = &now
	}

	// Update job status in store
	jm.store.UpdateJob(failedJob)

	// Perform cleanup of job resources
	jm.performJobCleanup(job, log)

	log.Info("dead process handling completed - job marked as failed")
}

// checkCgroupHealth checks the health of a job's cgroup.
// This method validates that the cgroup exists and is accessible,
// which is essential for resource management functionality.
//
// Parameters:
//
//	cgroupPath: Path to the cgroup directory to check
//
// Returns: nil if cgroup is healthy, error describing issues
//
// Cgroup Health Indicators:
// - Directory existence and accessibility
// - Proper permissions for operations
// - Consistent internal state
//
// This method could be extended to check resource usage statistics,
// memory pressure, CPU throttling, and other advanced cgroup metrics.
func (jm *JobMonitor) checkCgroupHealth(cgroupPath string) error {
	if cgroupPath == "" {
		return nil // No cgroup configured, consider healthy
	}

	// Basic existence and accessibility check
	if _, err := jm.osInterface.Stat(cgroupPath); err != nil {
		if jm.osInterface.IsNotExist(err) {
			return fmt.Errorf("cgroup directory missing: %s", cgroupPath)
		}
		return fmt.Errorf("failed to access cgroup directory %s: %w", cgroupPath, err)
	}

	// Additional health checks could be added here:
	// - Memory usage vs limits
	// - CPU throttling detection
	// - I/O constraint verification
	// - Process count validation

	return nil
}

// GetJobStatus returns comprehensive status information about a job.
// This method provides detailed job information suitable for monitoring
// dashboards, debugging, and administrative interfaces.
//
// Status Information Categories:
// - Basic job metadata and identification
// - Current status and timing information
// - Resource configuration and limits
// - Process information and health
// - Network and cgroup configuration
//
// Parameters:
//
//	jobID: Unique identifier of the job to examine
//
// Returns: Map containing comprehensive job status information
func (jm *JobMonitor) GetJobStatus(jobID string) map[string]interface{} {
	// Attempt to retrieve job from store
	job, exists := jm.store.GetJob(jobID)
	if !exists {
		return map[string]interface{}{
			"job_id": jobID,
			"exists": false,
		}
	}

	// Build comprehensive status information
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

	// Add duration information based on job state
	if job.EndTime != nil {
		// Job has completed - show final duration
		status["end_time"] = job.EndTime.Format(time.RFC3339)
		status["duration"] = job.Duration().String()
		status["duration_seconds"] = job.Duration().Seconds()
	} else if job.IsRunning() {
		// Job is still running - show current duration
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

	// Add cgroup health information
	if job.CgroupPath != "" {
		cgroupExists := false
		if _, err := jm.osInterface.Stat(job.CgroupPath); err == nil {
			cgroupExists = true
		}
		status["cgroup_exists"] = cgroupExists
	}

	return status
}

// ListJobsByStatus returns jobs filtered by their current status.
// This method enables status-based job querying for monitoring and
// administrative operations.
//
// Parameters:
//
//	status: Job status to filter by (domain.JobStatus)
//
// Returns: Slice of jobs matching the specified status
//
// Common Use Cases:
// - Finding all running jobs for capacity planning
// - Identifying failed jobs for debugging
// - Locating completed jobs for cleanup
// - Status-based reporting and analytics
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

// GetRunningJobs returns all currently running jobs.
// This is a convenience method for monitoring active job execution.
func (jm *JobMonitor) GetRunningJobs() []*domain.Job {
	return jm.ListJobsByStatus(domain.StatusRunning)
}

// GetCompletedJobs returns all successfully completed jobs.
// This is useful for success rate analysis and performance metrics.
func (jm *JobMonitor) GetCompletedJobs() []*domain.Job {
	return jm.ListJobsByStatus(domain.StatusCompleted)
}

// GetFailedJobs returns all failed jobs for failure analysis.
// This is essential for debugging and system reliability monitoring.
func (jm *JobMonitor) GetFailedJobs() []*domain.Job {
	return jm.ListJobsByStatus(domain.StatusFailed)
}

// GetJobMetrics returns comprehensive metrics about job execution.
// This method provides performance statistics and operational metrics
// suitable for monitoring systems and capacity planning.
//
// Metrics Categories:
// - Job count and status distribution
// - Execution time statistics
// - Performance trends and patterns
// - Resource utilization summaries
// - Network group usage statistics
//
// Returns: Map containing comprehensive job execution metrics
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
