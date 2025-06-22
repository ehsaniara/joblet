# Worker

[![Tests](https://github.com/ehsaniara/worker/actions/workflows/ci.yml/badge.svg)](https://github.com/ehsaniara/worker/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/ehsaniara/worker)](https://goreportcard.com/report/github.com/ehsaniara/worker)
[![Go Version](https://img.shields.io/github/go-mod/go-version/ehsaniara/worker)](https://golang.org/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Release](https://img.shields.io/github/release/ehsaniara/worker.svg)](https://github.com/ehsaniara/worker/releases/latest)

A secure distributed job execution system that lets you run commands on remote Linux servers with resource limits and
real-time monitoring.

![worker-thum.png](docs/worker-thum.png)

## Why Worker vs Docker/Kubernetes?

### Performance Advantages

- **⚡ Faster Startup**: Sub-second cold starts vs Docker's seconds
- **🪶 Lower Resource Overhead**: No container runtime tax or image layers
- **🎯 Direct Process Execution**: Native Linux processes, not containerized abstractions
- **🌐 Host Networking**: Maximum network performance without virtual networking overhead

### Operational Simplicity

- **📦 No Images Required**: Execute commands directly without building/managing container images
- **🔧 Minimal Dependencies**: Just the Worker binary and certificates - no Docker daemon or Kubernetes cluster
- **⚡ Instant Deployment**: No image pulls, registries, or orchestration complexity
- **💡 Simple Debugging**: Direct process access, standard Linux tooling works immediately

### Resource Efficiency

- **💾 Memory Efficient**: Process-level isolation without container memory overhead
- **🏎️ CPU Efficiency**: No virtualization layer between your code and the CPU
- **📊 Precise Control**: Direct cgroups v2 integration for exact resource limits
- **🧹 Automatic Cleanup**: No orphaned containers or dangling images

### Security Benefits

- **🔒 Process-Native Isolation**: Linux namespaces without container attack surface
- **🛡️ Mutual TLS**: Built-in secure authentication, no separate network policies needed
- **🎭 Role-Based Access**: Simple admin/viewer permissions vs complex RBAC configurations
- **🔐 Certificate Management**: Straightforward PKI vs Kubernetes secret management
- **🔑 Fine-Grained Authorization**: Operation-level permissions with certificate-based roles

### Authorization Layer

Worker implements a sophisticated role-based authorization system that's simpler than Kubernetes RBAC:

| Role       | Run Jobs | View Jobs | Stop Jobs | Stream Logs | Certificate OU |
|------------|----------|-----------|-----------|-------------|----------------|
| **Admin**  | ✅        | ✅         | ✅         | ✅           | `OU=admin`     |
| **Viewer** | ❌        | ✅         | ❌         | ✅           | `OU=viewer`    |

**Authorization Features:**

- **Certificate-Based Roles**: Roles embedded in X.509 certificate Organizational Unit (OU) field
- **Operation-Level Control**: Each API operation checked against user role
- **Automatic Role Detection**: System automatically extracts role from client certificates
- **Case-Insensitive**: Role matching works regardless of case (ADMIN, admin, Admin)
- **Secure by Default**: Unknown roles are denied all operations
- **No External Dependencies**: No need for external identity providers or complex policy engines

**vs Kubernetes RBAC:**

- **Simpler Setup**: 2 roles vs dozens of K8s roles/bindings
- **Certificate Integration**: Built into TLS authentication vs separate service accounts
- **No YAML Configuration**: Role permissions are code-defined, not configuration-managed
- **Immediate Effect**: No need to restart services or wait for policy propagation

### Use Case Advantages

| Scenario                 | Worker                     | Docker/K8s                             |
|--------------------------|----------------------------|----------------------------------------|
| **CI/CD Builds**         | Instant job start          | Image build + container start overhead |
| **Serverless Functions** | Sub-second cold starts     | Multi-second container initialization  |
| **Batch Processing**     | Direct resource allocation | Container scheduling overhead          |
| **Development Tools**    | Live code execution        | Build/deploy/test cycle                |

## Features

- **Secure**: Mutual TLS authentication with certificate-based access
- **Resource Control**: Set CPU, memory, and I/O limits for jobs
- **Real-time Monitoring**: Stream live output from running jobs
- **Process Isolation**: Jobs run in isolated environments
- **Easy to Use**: Simple CLI interface

## Quick Start

### Prerequisites

- Linux system with cgroups v2 support
- Go 1.19+ (for building from source)
- Root/sudo access for initial setup

### Installation

```bash
# Clone the repository
git clone https://github.com/ehsaniara/worker.git
cd worker

# Build the binaries
make build

# Generate certificates (for local testing)
make certs-local

# Start the worker service
sudo ./bin/worker
```

### Basic Usage

```bash
# Run a simple command
./bin/cli run echo "Hello World"

# Run a command with resource limits
./bin/cli run --max-cpu=50 --max-memory=512 python3 script.py

# List all jobs
./bin/cli list

# View job status
./bin/cli status <job-id>

# Stream live output
./bin/cli log <job-id>

# Stop a running job
./bin/cli stop <job-id>
```

## Authorization & Security

### Certificate-Based Authentication

Worker uses mutual TLS with role-based authorization built into X.509 certificates:

```bash
# Generate admin certificates (full access)
make certs-download-admin-simple REMOTE_HOST=your-server.com

# Generate viewer certificates (read-only)  
make certs-download-viewer REMOTE_HOST=your-server.com

# Use admin certificate
./bin/cli --cert certs/admin-client-cert.pem --key certs/admin-client-key.pem run echo "Admin job"

# Use viewer certificate (this would fail)
./bin/cli --cert certs/viewer-client-cert.pem --key certs/viewer-client-key.pem run echo "Denied"
```

### Role Permissions

| Operation           | Admin | Viewer | Certificate OU      |
|---------------------|-------|--------|---------------------|
| **Run Jobs**        | ✅     | ❌      | `OU=admin` required |
| **View Job Status** | ✅     | ✅      | Any valid role      |
| **List Jobs**       | ✅     | ✅      | Any valid role      |
| **Stop Jobs**       | ✅     | ❌      | `OU=admin` required |
| **Stream Logs**     | ✅     | ✅      | Any valid role      |

### Security Features

- **Mutual TLS**: Both client and server authenticate with certificates
- **Role Extraction**: Automatic role detection from certificate OU field
- **Operation Validation**: Each API call checked against user permissions
- **Secure by Default**: Unknown roles denied all access
- **No External Dependencies**: Self-contained authentication system

## Configuration

### Resource Limits

- `--max-cpu`: CPU limit as percentage (e.g., 50 = 50% of one core)
- `--max-memory`: Memory limit in MB
- `--max-iobps`: I/O bandwidth limit in bytes per second

### Certificates

For production use, generate proper certificates:

```bash
# Generate certificates for remote deployment
make certs-remote REMOTE_HOST=your-server.com
```

## Remote Deployment

```bash
# Deploy to remote server
make deploy-safe REMOTE_HOST=your-server.com REMOTE_USER=admin

# Check service status
make service-status REMOTE_HOST=your-server.com

# View logs
make live-log REMOTE_HOST=your-server.com
```

## Architecture

The system consists of three main components:

- **Worker Service**: Main server that executes jobs
- **CLI Client**: Command-line interface for job management
- **Job-Init**: Handles process isolation and resource setup

Jobs run in isolated environments but use host networking for compatibility.

## API

The system provides a gRPC API with the following main operations:

- `Run` - Execute a new job
- `Status` - Get job information
- `List` - List all jobs
- `Stop` - Terminate a job
- `Log` - Stream job output

## Troubleshooting

### Common Issues

**Permission errors**: Ensure the worker service has proper sudo privileges

**Certificate issues**: Regenerate certificates with `make certs-local` or `make certs-remote`

**Connection refused**: Check if the worker service is running with `systemctl status worker`

### Getting Help

1. Check the service logs: `journalctl -u worker -f`
2. Verify certificates: `make check-certs-remote REMOTE_HOST=your-server`
3. Test connection: `make test-connection REMOTE_HOST=your-server`
4. Test authorization: Use viewer cert and try to run a job (should fail with permission denied)

### Authorization Issues

**Permission Denied Errors:**

```bash
# Check certificate role
openssl x509 -in certs/client-cert.pem -noout -text | grep "Organizational Unit"

# Verify you're using the right certificate
./bin/cli --cert certs/admin-client-cert.pem --key certs/admin-client-key.pem run echo "test"
```

**Certificate Problems:**

```bash
# Regenerate admin certificates
make certs-download-admin-simple REMOTE_HOST=your-server.com

# Test TLS connection
make test-tls REMOTE_HOST=your-server.com

# Verify certificate chain
make verify-cert-chain REMOTE_HOST=your-server.com
```

## Contributing

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/amazing-feature`
3. Make your changes and test them
4. Submit a pull request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- [gRPC](https://grpc.io/) - High-performance RPC framework
- [Linux Cgroups v2](https://www.kernel.org/doc/Documentation/cgroup-v2.txt) - Resource management
- [Cobra](https://github.com/spf13/cobra) - CLI framework for Go