//go:build linux

package linux

// Package linux provides the main Linux platform worker implementation for job execution.
// This package orchestrates all components needed for secure, isolated job execution
// on Linux systems using namespaces, cgroups, and process management.
//
// Thread Safety: All public methods are thread-safe and can be called
// concurrently from multiple goroutines.
import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"job-worker/internal/worker/domain"
	"job-worker/internal/worker/interfaces"
	"job-worker/internal/worker/platform/linux/network"
	"job-worker/internal/worker/platform/linux/process"
	"job-worker/internal/worker/resource"
	"job-worker/pkg/logger"
	osinterface "job-worker/pkg/os"
)

// jobCounter provides atomic job ID generation across all worker instances.
var jobCounter int64

// Worker is the main Linux platform worker implementation.
type Worker struct {
	// Core job management components
	starter *JobStarter // Handles job initialization and launching
	stopper *JobStopper // Handles job termination and cleanup
	monitor *JobMonitor // Handles job monitoring and health checks

	// Essential dependencies
	store interfaces.Store // Job data persistence and state management

	// Platform-specific components
	networkManager   *network.Manager    // Network namespace and isolation management
	processLauncher  *process.Launcher   // Low-level process creation and management
	processCleaner   *process.Cleaner    // Process termination and cleanup
	processValidator *process.Validator  // Input validation and security checks
	cgroup           interfaces.Resource // Cgroup resource management

	// Configuration and logging
	config *Config        // Platform-specific configuration
	logger *logger.Logger // Structured logging with context
}

// Dependencies holds all dependencies needed by the Linux worker.
type Dependencies struct {
	// Core interfaces
	Store  interfaces.Store    // Job state management (required)
	Cgroup interfaces.Resource // Resource management (required)

	// OS abstraction interfaces (enable testing with mocks)
	CmdFactory    osinterface.CommandFactory   // Command creation
	Syscall       osinterface.SyscallInterface // System call interface
	OsInterface   osinterface.OsInterface      // OS operation interface
	ExecInterface osinterface.ExecInterface    // Executable resolution

	// Platform-specific components
	NetworkManager   *network.Manager   // Network isolation management
	ProcessLauncher  *process.Launcher  // Process lifecycle management
	ProcessCleaner   *process.Cleaner   // Process cleanup operations
	ProcessValidator *process.Validator // Security and validation

	// Configuration
	Config *Config // Platform-specific settings
}

// NewWorker creates a new Linux worker with all dependencies.
func NewWorker(deps *Dependencies) *Worker {
	// Create logger with component identification for debugging
	workerLogger := logger.New().WithField("component", "linux-worker")

	// Initialize main worker structure with provided dependencies
	worker := &Worker{
		// Core state management
		store: deps.Store,

		// Network and process management components
		networkManager:   deps.NetworkManager,
		processLauncher:  deps.ProcessLauncher,
		processCleaner:   deps.ProcessCleaner,
		processValidator: deps.ProcessValidator,

		// Resource management
		cgroup: deps.Cgroup,

		// Configuration and logging
		config: deps.Config,
		logger: workerLogger,
	}

	// Initialize sub-components with shared worker reference
	worker.starter = NewJobStarter(worker, deps)
	worker.stopper = NewJobStopper(worker, deps)
	worker.monitor = NewJobMonitor(worker, deps)

	workerLogger.Info("Linux worker initialized successfully",
		"maxConcurrentJobs", deps.Config.MaxConcurrentJobs,
		"cgroupsBaseDir", deps.Config.CgroupsBaseDir)

	return worker
}

// NewPlatformWorker creates a Linux-specific worker with default dependencies.
func NewPlatformWorker(store interfaces.Store) interfaces.JobWorker {
	// Initialize configuration with production defaults
	config := DefaultConfig()

	// Create real OS interfaces for production use
	osInterface := &osinterface.DefaultOs{}
	syscallInterface := &osinterface.DefaultSyscall{}
	cmdFactory := &osinterface.DefaultCommandFactory{}
	execInterface := &osinterface.DefaultExec{}

	// Setup network management components
	networkPaths := network.NewDefaultPaths()
	namespaceOps := network.NewNamespaceOperations(
		syscallInterface,
		osInterface,
		networkPaths,
	)

	// Initialize subnet allocator for network group isolation
	subnetAllocator := network.NewSubnetAllocator(network.BaseNetwork)

	// Assemble network manager with all dependencies
	networkDeps := &network.Dependencies{
		SubnetAllocator: subnetAllocator,
		NamespaceOps:    namespaceOps,
		Syscall:         syscallInterface,
		OsInterface:     osInterface,
		Paths:           networkPaths,
	}
	networkManager := network.NewManager(networkDeps)

	// Create process management components
	processValidator := process.NewValidator(osInterface, execInterface)
	processLauncher := process.NewLauncher(
		cmdFactory,
		syscallInterface,
		osInterface,
		processValidator,
	)
	processCleaner := process.NewCleaner(
		syscallInterface,
		osInterface,
		processValidator,
	)

	// Assemble all dependencies for worker creation
	deps := &Dependencies{
		Store:            store,
		Cgroup:           resource.New(),
		CmdFactory:       cmdFactory,
		Syscall:          syscallInterface,
		OsInterface:      osInterface,
		ExecInterface:    execInterface,
		NetworkManager:   networkManager,
		ProcessLauncher:  processLauncher,
		ProcessCleaner:   processCleaner,
		ProcessValidator: processValidator,
		Config:           config,
	}

	return NewWorker(deps)
}

