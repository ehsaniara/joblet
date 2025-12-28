package adapters

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ehsaniara/joblet/internal/joblet/domain"
	"github.com/ehsaniara/joblet/internal/joblet/interfaces"
	"github.com/ehsaniara/joblet/internal/joblet/pubsub"
	"github.com/ehsaniara/joblet/internal/joblet/state"
	pb "github.com/ehsaniara/joblet/internal/proto/gen/persist"
	"github.com/ehsaniara/joblet/pkg/logger"
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
// It provides job storage with buffer management and pub-sub capabilities.
// Logs are buffered in-memory for real-time streaming and forwarded to persist via IPC.
type jobStoreAdapter struct {
	// Generic storage backends
	jobStore jobStore[string, *domain.Job]
	logMgr   *SimpleLogManager
	pubsub   pubsub.PubSub[JobEvent]

	// Persist client for deleting historical logs and metrics
	persistClient pb.PersistServiceClient

	// State client for persistent job state across restarts
	stateClient state.StateClient

	// Task management (maintains compatibility with current Task-based approach)
	tasks      map[string]*taskWrapper
	tasksMutex sync.RWMutex

	logger         *logger.Logger
	closed         bool
	closeMutex     sync.RWMutex
	persistEnabled bool // If false, skip all buffering (live streaming only)
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
	JobUUID   string            `json:"job_uuid"`
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
	persistClient pb.PersistServiceClient,
	stateClient state.StateClient,
	persistEnabled bool,
	logger *logger.Logger,
) JobStorer {
	if logger == nil {
		logger = logger.WithField("component", "job-store-adapter")
	}

	return &jobStoreAdapter{
		jobStore:       store,
		logMgr:         logMgr,
		pubsub:         pubsub,
		persistClient:  persistClient,
		stateClient:    stateClient,
		persistEnabled: persistEnabled,
		tasks:          make(map[string]*taskWrapper),
		logger:         logger,
	}
}

// CreateNewJob adds a new job to the store with complete initialization.
// Sets up the job record, creates a buffer for log storage, initializes task wrapper,
// and publishes a creation event. Handles resource creation failures gracefully.
func (a *jobStoreAdapter) CreateNewJob(job *domain.Job) {

	a.closeMutex.RLock()
	if a.closed {
		a.closeMutex.RUnlock()
		a.logger.Warn("attempted to create job on closed store", "job_uuid", job.Uuid)
		return
	}
	a.closeMutex.RUnlock()

	// Store the job
	ctx := context.Background()
	if err := a.jobStore.Create(ctx, job.Uuid, job); err != nil {
		if IsConflictError(err) {
			a.logger.Warn("job already exists, not creating new task", "job_uuid", job.Uuid)
			return
		}
		a.logger.Error("failed to create job in store", "job_uuid", job.Uuid, "error", err)
		return
	}

	logBuffer := a.logMgr.GetBuffer(job.Uuid)
	a.logger.Debug("log buffer created successfully for job", "job_uuid", job.Uuid)

	task := &taskWrapper{
		job:         job.DeepCopy(),
		logBuffer:   logBuffer,
		subscribers: make(map[string]*subscriptionContext),
		logger:      a.logger.WithField("job_uuid", job.Uuid),
		pubsub:      a.pubsub,
	}

	// Store task wrapper
	a.tasksMutex.Lock()
	a.tasks[job.Uuid] = task
	a.tasksMutex.Unlock()

	// Publish creation event
	if err := a.publishEvent(JobEvent{
		Type:      "CREATED",
		JobUUID:   job.Uuid,
		Status:    string(job.Status),
		Timestamp: time.Now().Unix(),
	}); err != nil {
		a.logger.Warn("failed to publish job creation event", "job_uuid", job.Uuid, "error", err)
	}

	// Persist to state backend (async fire-and-forget)
	if a.stateClient != nil {
		// Create a copy to avoid data races if caller modifies the job
		jobCopy := *job
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := a.stateClient.Create(ctx, &jobCopy); err != nil {
				a.logger.Error("failed to persist job state", "job_uuid", jobCopy.Uuid, "error", err)
			} else {
				a.logger.Debug("job state persisted successfully", "job_uuid", jobCopy.Uuid)
			}
		}()
	}

	a.logger.Debug("job created successfully", "job_uuid", job.Uuid, "status", string(job.Status))
}

