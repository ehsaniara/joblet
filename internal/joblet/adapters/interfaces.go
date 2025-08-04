package adapters

import (
	"context"
	"joblet/internal/joblet/domain"
	"joblet/internal/joblet/interfaces"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

// DomainStreamer defines the interface for streaming data to clients.
// Provides methods for sending log data and keepalive messages.
//
//counterfeiter:generate . DomainStreamer
type DomainStreamer interface {
	// SendData sends log data chunk to the client stream.
	SendData(data []byte) error
	// SendKeepalive sends a keepalive message to maintain connection.
	SendKeepalive() error
	// Context returns the streaming context for cancellation handling.
	Context() context.Context
}

// JobStoreAdapter provides job storage with buffer management and pub-sub capabilities.
//
//counterfeiter:generate . JobStoreAdapter
type JobStoreAdapter interface {
	// Core job management operations
	CreateNewJob(job *domain.Job)
	UpdateJob(job *domain.Job)
	GetJob(id string) (*domain.Job, bool)
	ListJobs() []*domain.Job
	WriteToBuffer(jobId string, chunk []byte)
	GetOutput(id string) ([]byte, bool, error)
	SendUpdatesToClient(ctx context.Context, id string, stream DomainStreamer) error

	// Lifecycle management
	Close() error
}

// VolumeStoreAdapter provides volume storage.
//
//counterfeiter:generate . VolumeStoreAdapter
type VolumeStoreAdapter interface {
	// Embed the standard VolumeStore interface
	interfaces.VolumeStore

	// Lifecycle management
	Close() error
}

// NetworkStoreAdapter provides network storage.
//
//counterfeiter:generate . NetworkStoreAdapter
type NetworkStoreAdapter interface {
	// Network configuration management
	CreateNetwork(config *NetworkConfig) error
	GetNetwork(name string) (*NetworkConfig, bool)
	ListNetworks() []*NetworkConfig
	RemoveNetwork(name string) error

	// Job network assignment
	AssignJobToNetwork(jobID, networkName string, allocation *JobNetworkAllocation) error
	GetJobNetworkAllocation(jobID string) (*JobNetworkAllocation, bool)
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
