package adapters

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"joblet/pkg/logger"
)

// Local interface definitions to avoid import cycles with pkg/store
type networkStore[K comparable, V any] interface {
	Create(ctx context.Context, key K, value V) error
	Get(ctx context.Context, key K) (V, bool, error)
	List(ctx context.Context) ([]V, error)
	Delete(ctx context.Context, key K) error
	Close() error
}

// Note: Error checking helpers are defined in job_store_adapter.go to avoid redeclaration

// networkStoreAdapter implements NetworkStoreAdapter using generic store backends.
// It manages network configurations and job-to-network assignments with IP allocation.
type networkStoreAdapter struct {
	// Generic storage backends
	networkStore    networkStore[string, *NetworkConfig]
	allocationStore networkStore[string, *JobNetworkAllocation]

	// IP address management per network
	ipPools    map[string]*ipPool
	poolsMutex sync.RWMutex

	logger     *logger.Logger
	closed     bool
	closeMutex sync.RWMutex
}

// ipPool manages IP allocation for a specific network
type ipPool struct {
	cidr      *net.IPNet
	allocated map[string]bool // IP -> allocated
	available []string        // Available IPs
	mutex     sync.RWMutex
}

// NewNetworkStoreAdapter creates a new network store adapter with the specified backends.
func NewNetworkStoreAdapter(
	networkStore networkStore[string, *NetworkConfig],
	allocationStore networkStore[string, *JobNetworkAllocation],
	logger *logger.Logger,
) NetworkStoreAdapter {
	if logger == nil {
		logger = logger.WithField("component", "network-store-adapter")
	}

	return &networkStoreAdapter{
		networkStore:    networkStore,
		allocationStore: allocationStore,
		ipPools:         make(map[string]*ipPool),
		logger:          logger,
	}
}

// CreateNetwork creates a new network configuration.
func (a *networkStoreAdapter) CreateNetwork(config *NetworkConfig) error {

	a.closeMutex.RLock()
	if a.closed {
		a.closeMutex.RUnlock()
		return fmt.Errorf("network store adapter is closed")
	}
	a.closeMutex.RUnlock()

	if config == nil {
		return fmt.Errorf("network config cannot be nil")
	}

	if config.Name == "" {
		return fmt.Errorf("network name cannot be empty")
	}

	// Validate network configuration
	if err := a.validateNetworkConfig(config); err != nil {
		return fmt.Errorf("network config validation failed: %w", err)
	}

	// Set timestamps
	now := time.Now().Unix()
	config.CreatedAt = now
	config.UpdatedAt = now

	// Store the network
	ctx := context.Background()
	if err := a.networkStore.Create(ctx, config.Name, config); err != nil {
		if IsConflictError(err) {
			return fmt.Errorf("network already exists: %s", config.Name)
		}
		a.logger.Error("failed to create network in store", "networkName", config.Name, "error", err)
		return fmt.Errorf("failed to create network: %w", err)
	}

	// Initialize IP pool if CIDR is provided
	if config.CIDR != "" {
		if err := a.initializeIPPool(config.Name, config.CIDR); err != nil {
			// Clean up the network if IP pool initialization fails
			_ = a.networkStore.Delete(ctx, config.Name)
			return fmt.Errorf("failed to initialize IP pool: %w", err)
		}
	}

	a.logger.Info("network created successfully",
		"networkName", config.Name,
		"type", config.Type,
		"cidr", config.CIDR)

	return nil
}

// GetNetwork retrieves a network configuration by name.
func (a *networkStoreAdapter) GetNetwork(name string) (*NetworkConfig, bool) {
	a.closeMutex.RLock()
	if a.closed {
		a.closeMutex.RUnlock()
		return nil, false
	}
	a.closeMutex.RUnlock()

	if name == "" {
		a.logger.Debug("empty network name provided")
		return nil, false
	}

	ctx := context.Background()
	config, exists, err := a.networkStore.Get(ctx, name)
	if err != nil {
		a.logger.Error("failed to get network from store", "networkName", name, "error", err)
		return nil, false
	}

	if exists {
		a.logger.Debug("network retrieved successfully", "networkName", name, "type", config.Type)
		// Return a copy to prevent external modification
		configCopy := *config
		if config.DNS != nil {
			configCopy.DNS = make([]string, len(config.DNS))
			copy(configCopy.DNS, config.DNS)
		}
		if config.Metadata != nil {
			configCopy.Metadata = make(map[string]string)
			for k, v := range config.Metadata {
				configCopy.Metadata[k] = v
			}
		}
		return &configCopy, true
	}

	a.logger.Debug("network not found", "networkName", name)
	return nil, false
}

