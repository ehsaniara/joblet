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
- **Size limits**: Configurable size constraints
- **Isolation**: Each job sees only its assigned volumes
- **Mount location**: Volumes mounted at `/volumes/<name>`
- **Performance**: Memory volumes for high-speed I/O

## Volume Types

### Filesystem Volumes

Persistent disk-based storage that survives job restarts and system reboots.

**Characteristics:**
- Data persists indefinitely
- Backed by disk storage (ext4 by default)
- Suitable for databases, file storage, results
- Subject to disk I/O performance
- Can be backed up

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
- Data lost on system restart
- Limited by available RAM
- No disk I/O bottleneck
- Cannot be backed up

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

# Precise sizes
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
rnx volume create my data --size=1GB      # Contains space
```

## Using Volumes in Jobs

### Mounting Volumes

```bash
# Mount single volume
rnx run --volume=mydata ls -la /volumes/mydata

# Mount multiple volumes
rnx run \
  --volume=input-data \
  --volume=output-data \
  --volume=cache \
  python3 process.py

# Volume is mounted read-write by default
rnx run --volume=config cat /volumes/config/settings.json
```

### Reading and Writing Data

```bash
# Write to volume
rnx run --volume=results bash -c '
  echo "Processing results" > /volumes/results/output.txt
  date >> /volumes/results/output.txt
'

# Read from volume
rnx run --volume=results cat /volumes/results/output.txt

# Copy files to volume
rnx run --volume=backup --upload=data.tar.gz \
  cp data.tar.gz /volumes/backup/

# Process files in volume
rnx run --volume=dataset python3 -c "
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
rnx run --volume=data1 --volume=data2 ls -la /volumes/
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

# JSON output for scripting
rnx volume list --json

# Filter by type (requires parsing)
rnx volume list --json | jq '.[] | select(.type == "memory")'
```

### Checking Volume Usage

```bash
# Check space usage in filesystem volume
rnx run --volume=mydata df -h /volumes/mydata

