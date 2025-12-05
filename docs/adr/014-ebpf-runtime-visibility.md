# ADR-014: Unified Job Telemetry with eBPF Visibility

## Status

**Implemented** (December 2025)

## Context

Joblet currently collects job metrics (CPU, memory, I/O) from cgroups v2 and stores them in the persist service. However, we have **zero visibility** into what jobs are actually doing - what binaries they execute, what network connections they make, what files they access.

We want to add eBPF-based activity tracking. Rather than building a separate system, we should unify metrics and activity events into a **single telemetry pipeline** because:

1. Both are time-series data about job behavior
2. Both need the same storage backends (local persist or CloudWatch)
3. Both need the same retention policies
4. Both are queried together when debugging jobs
5. Users want one place to see everything about a job

### Current State

```
Metrics Collection (cgroups v2)
         │
         ▼
   Persist Service ──► Local Storage OR CloudWatch Metrics
```

### Proposed State

```
Metrics (cgroups v2)     Activity (eBPF)
         │                     │
         └──────────┬──────────┘
                    ▼
           Telemetry Collector
                    │
                    ▼
           Persist Service ──► Local Storage OR CloudWatch
```

## Decision

Implement a **unified telemetry system** that treats metrics and activity events as two types of job telemetry, flowing through the same pipeline to the same storage backends.

### Unified View

```
$ rnx job watch ml-training-7a3f

╭─ JOB: ml-training-7a3f ─────────────────────────────────────────╮
│  STATUS: RUNNING    RUNTIME: python-3.11-ml    DURATION: 5m32s  │
╰─────────────────────────────────────────────────────────────────╯

RESOURCES:                                 PROCESSES:
  CPU:    ████████░░ 78%  (limit: 100%)     PID   CMD              CPU   MEM
  Memory: ██████░░░░ 2.1/4.0 GB             1234  python train.py  78%   2.1GB
  GPU:    ███████░░░ 68%  (3.2/4.0 GB)      1245  └─ dataloader    12%   512MB
  Disk:   12 MB/s read, 2 MB/s write
  Net:    ↓ 45 MB/s  ↑ 2 MB/s

ACTIVITY:
  [5m30s] EXEC    python train.py --epochs 100
  [5m28s] FILE    /data/training/dataset.csv (read)
  [5m25s] NET     connect 10.0.1.50:5432 → postgres
  [3m12s] FILE    /models/checkpoint-001.pt (write)
  [0m02s] EXEC    nvidia-smi --query-gpu=memory.used

CONNECTIONS:
  10.0.1.50:5432    ESTABLISHED  postgres
  10.0.1.55:6379    ESTABLISHED  redis
```

### Telemetry Types

| Type | Source | Frequency | Data |
|------|--------|-----------|------|
| **Metrics** | cgroups v2 | Periodic (1-5s) | CPU, memory, disk I/O, network bytes, GPU |
| **Activity** | eBPF | Event-driven | Process exec, network connect/accept, socket data, memory mappings, file access |

**eBPF Event Types:**

| Event | CLI Display | Description |
|-------|------------|-------------|
| exec | EXEC | Process executions (fork/exec syscalls) |
| connect | NET | Outgoing network connections (connect syscall) |
| accept | ACCEPT | Incoming network connections (accept syscall) |
| socket_data | SEND/RECV | Socket data transfers (sendto/recvfrom syscalls) |
| mmap | MMAP | Memory mappings with executable permissions |
| mprotect | MPROTECT | Memory protection changes adding exec permission |
| file | FILE | File access (open/read/write) - optional, high volume |

### Unified Event Model

