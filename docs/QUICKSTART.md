# Quick Start Guide

Get Joblet up and running in 5 minutes! This guide covers the essential steps to install and start using Joblet.

## 🚀 Installation

### Option 1: Download Pre-built Binaries

```bash
# Download the latest release
curl -L https://github.com/ehsaniara/joblet/releases/latest/download/joblet-linux-amd64.tar.gz | tar xz
sudo mv joblet /usr/local/bin/
sudo mv rnx /usr/local/bin/
```

### Option 2: Install from Source

```bash
# Clone the repository
git clone https://github.com/ehsaniara/joblet.git
cd joblet

# Build binaries
make build

# Install binaries
sudo make install
```

## 🔧 Server Setup

### 1. Generate Certificates

```bash
# Set your server address
export JOBLET_SERVER_ADDRESS='your-server-ip'

# Generate certificates with embedded configuration
sudo /usr/local/bin/certs_gen_embedded.sh
```

This creates:

- `/opt/joblet/config/joblet-config.yml` - Server configuration
- `/opt/joblet/config/rnx-config.yml` - Client configuration

### 2. Start Joblet Server

```bash
# Option 1: Run directly
sudo joblet

# Option 2: Install as systemd service
sudo systemctl enable joblet
sudo systemctl start joblet
```

### 3. Verify Server Status

```bash
sudo systemctl status joblet
```

## 💻 Client Setup

### 1. Copy Client Configuration

On your client machine:

```bash
# Create config directory
mkdir -p ~/.rnx

# Copy the client configuration from server
scp server:/opt/joblet/config/rnx-config.yml ~/.rnx/
```

### 2. Test Connection

```bash
# List jobs (should show "No jobs found" initially)
rnx list
```

## 🎯 First Job

### Run a Simple Command

```bash
rnx run echo "Hello, Joblet!"
```

Output:

```
Job started:
ID: 1
Command: echo Hello, Joblet!
Status: RUNNING
StartTime: 2025-08-03T10:00:00Z
```

### Check Job Status

```bash
rnx status 1
```

### View Job Logs

```bash
rnx log 1
```

## 📊 Resource Limits Example

Run a Python script with resource limits:

```bash
rnx run --max-cpu=50 --max-memory=512 --max-iobps=10485760 \
  python3 -c "import time; print('Processing...'); time.sleep(5); print('Done!')"
```

This limits the job to:

- 50% CPU usage
- 512 MB memory
- 10 MB/s I/O bandwidth

## 💾 Using Volumes

Create persistent storage:

```bash
# Create a 1GB filesystem volume
rnx volume create mydata --size=1GB --type=filesystem

# Run job with volume mounted
rnx run --volume=mydata sh -c 'echo "Persistent data" > /volumes/mydata/data.txt'

# Verify data persists
rnx run --volume=mydata cat /volumes/mydata/data.txt
```

## 🌐 Network Isolation

Create an isolated network:

```bash
# Create custom network
rnx network create isolated --cidr=10.10.0.0/24

# Run job in isolated network
rnx run --network=isolated ping -c 3 google.com
# This will fail - no internet access in isolated network

# Run with default bridge network (internet access)
rnx run --network=bridge ping -c 3 google.com
```

## 📁 File Uploads

Upload files to job workspace:

```bash
# Create a test script
echo '#!/bin/bash
echo "Running script in Joblet!"
echo "Hostname: $(hostname)"
echo "Working directory: $(pwd)"
' > test.sh

# Upload and run
rnx run --upload=test.sh bash test.sh
```

## 📅 Scheduled Jobs

Schedule a job for future execution:

```bash
# Run in 5 minutes
rnx run --schedule="5min" echo "Scheduled job executed!"

# Run at specific time
rnx run --schedule="2025-08-03T15:00:00" echo "Scheduled for 3 PM"
```

## 🔍 Monitoring

Watch real-time system metrics:

```bash
# Monitor system metrics
rnx monitor

# Get current system status
rnx monitor status

# Stream job logs in real-time
rnx log <job-id> --follow
```

## 🎉 Next Steps

Congratulations! You've successfully:

- ✅ Installed Joblet server and client
- ✅ Run your first job
- ✅ Applied resource limits
- ✅ Used volumes and networks
- ✅ Uploaded files and scheduled jobs

### Learn More

- [RNX CLI Reference](./RNX_CLI_REFERENCE.md) - All commands and options
- [Job Execution Guide](./JOB_EXECUTION.md) - Advanced job features
- [Configuration Guide](./CONFIGURATION.md) - Server and client configuration
- [Security Guide](./SECURITY.md) - mTLS and authentication

### Common Commands Cheat Sheet

```bash
# Job Management
rnx run <command>           # Run a job
rnx list                    # List all jobs
rnx status <job-id>         # Check job status
rnx log <job-id>            # View job logs
rnx stop <job-id>           # Stop running job

# Volume Management
rnx volume create <name>    # Create volume
rnx volume list             # List volumes
rnx volume remove <name>    # Remove volume

# Network Management
rnx network create <name>   # Create network
rnx network list            # List networks
rnx network delete <name>   # Delete network

# System Monitoring
rnx monitor                 # Real-time metrics
rnx monitor status          # Current status
```

Need help? Check the [Troubleshooting Guide](./TROUBLESHOOTING.md) or run `rnx help`.