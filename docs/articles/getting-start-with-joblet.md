# Getting Started with Joblet: A Complete Setup Guide for Ubuntu Server and macOS Client

## Introduction: What is Joblet?

Joblet is a powerful job execution system that brings enterprise-grade resource isolation and security to
Linux environments. Built with a focus on simplicity and security, Joblet leverages Linux namespaces and cgroups v2 to
provide complete process isolation, making it ideal for running untrusted code, CI/CD pipelines, data processing
workloads, and any scenario requiring strict resource control.

### Key Features

- **Two-Stage Security Architecture**: Server Mode (privileged) creates cgroups and namespaces, while Init Mode (
  unprivileged) runs jobs as PID 1 in isolation
- **Real-time Job Streaming**: Stream job output from start to finish with `rnx job log <job-id>`
- **mTLS Communication**: Secure client-server communication using mutual TLS
- **Resource Limits**: Fine-grained control over CPU, memory, and I/O resources
- **Pre-built Runtimes**: Python, Java (OpenJDK), GraalVM environments with instant startup
- **Cross-platform Client**: RNX CLI works on macOS, Windows, and Linux desktops
- **Web Admin UI**: Beautiful React-based dashboard for job management and monitoring

## Part 1: Installing Joblet Server on Ubuntu Linux

Let's start by setting up the Joblet server on an Ubuntu Linux machine. The server component requires Linux due to its
dependency on kernel features like cgroups and namespaces.

### Prerequisites

- Ubuntu 20.04 or later (also supports RHEL/CentOS 8+, Amazon Linux)
- Root or sudo access
- At least 2GB RAM and 20GB disk space
- Port 50051 available for gRPC communication

### Quick Installation (Recommended)

The easiest way to install Joblet on Ubuntu/Debian is using the .deb package:

```bash
# Download and install the latest .deb package
wget $(curl -s https://api.github.com/repos/ehsaniara/joblet/releases/latest | grep "browser_download_url.*_amd64\.deb" | cut -d '"' -f 4)
sudo dpkg -i joblet_*_amd64.deb

# Start the Joblet service
sudo systemctl start joblet
sudo systemctl enable joblet

# Verify the installation
sudo systemctl status joblet
joblet --version
rnx --version
```

That's it! Joblet is now installed and running. The .deb package automatically:

- Installs both joblet and rnx binaries
- Creates all necessary directories
- Sets up systemd service
- Generates mTLS certificates
- Creates default configuration

### Retrieving the Client Configuration

After installation, the RNX client configuration (with embedded certificates) is available at:

```bash
# View the client configuration
sudo cat /opt/joblet/config/rnx-config.yml

# Copy this file to use on client machines (macOS, Windows, etc.)
```

### Alternative: Manual Installation

If you prefer manual installation or are using a non-Debian based system:

```bash
# Download and extract
curl -L https://github.com/ehsaniara/joblet/releases/latest/download/joblet-linux-amd64.tar.gz | tar xz

# Run the installation script
cd rnx-linux-amd64
sudo ./install.sh

# Start the service
sudo systemctl start joblet
sudo systemctl enable joblet
```

### Verify Installation

```bash
# Check service status
sudo systemctl status joblet

# Test with local RNX client
rnx job list
rnx job run echo "Joblet is working!"

# Check logs if needed
sudo journalctl -u joblet -f
```

### Customizing Configuration (Optional)

The default configuration works well for most use cases, but you can customize it if needed:

```bash
# Edit the configuration
sudo nano /opt/joblet/config/joblet.yml

# Restart service after changes
sudo systemctl restart joblet
```

Common customizations:

- Change gRPC port (default: 50051)
- Adjust resource limits
- Configure logging levels
- Set custom certificate paths

## Part 2: Setting Up RNX Client on macOS

Now let's set up the RNX client on your macOS workstation. The client can communicate with the Joblet server to submit
and manage jobs.

### Option 1: Install via Homebrew (Recommended)

```bash
# Add the Joblet tap
brew tap ehsaniara/joblet https://github.com/ehsaniara/joblet

# Install RNX CLI
brew install ehsaniara/joblet/rnx

# Verify installation
rnx --version
```

### Option 2: Manual Installation

