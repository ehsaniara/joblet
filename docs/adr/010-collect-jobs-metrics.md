# ADR: Job Metrics Collection and Storage Architecture

## Status

**IMPLEMENTED** - Metrics collection is fully implemented and enabled by default as of v4.5.2.

### Implementation Summary

- ✅ **Metrics Collection**: CPU, Memory, I/O, Network, Process, and GPU metrics collected from cgroups v2
- ✅ **Time-Series Storage**: Metrics stored as gzipped JSON Lines files in `/opt/joblet/metrics/`
- ✅ **Historical Replay**: Complete metrics timeline available for completed jobs
- ✅ **Live Streaming**: Real-time metrics streaming via gRPC with pub/sub
- ✅ **Streaming Completeness**: Drain mode ensures final metrics samples are captured (v5.0.0)
- ✅ **Short UUID Support**: Commands work with both full and short (8-char) UUIDs
- ✅ **CLI Commands**: `rnx job metrics` streams historical + live metrics (no flags needed)
- ⏳ **Retention Cleanup**: Automatic cleanup based on retention period (TODO)
- ⏳ **Aggregations/Rollups**: Historical data aggregation for long-term storage (TODO)

### Key Design Decisions Made

1. **Simplified Configuration**: Reduced from complex nested structure to 4 simple fields:
    - `enabled` (bool)
    - `default_sample_rate` (duration)
    - `storage_dir` (string)
    - `retention_days` (int)

2. **Storage Format**: JSONL with gzip compression
    - Provides good compression (~10x) while remaining human-readable
    - One job metrics file per job: `/opt/joblet/metrics/<uuid>/<timestamp>.jsonl.gz`

3. **Historical + Live Streaming**: Metrics stream serves historical data first, then live data
    - Users can replay complete job timeline automatically
    - For completed jobs: shows all metrics then exits
    - For running jobs: shows historical metrics then continues streaming live until completion

4. **No Pub/Sub for Storage**: Metrics are written directly to disk, pub/sub is optional
    - Simplifies architecture
    - Reduces memory overhead
    - Pub/sub used only for live-streaming to clients

5. **UUID Resolution**: Disk reader resolves short UUIDs by scanning metrics directory
    - Allows viewing metrics for completed jobs no longer in memory
    - Fallback when job store lookup fails

## Context

Joblet currently implements a pub-sub mechanism for job log streaming with the following characteristics:

- **Producer**: Job process writes logs to a channel
- **Subscribers**: Multiple consumers can subscribe (1 disk writer + N streaming clients)
- **Storage**: AsyncLogSystem persists logs to local disk
- **Streaming**: Live streaming to multiple `rnx log <job-id>` clients

We need a similar architecture for **time-series job metrics** (CPU, memory, I/O, network) with configurable sample
rates.

### Current Architecture Analysis

#### Existing Log Streaming Pattern

```
Job Process → [Log Channel] → ├── Disk Writer Subscriber
                              └── Client Stream Subscriber(s)
```

#### System Monitoring (exists but separate)

- Host-level metrics collection via `/internal/joblet/monitoring`
- Cgroups v2 provides per-job resource statistics
- No job-specific metrics persistence or streaming

## Decision

Implement a **parallel metrics collection system** using the same pub-sub pattern as logs, with time-series
considerations.

## Architecture Design

### Key Components

#### 1. Metrics Collector (Producer)

- **Responsibility**: Read cgroup statistics at configured intervals
- **Location**: Runs as goroutine alongside job process
- **Lifecycle**: Starts with job, stops with job completion
- **Data Source**: `/sys/fs/cgroup/joblet.slice/joblet.service/job-{id}/`
- **Publishing**: Emits to `metrics.job.{jobID}` topic

#### 2. Metrics Channel/Topic

- **Pattern**: Same as logs - `topic = "metrics.job.{jobID}"`
- **Buffer Size**: Configurable (default: 1000 samples)
- **Message Type**: Time-series metric event with timestamp
- **Overflow Strategy**: Drop oldest (metrics are sampled, loss acceptable)

#### 3. Storage Subscriber

- **Pattern**: Single persistent subscriber per job
- **Storage Format**: Time-series optimized format (JSONL or binary)
- **File Organization**: `/opt/joblet/metrics/{job-id}/{timestamp}.metrics`
- **Rotation**: Size or time-based (e.g., hourly files)
- **Retention**: Configurable cleanup (e.g., 7-30 days)

