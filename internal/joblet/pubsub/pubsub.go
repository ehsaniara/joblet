package pubsub

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// PubSub provides in-memory publish-subscribe messaging for single-machine operation.
// It uses Go channels for message delivery and maintains topic subscriptions in memory.
//
//counterfeiter:generate . PubSub
type PubSub[T any] interface {
	// Publish sends a message to the specified topic.
	Publish(ctx context.Context, topic string, message T) error

	// Subscribe creates a subscription to the specified topic.
	Subscribe(ctx context.Context, topic string) (<-chan Message[T], func(), error)

	// Close gracefully shuts down the pub-sub system.
	Close() error

	// Health returns the current health status.
	Health(ctx context.Context) error
}

// Message represents a published message with metadata.
type Message[T any] struct {
	// ID is a unique identifier for this message.
	ID string

	// Topic is the topic this message was published to.
	Topic string

	// Payload is the actual message content.
	Payload T

	// Timestamp when the message was published.
	Timestamp time.Time

	// Attributes contains optional message metadata.
	Attributes map[string]string
}

// Config contains configuration options for the pub-sub system.
type PubSubConfig struct {
	// BufferSize for each subscription channel.
	BufferSize int `yaml:"buffer_size" json:"buffer_size"`
}

// TopicStats provides statistics about a topic.
type TopicStats struct {
	Topic           string
	MessageCount    int64
	SubscriberCount int
	LastMessageTime *time.Time
	BytesPublished  int64
}

// SubscriptionStats provides statistics about a subscription.
type SubscriptionStats struct {
	Topic            string
	SubscriberID     string
	MessagesReceived int64
	LastMessageTime  *time.Time
	IsActive         bool
}

// memoryPubSub provides the in-memory implementation.
type memoryPubSub[T any] struct {
	topics      map[string]*topic[T]
	topicsMutex sync.RWMutex
	bufferSize  int
	closed      bool
	closeMutex  sync.RWMutex
	messageID   int64
	idMutex     sync.Mutex
}

// topic represents a single topic with its subscribers.
type topic[T any] struct {
	name        string
	subscribers map[string]*subscriber[T]
	subMutex    sync.RWMutex
	stats       *TopicStats
	statsMutex  sync.RWMutex
}

// subscriber represents a single subscription to a topic.
type subscriber[T any] struct {
	id         string
	channel    chan Message[T]
	cancel     context.CancelFunc
	stats      *SubscriptionStats
	statsMutex sync.RWMutex
}

// NewPubSub creates a new in-memory pub-sub system.
func NewPubSub[T any](config *PubSubConfig) PubSub[T] {
	bufferSize := 10 // default
	if config != nil && config.BufferSize > 0 {
		bufferSize = config.BufferSize
	}

	return &memoryPubSub[T]{
		topics:     make(map[string]*topic[T]),
		bufferSize: bufferSize,
	}
}

// Publish sends a message to the specified topic.
func (p *memoryPubSub[T]) Publish(ctx context.Context, topicName string, message T) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	p.closeMutex.RLock()
	if p.closed {
		p.closeMutex.RUnlock()
		return ErrPublisherClosed
	}
	p.closeMutex.RUnlock()

	// Get or create topic
	t := p.getOrCreateTopic(topicName)

	// Create message
	msg := Message[T]{
		ID:         fmt.Sprintf("%d", p.nextMessageID()),
		Topic:      topicName,
		Payload:    message,
		Timestamp:  time.Now(),
		Attributes: make(map[string]string),
	}

	// Send to all subscribers
	t.subMutex.RLock()
	subscribers := make([]*subscriber[T], 0, len(t.subscribers))
	for _, sub := range t.subscribers {
		subscribers = append(subscribers, sub)
	}
	t.subMutex.RUnlock()

	// Update topic stats
	t.statsMutex.Lock()
	t.stats.MessageCount++
	t.stats.BytesPublished += int64(len(fmt.Sprintf("%+v", message))) // rough estimate
	now := time.Now()
	t.stats.LastMessageTime = &now
	t.statsMutex.Unlock()

	// Deliver to subscribers (non-blocking)
	for _, sub := range subscribers {
		select {
		case sub.channel <- msg:
			// Update subscriber stats
			sub.statsMutex.Lock()
			sub.stats.MessagesReceived++
			sub.stats.LastMessageTime = &now
			sub.statsMutex.Unlock()
		default:
			// Channel is full, skip this subscriber
		}
	}

	return nil
}