// ListNetworks returns all network configurations.
func (a *networkStoreAdapter) ListNetworks() []*NetworkConfig {
	a.closeMutex.RLock()
	if a.closed {
		a.closeMutex.RUnlock()
		return []*NetworkConfig{}
	}
	a.closeMutex.RUnlock()

	ctx := context.Background()
	networks, err := a.networkStore.List(ctx)
	if err != nil {
		a.logger.Error("failed to list networks from store", "error", err)
		return []*NetworkConfig{}
	}

	// Create copies to prevent external modification
	result := make([]*NetworkConfig, len(networks))
	for i, network := range networks {
		configCopy := *network
		if network.DNS != nil {
			configCopy.DNS = make([]string, len(network.DNS))
			copy(configCopy.DNS, network.DNS)
		}
		if network.Metadata != nil {
			configCopy.Metadata = make(map[string]string)
			for k, v := range network.Metadata {
				configCopy.Metadata[k] = v
			}
		}
		result[i] = &configCopy
	}

	a.logger.Debug("networks listed successfully", "count", len(result))
	return result
}

// RemoveNetwork removes a network configuration.
func (a *networkStoreAdapter) RemoveNetwork(name string) error {

	a.closeMutex.RLock()
	if a.closed {
		a.closeMutex.RUnlock()
		return fmt.Errorf("network store adapter is closed")
	}
	a.closeMutex.RUnlock()

	if name == "" {
		return fmt.Errorf("network name cannot be empty")
	}

	// Check if any jobs are still assigned to this network
	jobsInNetwork := a.ListJobsInNetwork(name)
	if len(jobsInNetwork) > 0 {
		return fmt.Errorf("network is still in use by %d job(s)", len(jobsInNetwork))
	}

	// Get network for statistics before deletion
	ctx := context.Background()
	network, exists, err := a.networkStore.Get(ctx, name)
	if err != nil {
		a.logger.Error("failed to get network for deletion", "networkName", name, "error", err)
		return fmt.Errorf("failed to check network: %w", err)
	}

	if !exists {
		return fmt.Errorf("network not found: %s", name)
	}

	// Remove from store
	if err := a.networkStore.Delete(ctx, name); err != nil {
		if err.Error() == "key not found" {
			return fmt.Errorf("network not found: %s", name)
		}
		a.logger.Error("failed to remove network from store", "networkName", name, "error", err)
		return fmt.Errorf("failed to remove network: %w", err)
	}

	// Remove IP pool
	a.poolsMutex.Lock()
	delete(a.ipPools, name)
	a.poolsMutex.Unlock()

	a.logger.Info("network removed successfully",
		"networkName", name,
		"type", network.Type)

	return nil
}

// AssignJobToNetwork assigns a job to a network with IP allocation.
func (a *networkStoreAdapter) AssignJobToNetwork(jobID, networkName string, allocation *JobNetworkAllocation) error {

	a.closeMutex.RLock()
	if a.closed {
		a.closeMutex.RUnlock()
		return fmt.Errorf("network store adapter is closed")
	}
	a.closeMutex.RUnlock()

	if jobID == "" {
		return fmt.Errorf("job ID cannot be empty")
	}

	if networkName == "" {
		return fmt.Errorf("network name cannot be empty")
	}

	if allocation == nil {
		return fmt.Errorf("job network allocation cannot be nil")
	}

	// Check if network exists
	_, exists := a.GetNetwork(networkName)
	if !exists {
		return fmt.Errorf("network not found: %s", networkName)
	}

	// Set allocation timestamp
	allocation.AssignedAt = time.Now().Unix()
	allocation.JobID = jobID
	allocation.NetworkName = networkName

	// Store the allocation
	ctx := context.Background()
	if err := a.allocationStore.Create(ctx, jobID, allocation); err != nil {
		if IsConflictError(err) {
			return fmt.Errorf("job already assigned to a network: %s", jobID)
		}
		a.logger.Error("failed to create job allocation in store", "jobId", jobID, "networkName", networkName, "error", err)
		return fmt.Errorf("failed to assign job to network: %w", err)
	}

	a.logger.Info("job assigned to network successfully",
		"jobId", jobID,
		"networkName", networkName,
		"ipAddress", allocation.IPAddress)

	return nil
}

