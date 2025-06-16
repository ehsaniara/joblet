//go:build linux

package worker

import (
	"context"
	"job-worker/internal/worker/domain"
	"job-worker/internal/worker/interfaces"
	"job-worker/internal/worker/platform/linux"
)

// linuxWorker is a thin wrapper around the Linux platform worker
type linuxWorker struct {
	platformWorker interfaces.JobWorker
}

// NewPlatformWorker creates a Linux-specific worker implementation
func NewPlatformWorker(store interfaces.Store) interfaces.JobWorker {
	return &linuxWorker{
		platformWorker: linux.NewPlatformWorker(store),
	}
}

// StartJob delegates to the platform worker
func (w *linuxWorker) StartJob(ctx context.Context, command string, args []string, maxCPU, maxMemory, maxIOBPS int32, networkGroupID string) (*domain.Job, error) {
	return w.platformWorker.StartJob(ctx, command, args, maxCPU, maxMemory, maxIOBPS, networkGroupID)
}

// StopJob delegates to the platform worker
func (w *linuxWorker) StopJob(ctx context.Context, jobId string) error {
	return w.platformWorker.StopJob(ctx, jobId)
}

// Ensure linuxWorker implements both interfaces
var _ interfaces.JobWorker = (*linuxWorker)(nil)
var _ PlatformWorker = (*linuxWorker)(nil)
