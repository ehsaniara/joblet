//go:build linux

package core

import (
	"context"
	"fmt"

	"github.com/ehsaniara/joblet/internal/joblet/adapters"
	"github.com/ehsaniara/joblet/internal/joblet/core/interfaces"
	"github.com/ehsaniara/joblet/internal/joblet/domain"
	"github.com/ehsaniara/joblet/pkg/config"
)

// linuxJoblet is a thin wrapper around the Linux joblet
type linuxJoblet struct {
	platformJoblet interfaces.Joblet
}

// NewJoblet creates a Linux joblet
func NewJoblet(store adapters.JobStorer, metricsStore *adapters.MetricsStoreAdapter, cfg *config.Config, networkStoreAdapter adapters.NetworkStorer) interfaces.Joblet {
	return &linuxJoblet{
		platformJoblet: NewPlatformJoblet(store, metricsStore, cfg, networkStoreAdapter),
	}
}

// StartJob delegates to the platform joblet
func (w *linuxJoblet) StartJob(ctx context.Context, req interfaces.StartJobRequest) (*domain.Job, error) {
	// Add debug logging to see if this wrapper is called
	fmt.Printf("LINUX JOBLET WRAPPER StartJob called - network: %s, volumes: %v, runtime: %s\n", req.Network, req.Volumes, req.Runtime)
	return w.platformJoblet.StartJob(ctx, req)
}

// StopJob delegates to the platform joblet
func (w *linuxJoblet) StopJob(ctx context.Context, req interfaces.StopJobRequest) error {
	return w.platformJoblet.StopJob(ctx, req)
}

// DeleteJob delegates to the platform joblet
func (w *linuxJoblet) DeleteJob(ctx context.Context, req interfaces.DeleteJobRequest) error {
	return w.platformJoblet.DeleteJob(ctx, req)
}

// DeleteAllJobs delegates to the platform joblet
func (w *linuxJoblet) DeleteAllJobs(ctx context.Context, req interfaces.DeleteAllJobsRequest) (*interfaces.DeleteAllJobsResponse, error) {
	return w.platformJoblet.DeleteAllJobs(ctx, req)
}

// ExecuteScheduledJob delegates to the platform joblet
func (w *linuxJoblet) ExecuteScheduledJob(ctx context.Context, req interfaces.ExecuteScheduledJobRequest) error {
	return w.platformJoblet.ExecuteScheduledJob(ctx, req)
}

// Ensure linuxJoblet implements interfaces
var _ interfaces.Joblet = (*linuxJoblet)(nil)
