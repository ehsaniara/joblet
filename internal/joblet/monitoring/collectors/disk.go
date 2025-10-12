package collectors

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ehsaniara/joblet/internal/joblet/monitoring/domain"
	"github.com/ehsaniara/joblet/pkg/constants"
	"github.com/ehsaniara/joblet/pkg/logger"
)

// VolumeManager interface for accessing volume information
type VolumeManager interface {
	ListVolumes() []*Volume
	GetVolumeUsage(volumeName string) (used int64, available int64, err error)
}

// Volume represents volume information needed for monitoring
type Volume struct {
	Name      string
	Type      string
	Size      string
	SizeBytes int64
	Path      string
}

// DiskCollector collects disk metrics from /proc/mounts and /proc/diskstats
type DiskCollector struct {
	logger         *logger.Logger
	lastStats      map[string]*diskIOStats
	lastTime       time.Time
	volumeManager  VolumeManager
	volumeBasePath string
}

type diskIOStats struct {
	readsCompleted  uint64
	writesCompleted uint64
	readBytes       uint64
	writeBytes      uint64
	readTime        uint64
	writeTime       uint64
	ioTime          uint64
	weightedIOTime  uint64
}

// NewDiskCollector creates a new disk metrics collector
func NewDiskCollector() *DiskCollector {
	return &DiskCollector{
		logger:    logger.WithField("component", "disk-collector"),
		lastStats: make(map[string]*diskIOStats),
	}
}

// NewDiskCollectorWithVolumeManager creates a new disk metrics collector with volume management
func NewDiskCollectorWithVolumeManager(volumeManager VolumeManager, volumeBasePath string) *DiskCollector {
	return &DiskCollector{
		logger:         logger.WithField("component", "disk-collector"),
		lastStats:      make(map[string]*diskIOStats),
		volumeManager:  volumeManager,
		volumeBasePath: volumeBasePath,
	}
}

// Collect gathers current disk metrics
func (c *DiskCollector) Collect() ([]domain.DiskMetrics, error) {
	// Get mounted filesystems
	mounts, err := c.getMounts()
	if err != nil {
		return nil, fmt.Errorf("failed to get mounts: %w", err)
	}

	// Get disk I/O statistics
	ioStats, err := c.readDiskStats()
	if err != nil {
		c.logger.Warn("failed to read disk stats", "error", err)
		// Continue without I/O stats
		ioStats = make(map[string]*diskIOStats)
	}

	currentTime := time.Now()
	var metrics []domain.DiskMetrics

	for _, mount := range mounts {
		// Get filesystem usage
		var stat syscall.Statfs_t
		if err := syscall.Statfs(mount.mountPoint, &stat); err != nil {
			c.logger.Debug("failed to stat filesystem", "mount", mount.mountPoint, "error", err)
			continue
		}

		totalBytes := stat.Blocks * uint64(stat.Bsize)
		freeBytes := stat.Bavail * uint64(stat.Bsize)
		usedBytes := totalBytes - freeBytes

		usagePercent := 0.0
		if totalBytes > 0 {
			usagePercent = float64(usedBytes) / float64(totalBytes) * 100.0
		}

		diskMetric := domain.DiskMetrics{
			MountPoint:   mount.mountPoint,
			Device:       mount.device,
			FileSystem:   mount.fsType,
			TotalBytes:   totalBytes,
			UsedBytes:    usedBytes,
			FreeBytes:    freeBytes,
			UsagePercent: usagePercent,
			InodesTotal:  stat.Files,
			InodesUsed:   stat.Files - stat.Ffree,
			InodesFree:   stat.Ffree,
		}

		// Add I/O statistics if available
		if ioStat, exists := ioStats[mount.deviceName]; exists {
			if c.lastStats[mount.deviceName] != nil && c.lastTime.Before(currentTime) {
				lastStat := c.lastStats[mount.deviceName]
				timeDelta := currentTime.Sub(c.lastTime).Seconds()

				if timeDelta > 0 {
					diskMetric.ReadIOPS = uint64(float64(ioStat.readsCompleted-lastStat.readsCompleted) / timeDelta)
					diskMetric.WriteIOPS = uint64(float64(ioStat.writesCompleted-lastStat.writesCompleted) / timeDelta)
					diskMetric.ReadThroughput = uint64(float64(ioStat.readBytes-lastStat.readBytes) / timeDelta)
					diskMetric.WriteThroughput = uint64(float64(ioStat.writeBytes-lastStat.writeBytes) / timeDelta)
				}
			}
		}

		metrics = append(metrics, diskMetric)
	}

	// Add joblet volumes if volume manager is available
	if c.volumeManager != nil {
		volumeMetrics, err := c.collectVolumeMetrics()
		if err != nil {
			c.logger.Warn("failed to collect volume metrics", "error", err)
		} else {
			metrics = append(metrics, volumeMetrics...)
		}
	}

	// Store current stats for next calculation
	c.lastStats = ioStats
	c.lastTime = currentTime

	return metrics, nil
}

type mountInfo struct {
	device     string
	mountPoint string
	fsType     string
	deviceName string // Short device name for matching with diskstats
}

