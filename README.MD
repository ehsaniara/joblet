# Joblet / RNX - Secure Linux Process Execution Platform

[![Tests](https://github.com/ehsaniara/joblet/actions/workflows/ci.yml/badge.svg)](https://github.com/ehsaniara/joblet/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/ehsaniara/joblet)](https://goreportcard.com/report/github.com/ehsaniara/joblet)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

<img align="right" width="159px" src="docs/joblet-thumbnail.png">

**Joblet** is a secure Linux job execution platform that simplifies running isolated processes with comprehensive
resource management. It handles process isolation, resource limits, GPU acceleration, and monitoring automatically,
letting you focus on your applications rather than infrastructure complexity.

> **📚 Complete Documentation: [/docs/README.md](docs/README.md)**

> **🤖 AI Agent Ready:** Provides sandboxed code execution with GPU support, resource controls, and complete process
> isolation for AI workloads.

## 🎯 Key Capabilities

- **🔒 Process Isolation**: Jobs run in secure sandboxes with separate namespaces, preventing interference between
  workloads
- **⚡ GPU Acceleration**: Native NVIDIA GPU support with automatic CUDA environment setup for AI/ML workloads
- **🎛️ Resource Management**: Comprehensive CPU, memory, I/O, and GPU resource limits with fine-grained control
- **🌐 Network Isolation**: Flexible networking options from complete isolation to custom networks for secure
  communication
- **💾 Storage Solutions**: Persistent volumes for data retention and temporary storage with automatic cleanup
- **⏰ Job Scheduling**: Execute jobs immediately, schedule for specific times, or set delayed execution with simple
  syntax
- **🛡️ Enterprise Security**: Production-grade mTLS encryption with certificate-based authentication and role-based
  access control
- **🐍 Runtime Environments**: Pre-configured Python ML, Java, and GraalVM environments eliminate container image
  management
- **🎨 Management Interface**: Modern React-based web UI for comprehensive system administration and monitoring

## 📦 Getting Started

### A. Joblet Server Installation (Linux)

#### Option 1: Package Installation (Ubuntu/Debian)

```bash
# Download and install the latest release
wget $(curl -s https://api.github.com/repos/ehsaniara/joblet/releases/latest | grep "browser_download_url.*_amd64\.deb" | cut -d '"' -f 4)
sudo dpkg -i joblet_*_amd64.deb

# Start the service
sudo systemctl start joblet
sudo systemctl enable joblet
```

#### Option 2: Manual Installation (Any Linux Distribution)

```bash
# Download and extract the release
curl -L -o rnx-linux-amd64.tar.gz $(curl -s https://api.github.com/repos/ehsaniara/joblet/releases/latest | grep "browser_download_url.*linux-amd64.tar.gz" | cut -d '"' -f 4)
tar -xzf rnx-linux-amd64.tar.gz
cd rnx-linux-amd64

# Run the installation script
sudo ./install.sh
sudo systemctl start joblet
```

### B. RNX Client Installation (Cross-Platform)

#### macOS (Homebrew)

```bash
# Add the Joblet tap
brew tap ehsaniara/joblet https://github.com/ehsaniara/joblet
brew install rnx              # Installs RNX CLI + Admin UI (default, requires Node.js 18+)
brew install rnx --cli-only   # CLI only (without Admin UI and Node.js)

# Copy configuration from server
scp user@joblet-server:/opt/joblet/config/rnx-config.yml ~/.rnx/
```

**Note**: Admin UI requires Node.js 18+. If not installed or version is too old, Homebrew will automatically install/upgrade it.

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

### C. Verify Installation

```bash
# Test the connection and run your first job
rnx job list                           # List current jobs
rnx job run echo "Hello from Joblet!" # Execute a simple test job
rnx monitor status                     # Check server health

# Launch Web UI (if installed with --with-admin):
rnx admin                              # Opens dashboard at http://localhost:5173
```

### D. Python SDK Usage (Alternative to CLI)

```python
# Install: pip install joblet-sdk
from joblet import JobletClient

with JobletClient(host="your-joblet-server") as client:
    # Run a job programmatically
    job = client.jobs.run_job(
        command="echo",
        args=["Hello from Python!"],
        name="python-job"
    )
    print(f"Job started: {job['job_uuid']}")
```

## 🚀 Usage Examples

### YAML Workflows (Recommended for Complex Jobs)

```bash
# Execute pre-configured workflows
rnx job run --workflow=jobs.yaml:ml-analysis      # Python ML analysis
rnx job run --workflow=jobs.yaml:hello-joblet     # Java application
rnx job run --workflow=ml-pipeline.yaml           # ML pipeline with GPU support

# Monitor execution status
rnx job status <job-uuid>
rnx job status --workflow <workflow-uuid>
```

### Command Line Interface

```bash
# Basic job execution with resource isolation
rnx job run echo "Hello World"
rnx job run --max-cpu=50 --max-memory=512 python3 script.py

# Pre-built runtime environments
rnx job run --runtime=python-3.11-ml python analysis.py    # ML-ready Python environment
rnx job run --runtime=openjdk-21 java App.java
rnx job run --runtime=graalvmjdk-21 java GraalApp.java

# GPU acceleration for compute-intensive workloads
rnx job run --gpu=1 --gpu-memory=8GB python train_model.py
rnx job run --gpu=2 --runtime=python-3.11-ml python distributed_training.py

# File uploads and environment configuration
rnx job run --upload=data.csv --upload=script.py python3 script.py

# Environment variables (visible and secure)
rnx job run --env=NODE_ENV=production node app.js
rnx job run --secret-env=API_KEY=secret python app.py       # Hidden from logs

# Network isolation modes
rnx job run --network=none secure_task.sh                   # No network access
rnx job run --network=isolated wget https://api.com         # External access only
rnx job run --network=bridge api_server.py                  # Inter-job communication

# Persistent storage management
rnx volume create mydata --size=1GB
rnx job run --volume=mydata python3 process_data.py

# Job scheduling options
rnx job run --schedule="1hour" backup.sh
rnx job run --schedule="2025-12-25T00:00:00" maintenance.py
```

## 🎨 Web Administration Interface

Available on macOS via Homebrew with `--with-admin` option:

- **📊 System Monitoring**: Real-time CPU, memory, disk, and network metrics with visual dashboards
- **🎮 Job Management**: Comprehensive job listing, filtering, control, and log streaming capabilities
- **🔗 Workflow Visualization**: Interactive dependency graphs and execution timelines
- **⚙️ System Administration**: Intuitive volume, network, and runtime environment management

## 📊 System Requirements

### Joblet Server (Linux)

- **Operating System**: Linux kernel 4.6+ (Ubuntu 18.04+, CentOS 8+, or equivalent distributions)
- **Hardware**: Multi-core CPU, 1GB+ RAM, 5GB+ disk space (scales with workload)
- **Network**: Port 50051 accessible for gRPC communication
- **Privileges**: Root access required for namespace and cgroup management
- **Dependencies**: cgroups v2, iptables, iproute2, bridge-utils (standard on most distributions)
- **GPU Support**: NVIDIA drivers (optional, required only for GPU workloads)

### RNX Client (Cross-Platform)

- **Operating Systems**: Linux, macOS 10.15+, Windows 10+
- **Resources**: Minimal - 50MB+ RAM, network connectivity to server port 50051
- **Distribution**: Single binary executable with no additional dependencies

## 📚 Documentation

- **[Quick Start Guide](docs/QUICKSTART.md)** - Get up and running quickly with step-by-step instructions
- **[Installation Guide](docs/INSTALLATION.md)** - Comprehensive installation procedures for all platforms
- **[GPU Support Guide](docs/GPU_SUPPORT.md)** - Complete guide to NVIDIA GPU acceleration and CUDA integration
- **[Job Execution Guide](docs/JOB_EXECUTION.md)** - Detailed job management and execution patterns
- **[RNX CLI Reference](docs/RNX_CLI_REFERENCE.md)** - Complete command reference with examples
- **[Security Guide](docs/SECURITY.md)** - Security configuration and best practices
- **[Working Examples](examples/)** - Production-ready workflow templates and configurations

## 🔗 Related Projects

- **[Joblet Proto](https://github.com/ehsaniara/joblet-proto)**
- **[Joblet Python SDK](https://github.com/ehsaniara/joblet-sdk-python/)**
- **[Joblet MCP Server](https://github.com/ehsaniara/joblet-mcp-server)**  

**Developing a custom client?** Start with the [protocol definitions](https://github.com/ehsaniara/joblet-proto) to
generate language-specific bindings.

## 📄 License

MIT License - see [LICENSE](LICENSE) file for details.

