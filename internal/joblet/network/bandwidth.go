package network

import (
	"fmt"
	"joblet/pkg/logger"
	"os/exec"
	"strconv"
)

// BandwidthLimiter handles network bandwidth limiting using tc (traffic control)
type BandwidthLimiter struct {
	logger *logger.Logger
}

// NewBandwidthLimiter creates a new bandwidth limiter
func NewBandwidthLimiter() *BandwidthLimiter {
	return &BandwidthLimiter{
		logger: logger.WithField("component", "bandwidth-limiter"),
	}
}

// ApplyJobLimits applies bandwidth limits to a job's veth interface
func (bl *BandwidthLimiter) ApplyJobLimits(vethName string, limits NetworkLimits) error {
	// Apply egress (outgoing) limits on the host-side veth
	if limits.EgressBPS > 0 {
		if err := bl.applyEgressLimit(vethName, limits.EgressBPS, limits.BurstSize); err != nil {
			return fmt.Errorf("failed to apply egress limit: %w", err)
		}
	}

	// Apply ingress (incoming) limits using ifb (intermediate functional block)
	if limits.IngressBPS > 0 {
		if err := bl.applyIngressLimit(vethName, limits.IngressBPS, limits.BurstSize); err != nil {
			return fmt.Errorf("failed to apply ingress limit: %w", err)
		}
	}

	bl.logger.Debug("applied bandwidth limits",
		"interface", vethName,
		"egress", formatBandwidth(limits.EgressBPS),
		"ingress", formatBandwidth(limits.IngressBPS))

	return nil
}

// applyEgressLimit applies outgoing bandwidth limit
func (bl *BandwidthLimiter) applyEgressLimit(iface string, bps int64, burst int) error {
	// Remove existing qdisc
	_ = bl.execCommand("tc", "qdisc", "del", "dev", iface, "root")

	// Calculate rate and burst
	rate := formatTCRate(bps)
	burstSize := calculateBurst(bps, burst)

	if err := bl.execCommand("tc", "qdisc", "add", "dev", iface, "root", "handle", "1:", "htb"); err != nil {
		return err
	}

	// Add HTB class with rate limit
	if err := bl.execCommand("tc", "class", "add", "dev", iface, "parent", "1:", "classid", "1:1",
		"htb", "rate", rate, "burst", burstSize); err != nil {
		return err
	}

	// filter to direct all traffic to the limited class
	if err := bl.execCommand("tc", "filter", "add", "dev", iface, "protocol", "ip", "parent", "1:0",
		"prio", "1", "u32", "match", "ip", "dst", "0.0.0.0/0", "flowid", "1:1"); err != nil {
		return err
	}

	return nil
}

// applyIngressLimit applies incoming bandwidth limit using IFB
func (bl *BandwidthLimiter) applyIngressLimit(iface string, bps int64, burst int) error {
	// Ensure IFB module is loaded
	_ = bl.execCommand("modprobe", "ifb")

	// Find available IFB device
	ifbDev := bl.findAvailableIFB()
	if ifbDev == "" {
		return fmt.Errorf("no available IFB device")
	}

	// Bring up IFB device
	if err := bl.execCommand("ip", "link", "set", "dev", ifbDev, "up"); err != nil {
		return err
	}

	// Redirect ingress traffic to IFB
	_ = bl.execCommand("tc", "qdisc", "del", "dev", iface, "ingress")
	if err := bl.execCommand("tc", "qdisc", "add", "dev", iface, "ingress"); err != nil {
		return err
	}

	// filter to redirect to IFB
	if err := bl.execCommand("tc", "filter", "add", "dev", iface, "parent", "ffff:", "protocol", "ip",
		"u32", "match", "ip", "src", "0.0.0.0/0", "action", "mirred", "egress", "redirect", "dev", ifbDev); err != nil {
		return err
	}

	// Apply rate limit on IFB device (as egress)
	return bl.applyEgressLimit(ifbDev, bps, burst)
}

// RemoveJobLimits removes bandwidth limits from a job
func (bl *BandwidthLimiter) RemoveJobLimits(vethName string) error {
	// Remove egress qdisc
	_ = bl.execCommand("tc", "qdisc", "del", "dev", vethName, "root")

	// Remove ingress qdisc
	_ = bl.execCommand("tc", "qdisc", "del", "dev", vethName, "ingress")

	bl.logger.Debug("removed bandwidth limits", "interface", vethName)
	return nil
}

// GetJobStatistics retrieves bandwidth usage statistics
func (bl *BandwidthLimiter) GetJobStatistics(vethName string) (*BandwidthStats, error) {
	// Get statistics from tc
	_, err := bl.getOutput("tc", "-s", "class", "show", "dev", vethName)
	if err != nil {
		return nil, err
	}

	// Parse statistics (simplified - real implementation would parse tc output)
	stats := &BandwidthStats{
		Interface:       vethName,
		BytesSent:       0,
		BytesReceived:   0,
		PacketsSent:     0,
		PacketsReceived: 0,
	}

	// This would parse the tc output to extract actual statistics
	// For now, return empty stats
	return stats, nil
}

// Helper functions

func (bl *BandwidthLimiter) execCommand(args ...string) error {
	cmd := exec.Command(args[0], args[1:]...)
	if output, err := cmd.CombinedOutput(); err != nil {
		bl.logger.Debug("tc command failed",
			"command", args,
			"output", string(output),
			"error", err)
		// Don't return error for cleanup commands
		if args[1] == "del" {
			return nil
		}
		return fmt.Errorf("%s: %s", err, string(output))
	}
	return nil
}

func (bl *BandwidthLimiter) getOutput(args ...string) (string, error) {
	cmd := exec.Command(args[0], args[1:]...)
	output, err := cmd.Output()
	return string(output), err
}

func (bl *BandwidthLimiter) findAvailableIFB() string {
	// Check for first available IFB device
	for i := 0; i < 10; i++ {
		ifb := fmt.Sprintf("ifb%d", i)
		// Check if device exists
		if err := exec.Command("ip", "link", "show", ifb).Run(); err == nil {
			return ifb
		}
	}
	return ""
}

// formatTCRate formats bandwidth for tc command
func formatTCRate(bps int64) string {
	if bps >= 1000000 {
		return fmt.Sprintf("%dmbit", bps/1000000)
	} else if bps >= 1000 {
		return fmt.Sprintf("%dkbit", bps/1000)
	}
	return fmt.Sprintf("%dbit", bps)
}

// calculateBurst calculates burst size
func calculateBurst(bps int64, burst int) string {
	if burst > 0 {
		return fmt.Sprintf("%dk", burst)
	}
	// Default: 10ms worth of traffic
	burstBytes := bps / 100
	if burstBytes < 1500 {
		burstBytes = 1500 // Minimum MTU
	}
	return strconv.FormatInt(burstBytes, 10)
}

// formatBandwidth formats bandwidth for logging
func formatBandwidth(bps int64) string {
	if bps <= 0 {
		return "unlimited"
	}
	if bps >= 1000000000 {
		return fmt.Sprintf("%.1f Gbps", float64(bps)/1000000000)
	} else if bps >= 1000000 {
		return fmt.Sprintf("%.1f Mbps", float64(bps)/1000000)
	} else if bps >= 1000 {
		return fmt.Sprintf("%.1f Kbps", float64(bps)/1000)
	}
	return fmt.Sprintf("%d bps", bps)
}
