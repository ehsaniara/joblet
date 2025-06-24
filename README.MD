# Worker - Isolated Job Execution System

[![Tests](https://github.com/ehsaniara/worker/actions/workflows/ci.yml/badge.svg)](https://github.com/ehsaniara/worker/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/ehsaniara/worker)](https://goreportcard.com/report/github.com/ehsaniara/worker)
[![Go Version](https://img.shields.io/github/go-mod/go-version/ehsaniara/worker)](https://golang.org/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Release](https://img.shields.io/github/release/ehsaniara/worker.svg)](https://github.com/ehsaniara/worker/releases/latest)

A high-performance job execution system built in Go that provides secure, isolated execution environments using Linux namespaces and cgroups. The system enables safe execution of arbitrary commands with comprehensive resource management, filesystem isolation, and real-time monitoring capabilities.

## Table of Contents

- [Features](#features)
- [Quick Start](#quick-start)
- [Architecture](#architecture)
- [API Usage](#api-usage)
- [System Validation](#system-validation)
- [Development](#development)
- [Performance](#performance)
- [Security](#security)
- [Contributing](#contributing)
- [License](#license)

## Features

### **Isolation & Security**
- **User Namespace Isolation**: Runs jobs with mapped user/group IDs for enhanced security
- **Filesystem Isolation**: Creates isolated filesystem environments with bind mounting
- **Process Isolation**: Uses PID, IPC, UTS, and mount namespaces
- **Resource Limits**: CPU, memory, and I/O bandwidth limiting via cgroups v2

### **Job Management**
- **Real-time Streaming**: Live log streaming with WebSocket-like interfaces
- **Job State Management**: Complete lifecycle management (initializing, running, completed, failed, stopped)
- **Concurrent Execution**: Support for multiple simultaneous jobs
- **Persistent Storage**: Job state and output persistence

### **Monitoring & Observability**
- **Live Log Streaming**: Real-time output streaming with subscriber management
- **Resource Monitoring**: CPU, memory, and I/O usage tracking
- **Job Status Updates**: Real-time status updates via publish/subscribe pattern
- **Comprehensive Logging**: Structured logging with multiple levels

### **System Requirements**
- **Kernel**: Linux 4.6+ (for cgroup namespaces and user namespace support)
- **Go**: 1.21+ for building from source
- **Cgroups**: cgroup v2 filesystem mounted at `/sys/fs/cgroup`
- **User Namespaces**: Properly configured subuid/subgid mappings
- **Privileges**: Root access required for namespace and cgroup management

## Demo

```bash
# Start the worker service
sudo ./worker

# In another terminal, submit a job
curl -X POST http://localhost:8080/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "command": "echo",
    "args": ["Hello, isolated world!"],
    "limits": {
      "maxCPU": 50,
      "maxMemory": 128,
      "maxIOBPS": 1048576
    }
  }'

# Stream job logs
curl -N http://localhost:8080/jobs/{jobId}/stream
```

## Quick Start

### Prerequisites

Ensure your system meets the requirements:

```bash
# Check kernel version (need 4.6+)
uname -r

# Verify cgroup v2 is mounted
ls /sys/fs/cgroup/cgroup.controllers

# Check user namespace support
cat /proc/sys/user/max_user_namespaces

# Setup subuid/subgid for user mappings
echo "worker:100000:65536" | sudo tee -a /etc/subuid
echo "worker:100000:65536" | sudo tee -a /etc/subgid
```

### Installation

```bash
# Clone the repository
git clone https://github.com/ehsaniara/worker.git
cd worker

# Build the project
go build -o worker ./cmd/worker

# Build the job initializer
go build -o job-init ./cmd/job-init

# Install system service (optional)
sudo cp worker.service /etc/systemd/system/
sudo systemctl enable worker
sudo systemctl start worker
```

### Configuration

Environment variables for customization:

```bash
# User namespace configuration
export USER_NAMESPACE_ENABLED=true
export USER_NAMESPACE_UID=1000
export USER_NAMESPACE_GID=1000

# Filesystem isolation
export FILESYSTEM_ISOLATION=enabled

# Logging
export WORKER_LOG_LEVEL=debug

# Resource limits
export MAX_JOBS=10
export DEFAULT_CPU_LIMIT=100
export DEFAULT_MEMORY_LIMIT=512M
export DEFAULT_IO_LIMIT=10485760
```

## Architecture

### Core Components

**Job Manager (`worker.go`)**
- Orchestrates job lifecycle from creation to completion
- Manages isolation setup and resource allocation
- Coordinates between all subsystems

**Store (`store.go`, `task.go`)**
- Thread-safe job state management
- Real-time log streaming via publish/subscribe
- Job output buffering and retrieval

**Filesystem Manager (`manager.go`)**
- Creates isolated filesystem environments
- Copies essential binaries and libraries
- Sets up bind mount points for virtualization

**User Namespace Manager**
- Manages UID/GID mappings for isolation
- Validates subuid/subgid configuration
- Handles namespace creation and cleanup

**Cgroup Manager (`cgroup.go`)**
- Sets CPU, memory, and I/O limits
- Creates and manages cgroup hierarchies
- Enforces resource constraints

**Launcher (`launcher.go`)**
- Executes jobs within configured namespaces
- Handles process creation with proper attributes
- Manages command execution lifecycle

### Job Execution Flow

1. **Job Creation**: Job submitted with command, args, and resource limits
2. **Isolation Setup**:
    - User namespace mapping created
    - Isolated filesystem prepared
    - Cgroup limits configured
3. **Process Launch**: Job launched with all isolation mechanisms
4. **Monitoring**: Real-time log streaming and status updates
5. **Cleanup**: Resources cleaned up upon job completion

## API Usage

### Job Creation

```go
job := &domain.Job{
    Id:      "job-123",
    Command: "python3",
    Args:    []string{"script.py"},
    Limits: domain.JobLimits{
        MaxCPU:    100,    // CPU limit
        MaxMemory: 512,    // Memory in MB
        MaxIOBPS:  1048576, // I/O bandwidth
    },
}

result, err := worker.StartJob(ctx, job)
```

### Log Streaming

```go
// Stream job logs in real-time
err := store.SendUpdatesToClient(ctx, jobID, stream)
```

### Job Status Monitoring

```go
// Get current job status
output, isRunning, err := store.GetOutput(jobID)

// List all jobs
jobs := store.ListJobs()
```

## System Validation

The system includes comprehensive validation checks:

```bash
# Run validation checks
./worker --validate

# Check individual components
./worker --check-namespaces
./worker --check-cgroups
./worker --check-filesystem
```

### Common Issues & Solutions

**User namespace creation fails:**
```bash
# Enable unprivileged user namespaces
echo 1 | sudo tee /proc/sys/kernel/unprivileged_userns_clone

# Check/add subuid entries
sudo usermod --add-subuids 100000-165535 worker
sudo usermod --add-subgids 100000-165535 worker
```

**Cgroup v2 not available:**
```bash
# Mount cgroup v2 (if not mounted)
sudo mount -t cgroup2 none /sys/fs/cgroup

# Add to /etc/fstab for persistence
echo "none /sys/fs/cgroup cgroup2 defaults 0 0" | sudo tee -a /etc/fstab
```

## Development

### Building

```bash
# Build all components
go build -o worker ./cmd/worker
go build -o job-init ./cmd/job-init

# Or use Go install
go install ./cmd/worker
go install ./cmd/job-init

# Cross-compile for different architectures
GOOS=linux GOARCH=amd64 go build -o worker-linux-amd64 ./cmd/worker
```

### Testing

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests with race detection
go test -race ./...

# Run integration tests
go test -tags=integration ./...

# Test specific package
go test ./internal/worker/core/store

# Benchmark tests
go test -bench=. ./...
```

### Code Structure

```
├── cmd/
│   ├── worker/         # Main worker binary
│   └── job-init/       # Job initialization binary
├── internal/worker/
│   ├── core/           # Core business logic
│   ├── domain/         # Domain models
│   └── infra/          # Infrastructure components
├── pkg/
│   ├── logger/         # Logging utilities
│   └── os/             # OS interface abstractions
└── scripts/            # Build and deployment scripts
```

## Performance

### Benchmarks

- **Job Startup**: < 100ms per job
- **Memory Overhead**: ~10MB per isolated job
- **Log Streaming**: > 1000 concurrent streams supported
- **Job Throughput**: 100+ jobs/second (system dependent)

### Scaling

- Supports up to 1000 concurrent jobs (configurable)
- Horizontal scaling via multiple worker instances
- Resource usage scales linearly with job count

## Security

### Isolation Guarantees

- **Process Isolation**: Complete PID namespace separation
- **Filesystem Isolation**: Restricted filesystem access via mount namespaces
- **User Isolation**: Mapped UIDs prevent privilege escalation
- **Resource Isolation**: Cgroup limits prevent resource exhaustion

### Security Best Practices

- Run worker process as dedicated user
- Configure appropriate subuid/subgid ranges
- Monitor resource usage and set conservative limits
- Regular security updates for system components

## Contributing

We welcome contributions! Please see our [Contributing Guidelines](CONTRIBUTING.md) for details.

### How to Contribute

1. **Fork** the repository on GitHub: [https://github.com/ehsaniara/worker](https://github.com/ehsaniara/worker)
2. **Clone** your fork locally:
   ```bash
   git clone https://github.com/YOUR-USERNAME/worker.git
   cd worker
   ```
3. **Create** a feature branch:
   ```bash
   git checkout -b feature/amazing-feature
   ```
4. **Make** your changes and add tests
5. **Commit** your changes:
   ```bash
   git commit -m 'Add amazing feature'
   ```
6. **Push** to your fork:
   ```bash
   git push origin feature/amazing-feature
   ```
7. **Create** a Pull Request on GitHub

### Development Guidelines

- Follow Go best practices and conventions
- Add comprehensive tests for new features
- Update documentation for API changes
- Ensure backward compatibility when possible

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Support

For issues and questions:

- 🐛 **Report bugs**: [Create an issue](https://github.com/ehsaniara/worker/issues/new) on GitHub
- 💡 **Feature requests**: [Open a feature request](https://github.com/ehsaniara/worker/issues/new)
- 📖 **Documentation**: Check the [Wiki](https://github.com/ehsaniara/worker/wiki) for detailed guides
- 💬 **Discussions**: Join [GitHub Discussions](https://github.com/ehsaniara/worker/discussions)

### Maintainer

This project is maintained by [Ehsan Iara](https://github.com/ehsaniara).

---

**⭐ Star this repository if you find it useful!**

Built with ❤️ using Go and Linux namespaces for secure, isolated job execution.