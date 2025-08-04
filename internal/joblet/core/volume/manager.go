package volume

import (
	"encoding/json"
	"fmt"
	"joblet/internal/joblet/domain"
	"joblet/internal/joblet/state"
	"joblet/pkg/logger"
	"joblet/pkg/platform"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// Manager handles volume creation, management, and cleanup
type Manager struct {
	volumeStore state.VolumeStore
	platform    platform.Platform
	logger      *logger.Logger
	basePath    string // Base directory for volume storage (e.g., /opt/joblet/volumes)
}

// NewManager creates a new volume manager instance with the provided dependencies.
// It initializes the manager with a volume store for state management, platform abstraction
// for OS operations, and a base path where volumes will be stored on disk.
func NewManager(volumeStore state.VolumeStore, platform platform.Platform, basePath string) *Manager {
	return &Manager{
		volumeStore: volumeStore,
		platform:    platform,
		logger:      logger.WithField("component", "volume-manager"),
		basePath:    basePath,
	}
}

// CreateVolume creates a new volume with the specified name, size, and type.
// It creates the volume object, sets up the filesystem storage (directory structure,
// metadata file), handles type-specific setup (tmpfs for memory volumes, loop devices
// for filesystem volumes), and stores the volume in the state store. Returns the
// created volume or an error if any step fails.
func (m *Manager) CreateVolume(name, size string, volumeType domain.VolumeType) (*domain.Volume, error) {
	log := m.logger.WithField("volume", name)
	log.Debug("creating new volume", "size", size, "type", string(volumeType))

	// Create the volume domain object
	volume, err := domain.NewVolume(name, size, volumeType)
	if err != nil {
		return nil, fmt.Errorf("failed to create volume object: %w", err)
	}

	// Set the host path where the volume will be stored
	volume.Path = filepath.Join(m.basePath, name)

	// Create the actual volume storage on the filesystem
	if err := m.createVolumeStorage(volume); err != nil {
		return nil, fmt.Errorf("failed to create volume storage: %w", err)
	}

	// Store the volume in the state store
	if err := m.volumeStore.CreateVolume(volume); err != nil {
		// Cleanup the created storage if state store fails
		_ = m.cleanupVolumeStorage(volume)
		return nil, fmt.Errorf("failed to store volume: %w", err)
	}

	log.Debug("volume created successfully", "path", volume.Path, "sizeBytes", volume.SizeBytes)
	return volume, nil
}

// ListVolumes retrieves and returns all volumes currently registered in the system.
// The volumes are fetched from the volume store and include both filesystem and
// memory volumes. This is used by CLI commands and API endpoints to display
// volume information to users.
func (m *Manager) ListVolumes() []*domain.Volume {
	log := m.logger.WithField("operation", "list-volumes")
	volumes := m.volumeStore.ListVolumes()
	log.Debug("listed volumes", "count", len(volumes))
	return volumes
}

// GetVolume retrieves a specific volume by its name from the volume store.
// Returns the volume object and a boolean indicating whether the volume was found.
// This is used to validate volume existence before job execution and for
// volume inspection operations.
func (m *Manager) GetVolume(name string) (*domain.Volume, bool) {
	return m.volumeStore.GetVolume(name)
}

// RemoveVolume removes a volume from the system by name.
// It first removes the volume from the state store (which checks if the volume
// is currently in use by any jobs), then cleans up the filesystem storage
// including unmounting tmpfs/loop devices and removing directories. Returns
// an error if the volume is not found or is currently in use.
func (m *Manager) RemoveVolume(name string) error {
	log := m.logger.WithField("volume", name)
	log.Debug("removing volume")

	// Get volume details before removal
	volume, exists := m.volumeStore.GetVolume(name)
	if !exists {
		return fmt.Errorf("volume %s not found", name)
	}

	// Remove from state store first (this checks if volume is in use)
	if err := m.volumeStore.RemoveVolume(name); err != nil {
		return err
	}

	// Clean up the filesystem storage
	if err := m.cleanupVolumeStorage(volume); err != nil {
		log.Warn("failed to cleanup volume storage", "error", err)
		// Don't return error here - volume is already removed from state
	}

	log.Debug("volume removed successfully")
	return nil
}

// AttachVolumeToJob increments the usage count for volumes that will be used by a job.
// This prevents volumes from being deleted while jobs are using them. If any volume
// fails to attach, it rolls back the job counts for previously attached volumes.
// This is called before job execution to reserve the volumes.
func (m *Manager) AttachVolumeToJob(volumeNames []string) error {
	log := m.logger.WithField("operation", "attach-volumes")
	log.Debug("attaching volumes to job", "volumes", volumeNames)

	// Increment job count for each volume
	for _, volumeName := range volumeNames {
		if err := m.volumeStore.IncrementJobCount(volumeName); err != nil {
			// If any volume fails, try to decrement the ones we already incremented
			for i := 0; i < len(volumeNames) && volumeNames[i] != volumeName; i++ {
				_ = m.volumeStore.DecrementJobCount(volumeNames[i])
			}
			return fmt.Errorf("failed to attach volume %s: %w", volumeName, err)
		}
	}

	log.Debug("volumes attached successfully", "count", len(volumeNames))
	return nil
}

// DetachVolumeFromJob decrements the usage count for volumes when a job completes.
// This allows volumes to be deleted once no jobs are using them. Errors are logged
// but not returned since job completion should not fail due to volume detachment
// issues. This is called after job completion to release volume reservations.
func (m *Manager) DetachVolumeFromJob(volumeNames []string) {
	log := m.logger.WithField("operation", "detach-volumes")
	log.Debug("detaching volumes from job", "volumes", volumeNames)

	for _, volumeName := range volumeNames {
		if err := m.volumeStore.DecrementJobCount(volumeName); err != nil {
			log.Warn("failed to detach volume", "volume", volumeName, "error", err)
		}
	}

	log.Debug("volumes detached")
}

// createVolumeStorage creates the actual filesystem storage structure for a volume.
// It creates the volume directory, data subdirectory, and metadata file containing
// volume information. For filesystem volumes, it attempts to set up size-limited
// storage using loop devices. For memory volumes, it mounts a tmpfs with size limits.
// This is an internal method called during volume creation.
func (m *Manager) createVolumeStorage(volume *domain.Volume) error {
	log := m.logger.WithField("volume", volume.Name)

	// Create volume directory structure
	volumeDir := volume.Path
	dataDir := filepath.Join(volumeDir, "data")
	metaFile := filepath.Join(volumeDir, "volume-info.json")

	// Create directories
	if err := m.platform.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create volume data directory: %w", err)
	}

	// Write volume metadata
	metadata := fmt.Sprintf(`{
  "name": "%s",
  "type": "%s",
  "size": "%s",
  "sizeBytes": %d,
  "createdTime": "%s"
}`, volume.Name, string(volume.Type), volume.Size, volume.SizeBytes, volume.CreatedTime.Format(time.RFC3339))

	if err := m.platform.WriteFile(metaFile, []byte(metadata), 0644); err != nil {
		return fmt.Errorf("failed to write volume metadata: %w", err)
	}

	// For filesystem volumes, set up size limits if supported
	if volume.Type == domain.VolumeTypeFilesystem {
		if err := m.setupFilesystemVolume(volume, dataDir); err != nil {
			log.Warn("failed to setup filesystem volume limits", "error", err)
			// Continue - basic volume creation succeeded
		}
	} else if volume.Type == domain.VolumeTypeMemory {
		if err := m.setupMemoryVolume(volume, dataDir); err != nil {
			return fmt.Errorf("failed to setup memory volume: %w", err)
		}
	}

	log.Debug("volume storage created", "dataDir", dataDir, "metaFile", metaFile)
	return nil
}

