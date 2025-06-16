//go:build linux

package network

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"job-worker/pkg/logger"
	osinterface "job-worker/pkg/os"
)

// Group represents a network isolation group with configuration
type Group struct {
	GroupID       string    `json:"group_id"`
	JobCount      int32     `json:"job_count"`
	NamePath      string    `json:"name_path"`
	CreatedAt     time.Time `json:"created_at"`
	NetworkConfig *Config   `json:"network_config"`
}

// GroupInfo contains information returned when handling network groups
type GroupInfo struct {
	NamePath           string
	SysProcAttr        *syscall.SysProcAttr
	IsNewGroup         bool
	NeedsNamespaceJoin bool
	NetworkConfig      *Config
}

// Manager handles network group management and coordination
// Implements NamespaceCleaner interface for process cleanup integration
type Manager struct {
	groups          map[string]*Group
	mutex           sync.RWMutex
	subnetAllocator *SubnetAllocator
	namespaceOps    *NamespaceOperations
	syscall         osinterface.SyscallInterface
	osInterface     osinterface.OsInterface
	paths           *NetworkPaths
	logger          *logger.Logger
}

// Dependencies holds all dependencies needed by the network manager
type Dependencies struct {
	SubnetAllocator *SubnetAllocator
	NamespaceOps    *NamespaceOperations
	Syscall         osinterface.SyscallInterface
	OsInterface     osinterface.OsInterface
	Paths           *NetworkPaths
}

// NewManager creates a new network manager
func NewManager(deps *Dependencies) *Manager {
	if deps.Paths == nil {
		deps.Paths = NewDefaultPaths()
	}

	return &Manager{
		groups:          make(map[string]*Group),
		subnetAllocator: deps.SubnetAllocator,
		namespaceOps:    deps.NamespaceOps,
		syscall:         deps.Syscall,
		osInterface:     deps.OsInterface,
		paths:           deps.Paths,
		logger:          logger.New().WithField("component", "network-manager"),
	}
}

// RemoveNamespace removes a namespace file or symlink (implements NamespaceCleaner interface)
func (m *Manager) RemoveNamespace(nsPath string, isBound bool) error {
	if m.namespaceOps == nil {
		return fmt.Errorf("namespace operations not available")
	}

	log := m.logger.WithFields("nsPath", nsPath, "isBound", isBound)
	log.Debug("removing namespace via manager")

	return m.namespaceOps.RemoveNamespace(nsPath, isBound)
}

// HandleNetworkGroup handles network group creation or joining
func (m *Manager) HandleNetworkGroup(groupID, jobID string) (*GroupInfo, error) {
	if groupID == "" {
		return nil, fmt.Errorf("groupID cannot be empty")
	}
	if jobID == "" {
		return nil, fmt.Errorf("jobID cannot be empty")
	}

	log := m.logger.WithFields("groupID", groupID, "jobID", jobID)
	log.Debug("handling network group request")

	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Check if group exists
	if group, exists := m.groups[groupID]; exists {
		return m.joinExistingGroup(group, jobID)
	}

	return m.createNewGroup(groupID, jobID)
}

// joinExistingGroup handles joining an existing network group
func (m *Manager) joinExistingGroup(group *Group, jobID string) (*GroupInfo, error) {
	log := m.logger.WithFields("groupID", group.GroupID, "jobID", jobID)

	log.Info("joining existing network group",
		"currentJobs", group.JobCount,
		"subnet", group.NetworkConfig.Subnet,
		"gateway", group.NetworkConfig.Gateway)

	// Verify the namespace file still exists
	if _, err := m.osInterface.Stat(group.NamePath); err != nil {
		log.Warn("network group namespace file missing, recreating group",
			"path", group.NamePath,
			"error", err)

		// Remove the broken group and recreate
		delete(m.groups, group.GroupID)
		return m.createNewGroup(group.GroupID, jobID)
	}

	// Create sysProcAttr for joining existing namespace
	// No new network namespace - we'll join existing namespace using setns before fork
	sysProcAttr := m.syscall.CreateProcessGroup()
	sysProcAttr.Cloneflags = syscall.CLONE_NEWPID |
		syscall.CLONE_NEWNS |
		syscall.CLONE_NEWIPC |
		syscall.CLONE_NEWUTS
	// Note: No CLONE_NEWNET - we'll join existing namespace using setns before fork

	return &GroupInfo{
		NamePath:           group.NamePath,
		SysProcAttr:        sysProcAttr,
		IsNewGroup:         false,
		NeedsNamespaceJoin: true,
		NetworkConfig:      group.NetworkConfig.Clone(),
	}, nil
}

