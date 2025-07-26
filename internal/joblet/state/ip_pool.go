package state

import (
	"net"
	"sync"
)

// IPPool manages IP address allocation for a network
type IPPool struct {
	subnet    *net.IPNet
	allocated map[uint32]bool
	next      uint32
	mu        sync.Mutex
}

// NewIPPool creates a new IP pool for the given CIDR
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

// Allocate assigns the next available IP address
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

// Release returns an IP address to the pool
func (p *IPPool) Release(ip net.IP) {
	p.mu.Lock()
	defer p.mu.Unlock()

	offset := p.ipToOffset(ip)
	if offset > 0 {
		delete(p.allocated, offset)
	}
}

// GetAvailableCount returns the number of available IPs
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

// IsIPInPool checks if an IP belongs to this pool
func (p *IPPool) IsIPInPool(ip net.IP) bool {
	return p.subnet.Contains(ip)
}

// GetGatewayIP returns the gateway IP for this network (typically .1)
func (p *IPPool) GetGatewayIP() net.IP {
	return p.offsetToIP(1)
}

// GetNetworkAddress returns the network address
func (p *IPPool) GetNetworkAddress() net.IP {
	return p.subnet.IP
}

// GetBroadcastAddress returns the broadcast address
func (p *IPPool) GetBroadcastAddress() net.IP {
	ones, bits := p.subnet.Mask.Size()
	subnetSize := uint32(1 << (bits - ones))
	return p.offsetToIP(subnetSize - 1)
}

// Helper methods

// offsetToIP converts an offset to an IP address
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

// ipToOffset converts an IP to its offset within the subnet
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

// ipToUint32 converts a 4-byte IP to uint32
func ipToUint32(ip net.IP) uint32 {
	if len(ip) != 4 {
		return 0
	}
	return uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3])
}

// uint32ToIP converts uint32 to IP
func uint32ToIP(n uint32, ip net.IP) {
	ip[0] = byte(n >> 24)
	ip[1] = byte(n >> 16)
	ip[2] = byte(n >> 8)
	ip[3] = byte(n)
}

// GetSubnetMask returns the subnet mask
func (p *IPPool) GetSubnetMask() net.IPMask {
	return p.subnet.Mask
}

// GetCIDR returns the CIDR notation of the subnet
func (p *IPPool) GetCIDR() string {
	return p.subnet.String()
}

// Reset clears all allocations (useful for testing)
func (p *IPPool) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.allocated = make(map[uint32]bool)
	p.next = 2
}
