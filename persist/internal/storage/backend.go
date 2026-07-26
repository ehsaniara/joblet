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
	ReadLogs(ctx context.Context, query *LogQuery) (*EventReader[*ipcpb.LogLine], error)
	ReadMetrics(ctx context.Context, query *MetricQuery) (*EventReader[*ipcpb.Metric], error)
	ReadExecEvents(ctx context.Context, query *TelemetryQuery) (*EventReader[*ipcpb.ExecEvent], error)
	ReadConnectEvents(ctx context.Context, query *TelemetryQuery) (*EventReader[*ipcpb.ConnectEvent], error)
	ReadFileEvents(ctx context.Context, query *TelemetryQuery) (*EventReader[*ipcpb.FileEvent], error)
	ReadAcceptEvents(ctx context.Context, query *TelemetryQuery) (*EventReader[*ipcpb.AcceptEvent], error)
	ReadSocketDataEvents(ctx context.Context, query *TelemetryQuery) (*EventReader[*ipcpb.SocketDataEvent], error)
	ReadMmapEvents(ctx context.Context, query *TelemetryQuery) (*EventReader[*ipcpb.MmapEvent], error)
	ReadMprotectEvents(ctx context.Context, query *TelemetryQuery) (*EventReader[*ipcpb.MprotectEvent], error)

	// Management operations
	DeleteJob(jobID string) error

	// Lifecycle
	Close() error
}

// LogQuery parameters
type LogQuery struct {
	JobUUID   string
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
	JobUUID     string
	NodeID      string // Node ID where job ran (for CloudWatch multi-node lookup)
	StartTime   *int64
	EndTime     *int64
	Aggregation string
	Limit       int
	Offset      int
}

// EventReader provides generic streaming access to events of type T.
type EventReader[T any] struct {
	Channel chan T
	Error   chan error
	Done    chan struct{}
}

// NewEventReader creates a new EventReader with standard buffer sizes
func NewEventReader[T any](bufferSize int) *EventReader[T] {
	if bufferSize <= 0 {
		bufferSize = 100
	}
	return &EventReader[T]{
		Channel: make(chan T, bufferSize),
		Error:   make(chan error, 1),
		Done:    make(chan struct{}),
	}
}

// Close closes all channels. Should be called by the producer goroutine.
func (r *EventReader[T]) Close() {
	close(r.Channel)
	close(r.Error)
	close(r.Done)
}

// SendError sends an error to the error channel if not nil
func (r *EventReader[T]) SendError(err error) {
	if err != nil {
		select {
		case r.Error <- err:
		default:
			// Error channel full, skip
		}
	}
}

// TelemetryQuery parameters for exec and connect events
type TelemetryQuery struct {
	JobUUID   string
	NodeID    string // Node ID where job ran (for CloudWatch multi-node lookup)
	StartTime *int64
	EndTime   *int64
	Limit     int
	Offset    int
}

// NewBackend creates a new storage backend based on configuration
func NewBackend(cfg *config.StorageConfig, nodeID string, log *logger.Logger) (Backend, error) {
	switch cfg.Type {
	case "local", "":
		return NewLocalBackend(cfg, log)
	case "cloudwatch":
		return NewCloudWatchBackend(cfg, nodeID, log)
	case "s3":
		return NewS3Backend(cfg, nodeID, log)
	default:
		return nil, fmt.Errorf("unknown storage backend type: %s", cfg.Type)
	}
}
