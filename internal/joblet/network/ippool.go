package network

import (
	"fmt"
	"net"
	"sync"
)

// IPPoolManager implements the IPPool interface
type IPPoolManager struct {
	pools map[string]*networkPool
	mutex sync.RWMutex
}

// networkPool manages IP allocation for a specific network
type networkPool struct {
	cidr      *net.IPNet
	allocated map[string]bool // IP string -> allocated
	mutex     sync.RWMutex
}

// NewIPPoolManager creates a new IP pool manager
func NewIPPoolManager() *IPPoolManager {
	return &IPPoolManager{
		pools: make(map[string]*networkPool),
	}
}

// InitializePool initializes an IP pool for a network
func (ip *IPPoolManager) InitializePool(networkName, cidr string) error {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf("invalid CIDR %s: %w", cidr, err)
	}

	ip.mutex.Lock()
	defer ip.mutex.Unlock()

	ip.pools[networkName] = &networkPool{
		cidr:      ipNet,
		allocated: make(map[string]bool),
	}

	return nil
}

// AllocateIP allocates an IP from the pool
func (ip *IPPoolManager) AllocateIP(networkName string) (net.IP, error) {
	ip.mutex.RLock()
	pool, exists := ip.pools[networkName]
	ip.mutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no IP pool for network %s", networkName)
	}

	pool.mutex.Lock()
	defer pool.mutex.Unlock()

	// Find next available IP
	for addr := pool.cidr.IP.Mask(pool.cidr.Mask); pool.cidr.Contains(addr); ip.incrementIP(addr) {
		addrStr := addr.String()

		// Skip network and broadcast addresses
		if addr.Equal(pool.cidr.IP) || ip.isBroadcast(addr, pool.cidr) {
			continue
		}

		if !pool.allocated[addrStr] {
			pool.allocated[addrStr] = true
			// Return a copy of the IP
			allocatedIP := make(net.IP, len(addr))
			copy(allocatedIP, addr)
			return allocatedIP, nil
		}
	}

	return nil, fmt.Errorf("no available IPs in network %s", networkName)
}

// ReleaseIP releases an IP back to the pool
func (ip *IPPoolManager) ReleaseIP(networkName string, addr net.IP) error {
	ip.mutex.RLock()
	pool, exists := ip.pools[networkName]
	ip.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("no IP pool for network %s", networkName)
	}

	pool.mutex.Lock()
	defer pool.mutex.Unlock()

	addrStr := addr.String()
	if !pool.allocated[addrStr] {
		return fmt.Errorf("IP %s is not allocated in network %s", addrStr, networkName)
	}

	delete(pool.allocated, addrStr)
	return nil
}

// IsIPAvailable checks if an IP is available
func (ip *IPPoolManager) IsIPAvailable(networkName string, addr net.IP) bool {
	ip.mutex.RLock()
	pool, exists := ip.pools[networkName]
	ip.mutex.RUnlock()

	if !exists {
		return false
	}

	pool.mutex.RLock()
	defer pool.mutex.RUnlock()

	if !pool.cidr.Contains(addr) {
		return false
	}

	// Skip network and broadcast addresses
	if addr.Equal(pool.cidr.IP) || ip.isBroadcast(addr, pool.cidr) {
		return false
	}

	return !pool.allocated[addr.String()]
}

// GetAvailableIPs returns all available IPs
func (ip *IPPoolManager) GetAvailableIPs(networkName string) ([]net.IP, error) {
	ip.mutex.RLock()
	pool, exists := ip.pools[networkName]
	ip.mutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no IP pool for network %s", networkName)
	}

	pool.mutex.RLock()
	defer pool.mutex.RUnlock()

	var available []net.IP
	for addr := pool.cidr.IP.Mask(pool.cidr.Mask); pool.cidr.Contains(addr); ip.incrementIP(addr) {
		// Skip network and broadcast addresses
		if addr.Equal(pool.cidr.IP) || ip.isBroadcast(addr, pool.cidr) {
			continue
		}

		addrStr := addr.String()
		if !pool.allocated[addrStr] {
			// Add a copy of the IP
			availableIP := make(net.IP, len(addr))
			copy(availableIP, addr)
			available = append(available, availableIP)
		}
	}

	return available, nil
}

// GetAllocatedIPs returns all allocated IPs
func (ip *IPPoolManager) GetAllocatedIPs(networkName string) ([]net.IP, error) {
	ip.mutex.RLock()
	pool, exists := ip.pools[networkName]
	ip.mutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no IP pool for network %s", networkName)
	}

	pool.mutex.RLock()
	defer pool.mutex.RUnlock()

	var allocated []net.IP
	for addrStr := range pool.allocated {
		addr := net.ParseIP(addrStr)
		if addr != nil {
			allocated = append(allocated, addr)
		}
	}

	return allocated, nil
}

// incrementIP increments an IP address
func (ip *IPPoolManager) incrementIP(addr net.IP) {
	for j := len(addr) - 1; j >= 0; j-- {
		addr[j]++
		if addr[j] > 0 {
			break
		}
	}
}

// isBroadcast checks if IP is the broadcast address for the network
func (ip *IPPoolManager) isBroadcast(addr net.IP, ipNet *net.IPNet) bool {
	// Convert to 4-byte representation for IPv4
	if addr4 := addr.To4(); addr4 != nil {
		addr = addr4
	}

	// Create broadcast address
	broadcast := make(net.IP, len(addr))
	copy(broadcast, addr)

	// Set host bits to 1
	for i := 0; i < len(addr); i++ {
		broadcast[i] = addr[i] | ^ipNet.Mask[i]
	}

	return addr.Equal(broadcast)
}
