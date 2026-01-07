package telemetry

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ehsaniara/joblet/pkg/logger"
)

//counterfeiter:generate . EventPersister

// EventPersister is an interface for persisting telemetry events to storage.
// This is typically implemented by the IPC writer to send events to persist.
type EventPersister interface {
	// PersistExecEvent persists a process execution event
	PersistExecEvent(jobID string, timestamp int64, sequence uint64, data *ExecData) error
	// PersistConnectEvent persists a network connection event
	PersistConnectEvent(jobID string, timestamp int64, sequence uint64, data *ConnectData) error
	// PersistMetrics persists resource metrics (CPU, memory, disk I/O, network)
	PersistMetrics(jobID string, timestamp int64, sequence uint64, data *MetricsData) error
	// PersistAcceptEvent persists an incoming connection accept event
	PersistAcceptEvent(jobID string, timestamp int64, sequence uint64, data *AcceptData) error
	// PersistSocketDataEvent persists a sendto/recvfrom event
	PersistSocketDataEvent(jobID string, timestamp int64, sequence uint64, data *SocketDataData) error
	// PersistMmapEvent persists a memory mapping event
	PersistMmapEvent(jobID string, timestamp int64, sequence uint64, data *MmapData) error
	// PersistMprotectEvent persists a memory protection change event
	PersistMprotectEvent(jobID string, timestamp int64, sequence uint64, data *MprotectData) error
	// PersistFileEvent persists a file access event
	PersistFileEvent(jobID string, timestamp int64, sequence uint64, data *FileData) error
}

// Collector manages telemetry collection and streaming for jobs.
// It aggregates events from multiple sources (metrics collector, eBPF monitor)
// and provides a unified interface for streaming telemetry to clients.
type Collector struct {
	mu         sync.RWMutex
	buffers    map[string]*eventBuffer // jobID -> buffer
	bufferSize int
	logger     *logger.Logger
	persister  EventPersister // Optional persister for storing events
	sequence   uint64         // Global sequence counter for persistence
}

// eventBuffer holds telemetry events for a single job
type eventBuffer struct {
	events    []*Event
	mu        sync.RWMutex
	listeners []chan *Event
}

// NewCollector creates a new telemetry collector.
func NewCollector(bufferSize int) *Collector {
	if bufferSize <= 0 {
		bufferSize = 100000 // Default buffer size (100k events for high-frequency eBPF)
	}
	return &Collector{
		buffers:    make(map[string]*eventBuffer),
		bufferSize: bufferSize,
		logger:     logger.WithField("component", "telemetry-collector"),
	}
}

// SetPersister sets the event persister for forwarding events to storage.
// This should be called after the IPC manager is initialized.
func (c *Collector) SetPersister(persister EventPersister) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.persister = persister
	c.logger.Info("telemetry persister configured")
}

// getOrCreateBuffer returns the event buffer for a job, creating it if necessary.
func (c *Collector) getOrCreateBuffer(jobID string) *eventBuffer {
	c.mu.Lock()
	defer c.mu.Unlock()

	if buf, exists := c.buffers[jobID]; exists {
		return buf
	}

	buf := &eventBuffer{
		events:    make([]*Event, 0, c.bufferSize),
		listeners: make([]chan *Event, 0),
	}
	c.buffers[jobID] = buf
	return buf
}

// Emit adds a telemetry event to the collector.
// The event is buffered and sent to all active listeners.
func (c *Collector) Emit(event *Event) {
	if event == nil || event.JobUUID == "" {
		return
	}

	buf := c.getOrCreateBuffer(event.JobUUID)
	buf.mu.Lock()
	defer buf.mu.Unlock()

	// Add to buffer (ring buffer behavior when full)
	if len(buf.events) >= c.bufferSize {
		// Shift events to make room (drop oldest)
		copy(buf.events, buf.events[1:])
		buf.events = buf.events[:len(buf.events)-1]
	}
	buf.events = append(buf.events, event)

	// Notify all listeners (non-blocking)
	for _, ch := range buf.listeners {
		select {
		case ch <- event:
		default:
			// Listener is slow, skip this event for them
		}
	}
}

