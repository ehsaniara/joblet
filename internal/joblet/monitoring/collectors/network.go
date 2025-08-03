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

// NetworkCollector collects network interface metrics from /proc/net/dev
type NetworkCollector struct {
	logger    *logger.Logger
	lastStats map[string]*networkInterfaceStats
	lastTime  time.Time
}

type networkInterfaceStats struct {
	bytesReceived   uint64
	packetsReceived uint64
	errorsReceived  uint64
	dropsReceived   uint64
	bytesSent       uint64
	packetsSent     uint64
	errorsSent      uint64
	dropsSent       uint64
}

// NewNetworkCollector creates a new network metrics collector
func NewNetworkCollector() *NetworkCollector {
	return &NetworkCollector{
		logger:    logger.WithField("component", "network-collector"),
		lastStats: make(map[string]*networkInterfaceStats),
	}
}

// Collect gathers current network metrics
func (c *NetworkCollector) Collect() ([]domain.NetworkMetrics, error) {
	currentStats, err := c.readNetworkStats()
	if err != nil {
		return nil, fmt.Errorf("failed to read network stats: %w", err)
	}

	currentTime := time.Now()
	var metrics []domain.NetworkMetrics

	for interfaceName, current := range currentStats {
		// Skip loopback and virtual interfaces we don't care about
		if c.shouldSkipInterface(interfaceName) {
			continue
		}

		metric := domain.NetworkMetrics{
			Interface:       interfaceName,
			BytesReceived:   current.bytesReceived,
			BytesSent:       current.bytesSent,
			PacketsReceived: current.packetsReceived,
			PacketsSent:     current.packetsSent,
			ErrorsReceived:  current.errorsReceived,
			ErrorsSent:      current.errorsSent,
			DropsReceived:   current.dropsReceived,
			DropsSent:       current.dropsSent,
		}

		// Calculate throughput metrics if we have previous stats
		if last, exists := c.lastStats[interfaceName]; exists && c.lastTime.Before(currentTime) {
			timeDelta := currentTime.Sub(c.lastTime).Seconds()

			if timeDelta > 0 {
				// Calculate throughput in bytes per second
				metric.RxThroughputBPS = float64(current.bytesReceived-last.bytesReceived) / timeDelta
				metric.TxThroughputBPS = float64(current.bytesSent-last.bytesSent) / timeDelta

				// Calculate packets per second
				metric.RxPacketsPerSec = float64(current.packetsReceived-last.packetsReceived) / timeDelta
				metric.TxPacketsPerSec = float64(current.packetsSent-last.packetsSent) / timeDelta
			}
		}

		metrics = append(metrics, metric)
	}

	// Store current stats for next calculation
	c.lastStats = currentStats
	c.lastTime = currentTime

	return metrics, nil
}

// readNetworkStats reads network interface statistics from /proc/net/dev
func (c *NetworkCollector) readNetworkStats() (map[string]*networkInterfaceStats, error) {
	file, err := os.Open("/proc/net/dev")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	stats := make(map[string]*networkInterfaceStats)
	scanner := bufio.NewScanner(file)

	// Skip the first two header lines
	for i := 0; i < 2 && scanner.Scan(); i++ {
		// Skip header lines
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Find the colon that separates interface name from stats
		colonIndex := strings.Index(line, ":")
		if colonIndex == -1 {
			continue
		}

		interfaceName := strings.TrimSpace(line[:colonIndex])
		statsLine := strings.TrimSpace(line[colonIndex+1:])

		fields := strings.Fields(statsLine)
		if len(fields) < 16 {
			c.logger.Debug("insufficient fields in network stats", "interface", interfaceName, "fields", len(fields))
			continue
		}

		// Parse receive stats
		bytesReceived, _ := strconv.ParseUint(fields[0], 10, 64)
		packetsReceived, _ := strconv.ParseUint(fields[1], 10, 64)
		errorsReceived, _ := strconv.ParseUint(fields[2], 10, 64)
		dropsReceived, _ := strconv.ParseUint(fields[3], 10, 64)

		// Parse transmit stats (starting at field 8)
		bytesSent, _ := strconv.ParseUint(fields[8], 10, 64)
		packetsSent, _ := strconv.ParseUint(fields[9], 10, 64)
		errorsSent, _ := strconv.ParseUint(fields[10], 10, 64)
		dropsSent, _ := strconv.ParseUint(fields[11], 10, 64)

		stats[interfaceName] = &networkInterfaceStats{
			bytesReceived:   bytesReceived,
			packetsReceived: packetsReceived,
			errorsReceived:  errorsReceived,
			dropsReceived:   dropsReceived,
			bytesSent:       bytesSent,
			packetsSent:     packetsSent,
			errorsSent:      errorsSent,
			dropsSent:       dropsSent,
		}
	}

	return stats, scanner.Err()
}

// shouldSkipInterface determines if an interface should be skipped in metrics
func (c *NetworkCollector) shouldSkipInterface(interfaceName string) bool {
	// Skip loopback
	if interfaceName == "lo" {
		return true
	}

	// Skip common virtual interfaces that are typically not interesting
	skipPrefixes := []string{
		"veth",   // Docker/container virtual ethernet
		"br-",    // Docker bridges
		"docker", // Docker interfaces
		"virbr",  // libvirt bridges
		"tap",    // TAP interfaces
		"tun",    // TUN interfaces
	}

	for _, prefix := range skipPrefixes {
		if strings.HasPrefix(interfaceName, prefix) {
			return true
		}
	}

	return false
}
