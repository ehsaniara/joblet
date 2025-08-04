package state

import (
	"context"
	"errors"
	"joblet/internal/joblet/domain"
	"joblet/pkg/logger"
	"sync"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

// Store defines the interface for managing job state and output buffering.
// It provides thread-safe operations for job lifecycle management and real-time log streaming.
//
//counterfeiter:generate . Store
type Store interface {
	// CreateNewJob adds a new job to the store with initial state.
	// Creates a new Task wrapper for the job and stores it by job ID.
	CreateNewJob(job *domain.Job)
	// UpdateJob updates an existing job's state and notifies subscribers.
	// Publishes status updates and shuts down subscribers if job is completed.
	UpdateJob(job *domain.Job)
	// GetJob retrieves a job by ID.
	// Returns the job and true if found, nil and false otherwise.
	GetJob(id string) (*domain.Job, bool)
	// ListJobs returns all jobs currently stored in the system.
	// Returns a slice containing copies of all job instances.
	ListJobs() []*domain.Job
	// WriteToBuffer appends log data to a job's output buffer.
	// Notifies subscribers of new log chunks for real-time streaming.
	WriteToBuffer(jobId string, chunk []byte)
	// GetOutput retrieves the complete output buffer for a job.
	// Returns the buffer data, whether job is still running, and any error.
	GetOutput(id string) ([]byte, bool, error)
	// SendUpdatesToClient streams job logs and status updates to a client.
	// Handles both existing buffer content and real-time updates for running jobs.
	SendUpdatesToClient(ctx context.Context, id string, stream DomainStreamer) error
}

// DomainStreamer defines the interface for streaming data to clients.
// Provides methods for sending log data and keepalive messages.
//
//counterfeiter:generate . DomainStreamer
type DomainStreamer interface {
	// SendData sends log data chunk to the client stream.
	SendData(data []byte) error
	// SendKeepalive sends a keepalive message to maintain connection.
	SendKeepalive() error
	// Context returns the streaming context for cancellation handling.
	Context() context.Context
}

type store struct {
	tasks  map[string]*Task
	mutex  sync.RWMutex
	logger *logger.Logger
}

// New creates a new Store instance with initialized task map and logger.
// Returns a thread-safe store implementation for job state management.
func New() Store {
	s := &store{
		tasks:  make(map[string]*Task),
		logger: logger.WithField("component", "store"),
	}

	s.logger.Debug("store initialized")
	return s
}

// WriteToBuffer appends log data to the specified job's output buffer.
// Thread-safe operation that notifies subscribers of new log chunks.
// Warns if attempting to write to non-existent job.
func (st *store) WriteToBuffer(jobId string, chunk []byte) {
	st.mutex.RLock()
	defer st.mutex.RUnlock()

	tk, exists := st.tasks[jobId]
	if !exists {
		st.logger.Warn("attempted to write to buffer for non-existent job", "jobId", jobId, "chunkSize", len(chunk))
		return
	}

	tk.WriteToBuffer(chunk)
}

// GetJob retrieves a job by its ID from the store.
// Returns a deep copy of the job to prevent external mutations.
// Returns nil and false if job doesn't exist.
func (st *store) GetJob(id string) (*domain.Job, bool) {
	st.mutex.RLock()
	defer st.mutex.RUnlock()

	j, exists := st.tasks[id]
	if !exists {
		st.logger.Debug("job not found in store", "jobId", id)
		return nil, false
	}

	job := j.GetJob()
	st.logger.Debug("job retrieved from store", "jobId", id, "status", string(job.Status))

	return job, true
}

// CreateNewJob adds a new job to the store with complete initialization.
// Creates a Task wrapper for the job and stores it by job ID.
// Warns if job already exists and skips creation to prevent duplicates.
func (st *store) CreateNewJob(job *domain.Job) {
	st.mutex.Lock()
	defer st.mutex.Unlock()

	if _, exist := st.tasks[job.Id]; exist {
		st.logger.Warn("job already exists, not creating new task", "jobId", job.Id)
		return
	}

	st.tasks[job.Id] = NewTask(job)

	st.logger.Debug("new task created", "jobId", job.Id, "command", job.Command, "totalTasks", len(st.tasks))
}

// UpdateJob updates an existing job's state and publishes changes.
// Notifies all subscribers of status updates and shuts down completed jobs.
// Warns if attempting to update non-existent job.
func (st *store) UpdateJob(job *domain.Job) {
	st.mutex.RLock()
	tk, exists := st.tasks[job.Id]
	st.mutex.RUnlock()

	if !exists {
		st.logger.Warn("attempted to update non-existent job", "jobId", job.Id, "status", string(job.Status))
		return
	}

	tk.UpdateJob(job)

	tk.Publish(Update{
		JobID:  job.Id,
		Status: string(job.Status),
	})

	// if completed/failed/stopped, shut down subscribers
	if job.IsCompleted() {
		st.logger.Debug("shutting down completed job", "jobId", job.Id, "finalStatus", string(job.Status))
		tk.Shutdown()
	}
}

// ListJobs returns all jobs currently stored in the system.
// Returns deep copies of jobs to prevent external mutations.
// Thread-safe operation with read lock protection.
func (st *store) ListJobs() []*domain.Job {
	st.mutex.RLock()
	defer st.mutex.RUnlock()

	jobs := make([]*domain.Job, 0, len(st.tasks))
	statusCounts := make(map[string]int)

	for _, tk := range st.tasks {
		job := tk.GetJob()
		jobs = append(jobs, job)
		statusCounts[string(job.Status)]++
	}

	return jobs
}

// GetOutput retrieves the complete output buffer for a job.
// Returns the buffer data, whether job is still running, and any error.
// Returns error if job doesn't exist in the store.
func (st *store) GetOutput(id string) ([]byte, bool, error) {
	st.mutex.RLock()
	tk, exists := st.tasks[id]
	st.mutex.RUnlock()

	if !exists {
		st.logger.Debug("output requested for non-existent job", "jobId", id)
		return nil, false, errors.New("job not found")
	}

	buffer := tk.GetBuffer()
	isRunning := tk.IsRunning()

	st.logger.Debug("job output retrieved", "jobId", id, "outputSize", len(buffer), "isRunning", isRunning)

	return buffer, isRunning, nil
}

// SendUpdatesToClient streams job logs and status updates to a client.
// Sends existing buffer content immediately, then subscribes to real-time updates.
// Handles scheduled, running, and completed jobs appropriately.
// Returns error if job doesn't exist or streaming fails.
func (st *store) SendUpdatesToClient(ctx context.Context, id string, stream DomainStreamer) error {
	st.mutex.RLock()
	task, exists := st.tasks[id]
	st.mutex.RUnlock()

	if !exists {
		st.logger.Warn("stream requested for non-existent job", "jobId", id)
		return errors.New("job not found")
	}

	select {
	case <-ctx.Done():
		st.logger.Debug("stream cancelled before start", "jobId", id, "error", ctx.Err())
		return ctx.Err()
	default:
	}

	jobCopy := task.GetJob()
	st.logger.Debug("starting log stream", "jobId", id, "status", string(jobCopy.Status))

	// Send existing buffer content for both completed AND running jobs
	buffer := task.GetBuffer()
	if len(buffer) > 0 {
		if err := stream.SendData(buffer); err != nil {
			st.logger.Warn("failed to send existing logs", "jobId", id, "error", err)
			return err
		}
		st.logger.Debug("sent existing logs", "jobId", id, "logSize", len(buffer))
	}

	// If job is completed, we're done after sending the buffer
	if jobCopy.IsCompleted() {
		st.logger.Debug("job is completed, finishing stream", "jobId", id)
		return nil
	}

	// For scheduled and running jobs, subscribe to updates
	updates, unsubscribe := task.Subscribe()
	defer unsubscribe()

	st.logger.Debug("streaming updates started", "jobId", id, "initialStatus", string(jobCopy.Status))

	// If job is scheduled, we'll get status updates when it starts
	// If job is running, we'll get log chunks and status updates
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-stream.Context().Done():
			return errors.New("stream cancelled by client")

		case update, ok := <-updates:
			if !ok {
				// channel closed, job completed or task cleaned up
				return nil
			}

			// Handle status updates (including SCHEDULED -> RUNNING transitions)
			// If job completed, we're done
			if update.Status == "COMPLETED" || update.Status == "FAILED" || update.Status == "STOPPED" {
				return nil
			}

			// Handle log chunks
			if len(update.LogChunk) > 0 {
				if streamErr := stream.SendData(update.LogChunk); streamErr != nil {
					st.logger.Warn("failed to send log chunk", "jobId", id, "chunkSize", len(update.LogChunk), "error", streamErr)
					return streamErr
				}
			}
		}
	}
}