// EmitMetrics is a convenience method to emit a metrics event.
func (c *Collector) EmitMetrics(jobID string, data *MetricsData) {
	event := NewMetricsEvent(jobID, data)
	c.Emit(event)

	// Also persist to storage if persister is configured
	c.mu.RLock()
	persister := c.persister
	c.mu.RUnlock()

	if persister != nil {
		seq := atomic.AddUint64(&c.sequence, 1)
		if err := persister.PersistMetrics(jobID, event.Timestamp.UnixNano(), seq, data); err != nil {
			c.logger.Warn("failed to persist metrics", "job_uuid", jobID, "error", err)
		}
	}
}

// EmitExec is a convenience method to emit a process execution event.
func (c *Collector) EmitExec(jobID string, data *ExecData) {
	event := NewExecEvent(jobID, data)
	c.Emit(event)

	// Also persist to storage if persister is configured
	c.mu.RLock()
	persister := c.persister
	c.mu.RUnlock()

	if persister != nil {
		seq := atomic.AddUint64(&c.sequence, 1)
		if err := persister.PersistExecEvent(jobID, event.Timestamp.UnixNano(), seq, data); err != nil {
			c.logger.Warn("failed to persist exec event", "job_uuid", jobID, "error", err)
		}
	}
}

// EmitConnect is a convenience method to emit a network connection event.
func (c *Collector) EmitConnect(jobID string, data *ConnectData) {
	event := NewConnectEvent(jobID, data)
	c.Emit(event)

	// Also persist to storage if persister is configured
	c.mu.RLock()
	persister := c.persister
	c.mu.RUnlock()

	if persister != nil {
		seq := atomic.AddUint64(&c.sequence, 1)
		if err := persister.PersistConnectEvent(jobID, event.Timestamp.UnixNano(), seq, data); err != nil {
			c.logger.Warn("failed to persist connect event", "job_uuid", jobID, "error", err)
		}
	}
}

// EmitFile is a convenience method to emit a file access event.
func (c *Collector) EmitFile(jobID string, data *FileData) {
	event := NewFileEvent(jobID, data)
	c.Emit(event)

	// Also persist to storage if persister is configured
	c.mu.RLock()
	persister := c.persister
	c.mu.RUnlock()

	if persister != nil {
		seq := atomic.AddUint64(&c.sequence, 1)
		if err := persister.PersistFileEvent(jobID, event.Timestamp.UnixNano(), seq, data); err != nil {
			c.logger.Warn("failed to persist file event", "job_uuid", jobID, "error", err)
		}
	}
}

// EmitAccept is a convenience method to emit an incoming connection accept event.
func (c *Collector) EmitAccept(jobID string, data *AcceptData) {
	event := NewAcceptEvent(jobID, data)
	c.Emit(event)

	// Also persist to storage if persister is configured
	c.mu.RLock()
	persister := c.persister
	c.mu.RUnlock()

	if persister != nil {
		seq := atomic.AddUint64(&c.sequence, 1)
		if err := persister.PersistAcceptEvent(jobID, event.Timestamp.UnixNano(), seq, data); err != nil {
			c.logger.Warn("failed to persist accept event", "job_uuid", jobID, "error", err)
		}
	}
}

// EmitSocketData is a convenience method to emit a sendto/recvfrom event.
func (c *Collector) EmitSocketData(jobID string, data *SocketDataData) {
	event := NewSocketDataEvent(jobID, data)
	c.Emit(event)

	// Also persist to storage if persister is configured
	c.mu.RLock()
	persister := c.persister
	c.mu.RUnlock()

	if persister != nil {
		seq := atomic.AddUint64(&c.sequence, 1)
		if err := persister.PersistSocketDataEvent(jobID, event.Timestamp.UnixNano(), seq, data); err != nil {
			c.logger.Warn("failed to persist socket data event", "job_uuid", jobID, "error", err)
		}
	}
}

