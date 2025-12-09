package storage

import (
	"context"
	"fmt"

	ipcpb "github.com/ehsaniara/joblet/internal/proto/gen/ipc"
	"github.com/ehsaniara/joblet/persist/internal/config"
	"github.com/ehsaniara/joblet/pkg/logger"
)

// Backend is the storage backend interface
type Backend interface {
	// Write operations
	WriteLogs(jobID string, logs []*ipcpb.LogLine) error
	WriteMetrics(jobID string, metrics []*ipcpb.Metric) error
	WriteExecEvents(jobID string, events []*ipcpb.ExecEvent) error
	WriteConnectEvents(jobID string, events []*ipcpb.ConnectEvent) error
	WriteFileEvents(jobID string, events []*ipcpb.FileEvent) error
	WriteAcceptEvents(jobID string, events []*ipcpb.AcceptEvent) error
	WriteSocketDataEvents(jobID string, events []*ipcpb.SocketDataEvent) error
	WriteMmapEvents(jobID string, events []*ipcpb.MmapEvent) error
	WriteMprotectEvents(jobID string, events []*ipcpb.MprotectEvent) error

	// Read operations
	ReadLogs(ctx context.Context, query *LogQuery) (*LogReader, error)
	ReadMetrics(ctx context.Context, query *MetricQuery) (*MetricReader, error)
	ReadExecEvents(ctx context.Context, query *TelemetryQuery) (*ExecEventReader, error)
	ReadConnectEvents(ctx context.Context, query *TelemetryQuery) (*ConnectEventReader, error)
	ReadFileEvents(ctx context.Context, query *TelemetryQuery) (*FileEventReader, error)
	ReadAcceptEvents(ctx context.Context, query *TelemetryQuery) (*AcceptEventReader, error)
	ReadSocketDataEvents(ctx context.Context, query *TelemetryQuery) (*SocketDataEventReader, error)
	ReadMmapEvents(ctx context.Context, query *TelemetryQuery) (*MmapEventReader, error)
	ReadMprotectEvents(ctx context.Context, query *TelemetryQuery) (*MprotectEventReader, error)

	// Management operations
	DeleteJob(jobID string) error

	// Lifecycle
	Close() error
}

// LogQuery parameters
type LogQuery struct {
	JobID     string
	NodeID    string // Node ID where job ran (for CloudWatch multi-node lookup)
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
	NodeID      string // Node ID where job ran (for CloudWatch multi-node lookup)
	StartTime   *int64
	EndTime     *int64
	Aggregation string
	Limit       int
	Offset      int
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

// TelemetryQuery parameters for exec and connect events
type TelemetryQuery struct {
	JobID     string
	NodeID    string // Node ID where job ran (for CloudWatch multi-node lookup)
	StartTime *int64
	EndTime   *int64
	Limit     int
	Offset    int
}

// ExecEventReader provides streaming access to exec events
type ExecEventReader struct {
	Channel chan *ipcpb.ExecEvent
	Error   chan error
	Done    chan struct{}
}

// ConnectEventReader provides streaming access to connect events
type ConnectEventReader struct {
	Channel chan *ipcpb.ConnectEvent
	Error   chan error
	Done    chan struct{}
}

// FileEventReader provides streaming access to file events
type FileEventReader struct {
	Channel chan *ipcpb.FileEvent
	Error   chan error
	Done    chan struct{}
}

// AcceptEventReader provides streaming access to accept events
type AcceptEventReader struct {
	Channel chan *ipcpb.AcceptEvent
	Error   chan error
	Done    chan struct{}
}

// SocketDataEventReader provides streaming access to socket data events
type SocketDataEventReader struct {
	Channel chan *ipcpb.SocketDataEvent
	Error   chan error
	Done    chan struct{}
}

// MmapEventReader provides streaming access to mmap events
type MmapEventReader struct {
	Channel chan *ipcpb.MmapEvent
	Error   chan error
	Done    chan struct{}
}

// MprotectEventReader provides streaming access to mprotect events
type MprotectEventReader struct {
	Channel chan *ipcpb.MprotectEvent
	Error   chan error
	Done    chan struct{}
}

// NewBackend creates a new storage backend based on configuration
func NewBackend(cfg *config.StorageConfig, nodeID string, log *logger.Logger) (Backend, error) {
	switch cfg.Type {
	case "local", "":
		return NewLocalBackend(cfg, log)
	case "cloudwatch":
		return NewCloudWatchBackend(cfg, nodeID, log)
	case "s3":
		return nil, fmt.Errorf("S3 backend not implemented yet (v2.0)")
	default:
		return nil, fmt.Errorf("unknown storage backend type: %s", cfg.Type)
	}
}