// UpdateJob updates an existing job's state and publishes changes.
// Compares old and new status, persists changes to store, publishes update events,
// and cleans up subscribers when job reaches a completed state.
func (a *jobStoreAdapter) UpdateJob(job *domain.Job) {

	a.closeMutex.RLock()
	if a.closed {
		a.closeMutex.RUnlock()
		a.logger.Warn("attempted to update job on closed store", "job_uuid", job.Uuid)
		return
	}
	a.closeMutex.RUnlock()

	a.tasksMutex.RLock()
	task, exists := a.tasks[job.Uuid]
	if !exists {
		a.tasksMutex.RUnlock()
		a.logger.Warn("attempted to update non-existent job", "job_uuid", job.Uuid)
		return
	}
	oldStatus := string(task.job.Status)
	a.tasksMutex.RUnlock()

	newStatus := string(job.Status)

	// Update in store
	ctx := context.Background()
	if err := a.jobStore.Update(ctx, job.Uuid, job); err != nil {
		a.logger.Error("failed to update job in store", "job_uuid", job.Uuid, "error", err)
		return
	}

	// Update task wrapper with write lock
	a.tasksMutex.Lock()
	task, exists = a.tasks[job.Uuid]
	if exists {
		task.job = job.DeepCopy()
	}
	a.tasksMutex.Unlock()

	// Publish update event
	if err := a.publishEvent(JobEvent{
		Type:      "UPDATED",
		JobUUID:   job.Uuid,
		Status:    newStatus,
		Timestamp: time.Now().Unix(),
	}); err != nil {
		a.logger.Warn("failed to publish job update event", "job_uuid", job.Uuid, "error", err)
	}

	// Persist state update (async fire-and-forget)
	if a.stateClient != nil {
		// Create a copy to avoid data races if caller modifies the job
		jobCopy := *job
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := a.stateClient.Update(ctx, &jobCopy); err != nil {
				a.logger.Error("failed to update job state", "job_uuid", jobCopy.Uuid, "error", err)
			} else {
				a.logger.Debug("job state updated successfully", "job_uuid", jobCopy.Uuid)
			}
		}()
	}

	// Don't cleanup subscribers immediately - let them drain final log chunks
	// Subscribers will terminate themselves after receiving UPDATED event and draining
	if job.IsCompleted() {
		a.logger.Debug("job completed, subscribers will drain and cleanup", "job_uuid", job.Uuid, "finalStatus", newStatus)
		// Cleanup will happen automatically after drain deadline in subscribeToJobUpdates
	}

	a.logger.Debug("job updated successfully", "job_uuid", job.Uuid, "oldStatus", oldStatus, "newStatus", newStatus)
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
		a.logger.Error("failed to get job from store", "job_uuid", id, "error", err)
		return nil, false
	}

	if exists {
		a.logger.Debug("job retrieved successfully", "job_uuid", id, "status", string(job.Status))
		return job.DeepCopy(), true
	}

	a.logger.Debug("job not found", "job_uuid", id)
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
			a.logger.Debug("found unique job by prefix", "prefix", prefix, "job_uuid", matches[0].Uuid)
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
// When persist is enabled: Buffers data + publishes to pubsub (for IPC forwarding and live streaming)
// When persist is disabled: Only publishes to pubsub (live streaming only, no buffering)
// Supports UUID prefix resolution.
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
	if !exists {
		a.tasksMutex.RUnlock()
		a.logger.Warn("attempted to write to buffer for non-existent job", "job_uuid", resolvedUuid, "chunkSize", len(chunk))
		return
	}

	// Capture logBuffer pointer while holding lock to avoid race conditions
	logBuffer := task.logBuffer
	a.tasksMutex.RUnlock()

	// Only write to buffer if persist is enabled (gap prevention)
	// When persist is disabled, skip buffering to avoid unbounded growth
	if a.persistEnabled && logBuffer != nil {
		if err := logBuffer.Write(chunk); err != nil {
			a.logger.Error("failed to write to job log buffer", "job_uuid", resolvedUuid, "error", err)
			return
		}
		a.logger.Debug("successfully wrote to buffer", "job_uuid", resolvedUuid, "chunkSize", len(chunk))
	} else if !a.persistEnabled {
		a.logger.Debug("persist disabled - skipping buffer write (live streaming only)", "job_uuid", resolvedUuid, "chunkSize", len(chunk))
	}

	// Always publish to pubsub for live streaming (and IPC forwarding when enabled)
	if err := a.publishEvent(JobEvent{
		Type:      "LOG_CHUNK",
		JobUUID:   resolvedUuid,
		LogChunk:  chunk,
		Timestamp: time.Now().Unix(),
	}); err != nil {
		a.logger.Warn("failed to publish log chunk event", "job_uuid", resolvedUuid, "error", err)
	}

	a.logger.Debug("log chunk written", "job_uuid", resolvedUuid, "chunkSize", len(chunk))
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
	if !exists {
		a.tasksMutex.RUnlock()
		a.logger.Debug("output requested for non-existent job", "job_uuid", resolvedUuid)
		return nil, false, fmt.Errorf("job not found")
	}

	// Capture values while holding lock to avoid race conditions
	logBuffer := task.logBuffer
	isRunning := task.job.IsRunning()
	a.tasksMutex.RUnlock()

	if logBuffer == nil {
		return []byte{}, isRunning, nil
	}

	chunks := logBuffer.ReadAll()
	if len(chunks) == 0 {
		a.logger.Debug("no data available in job log buffer", "job_uuid", id)
		return []byte{}, isRunning, nil
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

	a.logger.Debug("job output retrieved", "job_uuid", id, "outputSize", len(data), "isRunning", isRunning)

	return data, isRunning, nil
}