// EmitMmap is a convenience method to emit a memory mapping event.
func (c *Collector) EmitMmap(jobID string, data *MmapData) {
	event := NewMmapEvent(jobID, data)
	c.Emit(event)

	// Also persist to storage if persister is configured
	c.mu.RLock()
	persister := c.persister
	c.mu.RUnlock()

	if persister != nil {
		seq := atomic.AddUint64(&c.sequence, 1)
		if err := persister.PersistMmapEvent(jobID, event.Timestamp.UnixNano(), seq, data); err != nil {
			c.logger.Warn("failed to persist mmap event", "job_uuid", jobID, "error", err)
		}
	}
}

// EmitMprotect is a convenience method to emit a memory protection change event.
func (c *Collector) EmitMprotect(jobID string, data *MprotectData) {
	event := NewMprotectEvent(jobID, data)
	c.Emit(event)

	// Also persist to storage if persister is configured
	c.mu.RLock()
	persister := c.persister
	c.mu.RUnlock()

	if persister != nil {
		seq := atomic.AddUint64(&c.sequence, 1)
		if err := persister.PersistMprotectEvent(jobID, event.Timestamp.UnixNano(), seq, data); err != nil {
			c.logger.Warn("failed to persist mprotect event", "job_uuid", jobID, "error", err)
		}
	}
}

// Stream streams telemetry events for a job to the provided callback.
// It first sends all buffered events, then streams live events until the context is cancelled.
func (c *Collector) Stream(ctx context.Context, jobID string, filter *EventFilter, callback func(*Event) error) error {
	buf := c.getOrCreateBuffer(jobID)

	// Create a listener channel
	ch := make(chan *Event, 100)

	// Register listener
	buf.mu.Lock()
	buf.listeners = append(buf.listeners, ch)
	// Get current buffered events
	bufferedEvents := make([]*Event, len(buf.events))
	copy(bufferedEvents, buf.events)
	buf.mu.Unlock()

	// Cleanup on exit
	defer func() {
		buf.mu.Lock()
		for i, listener := range buf.listeners {
			if listener == ch {
				buf.listeners = append(buf.listeners[:i], buf.listeners[i+1:]...)
				break
			}
		}
		buf.mu.Unlock()
		close(ch)
	}()

	// Send buffered events first
	for _, event := range bufferedEvents {
		if filter == nil || filter.Matches(event) {
			if err := callback(event); err != nil {
				return err
			}
		}
	}

	// Stream live events
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-ch:
			if !ok {
				return nil
			}
			if filter == nil || filter.Matches(event) {
				if err := callback(event); err != nil {
					return err
				}
			}
		}
	}
}

// GetBufferedEvents returns all buffered events for a job that match the filter.
func (c *Collector) GetBufferedEvents(jobID string, filter *EventFilter, startTime, endTime time.Time, limit int) []*Event {
	c.mu.RLock()
	buf, exists := c.buffers[jobID]
	c.mu.RUnlock()

	if !exists {
		return nil
	}

	buf.mu.RLock()
	defer buf.mu.RUnlock()

	var result []*Event
	for _, event := range buf.events {
		// Time filter
		if !startTime.IsZero() && event.Timestamp.Before(startTime) {
			continue
		}
		if !endTime.IsZero() && event.Timestamp.After(endTime) {
			continue
		}

		// Type filter
		if filter != nil && !filter.Matches(event) {
			continue
		}

		result = append(result, event)

		// Limit
		if limit > 0 && len(result) >= limit {
			break
		}
	}

	return result
}

// ClearJob removes all buffered events for a job.
// This should be called when a job is deleted.
func (c *Collector) ClearJob(jobID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if buf, exists := c.buffers[jobID]; exists {
		buf.mu.Lock()
		// Close all listeners
		for _, ch := range buf.listeners {
			close(ch)
		}
		buf.listeners = nil
		buf.events = nil
		buf.mu.Unlock()
	}
	delete(c.buffers, jobID)
}

// JobCount returns the number of jobs with buffered telemetry.
func (c *Collector) JobCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.buffers)
}

// EventCount returns the total number of buffered events across all jobs.
func (c *Collector) EventCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	count := 0
	for _, buf := range c.buffers {
		buf.mu.RLock()
		count += len(buf.events)
		buf.mu.RUnlock()
	}
	return count
}
