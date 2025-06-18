//go:build linux

package jobworker

import (
	"context"
	"job-worker/internal/worker/domain"
	"job-worker/internal/worker/interfaces"
	"job-worker/internal/worker/jobworker/linux"
)

// linuxWorker is a thin wrapper around the Linux worker
type linuxWorker struct {
	platformWorker interfaces.JobWorker
}

// NewLinuxWorker creates a Linux worker
func NewLinuxWorker(store interfaces.Store) interfaces.JobWorker {
	return &linuxWorker{
		platformWorker: linux.NewPlatformWorker(store),
	}
}

// StartJob delegates to the platform worker
func (w *linuxWorker) StartJob(ctx context.Context, command string, args []string, maxCPU, maxMemory, maxIOBPS int32) (*domain.Job, error) {
	return w.platformWorker.StartJob(ctx, command, args, maxCPU, maxMemory, maxIOBPS)
}

// StopJob delegates to the platform worker
func (w *linuxWorker) StopJob(ctx context.Context, jobId string) error {
	return w.platformWorker.StopJob(ctx, jobId)
}

// Ensure linuxWorker implements interfaces
var _ interfaces.JobWorker = (*linuxWorker)(nil)
