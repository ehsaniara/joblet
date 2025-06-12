# Job Worker

[![Tests](https://github.com/ehsaniara/job-worker/actions/workflows/ci.yml/badge.svg)](https://github.com/ehsaniara/job-worker/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/ehsaniara/job-worker)](https://goreportcard.com/report/github.com/ehsaniara/job-worker)
[![Go Version](https://img.shields.io/github/go-mod/go-version/ehsaniara/job-worker)](https://golang.org/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Release](https://img.shields.io/github/release/ehsaniara/job-worker.svg)](https://github.com/ehsaniara/job-worker/releases/latest)

> A high-performance distributed job execution system with secure remote execution, resource management, and real-time
> streaming capabilities.

## ✨ Key Features

- 🔒 **Secure Remote Execution** - Mutual TLS authentication with role-based authorization
- 🎛️ **Resource Management** - CPU, memory, and I/O bandwidth limiting via Linux cgroups v2
- 📡 **Real-time Streaming** - Live output streaming of job execution logs
- 🔐 **Process Isolation** - Containerized execution environment with proper cleanup
- 🌐 **gRPC API** - High-performance API with protobuf serialization
- ⚡ **Two-Stage Execution** - job-init binary ensures proper resource isolation
- 🏗️ **Namespace Isolation** - PID, mount, IPC, and UTS namespace isolation
- 📊 **Comprehensive Monitoring** - Job status tracking and resource usage monitoring

## 🚀 Quick Start

### Prerequisites

**System Requirements:**

- **Operating System**: Linux (primary) or macOS (limited functionality)
- **Go Version**: 1.23 or later
- **Linux Requirements**:
    - Kernel 4.5+ (for cgroups v2)
    - Root access (for cgroup management)
    - systemd (recommended for service management)

### Installation Options

#### Option 1: Install from Source

```bash
# Install binaries directly
go install github.com/ehsaniara/job-worker/cmd/worker@latest
go install github.com/ehsaniara/job-worker/cmd/cli@latest
```

#### Option 2: Download Pre-built Binaries

```bash
# Download latest release
wget https://github.com/ehsaniara/job-worker/releases/latest/download/job-worker-linux-amd64.tar.gz
tar -xzf job-worker-linux-amd64.tar.gz
```

#### Option 3: Build from Source (Recommended for Development)

```bash
# Clone and build
git clone https://github.com/ehsaniara/job-worker.git
cd job-worker
make setup-dev  # Builds binaries + generates certificates
```

### Local Development Setup

```bash
# 1. Complete development setup
make setup-dev

# 2. Start the server
./bin/job-worker

# 3. Create your first job (in another terminal)
./bin/cli --cert certs/admin-client-cert.pem --key certs/admin-client-key.pem create echo "Hello, World!"

# 4. Check job status
./bin/cli get <job-id>

# 5. Stream job output in real-time
./bin/cli stream <job-id>
```

### Remote Server Setup

```bash
# Complete remote setup (requires SSH access)
export REMOTE_HOST=your-server.com
make setup-remote-passwordless

# Download client certificates for local CLI
make certs-download-admin-simple

# Connect to remote server
./bin/cli --server=$REMOTE_HOST:50051 --cert certs/client-cert.pem --key certs/client-key.pem create echo "Hello from remote!"
```

## 📖 Usage Examples

### Basic Job Management

```bash
# Create simple jobs
./bin/cli create echo "Hello, World!"
./bin/cli create python3 script.py
./bin/cli create bash -c "for i in {1..10}; do echo 'Processing $i'; sleep 1; done"

# List all jobs
./bin/cli list

# Get detailed job information
./bin/cli get <job-id>

# Stop a running job
./bin/cli stop <job-id>

# Stream live job output
./bin/cli stream <job-id>
```

### Resource-Limited Jobs

```bash
# Limit CPU usage (50% of one core)
./bin/cli create --max-cpu=50 python3 heavy_computation.py

# Limit memory usage (512 MB)
./bin/cli create --max-memory=512 java -jar memory-intensive-app.jar

# Combine multiple limits
./bin/cli create --max-cpu=25 --max-memory=256 --max-iobps=1000 intensive-task.sh
```

### Remote Server Management

```bash
# Deploy to production server
make deploy-safe REMOTE_HOST=prod.example.com

# Monitor service status
make service-status

# View live service logs
make live-log

# Check certificate status
make check-certs-remote
```

## 🏗️ Architecture

![Architecture Diagram](docs/high-level.svg)

### Core Components

- **Worker Service**: Main server handling job execution and management
- **CLI Client**: Command-line interface for interacting with the service
- **Job-Init**: Initialization binary ensuring proper process isolation
- **gRPC API**: High-performance communication layer
- **Store**: In-memory job state management with real-time updates
- **Resource Manager**: cgroups v2 integration for resource isolation

## 🔐 Security & Authentication

### Certificate-Based Authentication

The system uses mutual TLS with role-based access control:

| Role       | Create Jobs | View Jobs | Stop Jobs | Stream Logs |
|------------|-------------|-----------|-----------|-------------|
| **Admin**  | ✅           | ✅         | ✅         | ✅           |
| **Viewer** | ❌           | ✅         | ❌         | ✅           |

Roles are determined by the `OU` (Organizational Unit) field in client certificates:

- `OU=admin` → Full administrative privileges
- `OU=viewer` → Read-only access

### Certificate Management

```bash
# Generate certificates locally
make certs-local

# Generate certificates on remote server
make certs-remote-passwordless

# Download admin certificates
make certs-download-admin-simple

# Download viewer certificates
make certs-download-viewer

# Verify certificate chain
make verify-cert-chain

# Examine certificate details
make examine-certs
```

## ⚙️ Configuration

### Environment Variables

```bash
# Server Configuration
export JOB_WORKER_ADDR="0.0.0.0:50051"
export JOB_WORKER_CERT_PATH="./certs"
export JOB_WORKER_CGROUP_BASE="/sys/fs/cgroup"

# Default Resource Limits
export JOB_WORKER_DEFAULT_CPU=100      # 100% of one core
export JOB_WORKER_DEFAULT_MEMORY=512   # 512 MB
export JOB_WORKER_DEFAULT_IO=0         # Unlimited

# Logging
export LOG_LEVEL="INFO"  # DEBUG, INFO, WARN, ERROR
```

### Client Configuration

```bash
# Connect to remote server
./bin/cli --server=remote-host:50051 create echo "hello"

# Use custom certificate path
./bin/cli --cert-path=./custom-certs create echo "hello"
```

## 🛠️ Development

### Build Commands

```bash
# Build all components
make all

# Build individual components
make cli     # macOS CLI for local development
make worker  # Linux server binary
make init    # Linux job-init binary

# Development setup
make setup-dev

# Clean build artifacts
make clean
```

### Testing & Debugging

```bash
# Run tests
go test -v ./...

# Test TLS connection
make test-tls

# Monitor service logs
make live-log

# Check service status
make service-status

# Test SSH connection to remote server
make test-connection
```

## 📋 API Reference

### gRPC Service Definition

The service provides the following operations:

- `CreateJob` - Create and start a new job
- `GetJob` - Retrieve job details by ID
- `StopJob` - Terminate a running job
- `GetJobs` - List all jobs
- `GetJobsStream` - Stream job output in real-time

For detailed API documentation, see [API Reference](docs/API.md).

### CLI Commands

```bash
# Job creation with options
cli create [--max-cpu=N] [--max-memory=N] [--max-iobps=N] <command> [args...]

# Job management
cli get <job-id>          # Get job details
cli list                  # List all jobs
cli stop <job-id>         # Stop running job
cli stream <job-id>       # Stream job output

# Connection options
cli --server=host:port    # Connect to remote server
cli --cert=path           # Client certificate
cli --key=path            # Client private key
```

## 📚 Documentation

- 📋 [Technical Design](docs/DESIGN.md) - Detailed system architecture and design decisions
- 🔧 [API Reference](docs/API.md) - Complete gRPC API documentation
- 🚀 [Deployment Guide](docs/DEPLOYMENT.md) - Production deployment strategies
- 🤝 [Contributing](CONTRIBUTING.md) - Development guidelines and contribution process

## 🔧 Troubleshooting

### Common Issues

**Certificate Issues:**

```bash
# Check certificate status
make check-certs-remote
make examine-certs

# Regenerate certificates
make certs-local          # For local development
make certs-remote-passwordless  # For remote server
```

**Permission Issues:**

```bash
# Fix certificate permissions
make fix-cert-permissions

# Ensure proper sudo access for deployment
make test-connection
```

**Service Issues:**

```bash
# Check service status
make service-status

# View service logs
make live-log

# Restart service
ssh user@remote "sudo systemctl restart job-worker.service"
```

## 🤝 Contributing

We welcome contributions! Here's how to get started:

1. **Fork the repository**
2. **Create a feature branch**: `git checkout -b feature/amazing-feature`
3. **Make your changes and add tests**
4. **Run tests**: `go test -v ./...`
5. **Commit your changes**: `git commit -m 'Add amazing feature'`
6. **Push to your branch**: `git push origin feature/amazing-feature`
7. **Open a Pull Request**

### Development Workflow

```bash
# Setup development environment
make setup-dev

# Build and test
make all
go test -v ./...

# Deploy to test server
make deploy-safe REMOTE_HOST=test-server.com

# Monitor and debug
make live-log
make service-status
```

### Isolation of Process

Isolation check:

* First create a job: `./bin/cli create sleep 300`
* Then create a second job ` ./bin/cli create ps aux`
* But when you look at the second job's output `./bin/cli stream 2` only the second jobs process tree shown

```bash
USER         PID %CPU %MEM    VSZ   RSS TTY      STAT START   TIME COMMAND
root           1  0.0  0.0  10044  1584 ?        R    08:56   0:00 /usr/bin/ps aux
```

## 📝 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- [gRPC](https://grpc.io/) - High-performance RPC framework
- [Linux Cgroups](https://www.kernel.org/doc/Documentation/cgroup-v2.txt) - Resource isolation and management
- [Cobra](https://github.com/spf13/cobra) - Modern CLI framework for Go
- [Protocol Buffers](https://developers.google.com/protocol-buffers) - Language-neutral data serialization

## 📊 Performance & Monitoring

The system provides comprehensive monitoring capabilities:

- **Resource Usage**: Real-time CPU, memory, and I/O monitoring via cgroups
- **Job Metrics**: Execution time, exit codes, and status tracking
- **System Health**: Service status, connection monitoring, and error tracking
- **Live Streaming**: Real-time log output with efficient buffering

---

<div align="center">
  <sub>by <a href="https://github.com/ehsaniara">Jay Ehsaniara</a></sub>
</div>