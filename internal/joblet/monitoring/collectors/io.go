package collectors

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"joblet/internal/joblet/monitoring/domain"
	"joblet/pkg/logger"
)

// IOCollector collects system-wide I/O metrics from /proc/diskstats
type IOCollector struct {
	logger    *logger.Logger
	lastStats *ioSystemStats
	lastTime  time.Time
}

type ioSystemStats struct {
	readsCompleted  uint64
	writesCompleted uint64
	readBytes       uint64
	writeBytes      uint64
	readTime        uint64
	writeTime       uint64
	ioTime          uint64
	weightedIOTime  uint64
	devicesInIO     uint64
}

// NewIOCollector creates a new I/O metrics collector
func NewIOCollector() *IOCollector {
	return &IOCollector{
		logger: logger.WithField("component", "io-collector"),
	}
}

// Collect gathers current system-wide I/O metrics
func (c *IOCollector) Collect() (*domain.IOMetrics, error) {
	currentStats, err := c.readSystemIOStats()
	if err != nil {
		return nil, fmt.Errorf("failed to read I/O stats: %w", err)
	}

	currentTime := time.Now()

	metrics := &domain.IOMetrics{
		ReadsCompleted:  currentStats.readsCompleted,
		WritesCompleted: currentStats.writesCompleted,
		ReadBytes:       currentStats.readBytes,
		WriteBytes:      currentStats.writeBytes,
		ReadTime:        currentStats.readTime,
		WriteTime:       currentStats.writeTime,
		IOTime:          currentStats.ioTime,
		WeightedIOTime:  currentStats.weightedIOTime,
	}

	// Calculate derived metrics if we have previous stats
	if c.lastStats != nil && c.lastTime.Before(currentTime) {
		timeDelta := currentTime.Sub(c.lastTime).Seconds()

		if timeDelta > 0 {
			// Calculate queue depth (average number of I/O operations in flight)
			weightedDelta := float64(currentStats.weightedIOTime - c.lastStats.weightedIOTime)
			timeDeltaMs := timeDelta * 1000.0 // Convert to milliseconds

			if timeDeltaMs > 0 {
				metrics.QueueDepth = weightedDelta / timeDeltaMs
			}

			// Calculate utilization percentage
			ioDelta := float64(currentStats.ioTime - c.lastStats.ioTime)
			if timeDeltaMs > 0 {
				metrics.Utilization = (ioDelta / timeDeltaMs) * 100.0
				// Cap at 100%
				if metrics.Utilization > 100.0 {
					metrics.Utilization = 100.0
				}
			}
		}
	}

	// Store current stats for next calculation
	c.lastStats = currentStats
	c.lastTime = currentTime

	return metrics, nil
}

// readSystemIOStats aggregates I/O statistics across all block devices
func (c *IOCollector) readSystemIOStats() (*ioSystemStats, error) {
	file, err := os.Open("/proc/diskstats")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	stats := &ioSystemStats{}
	scanner := bufio.NewScanner(file)
	devicesWithIO := make(map[string]bool)

	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 14 {
			continue
		}

		deviceName := fields[2]

		// Skip virtual devices and partitions - focus on whole devices
		if c.shouldSkipDevice(deviceName) {
			continue
		}

		// Parse stats
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
		readBytes := readSectors * sectorSize
		writeBytes := writeSectors * sectorSize

		// Aggregate stats
		stats.readsCompleted += readCompleted
		stats.writesCompleted += writeCompleted
		stats.readBytes += readBytes
		stats.writeBytes += writeBytes
		stats.readTime += readTime
		stats.writeTime += writeTime
		stats.ioTime += ioTime
		stats.weightedIOTime += weightedIOTime

		// Track devices that have any I/O activity
		if readCompleted > 0 || writeCompleted > 0 {
			devicesWithIO[deviceName] = true
		}
	}

	stats.devicesInIO = uint64(len(devicesWithIO))

	return stats, scanner.Err()
}

// shouldSkipDevice determines if a device should be skipped in I/O aggregation
func (c *IOCollector) shouldSkipDevice(deviceName string) bool {
	// Skip loop devices
	if strings.HasPrefix(deviceName, "loop") {
		return true
	}

	// Skip RAM disks
	if strings.HasPrefix(deviceName, "ram") {
		return true
	}

	// Skip device mapper virtual devices (usually LVM)
	if strings.HasPrefix(deviceName, "dm-") {
		return true
	}

	// Skip CD/DVD drives
	if strings.HasPrefix(deviceName, "sr") {
		return true
	}

	// Skip partitions - we want whole devices only
	// This is a heuristic: if the device name ends with a digit, it's likely a partition
	if len(deviceName) > 0 {
		lastChar := deviceName[len(deviceName)-1]
		if lastChar >= '0' && lastChar <= '9' {
			// Check if it's a numbered partition (like sda1, sdb2, nvme0n1p1)
			// We'll be conservative and only skip obvious partitions
			if strings.Contains(deviceName, "sd") && len(deviceName) == 4 { // e.g., sda1
				return true
			}
			if strings.Contains(deviceName, "nvme") && strings.Contains(deviceName, "p") {
				return true // e.g., nvme0n1p1
			}
			if strings.Contains(deviceName, "mmcblk") && strings.Contains(deviceName, "p") {
				return true // e.g., mmcblk0p1
			}
		}
	}

	return false
}
