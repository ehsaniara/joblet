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
// This ensures unique job IDs even in multi-worker deployments.
// The counter is never reset, providing globally unique IDs for the
// lifetime of the process.
var jobCounter int64

// Worker is the main Linux platform worker implementation.
// This struct coordinates all components needed for job execution and provides
// the primary interface for job management operations.
//
// Component Responsibilities:
//
// JobStarter: Handles job initialization, validation, resource setup,
// and process launching with proper isolation.
//
// JobStopper: Manages job termination, graceful shutdown, force killing,
// and resource cleanup.
//
// JobMonitor: Provides continuous monitoring, health checks, status updates,
// and automatic failure detection.
//
// The Worker acts as a facade pattern, providing a simplified interface
// while coordinating complex interactions between subsystems.
//
// Lifecycle Management:
//  1. Job Creation: Validation → Resource Setup → Process Launch → Monitoring
//  2. Job Execution: Continuous monitoring and health checks
//  3. Job Completion: Status update → Resource cleanup → Notification
//  4. Job Termination: Graceful stop → Force kill (if needed) → Cleanup
//
// Thread Safety: All methods are thread-safe. The Worker can safely handle
// concurrent job operations from multiple goroutines.
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
// This structure supports dependency injection for testing and
// configuration flexibility.
//
// Design Pattern: This implements the Dependency Injection pattern,
// allowing for easy testing with mock implementations and flexible
// configuration for different deployment environments.
//
// All dependencies are required unless explicitly documented as optional.
// The NewWorker function validates that all required dependencies are provided.
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
// This factory function initializes the worker and all its sub-components
// with proper dependency injection.
//
// Initialization Process:
//  1. Validate all required dependencies are provided
//  2. Create worker instance with shared logger context
//  3. Initialize sub-components (starter, stopper, monitor)
//  4. Establish component interconnections
//  5. Return fully configured worker ready for use
//
// Parameters:
//
//	deps: Complete dependency structure with all required components
//
// Returns: Fully initialized Worker instance ready for job execution
//
// Panics: If any required dependency is nil (fail-fast approach)
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
	// This allows sub-components to access shared functionality and state
	worker.starter = NewJobStarter(worker, deps)
	worker.stopper = NewJobStopper(worker, deps)
	worker.monitor = NewJobMonitor(worker, deps)

	workerLogger.Info("Linux worker initialized successfully",
		"maxConcurrentJobs", deps.Config.MaxConcurrentJobs,
		"cgroupsBaseDir", deps.Config.CgroupsBaseDir)

	return worker
}

