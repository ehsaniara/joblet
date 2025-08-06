package state

import (
	"net"
	"sync"
)

// IPPool manages IP address allocation for a network with thread-safe operations.
// It maintains a pool of available IP addresses within a CIDR subnet and tracks
// allocated addresses to prevent conflicts. Reserves .1 for gateway usage.
type IPPool struct {
	subnet    *net.IPNet
	allocated map[uint32]bool
	next      uint32
	mu        sync.Mutex
}

// NewIPPool creates a new IP pool for the given CIDR subnet.
// Parses the CIDR notation and initializes the allocation tracking map.
// Panics if the CIDR is invalid (should be validated before calling).
func NewIPPool(cidr string) *IPPool {
	_, subnet, err := net.ParseCIDR(cidr)
	if err != nil {
		// This should never happen as CIDR is validated before
		panic("invalid CIDR in IPPool: " + cidr)
	}

	return &IPPool{
		subnet:    subnet,
		allocated: make(map[uint32]bool),
		next:      1, // Start from .1 (gateway is usually .1, so we'll skip it)
	}
}

// Allocate assigns the next available IP address from the pool.
// Thread-safe operation that finds the next unallocated IP starting from .2
// (reserving .1 for gateway). Returns nil if pool is exhausted.
func (p *IPPool) Allocate() net.IP {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Calculate the size of the subnet
	ones, bits := p.subnet.Mask.Size()
	subnetSize := uint32(1 << (bits - ones))

	// Start from .2 (reserve .1 for gateway)
	startOffset := uint32(2)

	// Don't allocate network or broadcast addresses
	maxOffset := subnetSize - 2

	// Find next available IP
	attempts := 0
	for attempts < int(maxOffset) {
		// Skip gateway IP (.1)
		if p.next == 1 {
			p.next = 2
		}

		if p.next > maxOffset {
			p.next = startOffset
		}

		if !p.allocated[p.next] {
			p.allocated[p.next] = true
			ip := p.offsetToIP(p.next)
			p.next++
			return ip
		}

		p.next++
		attempts++
	}

	// Pool exhausted
	return nil
}

// Release returns an IP address to the pool for reuse.
// Thread-safe operation that marks the IP as available for future allocation.
// Ignores IPs that don't belong to this pool's subnet.
func (p *IPPool) Release(ip net.IP) {
	p.mu.Lock()
	defer p.mu.Unlock()

	offset := p.ipToOffset(ip)
	if offset > 0 {
		delete(p.allocated, offset)
	}
}

// GetAvailableCount returns the number of available IP addresses in the pool.
// Thread-safe operation that calculates available IPs by subtracting allocated,
// network, broadcast, and gateway addresses from the total subnet size.
func (p *IPPool) GetAvailableCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	ones, bits := p.subnet.Mask.Size()
	subnetSize := uint32(1 << (bits - ones))

	// Subtract network address, broadcast, gateway, and allocated
	maxUsable := subnetSize - 3
	allocated := len(p.allocated)

	available := int(maxUsable) - allocated
	if available < 0 {
		return 0
	}
	return available
}

// IsIPInPool checks if an IP address belongs to this pool's subnet.
// Returns true if the IP is within the CIDR range, false otherwise.
func (p *IPPool) IsIPInPool(ip net.IP) bool {
	return p.subnet.Contains(ip)
}

// GetGatewayIP returns the gateway IP address for this network.
// Always returns the first usable address (.1) which is reserved for gateway use.
func (p *IPPool) GetGatewayIP() net.IP {
	return p.offsetToIP(1)
}

// GetNetworkAddress returns the network address (first address in CIDR).
// This is the base address of the subnet and cannot be assigned to hosts.
func (p *IPPool) GetNetworkAddress() net.IP {
	return p.subnet.IP
}

// GetBroadcastAddress returns the broadcast address (last address in CIDR).
// This is the highest address in the subnet and cannot be assigned to hosts.
func (p *IPPool) GetBroadcastAddress() net.IP {
	ones, bits := p.subnet.Mask.Size()
	subnetSize := uint32(1 << (bits - ones))
	return p.offsetToIP(subnetSize - 1)
}

// Helper methods for IP address calculations

// offsetToIP converts a numeric offset to an IP address within the subnet.
// Takes the base network IP and adds the offset to generate the target IP.
// Only supports IPv4 addresses currently.
func (p *IPPool) offsetToIP(offset uint32) net.IP {
	// Convert base IP to 4-byte representation
	baseIP := p.subnet.IP.To4()
	if baseIP == nil {
		// IPv6 not supported yet
		return nil
	}

	// Calculate IP by adding offset
	ip := make(net.IP, 4)
	ipInt := ipToUint32(baseIP) + offset
	uint32ToIP(ipInt, ip)

	return ip
}

// ipToOffset converts an IP address to its numeric offset within the subnet.
// Returns 0 if the IP doesn't belong to this pool's subnet.
// Only supports IPv4 addresses currently.
func (p *IPPool) ipToOffset(ip net.IP) uint32 {
	if !p.subnet.Contains(ip) {
		return 0
	}

	ip4 := ip.To4()
	if ip4 == nil {
		return 0
	}

	baseInt := ipToUint32(p.subnet.IP.To4())
	ipInt := ipToUint32(ip4)

	return ipInt - baseInt
}

// ipToUint32 converts a 4-byte IPv4 address to its uint32 representation.
// Returns 0 for invalid or non-IPv4 addresses.
func ipToUint32(ip net.IP) uint32 {
	if len(ip) != 4 {
		return 0
	}
	return uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3])
}

// uint32ToIP converts a uint32 value to IPv4 address bytes.
// Modifies the provided IP slice in-place with the converted address.
func uint32ToIP(n uint32, ip net.IP) {
	ip[0] = byte(n >> 24)
	ip[1] = byte(n >> 16)
	ip[2] = byte(n >> 8)
	ip[3] = byte(n)
}

// GetSubnetMask returns the subnet mask for this IP pool.
// Used for network calculations and routing configuration.
func (p *IPPool) GetSubnetMask() net.IPMask {
	return p.subnet.Mask
}

// GetCIDR returns the CIDR notation string of the subnet.
// Useful for configuration display and network setup.
func (p *IPPool) GetCIDR() string {
	return p.subnet.String()
}

// Reset clears all IP allocations and resets the allocation counter.
// Thread-safe operation primarily used for testing and cleanup scenarios.
// Reinitializes the pool to its empty state.
func (p *IPPool) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.allocated = make(map[uint32]bool)
	p.next = 2
}
