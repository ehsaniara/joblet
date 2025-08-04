package adapters

import (
	"context"
	"fmt"
	"sync"

	"joblet/internal/joblet/domain"
	"joblet/internal/joblet/pubsub"
	"joblet/pkg/buffer"
	"joblet/pkg/logger"
)

// AdapterFactory creates store adapters using direct implementations.
// This factory enables easy configuration-driven creation of adapters
// with different storage backends (memory, RocksDB, Redis, etc.).
type AdapterFactory struct {
	logger *logger.Logger
}

// NewAdapterFactory creates a new adapter factory.
func NewAdapterFactory(logger *logger.Logger) *AdapterFactory {
	if logger == nil {
		logger = logger.WithField("component", "adapter-factory")
	}

	return &AdapterFactory{
		logger: logger,
	}
}

// CreateJobStoreAdapter creates a job store adapter with the specified configuration.
func (f *AdapterFactory) CreateJobStoreAdapter(config *JobStoreConfig) (JobStoreAdapter, error) {
	if config == nil {
		return nil, fmt.Errorf("job store config is required")
	}

	// Validate configuration
	if err := f.validateJobStoreConfig(config); err != nil {
		return nil, fmt.Errorf("invalid job store config: %w", err)
	}

	// For now, only support memory backend to avoid import cycles
	if config.Store.Backend != "memory" {
		return nil, fmt.Errorf("only memory backend is supported currently")
	}
	// PubSub is always memory for single-machine operation

	// Create stores directly to avoid import cycles
	// Use simple in-memory implementation
	jobStore := &simpleMemoryStore[string, *domain.Job]{
		data: make(map[string]*domain.Job),
	}
	// Create memory pub-sub directly with simplified config
	pubsubConfig := &pubsub.PubSubConfig{
		BufferSize: config.PubSub.BufferSize,
	}
	pubsubSystem := pubsub.NewPubSub[JobEvent](pubsubConfig)

	// Create a simple buffer manager with proper tracking
	bufferMgr := NewSimpleBufferManager()

	// Create buffer configuration from config
	bufferConfig := &buffer.BufferConfig{
		Type:                 config.BufferManager.DefaultBufferConfig.Type,
		MaxCapacity:          config.BufferManager.DefaultBufferConfig.MaxCapacity,
		MaxSubscribers:       config.BufferManager.DefaultBufferConfig.MaxSubscribers,
		SubscriberBufferSize: config.BufferManager.DefaultBufferConfig.SubscriberBufferSize,
		EnableMetrics:        config.BufferManager.DefaultBufferConfig.EnableMetrics,
	}

	// Create adapter
	adapter := NewJobStoreAdapter(jobStore, bufferMgr, pubsubSystem, bufferConfig, f.logger)

	f.logger.Info("job store adapter created successfully",
		"storeBackend", config.Store.Backend,
		"pubsubBackend", "memory")

	return adapter, nil
}

// CreateVolumeStoreAdapter creates a volume store adapter with the specified configuration.
func (f *AdapterFactory) CreateVolumeStoreAdapter(config *VolumeStoreConfig) (VolumeStoreAdapter, error) {
	if config == nil {
		return nil, fmt.Errorf("volume store config is required")
	}

	// Validate configuration
	if err := f.validateVolumeStoreConfig(config); err != nil {
		return nil, fmt.Errorf("invalid volume store config: %w", err)
	}

	// For now, only support memory backend to avoid import cycles
	if config.Store.Backend != "memory" {
		return nil, fmt.Errorf("only memory backend is supported currently")
	}

	// Create volume store directly to avoid import cycles
	// Use simple in-memory implementation
	volumeStore := &simpleMemoryStore[string, *domain.Volume]{
		data: make(map[string]*domain.Volume),
	}

	// Create adapter
	adapter := NewVolumeStoreAdapter(volumeStore, f.logger)

	f.logger.Info("volume store adapter created successfully", "storeBackend", config.Store.Backend)

	return adapter, nil
}

