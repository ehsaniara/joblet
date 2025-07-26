package state

import (
	"encoding/json"
	"fmt"
	"joblet/internal/joblet/network"
	"joblet/pkg/config"
	"joblet/pkg/logger"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// NetworkRuntime represents runtime state for a network
type NetworkRuntime struct {
	Config network.NetworkConfig
	IPPool *IPPool
	Jobs   sync.Map // JobID -> *network.JobAllocation
}

// NetworkStore manages network configurations and runtime state
type NetworkStore struct {
	configFile string
	configs    map[string]*network.NetworkConfig
	runtime    map[string]*NetworkRuntime
	mu         sync.RWMutex
	logger     *logger.Logger
	config     *config.NetworkConfig

	// Network components (interfaces to avoid circular dependency)
	validator     NetworkValidator
	cleaner       NetworkCleaner
	dnsManager    DNSManager
	limiter       BandwidthLimiter
	bridgeManager BridgeManager
}

// BridgeManager interface to avoid circular import with setup.go
type BridgeManager interface {
	RemoveBridge(networkName string) error
}

// NetworkValidator Interfaces to avoid circular dependencies
type NetworkValidator interface {
	ValidateBridgeName(name string) error
	ValidateCIDR(cidr string, existingNetworks map[string]string) error
}

type NetworkCleaner interface {
	StartPeriodicCleanup(interval time.Duration)
	CleanupOrphanedInterfaces() error
}

type DNSManager interface {
	SetupJobDNS(pid int, alloc *network.JobAllocation, networkJobs map[string]*network.JobAllocation) error
	CleanupJobDNS(jobID string) error
	UpdateNetworkDNS(networkName string, activeJobs []*network.JobAllocation) error
}

type BandwidthLimiter interface {
	ApplyJobLimits(vethName string, limits network.NetworkLimits) error
	RemoveJobLimits(vethName string) error
}

// NewNetworkStore creates a new network store
func NewNetworkStore(cfg *config.NetworkConfig) *NetworkStore {
	ns := &NetworkStore{
		configFile: filepath.Join(cfg.StateDir, "networks.json"),
		configs:    make(map[string]*network.NetworkConfig),
		runtime:    make(map[string]*NetworkRuntime),
		logger:     logger.WithField("component", "network-store"),
		config:     cfg,

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

// SetBridgeManager setter method to inject the bridge manager
func (ns *NetworkStore) SetBridgeManager(bm BridgeManager) {
	ns.bridgeManager = bm
}

// Initialize loads networks from disk and sets up defaults
func (ns *NetworkStore) Initialize() error {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	// Load existing networks
	if err := ns.loadNetworkConfigs(); err != nil {
		ns.logger.Warn("failed to load network configs, using defaults", "error", err)
	}

	// Ensure default networks exist from config
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
		// Fallback if no config provided
		if _, exists := ns.configs["bridge"]; !exists {
			ns.configs["bridge"] = &network.NetworkConfig{
				CIDR:   "172.20.0.0/16",
				Bridge: "joblet0",
			}
		}
	}

	// Initialize runtime state for all networks
	for name, config := range ns.configs {
		ns.runtime[name] = &NetworkRuntime{
			Config: *config,
			IPPool: NewIPPool(config.CIDR),
		}
	}

	// Save configuration
	return ns.saveNetworkConfigs()
}

// CreateNetwork creates a new custom network
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

// RemoveNetwork removes a custom network
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

// ListNetworks returns all networks with their job counts
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

// AssignJobToNetwork allocates network resources for a job
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

// SetupJobNetworkComplete is called after network setup to configure DNS
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

// ApplyBandwidthLimits applies bandwidth limits to a job
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

// ReleaseJob releases network resources for a job
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

// GetJobAllocation retrieves a job's network allocation
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

// CleanupOrphaned performs manual cleanup of orphaned resources
func (ns *NetworkStore) CleanupOrphaned() error {
	return ns.cleaner.CleanupOrphanedInterfaces()
}

// Helper methods

func (ns *NetworkStore) loadNetworkConfigs() error {
	data, err := os.ReadFile(ns.configFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // First run
		}
		return err
	}

	return json.Unmarshal(data, &ns.configs)
}

func (ns *NetworkStore) saveNetworkConfigs() error {
	// Ensure directory exists
	dir := filepath.Dir(ns.configFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(ns.configs, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(ns.configFile, data, 0644)
}

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

// GetNetworkConfig retrieves the configuration for a network
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
