package network

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
)

// NetworkValidator handles validation of network configurations
type NetworkValidator struct{}

// NewNetworkValidator creates a new network validator
func NewNetworkValidator() *NetworkValidator {
	return &NetworkValidator{}
}

// ValidateCIDR validates a CIDR block for network creation, checking format validity,
// minimum subnet size requirements (/30 minimum), and overlap detection with existing networks.
// It also validates against system network conflicts to prevent routing issues.
func (nv *NetworkValidator) ValidateCIDR(cidr string, existingNetworks map[string]string) error {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf("invalid CIDR format: %w", err)
	}

	// Check minimum subnet size (/30 minimum for at least 2 usable IPs)
	ones, _ := ipNet.Mask.Size()
	if ones > 30 {
		return fmt.Errorf("subnet too small: minimum /30 required")
	}

	// Check for overlaps with existing networks
	for name, existingCIDR := range existingNetworks {
		if nv.networksOverlap(cidr, existingCIDR) {
			return fmt.Errorf("CIDR %s overlaps with network '%s' (%s)",
				cidr, name, existingCIDR)
		}
	}

	// Check for conflicts with system networks
	if err := nv.checkSystemNetworkConflicts(cidr); err != nil {
		return err
	}

	return nil
}

// ValidateBridgeName checks if bridge name is valid and available
func (nv *NetworkValidator) ValidateBridgeName(name string) error {
	// Check name length and format
	if len(name) > 15 { // Linux interface name limit
		return fmt.Errorf("bridge name too long (max 15 chars)")
	}

	// Check for invalid characters
	if !isValidInterfaceName(name) {
		return fmt.Errorf("invalid bridge name: must contain only alphanumeric, dash, and underscore")
	}

	// Check if bridge already exists
	bridgeName := fmt.Sprintf("joblet-%s", name)
	if nv.interfaceExists(bridgeName) {
		return fmt.Errorf("bridge %s already exists", bridgeName)
	}

	return nil
}

// networksOverlap performs comprehensive overlap detection between two CIDR blocks.
// It checks multiple overlap scenarios including network containment, broadcast address
// conflicts, and ensures complete network isolation to prevent routing conflicts.
func (nv *NetworkValidator) networksOverlap(cidr1, cidr2 string) bool {
	_, net1, _ := net.ParseCIDR(cidr1)
	_, net2, _ := net.ParseCIDR(cidr2)

	// Check if either network contains the other's network address
	if net1.Contains(net2.IP) || net2.Contains(net1.IP) {
		return true
	}

	// Check if either network contains the other's broadcast address
	broadcast1 := getBroadcastAddr(net1)
	broadcast2 := getBroadcastAddr(net2)

	if net1.Contains(broadcast2) || net2.Contains(broadcast1) {
		return true
	}

	// Check if networks share any IP range
	return rangesOverlap(net1, net2)
}

// checkSystemNetworkConflicts checks against system networks
func (nv *NetworkValidator) checkSystemNetworkConflicts(cidr string) error {
	// Common system networks to check
	systemNetworks := []string{
		"127.0.0.0/8",        // Loopback
		"169.254.0.0/16",     // Link-local
		"224.0.0.0/4",        // Multicast
		"255.255.255.255/32", // Broadcast
	}

	_, targetNet, _ := net.ParseCIDR(cidr)

	for _, sysCIDR := range systemNetworks {
		_, sysNet, _ := net.ParseCIDR(sysCIDR)
		if nv.networksOverlap(cidr, sysCIDR) {
			return fmt.Errorf("CIDR conflicts with system network %s", sysCIDR)
		}

		// ensure we're not in reserved space
		if sysNet.Contains(targetNet.IP) {
			return fmt.Errorf("CIDR %s is within reserved network space %s", cidr, sysCIDR)
		}
	}

	// Check against existing interfaces
	return nv.checkExistingInterfaces(cidr)
}

// checkExistingInterfaces checks conflicts with existing network interfaces
func (nv *NetworkValidator) checkExistingInterfaces(cidr string) error {
	cmd := exec.Command("ip", "addr", "show")
	output, err := cmd.Output()
	if err != nil {
		return nil // If we can't check, allow it
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "inet ") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				existingCIDR := fields[1]
				if nv.networksOverlap(cidr, existingCIDR) {
					return fmt.Errorf("CIDR conflicts with existing interface: %s", existingCIDR)
				}
			}
		}
	}

	return nil
}

// Helper functions

func (nv *NetworkValidator) interfaceExists(name string) bool {
	cmd := exec.Command("ip", "link", "show", name)
	err := cmd.Run()
	return err == nil
}

func isValidInterfaceName(name string) bool {
	for _, r := range name {
		if !((r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '-' || r == '_') {
			return false
		}
	}
	return true
}

func getBroadcastAddr(n *net.IPNet) net.IP {
	ip := n.IP.To4()
	if ip == nil {
		return nil
	}

	broadcast := make(net.IP, 4)
	for i := 0; i < 4; i++ {
		broadcast[i] = ip[i] | ^n.Mask[i]
	}
	return broadcast
}

func rangesOverlap(net1, net2 *net.IPNet) bool {
	// Convert to comparable integers
	start1 := ipToUint32(net1.IP)
	end1 := ipToUint32(getBroadcastAddr(net1))

	start2 := ipToUint32(net2.IP)
	end2 := ipToUint32(getBroadcastAddr(net2))

	// Check if ranges overlap
	return !(end1 < start2 || end2 < start1)
}

func ipToUint32(ip net.IP) uint32 {
	ip = ip.To4()
	if ip == nil {
		return 0
	}
	return uint32(ip[0])<<24 + uint32(ip[1])<<16 + uint32(ip[2])<<8 + uint32(ip[3])
}
