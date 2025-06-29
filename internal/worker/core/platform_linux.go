//go:build linux

package core

import (
	"context"
	"worker/internal/worker/core/interfaces"
	"worker/internal/worker/core/linux"
	"worker/internal/worker/domain"
	"worker/internal/worker/state"
	"worker/pkg/config"
)

// linuxWorker is a thin wrapper around the Linux worker
type linuxWorker struct {
	platformWorker interfaces.Worker
}

// NewWorker creates a Linux worker
func NewWorker(store state.Store, cfg *config.Config) interfaces.Worker {
	return &linuxWorker{
		platformWorker: linux.NewPlatformWorker(store, cfg),
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
