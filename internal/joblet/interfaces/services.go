package interfaces

import (
	"context"
	"joblet/internal/joblet/core/interfaces"
	"joblet/internal/joblet/domain"
)

// ServiceRegistry centralizes service dependencies to avoid circular imports
type ServiceRegistry interface {
	GetJobService() JobService
	GetVolumeService() VolumeService
	GetNetworkService() NetworkService
	GetMonitoringService() MonitoringService
	GetRuntimeService() RuntimeService
}

// JobServiceInterface defines core job operations for lifecycle management
type JobService interface {
	// StartJob initiates a new job execution with the given configuration
	StartJob(ctx context.Context, req interfaces.StartJobRequest) (*domain.Job, error)
	// StopJob terminates a running job gracefully or forcefully
	StopJob(ctx context.Context, req interfaces.StopJobRequest) error
	// DeleteJob removes a job and all associated data permanently
	DeleteJob(ctx context.Context, req interfaces.DeleteJobRequest) error
	// ExecuteScheduledJob transitions a scheduled job to active execution
	ExecuteScheduledJob(ctx context.Context, req interfaces.ExecuteScheduledJobRequest) error
	// GetJobStatus retrieves current status and metadata for a specific job
	GetJobStatus(ctx context.Context, jobID string) (*domain.Job, bool)
	// ListJobs returns all jobs visible to the current context
	ListJobs(ctx context.Context) []*domain.Job
}

// VolumeServiceInterface defines volume operations for persistent storage management
type VolumeService interface {
	// CreateVolume creates a new persistent volume with specified name and size
	CreateVolume(ctx context.Context, name string, size int64) error
	// DeleteVolume removes a volume and all its data permanently
	DeleteVolume(ctx context.Context, name string) error
	// ListVolumes returns all available volumes in the system
	ListVolumes(ctx context.Context) ([]string, error)
	// MountVolume attaches a volume to the specified mount path for job access
	MountVolume(ctx context.Context, volumeName, mountPath string) error
}

// NetworkServiceInterface defines network operations for job isolation and connectivity
type NetworkService interface {
	// CreateNetwork creates a new isolated network with the given configuration
	CreateNetwork(ctx context.Context, name string, config interface{}) error
	// DeleteNetwork removes a network and cleans up all associated resources
	DeleteNetwork(ctx context.Context, name string) error
	// ListNetworks returns all available networks in the system
	ListNetworks(ctx context.Context) ([]string, error)
	// AssignJobToNetwork connects a job to the specified network for communication
	AssignJobToNetwork(ctx context.Context, jobID, networkName string) error
}

// MonitoringServiceInterface defines monitoring operations for system and job observability
type MonitoringService interface {
	// CollectSystemMetrics gathers current system resource usage and health data
	CollectSystemMetrics(ctx context.Context) (map[string]interface{}, error)
	// GetJobMetrics retrieves performance and resource metrics for a specific job
	GetJobMetrics(ctx context.Context, jobID string) (map[string]interface{}, error)
	// StartMonitoring begins metric collection and monitoring for a job
	StartMonitoring(ctx context.Context, jobID string) error
	// StopMonitoring ends metric collection and cleanup monitoring resources
	StopMonitoring(ctx context.Context, jobID string) error
}

// RuntimeService defines runtime operations for execution environment management
type RuntimeService interface {
	// ListRuntimes returns all available runtime environments in the system
	ListRuntimes(ctx context.Context) ([]string, error)
	// InstallRuntime downloads and installs a new runtime environment by specification
	InstallRuntime(ctx context.Context, spec string) error
	// RemoveRuntime uninstalls a runtime environment and cleans up its files
	RemoveRuntime(ctx context.Context, spec string) error
	// ValidateRuntime checks if a runtime specification is valid and available
	ValidateRuntime(ctx context.Context, spec string) (bool, error)
}
