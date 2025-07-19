package scheduler

import (
	"context"
	"joblet/internal/joblet/domain"
	"joblet/pkg/logger"
	"sync"
	"time"
)

// JobExecutor defines the interface for executing jobs
type JobExecutor interface {
	ExecuteScheduledJob(ctx context.Context, job *domain.Job) error
}

// Scheduler manages scheduled job execution using a priority queue and sleep-until-next strategy
type Scheduler struct {
	queue    *PriorityQueue
	executor JobExecutor
	logger   *logger.Logger

	// Control channels
	newJobSignal chan struct{}
	stopSignal   chan struct{}

	// State management
	running  bool
	runMutex sync.RWMutex

	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc
}

// New creates a new scheduler instance
func New(executor JobExecutor) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())

	return &Scheduler{
		queue:        NewPriorityQueue(),
		executor:     executor,
		logger:       logger.WithField("component", "scheduler"),
		newJobSignal: make(chan struct{}, 1), // Buffered to prevent blocking
		stopSignal:   make(chan struct{}),
		ctx:          ctx,
		cancel:       cancel,
	}
}

// Start begins the scheduler's main loop
func (s *Scheduler) Start() error {
	s.runMutex.Lock()
	if s.running {
		s.runMutex.Unlock()
		return nil // Already running
	}
	s.running = true
	s.runMutex.Unlock()

	s.logger.Info("scheduler starting")

	go s.run()

	s.logger.Info("scheduler started successfully")
	return nil
}

// Stop gracefully stops the scheduler
func (s *Scheduler) Stop() error {
	s.runMutex.Lock()
	if !s.running {
		s.runMutex.Unlock()
		return nil // Already stopped
	}
	s.running = false
	s.runMutex.Unlock()

	s.logger.Info("scheduler stopping")

	// Signal stop and cancel context
	close(s.stopSignal)
	s.cancel()

	s.logger.Info("scheduler stopped")
	return nil
}

// AddJob adds a new scheduled job to the queue
func (s *Scheduler) AddJob(job *domain.Job) error {
	if job.ScheduledTime == nil {
		return nil // Not a scheduled job, ignore
	}

	s.logger.Debug("adding scheduled job",
		"jobId", job.Id,
		"scheduledTime", job.ScheduledTime.Format(time.RFC3339),
		"command", job.Command)

	s.queue.Add(job)

	// Wake up the scheduler to recalculate sleep time
	select {
	case s.newJobSignal <- struct{}{}:
	default:
		// Channel is full, scheduler will wake up soon anyway
	}

	s.logger.Debug("scheduled job added successfully",
		"jobId", job.Id,
		"queueSize", s.queue.Size())

	return nil
}

// RemoveJob removes a job from the schedule queue
func (s *Scheduler) RemoveJob(jobID string) bool {
	removed := s.queue.Remove(jobID)
	if removed {
		s.logger.Debug("job removed from schedule", "jobId", jobID)
		// Wake up scheduler in case this was the next job
		select {
		case s.newJobSignal <- struct{}{}:
		default:
		}
	}
	return removed
}

// GetScheduledJobs returns all currently scheduled jobs
func (s *Scheduler) GetScheduledJobs() []*domain.Job {
	return s.queue.GetAll()
}

// GetQueueSize returns the number of jobs in the schedule queue
func (s *Scheduler) GetQueueSize() int {
	return s.queue.Size()
}

// run is the main scheduler loop using sleep-until-next strategy
func (s *Scheduler) run() {
	s.logger.Debug("scheduler main loop started")

	for {
		select {
		case <-s.stopSignal:
			s.logger.Debug("scheduler received stop signal")
			return
		case <-s.ctx.Done():
			s.logger.Debug("scheduler context cancelled")
			return
		default:
			// Continue with normal processing
		}

		// Get the next job to execute
		nextJob := s.queue.Peek()
		if nextJob == nil {
			// No jobs scheduled, wait for a new job to be added
			s.logger.Debug("no jobs scheduled, waiting for new jobs")
			select {
			case <-s.newJobSignal:
				s.logger.Debug("received new job signal, rechecking queue")
				continue
			case <-s.stopSignal:
				s.logger.Debug("scheduler stopping while waiting for jobs")
				return
			case <-s.ctx.Done():
				s.logger.Debug("scheduler context cancelled while waiting")
				return
			}
		}

		now := time.Now()
		scheduledTime := *nextJob.ScheduledTime

		if scheduledTime.After(now) {
			// Job is not due yet, sleep until it's time
			sleepDuration := scheduledTime.Sub(now)
			s.logger.Debug("sleeping until next job",
				"jobId", nextJob.Id,
				"sleepDuration", sleepDuration,
				"scheduledTime", scheduledTime.Format(time.RFC3339))

			select {
			case <-time.After(sleepDuration):
				// Time to execute the job
				s.logger.Debug("sleep completed, job is due", "jobId", nextJob.Id)
			case <-s.newJobSignal:
				// New job added, might be earlier than current job
				s.logger.Debug("received new job signal during sleep, rechecking queue")
				continue
			case <-s.stopSignal:
				s.logger.Debug("scheduler stopping during sleep")
				return
			case <-s.ctx.Done():
				s.logger.Debug("scheduler context cancelled during sleep")
				return
			}
		}

		// Job is due, remove it from queue and execute
		jobToExecute := s.queue.Next()
		if jobToExecute == nil {
			// Queue was modified while we were sleeping
			continue
		}

		s.executeJob(jobToExecute)
	}
}

// executeJob handles the execution of a single scheduled job
func (s *Scheduler) executeJob(job *domain.Job) {
	s.logger.Info("executing scheduled job",
		"jobId", job.Id,
		"command", job.Command,
		"originalScheduledTime", job.ScheduledTime.Format(time.RFC3339))

	// Execute the job using the provided executor
	if err := s.executor.ExecuteScheduledJob(s.ctx, job); err != nil {
		s.logger.Error("failed to execute scheduled job",
			"jobId", job.Id,
			"error", err)
	} else {
		s.logger.Debug("scheduled job execution initiated successfully", "jobId", job.Id)
	}
}

// IsRunning returns true if the scheduler is currently running
func (s *Scheduler) IsRunning() bool {
	s.runMutex.RLock()
	defer s.runMutex.RUnlock()
	return s.running
}
