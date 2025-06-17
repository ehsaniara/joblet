package network

import (
	"fmt"
	"net"
	"sync"
)

// IPPool manages IP allocation within a network group
type IPPool struct {
	GroupID      string
	Subnet       *net.IPNet
	Gateway      net.IP
	AllocatedIPs map[string]net.IP // jobID -> assigned IP
	AvailableIPs []net.IP          // pool of available IPs
	NextIndex    int               // index of next IP to assign
	mutex        sync.RWMutex
}

// IPPoolManager manages IP pools for all network groups
type IPPoolManager struct {
	pools map[string]*IPPool
	mutex sync.RWMutex
}

// NewIPPoolManager creates a new IP pool manager
func NewIPPoolManager() *IPPoolManager {
	return &IPPoolManager{
		pools: make(map[string]*IPPool),
	}
}

// GetOrCreatePool gets existing pool or creates new one for network group
func (ipm *IPPoolManager) GetOrCreatePool(groupID, subnetCIDR string) (*IPPool, error) {
	ipm.mutex.Lock()
	defer ipm.mutex.Unlock()

	// Return existing pool
	if pool, exists := ipm.pools[groupID]; exists {
		return pool, nil
	}

	// Create new pool
	_, subnet, err := net.ParseCIDR(subnetCIDR)
	if err != nil {
		return nil, fmt.Errorf("invalid subnet CIDR %s: %w", subnetCIDR, err)
	}

	// Calculate gateway (typically .1)
	gateway := subnet.IP.Mask(subnet.Mask)
	gateway[len(gateway)-1] = 1

	pool := &IPPool{
		GroupID:      groupID,
		Subnet:       subnet,
		Gateway:      gateway,
		AllocatedIPs: make(map[string]net.IP),
		NextIndex:    2, // Start from .2 (gateway is .1)
	}

	// Generate available IP pool
	pool.generateAvailableIPs()
	ipm.pools[groupID] = pool

	return pool, nil
}

// generateAvailableIPs generates the pool of available IPs
func (pool *IPPool) generateAvailableIPs() {
	// Get network size
	ones, bits := pool.Subnet.Mask.Size()
	hostBits := bits - ones
	maxHosts := (1 << hostBits) - 2 // -2 for network and broadcast

	// Generate IPs from .2 to .254 (skip .1 for gateway)
	baseIP := pool.Subnet.IP.Mask(pool.Subnet.Mask)

	for i := 2; i <= maxHosts && i <= 254; i++ {
		ip := make(net.IP, len(baseIP))
		copy(ip, baseIP)
		ip[len(ip)-1] = byte(i)
		pool.AvailableIPs = append(pool.AvailableIPs, ip)
	}
}

// AllocateIP allocates next available IP to a job
func (ipm *IPPoolManager) AllocateIP(groupID, jobID, subnetCIDR string) (net.IP, error) {
	pool, err := ipm.GetOrCreatePool(groupID, subnetCIDR)
	if err != nil {
		return nil, err
	}

	pool.mutex.Lock()
	defer pool.mutex.Unlock()

	// Check if job already has IP
	if existingIP, exists := pool.AllocatedIPs[jobID]; exists {
		return existingIP, nil
	}

	// Check if we have available IPs
	if pool.NextIndex >= len(pool.AvailableIPs) {
		return nil, fmt.Errorf("no available IPs in pool for group %s", groupID)
	}

	// Allocate next IP
	assignedIP := pool.AvailableIPs[pool.NextIndex]
	pool.AllocatedIPs[jobID] = assignedIP
	pool.NextIndex++

	return assignedIP, nil
}

// ReleaseIP releases an IP back to the pool
func (ipm *IPPoolManager) ReleaseIP(groupID, jobID string) error {
	ipm.mutex.RLock()
	pool, exists := ipm.pools[groupID]
	ipm.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("pool not found for group %s", groupID)
	}

	pool.mutex.Lock()
	defer pool.mutex.Unlock()

	// Remove from allocated IPs
	if _, exists := pool.AllocatedIPs[jobID]; exists {
		delete(pool.AllocatedIPs, jobID)
		return nil
	}

	return fmt.Errorf("IP not found for job %s in group %s", jobID, groupID)
}

// GetJobIP returns the assigned IP for a job
func (ipm *IPPoolManager) GetJobIP(groupID, jobID string) (net.IP, bool) {
	ipm.mutex.RLock()
	pool, exists := ipm.pools[groupID]
	ipm.mutex.RUnlock()

	if !exists {
		return nil, false
	}

	pool.mutex.RLock()
	defer pool.mutex.RUnlock()

	ip, exists := pool.AllocatedIPs[jobID]
	return ip, exists
}

// GetPoolStats returns statistics about IP pool usage
func (ipm *IPPoolManager) GetPoolStats(groupID string) map[string]interface{} {
	ipm.mutex.RLock()
	pool, exists := ipm.pools[groupID]
	ipm.mutex.RUnlock()

	if !exists {
		return map[string]interface{}{
			"exists": false,
		}
	}

	pool.mutex.RLock()
	defer pool.mutex.RUnlock()

	return map[string]interface{}{
		"exists":        true,
		"group_id":      pool.GroupID,
		"subnet":        pool.Subnet.String(),
		"gateway":       pool.Gateway.String(),
		"total_ips":     len(pool.AvailableIPs),
		"allocated_ips": len(pool.AllocatedIPs),
		"available_ips": len(pool.AvailableIPs) - pool.NextIndex + 2,
		"next_ip":       pool.NextIndex,
	}
}

// ListAllocatedIPs returns all allocated IPs in a group
func (ipm *IPPoolManager) ListAllocatedIPs(groupID string) map[string]string {
	ipm.mutex.RLock()
	pool, exists := ipm.pools[groupID]
	ipm.mutex.RUnlock()

	if !exists {
		return nil
	}

	pool.mutex.RLock()
	defer pool.mutex.RUnlock()

	result := make(map[string]string)
	for jobID, ip := range pool.AllocatedIPs {
		result[jobID] = ip.String()
	}
	return result
}
