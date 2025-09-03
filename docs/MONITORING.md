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

- Complete remote server overview
- Server cloud environment detection
- Server-side volume usage statistics
- Server process state breakdown
- Server network interface details

**Usage:**

```bash
rnx monitor status                    # Human-readable server status
rnx monitor status --json            # JSON format (server data)
rnx --node=production monitor status # Specific server node
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

- **Interface Statistics**: All network interfaces
- **Traffic Throughput**: Real-time RX/TX rates
- **Packet Counts**: Received/transmitted packets
- **Error Statistics**: Network errors and drops

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
rnx list --json | jq '.[] | select(.status=="running")'
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
rnx run --max-cpu=50 heavy-computation.py
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

## Related Documentation

- [RNX CLI Reference](./RNX_CLI_REFERENCE.md) - Complete CLI command reference
- [Volume Management](./VOLUME_MANAGEMENT.md) - Managing persistent volumes
- [Troubleshooting](./TROUBLESHOOTING.md) - Common issues and solutions
- [Admin UI](./ADMIN_UI.md) - Web-based monitoring interface

For additional help, run `rnx monitor --help` or see the [troubleshooting guide](./TROUBLESHOOTING.md).