// CreateNetworkStoreAdapter creates a network store adapter with the specified configuration.
func (f *AdapterFactory) CreateNetworkStoreAdapter(config *NetworkStoreConfig) (NetworkStoreAdapter, error) {
	if config == nil {
		return nil, fmt.Errorf("network store config is required")
	}

	// Validate configuration
	if err := f.validateNetworkStoreConfig(config); err != nil {
		return nil, fmt.Errorf("invalid network store config: %w", err)
	}

	// Support both memory and file backends
	if config.NetworkStore.Backend != "memory" && config.NetworkStore.Backend != "file" {
		return nil, fmt.Errorf("unsupported network store backend: %s (supported: memory, file)", config.NetworkStore.Backend)
	}
	if config.AllocationStore.Backend != "memory" && config.AllocationStore.Backend != "file" {
		return nil, fmt.Errorf("unsupported allocation store backend: %s (supported: memory, file)", config.AllocationStore.Backend)
	}

	// Create stores directly to avoid import cycles
	// Use simple in-memory implementation
	networkStore := &simpleMemoryStore[string, *NetworkConfig]{
		data: make(map[string]*NetworkConfig),
	}
	allocationStore := &simpleMemoryStore[string, *JobNetworkAllocation]{
		data: make(map[string]*JobNetworkAllocation),
	}

	// Create adapter
	adapter := NewNetworkStoreAdapter(networkStore, allocationStore, f.logger)

	f.logger.Info("network store adapter created successfully",
		"networkStoreBackend", config.NetworkStore.Backend,
		"allocationStoreBackend", config.AllocationStore.Backend)

	return adapter, nil
}

// Helper methods

func (f *AdapterFactory) validateJobStoreConfig(config *JobStoreConfig) error {
	if config.Store == nil {
		return fmt.Errorf("store config is required")
	}

	// Simple validation for memory backend
	if config.Store.Backend != "memory" {
		return fmt.Errorf("only memory backend is supported currently")
	}

	if config.PubSub == nil {
		return fmt.Errorf("pub-sub config is required")
	}

	// PubSub is always memory for single-machine operation - validate basic fields
	if config.PubSub.BufferSize < 0 {
		return fmt.Errorf("pubsub buffer_size cannot be negative")
	}

	return nil
}

func (f *AdapterFactory) validateVolumeStoreConfig(config *VolumeStoreConfig) error {
	if config.Store == nil {
		return fmt.Errorf("store config is required")
	}

	// Simple validation for memory backend
	if config.Store.Backend != "memory" {
		return fmt.Errorf("only memory backend is supported currently")
	}
	return nil
}

func (f *AdapterFactory) validateNetworkStoreConfig(config *NetworkStoreConfig) error {
	if config.NetworkStore == nil {
		return fmt.Errorf("network store config is required")
	}

	// Support both memory and file backends
	if config.NetworkStore.Backend != "memory" && config.NetworkStore.Backend != "file" {
		return fmt.Errorf("unsupported network store backend: %s (supported: memory, file)", config.NetworkStore.Backend)
	}

	if config.AllocationStore == nil {
		return fmt.Errorf("allocation store config is required")
	}

	// Support both memory and file backends
	if config.AllocationStore.Backend != "memory" && config.AllocationStore.Backend != "file" {
		return fmt.Errorf("unsupported allocation store backend: %s (supported: memory, file)", config.AllocationStore.Backend)
	}

	return nil
}

// Configuration structs

// StoreConfig contains configuration for a store backend.
// This is a temporary duplicate to avoid import cycles - will be cleaned up later.
type StoreConfig struct {
	Backend string        `yaml:"backend" json:"backend"`
	Path    string        `yaml:"path" json:"path"`
	Memory  *MemoryConfig `yaml:"memory,omitempty" json:"memory,omitempty"`
}

// MemoryConfig configures the memory store backend.
// This is a temporary duplicate to avoid import cycles - will be cleaned up later.
type MemoryConfig struct {
	// Currently empty - no configuration needed for unlimited memory stores
}

// JobStoreConfig contains configuration for creating a job store adapter.
type JobStoreConfig struct {
	Store         *StoreConfig         `yaml:"store" json:"store"`
	PubSub        *pubsub.PubSubConfig `yaml:"pubsub" json:"pubsub"`
	BufferManager *BufferManagerConfig `yaml:"buffer_manager,omitempty" json:"buffer_manager,omitempty"`
}

// VolumeStoreConfig contains configuration for creating a volume store adapter.
type VolumeStoreConfig struct {
	Store *StoreConfig `yaml:"store" json:"store"`
}

// NetworkStoreConfig contains configuration for creating a network store adapter.
type NetworkStoreConfig struct {
	NetworkStore    *StoreConfig `yaml:"network_store" json:"network_store"`
	AllocationStore *StoreConfig `yaml:"allocation_store" json:"allocation_store"`
}

// BufferManagerConfig contains configuration for the buffer manager.
type BufferManagerConfig struct {
	DefaultBufferConfig *buffer.BufferConfig `yaml:"default_buffer_config" json:"default_buffer_config"`
	MaxBuffers          int                  `yaml:"max_buffers" json:"max_buffers"`
	CleanupInterval     string               `yaml:"cleanup_interval" json:"cleanup_interval"`
}

