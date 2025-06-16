package network

import (
	"fmt"
	"net"
	"sync"

	"job-worker/pkg/logger"
)

// SubnetAllocator manages dynamic subnet allocation to avoid conflicts
type SubnetAllocator struct {
	baseNetwork      string
	allocatedSubnets map[string]string // subnet -> groupID mapping
	mutex            sync.RWMutex
	logger           *logger.Logger
}

// AllocationInfo contains information about an allocated subnet
type AllocationInfo struct {
	GroupID string
	Subnet  string
	Gateway string
	InUse   bool
}

// NewSubnetAllocator creates a new subnet allocator
func NewSubnetAllocator(baseNetwork string) *SubnetAllocator {
	if baseNetwork == "" {
		baseNetwork = BaseNetwork
	}

	return &SubnetAllocator{
		baseNetwork:      baseNetwork,
		allocatedSubnets: make(map[string]string),
		logger:           logger.New().WithField("component", "subnet-allocator"),
	}
}

// AllocateSubnet allocates a unique subnet for a network group
func (sa *SubnetAllocator) AllocateSubnet(groupID string) (*Config, error) {
	if groupID == "" {
		return nil, fmt.Errorf("groupID cannot be empty")
	}

	sa.mutex.Lock()
	defer sa.mutex.Unlock()

	sa.logger.Debug("allocating subnet", "groupID", groupID, "baseNetwork", sa.baseNetwork)

	// Check if group already has an allocated subnet
	for subnet, existingGroupID := range sa.allocatedSubnets {
		if existingGroupID == groupID {
			gateway := sa.calculateGatewayFromSubnet(subnet)

			config := &Config{
				Subnet:    subnet,
				Gateway:   gateway,
				Interface: DefaultInterface,
			}

			sa.logger.Debug("found existing subnet allocation",
				"groupID", groupID,
				"subnet", subnet,
				"gateway", gateway)

			return config, nil
		}
	}

	// Allocate new subnet using simple increment strategy
	// 172.20.0.0/24, 172.20.1.0/24, 172.20.2.0/24, etc.
	for i := 0; i < 256; i++ {
		subnet := fmt.Sprintf("172.20.%d.0/24", i)
		gateway := fmt.Sprintf("172.20.%d.1", i)

		if _, exists := sa.allocatedSubnets[subnet]; !exists {
			// Validate the generated subnet
			if err := sa.validateSubnetFormat(subnet); err != nil {
				sa.logger.Warn("generated invalid subnet, skipping",
					"subnet", subnet,
					"error", err)
				continue
			}

			sa.allocatedSubnets[subnet] = groupID

			config := &Config{
				Subnet:    subnet,
				Gateway:   gateway,
				Interface: DefaultInterface,
			}

			sa.logger.Info("allocated new subnet",
				"groupID", groupID,
				"subnet", subnet,
				"gateway", gateway,
				"totalAllocated", len(sa.allocatedSubnets))

			return config, nil
		}
	}

	sa.logger.Error("no available subnets in range",
		"baseNetwork", sa.baseNetwork,
		"totalAllocated", len(sa.allocatedSubnets))

	return nil, fmt.Errorf("no available subnets in range %s", sa.baseNetwork)
}

// ReleaseSubnet releases a subnet allocation
func (sa *SubnetAllocator) ReleaseSubnet(groupID string) {
	if groupID == "" {
		sa.logger.Warn("attempted to release subnet with empty groupID")
		return
	}

	sa.mutex.Lock()
	defer sa.mutex.Unlock()

	var releasedSubnet string
	for subnet, existingGroupID := range sa.allocatedSubnets {
		if existingGroupID == groupID {
			delete(sa.allocatedSubnets, subnet)
			releasedSubnet = subnet
			break
		}
	}

	if releasedSubnet != "" {
		sa.logger.Info("released subnet allocation",
			"groupID", groupID,
			"subnet", releasedSubnet,
			"remainingAllocated", len(sa.allocatedSubnets))
	} else {
		sa.logger.Warn("no subnet allocation found to release",
			"groupID", groupID)
	}
}

// GetAllocation returns the current subnet allocation for a group
func (sa *SubnetAllocator) GetAllocation(groupID string) (*Config, bool) {
	if groupID == "" {
		return nil, false
	}

	sa.mutex.RLock()
	defer sa.mutex.RUnlock()

	for subnet, existingGroupID := range sa.allocatedSubnets {
		if existingGroupID == groupID {
			gateway := sa.calculateGatewayFromSubnet(subnet)

			config := &Config{
				Subnet:    subnet,
				Gateway:   gateway,
				Interface: DefaultInterface,
			}

			return config, true
		}
	}

	return nil, false
}

// ListAllocations returns all current subnet allocations
func (sa *SubnetAllocator) ListAllocations() []AllocationInfo {
	sa.mutex.RLock()
	defer sa.mutex.RUnlock()

	allocations := make([]AllocationInfo, 0, len(sa.allocatedSubnets))

	for subnet, groupID := range sa.allocatedSubnets {
		gateway := sa.calculateGatewayFromSubnet(subnet)

		allocation := AllocationInfo{
			GroupID: groupID,
			Subnet:  subnet,
			Gateway: gateway,
			InUse:   true,
		}

		allocations = append(allocations, allocation)
	}

	return allocations
}

// GetStats returns allocation statistics
func (sa *SubnetAllocator) GetStats() map[string]interface{} {
	sa.mutex.RLock()
	defer sa.mutex.RUnlock()

	return map[string]interface{}{
		"total_allocated": len(sa.allocatedSubnets),
		"base_network":    sa.baseNetwork,
		"max_subnets":     256, // Based on our /24 allocation strategy
		"available":       256 - len(sa.allocatedSubnets),
	}
}

// calculateGatewayFromSubnet calculates the gateway IP (.1) for a given subnet
func (sa *SubnetAllocator) calculateGatewayFromSubnet(subnet string) string {
	_, ipNet, err := net.ParseCIDR(subnet)
	if err != nil {
		sa.logger.Warn("failed to parse subnet for gateway calculation",
			"subnet", subnet,
			"error", err)
		return ""
	}

	return sa.calculateGateway(ipNet)
}

// calculateGateway calculates the gateway IP (.1) for a given network
func (sa *SubnetAllocator) calculateGateway(ipNet *net.IPNet) string {
	ip := ipNet.IP.Mask(ipNet.Mask)
	ip[len(ip)-1] = 1 // Set last octet to 1
	return ip.String()
}

// validateSubnetFormat validates that a subnet string is properly formatted
func (sa *SubnetAllocator) validateSubnetFormat(subnet string) error {
	_, _, err := net.ParseCIDR(subnet)
	if err != nil {
		return fmt.Errorf("invalid subnet format %s: %w", subnet, err)
	}
	return nil
}

// IsSubnetAllocated checks if a specific subnet is already allocated
func (sa *SubnetAllocator) IsSubnetAllocated(subnet string) bool {
	sa.mutex.RLock()
	defer sa.mutex.RUnlock()

	_, allocated := sa.allocatedSubnets[subnet]
	return allocated
}

// GetGroupForSubnet returns the group ID that owns a specific subnet
func (sa *SubnetAllocator) GetGroupForSubnet(subnet string) (string, bool) {
	sa.mutex.RLock()
	defer sa.mutex.RUnlock()

	groupID, exists := sa.allocatedSubnets[subnet]
	return groupID, exists
}
