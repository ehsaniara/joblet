//go:build darwin

package core

import (
	"context"
	"fmt"
	"worker/internal/worker/core/interfaces"
	"worker/internal/worker/domain"
	"worker/internal/worker/state"
	"worker/pkg/config"
	"worker/pkg/logger"
)

// darwinWorker provides a basic implementation for macOS development
type darwinWorker struct {
	logger *logger.Logger
	config *config.Config
}

// NewWorker creates a Darwin worker for development (SAME FUNCTION NAME as Linux)
func NewWorker(store state.Store, cfg *config.Config) interfaces.Worker {
	return &darwinWorker{
		logger: logger.New().WithField("component", "darwin-worker"),
		config: cfg,
	}
}

// StartJob provides basic job execution on macOS (for development/testing)
func (w *darwinWorker) StartJob(ctx context.Context, command string, args []string, maxCPU, maxMemory, maxIOBPS int32) (*domain.Job, error) {
	w.logger.Warn("Darwin worker has limited functionality - jobs will not be isolated")
	return nil, fmt.Errorf("Darwin worker not fully implemented - use Linux for production")
}

// StopJob stops a job on macOS (basic implementation)
func (w *darwinWorker) StopJob(ctx context.Context, jobId string) error {
	w.logger.Warn("Darwin worker stop job called")
	return fmt.Errorf("Darwin worker not fully implemented")
}

// Ensure darwinWorker implements interfaces
var _ interfaces.Worker = (*darwinWorker)(nil)