```go
// internal/joblet/telemetry/event.go

type TelemetryEvent struct {
    Timestamp time.Time     `json:"timestamp"`
    JobID     string        `json:"job_id"`
    Type      TelemetryType `json:"type"`
    Data      interface{}   `json:"data"`
}

type TelemetryType string

const (
    TelemetryMetrics  TelemetryType = "metrics"
    TelemetryExec     TelemetryType = "exec"
    TelemetryConnect  TelemetryType = "connect"
    TelemetryFile     TelemetryType = "file"
)

// Metrics data (from cgroups)
type MetricsData struct {
    CPUPercent    float64 `json:"cpu_percent"`
    MemoryBytes   int64   `json:"memory_bytes"`
    MemoryLimit   int64   `json:"memory_limit"`
    DiskReadBytes int64   `json:"disk_read_bytes"`
    DiskWriteBytes int64  `json:"disk_write_bytes"`
    NetRecvBytes  int64   `json:"net_recv_bytes"`
    NetSentBytes  int64   `json:"net_sent_bytes"`
    GPUPercent    float64 `json:"gpu_percent,omitempty"`
    GPUMemoryBytes int64  `json:"gpu_memory_bytes,omitempty"`
}

// Activity data (from eBPF)
type ExecData struct {
    PID     uint32   `json:"pid"`
    Binary  string   `json:"binary"`
    Args    []string `json:"args"`
    ExitCode int32   `json:"exit_code,omitempty"`
}

type ConnectData struct {
    PID      uint32 `json:"pid"`
    Address  string `json:"address"`
    Port     uint16 `json:"port"`
    Protocol string `json:"protocol"`  // tcp, udp
}

type FileData struct {
    PID       uint32 `json:"pid"`
    Path      string `json:"path"`
    Operation string `json:"operation"`  // read, write, create
}
```

### Architecture

**Key Constraint**: Core joblet has NO AWS dependencies. CloudWatch integration lives in persist service only.

```
┌─────────────────────────────────────────────────────────────────────┐
│                    JOBLET (core binary)                             │
│              Native Linux only, no AWS SDK                          │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  ┌─────────────────┐              ┌─────────────────┐               │
│  │ Metrics Collector│              │  eBPF Monitor   │               │
│  │ (cgroups v2)     │              │ (cilium/ebpf)   │               │
│  └────────┬─────────┘              └────────┬────────┘               │
│           │                                  │                        │
│           │ MetricsData                      │ ExecData/ConnectData   │
│           │                                  │                        │
│           └──────────────┬───────────────────┘                       │
│                          ▼                                           │
│              ┌───────────────────────┐                              │
│              │  Telemetry Collector  │                              │
│              │  - Unify events       │                              │
│              │  - Buffer/batch       │                              │
│              └───────────┬───────────┘                              │
│                          │                                           │
│           ┌──────────────┴──────────────┐                           │
│           ▼                             ▼                           │
│     ┌──────────┐              ┌──────────────┐                      │
│     │  gRPC    │              │ Unix Socket  │                      │
│     │ Stream   │              │ IPC to       │                      │
│     │(clients) │              │ Persist      │                      │
│     └──────────┘              └──────┬───────┘                      │
│                                      │                               │
└──────────────────────────────────────┼───────────────────────────────┘
                                       │
                                       ▼
┌─────────────────────────────────────────────────────────────────────┐
│                    PERSIST (separate binary)                        │
│                    May have AWS SDK if configured                   │
├─────────────────────────────────────────────────────────────────────┤
│                          │                                           │
│           ┌──────────────┴──────────────┐                           │
│           ▼                             ▼                           │
│     ┌───────────┐               ┌────────────┐                      │
│     │   Local   │               │ CloudWatch │                      │
│     │  Storage  │               │ (optional) │                      │
│     └───────────┘               └────────────┘                      │
└─────────────────────────────────────────────────────────────────────┘
```

### Storage Backends

#### Local (Standalone VM)

```
/var/lib/joblet/telemetry/
├── {job_id}/
│   └── events.jsonl     # All telemetry, line-delimited JSON
```

Format:
```json
{"timestamp":"2025-12-04T10:30:00Z","job_id":"abc123","type":"metrics","data":{"cpu_percent":45.2,"memory_bytes":2147483648}}
{"timestamp":"2025-12-04T10:30:01Z","job_id":"abc123","type":"exec","data":{"pid":1234,"binary":"python","args":["train.py"]}}
{"timestamp":"2025-12-04T10:30:02Z","job_id":"abc123","type":"connect","data":{"pid":1234,"address":"10.0.1.50","port":5432,"protocol":"tcp"}}
```

#### AWS CloudWatch

```
CloudWatch Logs:
  Log Group: /joblet/{node_id}
  Log Streams:
    - {job_id}-logs         # stdout/stderr logs
    - {job_id}-metrics      # Resource metrics
    - {job_id}-exec-events  # Process execution events (eBPF)
    - {job_id}-connect-events # Network connection events (eBPF)
  Format: JSON

CloudWatch Metrics (for dashboards):
  Namespace: Joblet/Jobs
  Dimensions: JobID, NodeID
  Metrics: CPUPercent, MemoryBytes, GPUPercent
```

