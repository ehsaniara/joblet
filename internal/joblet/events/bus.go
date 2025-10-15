package events

import (
	"context"
	"sync"

	"github.com/ehsaniara/joblet/internal/joblet/domain"
	"github.com/ehsaniara/joblet/pkg/errors"
)

// EventType represents different event types in the system
type EventType string

const (
	JobStarted      EventType = "job.started"
	JobCompleted    EventType = "job.completed"
	JobFailed       EventType = "job.failed"
	JobStopped      EventType = "job.stopped"
	VolumeCreated   EventType = "volume.created"
	VolumeDeleted   EventType = "volume.deleted"
	NetworkSetup    EventType = "network.setup"
	NetworkTornDown EventType = "network.torn_down"
)

// Event represents a system event
type Event struct {
	Type      EventType
	JobID     string
	Data      interface{}
	Timestamp int64
}

// EventHandler handles system events
type EventHandler interface {
	Handle(ctx context.Context, event Event) error
	SupportedEvents() []EventType
}

// EventBus manages event publishing and subscription
type EventBus interface {
	Publish(ctx context.Context, event Event) error
	Subscribe(eventType EventType, handler EventHandler) error
	Unsubscribe(eventType EventType, handler EventHandler) error
}

// InMemoryEventBus is a simple in-memory event bus implementation
type InMemoryEventBus struct {
	handlers map[EventType][]EventHandler
	mutex    sync.RWMutex
}

// NewInMemoryEventBus creates a new in-memory event bus for decoupled component communication
func NewInMemoryEventBus() *InMemoryEventBus {
	return &InMemoryEventBus{
		handlers: make(map[EventType][]EventHandler),
	}
}

// Publish sends an event to all registered handlers concurrently
func (b *InMemoryEventBus) Publish(ctx context.Context, event Event) error {
	b.mutex.RLock()
	handlers, exists := b.handlers[event.Type]
	b.mutex.RUnlock()

	if !exists {
		return nil
	}
	var wg sync.WaitGroup
	errs := make([]error, 0)
	errorMutex := sync.Mutex{}

	for _, handler := range handlers {
		wg.Add(1)
		go func(h EventHandler) {
			defer wg.Done()
			if err := h.Handle(ctx, event); err != nil {
				errorMutex.Lock()
				errs = append(errs, err)
				errorMutex.Unlock()
			}
		}(handler)
	}

	wg.Wait()

	if len(errs) > 0 {
		return errors.JoinErrors(errs...)
	}

	return nil
}

// Subscribe registers an event handler to receive events of a specific type
func (b *InMemoryEventBus) Subscribe(eventType EventType, handler EventHandler) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if b.handlers[eventType] == nil {
		b.handlers[eventType] = make([]EventHandler, 0)
	}

	b.handlers[eventType] = append(b.handlers[eventType], handler)
	return nil
}

// Unsubscribe removes an event handler from receiving events of a specific type
func (b *InMemoryEventBus) Unsubscribe(eventType EventType, handler EventHandler) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	handlers, exists := b.handlers[eventType]
	if !exists {
		return nil
	}

	for i, h := range handlers {
		if h == handler {
			b.handlers[eventType] = append(handlers[:i], handlers[i+1:]...)
			break
		}
	}

	return nil
}

// JobEventData contains job-specific event data
type JobEventData struct {
	Job    *domain.Job
	Reason string
	Error  error
}

// VolumeEventData contains volume-specific event data
type VolumeEventData struct {
	VolumeName string
	Size       int64
	MountPath  string
}

// NetworkEventData contains network-specific event data
type NetworkEventData struct {
	NetworkName string
	JobID       string
	Config      interface{} // Generic config to avoid import coupling
}
