//go:build linux

package worker

import (
	"context"
	"job-worker/internal/worker/domain"
	"job-worker/internal/worker/interfaces"
	"job-worker/internal/worker/platform/linux"
)

// linuxWorker is a thin wrapper around the simplified Linux worker
type linuxWorker struct {
	platformWorker interfaces.JobWorker
}

// NewPlatformWorker creates a simplified Linux worker (host networking only)
func NewPlatformWorker(store interfaces.Store) interfaces.JobWorker {
	return &linuxWorker{
		platformWorker: linux.NewPlatformWorker(store),
	}
}

// StartJob delegates to the platform worker (no network parameter)
func (w *linuxWorker) StartJob(ctx context.Context, command string, args []string, maxCPU, maxMemory, maxIOBPS int32) (*domain.Job, error) {
	return w.platformWorker.StartJob(ctx, command, args, maxCPU, maxMemory, maxIOBPS)
}

// StopJob delegates to the platform worker
func (w *linuxWorker) StopJob(ctx context.Context, jobId string) error {
	return w.platformWorker.StopJob(ctx, jobId)
}

// Ensure linuxWorker implements interfaces
var _ interfaces.JobWorker = (*linuxWorker)(nil)
var _ PlatformWorker = (*linuxWorker)(nil)
