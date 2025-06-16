//go:build linux

package linux

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

var jobCounter int64

// Worker is the main Linux platform worker implementation
type Worker struct {
	starter          *JobStarter
	stopper          *JobStopper
	monitor          *JobMonitor
	store            interfaces.Store
	networkManager   *network.Manager
	processLauncher  *process.Launcher
	processCleaner   *process.Cleaner
	processValidator *process.Validator
	cgroup           interfaces.Resource
	config           *Config
	logger           *logger.Logger
}

// Dependencies holds all dependencies needed by the Linux worker
type Dependencies struct {
	Store            interfaces.Store
	Cgroup           interfaces.Resource
	CmdFactory       osinterface.CommandFactory
	Syscall          osinterface.SyscallInterface
	OsInterface      osinterface.OsInterface
	ExecInterface    osinterface.ExecInterface
	NetworkManager   *network.Manager
	ProcessLauncher  *process.Launcher
	ProcessCleaner   *process.Cleaner
	ProcessValidator *process.Validator
	Config           *Config
}

// NewWorker creates a new Linux worker with all dependencies
func NewWorker(deps *Dependencies) *Worker {
	workerLogger := logger.New().WithField("component", "linux-worker")

	worker := &Worker{
		store:            deps.Store,
		networkManager:   deps.NetworkManager,
		processLauncher:  deps.ProcessLauncher,
		processCleaner:   deps.ProcessCleaner,
		processValidator: deps.ProcessValidator,
		cgroup:           deps.Cgroup,
		config:           deps.Config,
		logger:           workerLogger,
	}

	// Create sub-components with the worker's dependencies
	worker.starter = NewJobStarter(worker, deps)
	worker.stopper = NewJobStopper(worker, deps)
	worker.monitor = NewJobMonitor(worker, deps)

	return worker
}

