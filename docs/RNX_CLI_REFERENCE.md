# RNX CLI Reference

Complete reference for the RNX command-line interface, including all commands, options, and examples.

## Table of Contents

- [Global Options](#global-options)
- [Job Commands](#job-commands)
  - [run](#rnx-run)
  - [list](#rnx-list)
  - [status](#rnx-status)
  - [log](#rnx-log)
  - [stop](#rnx-stop)
- [Volume Commands](#volume-commands)
  - [volume create](#rnx-volume-create)
  - [volume list](#rnx-volume-list)
  - [volume remove](#rnx-volume-remove)
- [Network Commands](#network-commands)
  - [network create](#rnx-network-create)
  - [network list](#rnx-network-list)
  - [network delete](#rnx-network-delete)
- [System Commands](#system-commands)
  - [monitor](#rnx-monitor)
  - [nodes](#rnx-nodes)
  - [help](#rnx-help)

## Global Options

Options available for all commands:

```bash
--config <path>    # Path to configuration file (default: searches standard locations)
--node <name>      # Node name from configuration (default: "default")
--help, -h         # Show help for command
--version, -v      # Show version information
```

### Configuration File Locations

RNX searches for configuration in this order:
1. `./rnx-config.yml`
2. `./config/rnx-config.yml`
3. `~/.rnx/rnx-config.yml`
4. `/etc/joblet/rnx-config.yml`
5. `/opt/joblet/config/rnx-config.yml`

## Job Commands

### `rnx run`

Execute a command on the Joblet server.

```bash
rnx run [flags] <command> [args...]
```

#### Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--max-cpu` | Maximum CPU usage percentage (0-10000) | 0 (unlimited) |
| `--max-memory` | Maximum memory in MB | 0 (unlimited) |
| `--max-iobps` | Maximum I/O bytes per second | 0 (unlimited) |
| `--cpu-cores` | CPU cores to use (e.g., "0-3" or "1,3,5") | "" (all cores) |
| `--network` | Network mode: bridge, host, none, or custom | "bridge" |
| `--volume` | Volume to mount (can be specified multiple times) | none |
| `--env` | Environment variable (can be specified multiple times) | none |
| `--workdir` | Working directory inside container | "/work" |
| `--upload` | Upload file to workspace (can be specified multiple times) | none |
| `--upload-dir` | Upload directory to workspace | none |
| `--schedule` | Schedule job execution (duration or RFC3339 time) | immediate |
| `--name` | Job name for identification | auto-generated |

#### Examples

```bash
# Simple command
rnx run echo "Hello, World!"

# With resource limits
rnx run --max-cpu=50 --max-memory=512 --max-iobps=10485760 \
  python3 intensive_script.py

# CPU core binding
rnx run --cpu-cores="0-3" stress-ng --cpu 4 --timeout 60s

# Multiple volumes
rnx run --volume=data --volume=config \
  python3 process.py

# Environment variables
rnx run --env=DEBUG=true --env=API_KEY=secret \
  node app.js

# File upload
rnx run --upload=script.py --upload=data.csv \
  python3 script.py data.csv

# Directory upload
rnx run --upload-dir=./project \
  npm start

# Scheduled execution
rnx run --schedule="30min" backup.sh
rnx run --schedule="2025-08-03T15:00:00" maintenance.sh

# Custom network
rnx run --network=isolated ping google.com

# Complex example
rnx run \
  --max-cpu=200 \
  --max-memory=2048 \
  --cpu-cores="0,2,4,6" \
  --network=mynet \
  --volume=persistent-data \
  --env=PYTHONPATH=/app \
  --upload-dir=./src \
  --workdir=/work/src \
  python3 main.py --process-data
```

### `rnx list`

List all jobs on the server.

```bash
rnx list [flags]
```

#### Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--json` | Output in JSON format | false |

#### Output Format

**Table Format (default):**
- **ID**: Job identifier  
- **STATUS**: Current job status (RUNNING, COMPLETED, FAILED, STOPPED, SCHEDULED)
- **START TIME**: When the job started (format: YYYY-MM-DD HH:MM:SS)
- **COMMAND**: The command being executed (truncated to 80 chars if too long)

**JSON Format:**
Outputs a JSON array with detailed job information including all resource limits, volumes, network, and scheduling information.

#### Examples

```bash
# List all jobs (table format)
rnx list

# Example output:
# ID     STATUS      START TIME           COMMAND
# -----  ----------  -------------------  -------
# 1      COMPLETED   2025-08-03 10:15:32  echo "Hello World"
# 2      RUNNING     2025-08-03 10:16:45  python3 script.py
# 3      FAILED      2025-08-03 10:17:20  invalid_command
# 4      SCHEDULED   N/A                  backup.sh

# JSON output for scripting
rnx list --json

# Example JSON output:
# [
#   {
#     "id": "1",
#     "status": "COMPLETED",
#     "start_time": "2025-08-03T10:15:32Z",
#     "end_time": "2025-08-03T10:15:33Z",
#     "command": "echo",
#     "args": ["Hello World"],
#     "exit_code": 0
#   },
#   {
#     "id": "2",
#     "status": "RUNNING",
#     "start_time": "2025-08-03T10:16:45Z",
#     "command": "python3",
#     "args": ["script.py"],
#     "max_cpu": 100,
#     "max_memory": 512,
#     "cpu_cores": "0-3",
#     "scheduled_time": "2025-08-03T15:00:00Z"
#   }
# ]

# Filter with jq
rnx list --json | jq '.[] | select(.status == "FAILED")'
rnx list --json | jq '.[] | select(.max_memory > 1024)'
```

### `rnx status`

Get detailed status of a specific job.

```bash
rnx status [flags] <job-id>
```

#### Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--json` | Output in JSON format | false |

#### Examples

```bash
# Get job status (human-readable format)
rnx status 42

# Get job status in JSON format
rnx status --json 42

# Check multiple jobs
for id in 1 2 3; do rnx status $id; done

# JSON output for scripting
rnx status --json 42 | jq .status
rnx status --json 42 | jq .exit_code

# Example JSON output:
# {
#   "id": "42",
#   "command": "python3",
#   "args": ["process_data.py"],
#   "maxCPU": 100,
#   "cpuCores": "0-3",
#   "maxMemory": 512,
#   "maxIOBPS": 0,
#   "status": "COMPLETED",
#   "startTime": "2025-08-03T10:15:32Z",
#   "endTime": "2025-08-03T10:18:45Z",
#   "exitCode": 0,
#   "scheduledTime": ""
# }
```

#### Output includes:
- Job ID and command
- Current status
- Start/end times
- Resource limits
- Exit code (if completed)
- Scheduling information

### `rnx log`

View or stream job logs.

```bash
rnx log [flags] <job-id>
```

#### Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--follow`, `-f` | Stream logs in real-time | false |
| `--tail` | Number of lines to show from end | all |
| `--timestamps` | Show timestamps | false |

#### Examples

```bash
# View complete logs
rnx log 42

# Stream logs in real-time
rnx log -f 42

# Show last 100 lines
rnx log --tail=100 42

# With timestamps
rnx log --timestamps 42
```

### `rnx stop`

Stop a running or scheduled job.

```bash
rnx stop <job-id>
```

#### Examples

```bash
# Stop a running job
rnx stop 42

# Stop multiple jobs
rnx list --json | jq -r '.[] | select(.status == "RUNNING") | .id' | xargs -I {} rnx stop {}
```

## Volume Commands

### `rnx volume create`

Create a new volume for persistent storage.

```bash
rnx volume create <name> [flags]
```

#### Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--size` | Volume size (e.g., 1GB, 500MB) | required |
| `--type` | Volume type: filesystem or memory | "filesystem" |

#### Examples

```bash
# Create 1GB filesystem volume
rnx volume create mydata --size=1GB

# Create 512MB memory volume (tmpfs)
rnx volume create cache --size=512MB --type=memory

# Create volumes for different purposes
rnx volume create db-data --size=10GB --type=filesystem
rnx volume create temp-processing --size=2GB --type=memory
```

### `rnx volume list`

List all volumes.

```bash
rnx volume list [flags]
```

#### Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--json` | Output in JSON format | false |

#### Examples

```bash
# List all volumes
rnx volume list

# JSON output
rnx volume list --json

# Check volume usage
rnx volume list --json | jq '.[] | select(.size_used > .size_total * 0.8)'
```

### `rnx volume remove`

Remove a volume.

```bash
rnx volume remove <name>
```

#### Examples

```bash
# Remove single volume
rnx volume remove mydata

# Remove all volumes (careful!)
rnx volume list --json | jq -r '.[].name' | xargs -I {} rnx volume remove {}
```

## Network Commands

### `rnx network create`

Create a custom network.

```bash
rnx network create <name> [flags]
```

#### Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--cidr` | Network CIDR (e.g., 10.10.0.0/24) | required |

#### Examples

```bash
# Create basic network
rnx network create mynet --cidr=10.10.0.0/24

# Create multiple networks for different environments
rnx network create dev --cidr=10.10.0.0/24
rnx network create test --cidr=10.20.0.0/24
rnx network create prod --cidr=10.30.0.0/24
```

### `rnx network list`

List all networks.

```bash
rnx network list [flags]
```

#### Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--json` | Output in JSON format | false |

#### Examples

```bash
# List all networks
rnx network list

# JSON output
rnx network list --json
```

### `rnx network delete`

Delete a network.

```bash
rnx network delete <name>
```

#### Examples

```bash
# Delete network
rnx network delete mynet

# Delete all custom networks
rnx network list --json | jq -r '.[] | select(.name != "bridge") | .name' | xargs -I {} rnx network delete {}
```

## System Commands

### `rnx monitor`

Monitor system metrics in real-time.

```bash
rnx monitor [subcommand] [flags]
```

#### Subcommands

- `status` - Show current system status
- (default) - Stream real-time metrics

#### Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--interval` | Update interval in seconds | 2 |
| `--json` | Output in JSON format | false |

#### Examples

```bash
# Real-time monitoring
rnx monitor

# Update every 5 seconds
rnx monitor --interval=5

# Get current status
rnx monitor status

# JSON output for metrics collection
rnx monitor status --json

# Continuous JSON stream
rnx monitor --json --interval=10
```

### `rnx nodes`

List configured nodes.

```bash
rnx nodes
```

#### Examples

```bash
# List all nodes
rnx nodes

# Use specific node
rnx --node=production list
rnx --node=staging run echo "test"
```

### `rnx help`

Show help information.

```bash
rnx help [command]
```

#### Examples

```bash
# General help
rnx help

# Command-specific help
rnx help run
rnx help volume create

# Show configuration help
rnx help config
```

## Advanced Usage

### Scripting with RNX

```bash
#!/bin/bash
# Batch processing script

# Process files in parallel with resource limits
for file in *.csv; do
  rnx run \
    --max-cpu=100 \
    --max-memory=1024 \
    --upload="$file" \
    --name="process-$file" \
    python3 process.py "$file" &
done

# Wait for all jobs
wait

# Collect results
rnx list --json | jq -r '.[] | select(.status == "COMPLETED" and (.name | startswith("process-"))) | .id' | \
while read job_id; do
  rnx log "$job_id" > "result-$job_id.txt"
done
```

### CI/CD Integration

```yaml
# GitHub Actions example
- name: Run tests in Joblet
  run: |
    rnx run \
      --max-cpu=400 \
      --max-memory=4096 \
      --volume=test-results \
      --upload-dir=. \
      --env=CI=true \
      npm test
    
    # Check job status
    JOB_ID=$(rnx list --json | jq -r '.[-1].id')
    rnx status $JOB_ID
    
    # Get test results
    rnx run --volume=test-results cat /volumes/test-results/report.xml
```

### Monitoring and Alerting

```bash
# Monitor job failures
while true; do
  FAILED=$(rnx list --json | jq '[.[] | select(.status == "FAILED")] | length')
  if [ $FAILED -gt 0 ]; then
    echo "Alert: $FAILED failed jobs detected"
    rnx list --json | jq '.[] | select(.status == "FAILED")'
  fi
  sleep 60
done
```

## Configuration Examples

### Multi-node Configuration

```yaml
version: "3.0"
nodes:
  default:
    address: "prod-server:50051"
    cert: |
      -----BEGIN CERTIFICATE-----
      ...
    key: |
      -----BEGIN PRIVATE KEY-----
      ...
    ca: |
      -----BEGIN CERTIFICATE-----
      ...
  
  staging:
    address: "staging-server:50051"
    cert: |
      -----BEGIN CERTIFICATE-----
      ...
    # ... rest of credentials
  
  viewer:
    address: "prod-server:50051"
    cert: |
      -----BEGIN CERTIFICATE-----
      # Viewer certificate with OU=viewer
      ...
    # ... rest of credentials
```

### Usage with Different Nodes

```bash
# Production jobs
rnx --node=default run production-task.sh

# Staging tests
rnx --node=staging run test-suite.sh

# Read-only access
rnx --node=viewer list
rnx --node=viewer monitor status
```

## Best Practices

1. **Resource Limits**: Always set appropriate resource limits for production jobs
2. **Volumes**: Use filesystem volumes for persistent data, memory volumes for temporary data
3. **Networks**: Create isolated networks for security-sensitive workloads
4. **Monitoring**: Use `rnx monitor` to track resource usage
5. **Scheduling**: Use ISO 8601 format for precise scheduling
6. **Error Handling**: Check exit codes and logs for job failures
7. **Cleanup**: Remove unused volumes and networks regularly

## Troubleshooting

See [Troubleshooting Guide](./TROUBLESHOOTING.md) for common issues and solutions.