#### 4. Streaming Subscribers

- **Pattern**: Multiple concurrent subscribers
- **Use Cases**:
    - Real-time monitoring dashboards
    - CLI streaming (`rnx job metrics`)
    - External metric collectors
- **Backpressure**: Slow consumers dropped (same as logs)

### Data Flow Patterns

#### Collection Flow

```
Cgroup Stats → Collector → Normalize → Publish → Topic
     ↑                                              │
     └──────── Sample Rate Timer ──────────────────┘
```

#### Storage Flow

```
Topic → Storage Subscriber → Buffer → Batch Write → Disk
                                │
                                └→ Rotation Check → Archive
```

#### Query Flow

```
Historical: Disk Files → Reader → Deserialize → Filter → Response
Live:       Topic → Stream Subscriber → Filter → Stream Response
```

### Configuration Design

this or similar

```yaml
metrics:
  # Global settings
  enabled: true
  default_sample_rate: 5s

  # Collection settings
  collection:
    min_interval: 1s          # Rate limiting
    max_interval: 60s         # Maximum sample period
    batch_size: 100           # Points per write

  # Storage settings  
  storage:
    directory: /opt/joblet/metrics
    format: jsonl             # jsonl, protobuf, or msgpack
    compression: gzip         # none, gzip, zstd
    retention:
      days: 30
    rotation:
      strategy: size          # size, time, or hybrid
      max_file_size: 100MB
      time_interval: 1h

  # Pub-Sub settings
  pubsub:
    buffer_size: 1000
    max_subscribers: 100
    subscriber_timeout: 30s
```

### Time-Series Considerations

#### Sampling Strategy

- **Fixed Interval**: Default 5s, configurable per job
- **Adaptive Sampling**: Increase rate during high activity
- **Aligned Timestamps**: Round to nearest second for aggregation
- **Missing Data**: Use previous value or interpolate

#### Storage Optimization

```
Raw Data (5s) → 1-min Rollups → 5-min Rollups → Hourly Summaries
   (7 days)        (30 days)       (90 days)        (1 year)
```

#### Metric Types

- **Gauges**: Memory usage, CPU percent (last value)
- **Counters**: Network bytes, I/O operations (cumulative)
- **Histograms**: Response times, queue depths (distributions)
- **Rates**: Calculated from counters (bytes/sec, ops/sec)

### API Design

#### gRPC Service Extensions

```protobuf
service JobService {
  // Existing log streaming
  rpc GetJobLogs(GetJobLogsReq) returns (stream GetJobLogsRes);

  // Metrics streaming endpoint (IMPLEMENTED)
  rpc StreamJobMetrics(JobMetricsRequest) returns (stream JobMetricsResponse);

  // Historical queries handled by persist.QueryMetrics service
  // Aggregation should be performed client-side using StreamJobMetrics
}
```

#### CLI Commands

```bash
# Stream metrics (historical + live for running jobs)
rnx job metrics <job-id>

# JSON output
rnx --json job metrics <job-id>

# Future enhancements (not yet implemented):
# Historical query
# rnx job metrics <job-id> --from="1h ago" --to="now"
#
# Aggregated view
# rnx job metrics <job-id> --summary --period=5m
#
# Export formats
# rnx job metrics <job-id> --format=prometheus
```

### Comparison with Log System

| Aspect             | Log System       | Metrics System           |
|--------------------|------------------|--------------------------|
| **Data Type**      | Text streams     | Time-series numbers      |
| **Volume**         | Variable (KB-GB) | Predictable (fixed rate) |
| **Retention**      | Days-weeks       | Weeks-months             |
| **Loss Tolerance** | Zero (critical)  | Acceptable (sampled)     |
| **Storage Format** | Plain text       | Compressed binary        |
| **Query Pattern**  | Sequential read  | Time-range queries       |
| **Aggregation**    | Not needed       | Required (rollups)       |

### Integration Points

#### With Existing Components

1. **ResourceManager**: Start/stop collector with job lifecycle
2. **CgroupManager**: Reuse cgroup paths and validation
3. **AsyncLogSystem**: Share storage management patterns
4. **PubSub System**: Reuse channel infrastructure
5. **JobStoreAdapter**: Extend for metrics streaming

#### With External Systems

- **Prometheus**: Export endpoint or push gateway
- **Grafana**: JSON datasource API
- **CloudWatch/Datadog**: Forwarding agents
- **Time-series DBs**: InfluxDB, TimescaleDB exporters