// StartJob implements the JobWorker interface for job creation.
func (w *Worker) StartJob(ctx context.Context, command string, args []string, maxCPU, maxMemory, maxIOBPS int32, networkGroupID string) (*domain.Job, error) {
	// Create structured request for internal processing
	req := &StartJobRequest{
		Command:        command,
		Args:           args,
		MaxCPU:         maxCPU,
		MaxMemory:      maxMemory,
		MaxIOBPS:       maxIOBPS,
		NetworkGroupID: networkGroupID,
	}

	// Delegate to specialized job starter component
	return w.starter.StartJob(ctx, req)
}

// StopJob implements the JobWorker interface for job termination.
func (w *Worker) StopJob(ctx context.Context, jobID string) error {
	// Create structured request with default settings
	req := &StopJobRequest{
		JobID:           jobID,
		GracefulTimeout: GracefulShutdownTimeout,
		ForceKill:       false,
	}

	// Delegate to specialized job stopper component
	return w.stopper.StopJob(ctx, req)
}

// GetNextJobID generates a unique job ID for new jobs.
func (w *Worker) GetNextJobID() string {
	nextID := atomic.AddInt64(&jobCounter, 1)
	return fmt.Sprintf("%d", nextID)
}

// Getter methods for accessing worker components
func (w *Worker) GetStore() interfaces.Store {
	return w.store
}

func (w *Worker) GetNetworkManager() *network.Manager {
	return w.networkManager
}

func (w *Worker) GetProcessLauncher() *process.Launcher {
	return w.processLauncher
}

func (w *Worker) GetProcessCleaner() *process.Cleaner {
	return w.processCleaner
}

func (w *Worker) GetProcessValidator() *process.Validator {
	return w.processValidator
}

func (w *Worker) GetCgroup() interfaces.Resource {
	return w.cgroup
}

func (w *Worker) GetConfig() *Config {
	return w.config
}

func (w *Worker) GetLogger() *logger.Logger {
	return w.logger
}

// StartMonitoring initiates monitoring for a running job.
func (w *Worker) StartMonitoring(ctx context.Context, cmd osinterface.Command, job *domain.Job) {
	w.monitor.StartMonitoring(ctx, cmd, job)
}

// GetJobInitPath returns the path to the job-init binary.
func (w *Worker) GetJobInitPath() (string, error) {
	return w.starter.getJobInitPath()
}

// CreateJobDomain creates a job domain object with proper initialization.
func (w *Worker) CreateJobDomain(req *StartJobRequest, jobID string, resolvedCommand string) *domain.Job {
	// Create resource limits structure from request parameters
	limits := domain.ResourceLimits{
		MaxCPU:    req.MaxCPU,
		MaxMemory: req.MaxMemory,
		MaxIOBPS:  req.MaxIOBPS,
	}

	// Build complete job domain object with all required fields
	return &domain.Job{
		Id:             jobID,
		Command:        resolvedCommand,
		Args:           append([]string(nil), req.Args...), // Deep copy args
		Limits:         limits,
		Status:         domain.StatusInitializing,
		CgroupPath:     w.config.BuildCgroupPath(jobID),
		StartTime:      time.Now(),
		NetworkGroupID: req.NetworkGroupID,
		// CRITICAL: Use unique interface name per job
		InterfaceName: fmt.Sprintf("job-%s", jobID),
	}
}

// ValidateAndResolveCommand performs security validation and path resolution.
func (w *Worker) ValidateAndResolveCommand(command string) (string, error) {
	// Perform security validation first
	if err := w.processValidator.ValidateCommand(command); err != nil {
		return "", fmt.Errorf("command validation failed: %w", err)
	}

	// Resolve command to full path for secure execution
	resolvedCommand, err := w.processValidator.ResolveCommand(command)
	if err != nil {
		return "", fmt.Errorf("command resolution failed: %w", err)
	}

	return resolvedCommand, nil
}

// CleanupJobResources performs comprehensive cleanup of job resources.
func (w *Worker) CleanupJobResources(jobID string, networkGroupID string, isNewNetworkGroup bool) {
	log := w.logger.WithField("jobID", jobID)
	log.Debug("cleaning up job resources",
		"networkGroup", networkGroupID,
		"isNewGroup", isNewNetworkGroup)

	// Always cleanup cgroup resources
	if w.cgroup != nil {
		w.cgroup.CleanupCgroup(jobID)
		log.Debug("cgroup cleanup initiated")
	}

	// Handle network cleanup based on group membership
	if networkGroupID != "" {
		if isNewNetworkGroup {
			// For groups that failed during creation, clean up the entire group
			if err := w.networkManager.CleanupGroup(networkGroupID); err != nil {
				log.Warn("failed to cleanup network group",
					"groupID", networkGroupID,
					"error", err)
			} else {
				log.Debug("network group cleaned up", "groupID", networkGroupID)
			}
		} else {
			// For existing groups, just decrement the job count
			if err := w.networkManager.DecrementJobCount(networkGroupID); err != nil {
				log.Warn("failed to decrement network group count",
					"groupID", networkGroupID,
					"error", err)
			} else {
				log.Debug("network group job count decremented", "groupID", networkGroupID)
			}
		}
	}

	log.Info("job resource cleanup completed")
}

