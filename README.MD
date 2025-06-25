# Job Worker

[![Tests](https://github.com/ehsaniara/job-worker/actions/workflows/ci.yml/badge.svg)](https://github.com/ehsaniara/job-worker/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/ehsaniara/job-worker)](https://goreportcard.com/report/github.com/ehsaniara/job-worker)
[![Go Version](https://img.shields.io/github/go-mod/go-version/ehsaniara/job-worker)](https://golang.org/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Release](https://img.shields.io/github/release/ehsaniara/job-worker.svg)](https://github.com/ehsaniara/job-worker/releases/latest)

Job Worker is a high-performance distributed job execution system built in Go that provides secure remote command execution with comprehensive resource management and real-time monitoring capabilities. The system uses a sophisticated gRPC-based architecture with mutual TLS authentication and role-based authorization, allowing administrators to securely execute arbitrary commands on remote Linux systems while enforcing strict CPU, memory, and I/O bandwidth limits through Linux cgroups v2. Jobs run in isolated process namespaces (PID, mount, IPC, UTS) while utilizing host networking for maximum compatibility and performance.

>The system features a two-stage execution model with a specialized job-init binary that handles proper namespace setup and resource isolation, while the main worker service provides real-time log streaming, job lifecycle management, and automatic cleanup. A full-featured CLI client enables easy job creation, monitoring, and management, with support for streaming live output from running jobs. The architecture is designed for production environments requiring secure, isolated execution of workloads with precise resource controls, making it ideal for distributed computing, CI/CD pipelines, or any scenario where secure remote job execution with resource guarantees is needed.

## ✨ Key Features

- 🔒 **Secure Remote Execution** - Mutual TLS authentication with role-based authorization (admin/viewer roles)
- 🎛️ **Advanced Resource Management** - CPU, memory, and I/O bandwidth limiting via Linux cgroups v2
- 📡 **Real-time Streaming** - Live output streaming of job execution logs with buffering
- 🔐 **Process Isolation** - PID, mount, IPC, and UTS namespace isolation with containerized execution
- 🌐 **Host Networking** - Jobs use host network interface for maximum compatibility and performance
- ⚡ **High-Performance gRPC API** - Protocol buffer serialization with configurable message limits
- 📊 **Two-Stage Execution Architecture** - Dedicated job-init binary ensures proper resource isolation and namespace
  setup
- 🏗️ **Advanced Process Management** - Full /proc remounting and private mount propagation
- 📈 **Comprehensive Monitoring** - Job status tracking, resource usage monitoring, and real-time subscriber management
- 🔄 **Robust Error Handling** - Graceful shutdown, emergency cleanup, and process group management

## 🚀 Quick Start

### Prerequisites

**System Requirements:**

- **Kernel**: Linux 4.6+ (for cgroup namespace support)
- **Cgroups**: cgroups v2 enabled and mounted at /sys/fs/cgroup
- **Namespaces**: Cgroup namespace support in kernel
- **Root Access**: Required for cgroup management and namespace operations

## 📖 Usage Examples

### Basic Job Management

```bash
# Create simple jobs (all use host networking)
./bin/cli create echo "Hello, World!"
./bin/cli create python3 script.py
./bin/cli create bash -c "for i in {1..10}; do echo 'Processing $i'; sleep 1; done"

# List all jobs with status breakdown
./bin/cli list

# Get detailed job information
./bin/cli get <job-id>

# Stop a running job (graceful SIGTERM, then SIGKILL)
./bin/cli stop <job-id>

# Stream live job output with real-time updates
./bin/cli stream <job-id>
```

### Resource-Limited Jobs

```bash
# Limit CPU usage to 50% of one core
./bin/cli create --max-cpu=50 python3 heavy_computation.py

# Limit memory usage to 512 MB
./bin/cli create --max-memory=512 java -jar memory-intensive-app.jar

# Limit I/O bandwidth (bytes per second)
./bin/cli create --max-iobps=1000000 dd if=/dev/zero of=/tmp/test bs=1M count=100

# Combine multiple resource limits
./bin/cli create --max-cpu=25 --max-memory=256 --max-iobps=1000000 intensive-task.sh
```

### Advanced Job Scenarios

All jobs share the host network interface and can communicate with each other and external services directly:

```bash
# Network-accessible job (uses host networking)
./bin/cli create --max-cpu=100 --max-memory=1024 python3 -c "
import socket
import time
s = socket.socket()
s.bind(('0.0.0.0', 8080))
s.listen(5)
print('Server listening on host port 8080')
for i in range(60):
    print(f'Iteration {i}: Server running...')
    time.sleep(1)
"

# Job that can access external services
./bin/cli create curl -s https://api.github.com/users/octocat

# Job with complex command line arguments
./bin/cli create bash -c "echo 'Network test:' && curl -s ifconfig.me && echo '' && echo 'Finished at:' && date"
```

### Remote Server Management

```bash
# Deploy to production server (safe with password prompt)
make deploy-safe REMOTE_HOST=prod.example.com REMOTE_USER=admin

# Deploy with passwordless sudo (requires setup)
make deploy-passwordless REMOTE_HOST=prod.example.com

# Monitor service status
make service-status REMOTE_HOST=prod.example.com

# View live service logs
make live-log REMOTE_HOST=prod.example.com

# Check certificate status
make check-certs-remote REMOTE_HOST=prod.example.com
```

## 🏗️ Architecture

The Job Worker system uses a sophisticated multi-component architecture designed for security, isolation, and
performance with host networking:

### Core Components

**Worker Service (`cmd/worker/main.go`)**

- Main gRPC server handling job execution and management
- Advanced resource management via cgroups v2
- Real-time job monitoring and status tracking
- All jobs use host networking for maximum compatibility

**CLI Client (`cmd/cli/`)**

- Command-line interface with full feature support
- TLS certificate management and authentication
- Real-time streaming capabilities
- Comprehensive job management commands

**Job-Init Binary (`cmd/job-init/main.go`)**

- Specialized initialization binary for process isolation
- Namespace setup (PID, mount, IPC, UTS) - **Network namespace uses host**
- Cgroup membership management
- Platform-specific execution handling

**gRPC API Layer**

- High-performance Protocol Buffer serialization
- Mutual TLS with certificate-based authentication
- Role-based access control (admin/viewer)
- Streaming support for real-time log output

**Store & Task Management (`internal/worker/store/`)**

- In-memory job state management with thread safety
- Real-time subscriber management for log streaming
- Advanced buffering and update distribution
- Automatic cleanup for completed jobs

**Resource Manager (`internal/worker/resource/`)**

- Linux cgroups v2 integration
- CPU, memory, and I/O bandwidth limiting
- Automatic cgroup cleanup and process termination
- Graceful and forced job termination

### Process Isolation Features

The system provides comprehensive process isolation while maintaining host networking:

**Namespace Isolation:**

- **PID Namespace**: Jobs see only their own process tree
- **Mount Namespace**: Isolated filesystem view with /proc remounting
- **IPC Namespace**: Separate inter-process communication
- **UTS Namespace**: Isolated hostname and domain name
- **Network**: Uses host networking (no network namespace isolation)

**Verification Example:**

```bash
# Create a long-running job
./bin/cli create sleep 300

# Create a process listing job
./bin/cli create ps aux

# View the second job's output - only its own processes are visible
./bin/cli stream <job-2-id>
```

Output shows process isolation with host networking:

```
USER         PID %CPU %MEM    VSZ   RSS TTY      STAT START   TIME COMMAND
root           1  0.0  0.0  10044  1584 ?        R    08:56   0:00 /usr/bin/ps aux
```

The job can access the host network:

```bash
# Test network connectivity (uses host network interface)
./bin/cli create curl -s ifconfig.me
```

## 🔐 Security & Authentication

### Certificate-Based Authentication

The system implements mutual TLS with sophisticated role-based access control:

| Role       | Create Jobs | View Jobs | Stop Jobs | Stream Logs | Certificate OU |
|------------|-------------|-----------|-----------|-------------|----------------|
| **Admin**  | ✅           | ✅         | ✅         | ✅           | `OU=admin`     |
| **Viewer** | ❌           | ✅         | ❌         | ✅           | `OU=viewer`    |

### Certificate Management

```bash
# Generate certificates locally for development
make certs-local

# Generate certificates on remote server (passwordless)
make certs-remote-passwordless

# Download admin certificates (full permissions)
make certs-download-admin-simple

# Download viewer certificates (read-only)
make certs-download-viewer

# Examine certificate details and validation
make examine-certs

# Verify complete certificate chain
make verify-cert-chain

# Test TLS connection to server
make test-tls REMOTE_HOST=your-server.com
```

### Security Features

- **TLS 1.3 Minimum**: Modern encryption standards
- **Mutual Authentication**: Both client and server certificates validated
- **Certificate Authority Validation**: Custom CA for secure certificate chain
- **Role-Based Authorization**: Operation-level permission control
- **Secure Key Storage**: Proper file permissions and key management
- **Host Network Security**: Jobs use host networking but remain process-isolated

## ⚙️ Configuration

### Environment Variables

```bash
# Server Configuration
export WORKER_ADDR="0.0.0.0:50051"        # Server listening address
export WORKER_CERT_PATH="./certs"          # Certificate directory path
export WORKER_CGROUP_BASE="/sys/fs/cgroup" # Cgroups v2 base directory

# Default Resource Limits (applied when not specified)
export WORKER_DEFAULT_CPU=100      # 100% of one CPU core
export WORKER_DEFAULT_MEMORY=512   # 512 MB memory limit
export WORKER_DEFAULT_IO=0         # Unlimited I/O (0 = disabled)

# Logging Configuration
export LOG_LEVEL="INFO"  # DEBUG, INFO, WARN, ERROR

# gRPC Configuration
export MAX_RECV_MSG_SIZE=4194304       # 4MB max receive message size
export MAX_SEND_MSG_SIZE=4194304       # 4MB max send message size
export MAX_HEADER_LIST_SIZE=8192       # 8KB max header list size

# Cleanup Configuration
export CLEANUP_TIMEOUT=30              # 30 second cgroup cleanup timeout

# Network Configuration
export HOST_NETWORKING=true            # Enable host networking (default)
```

### Client Configuration

```bash
# Connect to remote server with custom address
./bin/cli --server=remote-host:50051 create echo "hello"

# Use custom certificate directory
./bin/cli --cert-path=./custom-certs create echo "hello"

# Override default server address in config
export REMOTE_HOST=prod.example.com
./bin/cli create echo "hello"
```

### Resource Limit Defaults

When resource limits are not specified (or set to 0), the system applies these defaults:

- **CPU**: 100% (one full core)
- **Memory**: 512 MB
- **I/O**: Unlimited (not constrained)
- **Network**: Host networking (shared with system)

## 📋 API Reference

### gRPC Service Definition

The `JobService` provides comprehensive job management operations:

```protobuf
service JobService {
  rpc CreateJob(CreateJobReq) returns (CreateJobRes)      // Create and start job
  rpc GetJob(GetJobReq) returns (GetJobRes)              // Get job details
  rpc StopJob(StopJobReq) returns (StopJobRes)           // Stop running job
  rpc GetJobs(EmptyRequest) returns (Jobs)               // List all jobs
  rpc GetJobsStream(GetJobsStreamReq) returns (stream DataChunk)  // Stream job logs
}
```

### Job Resource Specifications

```protobuf
message CreateJobReq {
  string command = 1;                    // Command to execute
  repeated string args = 2;              // Command arguments
  int32 maxCPU = 3;                     // CPU limit (percentage)
  int32 maxMemory = 4;                  // Memory limit (MB)
  int32 maxIOBPS = 5;                   // I/O limit (bytes/second)
  // Network: All jobs use host networking
}
```

### Job Status States

Jobs progress through these states:

- `INITIALIZING` - Job created, setting up resources
- `RUNNING` - Job actively executing
- `COMPLETED` - Job finished successfully (exit code 0)
- `FAILED` - Job finished with error (non-zero exit code)
- `STOPPED` - Job terminated by stop request

### CLI Commands Reference

```bash
# Job Creation with Resource Limits (all use host networking)
cli create [--max-cpu=N] [--max-memory=N] [--max-iobps=N] <command> [args...]

# Job Management Operations
cli get <job-id>          # Get detailed job information
cli list                  # List all jobs with status summary
cli stop <job-id>         # Stop running job (graceful then forced)
cli stream <job-id>       # Stream real-time job output

# Connection Configuration
cli --server=host:port    # Connect to specific server
cli --cert=path           # Specify client certificate path
cli --key=path            # Specify client private key path
```

## 🔧 Troubleshooting

### Common Issues & Solutions

**Certificate Issues:**

```bash
# Check certificate status and validity
make check-certs-remote REMOTE_HOST=your-server.com
make examine-certs

# Regenerate certificates if needed
make certs-local          # For local development
make certs-remote-passwordless  # For remote server

# Fix certificate permissions
make fix-cert-permissions REMOTE_HOST=your-server.com
```

**Permission Issues:**

```bash
# Test SSH connection and sudo access
make test-connection REMOTE_HOST=your-server.com

# Ensure proper sudo configuration for passwordless deployment
sudo visudo  # Add: your-user ALL=(ALL) NOPASSWD: ALL
```

**Service Issues:**

```bash
# Check comprehensive service status
make service-status REMOTE_HOST=your-server.com

# View detailed service logs
make live-log REMOTE_HOST=your-server.com

# Restart service manually
ssh user@remote "sudo systemctl restart job-worker.service"

# Check cgroups v2 availability
ls -la /sys/fs/cgroup/  # Should show cgroup v2 structure
```

**Resource Limit Issues:**

```bash
# Verify cgroups v2 support
mount | grep cgroup2
cat /proc/filesystems | grep cgroup

# Check available controllers
cat /sys/fs/cgroup/cgroup.controllers
# Should show: cpuset cpu io memory hugetlb pids rdma misc
```

**Network & TLS Issues:**

```bash
# Test TLS connection
make test-tls REMOTE_HOST=your-server.com

# Verify port accessibility
telnet your-server.com 50051

# Check certificate chain
make verify-cert-chain REMOTE_HOST=your-server.com

# Test host networking (jobs can access external services)
./bin/cli create curl -s ifconfig.me
```

## 🎯 Networking Architecture

### Host Networking Design

All jobs use the host network interface, providing:

**Benefits:**

- **Maximum Compatibility**: No network configuration required
- **Direct Access**: Jobs can bind to host ports and access external services
- **Performance**: No network overhead from namespace isolation
- **Simplicity**: Straightforward deployment and management

**Security Considerations:**

- Jobs share the host network interface
- Process isolation still maintained through other namespaces
- Port conflicts possible between jobs
- External firewall rules apply to all jobs

**Example Use Cases:**

```bash
# Web server job (binds to host port)
./bin/cli create python3 -m http.server 8080

# Database connection job
./bin/cli create mysql -h external-db.com -u user -p

# API client job
./bin/cli create curl -X POST https://api.service.com/webhook
```

## 🤝 Contributing

We welcome contributions! Here's how to get started:

### Development Workflow

1. **Fork the repository and clone**
   ```bash
   git clone https://github.com/your-username/job-worker.git
   cd job-worker
   ```

2. **Set up development environment**
   ```bash
   make setup-dev  # Builds binaries and generates certificates
   ```

3. **Create feature branch**
   ```bash
   git checkout -b feature/amazing-feature
   ```

4. **Make changes and test**
   ```bash
   # Run tests
   go test -v ./...
   
   # Test locally
   ./bin/job-worker &
   ./bin/cli --cert certs/admin-client-cert.pem --key certs/admin-client-key.pem create echo "test"
   ```

5. **Test on remote server**
   ```bash
   make deploy-safe REMOTE_HOST=test-server.com
   ```

6. **Commit and push**
   ```bash
   git commit -m 'Add amazing feature'
   git push origin feature/amazing-feature
   ```

7. **Open Pull Request**

### Code Quality Standards

- **Go Code**: Follow `gofmt` and `golint` standards
- **Testing**: Comprehensive test coverage with mocks
- **Documentation**: Update README and inline documentation
- **Security**: Certificate management and proper error handling

## 📊 Performance & Monitoring

### System Capabilities

- **Concurrent Jobs**: Hundreds of simultaneous jobs with proper isolation
- **Resource Enforcement**: Hard limits via cgroups v2
- **Real-time Streaming**: Low-latency log streaming with buffering
- **Memory Efficiency**: Minimal overhead per job
- **Network Performance**: Host networking for maximum throughput

### Monitoring Features

- **Resource Tracking**: Real-time CPU, memory, and I/O monitoring
- **Job Metrics**: Execution time, exit codes, status transitions
- **System Health**: Service status, connection monitoring
- **Log Management**: Efficient buffering and subscriber handling
- **Error Tracking**: Comprehensive error logging and recovery

### Performance Optimization

- **Process Groups**: Efficient process tree management
- **Namespace Isolation**: Minimal overhead process isolation
- **Host Networking**: Direct network access without virtualization overhead
- **Cgroup Management**: Automatic cleanup and resource reclamation
- **Streaming Optimization**: Buffered log delivery with timeout handling
- **Connection Pooling**: Efficient gRPC connection management

## 📝 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- [gRPC](https://grpc.io/) - High-performance RPC framework
- [Linux Cgroups v2](https://www.kernel.org/doc/Documentation/cgroup-v2.txt) - Resource isolation and management
- [Cobra](https://github.com/spf13/cobra) - Modern CLI framework for Go
- [Protocol Buffers](https://developers.google.com/protocol-buffers) - Language-neutral data serialization
- [Linux Namespaces](https://man7.org/linux/man-pages/man7/namespaces.7.html) - Process isolation mechanisms

---

<div align="center">
  <sub>By <a href="https://github.com/ehsaniara">Jay Ehsaniara</a></sub>
</div>