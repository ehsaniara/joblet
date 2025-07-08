//go:build linux

package core

import (
	"context"
	"joblet/internal/joblet/core/interfaces"
	"joblet/internal/joblet/domain"
	"joblet/internal/joblet/state"
	"joblet/pkg/config"
)

// linuxJoblet is a thin wrapper around the Linux joblet
type linuxJoblet struct {
	platformJoblet interfaces.Joblet
}

// NewJoblet creates a Linux joblet
func NewJoblet(store state.Store, cfg *config.Config) interfaces.Joblet {
	return &linuxJoblet{
		platformJoblet: NewPlatformJoblet(store, cfg),
	}
}

// StartJob delegates to the platform joblet
func (w *linuxJoblet) StartJob(ctx context.Context, command string, args []string, maxCPU, maxMemory, maxIOBPS int32, cpuCores string) (*domain.Job, error) {
	return w.platformJoblet.StartJob(ctx, command, args, maxCPU, maxMemory, maxIOBPS, cpuCores)
}

// StopJob delegates to the platform joblet
func (w *linuxJoblet) StopJob(ctx context.Context, jobId string) error {
	return w.platformJoblet.StopJob(ctx, jobId)
}

// Ensure linuxJoblet implements interfaces
var _ interfaces.Joblet = (*linuxJoblet)(nil)