CloudWatch Insights query examples:
```sql
-- Query exec events for a job
fields @timestamp, pid, filename, args
| filter @logStream like "abc123-exec-events"
| sort @timestamp desc
| limit 100

-- Query network connections for a job
fields @timestamp, pid, dst_addr, dst_port, protocol
| filter @logStream like "abc123-connect-events"
| sort @timestamp desc
| limit 100

-- Find all processes that connected to a specific host
fields @timestamp, job_id, pid, comm, dst_addr, dst_port
| filter dst_addr = "10.0.1.50"
| sort @timestamp desc
```

### gRPC API

```protobuf
service JobService {
    // Existing RPCs...

    // Stream live telemetry (metrics + activity)
    rpc StreamJobTelemetry(StreamTelemetryRequest) returns (stream TelemetryEvent);

    // Get historical telemetry
    rpc GetJobTelemetry(GetTelemetryRequest) returns (stream TelemetryEvent);
}

message StreamTelemetryRequest {
    string job_uuid = 1;
    repeated string types = 2;  // ["metrics", "exec", "connect", "file"] or empty for all
}

message GetTelemetryRequest {
    string job_uuid = 1;
    repeated string types = 2;
    int64 start_time = 3;  // Unix nanos
    int64 end_time = 4;
    int32 limit = 5;
}

message TelemetryEvent {
    int64 timestamp = 1;
    string job_id = 2;
    string type = 3;

    oneof data {
        MetricsData metrics = 10;
        ExecData exec = 11;
        ConnectData connect = 12;
        FileData file = 13;
    }
}

message MetricsData {
    double cpu_percent = 1;
    int64 memory_bytes = 2;
    int64 memory_limit = 3;
    int64 disk_read_bytes = 4;
    int64 disk_write_bytes = 5;
    int64 net_recv_bytes = 6;
    int64 net_sent_bytes = 7;
    double gpu_percent = 8;
    int64 gpu_memory_bytes = 9;
}

message ExecData {
    uint32 pid = 1;
    string binary = 2;
    repeated string args = 3;
    int32 exit_code = 4;
}

message ConnectData {
    uint32 pid = 1;
    string address = 2;
    uint32 port = 3;
    string protocol = 4;
}

message FileData {
    uint32 pid = 1;
    string path = 2;
    string operation = 3;
}
```

### CLI Integration

The `rnx job metrics` command provides unified telemetry viewing (like `rnx job log`):

```bash
# View metrics - smart behavior based on job status
$ rnx job metrics <job-id>
# - If job is RUNNING  → streams live metrics
# - If job is COMPLETED → shows historical metrics, then exits

# Include eBPF telemetry events
$ rnx job metrics <job-id> --tel

# Short UUIDs are supported (first 8 characters)
$ rnx job metrics f47ac10b --tel

# Filter eBPF events with grep
$ rnx job metrics f47ac10b --tel | grep EXEC     # Process executions
$ rnx job metrics f47ac10b --tel | grep NET      # Outgoing connections
$ rnx job metrics f47ac10b --tel | grep ACCEPT   # Incoming connections
$ rnx job metrics f47ac10b --tel | grep MMAP     # Memory mappings with exec

# JSON output
$ rnx --json job metrics f47ac10b
```

### Configuration

```yaml
# /opt/joblet/config/config.yml (joblet core)
telemetry:
  # What to collect
  metrics:
    enabled: true
    interval: 5s

  activity:
    enabled: true  # requires eBPF support (Linux 5.8+)
    events:
      exec: true      # process execution
      connect: true   # network connections
      file: false     # file access (high volume, optional)
```

```yaml
# /opt/joblet/config/persist.yml (persist service - separate)
storage:
  type: local  # or "cloudwatch"

  # Local storage settings
  local:
    path: /var/lib/joblet/telemetry
    retention_days: 7

  # CloudWatch settings (only if type: cloudwatch)
  cloudwatch:
    region: us-west-2
    log_group: /joblet/telemetry
```

### eBPF Component

```
internal/joblet/ebpf/
├── visibility/
│   ├── monitor.go           # Go monitor, emits TelemetryEvents
│   ├── bpf/
│   │   ├── visibility.c     # eBPF program
│   │   ├── vmlinux.h
│   │   └── visibility_bpfel.go  # Generated
│   └── cgroup.go            # Cgroup ID helpers
```

The eBPF monitor emits the same `TelemetryEvent` type as metrics, just with different event types.

### File Structure

