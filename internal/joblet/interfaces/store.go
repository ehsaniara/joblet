package interfaces

import (
	"context"
	"joblet/internal/joblet/domain"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

// Store defines the interface for managing job state and output buffering.
// It provides thread-safe operations for job lifecycle management and real-time log streaming.
//
//counterfeiter:generate . Store
type Store interface {
	// CreateNewJob adds a new job to the store with initial state.
	// Creates a new Task wrapper for the job and stores it by job ID.
	CreateNewJob(job *domain.Job)
	// UpdateJob updates an existing job's state and notifies subscribers.
	// Publishes status updates and shuts down subscribers if job is completed.
	UpdateJob(job *domain.Job)
	// GetJob retrieves a job by ID.
	// Returns the job and true if found, nil and false otherwise.
	GetJob(id string) (*domain.Job, bool)
	// ListJobs returns all jobs currently stored in the system.
	// Returns a slice containing copies of all job instances.
	ListJobs() []*domain.Job
	// WriteToBuffer appends log data to a job's output buffer.
	// Notifies subscribers of new log chunks for real-time streaming.
	WriteToBuffer(jobId string, chunk []byte)
	// GetOutput retrieves the complete output buffer for a job.
	// Returns the buffer data, whether job is still running, and any error.
	GetOutput(id string) ([]byte, bool, error)
	// SendUpdatesToClient streams job logs and status updates to a client.
	// Handles both existing buffer content and real-time updates for running jobs.
	SendUpdatesToClient(ctx context.Context, id string, stream DomainStreamer) error
}

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

// VolumeStore defines the interface for managing volume storage operations.
// Provides thread-safe operations for volume lifecycle management and usage tracking.
//
//counterfeiter:generate . VolumeStore
type VolumeStore interface {
	// CreateVolume adds a new volume to the store.
	CreateVolume(volume *domain.Volume) error
	// GetVolume retrieves a volume by name.
	GetVolume(name string) (*domain.Volume, bool)
	// ListVolumes returns all volumes currently stored in the system.
	ListVolumes() []*domain.Volume
	// RemoveVolume removes a volume from the store by name.
	RemoveVolume(name string) error
	// IncrementJobCount increases the usage count for a volume.
	IncrementJobCount(name string) error
	// DecrementJobCount decreases the usage count for a volume.
	DecrementJobCount(name string) error
}
