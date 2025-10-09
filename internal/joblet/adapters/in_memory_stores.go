package adapters

import (
	"context"
	"sync"

	"joblet/internal/joblet/domain"
	"joblet/internal/joblet/pubsub"
	"joblet/pkg/config"
	"joblet/pkg/logger"
)

// Direct constructors to replace the over-engineered factory pattern

// NewJobStore creates a job store with buffer configuration and log persistence
func NewJobStore(cfg *config.BuffersConfig, logger *logger.Logger) JobStorer {
	store := &SimpleJobStore{
		jobs:   make(map[string]*domain.Job),
		logger: logger.WithField("component", "job-store"),
	}

	// Use configured pubsub buffer size
	bufferSize := 10000 // Default
	if cfg != nil && cfg.PubsubBufferSize > 0 {
		bufferSize = cfg.PubsubBufferSize
	}

	pubsubSystem := pubsub.NewPubSub[JobEvent](
		pubsub.WithBufferSize[JobEvent](bufferSize),
	)

	logMgr := NewSimpleLogManager()

	// Create with log persistence if configured
	if cfg != nil && cfg.LogPersistence.Directory != "" {
		return NewJobStorerWithLogPersistence(store, logMgr, pubsubSystem, &cfg.LogPersistence, logger)
	}

	return NewJobStorer(store, logMgr, pubsubSystem, logger)
}

// NewVolumeStore creates a volume store directly
func NewVolumeStore(logger *logger.Logger) VolumeStorer {
	store := &SimpleVolumeStore{
		volumes: make(map[string]*domain.Volume),
		logger:  logger.WithField("component", "volume-store"),
	}
	return NewVolumeStoreAdapter(store, logger)
}

// NewNetworkStore creates a network store directly
func NewNetworkStore(logger *logger.Logger) NetworkStorer {
	networkStore := &SimpleNetworkStore{
		networks: make(map[string]*NetworkConfig),
		logger:   logger.WithField("component", "network-store"),
	}
	allocationStore := &SimpleAllocationStore{
		allocations: make(map[string]*JobNetworkAllocation),
		logger:      logger.WithField("component", "allocation-store"),
	}
	return NewNetworkStoreAdapter(networkStore, allocationStore, logger)
}

// Simple implementations without generic complexity

type SimpleJobStore struct {
	jobs   map[string]*domain.Job
	mutex  sync.RWMutex
	logger *logger.Logger
}

func (s *SimpleJobStore) Create(ctx context.Context, key string, value *domain.Job) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.jobs[key] = value
	return nil
}

func (s *SimpleJobStore) Get(ctx context.Context, key string) (*domain.Job, bool, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	value, exists := s.jobs[key]
	return value, exists, nil
}

func (s *SimpleJobStore) Update(ctx context.Context, key string, value *domain.Job) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.jobs[key] = value
	return nil
}

func (s *SimpleJobStore) List(ctx context.Context) ([]*domain.Job, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	result := make([]*domain.Job, 0, len(s.jobs))
	for _, job := range s.jobs {
		result = append(result, job)
	}
	return result, nil
}

func (s *SimpleJobStore) Delete(ctx context.Context, key string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	delete(s.jobs, key)
	return nil
}

func (s *SimpleJobStore) Close() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.jobs = make(map[string]*domain.Job)
	return nil
}

type SimpleVolumeStore struct {
	volumes map[string]*domain.Volume
	mutex   sync.RWMutex
	logger  *logger.Logger
}

func (s *SimpleVolumeStore) Create(ctx context.Context, key string, value *domain.Volume) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.volumes[key] = value
	return nil
}

func (s *SimpleVolumeStore) Get(ctx context.Context, key string) (*domain.Volume, bool, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	value, exists := s.volumes[key]
	return value, exists, nil
}

func (s *SimpleVolumeStore) Update(ctx context.Context, key string, value *domain.Volume) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.volumes[key] = value
	return nil
}

func (s *SimpleVolumeStore) List(ctx context.Context) ([]*domain.Volume, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	result := make([]*domain.Volume, 0, len(s.volumes))
	for _, volume := range s.volumes {
		result = append(result, volume)
	}
	return result, nil
}

func (s *SimpleVolumeStore) Delete(ctx context.Context, key string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	delete(s.volumes, key)
	return nil
}

func (s *SimpleVolumeStore) Close() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.volumes = make(map[string]*domain.Volume)
	return nil
}

// VolumeStore interface methods

func (s *SimpleVolumeStore) CreateVolume(volume *domain.Volume) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.volumes[volume.Name] = volume
	return nil
}

func (s *SimpleVolumeStore) GetVolume(name string) (*domain.Volume, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	volume, exists := s.volumes[name]
	return volume, exists
}

func (s *SimpleVolumeStore) ListVolumes() []*domain.Volume {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	result := make([]*domain.Volume, 0, len(s.volumes))
	for _, volume := range s.volumes {
		result = append(result, volume)
	}
	return result
}

func (s *SimpleVolumeStore) RemoveVolume(name string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	delete(s.volumes, name)
	return nil
}

func (s *SimpleVolumeStore) IncrementJobCount(name string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if volume, exists := s.volumes[name]; exists {
		volume.JobCount++
	}
	return nil
}

func (s *SimpleVolumeStore) DecrementJobCount(name string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if volume, exists := s.volumes[name]; exists && volume.JobCount > 0 {
		volume.JobCount--
	}
	return nil
}

type SimpleNetworkStore struct {
	networks map[string]*NetworkConfig
	mutex    sync.RWMutex
	logger   *logger.Logger
}

func (s *SimpleNetworkStore) Create(ctx context.Context, key string, value *NetworkConfig) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.networks[key] = value
	return nil
}

func (s *SimpleNetworkStore) Get(ctx context.Context, key string) (*NetworkConfig, bool, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	value, exists := s.networks[key]
	return value, exists, nil
}

func (s *SimpleNetworkStore) Update(ctx context.Context, key string, value *NetworkConfig) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.networks[key] = value
	return nil
}

func (s *SimpleNetworkStore) List(ctx context.Context) ([]*NetworkConfig, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	result := make([]*NetworkConfig, 0, len(s.networks))
	for _, network := range s.networks {
		result = append(result, network)
	}
	return result, nil
}

func (s *SimpleNetworkStore) Delete(ctx context.Context, key string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	delete(s.networks, key)
	return nil
}

func (s *SimpleNetworkStore) Close() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.networks = make(map[string]*NetworkConfig)
	return nil
}

type SimpleAllocationStore struct {
	allocations map[string]*JobNetworkAllocation
	mutex       sync.RWMutex
	logger      *logger.Logger
}

func (s *SimpleAllocationStore) Create(ctx context.Context, key string, value *JobNetworkAllocation) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.allocations[key] = value
	return nil
}

func (s *SimpleAllocationStore) Get(ctx context.Context, key string) (*JobNetworkAllocation, bool, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	value, exists := s.allocations[key]
	return value, exists, nil
}

func (s *SimpleAllocationStore) Update(ctx context.Context, key string, value *JobNetworkAllocation) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.allocations[key] = value
	return nil
}

func (s *SimpleAllocationStore) List(ctx context.Context) ([]*JobNetworkAllocation, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	result := make([]*JobNetworkAllocation, 0, len(s.allocations))
	for _, allocation := range s.allocations {
		result = append(result, allocation)
	}
	return result, nil
}

func (s *SimpleAllocationStore) Delete(ctx context.Context, key string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	delete(s.allocations, key)
	return nil
}

func (s *SimpleAllocationStore) Close() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.allocations = make(map[string]*JobNetworkAllocation)
	return nil
}