// SendUpdatesToClient streams job logs and status updates to a client.
// First sends existing buffer content, then subscribes to real-time updates.
// Handles completed jobs by sending existing logs and terminating immediately.
func (a *jobStoreAdapter) SendUpdatesToClient(ctx context.Context, id string, stream interfaces.DomainStreamer) error {
	return a.SendUpdatesToClientWithSkip(ctx, id, stream, 0)
}

// SendUpdatesToClientWithSkip streams logs to client, skipping already-sent items from persist.
func (a *jobStoreAdapter) SendUpdatesToClientWithSkip(ctx context.Context, id string, stream interfaces.DomainStreamer, skipCount int) error {
	resolvedUuid, err := a.resolveUuidByPrefix(id)
	if err != nil {
		a.logger.Warn("failed to resolve job UUID", "input", id, "error", err)
		return fmt.Errorf("job not found")
	}

	a.tasksMutex.RLock()
	task, exists := a.tasks[resolvedUuid]
	if !exists {
		a.tasksMutex.RUnlock()
		a.logger.Warn("stream requested for non-existent job", "job_uuid", resolvedUuid)
		return fmt.Errorf("job not found")
	}

	// Capture values while holding lock to avoid race conditions
	logBuffer := task.logBuffer
	isCompleted := task.job.IsCompleted()
	a.tasksMutex.RUnlock()

	// Send existing buffer content, skipping items already sent by persist
	// ONLY when persist is enabled - otherwise skip buffer entirely to avoid stale data
	if a.persistEnabled && logBuffer != nil {
		var chunks [][]byte
		if skipCount > 0 {
			chunks = logBuffer.ReadAfterSkip(skipCount)
			a.logger.Debug("reading buffer with skip", "job_uuid", id, "skipCount", skipCount, "remainingChunks", len(chunks))
		} else {
			chunks = logBuffer.ReadAll()
		}

		if len(chunks) > 0 {
			for _, chunk := range chunks {
				if err := stream.SendData(chunk); err != nil {
					a.logger.Warn("failed to send existing log chunk", "job_uuid", id, "error", err)
					return err
				}
			}
			a.logger.Debug("sent existing logs", "job_uuid", id, "chunkCount", len(chunks), "skipped", skipCount)
		}
	} else if !a.persistEnabled {
		a.logger.Debug("persist disabled - skipping buffer read (live streaming only)", "job_uuid", id)
	}

	// If job is completed, we're done
	if isCompleted {
		a.logger.Debug("job is completed, finishing stream", "job_uuid", id)
		return nil
	}

	return a.subscribeToJobUpdates(ctx, resolvedUuid, stream)
}