## Consequences

### Positive

1. **Consistency**: Reuses proven pub-sub pattern from logs
2. **Scalability**: Multiple consumers without collector impact
3. **Flexibility**: Per-job sample rates and retention
4. **Real-time**: Live streaming for monitoring
5. **Historical**: Time-series queries for analysis
6. **Low Overhead**: Single collector goroutine per job

### Negative

1. **Storage Growth**: ~2MB/job/hour at 5s intervals
2. **Complexity**: Additional subsystem to maintain
3. **Memory Usage**: Buffering for batched writes
4. **Query Performance**: Historical queries need indexing

### Risks & Mitigations

| Risk                    | Impact | Mitigation                                |
|-------------------------|--------|-------------------------------------------|
| **Storage Exhaustion**  | High   | Automatic retention policies, compression |
| **Collection Overhead** | Medium | Rate limiting, cgroup cache               |
| **Subscriber Overload** | Low    | Backpressure, max subscriber limits       |
| **Data Loss**           | Low    | Acceptable for metrics (sampled data)     |

## Alternatives Considered

### Alternative 1: Direct Database Write

- **Pros**: No intermediate pub-sub, direct persistence
- **Cons**: No real-time streaming, tight coupling
- **Rejected**: Doesn't support multiple consumers

### Alternative 2: Prometheus Node Exporter

- **Pros**: Standard tooling, existing ecosystem
- **Cons**: External dependency, complex setup
- **Rejected**: Not embedded/self-contained

### Alternative 3: In-Memory Ring Buffer

- **Pros**: Fast, simple
- **Cons**: Data loss on restart, limited history
- **Rejected**: Need persistent historical data

### Alternative 4: Extend Log System

- **Pros**: Reuse everything, simple
- **Cons**: Mixing concerns, not optimized for metrics
- **Rejected**: Different data characteristics

## Metrics Collection Details

### Metrics Collected from Jobs

All metrics are collected from the job's cgroup v2 statistics at `/sys/fs/cgroup/joblet.slice/joblet.service/job-{id}/`.
Each metric includes a timestamp and job ID for correlation.

#### CPU Metrics

Collected from `cpu.stat`, `cpu.pressure`, and `cpu.max`:

| Metric                 | Source File  | Description                             | Type    |
|------------------------|--------------|-----------------------------------------|---------|
| `cpu.usage_usec`       | cpu.stat     | Total CPU time consumed in microseconds | Counter |
| `cpu.user_usec`        | cpu.stat     | CPU time in user mode                   | Counter |
| `cpu.system_usec`      | cpu.stat     | CPU time in kernel mode                 | Counter |
| `cpu.nr_periods`       | cpu.stat     | Number of enforcement periods           | Counter |
| `cpu.nr_throttled`     | cpu.stat     | Number of throttled periods             | Counter |
| `cpu.throttled_usec`   | cpu.stat     | Total throttled time                    | Counter |
| `cpu.usage_percent`    | Calculated   | Current CPU usage percentage            | Gauge   |
| `cpu.throttle_percent` | Calculated   | Percentage of time throttled            | Gauge   |
| `cpu.pressure.some`    | cpu.pressure | PSI some pressure (10s, 60s, 300s avg)  | Gauge   |
| `cpu.pressure.full`    | cpu.pressure | PSI full pressure (10s, 60s, 300s avg)  | Gauge   |

#### Memory Metrics

Collected from `memory.current`, `memory.stat`, `memory.events`, `memory.pressure`:

| Metric                  | Source File     | Description                      | Type    |
|-------------------------|-----------------|----------------------------------|---------|
| `memory.current`        | memory.current  | Current memory usage in bytes    | Gauge   |
| `memory.max`            | memory.max      | Memory limit in bytes            | Gauge   |
| `memory.usage_percent`  | Calculated      | Memory usage percentage          | Gauge   |
| `memory.anon`           | memory.stat     | Anonymous memory                 | Gauge   |
| `memory.file`           | memory.stat     | File-backed memory (cache)       | Gauge   |
| `memory.kernel_stack`   | memory.stat     | Kernel stack memory              | Gauge   |
| `memory.slab`           | memory.stat     | Kernel slab memory               | Gauge   |
| `memory.sock`           | memory.stat     | Socket buffer memory             | Gauge   |
| `memory.shmem`          | memory.stat     | Shared memory                    | Gauge   |
| `memory.file_mapped`    | memory.stat     | Memory-mapped files              | Gauge   |
| `memory.file_dirty`     | memory.stat     | Dirty page cache                 | Gauge   |
| `memory.file_writeback` | memory.stat     | Pages under writeback            | Gauge   |
| `memory.pgfault`        | memory.stat     | Page fault count                 | Counter |
| `memory.pgmajfault`     | memory.stat     | Major page fault count           | Counter |
| `memory.oom_events`     | memory.events   | OOM killer invocations           | Counter |
| `memory.oom_kill`       | memory.events   | Processes killed by OOM          | Counter |
| `memory.pressure.some`  | memory.pressure | Memory pressure (10s, 60s, 300s) | Gauge   |
| `memory.pressure.full`  | memory.pressure | Full memory pressure             | Gauge   |

