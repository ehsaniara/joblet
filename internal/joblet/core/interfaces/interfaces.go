package interfaces

import (
	"context"
	"github.com/ehsaniara/joblet/internal/joblet/domain"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

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

	//SetExtraFiles(files []*os.File)
}

// Import the adapters interfaces and use them directly
// This avoids duplication and ensures compatibility
