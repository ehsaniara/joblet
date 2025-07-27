package network

import (
	"fmt"
	"joblet/pkg/logger"
	"joblet/pkg/platform"
	"net"
	"os"
	"os/exec"
	"strings"
)

// NetworkSetup handles network namespace and configuration
type NetworkSetup struct {
	platform     platform.Platform
	logger       *logger.Logger
	networkStore NetworkStoreInterface
}

// NetworkStoreInterface this Interface to avoid circular dependency
type NetworkStoreInterface interface {
	GetNetworkConfig(name string) (*NetworkConfig, error)
}

// NewNetworkSetup creates a new network setup instance
func NewNetworkSetup(platform platform.Platform, networkStore NetworkStoreInterface) *NetworkSetup {
	return &NetworkSetup{
		platform:     platform,
		logger:       logger.WithField("component", "network-setup"),
		networkStore: networkStore,
	}
}

// SetupJobNetwork configures network for a job
func (ns *NetworkSetup) SetupJobNetwork(alloc *JobAllocation, pid int) error {
	log := ns.logger.WithFields(
		"jobID", alloc.JobID,
		"network", alloc.Network,
		"pid", pid)

	// Verify process exists before setup
	procPath := fmt.Sprintf("/proc/%d", pid)
	if _, err := os.Stat(procPath); err != nil {
		return fmt.Errorf("process %d does not exist: %w", pid, err)
	}

	log.Debug("setting up network for job")

	switch alloc.Network {
	case "none":
		log.Debug("no network configured for job")
		return nil

	case "isolated":
		return ns.setupIsolatedNetwork(pid)

	default:
		return ns.setupBridgeNetwork(alloc, pid)
	}
}

func (ns *NetworkSetup) setupIsolatedNetwork(pid int) error {
	log := ns.logger.WithFields("pid", pid)

	// 1. Enable IP forwarding
	if err := os.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1"), 0644); err != nil {
		ns.logger.Warn("failed to enable IP forwarding", "error", err)
	}

	// 2. Create veth pair
	vethHost := fmt.Sprintf("viso%d", pid%10000)
	vethPeer := fmt.Sprintf("viso%dp", pid%10000)

	log.Debug("creating veth pair", "host", vethHost, "peer", vethPeer)

	if err := ns.execCommand("ip", "link", "add", vethHost, "type", "veth", "peer", "name", vethPeer); err != nil {
		return fmt.Errorf("failed to create veth pair: %w", err)
	}

	// Cleanup function for error cases
	cleanup := func() {
		err := ns.execCommand("ip", "link", "delete", vethHost)
		if err != nil {
			return
		}
	}

	// 3. Move peer to namespace
	if err := ns.execCommand("ip", "link", "set", vethPeer, "netns", fmt.Sprintf("%d", pid)); err != nil {
		cleanup()
		return fmt.Errorf("failed to move veth to namespace: %w", err)
	}

	// 4. Configure host side
	if err := ns.execCommand("ip", "addr", "add", "10.255.255.1/30", "dev", vethHost); err != nil {
		cleanup()
		return fmt.Errorf("failed to configure host veth: %w", err)
	}

	if err := ns.execCommand("ip", "link", "set", vethHost, "up"); err != nil {
		cleanup()
		return fmt.Errorf("failed to bring up host veth: %w", err)
	}

	// 5. Setup NAT - Check if rule already exists to avoid duplicates
	natRuleExists := ns.execCommand("iptables", "-t", "nat", "-C", "POSTROUTING",
		"-s", "10.255.255.2/32", "-j", "MASQUERADE") == nil

	if !natRuleExists {
		if err := ns.execCommand("iptables", "-t", "nat", "-A", "POSTROUTING",
			"-s", "10.255.255.2/32", "-j", "MASQUERADE"); err != nil {
			ns.logger.Warn("failed to setup NAT", "error", err)
		}
	}

	// 6. Setup FORWARD rules - Insert at beginning for priority
	// Check if rules exist first to avoid duplicates
	inRuleExists := ns.execCommand("iptables", "-C", "FORWARD",
		"-i", vethHost, "-j", "ACCEPT") == nil

	if !inRuleExists {
		if err := ns.execCommand("iptables", "-I", "FORWARD", "1",
			"-i", vethHost, "-j", "ACCEPT"); err != nil {
			ns.logger.Warn("failed to add FORWARD in rule", "error", err)
		}
	}

	outRuleExists := ns.execCommand("iptables", "-C", "FORWARD",
		"-o", vethHost, "-j", "ACCEPT") == nil

	if !outRuleExists {
		if err := ns.execCommand("iptables", "-I", "FORWARD", "1",
			"-o", vethHost, "-j", "ACCEPT"); err != nil {
			ns.logger.Warn("failed to add FORWARD out rule", "error", err)
		}
	}

	// 7. Configure namespace side
	netnsPath := fmt.Sprintf("/proc/%d/ns/net", pid)
	nsCommands := [][]string{
		{"ip", "addr", "add", "10.255.255.2/30", "dev", vethPeer},
		{"ip", "link", "set", vethPeer, "up"},
		{"ip", "link", "set", "lo", "up"},
		{"ip", "route", "add", "default", "via", "10.255.255.1"},
	}

	for _, cmd := range nsCommands {
		if err := ns.execInNamespace(netnsPath, cmd...); err != nil {
			return fmt.Errorf("failed to configure namespace: %w", err)
		}
	}

	log.Debug("isolated network setup completed successfully")
	return nil
}

