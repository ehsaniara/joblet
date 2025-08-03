package collectors

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"joblet/internal/joblet/monitoring/domain"
	"joblet/pkg/logger"
)

// MemoryCollector collects memory metrics from /proc/meminfo
type MemoryCollector struct {
	logger *logger.Logger
}

// NewMemoryCollector creates a new memory metrics collector
func NewMemoryCollector() *MemoryCollector {
	return &MemoryCollector{
		logger: logger.WithField("component", "memory-collector"),
	}
}

// Collect gathers current memory metrics
func (c *MemoryCollector) Collect() (*domain.MemoryMetrics, error) {
	meminfo, err := c.readMemInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to read memory info: %w", err)
	}

	// Calculate derived metrics
	usedBytes := meminfo["MemTotal"] - meminfo["MemFree"] - meminfo["Buffers"] - meminfo["Cached"]
	usagePercent := 0.0
	if meminfo["MemTotal"] > 0 {
		usagePercent = float64(usedBytes) / float64(meminfo["MemTotal"]) * 100.0
	}

	metrics := &domain.MemoryMetrics{
		TotalBytes:     meminfo["MemTotal"] * 1024,                          // Convert KB to bytes
		UsedBytes:      usedBytes * 1024,                                    // Convert KB to bytes
		FreeBytes:      meminfo["MemFree"] * 1024,                           // Convert KB to bytes
		AvailableBytes: meminfo["MemAvailable"] * 1024,                      // Convert KB to bytes
		CachedBytes:    meminfo["Cached"] * 1024,                            // Convert KB to bytes
		BufferedBytes:  meminfo["Buffers"] * 1024,                           // Convert KB to bytes
		SwapTotal:      meminfo["SwapTotal"] * 1024,                         // Convert KB to bytes
		SwapUsed:       (meminfo["SwapTotal"] - meminfo["SwapFree"]) * 1024, // Convert KB to bytes
		SwapFree:       meminfo["SwapFree"] * 1024,                          // Convert KB to bytes
		UsagePercent:   usagePercent,
	}

	return metrics, nil
}

// readMemInfo reads memory information from /proc/meminfo
func (c *MemoryCollector) readMemInfo() (map[string]uint64, error) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	meminfo := make(map[string]uint64)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		// Remove colon from field name
		key := strings.TrimSuffix(fields[0], ":")

		// Parse value (assuming it's in KB)
		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			c.logger.Debug("failed to parse memory value", "key", key, "value", fields[1])
			continue
		}

		meminfo[key] = value
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Ensure we have the minimum required fields
	requiredFields := []string{"MemTotal", "MemFree", "MemAvailable", "Cached", "Buffers", "SwapTotal", "SwapFree"}
	for _, field := range requiredFields {
		if _, exists := meminfo[field]; !exists {
			meminfo[field] = 0
		}
	}

	return meminfo, nil
}
