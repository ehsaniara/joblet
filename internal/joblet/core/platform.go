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
func NewJoblet(store state.Store, cfg *config.Config, networkStore *state.NetworkStore) interfaces.Joblet {
	return &linuxJoblet{
		platformJoblet: NewPlatformJoblet(store, cfg, networkStore),
	}
}

// StartJob delegates to the platform joblet
func (w *linuxJoblet) StartJob(ctx context.Context, req interfaces.StartJobRequest) (*domain.Job, error) {
	return w.platformJoblet.StartJob(ctx, req)
}

// StopJob delegates to the platform joblet
func (w *linuxJoblet) StopJob(ctx context.Context, req interfaces.StopJobRequest) error {
	return w.platformJoblet.StopJob(ctx, req)
}

// ExecuteScheduledJob delegates to the platform joblet
func (w *linuxJoblet) ExecuteScheduledJob(ctx context.Context, req interfaces.ExecuteScheduledJobRequest) error {
	return w.platformJoblet.ExecuteScheduledJob(ctx, req)
}

// Ensure linuxJoblet implements interfaces
var _ interfaces.Joblet = (*linuxJoblet)(nil)