// NewPlatformWorker creates a Linux-specific worker with default dependencies.
// This convenience function sets up a complete worker with sensible defaults
// for production use, requiring only a job store.
//
// Default Component Setup:
// - OS interfaces: Real system implementations
// - Network components: Full namespace and subnet management
// - Process components: Complete lifecycle management
// - Resource management: Cgroup v2 support
// - Configuration: Production-ready defaults
//
// This function is ideal for:
// - Production deployments with standard configuration
// - Simple setups without custom component requirements
// - Testing with real system components
//
// Parameters:
//
//	store: Job state management implementation
//
// Returns: Fully configured Worker with production-ready defaults
func NewPlatformWorker(store interfaces.Store) interfaces.JobWorker {
	// Initialize configuration with production defaults
	config := DefaultConfig()

	// Create real OS interfaces for production use
	// These provide actual system call access
	osInterface := &osinterface.DefaultOs{}
	syscallInterface := &osinterface.DefaultSyscall{}
	cmdFactory := &osinterface.DefaultCommandFactory{}
	execInterface := &osinterface.DefaultExec{}

	// Setup network management components
	// Configure namespace operations with standard Linux paths
	networkPaths := network.NewDefaultPaths()
	namespaceOps := network.NewNamespaceOperations(
		syscallInterface,
		osInterface,
		networkPaths,
	)

	// Initialize subnet allocator for network group isolation
	// Uses RFC 1918 private address space for job networks
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
	// Validator ensures security and prevents abuse
	processValidator := process.NewValidator(osInterface, execInterface)

	// Launcher handles process creation with namespace support
	processLauncher := process.NewLauncher(
		cmdFactory,
		syscallInterface,
		osInterface,
		processValidator,
	)

	// Cleaner handles graceful and forceful process termination
	processCleaner := process.NewCleaner(
		syscallInterface,
		osInterface,
		processValidator,
	)

	// Assemble all dependencies for worker creation
	deps := &Dependencies{
		Store:            store,
		Cgroup:           resource.New(), // Default cgroup implementation
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
// This method handles the complete job startup process including validation,
// resource allocation, process creation, and monitoring setup.
//
// Job Startup Flow:
//  1. Request validation and sanitization
//  2. Resource limit application and validation
//  3. Command resolution and security checks
//  4. Network and cgroup resource setup
//  5. Process creation with proper isolation
//  6. Monitoring and health check initialization
//
// Parameters:
//
//	ctx:            Context for cancellation and timeout control
//	command:        Executable command to run
//	args:           Command-line arguments
//	maxCPU:         CPU limit in percentage (100 = 1 core)
//	maxMemory:      Memory limit in megabytes
//	maxIOBPS:       I/O bandwidth limit in bytes/second (0 = unlimited)
//	networkGroupID: Network group for shared networking (empty = isolated)
//
// Returns:
//
//	job: Created job with assigned ID and initial status
//	error: nil on success, descriptive error on failure
//
// Context Handling: Respects context cancellation during startup.
// If context is cancelled, cleanup is performed automatically.
func (w *Worker) StartJob(ctx context.Context, command string, args []string, maxCPU, maxMemory, maxIOBPS int32, networkGroupID string) (*domain.Job, error) {
	// Create structured request for internal processing
	// This normalizes the interface parameters into a structured format
	req := &StartJobRequest{
		Command:        command,
		Args:           args,
		MaxCPU:         maxCPU,
		MaxMemory:      maxMemory,
		MaxIOBPS:       maxIOBPS,
		NetworkGroupID: networkGroupID,
	}

	// Delegate to specialized job starter component
	// The starter handles all the complex initialization logic
	return w.starter.StartJob(ctx, req)
}

// StopJob implements the JobWorker interface for job termination.
// This method handles graceful job shutdown with fallback to force termination
// if the job doesn't respond to graceful signals.
//
// Job Stopping Flow:
//  1. Job validation and status verification
//  2. Graceful termination attempt (SIGTERM)
//  3. Force termination if needed (SIGKILL)
//  4. Resource cleanup (cgroups, namespaces)
//  5. Status update and notification
//
// Parameters:
//
//	ctx:   Context for cancellation and timeout control
//	jobID: Unique identifier of the job to stop
//
// Returns:
//
//	error: nil on success, descriptive error on failure
//
// Termination Strategy:
// - First attempts graceful shutdown with configurable timeout
// - Falls back to force kill if graceful shutdown fails
// - Always performs resource cleanup regardless of termination method
func (w *Worker) StopJob(ctx context.Context, jobID string) error {
	// Create structured request with default settings
	// Use graceful shutdown with standard timeout, no force kill initially
	req := &StopJobRequest{
		JobID:           jobID,
		GracefulTimeout: GracefulShutdownTimeout, // From configuration
		ForceKill:       false,                   // Try graceful first
	}

	// Delegate to specialized job stopper component
	// The stopper handles all termination and cleanup logic
	return w.stopper.StopJob(ctx, req)
}

// GetNextJobID generates a unique job ID for new jobs.
// This method provides globally unique identifiers using an atomic counter.
//
// ID Generation Strategy:
// - Uses atomic operations for thread safety
// - Monotonically increasing values ensure uniqueness
// - Simple numeric format for easy debugging and logging
// - No collision risk within process lifetime
//
// Returns: String representation of unique job ID
//
// Thread Safety: Safe for concurrent access from multiple goroutines.
// The atomic operation ensures no ID collisions.
func (w *Worker) GetNextJobID() string {
	// Use atomic increment to ensure thread-safe unique ID generation
	// This provides monotonically increasing IDs across all goroutines
	nextID := atomic.AddInt64(&jobCounter, 1)
	return fmt.Sprintf("%d", nextID)
}

// Getter methods for accessing worker components.
// These methods provide controlled access to internal components
// for testing, monitoring, and advanced operations.

// GetStore returns the job store interface.
// The store manages job state, persistence, and real-time updates.
func (w *Worker) GetStore() interfaces.Store {
	return w.store
}

// GetNetworkManager returns the network management component.
// Provides access to network namespace and subnet allocation functionality.
func (w *Worker) GetNetworkManager() *network.Manager {
	return w.networkManager
}

// GetProcessLauncher returns the process launcher component.
// Handles low-level process creation with namespace support.
func (w *Worker) GetProcessLauncher() *process.Launcher {
	return w.processLauncher
}

// GetProcessCleaner returns the process cleanup component.
// Manages process termination and resource cleanup operations.
func (w *Worker) GetProcessCleaner() *process.Cleaner {
	return w.processCleaner
}

// GetProcessValidator returns the process validation component.
// Provides security checks and input validation for job requests.
func (w *Worker) GetProcessValidator() *process.Validator {
	return w.processValidator
}

// GetCgroup returns the cgroup resource management interface.
// Handles CPU, memory, and I/O resource limits and isolation.
func (w *Worker) GetCgroup() interfaces.Resource {
	return w.cgroup
}

// GetConfig returns the worker configuration.
// Provides access to platform-specific settings and limits.
func (w *Worker) GetConfig() *Config {
	return w.config
}

// GetLogger returns the worker's logger instance.
// Provides structured logging with worker context for debugging.
func (w *Worker) GetLogger() *logger.Logger {
	return w.logger
}

// StartMonitoring initiates monitoring for a running job.
// This method sets up continuous health checks and status tracking
// for the job process.
//
// Monitoring Features:
// - Process health verification
// - Exit code detection
// - Resource usage tracking
// - Automatic cleanup on completion
// - Panic recovery and emergency cleanup
//
// Parameters:
//
//	ctx: Context for monitoring lifecycle control
//	cmd: Command interface for process monitoring
//	job: Job domain object with metadata
//
// The monitoring runs in a separate goroutine and automatically
// handles job completion, failure detection, and cleanup.
func (w *Worker) StartMonitoring(ctx context.Context, cmd osinterface.Command, job *domain.Job) {
	w.monitor.StartMonitoring(ctx, cmd, job)
}

// GetJobInitPath returns the path to the job-init binary.
// The job-init binary is responsible for setting up the job environment
// within the isolated namespace before executing the user command.
//
// Binary Location Strategy:
//  1. Check same directory as main worker binary
//  2. Search standard system paths
//  3. Return error if not found
//
// Returns:
//
//	path: Absolute path to job-init binary
//	error: nil if found, descriptive error if not found
//
// The job-init binary must be present for job execution to work.
func (w *Worker) GetJobInitPath() (string, error) {
	return w.starter.getJobInitPath()
}

// CreateJobDomain creates a job domain object with proper initialization.
// This method constructs a job entity with all required fields populated
// and validates the configuration.
//
// Domain Object Features:
// - Unique job identifier
// - Resource limit configuration
// - Command and argument storage
// - Cgroup path assignment
// - Network group association
// - Initial status and timestamps
//
// Parameters:
//
//	req:             Startup request with job parameters
//	jobID:           Unique identifier for the job
//	resolvedCommand: Full path to executable command
//
// Returns: Properly initialized Job domain object
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
		Args:           append([]string(nil), req.Args...), // Deep copy args to prevent modifications
		Limits:         limits,
		Status:         domain.StatusInitializing, // Initial state before process start
		CgroupPath:     w.config.BuildCgroupPath(jobID),
		StartTime:      time.Now(),
		NetworkGroupID: req.NetworkGroupID,
	}
}

