package domain

import (
	"errors"
	"strings"
	"time"

	"joblet/internal/joblet/domain/values"
)

// JobStatus represents the current state of a job
type JobStatus string

const (
	StatusPending      JobStatus = "PENDING"
	StatusRunning      JobStatus = "RUNNING"
	StatusCompleted    JobStatus = "COMPLETED"
	StatusFailed       JobStatus = "FAILED"
	StatusStopped      JobStatus = "STOPPED"
	StatusScheduled    JobStatus = "SCHEDULED"
	StatusInitializing JobStatus = "INITIALIZING"
	StatusCanceled     JobStatus = "CANCELED"
	StatusStopping     JobStatus = "STOPPING"
)

// Legacy constants for backwards compatibility
const (
	JobStatusRunning   = StatusRunning
	JobStatusCompleted = StatusCompleted
	JobStatusFailed    = StatusFailed
	JobStatusScheduled = StatusScheduled
	JobStatusStopping  = StatusStopping
)

var (
	// ErrInvalidCommand is returned when job command is empty
	ErrInvalidCommand = errors.New("job command cannot be empty")
)

// Job represents a job with value objects replacing primitive obsession.
//
// JOB NAMES FEATURE:
// The Name field provides human-readable identification for jobs within workflows.
// - For workflow jobs: Contains the job name from YAML (e.g., "setup-data", "process-data")
// - For individual jobs: Empty string (only Id is used for identification)
// This enables better workflow visibility and dependency tracking in CLI tools.
type Job struct {
	// Core identifiers
	Uuid string // Unique identifier for job tracking (kept as string for backward compatibility)
	Name string // Human-readable job name (workflow jobs only, empty for individual jobs)

	// Execution details
	Command string   // Executable command path
	Args    []string // Command line arguments
	Type    JobType  // Job type (standard or runtime-build)

	// Resource management
	Limits     ResourceLimits // CPU/memory/IO constraints using value objects
	CgroupPath string         // Filesystem path for resource limits (kept as string for backward compatibility)

	// State tracking
	Status JobStatus // Current execution state
	Pid    int32     // Process ID when running (kept as int32 for backward compatibility)

	// Timing
	StartTime     time.Time  // Job creation timestamp
	EndTime       *time.Time // Completion timestamp (nil if running)
	ScheduledTime *time.Time // When the job should start (nil for immediate execution)

	// Process result
	ExitCode int32 // Process exit status

	// Infrastructure
	Network string   // Network name (kept as string for backward compatibility)
	Volumes []string // Volume names to mount (kept as string slice for backward compatibility)
	Runtime string   // Runtime specification (kept as string for backward compatibility)

	// Workflow integration
	WorkflowUuid     string       // UUID of parent workflow (empty for individual jobs)
	WorkingDirectory string       // Execution directory path
	Uploads          []FileUpload // Files uploaded with the job
	Dependencies     []string     // Job names this job depends on (workflow jobs only)

	// Environment
	Environment       map[string]string // Environment variables (kept as map for backward compatibility)
	SecretEnvironment map[string]string // Secret environment variables (kept as map for backward compatibility)

	// Legacy fields for backward compatibility
	StartedAt   time.Time // Alias for StartTime (used by monitoring)
	CompletedAt time.Time // Populated when job completes

	// Value object accessors (for new code to use value objects)
	jobID       *values.JobID       // Cached value object for UUID
	cgroupPath  *values.CgroupPath  // Cached value object for CgroupPath
	processID   *values.ProcessID   // Cached value object for PID
	networkName *values.NetworkName // Cached value object for Network
	volumeNames *values.VolumeNames // Cached value object for Volumes
	runtimeSpec *values.RuntimeSpec // Cached value object for Runtime
	environment *values.Environment // Cached value object for Environment
	secretEnv   *values.Environment // Cached value object for SecretEnvironment
}

// IsRunning returns true if the job is currently running
func (j *Job) IsRunning() bool {
	return j.Status == StatusRunning
}

// IsCompleted returns true if the job has completed execution
func (j *Job) IsCompleted() bool {
	return j.Status == StatusCompleted || j.Status == StatusFailed || j.Status == StatusStopped
}

