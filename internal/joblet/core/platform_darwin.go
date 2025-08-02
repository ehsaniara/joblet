//go:build darwin

package core

import (
	"context"
	"fmt"
	"joblet/internal/joblet/core/interfaces"
	"joblet/internal/joblet/domain"
	"joblet/internal/joblet/state"
	"joblet/pkg/config"
	"joblet/pkg/logger"
)

// darwinJoblet provides a basic implementation for macOS development
type darwinJoblet struct {
	logger *logger.Logger
	config *config.Config
}

// NewJoblet creates a Darwin joblet for development
func NewJoblet(store state.Store, cfg *config.Config, networkStore *state.NetworkStore) interfaces.Joblet {
	return &darwinJoblet{
		logger: logger.New().WithField("component", "darwin-joblet"),
		config: cfg,
	}
}

// StartJob provides basic job execution on macOS (for development/testing)
func (w *darwinJoblet) StartJob(ctx context.Context, req interfaces.StartJobRequest) (*domain.Job, error) {
	w.logger.Warn("Darwin joblet has limited functionality - jobs will not be isolated")
	if len(req.Uploads) > 0 {
		w.logger.Warn("File uploads are not supported on Darwin", "uploadCount", len(req.Uploads))
	}
	if req.Schedule != "" {
		w.logger.Warn("Job scheduling is not supported on Darwin", "schedule", req.Schedule)
	}
	return nil, fmt.Errorf("Darwin joblet not fully implemented - use Linux for production")
}

// StopJob stops a job on macOS (basic implementation)
func (w *darwinJoblet) StopJob(ctx context.Context, req interfaces.StopJobRequest) error {
	w.logger.Warn("Darwin joblet stop job called", "jobID", req.JobID)
	return fmt.Errorf("Darwin joblet not fully implemented")
}

// ExecuteScheduledJob executes a scheduled job on macOS (basic implementation)
func (w *darwinJoblet) ExecuteScheduledJob(ctx context.Context, req interfaces.ExecuteScheduledJobRequest) error {
	w.logger.Warn("Darwin joblet scheduled execution called", "jobID", req.Job.Id)
	return fmt.Errorf("Darwin joblet not fully implemented")
}

// Ensure darwinJoblet implements interfaces
var _ interfaces.Joblet = (*darwinJoblet)(nil)
