//go:build darwin

package core

import (
	"context"
	"fmt"
	"sync/atomic"
	"worker/internal/worker/core/interfaces"
	"worker/internal/worker/domain"
	"worker/internal/worker/store"
	"worker/pkg/logger"
)

// darwinWorker provides a basic implementation for macOS development
type darwinWorker struct {
	store      store.Store
	jobCounter int64
	logger     *logger.Logger
}

// NewLinuxWorker creates a basic macOS worker for development
func NewLinuxWorker(store store.Store) interfaces.Worker {
	return &darwinWorker{
		store:  store,
		logger: logger.New().WithField("component", "darwin-worker"),
	}
}

// StartJob provides basic job execution on macOS (for development/testing)
func (w *darwinWorker) StartJob(ctx context.Context, command string, args []string, maxCPU, maxMemory, maxIOBPS int32) (*domain.Job, error) {
	panic("implement me")
}

// StopJob stops a job on macOS (basic implementation)
func (w *darwinWorker) StopJob(ctx context.Context, jobId string) error {
	panic("implement me")
}

func (w *darwinWorker) getNextJobID() string {
	nextID := atomic.AddInt64(&w.jobCounter, 1)
	return fmt.Sprintf("darwin-%d", nextID)
}

// Ensure darwinWorker implements interfaces
var _ interfaces.Worker = (*darwinWorker)(nil)
