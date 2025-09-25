package interfaces

import (
	"joblet/internal/joblet/domain"
)

// StartJobRequest encapsulates all parameters needed to start a job
type StartJobRequest struct {
	// Job identification
	Name string // Human-readable job name (for workflows, empty for individual jobs)

	// Command and arguments
	Command string
	Args    []string

	// Resource limits
	Resources ResourceLimits

	// File uploads
	Uploads []domain.FileUpload

	// Scheduling
	Schedule string // empty for immediate execution

	// Network configuration
	Network string // network name or empty for default

	// Volume mounts
	Volumes []string // volume names to mount

	// Runtime specification
	Runtime string // runtime specification (e.g., "python-3.11-ml")

	// Environment variables
	Environment       map[string]string // Regular environment variables (visible in logs)
	SecretEnvironment map[string]string // Secret environment variables (hidden from logs)

	// Job type determines isolation level
	JobType domain.JobType // JobTypeStandard (production isolation) or JobTypeRuntimeBuild (builder chroot)

	// GPU resource requirements
	GPUCount    int32 // Number of GPUs requested (0 = no GPU)
	GPUMemoryMB int64 // Minimum GPU memory requirement in MB (0 = any)

	// Workflow integration
	WorkflowUuid     string   // UUID of parent workflow (empty for individual jobs)
	WorkingDirectory string   // Execution directory path
	Dependencies     []string // Job names this job depends on (workflow jobs only)
}

// ResourceLimits encapsulates resource constraints for a job
type ResourceLimits struct {
	MaxCPU    int32  // CPU percentage (0 = unlimited)
	MaxMemory int32  // Memory in MB (0 = unlimited)
	MaxIOBPS  int32  // IO bandwidth in bytes/sec (0 = unlimited)
	CPUCores  string // CPU core specification (empty = no restriction)
}

// StopJobRequest encapsulates parameters for stopping a job
type StopJobRequest struct {
	JobID  string
	Force  bool   // Force kill if graceful stop fails
	Reason string // Optional reason for audit
}

// DeleteJobRequest encapsulates parameters for deleting a job
type DeleteJobRequest struct {
	JobID  string
	Reason string // Optional reason for audit/logging
}

// DeleteAllJobsRequest encapsulates parameters for deleting all non-running jobs
type DeleteAllJobsRequest struct {
	Reason string // Optional reason for audit/logging
}

// DeleteAllJobsResponse contains the result of deleting all non-running jobs
type DeleteAllJobsResponse struct {
	DeletedCount int // Number of jobs deleted
	SkippedCount int // Number of jobs skipped (running/scheduled)
}

// ExecuteScheduledJobRequest for executing a scheduled job
type ExecuteScheduledJobRequest struct {
	Job *domain.Job
}
