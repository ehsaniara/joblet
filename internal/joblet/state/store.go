package state

import (
	"context"
	"errors"
	"joblet/internal/joblet/domain"
	"joblet/pkg/logger"
	"sync"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

//counterfeiter:generate . Store
type Store interface {
	CreateNewJob(job *domain.Job)
	UpdateJob(job *domain.Job)
	GetJob(id string) (*domain.Job, bool)
	ListJobs() []*domain.Job
	WriteToBuffer(jobId string, chunk []byte)
	GetOutput(id string) ([]byte, bool, error)
	SendUpdatesToClient(ctx context.Context, id string, stream DomainStreamer) error
}

//counterfeiter:generate . DomainStreamer
type DomainStreamer interface {
	SendData(data []byte) error
	SendKeepalive() error
	Context() context.Context
}

type store struct {
	tasks  map[string]*Task
	mutex  sync.RWMutex
	logger *logger.Logger
}

func New() Store {
	s := &store{
		tasks:  make(map[string]*Task),
		logger: logger.WithField("component", "store"),
	}

	s.logger.Debug("store initialized")
	return s
}

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

// CreateNewJob to add new job with all fields in the job struct, used only at the time of create
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

// SendUpdatesToClient sends the job log updates for scheduled, running, and completed jobs
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
