package adapters

import (
	"context"

	"github.com/ehsaniara/joblet/internal/joblet/domain"
	"github.com/ehsaniara/joblet/internal/joblet/interfaces"
	"github.com/ehsaniara/joblet/internal/joblet/network"
	"github.com/ehsaniara/joblet/internal/joblet/pubsub"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

// JobStorer handles all the job storage stuff - keeping track of jobs, their logs,
// and making sure everyone who cares about job updates gets notified.
//
//counterfeiter:generate . JobStorer
type JobStorer interface {
	// Basic job stuff - create, update, find, list
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

	// Taking care of job logs
	DeleteJobLogs(jobID string) error

	// Cleanup - get rid of jobs and all their stuff when we're done
	DeleteJob(jobID string) error

	// PubSub access for IPC integration
	PubSub() pubsub.PubSub[JobEvent]

	// Shutting down gracefully
	Close() error
}

// VolumeStorer handles all our volume storage needs - creating them, tracking them,
// and cleaning them up when we're done.
type VolumeStorer interface {
	// All the basic volume operations
	interfaces.VolumeStore

	// Cleanup when done
	Close() error
}

// NetworkStorer manages network configurations and job network allocations.
// Keeps track of which jobs are using which networks and how they're connected.
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
