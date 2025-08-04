package state

import (
	"fmt"
	"joblet/internal/joblet/domain"
	"joblet/pkg/logger"
	"sync"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

// VolumeStore provides thread-safe storage and management of volume metadata.
// It maintains an in-memory registry of all volumes in the system and provides
// operations for creating, retrieving, listing, and removing volumes. It also
// tracks volume usage by jobs to prevent deletion of volumes currently in use.
//
//counterfeiter:generate . VolumeStore
type VolumeStore interface {
	// CreateVolume adds a new volume to the store.
	// Returns an error if a volume with the same name already exists.
	CreateVolume(volume *domain.Volume) error

	// GetVolume retrieves a volume by name.
	// Returns the volume and true if found, nil and false otherwise.
	GetVolume(name string) (*domain.Volume, bool)

	// ListVolumes returns all volumes currently stored in the system.
	// Returns a slice of volume copies to prevent external modification.
	ListVolumes() []*domain.Volume

	// RemoveVolume removes a volume from the store by name.
	// Returns an error if the volume doesn't exist or is currently in use.
	RemoveVolume(name string) error

	// IncrementJobCount increases the usage count for a volume.
	// This prevents the volume from being deleted while jobs are using it.
	IncrementJobCount(name string) error

	// DecrementJobCount decreases the usage count for a volume.
	// This allows the volume to be deleted when no jobs are using it.
	DecrementJobCount(name string) error
}

type volumeStore struct {
	volumes map[string]*domain.Volume
	mutex   sync.RWMutex
	logger  *logger.Logger
}

// NewVolumeStore creates a new thread-safe volume store instance.
// The store maintains an in-memory map of volumes and uses a read-write mutex
// for concurrent access. It initializes with an empty volume registry.
func NewVolumeStore() VolumeStore {
	vs := &volumeStore{
		volumes: make(map[string]*domain.Volume),
		logger:  logger.WithField("component", "volume-store"),
	}

	vs.logger.Debug("volume store initialized")
	return vs
}

// CreateVolume adds a new volume to the store with thread-safe access.
// It checks for name conflicts and stores a deep copy of the volume to prevent
// external modifications. Returns an error if a volume with the same name already exists.
func (vs *volumeStore) CreateVolume(volume *domain.Volume) error {
	vs.mutex.Lock()
	defer vs.mutex.Unlock()

	if _, exists := vs.volumes[volume.Name]; exists {
		vs.logger.Warn("attempted to create volume that already exists", "volumeName", volume.Name)
		return fmt.Errorf("volume %s already exists", volume.Name)
	}

	vs.volumes[volume.Name] = volume.DeepCopy()
	vs.logger.Debug("volume created", "volumeName", volume.Name, "type", string(volume.Type), "size", volume.Size)

	return nil
}

// GetVolume retrieves a volume by name with thread-safe read access.
// Returns a deep copy of the volume to prevent external modifications and
// a boolean indicating whether the volume was found. Logs the retrieval
// operation including volume metadata for debugging purposes.
func (vs *volumeStore) GetVolume(name string) (*domain.Volume, bool) {
	vs.mutex.RLock()
	defer vs.mutex.RUnlock()

	volume, exists := vs.volumes[name]
	if !exists {
		vs.logger.Debug("volume not found", "volumeName", name)
		return nil, false
	}

	vs.logger.Debug("volume retrieved", "volumeName", name, "type", string(volume.Type), "jobCount", volume.JobCount)
	return volume.DeepCopy(), true
}

// ListVolumes returns all volumes currently stored in the system.
// Uses read-only access for thread safety and returns deep copies of all
// volumes to prevent external modifications. The returned slice can be
// safely modified by callers without affecting the stored volumes.
func (vs *volumeStore) ListVolumes() []*domain.Volume {
	vs.mutex.RLock()
	defer vs.mutex.RUnlock()

	volumes := make([]*domain.Volume, 0, len(vs.volumes))
	for _, volume := range vs.volumes {
		volumes = append(volumes, volume.DeepCopy())
	}

	vs.logger.Debug("listed volumes", "count", len(volumes))
	return volumes
}

// RemoveVolume removes a volume from the store by name with thread-safe access.
// It validates that the volume exists and is not currently in use by any jobs
// before removing it. Returns an error if the volume is not found or is in use.
// This prevents accidental deletion of volumes that jobs depend on.
func (vs *volumeStore) RemoveVolume(name string) error {
	vs.mutex.Lock()
	defer vs.mutex.Unlock()

	volume, exists := vs.volumes[name]
	if !exists {
		vs.logger.Debug("attempted to remove non-existent volume", "volumeName", name)
		return fmt.Errorf("volume %s not found", name)
	}

	if volume.IsInUse() {
		vs.logger.Warn("attempted to remove volume that is in use", "volumeName", name, "jobCount", volume.JobCount)
		return fmt.Errorf("volume %s is in use by %d job(s)", name, volume.JobCount)
	}

	delete(vs.volumes, name)
	vs.logger.Debug("volume removed", "volumeName", name)

	return nil
}

// IncrementJobCount increases the usage count for a volume with thread-safe access.
// This is called when a job starts using a volume to prevent the volume from
// being deleted while in use. Returns an error if the volume is not found.
// The operation is atomic and logs the new count for debugging purposes.
func (vs *volumeStore) IncrementJobCount(name string) error {
	vs.mutex.Lock()
	defer vs.mutex.Unlock()

	volume, exists := vs.volumes[name]
	if !exists {
		vs.logger.Warn("attempted to increment job count for non-existent volume", "volumeName", name)
		return fmt.Errorf("volume %s not found", name)
	}

	volume.IncrementJobCount()
	vs.logger.Debug("incremented job count for volume", "volumeName", name, "newCount", volume.JobCount)

	return nil
}

// DecrementJobCount decreases the usage count for a volume with thread-safe access.
// This is called when a job finishes using a volume to allow the volume to be
// deleted when no longer in use. Returns an error if the volume is not found.
// The operation is atomic and logs the new count for debugging purposes.
func (vs *volumeStore) DecrementJobCount(name string) error {
	vs.mutex.Lock()
	defer vs.mutex.Unlock()

	volume, exists := vs.volumes[name]
	if !exists {
		vs.logger.Warn("attempted to decrement job count for non-existent volume", "volumeName", name)
		return fmt.Errorf("volume %s not found", name)
	}

	volume.DecrementJobCount()
	vs.logger.Debug("decremented job count for volume", "volumeName", name, "newCount", volume.JobCount)

	return nil
}
