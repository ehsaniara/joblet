package storage

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

import (
	"context"

	"github.com/ehsaniara/joblet/internal/joblet/domain"
)

//counterfeiter:generate . Backend

// Backend defines the interface for job state storage backends.
// Implementations: memory, DynamoDB, Redis, PostgreSQL, etc.
type Backend interface {
	// Create a new job state
	Create(ctx context.Context, job *domain.Job) error

	// Get a job by ID
	Get(ctx context.Context, jobID string) (*domain.Job, error)

	// Update an existing job state
	Update(ctx context.Context, job *domain.Job) error

	// Delete a job state
	Delete(ctx context.Context, jobID string) error

	// List all jobs with optional filter
	List(ctx context.Context, filter *Filter) ([]*domain.Job, error)

	// Sync bulk job states (used for reconciliation)
	Sync(ctx context.Context, jobs []*domain.Job) error

	// Close the backend connection
	Close() error

	// HealthCheck verifies backend availability
	HealthCheck(ctx context.Context) error
}

// Filter for listing jobs
type Filter struct {
	Status   string   // Filter by status (PENDING, RUNNING, COMPLETED, FAILED)
	NodeID   string   // Filter by node ID
	Limit    int      // Max number of results (0 = unlimited)
	SortBy   string   // Sort field (createdAt, updatedAt)
	SortDesc bool     // Sort descending
	Statuses []string // Multiple statuses (OR condition)
}

// Config holds backend configuration
// All operations are async fire-and-forget for high performance
type Config struct {
	Backend  string          `yaml:"backend" json:"backend"` // "memory", "dynamodb", "redis"
	DynamoDB *DynamoDBConfig `yaml:"dynamodb" json:"dynamodb"`
	Redis    *RedisConfig    `yaml:"redis" json:"redis"`
}

// DynamoDBConfig holds DynamoDB-specific configuration
type DynamoDBConfig struct {
	Region        string `yaml:"region" json:"region"`
	TableName     string `yaml:"table_name" json:"table_name"`
	TTLEnabled    bool   `yaml:"ttl_enabled" json:"ttl_enabled"`
	TTLAttribute  string `yaml:"ttl_attribute" json:"ttl_attribute"`
	TTLDays       int    `yaml:"ttl_days" json:"ttl_days"`
	ReadCapacity  int64  `yaml:"read_capacity" json:"read_capacity"`
	WriteCapacity int64  `yaml:"write_capacity" json:"write_capacity"`
	BatchSize     int    `yaml:"batch_size" json:"batch_size"`
	BatchInterval string `yaml:"batch_interval" json:"batch_interval"`
}

// RedisConfig holds Redis-specific configuration (future)
type RedisConfig struct {
	Endpoint string `yaml:"endpoint" json:"endpoint"`
	Password string `yaml:"password" json:"password"`
	DB       int    `yaml:"db" json:"db"`
	TTLDays  int    `yaml:"ttl_days" json:"ttl_days"`
}

// NewBackend creates a new storage backend based on configuration
func NewBackend(cfg *Config) (Backend, error) {
	switch cfg.Backend {
	case "memory", "":
		return NewMemoryBackend(), nil
	case "dynamodb":
		return NewDynamoDBBackend(cfg.DynamoDB)
	case "redis":
		// Future implementation
		return nil, ErrBackendNotImplemented
	default:
		return nil, ErrInvalidBackend
	}
}

// Error types
var (
	ErrJobNotFound           = &StorageError{Code: "JOB_NOT_FOUND", Message: "job not found"}
	ErrJobAlreadyExists      = &StorageError{Code: "JOB_EXISTS", Message: "job already exists"}
	ErrOptimisticLockFailed  = &StorageError{Code: "LOCK_FAILED", Message: "optimistic lock failed"}
	ErrInvalidBackend        = &StorageError{Code: "INVALID_BACKEND", Message: "invalid storage backend"}
	ErrBackendNotImplemented = &StorageError{Code: "NOT_IMPLEMENTED", Message: "backend not implemented"}
	ErrBackendUnavailable    = &StorageError{Code: "UNAVAILABLE", Message: "backend unavailable"}
)

// StorageError represents a storage operation error
type StorageError struct {
	Code    string
	Message string
	Err     error
}

func (e *StorageError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

func (e *StorageError) Unwrap() error {
	return e.Err
}
