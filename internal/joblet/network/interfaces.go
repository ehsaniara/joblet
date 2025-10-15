package network

import (
	"context"
	"net"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

// Manager defines the interface for network management operations
type Manager interface {
	// Core network operations
	CreateNetwork(name string, config *NetworkConfig) error
	DestroyNetwork(name string) error
	ListNetworks() ([]NetworkInfo, error)
	NetworkExists(name string) bool

	// IP allocation and management
	AllocateIP(networkName, jobID string) (*JobAllocation, error)
	ReleaseIP(jobID string) error
	GetAllocation(jobID string) (*JobAllocation, error)
	ListAllocations(networkName string) ([]JobAllocation, error)

	// Job network lifecycle
	SetupJobNetworking(jobID, networkName string) (*JobAllocation, error)
	CleanupJobNetworking(jobID string) error

	// Network configuration and validation
	ValidateNetworkConfig(config *NetworkConfig) error
	GetNetworkInfo(name string) (*NetworkInfo, error)

	// Network monitoring
	StartMonitoring(ctx context.Context) error
	StopMonitoring() error
	GetBandwidthStats(jobID string) (*BandwidthStats, error)
	GetNetworkStats(networkName string) (*BandwidthStats, error)
	SetBandwidthLimits(jobID string, limits *NetworkLimits) error
}

// Validator defines interface for network validation
// Kept minimal - only essential validation methods
type Validator interface {
	ValidateNetworkName(name string) error
	ValidateCIDR(cidr string) error
	ValidateNetworkConfig(config *NetworkConfig) error
	ValidateJobNetworking(jobID, networkName string) error
}

// Monitor defines interface for network monitoring
// Optional component - can be nil if monitoring not needed
type Monitor interface {
	StartMonitoring(ctx context.Context) error
	StopMonitoring() error
	GetBandwidthStats(jobID string) (*BandwidthStats, error)
	GetNetworkStats(networkName string) (*BandwidthStats, error)
	SetBandwidthLimits(jobID string, limits *NetworkLimits) error
}

// IPPool defines interface for IP address pool management
type IPPool interface {
	AllocateIP(networkName string) (net.IP, error)
	ReleaseIP(networkName string, ip net.IP) error
	IsIPAvailable(networkName string, ip net.IP) bool
	GetAvailableIPs(networkName string) ([]net.IP, error)
	GetAllocatedIPs(networkName string) ([]net.IP, error)
}

// Setup defines interface for network infrastructure operations
type Setup interface {
	CreateBridge(bridgeName, cidr string) error
	DeleteBridge(bridgeName string) error
	BridgeExists(bridgeName string) bool
	CreateVethPair(hostVeth, peerVeth string) error
	DeleteVethPair(hostVeth, peerVeth string) error
	AttachVethToBridge(bridgeName, vethName string) error
	SetupNamespace(jobID string, allocation *JobAllocation) error
	CleanupNamespace(jobID string) error
}

// DNS defines interface for DNS operations
type DNS interface {
	SetupDNS(jobID, hostname string, ip net.IP) error
	CleanupDNS(jobID string) error
}
