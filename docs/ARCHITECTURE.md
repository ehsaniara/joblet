# Joblet Architecture

## Overview

Joblet is a high-performance job execution system with a monorepo structure containing two main services and a CLI tool.

## Repository Structure

```
joblet/
├── cmd/
│   ├── joblet/          # Main service daemon
│   └── rnx/             # CLI client
├── internal/
│   ├── joblet/          # Main service implementation
│   ├── rnx/             # CLI implementation
│   └── proto/           # Internal protocol buffers (IPC, persist)
├── persist/             # Persistence service (sub-module)
│   ├── cmd/persist/
│   ├── internal/
│   └── go.mod           # Separate Go module
├── state/               # State persistence service (sub-module)
│   ├── cmd/state/       # State service binary
│   ├── internal/
│   │   ├── storage/     # Backend interface (Memory, DynamoDB, Redis)
│   │   └── ipc/         # IPC server
│   └── go.mod           # Separate Go module (AWS SDK dependencies)
├── api/
│   └── gen/             # Generated public API (from external joblet-proto)
├── pkg/                 # Shared packages
│   ├── config/          # Configuration management
│   ├── logger/          # Logging utilities
│   ├── security/        # TLS/mTLS support
│   └── client/          # gRPC clients
└── scripts/             # Build and deployment scripts
```

## Components

### 1. Joblet (Main Service)

- **Binary**: `bin/joblet`
- **Port**: `:50051` (gRPC)
- **Purpose**: Job execution engine
- **Responsibilities**:
    - Job scheduling and execution
    - Resource isolation (cgroups, namespaces)
    - GPU allocation
    - Network isolation
    - Runtime management
    - Live metrics streaming

### 2. Joblet-Persist (Persistence Service)

- **Binary**: `bin/persist`
- **Port**: `:50052` (gRPC)
- **Purpose**: Historical data persistence and queries
- **Responsibilities**:
    - Receive logs/metrics via IPC
    - Store to disk (local) or cloud (S3, CloudWatch)
    - Historical queries (logs, metrics)
    - Data retention and cleanup
    - Compression and archival

### 3. State (Job State Persistence Service)

- **Binary**: `bin/state`
- **Communication**: Unix socket IPC only (`/opt/joblet/run/state-ipc.sock`)
- **Purpose**: Job metadata persistence across restarts
- **Responsibilities**:
    - Persist job state (status, exit code, timestamps) via async IPC
    - Store to pluggable backends (Memory, DynamoDB, Redis)
    - Sync jobs on joblet startup
    - Auto-reconnection and graceful degradation
    - TTL-based cleanup (DynamoDB)
- **Backend Support**:
    - **Memory**: RAM-only (testing, lost on restart)
    - **DynamoDB**: AWS cloud persistence (production, survives restarts)
    - **Redis**: Planned for future releases

### 4. RNX (CLI Client)

- **Binary**: `bin/rnx`
- **Purpose**: Command-line interface
- **Features**:
    - Job management (run, status, stop, logs)
    - Workflow orchestration
    - Network/volume management
    - Runtime installation
    - Multi-node support

## Communication Architecture

```
┌─────────────┐                    ┌──────────────────┐
│  RNX Client │◄──── gRPC ────────►│  Joblet Service  │
└─────────────┘     :50051         │     (main)       │
                                   └──────────────────┘
                                           │
                                           │ Unix Socket
                                           │ IPC
                                           │
                                           ▼
                                   ┌──────────────────┐
                                   │ Joblet-Persist   │
                                   │   (storage)      │
                                   └──────────────────┘
                                           │
                                           │ gRPC
                                           ▼
                                   ┌──────────────────┐
                                   │   RNX Queries    │
                                   │ (historical data)│
                                   └──────────────────┘
```

### Communication Protocols

1. **Public API** (RNX ↔ Joblet)
    - Protocol: gRPC
    - Proto: `joblet.proto` (external, versioned)
    - Source: `github.com/ehsaniara/joblet-proto`
    - Generated: `api/gen/`

2. **Internal IPC** (Joblet ↔ Persist)
    - Protocol: Unix Domain Socket
    - Proto: `internal/proto/ipc.proto`
    - Messages: Logs, Metrics
    - Socket: `/opt/joblet/run/persist.sock`

3. **Query API** (RNX ↔ Persist)
    - Protocol: gRPC
    - Proto: `internal/proto/persist.proto`
    - Services: QueryLogs, QueryMetrics

## Protocol Buffer Organization

### External Proto (Public API)