// PubSub returns the pub-sub instance for external integration (e.g., IPC)
func (a *jobStoreAdapter) PubSub() pubsub.PubSub[JobEvent] {
	return a.pubsub
}

// SyncFromPersistentState restores jobs from state storage on startup.
func (a *jobStoreAdapter) SyncFromPersistentState(ctx context.Context) error {
	if a.stateClient == nil {
		a.logger.Warn("state client not available, cannot sync jobs from persistent storage")
		return fmt.Errorf("state client not available")
	}

	a.logger.Info("syncing jobs from persistent state storage")

	// List all jobs from persistent storage
	jobs, err := a.stateClient.List(ctx, nil)
	if err != nil {
		a.logger.Error("failed to list jobs from persistent state", "error", err)
		return fmt.Errorf("failed to sync from persistent state: %w", err)
	}

	if len(jobs) == 0 {
		a.logger.Info("no jobs found in persistent state storage")
		return nil
	}

	a.logger.Info("loading jobs from persistent state", "count", len(jobs))

	// Load each job into memory
	successCount := 0
	for _, job := range jobs {
		// Store in in-memory job store
		if err := a.jobStore.Create(ctx, job.Uuid, job); err != nil {
			a.logger.Warn("failed to restore job from persistent state", "job_uuid", job.Uuid, "error", err)
			continue
		}

		// Create task wrapper for job management
		logBuffer := a.logMgr.GetBuffer(job.Uuid)
		task := &taskWrapper{
			job:         job.DeepCopy(),
			logBuffer:   logBuffer,
			subscribers: make(map[string]*subscriptionContext),
			logger:      a.logger.WithField("job_uuid", job.Uuid),
			pubsub:      a.pubsub,
		}

		a.tasksMutex.Lock()
		a.tasks[job.Uuid] = task
		a.tasksMutex.Unlock()

		successCount++
		a.logger.Debug("restored job from persistent state", "job_uuid", job.Uuid, "status", string(job.Status))
	}

	a.logger.Info("successfully synced jobs from persistent state",
		"total", len(jobs),
		"success", successCount,
		"failed", len(jobs)-successCount)

	return nil
}

// Close gracefully shuts down the adapter and releases resources.
// Stops cleanup routines, closes all subscriptions, and closes all backend
// resources (buffer manager, pubsub, job store).
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

	// Logs managed by IPC → persist subprocess (no async system to close)
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