// setupFilesystemVolume creates a filesystem volume with enforced size limits.
// It attempts to create a loop-mounted filesystem using a backing file to provide
// hard size limits. If loop device setup fails, it falls back to a regular directory
// without size enforcement. This provides better resource control for filesystem volumes.
func (m *Manager) setupFilesystemVolume(volume *domain.Volume, dataDir string) error {
	log := m.logger.WithField("volume", volume.Name)
	log.Debug("setting up filesystem volume with size limit", "path", dataDir, "sizeLimit", volume.Size)

	// Create a loop-mounted filesystem to enforce size limits
	if err := m.createLoopFilesystem(volume, dataDir); err != nil {
		log.Warn("failed to create loop filesystem, falling back to directory", "error", err)
		// Fallback to simple directory creation (no size enforcement)
		return nil
	}

	log.Debug("filesystem volume created with size enforcement", "path", dataDir, "sizeBytes", volume.SizeBytes)
	return nil
}

// setupMemoryVolume creates a memory-based volume using tmpfs with size limits.
// It mounts a tmpfs filesystem at the volume's data directory with the specified
// size limit. Memory volumes provide fast I/O but are cleared when unmounted.
// Returns an error if the tmpfs mount fails.
func (m *Manager) setupMemoryVolume(volume *domain.Volume, dataDir string) error {
	// Mount tmpfs with size limit
	sizeOpt := fmt.Sprintf("size=%d", volume.SizeBytes)
	flags := uintptr(0)

	if err := m.platform.Mount("tmpfs", dataDir, "tmpfs", flags, sizeOpt); err != nil {
		return fmt.Errorf("failed to mount tmpfs: %w", err)
	}

	log := m.logger.WithField("volume", volume.Name)
	log.Debug("memory volume mounted", "path", dataDir, "size", volume.Size)
	return nil
}