- **Repository**: `github.com/ehsaniara/joblet-proto`
- **Version**: v1.0.9 (from GitHub)
- **File**: `joblet.proto`
- **Purpose**: Public API contract for clients
- **Generated**: `api/gen/joblet.pb.go`, `joblet_grpc.pb.go`

### Internal Protos (Monorepo)

- **Location**: `internal/proto/`
- **Files**:
    - `ipc.proto` - IPC messages (logs, metrics)
    - `persist.proto` - Persistence service API
- **Generated**: `internal/proto/gen/`
- **Purpose**: Internal communication only

## Configuration

### Unified Config File

Both services use: `/opt/joblet/config/joblet-config.yml`

```yaml
# Main service config (top-level)
server:
  port: 50051
joblet:
  maxConcurrentJobs: 100
# ... other main config

# Persist service config (nested)
persist:
  server:
    grpc_socket: "/opt/joblet/run/persist-grpc.sock"  # Unix socket for gRPC queries
  ipc:
    socket: "/opt/joblet/run/persist-ipc.sock"  # Unix socket for log/metric writes
  # ... other persist config
```

The persist service automatically detects and reads from the `persist:` section.

## Security

### TLS/mTLS Support

Both services support TLS encryption:

#### Main Joblet Service

- **TLS**: Required (embedded certificates in config)
- **Generation**: `scripts/certs_gen_embedded.sh`
- **Configuration**: Auto-embedded in `security:` section
- **Client Auth**: Mutual TLS with client certificates

#### Persist Service

- **TLS**: Optional (file-based certificates)
- **Configuration**: `persist.tls` section
- **Default**: Disabled (runs on localhost via Unix socket)
- **Use Case**: Enable when exposing persist API externally

#### Shared TLS Library

- **Location**: `pkg/security/tls.go`
- **Functions**:
    - `LoadServerTLSConfig()` - Server-side TLS
    - `LoadClientTLSConfig()` - Client-side TLS
- **Features**:
    - TLS 1.2 minimum
    - Optional mTLS with client certificate validation
    - CA certificate verification

## Build System

### Makefile Targets

- `make all` - Build all binaries
- `make joblet` - Build main service only
- `make rnx` - Build CLI only
- `make persist` - Build persist service only
- `make proto` - Generate all proto files
- `make clean` - Remove build artifacts
- `make deploy` - Deploy to remote server
- `make test` - Run all tests

### Proto Generation

Proto generation uses two approaches:

1. **`scripts/generate-proto.sh`** - External public API
    - Downloads from `joblet-proto` module
    - Generates `api/gen/`

2. **`go generate ./internal/proto`** - Internal protos (IPC and Persist)
    - Uses local `internal/proto/` files
    - Generates `internal/proto/gen/ipc/` and `internal/proto/gen/persist/`
    - Defined via `//go:generate` directives in `internal/proto/generate.go`

Developers can regenerate protos with:

```bash
make proto              # Regenerate all protos
go generate ./internal/proto  # Regenerate internal protos only
```

## Data Flow

### Job Execution Flow

```
1. RNX sends RunJob request → Joblet
2. Joblet creates job record in memory + sends to State (async IPC)
3. State persists job metadata to backend (Memory/DynamoDB)
4. Joblet creates isolated environment (cgroups, namespaces)
5. Joblet executes job and streams live logs → RNX
6. Joblet collects metrics (CPU, memory, GPU, I/O)
7. Joblet sends logs/metrics → Persist (via IPC)
8. Persist stores to disk (/opt/joblet/logs, /opt/joblet/metrics)
9. Joblet updates job status + sends to State (async IPC)
```

### Historical Query Flow

```
1. RNX sends QueryLogs request → Persist
2. Persist reads from disk storage
3. Persist streams results → RNX
```

### State Persistence Flow

```
1. Joblet job lifecycle events (create, update, complete) → State (async IPC)
2. State writes to backend (Memory/DynamoDB) with 5s timeout
3. On joblet restart: Joblet requests job sync → State
4. State reads from backend → returns all jobs
5. Joblet populates in-memory cache with persisted jobs
```

## Storage Layout

```
/opt/joblet/
├── bin/                    # Binaries
│   ├── joblet
│   ├── rnx
│   ├── persist
│   └── state               # State persistence service
├── config/                 # Configuration
│   └── joblet-config.yml   # Unified config (all services)
├── run/                    # Runtime files
│   ├── persist-ipc.sock    # IPC socket (log/metric writes)
│   ├── persist-grpc.sock   # gRPC socket (historical queries)
│   └── state-ipc.sock      # IPC socket (job state persistence)
├── jobs/                   # Job workspaces
│   └── {job-uuid}/         # Per-job directory
├── logs/                   # Historical logs
│   └── {job-uuid}/         # Per-job log files
├── metrics/                # Historical metrics
│   └── {job-uuid}/         # Per-job metric files
├── volumes/                # Named volumes
├── network/                # Network state
└── runtimes/               # Runtime environments
```

