# Job Execution Guide

Comprehensive guide to executing jobs with Joblet, including resource management, isolation, and advanced features.

## Table of Contents

- [Basic Job Execution](#basic-job-execution)
- [Resource Limits](#resource-limits)
- [File Management](#file-management)
- [Environment Variables](#environment-variables)
- [Job Scheduling](#job-scheduling)
- [Job Lifecycle](#job-lifecycle)
- [Output and Logging](#output-and-logging)
- [Advanced Features](#advanced-features)
- [Best Practices](#best-practices)

## Basic Job Execution

### Simple Commands

All `rnx job run` commands execute with **production isolation** (minimal chroot):

```bash
# Run a single command
rnx job run echo "Hello, Joblet!"

# Run command with arguments
rnx job run ls -la /

# Run shell command
rnx job run sh -c "echo 'Current time:' && date"

# Run Python script
rnx job run python3 -c "print('Hello from Python')"
```

**Execution Environment:**

- **Service**: JobService (production operations)
- **Job Type**: `standard` (automatic)
- **Isolation**: Minimal chroot with secure boundaries
- **Purpose**: Safe execution of user workloads

### Command Chaining

```bash
# Using shell
rnx job run bash -c "cd /tmp && echo 'test' > file.txt && cat file.txt"

# Using && operator
rnx job run sh -c "apt update && apt install -y curl"

# Multi-line scripts
rnx job run bash -c '
  echo "Starting process..."
  for i in {1..5}; do
    echo "Step $i"
    sleep 1
  done
  echo "Process complete!"
'
```

## Resource Limits

### CPU Limits

```bash
# Limit to 50% of one CPU core
rnx job run --max-cpu=50 stress-ng --cpu 1 --timeout 60s

# Simple CPU test: 50% usage on core 0 for 2 minutes
rnx job run --max-cpu=50 --cpu-cores=0 bash -c 'timeout 120s bash -c "while true; do :; done"; exit 0'

# Limit to 2 full CPU cores (200%)
rnx job run --max-cpu=200 python3 parallel_processing.py

# Bind to specific CPU cores
rnx job run --cpu-cores="0,2,4,6" compute_intensive_task

# Range of cores
rnx job run --cpu-cores="0-3" make -j4
```

CPU limit calculation:

- 100 = 1 full CPU core
- 50 = 0.5 CPU core (50% of one core)
- 200 = 2 full CPU cores
- 0 = Unlimited (use all available cores)

### Memory Limits

```bash
# Limit to 512MB
rnx job run --max-memory=512 python3 data_processing.py

# Limit to 2GB
rnx job run --max-memory=2048 java -jar app.jar

# Combine with CPU limits
rnx job run --max-cpu=150 --max-memory=1024 node app.js
```

Memory behavior:

- Process killed if limit exceeded (OOM)
- Includes all memory types (RSS, cache, etc.)
- Swap disabled within job

### I/O Bandwidth Limits

```bash
# Limit to 10MB/s
rnx job run --max-iobps=10485760 dd if=/dev/zero of=/work/test.dat bs=1M count=100

# Limit to 100MB/s
rnx job run --max-iobps=104857600 tar -czf backup.tar.gz /data

# Combine all limits
rnx job run \
  --max-cpu=100 \
  --max-memory=2048 \
  --max-iobps=52428800 \
  rsync -av /source/ /dest/
```

I/O limit notes:

- Applies to all block devices
- Includes read and write operations
- Value in bytes per second

## File Management

### Uploading Files

```bash
# Upload single file
rnx job run --upload=script.py python3 script.py

# Upload multiple files
rnx job run \
  --upload=config.json \
  --upload=data.csv \
  python3 process.py config.json data.csv

# Upload and rename
echo "data" > local.txt
rnx job run --upload=local.txt cat local.txt
```

### Uploading Directories

```bash
# Upload entire directory
rnx job run --upload-dir=./project npm start

# Upload directory with specific structure
rnx job run --upload-dir=./src python3 -m src.main

# Large directory upload
rnx job run --upload-dir=./dataset python3 train_model.py
```

### Working Directory

```bash
# Default working directory is /work
rnx job run pwd  # Output: /work

# Files uploaded are available in /work
rnx job run --upload-dir=./app ls -la
```

### File Access in Jobs

```bash
# Uploaded files are in current directory
rnx job run --upload=data.txt cat data.txt

# Access uploaded directory contents
rnx job run --upload-dir=./config ls -la

# Write output files
rnx job run --volume=output python3 -c "
with open('/volumes/output/result.txt', 'w') as f:
    f.write('Processing complete')
"
```

## Environment Variables

### Setting Environment Variables

```bash
# Single variable
rnx job run --env=DEBUG=true --runtime=openjdk:21 java App

# Multiple variables
rnx job run \
  --env=DATABASE_URL=postgres://localhost/db \
  --env=API_KEY=secret123 \
  --env=JAVA_ENV=production \
  --runtime=openjdk:21 java Application

# Variables with spaces
rnx job run --env="MESSAGE=Hello World" echo '$MESSAGE'
```

### Default Environment

Every job has these variables set:

- `PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin`
- `HOME=/work`
- `USER=nobody` (or configured user)
- `PWD=/work`

### Using Environment Files

```bash
# Create env file for ML training job
cat > .env << EOF
MODEL_PATH=/volumes/models/bert-base
DATASET_PATH=/volumes/datasets/training
GPU_MEMORY_FRACTION=0.8
BATCH_SIZE=32
LEARNING_RATE=0.001
EOF

# Upload and source for training job
rnx job run --upload=.env python3 train_model.py
```

## Job Scheduling

### Relative Time Scheduling

```bash
# Run in 5 minutes
rnx job run --schedule="5min" backup.sh

# Run in 2 hours
rnx job run --schedule="2h" cleanup.sh

# Run in 30 seconds
rnx job run --schedule="30s" quick_task.sh

# Supported units: s, m, min, h, d
```

### Absolute Time Scheduling

```bash
# Run at specific time (RFC3339 format)
rnx job run --schedule="2025-08-03T15:00:00Z" daily_report.py

# With timezone
rnx job run --schedule="2025-08-03T15:00:00-07:00" meeting_reminder.sh

# Tomorrow at noon
TOMORROW_NOON=$(date -d "tomorrow 12:00" --rfc-3339=seconds)
rnx job run --schedule="$TOMORROW_NOON" lunch_reminder.sh
```

### Managing Scheduled Jobs

```bash
# List scheduled jobs
rnx job list --json | jq '.[] | select(.status == "SCHEDULED")'

# Cancel scheduled job
rnx job stop <job-uuid>

# Example with actual UUID:
rnx job stop f47ac10b-58cc-4372-a567-0e02b2c3d479

# Check when job will run
rnx job status <job-uuid>
```

## Job Lifecycle

### Job Identification

Each job is assigned a unique UUID (Universally Unique Identifier) when created. Job UUIDs are in the format:
`f47ac10b-58cc-4372-a567-0e02b2c3d479`

Use job UUIDs to:

- Check job status: `rnx job status <job-uuid>`
- View job logs: `rnx job log <job-uuid>`
- Stop running jobs: `rnx job stop <job-uuid>`

### Job States

1. **INITIALIZING** - Job accepted, preparing execution
2. **SCHEDULED** - Job scheduled for future execution
3. **RUNNING** - Job actively executing
4. **COMPLETED** - Job finished successfully (exit code 0)
5. **FAILED** - Job finished with error (non-zero exit code)
6. **STOPPED** - Job manually stopped

### Monitoring Job Progress

```bash
# Real-time status (shows job name for workflow jobs)
watch -n 1 rnx job status <job-uuid>

# Example with actual UUID:
watch -n 1 rnx job status f47ac10b-58cc-4372-a567-0e02b2c3d479

# Workflow status with job names and dependencies
rnx job status --workflow <workflow-id>

# Stream logs (use Ctrl+C to stop)
rnx job log <job-uuid>

# Example with actual UUID:
rnx job log f47ac10b-58cc-4372-a567-0e02b2c3d479

# List running jobs (shows names and status)
rnx job list --json | jq '.[] | select(.status == "RUNNING")'

# Filter by job name (for workflow jobs)
rnx job list --json | jq '.[] | select(.name == "process-data")'
```

### Job Completion

```bash
# Check exit code
rnx job status <job-uuid> | grep "Exit Code"

# Get final output
rnx job log <job-uuid> | tail -20

# Script to wait for completion
JOB_UUID=$(rnx job run --json long_task.sh | jq -r .id)
# JOB_UUID will be something like: f47ac10b-58cc-4372-a567-0e02b2c3d479
while [[ $(rnx job status --json $JOB_UUID | jq -r .status) == "RUNNING" ]]; do
  sleep 5
done
echo "Job completed with exit code: $(rnx job status --json $JOB_UUID | jq .exit_code)"
```

## Output and Logging

### Capturing Output

```bash
# Save logs to file
rnx job log <job-uuid> > output.log

# Example with actual UUID:
rnx job log f47ac10b-58cc-4372-a567-0e02b2c3d479 > output.log

# Stream to file
rnx job log <job-uuid> | tee running.log

# Parse JSON output
rnx job run --json echo "test" | jq .

# Get only stdout
rnx job log <job-uuid> 2>/dev/null
```

### Log Formatting

```bash
# Stream logs (use Ctrl+C to stop)
rnx job log <job-uuid>

# Filter logs with grep
rnx job log <job-uuid> | grep ERROR

# Get last N lines using standard tools
rnx job log <job-uuid> | tail -100
```

### Persistent Output

```bash
# Use volume for output files
rnx volume create results --size=10GB
rnx job run --volume=results python3 analysis.py

# Retrieve results
rnx job run --volume=results cat /volumes/results/report.pdf > report.pdf
```

## Advanced Features

### Complex Workflows

```bash
#!/bin/bash
# Multi-stage data processing pipeline

# Stage 1: Data preparation
PREP_JOB=$(rnx job run --json \
  --volume=pipeline-data \
  --upload=prepare_data.py \
  python3 prepare_data.py | jq -r .id)
# PREP_JOB will be something like: a1b2c3d4-e5f6-7890-abcd-ef1234567890

# Wait for completion
while [[ $(rnx job status --json $PREP_JOB | jq -r .status) == "RUNNING" ]]; do
  sleep 2
done

# Stage 2: Parallel processing
for i in {1..4}; do
  rnx job run \
    --volume=pipeline-data \
    --max-cpu=100 \
    --upload=process_chunk.py \
    python3 process_chunk.py $i &
done
wait

# Stage 3: Merge results
rnx job run \
  --volume=pipeline-data \
  --upload=merge_results.py \
  python3 merge_results.py
```

### Job Dependencies

```bash
# Simple dependency chain
JOB1=$(rnx job run --json setup.sh | jq -r .id)
# JOB1 will be something like: 12345678-abcd-ef12-3456-7890abcdef12
# Wait for job1
while [[ $(rnx job status --json $JOB1 | jq -r .status) == "RUNNING" ]]; do
  sleep 1
done

# Only run if job1 succeeded
if [[ $(rnx job status --json $JOB1 | jq .exit_code) -eq 0 ]]; then
  rnx job run process.sh
fi
```

### Resource Pools

```bash
# Create dedicated network for job group
rnx network create batch-network --cidr=10.50.0.0/24

# Run jobs in same network
for task in tasks/*.sh; do
  rnx job run \
    --network=batch-network \
    --volume=shared-data \
    --max-cpu=50 \
    bash "$task"
done
```

### Interactive Jobs

```bash
# Note: Joblet doesn't support interactive TTY, but you can simulate:

# Create script that reads from volume
cat > interactive.sh << 'EOF'
#!/bin/bash
while true; do
  if [[ -f /volumes/commands/next.txt ]]; then
    cmd=$(cat /volumes/commands/next.txt)
    rm /volumes/commands/next.txt
    eval "$cmd"
  fi
  sleep 1
done
EOF

# Start "interactive" job
rnx volume create commands --size=100MB
rnx job run --volume=commands --upload=interactive.sh bash interactive.sh

# Send commands
rnx job run --volume=commands sh -c 'echo "ls -la" > /volumes/commands/next.txt'
```

## Best Practices

### 1. Resource Planning

```bash
# Test resource requirements first
rnx job run --max-memory=512 python3 script.py

# If OOM, increase limit
rnx job run --max-memory=1024 python3 script.py

# Monitor actual usage
rnx monitor status
```

### 2. Error Handling

```bash
# Robust job submission
submit_job() {
  local cmd="$1"
  local max_retries=3
  local retry=0
  
  while [ $retry -lt $max_retries ]; do
    JOB_UUID=$(rnx job run --json $cmd | jq -r .id)
    
    if [ $? -eq 0 ]; then
      echo "Job submitted: $JOB_UUID"  # e.g., f47ac10b-58cc-4372-a567-0e02b2c3d479
      return 0
    fi
    
    retry=$((retry + 1))
    sleep 2
  done
  
  echo "Failed to submit job after $max_retries attempts"
  return 1
}
```

### 3. Cleanup

```bash
# Always cleanup volumes
trap 'rnx volume remove temp-vol 2>/dev/null' EXIT

rnx volume create temp-vol --size=1GB
rnx job run --volume=temp-vol process_data.sh
```

### 4. Logging

```bash
# Comprehensive logging
JOB_UUID=$(rnx job run --json \
  backup.sh | jq -r .id)
# JOB_UUID will be something like: f47ac10b-58cc-4372-a567-0e02b2c3d479

# Save all job info
mkdir -p logs/$JOB_UUID
rnx job status $JOB_UUID > logs/$JOB_UUID/status.txt
rnx job log $JOB_UUID > logs/$JOB_UUID/output.log
```

### 5. Security

```bash
# Don't embed secrets in commands
# Bad:
rnx job run curl -H "Authorization: Bearer secret123" api.example.com

# Good:
rnx job run --env=API_TOKEN=secret123 \
  curl -H "Authorization: Bearer \$API_TOKEN" api.example.com
```

## Troubleshooting

Common issues and solutions:

1. **Job Fails Immediately**
    - Check command syntax
    - Verify uploaded files exist
    - Check resource limits

2. **Out of Memory**
    - Increase --max-memory
    - Optimize memory usage
    - Use streaming processing

3. **Job Hangs**
    - Check CPU limits
    - Monitor with `rnx job log <job-uuid>`
    - Set appropriate timeout

4. **File Not Found**
    - Verify upload succeeded
    - Check working directory
    - Use absolute paths

See [Troubleshooting Guide](./TROUBLESHOOTING.md) for more solutions.