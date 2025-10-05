# Job Metrics Configuration Guide

## Overview

Joblet includes comprehensive job metrics collection to monitor resource usage (CPU, memory, I/O, GPU) for all running jobs. Metrics are collected periodically and can be streamed in real-time or stored for historical analysis.

## Current Status

**Metrics are ENABLED by default** (as of the current release). Metrics collection provides comprehensive resource usage tracking with minimal performance overhead. You can customize the configuration in your `joblet-config.yml`.

## Configuration Structure

### Basic Configuration

Add the following section to your `/opt/joblet/joblet-config.yml` (or your custom config path):

```yaml
job_metrics:
  enabled: true                       # Enable metrics collection
  default_sample_rate: 5s             # Sample every 5 seconds
  storage_dir: "/opt/joblet/metrics"  # Where to store metrics files
  retention_days: 7                   # Keep metrics for 7 days
```

### Full Configuration Example

```yaml
job_metrics:
  enabled: true                       # Enable metrics collection
  default_sample_rate: 5s             # Sample every 5 seconds
  storage_dir: "/opt/joblet/metrics"  # Where to store metrics files
  retention_days: 7                   # Keep metrics for 7 days
```

## Prerequisites

Before enabling metrics, ensure:

1. **Directory exists and is writable**:
   ```bash
   sudo mkdir -p /opt/joblet/metrics
   sudo chown joblet:joblet /opt/joblet/metrics  # Or appropriate user
   sudo chmod 755 /opt/joblet/metrics
   ```

2. **Sufficient disk space**: Metrics can consume significant disk space depending on:
   - Number of jobs
   - Sample rate
   - Retention period

## What Metrics Are Collected?

For each job, the following metrics are collected at the configured sample interval:

### CPU Metrics
- Usage percentage
- User time / System time (microseconds)
- Throttling (when CPU limit is hit)
- CPU pressure (PSI - Pressure Stall Information)

### Memory Metrics
- Current usage and peak usage
- Anonymous memory (heap, stack)
- File cache
- Page faults (minor and major)
- OOM events
- Memory pressure (PSI)

### I/O Metrics
- Read/Write bytes and IOPS (per device and aggregated)
- I/O rates (bytes/sec)
- I/O pressure (PSI)

### Process Metrics
- Process count
- Thread count
- Open file descriptors
- Process limit events

### GPU Metrics (if GPUs allocated)
- GPU utilization percentage
- GPU memory usage (used/total/free)
- Temperature
- Power draw
- Clock speeds (SM and memory)
- ECC errors
- Process count and memory

### Network Metrics
- RX/TX bytes and packets
- Network rates (bytes/sec)
- Errors and drops

## Using Metrics

### CLI Commands

#### View Metrics for a Job
```bash
# Shows all metrics from job start (similar to logs)
rnx job metrics <job-uuid>

# Also supports short UUIDs
rnx job metrics f47ac10b
```

#### Get Metrics as JSON
```bash
# Stream all metrics as JSON (one sample per line)
rnx --json job metrics <job-uuid>
```

### How Metrics Streaming Works

Joblet provides complete time-series metrics similar to logs:

1. **Historical Metrics First**: The system reads all historical samples from disk (stored as gzipped JSON Lines files in `/opt/joblet/metrics/<job-uuid>/`)

2. **Live Metrics Second**: For running jobs, after replaying all historical data, the stream continues with live real-time metrics until the job completes

3. **Works for Completed Jobs**: You can view complete metrics history even after a job has finished and is no longer in memory

**Behavior:**
- **For completed jobs**: Shows all metrics from start to finish, then exits
- **For running jobs**: Shows all metrics from start to current, then continues streaming live until job completes or Ctrl+C

**Example Usage:**
```bash
# View metrics for a completed job (shows complete history then exits)
rnx job metrics f47ac10b

# View metrics for a running job (shows history + live stream)
rnx job metrics a1b2c3d4

# This will show:
# - Sample 1 (job start): 0% CPU, 12KB memory
# - Sample 2 (5s later): 0.4% CPU, 704KB memory
# - Sample 3 (10s later): 0.06% CPU, 716KB memory
# ... all samples, continuing live if job is still running
```

