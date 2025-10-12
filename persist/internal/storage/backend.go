package storage

import (
	"context"
	"fmt"

	ipcpb "github.com/ehsaniara/joblet/internal/proto/gen/ipc"
	"github.com/ehsaniara/joblet/persist/internal/config"
	"github.com/ehsaniara/joblet/persist/pkg/logger"
)

// Backend is the storage backend interface
type Backend interface {
	// Write operations
	WriteLogs(jobID string, logs []*ipcpb.LogLine) error
	WriteMetrics(jobID string, metrics []*ipcpb.Metric) error

	// Read operations
	ReadLogs(ctx context.Context, query *LogQuery) (*LogReader, error)
	ReadMetrics(ctx context.Context, query *MetricQuery) (*MetricReader, error)

	// Management operations
	DeleteJob(jobID string) error
	ListJobs(filter *JobFilter) ([]string, error)
	GetJobInfo(jobID string) (*JobInfo, error)

	// Lifecycle
	Close() error
}

// LogQuery parameters
type LogQuery struct {
	JobID     string
	Stream    ipcpb.StreamType
	StartTime *int64
	EndTime   *int64
	Limit     int
	Offset    int
	Filter    string
}

// MetricQuery parameters
type MetricQuery struct {
	JobID       string
	StartTime   *int64
	EndTime     *int64
	Aggregation string
	Limit       int
	Offset      int
}

// JobFilter for listing jobs
type JobFilter struct {
	Since  *int64
	Until  *int64
	Limit  int
	Offset int
}

// JobInfo contains job metadata
type JobInfo struct {
	JobID       string
	CreatedAt   int64
	LastUpdated int64
	LogCount    int64
	MetricCount int64
	SizeBytes   int64
}

// LogReader provides streaming access to logs
type LogReader struct {
	Channel chan *ipcpb.LogLine
	Error   chan error
	Done    chan struct{}
}

// MetricReader provides streaming access to metrics
type MetricReader struct {
	Channel chan *ipcpb.Metric
	Error   chan error
	Done    chan struct{}
}

// NewBackend creates a new storage backend based on configuration
func NewBackend(cfg *config.StorageConfig, log *logger.Logger) (Backend, error) {
	switch cfg.Type {
	case "local":
		return NewLocalBackend(cfg, log)
	case "cloudwatch":
		return nil, fmt.Errorf("CloudWatch backend not implemented yet (v2.0)")
	case "s3":
		return nil, fmt.Errorf("S3 backend not implemented yet (v2.0)")
	default:
		return nil, fmt.Errorf("unknown storage backend type: %s", cfg.Type)
	}
}