```bash
# For Apple Silicon (M1/M2/M3)
curl -L https://github.com/ehsaniara/joblet/releases/latest/download/rnx-darwin-arm64.tar.gz | tar xz

# For Intel Macs
# curl -L https://github.com/ehsaniara/joblet/releases/latest/download/rnx-darwin-amd64.tar.gz | tar xz

# Install the binary
chmod +x rnx-darwin-arm64/bin/rnx
sudo mv rnx-darwin-arm64/bin/rnx /usr/local/bin/

# Create config directory
mkdir -p ~/.rnx
```

### Configure RNX Client

Copy the configuration file from your Ubuntu server to your Mac:

```bash
# From your Mac, copy the config file from the server
scp ubuntu@YOUR_SERVER_IP:/opt/joblet/config/rnx-config.yml ~/.rnx/

# Or manually create ~/.rnx/rnx-config.yml with the contents from the server
```

### Test the Connection

```bash
# List jobs (should connect to server)
rnx job list

# Check server status
rnx monitor status

# Run a simple test job
rnx job run echo "Hello from macOS client!"
```

## Part 3: Running the Admin UI

The Admin UI provides a beautiful web interface for managing jobs, viewing logs, and monitoring your Joblet cluster.
It's now available as a standalone package that connects directly to the Joblet server via gRPC.

### Prerequisites

You'll need Node.js 18+ to run the Admin UI:

```bash
# Check if Node.js is installed
node --version

# If not installed, install via Homebrew
brew install node
```

### Installing and Starting the Admin UI

```bash
# Clone the joblet-admin repository (separate from main Joblet repo)
git clone https://github.com/ehsaniara/joblet-admin
cd joblet-admin

# Install dependencies
npm install

# Start the admin interface
npm run dev

# The UI will be available at http://localhost:3000
```

