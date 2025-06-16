package network

import (
	"fmt"
	"net"
)

// Config holds network configuration settings
type Config struct {
	Subnet    string `json:"subnet"`    // e.g., "172.20.0.0/24"
	Gateway   string `json:"gateway"`   // e.g., "172.20.0.1"
	Interface string `json:"interface"` // e.g., "internal0"
}

// NetworkPaths holds filesystem paths for network namespaces
type NetworkPaths struct {
	NetnsPath   string // e.g., "/tmp/shared-netns/"
	VarRunNetns string // e.g., "/var/run/netns/"
}

// Constants for default network configuration
const (
	DefaultSubnet    = "172.20.0.0/24"
	DefaultGateway   = "172.20.0.1"
	DefaultInterface = "internal0"
	BaseNetwork      = "172.20.0.0/16"

	// Default paths
	DefaultNetnsPath   = "/tmp/shared-netns/"
	DefaultVarRunNetns = "/var/run/netns/"
)

// NewDefaultConfig creates a default network configuration
func NewDefaultConfig() *Config {
	return &Config{
		Subnet:    DefaultSubnet,
		Gateway:   DefaultGateway,
		Interface: DefaultInterface,
	}
}

// NewDefaultPaths creates default network paths
func NewDefaultPaths() *NetworkPaths {
	return &NetworkPaths{
		NetnsPath:   DefaultNetnsPath,
		VarRunNetns: DefaultVarRunNetns,
	}
}

// Validate validates the network configuration
func (c *Config) Validate() error {
	if c == nil {
		return fmt.Errorf("network config cannot be nil")
	}

	// Validate subnet
	_, ipNet, err := net.ParseCIDR(c.Subnet)
	if err != nil {
		return fmt.Errorf("invalid subnet %s: %w", c.Subnet, err)
	}

	// Validate gateway IP
	gatewayIP := net.ParseIP(c.Gateway)
	if gatewayIP == nil {
		return fmt.Errorf("invalid gateway IP %s", c.Gateway)
	}

	// Check if gateway is within subnet
	if !ipNet.Contains(gatewayIP) {
		return fmt.Errorf("gateway IP %s is not within subnet %s", c.Gateway, c.Subnet)
	}

	// Validate interface name
	if c.Interface == "" {
		return fmt.Errorf("interface name cannot be empty")
	}

	return nil
}

// Clone creates a deep copy of the network configuration
func (c *Config) Clone() *Config {
	if c == nil {
		return nil
	}

	return &Config{
		Subnet:    c.Subnet,
		Gateway:   c.Gateway,
		Interface: c.Interface,
	}
}

// String returns a string representation of the config
func (c *Config) String() string {
	if c == nil {
		return "nil"
	}
	return fmt.Sprintf("Config{Subnet: %s, Gateway: %s, Interface: %s}",
		c.Subnet, c.Gateway, c.Interface)
}
