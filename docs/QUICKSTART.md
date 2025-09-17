# Quick Start Guide

Installation and setup guide for Joblet.

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

### 2. Verify Version and Connection

```bash
# Check client and server versions
rnx --version

# This shows:
# - RNX Client version (your local CLI)
# - Joblet Server version (remote server)
# If versions differ significantly, consider updating

# Test connection by listing jobs
rnx job list  # Should show "No jobs found" initially
```

## 🎯 First Job

### Run a Simple Command

```bash
rnx job run echo "Hello, Joblet!"
```

Output:

```
Job started:
UUID: 550e8400-e29b-41d4-a716-446655440000
Command: echo Hello, Joblet!
Status: RUNNING
StartTime: 2025-08-03T10:00:00Z
```

### Check Job Status

```bash
rnx job status 550e8400-e29b-41d4-a716-446655440000
```

### View Job Logs

```bash
rnx job log 550e8400-e29b-41d4-a716-446655440000
```

## 📊 Resource Limits Example

Run a Python script with resource limits:

```bash
rnx job run --max-cpu=50 --max-memory=512 --max-iobps=10485760 \
  python3 -c "import time; print('Processing...'); time.sleep(5); print('Done!')"
```

This limits the job to:

- 50% CPU usage
- 512 MB memory
- 10 MB/s I/O bandwidth

## 🚀 Using Runtime Environments

Runtime environments provide pre-built, isolated environments for instant access to programming languages:

### Install Available Runtimes

```bash
# Install Python with ML packages (475MB, instant startup)
rnx runtime install python-3.11-ml

# Install Java OpenJDK 21 (292MB, instant startup)  
rnx runtime install openjdk:21

# List installed runtimes
rnx runtime list
```

### Use Runtimes

```bash
# Python with ML packages - no installation delay!
rnx job run --runtime=python-3.11-ml python3 -c "import pandas, numpy; print('ML ready!')"

# Java compilation and execution
rnx job run --runtime=openjdk:21 --upload=HelloWorld.java javac HelloWorld.java
rnx job run --runtime=openjdk:21 java HelloWorld

# Check runtime information
rnx runtime info python-3.11-ml
```

**Benefits:**

- ⚡ **Instant startup**: 2-3 seconds vs 5-45 minutes traditional package installation
- 🔒 **Isolated**: No host contamination, complete dependency isolation
- 📦 **Pre-configured**: All packages and tools ready to use

## 💾 Using Volumes

Create persistent storage:

```bash
# Create a 1GB filesystem volume
rnx volume create mydata --size=1GB --type=filesystem

# Run job with volume mounted
rnx job run --volume=mydata sh -c 'echo "Persistent data" > /volumes/mydata/data.txt'

# Verify data persists
rnx job run --volume=mydata cat /volumes/mydata/data.txt
```

## 🌐 Network Isolation

Create an isolated network:

```bash
# Create custom network
rnx network create isolated --cidr=10.10.0.0/24

# Run job in isolated network
rnx job run --network=isolated ping -c 3 google.com
# This will fail - no internet access in isolated network

# Run with default bridge network (internet access)
rnx job run --network=bridge ping -c 3 google.com
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
rnx job run --upload=test.sh bash test.sh
```

## 📅 Scheduled Jobs

Schedule a job for future execution:

```bash
# Run in 5 minutes
rnx job run --schedule="5min" echo "Scheduled job executed!"

# Run at specific time
rnx job run --schedule="2025-08-03T15:00:00" echo "Scheduled for 3 PM"
```

## 🏗️ Runtime Systems

Joblet provides pre-built runtime environments for instant job execution:

### Install a Runtime

```bash
# Install Java 21 runtime (automatically uses builder isolation)
rnx runtime install openjdk-21

# Install Python ML runtime with data science packages
rnx runtime install python-3.11-ml
```

**What happens during installation:**

1. Uses **RuntimeService** → automatically applies builder chroot
2. Downloads and installs runtime in isolated builder environment
3. **Cleanup phase** creates isolated runtime structure
4. Runtime ready for secure production use

### Using Runtimes

```bash
# Run Java application with isolated runtime
rnx job run --runtime=openjdk-21 java -version

# Run Python ML script with pre-installed packages
rnx job run --runtime=python-3.11-ml python3 -c "import pandas, numpy; print('ML ready!')"

# List available runtimes
rnx runtime list
```

**Security Benefits:**

- Production jobs use **isolated runtime copies** (no host OS access)
- Runtime files copied to `/opt/joblet/runtimes/{runtime}/isolated/`
- Complete filesystem isolation between runtime building and production use

## 🔍 Monitoring

Watch real-time system metrics:

```bash
# Monitor system metrics
rnx monitor

# Get current system status
rnx monitor status

# Stream job logs (use Ctrl+C to stop)
rnx job log <job-uuid>
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
rnx job run <command>           # Run a job
rnx job list                    # List all jobs
rnx job status <job-uuid>       # Check job status
rnx job log <job-uuid>          # View job logs
rnx job stop <job-uuid>         # Stop running job
rnx job delete <job-uuid>       # Delete specific job
rnx job delete-all              # Delete all non-running jobs

# Workflow Management
rnx job run --workflow=file.yaml    # Run workflow
rnx job list --workflow             # List workflows
rnx job status --workflow <uuid>    # Check workflow status
rnx job status --workflow --detail <uuid> # View workflow status + YAML
rnx job status --workflow --json --detail <uuid> # JSON output with YAML content

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