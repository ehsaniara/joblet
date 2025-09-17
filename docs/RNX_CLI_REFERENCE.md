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
    - [delete](#rnx-delete)
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
    - [runtime install](#rnx-runtime-install)
    - [runtime test](#rnx-runtime-test)
    - [runtime validate](#rnx-runtime-validate)
    - [runtime remove](#rnx-runtime-remove)
- [System Commands](#system-commands)
    - [version](#rnx-version)
    - [monitor](#rnx-monitor)
    - [nodes](#rnx-nodes)
    - [admin](#rnx-admin)
    - [config-help](#rnx-config-help)
    - [help](#rnx-help)

## Global Options

Options available for all commands:

```bash
--config <path>    # Path to configuration file (default: searches standard locations)
--node <name>      # Node name from configuration (default: "default")
--json             # Output in JSON format
--version, -v      # Show version information for both client and server
--help, -h         # Show help for command
```

### Configuration File Locations

RNX searches for configuration in this order:

1. `./rnx-config.yml`
2. `./config/rnx-config.yml`
3. `~/.rnx/rnx-config.yml`
4. `/etc/joblet/rnx-config.yml`
5. `/opt/joblet/config/rnx-config.yml`

## Job Commands

### `rnx job run`

Execute a command on the Joblet server.

```bash
rnx job run [flags] <command> [args...]
```

#### Flags

| Flag               | Description                                                | Default        |
|--------------------|------------------------------------------------------------|----------------|
| `--max-cpu`        | Maximum CPU usage percentage (0-10000)                     | 0 (unlimited)  |
| `--max-memory`     | Maximum memory in MB                                       | 0 (unlimited)  |
| `--max-iobps`      | Maximum I/O bytes per second                               | 0 (unlimited)  |
| `--cpu-cores`      | CPU cores to use (e.g., "0-3" or "1,3,5")                  | "" (all cores) |
| `--network`        | Network mode: bridge, isolated, none, or custom            | "bridge"       |
| `--volume`         | Volume to mount (can be specified multiple times)          | none           |
| `--upload`         | Upload file to workspace (can be specified multiple times) | none           |
| `--upload-dir`     | Upload directory to workspace                              | none           |
| `--runtime`        | Use pre-built runtime (e.g., openjdk-21, python-3.11-ml)   | none           |
| `--env, -e`        | Environment variable (KEY=VALUE, visible in logs)          | none           |
| `--secret-env, -s` | Secret environment variable (KEY=VALUE, hidden from logs)  | none           |
| `--schedule`       | Schedule job execution (duration or RFC3339 time)          | immediate      |
| `--workflow`       | YAML workflow file for workflow execution                  | none           |

#### Examples

```bash
# Simple command
rnx job run echo "Hello, World!"

# With resource limits
rnx job run --max-cpu=50 --max-memory=512 --max-iobps=10485760 \
  python3 intensive_script.py

# CPU core binding
rnx job run --cpu-cores="0-3" stress-ng --cpu 4 --timeout 60s

# Multiple volumes
rnx job run --volume=data --volume=config \
  python3 process.py

# Environment variables (regular - visible in logs)
rnx job run --env="NODE_ENV=production" --env="PORT=8080" \
  node app.js

# Secret environment variables (hidden from logs)
rnx job run --secret-env="API_KEY=dummy_api_key_123" --secret-env="DB_PASSWORD=secret" \
  python app.py

# Mixed environment variables
rnx job run --env="DEBUG=true" --secret-env="SECRET_KEY=mysecret" \
  python app.py


# File upload
rnx job run --upload=script.py --upload=data.csv \
  python3 script.py data.csv

# Directory upload
rnx job run --upload-dir=./project \
  npm start

# Scheduled execution
rnx job run --schedule="30min" backup.sh
rnx job run --schedule="2025-08-03T15:00:00" maintenance.sh

# Custom network
rnx job run --network=isolated ping google.com

# Workflow execution
rnx job run --workflow=ml-pipeline.yaml           # Execute full workflow
rnx job run --workflow=jobs.yaml:ml-analysis      # Execute specific job from workflow

# Using runtime
rnx job run --runtime=python-3.11-ml python -c "import torch; print(torch.__version__)"
rnx job run --runtime=openjdk-21 java -version

# Complex example
rnx job run \
  --max-cpu=200 \
  --max-memory=2048 \
  --cpu-cores="0,2,4,6" \
  --network=mynet \
  --volume=persistent-data \
  --env=PYTHONPATH=/app \
  --upload-dir=./src \
  --runtime=python-3.11-ml \
  python3 main.py --process-data
```

#### Workflow Validation

When using `--workflow`, Joblet performs comprehensive pre-execution validation:

```bash
$ rnx job run --workflow=my-workflow.yaml
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

### `rnx job list`

List all jobs or workflows on the server.

```bash
rnx job list [flags]              # List all jobs
rnx job list --workflow [flags]   # List all workflows
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
rnx job list

# Example output:
# UUID                                 NAME         STATUS      START TIME           COMMAND
# ------------------------------------  ------------ ----------  -------------------  -------
# f47ac10b-58cc-4372-a567-0e02b2c3d479  setup-data   COMPLETED   2025-08-03 10:15:32  echo "Hello World"
# a1b2c3d4-e5f6-7890-abcd-ef1234567890  process-data RUNNING     2025-08-03 10:16:45  python3 script.py
# b2c3d4e5-f6a7-8901-bcde-f23456789012  -            FAILED      2025-08-03 10:17:20  invalid_command
# c3d4e5f6-a7b8-9012-cdef-345678901234  -            SCHEDULED   N/A                  backup.sh

# List all workflows (table format)
rnx job list --workflow

# Example output:
# UUID                                 WORKFLOW             STATUS      PROGRESS
# ------------------------------------ -------------------- ----------- ---------
# a1b2c3d4-e5f6-7890-1234-567890abcdef data-pipeline.yaml   RUNNING     3/5
# b2c3d4e5-f6a7-8901-2345-678901bcdefg ml-pipeline.yaml     COMPLETED   5/5

# JSON output for scripting
rnx job list --json

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
rnx job list --json | jq '.[] | select(.status == "FAILED")'
rnx job list --json | jq '.[] | select(.max_memory > 1024)'
```

### `rnx job status`

Get detailed status of a specific job or workflow.

```bash
rnx job status [flags] <job-uuid>              # Get job status
rnx job status --workflow <workflow-uuid>      # Get workflow status
rnx job status --workflow --detail <workflow-uuid>  # Get workflow status with YAML content
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
- **YAML Content Display**: Use `--detail` flag to view the original workflow YAML content
- **Multi-workstation Access**: YAML content is stored server-side, accessible from any client
- **Job ID Display**: Started jobs show actual job UUIDs (e.g., "f47ac10b-58cc-4372-a567-0e02b2c3d479", "
  a1b2c3d4-e5f6-7890-abcd-ef1234567890"), non-started jobs show "0"

#### Flags

| Flag               | Description                    | Default | Notes                            |
|--------------------|--------------------------------|---------|----------------------------------|
| `--workflow`, `-w` | Explicitly get workflow status | false   | Required for workflow operations |
| `--detail`, `-d`   | Show original YAML content     | false   | Only works with `--workflow`     |
| `--json`           | Output in JSON format          | false   | Available for jobs and workflows |

#### Examples

```bash
# Get job status (human-readable format)
rnx job status f47ac10b-58cc-4372-a567-0e02b2c3d479

# Get workflow status
rnx job status --workflow a1b2c3d4-e5f6-7890-1234-567890abcdef

# Get workflow status with original YAML content
rnx job status --workflow --detail a1b2c3d4-e5f6-7890-1234-567890abcdef

# Get status in JSON format
rnx job status --json f47ac10b-58cc-4372-a567-0e02b2c3d479    # Job JSON output
rnx job status --workflow --json a1b2c3d4-e5f6-7890-1234-567890abcdef     # Workflow JSON output
rnx job status --workflow --json --detail a1b2c3d4-e5f6-7890-1234-567890abcdef  # Workflow JSON with YAML content

# Check multiple jobs/workflows
for uuid in f47ac10b-58cc-4372-a567-0e02b2c3d479 a1b2c3d4-e5f6-7890-1234-567890abcdef; do rnx job status $uuid; done

# JSON output for scripting
rnx job status --json f47ac10b-58cc-4372-a567-0e02b2c3d479 | jq .status      # Job status
rnx job status --workflow --json a1b2c3d4-e5f6-7890-1234-567890abcdef | jq .total_jobs   # Workflow progress
rnx job status --workflow --json --detail a1b2c3d4-e5f6-7890-1234-567890abcdef | jq .yaml_content  # Extract YAML content

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

#### Example Workflow JSON Output with YAML Content

```bash
# rnx job status --workflow --json --detail a1b2c3d4-e5f6-7890-1234-567890abcdef
{
  "uuid": "a1b2c3d4-e5f6-7890-1234-567890abcdef",
  "workflow": "data-pipeline.yaml",
  "status": "RUNNING", 
  "total_jobs": 4,
  "completed_jobs": 2,
  "failed_jobs": 0,
  "created_at": {
    "seconds": 1691234567,
    "nanos": 0
  },
  "yaml_content": "jobs:\n  setup-data:\n    command: \"python3\"\n    args: [\"extract.py\"]\n    runtime: \"python-3.11-ml\"\n  process-data:\n    command: \"python3\"\n    args: [\"transform.py\"]\n    requires:\n      - setup-data: \"COMPLETED\"\n",
  "jobs": [
    {
      "id": "f47ac10b-58cc-4372-a567-0e02b2c3d479",
      "name": "setup-data",
      "status": "COMPLETED",
      "exit_code": 0
    },
    {
      "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890", 
      "name": "process-data",
      "status": "RUNNING",
      "dependencies": ["setup-data"]
    }
  ]
}
```

**Key Features:**

- **`yaml_content`** field contains original workflow YAML when `--detail` flag is used
- **Machine-readable format** for automation and scripting
- **Complete workflow metadata** including job details and dependencies

### `rnx job log`

Stream job logs in real-time.

```bash
rnx job log <job-uuid>
```

Streams logs from running or completed jobs. Use Ctrl+C to stop following the log stream.

#### Examples

```bash
# Stream logs from a job
rnx job log f47ac10b-58cc-4372-a567-0e02b2c3d479

# Use standard Unix tools for filtering
rnx job log f47ac10b-58cc-4372-a567-0e02b2c3d479 | tail -100
rnx job log f47ac10b-58cc-4372-a567-0e02b2c3d479 | grep ERROR

# Save logs to file
rnx job log f47ac10b-58cc-4372-a567-0e02b2c3d479 > output.log
```

### `rnx job stop`

Stop a running or scheduled job.

```bash
rnx job stop <job-uuid>
```

#### Examples

```bash
# Stop a running job
rnx job stop f47ac10b-58cc-4372-a567-0e02b2c3d479

# Stop multiple jobs
rnx job list --json | jq -r '.[] | select(.status == "RUNNING") | .id' | xargs -I {} rnx job stop {}
```

### `rnx job delete`

Delete a job completely from the system.

```bash
rnx job delete <job-uuid>
```

Permanently removes the specified job including logs, metadata, and all associated resources. The job must be in a
completed, failed, or stopped state - running jobs cannot be deleted directly and must be stopped first.

#### Examples

```bash
# Delete a completed job
rnx job delete f47ac10b-58cc-4372-a567-0e02b2c3d479

# Delete using short UUID (if unique)
rnx job delete f47ac10b
```

### `rnx job delete-all`

Delete all non-running jobs from the system.

```bash
rnx job delete-all [flags]
```

Permanently removes all jobs that are not currently running or scheduled. Jobs in completed, failed, or stopped states
will be deleted. Running and scheduled jobs are preserved and will not be affected.

Complete deletion includes:

- Job records and metadata
- Log files and buffers
- Subscriptions and streams
- Any remaining resources

#### Flags

- `--json`: Output results in JSON format

#### Examples

```bash
# Delete all non-running jobs
rnx job delete-all

# Delete all non-running jobs with JSON output
rnx job delete-all --json
```

**Example JSON Output:**

```json
{
  "success": true,
  "message": "Successfully deleted 3 jobs, skipped 1 running/scheduled jobs",
  "deleted_count": 3,
  "skipped_count": 1
}
```

**Note:** This operation is irreversible. Once deleted, job information and logs cannot be recovered. Only non-running
jobs are affected.

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

| Flag            | Description                                                        | Default |
|-----------------|--------------------------------------------------------------------|---------|
| `--json`        | Output in JSON format                                              | false   |
| `--github-repo` | List runtimes from GitHub repository (owner/repo/tree/branch/path) | none    |

#### Examples

```bash
# List locally installed runtimes
rnx runtime list

# List available runtimes from GitHub repository
rnx runtime list --github-repo=owner/repo/tree/main/runtimes

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
rnx runtime info python-3.11-ml
rnx runtime info openjdk:21
```

### `rnx runtime install`

Install a runtime environment from GitHub or local files.

```bash
rnx runtime install <runtime-spec> [flags]
```

#### Flags

| Flag            | Short | Description                                                  | Default |
|-----------------|-------|--------------------------------------------------------------|---------|
| `--force`       | `-f`  | Force reinstall by deleting existing runtime                 | false   |
| `--github-repo` |       | Install from GitHub repository (owner/repo/tree/branch/path) | none    |

#### Description

The install command downloads and executes platform-specific setup scripts in a secure builder chroot environment. It
automatically detects the host platform (Ubuntu, Amazon Linux, RHEL) and architecture (AMD64, ARM64) to run the
appropriate setup script.

When using `--force`, the command will:

1. Delete the existing runtime at `/opt/joblet/runtimes/<runtime-name>` if it exists
2. Proceed with fresh installation
3. Continue even if deletion fails (with warning)

#### Examples

```bash
# Install from local codebase
rnx runtime install python-3.11-ml
rnx runtime install openjdk-21

# Install from GitHub repository
rnx runtime install openjdk-21 --github-repo=ehsaniara/joblet/tree/main/runtimes
rnx runtime install python-3.11-ml --github-repo=owner/repo/tree/branch/path

# Force reinstall (delete existing runtime first)
rnx runtime install python-3.11-ml --force
rnx runtime install openjdk-21 -f
```

### `rnx runtime test`

Test a runtime environment to verify it's working correctly.

```bash
rnx runtime test <runtime-spec>
```

#### Examples

```bash
# Test runtime functionality
rnx runtime test python-3.11-ml
rnx runtime test openjdk:21
```

### `rnx runtime remove`

Remove a runtime environment.

```bash
rnx runtime remove <runtime-spec>
```

#### Examples

```bash
# Remove a runtime
rnx runtime remove python-3.11-ml
rnx runtime remove openjdk-21
```

### `rnx runtime validate`

Validate a runtime specification format and check if it's supported.

```bash
rnx runtime validate <runtime-spec>
```

#### Examples

```bash
# Validate basic spec
rnx runtime validate python-3.11-ml

# Validate spec with variants
rnx runtime validate openjdk:21
```

## System Commands

### `rnx version`

Display version information for both RNX client and Joblet server.

```bash
rnx version [flags]
```

#### Flags

| Flag     | Description                      | Default |
|----------|----------------------------------|---------|
| `--json` | Output version info as JSON     | false   |

#### Examples

```bash
# Show version information
rnx version

# Output:
# RNX Client:
# rnx version v4.3.3 (4c11220)
# Built: 2025-09-14T05:17:17Z
# Commit: 4c11220b6e4f98960853fa0379b5c25d2f19e33f
# Go: go1.24.0
# Platform: linux/amd64
#
# Joblet Server (default):
# joblet version v4.3.3 (4c11220)
# Built: 2025-09-14T05:18:24Z
# Commit: 4c11220b6e4f98960853fa0379b5c25d2f19e33f
# Go: go1.24.0
# Platform: linux/amd64

# Show version as JSON
rnx version --json

# Use --version flag (alternative)
rnx --version
```

#### Version Information Details

- **Client Version**: The version of the RNX CLI tool running on your local machine
- **Server Version**: The version of the Joblet server it's connected to (from config)
- **Version Format**: `vMAJOR.MINOR.PATCH[+dev]` where `+dev` indicates development builds after the tagged release
- **Build Information**: Includes git commit hash, build date, Go version, and platform

#### Use Cases

- **Version Compatibility**: Ensure client and server versions are compatible
- **Debugging**: Identify specific builds when reporting issues
- **Deployment Tracking**: Verify which version is deployed on production servers
- **Development**: Track development builds with `+dev` suffix

### `rnx monitor`

Monitor comprehensive remote joblet server metrics including CPU, memory, disk, network, processes, and volumes.

```bash
rnx monitor <subcommand> [flags]
```

#### Subcommands

- `status` - Display comprehensive remote server status with detailed resource information
- `top` - Show current remote server metrics in condensed format with top processes
- `watch` - Stream real-time remote server metrics with configurable refresh intervals

#### Common Flags

| Flag         | Description                             | Default |
|--------------|-----------------------------------------|---------|
| `--json`     | Output in UI-compatible JSON format     | false   |
| `--interval` | Update interval in seconds (watch only) | 5       |
| `--filter`   | Filter metrics by type (top/watch only) | all     |
| `--compact`  | Use compact display format (watch only) | false   |

#### Available Server Metric Types (for --filter)

- `cpu` - Server CPU usage, load averages, per-core utilization
- `memory` - Server memory and swap usage with detailed breakdowns
- `disk` - Server disk usage for all mount points and joblet volumes
- `network` - Server network interface statistics with live throughput
- `io` - Server I/O operations, throughput, and utilization
- `process` - Server process statistics with top consumers

#### Monitoring Features

**Enhanced Remote Server Monitoring:**

- Real-time server resource utilization tracking from client
- Server cloud environment detection (AWS, GCP, Azure, KVM, etc.)
- Remote joblet volume usage and availability monitoring
- Server network throughput and packet statistics
- Server process state tracking (running, sleeping, stopped, zombie)
- Server per-core CPU utilization breakdown

**Remote JSON Data Format:**

- UI-compatible JSON structure with server data for dashboards
- Structured server metrics for monitoring tool integrations
- Real-time server data streaming for live monitoring systems

#### Examples

```bash
# Comprehensive remote server status
rnx monitor status

# JSON server data for dashboards/APIs
rnx monitor status --json

# Current server metrics with top processes
rnx monitor top

# Filter specific server metrics
rnx monitor top --filter=cpu,memory

# Real-time server monitoring (5s intervals)
rnx monitor watch

# Faster server monitoring refresh rate
rnx monitor watch --interval=2

# Monitor specific server resources
rnx monitor watch --filter=disk,network

# JSON server streaming for monitoring tools
rnx monitor watch --json --interval=10

# Compact format for server monitoring
rnx monitor watch --compact

# Monitor specific joblet server node
rnx --node=production monitor status
```

#### JSON Output Structure

The `--json` flag produces UI-compatible output with the following structure:

```json
{
  "hostInfo": {
    "hostname": "server-name",
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
    "perCoreUsage": [0.1, 0.2, 0.05, 0.3, ...]
  },
  "memoryInfo": {
    "total": 4100255744,
    "used": 378679296,
    "percent": 9.23,
    "swap": { "total": 0, "used": 0, "percent": 0 }
  },
  "disksInfo": {
    "disks": [
      {
        "name": "/dev/sda1",
        "mountpoint": "/",
        "filesystem": "ext4",
        "size": 19896352768,
        "used": 11143790592,
        "percent": 56.01
      },
      {
        "name": "analytics-data",
        "mountpoint": "/opt/joblet/volumes/analytics-data",
        "filesystem": "joblet-volume",
        "size": 1073741824,
        "used": 52428800,
        "percent": 4.88
      }
    ]
  },
  "networkInfo": {
    "interfaces": [...],
    "totalRxBytes": 1234567890,
    "totalTxBytes": 987654321
  },
  "processesInfo": {
    "processes": [...],
    "totalProcesses": 149
  }
}
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

### `rnx admin`

Launch the Joblet Admin UI server.

```bash
rnx admin [flags]
```

#### Flags

| Flag             | Description                   | Default   |
|------------------|-------------------------------|-----------|
| `--port, -p`     | Port to run the admin server  | 5173      |
| `--bind-address` | Address to bind the server to | "0.0.0.0" |

#### Examples

```bash
# Start admin UI with default settings
rnx admin

# Use custom port
rnx admin --port 8080

# Bind to all interfaces
rnx admin --bind-address 0.0.0.0 --port 5173
```

### `rnx config-help`

Show configuration file examples with embedded certificates.

```bash
rnx config-help
```

#### Examples

```bash
# Show configuration examples
rnx config-help
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
  rnx job run \
    --max-cpu=100 \
    --max-memory=1024 \
    --upload="$file" \
    python3 process.py "$file" &
done

# Wait for all jobs
wait

# Collect results
rnx job list --json | jq -r '.[] | select(.status == "COMPLETED") | .id' | \
while read job_uuid; do
  rnx job log "$job_uuid" > "result-$(echo $job_uuid | cut -c1-8).txt"
done
```

### CI/CD Integration

```yaml
# GitHub Actions example
- name: Run tests in Joblet
  run: |
    rnx job run \
      --max-cpu=400 \
      --max-memory=4096 \
      --volume=test-results \
      --upload-dir=. \
      --env=CI=true \
      npm test

    # Check job status
    JOB_UUID=$(rnx job list --json | jq -r '.[-1].uuid')
    rnx job status $JOB_UUID

    # Get test results
    rnx job run --volume=test-results cat /volumes/test-results/report.xml
```

### Monitoring and Alerting

```bash
# Monitor job failures
while true; do
  FAILED=$(rnx job list --json | jq '[.[] | select(.status == "FAILED")] | length')
  if [ $FAILED -gt 0 ]; then
    echo "Alert: $FAILED failed jobs detected"
    rnx job list --json | jq '.[] | select(.status == "FAILED")'
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