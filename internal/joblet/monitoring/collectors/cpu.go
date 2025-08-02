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

// CPUCollector collects CPU metrics from /proc/stat and /proc/loadavg
type CPUCollector struct {
	logger    *logger.Logger
	lastStats *cpuStats
	lastTime  time.Time
}

type cpuStats struct {
	user      uint64
	nice      uint64
	system    uint64
	idle      uint64
	iowait    uint64
	irq       uint64
	softirq   uint64
	steal     uint64
	guest     uint64
	guestNice uint64
	cores     []coreStats
}

type coreStats struct {
	user    uint64
	nice    uint64
	system  uint64
	idle    uint64
	iowait  uint64
	irq     uint64
	softirq uint64
	steal   uint64
}

// NewCPUCollector creates a new CPU metrics collector
func NewCPUCollector() *CPUCollector {
	return &CPUCollector{
		logger: logger.WithField("component", "cpu-collector"),
	}
}

// Collect gathers current CPU metrics
func (c *CPUCollector) Collect() (*domain.CPUMetrics, error) {
	// Read current CPU stats
	currentStats, err := c.readCPUStats()
	if err != nil {
		return nil, fmt.Errorf("failed to read CPU stats: %w", err)
	}

	// Read load average
	loadAvg, err := c.readLoadAverage()
	if err != nil {
		return nil, fmt.Errorf("failed to read load average: %w", err)
	}

	currentTime := time.Now()

	metrics := &domain.CPUMetrics{
		LoadAverage: loadAvg,
		Cores:       len(currentStats.cores),
	}

	// Calculate usage percentages if we have previous stats
	if c.lastStats != nil && c.lastTime.Before(currentTime) {
		// Calculate overall CPU usage
		totalDelta := c.calculateTotalDelta(currentStats, c.lastStats)
		idleDelta := float64(currentStats.idle - c.lastStats.idle)

		if totalDelta > 0 {
			metrics.UsagePercent = (1.0 - idleDelta/totalDelta) * 100.0
			metrics.UserTime = float64(currentStats.user-c.lastStats.user) / totalDelta * 100.0
			metrics.SystemTime = float64(currentStats.system-c.lastStats.system) / totalDelta * 100.0
			metrics.IdleTime = idleDelta / totalDelta * 100.0
			metrics.IOWaitTime = float64(currentStats.iowait-c.lastStats.iowait) / totalDelta * 100.0
			metrics.StealTime = float64(currentStats.steal-c.lastStats.steal) / totalDelta * 100.0
		}

		// Calculate per-core usage
		metrics.PerCoreUsage = make([]float64, len(currentStats.cores))
		for i, core := range currentStats.cores {
			if i < len(c.lastStats.cores) {
				lastCore := c.lastStats.cores[i]
				coreTotalDelta := c.calculateCoreTotalDelta(&core, &lastCore)
				coreIdleDelta := float64(core.idle - lastCore.idle)

				if coreTotalDelta > 0 {
					metrics.PerCoreUsage[i] = (1.0 - coreIdleDelta/coreTotalDelta) * 100.0
				}
			}
		}
	}

	// Store current stats for next calculation
	c.lastStats = currentStats
	c.lastTime = currentTime

	return metrics, nil
}

// readCPUStats reads CPU statistics from /proc/stat
func (c *CPUCollector) readCPUStats() (*cpuStats, error) {
	file, err := os.Open("/proc/stat")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	stats := &cpuStats{}

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "cpu ") {
			// Parse overall CPU stats
			fields := strings.Fields(line)
			if len(fields) < 8 {
				continue
			}

			stats.user, _ = strconv.ParseUint(fields[1], 10, 64)
			stats.nice, _ = strconv.ParseUint(fields[2], 10, 64)
			stats.system, _ = strconv.ParseUint(fields[3], 10, 64)
			stats.idle, _ = strconv.ParseUint(fields[4], 10, 64)
			stats.iowait, _ = strconv.ParseUint(fields[5], 10, 64)
			stats.irq, _ = strconv.ParseUint(fields[6], 10, 64)
			stats.softirq, _ = strconv.ParseUint(fields[7], 10, 64)

			if len(fields) > 8 {
				stats.steal, _ = strconv.ParseUint(fields[8], 10, 64)
			}
			if len(fields) > 9 {
				stats.guest, _ = strconv.ParseUint(fields[9], 10, 64)
			}
			if len(fields) > 10 {
				stats.guestNice, _ = strconv.ParseUint(fields[10], 10, 64)
			}
		} else if strings.HasPrefix(line, "cpu") && len(line) > 3 {
			// Parse per-core CPU stats
			fields := strings.Fields(line)
			if len(fields) < 8 {
				continue
			}

			core := coreStats{
				user:    parseUint64(fields[1]),
				nice:    parseUint64(fields[2]),
				system:  parseUint64(fields[3]),
				idle:    parseUint64(fields[4]),
				iowait:  parseUint64(fields[5]),
				irq:     parseUint64(fields[6]),
				softirq: parseUint64(fields[7]),
			}

			if len(fields) > 8 {
				core.steal = parseUint64(fields[8])
			}

			stats.cores = append(stats.cores, core)
		}
	}

	return stats, scanner.Err()
}

// readLoadAverage reads load average from /proc/loadavg
func (c *CPUCollector) readLoadAverage() ([3]float64, error) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return [3]float64{}, err
	}

	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return [3]float64{}, fmt.Errorf("invalid loadavg format")
	}

	load1, _ := strconv.ParseFloat(fields[0], 64)
	load5, _ := strconv.ParseFloat(fields[1], 64)
	load15, _ := strconv.ParseFloat(fields[2], 64)

	return [3]float64{load1, load5, load15}, nil
}

// calculateTotalDelta calculates the total CPU time delta
func (c *CPUCollector) calculateTotalDelta(current, last *cpuStats) float64 {
	currentTotal := current.user + current.nice + current.system + current.idle +
		current.iowait + current.irq + current.softirq + current.steal
	lastTotal := last.user + last.nice + last.system + last.idle +
		last.iowait + last.irq + last.softirq + last.steal

	return float64(currentTotal - lastTotal)
}

// calculateCoreTotalDelta calculates the total CPU time delta for a core
func (c *CPUCollector) calculateCoreTotalDelta(current, last *coreStats) float64 {
	currentTotal := current.user + current.nice + current.system + current.idle +
		current.iowait + current.irq + current.softirq + current.steal
	lastTotal := last.user + last.nice + last.system + last.idle +
		last.iowait + last.irq + last.softirq + last.steal

	return float64(currentTotal - lastTotal)
}

// parseUint64 safely parses a string to uint64
func parseUint64(s string) uint64 {
	val, _ := strconv.ParseUint(s, 10, 64)
	return val
}
