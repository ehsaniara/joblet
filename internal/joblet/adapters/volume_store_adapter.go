package adapters

import (
	"context"
	"fmt"
	"sync"
	"time"

	"joblet/internal/joblet/domain"
	"joblet/pkg/logger"
)

// Local interface definitions to avoid import cycles with pkg/store
type volumeStore[K comparable, V any] interface {
	Create(ctx context.Context, key K, value V) error
	Get(ctx context.Context, key K) (V, bool, error)
	List(ctx context.Context) ([]V, error)
	Delete(ctx context.Context, key K) error
	Close() error
}

// Note: Error checking helpers are defined in job_store_adapter.go to avoid redeclaration

// volumeStoreAdapter implements VolumeStoreAdapter using the new generic store backend.
// It maintains full compatibility with the existing state.VolumeStore interface.
type volumeStoreAdapter struct {
	// Generic storage backend
	volumeStore volumeStore[string, *domain.Volume]

	// Job usage tracking (volume name -> job count)
	jobCounts   map[string]int
	countsMutex sync.RWMutex

	logger     *logger.Logger
	closed     bool
	closeMutex sync.RWMutex
}

// NewVolumeStoreAdapter creates a new volume store adapter with the specified backend.
func NewVolumeStoreAdapter(
	volumeStore volumeStore[string, *domain.Volume],
	logger *logger.Logger,
) VolumeStoreAdapter {
	if logger == nil {
		logger = logger.WithField("component", "volume-store-adapter")
	}

	return &volumeStoreAdapter{
		volumeStore: volumeStore,
		jobCounts:   make(map[string]int),
		logger:      logger,
	}
}

// CreateVolume adds a new volume to the store.
func (a *volumeStoreAdapter) CreateVolume(volume *domain.Volume) error {

	a.closeMutex.RLock()
	if a.closed {
		a.closeMutex.RUnlock()
		return fmt.Errorf("volume store adapter is closed")
	}
	a.closeMutex.RUnlock()

	if volume == nil {
		return fmt.Errorf("volume cannot be nil")
	}

	if volume.Name == "" {
		return fmt.Errorf("volume name cannot be empty")
	}

	// Validate volume
	if err := a.validateVolume(volume); err != nil {
		return fmt.Errorf("volume validation failed: %w", err)
	}

	// Set creation timestamp - use domain model fields
	now := time.Now()
	volume.CreatedTime = now

	// Store the volume
	ctx := context.Background()
	if err := a.volumeStore.Create(ctx, volume.Name, volume); err != nil {
		if IsConflictError(err) {
			return fmt.Errorf("volume already exists: %s", volume.Name)
		}
		a.logger.Error("failed to create volume in store", "volumeName", volume.Name, "error", err)
		return fmt.Errorf("failed to create volume: %w", err)
	}

	// Initialize job count
	a.countsMutex.Lock()
	a.jobCounts[volume.Name] = 0
	a.countsMutex.Unlock()

	a.logger.Info("volume created successfully",
		"volumeName", volume.Name,
		"type", volume.Type,
		"size", volume.SizeBytes)

	return nil
}

// GetVolume retrieves a volume by name.
func (a *volumeStoreAdapter) GetVolume(name string) (*domain.Volume, bool) {

	a.closeMutex.RLock()
	if a.closed {
		a.closeMutex.RUnlock()
		return nil, false
	}
	a.closeMutex.RUnlock()

	if name == "" {
		a.logger.Debug("empty volume name provided")
		return nil, false
	}

	ctx := context.Background()
	volume, exists, err := a.volumeStore.Get(ctx, name)
	if err != nil {
		a.logger.Error("failed to get volume from store", "volumeName", name, "error", err)
		return nil, false
	}

	if exists {
		a.logger.Debug("volume retrieved successfully",
			"volumeName", name,
			"type", volume.Type)
		return volume.DeepCopy(), true
	}

	a.logger.Debug("volume not found", "volumeName", name)
	return nil, false
}

// ListVolumes returns all volumes currently stored in the system.
func (a *volumeStoreAdapter) ListVolumes() []*domain.Volume {

	a.closeMutex.RLock()
	if a.closed {
		a.closeMutex.RUnlock()
		return []*domain.Volume{}
	}
	a.closeMutex.RUnlock()

	ctx := context.Background()
	volumes, err := a.volumeStore.List(ctx)
	if err != nil {
		a.logger.Error("failed to list volumes from store", "error", err)
		return []*domain.Volume{}
	}

	// Create deep copies
	result := make([]*domain.Volume, len(volumes))
	for i, volume := range volumes {
		result[i] = volume.DeepCopy()
	}

	a.logger.Debug("volumes listed successfully",
		"count", len(result))

	return result
}

