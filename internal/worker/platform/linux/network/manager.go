//go:build linux

// Package network provides high-level network group management for job isolation.
// This package orchestrates network namespace operations, subnet allocation,
// and group lifecycle management to provide isolated network environments
// for job execution.
//
// The Manager coordinates between:
// - SubnetAllocator: Dynamic IP subnet allocation to prevent conflicts
// - NamespaceOperations: Low-level namespace creation and management
// - Network groups: Logical groupings of jobs sharing network isolation
//
// Network Group Types:
// 1. Isolated Jobs: Each job gets its own private namespace (no networkGroupID)
// 2. Network Groups: Multiple jobs share a namespace (same networkGroupID)
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
// Each group maintains:
// - A unique network configuration (subnet, gateway, interface)
// - A count of active jobs using the group
// - A persistent namespace reference (bind mount)
// - Metadata for monitoring and debugging
//
// Groups are automatically created when the first job references them
// and cleaned up when the last job finishes.
type Group struct {
	GroupID       string    `json:"group_id"`       // Unique identifier for the network group
	JobCount      int32     `json:"job_count"`      // Atomic counter of active jobs in this group
	NamePath      string    `json:"name_path"`      // Filesystem path to namespace bind mount
	CreatedAt     time.Time `json:"created_at"`     // Timestamp when group was created
	NetworkConfig *Config   `json:"network_config"` // Network settings (subnet, gateway, interface)
}

// GroupInfo contains information returned when handling network group operations.
// This structure provides all the details needed by the job launcher to
// properly configure a process for network group membership.
type GroupInfo struct {
	NamePath           string               // Path to namespace file (for setns operations)
	SysProcAttr        *syscall.SysProcAttr // Process attributes for namespace creation/joining
	IsNewGroup         bool                 // true if this group was just created
	NeedsNamespaceJoin bool                 // true if process should join existing namespace
	NetworkConfig      *Config              // Network configuration for the group
}

// Manager handles network group management and coordination.
//
// The Manager implements a sophisticated network isolation strategy:
//
//  1. **Group Lifecycle Management**: Creates groups on-demand, tracks job counts,
//     and automatically cleans up empty groups.
//
//  2. **Namespace Coordination**: Decides between creating new namespaces or
//     joining existing ones based on group membership.
//
//  3. **Subnet Management**: Ensures each group gets a unique subnet to prevent
//     IP conflicts between different job groups.
//
//  4. **Process Integration**: Provides the correct system call attributes for
//     process creation based on networking requirements.
//
// Thread Safety: All operations are thread-safe using read-write mutexes.
// The manager supports concurrent access from multiple job workers.
//
// Implements NamespaceCleaner interface for process cleanup integration.
type Manager struct {
	groups          map[string]*Group            // Maps groupID -> Group (protected by mutex)
	mutex           sync.RWMutex                 // Protects concurrent access to groups map
	subnetAllocator *SubnetAllocator             // Manages dynamic subnet allocation
	namespaceOps    *NamespaceOperations         // Handles low-level namespace operations
	syscall         osinterface.SyscallInterface // System call interface (for testing)
	osInterface     osinterface.OsInterface      // OS interface (for testing)
	paths           *NetworkPaths                // Configured filesystem paths
	logger          *logger.Logger               // Structured logger for debugging
}

// Dependencies holds all dependencies needed by the network manager.
// This structure supports dependency injection for testing and configuration.
type Dependencies struct {
	SubnetAllocator *SubnetAllocator             // For dynamic subnet allocation
	NamespaceOps    *NamespaceOperations         // For namespace operations
	Syscall         osinterface.SyscallInterface // System call interface
	OsInterface     osinterface.OsInterface      // OS operations interface
	Paths           *NetworkPaths                // Network filesystem paths
}

// NewManager creates a new network manager with the specified dependencies.
// This factory function sets up all the required components and returns
// a fully configured manager ready for use.
//
// Parameters:
//   - deps: All required dependencies (uses defaults for nil values)
//
// Returns: Configured Manager instance
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