// setupBridgeNetwork sets up bridge network with internal connectivity
func (ns *NetworkSetup) setupBridgeNetwork(alloc *JobAllocation, pid int) error {
	log := ns.logger.WithFields(
		"bridge", alloc.Network,
		"ip", alloc.IP.String(),
		"vethHost", alloc.VethHost,
		"vethPeer", alloc.VethPeer)

	// Ensure bridge exists
	if err := ns.ensureBridge(alloc.Network); err != nil {
		return fmt.Errorf("failed to ensure bridge: %w", err)
	}

	// Create veth pair
	if err := ns.execCommand("ip", "link", "add", alloc.VethHost, "type", "veth", "peer", "name", alloc.VethPeer); err != nil {
		return fmt.Errorf("failed to create veth pair: %w", err)
	}

	// Attach host side to bridge
	bridgeName := fmt.Sprintf("joblet-%s", alloc.Network)
	if alloc.Network == "bridge" {
		bridgeName = "joblet0"
	}

	if err := ns.execCommand("ip", "link", "set", alloc.VethHost, "master", bridgeName); err != nil {
		return fmt.Errorf("failed to attach veth to bridge: %w", err)
	}
	if err := ns.execCommand("ip", "link", "set", alloc.VethHost, "up"); err != nil {
		return fmt.Errorf("failed to bring up host veth: %w", err)
	}

	// Move peer to namespace
	if err := ns.execCommand("ip", "link", "set", alloc.VethPeer, "netns", fmt.Sprintf("%d", pid)); err != nil {
		return fmt.Errorf("failed to move veth to namespace: %w", err)
	}

	// Configure namespace
	netnsPath := fmt.Sprintf("/proc/%d/ns/net", pid)

	// Calculate network details
	_, ipNet, _ := net.ParseCIDR(ns.getNetworkCIDR(alloc.Network))
	prefixLen, _ := ipNet.Mask.Size()

	nsCommands := [][]string{
		{"ip", "addr", "add", fmt.Sprintf("%s/%d", alloc.IP.String(), prefixLen), "dev", alloc.VethPeer},
		{"ip", "link", "set", alloc.VethPeer, "up"},
		{"ip", "link", "set", "lo", "up"},
		{"ip", "route", "add", "default", "via", ns.getGatewayIP(alloc.Network)},
	}

	for _, cmd := range nsCommands {
		if err := ns.execInNamespace(netnsPath, cmd...); err != nil {
			return fmt.Errorf("failed to configure namespace: %w", err)
		}
	}

	// Setup hosts file if hostname is specified
	if alloc.Hostname != "" {
		if err := ns.setupHostsFile(pid, alloc); err != nil {
			log.Warn("failed to setup hosts file", "error", err)
		}
	}

	log.Debug("network setup completed successfully")
	return nil
}