// IsScheduled returns true if the job is scheduled for future execution
func (j *Job) IsScheduled() bool {
	return j.Status == StatusScheduled
}

// IsRuntimeBuild returns true if this is a runtime build job
func (j *Job) IsRuntimeBuild() bool {
	return j.Type == JobTypeRuntimeBuild
}

// GetType returns the job type, defaulting to standard if not specified
func (j *Job) GetType() JobType {
	if j.Type == "" {
		return JobTypeStandard
	}
	return j.Type
}

// HasResourceLimits returns true if any resource limits are set
func (j *Job) HasResourceLimits() bool {
	return j.Limits.HasCPULimit() ||
		j.Limits.HasMemoryLimit() ||
		j.Limits.HasIOLimit() ||
		j.Limits.HasCoreRestriction()
}

// GetDuration returns the job execution duration
func (j *Job) GetDuration() time.Duration {
	if j.EndTime == nil {
		if j.IsRunning() {
			return time.Since(j.StartTime)
		}
		return 0
	}
	return j.EndTime.Sub(j.StartTime)
}

// GetScheduleDelay returns how long until the job is scheduled to run
func (j *Job) GetScheduleDelay() time.Duration {
	if j.ScheduledTime == nil || !j.IsScheduled() {
		return 0
	}
	delay := time.Until(*j.ScheduledTime)
	if delay < 0 {
		return 0 // Already past scheduled time
	}
	return delay
}

// Validate validates the job configuration
func (j *Job) Validate() error {
	if j.Command == "" {
		return ErrInvalidCommand
	}

	// Validate using primitive fields (value object validation happens on access)
	if strings.TrimSpace(j.Uuid) == "" {
		return errors.New("job ID cannot be empty")
	}

	// Network validation (basic)
	if j.Network != "" && j.Network != "isolated" {
		// Validate network name format if needed
		if len(j.Network) > 63 {
			return errors.New("network name too long")
		}
	}

	// Volume validation (basic)
	for _, volume := range j.Volumes {
		if strings.TrimSpace(volume) == "" {
			return errors.New("volume name cannot be empty")
		}
	}

	return nil
}

// Value object accessors (for new code to use value objects)

// JobIDValue returns the job ID as a value object, creating it if needed
func (j *Job) JobIDValue() values.JobID {
	if j.jobID == nil {
		// Create and cache the value object
		if jobID, err := values.NewJobID(j.Uuid); err == nil {
			j.jobID = &jobID
		} else {
			// Return empty JobID if invalid
			j.jobID = &values.JobID{}
		}
	}
	return *j.jobID
}

// SetJobIDValue sets the job ID from a value object and updates the string field
func (j *Job) SetJobIDValue(jobID values.JobID) {
	j.Uuid = jobID.String()
	j.jobID = &jobID
}

// CgroupPathValue returns the cgroup path as a value object, creating it if needed
func (j *Job) CgroupPathValue() values.CgroupPath {
	if j.cgroupPath == nil && j.CgroupPath != "" {
		// Create and cache the value object
		if cgroupPath, err := values.NewCgroupPath(j.CgroupPath); err == nil {
			j.cgroupPath = &cgroupPath
		}
	}
	if j.cgroupPath != nil {
		return *j.cgroupPath
	}
	return values.CgroupPath{} // Return empty if not set
}

// SetCgroupPathValue sets the cgroup path from a value object and updates the string field
func (j *Job) SetCgroupPathValue(path values.CgroupPath) {
	j.CgroupPath = path.String()
	j.cgroupPath = &path
}

// ProcessIDValue returns the process ID as a value object, creating it if needed
func (j *Job) ProcessIDValue() *values.ProcessID {
	if j.processID == nil && j.Pid > 0 {
		// Create and cache the value object
		if processID, err := values.NewProcessID(j.Pid); err == nil {
			j.processID = &processID
		}
	}
	return j.processID
}

