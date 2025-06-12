# Job Worker

[![Tests](https://github.com/ehsaniara/job-worker/actions/workflows/test.yml/badge.svg)](https://github.com/ehsaniara/job-worker/actions/workflows/test.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/ehsaniara/job-worker)](https://goreportcard.com/report/github.com/ehsaniara/job-worker)
[![Go Version](https://img.shields.io/github/go-mod/go-version/ehsaniara/job-worker)](https://golang.org/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Release](https://img.shields.io/github/release/ehsaniara/job-worker.svg)](https://github.com/ehsaniara/job-worker/releases/latest)

> A distributed job execution system with secure remote execution, resource management, and real-time streaming.

## ✨ Features

- 🔒 **Secure Remote Execution** - Mutual TLS authentication with role-based authorization
- 🎛️ **Resource Management** - CPU, memory, and I/O bandwidth limiting via Linux cgroups v2
- 📡 **Real-time Streaming** - Live output streaming of job execution logs
- 🔐 **Process Isolation** - Containerized execution environment with proper cleanup
- 🌐 **gRPC API** - High-performance API with protobuf serialization
- ⚡ **Two-Stage Execution** - job-init binary ensures proper resource isolation

## 🚀 Quick Start

### Installation

```bash

# Install from source
go install github.com/ehsaniara/job-worker/cmd/job-worker@latest
go install github.com/ehsaniara/job-worker/cmd/job-worker-cli@latest

# Or download from releases
wget https://github.com/ehsaniara/job-worker/releases/latest/download/job-worker-linux-amd64.tar.gz
tar -xzf job-worker-linux-amd64.tar.gz
```

### Basic Usage

```bash

# 1. Start the server (requires root for cgroups)
sudo ./job-worker

# 2. Create a simple job
./cli create echo "Hello, World!"
# Output: Job created: ID: 1

# 3. Check job status
./cli get 1

# 4. Stream job output in real-time
./cli stream 1

# 5. List all jobs
./cli list

# 6. Stop a running job
./cli stop 1
```

### Advanced Usage

```bash

# Create job with resource limits
./cli create --max-cpu=50 --max-memory=512 --max-iobps=1000 \
  python3 heavy_script.py

# Run a complex command
./cli create bash -c "for i in {1..10}; do echo 'Processing $i'; sleep 1; done"

# Long-running job with streaming
./cli create ping -c 100 google.com
./cli stream <job-id>
```

## 🏗️ Architecture

![high-level.svg](docs/high-level.svg)

## 📋 Requirements

### System Requirements

- **Operating System**: Linux with cgroups v2 support
- **Go Version**: 1.21 or later
- **Privileges**: Root access (for cgroup management)
- **Dependencies**:
  - Linux kernel 4.5+ (for cgroups v2)
  - systemd (recommended)

### TLS Certificates

Generate certificates for secure communication:

```bash

# Create certificate directory
mkdir certs

# Generate CA certificate
openssl genrsa -out certs/ca-key.pem 4096
openssl req -new -x509 -key certs/ca-key.pem -sha256 \
  -subj "/C=US/ST=CA/O=JobWorker/CN=JobWorker CA" \
  -days 3650 -out certs/ca-cert.pem

# Generate server certificate
openssl genrsa -out certs/server-key.pem 4096
openssl req -subj "/C=US/ST=CA/O=JobWorker/CN=localhost" \
  -sha256 -new -key certs/server-key.pem -out certs/server.csr
openssl x509 -req -in certs/server.csr -CA certs/ca-cert.pem \
  -CAkey certs/ca-key.pem -out certs/server-cert.pem -days 365 -sha256

# Generate admin client certificate
openssl genrsa -out certs/client-key.pem 4096
openssl req -subj "/C=US/ST=CA/O=JobWorker/OU=admin/CN=admin-client" \
  -new -key certs/client-key.pem -out certs/client.csr
openssl x509 -req -in certs/client.csr -CA certs/ca-cert.pem \
  -CAkey certs/ca-key.pem -out certs/client-cert.pem -days 365 -sha256
```

## 🔧 Configuration

### Server Configuration

```bash

# Environment variables
export JOB_WORKER_ADDR="0.0.0.0:50051"
export JOB_WORKER_CERT_PATH="./certs"
export JOB_WORKER_CGROUP_BASE="/sys/fs/cgroup"

# Default resource limits
export JOB_WORKER_DEFAULT_CPU=100      # 100% of one core
export JOB_WORKER_DEFAULT_MEMORY=512   # 512 MB
export JOB_WORKER_DEFAULT_IO=0         # Unlimited
```

## 🔐 Security & Permissions

### Role-Based Access Control

| Role | Create Jobs | View Jobs | Stop Jobs | Stream Logs |
|------|-------------|-----------|-----------|-------------|
| **Admin** | ✅ | ✅ | ✅ | ✅ |
| **Viewer** | ❌ | ✅ | ❌ | ✅ |

### Certificate-Based Authentication

Roles are determined by the `OU` (Organizational Unit) field in client certificates:
- `OU=admin` → Admin privileges
- `OU=viewer` → Read-only access

## 📊 Job Lifecycle

```
INITIALIZING → RUNNING → COMPLETED/FAILED/STOPPED
     ↓            ↓            ↓
  • Cgroup     • Process    • Cleanup
  • Setup      • Monitor    • Resources
  • Limits     • Stream     • Final State
```

### Job States

- **INITIALIZING**: Job created, setting up cgroup and starting process
- **RUNNING**: Process executing, logs streaming available
- **COMPLETED**: Process finished successfully (exit code 0)
- **FAILED**: Process finished with error (exit code ≠ 0)
- **STOPPED**: Process terminated by user request

## 🛠️ Development

### Building from Source

```bash

# Clone the repository
git clone https://github.com/ehsaniara/job-worker.git
cd job-worker

# Install dependencies
go mod download

# Build all binaries
make build

# Run tests
make test

# Run with race detection
make test-race
```

### Project Structure

```
job-worker/
├── cmd/
│   ├── server/         # Main server binary
│   ├── cli/           # CLI client binary
│   └── job-init/      # Job initialization binary
├── internal/
│   ├── worker/        # Core business logic
│   ├── server/        # gRPC server implementation
│   └── config/        # Configuration management
├── api/
│   └── gen/          # Generated protobuf code
└── pkg/              # Shared packages
```

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

### Development Workflow

```bash

# Setup development environment
make dev-setup

# Run linting
make lint

# Run all tests
make test-all

# Generate protobuf code
make generate
```

## 📝 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- [gRPC](https://grpc.io/) for high-performance RPC framework
- [Linux Cgroups](https://www.kernel.org/doc/Documentation/cgroup-v2.txt) for resource isolation
- [Cobra](https://github.com/spf13/cobra) for CLI framework

---

<div align="center">
  <sub>by <a href="https://github.com/ehsaniara">Jay Ehsaniara</a></sub>
</div>