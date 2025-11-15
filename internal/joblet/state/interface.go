package state

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

import (
	"context"

	"github.com/ehsaniara/joblet/internal/joblet/domain"
)

// StateClient defines the interface for state persistence operations
// Both Client and PooledClient implement this interface
//
//counterfeiter:generate . StateClient
type StateClient interface {
	// Connect establishes initial connection to state service
	Connect() error

	// Close closes all connections
	Close() error

	// Create creates a new job state
	Create(ctx context.Context, job *domain.Job) error

	// Update updates an existing job state
	Update(ctx context.Context, job *domain.Job) error

	// Delete deletes a job state
	Delete(ctx context.Context, jobID string) error

	// Get retrieves a job state
	Get(ctx context.Context, jobID string) (*domain.Job, error)

	// List retrieves all job states with optional filter
	List(ctx context.Context, filter *Filter) ([]*domain.Job, error)

	// Sync synchronizes bulk job states (for reconciliation)
	Sync(ctx context.Context, jobs []*domain.Job) error

	// Ping checks if the state service is healthy
	Ping(ctx context.Context) error
}

// Ensure PooledClient satisfies the interface
var _ StateClient = (*PooledClient)(nil)
