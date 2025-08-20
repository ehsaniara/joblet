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
    - [network remove](#rnx-network-remove)
- [Runtime Commands](#runtime-commands)
    - [runtime list](#rnx-runtime-list)
    - [runtime info](#rnx-runtime-info)
    - [runtime test](#rnx-runtime-test)
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

| Flag              | Description                                                | Default        |
|-------------------|------------------------------------------------------------|----------------|
| `--max-cpu`       | Maximum CPU usage percentage (0-10000)                     | 0 (unlimited)  |
| `--max-memory`    | Maximum memory in MB                                       | 0 (unlimited)  |
| `--max-iobps`     | Maximum I/O bytes per second                               | 0 (unlimited)  |
| `--cpu-cores`     | CPU cores to use (e.g., "0-3" or "1,3,5")                  | "" (all cores) |
| `--network`       | Network mode: bridge, isolated, none, or custom            | "bridge"       |
| `--volume`        | Volume to mount (can be specified multiple times)          | none           |
| `--upload`        | Upload file to workspace (can be specified multiple times) | none           |
| `--upload-dir`    | Upload directory to workspace                              | none           |
| `--env, -e`       | Environment variable (KEY=VALUE, visible in logs)          | none           |
| `--secret-env, -s`| Secret environment variable (KEY=VALUE, hidden from logs)  | none           |
| `--schedule`      | Schedule job execution (duration or RFC3339 time)          | immediate      |
| `--workflow`      | YAML workflow file for workflow execution                  | none           |

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

# Environment variables (regular - visible in logs)
rnx run --env="NODE_ENV=production" --env="PORT=8080" \
  node app.js

# Secret environment variables (hidden from logs)
rnx run --secret-env="API_KEY=dummy_api_key_123" --secret-env="DB_PASSWORD=secret" \
  python app.py

# Mixed environment variables
rnx run --env="DEBUG=true" --secret-env="SECRET_KEY=mysecret" \
  python app.py


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

# Workflow execution
rnx run --workflow=ml-pipeline.yaml           # Execute full workflow
rnx run --workflow=jobs.yaml:ml-analysis      # Execute specific job from workflow

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

#### Workflow Validation

When using `--workflow`, Joblet performs comprehensive pre-execution validation:

```bash
$ rnx run --workflow=my-workflow.yaml
🔍 Validating workflow prerequisites...
✅ No circular dependencies found
✅ All required volumes exist
✅ All required networks exist
✅ All required runtimes exist
✅ All job dependencies are valid
🎉 Workflow validation completed successfully!
```

**Validation Checks:**
- **Circular Dependencies**: Prevents infinite dependency loops
- **Network Validation**: Confirms all specified networks exist (built-in: none, isolated, bridge + custom networks)
- **Volume Validation**: Verifies all referenced volumes are available
- **Runtime Validation**: Checks runtime availability with name normalization
- **Job Dependencies**: Ensures all dependencies reference existing jobs

**Error Example:**
```bash
Error: workflow validation failed: network validation failed: missing networks: [non-existent-network]. Available networks: [bridge isolated none custom-net]
```

### `rnx list`

List all jobs or workflows on the server.

```bash
rnx list [flags]              # List all jobs
rnx list --workflow [flags]   # List all workflows
```

#### Flags

| Flag         | Description                    | Default |
|--------------|--------------------------------|---------|
| `--json`     | Output in JSON format          | false   |
| `--workflow` | List workflows instead of jobs | false   |

#### Output Format

**Table Format (default):**

- **ID**: Job UUID (36-character identifier)
- **NAME**: Job name (from workflows, "-" for individual jobs)
- **STATUS**: Current job status (RUNNING, COMPLETED, FAILED, STOPPED, SCHEDULED)
- **START TIME**: When the job started (format: YYYY-MM-DD HH:MM:SS)
- **COMMAND**: The command being executed (truncated to 80 chars if too long)

**JSON Format:**
Outputs a JSON array with detailed job information including all resource limits, volumes, network, and scheduling
information.

#### Examples

```bash
# List all jobs (table format)
rnx list

# Example output:
# UUID                                 NAME         STATUS      START TIME           COMMAND
# ------------------------------------  ------------ ----------  -------------------  -------
# f47ac10b-58cc-4372-a567-0e02b2c3d479  setup-data   COMPLETED   2025-08-03 10:15:32  echo "Hello World"
# a1b2c3d4-e5f6-7890-abcd-ef1234567890  process-data RUNNING     2025-08-03 10:16:45  python3 script.py
# b2c3d4e5-f6a7-8901-bcde-f23456789012  -            FAILED      2025-08-03 10:17:20  invalid_command
# c3d4e5f6-a7b8-9012-cdef-345678901234  -            SCHEDULED   N/A                  backup.sh

# List all workflows (table format)
rnx list --workflow

# Example output:
# UUID                                 WORKFLOW             STATUS      PROGRESS
# ------------------------------------ -------------------- ----------- ---------
# a1b2c3d4-e5f6-7890-1234-567890abcdef data-pipeline.yaml   RUNNING     3/5
# b2c3d4e5-f6a7-8901-2345-678901bcdefg ml-pipeline.yaml     COMPLETED   5/5

# JSON output for scripting
rnx list --json

# Example JSON output:
# [
#   {
#     "id": "f47ac10b-58cc-4372-a567-0e02b2c3d479",
#     "name": "setup-data",
#     "status": "COMPLETED",
#     "start_time": "2025-08-03T10:15:32Z",
#     "end_time": "2025-08-03T10:15:33Z",
#     "command": "echo",
#     "args": ["Hello World"],
#     "exit_code": 0
#   },
#   {
#     "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
#     "name": "process-data",
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

Get detailed status of a specific job or workflow.

```bash
rnx status [flags] <job-uuid>              # Get job status
rnx status --workflow <workflow-uuid>      # Get workflow status
```

#### Job Status

- **Job UUIDs**: 36-character UUID identifiers (e.g., "f47ac10b-58cc-4372-a567-0e02b2c3d479")

#### Workflow Status

- **Workflow UUIDs**: 36-character UUID identifiers (e.g., "a1b2c3d4-e5f6-7890-1234-567890abcdef")  
- **Workflow IDs**: Numeric identifiers (e.g., 1, 2, 3)

**Workflow Status Features:**
- Displays job names, dependencies, status, and exit codes in a tabular format
- Shows dependency relationships between workflow jobs  
- Real-time progress tracking with job-level details
- Color-coded status indicators (RUNNING, COMPLETED, FAILED, etc.)
- **Job ID Display**: Started jobs show actual job UUIDs (e.g., "f47ac10b-58cc-4372-a567-0e02b2c3d479", "a1b2c3d4-e5f6-7890-abcd-ef1234567890"), non-started jobs show "0"

#### Flags

| Flag         | Description                    | Default |
|--------------|--------------------------------|---------|
| `--workflow` | Explicitly get workflow status | false   |
| `--json`     | Output in JSON format          | false   |

#### Examples

```bash
# Get job status (human-readable format)
rnx status f47ac10b-58cc-4372-a567-0e02b2c3d479

# Get workflow status
rnx status --workflow a1b2c3d4-e5f6-7890-1234-567890abcdef

# Get status in JSON format
rnx status --json f47ac10b-58cc-4372-a567-0e02b2c3d479    # Job JSON output
rnx status --workflow --json a1b2c3d4-e5f6-7890-1234-567890abcdef     # Workflow JSON output

# Check multiple jobs/workflows
for uuid in f47ac10b-58cc-4372-a567-0e02b2c3d479 a1b2c3d4-e5f6-7890-1234-567890abcdef; do rnx status $uuid; done

# JSON output for scripting
rnx status --json f47ac10b-58cc-4372-a567-0e02b2c3d479 | jq .status      # Job status
rnx status --workflow --json a1b2c3d4-e5f6-7890-1234-567890abcdef | jq .total_jobs   # Workflow progress

# Example workflow status output:
# Workflow UUID: a1b2c3d4-e5f6-7890-1234-567890abcdef
# Workflow: data-pipeline.yaml
# Status: RUNNING
# Progress: 2/4 jobs completed
# 
# Jobs in Workflow:
# -----------------------------------------------------------------------------------------
# JOB ID                                  JOB NAME             STATUS       EXIT CODE  DEPENDENCIES        
# -------------------------------------------------------------------------------------------------------------
# f47ac10b-58cc-4372-a567-0e02b2c3d479    setup-data           COMPLETED    0          -                   
# a1b2c3d4-e5f6-7890-abcd-ef1234567890    process-data         COMPLETED    0          setup-data          
# 0                                       validate-results     PENDING      -          process-data        
# 0                                       generate-report      PENDING      -          validate-results    

# Example JSON output for individual job:
# {
#   "uuid": "f47ac10b-58cc-4372-a567-0e02b2c3d479",
#   "name": "process-data",
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
rnx log [flags] <job-uuid>
```

#### Flags

| Flag             | Description                      | Default |
|------------------|----------------------------------|---------|
| `--follow`, `-f` | Stream logs in real-time         | false   |
| `--tail`         | Number of lines to show from end | all     |
| `--timestamps`   | Show timestamps                  | false   |

#### Examples

```bash
# View complete logs
rnx log f47ac10b-58cc-4372-a567-0e02b2c3d479

# Stream logs in real-time
rnx log -f f47ac10b-58cc-4372-a567-0e02b2c3d479

# Show last 100 lines
rnx log --tail=100 f47ac10b-58cc-4372-a567-0e02b2c3d479

# With timestamps
rnx log --timestamps f47ac10b-58cc-4372-a567-0e02b2c3d479
```

### `rnx stop`

Stop a running or scheduled job.

```bash
rnx stop <job-uuid>
```

#### Examples

```bash
# Stop a running job
rnx stop f47ac10b-58cc-4372-a567-0e02b2c3d479

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

| Flag     | Description                       | Default      |
|----------|-----------------------------------|--------------|
| `--size` | Volume size (e.g., 1GB, 500MB)    | required     |
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

| Flag     | Description           | Default |
|----------|-----------------------|---------|
| `--json` | Output in JSON format | false   |

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

| Flag     | Description                       | Default  |
|----------|-----------------------------------|----------|
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

| Flag     | Description           | Default |
|----------|-----------------------|---------|
| `--json` | Output in JSON format | false   |

#### Examples

```bash
# List all networks
rnx network list

# JSON output
rnx network list --json
```

### `rnx network remove`

Remove a custom network. Built-in networks cannot be removed.

```bash
rnx network remove <name>
```

#### Examples

```bash
# Remove network
rnx network remove mynet

# Remove all custom networks (keep built-in networks)
rnx network list --json | jq -r '.networks[] | select(.builtin == false) | .name' | xargs -I {} rnx network remove {}
```

## Runtime Commands

### `rnx runtime list`

List all available runtime environments.

```bash
rnx runtime list [flags]
```

#### Flags

| Flag     | Description           | Default |
|----------|-----------------------|---------|
| `--json` | Output in JSON format | false   |

#### Examples

```bash
# List all runtimes
rnx runtime list

# JSON output
rnx runtime list --json
```

### `rnx runtime info`

Get detailed information about a specific runtime environment.

```bash
rnx runtime info <runtime-spec>
```

#### Examples

```bash
# Get runtime details
rnx runtime info python:3.11-ml
rnx runtime info java:17
rnx runtime info nodejs:18
```

### `rnx runtime test`

Test a runtime environment to verify it's working correctly.

```bash
rnx runtime test <runtime-spec>
```

#### Examples

```bash
# Test runtime functionality
rnx runtime test python:3.11-ml
rnx runtime test java:17
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

| Flag         | Description                | Default |
|--------------|----------------------------|---------|
| `--interval` | Update interval in seconds | 2       |
| `--json`     | Output in JSON format      | false   |

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

List configured nodes from the client configuration file.

```bash
rnx nodes [flags]
```

#### Flags

| Flag     | Description           | Default |
|----------|-----------------------|---------|
| `--json` | Output in JSON format | false   |

#### Examples

```bash
# List all nodes
rnx nodes

# JSON output
rnx nodes --json

# Use specific node for commands
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
    python3 process.py "$file" &
done

# Wait for all jobs
wait

# Collect results
rnx list --json | jq -r '.[] | select(.status == "COMPLETED") | .id' | \
while read job_uuid; do
  rnx log "$job_uuid" > "result-$(echo $job_uuid | cut -c1-8).txt"
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
    JOB_UUID=$(rnx list --json | jq -r '.[-1].uuid')
    rnx status $JOB_UUID

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