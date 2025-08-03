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

```bash
# Run a single command
rnx run echo "Hello, Joblet!"

# Run command with arguments
rnx run ls -la /

# Run shell command
rnx run sh -c "echo 'Current time:' && date"

# Run Python script
rnx run python3 -c "print('Hello from Python')"
```

### Command Chaining

```bash
# Using shell
rnx run bash -c "cd /tmp && echo 'test' > file.txt && cat file.txt"

# Using && operator
rnx run sh -c "apt update && apt install -y curl"

# Multi-line scripts
rnx run bash -c '
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
rnx run --max-cpu=50 stress-ng --cpu 1 --timeout 60s

# Limit to 2 full CPU cores (200%)
rnx run --max-cpu=200 python3 parallel_processing.py

# Bind to specific CPU cores
rnx run --cpu-cores="0,2,4,6" compute_intensive_task

# Range of cores
rnx run --cpu-cores="0-3" make -j4
```

CPU limit calculation:
- 100 = 1 full CPU core
- 50 = 0.5 CPU core (50% of one core)
- 200 = 2 full CPU cores
- 0 = Unlimited (use all available cores)

### Memory Limits

```bash
# Limit to 512MB
rnx run --max-memory=512 python3 data_processing.py

# Limit to 2GB
rnx run --max-memory=2048 java -jar app.jar

# Combine with CPU limits
rnx run --max-cpu=150 --max-memory=1024 node app.js
```

Memory behavior:
- Process killed if limit exceeded (OOM)
- Includes all memory types (RSS, cache, etc.)
- Swap disabled within job

### I/O Bandwidth Limits

```bash
# Limit to 10MB/s
rnx run --max-iobps=10485760 dd if=/dev/zero of=/work/test.dat bs=1M count=100

# Limit to 100MB/s
rnx run --max-iobps=104857600 tar -czf backup.tar.gz /data

# Combine all limits
rnx run \
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
rnx run --upload=script.py python3 script.py

# Upload multiple files
rnx run \
  --upload=config.json \
  --upload=data.csv \
  python3 process.py config.json data.csv

# Upload and rename
echo "data" > local.txt
rnx run --upload=local.txt cat local.txt
```

### Uploading Directories

```bash
# Upload entire directory
rnx run --upload-dir=./project npm start

# Upload directory with specific structure
rnx run --upload-dir=./src python3 -m src.main

# Large directory upload
rnx run --upload-dir=./dataset python3 train_model.py
```

### Working Directory

```bash
# Set working directory
rnx run --workdir=/work/app --upload-dir=./app npm start

# Default is /work
rnx run pwd  # Output: /work
```

### File Access in Jobs

```bash
# Uploaded files are in current directory
rnx run --upload=data.txt cat data.txt

# Access uploaded directory contents
rnx run --upload-dir=./config ls -la

# Write output files
rnx run --volume=output python3 -c "
with open('/volumes/output/result.txt', 'w') as f:
    f.write('Processing complete')
"
```

## Environment Variables

### Setting Environment Variables

```bash
# Single variable
rnx run --env=DEBUG=true node app.js

# Multiple variables
rnx run \
  --env=DATABASE_URL=postgres://localhost/db \
  --env=API_KEY=secret123 \
  --env=NODE_ENV=production \
  node server.js

# Variables with spaces
rnx run --env="MESSAGE=Hello World" echo '$MESSAGE'
```

### Default Environment

Every job has these variables set:
- `PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin`
- `HOME=/work`
- `USER=nobody` (or configured user)
- `PWD=/work`

### Using Environment Files

```bash
# Create env file
cat > .env << EOF
DATABASE_URL=postgres://localhost/mydb
REDIS_URL=redis://localhost:6379
API_TOKEN=secret
EOF

# Upload and source
rnx run --upload=.env bash -c 'source .env && echo $DATABASE_URL'
```

## Job Scheduling

### Relative Time Scheduling

```bash
# Run in 5 minutes
rnx run --schedule="5min" backup.sh

# Run in 2 hours
rnx run --schedule="2h" cleanup.sh

# Run in 30 seconds
rnx run --schedule="30s" quick_task.sh

# Supported units: s, m, min, h, d
```

### Absolute Time Scheduling

```bash
# Run at specific time (RFC3339 format)
rnx run --schedule="2025-08-03T15:00:00Z" daily_report.py

# With timezone
rnx run --schedule="2025-08-03T15:00:00-07:00" meeting_reminder.sh

# Tomorrow at noon
TOMORROW_NOON=$(date -d "tomorrow 12:00" --rfc-3339=seconds)
rnx run --schedule="$TOMORROW_NOON" lunch_reminder.sh
```

### Managing Scheduled Jobs

```bash
# List scheduled jobs
rnx list --json | jq '.[] | select(.status == "SCHEDULED")'

# Cancel scheduled job
rnx stop <job-id>

# Check when job will run
rnx status <job-id>
```