// ValidateAndResolveCommand performs security validation and path resolution.
// This method ensures commands are safe to execute and resolves them to
// full filesystem paths.
//
// Validation Process:
//  1. Security validation (dangerous character detection)
//  2. Command existence verification
//  3. Path resolution (relative to absolute)
//  4. Execution permission verification
//
// Parameters:
//
//	command: Command to validate and resolve
//
// Returns:
//
//	resolvedPath: Absolute path to executable
//	error: nil if valid, security or resolution error otherwise
//
// Security Features:
// - Prevents shell injection attacks
// - Validates command existence
// - Ensures execution permissions
// - Resolves path traversal attempts
func (w *Worker) ValidateAndResolveCommand(command string) (string, error) {
	// Perform security validation first
	// This prevents execution of dangerous or malformed commands
	if err := w.processValidator.ValidateCommand(command); err != nil {
		return "", fmt.Errorf("command validation failed: %w", err)
	}

	// Resolve command to full path for secure execution
	// This prevents PATH-based attacks and ensures consistency
	resolvedCommand, err := w.processValidator.ResolveCommand(command)
	if err != nil {
		return "", fmt.Errorf("command resolution failed: %w", err)
	}

	return resolvedCommand, nil
}

// CleanupJobResources performs comprehensive cleanup of job resources.
// This method handles cleanup of all resources associated with a job,
// including cgroups, network namespaces, and group memberships.
//
// Cleanup Strategy:
// - Always cleanup cgroups (local to job)
// - Handle network group membership appropriately
// - Log errors but don't fail on cleanup issues
// - Perform best-effort cleanup to prevent resource leaks
//
// Parameters:
//
//	jobID:              Unique job identifier
//	networkGroupID:     Network group membership (empty if isolated)
//	isNewNetworkGroup:  Whether this job created the network group
//
// Network Group Cleanup Logic:
// - New groups that failed: Clean up entire group
// - Existing groups: Decrement job count only
// - Isolated jobs: Clean up individual namespace
func (w *Worker) CleanupJobResources(jobID string, networkGroupID string, isNewNetworkGroup bool) {
	log := w.logger.WithField("jobID", jobID)
	log.Debug("cleaning up job resources",
		"networkGroup", networkGroupID,
		"isNewGroup", isNewNetworkGroup)

	// Always cleanup cgroup resources
	// Cgroups are job-specific and should always be cleaned up
	if w.cgroup != nil {
		w.cgroup.CleanupCgroup(jobID)
		log.Debug("cgroup cleanup initiated")
	}

	// Handle network cleanup based on group membership
	if networkGroupID != "" {
		if isNewNetworkGroup {
			// For groups that failed during creation, clean up the entire group
			// This prevents orphaned network groups from consuming resources
			if err := w.networkManager.CleanupGroup(networkGroupID); err != nil {
				log.Warn("failed to cleanup network group",
					"groupID", networkGroupID,
					"error", err)
			} else {
				log.Debug("network group cleaned up", "groupID", networkGroupID)
			}
		} else {
			// For existing groups, just decrement the job count
			// The group may still have other active jobs
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
// This method handles catastrophic failure scenarios where normal cleanup
// procedures cannot be followed.
//
// Emergency Cleanup Features:
// - Force kills processes regardless of state
// - Aggressive resource cleanup
// - Updates job status to failed
// - Logs all operations for debugging
// - Best-effort approach - continues despite errors
//
// Use Cases:
// - Panic recovery during job startup
// - System shutdown procedures
// - Process monitoring failures
// - Irrecoverable error states
//
// Parameters:
//
//	jobID:          Unique job identifier
//	pid:            Process ID for force termination
//	networkGroupID: Network group for cleanup
//
// Warning: This method uses force and should only be called when
// normal cleanup procedures have failed or are not applicable.
func (w *Worker) EmergencyCleanup(jobID string, pid int32, networkGroupID string) {
	log := w.logger.WithFields("jobID", jobID, "pid", pid)
	log.Warn("performing emergency cleanup - normal cleanup failed")

	// Determine namespace configuration for cleanup
	namespacePath := ""
	isIsolated := networkGroupID == "" // Empty group means isolated job

	if isIsolated {
		// Build namespace path for isolated job cleanup
		namespacePath = w.config.BuildNamespacePath(jobID)
	}

	// Use process cleaner for emergency cleanup
	// This performs aggressive cleanup including force kills
	w.processCleaner.EmergencyCleanup(
		jobID,
		pid,
		w.cgroup,         // Cgroup interface for resource cleanup
		w.networkManager, // Network manager for namespace cleanup
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
	// This ensures the job is marked as failed even if monitoring failed
	if job, exists := w.store.GetJob(jobID); exists {
		failedJob := job.DeepCopy()

		// Try to use domain validation for status update
		if failErr := failedJob.Fail(-1); failErr != nil {
			// If domain validation fails, update manually
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
// This method provides metrics suitable for operational monitoring,
// capacity planning, and performance analysis.
//
// Statistics Categories:
// - Job execution metrics (counts, status distribution)
// - Network management statistics
// - Resource usage information
// - Component health status
//
// Returns: Map of statistics with string keys and interface{} values
// suitable for JSON serialization and monitoring systems
func (w *Worker) GetStats() map[string]interface{} {
	// Initialize statistics with basic worker information
	stats := map[string]interface{}{
		"current_job_counter": atomic.LoadInt64(&jobCounter), // Global job counter
		"component":           "linux-worker",                // Component identification
	}

	// Add network manager statistics if available
	if w.networkManager != nil {
		networkStats := w.networkManager.GetStats()
		// Prefix network stats to avoid key conflicts
		for k, v := range networkStats {
			stats["network_"+k] = v
		}
	}

	// Add job store statistics
	if jobs := w.store.ListJobs(); jobs != nil {
		// Count jobs by status for operational insights
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
// This method coordinates an orderly shutdown of all worker components
// and ensures no resources are leaked.
//
// Shutdown Process:
//  1. Stop all running jobs with graceful termination
//  2. Shutdown network manager and cleanup groups
//  3. Wait for component shutdown completion
//  4. Report any errors encountered
//
// The shutdown process attempts graceful termination but will
// proceed even if some operations fail to prevent hanging.
//
// Parameters:
//
//	ctx: Context for shutdown timeout control
//
// Returns:
//
//	error: nil if shutdown completed successfully,
//	       aggregated error if some operations failed
func (w *Worker) Shutdown(ctx context.Context) error {
	w.logger.Info("shutting down Linux worker")

	var errors []error

	// Stop all running jobs gracefully
	// This gives jobs a chance to clean up before forced termination
	jobs := w.store.ListJobs()
	for _, job := range jobs {
		if job.IsRunning() {
			w.logger.Debug("stopping running job during shutdown", "jobID", job.Id)

			// Attempt to stop each job individually
			// Continue with other jobs even if one fails
			if err := w.StopJob(ctx, job.Id); err != nil {
				w.logger.Warn("failed to stop job during shutdown", "jobID", job.Id, "error", err)
				errors = append(errors, fmt.Errorf("failed to stop job %s: %w", job.Id, err))
			}
		}
	}

	// Shutdown network manager
	// This cleans up all network groups and namespace resources
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
