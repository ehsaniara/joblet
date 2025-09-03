# Joblet / RNX - Secure Linux Process Execution Platform

[![Tests](https://github.com/ehsaniara/joblet/actions/workflows/ci.yml/badge.svg)](https://github.com/ehsaniara/joblet/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/ehsaniara/joblet)](https://goreportcard.com/report/github.com/ehsaniara/joblet)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

<img align="right" width="159px" src="docs/joblet-thumbnail.png">

**Joblet** provides **secure, isolated job execution** on Linux systems. Run commands remotely with complete process
isolation, resource limits, real-time monitoring, and network isolation - protecting your host system from malicious or
resource-hungry processes.

> **🤖 Agentic AI Foundation:** Purpose-built for AI agent workloads with sandboxed code execution, resource-controlled
> tool usage, and secure multi-tenant isolation - enabling AI agents to safely execute arbitrary code without compromising
> host security.

## 🎯 Key Features

- **Process Isolation**: Jobs run in separate PID namespaces with limited filesystem access
- **Resource Control**: CPU, memory, I/O bandwidth limits with specific core binding
- **Network Isolation**: Custom networks with traffic segmentation
- **Volume Management**: Persistent storage with filesystem and memory-based volumes
- **Job Scheduling**: Future execution with flexible time specifications
- **Real-time Monitoring**: Live logs, system metrics, and job status tracking
- **mTLS Security**: Certificate-based authentication with role-based access control
- **Pre-built Runtimes**: Python ML, Java, Node.js environments with instant startup (2-3 seconds vs 5-45 minutes)
- **Web Admin UI**: React-based interface for comprehensive system management (macOS)

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
rnx list                           # List jobs
rnx run echo "Hello from Joblet!" # Run a simple job
rnx monitor status                 # Check server metrics

# Web admin UI (macOS with --with-admin):
rnx admin                          # Launch at http://localhost:5173
```

## 🚀 Basic Usage

### YAML Workflows (Simplified Execution)

```bash
# Run pre-configured jobs with workflows
rnx run --workflow=jobs.yaml:ml-analysis      # Python ML analysis
rnx run --workflow=jobs.yaml:hello-joblet     # Java application
rnx run --workflow=ml-pipeline.yaml           # Full ML workflow

# Check status
rnx status <job-uuid>
rnx status --workflow <workflow-uuid>
```

### Traditional Commands

```bash
# Run commands with isolation
rnx run echo "Hello World"
rnx run --max-cpu=50 --max-memory=512 python3 script.py

# Pre-built runtimes for instant startup
rnx run --runtime=python-3.11-ml python analysis.py    # 2-3 seconds
rnx run --runtime=openjdk-21 java App.java
rnx run --runtime=nodejs-18 node server.js

# Upload files and run
rnx run --upload=data.csv --upload=script.py python3 script.py

# Environment variables
rnx run --env=NODE_ENV=production node app.js
rnx run --secret-env=API_KEY=secret python app.py

# Network isolation
rnx run --network=none secure_task.sh           # No network
rnx run --network=isolated wget https://api.com # External only
rnx run --network=bridge api_server.py          # Inter-job communication

# Persistent storage
rnx volume create mydata --size=1GB
rnx run --volume=mydata python3 process_data.py

# Job scheduling
rnx run --schedule="1hour" backup.sh
rnx run --schedule="2025-12-25T00:00:00" maintenance.py
```

## 🎨 Web Admin UI Features

Available on macOS via Homebrew with `--with-admin`:

- **System Monitoring**: Real-time CPU, memory, disk, network metrics
- **Job Management**: List, filter, start/stop jobs with live logs
- **Workflow Visualization**: Dependency graphs and execution timelines
- **Administration**: Volume and network management, runtime configuration

## 📊 System Requirements

### Joblet Server (Linux)

- Linux kernel 4.6+ (Ubuntu 18.04+, CentOS 8+)
- Multi-core CPU, 1GB+ RAM, 5GB+ disk
- Port 50051 (gRPC), root access for namespaces
- cgroups v2, iptables, iproute2, bridge-utils

### RNX Client

- Linux, macOS 10.15+, or Windows 10+
- 50MB+ RAM, network access to port 50051

## 📚 Documentation

- **[Quick Start Guide](docs/QUICKSTART.md)** - Get running in 5 minutes
- **[Installation Guide](docs/INSTALLATION.md)** - Detailed setup for all platforms
- **[Job Execution Guide](docs/JOB_EXECUTION.md)** - Complete job management
- **[RNX CLI Reference](docs/RNX_CLI_REFERENCE.md)** - All commands and options
- **[Security Guide](docs/SECURITY.md)** - mTLS, RBAC, best practices
- **[Examples](examples/)** - Working code samples

## 🤝 Contributing

1. Fork the repository
2. Create feature branch: `git checkout -b feature/amazing-feature`
3. Commit changes: `git commit -m 'Add amazing feature'`
4. Push to branch: `git push origin feature/amazing-feature`
5. Open Pull Request

## 📄 License

MIT License - see [LICENSE](LICENSE) file for details.

---

**Quick Links:
** [Releases](https://github.com/ehsaniara/joblet/releases) | [Examples](examples/) | [Issues](https://github.com/ehsaniara/joblet/issues)