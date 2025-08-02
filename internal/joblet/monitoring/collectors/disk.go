package collectors

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"joblet/internal/joblet/monitoring/domain"
	"joblet/pkg/logger"
)

// DiskCollector collects disk metrics from /proc/mounts and /proc/diskstats
type DiskCollector struct {
	logger    *logger.Logger
	lastStats map[string]*diskIOStats
	lastTime  time.Time
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

		// Convert sectors to bytes (assuming 512 bytes per sector)
		const sectorSize = 512

		stats[deviceName] = &diskIOStats{
			readsCompleted:  readCompleted,
			writesCompleted: writeCompleted,
			readBytes:       readSectors * sectorSize,
			writeBytes:      writeSectors * sectorSize,
			readTime:        readTime,
			writeTime:       writeTime,
			ioTime:          ioTime,
			weightedIOTime:  weightedIOTime,
		}
	}

	return stats, scanner.Err()
}
