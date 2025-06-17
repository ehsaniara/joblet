//go:build linux

// Package network provides high-level network group management for job isolation.
// This package orchestrates network namespace operations, subnet allocation,
// group lifecycle management, and automatic IP assignment to provide isolated
// network environments for job execution.
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

// Group represents a network isolation group with its configuration and state.
type Group struct {
	GroupID       string    `json:"group_id"`       // Unique identifier for the network group
	JobCount      int32     `json:"job_count"`      // Atomic counter of active jobs in this group
	NamePath      string    `json:"name_path"`      // Filesystem path to namespace bind mount
	CreatedAt     time.Time `json:"created_at"`     // Timestamp when group was created
	NetworkConfig *Config   `json:"network_config"` // Network settings (subnet, gateway, interface)
}

// GroupInfo contains information returned when handling network group operations.
type GroupInfo struct {
	NamePath           string               // Path to namespace file (for setns operations)
	SysProcAttr        *syscall.SysProcAttr // Process attributes for namespace creation/joining
	IsNewGroup         bool                 // true if this group was just created
	NeedsNamespaceJoin bool                 // true if process should join existing namespace
	NetworkConfig      *Config              // Network configuration for the group
}

// Manager handles network group management, automatic IP assignment, and coordination.
type Manager struct {
	groups          map[string]*Group            // Maps groupID -> Group (protected by mutex)
	mutex           sync.RWMutex                 // Protects concurrent access to groups map
	subnetAllocator *SubnetAllocator             // Manages dynamic subnet allocation
	ipPoolManager   *IPPoolManager               // Manages IP assignment within groups
	namespaceOps    *NamespaceOperations         // Handles low-level namespace operations
	syscall         osinterface.SyscallInterface // System call interface (for testing)
	osInterface     osinterface.OsInterface      // OS interface (for testing)
	paths           *NetworkPaths                // Configured filesystem paths
	logger          *logger.Logger               // Structured logger for debugging
}

// Dependencies holds all dependencies needed by the network manager.
type Dependencies struct {
	SubnetAllocator *SubnetAllocator             // For dynamic subnet allocation
	NamespaceOps    *NamespaceOperations         // For namespace operations
	Syscall         osinterface.SyscallInterface // System call interface
	OsInterface     osinterface.OsInterface      // OS operations interface
	Paths           *NetworkPaths                // Network filesystem paths
}

// NewManager creates a new network manager with the specified dependencies.
func NewManager(deps *Dependencies) *Manager {
	if deps.Paths == nil {
		deps.Paths = NewDefaultPaths()
	}

	return &Manager{
		groups:          make(map[string]*Group),
		subnetAllocator: deps.SubnetAllocator,
		ipPoolManager:   NewIPPoolManager(),
		namespaceOps:    deps.NamespaceOps,
		syscall:         deps.Syscall,
		osInterface:     deps.OsInterface,
		paths:           deps.Paths,
		logger:          logger.New().WithField("component", "network-manager"),
	}
}

// HandleNetworkGroupWithAutoIP handles network group creation/joining with automatic IP assignment.
// This is the main entry point for Docker-style automatic IP assignment.
func (m *Manager) HandleNetworkGroupWithAutoIP(groupID, jobID string) (*GroupInfo, net.IP, error) {
	if groupID == "" {
		return nil, nil, fmt.Errorf("groupID cannot be empty")
	}
	if jobID == "" {
		return nil, nil, fmt.Errorf("jobID cannot be empty")
	}

	log := m.logger.WithFields("groupID", groupID, "jobID", jobID)
	log.Debug("handling network group request with automatic IP assignment")

	// Use exclusive lock since we might modify the groups map
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Check if group already exists
	if group, exists := m.groups[groupID]; exists {
		log.Info("joining existing network group",
			"existingJobs", atomic.LoadInt32(&group.JobCount),
			"subnet", group.NetworkConfig.Subnet)

		groupInfo, err := m.joinExistingGroup(group, jobID)
		if err != nil {
			return nil, nil, err
		}

		// Server automatically assigns next available IP
		assignedIP, err := m.ipPoolManager.AllocateIP(groupID, jobID, group.NetworkConfig.Subnet)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to auto-assign IP for existing group: %w", err)
		}

		log.Info("auto-assigned IP for existing group",
			"ip", assignedIP.String(),
			"totalJobs", atomic.LoadInt32(&group.JobCount)+1)

		return groupInfo, assignedIP, nil
	}

	// Create new group
	log.Info("creating new network group with automatic IP assignment")

	groupInfo, err := m.createNewGroup(groupID, jobID)
	if err != nil {
		return nil, nil, err
	}

	// Server automatically assigns first IP in new group
	group := m.groups[groupID]
	assignedIP, err := m.ipPoolManager.AllocateIP(groupID, jobID, group.NetworkConfig.Subnet)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to auto-assign IP for new group: %w", err)
	}

	log.Info("auto-assigned IP for new group",
		"ip", assignedIP.String(),
		"subnet", group.NetworkConfig.Subnet,
		"gateway", group.NetworkConfig.Gateway)

	return groupInfo, assignedIP, nil
}

