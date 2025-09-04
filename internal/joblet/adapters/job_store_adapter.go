package adapters

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"joblet/internal/joblet/domain"
	"joblet/internal/joblet/interfaces"
	"joblet/internal/joblet/pubsub"
	"joblet/pkg/config"
	"joblet/pkg/logger"
)

// Local interface definitions to avoid import cycles with pkg/store
type jobStore[K comparable, V any] interface {
	Create(ctx context.Context, key K, value V) error
	Get(ctx context.Context, key K) (V, bool, error)
	Update(ctx context.Context, key K, value V) error
	Delete(ctx context.Context, key K) error
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

// jobStoreAdapter implements JobStorer using the new generic packages.
// It provides job storage with buffer management and pub-sub capabilities
// while using pluggable backends internally.
type jobStoreAdapter struct {
	// Generic storage backends
	jobStore jobStore[string, *domain.Job]
	logMgr   *SimpleLogManager
	pubsub   pubsub.PubSub[JobEvent]

	// Configuration for log persistence
	logPersistenceConfig *config.LogPersistenceConfig

	// Async log system for rate-decoupled logging
	asyncLogSystem *AsyncLogSystem

	// Task management (maintains compatibility with current Task-based approach)
	tasks      map[string]*taskWrapper
	tasksMutex sync.RWMutex

	logger     *logger.Logger
	closed     bool
	closeMutex sync.RWMutex
}

// Ensure jobStoreAdapter implements the interfaces
var _ JobStorer = (*jobStoreAdapter)(nil)

// taskWrapper wraps a job with buffer and subscription management
// to maintain compatibility with the current Task abstraction.
type taskWrapper struct {
	job         *domain.Job
	logBuffer   *SimpleLogBuffer
	subscribers map[string]*subscriptionContext
	subMutex    sync.RWMutex
	logger      *logger.Logger
	pubsub      pubsub.PubSub[JobEvent]
}

// subscriptionContext manages a single client subscription.
type subscriptionContext struct {
	id          string
	stream      interfaces.DomainStreamer
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

// NewJobStorer creates a new job store adapter with the specified backends.
func NewJobStorer(
	store jobStore[string, *domain.Job],
	logMgr *SimpleLogManager,
	pubsub pubsub.PubSub[JobEvent],
	logger *logger.Logger,
) JobStorer {
	if logger == nil {
		logger = logger.WithField("component", "job-store-adapter")
	}

	return &jobStoreAdapter{
		jobStore: store,
		logMgr:   logMgr,
		pubsub:   pubsub,
		tasks:    make(map[string]*taskWrapper),
		logger:   logger,
	}
}

// NewJobStorerWithLogPersistence creates a new job store adapter with log persistence
func NewJobStorerWithLogPersistence(
	store jobStore[string, *domain.Job],
	logMgr *SimpleLogManager,
	pubsub pubsub.PubSub[JobEvent],
	logPersistenceConfig *config.LogPersistenceConfig,
	logger *logger.Logger,
) JobStorer {
	if logger == nil {
		logger = logger.WithField("component", "job-store-adapter")
	}

	adapter := &jobStoreAdapter{
		jobStore:             store,
		logMgr:               logMgr,
		pubsub:               pubsub,
		logPersistenceConfig: logPersistenceConfig,
		tasks:                make(map[string]*taskWrapper),
		logger:               logger,
	}

	// Initialize async log system for rate-decoupled logging
	if logPersistenceConfig != nil {
		adapter.asyncLogSystem = NewAsyncLogSystem(logPersistenceConfig, logger)
	}

	// Automatic log cleanup disabled - logs preserved until manually deleted

	return adapter
}

// CreateNewJob adds a new job to the store with complete initialization.
// Sets up the job record, creates a buffer for log storage, initializes task wrapper,
// and publishes a creation event. Handles resource creation failures gracefully.
func (a *jobStoreAdapter) CreateNewJob(job *domain.Job) {

	a.closeMutex.RLock()
	if a.closed {
		a.closeMutex.RUnlock()
		a.logger.Warn("attempted to create job on closed store", "jobId", job.Uuid)
		return
	}
	a.closeMutex.RUnlock()

	// Store the job
	ctx := context.Background()
	if err := a.jobStore.Create(ctx, job.Uuid, job); err != nil {
		if IsConflictError(err) {
			a.logger.Warn("job already exists, not creating new task", "jobId", job.Uuid)
			return
		}
		a.logger.Error("failed to create job in store", "jobId", job.Uuid, "error", err)
		return
	}

	logBuffer := a.logMgr.GetBuffer(job.Uuid)
	a.logger.Debug("log buffer created successfully for job", "jobId", job.Uuid)

	task := &taskWrapper{
		job:         job.DeepCopy(),
		logBuffer:   logBuffer,
		subscribers: make(map[string]*subscriptionContext),
		logger:      a.logger.WithField("jobId", job.Uuid),
		pubsub:      a.pubsub,
	}

	// Store task wrapper
	a.tasksMutex.Lock()
	a.tasks[job.Uuid] = task
	a.tasksMutex.Unlock()

	// Publish creation event
	if err := a.publishEvent(JobEvent{
		Type:      "CREATED",
		JobID:     job.Uuid,
		Status:    string(job.Status),
		Timestamp: time.Now().Unix(),
	}); err != nil {
		a.logger.Warn("failed to publish job creation event", "jobId", job.Uuid, "error", err)
	}

	a.logger.Debug("job created successfully", "jobId", job.Uuid, "status", string(job.Status))
}

// UpdateJob updates an existing job's state and publishes changes.
// Compares old and new status, persists changes to store, publishes update events,
// and cleans up subscribers when job reaches a completed state.
func (a *jobStoreAdapter) UpdateJob(job *domain.Job) {

	a.closeMutex.RLock()
	if a.closed {
		a.closeMutex.RUnlock()
		a.logger.Warn("attempted to update job on closed store", "jobId", job.Uuid)
		return
	}
	a.closeMutex.RUnlock()

	a.tasksMutex.RLock()
	task, exists := a.tasks[job.Uuid]
	a.tasksMutex.RUnlock()

	if !exists {
		a.logger.Warn("attempted to update non-existent job", "jobId", job.Uuid)
		return
	}

	oldStatus := string(task.job.Status)
	newStatus := string(job.Status)

	// Update in store
	ctx := context.Background()
	if err := a.jobStore.Update(ctx, job.Uuid, job); err != nil {
		a.logger.Error("failed to update job in store", "jobId", job.Uuid, "error", err)
		return
	}

	// Update task wrapper
	task.job = job.DeepCopy()

	// Publish update event
	if err := a.publishEvent(JobEvent{
		Type:      "UPDATED",
		JobID:     job.Uuid,
		Status:    newStatus,
		Timestamp: time.Now().Unix(),
	}); err != nil {
		a.logger.Warn("failed to publish job update event", "jobId", job.Uuid, "error", err)
	}

	// Clean up completed jobs
	if job.IsCompleted() {
		a.logger.Debug("job completed, cleaning up subscribers", "jobId", job.Uuid, "finalStatus", newStatus)
		task.cleanupSubscribers()
	}

	a.logger.Debug("job updated successfully", "jobId", job.Uuid, "oldStatus", oldStatus, "newStatus", newStatus)
}

// GetJob retrieves a job by its ID from the store.
// Returns a deep copy of the job if found, nil and false otherwise.
// Handles store access errors and closed adapter states gracefully.
func (a *jobStoreAdapter) Job(id string) (*domain.Job, bool) {

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

// GetJobByPrefix retrieves a job by UUID prefix (supports short-form UUID lookup).
// Returns the job if exactly one match is found, otherwise returns nil and false.
// If multiple jobs match the prefix, logs a warning and returns nil, false.
func (a *jobStoreAdapter) JobByPrefix(prefix string) (*domain.Job, bool) {
	// First try exact match
	if job, exists := a.Job(prefix); exists {
		return job, true
	}

	// If exact match fails and prefix is shorter than full UUID, try prefix matching
	if len(prefix) < 36 { // Full UUID is 36 characters
		a.closeMutex.RLock()
		if a.closed {
			a.closeMutex.RUnlock()
			return nil, false
		}
		a.closeMutex.RUnlock()

		ctx := context.Background()
		jobs, err := a.jobStore.List(ctx)
		if err != nil {
			a.logger.Error("failed to list jobs for prefix search", "prefix", prefix, "error", err)
			return nil, false
		}

		var matches []*domain.Job
		for _, job := range jobs {
			if strings.HasPrefix(job.Uuid, prefix) {
				matches = append(matches, job)
			}
		}

		if len(matches) == 1 {
			a.logger.Debug("found unique job by prefix", "prefix", prefix, "jobId", matches[0].Uuid)
			return matches[0].DeepCopy(), true
		} else if len(matches) > 1 {
			var matchedUuids []string
			for _, job := range matches {
				matchedUuids = append(matchedUuids, job.Uuid)
			}
			a.logger.Warn("prefix matches multiple jobs - ambiguous", "prefix", prefix, "matches", matchedUuids)
			return nil, false
		}
	}

	a.logger.Debug("no job found with prefix", "prefix", prefix)
	return nil, false
}

// ListJobs returns all jobs currently stored in the system.
// Creates deep copies of all jobs to prevent external modification.
// Returns empty slice on error or when adapter is closed.
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

	result := make([]*domain.Job, len(jobs))
	for i, job := range jobs {
		result[i] = job.DeepCopy()
	}

	return result
}

// WriteToBuffer appends log data to the specified job's output buffer.
// Writes to both in-memory buffer (for real-time streaming) and async log system
// (for persistence). Publishes log chunk events and supports UUID prefix resolution.
func (a *jobStoreAdapter) WriteToBuffer(jobID string, chunk []byte) {
	if len(chunk) == 0 {
		return
	}

	// For WriteToBuffer, jobID should already be the full UUID since it's called internally
	// from job execution, but we still support prefix resolution for consistency
	resolvedUuid, err := a.resolveUuidByPrefix(jobID)
	if err != nil {
		a.logger.Warn("failed to resolve job UUID for buffer write", "input", jobID, "error", err)
		return
	}

	a.tasksMutex.RLock()
	task, exists := a.tasks[resolvedUuid]
	a.tasksMutex.RUnlock()

	if !exists {
		a.logger.Warn("attempted to write to buffer for non-existent job", "jobId", resolvedUuid, "chunkSize", len(chunk))
		return
	}

	// Write to log buffer
	if task.logBuffer != nil {
		if err := task.logBuffer.Write(chunk); err != nil {
			a.logger.Error("failed to write to job log buffer", "jobId", resolvedUuid, "error", err)
			return
		}
		a.logger.Debug("successfully wrote to buffer", "jobId", resolvedUuid, "chunkSize", len(chunk))
	} else {
		a.logger.Warn("no buffer available for job - logs will not be stored", "jobId", resolvedUuid, "chunkSize", len(chunk))
	}

	// Write to async log system for rate-decoupled persistence
	if a.asyncLogSystem != nil {
		a.asyncLogSystem.WriteLog(resolvedUuid, chunk)
	}

	// Publish log chunk event
	if err := a.publishEvent(JobEvent{
		Type:      "LOG_CHUNK",
		JobID:     resolvedUuid,
		LogChunk:  chunk,
		Timestamp: time.Now().Unix(),
	}); err != nil {
		a.logger.Warn("failed to publish log chunk event", "jobId", resolvedUuid, "error", err)
	}

	a.logger.Debug("log chunk written", "jobId", resolvedUuid, "chunkSize", len(chunk))
}

// GetOutput retrieves the complete output buffer for a job.
// Returns the full buffer content, job running status, and any read errors.
// Supports UUID prefix resolution for convenience.
func (a *jobStoreAdapter) Output(id string) ([]byte, bool, error) {
	// Resolve UUID by prefix first
	resolvedUuid, err := a.resolveUuidByPrefix(id)
	if err != nil {
		a.logger.Debug("failed to resolve job UUID for output", "input", id, "error", err)
		return nil, false, fmt.Errorf("job not found")
	}

	a.tasksMutex.RLock()
	task, exists := a.tasks[resolvedUuid]
	a.tasksMutex.RUnlock()

	if !exists {
		a.logger.Debug("output requested for non-existent job", "jobId", resolvedUuid)
		return nil, false, fmt.Errorf("job not found")
	}

	if task.logBuffer == nil {
		return []byte{}, task.job.IsRunning(), nil
	}

	chunks := task.logBuffer.ReadAll()
	if len(chunks) == 0 {
		a.logger.Debug("no data available in job log buffer", "jobId", id)
		return []byte{}, task.job.IsRunning(), nil
	}

	// Combine all chunks into a single byte slice
	var totalSize int
	for _, chunk := range chunks {
		totalSize += len(chunk)
	}

	data := make([]byte, 0, totalSize)
	for _, chunk := range chunks {
		data = append(data, chunk...)
	}

	isRunning := task.job.IsRunning()
	a.logger.Debug("job output retrieved", "jobId", id, "outputSize", len(data), "isRunning", isRunning)

	return data, isRunning, nil
}

// SendUpdatesToClient streams job logs and status updates to a client.
// First sends existing buffer content, then subscribes to real-time updates.
// Handles completed jobs by sending existing logs and terminating immediately.
func (a *jobStoreAdapter) SendUpdatesToClient(ctx context.Context, id string, stream interfaces.DomainStreamer) error {
	// Resolve UUID by prefix first
	resolvedUuid, err := a.resolveUuidByPrefix(id)
	if err != nil {
		a.logger.Warn("failed to resolve job UUID", "input", id, "error", err)
		return fmt.Errorf("job not found")
	}

	a.tasksMutex.RLock()
	task, exists := a.tasks[resolvedUuid]
	a.tasksMutex.RUnlock()

	if !exists {
		a.logger.Warn("stream requested for non-existent job", "jobId", resolvedUuid)
		return fmt.Errorf("job not found")
	}

	// Send existing buffer content first
	if task.logBuffer != nil {
		chunks := task.logBuffer.ReadAll()
		if len(chunks) > 0 {
			for _, chunk := range chunks {
				if err := stream.SendData(chunk); err != nil {
					a.logger.Warn("failed to send existing log chunk", "jobId", id, "error", err)
					return err
				}
			}
			a.logger.Debug("sent existing logs", "jobId", id, "chunkCount", len(chunks))
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
// Stops cleanup routines, closes all subscriptions, shuts down async log system,
// and closes all backend resources (buffer manager, pubsub, job store).
func (a *jobStoreAdapter) Close() error {
	a.closeMutex.Lock()
	defer a.closeMutex.Unlock()

	if a.closed {
		return nil
	}
	a.closed = true

	// No cleanup goroutine to stop - automatic log cleanup has been disabled

	a.tasksMutex.Lock()
	for _, task := range a.tasks {
		task.cleanupSubscribers()
	}
	a.tasks = make(map[string]*taskWrapper)
	a.tasksMutex.Unlock()

	if a.asyncLogSystem != nil {
		if err := a.asyncLogSystem.Close(); err != nil {
			a.logger.Error("failed to close async log system", "error", err)
		}
	}

	// Simple log manager doesn't need explicit closing - just clear resources
	// All buffers are cleaned up through job cleanup process

	if err := a.pubsub.Close(); err != nil {
		a.logger.Error("failed to close pub-sub", "error", err)
	}

	if err := a.jobStore.Close(); err != nil {
		a.logger.Error("failed to close job store", "error", err)
	}

	a.logger.Debug("job store adapter closed successfully")
	return nil
}

// ensureNotClosed checks if the adapter is closed and returns an error if so
func (a *jobStoreAdapter) ensureNotClosed() error {
	a.closeMutex.RLock()
	defer a.closeMutex.RUnlock()
	if a.closed {
		return fmt.Errorf("adapter is closed")
	}
	return nil
}

// resolveJobUuid resolves a job ID to UUID, used across multiple operations
func (a *jobStoreAdapter) resolveJobUuid(jobID string, operation string) (string, error) {
	resolvedUuid, err := a.resolveUuidByPrefix(jobID)
	if err != nil {
		a.logger.Debug("failed to resolve job UUID", "input", jobID, "operation", operation, "error", err)
		return "", fmt.Errorf("job not found: %s", jobID)
	}
	return resolvedUuid, nil
}

// validateAndResolveJob resolves a job ID to UUID and retrieves the job, used across multiple operations
func (a *jobStoreAdapter) validateAndResolveJob(jobID string, operation string) (string, *domain.Job, error) {
	resolvedUuid, err := a.resolveJobUuid(jobID, operation)
	if err != nil {
		return "", nil, err
	}

	job, exists := a.Job(resolvedUuid)
	if !exists {
		return "", nil, fmt.Errorf("job not found: %s", resolvedUuid)
	}

	return resolvedUuid, job, nil
}

// validateJobNotRunning checks if a job is in a state that allows certain operations
func (a *jobStoreAdapter) validateJobNotRunning(job *domain.Job) error {
	if job.Status == "RUNNING" || job.Status == "INITIALIZING" {
		return fmt.Errorf("cannot perform operation on running job %s (status: %s) - stop the job first",
			job.Uuid, job.Status)
	}
	return nil
}

// cleanupTaskWrapper removes subscriptions but preserves buffers for log access
func (a *jobStoreAdapter) cleanupTaskWrapper(jobID string) error {
	a.tasksMutex.Lock()
	defer a.tasksMutex.Unlock()

	task, exists := a.tasks[jobID]
	if !exists {
		return nil // No task to cleanup
	}

	// Cancel subscriptions
	task.cleanupSubscribers()

	// Preserve buffer and task for log access after job completion
	// Only remove subscriptions to prevent resource leaks
	return nil
}

// completeTaskCleanup fully removes task wrapper, buffers, and logs (used only for explicit log deletion)
func (a *jobStoreAdapter) completeTaskCleanup(jobID string) error {
	a.tasksMutex.Lock()
	defer a.tasksMutex.Unlock()

	task, exists := a.tasks[jobID]
	if !exists {
		return nil // No task to cleanup
	}

	// Cancel subscriptions
	task.cleanupSubscribers()

	// Remove log buffer from manager (deletes logs)
	if task.logBuffer != nil {
		a.logMgr.RemoveBuffer(jobID)
	}
	delete(a.tasks, jobID)
	return nil
}

// publishJobEvent creates and publishes a job event with consistent structure
func (a *jobStoreAdapter) publishJobEvent(eventType, jobID string, metadata map[string]string) error {
	event := JobEvent{
		Type:      eventType,
		JobID:     jobID,
		Timestamp: time.Now().Unix(),
		Metadata:  metadata,
	}

	if err := a.publishEvent(event); err != nil {
		a.logger.Warn("failed to publish job event", "type", eventType, "jobId", jobID, "error", err)
		return err
	}
	return nil
}

// DeleteJobLogs deletes log files and buffers for a specific job.
// This method should only be called when user explicitly requests log deletion.
func (a *jobStoreAdapter) DeleteJobLogs(jobID string) error {
	if err := a.ensureNotClosed(); err != nil {
		return err
	}

	// Resolve UUID by prefix first
	resolvedUuid, err := a.resolveJobUuid(jobID, "log deletion")
	if err != nil {
		return err
	}

	// Perform complete cleanup including buffer removal
	if err := a.completeTaskCleanup(resolvedUuid); err != nil {
		return fmt.Errorf("failed to delete job logs: %w", err)
	}
	return nil
}

// DeleteJob performs complete job deletion but preserves logs for user control.
func (a *jobStoreAdapter) DeleteJob(jobID string) error {
	if err := a.ensureNotClosed(); err != nil {
		return err
	}

	resolvedUuid, job, err := a.validateAndResolveJob(jobID, "deletion")
	if err != nil {
		return err
	}

	a.logger.Info("complete job deletion requested", "jobId", resolvedUuid)

	if err := a.validateJobNotRunning(job); err != nil {
		a.logger.Warn("cannot delete running job", "jobId", resolvedUuid, "status", job.Status)
		return err
	}

	// Cleanup operations
	if err := a.cleanupTaskWrapper(resolvedUuid); err != nil {
		a.logger.Warn("task cleanup failed", "jobId", resolvedUuid, "error", err)
	}

	// Logs are preserved and must be deleted manually if needed

	// Remove from store
	if err := a.jobStore.Delete(context.Background(), resolvedUuid); err != nil {
		a.logger.Error("failed to delete job from store", "jobId", resolvedUuid, "error", err)
		return fmt.Errorf("failed to delete job from store: %w", err)
	}

	// Publish event
	_ = a.publishJobEvent("DELETED", resolvedUuid, map[string]string{"reason": "user_requested"})

	a.logger.Info("job deletion completed successfully", "jobId", resolvedUuid)
	return nil
}

// Helper methods

// resolveUuidByPrefix resolves a UUID prefix to a full UUID.
// Returns the input if it's already a full UUID (36 chars).
// Searches task map for unique prefix matches, returns error for ambiguous or missing matches.
func (a *jobStoreAdapter) resolveUuidByPrefix(prefix string) (string, error) {
	// If it's already a full UUID (36 characters), return as-is
	if len(prefix) == 36 {
		return prefix, nil
	}

	// Search for matching UUIDs in tasks map
	a.tasksMutex.RLock()
	defer a.tasksMutex.RUnlock()

	var matches []string
	for uuid := range a.tasks {
		if strings.HasPrefix(uuid, prefix) {
			matches = append(matches, uuid)
		}
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("no job found with prefix %s", prefix)
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("prefix %s matches multiple jobs: %v", prefix, matches)
	}

	return matches[0], nil
}

// publishEvent publishes a job event to the pub-sub system.
// Uses job-specific topics ("job.{jobId}") for targeted subscriptions.
// Logs success/failure for debugging and monitoring.
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

// subscribeToJobUpdates creates a real-time subscription for job events.
// Handles LOG_CHUNK events by streaming data to client, and UPDATED events by checking
// for job completion. Manages subscription lifecycle and cleanup automatically.
func (a *jobStoreAdapter) subscribeToJobUpdates(ctx context.Context, jobID string, task *taskWrapper, stream interfaces.DomainStreamer) error {
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

// cleanupSubscribers cancels and removes all active subscriptions for this task.
// Called when job completes to free resources. Buffer remains available for historical access.
func (t *taskWrapper) cleanupSubscribers() {
	t.subMutex.Lock()
	defer t.subMutex.Unlock()

	for _, sub := range t.subscribers {
		sub.cancel()
		sub.unsubscribe()
	}
	t.subscribers = make(map[string]*subscriptionContext)

	// Note: AsyncLogSystem handles log file management automatically

	// Note: We intentionally do NOT close the buffer here.
	// The buffer should remain readable to allow viewing historical logs
	// after job completion. The buffer will be closed when the entire
	// adapter is shut down.
}
