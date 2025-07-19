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
func NewJoblet(store state.Store, cfg *config.Config) interfaces.Joblet {
	return &darwinJoblet{
		logger: logger.New().WithField("component", "darwin-joblet"),
		config: cfg,
	}
}

// StartJob provides basic job execution on macOS (for development/testing)
func (w *darwinJoblet) StartJob(ctx context.Context, command string, args []string, maxCPU, maxMemory, maxIOBPS int32, cpuCores string, uploads []domain.FileUpload, schedule string) (*domain.Job, error) {
	w.logger.Warn("Darwin joblet has limited functionality - jobs will not be isolated")
	if len(uploads) > 0 {
		w.logger.Warn("File uploads are not supported on Darwin", "uploadCount", len(uploads))
	}
	if schedule != "" {
		w.logger.Warn("Job scheduling is not supported on Darwin", "schedule", schedule)
	}
	return nil, fmt.Errorf("Darwin joblet not fully implemented - use Linux for production")
}

// StopJob stops a job on macOS (basic implementation)
func (w *darwinJoblet) StopJob(ctx context.Context, jobId string) error {
	w.logger.Warn("Darwin joblet stop job called")
	return fmt.Errorf("Darwin joblet not fully implemented")
}

// ExecuteScheduledJob executes a scheduled job on macOS (basic implementation)
func (w *darwinJoblet) ExecuteScheduledJob(ctx context.Context, job *domain.Job) error {
	w.logger.Warn("Darwin joblet scheduled execution called")
	return fmt.Errorf("Darwin joblet not fully implemented")
}

// Ensure darwinJoblet implements interfaces
var _ interfaces.Joblet = (*darwinJoblet)(nil)