// RemoveNamespace removes a namespace file or symlink.
// This method implements the NamespaceCleaner interface, allowing the
// network manager to be used by process cleanup operations.
//
// The method delegates to the underlying NamespaceOperations for the
// actual removal, but provides logging and error context at the manager level.
//
// Parameters:
//   - nsPath: Path to namespace reference to remove
//   - isBound: true for bind mounts, false for symlinks
//
// Returns: nil on success, error with context on failure
func (m *Manager) RemoveNamespace(nsPath string, isBound bool) error {
	if m.namespaceOps == nil {
		return fmt.Errorf("namespace operations not available")
	}

	log := m.logger.WithFields("nsPath", nsPath, "isBound", isBound)
	log.Debug("removing namespace via manager")

	return m.namespaceOps.RemoveNamespace(nsPath, isBound)
}

// HandleNetworkGroup handles network group creation or joining operations.
// This is the main entry point for network group management. It implements
// the following decision logic:
//
// 1. **Existing Group**: If groupID exists, join the existing group
//   - Verify namespace file still exists (auto-repair if missing)
//   - Return GroupInfo configured for namespace joining
//
// 2. **New Group**: If groupID doesn't exist, create a new group
//   - Allocate unique subnet configuration
//   - Set up for new namespace creation
//   - Return GroupInfo configured for namespace creation
//
// The method is idempotent - calling it multiple times with the same
// groupID will return consistent results.
//
// Parameters:
//   - groupID: Unique identifier for the network group
//   - jobID: Job requesting group membership (for logging/tracking)
//
// Returns: GroupInfo with all details needed for process configuration
func (m *Manager) HandleNetworkGroup(groupID, jobID string) (*GroupInfo, error) {
	if groupID == "" {
		return nil, fmt.Errorf("groupID cannot be empty")
	}
	if jobID == "" {
		return nil, fmt.Errorf("jobID cannot be empty")
	}

	log := m.logger.WithFields("groupID", groupID, "jobID", jobID)
	log.Debug("handling network group request")

	// Use exclusive lock since we might modify the groups map
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Check if group already exists
	if group, exists := m.groups[groupID]; exists {
		return m.joinExistingGroup(group, jobID)
	}

	return m.createNewGroup(groupID, jobID)
}