# Detailed usage
rnx run --volume=mydata du -sh /volumes/mydata/*

# Find large files
rnx run --volume=logs \
  find /volumes/logs -type f -size +100M -exec ls -lh {} \;
```

### Removing Volumes

```bash
# Remove single volume
rnx volume remove mydata

# Remove with confirmation
echo "y" | rnx volume remove important-data

# Force remove (if implemented)
rnx volume remove temp-cache --force

# Remove all volumes matching pattern
rnx volume list --json | \
  jq -r '.[] | select(.name | startswith("temp-")) | .name' | \
  xargs -I {} rnx volume remove {}
```

## Data Persistence

### Persistent Data Workflows

```bash
# 1. Create volume for persistent storage
rnx volume create ml-checkpoints --size=50GB

# 2. Save model checkpoints during training
rnx run \
  --volume=ml-checkpoints \
  --upload=train.py \
  --max-cpu=800 \
  --max-memory=16384 \
  python3 train.py --checkpoint-dir=/volumes/ml-checkpoints

# 3. Resume training from checkpoint
rnx run \
  --volume=ml-checkpoints \
  --upload=train.py \
  python3 train.py --resume=/volumes/ml-checkpoints/latest.pth

# 4. Export final model
rnx run \
  --volume=ml-checkpoints \
  cp /volumes/ml-checkpoints/best_model.pth /work/
rnx log <job-id> > model.pth
```

### Data Sharing Between Jobs

```bash
# Job 1: Generate data
rnx run --volume=shared-data python3 -c "
import json
data = {'status': 'processed', 'count': 1000}
with open('/volumes/shared-data/status.json', 'w') as f:
    json.dump(data, f)
"

# Job 2: Read shared data
rnx run --volume=shared-data python3 -c "
import json
with open('/volumes/shared-data/status.json', 'r') as f:
    data = json.load(f)
print(f'Status: {data[\"status\"]}, Count: {data[\"count\"]}')
"
```

### Backup and Restore

```bash
# Backup volume data
BACKUP_JOB=$(rnx run --json \
  --volume=important-data \
  tar -czf /work/backup.tar.gz -C /volumes/important-data . \
  | jq -r .id)

# Wait for completion
sleep 5

# Download backup
rnx log $BACKUP_JOB > important-data-backup.tar.gz

# Restore to new volume
rnx volume create restored-data --size=10GB
rnx run \
  --volume=restored-data \
  --upload=important-data-backup.tar.gz \
  tar -xzf important-data-backup.tar.gz -C /volumes/restored-data
```

## Performance Considerations

### Filesystem Volume Performance

```bash
# Test write performance
rnx run --volume=perf-test dd \
  if=/dev/zero \
  of=/volumes/perf-test/test.dat \
  bs=1M count=1000 \
  conv=fdatasync

# Test read performance
rnx run --volume=perf-test dd \
  if=/volumes/perf-test/test.dat \
  of=/dev/null \
  bs=1M

# Random I/O test
rnx run --volume=perf-test fio \
  --name=random-rw \
  --ioengine=posixaio \
  --rw=randrw \
  --bs=4k \
  --numjobs=4 \
  --size=1g \
  --runtime=30 \
  --directory=/volumes/perf-test
```

### Memory Volume Performance

```bash
# Memory volumes are much faster
rnx run --volume=mem-cache --max-memory=2048 python3 -c "
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

# Process large dataset with staging
rnx run \
  --volume=source-data \
  --volume=temp-work \
  --volume=results \
  python3 -c "
# Read from persistent storage
# Process in memory volume
# Write results to persistent storage
"

# Clean up temporary files regularly
rnx run --volume=logs bash -c '
  find /volumes/logs -name "*.tmp" -mtime +7 -delete
  find /volumes/logs -name "*.log" -mtime +30 -delete
'
```

## Best Practices

### 1. Volume Sizing

```bash
# Start small and grow as needed
rnx volume create test-vol --size=1GB

# Monitor usage
rnx run --volume=test-vol df -h /volumes/test-vol

# Create new larger volume if needed
rnx volume create test-vol-large --size=10GB

# Migrate data
rnx run \
  --volume=test-vol \
  --volume=test-vol-large \
  cp -r /volumes/test-vol/* /volumes/test-vol-large/
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
# Create directory structure
rnx run --volume=project-data bash -c '
  mkdir -p /volumes/project-data/{input,output,temp,logs}
  mkdir -p /volumes/project-data/archives/$(date +%Y/%m)
'

# Use subdirectories for organization
rnx run --volume=ml-data bash -c '
  mkdir -p /volumes/ml-data/{datasets,models,checkpoints,metrics}
'
```

### 4. Cleanup Strategy

```bash
# Regular cleanup job
cat > cleanup.sh << 'EOF'
#!/bin/bash
# Remove old temporary files
find /volumes/temp-data -mtime +7 -delete

# Compress old logs
find /volumes/logs -name "*.log" -mtime +30 | while read log; do
  gzip "$log"
done

# Remove empty directories
find /volumes/data -type d -empty -delete
EOF

# Schedule weekly cleanup
rnx run \
  --schedule="168h" \
  --volume=temp-data \
  --volume=logs \
  --volume=data \
  --upload=cleanup.sh \
  bash cleanup.sh
```

### 5. Security

```bash
# Sensitive data handling
# Create volume for secrets
rnx volume create secrets --size=100MB

# Store encrypted data
rnx run --volume=secrets --env=ENCRYPTION_KEY=xxx bash -c '
  echo "sensitive data" | openssl enc -aes-256-cbc -k "$ENCRYPTION_KEY" \
    > /volumes/secrets/data.enc
'

# Retrieve and decrypt
rnx run --volume=secrets --env=ENCRYPTION_KEY=xxx bash -c '
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
# Ensure joblet runs with necessary privileges
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
rnx run --volume=full-vol df -h /volumes/full-vol

# Clean up or create larger volume
rnx volume create full-vol-v2 --size=20GB
```

**4. Permission Denied**
```bash
# Error: "Permission denied"
# Volumes are owned by job user (usually 'nobody')
# Fix by using appropriate permissions
rnx run --volume=data chmod -R 777 /volumes/data
```

**5. Memory Volume Full**
```bash
# Memory volumes limited by available RAM
# Check system memory
rnx monitor status

# Use smaller memory volume or filesystem volume
rnx volume create cache --size=512MB --type=memory
```

### Debugging Tips

```bash
# Check volume mount
rnx run --volume=debug-vol mount | grep volumes

# Verify volume permissions
rnx run --volume=debug-vol ls -la /volumes/

# Test write access
rnx run --volume=debug-vol touch /volumes/debug-vol/test.txt

# Check filesystem type (filesystem volumes)
rnx run --volume=debug-vol stat -f /volumes/debug-vol
```

## Examples

### Database Storage

```bash
# Create volume for PostgreSQL
rnx volume create postgres-data --size=50GB

# Run PostgreSQL with persistent storage
rnx run \
  --volume=postgres-data \
  --env=POSTGRES_PASSWORD=secret \
  --env=PGDATA=/volumes/postgres-data \
  --network=db-network \
  postgres:latest
```

### Build Cache

```bash
# Create build cache volume
rnx volume create build-cache --size=10GB --type=memory

# Use for faster builds
rnx run \
  --volume=build-cache \
  --upload-dir=./src \
  --env=NPM_CACHE_DIR=/volumes/build-cache/npm \
  npm install && npm build
```

### Data Pipeline

```bash
# Create volumes for pipeline stages
rnx volume create raw-data --size=100GB
rnx volume create processed-data --size=50GB
rnx volume create final-results --size=10GB

# Stage 1: Ingest
rnx run --volume=raw-data ingest_data.sh

# Stage 2: Process
rnx run \
  --volume=raw-data \
  --volume=processed-data \
  process_data.py

# Stage 3: Analysis
rnx run \
  --volume=processed-data \
  --volume=final-results \
  analyze_results.py
```

## See Also

- [Job Execution Guide](./JOB_EXECUTION.md)
- [Network Management](./NETWORK_MANAGEMENT.md)
- [Configuration Guide](./CONFIGURATION.md)