// ensureBridge ensures the bridge exists and is configured
func (ns *NetworkSetup) ensureBridge(networkName string) error {
	bridgeName := fmt.Sprintf("joblet-%s", networkName)
	if networkName == "bridge" {
		bridgeName = "joblet0"
	}

	// Check if bridge exists
	if err := ns.execCommand("ip", "link", "show", bridgeName); err == nil {
		ns.logger.Debug("bridge already exists, skipping creation", "bridge", bridgeName)
		return nil
	}

	// Create bridge
	if err := ns.execCommand("ip", "link", "add", bridgeName, "type", "bridge"); err != nil {
		return fmt.Errorf("failed to create bridge: %w", err)
	}

	// Configure bridge
	cidr := ns.getNetworkCIDR(networkName)
	gateway := ns.getGatewayIP(networkName)

	if err := ns.execCommand("ip", "addr", "add",
		fmt.Sprintf("%s/%s", gateway, strings.Split(cidr, "/")[1]),
		"dev", bridgeName); err != nil {
		return fmt.Errorf("failed to configure bridge IP: %w", err)
	}

	if err := ns.execCommand("ip", "link", "set", bridgeName, "up"); err != nil {
		return fmt.Errorf("failed to bring up bridge: %w", err)
	}

	// Enable IP forwarding
	if err := os.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1"), 0644); err != nil {
		ns.logger.Warn("failed to enable IP forwarding", "error", err)
	}

	// Setup NAT for external access
	if err := ns.execCommand("iptables", "-t", "nat", "-A", "POSTROUTING",
		"-s", cidr, "-j", "MASQUERADE"); err != nil {
		ns.logger.Warn("failed to setup NAT", "error", err)
	}

	// Setup network isolation
	if err := ns.setupNetworkIsolation(networkName, bridgeName); err != nil {
		ns.logger.Warn("failed to setup network isolation",
			"network", networkName, "error", err)
		// Don't fail the bridge creation, just log the warning
	}

	return nil
}

// CleanupJobNetwork cleans up network resources for a job
func (ns *NetworkSetup) CleanupJobNetwork(alloc *JobAllocation) error {
	if alloc.Network == "none" {
		return nil
	}

	// For isolated network, we need to extract veth name from PID
	if alloc.Network == "isolated" {
		// Try to find and delete the veth interface
		// The veth name was based on PID, but we might not have that info here
		// The kernel will clean it up anyway when namespace is destroyed
		ns.logger.Debug("isolated network cleanup - kernel will remove veth with namespace")

		// Try to remove the NAT rule (using the standard isolated IP)
		if err := ns.execCommand("iptables", "-t", "nat", "-D", "POSTROUTING",
			"-s", "10.255.255.2/32", "-j", "MASQUERADE"); err != nil {
			ns.logger.Debug("failed to remove NAT rule", "error", err)
		}
		return nil
	}

	// For bridge networks, remove veth pair
	if alloc.VethHost != "" {
		if err := ns.execCommand("ip", "link", "delete", alloc.VethHost); err != nil {
			// Log but don't fail - might already be cleaned up
			ns.logger.Debug("failed to delete veth", "veth", alloc.VethHost, "error", err)
		}
	}

	return nil
}

// Helper methods

func (ns *NetworkSetup) execCommand(args ...string) error {
	cmd := exec.Command(args[0], args[1:]...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %s", err, string(output))
	}
	return nil
}

func (ns *NetworkSetup) execInNamespace(netnsPath string, args ...string) error {
	nsenterArgs := append([]string{"--net=" + netnsPath}, args...)
	cmd := exec.Command("nsenter", nsenterArgs...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %s", err, string(output))
	}
	return nil
}

func (ns *NetworkSetup) getNetworkCIDR(networkName string) string {
	if ns.networkStore != nil {
		config, err := ns.networkStore.GetNetworkConfig(networkName)
		if err == nil && config != nil {
			return config.CIDR
		}
	}

	// Fallback for when networkStore is not available
	if networkName == "bridge" {
		return "172.20.0.0/16"
	}
	return "10.1.0.0/24"
}

func (ns *NetworkSetup) getGatewayIP(networkName string) string {
	cidr := ns.getNetworkCIDR(networkName)
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		// Fallback
		return "10.1.0.1"
	}

	// Gateway is first usable IP (.1)
	gateway := make(net.IP, 4)
	copy(gateway, ipNet.IP.To4())
	gateway[3] = 1

	return gateway.String()
}

func (ns *NetworkSetup) setupHostsFile(pid int, alloc *JobAllocation) error {
	// Setup custom hosts file for the job
	hostsContent := fmt.Sprintf(`127.0.0.1   localhost
%s   %s

# Other jobs in the same network can be resolved here
`, alloc.IP.String(), alloc.Hostname)

	// Write to a temporary file
	hostsPath := fmt.Sprintf("/tmp/joblet-hosts-%s", alloc.JobID)
	if err := os.WriteFile(hostsPath, []byte(hostsContent), 0644); err != nil {
		return err
	}

	// Bind mount into the namespace
	targetPath := fmt.Sprintf("/proc/%d/root/etc/hosts", pid)
	if err := ns.execCommand("mount", "--bind", hostsPath, targetPath); err != nil {
		return fmt.Errorf("failed to mount hosts file: %w", err)
	}

	return nil
}

