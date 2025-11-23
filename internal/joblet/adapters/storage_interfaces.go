package adapters

import (
	"context"

	"github.com/ehsaniara/joblet/internal/joblet/domain"
	"github.com/ehsaniara/joblet/internal/joblet/interfaces"
	"github.com/ehsaniara/joblet/internal/joblet/network"
	"github.com/ehsaniara/joblet/internal/joblet/pubsub"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

// JobStorer manages job storage, logs, and client notifications.
//
//counterfeiter:generate . JobStorer
type JobStorer interface {
	// Job CRUD operations
	CreateNewJob(job *domain.Job)
	UpdateJob(job *domain.Job)
	Job(id string) (*domain.Job, bool)
	JobByPrefix(prefix string) (*domain.Job, bool)
	ResolveJobUUID(idOrPrefix string) (string, error)
	ListJobs() []*domain.Job
	WriteToBuffer(jobID string, chunk []byte)
	Output(id string) ([]byte, bool, error)
	SendUpdatesToClient(ctx context.Context, id string, stream interfaces.DomainStreamer) error
	SendUpdatesToClientWithSkip(ctx context.Context, id string, stream interfaces.DomainStreamer, skipCount int) error

	// Log management
	DeleteJobLogs(jobID string) error

	// Job deletion
	DeleteJob(jobID string) error

	// PubSub access for IPC integration
	PubSub() pubsub.PubSub[JobEvent]

	// State synchronization
	SyncFromPersistentState(ctx context.Context) error

	// Health checks
	HealthCheckServices(ctx context.Context) error

	// Lifecycle
	Close() error
}

// VolumeStorer manages volume creation, tracking, and cleanup.
type VolumeStorer interface {
	interfaces.VolumeStore
	Close() error
}

// NetworkStorer manages network configurations and job network allocations.
type NetworkStorer interface {
	// Setting up and managing network configs
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

// MetricsStorer handles job metrics collection and storage.
// Manages collectors that gather resource usage data and persist metrics.
type MetricsStorer interface {
	// StreamMetrics streams real-time metrics for a job
	StreamMetrics(ctx context.Context, jobID string) (<-chan interface{}, error)

	// GetHistoricalMetrics retrieves historical metrics for a job
	GetHistoricalMetrics(jobID string, startTime, endTime int64) ([]interface{}, error)

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