// cleanupVolumeStorage removes all filesystem storage associated with a volume.
// For memory volumes, it unmounts the tmpfs. For filesystem volumes with loop
// devices, it unmounts the filesystem, detaches the loop device, and removes
// the backing file. Finally, it removes the entire volume directory structure.
// This is called during volume removal to clean up all storage resources.
func (m *Manager) cleanupVolumeStorage(volume *domain.Volume) error {
	log := m.logger.WithField("volume", volume.Name)
	dataDir := filepath.Join(volume.Path, "data")

	if volume.Type == domain.VolumeTypeMemory {
		// For memory volumes, unmount the tmpfs first
		if err := m.platform.Unmount(dataDir, 0x1); err != nil { // 0x1 = MNT_FORCE
			log.Warn("failed to unmount tmpfs", "error", err)
		}
	} else if volume.Type == domain.VolumeTypeFilesystem {
		// For filesystem volumes, handle loop device cleanup
		m.cleanupLoopFilesystem(volume, dataDir)
	}

	// Remove the entire volume directory
	if err := m.platform.RemoveAll(volume.Path); err != nil {
		return fmt.Errorf("failed to remove volume directory: %w", err)
	}

	log.Debug("volume storage cleaned up", "path", volume.Path)
	return nil
}

// cleanupLoopFilesystem handles cleanup of filesystem volumes that use loop devices.
// It reads the loop device information from the volume metadata, unmounts the
// filesystem, detaches the loop device, and removes the backing file. Errors
// are logged but don't prevent cleanup from continuing. This ensures proper
// cleanup of loop device resources to prevent system resource leaks.
func (m *Manager) cleanupLoopFilesystem(volume *domain.Volume, dataDir string) {
	log := m.logger.WithField("volume", volume.Name)

	// Try to get loop device info
	loopDevice, backingFile, err := m.getLoopDeviceFromInfo(volume.Path)
	if err != nil {
		log.Debug("no loop device info found, assuming regular directory", "error", err)
		return
	}

	// Unmount the filesystem
	if err := m.platform.Unmount(dataDir, 0x1); err != nil { // 0x1 = MNT_FORCE
		log.Warn("failed to unmount loop filesystem", "error", err)
	}

	// Detach loop device
	if err := m.detachLoopDevice(loopDevice); err != nil {
		log.Warn("failed to detach loop device", "device", loopDevice, "error", err)
	}

	// Remove backing file
	if err := m.platform.Remove(backingFile); err != nil {
		log.Warn("failed to remove backing file", "file", backingFile, "error", err)
	}

	log.Debug("loop filesystem cleaned up", "loopDevice", loopDevice, "backingFile", backingFile)
}

