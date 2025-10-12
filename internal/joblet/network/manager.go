package network

import (
	"context"
	"fmt"
	"sync"

	"github.com/ehsaniara/joblet/pkg/logger"
)

// NetworkManager implements the Manager interface
type NetworkManager struct {
	validator Validator
	ipPool    IPPool
	setup     Setup
	dns       DNS
	monitor   Monitor
	logger    *logger.Logger

	// State management
	networks    map[string]*NetworkConfig
	allocations map[string]*JobAllocation // jobID -> allocation
	mutex       sync.RWMutex
}

// NewNetworkManager creates a new network manager
func NewNetworkManager(validator Validator, ipPool IPPool, setup Setup, dns DNS) *NetworkManager {
	return &NetworkManager{
		validator:   validator,
		ipPool:      ipPool,
		setup:       setup,
		dns:         dns,
		monitor:     nil, // Optional monitoring
		logger:      logger.WithField("component", "network-manager"),
		networks:    make(map[string]*NetworkConfig),
		allocations: make(map[string]*JobAllocation),
	}
}

// SetMonitor sets the optional network monitor
func (nm *NetworkManager) SetMonitor(monitor Monitor) {
	nm.monitor = monitor
}

// CreateNetwork creates a new network
func (nm *NetworkManager) CreateNetwork(name string, config *NetworkConfig) error {
	if err := nm.validator.ValidateNetworkName(name); err != nil {
		return fmt.Errorf("invalid network name: %w", err)
	}

	if err := nm.ValidateNetworkConfig(config); err != nil {
		return fmt.Errorf("invalid network config: %w", err)
	}

	nm.mutex.Lock()
	defer nm.mutex.Unlock()

	if _, exists := nm.networks[name]; exists {
		return fmt.Errorf("network %s already exists", name)
	}

	// Create bridge infrastructure
	if err := nm.setup.CreateBridge(config.Bridge, config.CIDR); err != nil {
		return fmt.Errorf("failed to create bridge: %w", err)
	}

	nm.networks[name] = config
	return nil
}

// DestroyNetwork removes a network
func (nm *NetworkManager) DestroyNetwork(name string) error {
	nm.mutex.Lock()
	defer nm.mutex.Unlock()

	config, exists := nm.networks[name]
	if !exists {
		return fmt.Errorf("network %s does not exist", name)
	}

	// Check if any jobs are still using this network
	for _, allocation := range nm.allocations {
		if allocation.Network == name {
			return fmt.Errorf("network %s is still in use by job %s", name, allocation.JobID)
		}
	}

	// Clean up bridge infrastructure
	if err := nm.setup.DeleteBridge(config.Bridge); err != nil {
		return fmt.Errorf("failed to delete bridge: %w", err)
	}

	delete(nm.networks, name)
	return nil
}

// ListNetworks returns all networks
func (nm *NetworkManager) ListNetworks() ([]NetworkInfo, error) {
	nm.mutex.RLock()
	defer nm.mutex.RUnlock()

	networks := make([]NetworkInfo, 0, len(nm.networks))
	for name, config := range nm.networks {
		jobCount := 0
		for _, allocation := range nm.allocations {
			if allocation.Network == name {
				jobCount++
			}
		}

		networks = append(networks, NetworkInfo{
			Name:     name,
			CIDR:     config.CIDR,
			Bridge:   config.Bridge,
			JobCount: jobCount,
		})
	}

	return networks, nil
}

// NetworkExists checks if a network exists
func (nm *NetworkManager) NetworkExists(name string) bool {
	nm.mutex.RLock()
	defer nm.mutex.RUnlock()

	_, exists := nm.networks[name]
	return exists
}

// AllocateIP allocates an IP for a job
func (nm *NetworkManager) AllocateIP(networkName, jobID string) (*JobAllocation, error) {
	nm.mutex.Lock()
	defer nm.mutex.Unlock()

	// Check if job already has allocation
	if existing, exists := nm.allocations[jobID]; exists {
		return existing, nil
	}

	// Check if network exists
	if !nm.NetworkExists(networkName) {
		return nil, fmt.Errorf("network %s does not exist", networkName)
	}

	// Allocate IP from pool
	ip, err := nm.ipPool.AllocateIP(networkName)
	if err != nil {
		return nil, fmt.Errorf("failed to allocate IP: %w", err)
	}

	allocation := &JobAllocation{
		JobID:    jobID,
		Network:  networkName,
		IP:       ip,
		Hostname: fmt.Sprintf("job-%s", jobID[:8]),
		VethHost: fmt.Sprintf("veth-h-%s", jobID[:8]),
		VethPeer: fmt.Sprintf("veth-p-%s", jobID[:8]),
	}

	nm.allocations[jobID] = allocation
	return allocation, nil
}

// ReleaseIP releases IP allocation for a job
func (nm *NetworkManager) ReleaseIP(jobID string) error {
	nm.mutex.Lock()
	defer nm.mutex.Unlock()

	allocation, exists := nm.allocations[jobID]
	if !exists {
		return fmt.Errorf("no allocation found for job %s", jobID)
	}

	// Release IP back to pool
	if err := nm.ipPool.ReleaseIP(allocation.Network, allocation.IP); err != nil {
		return fmt.Errorf("failed to release IP: %w", err)
	}

	delete(nm.allocations, jobID)
	return nil
}

