# Joblet Persist

Dedicated persistence service for the Joblet job execution platform. Handles all log and metrics storage, queries, and lifecycle management.

## Overview

`joblet-persist` is a separate service that receives logs and metrics from `joblet-core` via Unix domain sockets (IPC) and provides:

- **Persistent storage** - Local filesystem storage (v1.0) with cloud backends coming in v2.0+
- **Historical queries** - gRPC API for querying stored logs and metrics
- **Data lifecycle** - Retention policies, cleanup, compression, and rotation
- **Multiple backends** - Pluggable storage architecture (local, CloudWatch, S3)

## Architecture

```
joblet-core (execution)
     │
     │ IPC (Unix Socket)
     ▼
joblet-persist (storage)
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

### v2.0 (Planned)
- [ ] CloudWatch Logs integration
- [ ] S3 archival
- [ ] Multi-backend routing
- [ ] Advanced querying (full-text search, time-range aggregation)

## Building

```bash
go build -o bin/joblet-persist ./cmd/joblet-persist
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
./bin/joblet-persist

# With custom config
./bin/joblet-persist -config /path/to/config.yml
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
- Message types: Logs, Metrics
- Format: `[4-byte length][protobuf message]`

## Storage Layout

```
/opt/joblet/
├── logs/
│   └── <job-uuid>/
│       ├── stdout.log.gz
│       └── stderr.log.gz
├── metrics/
│   └── <job-uuid>/
│       └── metrics.jsonl.gz
└── job_index.json
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
joblet-persist/
├── cmd/
│   └── joblet-persist/    # Main entry point
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