// joinExistingGroup handles the process of joining an existing network group.
func (m *Manager) joinExistingGroup(group *Group, jobID string) (*GroupInfo, error) {
	log := m.logger.WithFields("groupID", group.GroupID, "jobID", jobID)

	log.Info("joining existing network group",
		"currentJobs", atomic.LoadInt32(&group.JobCount),
		"subnet", group.NetworkConfig.Subnet,
		"gateway", group.NetworkConfig.Gateway)

	// Verify the namespace file still exists
	if _, err := m.osInterface.Stat(group.NamePath); err != nil {
		log.Warn("network group namespace file missing, recreating group",
			"path", group.NamePath,
			"error", err)

		// Remove the broken group and recreate it
		delete(m.groups, group.GroupID)
		return m.createNewGroup(group.GroupID, jobID)
	}

	// Create sysProcAttr for joining existing namespace
	// Key difference: NO CLONE_NEWNET flag because we'll join existing namespace
	sysProcAttr := m.syscall.CreateProcessGroup()
	sysProcAttr.Cloneflags = syscall.CLONE_NEWPID | // Still need isolated PID namespace
		syscall.CLONE_NEWNS | // Still need isolated mount namespace
		syscall.CLONE_NEWIPC | // Still need isolated IPC namespace
		syscall.CLONE_NEWUTS // Still need isolated UTS namespace
	// Note: No CLONE_NEWNET - we'll join existing namespace using setns before fork

	return &GroupInfo{
		NamePath:           group.NamePath,
		SysProcAttr:        sysProcAttr,
		IsNewGroup:         false, // This is an existing group
		NeedsNamespaceJoin: true,  // Process needs to call setns() before fork
		NetworkConfig:      group.NetworkConfig.Clone(),
	}, nil
}

// createNewGroup creates a new network group with unique configuration.
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

	// Build namespace path for persistent storage
	nsPath := m.namespaceOps.BuildNamespacePath(groupID, true) // true = persistent

	// Validate namespace path for security
	if err := m.namespaceOps.ValidateNamespacePath(nsPath); err != nil {
		return nil, fmt.Errorf("invalid namespace path: %w", err)
	}

	// Create sysProcAttr with new network namespace
	sysProcAttr := m.createSysProcAttrWithNamespaces()

	// Create the group structure
	group := &Group{
		GroupID:       groupID,
		JobCount:      0, // Will be incremented when jobs are added
		NamePath:      nsPath,
		CreatedAt:     time.Now(),
		NetworkConfig: networkConfig.Clone(),
	}

	// Store the group in our map
	m.groups[groupID] = group

	log.Info("network group created with configuration",
		"subnet", networkConfig.Subnet,
		"gateway", networkConfig.Gateway,
		"interface", networkConfig.Interface,
		"namespacePath", nsPath)

	return &GroupInfo{
		NamePath:           nsPath,
		SysProcAttr:        sysProcAttr,
		IsNewGroup:         true,  // This is a newly created group
		NeedsNamespaceJoin: false, // Process will create new namespace, not join existing
		NetworkConfig:      networkConfig.Clone(),
	}, nil
}

// getNetworkConfigForGroup returns network configuration for a group.
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
				ip[len(ip)-1] = 1
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

// IncrementJobCount increments the job count for a network group.
func (m *Manager) IncrementJobCount(groupID string) error {
	if groupID == "" {
		return fmt.Errorf("groupID cannot be empty")
	}

	m.mutex.RLock()
	group, exists := m.groups[groupID]
	m.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("network group not found: %s", groupID)
	}

	newCount := atomic.AddInt32(&group.JobCount, 1)
	m.logger.Debug("incremented job count for network group",
		"groupID", groupID,
		"newCount", newCount)

	return nil
}