// RemoveVolume removes a volume from the store by name.
func (a *volumeStoreAdapter) RemoveVolume(name string) error {

	a.closeMutex.RLock()
	if a.closed {
		a.closeMutex.RUnlock()
		return fmt.Errorf("volume store adapter is closed")
	}
	a.closeMutex.RUnlock()

	if name == "" {
		return fmt.Errorf("volume name cannot be empty")
	}

	// Check if volume is in use
	a.countsMutex.RLock()
	jobCount := a.jobCounts[name]
	a.countsMutex.RUnlock()

	if jobCount > 0 {
		return fmt.Errorf("volume is currently in use by %d job(s)", jobCount)
	}

	// Get volume for statistics before deletion
	ctx := context.Background()
	volume, exists, err := a.volumeStore.Get(ctx, name)
	if err != nil {
		a.logger.Error("failed to get volume for deletion", "volumeName", name, "error", err)
		return fmt.Errorf("failed to check volume: %w", err)
	}

	if !exists {
		return fmt.Errorf("volume not found: %s", name)
	}

	// Remove from store
	if err := a.volumeStore.Delete(ctx, name); err != nil {
		if err.Error() == "key not found" {
			return fmt.Errorf("volume not found: %s", name)
		}
		a.logger.Error("failed to remove volume from store", "volumeName", name, "error", err)
		return fmt.Errorf("failed to remove volume: %w", err)
	}

	// Remove job count tracking
	a.countsMutex.Lock()
	delete(a.jobCounts, name)
	a.countsMutex.Unlock()

	a.logger.Info("volume removed successfully",
		"volumeName", name,
		"type", volume.Type)

	return nil
}

// IncrementJobCount increases the usage count for a volume.
func (a *volumeStoreAdapter) IncrementJobCount(name string) error {
	a.closeMutex.RLock()
	if a.closed {
		a.closeMutex.RUnlock()
		return fmt.Errorf("volume store adapter is closed")
	}
	a.closeMutex.RUnlock()

	if name == "" {
		return fmt.Errorf("volume name cannot be empty")
	}

	// Check if volume exists
	_, exists := a.GetVolume(name)
	if !exists {
		return fmt.Errorf("volume not found: %s", name)
	}

	a.countsMutex.Lock()
	a.jobCounts[name]++
	newCount := a.jobCounts[name]
	a.countsMutex.Unlock()

	a.logger.Debug("incremented job count for volume",
		"volumeName", name,
		"newCount", newCount)

	return nil
}

// DecrementJobCount decreases the usage count for a volume.
func (a *volumeStoreAdapter) DecrementJobCount(name string) error {
	a.closeMutex.RLock()
	if a.closed {
		a.closeMutex.RUnlock()
		return fmt.Errorf("volume store adapter is closed")
	}
	a.closeMutex.RUnlock()

	if name == "" {
		return fmt.Errorf("volume name cannot be empty")
	}

	a.countsMutex.Lock()
	defer a.countsMutex.Unlock()

	count, exists := a.jobCounts[name]
	if !exists {
		return fmt.Errorf("volume not found: %s", name)
	}

	if count <= 0 {
		a.logger.Warn("attempted to decrement job count below zero", "volumeName", name)
		return nil
	}

	a.jobCounts[name]--
	newCount := a.jobCounts[name]

	a.logger.Debug("decremented job count for volume",
		"volumeName", name,
		"newCount", newCount)

	return nil
}

// Close gracefully shuts down the adapter and releases resources.
func (a *volumeStoreAdapter) Close() error {
	a.closeMutex.Lock()
	defer a.closeMutex.Unlock()

	if a.closed {
		return nil
	}
	a.closed = true

	// Clear job counts
	a.countsMutex.Lock()
	a.jobCounts = make(map[string]int)
	a.countsMutex.Unlock()

	// Close backend store
	if err := a.volumeStore.Close(); err != nil {
		a.logger.Error("failed to close volume store", "error", err)
		return fmt.Errorf("failed to close volume store: %w", err)
	}

	a.logger.Debug("volume store adapter closed successfully")
	return nil
}

// Helper methods

func (a *volumeStoreAdapter) validateVolume(volume *domain.Volume) error {
	if volume.Name == "" {
		return fmt.Errorf("volume name is required")
	}

	if volume.Type == "" {
		return fmt.Errorf("volume type is required")
	}

	// Validate volume type
	if volume.Type != domain.VolumeTypeFilesystem && volume.Type != domain.VolumeTypeMemory {
		return fmt.Errorf("invalid volume type: %s", volume.Type)
	}

	if volume.SizeBytes <= 0 {
		return fmt.Errorf("volume size must be positive")
	}

	// Additional validation for filesystem volumes
	if volume.Type == domain.VolumeTypeFilesystem {
		if volume.Path == "" {
			return fmt.Errorf("filesystem volume requires a path")
		}
	}

	return nil
}
