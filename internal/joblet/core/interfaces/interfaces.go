package interfaces

import (
	"context"
	"joblet/internal/joblet/domain"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

//counterfeiter:generate . Joblet
type Joblet interface {
	// StartJob starts a job immediately or schedules it for future execution
	StartJob(ctx context.Context, req StartJobRequest) (*domain.Job, error)

	// StopJob stops a running job or removes a scheduled job
	StopJob(ctx context.Context, req StopJobRequest) error

	// ExecuteScheduledJob transitions a scheduled job to execution (used by scheduler)
	ExecuteScheduledJob(ctx context.Context, req ExecuteScheduledJobRequest) error

	//SetExtraFiles(files []*os.File)
}
