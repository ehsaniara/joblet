package state

import (
	"fmt"
	"joblet/internal/joblet/domain"
	"joblet/pkg/logger"
	"sync"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

//counterfeiter:generate . VolumeStore
type VolumeStore interface {
	CreateVolume(volume *domain.Volume) error
	GetVolume(name string) (*domain.Volume, bool)
	ListVolumes() []*domain.Volume
	RemoveVolume(name string) error
	IncrementJobCount(name string) error
	DecrementJobCount(name string) error
}

type volumeStore struct {
	volumes map[string]*domain.Volume
	mutex   sync.RWMutex
	logger  *logger.Logger
}

func NewVolumeStore() VolumeStore {
	vs := &volumeStore{
		volumes: make(map[string]*domain.Volume),
		logger:  logger.WithField("component", "volume-store"),
	}

	vs.logger.Debug("volume store initialized")
	return vs
}

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