// setupNetworkIsolation adds iptables rules to isolate this network from others
func (ns *NetworkSetup) setupNetworkIsolation(networkName string, bridgeName string) error {
	// Don't isolate the default bridge network (for backward compatibility)
	// You might want to change this policy
	if networkName == "bridge" {
		ns.logger.Debug("skipping isolation for default bridge network")
		return nil
	}

	// Get list of other bridges to isolate from
	bridges := ns.getExistingBridges()

	for _, otherBridge := range bridges {
		// Skip self
		if otherBridge == bridgeName {
			continue
		}

		// Skip if it's not a joblet bridge
		if !strings.HasPrefix(otherBridge, "joblet") {
			continue
		}

		// Add DROP rules in both directions
		// Block traffic FROM this bridge TO other bridge
		if err := ns.execCommand("iptables", "-I", "FORWARD", "1",
			"-i", bridgeName, "-o", otherBridge, "-j", "DROP"); err != nil {
			ns.logger.Warn("failed to add isolation rule",
				"from", bridgeName, "to", otherBridge, "error", err)
		}

		// Block traffic FROM other bridge TO this bridge
		if err := ns.execCommand("iptables", "-I", "FORWARD", "1",
			"-i", otherBridge, "-o", bridgeName, "-j", "DROP"); err != nil {
			ns.logger.Warn("failed to add isolation rule",
				"from", otherBridge, "to", bridgeName, "error", err)
		}

		ns.logger.Debug("added isolation rules between bridges",
			"bridge1", bridgeName, "bridge2", otherBridge)
	}

	return nil
}

// getExistingBridges returns a list of existing bridge interfaces
func (ns *NetworkSetup) getExistingBridges() []string {
	var bridges []string

	// List all network interfaces
	output, err := exec.Command("ip", "link", "show", "type", "bridge").Output()
	if err != nil {
		ns.logger.Warn("failed to list bridges", "error", err)
		return bridges
	}

	// Parse output to get bridge names
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		// Format: "3: joblet0: <BROADCAST,MULTICAST,UP,LOWER_UP>..."
		if strings.Contains(line, ": joblet") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				// Remove the trailing ':'
				bridgeName := strings.TrimSuffix(parts[1], ":")
				bridges = append(bridges, bridgeName)
			}
		}
	}

	return bridges
}

// Add this to the network removal logic

// cleanupNetworkIsolation removes iptables rules for a network being deleted
func (ns *NetworkSetup) cleanupNetworkIsolation(bridgeName string) error {
	// Get list of other bridges
	bridges := ns.getExistingBridges()

	for _, otherBridge := range bridges {
		// Skip self
		if otherBridge == bridgeName {
			continue
		}

		// Remove DROP rules in both directions
		// Use -D (delete) instead of -I (insert)
		_ = ns.execCommand("iptables", "-D", "FORWARD",
			"-i", bridgeName, "-o", otherBridge, "-j", "DROP")

		_ = ns.execCommand("iptables", "-D", "FORWARD",
			"-i", otherBridge, "-o", bridgeName, "-j", "DROP")
	}

	// Also remove NAT rule for this network's CIDR
	// This would need the CIDR passed in or looked up
	// For now, this is a simplified version

	return nil
}

// RemoveBridge Call this when removing a network
func (ns *NetworkSetup) RemoveBridge(networkName string) error {
	bridgeName := fmt.Sprintf("joblet-%s", networkName)
	if networkName == "bridge" {
		bridgeName = "joblet0"
	}

	// Clean up isolation rules first
	if err := ns.cleanupNetworkIsolation(bridgeName); err != nil {
		ns.logger.Warn("failed to cleanup isolation rules", "error", err)
	}

	// Get CIDR for NAT cleanup
	cidr := ns.getNetworkCIDR(networkName)

	// Remove NAT rule
	if err := ns.execCommand("iptables", "-t", "nat", "-D", "POSTROUTING",
		"-s", cidr, "-j", "MASQUERADE"); err != nil {
		ns.logger.Debug("failed to remove NAT rule", "error", err)
	}

	// Bring down the bridge
	if err := ns.execCommand("ip", "link", "set", bridgeName, "down"); err != nil {
		ns.logger.Debug("failed to bring down bridge", "error", err)
	}

	// Delete the bridge
	if err := ns.execCommand("ip", "link", "delete", bridgeName); err != nil {
		return fmt.Errorf("failed to delete bridge: %w", err)
	}

	ns.logger.Info("removed bridge", "bridge", bridgeName, "network", networkName)
	return nil
}