## Job Lifecycle

### Job States

1. **INITIALIZING** - Job accepted, preparing execution
2. **SCHEDULED** - Job scheduled for future execution  
3. **RUNNING** - Job actively executing
4. **COMPLETED** - Job finished successfully (exit code 0)
5. **FAILED** - Job finished with error (non-zero exit code)
6. **STOPPED** - Job manually stopped

### Monitoring Job Progress

```bash
# Real-time status
watch -n 1 rnx status <job-id>

# Follow logs
rnx log -f <job-id>

# List running jobs
rnx list --json | jq '.[] | select(.status == "RUNNING")'
```

### Job Completion

```bash
# Check exit code
rnx status <job-id> | grep "Exit Code"

# Get final output
rnx log <job-id> | tail -20

# Script to wait for completion
JOB_ID=$(rnx run --json long_task.sh | jq -r .id)
while [[ $(rnx status --json $JOB_ID | jq -r .status) == "RUNNING" ]]; do
  sleep 5
done
echo "Job completed with exit code: $(rnx status --json $JOB_ID | jq .exit_code)"
```

## Output and Logging

### Capturing Output

```bash
# Save logs to file
rnx log <job-id> > output.log

# Stream to file
rnx log -f <job-id> | tee running.log

# Parse JSON output
rnx run --json echo "test" | jq .

# Get only stdout
rnx log <job-id> 2>/dev/null
```

### Log Formatting

```bash
# With timestamps
rnx log --timestamps <job-id>

# Last N lines
rnx log --tail=100 <job-id>

# Follow with grep
rnx log -f <job-id> | grep ERROR
```

### Persistent Output

```bash
# Use volume for output files
rnx volume create results --size=10GB
rnx run --volume=results python3 analysis.py

# Retrieve results
rnx run --volume=results cat /volumes/results/report.pdf > report.pdf
```

## Advanced Features

### Complex Workflows

```bash
#!/bin/bash
# Multi-stage data processing pipeline

# Stage 1: Data preparation
PREP_JOB=$(rnx run --json \
  --volume=pipeline-data \
  --upload=prepare_data.py \
  python3 prepare_data.py | jq -r .id)

# Wait for completion
while [[ $(rnx status --json $PREP_JOB | jq -r .status) == "RUNNING" ]]; do
  sleep 2
done

# Stage 2: Parallel processing
for i in {1..4}; do
  rnx run \
    --volume=pipeline-data \
    --max-cpu=100 \
    --upload=process_chunk.py \
    python3 process_chunk.py $i &
done
wait

# Stage 3: Merge results
rnx run \
  --volume=pipeline-data \
  --upload=merge_results.py \
  python3 merge_results.py
```

### Job Dependencies

```bash
# Simple dependency chain
JOB1=$(rnx run --json setup.sh | jq -r .id)
# Wait for job1
while [[ $(rnx status --json $JOB1 | jq -r .status) == "RUNNING" ]]; do
  sleep 1
done

# Only run if job1 succeeded
if [[ $(rnx status --json $JOB1 | jq .exit_code) -eq 0 ]]; then
  rnx run process.sh
fi
```

### Resource Pools

```bash
# Create dedicated network for job group
rnx network create batch-network --cidr=10.50.0.0/24

# Run jobs in same network
for task in tasks/*.sh; do
  rnx run \
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
rnx run --volume=commands --upload=interactive.sh bash interactive.sh

# Send commands
rnx run --volume=commands sh -c 'echo "ls -la" > /volumes/commands/next.txt'
```

## Best Practices

### 1. Resource Planning

```bash
# Test resource requirements first
rnx run --max-memory=512 python3 script.py

# If OOM, increase limit
rnx run --max-memory=1024 python3 script.py

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
    JOB_ID=$(rnx run --json $cmd | jq -r .id)
    
    if [ $? -eq 0 ]; then
      echo "Job submitted: $JOB_ID"
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
rnx run --volume=temp-vol process_data.sh
```

### 4. Logging

```bash
# Comprehensive logging
JOB_ID=$(rnx run --json \
  --name="daily-backup-$(date +%Y%m%d)" \
  backup.sh | jq -r .id)

# Save all job info
mkdir -p logs/$JOB_ID
rnx status $JOB_ID > logs/$JOB_ID/status.txt
rnx log $JOB_ID > logs/$JOB_ID/output.log
```

### 5. Security

```bash
# Don't embed secrets in commands
# Bad:
rnx run curl -H "Authorization: Bearer secret123" api.example.com

# Good:
rnx run --env=API_TOKEN=secret123 \
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
   - Monitor with `rnx log <job-id>`
   - Set appropriate timeout

4. **File Not Found**
   - Verify upload succeeded
   - Check working directory
   - Use absolute paths

See [Troubleshooting Guide](./TROUBLESHOOTING.md) for more solutions.