// GetAllocation returns allocation for a job
func (nm *NetworkManager) GetAllocation(jobID string) (*JobAllocation, error) {
	nm.mutex.RLock()
	defer nm.mutex.RUnlock()

	allocation, exists := nm.allocations[jobID]
	if !exists {
		return nil, fmt.Errorf("no allocation found for job %s", jobID)
	}

	return allocation, nil
}

// ListAllocations returns all allocations for a network
func (nm *NetworkManager) ListAllocations(networkName string) ([]JobAllocation, error) {
	nm.mutex.RLock()
	defer nm.mutex.RUnlock()

	var allocations []JobAllocation
	for _, allocation := range nm.allocations {
		if allocation.Network == networkName {
			allocations = append(allocations, *allocation)
		}
	}

	return allocations, nil
}

// SetupJobNetworking sets up networking for a job
func (nm *NetworkManager) SetupJobNetworking(jobID, networkName string) (*JobAllocation, error) {
	// Validate job networking
	if err := nm.validator.ValidateJobNetworking(jobID, networkName); err != nil {
		return nil, fmt.Errorf("job networking validation failed: %w", err)
	}

	// Allocate IP
	allocation, err := nm.AllocateIP(networkName, jobID)
	if err != nil {
		return nil, fmt.Errorf("IP allocation failed: %w", err)
	}

	// Setup network namespace
	nm.logger.Debug("calling SetupNamespace for job", "jobID", jobID, "allocation", allocation != nil)
	if err := nm.setup.SetupNamespace(jobID, allocation); err != nil {
		nm.logger.Error("SetupNamespace failed", "jobID", jobID, "error", err)
		// Cleanup on failure (ignore errors)
		_ = nm.ReleaseIP(jobID)
		return nil, fmt.Errorf("namespace setup failed: %w", err)
	}
	nm.logger.Debug("SetupNamespace completed successfully", "jobID", jobID)

	// Setup DNS
	if err := nm.dns.SetupDNS(jobID, allocation.Hostname, allocation.IP); err != nil {
		// Cleanup on failure (ignore errors)
		_ = nm.setup.CleanupNamespace(jobID)
		_ = nm.ReleaseIP(jobID)
		return nil, fmt.Errorf("DNS setup failed: %w", err)
	}

	return allocation, nil
}

// CleanupJobNetworking cleans up networking for a job
func (nm *NetworkManager) CleanupJobNetworking(jobID string) error {
	var errs []error

	// Cleanup DNS
	if err := nm.dns.CleanupDNS(jobID); err != nil {
		errs = append(errs, fmt.Errorf("DNS cleanup failed: %w", err))
	}

	// Cleanup namespace
	if err := nm.setup.CleanupNamespace(jobID); err != nil {
		errs = append(errs, fmt.Errorf("namespace cleanup failed: %w", err))
	}

	// Release IP
	if err := nm.ReleaseIP(jobID); err != nil {
		errs = append(errs, fmt.Errorf("IP release failed: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("cleanup errors: %v", errs)
	}

	return nil
}

// ValidateNetworkConfig validates network configuration
func (nm *NetworkManager) ValidateNetworkConfig(config *NetworkConfig) error {
	return nm.validator.ValidateNetworkConfig(config)
}

// GetNetworkInfo returns network information
func (nm *NetworkManager) GetNetworkInfo(name string) (*NetworkInfo, error) {
	nm.mutex.RLock()
	defer nm.mutex.RUnlock()

	config, exists := nm.networks[name]
	if !exists {
		return nil, fmt.Errorf("network %s does not exist", name)
	}

	jobCount := 0
	for _, allocation := range nm.allocations {
		if allocation.Network == name {
			jobCount++
		}
	}

	return &NetworkInfo{
		Name:     name,
		CIDR:     config.CIDR,
		Bridge:   config.Bridge,
		JobCount: jobCount,
	}, nil
}

// Monitoring methods - optional functionality

// StartMonitoring starts network monitoring if a monitor is configured
func (nm *NetworkManager) StartMonitoring(ctx context.Context) error {
	if nm.monitor == nil {
		nm.logger.Debug("no monitor configured, skipping monitoring")
		return nil
	}
	return nm.monitor.StartMonitoring(ctx)
}

// StopMonitoring stops network monitoring if a monitor is configured
func (nm *NetworkManager) StopMonitoring() error {
	if nm.monitor == nil {
		return nil
	}
	return nm.monitor.StopMonitoring()
}

// GetBandwidthStats returns bandwidth statistics for a job
func (nm *NetworkManager) GetBandwidthStats(jobID string) (*BandwidthStats, error) {
	if nm.monitor == nil {
		return nil, fmt.Errorf("monitoring not configured")
	}
	return nm.monitor.GetBandwidthStats(jobID)
}

// GetNetworkStats returns statistics for a network
func (nm *NetworkManager) GetNetworkStats(networkName string) (*BandwidthStats, error) {
	if nm.monitor == nil {
		return nil, fmt.Errorf("monitoring not configured")
	}
	return nm.monitor.GetNetworkStats(networkName)
}

// SetBandwidthLimits sets bandwidth limits for a job
func (nm *NetworkManager) SetBandwidthLimits(jobID string, limits *NetworkLimits) error {
	if nm.monitor == nil {
		return fmt.Errorf("monitoring not configured")
	}
	return nm.monitor.SetBandwidthLimits(jobID, limits)
}