#### I/O Metrics

Collected from `io.stat`, `io.pressure`, `io.max`:

| Metric             | Source File   | Description                   | Type    |
|--------------------|---------------|-------------------------------|---------|
| `io.rbytes`        | io.stat       | Bytes read per device         | Counter |
| `io.wbytes`        | io.stat       | Bytes written per device      | Counter |
| `io.rios`          | io.stat       | Read operations per device    | Counter |
| `io.wios`          | io.stat       | Write operations per device   | Counter |
| `io.dbytes`        | io.stat       | Bytes discarded               | Counter |
| `io.dios`          | io.stat       | Discard operations            | Counter |
| `io.read_bps`      | Calculated    | Read bandwidth (bytes/sec)    | Gauge   |
| `io.write_bps`     | Calculated    | Write bandwidth (bytes/sec)   | Gauge   |
| `io.read_iops`     | Calculated    | Read IOPS                     | Gauge   |
| `io.write_iops`    | Calculated    | Write IOPS                    | Gauge   |
| `io.pressure.some` | io.pressure   | I/O pressure (10s, 60s, 300s) | Gauge   |
| `io.pressure.full` | io.pressure   | Full I/O stall                | Gauge   |
| `io.bfq_weight`    | io.bfq.weight | I/O scheduler weight          | Gauge   |

#### Network Metrics

Collected from network namespace statistics (if network isolation enabled):

| Metric           | Source        | Description                 | Type    |
|------------------|---------------|-----------------------------|---------|
| `net.rx_bytes`   | /proc/net/dev | Bytes received              | Counter |
| `net.tx_bytes`   | /proc/net/dev | Bytes transmitted           | Counter |
| `net.rx_packets` | /proc/net/dev | Packets received            | Counter |
| `net.tx_packets` | /proc/net/dev | Packets transmitted         | Counter |
| `net.rx_errors`  | /proc/net/dev | Receive errors              | Counter |
| `net.tx_errors`  | /proc/net/dev | Transmit errors             | Counter |
| `net.rx_dropped` | /proc/net/dev | Packets dropped on receive  | Counter |
| `net.tx_dropped` | /proc/net/dev | Packets dropped on transmit | Counter |
| `net.rx_bps`     | Calculated    | Receive bandwidth           | Gauge   |
| `net.tx_bps`     | Calculated    | Transmit bandwidth          | Gauge   |

#### Process Metrics

Collected from `pids.current`, `pids.events`, and `/proc` filesystem:

| Metric                   | Source File        | Description                         | Type    |
|--------------------------|--------------------|-------------------------------------|---------|
| `pids.current`           | pids.current       | Current number of processes/threads | Gauge   |
| `pids.max`               | pids.max           | Maximum PIDs allowed                | Gauge   |
| `pids.events`            | pids.events        | PID limit hit count                 | Counter |
| `process.threads`        | /proc/[pid]/status | Total thread count                  | Gauge   |
| `process.state.running`  | /proc/[pid]/stat   | Running processes                   | Gauge   |
| `process.state.sleeping` | /proc/[pid]/stat   | Sleeping processes                  | Gauge   |
| `process.state.stopped`  | /proc/[pid]/stat   | Stopped processes                   | Gauge   |
| `process.state.zombie`   | /proc/[pid]/stat   | Zombie processes                    | Gauge   |
| `process.open_fds`       | /proc/[pid]/fd     | Open file descriptors               | Gauge   |
| `process.max_fds`        | /proc/[pid]/limits | FD limit                            | Gauge   |

#### GPU Metrics (NVIDIA/CUDA)

