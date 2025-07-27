package network

import (
	"fmt"
	"joblet/pkg/logger"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// NetworkCleaner handles cleanup of orphaned network resources
type NetworkCleaner struct {
	logger *logger.Logger
}

// NewNetworkCleaner creates a new network cleaner
func NewNetworkCleaner() *NetworkCleaner {
	return &NetworkCleaner{
		logger: logger.WithField("component", "network-cleaner"),
	}
}

// CleanupOrphanedInterfaces removes veth interfaces without active jobs
func (nc *NetworkCleaner) CleanupOrphanedInterfaces() error {
	// List all veth interfaces
	cmd := exec.Command("ip", "link", "show")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to list interfaces: %w", err)
	}

	// Pattern to match joblet veth interfaces
	vethPattern := regexp.MustCompile(`\d+: (veth[a-f0-9]{8}|viso\d+)@`)
	matches := vethPattern.FindAllStringSubmatch(string(output), -1)

	cleaned := 0
	for _, match := range matches {
		iface := match[1]

		// Check if interface is orphaned (no namespace attached)
		if nc.isOrphaned(iface) {
			if err := nc.removeInterface(iface); err != nil {
				nc.logger.Warn("failed to remove orphaned interface",
					"interface", iface,
					"error", err)
				continue
			}
			cleaned++
			nc.logger.Debug("removed orphaned interface", "interface", iface)
		}
	}

	if cleaned > 0 {
		nc.logger.Info("cleaned orphaned network interfaces", "count", cleaned)
	}

	return nil
}

// isOrphaned checks if a veth interface has no active namespace
func (nc *NetworkCleaner) isOrphaned(iface string) bool {
	// Check if peer exists in any namespace
	cmd := exec.Command("ip", "link", "show", iface)
	output, err := cmd.Output()
	if err != nil {
		return true // If we can't query it, consider it orphaned
	}

	// Check for "link-netns" or "@if" which indicates active peer
	outputStr := string(output)
	return !strings.Contains(outputStr, "link-netns") &&
		strings.Contains(outputStr, "@NONE")
}

// removeInterface safely removes a network interface
func (nc *NetworkCleaner) removeInterface(iface string) error {
	cmd := exec.Command("ip", "link", "delete", iface)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to delete %s: %s", iface, string(output))
	}
	return nil
}

// StartPeriodicCleanup runs cleanup every interval
func (nc *NetworkCleaner) StartPeriodicCleanup(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			if err := nc.CleanupOrphanedInterfaces(); err != nil {
				nc.logger.Error("periodic cleanup failed", "error", err)
			}
		}
	}()
}