// ValidateVolumes verifies that all requested volume names are valid and that
// the volumes exist in the system with accessible storage. It checks volume name
// format, existence in the state store, and physical presence of the data directory.
// This is called before job execution to ensure all required volumes are available
// and prevents job failures due to missing volumes.
func (m *Manager) ValidateVolumes(volumeNames []string) error {
	log := m.logger.WithField("operation", "validate-volumes")

	for _, volumeName := range volumeNames {
		if !domain.IsValidVolumeName(volumeName) {
			return fmt.Errorf("invalid volume name: %s", volumeName)
		}

		volume, exists := m.volumeStore.GetVolume(volumeName)
		if !exists {
			return fmt.Errorf("volume %s not found", volumeName)
		}

		// Check if volume storage actually exists
		dataDir := filepath.Join(volume.Path, "data")
		if _, err := m.platform.Stat(dataDir); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("volume %s storage missing at %s", volumeName, dataDir)
			}
			return fmt.Errorf("failed to access volume %s: %w", volumeName, err)
		}
	}

	log.Debug("volumes validated", "volumes", volumeNames)
	return nil
}

// GetVolumeUsage retrieves disk space usage statistics for a specific volume.
// It uses filesystem stats to determine the used and available space in bytes.
// This information is used for monitoring, capacity planning, and displaying
// volume usage to users. Returns used bytes, available bytes, and any error
// encountered while reading filesystem statistics.
func (m *Manager) GetVolumeUsage(volumeName string) (used int64, available int64, err error) {
	volume, exists := m.volumeStore.GetVolume(volumeName)
	if !exists {
		return 0, 0, fmt.Errorf("volume %s not found", volumeName)
	}

	dataDir := filepath.Join(volume.Path, "data")

	// Get filesystem stats
	var stat syscall.Statfs_t
	if err := syscall.Statfs(dataDir, &stat); err != nil {
		return 0, 0, fmt.Errorf("failed to get filesystem stats: %w", err)
	}

	blockSize := int64(stat.Bsize)
	totalBlocks := int64(stat.Blocks)
	freeBlocks := int64(stat.Bavail)

	totalBytes := totalBlocks * blockSize
	availableBytes := freeBlocks * blockSize
	usedBytes := totalBytes - availableBytes

	return usedBytes, availableBytes, nil
}

// createLoopFilesystem creates a size-limited filesystem using a loop device.
// It creates a sparse backing file of the specified size, sets up a loop device
// pointing to the file, creates an ext4 filesystem on the loop device, and mounts
// it at the volume's data directory. This provides hard size enforcement for
// filesystem volumes. The loop device information is stored for later cleanup.
func (m *Manager) createLoopFilesystem(volume *domain.Volume, dataDir string) error {
	log := m.logger.WithField("volume", volume.Name)

	// Create backing file for the loop device
	backingFile := filepath.Join(volume.Path, "volume.img")
	loopInfoFile := filepath.Join(volume.Path, "loop-info.txt")

	// Create sparse file with the specified size
	log.Debug("creating backing file", "path", backingFile, "size", volume.SizeBytes)
	if err := m.createSparseFile(backingFile, volume.SizeBytes); err != nil {
		return fmt.Errorf("failed to create backing file: %w", err)
	}

	// Set up loop device
	loopDevice, err := m.setupLoopDevice(backingFile)
	if err != nil {
		_ = m.platform.Remove(backingFile)
		return fmt.Errorf("failed to setup loop device: %w", err)
	}

	// Store loop device info for cleanup
	loopInfo := fmt.Sprintf("loop_device=%s\nbacking_file=%s\n", loopDevice, backingFile)
	if err := m.platform.WriteFile(loopInfoFile, []byte(loopInfo), 0644); err != nil {
		_ = m.detachLoopDevice(loopDevice)
		_ = m.platform.Remove(backingFile)
		return fmt.Errorf("failed to write loop info: %w", err)
	}

	// Create filesystem on loop device
	if err := m.createFilesystem(loopDevice); err != nil {
		_ = m.detachLoopDevice(loopDevice)
		_ = m.platform.Remove(backingFile)
		_ = m.platform.Remove(loopInfoFile)
		return fmt.Errorf("failed to create filesystem: %w", err)
	}

	// Mount the filesystem
	if err := m.platform.Mount(loopDevice, dataDir, "ext4", 0, ""); err != nil {
		_ = m.detachLoopDevice(loopDevice)
		_ = m.platform.Remove(backingFile)
		_ = m.platform.Remove(loopInfoFile)
		return fmt.Errorf("failed to mount loop filesystem: %w", err)
	}

	// Set proper permissions on mounted directory
	if err := syscall.Chmod(dataDir, 0755); err != nil {
		log.Warn("failed to set permissions on volume", "error", err)
	}

	log.Debug("loop filesystem created and mounted", "loopDevice", loopDevice, "mountPoint", dataDir)
	return nil
}

