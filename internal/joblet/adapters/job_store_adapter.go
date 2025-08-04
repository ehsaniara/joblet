package adapters

import (
	"context"
	"fmt"
	"sync"
	"time"

	"joblet/internal/joblet/domain"
	"joblet/internal/joblet/pubsub"
	"joblet/pkg/buffer"
	"joblet/pkg/logger"
)

// Local interface definitions to avoid import cycles with pkg/store
type jobStore[K comparable, V any] interface {
	Create(ctx context.Context, key K, value V) error
	Get(ctx context.Context, key K) (V, bool, error)
	Update(ctx context.Context, key K, value V) error
	List(ctx context.Context) ([]V, error)
	Close() error
}

// Error types to match store package behavior
var (
	ErrKeyExists   = fmt.Errorf("key already exists")
	ErrKeyNotFound = fmt.Errorf("key not found")
)

func IsConflictError(err error) bool {
	return err != nil && err.Error() == "key already exists"
}

// jobStoreAdapter implements JobStoreAdapter using the new generic packages.
// It provides job storage with buffer management and pub-sub capabilities
// while using pluggable backends internally.
type jobStoreAdapter struct {
	// Generic storage backends
	jobStore  jobStore[string, *domain.Job]
	bufferMgr buffer.BufferManager
	pubsub    pubsub.PubSub[JobEvent]

	// Configuration for buffer creation
	bufferConfig *buffer.BufferConfig

	// Task management (maintains compatibility with current Task-based approach)
	tasks      map[string]*taskWrapper
	tasksMutex sync.RWMutex

	logger     *logger.Logger
	closed     bool
	closeMutex sync.RWMutex
}

// taskWrapper wraps a job with buffer and subscription management
// to maintain compatibility with the current Task abstraction.
type taskWrapper struct {
	job         *domain.Job
	buffer      buffer.Buffer
	subscribers map[string]*subscriptionContext
	subMutex    sync.RWMutex
	logger      *logger.Logger
	pubsub      pubsub.PubSub[JobEvent]
}

// subscriptionContext manages a single client subscription.
type subscriptionContext struct {
	id          string
	stream      DomainStreamer
	updates     <-chan pubsub.Message[JobEvent]
	unsubscribe func()
	cancel      context.CancelFunc
}

