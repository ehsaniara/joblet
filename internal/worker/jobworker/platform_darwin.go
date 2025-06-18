//go:build darwin

package jobworker

import (
	"context"
	"fmt"
	"job-worker/internal/worker/domain"
	"job-worker/internal/worker/interfaces"
	"job-worker/pkg/logger"
	"sync/atomic"
	"time"
)

// darwinWorker provides a basic implementation for macOS development
type darwinWorker struct {
	store      interfaces.Store
	jobCounter int64
	logger     *logger.Logger
}

// NewLinuxWorker creates a basic macOS worker for development
func NewLinuxWorker(store interfaces.Store) interfaces.JobWorker {
	return &darwinWorker{
		store:  store,
		logger: logger.New().WithField("component", "darwin-worker"),
	}
}

// StartJob provides basic job execution on macOS (for development/testing)
func (w *darwinWorker) StartJob(ctx context.Context, command string, args []string, maxCPU, maxMemory, maxIOBPS int32) (*domain.Job, error) {
	jobID := w.getNextJobID()
	log := w.logger.WithFields("jobID", jobID, "command", command, "platform", "darwin")

	log.Warn("macOS worker: resource limits not enforced (development mode)")

	// Create basic job (no cgroups or namespaces on macOS)
	job := &domain.Job{
		Id:      jobID,
		Command: command,
		Args:    append([]string(nil), args...), // Copy args
		Limits: domain.ResourceLimits{
			MaxCPU:    maxCPU,    // Not enforced on macOS
			MaxMemory: maxMemory, // Not enforced on macOS
			MaxIOBPS:  maxIOBPS,  // Not enforced on macOS
		},
		Status:    domain.StatusInitializing,
		StartTime: time.Now(),
	}

	// Register job in store
	w.store.CreateNewJob(job)

	// For macOS development, simulate job execution
	completedJob := job.DeepCopy()
	if err := completedJob.MarkAsRunning(12345); err != nil { // Fake PID
		log.Warn("failed to mark job as running", "error", err)
		completedJob.Status = domain.StatusRunning
		completedJob.Pid = 12345
	}

	// Simulate job completion in background
	go func() {
		time.Sleep(500 * time.Millisecond) // Simulate brief execution
		finalJob := completedJob.DeepCopy()
		if err := finalJob.Complete(0); err != nil {
			log.Warn("failed to complete job", "error", err)
			finalJob.Status = domain.StatusCompleted
			finalJob.ExitCode = 0
			now := time.Now()
			finalJob.EndTime = &now
		}
		w.store.UpdateJob(finalJob)
		log.Info("macOS development job completed", "jobID", jobID)
	}()

	w.store.UpdateJob(completedJob)
	log.Info("macOS development job started", "jobID", jobID)
	return completedJob, nil
}

// StopJob stops a job on macOS (basic implementation)
func (w *darwinWorker) StopJob(ctx context.Context, jobId string) error {
	log := w.logger.WithFields("jobID", jobId, "platform", "darwin")
	log.Info("stopping macOS development job")

	job, exists := w.store.GetJob(jobId)
	if !exists {
		return fmt.Errorf("job not found: %s", jobId)
	}

	if job.IsCompleted() {
		return fmt.Errorf("job already completed: %s", jobId)
	}

	// Mark as stopped
	stoppedJob := job.DeepCopy()
	if err := stoppedJob.Stop(); err != nil {
		log.Warn("failed to stop job", "error", err)
		stoppedJob.Status = domain.StatusStopped
		stoppedJob.ExitCode = -1
		now := time.Now()
		stoppedJob.EndTime = &now
	}

	w.store.UpdateJob(stoppedJob)
	log.Info("macOS development job stopped", "jobID", jobId)
	return nil
}

func (w *darwinWorker) getNextJobID() string {
	nextID := atomic.AddInt64(&w.jobCounter, 1)
	return fmt.Sprintf("darwin-%d", nextID)
}

// Ensure darwinWorker implements interfaces
var _ interfaces.JobWorker = (*darwinWorker)(nil)
