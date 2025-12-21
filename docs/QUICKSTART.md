# Joblet Quick Start Guide

This guide provides step-by-step instructions for installing and configuring Joblet in your environment. The process
typically takes 10-15 minutes for a basic deployment.

## Installation Methods

### Method 1: Pre-Built Binary Installation

```bash
# Download the latest release
curl -L https://github.com/ehsaniara/joblet/releases/latest/download/joblet-linux-amd64.tar.gz | tar xz
sudo mv joblet /usr/local/bin/
sudo mv rnx /usr/local/bin/
```

### Method 2: Source Code Installation

```bash
# Clone the repository
git clone https://github.com/ehsaniara/joblet.git
cd joblet

# Build binaries
make build

# Install binaries
sudo make install
```

## Server Configuration

### Step 1: Certificate Generation

Joblet requires mutual TLS (mTLS) for secure communication between components. The certificate generation process
creates both server and client certificates with embedded configuration.

```bash
# Define the server's network address
export JOBLET_SERVER_ADDRESS='your-server-ip'

# Execute certificate generation with embedded configuration
sudo /usr/local/bin/certs_gen_embedded.sh
```

The certificate generation process produces the following artifacts:

- `/opt/joblet/config/joblet-config.yml` - Server configuration file with TLS certificates and operational parameters
- `/opt/joblet/config/rnx-config.yml` - Client configuration file containing connection details and authentication
  credentials

### Step 2: Initialize Joblet Server

The Joblet server can be launched in two operational modes:

#### Direct Execution Mode

```bash
# Launch server in foreground for testing
sudo joblet
```

#### System Service Mode (Recommended for Production)

```bash
# Register Joblet as a system service
sudo systemctl enable joblet

# Start the Joblet service
sudo systemctl start joblet
```

### Step 3: Verify Server Operation

Confirm the server is operational by checking its system service status:

```bash
# Check service status and recent logs
sudo systemctl status joblet

# View detailed service logs if needed
sudo journalctl -u joblet -n 50
```

## Client Configuration

### Step 1: Deploy Client Configuration

The RNX client requires the configuration file generated during server setup. Transfer this file to each client
workstation:

```bash
# Create config directory
mkdir -p ~/.rnx

# Copy the client configuration from server
scp server:/opt/joblet/config/rnx-config.yml ~/.rnx/
```

### Step 2: Validate Client Connectivity

Verify the client installation and server connectivity:

```bash
# Display version information
rnx --version
```

The version command displays:

- **RNX Client Version**: Version of the local command-line interface
- **Joblet Server Version**: Version of the remote server instance
- **API Compatibility**: Confirmation of API version compatibility

Note: Significant version discrepancies may require client or server updates.

```bash
# Validate server connectivity
rnx job list
```

Expected output for a new installation: "No jobs found"

## Initial Job Execution

### Execute Your First Job

Test the installation by executing a simple job:

```bash
# Submit a basic echo command as a job
rnx job run echo "Hello, Joblet!"
```

The command returns a job UUID and displays the execution output. This confirms that:

- The client can authenticate with the server
- The server can create and execute isolated processes
- The job execution pipeline is operational

Expected output format:

```
Job Initiated:
  UUID: 550e8400-e29b-41d4-a716-446655440000
  Command: echo Hello, Joblet!
  Status: RUNNING
  Timestamp: 2025-08-03T10:00:00Z
```

### Query Job Status

```bash
# Full UUID
rnx job status 550e8400-e29b-41d4-a716-446655440000

# Short UUID (first 8 characters) - works if it uniquely identifies the job
rnx job status 550e8400
```

### Retrieve Job Output

```bash
# Stream logs (supports short UUIDs)
rnx job log 550e8400

# View resource metrics
rnx job metrics 550e8400

# View eBPF telematics events (process executions, network connections)
rnx job telematics 550e8400
```

## Resource Management Configuration

The following example demonstrates resource constraint enforcement for a Python workload:

```bash
rnx job run --max-cpu=50 --max-memory=512 --max-iobps=10485760 \
  python3 -c "import time; print('Processing...'); time.sleep(5); print('Done!')"
```

Resource constraints applied:

- **CPU Allocation**: 50% of available compute capacity
- **Memory Quota**: 512 MB maximum memory consumption
- **I/O Bandwidth**: 10 MB/s disk throughput limitation

## Runtime Environment Management

Joblet's runtime environments provide production-ready, pre-configured execution contexts that eliminate dependency
management overhead and ensure consistent application behavior across deployments:

### Runtime Build Procedures

```bash
# Build Python with ML packages
rnx runtime build ./examples/python-3.11-ml/runtime.yaml

# Build Java OpenJDK 21
rnx runtime build ./examples/java-21/runtime.yaml

# List installed runtimes
rnx runtime list
```

### Runtime Utilization

```bash
# Python with ML packages - no installation delay!
rnx job run --runtime=python-3.11-ml python3 -c "import pandas, numpy; print('ML ready!')"

# Java compilation and execution
rnx job run --runtime=openjdk:21 --upload=HelloWorld.java javac HelloWorld.java
rnx job run --runtime=openjdk:21 java HelloWorld

# Check runtime information
rnx runtime info python-3.11-ml
```