// DecrementJobCount decrements the job count and handles automatic IP cleanup.
func (m *Manager) DecrementJobCount(groupID string) error {
	if groupID == "" {
		return fmt.Errorf("groupID cannot be empty")
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	group, exists := m.groups[groupID]
	if !exists {
		m.logger.Warn("attempted to decrement job count for non-existent group", "groupID", groupID)
		return nil
	}

	newCount := atomic.AddInt32(&group.JobCount, -1)
	m.logger.Debug("decremented job count for network group",
		"groupID", groupID,
		"newCount", newCount)

	// Cleanup empty group automatically
	if newCount <= 0 {
		return m.cleanupEmptyGroupWithIPs(groupID, group)
	}

	return nil
}

// ReleaseJobIP releases an IP when a job completes.
func (m *Manager) ReleaseJobIP(groupID, jobID string) error {
	if groupID == "" || jobID == "" {
		return nil // No group or job ID, nothing to release
	}

	log := m.logger.WithFields("groupID", groupID, "jobID", jobID)
	log.Debug("releasing job IP")

	err := m.ipPoolManager.ReleaseIP(groupID, jobID)
	if err != nil {
		log.Warn("failed to release IP", "error", err)
		return err
	}

	log.Info("job IP released successfully")
	return nil
}

// GetJobIP returns the assigned IP for a job.
func (m *Manager) GetJobIP(groupID, jobID string) (net.IP, bool) {
	return m.ipPoolManager.GetJobIP(groupID, jobID)
}

// cleanupEmptyGroupWithIPs cleans up a network group and releases all IPs.
func (m *Manager) cleanupEmptyGroupWithIPs(groupID string, group *Group) error {
	log := m.logger.WithField("groupID", groupID)
	log.Info("cleaning up empty network group with IP pool cleanup")

	// Release subnet allocation
	if m.subnetAllocator != nil {
		m.subnetAllocator.ReleaseSubnet(groupID)
	}

	// Remove namespace bind mount
	isBound := true
	if err := m.namespaceOps.RemoveNamespace(group.NamePath, isBound); err != nil {
		log.Warn("failed to remove namespace", "path", group.NamePath, "error", err)
	}

	// Remove from groups map
	delete(m.groups, groupID)

	log.Info("network group with IP pool cleaned up successfully",
		"subnet", group.NetworkConfig.Subnet,
		"namespacePath", group.NamePath)

	return nil
}

// GetGroup returns information about a network group.
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

// ListGroups returns a list of all network groups.
func (m *Manager) ListGroups() []*Group {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	groups := make([]*Group, 0, len(m.groups))
	for _, group := range m.groups {
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

// GetStats returns statistics about network groups and IP assignment.
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

	// Add IP pool stats
	allPoolStats := m.getAllIPPoolStats()
	stats["ip_pools"] = allPoolStats

	return stats
}

// GetIPPoolStats returns IP pool statistics for a specific group.
func (m *Manager) GetIPPoolStats(groupID string) map[string]interface{} {
	return m.ipPoolManager.GetPoolStats(groupID)
}

// getAllIPPoolStats returns stats for all IP pools.
func (m *Manager) getAllIPPoolStats() map[string]interface{} {
	allStats := make(map[string]interface{})

	m.mutex.RLock()
	for groupID := range m.groups {
		allStats[groupID] = m.ipPoolManager.GetPoolStats(groupID)
	}
	m.mutex.RUnlock()

	return allStats
}

// CleanupGroup forcefully cleans up a network group.
func (m *Manager) CleanupGroup(groupID string) error {
	if groupID == "" {
		return fmt.Errorf("groupID cannot be empty")
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	group, exists := m.groups[groupID]
	if !exists {
		m.logger.Debug("group not found during cleanup", "groupID", groupID)
		return nil
	}

	log := m.logger.WithField("groupID", groupID)
	log.Warn("forcefully cleaning up network group")

	// Reset job count to zero and cleanup
	atomic.StoreInt32(&group.JobCount, 0)
	return m.cleanupEmptyGroupWithIPs(groupID, group)
}

// ValidateGroupExists checks if a network group exists and is valid.
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

	// Verify namespace still exists on filesystem
	if _, err := m.osInterface.Stat(group.NamePath); err != nil {
		return fmt.Errorf("network group namespace missing: %s (%w)", group.NamePath, err)
	}

	return nil
}

// createSysProcAttrWithNamespaces creates syscall attributes with all required namespaces.
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

// PrepareEnvironment prepares environment variables for a job in a network group.
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

// Shutdown gracefully shuts down the network manager.
func (m *Manager) Shutdown() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.logger.Info("shutting down network manager", "totalGroups", len(m.groups))

	var errors []error

	// Clean up all groups systematically
	for groupID, group := range m.groups {
		log := m.logger.WithField("groupID", groupID)
		log.Debug("cleaning up group during shutdown")

		// Reset job count to force cleanup
		atomic.StoreInt32(&group.JobCount, 0)

		// Release subnet allocation
		if m.subnetAllocator != nil {
			m.subnetAllocator.ReleaseSubnet(groupID)
		}

		// Remove namespace (bind mount)
		isBound := true
		if err := m.namespaceOps.RemoveNamespace(group.NamePath, isBound); err != nil {
			log.Warn("failed to remove namespace during shutdown", "error", err)
			errors = append(errors, fmt.Errorf("failed to remove namespace for group %s: %w", groupID, err))
		}
	}

	// Clear groups map to release memory
	m.groups = make(map[string]*Group)

	if len(errors) > 0 {
		return fmt.Errorf("shutdown completed with %d errors (first error: %w)", len(errors), errors[0])
	}

	m.logger.Info("network manager shutdown completed successfully")
	return nil
}

// RemoveNamespace removes a namespace file or symlink (implements NamespaceCleaner interface).
func (m *Manager) RemoveNamespace(nsPath string, isBound bool) error {
	if m.namespaceOps == nil {
		return fmt.Errorf("namespace operations not available")
	}

	log := m.logger.WithFields("nsPath", nsPath, "isBound", isBound)
	log.Debug("removing namespace via manager")

	return m.namespaceOps.RemoveNamespace(nsPath, isBound)
}

// GetNamespaceOperations returns the namespace operations instance.
func (m *Manager) GetNamespaceOperations() *NamespaceOperations {
	return m.namespaceOps
}

// CreateNamespaceForJob creates a namespace artifact for a specific job.
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

// Advanced IP Management Methods

// ListJobIPs returns all allocated IPs for a network group.
func (m *Manager) ListJobIPs(groupID string) map[string]string {
	return m.ipPoolManager.ListAllocatedIPs(groupID)
}

// GetNetworkGroupSubnet returns the subnet for a network group.
func (m *Manager) GetNetworkGroupSubnet(groupID string) (string, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	group, exists := m.groups[groupID]
	if !exists {
		return "", false
	}

	return group.NetworkConfig.Subnet, true
}

// IsIPAllocated checks if an IP is allocated in any group.
func (m *Manager) IsIPAllocated(groupID string, ip net.IP) bool {
	allocatedIPs := m.ipPoolManager.ListAllocatedIPs(groupID)
	ipStr := ip.String()

	for _, allocatedIP := range allocatedIPs {
		if allocatedIP == ipStr {
			return true
		}
	}
	return false
}

// GetGroupForJob returns the group ID for a job (reverse lookup).
func (m *Manager) GetGroupForJob(jobID string) (string, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	for groupID := range m.groups {
		if _, exists := m.ipPoolManager.GetJobIP(groupID, jobID); exists {
			return groupID, true
		}
	}
	return "", false
}

// Health check and monitoring methods

// IsHealthy performs a basic health check of the network manager.
func (m *Manager) IsHealthy() bool {
	// Basic health checks
	if m.namespaceOps == nil || m.ipPoolManager == nil {
		return false
	}

	// Check that we can access filesystem paths
	if _, err := m.osInterface.Stat(m.paths.VarRunNetns); err != nil {
		return false
	}

	return true
}

// GetComponentStatus returns the status of each component.
func (m *Manager) GetComponentStatus() map[string]string {
	status := map[string]string{
		"manager":          "healthy",
		"namespace_ops":    "unknown",
		"ip_pool_manager":  "unknown",
		"subnet_allocator": "unknown",
	}

	if m.namespaceOps != nil {
		status["namespace_ops"] = "healthy"
	} else {
		status["namespace_ops"] = "unhealthy"
	}

	if m.ipPoolManager != nil {
		status["ip_pool_manager"] = "healthy"
	} else {
		status["ip_pool_manager"] = "unhealthy"
	}

	if m.subnetAllocator != nil {
		status["subnet_allocator"] = "healthy"
	} else {
		status["subnet_allocator"] = "unhealthy"
	}

	return status
}

// GetDetailedStats returns comprehensive statistics for monitoring.
func (m *Manager) GetDetailedStats() map[string]interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	stats := m.GetStats()

	// Add per-group details
	groupDetails := make(map[string]interface{})
	for groupID, group := range m.groups {
		groupDetails[groupID] = map[string]interface{}{
			"job_count":      atomic.LoadInt32(&group.JobCount),
			"created_at":     group.CreatedAt.Format(time.RFC3339),
			"subnet":         group.NetworkConfig.Subnet,
			"gateway":        group.NetworkConfig.Gateway,
			"namespace_path": group.NamePath,
			"allocated_ips":  m.ipPoolManager.ListAllocatedIPs(groupID),
		}
	}
	stats["group_details"] = groupDetails

	return stats
}