// createSparseFile creates a sparse file that appears to be the specified size
// but only allocates disk space as data is written. This is used as the backing
// file for loop devices in filesystem volumes. Uses the 'dd' command with seek
// to create the sparse file efficiently without allocating the full size upfront.
func (m *Manager) createSparseFile(path string, sizeBytes int64) error {
	// Use dd to create sparse file
	cmd := exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s", path), "bs=1", "count=0", fmt.Sprintf("seek=%d", sizeBytes))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("dd command failed: %w", err)
	}
	return nil
}

// setupLoopDevice finds an available loop device and attaches it to the specified
// backing file. It uses 'losetup' to find a free loop device and then attaches
// the backing file to it. Returns the loop device path (e.g., /dev/loop0) that
// can be used for mounting. This is part of the filesystem volume creation process.
func (m *Manager) setupLoopDevice(backingFile string) (string, error) {
	// Find next available loop device
	cmd := exec.Command("losetup", "-f")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to find free loop device: %w", err)
	}

	loopDevice := strings.TrimSpace(string(output))

	// Attach backing file to loop device
	cmd = exec.Command("losetup", loopDevice, backingFile)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to attach loop device: %w", err)
	}

	return loopDevice, nil
}

// createFilesystem formats the loop device with an ext4 filesystem.
// Uses 'mkfs.ext4' with the force flag to create the filesystem without
// interactive confirmation. This prepares the loop device for mounting and
// data storage. Ext4 is chosen for its reliability and feature set.
func (m *Manager) createFilesystem(device string) error {
	// Create ext4 filesystem
	cmd := exec.Command("mkfs.ext4", "-F", device)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mkfs.ext4 failed: %w", err)
	}
	return nil
}

// detachLoopDevice detaches the specified loop device, freeing it for reuse.
// Uses 'losetup -d' to detach the device. This is called during volume cleanup
// to ensure loop devices don't remain attached after volume removal, preventing
// resource leaks and allowing the loop device to be reused by other volumes.
func (m *Manager) detachLoopDevice(device string) error {
	cmd := exec.Command("losetup", "-d", device)
	return cmd.Run()
}

// getLoopDeviceFromInfo reads the stored loop device information from a volume's
// metadata file. This information includes the loop device path and backing file
// path, which are needed for proper cleanup when removing filesystem volumes.
// Returns the loop device path, backing file path, or an error if the information
// cannot be read or parsed.
func (m *Manager) getLoopDeviceFromInfo(volumePath string) (string, string, error) {
	loopInfoFile := filepath.Join(volumePath, "loop-info.txt")

	content, err := m.platform.ReadFile(loopInfoFile)
	if err != nil {
		return "", "", err
	}

	lines := strings.Split(string(content), "\n")
	var loopDevice, backingFile string

	for _, line := range lines {
		if strings.HasPrefix(line, "loop_device=") {
			loopDevice = strings.TrimPrefix(line, "loop_device=")
		} else if strings.HasPrefix(line, "backing_file=") {
			backingFile = strings.TrimPrefix(line, "backing_file=")
		}
	}

	if loopDevice == "" || backingFile == "" {
		return "", "", fmt.Errorf("invalid loop info file")
	}

	return loopDevice, backingFile, nil
}