For jobs with GPU allocation (`--gpu=N`), collect from `nvidia-smi` and NVIDIA Management Library (NVML):

| Metric                       | Source     | Description                          | Type    |
|------------------------------|------------|--------------------------------------|---------|
| `gpu.index`                  | nvidia-smi | GPU device index                     | Label   |
| `gpu.uuid`                   | nvidia-smi | GPU unique identifier                | Label   |
| `gpu.name`                   | nvidia-smi | GPU model name (e.g., "Tesla V100")  | Label   |
| `gpu.compute_capability`     | NVML       | CUDA compute capability              | Label   |
| `gpu.driver_version`         | nvidia-smi | NVIDIA driver version                | Label   |
| **Utilization Metrics**      |            |                                      |         |
| `gpu.utilization`            | nvidia-smi | GPU core utilization (0-100%)        | Gauge   |
| `gpu.memory.used`            | nvidia-smi | GPU memory used (MiB)                | Gauge   |
| `gpu.memory.total`           | nvidia-smi | GPU memory total (MiB)               | Gauge   |
| `gpu.memory.free`            | nvidia-smi | GPU memory free (MiB)                | Gauge   |
| `gpu.memory.percent`         | Calculated | Memory utilization percentage        | Gauge   |
| `gpu.encoder.utilization`    | nvidia-smi | Video encoder utilization            | Gauge   |
| `gpu.decoder.utilization`    | nvidia-smi | Video decoder utilization            | Gauge   |
| **Performance Metrics**      |            |                                      |         |
| `gpu.sm_clock`               | nvidia-smi | Streaming Multiprocessor clock (MHz) | Gauge   |
| `gpu.memory_clock`           | nvidia-smi | Memory clock speed (MHz)             | Gauge   |
| `gpu.pcie_throughput.rx`     | nvidia-smi | PCIe receive throughput (MB/s)       | Gauge   |
| `gpu.pcie_throughput.tx`     | nvidia-smi | PCIe transmit throughput (MB/s)      | Gauge   |
| **Thermal & Power**          |            |                                      |         |
| `gpu.temperature`            | nvidia-smi | GPU temperature (Celsius)            | Gauge   |
| `gpu.temperature.memory`     | nvidia-smi | Memory temperature (if available)    | Gauge   |
| `gpu.power.draw`             | nvidia-smi | Current power draw (Watts)           | Gauge   |
| `gpu.power.limit`            | nvidia-smi | Power limit (Watts)                  | Gauge   |
| `gpu.fan.speed`              | nvidia-smi | Fan speed percentage                 | Gauge   |
| **Error & Health**           |            |                                      |         |
| `gpu.ecc.errors.single`      | nvidia-smi | Single-bit ECC errors                | Counter |
| `gpu.ecc.errors.double`      | nvidia-smi | Double-bit ECC errors                | Counter |
| `gpu.xid.errors`             | nvidia-smi | XID error events                     | Counter |
| `gpu.retired.pages`          | nvidia-smi | Retired memory pages                 | Counter |
| `gpu.clock.throttle_reasons` | nvidia-smi | Throttling reason bitmask            | Gauge   |
| **Process Metrics**          |            |                                      |         |
| `gpu.processes.count`        | nvidia-smi | Number of processes using GPU        | Gauge   |
| `gpu.processes.memory_used`  | nvidia-smi | Memory used by job processes         | Gauge   |
| `gpu.compute_mode`           | nvidia-smi | Compute mode (exclusive/shared)      | Label   |

#### Multi-GPU Considerations

For jobs with multiple GPUs (`--gpu=4`):

- Metrics collected per GPU device
- Aggregated metrics also provided:
    - `gpu.total.utilization`: Average utilization across all GPUs
    - `gpu.total.memory.used`: Total memory used across all GPUs
    - `gpu.total.power.draw`: Total power consumption
    - `gpu.imbalance.score`: Load imbalance indicator (0-1)

#### CUDA-Specific Metrics

When CUDA applications are running:

| Metric                   | Source     | Description                      | Type    |
|--------------------------|------------|----------------------------------|---------|
| `cuda.kernel.launches`   | CUPTI      | Number of kernel launches        | Counter |
| `cuda.memory.transfers`  | CUPTI      | Host-device memory transfers     | Counter |
| `cuda.memory.bandwidth`  | CUPTI      | Memory bandwidth utilization     | Gauge   |
| `cuda.nvlink.throughput` | nvidia-smi | NVLink throughput (if available) | Gauge   |
| `cuda.tensor.core.usage` | DCGM       | Tensor core utilization          | Gauge   |