// JobEvent represents events published about job state changes.
type JobEvent struct {
	Type      string            `json:"type"` // CREATED, UPDATED, DELETED, LOG_CHUNK
	JobID     string            `json:"JobId"`
	Status    string            `json:"status,omitempty"`
	LogChunk  []byte            `json:"log_chunk,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Timestamp int64             `json:"timestamp"`
}

// NewJobStoreAdapter creates a new job store adapter with the specified backends.
func NewJobStoreAdapter(
	store jobStore[string, *domain.Job],
	bufferMgr buffer.BufferManager,
	pubsub pubsub.PubSub[JobEvent],
	bufferConfig *buffer.BufferConfig,
	logger *logger.Logger,
) JobStoreAdapter {
	if logger == nil {
		logger = logger.WithField("component", "job-store-adapter")
	}

	return &jobStoreAdapter{
		jobStore:     store,
		bufferMgr:    bufferMgr,
		pubsub:       pubsub,
		bufferConfig: bufferConfig,
		tasks:        make(map[string]*taskWrapper),
		logger:       logger,
	}
}

// CreateNewJob adds a new job to the store with complete initialization.
func (a *jobStoreAdapter) CreateNewJob(job *domain.Job) {

	a.closeMutex.RLock()
	if a.closed {
		a.closeMutex.RUnlock()
		a.logger.Warn("attempted to create job on closed store", "jobId", job.Id)
		return
	}
	a.closeMutex.RUnlock()

	// Store the job
	ctx := context.Background()
	if err := a.jobStore.Create(ctx, job.Id, job); err != nil {
		if IsConflictError(err) {
			a.logger.Warn("job already exists, not creating new task", "jobId", job.Id)
			return
		}
		a.logger.Error("failed to create job in store", "jobId", job.Id, "error", err)
		return
	}

	// Create buffer for job logs using configuration from factory
	bufferConfig := a.getBufferConfig()

	jobBuffer, err := a.bufferMgr.CreateBuffer(ctx, job.Id, *bufferConfig)
	if err != nil {
		a.logger.Error("failed to create buffer for job - log streaming will not work", "jobId", job.Id, "error", err, "bufferConfig", *bufferConfig)
		// Continue without buffer - logs won't be stored but job will work
	} else {
		a.logger.Debug("buffer created successfully for job", "jobId", job.Id, "bufferType", bufferConfig.Type)
	}

	// Create task wrapper
	task := &taskWrapper{
		job:         job.DeepCopy(),
		buffer:      jobBuffer,
		subscribers: make(map[string]*subscriptionContext),
		logger:      a.logger.WithField("jobId", job.Id),
		pubsub:      a.pubsub,
	}

	// Store task wrapper
	a.tasksMutex.Lock()
	a.tasks[job.Id] = task
	a.tasksMutex.Unlock()

	// Publish creation event
	if err := a.publishEvent(JobEvent{
		Type:      "CREATED",
		JobID:     job.Id,
		Status:    string(job.Status),
		Timestamp: time.Now().Unix(),
	}); err != nil {
		a.logger.Warn("failed to publish job creation event", "jobId", job.Id, "error", err)
	}

	a.logger.Debug("job created successfully", "jobId", job.Id, "status", string(job.Status))
}

// UpdateJob updates an existing job's state and publishes changes.
func (a *jobStoreAdapter) UpdateJob(job *domain.Job) {

	a.closeMutex.RLock()
	if a.closed {
		a.closeMutex.RUnlock()
		a.logger.Warn("attempted to update job on closed store", "jobId", job.Id)
		return
	}
	a.closeMutex.RUnlock()

	// Get existing job for status comparison
	a.tasksMutex.RLock()
	task, exists := a.tasks[job.Id]
	a.tasksMutex.RUnlock()

	if !exists {
		a.logger.Warn("attempted to update non-existent job", "jobId", job.Id)
		return
	}

	oldStatus := string(task.job.Status)
	newStatus := string(job.Status)

	// Update in store
	ctx := context.Background()
	if err := a.jobStore.Update(ctx, job.Id, job); err != nil {
		a.logger.Error("failed to update job in store", "jobId", job.Id, "error", err)
		return
	}

	// Update task wrapper
	task.job = job.DeepCopy()

	// Publish update event
	if err := a.publishEvent(JobEvent{
		Type:      "UPDATED",
		JobID:     job.Id,
		Status:    newStatus,
		Timestamp: time.Now().Unix(),
	}); err != nil {
		a.logger.Warn("failed to publish job update event", "jobId", job.Id, "error", err)
	}

	// Clean up completed jobs
	if job.IsCompleted() {
		a.logger.Debug("job completed, cleaning up subscribers", "jobId", job.Id, "finalStatus", newStatus)
		task.cleanupSubscribers()
	}

	a.logger.Debug("job updated successfully", "jobId", job.Id, "oldStatus", oldStatus, "newStatus", newStatus)
}

// GetJob retrieves a job by its ID from the store.
func (a *jobStoreAdapter) GetJob(id string) (*domain.Job, bool) {

	a.closeMutex.RLock()
	if a.closed {
		a.closeMutex.RUnlock()
		return nil, false
	}
	a.closeMutex.RUnlock()

	ctx := context.Background()
	job, exists, err := a.jobStore.Get(ctx, id)
	if err != nil {
		a.logger.Error("failed to get job from store", "jobId", id, "error", err)
		return nil, false
	}

	if exists {
		a.logger.Debug("job retrieved successfully", "jobId", id, "status", string(job.Status))
		return job.DeepCopy(), true
	}

	a.logger.Debug("job not found", "jobId", id)
	return nil, false
}

// ListJobs returns all jobs currently stored in the system.
func (a *jobStoreAdapter) ListJobs() []*domain.Job {

	a.closeMutex.RLock()
	if a.closed {
		a.closeMutex.RUnlock()
		return []*domain.Job{}
	}
	a.closeMutex.RUnlock()

	ctx := context.Background()
	jobs, err := a.jobStore.List(ctx)
	if err != nil {
		a.logger.Error("failed to list jobs from store", "error", err)
		return []*domain.Job{}
	}

	// Create deep copies
	result := make([]*domain.Job, len(jobs))
	for i, job := range jobs {
		result[i] = job.DeepCopy()
	}

	a.logger.Debug("jobs listed successfully", "count", len(result))
	return result
}

// WriteToBuffer appends log data to the specified job's output buffer.
func (a *jobStoreAdapter) WriteToBuffer(jobId string, chunk []byte) {
	if len(chunk) == 0 {
		return
	}

	a.tasksMutex.RLock()
	task, exists := a.tasks[jobId]
	a.tasksMutex.RUnlock()

	if !exists {
		a.logger.Warn("attempted to write to buffer for non-existent job", "jobId", jobId, "chunkSize", len(chunk))
		return
	}

	// Write to buffer
	if task.buffer != nil {
		if _, err := task.buffer.Write(chunk); err != nil {
			a.logger.Error("failed to write to job buffer", "jobId", jobId, "error", err)
			return
		}
		a.logger.Debug("successfully wrote to buffer", "jobId", jobId, "chunkSize", len(chunk))
	} else {
		a.logger.Warn("no buffer available for job - logs will not be stored", "jobId", jobId, "chunkSize", len(chunk))
	}

	// Publish log chunk event
	if err := a.publishEvent(JobEvent{
		Type:      "LOG_CHUNK",
		JobID:     jobId,
		LogChunk:  chunk,
		Timestamp: time.Now().Unix(),
	}); err != nil {
		a.logger.Warn("failed to publish log chunk event", "jobId", jobId, "error", err)
	}

	a.logger.Debug("log chunk written", "jobId", jobId, "chunkSize", len(chunk))
}

// GetOutput retrieves the complete output buffer for a job.
func (a *jobStoreAdapter) GetOutput(id string) ([]byte, bool, error) {
	a.tasksMutex.RLock()
	task, exists := a.tasks[id]
	a.tasksMutex.RUnlock()

	if !exists {
		a.logger.Debug("output requested for non-existent job", "jobId", id)
		return nil, false, fmt.Errorf("job not found")
	}

	if task.buffer == nil {
		return []byte{}, task.job.IsRunning(), nil
	}

	data, err := task.buffer.Read()
	if err != nil {
		a.logger.Error("failed to read job buffer", "jobId", id, "error", err)
		return nil, false, err
	}

	isRunning := task.job.IsRunning()
	a.logger.Debug("job output retrieved", "jobId", id, "outputSize", len(data), "isRunning", isRunning)

	return data, isRunning, nil
}

// SendUpdatesToClient streams job logs and status updates to a client.
func (a *jobStoreAdapter) SendUpdatesToClient(ctx context.Context, id string, stream DomainStreamer) error {
	a.tasksMutex.RLock()
	task, exists := a.tasks[id]
	a.tasksMutex.RUnlock()

	if !exists {
		a.logger.Warn("stream requested for non-existent job", "jobId", id)
		return fmt.Errorf("job not found")
	}

	// Send existing buffer content first
	if task.buffer != nil {
		data, err := task.buffer.Read()
		if err != nil {
			a.logger.Error("failed to read existing buffer", "jobId", id, "error", err)
			return err
		}

		if len(data) > 0 {
			if err := stream.SendData(data); err != nil {
				a.logger.Warn("failed to send existing logs", "jobId", id, "error", err)
				return err
			}
			a.logger.Debug("sent existing logs", "jobId", id, "logSize", len(data))
		}
	}

	// If job is completed, we're done
	if task.job.IsCompleted() {
		a.logger.Debug("job is completed, finishing stream", "jobId", id)
		return nil
	}

	// Subscribe to job events for real-time updates
	// This will block until the subscription ends
	return a.subscribeToJobUpdates(ctx, id, task, stream)
}

// Close gracefully shuts down the adapter and releases resources.
func (a *jobStoreAdapter) Close() error {
	a.closeMutex.Lock()
	defer a.closeMutex.Unlock()

	if a.closed {
		return nil
	}
	a.closed = true

	// Close all task subscriptions
	a.tasksMutex.Lock()
	for _, task := range a.tasks {
		task.cleanupSubscribers()
	}
	a.tasks = make(map[string]*taskWrapper)
	a.tasksMutex.Unlock()

	// Close backend resources
	if err := a.bufferMgr.Close(); err != nil {
		a.logger.Error("failed to close buffer manager", "error", err)
	}

	if err := a.pubsub.Close(); err != nil {
		a.logger.Error("failed to close pub-sub", "error", err)
	}

	if err := a.jobStore.Close(); err != nil {
		a.logger.Error("failed to close job store", "error", err)
	}

	a.logger.Debug("job store adapter closed successfully")
	return nil
}

// Helper methods

func (a *jobStoreAdapter) publishEvent(event JobEvent) error {
	ctx := context.Background()
	topic := fmt.Sprintf("job.%s", event.JobID)
	a.logger.Debug("publishing event", "jobId", event.JobID, "eventType", event.Type, "topic", topic, "chunkSize", len(event.LogChunk))

	err := a.pubsub.Publish(ctx, topic, event)
	if err != nil {
		a.logger.Error("failed to publish event", "jobId", event.JobID, "eventType", event.Type, "topic", topic, "error", err)
	} else {
		a.logger.Debug("successfully published event", "jobId", event.JobID, "eventType", event.Type, "topic", topic)
	}

	return err
}

func (a *jobStoreAdapter) getBufferConfig() *buffer.BufferConfig {
	if a.bufferConfig == nil {
		// No fallback values - application should fail fast if config missing
		panic("buffer configuration is required but not provided")
	}
	return a.bufferConfig
}

func (a *jobStoreAdapter) subscribeToJobUpdates(ctx context.Context, jobID string, task *taskWrapper, stream DomainStreamer) error {
	// Subscribe to job-specific events
	topic := fmt.Sprintf("job.%s", jobID)
	a.logger.Debug("subscribing to job events for streaming", "jobId", jobID, "topic", topic)

	updates, unsubscribe, err := a.pubsub.Subscribe(ctx, topic)
	if err != nil {
		a.logger.Error("failed to subscribe to job events", "jobId", jobID, "topic", topic, "error", err)
		return fmt.Errorf("failed to subscribe to job events: %w", err)
	}

	a.logger.Debug("successfully subscribed to job events", "jobId", jobID, "topic", topic)

	subCtx, cancel := context.WithCancel(ctx)
	subID := fmt.Sprintf("stream_%d", time.Now().UnixNano())

	// Store subscription context
	subContext := &subscriptionContext{
		id:          subID,
		stream:      stream,
		updates:     updates,
		unsubscribe: unsubscribe,
		cancel:      cancel,
	}

	task.subMutex.Lock()
	task.subscribers[subID] = subContext
	task.subMutex.Unlock()

	// Create a channel to signal when subscription ends
	done := make(chan error, 1)

	// Handle updates in a separate goroutine
	go func() {
		a.logger.Debug("started subscription handler goroutine", "jobId", jobID, "subId", subID)
		defer func() {
			a.logger.Debug("cleaning up subscription", "jobId", jobID, "subId", subID)
			unsubscribe()
			cancel()

			task.subMutex.Lock()
			delete(task.subscribers, subID)
			task.subMutex.Unlock()

			a.logger.Debug("subscription cleaned up", "jobId", jobID, "subId", subID)

			// Signal that we're done
			close(done)
		}()

		for {
			select {
			case <-subCtx.Done():
				a.logger.Debug("subscription context cancelled", "jobId", jobID, "subId", subID)
				done <- nil
				return
			case <-stream.Context().Done():
				a.logger.Debug("stream context cancelled", "jobId", jobID, "subId", subID)
				done <- nil
				return
			case msg, ok := <-updates:
				if !ok {
					a.logger.Debug("updates channel closed", "jobId", jobID, "subId", subID)
					done <- nil
					return
				}

				event := msg.Payload
				a.logger.Debug("received event from pubsub", "jobId", jobID, "eventType", event.Type, "chunkSize", len(event.LogChunk))

				// Handle different event types
				switch event.Type {
				case "LOG_CHUNK":
					if len(event.LogChunk) > 0 {
						a.logger.Debug("sending log chunk to client", "jobId", jobID, "chunkSize", len(event.LogChunk))
						if err := stream.SendData(event.LogChunk); err != nil {
							a.logger.Warn("failed to send log chunk to client", "jobId", jobID, "error", err)
							done <- err
							return
						}
						a.logger.Debug("successfully sent log chunk to client", "jobId", jobID, "chunkSize", len(event.LogChunk))
					}
				case "UPDATED":
					a.logger.Debug("received job status update", "jobId", jobID, "status", event.Status)
					// Status updates don't need to send data, just keep the connection alive
					if event.Status == "COMPLETED" || event.Status == "FAILED" || event.Status == "STOPPED" {
						a.logger.Debug("job completed, ending stream", "jobId", jobID, "finalStatus", event.Status)
						done <- nil
						return // Job completed, end the stream
					}
				}
			}
		}
	}()

	// Wait for the subscription to end
	a.logger.Debug("waiting for subscription to complete", "jobId", jobID)
	err = <-done
	a.logger.Debug("subscription completed", "jobId", jobID, "error", err)

	return err
}

func (t *taskWrapper) cleanupSubscribers() {
	t.subMutex.Lock()
	defer t.subMutex.Unlock()

	for _, sub := range t.subscribers {
		sub.cancel()
		sub.unsubscribe()
	}
	t.subscribers = make(map[string]*subscriptionContext)

	// Note: We intentionally do NOT close the buffer here.
	// The buffer should remain readable to allow viewing historical logs
	// after job completion. The buffer will be closed when the entire
	// adapter is shut down.
}
