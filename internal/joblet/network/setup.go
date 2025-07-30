package network

import (
	"bytes"
	"fmt"
	"joblet/pkg/logger"
	"joblet/pkg/platform"
	"net"
	"strings"
)

// NetworkSetup handles network namespace and configuration
type NetworkSetup struct {
	platform     platform.Platform
	logger       *logger.Logger
	networkStore NetworkStoreInterface
}

// NetworkStoreInterface defines the contract for network configuration storage.
// This interface is used to avoid circular dependencies between the network setup
// and network store packages. It provides access to network configuration data
// needed for bridge and IP address management.
type NetworkStoreInterface interface {
	GetNetworkConfig(name string) (*NetworkConfig, error)
}

// NewNetworkSetup creates a new network setup instance with platform abstraction.
// This constructor initializes the NetworkSetup with the provided platform interface
// for OS operations and a network store interface for configuration data.
// The platform abstraction enables testing and cross-platform compatibility.
func NewNetworkSetup(platform platform.Platform, networkStore NetworkStoreInterface) *NetworkSetup {
	return &NetworkSetup{
		platform:     platform,
		logger:       logger.WithField("component", "network-setup"),
		networkStore: networkStore,
	}
}

// SetupJobNetwork configures network isolation and connectivity for a job process.
// It validates the target process exists, then delegates to the appropriate network
// setup method based on the allocation type:
//   - "none": No network configuration (job runs without network access)
//   - "isolated": Creates an isolated network with NAT for external connectivity
//   - default: Sets up bridge-based networking with inter-job communication
//
// The method ensures proper network namespace configuration and resource allocation.
func (ns *NetworkSetup) SetupJobNetwork(alloc *JobAllocation, pid int) error {
	log := ns.logger.WithFields(
		"jobID", alloc.JobID,
		"network", alloc.Network,
		"pid", pid)

	// Verify process exists before setup
	procPath := fmt.Sprintf("/proc/%d", pid)
	if _, err := ns.platform.Stat(procPath); err != nil {
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

// setupIsolatedNetwork creates a completely isolated network environment for a job.
// This method implements a point-to-point network connection between the host and
// the job's network namespace using a veth pair. The setup includes:
//  1. Enabling IP forwarding on the host
//  2. Creating a veth pair (viso<pid> on host, viso<pid>p in namespace)
//  3. Configuring host-side networking (10.255.255.1/30)
//  4. Setting up NAT rules for external connectivity
//  5. Configuring FORWARD rules for traffic flow
//  6. Setting up namespace-side networking (10.255.255.2/30 with default route)
//
// This provides complete network isolation while allowing controlled external access.
func (ns *NetworkSetup) setupIsolatedNetwork(pid int) error {
	log := ns.logger.WithFields("pid", pid)

	// 1. Enable IP forwarding
	if err := ns.platform.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1"), 0644); err != nil {
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

// setupBridgeNetwork configures bridge-based networking for inter-job communication.
// This method creates a shared network environment where jobs can communicate with
// each other through a Linux bridge. The setup process includes:
//  1. Ensuring the target bridge exists and is properly configured
//  2. Creating a veth pair connecting the host bridge to the job namespace
//  3. Attaching the host-side veth to the appropriate bridge
//  4. Moving the peer veth into the job's network namespace
//  5. Configuring IP addressing and routing within the namespace
//  6. Setting up hostname resolution if specified
//
// Jobs on the same bridge can communicate directly using their assigned IP addresses.
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

// ensureBridge creates and configures a Linux bridge for job networking if it doesn't exist.
// This method handles the complete bridge lifecycle including:
//  1. Checking if the bridge already exists (avoiding duplicate creation)
//  2. Creating the bridge device using the Linux kernel bridge driver
//  3. Assigning the gateway IP address to the bridge interface
//  4. Bringing the bridge interface up for traffic forwarding
//  5. Enabling IP forwarding for inter-network communication
//  6. Setting up NAT rules for external connectivity
//  7. Configuring network isolation rules to prevent cross-network traffic
//
// The bridge serves as the central hub for all jobs in the same network.
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
	if err := ns.platform.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1"), 0644); err != nil {
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

// CleanupJobNetwork removes network resources allocated to a job after completion.
// This method performs network-type-specific cleanup operations:
//   - "none": No cleanup required
//   - "isolated": Removes NAT rules (veth interfaces cleaned up by kernel)
//   - bridge networks: Deletes the host-side veth interface
//
// The cleanup is designed to be idempotent and handle cases where resources
// may have already been cleaned up by the kernel or other processes.
// Namespace destruction automatically removes namespace-side interfaces.
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

// execCommand executes a system command using the platform abstraction layer.
// This helper method creates a command with stdout/stderr capture for error reporting.
// It returns a formatted error containing both the original error and command output
// if the command fails, enabling better debugging of network configuration issues.
func (ns *NetworkSetup) execCommand(args ...string) error {
	cmd := ns.platform.CreateCommand(args[0], args[1:]...)
	var output bytes.Buffer
	cmd.SetStdout(&output)
	cmd.SetStderr(&output)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %s", err, output.String())
	}
	return nil
}

// execInNamespace executes a command within a specific network namespace.
// This method uses nsenter to run commands in the target network namespace,
// enabling configuration of network interfaces and routes from the host context.
// The netnsPath should point to /proc/<pid>/ns/net for the target namespace.
func (ns *NetworkSetup) execInNamespace(netnsPath string, args ...string) error {
	nsenterArgs := append([]string{"--net=" + netnsPath}, args...)
	cmd := ns.platform.CreateCommand("nsenter", nsenterArgs...)
	var output bytes.Buffer
	cmd.SetStdout(&output)
	cmd.SetStderr(&output)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %s", err, output.String())
	}
	return nil
}

// getNetworkCIDR retrieves the CIDR block for a named network.
// This method first attempts to get the CIDR from the network store configuration,
// then falls back to hardcoded defaults if the store is unavailable:
//   - "bridge" network: 172.20.0.0/16
//   - other networks: 10.1.0.0/24
//
// The CIDR defines the IP address range available for job allocation.
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

// getGatewayIP calculates the gateway address for a network.
// This method derives the gateway IP by taking the network CIDR and setting
// the host portion to .1 (first usable address in the subnet).
// For example, 172.20.0.0/16 becomes gateway 172.20.0.1.
// Returns a fallback of 10.1.0.1 if CIDR parsing fails.
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

// setupHostsFile creates a custom /etc/hosts file for hostname resolution within the job.
// This method generates a hosts file containing:
//   - Standard localhost mapping (127.0.0.1 -> localhost)
//   - Job-specific hostname mapping (job IP -> job hostname)
//   - Placeholder for future inter-job hostname resolution
//
// The hosts file is bind-mounted into the job's namespace, replacing the default
// /etc/hosts and enabling hostname-based communication within the network.
func (ns *NetworkSetup) setupHostsFile(pid int, alloc *JobAllocation) error {
	// Setup custom hosts file for the job
	hostsContent := fmt.Sprintf(`127.0.0.1   localhost
%s   %s

# Other jobs in the same network can be resolved here
`, alloc.IP.String(), alloc.Hostname)

	// Write to a temporary file
	hostsPath := fmt.Sprintf("/tmp/joblet-hosts-%s", alloc.JobID)
	if err := ns.platform.WriteFile(hostsPath, []byte(hostsContent), 0644); err != nil {
		return err
	}

	// Bind mount into the namespace
	targetPath := fmt.Sprintf("/proc/%d/root/etc/hosts", pid)
	if err := ns.execCommand("mount", "--bind", hostsPath, targetPath); err != nil {
		return fmt.Errorf("failed to mount hosts file: %w", err)
	}

	return nil
}

// setupNetworkIsolation implements network segmentation using iptables rules.
// This method creates firewall rules to prevent cross-network communication by:
//  1. Discovering all existing joblet-managed bridge interfaces
//  2. Creating bidirectional DROP rules between the current bridge and others
//  3. Inserting rules at high priority (position 1) in the FORWARD chain
//  4. Skipping isolation for the default "bridge" network (for compatibility)
//
// This ensures that jobs in different networks cannot communicate with each other,
// providing network-level security isolation between different job groups.
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

// getExistingBridges discovers all joblet-managed bridge interfaces on the system.
// This method executes "ip link show type bridge" and parses the output to extract
// bridge interface names that match the joblet naming pattern ("joblet*").
// The parsing handles the standard iproute2 output format:
//
//	"<index>: <interface_name>: <flags> ..."
//
// This information is used for network isolation rule setup and cleanup operations.
// Returns a slice of bridge names or an empty slice if discovery fails.
func (ns *NetworkSetup) getExistingBridges() []string {
	var bridges []string

	// List all network interfaces
	cmd := ns.platform.CreateCommand("ip", "link", "show", "type", "bridge")
	var output bytes.Buffer
	cmd.SetStdout(&output)
	if err := cmd.Run(); err != nil {
		ns.logger.Warn("failed to list bridges", "error", err)
		return bridges
	}

	// Parse output to get bridge names
	lines := strings.Split(output.String(), "\n")
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

// cleanupNetworkIsolation removes firewall rules when a network bridge is deleted.
// This method performs the reverse operation of setupNetworkIsolation by:
//  1. Discovering all remaining bridge interfaces on the system
//  2. Removing bidirectional DROP rules between the target bridge and others
//  3. Using iptables -D (delete) to remove previously created isolation rules
//  4. Ignoring errors for rules that may not exist (idempotent operation)
//
// This cleanup prevents iptables rule accumulation and ensures proper firewall
// state when networks are dynamically created and destroyed.
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

// RemoveBridge completely removes a network bridge and associated resources.
// This method performs comprehensive bridge cleanup including:
//  1. Removing network isolation rules to prevent iptables rule leakage
//  2. Cleaning up NAT rules for the network's CIDR range
//  3. Bringing the bridge interface down gracefully
//  4. Deleting the bridge device from the kernel
//
// The operation is designed to be as thorough as possible while handling
// partial failures gracefully. Some cleanup steps may fail if resources
// were already cleaned up, but the core bridge deletion will still proceed.
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