// createNewGroup creates a new network group
func (m *Manager) createNewGroup(groupID, jobID string) (*GroupInfo, error) {
	log := m.logger.WithFields("groupID", groupID, "jobID", jobID)
	log.Info("creating new network group")

	// Get network configuration for this group
	networkConfig, err := m.getNetworkConfigForGroup(groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to get network config for group %s: %w", groupID, err)
	}

	// Validate network configuration
	if err := networkConfig.Validate(); err != nil {
		return nil, fmt.Errorf("invalid network config for group %s: %w", groupID, err)
	}

	// Build namespace path
	nsPath := m.namespaceOps.BuildNamespacePath(groupID, true) // Use persistent path

	// Validate namespace path
	if err := m.namespaceOps.ValidateNamespacePath(nsPath); err != nil {
		return nil, fmt.Errorf("invalid namespace path: %w", err)
	}

	// Create sysProcAttr with new network namespace
	sysProcAttr := m.createSysProcAttrWithNamespaces()

	// Create the group
	group := &Group{
		GroupID:       groupID,
		JobCount:      0,
		NamePath:      nsPath,
		CreatedAt:     time.Now(),
		NetworkConfig: networkConfig.Clone(),
	}

	m.groups[groupID] = group

	log.Info("network group created with configuration",
		"subnet", networkConfig.Subnet,
		"gateway", networkConfig.Gateway,
		"interface", networkConfig.Interface,
		"namespacePath", nsPath)

	return &GroupInfo{
		NamePath:           nsPath,
		SysProcAttr:        sysProcAttr,
		IsNewGroup:         true,
		NeedsNamespaceJoin: false,
		NetworkConfig:      networkConfig.Clone(),
	}, nil
}

// getNetworkConfigForGroup returns network configuration for a group
func (m *Manager) getNetworkConfigForGroup(groupID string) (*Config, error) {
	log := m.logger.WithField("groupID", groupID)

	// Option 1: Check for environment variable override
	envSubnetKey := fmt.Sprintf("NETWORK_GROUP_%s_SUBNET", strings.ToUpper(groupID))
	envGatewayKey := fmt.Sprintf("NETWORK_GROUP_%s_GATEWAY", strings.ToUpper(groupID))

	if customSubnet := m.osInterface.Getenv(envSubnetKey); customSubnet != "" {
		log.Debug("using custom subnet from environment", "subnet", customSubnet)

		gateway := m.osInterface.Getenv(envGatewayKey)
		if gateway == "" {
			// Calculate default gateway (.1) for the subnet
			if _, ipNet, err := net.ParseCIDR(customSubnet); err == nil {
				ip := ipNet.IP.Mask(ipNet.Mask)
				ip[len(ip)-1] = 1 // Set last octet to 1
				gateway = ip.String()
			} else {
				return nil, fmt.Errorf("invalid custom subnet %s: %w", customSubnet, err)
			}
		}

		config := &Config{
			Subnet:    customSubnet,
			Gateway:   gateway,
			Interface: DefaultInterface,
		}

		if err := config.Validate(); err != nil {
			return nil, fmt.Errorf("invalid custom network config: %w", err)
		}

		return config, nil
	}

	// Option 2: Dynamic allocation (prevents conflicts between groups)
	if m.subnetAllocator != nil {
		config, err := m.subnetAllocator.AllocateSubnet(groupID)
		if err == nil {
			log.Debug("allocated dynamic subnet",
				"subnet", config.Subnet,
				"gateway", config.Gateway)
			return config, nil
		}
		log.Warn("failed to allocate dynamic subnet, using default", "error", err)
	}

	// Option 3: Default configuration
	config := NewDefaultConfig()
	log.Debug("using default network configuration",
		"subnet", config.Subnet,
		"gateway", config.Gateway)

	return config, nil
}

