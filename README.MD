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

> **🤖 Agentic AI Foundation:** Joblet is purpose-built for AI agent workloads with **sandboxed code execution**, **resource-controlled tool usage**, **real-time monitoring**, and **secure multi-tenant isolation** - enabling AI agents to safely execute arbitrary code, run system commands, process data, and interact with external tools without compromising host security

## 🎯 Key Features

- **Process Isolation**: Jobs run in separate PID namespaces with limited filesystem access
- **Resource Control**: CPU, memory, I/O bandwidth limits with specific core binding
- **Network Isolation**: Custom networks with traffic segmentation (none/bridge/isolated/custom)
- **Volume Management**: Persistent storage with filesystem and memory-based volumes
- **Job Scheduling**: Future execution with flexible time specifications
- **Real-time Monitoring**: Live logs, system metrics, and job status tracking
- **mTLS Security**: Certificate-based authentication with role-based access control
- **Cross-platform CLI**: Manage jobs from Linux, macOS, or Windows

## 🔧 How It Works

Joblet consists of two components:

1. **Joblet Daemon** - Runs on Linux servers, executes jobs in isolated environments
2. **RNX CLI** - Connects from anywhere (Linux/macOS/Windows) to manage jobs

![joblet-how-it-works.svg](docs/joblet-how-it-works.svg)

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

| Feature / Tool            | **Joblet** (v2.10.0)                          | **Apache DolphinScheduler**                 | **Dkron**                              | **JobRunr**                     | **gVisor** / **Kata**                     |
|---------------------------|-----------------------------------------------|---------------------------------------------|----------------------------------------|---------------------------------|-------------------------------------------|
| **Type**                  | Linux-native job execution platform           | Distributed job orchestration engine        | Distributed cron job scheduler         | In-memory job scheduler for JVM | OCI container runtime sandboxing          |
| **Isolation Mechanism**   | Linux **namespaces** + **cgroups v2**         | None (relies on system agents)              | None (delegates to OS processes)       | JVM sandboxing                  | Full syscall interception / VM isolation  |
| **Execution Mode**        | Binary CLI + gRPC API                         | Web UI + REST API                           | Agent-based CLI + API                  | Java code only (embedded)       | Containerized / OCI only                  |
| **Filesystem Isolation**  | ✅ via chroot + mount namespace                | ❌                                           | ❌                                      | ❌                               | ✅ full container FS isolation             |
| **Network Isolation**     | Optional (separate netns)                     | ❌                                           | ❌                                      | ❌                               | ✅ (via container)                         |
| **Resource Limits**       | ✅ CPU, memory, I/O, pids via cgroups          | ❌                                           | ❌                                      | ❌                               | ✅ via kernel interface                    |
| **Programming Interface** | gRPC + binary CLI                             | REST API                                    | HTTP API + CLI                         | Java annotations / Spring       | OCI runtime interface only                |
| **Real-time Logging**     | ✅ Built-in SSE log stream + file              | Partial (log files in worker nodes)         | No streaming, logs stored in etcd/disk | Logs via application only       | No – depends on container logging driver  |
| **Job Types Supported**   | Bash, Python, binary, arbitrary Linux process | Shell scripts, SQL, Spark, Flink, Python    | Shell scripts, HTTP/Webhooks, Docker   | Java method jobs (Lambdas)      | Any Linux process                         |
| **Security Model**        | Local users or mTLS + RBAC                    | LDAP/SSO + role permissions                 | Basic Auth + token-based               | In-app only                     | Depends on container runtime              |
| **Deployment Complexity** | ✅ One binary                                  | ❌ Complex (ZooKeeper, DB, multiple workers) | ✅ Medium (agents per node)             | ✅ Simple (library import)       | ❌ Requires container runtime setup        |
| **Use Case Fit**          | Secure workload exec (CI/CD, scripts, etc.)   | Large ETL / batch orchestration             | Distributed cron with clustering       | Background tasks in Java apps   | Secure container workloads (e.g. sandbox) |

## 📦 Quick Start

```bash
# 1. Download and install (Linux)
wget $(curl -s https://api.github.com/repos/ehsaniara/joblet/releases/latest | grep "browser_download_url.*_amd64\.deb" | cut -d '"' -f 4)
sudo dpkg -i joblet_*_amd64.deb

# 2. Start the service
sudo systemctl start joblet
sudo systemctl enable joblet

# 3. Test it works
rnx run echo "Hello Joblet!"
rnx list
```

> **📋 Complete Installation Guide**: See [docs/INSTALLATION.md](docs/INSTALLATION.md) for detailed installation
> instructions covering all platforms (Ubuntu, RHEL, Amazon Linux, macOS, Windows) and deployment scenarios.

## 🚀 Basic Usage

```bash
# Run commands with full isolation
rnx run echo "Hello World"
rnx run --max-cpu=50 --max-memory=512 python3 script.py

# Upload files and run processing
rnx run --upload=data.csv --upload=script.py python3 script.py

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
```

> **📚 Complete Documentation**: For detailed guides covering all features, see the comprehensive documentation
> in [docs/](docs/):
>
> - **[Quick Start Guide](docs/QUICKSTART.md)** - Get running in 5 minutes
> - **[Job Execution Guide](docs/JOB_EXECUTION.md)** - Complete job management with examples
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
- **Network**: Port 50051 (configurable), kernel namespace support
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
rnx list                    # List all jobs
rnx status <job-id>         # Job details and status
rnx log <job-id>           # Stream job logs
rnx stop <job-id>          # Stop running job
rnx monitor status         # System metrics
```

### Key Flags

```bash
--max-cpu=50              # CPU limit (50%)
--max-memory=1024         # Memory limit (1GB)
--cpu-cores="0-3"         # Bind to specific cores  
--network=mynet           # Custom network
--volume=data             # Mount volume
--upload=file.py          # Upload files
--schedule="1hour"        # Schedule execution
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

### Python Data Analysis Example

The [Python Data Analysis Example](https://github.com/ehsaniara/joblet/tree/main/examples/python-data-analysis)
demonstrates:

- **File uploads**: Upload Python scripts and CSV data for processing
- **Dependency management**: Install pandas, numpy, matplotlib in chroot environment
- **Network isolation**: Run analysis in isolated network environments
- **Resource limits**: Control CPU and memory usage for data processing
- **Real-world workflow**: Process sales data, generate insights and visualizations

```bash
# Quick start - run pandas analysis with automatic pip installation
cd examples/python-data-analysis
rnx run --upload-dir=. bash run_with_pandas.sh

# Or use standard library version (no dependencies)
rnx run --upload=analyze_sales_simple.py --upload=data/sales_data.csv --upload=data/customers.csv python3 analyze_sales_simple.py
```

See the [full example](https://github.com/ehsaniara/joblet/tree/main/examples/python-data-analysis) for more approaches
including self-installing scripts and manual package management.

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

### Version 2.10.0 (August 2025)

#### 🎯 API Enhancements

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

#### 🔧 Technical Details

- No breaking changes to existing APIs
- Backward compatibility maintained for all client versions
- Improved error messages and logging throughout

## 💡 Project Name History

This project was previously known as "Worker" but was renamed to:

- **Joblet** (daemon) - Better describes the job execution platform
- **RNX** (CLI) - Remote eXecution, a concise and memorable name

The rename provides clearer branding and eliminates confusion with other "worker" tools in the ecosystem.