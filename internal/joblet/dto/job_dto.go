package dto

import "time"

// JobDTO represents a job for data transfer between layers
type JobDTO struct {
	Uuid              string            `json:"uuid"`
	Name              string            `json:"name,omitempty"` // Readable job name for workflow jobs
	Command           string            `json:"command"`
	Args              []string          `json:"args,omitempty"`
	Status            string            `json:"status"`
	Pid               int32             `json:"pid,omitempty"`
	StartTime         time.Time         `json:"start_time"`
	EndTime           *time.Time        `json:"end_time,omitempty"`
	ExitCode          int32             `json:"exit_code,omitempty"`
	ScheduledTime     *time.Time        `json:"scheduled_time,omitempty"`
	Network           string            `json:"network,omitempty"`
	Volumes           []string          `json:"volumes,omitempty"`
	Runtime           string            `json:"runtime,omitempty"`
	Environment       map[string]string `json:"environment,omitempty"`
	SecretEnvironment map[string]string `json:"secret_environment,omitempty"` // Will be masked in transport

	// Resource limits as simple types for transport
	ResourceLimits ResourceLimitsDTO `json:"resource_limits"`
}

// ResourceLimitsDTO represents resource constraints for data transfer
type ResourceLimitsDTO struct {
	MaxCPU    int32  `json:"max_cpu"`    // CPU percentage (0 = unlimited)
	MaxMemory int32  `json:"max_memory"` // Memory in MB (0 = unlimited)
	MaxIOBPS  int64  `json:"max_io_bps"` // IO bandwidth in bytes/sec (0 = unlimited)
	CPUCores  string `json:"cpu_cores"`  // CPU core specification (empty = no restriction)
}

// JobStatusDTO represents job status information for responses
type JobStatusDTO struct {
	Uuid      string `json:"uuid"`
	Status    string `json:"status"`
	StartTime string `json:"start_time"` // ISO 8601 format
	EndTime   string `json:"end_time"`   // ISO 8601 format, empty if not finished
	Duration  string `json:"duration"`   // Human readable duration
	ExitCode  int32  `json:"exit_code"`
}

// JobListItemDTO represents a job in list responses
type JobListItemDTO struct {
	Uuid      string `json:"uuid"`
	Name      string `json:"name,omitempty"`
	Command   string `json:"command"`
	Status    string `json:"status"`
	StartTime string `json:"start_time"` // ISO 8601 format
	Duration  string `json:"duration"`   // Readable duration
	Network   string `json:"network,omitempty"`
	Runtime   string `json:"runtime,omitempty"`
}
