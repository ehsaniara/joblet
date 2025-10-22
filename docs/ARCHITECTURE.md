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
│   ├── cmd/joblet-persist/
│   ├── internal/
│   └── go.mod           # Separate Go module
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

- **Binary**: `bin/joblet-persist`
- **Port**: `:50052` (gRPC)
- **Purpose**: Historical data persistence and queries
- **Responsibilities**:
    - Receive logs/metrics via IPC
    - Store to disk (local) or cloud (S3, CloudWatch)
    - Historical queries (logs, metrics)
    - Data retention and cleanup
    - Compression and archival

### 3. RNX (CLI Client)

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
2. Joblet creates isolated environment (cgroups, namespaces)
3. Joblet executes job and streams live logs → RNX
4. Joblet collects metrics (CPU, memory, GPU, I/O)
5. Joblet sends logs/metrics → Persist (via IPC)
6. Persist stores to disk (/opt/joblet/logs, /opt/joblet/metrics)
```

### Historical Query Flow

```
1. RNX sends QueryLogs request → Persist
2. Persist reads from disk storage
3. Persist streams results → RNX
```

## Storage Layout

```
/opt/joblet/
├── bin/                    # Binaries
│   ├── joblet
│   ├── rnx
│   └── joblet-persist
├── config/                 # Configuration
│   └── joblet-config.yml   # Unified config (both services)
├── run/                    # Runtime files
│   ├── persist-ipc.sock    # IPC socket (log/metric writes)
│   └── persist-grpc.sock   # gRPC socket (historical queries)
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
    - Automatically spawns joblet-persist as a subprocess
    - joblet-persist uses the same config file (persist: section)

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

### Persist Module

- **Path**: `github.com/ehsaniara/joblet/persist`
- **Go.mod**: `./persist/go.mod`
- **Dependencies**:
    - `github.com/ehsaniara/joblet` (local replace to parent)

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

### 3. Why Internal Protos?

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
