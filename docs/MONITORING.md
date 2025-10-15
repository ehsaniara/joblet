# Remote System Monitoring Guide

Comprehensive guide to monitoring remote joblet server resources and performance using RNX's client-side monitoring
capabilities.

## Table of Contents

- [Overview](#overview)
- [Client-Server Architecture](#client-server-architecture)
- [Quick Start](#quick-start)
- [Monitoring Commands](#monitoring-commands)
- [Metrics Types](#metrics-types)
- [JSON Integration](#json-integration)
- [Dashboard Integration](#dashboard-integration)
- [Troubleshooting](#troubleshooting)
- [Best Practices](#best-practices)

## Overview

RNX provides comprehensive **remote** monitoring capabilities from your client machine/workstation that track joblet
server resources:

- **Remote Server Resources**: CPU, memory, disk, and network utilization on the joblet server
- **Joblet Volumes**: Volume usage and availability across all server volume types
- **Server Process Information**: Running processes, resource consumption, and states on the joblet host
- **Cloud Environment**: Automatic detection of joblet server's cloud provider and instance details
- **Performance Metrics**: Real-time throughput, I/O operations, and load averages from the server

### Key Features

✅ **Remote Monitoring**: Monitor joblet server resources from your local workstation  
✅ **Client-Server Architecture**: Secure gRPC communication with mTLS authentication  
✅ **Volume Tracking**: Automatic detection and monitoring of server-side joblet volumes  
✅ **Cloud Detection**: Support for AWS, GCP, Azure, KVM, and bare metal server detection  
✅ **JSON Output**: UI-compatible format for dashboards and monitoring tools  
✅ **Resource Filtering**: Monitor specific server resources (CPU, memory, disk, network)  
✅ **Process Analysis**: Top consumers by CPU and memory usage on the server

## Client-Server Architecture

### How Remote Monitoring Works

```
┌─────────────────────┐    gRPC/mTLS     ┌─────────────────────┐
│   Client Machine    │ ◄──────────────► │   Joblet Server     │
│                     │                  │                     │
│  ┌───────────────┐  │                  │  ┌───────────────┐  │
│  │ rnx monitor   │  │   Monitor Req    │  │ Monitoring    │  │
│  │ (from laptop/ │  │ ──────────────►  │  │ Service       │  │
│  │ workstation)  │  │                  │  │               │  │
│  │               │  │   Metrics Data   │  │ Collects:     │  │
│  │ Displays:     │  │ ◄──────────────  │  │ - CPU/Memory  │  │
│  │ - Server CPU  │  │                  │  │ - Disk Usage  │  │
│  │ - Server Mem  │  │                  │  │ - Volumes     │  │
│  │ - Server Disk │  │                  │  │ - Processes   │  │
│  │ - Volumes     │  │                  │  │ - Network     │  │
│  └───────────────┘  │                  │  └───────────────┘  │
└─────────────────────┘                  └─────────────────────┘
```

### Configuration Requirements

**Client Side (Your Workstation):**

- RNX CLI installed with proper configuration file
- Network connectivity to joblet server
- Valid mTLS certificates for authentication

**Server Side (Joblet Host):**

- Joblet server running with monitoring service enabled
- gRPC port accessible (default: 50051)
- Monitoring collectors gathering system metrics

### Multi-Node Monitoring

Monitor different joblet servers from a single client:

```bash
# Monitor production server
rnx --node=production monitor status

# Monitor staging server  
rnx --node=staging monitor status

# Monitor development server
rnx --node=dev monitor watch --interval=5
```

## Quick Start

### Basic Remote Server Status

```bash
# Get comprehensive overview of joblet server resources
rnx monitor status

# JSON output for dashboards/APIs (server metrics)
rnx monitor status --json

# Monitor specific joblet server node
rnx --node=production monitor status
```

### Real-time Server Monitoring

```bash  
# Watch all server metrics from your workstation (5s refresh)
rnx monitor watch

# Faster refresh rate for real-time server monitoring
rnx monitor watch --interval=2

# Monitor specific server resources remotely
rnx monitor watch --filter=cpu,memory,disk
```

### Current Server Metrics

```bash
# Show current server metrics with top processes
rnx monitor top

# Filter by server resource type
rnx monitor top --filter=disk,network

# JSON output for monitoring tools (server data)
rnx monitor top --json
```

## Monitoring Commands

### `rnx monitor status`

Displays comprehensive **remote server** status including all server resources and joblet volumes.

**Features:**

- **Complete remote server overview** with version information
- **Joblet server version** - Full version, git tag, commit hash, build date
- **Host information** - Hostname, OS, kernel, uptime, node ID
- **Network interfaces** - IP addresses, MAC addresses, traffic statistics per interface
- **Server cloud environment detection** - AWS, GCP, Azure, KVM support
- **Server-side volume usage statistics** - All joblet volumes tracked
- **Server process state breakdown** - Running, sleeping, zombie processes
- **Resource utilization** - CPU, memory, disk, I/O metrics

**Enhanced Display (v4.7.2+):**

The status command now includes:

1. **Joblet Server Version Information** - Shows version, git tag, commit, build date, Go version
2. **Network Interface Details** - Each interface displays:
    - IP address (intelligently mapped to interfaces)
    - MAC address (hardware address)
    - RX/TX statistics with real-time rates
    - Packet counts and error tracking

**Usage:**

```bash
rnx monitor status                    # Server status with version info
rnx monitor status --json            # JSON format (server data)
rnx --node=production monitor status # Specific server node
```

**Example Output:**

```
System Status - 2025-10-08T18:45:14Z
Available: true

Host Information:
  Hostname:     joblet-server
  OS:           Ubuntu 22.04.2 LTS
  Kernel:       5.15.0-153-generic
  Architecture: amd64
  Uptime:       33d 4h 58m
  Node ID:      8eb41e22-2940-4f83-9066-7d739d057ad2
  Server IPs:   192.168.1.161, 172.20.0.1
  MAC Addresses: 5e:9f:b0:c0:61:22, 1e:45:87:fe:bc:53

Joblet Server:
  Version:      v4.7.2
  Git Tag:      v4.7.2
  Git Commit:   00df3a5ee3d6ae7e25078d610e05b977cd8a1812
  Build Date:   2025-10-08T18:44:40Z
  Go Version:   go1.24.0
  Platform:     linux/amd64

Network Interfaces:
  ens18:
    IP:   192.168.1.161
    MAC:  5e:9f:b0:c0:61:22
    RX:   16.1 GB (10264780 packets, 0 errors)
    TX:   986.9 MB (3558641 packets, 0 errors)
    Rate: RX 167 B/s TX 699 B/s
  joblet0:
    IP:   172.20.0.1
    MAC:  1e:45:87:fe:bc:53
    RX:   5.7 MB (73297 packets, 0 errors)
    TX:   6.7 MB (73490 packets, 0 errors)
    Rate: RX 0 B/s TX 0 B/s
```

### `rnx monitor top`

Shows current **remote server** metrics in a condensed format with top resource consumers.

**Features:**

- Top server processes by CPU and memory
- Server resource utilization summary
- Server network throughput rates
- Server disk I/O statistics

**Usage:**

```bash
rnx monitor top                          # All server metrics
rnx monitor top --filter=cpu,memory      # Specific server metrics only
rnx monitor top --json                   # JSON output (server data)
```

### `rnx monitor watch`

Real-time **remote server** monitoring with configurable refresh intervals.

**Features:**

- Live server metric updates from your workstation
- Configurable refresh rate for server monitoring
- Server resource filtering
- JSON streaming support for server data
- Compact display mode for server metrics

**Usage:**

```bash
rnx monitor watch                            # Default 5s server monitoring
rnx monitor watch --interval=1               # 1s server refresh
rnx monitor watch --filter=disk,network      # Specific server resources
rnx monitor watch --compact                  # Compact server format
rnx monitor watch --json --interval=10       # JSON server streaming
```

## Metrics Types

### CPU Metrics

- **Usage Percentage**: Overall CPU utilization
- **Load Averages**: 1, 5, and 15-minute load averages
- **Per-Core Usage**: Individual core utilization
- **Time Breakdowns**: User, system, idle, I/O wait, steal time

### Memory Metrics

- **Total/Used/Available**: Memory allocation details
- **Cache and Buffers**: System memory optimization
- **Swap Usage**: Virtual memory utilization
- **Usage Percentage**: Memory utilization rate

### Disk Metrics

- **Mount Points**: All filesystem mount points
- **Joblet Volumes**: Volume usage and availability
- **Space Usage**: Total, used, available, percentage
- **I/O Statistics**: Read/write operations and throughput
- **Inode Usage**: Filesystem metadata utilization

### Network Metrics

- **Interface Statistics**: All network interfaces with IP and MAC addresses
- **IP Addresses**: Per-interface IP address assignment (IPv4 and IPv6)
- **MAC Addresses**: Hardware addresses for physical network interfaces
- **Traffic Throughput**: Real-time RX/TX rates (bytes per second)
- **Packet Counts**: Received/transmitted packets with error tracking
- **Error Statistics**: Network errors and drops per interface
- **Rate Calculation**: Automatic throughput rate computation

**NetworkCollector Implementation:**
The monitoring system uses a dedicated NetworkCollector that:

- Reads interface statistics from `/proc/net/dev` for bandwidth metrics
- Retrieves actual IP addresses and MAC addresses directly from each interface using Go's `net` package
- Calculates real-time throughput metrics
- Filters virtual interfaces (veth, bridges, loopback)
- Tracks cumulative and rate-based metrics
- Supports both IPv4 and IPv6 addressing
- **No heuristics or guessing** - all interface data comes directly from the system

Location: `/internal/joblet/monitoring/collectors/network.go`

### Process Metrics

- **Process States**: Running, sleeping, stopped, zombie counts
- **Top Consumers**: Processes by CPU and memory usage
- **Thread Information**: Total thread counts
- **Resource Usage**: Per-process CPU and memory consumption

### Volume Metrics

- **Volume Detection**: Automatic discovery of joblet volumes
- **Usage Statistics**: Space utilization per volume
- **Volume Types**: Filesystem and memory volume support
- **Mount Information**: Volume mount points and paths

## JSON Integration

### Output Structure

The `--json` flag produces structured output optimized for dashboard integration:

```json
{
  "hostInfo": {
    "hostname": "joblet-server",
    "platform": "Ubuntu 22.04.2 LTS", 
    "arch": "amd64",
    "uptime": 152070,
    "cloudProvider": "AWS",
    "instanceType": "t3.medium",
    "region": "us-east-1"
  },
  "cpuInfo": {
    "cores": 8,
    "usage": 0.15,
    "loadAverage": [0.5, 0.3, 0.2],
    "perCoreUsage": [0.1, 0.2, 0.05, 0.3, 0.18, 0.07, 0.12, 0.09]
  },
  "memoryInfo": {
    "total": 4100255744,
    "used": 378679296,
    "available": 3556278272,
    "percent": 9.23,
    "cached": 1835712512,
    "swap": {
      "total": 2147479552,
      "used": 0,
      "percent": 0
    }
  },
  "disksInfo": {
    "disks": [
      {
        "name": "/dev/sda1",
        "mountpoint": "/",
        "filesystem": "ext4", 
        "size": 19896352768,
        "used": 11143790592,
        "available": 8752562176,
        "percent": 56.01
      },
      {
        "name": "analytics-data",
        "mountpoint": "/opt/joblet/volumes/analytics-data",
        "filesystem": "joblet-volume",
        "size": 1073741824,
        "used": 52428800,
        "available": 1021313024,
        "percent": 4.88
      }
    ],
    "totalSpace": 21936726016,
    "usedSpace": 11196219392
  },
  "networkInfo": {
    "interfaces": [
      {
        "name": "eth0",
        "type": "ethernet",
        "status": "up",
        "rxBytes": 1234567890,
        "txBytes": 987654321,
        "rxPackets": 123456,
        "txPackets": 98765
      }
    ],
    "totalRxBytes": 1234567890,
    "totalTxBytes": 987654321
  },
  "processesInfo": {
    "processes": [
      {
        "pid": 1234,
        "name": "joblet",
        "command": "/opt/joblet/joblet",
        "cpu": 2.5,
        "memory": 1.2,
        "memoryBytes": 49152000,
        "status": "sleeping"
      }
    ],
    "totalProcesses": 149
  }
}
```

### Streaming JSON

For real-time monitoring integrations:

```bash
# Stream JSON objects every 10 seconds
rnx monitor watch --json --interval=10

# Process with monitoring tools
rnx monitor watch --json | jq '.cpuInfo.usage'

# Forward to monitoring systems
rnx monitor watch --json --interval=30 | logger -t joblet-metrics
```

## Dashboard Integration

### Grafana Integration

Create a data source using the JSON output:

```bash
#!/bin/bash
# grafana-collector.sh
while true; do
  rnx monitor status --json > /var/lib/grafana/joblet-metrics.json
  sleep 60
done
```

### Prometheus Integration

Export metrics in Prometheus format:

```bash
#!/bin/bash
# prometheus-exporter.sh
METRICS=$(rnx monitor status --json)
CPU_USAGE=$(echo "$METRICS" | jq -r '.cpuInfo.usage')
MEMORY_PERCENT=$(echo "$METRICS" | jq -r '.memoryInfo.percent')

echo "joblet_cpu_usage $CPU_USAGE"  
echo "joblet_memory_percent $MEMORY_PERCENT"
```

### Custom Dashboards

Use the JSON API to build custom monitoring dashboards:

```javascript
// JavaScript example
async function getJobletMetrics() {
  const { exec } = require('child_process');
  
  return new Promise((resolve, reject) => {
    exec('rnx monitor status --json', (error, stdout) => {
      if (error) reject(error);
      else resolve(JSON.parse(stdout));
    });
  });
}

// Usage
const metrics = await getJobletMetrics();
console.log(`CPU Usage: ${metrics.cpuInfo.usage * 100}%`);
console.log(`Memory Usage: ${metrics.memoryInfo.percent}%`);
```

## Troubleshooting

### Common Issues

**No Volume Statistics Showing**

```bash
# Check if volumes exist
rnx volume list

# Create test volume
rnx volume create test-monitoring --size=100MB

# Verify monitoring detects it
rnx monitor status --json | grep "joblet-volume"
```

**High Resource Usage**

```bash
# Identify resource-heavy processes
rnx monitor top --filter=process

# Monitor specific resources
rnx monitor watch --filter=cpu,memory --interval=1

# Check for resource-intensive jobs
rnx job list --json | jq '.[] | select(.status=="running")'
```

**Network Monitoring Issues**

```bash
# Check active interfaces
rnx monitor status | grep -A 10 "Network Interfaces"

# Monitor network activity
rnx monitor watch --filter=network --interval=2
```

### Performance Optimization

**Reduce Monitoring Overhead**

```bash
# Use longer intervals for production
rnx monitor watch --interval=30

# Filter to essential metrics only
rnx monitor watch --filter=cpu,memory

# Use compact format for less output
rnx monitor watch --compact
```

**Efficient JSON Processing**

```bash
# Extract specific metrics only
rnx monitor status --json | jq '.cpuInfo'

# Monitor specific volumes
rnx monitor status --json | jq '.disksInfo.disks[] | select(.filesystem=="joblet-volume")'
```

## Best Practices

### 1. Regular Monitoring

```bash
# Set up automated monitoring
*/5 * * * * rnx monitor status --json > /var/log/joblet/metrics-$(date +%Y%m%d-%H%M).json
```

### 2. Resource Thresholds

```bash
# Create alerting scripts
#!/bin/bash
CPU_USAGE=$(rnx monitor status --json | jq -r '.cpuInfo.usage')
if (( $(echo "$CPU_USAGE > 0.8" | bc -l) )); then
  echo "ALERT: High CPU usage: $(echo "$CPU_USAGE * 100" | bc)%"
fi
```

### 3. Volume Management

```bash
# Monitor volume usage regularly
rnx monitor status --json | jq '.disksInfo.disks[] | select(.filesystem=="joblet-volume") | {name, percent}'

# Clean up unused volumes
rnx volume list | grep -v "in-use"
```

### 4. Performance Monitoring

```bash
# Monitor job performance impact
rnx monitor watch --filter=cpu,memory &
rnx job run --max-cpu=50 heavy-computation.py
```

### 5. Historical Tracking

```bash
# Log metrics for trend analysis  
rnx monitor status --json | jq '{timestamp: now, cpu: .cpuInfo.usage, memory: .memoryInfo.percent}' >> metrics.jsonl
```

### 6. Integration Testing

```bash
# Test monitoring integration
rnx monitor status --json | jq . > /dev/null && echo "JSON valid" || echo "JSON invalid"

# Verify all metrics present
REQUIRED_FIELDS="hostInfo cpuInfo memoryInfo disksInfo networkInfo processesInfo"
for field in $REQUIRED_FIELDS; do
  rnx monitor status --json | jq ".$field" > /dev/null || echo "Missing: $field"
done
```

### 7. Persist Service Monitoring

The persist service handles historical log and metric storage. Monitor its health and performance:

```bash
# Check persist service status (on server)
ssh server "systemctl status joblet-persist"

# View persist service logs
ssh server "journalctl -u joblet-persist -n 100 -f"

# Check IPC socket connectivity
ssh server "ls -la /opt/joblet/run/persist.sock"

# Monitor storage usage for logs and metrics
ssh server "du -sh /opt/joblet/logs /opt/joblet/metrics"

# Check for persist service errors
ssh server "journalctl -u joblet-persist --since '1 hour ago' | grep -i error"
```

**Persist Service Metrics:**

The persist service exposes its own metrics for monitoring:

```bash
# Persist service health (if gRPC endpoint is enabled)
curl http://server:9093/health

# Prometheus metrics (if enabled)
curl http://server:9092/metrics | grep persist_
```

**Key Metrics to Monitor:**

- **IPC Write Latency**: Average time to write logs/metrics via Unix socket
- **Storage Usage**: Disk space consumed by `/opt/joblet/logs` and `/opt/joblet/metrics`
- **Compression Ratio**: Efficiency of gzip compression (typically ~80%)
- **Query Latency**: Time to retrieve historical logs/metrics
- **Error Rate**: Failed writes or storage errors

**Storage Management:**

```bash
# Check current storage usage
ssh server "df -h /opt/joblet"

# Find largest log directories
ssh server "du -sh /opt/joblet/logs/* | sort -hr | head -10"

# Check metric storage
ssh server "du -sh /opt/joblet/metrics/* | sort -hr | head -10"

# Monitor storage growth rate
ssh server "watch -n 60 'du -sh /opt/joblet/logs /opt/joblet/metrics'"
```

**Automated Monitoring:**

```bash
# Log persist metrics to JSONL for analysis
while true; do
  ssh server "du -sk /opt/joblet/logs /opt/joblet/metrics" | \
    awk '{print "{\"timestamp\":" systime() ",\"path\":\"" $2 "\",\"size_kb\":" $1 "}"}' >> persist-metrics.jsonl
  sleep 300  # Every 5 minutes
done

# Alert on high storage usage
THRESHOLD=80  # Alert at 80% usage
USAGE=$(ssh server "df /opt/joblet | tail -1 | awk '{print \$5}' | sed 's/%//'")
if [ "$USAGE" -gt "$THRESHOLD" ]; then
  echo "ALERT: Persist storage at ${USAGE}% (threshold: ${THRESHOLD}%)"
fi
```

**Performance Tuning:**

Monitor persist service performance and adjust configuration:

```yaml
# In /opt/joblet/config/joblet-config.yml
persist:
  writer:
    flush_interval: "1s"      # Increase to reduce I/O, decrease for lower latency
    batch_size: 100           # Higher = better throughput, more memory

  query:
    cache:
      ttl: "5m"               # Cache query results to reduce disk I/O
    stream:
      buffer_size: 1024       # Buffer size for streaming queries
```

**Troubleshooting Persist Service:**

```bash
# Service not running
ssh server "sudo systemctl restart joblet-persist"

# Check if socket exists
ssh server "sudo ls -la /opt/joblet/run/persist.sock"

# Verify socket permissions (should be 600)
ssh server "sudo stat /opt/joblet/run/persist.sock"

# Test IPC connectivity from joblet service
ssh server "sudo lsof | grep persist.sock"

# Check for disk space issues
ssh server "df -h /opt/joblet && df -i /opt/joblet"
```

**Best Practices:**

1. **Monitor Storage Growth**: Set up alerts for storage thresholds
2. **Regular Cleanup**: Configure retention policies to auto-delete old data
3. **Performance Baseline**: Establish normal IPC latency and query times
4. **Backup Strategy**: Include `/opt/joblet/logs` and `/opt/joblet/metrics` in backups
5. **Log Rotation**: Ensure persist service logs don't fill up disk

---

## Related Documentation

- [RNX CLI Reference](./RNX_CLI_REFERENCE.md) - Complete CLI command reference
- [Volume Management](./VOLUME_MANAGEMENT.md) - Managing persistent volumes
- [Troubleshooting](./TROUBLESHOOTING.md) - Common issues and solutions
- [Admin UI](./ADMIN_UI.md) - Web-based monitoring interface
- [Persist Service Testing](../tests/e2e/PERSIST_TESTING.md) - E2E testing for persist service
- [Architecture](./ARCHITECTURE.md) - Persist service architecture and communication

For additional help, run `rnx monitor --help` or see the [troubleshooting guide](./TROUBLESHOOTING.md).