// GetJobNetworkAllocation retrieves the network allocation for a job.
func (a *networkStoreAdapter) GetJobNetworkAllocation(jobID string) (*JobNetworkAllocation, bool) {
	a.closeMutex.RLock()
	if a.closed {
		a.closeMutex.RUnlock()
		return nil, false
	}
	a.closeMutex.RUnlock()

	if jobID == "" {
		a.logger.Debug("empty job ID provided")
		return nil, false
	}

	ctx := context.Background()
	allocation, exists, err := a.allocationStore.Get(ctx, jobID)
	if err != nil {
		a.logger.Error("failed to get job allocation from store", "jobId", jobID, "error", err)
		return nil, false
	}

	if exists {
		a.logger.Debug("job allocation retrieved successfully", "jobId", jobID, "networkName", allocation.NetworkName)
		// Return a copy to prevent external modification
		allocationCopy := *allocation
		if allocation.Metadata != nil {
			allocationCopy.Metadata = make(map[string]string)
			for k, v := range allocation.Metadata {
				allocationCopy.Metadata[k] = v
			}
		}
		return &allocationCopy, true
	}

	a.logger.Debug("job allocation not found", "jobId", jobID)
	return nil, false
}

// RemoveJobFromNetwork removes a job's network assignment.
func (a *networkStoreAdapter) RemoveJobFromNetwork(jobID string) error {
	a.closeMutex.RLock()
	if a.closed {
		a.closeMutex.RUnlock()
		return fmt.Errorf("network store adapter is closed")
	}
	a.closeMutex.RUnlock()

	if jobID == "" {
		return fmt.Errorf("job ID cannot be empty")
	}

	// Get allocation before deletion for IP release
	allocation, exists := a.GetJobNetworkAllocation(jobID)
	if !exists {
		return fmt.Errorf("job not assigned to any network: %s", jobID)
	}

	// Release IP if allocated
	if allocation.IPAddress != "" {
		if err := a.ReleaseIP(allocation.NetworkName, allocation.IPAddress); err != nil {
			a.logger.Warn("failed to release IP address", "jobId", jobID, "ip", allocation.IPAddress, "error", err)
			// Continue with removal even if IP release fails
		}
	}

	// Remove from store
	ctx := context.Background()
	if err := a.allocationStore.Delete(ctx, jobID); err != nil {
		if err.Error() == "key not found" {
			return fmt.Errorf("job not assigned to any network: %s", jobID)
		}
		a.logger.Error("failed to remove job allocation from store", "jobId", jobID, "error", err)
		return fmt.Errorf("failed to remove job from network: %w", err)
	}

	a.logger.Info("job removed from network successfully",
		"jobId", jobID,
		"networkName", allocation.NetworkName)

	return nil
}

// ListJobsInNetwork returns all jobs assigned to a specific network.
func (a *networkStoreAdapter) ListJobsInNetwork(networkName string) []*JobNetworkAllocation {
	a.closeMutex.RLock()
	if a.closed {
		a.closeMutex.RUnlock()
		return []*JobNetworkAllocation{}
	}
	a.closeMutex.RUnlock()

	ctx := context.Background()
	allocations, err := a.allocationStore.List(ctx)
	if err != nil {
		a.logger.Error("failed to list job allocations from store", "error", err)
		return []*JobNetworkAllocation{}
	}

	// Filter by network name and create copies
	var result []*JobNetworkAllocation
	for _, allocation := range allocations {
		if allocation.NetworkName == networkName {
			allocationCopy := *allocation
			if allocation.Metadata != nil {
				allocationCopy.Metadata = make(map[string]string)
				for k, v := range allocation.Metadata {
					allocationCopy.Metadata[k] = v
				}
			}
			result = append(result, &allocationCopy)
		}
	}

	a.logger.Debug("jobs in network listed successfully", "networkName", networkName, "count", len(result))
	return result
}

// AllocateIP allocates an IP address from the network's pool.
func (a *networkStoreAdapter) AllocateIP(networkName string) (string, error) {
	a.closeMutex.RLock()
	if a.closed {
		a.closeMutex.RUnlock()
		return "", fmt.Errorf("network store adapter is closed")
	}
	a.closeMutex.RUnlock()

	if networkName == "" {
		return "", fmt.Errorf("network name cannot be empty")
	}

	a.poolsMutex.RLock()
	pool, exists := a.ipPools[networkName]
	a.poolsMutex.RUnlock()

	if !exists {
		return "", fmt.Errorf("no IP pool found for network: %s", networkName)
	}

	pool.mutex.Lock()
	defer pool.mutex.Unlock()

	if len(pool.available) == 0 {
		return "", fmt.Errorf("no available IP addresses in network: %s", networkName)
	}

	// Get first available IP
	ip := pool.available[0]
	pool.available = pool.available[1:]
	pool.allocated[ip] = true

	a.logger.Debug("IP allocated successfully", "networkName", networkName, "ip", ip)
	return ip, nil
}