// IncrementJobCount increments the job count for a network group
func (m *Manager) IncrementJobCount(groupID string) error {
	if groupID == "" {
		return fmt.Errorf("groupID cannot be empty")
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	group, exists := m.groups[groupID]
	if !exists {
		return fmt.Errorf("network group not found: %s", groupID)
	}

	newCount := atomic.AddInt32(&group.JobCount, 1)
	m.logger.Debug("incremented job count for network group",
		"groupID", groupID,
		"newCount", newCount)

	return nil
}

// DecrementJobCount decrements the job count for a network group and cleans up if empty
func (m *Manager) DecrementJobCount(groupID string) error {
	if groupID == "" {
		return fmt.Errorf("groupID cannot be empty")
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	group, exists := m.groups[groupID]
	if !exists {
		m.logger.Warn("attempted to decrement job count for non-existent group", "groupID", groupID)
		return nil // Don't error, group might have been cleaned up already
	}

	newCount := atomic.AddInt32(&group.JobCount, -1)
	m.logger.Debug("decremented job count for network group",
		"groupID", groupID,
		"newCount", newCount)

	// Cleanup empty group
	if newCount <= 0 {
		return m.cleanupEmptyGroup(groupID, group)
	}

	return nil
}

// cleanupEmptyGroup cleans up a network group that has no more jobs
func (m *Manager) cleanupEmptyGroup(groupID string, group *Group) error {
	log := m.logger.WithField("groupID", groupID)
	log.Info("cleaning up empty network group")

	// Release subnet allocation
	if m.subnetAllocator != nil {
		m.subnetAllocator.ReleaseSubnet(groupID)
	}

	// Remove namespace
	isBound := true // Network groups use bind mounts
	if err := m.namespaceOps.RemoveNamespace(group.NamePath, isBound); err != nil {
		log.Warn("failed to remove namespace", "path", group.NamePath, "error", err)
		// Continue with cleanup even if namespace removal fails
	}

	// Remove from groups map
	delete(m.groups, groupID)

	log.Info("network group cleaned up successfully",
		"subnet", group.NetworkConfig.Subnet,
		"namespacePath", group.NamePath)

	return nil
}

// GetGroup returns information about a network group
func (m *Manager) GetGroup(groupID string) (*Group, bool) {
	if groupID == "" {
		return nil, false
	}

	m.mutex.RLock()
	defer m.mutex.RUnlock()

	group, exists := m.groups[groupID]
	if !exists {
		return nil, false
	}

	// Return a copy to prevent external modification
	groupCopy := &Group{
		GroupID:       group.GroupID,
		JobCount:      atomic.LoadInt32(&group.JobCount),
		NamePath:      group.NamePath,
		CreatedAt:     group.CreatedAt,
		NetworkConfig: group.NetworkConfig.Clone(),
	}

	return groupCopy, true
}

// ListGroups returns a list of all network groups
func (m *Manager) ListGroups() []*Group {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	groups := make([]*Group, 0, len(m.groups))
	for _, group := range m.groups {
		// Return copies to prevent external modification
		groupCopy := &Group{
			GroupID:       group.GroupID,
			JobCount:      atomic.LoadInt32(&group.JobCount),
			NamePath:      group.NamePath,
			CreatedAt:     group.CreatedAt,
			NetworkConfig: group.NetworkConfig.Clone(),
		}
		groups = append(groups, groupCopy)
	}

	return groups
}

// GetStats returns statistics about network groups
func (m *Manager) GetStats() map[string]interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var totalJobs int32
	activeGroups := 0

	for _, group := range m.groups {
		jobCount := atomic.LoadInt32(&group.JobCount)
		totalJobs += jobCount
		if jobCount > 0 {
			activeGroups++
		}
	}

	stats := map[string]interface{}{
		"total_groups":  len(m.groups),
		"active_groups": activeGroups,
		"total_jobs":    totalJobs,
	}

	// Add subnet allocator stats if available
	if m.subnetAllocator != nil {
		subnetStats := m.subnetAllocator.GetStats()
		for k, v := range subnetStats {
			stats["subnet_"+k] = v
		}
	}

	return stats
}

// CleanupGroup forcefully cleans up a network group (used for emergency cleanup)
func (m *Manager) CleanupGroup(groupID string) error {
	if groupID == "" {
		return fmt.Errorf("groupID cannot be empty")
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	group, exists := m.groups[groupID]
	if !exists {
		m.logger.Debug("group not found during cleanup", "groupID", groupID)
		return nil // Already cleaned up
	}

	log := m.logger.WithField("groupID", groupID)
	log.Warn("forcefully cleaning up network group")

	// Reset job count and cleanup
	atomic.StoreInt32(&group.JobCount, 0)
	return m.cleanupEmptyGroup(groupID, group)
}

// ValidateGroupExists checks if a network group exists and is valid
func (m *Manager) ValidateGroupExists(groupID string) error {
	if groupID == "" {
		return fmt.Errorf("groupID cannot be empty")
	}

	m.mutex.RLock()
	defer m.mutex.RUnlock()

	group, exists := m.groups[groupID]
	if !exists {
		return fmt.Errorf("network group not found: %s", groupID)
	}

	// Verify namespace still exists
	if _, err := m.osInterface.Stat(group.NamePath); err != nil {
		return fmt.Errorf("network group namespace missing: %s (%w)", group.NamePath, err)
	}

	return nil
}

// createSysProcAttrWithNamespaces creates syscall attributes with all required namespaces
func (m *Manager) createSysProcAttrWithNamespaces() *syscall.SysProcAttr {
	sysProcAttr := m.syscall.CreateProcessGroup()

	sysProcAttr.Cloneflags = syscall.CLONE_NEWPID | // PID namespace - ALWAYS isolated
		syscall.CLONE_NEWNS | // Mount namespace - ALWAYS isolated
		syscall.CLONE_NEWIPC | // IPC namespace - ALWAYS isolated
		syscall.CLONE_NEWUTS | // UTS namespace - ALWAYS isolated
		syscall.CLONE_NEWNET // Network namespace - isolated by default

	m.logger.Debug("enabled namespace isolation",
		"flags", fmt.Sprintf("0x%x", sysProcAttr.Cloneflags),
		"namespaces", "pid,mount,ipc,uts,net")

	return sysProcAttr
}

// PrepareEnvironment prepares environment variables for a job in a network group
func (m *Manager) PrepareEnvironment(groupID string, isNewGroup bool) []string {
	var envVars []string

	if groupID != "" {
		envVars = append(envVars,
			fmt.Sprintf("NETWORK_GROUP_ID=%s", groupID),
			fmt.Sprintf("IS_NEW_NETWORK_GROUP=%t", isNewGroup))

		// Add network configuration if group exists
		m.mutex.RLock()
		if group, exists := m.groups[groupID]; exists && group.NetworkConfig != nil {
			envVars = append(envVars,
				fmt.Sprintf("INTERNAL_SUBNET=%s", group.NetworkConfig.Subnet),
				fmt.Sprintf("INTERNAL_GATEWAY=%s", group.NetworkConfig.Gateway),
				fmt.Sprintf("INTERNAL_INTERFACE=%s", group.NetworkConfig.Interface))

			m.logger.Debug("added network config to environment",
				"groupID", groupID,
				"subnet", group.NetworkConfig.Subnet,
				"gateway", group.NetworkConfig.Gateway,
				"interface", group.NetworkConfig.Interface)
		}
		m.mutex.RUnlock()
	}

	return envVars
}

// Shutdown gracefully shuts down the network manager
func (m *Manager) Shutdown() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.logger.Info("shutting down network manager", "totalGroups", len(m.groups))

	var errors []error

	// Clean up all groups
	for groupID, group := range m.groups {
		log := m.logger.WithField("groupID", groupID)
		log.Debug("cleaning up group during shutdown")

		// Reset job count
		atomic.StoreInt32(&group.JobCount, 0)

		// Release subnet allocation
		if m.subnetAllocator != nil {
			m.subnetAllocator.ReleaseSubnet(groupID)
		}

		// Remove namespace
		isBound := true
		if err := m.namespaceOps.RemoveNamespace(group.NamePath, isBound); err != nil {
			log.Warn("failed to remove namespace during shutdown", "error", err)
			errors = append(errors, fmt.Errorf("failed to remove namespace for group %s: %w", groupID, err))
		}
	}

	// Clear groups map
	m.groups = make(map[string]*Group)

	if len(errors) > 0 {
		return fmt.Errorf("shutdown completed with %d errors (first error: %w)", len(errors), errors[0])
	}

	m.logger.Info("network manager shutdown completed successfully")
	return nil
}

// GetNamespaceOperations returns the namespace operations instance
func (m *Manager) GetNamespaceOperations() *NamespaceOperations {
	return m.namespaceOps
}

// CreateNamespaceForJob creates a namespace artifact for a specific job
func (m *Manager) CreateNamespaceForJob(jobID string, pid int32, nsPath string, isNetworkGroup bool) error {
	if m.namespaceOps == nil {
		return fmt.Errorf("namespace operations not available")
	}

	log := m.logger.WithFields("jobID", jobID, "pid", pid, "nsPath", nsPath, "isNetworkGroup", isNetworkGroup)
	log.Debug("creating namespace for job")

	if isNetworkGroup {
		// Create bind mount for persistent network groups
		return m.namespaceOps.CreateNamespaceBindMount(pid, nsPath)
	} else {
		// Create symlink for isolated jobs
		return m.namespaceOps.CreateNamespaceSymlink(pid, nsPath)
	}
}