#### GPU Collection Challenges & Solutions

**Challenge 1: nvidia-smi overhead**

- Solution: Cache results for 1-2 seconds when multiple metrics requested
- Impact: ~50ms per collection vs ~500ms for repeated calls

**Challenge 2: GPU access in containers/namespaces**

- Solution: Collect from host, correlate via `CUDA_VISIBLE_DEVICES`
- Device mapping maintained in job metadata

**Challenge 3: Different GPU models have different metrics**

- Solution: Graceful degradation - collect available metrics
- Mark unavailable metrics as null/missing

#### Job Metadata (Context)

Additional context included with each metric sample:

| Field              | Description                   |
|--------------------|-------------------------------|
| `job_id`           | Unique job identifier         |
| `job_name`         | User-provided job name        |
| `runtime`          | Runtime environment (if used) |
| `timestamp`        | Collection time (RFC3339)     |
| `sample_interval`  | Current sampling interval     |
| `cgroup_path`      | Full cgroup path              |
| `limits.cpu`       | Configured CPU limit          |
| `limits.memory`    | Configured memory limit       |
| `limits.io`        | Configured I/O limit          |
| `gpu.allocation`   | GPU indices allocated to job  |
| `gpu.memory_limit` | GPU memory limit per device   |

### Calculation Methods

#### Rate Calculations

For counter metrics, rates are calculated using:

```
rate = (current_value - previous_value) / time_delta
```

#### Percentage Calculations

- **CPU**: `(usage_delta / (time_delta * cpu_cores)) * 100`
- **Memory**: `(current / limit) * 100`
- **Throttle**: `(throttled_time / total_time) * 100`

#### Pressure Stall Information (PSI)

PSI metrics show the percentage of time processes were stalled:

- **some**: At least one task stalled on resource
- **full**: All tasks stalled on resource
- Provided as 10-second, 60-second, and 300-second averages

### Collection Frequency Considerations

Different metrics have different optimal collection frequencies:

| Metric Category       | Recommended Interval | Rationale                                          |
|-----------------------|----------------------|----------------------------------------------------|
| CPU & Memory          | 5-10 seconds         | Balance between accuracy and overhead              |
| I/O Statistics        | 10-30 seconds        | I/O patterns need longer samples                   |
| Network               | 5-10 seconds         | Match traffic patterns                             |
| Process States        | 30-60 seconds        | Slow-changing metrics                              |
| PSI Metrics           | 10-30 seconds        | Already averaged by kernel                         |
| **GPU Metrics**       | **2-5 seconds**      | **GPU states change rapidly, nvidia-smi overhead** |
| **GPU Power/Thermal** | **5-10 seconds**     | **Thermal changes are gradual**                    |

#### Distributed Training Metrics

For multi-GPU training jobs using NCCL/NVLink:

| Metric                     | Source     | Description                         | Type      |
|----------------------------|------------|-------------------------------------|-----------|
| `gpu.nvlink.throughput.rx` | nvidia-smi | NVLink receive throughput per link  | Gauge     |
| `gpu.nvlink.throughput.tx` | nvidia-smi | NVLink transmit throughput per link | Gauge     |
| `gpu.nvlink.errors`        | nvidia-smi | NVLink transmission errors          | Counter   |
| `gpu.p2p.enabled`          | nvidia-smi | Peer-to-peer enabled status         | Boolean   |
| `gpu.p2p.throughput`       | DCGM       | P2P memory bandwidth                | Gauge     |
| `nccl.allreduce.time`      | NCCL_TRACE | AllReduce operation time            | Histogram |
| `nccl.broadcast.time`      | NCCL_TRACE | Broadcast operation time            | Histogram |
| `nccl.reduce.time`         | NCCL_TRACE | Reduce operation time               | Histogram |

#### MIG (Multi-Instance GPU) Support

For NVIDIA A100/H100 GPUs with MIG enabled:

