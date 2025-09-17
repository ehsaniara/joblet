# Volume Management Guide

Complete guide to managing persistent and temporary storage volumes in Joblet.

## Table of Contents

- [Volume Overview](#volume-overview)
- [Volume Types](#volume-types)
- [Creating Volumes](#creating-volumes)
- [Using Volumes in Jobs](#using-volumes-in-jobs)
- [Volume Operations](#volume-operations)
- [Data Persistence](#data-persistence)
- [Performance Considerations](#performance-considerations)
- [Best Practices](#best-practices)
- [Troubleshooting](#troubleshooting)

## Volume Overview

Joblet volumes provide persistent and temporary storage for jobs, enabling:

- Data persistence across job runs
- Shared storage between jobs
- High-performance temporary storage
- Isolation between different workloads

### Key Features

- **Two volume types**: Filesystem (persistent) and Memory (temporary)
- **Size limits**: Configurable size constraints with enforcement
- **Isolation**: Each job sees only its assigned volumes
- **Mount location**: Volumes mounted at `/volumes/<name>`
- **Performance**: Memory volumes for high-speed I/O

## Volume Types

### Filesystem Volumes

Persistent disk-based storage that survives job restarts and system reboots.

**Characteristics:**

- Data persists indefinitely
- Backed by loop devices with size enforcement (falls back to directories if loop setup fails)
- Suitable for databases, file storage, results
- Subject to disk I/O performance
- Can be manually backed up via job commands

**Use cases:**

- Database storage
- ML model checkpoints
- Processing results
- Configuration files
- Log archives

### Memory Volumes

Temporary RAM-based storage (tmpfs) cleared when the volume is removed.

**Characteristics:**

- Extremely fast read/write
- Data lost on system restart or volume removal
- Limited by available RAM
- No disk I/O bottleneck
- Size-limited tmpfs mounts

**Use cases:**

- Temporary processing space
- Cache storage
- Inter-job communication
- Build artifacts
- Session data

## Creating Volumes

### Basic Volume Creation

```bash
# Create 1GB filesystem volume
rnx volume create mydata --size=1GB --type=filesystem

# Create 512MB memory volume
rnx volume create cache --size=512MB --type=memory

# Default type is filesystem
rnx volume create storage --size=5GB
```

### Size Specifications

```bash
# Supported size units
rnx volume create small --size=100MB      # Megabytes
rnx volume create medium --size=5GB       # Gigabytes
rnx volume create large --size=1TB        # Terabytes
rnx volume create tiny --size=50KB        # Kilobytes

# Precise sizes in bytes
rnx volume create exact --size=1073741824  # Bytes (1GB)
```

### Naming Conventions

Volume names must:

- Start with a letter
- Contain only letters, numbers, hyphens, underscores
- Be unique within the Joblet instance
- Be 1-63 characters long

```bash
# Valid names
rnx volume create user-data --size=1GB
rnx volume create app_cache_v2 --size=500MB
rnx volume create Dataset2024 --size=10GB

# Invalid names (will fail)
rnx volume create 123data --size=1GB      # Starts with number
rnx volume create my.data --size=1GB      # Contains period
rnx volume create "my data" --size=1GB    # Contains space
```

## Using Volumes in Jobs

### Mounting Volumes

```bash
# Mount single volume
rnx job run --volume=mydata ls -la /volumes/mydata

# Mount multiple volumes
rnx job run \
  --volume=input-data \
  --volume=output-data \
  --volume=cache \
  python3 process.py

# Volume is mounted read-write by default
rnx job run --volume=config cat /volumes/config/settings.json
```

### Reading and Writing Data

```bash
# Write to volume
rnx job run --volume=results bash -c '
  echo "Processing results" > /volumes/results/output.txt
  date >> /volumes/results/output.txt
'

# Read from volume in separate job
rnx job run --volume=results cat /volumes/results/output.txt

# Copy files to volume
rnx job run --volume=backup --upload=data.tar.gz \
  cp data.tar.gz /volumes/backup/

# Process files in volume
rnx job run --volume=dataset python3 -c "
import os
files = os.listdir('/volumes/dataset')
print(f'Found {len(files)} files')
"
```

### Volume Paths

All volumes are mounted under `/volumes/` with their name:

```bash
# Volume 'mydata' → /volumes/mydata
# Volume 'cache' → /volumes/cache
# Volume 'ml-models' → /volumes/ml-models

# List all mounted volumes
rnx job run --volume=data1 --volume=data2 ls -la /volumes/
```

## Volume Operations

### Listing Volumes

```bash
# List all volumes
rnx volume list

# Output format:
# NAME          SIZE    TYPE         CREATED
# mydata        1GB     filesystem   2025-08-03 10:00:00
# cache         512MB   memory       2025-08-03 10:05:00

# JSON output
rnx volume list --json
```

### Checking Volume Usage

Since there's no built-in usage monitoring, use job commands to check volume usage:

```bash
# Check space usage in filesystem volume
rnx job run --volume=mydata df -h /volumes/mydata

# Detailed usage
rnx job run --volume=mydata du -sh /volumes/mydata/*

# Find large files
rnx job run --volume=logs \
  find /volumes/logs -type f -size +100M -exec ls -lh {} \;
```

### Removing Volumes

```bash
# Remove single volume
rnx volume remove mydata

# Note: Volume must not be in use by any active jobs
# If removal fails due to active jobs, stop the jobs first
```

## Data Persistence

### Persistent Data Workflows

```bash
# 1. Create volume for persistent storage
rnx volume create ml-checkpoints --size=50GB

# 2. Save model checkpoints during training
rnx job run \
  --volume=ml-checkpoints \
  --upload=train.py \
  --max-cpu=800 \
  --max-memory=16384 \
  python3 train.py --checkpoint-dir=/volumes/ml-checkpoints

# 3. Resume training from checkpoint in separate job
rnx job run \
  --volume=ml-checkpoints \
  --upload=train.py \
  python3 train.py --resume=/volumes/ml-checkpoints/latest.pth

# 4. Export final model (download logs to get file)
JOB_ID=$(rnx job run \
  --volume=ml-checkpoints \
  --json \
  bash -c 'cat /volumes/ml-checkpoints/best_model.pth' | jq -r .id)

# Wait for job completion then download
sleep 5
rnx job log $JOB_ID > model.pth
```

### Data Sharing Between Jobs

```bash
# Job 1: Generate data
rnx job run --volume=shared-data python3 -c "
import json
data = {'status': 'processed', 'count': 1000}
with open('/volumes/shared-data/status.json', 'w') as f:
    json.dump(data, f)
"

# Job 2: Read shared data (runs after Job 1 completes)
rnx job run --volume=shared-data python3 -c "
import json
with open('/volumes/shared-data/status.json', 'r') as f:
    data = json.load(f)
print(f'Status: {data[\"status\"]}, Count: {data[\"count\"]}')
"
```

### Manual Backup and Restore

Joblet doesn't have built-in backup commands, but you can implement backup workflows using job commands:

```bash
# Create backup job
BACKUP_JOB=$(rnx job run --json \
  --volume=important-data \
  tar -czf /work/backup.tar.gz -C /volumes/important-data . \
  | jq -r .id)

# Wait for completion
sleep 5

# Download backup by getting job logs
rnx job log $BACKUP_JOB > important-data-backup.tar.gz

# Restore to new volume
rnx volume create restored-data --size=10GB
rnx job run \
  --volume=restored-data \
  --upload=important-data-backup.tar.gz \
  tar -xzf important-data-backup.tar.gz -C /volumes/restored-data
```

## Performance Considerations

### Filesystem Volume Performance

Test volume performance using job commands:

```bash
# Test write performance
rnx job run --volume=perf-test dd \
  if=/dev/zero \
  of=/volumes/perf-test/test.dat \
  bs=1M count=1000 \
  conv=fdatasync

# Test read performance
rnx job run --volume=perf-test dd \
  if=/volumes/perf-test/test.dat \
  of=/dev/null \
  bs=1M

# Check volume mount and filesystem type
rnx job run --volume=perf-test bash -c '
  mount | grep /volumes/perf-test
  df -T /volumes/perf-test
'
```

### Memory Volume Performance

```bash
# Memory volumes are much faster for I/O operations
rnx job run --volume=mem-cache --max-memory=2048 python3 -c "
import time
import os

# Write test
start = time.time()
with open('/volumes/mem-cache/test.dat', 'wb') as f:
    f.write(os.urandom(500 * 1024 * 1024))  # 500MB
write_time = time.time() - start

# Read test
start = time.time()
with open('/volumes/mem-cache/test.dat', 'rb') as f:
    data = f.read()
read_time = time.time() - start

print(f'Write: {500/write_time:.2f} MB/s')
print(f'Read: {500/read_time:.2f} MB/s')
"
```

### Optimizing Volume Usage

```bash
# Use memory volumes for temporary data
rnx volume create temp-work --size=2GB --type=memory

# Process large dataset with staging pattern
rnx job run \
  --volume=source-data \
  --volume=temp-work \
  --volume=results \
  bash -c '
    # Copy input to fast memory volume
    cp /volumes/source-data/* /volumes/temp-work/
    
    # Process in memory
    process_data.py --input=/volumes/temp-work --output=/volumes/results
    
    # Clean up temporary files
    rm /volumes/temp-work/*
  '

# Regular cleanup using job scheduling
rnx job run --schedule="168h" --volume=logs bash -c '
  find /volumes/logs -name "*.tmp" -mtime +7 -delete
  find /volumes/logs -name "*.log" -mtime +30 -delete
'
```

## Best Practices

### 1. Volume Sizing

```bash
# Start with reasonable sizes and monitor usage
rnx volume create test-vol --size=1GB

# Monitor usage regularly
rnx job run --volume=test-vol df -h /volumes/test-vol

# Create larger volume if needed (no resize capability)
rnx volume create test-vol-large --size=10GB

# Migrate data manually
rnx job run \
  --volume=test-vol \
  --volume=test-vol-large \
  cp -r /volumes/test-vol/* /volumes/test-vol-large/

# Remove old volume after migration
rnx volume remove test-vol
```

### 2. Naming Strategy

```bash
# Use descriptive names with versioning
rnx volume create user-data-v1 --size=5GB
rnx volume create ml-models-2024 --size=50GB
rnx volume create cache-layer-prod --size=2GB --type=memory

# Environment-specific naming
rnx volume create dev-database --size=10GB
rnx volume create staging-uploads --size=20GB
rnx volume create prod-backups --size=100GB
```

### 3. Data Organization

```bash
# Create directory structure in jobs
rnx job run --volume=project-data bash -c '
  mkdir -p /volumes/project-data/{input,output,temp,logs}
  mkdir -p /volumes/project-data/archives/$(date +%Y/%m)
'

# Use subdirectories for organization
rnx job run --volume=ml-data bash -c '
  mkdir -p /volumes/ml-data/{datasets,models,checkpoints,metrics}
'
```

### 4. Cleanup Strategy

```bash
# Create cleanup script
cat > cleanup.sh << 'EOF'
#!/bin/bash
# Remove old temporary files
find /volumes/temp-data -name "*.tmp" -mtime +7 -delete

# Compress old logs  
find /volumes/logs -name "*.log" -mtime +30 -exec gzip {} \;

# Remove empty directories
find /volumes/data -type d -empty -delete
EOF

# Schedule regular cleanup (requires job scheduling)
rnx job run \
  --schedule="168h" \
  --volume=temp-data \
  --volume=logs \
  --volume=data \
  --upload=cleanup.sh \
  bash cleanup.sh
```

### 5. Security and Data Protection

```bash
# Handle sensitive data with encryption in jobs
rnx volume create secrets --size=100MB

# Store encrypted data
rnx job run --volume=secrets --env=ENCRYPTION_KEY=xxx bash -c '
  echo "sensitive data" | openssl enc -aes-256-cbc -k "$ENCRYPTION_KEY" \
    > /volumes/secrets/data.enc
'

# Retrieve and decrypt
rnx job run --volume=secrets --env=ENCRYPTION_KEY=xxx bash -c '
  openssl enc -aes-256-cbc -d -k "$ENCRYPTION_KEY" \
    < /volumes/secrets/data.enc
'
```

## Troubleshooting

### Common Issues

**1. Volume Creation Fails**

```bash
# Error: "failed to create volume: operation not permitted"
# Solution: Check server has proper permissions
# Ensure joblet runs with necessary privileges for loop device setup
```

**2. Volume Not Found**

```bash
# Error: "volume mydata not found"
# Check volume exists
rnx volume list

# Recreate if needed
rnx volume create mydata --size=1GB
```

**3. Out of Space**

```bash
# Error: "No space left on device"
# Check volume usage
rnx job run --volume=full-vol df -h /volumes/full-vol

# Create larger volume (no resize capability)
rnx volume create full-vol-v2 --size=20GB

# Migrate data
rnx job run --volume=full-vol --volume=full-vol-v2 \
  cp -r /volumes/full-vol/* /volumes/full-vol-v2/

# Remove old volume
rnx volume remove full-vol
```

**4. Permission Denied**

```bash
# Error: "Permission denied"
# Volumes are owned by job user
# Fix permissions within job
rnx job run --volume=data bash -c '
  # Check current permissions
  ls -la /volumes/data
  
  # Fix if needed (be careful with chmod 777)
  chmod -R 755 /volumes/data
'
```

**5. Memory Volume Full**

```bash
# Memory volumes limited by available RAM and specified size
# Check system memory and volume size
rnx job run --volume=mem-vol df -h /volumes/mem-vol

# Use smaller memory volume or switch to filesystem volume
rnx volume create cache-small --size=256MB --type=memory
```

**6. Volume Removal Blocked**

```bash
# Error: Volume is in use by active jobs
# List running jobs
rnx job list

# Stop jobs using the volume
rnx job stop <job-id>

# Then remove volume
rnx volume remove mydata
```

### Debugging Tips

```bash
# Check volume mount status
rnx job run --volume=debug-vol mount | grep volumes

# Verify volume permissions and ownership
rnx job run --volume=debug-vol ls -la /volumes/

# Test write access
rnx job run --volume=debug-vol bash -c '
  touch /volumes/debug-vol/test.txt
  echo "Write test successful"
  rm /volumes/debug-vol/test.txt
'

# Check filesystem type (for filesystem volumes)
rnx job run --volume=debug-vol stat -f /volumes/debug-vol

# For memory volumes, verify tmpfs mount
rnx job run --volume=mem-vol mount | grep tmpfs
```

### Volume State and Recovery

```bash
# Check volume metadata (stored in volume directory)
rnx job run --volume=debug-vol bash -c '
  if [ -f /volumes/debug-vol/.joblet_volume_meta.json ]; then
    cat /volumes/debug-vol/.joblet_volume_meta.json
  else
    echo "No volume metadata found"
  fi
'

# Verify volume size limits
rnx job run --volume=debug-vol df -h /volumes/debug-vol
```

## Examples

### Database Storage

```bash
# Create volume for PostgreSQL
rnx volume create postgres-data --size=50GB

# Run PostgreSQL with persistent storage
rnx job run \
  --volume=postgres-data \
  --env=POSTGRES_PASSWORD=secret \
  --env=PGDATA=/volumes/postgres-data \
  --network=db-network \
  --runtime=postgres:latest \
  postgres
```

### Build Cache

```bash
# Create build cache volume (memory for speed)
rnx volume create build-cache --size=2GB --type=memory

# Use for faster builds
rnx job run \
  --volume=build-cache \
  --upload-dir=./src \
  --env=MAVEN_CACHE_DIR=/volumes/build-cache/maven \
  --runtime=java:17 \
  bash -c "
    mkdir -p /volumes/build-cache/maven
    mvn -Dmaven.repo.local=/volumes/build-cache/maven install
    mvn -Dmaven.repo.local=/volumes/build-cache/maven package
  "
```

### Data Pipeline

```bash
# Create volumes for pipeline stages
rnx volume create raw-data --size=100GB
rnx volume create processed-data --size=50GB
rnx volume create final-results --size=10GB

# Stage 1: Ingest data
rnx job run --volume=raw-data --upload=ingest_data.sh bash ingest_data.sh

# Stage 2: Process (runs after stage 1)
rnx job run \
  --volume=raw-data \
  --volume=processed-data \
  --upload=process_data.py \
  python3 process_data.py

# Stage 3: Analysis (runs after stage 2)
rnx job run \
  --volume=processed-data \
  --volume=final-results \
  --upload=analyze_results.py \
  python3 analyze_results.py
```

## Limitations

### Current Limitations

- **No volume resizing**: Create new volume and migrate data manually
- **No built-in backup**: Implement backup workflows using job commands
- **No volume info command**: Use `rnx volume list` for volume information
- **No force removal**: Volume removal blocked if jobs are using it
- **No usage monitoring**: Check usage via job commands using `df` and `du`

### Planned Features

Check the project roadmap for upcoming volume management features.

## See Also

- [Job Execution Guide](./JOB_EXECUTION.md)
- [Network Management](./NETWORK_MANAGEMENT.md)
- [Configuration Guide](./CONFIGURATION.md)
- [Volume Test Suite](../tests/e2e/volume/)