// ReleaseIP releases an IP address back to the network's pool.
func (a *networkStoreAdapter) ReleaseIP(networkName, ip string) error {
	a.closeMutex.RLock()
	if a.closed {
		a.closeMutex.RUnlock()
		return fmt.Errorf("network store adapter is closed")
	}
	a.closeMutex.RUnlock()

	if networkName == "" {
		return fmt.Errorf("network name cannot be empty")
	}

	if ip == "" {
		return fmt.Errorf("IP address cannot be empty")
	}

	a.poolsMutex.RLock()
	pool, exists := a.ipPools[networkName]
	a.poolsMutex.RUnlock()

	if !exists {
		return fmt.Errorf("no IP pool found for network: %s", networkName)
	}

	pool.mutex.Lock()
	defer pool.mutex.Unlock()

	if !pool.allocated[ip] {
		return fmt.Errorf("IP address not allocated: %s", ip)
	}

	// Release IP
	delete(pool.allocated, ip)
	pool.available = append(pool.available, ip)

	a.logger.Debug("IP released successfully", "networkName", networkName, "ip", ip)
	return nil
}

// Close gracefully shuts down the adapter and releases resources.
func (a *networkStoreAdapter) Close() error {
	a.closeMutex.Lock()
	defer a.closeMutex.Unlock()

	if a.closed {
		return nil
	}
	a.closed = true

	// Clear IP pools
	a.poolsMutex.Lock()
	a.ipPools = make(map[string]*ipPool)
	a.poolsMutex.Unlock()

	// Close backend stores
	if err := a.networkStore.Close(); err != nil {
		a.logger.Error("failed to close network store", "error", err)
	}

	if err := a.allocationStore.Close(); err != nil {
		a.logger.Error("failed to close allocation store", "error", err)
	}

	a.logger.Debug("network store adapter closed successfully")
	return nil
}

// Helper methods

func (a *networkStoreAdapter) validateNetworkConfig(config *NetworkConfig) error {
	if config.Name == "" {
		return fmt.Errorf("network name is required")
	}

	if config.Type == "" {
		return fmt.Errorf("network type is required")
	}

	// Validate network type
	validTypes := []string{"bridge", "host", "none", "custom"}
	isValidType := false
	for _, validType := range validTypes {
		if config.Type == validType {
			isValidType = true
			break
		}
	}
	if !isValidType {
		return fmt.Errorf("invalid network type: %s (must be one of: %v)", config.Type, validTypes)
	}

	// Validate CIDR if provided
	if config.CIDR != "" {
		_, _, err := net.ParseCIDR(config.CIDR)
		if err != nil {
			return fmt.Errorf("invalid CIDR: %s", config.CIDR)
		}
	}

	// Validate gateway if provided
	if config.Gateway != "" {
		if net.ParseIP(config.Gateway) == nil {
			return fmt.Errorf("invalid gateway IP: %s", config.Gateway)
		}
	}

	// Validate DNS servers if provided
	for _, dns := range config.DNS {
		if net.ParseIP(dns) == nil {
			return fmt.Errorf("invalid DNS server IP: %s", dns)
		}
	}

	return nil
}

func (a *networkStoreAdapter) initializeIPPool(networkName, cidr string) error {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf("invalid CIDR: %s", cidr)
	}

	pool := &ipPool{
		cidr:      ipNet,
		allocated: make(map[string]bool),
		available: make([]string, 0),
	}

	// Generate available IP addresses
	ip := ipNet.IP
	for ipNet.Contains(ip) {
		// Skip network and broadcast addresses
		if !ip.Equal(ipNet.IP) && !ip.Equal(a.getBroadcastAddress(ipNet)) {
			pool.available = append(pool.available, ip.String())
		}
		ip = a.nextIP(ip)
	}

	a.poolsMutex.Lock()
	a.ipPools[networkName] = pool
	a.poolsMutex.Unlock()

	a.logger.Debug("IP pool initialized", "networkName", networkName, "cidr", cidr, "availableIPs", len(pool.available))
	return nil
}

func (a *networkStoreAdapter) nextIP(ip net.IP) net.IP {
	next := make(net.IP, len(ip))
	copy(next, ip)
	for j := len(next) - 1; j >= 0; j-- {
		next[j]++
		if next[j] > 0 {
			break
		}
	}
	return next
}

func (a *networkStoreAdapter) getBroadcastAddress(ipNet *net.IPNet) net.IP {
	ip := make(net.IP, len(ipNet.IP))
	for i := range ip {
		ip[i] = ipNet.IP[i] | ^ipNet.Mask[i]
	}
	return ip
}