// NewPlatformWorker creates a Linux-specific worker with default dependencies
func NewPlatformWorker(store interfaces.Store) interfaces.JobWorker {
	config := DefaultConfig()

	// Create OS interfaces
	osInterface := &osinterface.DefaultOs{}
	syscallInterface := &osinterface.DefaultSyscall{}
	cmdFactory := &osinterface.DefaultCommandFactory{}
	execInterface := &osinterface.DefaultExec{}

	// Create network components
	networkPaths := network.NewDefaultPaths()
	namespaceOps := network.NewNamespaceOperations(
		syscallInterface,
		osInterface,
		networkPaths,
	)
	subnetAllocator := network.NewSubnetAllocator(network.BaseNetwork)

	networkDeps := &network.Dependencies{
		SubnetAllocator: subnetAllocator,
		NamespaceOps:    namespaceOps,
		Syscall:         syscallInterface,
		OsInterface:     osInterface,
		Paths:           networkPaths,
	}
	networkManager := network.NewManager(networkDeps)

	// Create process components
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

	// Assemble dependencies
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

// StartJob implements the JobWorker interface
func (w *Worker) StartJob(ctx context.Context, command string, args []string, maxCPU, maxMemory, maxIOBPS int32, networkGroupID string) (*domain.Job, error) {
	req := &StartJobRequest{
		Command:        command,
		Args:           args,
		MaxCPU:         maxCPU,
		MaxMemory:      maxMemory,
		MaxIOBPS:       maxIOBPS,
		NetworkGroupID: networkGroupID,
	}

	return w.starter.StartJob(ctx, req)
}

// StopJob implements the JobWorker interface
func (w *Worker) StopJob(ctx context.Context, jobID string) error {
	req := &StopJobRequest{
		JobID:           jobID,
		GracefulTimeout: GracefulShutdownTimeout,
		ForceKill:       false,
	}

	return w.stopper.StopJob(ctx, req)
}

// GetNextJobID generates a unique job ID
func (w *Worker) GetNextJobID() string {
	return fmt.Sprintf("%d", atomic.AddInt64(&jobCounter, 1))
}

// GetStore returns the job store
func (w *Worker) GetStore() interfaces.Store {
	return w.store
}

// GetNetworkManager returns the network manager
func (w *Worker) GetNetworkManager() *network.Manager {
	return w.networkManager
}

// GetProcessLauncher returns the process launcher
func (w *Worker) GetProcessLauncher() *process.Launcher {
	return w.processLauncher
}

// GetProcessCleaner returns the process cleaner
func (w *Worker) GetProcessCleaner() *process.Cleaner {
	return w.processCleaner
}

// GetProcessValidator returns the process validator
func (w *Worker) GetProcessValidator() *process.Validator {
	return w.processValidator
}

// GetCgroup returns the cgroup interface
func (w *Worker) GetCgroup() interfaces.Resource {
	return w.cgroup
}

// GetConfig returns the worker configuration
func (w *Worker) GetConfig() *Config {
	return w.config
}

// GetLogger returns the worker logger
func (w *Worker) GetLogger() *logger.Logger {
	return w.logger
}

// StartMonitoring starts monitoring for a job
func (w *Worker) StartMonitoring(ctx context.Context, cmd osinterface.Command, job *domain.Job) {
	w.monitor.StartMonitoring(ctx, cmd, job)
}

// GetJobInitPath returns the path to the job-init binary
func (w *Worker) GetJobInitPath() (string, error) {
	return w.starter.getJobInitPath()
}

// CreateJobDomain creates a job domain object
func (w *Worker) CreateJobDomain(req *StartJobRequest, jobID string, resolvedCommand string) *domain.Job {
	limits := domain.ResourceLimits{
		MaxCPU:    req.MaxCPU,
		MaxMemory: req.MaxMemory,
		MaxIOBPS:  req.MaxIOBPS,
	}

	return &domain.Job{
		Id:             jobID,
		Command:        resolvedCommand,
		Args:           append([]string(nil), req.Args...),
		Limits:         limits,
		Status:         domain.StatusInitializing,
		CgroupPath:     w.config.BuildCgroupPath(jobID),
		StartTime:      time.Now(),
		NetworkGroupID: req.NetworkGroupID,
	}
}

// ValidateAndResolveCommand validates and resolves a command to its full path
func (w *Worker) ValidateAndResolveCommand(command string) (string, error) {
	// Validate command
	if err := w.processValidator.ValidateCommand(command); err != nil {
		return "", fmt.Errorf("command validation failed: %w", err)
	}

	// Resolve command path
	resolvedCommand, err := w.processValidator.ResolveCommand(command)
	if err != nil {
		return "", fmt.Errorf("command resolution failed: %w", err)
	}

	return resolvedCommand, nil
}

// CleanupJobResources performs cleanup of job resources
func (w *Worker) CleanupJobResources(jobID string, networkGroupID string, isNewNetworkGroup bool) {
	log := w.logger.WithField("jobID", jobID)
	log.Debug("cleaning up job resources")

	// Cleanup cgroup
	if w.cgroup != nil {
		w.cgroup.CleanupCgroup(jobID)
	}

	// Handle network cleanup
	if networkGroupID != "" {
		if isNewNetworkGroup {
			// For new groups that failed to start, cleanup the group
			if err := w.networkManager.CleanupGroup(networkGroupID); err != nil {
				log.Warn("failed to cleanup network group", "groupID", networkGroupID, "error", err)
			}
		} else {
			// For existing groups, just decrement the count
			if err := w.networkManager.DecrementJobCount(networkGroupID); err != nil {
				log.Warn("failed to decrement network group count", "groupID", networkGroupID, "error", err)
			}
		}
	}
}

// EmergencyCleanup performs emergency cleanup for a job
func (w *Worker) EmergencyCleanup(jobID string, pid int32, networkGroupID string) {
	log := w.logger.WithFields("jobID", jobID, "pid", pid)
	log.Warn("performing emergency cleanup")

	// Use the process cleaner for emergency cleanup
	namespacePath := ""
	isIsolated := networkGroupID == "" // If no network group, it's isolated

	if isIsolated {
		namespacePath = w.config.BuildNamespacePath(jobID)
	}

	w.processCleaner.EmergencyCleanup(
		jobID,
		pid,
		w.cgroup,
		w.networkManager,
		namespacePath,
		isIsolated,
	)

	// Handle network group cleanup
	if networkGroupID != "" {
		if err := w.networkManager.DecrementJobCount(networkGroupID); err != nil {
			log.Warn("failed to decrement network group count during emergency cleanup",
				"groupID", networkGroupID, "error", err)
		}
	}

	// Update job status to failed
	if job, exists := w.store.GetJob(jobID); exists {
		failedJob := job.DeepCopy()
		if failErr := failedJob.Fail(-1); failErr != nil {
			log.Warn("domain validation failed during emergency cleanup", "error", failErr)
			failedJob.Status = domain.StatusFailed
			failedJob.ExitCode = -1
			now := time.Now()
			failedJob.EndTime = &now
		}
		w.store.UpdateJob(failedJob)
		log.Info("job marked as failed during emergency cleanup")
	}
}

// GetStats returns worker statistics
func (w *Worker) GetStats() map[string]interface{} {
	stats := map[string]interface{}{
		"current_job_counter": atomic.LoadInt64(&jobCounter),
		"component":           "linux-worker",
	}

	// Add network manager stats
	if w.networkManager != nil {
		networkStats := w.networkManager.GetStats()
		for k, v := range networkStats {
			stats["network_"+k] = v
		}
	}

	// Add job store stats if available
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

// Shutdown gracefully shuts down the worker
func (w *Worker) Shutdown(ctx context.Context) error {
	w.logger.Info("shutting down Linux worker")

	var errors []error

	// Stop all running jobs
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

	if len(errors) > 0 {
		return fmt.Errorf("worker shutdown completed with %d errors (first: %w)", len(errors), errors[0])
	}

	w.logger.Info("Linux worker shutdown completed successfully")
	return nil
}

// Health check methods
func (w *Worker) IsHealthy() bool {
	// Basic health checks
	if w.store == nil || w.networkManager == nil || w.processLauncher == nil {
		return false
	}

	// Could add more sophisticated health checks here
	return true
}

// GetComponentStatus returns the status of each component
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