```
# Joblet core (no AWS deps)
internal/joblet/
├── telemetry/
│   ├── collector.go         # Unified telemetry collector
│   ├── event.go             # TelemetryEvent types
│   └── sender.go            # Send to persist via IPC
├── metrics/
│   └── collector.go         # Cgroups metrics (existing, refactored)
└── ebpf/
    └── visibility/          # eBPF activity tracking (new)
        ├── monitor.go       # Go monitor using cilium/ebpf
        ├── bpf/
        │   ├── visibility.c # eBPF C program
        │   └── vmlinux.h
        └── cgroup.go        # Cgroup ID helpers

# Persist service (may have AWS deps)
persist/internal/
├── storage/
│   ├── local.go             # Local file storage
│   └── cloudwatch.go        # CloudWatch storage (optional)
└── telemetry/
    └── handler.go           # Receive from joblet, store
```

## Consequences

### The Good

**Unified pipeline**: One system for all job observability data. Same storage, same API, same retention.

**Flexible storage**: Works on standalone VMs (local files) or AWS (CloudWatch). Same data format.

**Single query point**: Debug jobs with one command that shows metrics AND activity together.

**CloudWatch native**: Activity events work with CloudWatch Insights for powerful queries across all jobs.

**Extensible**: Easy to add new telemetry types (GPU events, custom metrics) through the same pipeline.

### Trade-offs

**eBPF requirement**: Activity events require Linux 5.8+ and eBPF support. Metrics work without it.

**Storage growth**: Activity events add volume. File events especially can be high volume (disabled by default).

**Migration**: Existing metrics storage format changes. Need migration path for persist service.

### Future Enhancements

1. **Seccomp profiles**: Generate from observed syscalls
2. **Anomaly detection**: Alert on unusual patterns
3. **Cost attribution**: Link telemetry to resource costs
4. **Distributed tracing**: Correlate jobs across nodes

## Implementation Plan

### Phase 1: Unified Telemetry Framework ✅ COMPLETED
- Define TelemetryEvent types in joblet (`internal/joblet/telemetry/event.go`)
- Implement TelemetryCollector (`internal/joblet/telemetry/collector.go`)
- Refactor existing metrics to emit TelemetryEvents
- IPC sender to persist service (`internal/joblet/ipc/persister.go`)

### Phase 2: Persist Service Updates ✅ COMPLETED
- Update persist to receive telemetry events (exec, connect events via IPC)
- Local storage backend for telemetry (`exec_events.jsonl.gz`, `connect_events.jsonl.gz`)
- CloudWatch storage backend (`{jobID}-exec-events`, `{jobID}-connect-events` streams)

### Phase 3: eBPF Activity Tracking ✅ COMPLETED
- eBPF program for execve, connect (`internal/joblet/ebpf/visibility/bpf/visibility.c`)
- Go monitor using cilium/ebpf (`internal/joblet/ebpf/visibility/monitor.go`)
- Integration with TelemetryCollector via EventPersister interface
- Job lifecycle hooks (start/stop monitoring by cgroup ID)

### Phase 4: gRPC API ✅ COMPLETED
- StreamJobTelemetry RPC (live)
- GetJobTelemetry RPC (historical)
- Single unified response format

### Phase 5: CLI ✅ COMPLETED
- `rnx job metrics --tel` command for unified metrics + eBPF telemetry
- Smart live/historical behavior (like `rnx job log`)
- Short UUID support (first 8 characters)
- Grep-friendly output format (EXEC, NET, ACCEPT, MMAP, etc.)

## Requirements

**Joblet core:**
- Linux kernel 5.8+ (for eBPF ring buffer)
- `CAP_BPF` and `CAP_PERFMON` (joblet runs as root)
- Build: `clang`, `llvm`, kernel headers (for eBPF C program)
- Go dependency: `github.com/cilium/ebpf` (exception to native-only rule)
- Runtime: No additional dependencies (eBPF bytecode embedded)

**Persist service:**
- Optional: AWS SDK (only if CloudWatch storage configured)

## References

- [ADR-007: Cgroups v2 Resource Management](007-cgroups-v2-resource-management.md)
- [ADR-010: Collect Jobs Metrics](010-collect-jobs-metrics.md)
- [ADR-011: CQRS Architecture with Persist](011-cqrs-architecture-with-persist.md)
- [cilium/ebpf Go library](https://github.com/cilium/ebpf)
- [CloudWatch Logs Insights](https://docs.aws.amazon.com/AmazonCloudWatch/latest/logs/AnalyzingLogData.html)