| Metric                 | Source     | Description                  | Type  |
|------------------------|------------|------------------------------|-------|
| `gpu.mig.instance_id`  | nvidia-smi | MIG instance identifier      | Label |
| `gpu.mig.profile`      | nvidia-smi | MIG profile (e.g., "1g.5gb") | Label |
| `gpu.mig.gi_id`        | nvidia-smi | GPU Instance ID              | Label |
| `gpu.mig.ci_id`        | nvidia-smi | Compute Instance ID          | Label |
| `gpu.mig.memory.total` | nvidia-smi | MIG instance memory          | Gauge |
| `gpu.mig.memory.used`  | nvidia-smi | MIG instance memory used     | Gauge |
| `gpu.mig.sm_count`     | nvidia-smi | Number of SMs in instance    | Gauge |

#### GPU-Specific Design Decisions

1. **Host-Level Collection**: GPU metrics collected at host level, not within job namespace
    - Rationale: nvidia-smi requires privileged access to GPU driver
    - Job correlation via `CUDA_VISIBLE_DEVICES` environment variable

2. **Caching Strategy**: Single nvidia-smi call cached for 1-2 seconds
    - Rationale: Each nvidia-smi call takes ~50ms
    - Multiple metrics extracted from single call

3. **Allocation Tracking**: Maintain GPU → Job mapping in memory
    - Updated when job starts/stops
    - Used to attribute GPU metrics to correct job

4. **Metric Granularity**: Per-GPU metrics, not aggregated
    - Allows detection of GPU imbalance
    - Essential for multi-GPU debugging

### Data Volume Estimation

At 5-second intervals with all metrics enabled:

| Component                  | Size per Sample | Samples/Hour | Hour Total        |
|----------------------------|-----------------|--------------|-------------------|
| CPU Metrics                | ~200 bytes      | 720          | 144 KB            |
| Memory Metrics             | ~400 bytes      | 720          | 288 KB            |
| I/O Metrics                | ~300 bytes      | 720          | 216 KB            |
| Network Metrics            | ~200 bytes      | 720          | 144 KB            |
| Process Metrics            | ~150 bytes      | 720          | 108 KB            |
| **GPU Metrics (per GPU)**  | **~500 bytes**  | **720**      | **360 KB**        |
| **Total per Job (no GPU)** | **~1250 bytes** | **720**      | **~900 KB/hour**  |
| **Total per Job (1 GPU)**  | **~1750 bytes** | **720**      | **~1.26 MB/hour** |
| **Total per Job (4 GPUs)** | **~3250 bytes** | **720**      | **~2.34 MB/hour** |

With compression (gzip ~70% reduction):

- **No GPU**: ~270 KB/hour per job
- **1 GPU**: ~378 KB/hour per job
- **4 GPUs**: ~702 KB/hour per job

### GPU Health Monitoring & Alerting

#### Critical GPU Thresholds

| Metric                  | Warning Threshold | Critical Threshold | Action                 |
|-------------------------|-------------------|--------------------|------------------------|
| Temperature             | >80°C             | >85°C              | Throttle/Alert         |
| Memory Usage            | >90%              | >95%               | Alert/Possible OOM     |
| Power Draw              | >90% of limit     | >95% of limit      | Throttle imminent      |
| ECC Errors (Double-bit) | >0                | >10 per hour       | Hardware issue         |
| XID Errors              | Any non-zero      | >5 per hour        | Driver/Hardware issue  |
| PCIe Throughput         | <50% expected     | <25% expected      | Bottleneck detected    |
| GPU Utilization         | <10% for 5 min    | -                  | Underutilization alert |

#### GPU-Specific Error Handling

```yaml
gpu_metrics:
  error_handling:
    nvidia_smi_failure:
      retry_count: 3
      retry_delay: 1s
      fallback: skip_gpu_metrics

    permission_denied:
      action: log_warning
      continue: true

    gpu_lost:
      action: alert_critical
      mark_job: gpu_failure

    ecc_errors:
      threshold: 10
      action: notify_admin
      job_action: continue_with_warning
```

## Success Criteria

1. **Performance**: <1% CPU overhead per job
2. **Latency**: <100ms from collection to storage
3. **Storage**: <10GB for 1000 job-hours
4. **Reliability**: 99.9% collection success rate
5. **Scalability**: 10,000 concurrent metrics streams

## References

- [Existing Log Streaming Architecture](./LOG_STREAMING.md)
- [Cgroups v2 Statistics](https://www.kernel.org/doc/html/latest/admin-guide/cgroup-v2.html#cpu)
- [Time-Series Best Practices](https://docs.influxdata.com/influxdb/v2.0/write-data/best-practices/)
- [Prometheus Data Model](https://prometheus.io/docs/concepts/data_model/)