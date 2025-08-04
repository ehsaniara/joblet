package state

import (
	"encoding/json"
	"fmt"
	"joblet/internal/joblet/network"
	"joblet/pkg/config"
	"joblet/pkg/logger"
	"joblet/pkg/platform"
	"net"
	"path/filepath"
	"sync"
	"time"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

// NetworkRuntime represents runtime state for a network including IP allocation.
// Contains the network configuration, IP pool for address management,
// and a thread-safe map of active job allocations within the network.
type NetworkRuntime struct {
	Config network.NetworkConfig
	IPPool *IPPool
	Jobs   sync.Map // JobID -> *network.JobAllocation
}

// NetworkStore manages network configurations and runtime state for job networking.
// Provides thread-safe operations for creating/removing networks, assigning jobs
// to networks, and managing network resources like IP allocation and DNS.
type NetworkStore struct {
	configFile string
	configs    map[string]*network.NetworkConfig
	runtime    map[string]*NetworkRuntime
	mu         sync.RWMutex
	logger     *logger.Logger
	config     *config.NetworkConfig
	platform   platform.Platform

	// Network components (interfaces to avoid circular dependency)
	validator     NetworkValidator
	cleaner       NetworkCleaner
	dnsManager    DNSManager
	limiter       BandwidthLimiter
	bridgeManager BridgeManager
}

// BridgeManager interface to avoid circular import with setup.go.
// Provides bridge network infrastructure management capabilities.
//
//counterfeiter:generate . BridgeManager
type BridgeManager interface {
	// RemoveBridge removes a network bridge by name.
	RemoveBridge(networkName string) error
}

// NetworkValidator provides network configuration validation.
// Validates bridge names and CIDR ranges to prevent conflicts.
//
//counterfeiter:generate . NetworkValidator
type NetworkValidator interface {
	// ValidateBridgeName ensures bridge name follows naming rules.
	ValidateBridgeName(name string) error
	// ValidateCIDR ensures CIDR doesn't conflict with existing networks.
	ValidateCIDR(cidr string, existingNetworks map[string]string) error
}

// NetworkCleaner provides cleanup operations for orphaned network resources.
//
//counterfeiter:generate . NetworkCleaner
type NetworkCleaner interface {
	// StartPeriodicCleanup starts automatic cleanup of orphaned interfaces.
	StartPeriodicCleanup(interval time.Duration)
	// CleanupOrphanedInterfaces removes abandoned network interfaces.
	CleanupOrphanedInterfaces() error
}

// DNSManager provides DNS resolution services for jobs within networks.
//
//counterfeiter:generate . DNSManager
type DNSManager interface {
	// SetupJobDNS configures DNS resolution for a job's network namespace.
	SetupJobDNS(pid int, alloc *network.JobAllocation, networkJobs map[string]*network.JobAllocation) error
	// CleanupJobDNS removes DNS configuration for a completed job.
	CleanupJobDNS(jobID string) error
	// UpdateNetworkDNS refreshes DNS records for all jobs in a network.
	UpdateNetworkDNS(networkName string, activeJobs []*network.JobAllocation) error
}

// BandwidthLimiter provides network traffic shaping capabilities.
//
//counterfeiter:generate . BandwidthLimiter
type BandwidthLimiter interface {
	// ApplyJobLimits applies bandwidth restrictions to a job's network interface.
	ApplyJobLimits(vethName string, limits network.NetworkLimits) error
	// RemoveJobLimits removes bandwidth restrictions from a network interface.
	RemoveJobLimits(vethName string) error
}

// NewNetworkStore creates a new network store with initialized components.
// Sets up network validators, cleaners, DNS manager, and bandwidth limiter.
// Starts periodic cleanup of orphaned network interfaces.
func NewNetworkStore(cfg *config.NetworkConfig, platform platform.Platform) *NetworkStore {
	ns := &NetworkStore{
		configFile: filepath.Join(cfg.StateDir, "networks.json"),
		configs:    make(map[string]*network.NetworkConfig),
		runtime:    make(map[string]*NetworkRuntime),
		logger:     logger.WithField("component", "network-store"),
		config:     cfg,
		platform:   platform,

		// Initialize network components
		validator:  network.NewNetworkValidator(),
		cleaner:    network.NewNetworkCleaner(),
		dnsManager: network.NewDNSManager(cfg.StateDir),
		limiter:    network.NewBandwidthLimiter(),
	}

	// Start periodic cleanup (every 5 minutes)
	ns.cleaner.StartPeriodicCleanup(5 * time.Minute)

	return ns
}

// SetBridgeManager injects the bridge manager dependency.
// Used to avoid circular dependencies during initialization.
func (ns *NetworkStore) SetBridgeManager(bm BridgeManager) {
	ns.bridgeManager = bm
}

// Initialize loads networks from disk and sets up default networks.
// Creates runtime state for each network including IP pools.
// Ensures default bridge network exists and saves configuration.
func (ns *NetworkStore) Initialize() error {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	// Load existing networks
	if err := ns.loadNetworkConfigs(); err != nil {
		ns.logger.Warn("failed to load network configs, using defaults", "error", err)
	}

	// Ensure default networks exist from networkConfig
	if ns.config != nil && ns.config.Networks != nil {
		for name, netDef := range ns.config.Networks {
			if _, exists := ns.configs[name]; !exists {
				ns.configs[name] = &network.NetworkConfig{
					CIDR:   netDef.CIDR,
					Bridge: netDef.BridgeName,
				}
			}
		}
	} else {
		// Fallback if no networkConfig provided
		if _, exists := ns.configs["bridge"]; !exists {
			ns.configs["bridge"] = &network.NetworkConfig{
				CIDR:   "172.20.0.0/16",
				Bridge: "joblet0",
			}
		}
	}

	// Initialize runtime state for all networks
	for name, networkConfig := range ns.configs {
		ns.runtime[name] = &NetworkRuntime{
			Config: *networkConfig,
			IPPool: NewIPPool(networkConfig.CIDR),
		}
	}

	// Save configuration
	return ns.saveNetworkConfigs()
}

// CreateNetwork creates a new custom network with the specified CIDR.
// Validates network name and CIDR, creates bridge name, and initializes
// runtime state including IP pool. Thread-safe operation.
func (ns *NetworkStore) CreateNetwork(name, cidr string) error {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	// Validate inputs
	if name == "" || name == "none" || name == "isolated" || name == "bridge" {
		return fmt.Errorf("invalid network name: %s", name)
	}

	if _, exists := ns.configs[name]; exists {
		return fmt.Errorf("network %s already exists", name)
	}

	// Validate bridge name
	if err := ns.validator.ValidateBridgeName(name); err != nil {
		return fmt.Errorf("invalid network name: %w", err)
	}

	// Build existing networks map for validation
	existingNetworks := make(map[string]string)
	for netName, netConfig := range ns.configs {
		existingNetworks[netName] = netConfig.CIDR
	}

	// Validate CIDR with comprehensive checks
	if err := ns.validator.ValidateCIDR(cidr, existingNetworks); err != nil {
		return fmt.Errorf("invalid CIDR: %w", err)
	}

	// Parse and normalize CIDR
	_, ipNet, _ := net.ParseCIDR(cidr)

	// Create bridge name
	bridge := fmt.Sprintf("joblet-%s", name)

	// Save configuration
	ns.configs[name] = &network.NetworkConfig{
		CIDR:   ipNet.String(),
		Bridge: bridge,
	}

	// Initialize runtime
	ns.runtime[name] = &NetworkRuntime{
		Config: *ns.configs[name],
		IPPool: NewIPPool(ipNet.String()),
	}

	ns.logger.Info("created network",
		"name", name,
		"cidr", ipNet.String(),
		"bridge", bridge)

	return ns.saveNetworkConfigs()
}

// RemoveNetwork removes a custom network after validation.
// Prevents removal of built-in networks and networks with active jobs.
// Removes bridge infrastructure and cleans up configuration.
func (ns *NetworkStore) RemoveNetwork(name string) error {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	// Prevent removal of built-in networks
	if name == "bridge" || name == "none" || name == "isolated" {
		return fmt.Errorf("cannot remove built-in network: %s", name)
	}

	runtime, exists := ns.runtime[name]
	if !exists {
		return fmt.Errorf("network %s not found", name)
	}

	// Check if network has active jobs
	jobCount := 0
	runtime.Jobs.Range(func(_, _ interface{}) bool {
		jobCount++
		return false
	})

	if jobCount > 0 {
		return fmt.Errorf("network %s has %d active jobs", name, jobCount)
	}

	// Remove the actual bridge
	if ns.bridgeManager != nil {
		if err := ns.bridgeManager.RemoveBridge(name); err != nil {
			ns.logger.Error("failed to remove bridge", "network", name, "error", err)
			// Continue with config cleanup even if bridge removal fails
		}
	} else {
		ns.logger.Warn("bridge manager not set, skipping bridge removal")
	}

	// Remove from configs and runtime
	delete(ns.configs, name)
	delete(ns.runtime, name)

	return ns.saveNetworkConfigs()
}

// ListNetworks returns all networks with their current job counts.
// Includes special networks (none, isolated) and runtime network information.
// Thread-safe read operation with network statistics.
func (ns *NetworkStore) ListNetworks() map[string]network.NetworkInfo {
	ns.mu.RLock()
	defer ns.mu.RUnlock()

	result := make(map[string]network.NetworkInfo)

	for name, runtime := range ns.runtime {
		jobCount := 0
		runtime.Jobs.Range(func(_, _ interface{}) bool {
			jobCount++
			return true
		})

		result[name] = network.NetworkInfo{
			Name:     name,
			CIDR:     runtime.Config.CIDR,
			Bridge:   runtime.Config.Bridge,
			JobCount: jobCount,
		}
	}

	// special networks
	result["none"] = network.NetworkInfo{
		Name:     "none",
		CIDR:     "N/A",
		Bridge:   "N/A",
		JobCount: 0,
	}

	result["isolated"] = network.NetworkInfo{
		Name:     "isolated",
		CIDR:     "N/A",
		Bridge:   "N/A",
		JobCount: 0,
	}

	return result
}

// AssignJobToNetwork allocates network resources for a job.
// Handles special networks (none, isolated) and assigns IP addresses
// for bridge networks. Creates veth pair names and stores allocation.
func (ns *NetworkStore) AssignJobToNetwork(jobID, networkName, hostname string) (*network.JobAllocation, error) {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	// Handle special networks
	if networkName == "none" || networkName == "isolated" {
		return &network.JobAllocation{
			JobID:    jobID,
			Network:  networkName,
			Hostname: hostname,
		}, nil
	}

	// Default to bridge if not specified
	if networkName == "" {
		networkName = "bridge"
	}

	runtime, exists := ns.runtime[networkName]
	if !exists {
		return nil, fmt.Errorf("network %s not found", networkName)
	}

	// Allocate IP
	ip := runtime.IPPool.Allocate()
	if ip == nil {
		return nil, fmt.Errorf("no available IPs in network %s", networkName)
	}

	// Generate veth names
	shortID := jobID
	if len(jobID) > 8 {
		shortID = jobID[:8]
	}
	vethHost := fmt.Sprintf("veth%s", shortID)
	vethPeer := fmt.Sprintf("veth%sp", shortID)

	// Create allocation
	alloc := &network.JobAllocation{
		JobID:    jobID,
		Network:  networkName,
		IP:       ip,
		Hostname: hostname,
		VethHost: vethHost,
		VethPeer: vethPeer,
	}

	// Store in runtime
	runtime.Jobs.Store(jobID, alloc)

	ns.logger.Debug("assigned job to network",
		"jobID", jobID,
		"network", networkName,
		"ip", ip.String(),
		"hostname", hostname)

	return alloc, nil
}

// SetupJobNetworkComplete configures DNS after network setup is complete.
// Sets up DNS resolution for the job and updates DNS for other jobs
// in the same network. Skips DNS for special networks.
func (ns *NetworkStore) SetupJobNetworkComplete(jobID string, pid int) error {
	alloc, err := ns.GetJobAllocation(jobID)
	if err != nil {
		return err
	}

	// Skip DNS for special networks
	if alloc.Network == "none" || alloc.Network == "isolated" {
		return nil
	}

	// Get all jobs in the network for DNS
	networkJobs := ns.getNetworkJobs(alloc.Network)

	// Setup DNS
	if err := ns.dnsManager.SetupJobDNS(pid, alloc, networkJobs); err != nil {
		ns.logger.Warn("failed to setup DNS", "error", err)
		// Don't fail job on DNS setup failure
	}

	// Update DNS for other jobs in the network
	go ns.updateNetworkDNS(alloc.Network)

	return nil
}

// ApplyBandwidthLimits applies ingress and egress bandwidth limits to a job.
// Configures traffic shaping on the job's veth interface.
// Skips networks without veth pairs (none, isolated).
func (ns *NetworkStore) ApplyBandwidthLimits(jobID string, ingressBPS, egressBPS int64) error {
	alloc, err := ns.GetJobAllocation(jobID)
	if err != nil {
		return err
	}

	// Skip for networks without veth
	if alloc.VethHost == "" {
		return nil
	}

	limits := network.NetworkLimits{
		IngressBPS: ingressBPS,
		EgressBPS:  egressBPS,
		BurstSize:  0, // Use default
	}

	return ns.limiter.ApplyJobLimits(alloc.VethHost, limits)
}

// ReleaseJob releases all network resources allocated to a job.
// Releases IP address, removes bandwidth limits, cleans up DNS,
// and updates DNS for remaining jobs in the network.
func (ns *NetworkStore) ReleaseJob(jobID string) {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	// Search all networks for the job
	for _, runtime := range ns.runtime {
		if val, exists := runtime.Jobs.LoadAndDelete(jobID); exists {
			alloc := val.(*network.JobAllocation)

			// Release IP
			if alloc.IP != nil {
				runtime.IPPool.Release(alloc.IP)
			}

			// Remove bandwidth limits
			if alloc.VethHost != "" {
				if err := ns.limiter.RemoveJobLimits(alloc.VethHost); err != nil {
					ns.logger.Debug("failed to remove bandwidth limits", "error", err)
				}
			}

			// Cleanup DNS
			if err := ns.dnsManager.CleanupJobDNS(jobID); err != nil {
				ns.logger.Debug("failed to cleanup DNS", "error", err)
			}

			ns.logger.Debug("released job from network",
				"jobID", jobID,
				"network", alloc.Network,
				"ip", alloc.IP)

			// Update DNS for remaining jobs
			go ns.updateNetworkDNS(alloc.Network)

			return
		}
	}
}

// GetJobAllocation retrieves a job's current network allocation.
// Searches all networks to find the job's allocation information.
// Thread-safe read operation.
func (ns *NetworkStore) GetJobAllocation(jobID string) (*network.JobAllocation, error) {
	ns.mu.RLock()
	defer ns.mu.RUnlock()

	for _, runtime := range ns.runtime {
		if val, exists := runtime.Jobs.Load(jobID); exists {
			return val.(*network.JobAllocation), nil
		}
	}

	return nil, fmt.Errorf("no network allocation found for job %s", jobID)
}

// CleanupOrphaned performs manual cleanup of orphaned network resources.
// Removes abandoned network interfaces that are no longer in use.
func (ns *NetworkStore) CleanupOrphaned() error {
	return ns.cleaner.CleanupOrphanedInterfaces()
}

// Helper methods for internal operations

// loadNetworkConfigs loads network configurations from persistent storage.
// Returns nil on first run when config file doesn't exist.
func (ns *NetworkStore) loadNetworkConfigs() error {
	data, err := ns.platform.ReadFile(ns.configFile)
	if err != nil {
		if ns.platform.IsNotExist(err) {
			return nil // First run
		}
		return err
	}

	return json.Unmarshal(data, &ns.configs)
}

// saveNetworkConfigs persists network configurations to disk.
// Ensures directory exists and saves JSON-formatted configuration.
func (ns *NetworkStore) saveNetworkConfigs() error {
	// Ensure directory exists
	dir := filepath.Dir(ns.configFile)
	if err := ns.platform.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(ns.configs, "", "  ")
	if err != nil {
		return err
	}

	return ns.platform.WriteFile(ns.configFile, data, 0644)
}

// getNetworkJobs retrieves all active job allocations for a network.
// Used for DNS updates and network management operations.
func (ns *NetworkStore) getNetworkJobs(networkName string) map[string]*network.JobAllocation {
	jobs := make(map[string]*network.JobAllocation)

	if runtime, exists := ns.runtime[networkName]; exists {
		runtime.Jobs.Range(func(key, value interface{}) bool {
			jobID := key.(string)
			alloc := value.(*network.JobAllocation)
			jobs[jobID] = alloc
			return true
		})
	}

	return jobs
}

// updateNetworkDNS updates DNS records for all jobs in a network.
// Called asynchronously when jobs join or leave networks.
func (ns *NetworkStore) updateNetworkDNS(networkName string) {
	// Get all active jobs in the network
	var activeJobs []*network.JobAllocation

	if runtime, exists := ns.runtime[networkName]; exists {
		runtime.Jobs.Range(func(_, value interface{}) bool {
			activeJobs = append(activeJobs, value.(*network.JobAllocation))
			return true
		})
	}

	// Update DNS for all jobs
	if err := ns.dnsManager.UpdateNetworkDNS(networkName, activeJobs); err != nil {
		ns.logger.Warn("failed to update network DNS", "error", err)
	}
}

// GetNetworkConfig retrieves the configuration for a network by name.
// Searches runtime state first, then stored configurations.
// Thread-safe read operation.
func (ns *NetworkStore) GetNetworkConfig(name string) (*network.NetworkConfig, error) {
	ns.mu.RLock()
	defer ns.mu.RUnlock()

	// Check runtime first
	if runtime, exists := ns.runtime[name]; exists {
		networkConfig := runtime.Config // This is already a network.NetworkConfig
		return &networkConfig, nil
	}

	// Check configs
	if networkConfig, exists := ns.configs[name]; exists {
		return networkConfig, nil
	}

	return nil, fmt.Errorf("network %s not found", name)
}