## Deployment

### Systemd Services

Single systemd service:

1. **joblet.service**
    - Binary: `/opt/joblet/bin/joblet`
    - Config: `/opt/joblet/config/joblet-config.yml`
    - Automatically spawns two subprocesses:
      - **persist**: Log/metric persistence (persist: section)
      - **state**: Job state persistence (state: section)

### Deployment Command

```bash
make deploy REMOTE_HOST=192.168.1.161 REMOTE_USER=jay
```

This builds all binaries, copies to remote server, and restarts services.

## Development Workflow

### 1. Make Changes

```bash
# Edit code in internal/, cmd/, or persist/
vim internal/joblet/core/executor.go
```

### 2. Update Protos (if needed)

```bash
# For public API changes - update external joblet-proto repo
# For internal changes - edit internal/proto/*.proto
make proto
```

### 3. Build and Test

```bash
make clean
make all
make test
```

### 4. Deploy

```bash
make deploy
```

## Module Structure

### Main Module

- **Path**: `github.com/ehsaniara/joblet`
- **Go.mod**: `./go.mod`
- **Dependencies**:
    - `github.com/ehsaniara/joblet-proto@v1.0.9` (external)
    - `github.com/ehsaniara/joblet/persist` (local replace)
    - `github.com/ehsaniara/joblet/state` (local replace)

### Persist Module

- **Path**: `github.com/ehsaniara/joblet/persist`
- **Go.mod**: `./persist/go.mod`
- **Dependencies**:
    - `github.com/ehsaniara/joblet` (local replace to parent)

### State Module

- **Path**: `github.com/ehsaniara/joblet/state`
- **Go.mod**: `./state/go.mod`
- **Dependencies**:
    - `github.com/ehsaniara/joblet` (local replace to parent)
    - `github.com/aws/aws-sdk-go-v2` (DynamoDB backend)
    - `github.com/aws/aws-sdk-go-v2/service/dynamodb` (DynamoDB client)

## Design Decisions

### 1. Why Monorepo?

- **Unified versioning**: All components version together
- **Shared code**: Common packages in `pkg/`
- **Simplified deployment**: Single build produces all binaries
- **Atomic changes**: Changes across services in single commit

### 2. Why Separate Persist Service?

- **Scalability**: Can scale persistence independently
- **Fault isolation**: Main service continues if persist fails
- **Performance**: Async IPC prevents blocking main service
- **Flexibility**: Can swap storage backends without changing main service

### 3. Why Separate State Service?

- **AWS Isolation**: Keeps AWS SDK dependencies (DynamoDB) out of main joblet binary
- **Fault isolation**: State service crashes don't kill joblet
- **Independent scaling**: Can upgrade state service independently
- **Multiple backends**: Easy to add Redis, PostgreSQL, or other backends
- **Performance**: Async fire-and-forget operations for maximum throughput
- **Separation of concerns**: Job state distinct from logs/metrics (persist)

### 4. Why Internal Protos?

- **Encapsulation**: IPC and persist protos are implementation details
- **Versioning**: Internal changes don't affect public API
- **Flexibility**: Can evolve internal protocols independently

### 4. Why External joblet-proto?

- **Client SDKs**: External clients can use versioned proto
- **Language bindings**: Generate clients in Python, Java, etc.
- **API stability**: Public API is stable and versioned

## Performance Characteristics

### Throughput

- **Job submission**: 1000+ jobs/sec
- **Log streaming**: 100MB/s per job
- **Metrics collection**: 5s sample rate, 1000s jobs/node

### Latency

- **Job start**: <100ms (warm) / <1s (cold with runtime)
- **Log delivery**: <10ms (live) / <100ms (historical)
- **Metrics query**: <50ms (cached) / <500ms (disk)

### Resource Usage

- **Joblet**: ~50MB RAM idle / +10MB per 100 jobs
- **Persist**: ~100MB RAM / +storage for history
- **RNX**: <10MB RAM

## Future Enhancements

### Planned (v2.0)

- [ ] Cloud storage backends (S3, CloudWatch)
- [ ] Distributed job scheduling (multi-node)
- [ ] Job priority and preemption
- [ ] Advanced workflow DAGs
- [ ] Web UI dashboard

### Considered

- [ ] Kubernetes integration
- [ ] Job checkpointing
- [ ] Spot instance support
- [ ] Cost optimization