// SetProcessIDValue sets the process ID from a value object and updates the int32 field
func (j *Job) SetProcessIDValue(pid *values.ProcessID) {
	if pid == nil {
		j.Pid = 0
		j.processID = nil
	} else {
		j.Pid = pid.Value()
		j.processID = pid
	}
}

// NetworkNameValue returns the network name as a value object, creating it if needed
func (j *Job) NetworkNameValue() values.NetworkName {
	if j.networkName == nil && j.Network != "" {
		// Create and cache the value object
		if networkName, err := values.NewNetworkName(j.Network); err == nil {
			j.networkName = &networkName
		}
	}
	if j.networkName != nil {
		return *j.networkName
	}
	return values.NetworkName{} // Return empty if not set
}

// SetNetworkNameValue sets the network name from a value object and updates the string field
func (j *Job) SetNetworkNameValue(network values.NetworkName) {
	j.Network = network.String()
	j.networkName = &network
}

// VolumeNamesValue returns the volume names as a value object, creating it if needed
func (j *Job) VolumeNamesValue() values.VolumeNames {
	if j.volumeNames == nil {
		// Create and cache the value object
		if volumeNames, err := values.NewVolumeNames(j.Volumes); err == nil {
			j.volumeNames = &volumeNames
		} else {
			// Return empty if invalid
			empty := values.VolumeNames{}
			j.volumeNames = &empty
		}
	}
	return *j.volumeNames
}

// SetVolumeNamesValue sets the volume names from a value object and updates the string slice field
func (j *Job) SetVolumeNamesValue(volumes values.VolumeNames) {
	j.Volumes = volumes.ToStringSlice()
	j.volumeNames = &volumes
}

// RuntimeSpecValue returns the runtime spec as a value object, creating it if needed
func (j *Job) RuntimeSpecValue() values.RuntimeSpec {
	if j.runtimeSpec == nil {
		// Create and cache the value object
		if runtimeSpec, err := values.NewRuntimeSpec(j.Runtime); err == nil {
			j.runtimeSpec = &runtimeSpec
		} else {
			// Return empty if invalid
			empty := values.RuntimeSpec{}
			j.runtimeSpec = &empty
		}
	}
	return *j.runtimeSpec
}

// SetRuntimeSpecValue sets the runtime spec from a value object and updates the string field
func (j *Job) SetRuntimeSpecValue(runtime values.RuntimeSpec) {
	j.Runtime = runtime.String()
	j.runtimeSpec = &runtime
}

// EnvironmentValue returns the environment as a value object, creating it if needed
func (j *Job) EnvironmentValue() values.Environment {
	if j.environment == nil {
		// Create and cache the value object
		env := values.NewEnvironment(j.Environment)
		j.environment = &env
	}
	return *j.environment
}

// SetEnvironmentValue sets the environment from a value object and updates the map field
func (j *Job) SetEnvironmentValue(env values.Environment) {
	j.Environment = env.ToMap()
	j.environment = &env
}

// SecretEnvironmentValue returns the secret environment as a value object, creating it if needed
func (j *Job) SecretEnvironmentValue() values.Environment {
	if j.secretEnv == nil {
		// Create and cache the value object
		env := values.NewEnvironment(j.SecretEnvironment)
		j.secretEnv = &env
	}
	return *j.secretEnv
}

// SetSecretEnvironmentValue sets the secret environment from a value object and updates the map field
func (j *Job) SetSecretEnvironmentValue(env values.Environment) {
	j.SecretEnvironment = env.ToMap()
	j.secretEnv = &env
}

// Copy creates a copy of the job (alias for DeepCopy for backward compatibility)
func (j *Job) Copy() *Job {
	return j.DeepCopy()
}