// joinExistingGroup handles the process of joining an existing network group.
// This method:
//
// 1. **Validates Group State**: Ensures the group's namespace still exists
// 2. **Auto-Repair**: If namespace is missing, recreates the group
// 3. **Configure Process Attributes**: Sets up syscall attributes for namespace joining
//
// The key difference from new group creation is that this configures the
// process to join an existing namespace via setns() rather than creating
// a new one via clone() with CLONE_NEWNET.
//
// Parameters:
//   - group: Existing group to join
//   - jobID: Job requesting membership
//
// Returns: GroupInfo configured for joining existing namespace
func (m *Manager) joinExistingGroup(group *Group, jobID string) (*GroupInfo, error) {
	log := m.logger.WithFields("groupID", group.GroupID, "jobID", jobID)

	log.Info("joining existing network group",
		"currentJobs", group.JobCount,
		"subnet", group.NetworkConfig.Subnet,
		"gateway", group.NetworkConfig.Gateway)

	// Verify the namespace file still exists
	// This can happen if the filesystem was cleaned up externally
	if _, err := m.osInterface.Stat(group.NamePath); err != nil {
		log.Warn("network group namespace file missing, recreating group",
			"path", group.NamePath,
			"error", err)

		// Remove the broken group and recreate it
		// This provides automatic recovery from filesystem issues
		delete(m.groups, group.GroupID)
		return m.createNewGroup(group.GroupID, jobID)
	}

	// Create sysProcAttr for joining existing namespace
	// Key difference: NO CLONE_NEWNET flag because we'll join existing namespace
	// using setns() before fork in the process launcher
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
// This method orchestrates the complete setup of a new network group:
//
// 1. **Network Configuration**: Gets or allocates unique subnet settings
// 2. **Namespace Path**: Builds the filesystem path for namespace persistence
// 3. **Validation**: Ensures all settings are valid and secure
// 4. **Group Creation**: Creates and stores the new group
// 5. **Process Configuration**: Sets up syscall attributes for namespace creation
//
// The new group will use a bind mount for namespace persistence, allowing
// multiple jobs to join the same network namespace over time.
//
// Parameters:
//   - groupID: Unique identifier for the new group
//   - jobID: Job requesting group creation
//
// Returns: GroupInfo configured for creating new namespace
func (m *Manager) createNewGroup(groupID, jobID string) (*GroupInfo, error) {
	log := m.logger.WithFields("groupID", groupID, "jobID", jobID)
	log.Info("creating new network group")

	// Get network configuration for this group
	// This may come from environment variables, dynamic allocation, or defaults
	networkConfig, err := m.getNetworkConfigForGroup(groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to get network config for group %s: %w", groupID, err)
	}

	// Validate network configuration for correctness
	// Ensures subnet/gateway are valid and consistent
	if err := networkConfig.Validate(); err != nil {
		return nil, fmt.Errorf("invalid network config for group %s: %w", groupID, err)
	}

	// Build namespace path for persistent storage
	// Use persistent=true for network groups (bind mounts)
	nsPath := m.namespaceOps.BuildNamespacePath(groupID, true)

	// Validate namespace path for security
	// Prevents path traversal and ensures path is in allowed directories
	if err := m.namespaceOps.ValidateNamespacePath(nsPath); err != nil {
		return nil, fmt.Errorf("invalid namespace path: %w", err)
	}

	// Create sysProcAttr with new network namespace
	// This configures the process to create all required namespaces
	sysProcAttr := m.createSysProcAttrWithNamespaces()

	// Create the group structure
	group := &Group{
		GroupID:       groupID,
		JobCount:      0, // Will be incremented when jobs are added
		NamePath:      nsPath,
		CreatedAt:     time.Now(),
		NetworkConfig: networkConfig.Clone(), // Store a copy to prevent external modification
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
// This method implements a three-tier configuration strategy:
//
// 1. **Environment Override**: Check for group-specific environment variables
//   - NETWORK_GROUP_<GROUPID>_SUBNET
//   - NETWORK_GROUP_<GROUPID>_GATEWAY
//
// 2. **Dynamic Allocation**: Use SubnetAllocator to get unique subnet
//   - Prevents conflicts between groups
//   - Automatic subnet management
//
// 3. **Default Configuration**: Fall back to system defaults
//   - Used when allocation fails or allocator unavailable
//
// This hierarchy allows for flexible configuration while providing sensible defaults.
//
// Parameters:
//   - groupID: Network group needing configuration
//
// Returns: Valid network configuration or error
func (m *Manager) getNetworkConfigForGroup(groupID string) (*Config, error) {
	log := m.logger.WithField("groupID", groupID)

	// Option 1: Check for environment variable override
	// This allows manual configuration of specific groups
	envSubnetKey := fmt.Sprintf("NETWORK_GROUP_%s_SUBNET", strings.ToUpper(groupID))
	envGatewayKey := fmt.Sprintf("NETWORK_GROUP_%s_GATEWAY", strings.ToUpper(groupID))

	if customSubnet := m.osInterface.Getenv(envSubnetKey); customSubnet != "" {
		log.Debug("using custom subnet from environment", "subnet", customSubnet)

		gateway := m.osInterface.Getenv(envGatewayKey)
		if gateway == "" {
			// Calculate default gateway (.1) for the subnet
			if _, ipNet, err := net.ParseCIDR(customSubnet); err == nil {
				ip := ipNet.IP.Mask(ipNet.Mask)
				ip[len(ip)-1] = 1 // Set last octet to 1 (standard gateway convention)
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

		// Validate the custom configuration
		if err := config.Validate(); err != nil {
			return nil, fmt.Errorf("invalid custom network config: %w", err)
		}

		return config, nil
	}

	// Option 2: Dynamic allocation (prevents conflicts between groups)
	// This is the preferred method for automatic network management
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
	// Fall back when other methods fail or are unavailable
	config := NewDefaultConfig()
	log.Debug("using default network configuration",
		"subnet", config.Subnet,
		"gateway", config.Gateway)

	return config, nil
}

// IncrementJobCount increments the job count for a network group.
// This method is called when a job successfully joins a network group.
// The job count is used to determine when groups can be safely cleaned up.
//
// Uses atomic operations for thread-safe counter manipulation without
// requiring locks for this specific operation.
//
// Parameters:
//   - groupID: Network group to increment
//
// Returns: nil on success, error if group not found
func (m *Manager) IncrementJobCount(groupID string) error {
	if groupID == "" {
		return fmt.Errorf("groupID cannot be empty")
	}

	// Use exclusive lock to safely access groups map
	m.mutex.Lock()
	defer m.mutex.Unlock()

	group, exists := m.groups[groupID]
	if !exists {
		return fmt.Errorf("network group not found: %s", groupID)
	}

	// Use atomic increment for thread-safe counter update
	newCount := atomic.AddInt32(&group.JobCount, 1)
	m.logger.Debug("incremented job count for network group",
		"groupID", groupID,
		"newCount", newCount)

	return nil
}

// DecrementJobCount decrements the job count for a network group and cleans up if empty.
// This method is called when a job leaves a network group (completion, failure, or stop).
//
// The method implements automatic cleanup: when the job count reaches zero,
// the group is automatically cleaned up to free resources:
// - Subnet allocation is released
// - Namespace bind mount is removed
// - Group is removed from active groups map
//
// Parameters:
//   - groupID: Network group to decrement
//
// Returns: nil on success, error if cleanup fails
func (m *Manager) DecrementJobCount(groupID string) error {
	if groupID == "" {
		return fmt.Errorf("groupID cannot be empty")
	}

	// Use exclusive lock for safe map access and potential cleanup
	m.mutex.Lock()
	defer m.mutex.Unlock()

	group, exists := m.groups[groupID]
	if !exists {
		m.logger.Warn("attempted to decrement job count for non-existent group", "groupID", groupID)
		return nil // Don't error - group might have been cleaned up already
	}

	// Use atomic decrement for thread-safe counter update
	newCount := atomic.AddInt32(&group.JobCount, -1)
	m.logger.Debug("decremented job count for network group",
		"groupID", groupID,
		"newCount", newCount)

	// Cleanup empty group automatically
	if newCount <= 0 {
		return m.cleanupEmptyGroup(groupID, group)
	}

	return nil
}

// cleanupEmptyGroup cleans up a network group that has no more jobs.
// This method performs comprehensive cleanup of all group resources:
//
// 1. **Subnet Release**: Returns allocated subnet to pool for reuse
// 2. **Namespace Cleanup**: Unmounts bind mount and removes file
// 3. **Group Removal**: Removes group from active groups map
//
// This cleanup is automatic and ensures that system resources are properly
// freed when groups are no longer needed.
//
// Parameters:
//   - groupID: ID of group to clean up
//   - group: Group structure to clean up
//
// Returns: nil on success, error if cleanup fails (group still removed)
func (m *Manager) cleanupEmptyGroup(groupID string, group *Group) error {
	log := m.logger.WithField("groupID", groupID)
	log.Info("cleaning up empty network group")

	// Release subnet allocation back to the pool
	// This allows the subnet to be reused by future groups
	if m.subnetAllocator != nil {
		m.subnetAllocator.ReleaseSubnet(groupID)
	}

	// Remove namespace bind mount
	// Network groups use bind mounts (persistent) rather than symlinks (temporary)
	isBound := true
	if err := m.namespaceOps.RemoveNamespace(group.NamePath, isBound); err != nil {
		log.Warn("failed to remove namespace", "path", group.NamePath, "error", err)
		// Continue with cleanup even if namespace removal fails
		// The group should still be removed from memory to prevent leaks
	}

	// Remove from groups map
	// This must be done last to ensure other operations can still find the group
	delete(m.groups, groupID)

	log.Info("network group cleaned up successfully",
		"subnet", group.NetworkConfig.Subnet,
		"namespacePath", group.NamePath)

	return nil
}

// GetGroup returns information about a network group.
// This method provides safe read-only access to group information.
// The returned group is a copy to prevent external modification of internal state.
//
// The job count is loaded atomically to ensure consistency even during
// concurrent job additions/removals.
//
// Parameters:
//   - groupID: Group to retrieve information for
//
// Returns: (Group copy, true) if found, (nil, false) if not found
func (m *Manager) GetGroup(groupID string) (*Group, bool) {
	if groupID == "" {
		return nil, false
	}

	// Use read lock for efficient concurrent access
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	group, exists := m.groups[groupID]
	if !exists {
		return nil, false
	}

	// Return a copy to prevent external modification
	// This is defensive programming to protect internal state
	groupCopy := &Group{
		GroupID:       group.GroupID,
		JobCount:      atomic.LoadInt32(&group.JobCount), // Atomic load for consistency
		NamePath:      group.NamePath,
		CreatedAt:     group.CreatedAt,
		NetworkConfig: group.NetworkConfig.Clone(), // Deep copy network config
	}

	return groupCopy, true
}

// ListGroups returns a list of all network groups.
// This method provides a snapshot of all active groups for monitoring,
// debugging, and administrative purposes.
//
// All returned groups are copies to prevent external modification.
// Job counts are loaded atomically for consistency.
//
// Returns: Slice of Group copies for all active groups
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

// GetStats returns statistics about network groups for monitoring and capacity planning.
// This method provides insights into:
// - Total number of groups (active and empty)
// - Number of groups with active jobs
// - Total jobs across all groups
// - Subnet allocation statistics
//
// Returns: Map of statistics suitable for monitoring systems
func (m *Manager) GetStats() map[string]interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var totalJobs int32
	activeGroups := 0

	// Calculate aggregate statistics across all groups
	for _, group := range m.groups {
		jobCount := atomic.LoadInt32(&group.JobCount)
		totalJobs += jobCount
		if jobCount > 0 {
			activeGroups++
		}
	}

	stats := map[string]interface{}{
		"total_groups":  len(m.groups), // All groups (including empty ones)
		"active_groups": activeGroups,  // Groups with active jobs
		"total_jobs":    totalJobs,     // Total jobs across all groups
	}

	// Add subnet allocator stats if available
	if m.subnetAllocator != nil {
		subnetStats := m.subnetAllocator.GetStats()
		for k, v := range subnetStats {
			stats["subnet_"+k] = v // Prefix to avoid key conflicts
		}
	}

	return stats
}

// CleanupGroup forcefully cleans up a network group.
// This method is used for emergency cleanup when normal cleanup fails
// or when administrative intervention is needed.
//
// Unlike normal cleanup, this method:
// - Forces job count to zero regardless of actual job state
// - Proceeds with cleanup even if jobs might still be running
// - Used during system shutdown or error recovery
//
// Parameters:
//   - groupID: Group to forcefully clean up
//
// Returns: nil if cleanup succeeded or group didn't exist
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

	// Reset job count to zero and cleanup
	// This bypasses normal job counting and forces cleanup
	atomic.StoreInt32(&group.JobCount, 0)
	return m.cleanupEmptyGroup(groupID, group)
}

// ValidateGroupExists checks if a network group exists and is valid.
// This method performs comprehensive validation:
// 1. Group exists in memory
// 2. Namespace file exists on filesystem
//
// This is useful for preflight checks before attempting to join a group.
//
// Parameters:
//   - groupID: Group to validate
//
// Returns: nil if group exists and is valid, descriptive error otherwise
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
	// This catches cases where external cleanup removed the namespace
	if _, err := m.osInterface.Stat(group.NamePath); err != nil {
		return fmt.Errorf("network group namespace missing: %s (%w)", group.NamePath, err)
	}

	return nil
}

// createSysProcAttrWithNamespaces creates syscall attributes with all required namespaces.
// This method configures process creation for full namespace isolation:
//
// - PID namespace: Process sees isolated process tree
// - Mount namespace: Process has isolated filesystem view
// - IPC namespace: Process has isolated inter-process communication
// - UTS namespace: Process has isolated hostname/domain
// - Network namespace: Process has isolated network stack
//
// This provides comprehensive isolation for job processes.
//
// Returns: Configured syscall attributes for namespace creation
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
// This method builds the environment variables that will be passed to job-init
// to configure network settings within the namespace.
//
// Environment variables provided:
// - NETWORK_GROUP_ID: Group identifier
// - IS_NEW_NETWORK_GROUP: Whether this is a new group (affects setup behavior)
// - INTERNAL_SUBNET: Subnet CIDR for internal networking
// - INTERNAL_GATEWAY: Gateway IP for internal networking
// - INTERNAL_INTERFACE: Interface name for internal networking
//
// Parameters:
//   - groupID: Network group identifier
//   - isNewGroup: Whether this is a new group requiring network setup
//
// Returns: Slice of environment variable strings
func (m *Manager) PrepareEnvironment(groupID string, isNewGroup bool) []string {
	var envVars []string

	if groupID != "" {
		envVars = append(envVars,
			fmt.Sprintf("NETWORK_GROUP_ID=%s", groupID),
			fmt.Sprintf("IS_NEW_NETWORK_GROUP=%t", isNewGroup))

		// Add network configuration if group exists
		// Use read lock for safe concurrent access
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
// This method performs comprehensive cleanup of all managed resources:
//
// 1. **Reset Job Counts**: Forces all groups to have zero jobs
// 2. **Release Subnets**: Returns all allocated subnets to pool
// 3. **Remove Namespaces**: Unmounts all bind mounts and removes files
// 4. **Clear State**: Removes all groups from memory
//
// This method is called during system shutdown to ensure clean resource cleanup.
//
// Returns: nil on complete success, error describing any cleanup failures
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

// GetNamespaceOperations returns the namespace operations instance.
// This method provides access to the underlying namespace operations
// for advanced use cases or direct namespace management.
//
// Returns: NamespaceOperations instance used by this manager
func (m *Manager) GetNamespaceOperations() *NamespaceOperations {
	return m.namespaceOps
}

// CreateNamespaceForJob creates a namespace artifact for a specific job.
// This method provides a job-focused interface for namespace creation,
// automatically choosing between symlinks and bind mounts based on the
// namespace type.
//
// The method handles:
// - Network groups: Creates persistent bind mounts
// - Isolated jobs: Creates temporary symlinks
//
// Parameters:
//   - jobID: Job requesting namespace creation
//   - pid: Process ID to create namespace reference for
//   - nsPath: Filesystem path for namespace reference
//   - isNetworkGroup: true for persistent (bind mount), false for temporary (symlink)
//
// Returns: nil on success, error with context on failure
func (m *Manager) CreateNamespaceForJob(jobID string, pid int32, nsPath string, isNetworkGroup bool) error {
	if m.namespaceOps == nil {
		return fmt.Errorf("namespace operations not available")
	}

	log := m.logger.WithFields("jobID", jobID, "pid", pid, "nsPath", nsPath, "isNetworkGroup", isNetworkGroup)
	log.Debug("creating namespace for job")

	if isNetworkGroup {
		// Create bind mount for persistent network groups
		// This allows the namespace to persist beyond the original process lifetime
		return m.namespaceOps.CreateNamespaceBindMount(pid, nsPath)
	} else {
		// Create symlink for isolated jobs
		// This provides temporary namespace access that cleans up automatically
		return m.namespaceOps.CreateNamespaceSymlink(pid, nsPath)
	}
}
