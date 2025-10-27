package adapters

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ehsaniara/joblet/internal/joblet/domain"
	"github.com/ehsaniara/joblet/internal/joblet/pubsub"
	"github.com/ehsaniara/joblet/internal/joblet/state"
	persistpb "github.com/ehsaniara/joblet/internal/proto/gen/persist"
	"github.com/ehsaniara/joblet/pkg/client"
	"github.com/ehsaniara/joblet/pkg/config"
	"github.com/ehsaniara/joblet/pkg/logger"
)

// Direct constructors to replace the over-engineered factory pattern

// WaitForStateService waits for state service to be ready with retries (public for server.go)
// Uses the Ping IPC message for efficient health checking
func WaitForStateService(client state.StateClient, logger *logger.Logger) error {
	return waitForStateService(client, logger)
}

// waitForStateService waits for state service to be ready with retries
// Uses the Ping IPC message for efficient health checking
func waitForStateService(client state.StateClient, logger *logger.Logger) error {
	const (
		maxRetries    = 30 // 30 attempts
		retryDelay    = 1 * time.Second
		healthTimeout = 2 * time.Second
	)

	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Try to connect
		if err := client.Connect(); err != nil {
			logger.Debug("state service connection attempt failed",
				"attempt", attempt, "maxRetries", maxRetries, "error", err)

			if attempt < maxRetries {
				time.Sleep(retryDelay)
				continue
			}
			return fmt.Errorf("failed to connect after %d attempts: %w", maxRetries, err)
		}

		// Connection succeeded, now verify it's healthy with Ping
		ctx, cancel := context.WithTimeout(context.Background(), healthTimeout)
		defer cancel()

		// Use Ping IPC message for efficient health check (no backend query overhead)
		err := client.Ping(ctx)
		if err != nil {
			logger.Debug("state service health check (Ping) failed",
				"attempt", attempt, "error", err)

			// Close the connection and retry
			client.Close()

			if attempt < maxRetries {
				time.Sleep(retryDelay)
				continue
			}
			return fmt.Errorf("state service not healthy after %d attempts: %w", maxRetries, err)
		}

		// Success - connection established and Ping responded
		logger.Info("state service health check passed (Ping)",
			"attempts", attempt, "totalTime", time.Duration(attempt-1)*retryDelay)
		return nil
	}

	return fmt.Errorf("state service did not become ready within %v", time.Duration(maxRetries)*retryDelay)
}

// WaitForPersistService waits for persist service to be ready with retries (public for server.go)
// Uses the Ping RPC for efficient health checking
func WaitForPersistService(socketPath string, logger *logger.Logger) (persistpb.PersistServiceClient, error) {
	if err := waitForPersistService(socketPath, logger); err != nil {
		return nil, err
	}
	// Connect and return the client
	return client.NewPersistClientUnix(socketPath)
}

// waitForPersistService waits for persist service to be ready with retries
// Uses the Ping RPC for efficient health checking
func waitForPersistService(socketPath string, logger *logger.Logger) error {
	const (
		maxRetries    = 30 // 30 attempts
		retryDelay    = 1 * time.Second
		healthTimeout = 2 * time.Second
	)

	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Try to connect
		persistClient, err := client.NewPersistClientUnix(socketPath)
		if err != nil {
			logger.Debug("persist service connection attempt failed",
				"attempt", attempt, "maxRetries", maxRetries, "error", err)

			if attempt < maxRetries {
				time.Sleep(retryDelay)
				continue
			}
			return fmt.Errorf("failed to connect after %d attempts: %w", maxRetries, err)
		}

		// Connection succeeded, now verify it's healthy with Ping RPC
		ctx, cancel := context.WithTimeout(context.Background(), healthTimeout)

		// Use Ping RPC for efficient health check (no business logic overhead)
		_, err = persistClient.Ping(ctx, &persistpb.PingRequest{})
		cancel()

		if err != nil {
			logger.Debug("persist service health check (Ping) failed",
				"attempt", attempt, "error", err)

			if attempt < maxRetries {
				time.Sleep(retryDelay)
				continue
			}
			return fmt.Errorf("persist service not healthy after %d attempts: %w", maxRetries, err)
		}

		// Success - connection established and Ping responded
		logger.Info("persist service health check passed (Ping)",
			"attempts", attempt, "totalTime", time.Duration(attempt-1)*retryDelay)
		return nil
	}

	return fmt.Errorf("persist service did not become ready within %v", time.Duration(maxRetries)*retryDelay)
}

// NewJobStore creates a job store with buffer configuration and log persistence
func NewJobStore(cfg *config.Config, persistEnabled bool, logger *logger.Logger) JobStorer {
	store := &SimpleJobStore{
		jobs:   make(map[string]*domain.Job),
		logger: logger.WithField("component", "job-store"),
	}

	// Use configured pubsub buffer size
	bufferSize := 10000 // Default
	if cfg != nil && cfg.Buffers.PubsubBufferSize > 0 {
		bufferSize = cfg.Buffers.PubsubBufferSize
	}

	pubsubSystem := pubsub.NewPubSub[JobEvent](
		pubsub.WithBufferSize[JobEvent](bufferSize),
	)

	logMgr := NewSimpleLogManager()

	// Create persist client for historical log/metric deletion
	// Health check is deferred - happens after subprocess startup in server.go
	persistSocketPath := "/opt/joblet/run/persist-grpc.sock"
	var persistClient persistpb.PersistServiceClient

	if persistEnabled {
		// Just create the client, no health check yet (will be checked after subprocess starts)
		logger.Info("persist service enabled - will connect after subprocess starts", "socket", persistSocketPath)
		persistClient = nil // Will be connected later via WaitForPersistService
	} else {
		// Persist disabled - don't connect at all
		persistClient = nil
		logger.Info("persist service disabled (ipc.enabled=false) - skipping connection")
	}

	// Create state client for persistent job state across restarts
	// Health check is deferred - happens after subprocess startup in server.go
	stateSocketPath := "/opt/joblet/run/state-ipc.sock"

	// Use pooled client for high-performance concurrent access (1000+ jobs)
	// Pool size defaults to 20 connections, tuned for high concurrency
	poolSize := 20
	if cfg != nil && cfg.State.PoolSize > 0 {
		poolSize = cfg.State.PoolSize
	}

	stateClient := state.NewPooledClient(stateSocketPath, poolSize, logger)
	logger.Info("pooled state client created - will connect after subprocess starts",
		"socket", stateSocketPath, "pool_size", poolSize)

	// Logs are buffered in-memory for real-time streaming and forwarded to persist via IPC
	return NewJobStorer(store, logMgr, pubsubSystem, persistClient, stateClient, persistEnabled, logger)
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
