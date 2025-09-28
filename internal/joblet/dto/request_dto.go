package dto

import "time"

// StartJobRequestDTO encapsulates parameters for starting a job
type StartJobRequestDTO struct {
	// Job identification
	Name string `json:"name,omitempty"` // Readable job name (for workflows)

	// Command and arguments
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`

	// Resource limits
	Resources ResourceLimitsDTO `json:"resources"`

	// File uploads
	Uploads []FileUploadDTO `json:"uploads,omitempty"`

	// Scheduling
	Schedule string `json:"schedule,omitempty"` // empty for immediate execution

	// Network configuration
	Network string `json:"network,omitempty"` // network name or empty for default

	// Volume mounts
	Volumes []string `json:"volumes,omitempty"` // volume names to mount

	// Runtime specification
	Runtime string `json:"runtime,omitempty"` // runtime specification (e.g., "python-3.11-ml")

	// Environment variables
	Environment       map[string]string `json:"environment,omitempty"`        // Regular environment variables
	SecretEnvironment map[string]string `json:"secret_environment,omitempty"` // Secret environment variables
}

// StopJobRequestDTO encapsulates parameters for stopping a job
type StopJobRequestDTO struct {
	JobID  string `json:"job_id"`
	Force  bool   `json:"force,omitempty"`  // Force kill if graceful stop fails
	Reason string `json:"reason,omitempty"` // Optional reason for audit
}

// DeleteJobRequestDTO encapsulates parameters for deleting a job
type DeleteJobRequestDTO struct {
	JobID  string `json:"job_id"`
	Reason string `json:"reason,omitempty"` // Optional reason for audit/logging
}

// FileUploadDTO represents a file upload for data transfer
type FileUploadDTO struct {
	Path        string `json:"path"`                   // Target path in job filesystem
	Content     []byte `json:"content"`                // File content
	Size        int64  `json:"size"`                   // Size in bytes
	Mode        uint32 `json:"mode,omitempty"`         // File permissions
	IsDirectory bool   `json:"is_directory,omitempty"` // Whether this is a directory
}

// JobResponseDTO represents the response after starting a job
type JobResponseDTO struct {
	JobUuid       string     `json:"job_uuid"`
	Status        string     `json:"status"`
	StartTime     time.Time  `json:"start_time"`
	ScheduledTime *time.Time `json:"scheduled_time,omitempty"`
	Message       string     `json:"message,omitempty"`
}

// OperationResponseDTO represents a generic operation response
type OperationResponseDTO struct {
	JobUuid string `json:"job_uuid,omitempty"`
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}