### Example Output

```
═══ Metrics Sample at 15:23:45 ═══
Job ID: abc123-def456
Sample Interval: 5s

CPU:
  Usage: 45.32%
  Throttled: 0.00%
  User Time: 123456 μs
  System Time: 45678 μs

Memory:
  Current: 512.0 MiB (50.00%)
  Peak: 768.0 MiB
  Anonymous: 256.0 MiB
  File Cache: 256.0 MiB

I/O:
  Read: 1.5 MiB/s (150 ops/s)
  Write: 512.0 KiB/s (50 ops/s)
  Total Read: 150.0 MiB
  Total Write: 50.0 MiB

Processes:
  Count: 3 (max: 100)
  Threads: 12
  Open FDs: 45/1024

GPU:
  GPU 0 (Tesla T4):
    Utilization: 78.5%
    Memory: 3.2 GiB / 16.0 GiB (20.0%)
    Temperature: 65.0°C
    Power: 45.2W / 70.0W
```

## Performance Impact

Metrics collection has minimal overhead:
- **CPU**: < 0.5% per job
- **Memory**: ~10MB per active collector
- **I/O**: Depends on sample rate and storage backend
  - With 5s sampling and gzip compression: ~10KB/hour per job

## Troubleshooting

### Metrics Not Showing
1. Check if metrics are enabled: `grep "job_metrics" /opt/joblet/joblet-config.yml`
2. Check if directory exists: `ls -la /opt/joblet/metrics`
3. Check joblet logs: `journalctl -u joblet.service | grep metrics`

### Permission Errors
```bash
sudo chown -R joblet:joblet /opt/joblet/metrics
sudo chmod 755 /opt/joblet/metrics
```

### Disk Space Issues
- Reduce retention period
- Increase compression level
- Increase rotation file size to reduce file count

## Production Recommendations

For production use:

1. **Enable metrics gradually**: Start with a few jobs to gauge impact
2. **Monitor disk usage**: Set up alerts for metrics directory size
3. **Adjust sample rate**: Use 10s or 15s for less critical workloads
4. **Set appropriate retention**: 7-30 days is usually sufficient
5. **Use compression**: `gzip` provides good balance of compression ratio and speed
6. **Consider external storage**: For long-term retention, consider exporting to time-series DB

## Storage Format

Metrics are stored as gzipped JSON Lines files:

- **Location**: `/opt/joblet/metrics/<job-uuid>/<timestamp>.jsonl.gz`
- **Format**: One JSON object per line (JSONL)
- **Compression**: gzip (provides ~10x compression)
- **Retention**: Automatically cleaned up after configured retention period

### File Structure Example
```
/opt/joblet/metrics/
├── f47ac10b-58cc-4372-a567-0e02b2c3d479/
│   └── 20251004-153045.jsonl.gz  # Gzipped JSON Lines
└── a1b2c3d4-e5f6-7890-1234-567890abcdef/
    └── 20251004-160230.jsonl.gz
```

### Reading Metrics Files Directly
```bash
# Decompress and view metrics
gzip -dc /opt/joblet/metrics/<job-uuid>/*.jsonl.gz | head -5

# Parse with jq
gzip -dc /opt/joblet/metrics/<job-uuid>/*.jsonl.gz | jq -c '{timestamp, cpu: .cpu.usage_percent}'
```

## Short UUID Support

All metrics commands support short UUIDs (first 8 characters):

```bash
# Full UUID
rnx job metrics f47ac10b-58cc-4372-a567-0e02b2c3d479

# Short UUID (equivalent)
rnx job metrics f47ac10b
```

The system will resolve the short UUID by:
1. First checking active jobs in memory
2. Then searching the metrics directory on disk for matching folders

## Migration from Previous Versions

If upgrading from a version without metrics:
1. Metrics are now enabled by default
2. Metrics directory will be created automatically
3. Restart joblet service to apply: `sudo systemctl restart joblet.service`
4. Metrics will be collected for all new jobs automatically

## API Access

Metrics are also available via gRPC API:

```go
client.StreamJobMetrics(ctx, &pb.JobMetricsRequest{
    Uuid: jobID,
})
```

See `api/gen/joblet.proto` for full API documentation.