// EmergencyCleanup performs emergency cleanup when normal cleanup fails.
func (w *Worker) EmergencyCleanup(jobID string, pid int32, networkGroupID string) {
	log := w.logger.WithFields("jobID", jobID, "pid", pid)
	log.Warn("performing emergency cleanup - normal cleanup failed")

	// Determine namespace configuration for cleanup
	namespacePath := ""
	isIsolated := networkGroupID == ""

	if isIsolated {
		namespacePath = w.config.BuildNamespacePath(jobID)
	}

	// Use process cleaner for emergency cleanup
	w.processCleaner.EmergencyCleanup(
		jobID,
		pid,
		w.cgroup,
		w.networkManager,
		namespacePath,
		isIsolated,
	)

	// Handle network group cleanup if applicable
	if networkGroupID != "" {
		if err := w.networkManager.DecrementJobCount(networkGroupID); err != nil {
			log.Warn("failed to decrement network group count during emergency cleanup",
				"groupID", networkGroupID,
				"error", err)
		}
	}

	// Update job status to failed in store
	if job, exists := w.store.GetJob(jobID); exists {
		failedJob := job.DeepCopy()

		if failErr := failedJob.Fail(-1); failErr != nil {
			log.Warn("domain validation failed during emergency cleanup",
				"error", failErr)
			failedJob.Status = domain.StatusFailed
			failedJob.ExitCode = -1
			now := time.Now()
			failedJob.EndTime = &now
		}

		w.store.UpdateJob(failedJob)
		log.Info("job marked as failed during emergency cleanup")
	}

	log.Warn("emergency cleanup completed")
}

// GetStats returns comprehensive worker statistics for monitoring.
func (w *Worker) GetStats() map[string]interface{} {
	stats := map[string]interface{}{
		"current_job_counter": atomic.LoadInt64(&jobCounter),
		"component":           "linux-worker",
	}

	// Add network manager statistics if available
	if w.networkManager != nil {
		networkStats := w.networkManager.GetStats()
		for k, v := range networkStats {
			stats["network_"+k] = v
		}
	}

	// Add job store statistics
	if jobs := w.store.ListJobs(); jobs != nil {
		statusCounts := make(map[string]int)
		for _, job := range jobs {
			statusCounts[string(job.Status)]++
		}
		stats["jobs_by_status"] = statusCounts
		stats["total_jobs"] = len(jobs)
	}

	return stats
}

// Shutdown gracefully shuts down the worker and all running jobs.
func (w *Worker) Shutdown(ctx context.Context) error {
	w.logger.Info("shutting down Linux worker")

	var errors []error

	// Stop all running jobs gracefully
	jobs := w.store.ListJobs()
	for _, job := range jobs {
		if job.IsRunning() {
			w.logger.Debug("stopping running job during shutdown", "jobID", job.Id)

			if err := w.StopJob(ctx, job.Id); err != nil {
				w.logger.Warn("failed to stop job during shutdown", "jobID", job.Id, "error", err)
				errors = append(errors, fmt.Errorf("failed to stop job %s: %w", job.Id, err))
			}
		}
	}

	// Shutdown network manager
	if w.networkManager != nil {
		if err := w.networkManager.Shutdown(); err != nil {
			w.logger.Warn("network manager shutdown failed", "error", err)
			errors = append(errors, fmt.Errorf("network manager shutdown failed: %w", err))
		}
	}

	// Report shutdown completion status
	if len(errors) > 0 {
		return fmt.Errorf("worker shutdown completed with %d errors (first: %w)", len(errors), errors[0])
	}

	w.logger.Info("Linux worker shutdown completed successfully")
	return nil
}

// Health check methods
func (w *Worker) IsHealthy() bool {
	if w.store == nil || w.networkManager == nil || w.processLauncher == nil {
		return false
	}
	return true
}

func (w *Worker) GetComponentStatus() map[string]string {
	status := map[string]string{
		"worker":           "healthy",
		"store":            "unknown",
		"network_manager":  "unknown",
		"process_launcher": "unknown",
		"cgroup":           "unknown",
	}

	if w.store != nil {
		status["store"] = "healthy"
	} else {
		status["store"] = "unhealthy"
	}

	if w.networkManager != nil {
		status["network_manager"] = "healthy"
	} else {
		status["network_manager"] = "unhealthy"
	}

	if w.processLauncher != nil {
		status["process_launcher"] = "healthy"
	} else {
		status["process_launcher"] = "unhealthy"
	}

	if w.cgroup != nil {
		status["cgroup"] = "healthy"
	} else {
		status["cgroup"] = "unhealthy"
	}

	return status
}