**Operational Advantages:**

- **Rapid Initialization**: Job startup in 2-3 seconds compared to 5-45 minutes for traditional package installation
- **Complete Isolation**: Zero host system contamination with full dependency encapsulation
- **Production Ready**: Pre-installed packages and tools configured for immediate use

## Persistent Storage Configuration

Configure persistent storage volumes for data retention across job executions:

```bash
# Create a 1GB filesystem volume
rnx volume create mydata --size=1GB --type=filesystem

# Run job with volume mounted
rnx job run --volume=mydata sh -c 'echo "Persistent data" > /volumes/mydata/data.txt'

# Verify data persists
rnx job run --volume=mydata cat /volumes/mydata/data.txt
```

## Network Isolation and Segmentation

Configure network isolation boundaries for secure job execution:

```bash
# Create custom network
rnx network create isolated --cidr=10.10.0.0/24

# Run job in isolated network
rnx job run --network=isolated ping -c 3 google.com
# This will fail - no internet access in isolated network

# Run with default bridge network (internet access)
rnx job run --network=bridge ping -c 3 google.com
```

## File Transfer and Staging

Transfer local files to the job execution environment:

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

## Job Scheduling

Configure jobs for deferred execution using time-based scheduling:

```bash
# Run in 5 minutes
rnx job run --schedule="5min" echo "Scheduled job executed!"

# Run at specific time
rnx job run --schedule="2025-08-03T15:00:00" echo "Scheduled for 3 PM"
```

## Runtime System Architecture

The runtime system provides containerized language environments with pre-installed dependencies, enabling immediate job
execution without environment preparation overhead:

### Runtime Build Process

```bash
# Build Java 21 runtime from YAML specification
rnx runtime build ./examples/java-21/runtime.yaml

# Build Python ML runtime with data science packages
rnx runtime build ./examples/python-3.11-ml/runtime.yaml
```

**Build Workflow:**

1. **YAML Parsing**: Parse and validate runtime.yaml specification
2. **Platform Detection**: Detect OS distribution and package manager
3. **Isolated Build**: Install runtime components in sandboxed builder context
4. **Cleanup and Packaging**: Create isolated runtime structure with dependency resolution
5. **Production Deployment**: Runtime available for secure job execution

### Runtime Execution

```bash
# Run Java application with isolated runtime
rnx job run --runtime=openjdk-21 java -version

# Run Python ML script with pre-installed packages
rnx job run --runtime=python-3.11-ml python3 -c "import pandas, numpy; print('ML ready!')"

# List available runtimes
rnx runtime list
```

**Security Architecture:**

- **Isolated Runtime Instances**: Production jobs utilize isolated runtime copies without host OS access
- **Filesystem Segmentation**: Runtime artifacts deployed to `/opt/joblet/runtimes/{runtime}/isolated/`
- **Build-Runtime Separation**: Complete isolation between runtime construction and production execution phases

## System Monitoring and Observability

Monitor system performance and job execution metrics in real-time:

```bash
# Get current system status
rnx monitor status

# Show current server metrics with top processes
rnx monitor top

# Stream real-time server metrics
rnx monitor watch

# Stream job logs (use Ctrl+C to stop)
rnx job log <job-uuid>
```

## Next Steps

You have successfully completed the initial Joblet deployment:

- Installed and configured Joblet server components
- Deployed RNX client with authentication
- Executed jobs with resource constraints
- Configured persistent storage and network isolation
- Implemented file staging and job scheduling

### Additional Resources

- [RNX CLI Reference](./RNX_CLI_REFERENCE.md) - All commands and options
- [Job Execution Guide](./JOB_EXECUTION.md) - Advanced job features
- [Configuration Guide](./CONFIGURATION.md) - Server and client configuration
- [Security Guide](./SECURITY.md) - mTLS and authentication

### Command Reference Summary

**Note:** All commands that take a `<job-uuid>` support short-form UUIDs (first 8 characters) if they uniquely identify
a job.

```bash
# Job Management
rnx job run <command>           # Run a job
rnx job list                    # List all jobs
rnx job status <job-uuid>       # Check job status (supports short UUID)
rnx job log <job-uuid>          # View job logs (supports short UUID)
rnx job metrics <job-uuid>      # View resource metrics (supports short UUID)
rnx job telematics <job-uuid>   # View eBPF telematics events (supports short UUID)
rnx job stop <job-uuid>         # Stop running job (supports short UUID)
rnx job cancel <job-uuid>       # Cancel scheduled job (supports short UUID)
rnx job delete <job-uuid>       # Delete specific job (supports short UUID)
rnx job delete-all              # Delete all non-running jobs

# Volume Management
rnx volume create <name>    # Create volume
rnx volume list             # List volumes
rnx volume remove <name>    # Remove volume

# Network Management
rnx network create <name>   # Create network
rnx network list            # List networks
rnx network remove <name>   # Remove network

# System Monitoring
rnx monitor status          # Current status
rnx monitor top             # Server metrics with top processes
rnx monitor watch           # Real-time streaming metrics
```

For additional assistance, consult the [Troubleshooting Guide](./TROUBLESHOOTING.md) or execute `rnx help` for
command-line documentation.