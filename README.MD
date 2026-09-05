# Joblet / RNX - Secure Linux Process Execution Platform

[![Tests](https://github.com/ehsaniara/joblet/actions/workflows/ci.yml/badge.svg)](https://github.com/ehsaniara/joblet/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/ehsaniara/joblet)](https://goreportcard.com/report/github.com/ehsaniara/joblet)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

<img align="right" width="159px" src="docs/joblet-thumbnail.png">

**Joblet** is a secure Linux job execution platform that simplifies running isolated processes with comprehensive
resource management. It handles process isolation, resource limits, GPU acceleration, and monitoring automatically,
letting you focus on your applications rather than infrastructure complexity.

> Complete Documentation: [/docs/README.md](docs/README.md)

> Provides sandboxed code execution with GPU support, resource controls, and complete process
> isolation for AI workloads.

**How it works:** Joblet runs each job as a native Linux process inside its own kernel-enforced sandbox
(namespaces + cgroups v2), with optional eBPF-based runtime monitoring, all driven through a single
mTLS-secured gRPC API. There are no container images to build and no registry to manage.

**Common use cases:** running untrusted or LLM-generated code in a sandbox, giving AI agents a secure
execution backend (via MCP), isolating tests and builds in CI/CD, and running GPU-accelerated ML/AI
workloads.

## Key Capabilities

- **🔒 Isolation**: Run in secure sandboxes, separate namespaces, preventing interference between
  workloads
- **🔬 eBPF Observability**: Kernel-level, per-job behavioral telemetry - process execution, network
  connections, file access, and memory-mapping events - for security auditing and runtime monitoring of
  untrusted code
- **⚡ GPU Acceleration**: Native NVIDIA GPU support with automatic CUDA env. setup for AI/ML workloads
- **🎛️ Resource Management**: CPU, memory, I/O, and GPU resource limits with fine-grained control
- **🌐 Network Isolation**: Networking options from full isolation to custom networks for secure
  communication
- **💾 Storage Solutions**: Persistent volumes for data retention and temporary storage with automatic cleanup
- **⏰ Job Scheduling**: Execute jobs immediately, schedule for specific times, or set delayed execution
- **⏱️ Execution Timeouts**: Per-job wall-clock limits that terminate runaway or hung jobs automatically
- **🛡️ Enterprise Security**: mTLS encryption with certificate-based authentication and role-based
  access control
- **🐍 Runtime Environments**: Build Python ML, Java, and custom runtime environments from declarative YAML
  specifications
- **🤖 AI-Native Integration**: Drive Joblet from AI assistants and agent frameworks through the
  [Joblet MCP Server](https://github.com/ehsaniara/joblet-mcp-server)
  and [Python SDK](https://github.com/ehsaniara/joblet-sdk-python/)

## Getting Started

> **☁️ Cloud Deployment**: Joblet can be deployed directly on AWS EC2 with automated installation DynamoDB and
> CloudWatch
> integration. See the [AWS Deployment Guide](docs/AWS_DEPLOYMENT.md) for one-click deployment with user data scripts.

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

#### Option 2: RPM Installation (RHEL/CentOS/Fedora)

```bash
# Download and install the latest release
wget $(curl -s https://api.github.com/repos/ehsaniara/joblet/releases/latest | grep "browser_download_url.*\.rpm" | cut -d '"' -f 4)
sudo rpm -i joblet-*.rpm

# Start the service
sudo systemctl start joblet
sudo systemctl enable joblet
```

### B. RNX Client Installation (Cross-Platform)

The `rnx` client is released separately from
[joblet-rnx](https://github.com/ehsaniara/joblet-rnx/releases/latest), with
binaries for Linux, macOS, and Windows (amd64/arm64). The server package
installer installs the latest rnx on the server host automatically; on other
machines use one of the options below (see the joblet-rnx README for all).

#### macOS and Linux (Homebrew)

```bash
brew tap ehsaniara/rnx https://github.com/ehsaniara/joblet-rnx
brew install rnx

# Copy your role's configuration from the server (files are root-only)
mkdir -p ~/.rnx
ssh user@joblet-server "sudo cat /opt/joblet/config/rnx-config-<role>.yml" > ~/.rnx/rnx-config.yml
```

#### Windows

```powershell
# Download rnx-<version>-windows-amd64.zip from
# https://github.com/ehsaniara/joblet-rnx/releases/latest and extract it
Expand-Archive rnx-*-windows-amd64.zip -DestinationPath .
New-Item -ItemType Directory -Force -Path "$env:USERPROFILE\bin"
Move-Item rnx-windows-amd64.exe "$env:USERPROFILE\bin\rnx.exe"
[Environment]::SetEnvironmentVariable("Path", $env:Path + ";$env:USERPROFILE\bin", [EnvironmentVariableTarget]::User)

# Copy configuration from Joblet server to %USERPROFILE%\.rnx\rnx-config.yml
```

### C. Verify Installation

```bash
# Test the connection and run your first job
rnx job list                           # List current jobs
rnx job run echo "Hello from Joblet!" # Execute a simple test job
rnx monitor status                     # Check server health

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

### Command Line Interface

```bash
# Basic job execution with resource isolation
rnx job run echo "Hello World"
rnx job run --max-cpu=50 --max-memory=512 python3 script.py

# Build and use runtime environments
rnx runtime build ./examples/python-3.11-ml/runtime.yaml   # Build Python ML runtime
rnx runtime build ./examples/java-21/runtime.yaml          # Build Java 21 runtime
rnx job run --runtime=python-3.11-ml python analysis.py    # Use Python ML runtime
rnx job run --runtime=openjdk-21 java App.java             # Use Java runtime

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

# Execution timeout (terminates job if exceeded)
rnx job run --timeout=5m long_running_task.sh
rnx job run --timeout=30s --max-memory=256 quick_check.sh
```

## 💡 Use Cases

### 🤖 Sandboxed code execution for AI agents

Run untrusted or LLM-generated code inside a kernel-enforced sandbox instead of on the host. Each job
gets its own PID, mount, network, IPC, and UTS namespaces with cgroup resource caps, so a misbehaving
script cannot touch the host or other jobs. eBPF telemetry records exactly what the code did - every
process it spawned, every network connection it opened, and every file it touched - giving you an audit
trail for code you did not write.

```bash
# Run model-generated code with no network, tight limits, and a hard timeout
rnx job run --network=none --max-cpu=50 --max-memory=512 --timeout=30s \
        --runtime=python-3.11-ml --upload=generated.py python3 generated.py
```

### 🧠 Agentic AI execution backend (MCP)

The [Joblet MCP Server](https://github.com/ehsaniara/joblet-mcp-server) exposes Joblet to AI assistants
and agent frameworks over the Model Context Protocol, so an agent can launch jobs, stream logs, manage
volumes and networks, and read GPU/metrics - all against isolated sandboxes rather than your shell.
Combined with the [Python SDK](https://github.com/ehsaniara/joblet-sdk-python/), this makes Joblet a
secure tool-use / code-execution layer for multi-agent systems.

```python
from joblet import JobletClient

with JobletClient(host="your-joblet-server") as client:
    job = client.jobs.run_job(
        command="python3", args=["agent_task.py"],
        runtime="python-3.11-ml", network="none", max_memory=1024,
    )
    print(f"Agent job started in sandbox: {job['job_uuid']}")
```

### 🔁 CI/CD test & build isolation

Replace heavyweight container builds in your pipeline with instant, reproducible runtime environments.
Each test or build runs isolated, with resource limits and timeouts, and no image registry to maintain.

```bash
# Isolated test run with a pre-built runtime
rnx job run --runtime=python-3.11-ml --upload=. pytest tests/
rnx job run --runtime=openjdk-21 --upload=pom.xml --upload=src/ mvn clean install

# Network-isolated integration testing across jobs
rnx network create test-env --cidr=10.10.0.0/24
rnx job run --network=test-env --runtime=openjdk-21 ./service-a
rnx job run --network=test-env --runtime=python-3.11-ml ./service-b
```

### ⚡ GPU-accelerated ML/AI workloads

Native NVIDIA GPU allocation with automatic CUDA setup and per-job GPU memory limits - no container GPU
plumbing required. See the [GPU Support Guide](docs/GPU_SUPPORT.md).

```bash
rnx job run --gpu=1 --gpu-memory=8GB --runtime=python-3.11-ml python train_model.py
```

## 🆚 Why Joblet instead of containers?

- **No images, no registry**: Jobs run as native Linux processes - skip `Dockerfile` builds, image
  versioning, and registry management entirely.
- **Lower overhead**: Direct process execution avoids container-runtime layers while keeping
  kernel-enforced isolation (namespaces + cgroups v2).
- **Built-in runtime visibility**: eBPF gives per-job syscall-level telemetry out of the box, instead of
  bolting on a separate runtime-security agent.
- **Secure by default**: mTLS + RBAC on the API, default-deny networking, and resource quotas to prevent
  noisy-neighbor and denial-of-service conditions.
- **Fast iteration**: Pre-built YAML runtimes start instantly - no rebuild cycle between runs.

## 🎨 Web Administration Interface

The Joblet Admin UI is available as a standalone package
at [joblet-admin repository](https://github.com/ehsaniara/joblet-admin):

- **System Monitoring**: Real-time CPU, memory, disk, and network metrics with visual dashboards
- **Job Management**: Comprehensive job listing, filtering, control, and log streaming capabilities
- **Workflow Visualization**: Interactive dependency graphs and execution timelines
- **System Administration**: Intuitive volume, network, and runtime environment management
- **Direct gRPC**: Connects directly to Joblet server via gRPC

To use the admin interface:

```bash
# Clone the joblet-admin repository
git clone https://github.com/ehsaniara/joblet-admin
cd joblet-admin

# Install and run
npm install
npm run dev

# Access at http://localhost:3000
```

**Learn more**: See the [Admin UI Documentation](docs/ADMIN_UI.md)

## System Requirements

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

## Documentation

- **[Quick Start Guide](docs/QUICKSTART.md)** - Get up and running quickly with step-by-step instructions
- **[Installation Guide](docs/INSTALLATION.md)** - Comprehensive installation procedures for all platforms
- **[AWS Deployment Guide](docs/AWS_DEPLOYMENT.md)** - Deploy Joblet on AWS EC2 with CloudWatch integration
- **[GPU Support Guide](docs/GPU_SUPPORT.md)** - Complete guide to NVIDIA GPU acceleration and CUDA integration
- **[Runtime System Guide](docs/RUNTIME_SYSTEM.md)** - Build and manage runtime environments from YAML specifications
- **[Job Execution Guide](docs/JOB_EXECUTION.md)** - Detailed job management and execution patterns
- **[RNX CLI Reference](docs/RNX_CLI_REFERENCE.md)** - Complete command reference with examples
- **[Security Guide](docs/SECURITY.md)** - Security configuration and best practices
- **[Monitoring Guide](docs/MONITORING.md)** - eBPF telemetry, metrics, and observability
- **[Working Examples](examples/)** - Production-ready runtime configurations and job examples
- **[Compatibility](COMPATIBILITY.md)** - Joblet ↔ `joblet-proto` ↔ client-tool version matrix

## Related Projects

- **[Joblet Admin UI](https://github.com/ehsaniara/joblet-admin)**
- **[Joblet Proto](https://github.com/ehsaniara/joblet-proto)**
- **[Joblet Python SDK](https://github.com/ehsaniara/joblet-sdk-python/)**
- **[Joblet MCP Server](https://github.com/ehsaniara/joblet-mcp-server)**

**Developing a custom client?** Start with the [protocol definitions](https://github.com/ehsaniara/joblet-proto) to
generate language-specific bindings.

## License

MIT License - see [LICENSE](LICENSE) file for details.

