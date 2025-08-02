package interfaces

import (
	"joblet/internal/joblet/domain"
)

// StartJobRequest encapsulates all parameters needed to start a job
type StartJobRequest struct {
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

// ExecuteScheduledJobRequest for executing a scheduled job
type ExecuteScheduledJobRequest struct {
	Job *domain.Job
}