// SimpleBufferManager provides a basic buffer manager implementation for development.
type SimpleBufferManager struct {
	buffers map[string]buffer.Buffer
	mutex   sync.RWMutex
}

func NewSimpleBufferManager() *SimpleBufferManager {
	return &SimpleBufferManager{
		buffers: make(map[string]buffer.Buffer),
	}
}

func (s *SimpleBufferManager) CreateBuffer(ctx context.Context, id string, config buffer.BufferConfig) (buffer.Buffer, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Check if buffer already exists
	if _, exists := s.buffers[id]; exists {
		return nil, fmt.Errorf("buffer %s already exists", id)
	}

	// Create a simple memory buffer using factory
	factory := buffer.NewFactory()
	buf, err := factory.NewBuffer(id, &config)
	if err != nil {
		return nil, err
	}
	s.buffers[id] = buf
	return buf, nil
}

func (s *SimpleBufferManager) GetBuffer(id string) (buffer.Buffer, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	buf, exists := s.buffers[id]
	return buf, exists
}

func (s *SimpleBufferManager) RemoveBuffer(id string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if buf, exists := s.buffers[id]; exists {
		if err := buf.Close(); err != nil {
			return fmt.Errorf("failed to close buffer %s: %w", id, err)
		}
		delete(s.buffers, id)
	}
	return nil
}

func (s *SimpleBufferManager) ListBuffers() []string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	ids := make([]string, 0, len(s.buffers))
	for id := range s.buffers {
		ids = append(ids, id)
	}
	return ids
}

func (s *SimpleBufferManager) Close() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Close all buffers
	for id, buf := range s.buffers {
		if e := buf.Close(); e != nil {
			// Log error but continue closing other buffers
			// We don't have a logger here, so we'll ignore the error
			_ = e
		}
		delete(s.buffers, id)
	}
	return nil
}

func (s *SimpleBufferManager) Stats() *buffer.BufferManagerStats {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return &buffer.BufferManagerStats{
		ActiveBuffers:       len(s.buffers),
		TotalBuffersCreated: int64(len(s.buffers)), // Simplified for development
		TotalBytesWritten:   0,                     // Not implemented for simplicity
		TotalBytesRead:      0,                     // Not implemented for simplicity
		MemoryUsage:         0,                     // Not implemented for simplicity
	}
}

func (s *SimpleBufferManager) CreatePersistentBuffer(ctx context.Context, id string, path string, config buffer.BufferConfig) (buffer.PersistentBuffer, error) {
	// For development, persistent buffers aren't supported in simple buffer manager
	return nil, fmt.Errorf("persistent buffers not supported in simple buffer manager")
}

func (s *SimpleBufferManager) CreateRingBuffer(ctx context.Context, id string, size int, config buffer.BufferConfig) (buffer.RingBuffer, error) {
	// For development, ring buffers aren't supported in simple buffer manager
	return nil, fmt.Errorf("ring buffers not supported in simple buffer manager")
}

// simpleMemoryStore provides a basic in-memory store implementation to avoid import cycles.
type simpleMemoryStore[K comparable, V any] struct {
	data   map[K]V
	mutex  sync.RWMutex
	closed bool
}

func (s *simpleMemoryStore[K, V]) Create(ctx context.Context, key K, value V) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.closed {
		return fmt.Errorf("store is closed")
	}

	if _, exists := s.data[key]; exists {
		return fmt.Errorf("key already exists")
	}

	s.data[key] = value
	return nil
}

func (s *simpleMemoryStore[K, V]) Get(ctx context.Context, key K) (V, bool, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var zero V
	if s.closed {
		return zero, false, fmt.Errorf("store is closed")
	}

	value, exists := s.data[key]
	return value, exists, nil
}

func (s *simpleMemoryStore[K, V]) Update(ctx context.Context, key K, value V) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.closed {
		return fmt.Errorf("store is closed")
	}

	// For simplicity, allow updates even if key doesn't exist
	s.data[key] = value
	return nil
}

func (s *simpleMemoryStore[K, V]) List(ctx context.Context) ([]V, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.closed {
		return nil, fmt.Errorf("store is closed")
	}

	result := make([]V, 0, len(s.data))
	for _, value := range s.data {
		result = append(result, value)
	}
	return result, nil
}

func (s *simpleMemoryStore[K, V]) Delete(ctx context.Context, key K) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.closed {
		return fmt.Errorf("store is closed")
	}

	if _, exists := s.data[key]; !exists {
		return fmt.Errorf("key not found")
	}

	delete(s.data, key)
	return nil
}

func (s *simpleMemoryStore[K, V]) Close() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.closed = true
	s.data = make(map[K]V)
	return nil
}
