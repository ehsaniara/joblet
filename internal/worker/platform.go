package worker

import (
	"context"
	"job-worker/internal/worker/domain"
	"job-worker/internal/worker/interfaces"
)

// PlatformWorker defines the interface that both Linux and macOS implementations will provide
type PlatformWorker interface {
	StartJob(ctx context.Context, command string, args []string, maxCPU, maxMemory, maxIOBPS int32) (*domain.Job, error)
	StopJob(ctx context.Context, jobId string) error
}

// New creates a platform-specific worker implementation
// This function works on all platforms and calls the appropriate platform-specific constructor
func New(store interfaces.Store) interfaces.JobWorker {
	return NewPlatformWorker(store)
}
