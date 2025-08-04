package buffer

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
)

// Factory provides methods to create buffer instances based on configuration.
type Factory struct{}

// NewFactory creates a new buffer factory.
func NewFactory() *Factory {
	return &Factory{}
}

// NewBuffer creates a buffer instance based on the provided configuration.
// Since joblet is Linux-only and uses memory buffers by default, this is simplified.
func (f *Factory) NewBuffer(id string, config *BufferConfig) (Buffer, error) {
	if config == nil {
		return nil, fmt.Errorf("buffer config is required")
	}

	// For now, only support memory buffers (the default and primary use case)
	// Ring and persistent buffers can be added back if actually needed
	switch config.Type {
	case "memory", "":
		return newMemoryBuffer(id, config), nil
	default:
		return nil, fmt.Errorf("unsupported buffer type: %s (only 'memory' is currently supported)", config.Type)
	}
}

// NewRingBuffer creates a ring buffer instance with the specified capacity.
// Simplified to use memory buffer with capacity limits for now.
func (f *Factory) NewRingBuffer(id string, capacity int, config *BufferConfig) (RingBuffer, error) {
	if config == nil {
		config = &BufferConfig{
			Type:            "memory",
			InitialCapacity: capacity,
			MaxCapacity:     capacity,
		}
	}

	// Use memory buffer with capacity constraints as a simple ring buffer
	config.Type = "memory"
	config.MaxCapacity = capacity

	buffer := newMemoryBuffer(id, config)

	// Memory buffer implements the RingBuffer interface
	if ringBuffer, ok := buffer.(RingBuffer); ok {
		return ringBuffer, nil
	}

	return nil, fmt.Errorf("memory buffer does not implement RingBuffer interface")
}

// NewPersistentBuffer creates a persistent buffer instance.
// Simplified implementation - currently not supported.
func (f *Factory) NewPersistentBuffer(id string, path string, config *BufferConfig) (PersistentBuffer, error) {
	return nil, fmt.Errorf("persistent buffers not currently supported - use memory buffers instead")
}

// GetSupportedTypes returns a list of all supported buffer types.
func (f *Factory) GetSupportedTypes() []string {
	return []string{"memory"}
}

// IsTypeSupported checks if a buffer type is supported.
func (f *Factory) IsTypeSupported(bufferType string) bool {
	return bufferType == "memory" || bufferType == ""
}

// memoryBuffer provides a simplified in-memory buffer implementation
type memoryBuffer struct {
	data        bytes.Buffer
	dataMutex   sync.RWMutex
	subscribers map[string]chan []byte
	subMutex    sync.RWMutex
	closed      bool
	closeMutex  sync.RWMutex
	config      *BufferConfig
	subCounter  int64
}

// newMemoryBuffer creates a new in-memory buffer
func newMemoryBuffer(id string, config *BufferConfig) Buffer {
	if config == nil {
		config = &BufferConfig{
			Type:                 "memory",
			InitialCapacity:      1024,
			MaxCapacity:          0, // unlimited
			MaxSubscribers:       0, // unlimited
			SubscriberBufferSize: 10,
			EnableMetrics:        false, // simplified - no metrics
		}
	}

	return &memoryBuffer{
		subscribers: make(map[string]chan []byte),
		config:      config,
	}
}

// Write appends data to the buffer
func (b *memoryBuffer) Write(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}

	b.closeMutex.RLock()
	if b.closed {
		b.closeMutex.RUnlock()
		return 0, ErrBufferClosed
	}
	b.closeMutex.RUnlock()

	b.dataMutex.Lock()

	// Check capacity limits
	if b.config.MaxCapacity > 0 && b.data.Len()+len(data) > b.config.MaxCapacity {
		b.dataMutex.Unlock()
		return 0, ErrBufferFull
	}

	n, err := b.data.Write(data)
	b.dataMutex.Unlock()

	if err != nil {
		return n, err
	}

	// Notify subscribers
	b.notifySubscribers(data)

	return n, nil
}

