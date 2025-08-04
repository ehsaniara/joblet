package adapters

import (
	"fmt"
	"joblet/internal/joblet/network"
)

// NetworkSetupBridge adapts NetworkStoreAdapter to provide the NetworkStoreInterface
// required by network.NetworkSetup. This allows NetworkSetup to work with the new
// adapter pattern while maintaining its existing interface requirements.
type NetworkSetupBridge struct {
	adapter NetworkStoreAdapter
}

// NewNetworkSetupBridge creates a bridge that adapts NetworkStoreAdapter to NetworkStoreInterface
func NewNetworkSetupBridge(adapter NetworkStoreAdapter) network.NetworkStoreInterface {
	if adapter == nil {
		return nil
	}
	return &NetworkSetupBridge{
		adapter: adapter,
	}
}

// GetNetworkConfig implements network.NetworkStoreInterface by converting between
// adapter types and network types. It retrieves network configuration from the adapter
// and converts it to the format expected by NetworkSetup.
func (b *NetworkSetupBridge) GetNetworkConfig(name string) (*network.NetworkConfig, error) {
	// Get network from adapter
	adapterConfig, exists := b.adapter.GetNetwork(name)
	if !exists {
		return nil, fmt.Errorf("network not found: %s", name)
	}

	// Convert adapter NetworkConfig to network NetworkConfig
	networkConfig := &network.NetworkConfig{
		CIDR:   adapterConfig.CIDR,
		Bridge: adapterConfig.BridgeName,
	}

	return networkConfig, nil
}
