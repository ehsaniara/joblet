package store

import (
	"context"
	"sync"
	"time"
	"worker/internal/worker/domain"
	"worker/internal/worker/interfaces"
	_errors "worker/pkg/errors"
	"worker/pkg/logger"
)

type store struct {
	tasks  map[string]*Task
	mutex  sync.RWMutex
	logger *logger.Logger
}

func New() interfaces.Store {
	s := &store{
		tasks:  make(map[string]*Task),
		logger: logger.WithField("component", "store"),
	}

	s.logger.Info("store initialized")
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

	st.logger.Info("new task created", "jobId", job.Id, "command", job.Command, "totalTasks", len(st.tasks))
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
		st.logger.Info("shutting down completed job", "jobId", job.Id, "finalStatus", string(job.Status))
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
		return nil, false, _errors.ErrJobNotFound
	}

	buffer := tk.GetBuffer()
	isRunning := tk.IsRunning()

	st.logger.Debug("job output retrieved", "jobId", id, "outputSize", len(buffer), "isRunning", isRunning)

	return buffer, isRunning, nil
}

// SendUpdatesToClient sends the job log updates only
func (st *store) SendUpdatesToClient(ctx context.Context, id string, stream interfaces.DomainStreamer) error {
	st.mutex.RLock()
	job, exists := st.tasks[id]
	st.mutex.RUnlock()

	if !exists {
		st.logger.Warn("stream requested for non-existent job", "jobId", id)
		return _errors.ErrJobNotFound
	}

	select {
	case <-ctx.Done():
		st.logger.Info("stream cancelled before start", "jobId", id, "error", ctx.Err())
		return ctx.Err()
	default:
	}

	if !job.IsRunning() {
		jobCopy := job.GetJob()
		st.logger.Warn("stream requested for non-running job", "jobId", id, "status", string(jobCopy.Status))
		return nil
	}

	updates, unsubscribe := job.Subscribe()
	defer unsubscribe()

	st.logger.Info("streaming updates started", "jobId", id)

	updateCount := 0
	logBytesSent := 0
	startTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			duration := time.Since(startTime)
			st.logger.Info("stream cancelled by context", "jobId", id, "duration", duration, "updatesSent", updateCount, "bytesSent", logBytesSent, "reason", ctx.Err())
			return ctx.Err()

		case <-stream.Context().Done():
			duration := time.Since(startTime)
			st.logger.Info("stream cancelled by client", "jobId", id, "duration", duration, "updatesSent", updateCount, "bytesSent", logBytesSent)
			return _errors.ErrStreamCancelled

		case update, ok := <-updates:
			if !ok {
				// channel closed, job completed or task cleaned up
				duration := time.Since(startTime)
				st.logger.Info("update channel closed", "jobId", id, "duration", duration, "updatesSent", updateCount, "bytesSent", logBytesSent)
				return nil
			}

			if update.LogChunk != nil {

				if streamErr := stream.SendData(update.LogChunk); streamErr != nil {
					st.logger.Warn("failed to send log chunk", "jobId", id, "chunkSize", len(update.LogChunk), "error", streamErr)
					return streamErr
				}

				updateCount++
				logBytesSent += len(update.LogChunk)

				st.logger.Debug("log chunk sent", "jobId", id, "chunkSize", len(update.LogChunk), "totalBytesSent", logBytesSent)
			}

			// exit if job is not running
			if update.Status != "" && update.Status != "RUNNING" {
				duration := time.Since(startTime)
				st.logger.Info("job status changed, ending stream", "jobId", id, "newStatus", update.Status, "duration", duration, "updatesSent", updateCount, "bytesSent", logBytesSent)
				return nil
			}

		}
	}
}
