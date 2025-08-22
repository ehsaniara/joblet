# Joblet / RNX - Secure Linux Process Execution Platform

[![Tests](https://github.com/ehsaniara/joblet/actions/workflows/ci.yml/badge.svg)](https://github.com/ehsaniara/joblet/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/ehsaniara/joblet)](https://goreportcard.com/report/github.com/ehsaniara/joblet)
[![Go Version](https://img.shields.io/github/go-mod/go-version/ehsaniara/joblet)](https://golang.org/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Release](https://img.shields.io/github/release/ehsaniara/joblet.svg)](https://github.com/ehsaniara/joblet/releases/latest)

<img align="right" width="159px" src="docs/joblet-thumbnail.png">

**Joblet** provides **secure, isolated job execution** on Linux systems. Run commands remotely with complete process
isolation, resource limits, real-time monitoring, network isolation, persistent volume storage, and scheduled
execution - protecting your host system from malicious or resource-hungry processes.

> **🤖 Agentic AI Foundation:** Joblet is purpose-built for AI agent workloads with **sandboxed code execution**, *
*resource-controlled tool usage**, **real-time monitoring**, and **secure multi-tenant isolation** - enabling AI agents
> to safely execute arbitrary code, run system commands, process data, and interact with external tools without
> compromising host security

## 🎯 Key Features

- **Process Isolation**: Jobs run in separate PID namespaces with limited filesystem access
- **Resource Control**: CPU, memory, I/O bandwidth limits with specific core binding
- **Network Isolation**: Custom networks with traffic segmentation (none/bridge/isolated/custom)
- **Volume Management**: Persistent storage with filesystem and memory-based volumes
- **Environment Variables**: Secure configuration with regular and secret variable support
- **Job Scheduling**: Future execution with flexible time specifications
- **Real-time Monitoring**: Live logs, system metrics, and job status tracking
- **Async Log Persistence**: Rate-decoupled log system optimized for HPC workloads with 5M+ writes/second
- **mTLS Security**: Certificate-based authentication with role-based access control
- **Cross-platform CLI**: Manage jobs from Linux, macOS, or Windows
- **Web Admin UI**: React-based web interface with comprehensive system monitoring, job management, and workflow visualization (macOS)

## 🔧 How It Works

Joblet consists of three components:

1. **Joblet Daemon** - Runs on Linux servers, executes jobs in isolated environments
2. **RNX CLI** - Connects from anywhere (Linux/macOS/Windows) to manage jobs
3. **Web Admin UI** - React-based web interface with comprehensive system monitoring, job management, workflow visualization, and real-time metrics (macOS via Homebrew)

### Security & Isolation Features

- **Process Isolation**: Jobs run in separate PID namespaces
- **Filesystem Isolation**: Chroot environments with limited host access
- **Network Isolation**: Four network modes with custom network support
- **Volume Management**: Persistent storage with filesystem and memory-based volumes
- **Resource Limits**: CPU, memory, I/O bandwidth, and CPU core controls
- **Disk Quotas**: Default 1MB limit for jobs without volumes, enforced quotas on volumes
- **CPU Core Binding**: Limit jobs to specific CPU cores for performance isolation
- **Network Security**: mTLS encryption with certificate-based authentication
- **Role-Based Access**: Admin (full control) and Viewer (read-only) roles
- **Job Scheduling**: Future execution with priority queue management

## 🧭 Comparison Table: Joblet vs Competitors

| Feature / Tool            | **Joblet** (v2.11.0)                                      | **Apache DolphinScheduler**                 | **Dkron**                              | **JobRunr**                     | **gVisor** / **Kata**                     |
|---------------------------|-----------------------------------------------------------|---------------------------------------------|----------------------------------------|---------------------------------|-------------------------------------------|
| **Type**                  | Linux-native job execution platform                       | Distributed job orchestration engine        | Distributed cron job scheduler         | In-memory job scheduler for JVM | OCI container runtime sandboxing          |
| **Isolation Mechanism**   | Linux **namespaces** + **cgroups v2**                     | None (relies on system agents)              | None (delegates to OS processes)       | JVM sandboxing                  | Full syscall interception / VM isolation  |
| **Pre-built Runtimes**    | ✅ **Python+ML, Java 17/21, Node.js 18** (instant startup) | ❌ Manual setup required                     | ❌ Manual setup required                | ❌ JVM only                      | ❌ Container images only                   |
| **Startup Performance**   | ✅ **2-3 seconds** (vs 5-45min traditional)                | 30-300+ seconds (package installation)      | Varies (depends on job complexity)     | Fast (in-memory only)           | 5-30 seconds (container pull/start)       |
| **Execution Mode**        | Binary CLI + gRPC API                                     | Web UI + REST API                           | Agent-based CLI + API                  | Java code only (embedded)       | Containerized / OCI only                  |
| **Filesystem Isolation**  | ✅ via chroot + mount namespace                            | ❌                                           | ❌                                      | ❌                               | ✅ full container FS isolation             |
| **Network Isolation**     | ✅ Custom networks + isolation modes                       | ❌                                           | ❌                                      | ❌                               | ✅ (via container)                         |
| **Resource Limits**       | ✅ CPU, memory, I/O, pids via cgroups                      | ❌                                           | ❌                                      | ❌                               | ✅ via kernel interface                    |
| **Programming Interface** | gRPC + binary CLI + runtime management                    | REST API                                    | HTTP API + CLI                         | Java annotations / Spring       | OCI runtime interface only                |
| **Real-time Logging**     | ✅ Built-in SSE log stream + file                          | Partial (log files in worker nodes)         | No streaming, logs stored in etcd/disk | Logs via application only       | No – depends on container logging driver  |
| **Job Types Supported**   | **Any Linux process** + **pre-built runtimes**            | Shell scripts, SQL, Spark, Flink, Python    | Shell scripts, HTTP/Webhooks, Docker   | Java method jobs (Lambdas)      | Any Linux process                         |
| **Security Model**        | mTLS + RBAC + complete isolation                          | LDAP/SSO + role permissions                 | Basic Auth + token-based               | In-app only                     | Depends on container runtime              |
| **Deployment Complexity** | ✅ **One binary** + optional runtime setup                 | ❌ Complex (ZooKeeper, DB, multiple workers) | ✅ Medium (agents per node)             | ✅ Simple (library import)       | ❌ Requires container runtime setup        |
| **Use Case Fit**          | **AI/ML, Pipeline, instant dev environments**             | Large ETL / batch orchestration             | Distributed cron with clustering       | Background tasks in Java apps   | Secure container workloads (e.g. sandbox) |

## 📦 Quick Start

### A. Joblet Server Installation (Linux Only)

The Joblet daemon runs on Linux servers and executes jobs in isolated environments.

#### Option 1: Package Installation (Recommended for Ubuntu/Debian)

```bash
# 1. Download and install the .deb package
wget $(curl -s https://api.github.com/repos/ehsaniara/joblet/releases/latest | grep "browser_download_url.*_amd64\.deb" | cut -d '"' -f 4)
sudo dpkg -i joblet_*_amd64.deb

# 2. Start the Joblet service
sudo systemctl start joblet
sudo systemctl enable joblet

# 3. Verify the server is running
sudo systemctl status joblet
```

#### Option 2: Manual Installation (Any Linux Distribution)

```bash
# 1. Download the Linux release
curl -L -o rnx-linux-amd64.tar.gz $(curl -s https://api.github.com/repos/ehsaniara/joblet/releases/latest | grep "browser_download_url.*linux-amd64.tar.gz" | cut -d '"' -f 4)
tar -xzf rnx-linux-amd64.tar.gz
cd rnx-linux-amd64

# 2. Run the installation script
sudo ./install.sh

# 3. Start the Joblet service
sudo systemctl start joblet
sudo systemctl enable joblet
```

The Joblet server will now be running on port 50051 (gRPC).

### B. RNX Client Installation (All Platforms)

The RNX CLI connects to Joblet servers from Linux, macOS, or Windows machines.

#### Linux Client

```bash
# If installed on the same machine as Joblet, RNX is already available
# For remote Linux clients, download the binary:
wget https://github.com/ehsaniara/joblet/releases/latest/download/rnx-linux-amd64.tar.gz
tar -xzf rnx-linux-amd64.tar.gz
sudo mv rnx-linux-amd64/bin/rnx /usr/local/bin/
```

#### macOS Client

**Option 1: Homebrew (Recommended)**

```bash
# Add the tap (specifying the full GitHub URL)
brew tap ehsaniara/joblet https://github.com/ehsaniara/joblet

# Install with interactive setup (detects Node.js for optional web UI)
brew install rnx

# Or specify installation type:
brew install ehsaniara/joblet/rnx --with-admin     # CLI + Web UI
brew install ehsaniara/joblet/rnx --without-admin  # CLI only

# Copy configuration from your Joblet server
scp user@joblet-server:/opt/joblet/config/rnx-config.yml ~/.rnx/
```

**Option 2: Manual Installation**

```bash
# 1. Download the appropriate binary
# Intel Mac:
curl -L -o rnx https://github.com/ehsaniara/joblet/releases/latest/download/rnx-darwin-amd64

# Apple Silicon:
curl -L -o rnx https://github.com/ehsaniara/joblet/releases/latest/download/rnx-darwin-arm64

# 2. Make executable and install
chmod +x rnx
sudo mv rnx /usr/local/bin/

# 3. Copy the configuration from your Joblet server
scp user@joblet-server:/opt/joblet/config/rnx-config.yml ~/.rnx/
```

#### Windows Client

```powershell
# 1. Download the Windows binary
Invoke-WebRequest -Uri "https://github.com/ehsaniara/joblet/releases/latest/download/rnx-windows-amd64.exe" -OutFile "rnx.exe"

# 2. Add to PATH (or move to a directory in PATH)
# Create directory if it doesn't exist
New-Item -ItemType Directory -Force -Path "$env:USERPROFILE\bin"
Move-Item rnx.exe "$env:USERPROFILE\bin\"

# 3. Add to PATH permanently (run as Administrator)
[Environment]::SetEnvironmentVariable("Path", $env:Path + ";$env:USERPROFILE\bin", [EnvironmentVariableTarget]::User)

# 4. Copy configuration from Joblet server (use WinSCP or similar)
# Place in %USERPROFILE%\.rnx\rnx-config.yml
```

> **Important Notes:**
> - **Joblet Server**: Linux only, runs as a system service on port 50051
> - **RNX Client**: Works on Linux, macOS, and Windows, connects to remote Joblet servers
> - **Web Admin UI**: Available on macOS via Homebrew with optional Node.js integration
> - **Configuration**: Clients need the `rnx-config.yml` file from the server (contains mTLS certificates)

### C. Quick Test

After installation, test your setup:

```bash
# From any client machine with RNX installed:
rnx list                           # List jobs (should connect to server)
rnx run echo "Hello from Joblet!"  # Run a simple job
rnx monitor status                 # Check server metrics

# If you installed the web admin UI (macOS Homebrew with --with-admin):
rnx admin                          # Launch web interface at http://localhost:5173
```

### 🎨 Web Admin UI Features

The optional Web Admin UI provides a comprehensive visual interface for managing Joblet:

#### **📊 System Monitoring Dashboard**
- **Host Information**: Hostname, platform, architecture, uptime, cloud environment details
- **CPU Metrics**: Real-time usage, per-core monitoring, load averages, temperature (if available)
- **Memory Details**: Total/used/available memory, buffers, cache, swap usage with visual graphs
- **Disk Information**: Usage by mount point, I/O statistics, filesystem details
- **Network Interfaces**: RX/TX statistics, packet counts, error rates per interface
- **Process Monitor**: Top processes by CPU/memory usage with search and sorting capabilities

#### **💼 Job Management**
- **Job List**: Paginated view with sortable columns (UUID, status, duration, start time)
- **Job Details**: Real-time logs, execution status, resource usage, scheduling info
- **Bulk Operations**: Start, stop, and manage multiple jobs simultaneously
- **Advanced Filtering**: Filter by status, runtime, network, date range

#### **🔄 Workflow Visualization**
- **Workflow Explorer**: List view of all workflows with drill-down capability  
- **Graph View**: Visual dependency graph showing job relationships and execution flow
- **Tree View**: Hierarchical display of workflow structure and job dependencies
- **Timeline View**: Execution timeline with job duration and dependencies
- **Real-time Updates**: Live status updates as workflows execute

#### **🎛️ Administration Tools**
- **Volume Management**: Create, delete, and monitor volume usage
- **Network Configuration**: Custom network creation and management
- **Runtime Information**: Available runtimes, package listings, health checks
- **User-selectable Settings**: Page sizes, refresh intervals, display preferences

> **🖥️ Access the Web UI**: After installing with Homebrew `--with-admin`, run `rnx admin` to launch the interface at http://localhost:5173

> **📋 Complete Installation Guide**: See [docs/INSTALLATION.md](docs/INSTALLATION.md) for detailed installation
> instructions covering all platforms (Ubuntu, RHEL, Amazon Linux, macOS, Windows) and deployment scenarios.
>
> **🍺 macOS Homebrew**: The Homebrew formula is available in this repository's `Formula` directory.
> The tap supports interactive installation with optional web admin UI.

## 🚀 Basic Usage

### 🎯 YAML Workflows (NEW - Simplified Job Execution)

All examples now support YAML workflows for one-liner job execution:

```bash
# Run pre-configured jobs with simple workflows
rnx run --workflow=jobs.yaml:ml-analysis      # Python ML data analysis
rnx run --workflow=jobs.yaml:hello-joblet     # Java application
rnx run --workflow=jobs.yaml:hello-world      # Basic demo
rnx run --workflow=jobs.yaml:sales-analysis   # Python analytics

# Run workflows with consolidated commands
rnx run --workflow=ml-pipeline.yaml           # Full ML workflow
rnx status <job-uuid>                        # Check job status
rnx status --workflow <workflow-uuid>        # Check workflow status

# Discover available jobs in any example directory
cat jobs.yaml                                 # Shows all configured jobs

# Workflow validation (prevents runtime failures)
rnx run --workflow=complex-pipeline.yaml     # Validates dependencies, networks, volumes, runtimes
```

**🛡️ Workflow Validation**: Joblet performs comprehensive validation before execution:

- ✅ **Circular Dependencies**: Detects dependency loops
- ✅ **Network Validation**: Confirms all networks exist (built-in + custom)
- ✅ **Volume Validation**: Verifies all volumes are available
- ✅ **Runtime Validation**: Checks runtime availability
- ✅ **Job Dependencies**: Ensures all dependencies reference existing jobs

### 💻 Traditional Commands

```bash
# Run commands with full isolation
rnx run echo "Hello World"
rnx run --max-cpu=50 --max-memory=512 python3 script.py

# Pre-built runtimes for instant startup (no package installation delays)
rnx run --runtime=python:3.11-ml python data_analysis.py    # 2-3 seconds vs 5-45 minutes
rnx run --runtime=java:17 javac HelloWorld.java && java HelloWorld
rnx run --runtime=java:21 java VirtualThreadsApp.java       # Modern Java features
rnx run --runtime=nodejs:18 node server.js                  # 2-3 seconds vs 60-300 seconds

# Upload files and run processing
rnx run --upload=data.csv --upload=script.py python3 script.py
rnx run --runtime=python:3.11-ml --upload=analysis.py python analysis.py

# Environment variables (regular and secret)
rnx run --env=NODE_ENV=production --env=PORT=8080 node app.js
rnx run --secret-env=API_KEY=secret123 --secret-env=DB_PASSWORD=pass123 python app.py
rnx run --env=NODE_ENV=prod --secret-env=API_KEY=secret node app.js

# Network isolation modes
rnx run --network=none secure_task.sh           # No network access
rnx run --network=isolated wget https://api.com # External access only  
rnx run --network=bridge api_server.py          # Inter-job communication

# Persistent storage
rnx volume create mydata --size=1GB
rnx run --volume=mydata python3 process_data.py

# Job scheduling
rnx run --schedule="1hour" backup.sh
rnx run --schedule="2025-12-25T00:00:00" maintenance.py

# Runtime management
rnx runtime list                              # Show available runtimes
rnx runtime info python:3.11-ml             # Runtime details and packages
rnx runtime test java:17                     # Test runtime functionality

# Runtime deployment (all sizes)
sudo unzip python:3.11-ml-runtime.zip -d /opt/joblet/runtimes/
```

> **📚 Complete Documentation**: For detailed guides covering all features, see the comprehensive documentation
> in [docs/](docs/):
>
> - **[Quick Start Guide](docs/QUICKSTART.md)** - Get running in 5 minutes
> - **[Job Execution Guide](docs/JOB_EXECUTION.md)** - Complete job management with examples
> - **[Environment Variables](docs/ENVIRONMENT_VARIABLES.md)** - Configuration, secrets, and workflow integration
> - **[Network Management](docs/NETWORK_MANAGEMENT.md)** - Custom networks and isolation
> - **[Volume Management](docs/VOLUME_MANAGEMENT.md)** - Persistent and temporary storage
> - **[RNX CLI Reference](docs/RNX_CLI_REFERENCE.md)** - Complete command reference
> - **[Configuration Guide](docs/CONFIGURATION.md)** - Server and client setup
> - **[Security Guide](docs/SECURITY.md)** - mTLS, RBAC, and security best practices
> - **[Troubleshooting](docs/TROUBLESHOOTING.md)** - Common issues and solutions

## ⚙️ Configuration

Joblet uses auto-generated configurations with embedded mTLS certificates for secure communication.

- **Server Configuration**: `/opt/joblet/config/joblet-config.yml` (auto-generated during installation)
- **Client Configuration**: `~/.rnx/rnx-config.yml` (copied from server after installation)

> **📋 Detailed Configuration**: See [Configuration Guide](docs/CONFIGURATION.md) for complete server/client setup,
> multi-environment configurations, certificate management, and advanced options.

## 🔒 Security & Requirements

**Security Features**: mTLS authentication, RBAC, process isolation, filesystem isolation, network isolation, resource
limits, CPU core binding, and upload security.

**Network Requirements**: Linux kernel with namespace support, IP forwarding, iptables with NAT, and bridge utilities (
automatically configured during installation).

> **🔐 Complete Security Guide**: See [Security Guide](docs/SECURITY.md) for detailed security architecture, certificate
> management, RBAC setup, and compliance considerations.

## 🛠️ Advanced Usage

> **🚀 Advanced Configuration**: For detailed guides on advanced topics, see:
> - **[Network Management](docs/NETWORK_MANAGEMENT.md)** - Custom networks, CIDR allocation, multi-tenant isolation
> - **[Job Execution Guide](docs/JOB_EXECUTION.md)** - CPU core management, NUMA allocation, resource optimization
> - **[Configuration Guide](docs/CONFIGURATION.md)** - Multi-environment setup, certificate management, service
    configuration

## 🚨 Troubleshooting

### Quick Health Check

```bash
# Check server status and connectivity
sudo systemctl status joblet
rnx list

# Monitor system metrics
rnx monitor status
```

### Getting Help

- **Configuration Examples**: `rnx config-help`
- **Node Information**: `rnx nodes`
- **GitHub Issues**: [Report bugs](https://github.com/ehsaniara/joblet/issues)

> **🔧 Complete Troubleshooting Guide**: See [Troubleshooting Guide](docs/TROUBLESHOOTING.md) for comprehensive solutions
> to common issues, performance monitoring, debugging commands, and diagnostic procedures.

## 📊 System Requirements

### Joblet Daemon (Linux Server)

- **OS**: Linux kernel 4.6+ (Ubuntu 18.04+, CentOS 8+)
- **CPU**: Multi-core CPU recommended for core limiting features
- **Memory**: 1GB+ RAM
- **Disk**: 5GB+ available space
- **Network**: Port 50051 (gRPC)
- **Privileges**: Root access for namespace operations
- **Cgroups**: cgroups v2 with cpuset controller support
- **Network Tools**: iptables, iproute2, bridge-utils

### RNX CLI (Client)

- **OS**: Linux, macOS 10.15+, Windows 10+
- **Memory**: 50MB+ RAM
- **Network**: Access to daemon port 50051

## 🚀 Command Reference

### Essential Commands

```bash
rnx run <command>           # Execute jobs with isolation  
rnx run --workflow=<yaml>   # Execute workflows from templates
rnx list                    # List all jobs
rnx list --workflow         # List all workflows
rnx status <job-uuid>       # Job details and status
rnx status --workflow <workflow-uuid> # Workflow details and status
rnx log <job-uuid>         # Stream job logs
rnx stop <job-uuid>        # Stop running job
rnx monitor status         # System metrics
```

### Key Flags

```bash
--max-cpu=50              # CPU limit (50%)
--max-memory=1024         # Memory limit (1GB)
--cpu-cores="0-3"         # Bind to specific cores  
--runtime=python:3.11-ml  # Use pre-built runtime
--runtime=nodejs:18       # Node.js 18 LTS runtime
--network=mynet           # Custom network
--volume=data             # Mount volume
--upload=file.py          # Upload files
--schedule="1hour"        # Schedule execution
--env=KEY=VALUE           # Set environment variable (visible in logs)
-e KEY=VALUE              # Short form of --env
--secret-env=KEY=VALUE    # Set secret environment variable (hidden from logs)
-s KEY=VALUE              # Short form of --secret-env

```

> **📝 Complete CLI Reference**: See [RNX CLI Reference](docs/RNX_CLI_REFERENCE.md) for comprehensive command
> documentation with all flags, options, examples, and specification formats.

## 🤝 Contributing

1. Fork the repository
2. Create feature branch: `git checkout -b feature/amazing-feature`
3. Commit changes: `git commit -m 'Add amazing feature'`
4. Push to branch: `git push origin feature/amazing-feature`
5. Open Pull Request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

**Quick Links:**

- [Releases](https://github.com/ehsaniara/joblet/releases) - Download latest version
- [Examples](https://github.com/ehsaniara/joblet/tree/main/examples) - Usage examples
- [Contributing](.github/CONTRIBUTING.md) - Development guide

## 📚 Examples

### 🎯 YAML Workflow Examples (NEW)

All examples now include YAML workflows for simplified execution:

```bash
# Python ML data analysis (pre-built runtime)
cd examples/python-3.11-ml
rnx run --workflow=jobs.yaml:ml-analysis        # 2-3 seconds vs 5-45 minutes

# Java enterprise development
cd examples/java-17  
rnx run --workflow=jobs.yaml:hello-joblet       # Instant compilation

# Basic Joblet concepts
cd examples/basic-usage
rnx run --workflow=jobs.yaml:hello-world        # Simple demo
rnx run --workflow=jobs.yaml:file-ops           # File operations

# Python analytics (standard library)
cd examples/python-analytics
rnx run --workflow=jobs.yaml:sales-analysis     # Sales data analysis
```

### Python Data Analysis Example

The [Python Analytics Example](examples/python-analytics/) demonstrates:

- **YAML Workflows**: Simple job execution with `rnx run --workflow=jobs.yaml:job-name`
- **External Scripts**: Debuggable Python scripts in `scripts/` directory
- **Standard Library Only**: No external dependencies required
- **Resource Limits**: Realistic CPU and memory usage
- **Real-world Workflow**: Sales analysis, customer segmentation, time series

```bash
# Quick start with workflows
cd examples/python-analytics
rnx run --workflow=jobs.yaml:sales-analysis     # Sales data analysis
rnx run --workflow=jobs.yaml:time-series        # Time series processing

# Traditional method
./run_demo.sh
```

See the [full example](examples/python-analytics/) for comprehensive analytics workflows.

### Workflow Examples

The [Workflow Examples](examples/workflows/) demonstrate advanced job orchestration and dependency management:

- **ML Pipeline**: Complete machine learning workflow with data prep → feature selection → training → evaluation →
  deployment
- **Data Pipeline**: ETL workflow with extraction → validation → transformation → loading → reporting
- **Parallel Processing**: Independent batch jobs running concurrently without dependencies
- **Multi-Workflow**: Multiple named workflows in a single template file
- **Deployment Pipeline**: Build → test → package → deploy → verify workflow

```bash
# Individual job execution
cd examples/workflows/ml-pipeline
rnx run --workflow=ml-pipeline.yaml:data-validation

# Full workflow orchestration with consolidated commands
cd examples/workflows/ml-pipeline
rnx run --workflow=ml-pipeline.yaml                    
rnx status --workflow <workflow-uuid>                               
```

**Features Demonstrated:**

- File uploads with `uploads.files` for script deployment
- Volume mounting with `volumes` for data persistence across jobs
- Runtime environments with `runtime: "python:3.11"`
- Resource limits (CPU/memory) per job
- Complex dependency expressions with boolean logic
- Sequential and parallel execution patterns

See the [workflow documentation](examples/workflows/README.md) for detailed guides and realistic job templates.

### Node.js Web API Example

The [Node.js Web API Example](https://github.com/ehsaniara/joblet/tree/main/examples/nodejs-web-api) demonstrates:

- **REST API Server**: Express.js server with full CRUD operations
- **Background Workers**: Asynchronous task processing with SQLite
- **Service Communication**: Multiple services working together in custom networks
- **npm Dependencies**: Package installation in isolated environment
- **Production Patterns**: Health checks, graceful shutdown, error handling

```bash
# Quick start - run API server and worker
cd examples/nodejs-web-api
rnx network create myapp --cidr=10.100.0.0/24

# Start API server
rnx run --upload-dir=. --network=myapp --max-memory=512 bash run-api.sh

# Start background worker (in another terminal)
rnx run --upload-dir=. --network=myapp --max-memory=256 node worker.js

# Test the API
rnx run --upload=test-api.sh --network=myapp bash test-api.sh
```

See the [full example](https://github.com/ehsaniara/joblet/tree/main/examples/nodejs-web-api) for API endpoints,
scheduling workers, and production considerations.

## 📋 Recent Changes & Improvements

### Latest Updates (January 2025) - Async Log System & Multi-Platform Release

- **Async Log Persistence**: Rate-decoupled async log system optimized for HPC workloads achieving 5M+ writes/second
- **HPC Optimization**: Producer-consumer pattern with overflow protection and microsecond write latency
- **Comprehensive Testing**: 11 async system tests, integration tests, and performance benchmarks
- **Multi-Platform Releases**: Automated GitHub Actions for Linux, macOS, Windows releases

#### 🎯 New Workflow Orchestration System

- **Multi-Job Workflows**: Execute complex job dependencies with automatic ordering
- **Volume Sharing**: Persistent data sharing between workflow jobs using mounted volumes
- **Dependency Management**: Define job dependencies with automatic execution scheduling
- **YAML Workflow Templates**: Complete workflows defined in single YAML files
- **Production-Ready**: End-to-end tested with data pipeline workflows (extract → validate → transform → load → report)

#### 🎯 New YAML Workflow System

- **Simplified Job Execution**: Run complex jobs with `rnx run --workflow=jobs.yaml:job-name`
- **Pre-configured Examples**: All 6 example directories now include `jobs.yaml` files
- **External Scripts**: Moved from embedded commands to debuggable external script files
- **Realistic Features**: Removed unsupported features, using only current Joblet capabilities
- **Resource Optimization**: Updated CPU/memory limits to realistic values (≤80% CPU, ≤6GB RAM)
- **Architectural Cleanup**: Moved `/examples/runtimes` to `/runtimes` for logical separation of infrastructure vs
  examples

#### 📂 Updated Examples

- ✅ **agentic-ai/**: AI workflows with ML inference, RAG systems, multi-agent coordination
- ✅ **basic-usage/**: Fundamental concepts with external shell scripts
- ✅ **java-17/**: Enterprise Java development with realistic features
- ✅ **java-21/**: Modern Java with Virtual Threads (no preview features)
- ✅ **python-3.11-ml/**: ML data analysis with proper runtime references
- ✅ **python-analytics/**: Clean analytics using Python standard library

### Version 2.11.0 (August 2025) - Runtime System Release

#### 🚀 Major New Feature: Pre-built Runtime System

- **Runtime Environments**: Complete isolated runtime environments for instant job execution
- **Python 3.11 + ML Runtime**: Pre-installed NumPy, Pandas, Scikit-learn, Matplotlib, SciPy (10-100x faster than
  package installation)
- **Java Runtimes**: OpenJDK 17 LTS and OpenJDK 21 with Maven, Virtual Threads, and modern features
- **Node.js 18 LTS Runtime**: Pre-installed Express, TypeScript, ESLint, Prettier, and development tools (20-100x faster
  than npm install)
- **Runtime Management**: Full CLI support - `rnx runtime list`, `rnx runtime info`, `rnx runtime test`
- **Performance Revolution**: 2-3 second startup vs 5-45 minutes for package installation

#### 📦 Zero-Contamination Runtime Deployment

- **Runtime Packaging**: Setup scripts automatically create deployment packages (`.zip`)
- **Clean Production Deploy**: Direct extraction deploys without installing build tools or dependencies
- **Build-Once, Deploy-Many**: Build runtimes on development hosts, deploy anywhere
- **Production Isolation**: Target hosts remain completely clean - no compilers, dev tools, or package managers needed

#### 🎯 Runtime System Features

- **Filesystem Isolation**: Runtime libraries mounted read-only in isolated containers
- **Environment Management**: Automatic PATH, PYTHONPATH, LD_LIBRARY_PATH, NODE_PATH setup
- **Package Pre-installation**: ML packages, development tools, and dependencies ready instantly
- **Compatibility**: Works seamlessly with existing isolation, networking, and security features
- **Automated Packaging**: Runtime setup scripts create deployment packages automatically

#### 🎯 API Enhancements

- **RuntimeService gRPC**: Complete runtime management via gRPC API
- **Runtime Resolution**: Smart runtime name parsing and resolution system
- **Runtime Validation**: Built-in testing and verification of runtime environments
- **GetJobLogs Implementation**: Full streaming logs support for real-time job output monitoring
- **ListJobs Implementation**: Complete job listing with metadata and status information

#### 📊 Monitoring Improvements

- **Enhanced CPU Metrics**: Detailed breakdown showing user, system, idle, and I/O wait percentages
- **Top Processes Display**: Monitor shows top 10 processes by CPU usage in formatted tables
- **Optimized Table Formatting**: Better column width management for improved readability
- **Network Interface Limits**: Display capped at 10 most active interfaces for clarity

#### 🛠️ Code Quality

- **Simplified Resource Limits**: Removed complex builder pattern for cleaner, more maintainable code
- **File Upload Enhancement**: Removed artificial 50MB/file and 100MB total limits - now unlimited
- **CI/CD Improvements**: Test suite gracefully handles containerized environments
- **Runtime Architecture**: Clean separation of concerns with runtime resolver, manager, and types

#### 🔧 Technical Details

- No breaking changes to existing APIs
- Backward compatibility maintained for all client versions
- Improved error messages and logging throughout

## 💡 Project Name History

This project was previously known as "Worker" but was renamed to:

- **Joblet** (daemon) - Better describes the job execution platform
- **RNX** (CLI) - Remote eXecution, a concise and memorable name

The rename provides clearer branding and eliminates confusion with other "worker" tools in the ecosystem.