**Note**: The Admin UI connects directly to the Joblet server via gRPC using the protobuf definitions
from [joblet-proto](https://github.com/ehsaniara/joblet-proto).

### Admin UI Features

The Admin UI provides:

- **Dashboard**: Real-time system metrics (CPU, memory, disk, network)
- **Job Management**: List, filter, and manage all jobs
- **Live Logs**: Stream job output in real-time
- **Workflow Visualization**: View job dependencies and execution flow
- **Volume Management**: Create and manage persistent volumes
- **Network Configuration**: Manage network isolation settings

## Part 4: Creating and Running Jobs

Now let's explore different ways to create and run jobs using Joblet.

### Example 1: Simple Command Execution

```bash
# Basic echo command
rnx job run echo "Hello, Joblet!"

# Run a Python script
rnx job run python3 -c "print('Processing data...')"

# Run with resource limits
rnx job run --max-cpu=50 --max-memory=512 stress --cpu 2 --timeout 10
```

### Example 2: File Upload and Processing

Create a simple Python script locally:

```python
# save as analyze.py
import pandas as pd
import sys

print("Loading data...")
df = pd.read_csv('data.csv')
print(f"Processed {len(df)} rows")
print(df.describe())
```

Create sample data:

```csv
# save as data.csv
name,age,score
Alice,25,95
Bob,30,87
Charlie,35,92
```

Run the analysis job:

```bash
# Upload files and run analysis
rnx job run --upload=analyze.py --upload=data.csv \
  --runtime=python-3.11-ml \
  python3 analyze.py
```

### Example 3: Using Pre-built Runtimes

After installing the runtimes (see "Installing Pre-built Runtimes" section above), you can use them for instant job
execution:

```bash
# Python with ML libraries (NumPy, Pandas, Scikit-learn)
# Note: Make sure you've installed the runtime first with: rnx runtime install python-3.11-ml
rnx job run --runtime=python-3.11-ml python3 -c "
import numpy as np
import pandas as pd
from sklearn.linear_model import LinearRegression
print('ML environment ready!')
print(f'NumPy version: {np.__version__}')
print(f'Pandas version: {pd.__version__}')
"

# Java application
# Note: Install first with: rnx runtime install openjdk-21
rnx job run --runtime=openjdk-21 java -version

# GraalVM for high-performance Java
# Note: Install first with: rnx runtime install graalvmjdk-21
rnx job run --runtime=graalvmjdk-21 java -version
```

The benefit of runtimes: After the initial installation, jobs start in 2-3 seconds with all dependencies ready, versus
minutes for cold starts.

### Example 4: YAML Workflow Definition

Create a workflow file `jobs.yaml`:

```yaml
version: "3.0"

jobs:
  data-prep:
    command: "python3"
    args: [ "-c", "print('Preparing data...'); import time; time.sleep(2)" ]
    runtime: "python-3.11"
    resources:
      max_cpu: 100
      max_memory: 512

  ml-training:
    command: "python3"
    args: [ "-c", "print('Training model...'); import time; time.sleep(3)" ]
    runtime: "python-3.11-ml"
    resources:
      max_cpu: 400
      max_memory: 2048
    requires:
      - data-prep: "COMPLETED"

  evaluation:
    command: "python3"
    args: [ "-c", "print('Evaluating model...')" ]
    runtime: "python-3.11-ml"
    requires:
      - ml-training: "COMPLETED"
```

Run the workflow:

```bash
# Run a specific job from the workflow
rnx job run --workflow=jobs.yaml:data-prep

# Run the entire workflow (all jobs with dependencies)
rnx job run --workflow=jobs.yaml

# Check workflow status
rnx job status --workflow <workflow-id>
```

### Example 5: Long-running Jobs with Monitoring

```bash
# Start a web server
rnx job run --runtime=python-3.11 \
  python3 -m http.server 8000 &

# Get the job ID
JOB_ID=$(rnx job list --json | jq -r '.[-1].uuid')

# Monitor the job
rnx job status $JOB_ID

# Stream logs in real-time
rnx job log $JOB_ID

# Stop the job when done
rnx job stop $JOB_ID
```

### Example 6: Network Isolation

```bash
# No network access
rnx job run --network=none python3 process_local.py

# External network only (no inter-job communication)
rnx job run --network=isolated curl https://api.github.com

# Custom network for job communication
rnx job run --network=backend --runtime=python-3.11 \
  python3 -m http.server 5000 &

# Another job on the same network can access it
rnx job run --network=backend \
  curl http://job_<first-job-id>:5000
```

### Example 7: Persistent Volumes

```bash
# Create a volume
rnx volume create mydata --size=1GB

# Write data to the volume
rnx job run --volume=mydata \
  bash -c "echo 'Persistent data' > /volumes/mydata/data.txt"

# Read data from the volume in another job
rnx job run --volume=mydata \
  cat /volumes/mydata/data.txt
```

## Part 5: Best Practices and Tips

### Security Best Practices

1. **Always use mTLS**: Never disable TLS in production
2. **Rotate certificates regularly**: Update certificates at least annually
3. **Limit network access**: Use `--network=none` or `--network=isolated` when possible
4. **Set resource limits**: Always specify CPU and memory limits for jobs
5. **Use secret environment variables**: For sensitive data, use `--secret-env`

### Performance Optimization

1. **Use pre-built runtimes**: They start in 2-3 seconds vs minutes for cold starts
2. **Reuse volumes**: For frequently accessed data, use persistent volumes
3. **Batch small jobs**: Combine multiple small tasks into single jobs
4. **Monitor resource usage**: Use `rnx monitor` to track system utilization

### Troubleshooting Common Issues

**Connection refused errors:**

```bash
# Check if Joblet service is running
sudo systemctl status joblet

# Verify port is open
sudo netstat -tlnp | grep 50051

# Check firewall rules
sudo ufw status
```

**Certificate errors:**

```bash
# Verify certificate paths in config
cat ~/.rnx/rnx-config.yml

# Test certificate validity
openssl x509 -in ~/.rnx/client.crt -text -noout
```

**Job failures:**

```bash
# Check job logs
rnx job log <job-id>

# View detailed job status
rnx job status <job-id>

# Check server logs
sudo journalctl -u joblet -f
```

## Conclusion

Joblet provides a powerful, secure, and flexible platform for job execution. With its two-stage security
architecture, real-time log streaming, and comprehensive resource management, it's ideal for everything from simple
scripts to complex ML pipelines.

The combination of the Linux-based Joblet server and cross-platform RNX client makes it easy to manage jobs from any
workstation while leveraging the power of Linux's isolation features. The web-based Admin UI adds a user-friendly layer
for monitoring and management.

### Next Steps

- Explore [workflow templates](https://github.com/ehsaniara/joblet/tree/main/examples/workflows) for complex pipelines
- Set up [scheduled jobs](https://github.com/ehsaniara/joblet/blob/main/docs/JOB_EXECUTION.md#scheduled-execution) for
  automation
- Configure [RBAC](https://github.com/ehsaniara/joblet/blob/main/docs/SECURITY.md) for multi-user environments

For more information, visit the [Joblet GitHub repository](https://github.com/ehsaniara/joblet) and check out the
comprehensive [documentation](https://github.com/ehsaniara/joblet/tree/main/docs).

Happy job orchestration with Joblet! 🚀