// Read returns the complete buffer contents
func (b *memoryBuffer) Read() ([]byte, error) {
	b.closeMutex.RLock()
	if b.closed {
		b.closeMutex.RUnlock()
		return nil, ErrBufferClosed
	}
	b.closeMutex.RUnlock()

	b.dataMutex.RLock()
	defer b.dataMutex.RUnlock()

	// Create a copy to prevent external mutations
	data := make([]byte, b.data.Len())
	copy(data, b.data.Bytes())

	return data, nil
}

// ReadFrom reads data from the provided reader into the buffer
func (b *memoryBuffer) ReadFrom(r io.Reader) (int64, error) {
	b.closeMutex.RLock()
	if b.closed {
		b.closeMutex.RUnlock()
		return 0, ErrBufferClosed
	}
	b.closeMutex.RUnlock()

	chunk := make([]byte, 4096)
	var totalBytes int64

	for {
		n, err := r.Read(chunk)
		if n > 0 {
			written, writeErr := b.Write(chunk[:n])
			totalBytes += int64(written)
			if writeErr != nil {
				return totalBytes, writeErr
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return totalBytes, err
		}
	}

	return totalBytes, nil
}

// Subscribe creates a subscription to new data written to the buffer
func (b *memoryBuffer) Subscribe(ctx context.Context) (<-chan []byte, func(), error) {
	b.closeMutex.RLock()
	if b.closed {
		b.closeMutex.RUnlock()
		return nil, nil, ErrBufferClosed
	}
	b.closeMutex.RUnlock()

	// Check subscriber limits
	b.subMutex.RLock()
	subscriberCount := len(b.subscribers)
	b.subMutex.RUnlock()

	if b.config.MaxSubscribers > 0 && subscriberCount >= b.config.MaxSubscribers {
		return nil, nil, ErrMaxSubscribersExceeded
	}

	// Create subscriber channel
	subID := b.nextSubscriberID()
	subChan := make(chan []byte, b.config.SubscriberBufferSize)

	b.subMutex.Lock()
	b.subscribers[subID] = subChan
	b.subMutex.Unlock()

	unsubscribe := func() {
		b.subMutex.Lock()
		if ch, exists := b.subscribers[subID]; exists {
			delete(b.subscribers, subID)
			close(ch)
		}
		b.subMutex.Unlock()
	}

	// Handle context cancellation
	go func() {
		<-ctx.Done()
		unsubscribe()
	}()

	return subChan, unsubscribe, nil
}

// Len returns the current buffer size in bytes
func (b *memoryBuffer) Len() int {
	b.dataMutex.RLock()
	defer b.dataMutex.RUnlock()
	return b.data.Len()
}

// Cap returns the buffer capacity in bytes
func (b *memoryBuffer) Cap() int {
	return b.config.MaxCapacity
}

// Clear removes all data from the buffer
func (b *memoryBuffer) Clear() error {
	b.closeMutex.RLock()
	if b.closed {
		b.closeMutex.RUnlock()
		return ErrBufferClosed
	}
	b.closeMutex.RUnlock()

	b.dataMutex.Lock()
	b.data.Reset()
	b.dataMutex.Unlock()

	return nil
}

// Close gracefully shuts down the buffer and releases resources
func (b *memoryBuffer) Close() error {
	b.closeMutex.Lock()
	defer b.closeMutex.Unlock()

	if b.closed {
		return nil
	}

	b.closed = true

	// Close all subscriber channels
	b.subMutex.Lock()
	for _, ch := range b.subscribers {
		close(ch)
	}
	b.subscribers = make(map[string]chan []byte)
	b.subMutex.Unlock()

	return nil
}

// IsClosed returns true if the buffer has been closed
func (b *memoryBuffer) IsClosed() bool {
	b.closeMutex.RLock()
	defer b.closeMutex.RUnlock()
	return b.closed
}

// Helper methods

func (b *memoryBuffer) notifySubscribers(data []byte) {
	b.subMutex.RLock()
	defer b.subMutex.RUnlock()

	// Create a copy for each subscriber to prevent data races
	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)

	for _, ch := range b.subscribers {
		select {
		case ch <- dataCopy:
			// Successfully sent
		default:
			// Channel is full, skip this subscriber
		}
	}
}

func (b *memoryBuffer) nextSubscriberID() string {
	b.subCounter++
	return fmt.Sprintf("sub_%d", b.subCounter)
}
