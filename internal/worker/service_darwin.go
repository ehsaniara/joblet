//go:build darwin

package worker

import (
	"context"
	"fmt"
	"job-worker/internal/config"
	"job-worker/internal/worker/domain"
	"job-worker/internal/worker/interfaces"
	"job-worker/pkg/logger"
	"strconv"
	"sync/atomic"
	"time"
)

var darwinJobCounter int64

type darwinWorker struct {
	store  interfaces.Store
	logger *logger.Logger
}

// NewPlatformWorker creates a macOS-specific worker implementation
func NewPlatformWorker(store interfaces.Store) interfaces.JobWorker {
	return &darwinWorker{
		store:  store,
		logger: logger.WithField("component", "worker-darwin"),
	}
}

func (w *darwinWorker) StartJob(ctx context.Context, command string, args []string, maxCPU, maxMemory, maxIOBPS int32) (*domain.Job, error) {
	// Remove networkGroupID parameter and all network-related code
	jobId := strconv.FormatInt(atomic.AddInt64(&darwinJobCounter, 1), 10)
	jobLogger := w.logger.WithField("jobId", jobId)

	jobLogger.Info("starting mock job on macOS with host networking",
		"command", command,
		"args", args)

	// Apply defaults
	if maxCPU <= 0 {
		maxCPU = config.DefaultCPULimitPercent
	}
	if maxMemory <= 0 {
		maxMemory = config.DefaultMemoryLimitMB
	}
	if maxIOBPS <= 0 {
		maxIOBPS = config.DefaultIOBPS
	}

	// Create simplified job (no network fields)
	job := &domain.Job{
		Id:      jobId,
		Command: command,
		Args:    append([]string(nil), args...),
		Limits: domain.ResourceLimits{
			MaxCPU:    maxCPU,
			MaxMemory: maxMemory,
			MaxIOBPS:  maxIOBPS,
		},
		Status:     domain.StatusInitializing,
		CgroupPath: fmt.Sprintf("/mock/cgroup/job-%s", jobId),
		StartTime:  time.Now(),
		Pid:        12345 + int32(atomic.LoadInt64(&darwinJobCounter)),
	}

	w.store.CreateNewJob(job)

	// Mark as running
	runningJob := job.DeepCopy()
	if err := runningJob.MarkAsRunning(job.Pid); err != nil {
		runningJob.Status = domain.StatusRunning
		runningJob.Pid = job.Pid
	}
	w.store.UpdateJob(runningJob)

	// Simulate job completion
	go w.simulateJobCompletion(ctx, runningJob)

	jobLogger.Info("mock job started with host networking", "pid", job.Pid)
	return runningJob, nil
}

func (w *darwinWorker) StopJob(ctx context.Context, jobId string) error {
	log := w.logger.WithField("jobId", jobId)

	select {
	case <-ctx.Done():
		log.Warn("stop job cancelled by context", "error", ctx.Err())
		return ctx.Err()
	default:
	}

	log.Info("stop job request received (macOS mock)")

	job, exists := w.store.GetJob(jobId)
	if !exists {
		log.Warn("job not found for stop operation")
		return fmt.Errorf("job not found: %s", jobId)
	}

	if !job.IsRunning() {
		log.Warn("attempted to stop non-running job", "currentStatus", string(job.Status))
		return fmt.Errorf("job is not running: %s (current status: %s)", jobId, job.Status)
	}

	log.Info("mock stopping job on macOS", "pid", job.Pid, "status", string(job.Status))

	// Mock stopping the job
	stoppedJob := job.DeepCopy()
	if stopErr := stoppedJob.Stop(); stopErr != nil {
		log.Warn("domain validation failed for stop", "domainError", stopErr)
		stoppedJob.Status = domain.StatusStopped
		stoppedJob.ExitCode = 0
		now := time.Now()
		stoppedJob.EndTime = &now
	} else {
		stoppedJob.ExitCode = 0
	}

	w.store.UpdateJob(stoppedJob)
	log.Info("mock job stopped on macOS", "jobId", jobId)

	return nil
}

// simulateJobCompletion simulates a job running for a few seconds then completing
func (w *darwinWorker) simulateJobCompletion(ctx context.Context, job *domain.Job) {
	log := w.logger.WithField("jobId", job.Id)

	// Simulate job running for 2-5 seconds
	duration := time.Duration(2+atomic.LoadInt64(&darwinJobCounter)%4) * time.Second

	select {
	case <-time.After(duration):
		// Job completed successfully
		log.Info("mock job completing", "duration", duration)

		completedJob := job.DeepCopy()
		if completeErr := completedJob.Complete(0); completeErr != nil {
			log.Warn("domain validation failed for job completion", "domainError", completeErr)
			completedJob.Status = domain.StatusCompleted
			completedJob.ExitCode = 0
			now := time.Now()
			completedJob.EndTime = &now
		}
		w.store.UpdateJob(completedJob)

		log.Info("mock job completed successfully on macOS", "jobId", job.Id, "duration", duration)

	case <-ctx.Done():
		// Context cancelled
		log.Info("mock job cancelled by context", "jobId", job.Id)
		return
	}
}