// DeepCopy creates a deep copy of the job including the Name field.
//
// RESPONSIBILITY:
// - Creates an independent copy of the Job struct with all fields properly duplicated
// - Handles deep copying of slices, maps, and pointer fields to prevent shared references
// - Ensures the Name field (job names feature) is properly preserved in the copy
// - Provides thread-safe job duplication for concurrent operations
//
// JOB NAMES INTEGRATION:
// - Preserves the Name field which contains human-readable job names for workflow jobs
// - Maintains job identity information for proper workflow status display
// - Ensures copied jobs retain their workflow context and naming information
//
// DEEP COPY BEHAVIOR:
// - Primitive fields: Direct value copy (Id, Name, Command, Status, etc.)
// - Slices: Creates new slices with copied elements (Args, Volumes)
// - Maps: Creates new maps with all key-value pairs copied (Environment, SecretEnvironment)
// - Pointers: Creates new pointer instances with copied values (EndTime, ScheduledTime)
// - Value objects: Safe to copy directly (ResourceLimits uses value semantics)
//
// THREAD SAFETY:
// - Safe for concurrent use as it creates completely independent copies
// - No shared state between original and copied job instances
// - Prevents race conditions when jobs are accessed from multiple goroutines
//
// USAGE:
// Called by job store adapters and workflow managers when job isolation is required.
func (j *Job) DeepCopy() *Job {
	if j == nil {
		return nil
	}

	jobCopy := &Job{
		// Core identifiers
		Uuid: j.Uuid,
		Name: j.Name,

		// Execution details
		Command: j.Command,
		Args:    make([]string, len(j.Args)),

		// Resource management
		Limits:     j.Limits,
		CgroupPath: j.CgroupPath,

		// State tracking
		Status: j.Status,
		Pid:    j.Pid,

		// Timing
		StartTime: j.StartTime,
		ExitCode:  j.ExitCode,

		// Infrastructure
		Network: j.Network,
		Volumes: make([]string, len(j.Volumes)),
		Runtime: j.Runtime,

		// Environment
		Environment:       make(map[string]string),
		SecretEnvironment: make(map[string]string),

		// Legacy fields
		StartedAt:   j.StartedAt,
		CompletedAt: j.CompletedAt,
	}

	// Copy slices
	copy(jobCopy.Args, j.Args)
	copy(jobCopy.Volumes, j.Volumes)

	// Deep copy environment maps
	for k, v := range j.Environment {
		jobCopy.Environment[k] = v
	}
	for k, v := range j.SecretEnvironment {
		jobCopy.SecretEnvironment[k] = v
	}

	// Copy pointers
	if j.EndTime != nil {
		endTime := *j.EndTime
		jobCopy.EndTime = &endTime
	}
	if j.ScheduledTime != nil {
		scheduledTime := *j.ScheduledTime
		jobCopy.ScheduledTime = &scheduledTime
	}

	return jobCopy
}

// MaskedSecretEnvironment returns secret environment with masked values for DTO conversion
func (j *Job) MaskedSecretEnvironment() map[string]string {
	if len(j.SecretEnvironment) == 0 {
		return j.SecretEnvironment
	}

	masked := make(map[string]string)
	for key := range j.SecretEnvironment {
		masked[key] = "***"
	}
	return masked
}

// ResourceLimitsToDTO converts resource limits to primitive values for DTO
func (j *Job) ResourceLimitsToDTO() (int32, int32, int64, string) {
	return j.Limits.CPU.Value(),
		j.Limits.Memory.Megabytes(),
		j.Limits.IOBandwidth.BytesPerSecond(),
		j.Limits.CPUCores.String()
}

// FormattedStartTime returns formatted start time for DTO conversion
func (j *Job) FormattedStartTime() string {
	return j.StartTime.Format("2006-01-02T15:04:05Z07:00")
}

// FormattedEndTime returns formatted end time for DTO conversion
func (j *Job) FormattedEndTime() string {
	if j.EndTime != nil {
		return j.EndTime.Format("2006-01-02T15:04:05Z07:00")
	}
	return ""
}

// FormattedDuration returns formatted duration for DTO conversion
func (j *Job) FormattedDuration() string {
	if j.EndTime != nil {
		return j.EndTime.Sub(j.StartTime).String()
	} else if j.IsRunning() {
		return time.Since(j.StartTime).String()
	}
	return ""
}

// FormattedScheduledTime returns formatted scheduled time for conversion
func (j *Job) FormattedScheduledTime() string {
	if j.ScheduledTime != nil {
		return j.ScheduledTime.Format("2006-01-02T15:04:05Z07:00")
	}
	return ""
}