// ResolveJobUUID resolves a job ID (short or full UUID) to a full UUID
// Implements the JobStorer interface
func (a *jobStoreAdapter) ResolveJobUUID(idOrPrefix string) (string, error) {
	return a.resolveJobUuid(idOrPrefix, "ResolveJobUUID")
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
		JobUUID:   jobID,
		Timestamp: time.Now().Unix(),
		Metadata:  metadata,
	}

	if err := a.publishEvent(event); err != nil {
		a.logger.Warn("failed to publish job event", "type", eventType, "job_uuid", jobID, "error", err)
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

	// Log files are managed by persist subprocess - request deletion via persist gRPC service
	if a.persistClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		resp, err := a.persistClient.DeleteJob(ctx, &pb.DeleteJobRequest{
			JobUuid: resolvedUuid,
		})
		if err != nil {
			a.logger.Warn("failed to delete logs from persist storage", "job_uuid", resolvedUuid, "error", err)
			return fmt.Errorf("failed to delete logs from persist storage: %w", err)
		}

		if !resp.Success {
			a.logger.Warn("persist reported log deletion failure", "job_uuid", resolvedUuid, "message", resp.Message)
			return fmt.Errorf("persist log deletion failed: %s", resp.Message)
		}

		a.logger.Info("successfully deleted logs from persist storage", "job_uuid", resolvedUuid)
	} else {
		a.logger.Warn("persist client not available - cannot delete historical log files", "job_uuid", resolvedUuid)
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

	a.logger.Info("complete job deletion requested", "job_uuid", resolvedUuid)

	if err := a.validateJobNotRunning(job); err != nil {
		a.logger.Warn("cannot delete running job", "job_uuid", resolvedUuid, "status", job.Status)
		return err
	}

	// Cleanup operations
	if err := a.cleanupTaskWrapper(resolvedUuid); err != nil {
		a.logger.Warn("task cleanup failed", "job_uuid", resolvedUuid, "error", err)
	}

	// Logs are preserved and must be deleted manually if needed

	// Remove from store
	if err := a.jobStore.Delete(context.Background(), resolvedUuid); err != nil {
		a.logger.Error("failed to delete job from store", "job_uuid", resolvedUuid, "error", err)
		return fmt.Errorf("failed to delete job from store: %w", err)
	}

	// Delete from state backend (async fire-and-forget)
	if a.stateClient != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := a.stateClient.Delete(ctx, resolvedUuid); err != nil {
				a.logger.Error("failed to delete job state", "job_uuid", resolvedUuid, "error", err)
			} else {
				a.logger.Debug("job state deleted successfully", "job_uuid", resolvedUuid)
			}
		}()
	}

	// Publish event
	_ = a.publishJobEvent("DELETED", resolvedUuid, map[string]string{"reason": "user_requested"})

	a.logger.Info("job deletion completed successfully", "job_uuid", resolvedUuid)
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
// Uses a single "jobs" topic for all jobs. Subscribers filter by JobID.
// Logs success/failure for debugging and monitoring.
func (a *jobStoreAdapter) publishEvent(event JobEvent) error {
	ctx := context.Background()
	// Publish to single "jobs" topic for all jobs
	topic := "jobs"
	a.logger.Debug("publishing event", "job_uuid", event.JobUUID, "eventType", event.Type, "topic", topic, "chunkSize", len(event.LogChunk))

	err := a.pubsub.Publish(ctx, topic, event)
	if err != nil {
		a.logger.Error("failed to publish event", "job_uuid", event.JobUUID, "eventType", event.Type, "topic", topic, "error", err)
	} else {
		a.logger.Debug("successfully published event", "job_uuid", event.JobUUID, "eventType", event.Type, "topic", topic)
	}

	return err
}

