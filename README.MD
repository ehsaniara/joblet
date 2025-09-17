# Joblet / RNX - Secure Linux Process Execution Platform

[![Tests](https://github.com/ehsaniara/joblet/actions/workflows/ci.yml/badge.svg)](https://github.com/ehsaniara/joblet/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/ehsaniara/joblet)](https://goreportcard.com/report/github.com/ehsaniara/joblet)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

<img align="right" width="159px" src="docs/joblet-thumbnail.png">

**Joblet** executes jobs on Linux systems with process isolation, resource limits, monitoring, and network isolation.

> **📚 Documentation: [/docs/README.md](docs/README.md)**

> **🤖 AI Agent Support:** Sandboxed code execution with resource controls and process isolation.

## 🎯 Key Features

- **Process Isolation**: Separate PID namespaces with limited filesystem access
- **Resource Control**: CPU, memory, I/O bandwidth limits with core binding
- **Network Isolation**: Custom networks with traffic segmentation
- **Volume Management**: Persistent storage (filesystem and memory-based)
- **Job Scheduling**: Delayed execution with time specifications
- **Monitoring**: Logs, metrics, and status tracking
- **mTLS Security**: Certificate-based authentication with RBAC
- **Pre-built Runtimes**: Python, Java, GraalVM environments
- **Web Admin UI**: React interface (macOS)

## 📦 Quick Start

### A. Joblet Server Installation (Linux Only)

#### Option 1: Package Installation (Ubuntu/Debian)

```bash
# Download and install the .deb package
wget $(curl -s https://api.github.com/repos/ehsaniara/joblet/releases/latest | grep "browser_download_url.*_amd64\.deb" | cut -d '"' -f 4)
sudo dpkg -i joblet_*_amd64.deb

# Start the Joblet service
sudo systemctl start joblet
sudo systemctl enable joblet
```

#### Option 2: Manual Installation (Any Linux)

```bash
# Download the Linux release
curl -L -o rnx-linux-amd64.tar.gz $(curl -s https://api.github.com/repos/ehsaniara/joblet/releases/latest | grep "browser_download_url.*linux-amd64.tar.gz" | cut -d '"' -f 4)
tar -xzf rnx-linux-amd64.tar.gz
cd rnx-linux-amd64

# Run the installation script
sudo ./install.sh
sudo systemctl start joblet
```

### B. RNX Client Installation

#### macOS (Homebrew)

```bash
brew tap ehsaniara/joblet https://github.com/ehsaniara/joblet
brew install rnx                                    # Interactive setup
brew install ehsaniara/joblet/rnx --with-admin     # CLI + Web UI
brew install ehsaniara/joblet/rnx --without-admin  # CLI only

# Copy configuration from your Joblet server
scp user@joblet-server:/opt/joblet/config/rnx-config.yml ~/.rnx/
```

#### Windows

```powershell
# Download the Windows binary
Invoke-WebRequest -Uri "https://github.com/ehsaniara/joblet/releases/latest/download/rnx-windows-amd64.exe" -OutFile "rnx.exe"

# Add to PATH
New-Item -ItemType Directory -Force -Path "$env:USERPROFILE\bin"
Move-Item rnx.exe "$env:USERPROFILE\bin\"
[Environment]::SetEnvironmentVariable("Path", $env:Path + ";$env:USERPROFILE\bin", [EnvironmentVariableTarget]::User)

# Copy configuration from Joblet server to %USERPROFILE%\.rnx\rnx-config.yml
```

### C. Quick Test

```bash
# From any client machine with RNX installed:
rnx job list                           # List jobs
rnx job run echo "Hello from Joblet!" # Run a simple job
rnx monitor status                 # Check server metrics

# Web admin UI (macOS with --with-admin):
rnx admin                          # Launch at http://localhost:5173
```

## 🚀 Basic Usage

### YAML Workflows (Simplified Execution)

```bash
# Run pre-configured jobs with workflows
rnx job run --workflow=jobs.yaml:ml-analysis      # Python ML analysis
rnx job run --workflow=jobs.yaml:hello-joblet     # Java application
rnx job run --workflow=ml-pipeline.yaml           # Full ML workflow

# Check status
rnx job status <job-uuid>
rnx job status --workflow <workflow-uuid>
```

### Traditional Commands

```bash
# Run commands with isolation
rnx job run echo "Hello World"
rnx job run --max-cpu=50 --max-memory=512 python3 script.py

# Pre-built runtimes
rnx job run --runtime=python-3.11-ml python analysis.py    # 2-3 seconds
rnx job run --runtime=openjdk-21 java App.java
rnx job run --runtime=graalvmjdk-21 java GraalApp.java

# Upload files and run
rnx job run --upload=data.csv --upload=script.py python3 script.py

# Environment variables
rnx job run --env=NODE_ENV=production node app.js
rnx job run --secret-env=API_KEY=secret python app.py

# Network isolation
rnx job run --network=none secure_task.sh           # No network
rnx job run --network=isolated wget https://api.com # External only
rnx job run --network=bridge api_server.py          # Inter-job communication

# Persistent storage
rnx volume create mydata --size=1GB
rnx job run --volume=mydata python3 process_data.py

# Job scheduling
rnx job run --schedule="1hour" backup.sh
rnx job run --schedule="2025-12-25T00:00:00" maintenance.py
```

## 🎨 Web Admin UI

Available on macOS via Homebrew with `--with-admin`:

- **System Monitoring**: CPU, memory, disk, network metrics
- **Job Management**: List, filter, start/stop jobs with logs
- **Workflow Visualization**: Dependency graphs and timelines
- **Administration**: Volume, network, and runtime configuration

## 📊 System Requirements

### Joblet Server (Linux Only)

- Linux kernel 4.6+ (Ubuntu 18.04+, CentOS 8+)
- Multi-core CPU, 1GB+ RAM, 5GB+ disk
- Port 50051 (gRPC), root access for namespaces
- cgroups v2, iptables, iproute2, bridge-utils

### RNX Client (Cross-Platform)

- Linux, macOS 10.15+, or Windows 10+
- 50MB+ RAM, network access to port 50051

## 📚 Documentation

- **[Quick Start Guide](docs/QUICKSTART.md)**
- **[Installation Guide](docs/INSTALLATION.md)**
- **[Job Execution Guide](docs/JOB_EXECUTION.md)**
- **[RNX CLI Reference](docs/RNX_CLI_REFERENCE.md)**
- **[Security Guide](docs/SECURITY.md)**
- **[Examples](examples/)**

## 📄 License

MIT License - see [LICENSE](LICENSE) file for details.

