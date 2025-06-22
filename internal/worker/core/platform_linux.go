//go:build linux

package core

import (
	"context"
	"worker/internal/worker/core/interfaces"
	"worker/internal/worker/core/linux"
	"worker/internal/worker/domain"
	"worker/internal/worker/store"
)

// linuxWorker is a thin wrapper around the Linux worker
type linuxWorker struct {
	platformWorker interfaces.Worker
}

// NewLinuxWorker creates a Linux worker
func NewLinuxWorker(store store.Store) interfaces.Worker {
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
var _ interfaces.Worker = (*linuxWorker)(nil)