// ScanVolumes discovers existing volumes on disk and loads them into the state store.
// This is called during server startup to restore volume state from persistent storage.
// It scans the volume base directory, reads volume metadata files, recreates volume
// objects, remounts memory volumes if needed, and adds them to the state store.
// This ensures volumes persist across server restarts.
func (m *Manager) ScanVolumes() error {
	log := m.logger.WithField("operation", "scan-volumes")
	log.Debug("scanning for existing volumes", "basePath", m.basePath)

	// Create base path if it doesn't exist
	if err := m.platform.MkdirAll(m.basePath, 0755); err != nil {
		return fmt.Errorf("failed to create volume base directory: %w", err)
	}

	// Read directory entries
	entries, err := os.ReadDir(m.basePath)
	if err != nil {
		return fmt.Errorf("failed to read volume directory: %w", err)
	}

	loadedCount := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		volumeName := entry.Name()
		volumePath := filepath.Join(m.basePath, volumeName)
		metaFile := filepath.Join(volumePath, "volume-info.json")

		// Check if volume metadata exists
		metaData, err := m.platform.ReadFile(metaFile)
		if err != nil {
			log.Warn("skipping directory without volume metadata", "name", volumeName, "error", err)
			continue
		}

		// Parse volume metadata
		var volumeInfo struct {
			Name        string `json:"name"`
			Type        string `json:"type"`
			Size        string `json:"size"`
			SizeBytes   int64  `json:"sizeBytes"`
			CreatedTime string `json:"createdTime"`
		}

		if err := json.Unmarshal(metaData, &volumeInfo); err != nil {
			log.Warn("failed to parse volume metadata", "volume", volumeName, "error", err)
			continue
		}

		// Recreate volume object
		volumeType := domain.VolumeType(volumeInfo.Type)
		if volumeType != domain.VolumeTypeFilesystem && volumeType != domain.VolumeTypeMemory {
			log.Warn("invalid volume type", "volume", volumeName, "type", volumeInfo.Type)
			continue
		}

		// Parse creation time
		createdTime, err := time.Parse(time.RFC3339, volumeInfo.CreatedTime)
		if err != nil {
			log.Warn("failed to parse volume creation time, using current time", "volume", volumeName, "error", err)
			createdTime = time.Now()
		}

		volume := &domain.Volume{
			Name:        volumeInfo.Name,
			Type:        volumeType,
			Size:        volumeInfo.Size,
			SizeBytes:   volumeInfo.SizeBytes,
			Path:        volumePath,
			CreatedTime: createdTime,
			JobCount:    0,
		}

		// For memory volumes, remount them
		if volume.Type == domain.VolumeTypeMemory {
			dataDir := filepath.Join(volumePath, "data")
			// Check if already mounted
			if !m.isMounted(dataDir) {
				if err := m.setupMemoryVolume(volume, dataDir); err != nil {
					log.Warn("failed to remount memory volume", "volume", volumeName, "error", err)
					continue
				}
			}
		}

		// Add to state store
		if err := m.volumeStore.CreateVolume(volume); err != nil {
			log.Warn("failed to load volume into state store", "volume", volumeName, "error", err)
			continue
		}

		loadedCount++
		log.Debug("loaded existing volume", "name", volumeName, "type", string(volumeType), "size", volumeInfo.Size)
	}

	log.Info("volume scan completed", "scanned", len(entries), "loaded", loadedCount)
	return nil
}

// isMounted checks whether a specific path is currently mounted by reading /proc/mounts.
// This is used to determine if memory volumes need to be remounted during volume
// scanning or if filesystem cleanup needs to unmount before removing directories.
// Returns true if the path is found in the mount table, false otherwise.
func (m *Manager) isMounted(path string) bool {
	// Read /proc/mounts to check if path is mounted
	content, err := m.platform.ReadFile("/proc/mounts")
	if err != nil {
		return false
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == path {
			return true
		}
	}

	return false
}