// getMounts reads mounted filesystems from /proc/mounts
func (c *DiskCollector) getMounts() ([]mountInfo, error) {
	file, err := os.Open("/proc/mounts")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var mounts []mountInfo
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			continue
		}

		device := fields[0]
		mountPoint := fields[1]
		fsType := fields[2]

		// Skip virtual filesystems
		if c.isVirtualFS(fsType) {
			continue
		}

		// Skip non-device mounts
		if !strings.HasPrefix(device, "/dev/") {
			continue
		}

		// Extract device name for diskstats matching
		deviceName := strings.TrimPrefix(device, "/dev/")

		mounts = append(mounts, mountInfo{
			device:     device,
			mountPoint: mountPoint,
			fsType:     fsType,
			deviceName: deviceName,
		})
	}

	return mounts, scanner.Err()
}

// isVirtualFS checks if the filesystem type is virtual
func (c *DiskCollector) isVirtualFS(fsType string) bool {
	virtualFS := map[string]bool{
		"proc":       true,
		"sysfs":      true,
		"devtmpfs":   true,
		"tmpfs":      true,
		"devpts":     true,
		"cgroup":     true,
		"cgroup2":    true,
		"pstore":     true,
		"bpf":        true,
		"debugfs":    true,
		"tracefs":    true,
		"securityfs": true,
		"hugetlbfs":  true,
		"mqueue":     true,
		"fusectl":    true,
	}
	return virtualFS[fsType]
}

// readDiskStats reads disk I/O statistics from /proc/diskstats
func (c *DiskCollector) readDiskStats() (map[string]*diskIOStats, error) {
	file, err := os.Open("/proc/diskstats")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	stats := make(map[string]*diskIOStats)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 14 {
			continue
		}

		deviceName := fields[2]

		// Skip loop devices and other virtual devices
		if strings.HasPrefix(deviceName, "loop") ||
			strings.HasPrefix(deviceName, "ram") ||
			strings.HasPrefix(deviceName, "dm-") {
			continue
		}

		readCompleted, _ := strconv.ParseUint(fields[3], 10, 64)
		writeCompleted, _ := strconv.ParseUint(fields[7], 10, 64)
		readSectors, _ := strconv.ParseUint(fields[5], 10, 64)
		writeSectors, _ := strconv.ParseUint(fields[9], 10, 64)
		readTime, _ := strconv.ParseUint(fields[6], 10, 64)
		writeTime, _ := strconv.ParseUint(fields[10], 10, 64)
		ioTime, _ := strconv.ParseUint(fields[12], 10, 64)
		weightedIOTime, _ := strconv.ParseUint(fields[13], 10, 64)

		// Convert sectors to bytes
		stats[deviceName] = &diskIOStats{
			readsCompleted:  readCompleted,
			writesCompleted: writeCompleted,
			readBytes:       readSectors * constants.DefaultSectorSize,
			writeBytes:      writeSectors * constants.DefaultSectorSize,
			readTime:        readTime,
			writeTime:       writeTime,
			ioTime:          ioTime,
			weightedIOTime:  weightedIOTime,
		}
	}

	return stats, scanner.Err()
}

// collectVolumeMetrics collects metrics for joblet volumes
func (c *DiskCollector) collectVolumeMetrics() ([]domain.DiskMetrics, error) {
	volumes := c.volumeManager.ListVolumes()
	var metrics []domain.DiskMetrics

	for _, volume := range volumes {
		dataDir := filepath.Join(volume.Path, "data")

		// Check if volume directory exists
		if _, err := os.Stat(dataDir); err != nil {
			c.logger.Debug("skipping volume with missing data directory", "volume", volume.Name, "error", err)
			continue
		}

		// Get filesystem stats for the volume
		var stat syscall.Statfs_t
		if err := syscall.Statfs(dataDir, &stat); err != nil {
			c.logger.Debug("failed to stat volume filesystem", "volume", volume.Name, "error", err)
			continue
		}

		totalBytes := stat.Blocks * uint64(stat.Bsize)
		freeBytes := stat.Bavail * uint64(stat.Bsize)
		usedBytes := totalBytes - freeBytes

		usagePercent := 0.0
		if totalBytes > 0 {
			usagePercent = float64(usedBytes) / float64(totalBytes) * 100.0
		}

		// Create volume-specific mount point and device name
		mountPoint := dataDir
		device := volume.Name
		filesystem := "joblet-volume"

		// For memory volumes, mark as tmpfs
		if volume.Type == "memory" {
			filesystem = "tmpfs"
			device = "tmpfs"
		}

		volumeMetric := domain.DiskMetrics{
			MountPoint:   mountPoint,
			Device:       device,
			FileSystem:   filesystem,
			TotalBytes:   totalBytes,
			UsedBytes:    usedBytes,
			FreeBytes:    freeBytes,
			UsagePercent: usagePercent,
			InodesTotal:  stat.Files,
			InodesUsed:   stat.Files - stat.Ffree,
			InodesFree:   stat.Ffree,
		}

		metrics = append(metrics, volumeMetric)
		c.logger.Debug("collected volume metrics", "volume", volume.Name, "type", volume.Type, "usage", usagePercent)
	}

	return metrics, nil
}
