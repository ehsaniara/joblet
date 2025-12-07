# Joblet Persist

Dedicated persistence service for the Joblet job execution platform. Handles all log, metrics, and eBPF telemetry
storage, queries, and lifecycle management.

## Overview

`persist` is a separate service that receives logs, metrics, and eBPF events from `joblet-core` via Unix domain
sockets (IPC) and provides:

- **Persistent storage** - Local filesystem storage and CloudWatch Logs integration
- **eBPF Event Storage** - Process execution (exec) and network connection events
- **Historical queries** - gRPC API for querying stored logs, metrics, and events
- **Data lifecycle** - Retention policies, cleanup, compression, and rotation
- **Multiple backends** - Pluggable storage architecture (local, CloudWatch, S3)

## Architecture

```
joblet-core (execution)
     │
     │ IPC (Unix Socket)
     ▼
persist (storage)
     │
     ├─► Local Filesystem
     ├─► CloudWatch (v2.0)
     └─► S3 (v2.0)
```

## Features

### v1.0 (Current)

- ✅ IPC server for receiving logs/metrics from joblet-core
- ✅ Local filesystem storage with compression
- ✅ File rotation and retention policies
- ✅ Job index for fast lookups
- ✅ gRPC API for historical queries
- ✅ Batch writing for performance

### v1.1 (Current)

- ✅ **CloudWatch Logs integration** - Ship logs, metrics, and eBPF events to CloudWatch
- ✅ **eBPF event storage** - Process execution (exec) and network connection events
- ✅ Multi-backend support (local + CloudWatch)

### v2.0 (Planned)

- [ ] S3 archival
- [ ] Advanced querying (full-text search, time-range aggregation)
- [ ] File access events (eBPF)

## Building

```bash
go build -o bin/persist ./cmd/persist
```

## Configuration

See `config.example.yml` for a complete configuration example.

Key configuration sections:

- **server** - gRPC server settings
- **ipc** - Unix socket configuration
- **storage** - Backend configuration (local/cloudwatch/s3)
- **writer** - Write pipeline tuning
- **query** - Query engine settings
- **monitoring** - Prometheus and health endpoints

## Running

```bash
# With default config
./bin/persist

# With custom config
./bin/persist -config /path/to/config.yml
```

## API

### gRPC Services

**PersistService** (port 50052 by default):

- `QueryLogs` - Stream logs for a job
- `QueryMetrics` - Stream metrics for a job
- `GetJobInfo` - Get job metadata
- `ListJobs` - List jobs with filters
- `DeleteJob` - Delete job data
- `GetStats` - Get service statistics
- `CleanupOldData` - Run retention cleanup

### IPC Protocol

Messages received from joblet-core via Unix socket at `/opt/joblet/run/persist.sock`:

- Protocol: Length-prefixed Protobuf
- Message types: Logs, Metrics, ExecEvents, ConnectEvents
- Format: `[4-byte length][protobuf message]`

**Message Types:**

| Type | Description |
|------|-------------|
| `MESSAGE_TYPE_LOG` | Job stdout/stderr log lines |
| `MESSAGE_TYPE_METRIC` | Resource metrics (CPU, memory, GPU, I/O) |
| `MESSAGE_TYPE_EXEC_EVENT` | Process execution events (from eBPF) |
| `MESSAGE_TYPE_CONNECT_EVENT` | Network connection events (from eBPF) |

## Storage Layout

### Local Backend

```
/opt/joblet/
├── logs/
│   └── <job-uuid>/
│       ├── stdout.log.gz
│       └── stderr.log.gz
├── metrics/
│   └── <job-uuid>/
│       └── metrics.jsonl.gz
├── events/
│   └── <job-uuid>/
│       ├── exec_events.jsonl.gz     # eBPF process execution events
│       └── connect_events.jsonl.gz  # eBPF network connection events
└── job_index.json
```

### CloudWatch Backend

```
CloudWatch Logs:
  Log Group: /joblet/{node_id}
  Log Streams per job:
    - {job_id}-logs           # stdout/stderr logs
    - {job_id}-metrics        # Resource metrics (JSON)
    - {job_id}-exec-events    # Process execution events (JSON)
    - {job_id}-connect-events # Network connection events (JSON)
```

## Monitoring

- **Prometheus metrics**: `http://localhost:9092/metrics`
- **Health check**: `http://localhost:9093/health`

Key metrics:

- `persist_ipc_messages_received_total`
- `persist_write_latency_seconds`
- `persist_storage_bytes_total`
- `persist_query_requests_total`

## Development

### Project Structure

```
persist/
├── cmd/
│   └── persist/           # Main entry point
├── internal/
│   ├── config/            # Configuration
│   ├── ipc/              # IPC server
│   ├── storage/          # Storage backends
│   │   ├── backend.go    # Interface
│   │   ├── local.go      # Local filesystem
│   │   └── index.go      # Job index
│   ├── query/            # Query engine (TODO)
│   └── server/           # gRPC server
└── pkg/
    ├── logger/           # Logging
    └── errors/           # Error types
```

### Adding a New Storage Backend

1. Implement the `storage.Backend` interface
2. Add configuration in `config/config.go`
3. Register in `storage.NewBackend()`

Example:

```go
type MyBackend struct { ... }

func (b *MyBackend) WriteLogs(jobID string, logs []*ipcpb.LogLine) error { ... }
func (b *MyBackend) WriteMetrics(jobID string, metrics []*ipcpb.Metric) error { ... }
func (b *MyBackend) WriteExecEvents(jobID string, events []*ipcpb.ExecEvent) error { ... }
func (b *MyBackend) WriteConnectEvents(jobID string, events []*ipcpb.ConnectEvent) error { ... }
// ... implement other interface methods
```

## License

Same as joblet-core

## Related Projects

- [joblet](https://github.com/ehsaniara/joblet) - Core job execution engine
- [joblet-proto](https://github.com/ehsaniara/joblet-proto) - Protobuf definitions
- [joblet-sdk-python](https://github.com/ehsaniara/joblet-sdk-python) - Python SDK
- [joblet-admin](https://github.com/ehsaniara/joblet-admin) - Admin UI
- [joblet-mcp-server](https://github.com/ehsaniara/joblet-mcp-server) - MCP Server
