package state

import (
	"joblet/internal/joblet/network"
	"time"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

// BridgeManager interface to avoid circular import with setup.go.
// Provides bridge network infrastructure management capabilities.
//
//counterfeiter:generate . BridgeManager
type BridgeManager interface {
	// RemoveBridge removes a network bridge by name.
	RemoveBridge(networkName string) error
}

// NetworkValidator provides network configuration validation.
// Ensures network configurations are valid and don't conflict with existing setups.
//
//counterfeiter:generate . NetworkValidator
type NetworkValidator interface {
	// ValidateBridgeName ensures bridge name follows naming rules.
	ValidateBridgeName(name string) error
	// ValidateCIDR ensures CIDR doesn't conflict with existing networks.
	ValidateCIDR(cidr string, existingNetworks map[string]string) error
}

// NetworkCleaner provides automatic cleanup of orphaned network resources.
// Handles cleanup of network interfaces and resources left behind by failed jobs.
//
//counterfeiter:generate . NetworkCleaner
type NetworkCleaner interface {
	// StartPeriodicCleanup starts automatic cleanup of orphaned interfaces.
	StartPeriodicCleanup(interval time.Duration)
	// CleanupOrphanedInterfaces removes abandoned network interfaces.
	CleanupOrphanedInterfaces() error
}

// DNSManager handles DNS configuration for job networking.
// Provides DNS resolution and hostname management within job networks.
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

// BandwidthLimiter provides network traffic shaping and QoS capabilities.
// Allows applying bandwidth restrictions to job network interfaces.
//
//counterfeiter:generate . BandwidthLimiter
type BandwidthLimiter interface {
	// ApplyJobLimits applies bandwidth restrictions to a job's network interface.
	ApplyJobLimits(vethName string, limits network.NetworkLimits) error
	// RemoveJobLimits removes bandwidth restrictions from a network interface.
	RemoveJobLimits(vethName string) error
}
