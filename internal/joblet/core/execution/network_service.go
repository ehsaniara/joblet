package execution

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/ehsaniara/joblet/internal/joblet/network"
	"github.com/ehsaniara/joblet/pkg/logger"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

// NetworkService handles job networking setup and management
type NetworkService struct {
	networkSetup *network.NetworkSetup
	networkStore NetworkStoreInterface
	logger       *logger.Logger
}

// NetworkStoreInterface defines the contract for network storage operations
//
//counterfeiter:generate . NetworkStoreInterface
type NetworkStoreInterface interface {
	AllocateIP(networkName string) (string, error)
	ReleaseIP(networkName, ipAddress string) error
	AssignJobToNetwork(jobID, networkName string, allocation *JobNetworkAllocation) error
	RemoveJobFromNetwork(jobID string) error
	GetJobAllocation(jobID string) (*JobNetworkAllocation, error)
}

// JobNetworkAllocation represents a job's network allocation
type JobNetworkAllocation struct {
	JobID       string
	NetworkName string
	IPAddress   string
	Hostname    string
	AssignedAt  int64
}

// NewNetworkService creates a new network service
func NewNetworkService(
	networkSetup *network.NetworkSetup,
	networkStore NetworkStoreInterface,
	logger *logger.Logger,
) *NetworkService {
	return &NetworkService{
		networkSetup: networkSetup,
		networkStore: networkStore,
		logger:       logger.WithField("component", "network-service"),
	}
}

// SetupNetworking allocates network resources for a job (phase 1 - before process launch)
func (ns *NetworkService) SetupNetworking(ctx context.Context, jobID, networkName string) (*NetworkAllocation, error) {
	log := ns.logger.WithField("jobID", jobID).WithField("network", networkName)
	log.Debug("allocating network resources for job")

	// Handle isolated network case
	if networkName == "isolated" {
		return &NetworkAllocation{
			JobID:   jobID,
			Network: "isolated",
		}, nil
	}

	// Handle none network case
	if networkName == "none" {
		return &NetworkAllocation{
			JobID:   jobID,
			Network: "none",
		}, nil
	}

	// Allocate IP from store
	ipAddress, err := ns.networkStore.AllocateIP(networkName)
	if err != nil {
		return nil, fmt.Errorf("failed to allocate IP: %w", err)
	}

	// Create network allocation record
	hostname := fmt.Sprintf("job_%s", jobID[:8])
	allocation := &JobNetworkAllocation{
		JobID:       jobID,
		NetworkName: networkName,
		IPAddress:   ipAddress,
		Hostname:    hostname,
		AssignedAt:  time.Now().Unix(),
	}

	// Assign job to network
	if err := ns.networkStore.AssignJobToNetwork(jobID, networkName, allocation); err != nil {
		// Release IP on error
		if releaseErr := ns.networkStore.ReleaseIP(networkName, ipAddress); releaseErr != nil {
			log.Warn("failed to release IP during cleanup", "ip", ipAddress, "error", releaseErr)
		}
		return nil, fmt.Errorf("failed to assign job to network: %w", err)
	}

	// Convert to execution layer format
	// Note: Actual namespace setup will happen in ConfigureNetworkNamespace after process launch
	netAlloc := &NetworkAllocation{
		JobID:    jobID,
		Network:  networkName,
		IP:       ipAddress,
		Hostname: hostname,
	}

	log.Info("network resources allocated", "ip", ipAddress, "hostname", hostname)
	return netAlloc, nil
}

// ConfigureNetworkNamespace sets up the network namespace for a job (phase 2 - after process launch)
func (ns *NetworkService) ConfigureNetworkNamespace(ctx context.Context, jobID string, pid int) error {
	log := ns.logger.WithField("jobID", jobID).WithField("pid", pid)
	log.Debug("configuring network namespace for job")

	// Get the allocation info from store
	allocInfo, err := ns.networkStore.GetJobAllocation(jobID)
	if err != nil {
		return fmt.Errorf("failed to get job allocation: %w", err)
	}

	// Handle special network types
	if allocInfo.NetworkName == "none" || allocInfo.NetworkName == "isolated" {
		log.Debug("skipping namespace setup for special network type", "network", allocInfo.NetworkName)
		return nil
	}

	// Create the network.JobAllocation structure for the setup
	netAllocation := &network.JobAllocation{
		JobID:    jobID,
		Network:  allocInfo.NetworkName,
		IP:       net.ParseIP(allocInfo.IPAddress),
		Hostname: allocInfo.Hostname,
		VethHost: fmt.Sprintf("veth-h-%s", jobID[:8]),
		VethPeer: fmt.Sprintf("veth-p-%s", jobID[:8]),
	}

	// Call the actual network namespace setup with the real PID
	log.Info("configuring network interfaces in namespace", "ip", allocInfo.IPAddress, "hostname", allocInfo.Hostname)
	if err := ns.networkSetup.SetupJobNetwork(netAllocation, pid); err != nil {
		return fmt.Errorf("failed to setup network namespace: %w", err)
	}

	log.Info("network namespace configured successfully")
	return nil
}

// CleanupNetworking cleans up networking for a job
func (ns *NetworkService) CleanupNetworking(ctx context.Context, jobID string) error {
	log := ns.logger.WithField("jobID", jobID)
	log.Debug("cleaning up job networking")

	// Remove job from network store
	if err := ns.networkStore.RemoveJobFromNetwork(jobID); err != nil {
		log.Warn("failed to remove job from network", "error", err)
		return err
	}

	log.Debug("job networking cleanup completed")
	return nil
}