// subscribeToJobUpdates creates a real-time subscription for job events.
// Handles LOG_CHUNK events by streaming data to client, and UPDATED events by checking
// for job completion. Manages subscription lifecycle and cleanup automatically.
func (a *jobStoreAdapter) subscribeToJobUpdates(ctx context.Context, jobID string, stream interfaces.DomainStreamer) error {
	// Subscribe to single "jobs" topic (filter by JobID in loop)
	topic := "jobs"
	a.logger.Debug("subscribing to job events for streaming", "job_uuid", jobID, "topic", topic)

	updates, unsubscribe, err := a.pubsub.Subscribe(ctx, topic)
	if err != nil {
		a.logger.Error("failed to subscribe to job events", "job_uuid", jobID, "topic", topic, "error", err)
		return fmt.Errorf("failed to subscribe to job events: %w", err)
	}

	a.logger.Debug("successfully subscribed to job events", "job_uuid", jobID, "topic", topic)

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

	// Register subscriber with proper locking to avoid race with task deletion
	a.tasksMutex.RLock()
	task, exists := a.tasks[jobID]
	if !exists {
		a.tasksMutex.RUnlock()
		unsubscribe()
		cancel()
		return fmt.Errorf("job no longer exists")
	}
	task.subMutex.Lock()
	task.subscribers[subID] = subContext
	task.subMutex.Unlock()
	a.tasksMutex.RUnlock()

	// Create a channel to signal when subscription ends
	done := make(chan error, 1)

	// Handle updates in a separate goroutine
	go func() {
		a.logger.Debug("started subscription handler goroutine", "job_uuid", jobID, "subId", subID)
		defer func() {
			a.logger.Debug("cleaning up subscription", "job_uuid", jobID, "subId", subID)
			unsubscribe()
			cancel()

			// Clean up subscriber with proper locking to avoid race with task deletion
			a.tasksMutex.RLock()
			if task, exists := a.tasks[jobID]; exists {
				task.subMutex.Lock()
				delete(task.subscribers, subID)
				task.subMutex.Unlock()
			}
			a.tasksMutex.RUnlock()

			a.logger.Debug("subscription cleaned up", "job_uuid", jobID, "subId", subID)

			// Signal that we're done
			close(done)
		}()

		jobCompleted := false
		drainDeadline := time.Time{}

		for {
			// If job completed, check if we've exceeded drain deadline
			if jobCompleted && time.Now().After(drainDeadline) {
				a.logger.Debug("drain deadline exceeded, ending stream", "job_uuid", jobID)
				done <- nil
				return
			}

			// Compute select timeout based on whether we're draining
			var selectTimeout <-chan time.Time
			if jobCompleted {
				// During drain, use short timeout to check deadline frequently
				selectTimeout = time.After(50 * time.Millisecond)
			}

			select {
			case <-subCtx.Done():
				a.logger.Debug("subscription context cancelled", "job_uuid", jobID, "subId", subID)
				done <- nil
				return
			case <-stream.Context().Done():
				a.logger.Debug("stream context cancelled", "job_uuid", jobID, "subId", subID)
				done <- nil
				return
			case <-selectTimeout:
				// Timeout during drain phase - continue to check deadline
				continue
			case msg, ok := <-updates:
				if !ok {
					a.logger.Debug("updates channel closed", "job_uuid", jobID, "subId", subID)
					done <- nil
					return
				}

				event := msg.Payload

				// Filter events for this specific job (all jobs use the same topic)
				if event.JobUUID != jobID {
					continue
				}

				a.logger.Debug("received event from pubsub", "job_uuid", jobID, "eventType", event.Type, "chunkSize", len(event.LogChunk))

				// Handle different event types
				switch event.Type {
				case "LOG_CHUNK":
					if len(event.LogChunk) > 0 {
						a.logger.Debug("sending log chunk to client", "job_uuid", jobID, "chunkSize", len(event.LogChunk))
						if err := stream.SendData(event.LogChunk); err != nil {
							a.logger.Warn("failed to send log chunk to client", "job_uuid", jobID, "error", err)
							done <- err
							return
						}
						a.logger.Debug("successfully sent log chunk to client", "job_uuid", jobID, "chunkSize", len(event.LogChunk))
					}
				case "UPDATED":
					a.logger.Debug("received job status update", "job_uuid", jobID, "status", event.Status)
					// When job reaches final status, enter drain mode instead of immediately terminating
					if event.Status == "COMPLETED" || event.Status == "FAILED" || event.Status == "STOPPED" {
						if !jobCompleted {
							jobCompleted = true
							// Set drain deadline to allow final log chunks to arrive
							drainDeadline = time.Now().Add(500 * time.Millisecond)
							a.logger.Debug("job completed, entering drain mode", "job_uuid", jobID, "finalStatus", event.Status, "drainDeadline", drainDeadline)
						}
					}
				}
			}
		}
	}()

	// Wait for the subscription to end
	a.logger.Debug("waiting for subscription to complete", "job_uuid", jobID)
	err = <-done
	a.logger.Debug("subscription completed", "job_uuid", jobID, "error", err)

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

// HealthCheckServices verifies state and persist services are healthy.
func (a *jobStoreAdapter) HealthCheckServices(ctx context.Context) error {
	if a.stateClient == nil {
		return fmt.Errorf("state client is nil")
	}

	if err := a.stateClient.Ping(ctx); err != nil {
		return fmt.Errorf("state service health check failed: %w", err)
	}

	a.logger.Info("state service health check passed (Ping)")

	// Check persist service (if enabled)
	if a.persistEnabled {
		// If client is nil, connect to it now (deferred from initialization)
		if a.persistClient == nil {
			persistClient, err := WaitForPersistService("/opt/joblet/run/persist-grpc.sock", a.logger)
			if err != nil {
				return fmt.Errorf("persist service connection failed: %w", err)
			}
			a.persistClient = persistClient
		}

		// Now do the Ping health check
		if _, err := a.persistClient.Ping(ctx, &pb.PingRequest{}); err != nil {
			return fmt.Errorf("persist service health check failed: %w", err)
		}
		a.logger.Info("persist service health check passed (Ping)")
	}

	return nil
}