// Subscribe creates a subscription to the specified topic.
func (p *memoryPubSub[T]) Subscribe(ctx context.Context, topicName string) (<-chan Message[T], func(), error) {
	p.closeMutex.RLock()
	if p.closed {
		p.closeMutex.RUnlock()
		return nil, nil, ErrSubscriberClosed
	}
	p.closeMutex.RUnlock()

	// Get or create topic
	t := p.getOrCreateTopic(topicName)

	// Create subscriber
	subCtx, cancel := context.WithCancel(ctx)
	subscriberID := fmt.Sprintf("sub_%d_%d", time.Now().UnixNano(), p.nextMessageID())

	sub := &subscriber[T]{
		id:      subscriberID,
		channel: make(chan Message[T], p.bufferSize),
		cancel:  cancel,
		stats: &SubscriptionStats{
			Topic:        topicName,
			SubscriberID: subscriberID,
			IsActive:     true,
		},
	}

	// Add to topic
	t.subMutex.Lock()
	t.subscribers[subscriberID] = sub
	t.subMutex.Unlock()

	// Update topic stats
	t.statsMutex.Lock()
	t.stats.SubscriberCount = len(t.subscribers)
	t.statsMutex.Unlock()

	unsubscribe := func() {
		cancel()

		// Remove from topic
		t.subMutex.Lock()
		if _, exists := t.subscribers[subscriberID]; exists {
			delete(t.subscribers, subscriberID)
			close(sub.channel)
		}
		t.subMutex.Unlock()

		// Update subscriber stats
		sub.statsMutex.Lock()
		sub.stats.IsActive = false
		sub.statsMutex.Unlock()

		// Update topic stats
		t.statsMutex.Lock()
		t.stats.SubscriberCount = len(t.subscribers)
		t.statsMutex.Unlock()
	}

	// Handle context cancellation
	go func() {
		<-subCtx.Done()
		unsubscribe()
	}()

	return sub.channel, unsubscribe, nil
}

// Close gracefully shuts down the pub-sub system.
func (p *memoryPubSub[T]) Close() error {
	p.closeMutex.Lock()
	defer p.closeMutex.Unlock()

	if p.closed {
		return nil
	}

	p.closed = true

	// Close all topics and subscribers
	p.topicsMutex.Lock()
	defer p.topicsMutex.Unlock()

	for _, t := range p.topics {
		t.subMutex.Lock()
		for _, sub := range t.subscribers {
			sub.cancel()
			close(sub.channel)
		}
		t.subscribers = make(map[string]*subscriber[T])
		t.subMutex.Unlock()
	}

	p.topics = make(map[string]*topic[T])
	return nil
}

// Health returns the current health status.
func (p *memoryPubSub[T]) Health(ctx context.Context) error {
	p.closeMutex.RLock()
	defer p.closeMutex.RUnlock()

	if p.closed {
		return ErrPublisherClosed
	}

	return nil
}

// Helper methods

func (p *memoryPubSub[T]) getOrCreateTopic(name string) *topic[T] {
	p.topicsMutex.RLock()
	if t, exists := p.topics[name]; exists {
		p.topicsMutex.RUnlock()
		return t
	}
	p.topicsMutex.RUnlock()

	// Create new topic
	p.topicsMutex.Lock()
	defer p.topicsMutex.Unlock()

	// Double-check after acquiring write lock
	if t, exists := p.topics[name]; exists {
		return t
	}

	t := &topic[T]{
		name:        name,
		subscribers: make(map[string]*subscriber[T]),
		stats: &TopicStats{
			Topic:           name,
			MessageCount:    0,
			SubscriberCount: 0,
			BytesPublished:  0,
		},
	}

	p.topics[name] = t
	return t
}

func (p *memoryPubSub[T]) nextMessageID() int64 {
	p.idMutex.Lock()
	defer p.idMutex.Unlock()
	p.messageID++
	return p.messageID
}
