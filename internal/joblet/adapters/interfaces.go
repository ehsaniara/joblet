package adapters

import (
	"context"
	"joblet/internal/joblet/domain"
	"joblet/internal/joblet/interfaces"
	"joblet/internal/joblet/network"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

// JobStoreAdapter provides job storage with buffer management and pub-sub capabilities.
//
//counterfeiter:generate . JobStorer
type JobStorer interface {
	// Core job management operations
	CreateNewJob(job *domain.Job)
	UpdateJob(job *domain.Job)
	Job(id string) (*domain.Job, bool)
	JobByPrefix(prefix string) (*domain.Job, bool)
	ListJobs() []*domain.Job
	WriteToBuffer(jobID string, chunk []byte)
	Output(id string) ([]byte, bool, error)
	SendUpdatesToClient(ctx context.Context, id string, stream interfaces.DomainStreamer) error

	// Log management
	DeleteJobLogs(jobID string) error

	// Job cleanup - complete job deletion including logs and metadata
	DeleteJob(jobID string) error

	// Lifecycle management
	Close() error
}

// VolumeStoreAdapter provides volume storage.
//
//counterfeiter:generate . VolumeStorer
type VolumeStorer interface {
	// Embed the standard VolumeStore interface
	interfaces.VolumeStore

	// Lifecycle management
	Close() error
}

// NetworkStoreAdapter provides network storage.
//
//counterfeiter:generate . NetworkStorer
type NetworkStorer interface {
	// Network configuration management
	CreateNetwork(config *NetworkConfig) error
	Network(name string) (*NetworkConfig, bool)
	NetworkConfig(name string) (*network.NetworkConfig, error)    // For network.NetworkSetup compatibility
	GetNetworkConfig(name string) (*network.NetworkConfig, error) // Deprecated: use NetworkConfig
	ListNetworks() []*NetworkConfig
	RemoveNetwork(name string) error

	// Job network assignment
	AssignJobToNetwork(jobID, networkName string, allocation *JobNetworkAllocation) error
	JobNetworkAllocation(jobID string) (*JobNetworkAllocation, bool)
	RemoveJobFromNetwork(jobID string) error
	ListJobsInNetwork(networkName string) []*JobNetworkAllocation

	// IP address management
	AllocateIP(networkName string) (string, error)
	ReleaseIP(networkName, ip string) error

	// Lifecycle management
	Close() error
}

// NetworkConfig represents a network configuration.
type NetworkConfig struct {
	Name       string            `json:"name"`
	Type       string            `json:"type"` // bridge, host, none, custom
	CIDR       string            `json:"cidr,omitempty"`
	BridgeName string            `json:"bridge_name,omitempty"`
	Gateway    string            `json:"gateway,omitempty"`
	DNS        []string          `json:"dns,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	CreatedAt  int64             `json:"created_at"`
	UpdatedAt  int64             `json:"updated_at"`
}

// JobNetworkAllocation represents a job's network assignment.
type JobNetworkAllocation struct {
	JobID       string            `json:"job_id"`
	NetworkName string            `json:"network_name"`
	IPAddress   string            `json:"ip_address,omitempty"`
	MACAddress  string            `json:"mac_address,omitempty"`
	Hostname    string            `json:"hostname,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	AssignedAt  int64             `json:"assigned_at"`
}
