package state

import (
	"bytes"
	"context"
	"joblet/internal/joblet/domain"
	"joblet/pkg/logger"
	"sync"
	"time"
)

// Task represents a job execution context with output buffering and pub/sub capabilities.
// Provides thread-safe access to job state, output buffer, and real-time update notifications.
type Task struct {
	id string

	job   *domain.Job
	jobMu sync.RWMutex

	buffer   bytes.Buffer
	bufferMu sync.RWMutex

	subscribers map[chan Update]bool
	subMu       sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc

	logger *logger.Logger
}

// Update represents a job state or log change notification.
// Used by the pub/sub system to notify subscribers of job updates.
type Update struct {
	// JobID identifies which job this update belongs to.
	JobID string
	// LogChunk contains new log data from the job execution.
	LogChunk []byte
	// Status indicates the current job status (RUNNING, COMPLETED, etc.).
	Status string
}

// NewTask creates a new Task instance for the given job.
// Initializes the task with a deep copy of the job, context for cancellation,
// and empty subscriber map for pub/sub notifications.
func NewTask(job *domain.Job) *Task {
	jobCopy := job.DeepCopy()

	ctx, cancel := context.WithCancel(context.Background())

	taskLogger := logger.WithField("taskId", job.Id)

	return &Task{
		id:          job.Id,
		job:         jobCopy,
		subscribers: make(map[chan Update]bool),
		ctx:         ctx,
		cancel:      cancel,
		logger:      taskLogger,
	}
}

// GetJob returns a deep copy of the current job state.
// Thread-safe operation that prevents external mutations of job data.
func (t *Task) GetJob() *domain.Job {
	t.jobMu.RLock()
	defer t.jobMu.RUnlock()

	return t.job.DeepCopy()
}

// Subscribe creates a new subscription channel for job updates.
// Returns a buffered channel for updates and an unsubscribe function.
// Automatically unsubscribes when task context is cancelled.
func (t *Task) Subscribe() (chan Update, func()) {
	// small buffer for immediate delivery
	ch := make(chan Update, 2)

	t.subMu.Lock()
	subscriberCount := len(t.subscribers)
	t.subscribers[ch] = true
	t.subMu.Unlock()

	t.logger.Debug("new subscriber added", "totalSubscribers", subscriberCount+1, "channelBuffer", 2)

	unsubscribe := func() {
		t.removeSubscriber(ch)
	}

	go func() {
		<-t.ctx.Done()
		unsubscribe()
	}()

	return ch, unsubscribe
}

// removeSubscriber safely removes a subscriber channel from the task.
// Closes the channel and removes it from the subscribers map.
// Thread-safe operation with proper cleanup.
func (t *Task) removeSubscriber(ch chan Update) {
	t.subMu.Lock()
	if _, exists := t.subscribers[ch]; exists {
		delete(t.subscribers, ch)
		remainingCount := len(t.subscribers)
		t.subMu.Unlock()
		close(ch)

		t.logger.Debug("subscriber removed", "remainingSubscribers", remainingCount)
	} else {
		t.subMu.Unlock()
		t.logger.Debug("attempted to remove non-existent subscriber")
	}
}

// UpdateJob updates the task's job state with a new version.
// Creates a deep copy to prevent external mutations and logs status changes.
// Thread-safe operation with proper locking.
func (t *Task) UpdateJob(job *domain.Job) {
	jobCopy := job.DeepCopy()

	oldStatus := ""
	t.jobMu.RLock()
	if t.job != nil {
		oldStatus = string(t.job.Status)
	}
	t.jobMu.RUnlock()

	t.jobMu.Lock()
	t.job = jobCopy
	t.jobMu.Unlock()

	newStatus := string(jobCopy.Status)
	if oldStatus != newStatus {
		t.logger.Debug("job status updated", "oldStatus", oldStatus, "newStatus", newStatus)
	}
}

// Publish sends an update to all subscribers with timeout protection.
// Removes slow subscribers that don't consume updates within timeout.
// Handles both log chunks and status updates appropriately.
func (t *Task) Publish(update Update) {
	t.subMu.RLock()
	subscriberCount := len(t.subscribers)
	if subscriberCount == 0 {
		t.subMu.RUnlock()
		return
	}

	channels := make([]chan Update, 0, subscriberCount)
	for ch := range t.subscribers {
		channels = append(channels, ch)
	}
	t.subMu.RUnlock()

	timeout := 50 * time.Millisecond
	successCount := 0
	timeoutCount := 0

	for _, ch := range channels {
		select {
		case ch <- update:
			successCount++
		case <-time.After(timeout):
			// Remove slow subscriber
			timeoutCount++
			t.logger.Warn("slow subscriber detected, removing", "timeout", timeout)
			go t.removeSubscriber(ch)
		case <-t.ctx.Done():
			t.logger.Debug("publish cancelled due to context done")
			return
		}
	}

	if len(update.LogChunk) > 0 {
		t.logger.Debug("log chunk published", "subscribers", subscriberCount,
			"successful", successCount, "timedOut", timeoutCount, "chunkSize", len(update.LogChunk))
	}

	if update.Status != "" {
		t.logger.Debug("status update published", "status", update.Status, "subscribers", subscriberCount, "successful", successCount)
	}
}

// WriteToBuffer appends log data to the task's output buffer.
// Thread-safe operation that also publishes the log chunk to subscribers.
// Ignores empty log data to avoid unnecessary operations.
func (t *Task) WriteToBuffer(logData []byte) {
	if len(logData) == 0 {
		return
	}

	t.bufferMu.Lock()
	t.buffer.Write(logData)
	t.bufferMu.Unlock()

	t.Publish(Update{
		JobID:    t.id,
		LogChunk: logData,
	})
}

// GetBuffer returns a copy of the complete output buffer.
// Thread-safe operation that creates a new byte slice copy.
// Returns nil if buffer is empty.
func (t *Task) GetBuffer() []byte {
	t.bufferMu.RLock()
	defer t.bufferMu.RUnlock()

	if t.buffer.Len() == 0 {
		return nil
	}

	data := make([]byte, t.buffer.Len())
	copy(data, t.buffer.Bytes())

	t.logger.Debug("buffer contents retrieved", "bufferSize", len(data))

	return data
}

// IsRunning checks if the job is currently in running state.
// Thread-safe operation that reads the job status.
func (t *Task) IsRunning() bool {
	t.jobMu.RLock()
	defer t.jobMu.RUnlock()

	isRunning := t.job.IsRunning()

	return isRunning
}

// Shutdown gracefully terminates the task and cleans up resources.
// Cancels context, closes all subscriber channels, and clears subscriber map.
// Logs warning if subscribers need to be force-closed.
func (t *Task) Shutdown() {
	t.subMu.RLock()
	subscriberCount := len(t.subscribers)
	t.subMu.RUnlock()

	t.logger.Debug("shutting down task", "activeSubscribers", subscriberCount)

	t.cancel()

	time.Sleep(10 * time.Millisecond)

	t.subMu.Lock()
	remainingCount := len(t.subscribers)
	for ch := range t.subscribers {
		close(ch)
	}

	t.subscribers = make(map[chan Update]bool)
	t.subMu.Unlock()

	if remainingCount > 0 {
		t.logger.Warn("force closed remaining subscribers", "count", remainingCount)
	}

	t.logger.Debug("task shutdown completed")
}
