# Job Worker

[![Tests](https://github.com/ehsaniara/job-worker/actions/workflows/ci.yml/badge.svg)](https://github.com/ehsaniara/job-worker/actions/workflows/ci.yml)
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
go install github.com/ehsaniara/job-worker/cmd/worker@latest
go install github.com/ehsaniara/job-worker/cmd/cli@latest

# Or download from releases
wget https://github.com/ehsaniara/job-worker/releases/latest/download/job-worker-linux-amd64.tar.gz
tar -xzf job-worker-linux-amd64.tar.gz

# Or use the Makefile for development
git clone https://github.com/ehsaniara/job-worker.git
cd job-worker
make setup-dev  # Builds binaries + generates certificates
```

### Basic Usage

```bash
# Local development (after make setup-dev)
./bin/job-worker  # Start server locally

# Create a simple job
./bin/cli --cert certs/admin-client-cert.pem --key certs/admin-client-key.pem create echo "Hello, World!"

# Remote server usage (after make setup-remote-passwordless)
./bin/cli --server=remote-host:50051 --cert certs/client-cert.pem --key certs/client-key.pem create echo "Hello, World!"

# Check job status
./bin/cli get <job-id>

# Stream job output in real-time
./bin/cli stream <job-id>

# List all jobs
./bin/cli list

# Stop a running job
./bin/cli stop <job-id>
```

### Advanced Usage

```bash
# Create job with resource limits
./bin/cli create --max-cpu=50 --max-memory=512 --max-iobps=1000 \
  python3 heavy_script.py

# Run a complex command
./bin/cli create bash -c "for i in {1..10}; do echo 'Processing $i'; sleep 1; done"

# Remote deployment and management
make deploy-safe REMOTE_HOST=prod.example.com
make live-log    # View live service logs
make service-status  # Check service health

# Certificate management for different roles
make certs-download-admin   # Admin privileges (create, stop, view)
make certs-download-viewer  # View-only privileges
```

## 🏗️ Architecture

![high-level.svg](docs/high-level.svg)

## 📖 Documentation

- 📋 [Technical Design](docs/DESIGN.md) - Detailed system architecture
- 🔧 [API Reference](docs/API.md) - gRPC API documentation
- 🚀 [Deployment Guide](docs/DEPLOYMENT.md) - Production deployment
- 🤝 [Contributing](CONTRIBUTING.md) - Development guidelines

## 📋 Requirements

### System Requirements

- **Operating System**: Linux or macOS (cgroups v2 for Linux)
- **Go Version**: 1.23 or later
- **Privileges**: Root access on Linux (for cgroup management)
- **Dependencies**:
    - Linux: kernel 4.5+ (for cgroups v2), systemd (recommended)
    - macOS: No special requirements (limited functionality)

### TLS Certificates

The project includes automated certificate generation:

```bash
# Generate certificates locally
make certs-local

# Or generate certificates on remote server
make certs-remote-passwordless

# Download client certificates for CLI usage
make certs-download-admin-simple   # Admin role
make certs-download-viewer         # Viewer role
```

Manual certificate generation (if needed):

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

### Client Configuration

```bash
# Connect to remote server
./job-worker-cli --server=remote-host:50051 create echo "hello"

# Use custom certificates
./job-worker-cli --cert-path=./custom-certs create echo "hello"
```

## 🔐 Security & Permissions

### Role-Based Access Control

| Role       | Create Jobs | View Jobs | Stop Jobs | Stream Logs |
|------------|-------------|-----------|-----------|-------------|
| **Admin**  | ✅           | ✅         | ✅         | ✅           |
| **Viewer** | ❌           | ✅         | ❌         | ✅           |

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

# Build all binaries
make all

# Or build individual components
make cli     # Build CLI for macOS (local development)
make worker  # Build job-worker for Linux deployment
make init    # Build job-init for Linux

# Development setup (builds + generates local certificates)
make setup-dev

# Clean build artifacts
make clean
```

### Remote Deployment

The project includes automated deployment to remote Linux servers:

```bash
# Configure deployment target
export REMOTE_HOST=your-server.com
export REMOTE_USER=your-username
export REMOTE_DIR=/opt/job-worker

# Complete remote setup (certificates + deployment)
make setup-remote-passwordless

# Deploy binaries only
make deploy-safe  # With password prompt (recommended)
make deploy-passwordless  # Requires passwordless sudo

# Generate certificates on remote server
make certs-remote-passwordless

# Download client certificates for local CLI
make certs-download-admin-simple
make certs-download-viewer
```

### Certificate Management

```bash
# Generate certificates locally
make certs-local

# Check remote certificate status
make check-certs-remote

# Examine certificate details
make examine-certs
make examine-server-cert

# Verify certificate chain
make verify-cert-chain

# Test TLS connection
make test-tls
```

### Monitoring & Debugging

```bash
# Check service status
make service-status

# View live logs
make live-log

# Test SSH connection
make test-connection
```

### Project Structure

```
job-worker/
├── cmd/
│   ├── worker/         # Main server binary (Linux)
│   ├── cli/           # CLI client binary (cross-platform)
│   └── job-init/      # Job initialization binary (Linux)
├── internal/
│   ├── worker/        # Core business logic
│   ├── server/        # gRPC server implementation
│   └── config/        # Configuration management
├── api/
│   └── gen/          # Generated protobuf code
├── pkg/              # Shared packages
├── etc/              # Scripts and configuration
│   └── certs_gen.sh  # Certificate generation script
├── bin/              # Built binaries (created by make)
├── certs/            # TLS certificates (created by make)
├── Makefile          # Build and deployment automation
└── test/             # Integration tests
```

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

### Development Workflow

```bash
# Complete development setup
make setup-dev

# Build individual components
make cli     # macOS CLI for local development
make worker  # Linux server binary
make init    # Linux job-init binary

# Run tests
go test -v ./...

# Certificate management
make examine-certs      # Examine certificate details
make verify-cert-chain  # Verify certificate validity
make test-tls          # Test TLS connection

# Remote deployment
make deploy-safe REMOTE_HOST=your-server.com
make live-log          # Monitor service logs
make service-status    # Check service health

# Generate protobuf code (if needed)
make generate
```

## 📝 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- [gRPC](https://grpc.io/) for high-performance RPC framework
- [Linux Cgroups](https://www.kernel.org/doc/Documentation/cgroup-v2.txt) for resource isolation
- [Cobra](https://github.com/spf13/cobra) for CLI framework

<div align="center">
  <sub>by <a href="https://github.com/ehsaniara">Jay Ehsaniara</a></sub>
</div>