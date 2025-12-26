# Basic Usage Examples

Shell scripts demonstrating core Joblet features.

## Quick Start

### Run All Examples

```bash
cd examples/basic-usage
./run_demos.sh
```

This runs all examples in sequence with explanations.

### Run Individual Examples

```bash
./01_simple_commands.sh    # Basic shell commands
./02_file_operations.sh    # File uploads and workspace
./03_resource_limits.sh    # CPU/memory limits
./04_volume_storage.sh     # Persistent volumes
./05_job_monitoring.sh     # Status and logs
./07_network_basics.sh     # Network isolation
```

## Examples Overview

| Example | Script | Description |
|---------|--------|-------------|
| Simple Commands | `01_simple_commands.sh` | Execute basic shell commands |
| File Operations | `02_file_operations.sh` | Upload files, workspace usage |
| Resource Limits | `03_resource_limits.sh` | CPU, memory, I/O limits |
| Volume Storage | `04_volume_storage.sh` | Persistent data between jobs |
| Job Monitoring | `05_job_monitoring.sh` | Status tracking, log viewing |
| Network Basics | `07_network_basics.sh` | Network isolation modes |

## Simple Commands

Basic job execution:

```bash
# Run a simple command
rnx job run echo "Hello Joblet"

# Run with arguments
rnx job run ls -la /

# Run bash script
rnx job run bash -c "echo 'Current time:' && date"
```

## File Operations

Working with files:

```bash
# Upload a file
rnx job run --upload=script.py python3 script.py

# Upload a directory
rnx job run --upload=./myproject python3 main.py

# Multiple uploads
rnx job run --upload=data.csv --upload=process.py python3 process.py
```

## Resource Limits

Control job resources:

```bash
# Memory limit
rnx job run --max-memory=512 python3 memory_test.py

# CPU limit
rnx job run --max-cpu=50 compute_task.sh

# Combined limits
rnx job run --max-memory=1024 --max-cpu=75 heavy_task.py
```

## Volume Storage

Persistent data between jobs:

```bash
# Create data in a volume
rnx job run --volume=mydata bash -c "echo 'Hello' > /volumes/mydata/file.txt"

# Read data from volume
rnx job run --volume=mydata cat /volumes/mydata/file.txt

# Share volume between jobs
rnx job run --volume=shared --upload=producer.py python3 producer.py
rnx job run --volume=shared --upload=consumer.py python3 consumer.py
```

## Job Monitoring

Track job status:

```bash
# List jobs
rnx job list

# Get job status
rnx job status <job-id>

# View logs
rnx job log <job-id>

# Follow logs in real-time
rnx job log -f <job-id>
```

## Network Modes

Network isolation options:

```bash
# Default (bridge network)
rnx job run curl https://example.com

# No network access
rnx job run --network=none python3 offline_task.py

# Isolated (external only)
rnx job run --network=isolated wget https://example.com
```

## Files Included

```
basic-usage/
├── 01_simple_commands.sh
├── 02_file_operations.sh
├── 03_resource_limits.sh
├── 04_volume_storage.sh
├── 05_job_monitoring.sh
├── 07_network_basics.sh
├── run_demos.sh           # Run all demos
├── sample_data.txt        # Sample data file
├── demo_dir/              # Sample directory
└── scripts/               # Helper scripts
```

## Related

- [Log Streaming](../log-streaming/README.md) - Real-time logging
- [Advanced Examples](../advanced/README.md) - Multi-job coordination
- [Python Examples](../python/README.md) - Python runtime examples
