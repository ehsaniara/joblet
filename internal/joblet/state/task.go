package state

import (
	"bytes"
	"context"
	"joblet/internal/joblet/domain"
	"joblet/pkg/logger"
	"sync"
	"time"
)

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

// Update used for pub/sub
type Update struct {
	JobID    string
	LogChunk []byte
	Status   string
}

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

func (t *Task) GetJob() *domain.Job {
	t.jobMu.RLock()
	defer t.jobMu.RUnlock()

	return t.job.DeepCopy()
}

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

func (t *Task) IsRunning() bool {
	t.jobMu.RLock()
	defer t.jobMu.RUnlock()

	isRunning := t.job.IsRunning()

	return isRunning
}

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
