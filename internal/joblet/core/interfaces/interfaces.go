package interfaces

import (
	"context"

	"github.com/ehsaniara/joblet/internal/joblet/domain"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

// TelematicsMonitor defines the interface for eBPF-based job telematics monitoring.
// This interface allows the joblet to track process execution and network connections
// for monitored jobs without creating import cycles.
type TelematicsMonitor interface {
	// AddJob starts monitoring a job by its cgroup ID.
	// The cgroupID is the cgroup v2 ID used to filter eBPF events.
	AddJob(jobID string, cgroupID uint64) error

	// RemoveJob stops monitoring a job.
	RemoveJob(jobID string) error
}

//counterfeiter:generate . Joblet
type Joblet interface {
	// StartJob starts a job immediately or schedules it for future execution
	StartJob(ctx context.Context, req StartJobRequest) (*domain.Job, error)

	// StopJob stops a running job or removes a scheduled job
	StopJob(ctx context.Context, req StopJobRequest) error

	// DeleteJob completely removes a job including logs and metadata
	DeleteJob(ctx context.Context, req DeleteJobRequest) error

	// DeleteAllJobs removes all non-running jobs including logs and metadata
	DeleteAllJobs(ctx context.Context, req DeleteAllJobsRequest) (*DeleteAllJobsResponse, error)

	// ExecuteScheduledJob transitions a scheduled job to execution (used by scheduler)
	ExecuteScheduledJob(ctx context.Context, req ExecuteScheduledJobRequest) error

	// SetTelematicsMonitor sets the eBPF telematics monitor for job activity tracking
	SetTelematicsMonitor(monitor TelematicsMonitor)
}

// Import the adapters interfaces and use them directly
// This avoids duplication and ensures compatibility
