# Worker

[![Tests](https://github.com/ehsaniara/job-worker/actions/workflows/ci.yml/badge.svg)](https://github.com/ehsaniara/job-worker/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/ehsaniara/job-worker)](https://goreportcard.com/report/github.com/ehsaniara/job-worker)
[![Go Version](https://img.shields.io/github/go-mod/go-version/ehsaniara/job-worker)](https://golang.org/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Release](https://img.shields.io/github/release/ehsaniara/job-worker.svg)](https://github.com/ehsaniara/job-worker/releases/latest)

![worker-thum.png](docs/worker-thum.png)

**Worker** is a secure job execution platform designed to **isolate system calls from clients**, providing safe remote
command execution on Linux systems. By creating complete process isolation barriers, Worker prevents clients from
directly accessing the host system while enabling controlled job execution with comprehensive resource management and
real-time monitoring.

## ✨ Key Features

### 🛡️ **System Call Isolation**

- **Complete Process Isolation**: Prevents direct client access to host system calls
- **Linux Namespace Barriers**: PID, mount, IPC, and UTS isolation per job
- **Controlled Execution Environment**: Jobs run in isolated containers with limited system access
- **Security by Design**: Clients cannot directly manipulate host system resources

### 🚀 **Safe Remote Execution**

- Execute commands remotely without exposing host system calls
- Real-time output streaming with live log monitoring
- Comprehensive job lifecycle management (start, stop, monitor)
- Support for long-running services and short-lived tasks

### 🛡️ **Enterprise Security**

- **Mutual TLS Authentication**: Certificate-based secure connections
- **Role-Based Access Control**: Admin and Viewer roles with granular permissions
- **Process Isolation**: Complete job isolation using Linux namespaces
- **Secure by Default**: All communications encrypted, no plaintext protocols

### ⚡ **Resource Management**

- **CPU Limiting**: Control CPU usage per job (percentage-based)
- **Memory Control**: Set memory limits in MB with hard enforcement
- **I/O Bandwidth**: Limit disk I/O operations per second
- **Host Networking**: Jobs share host network for maximum compatibility

### 🖥️ **Platform Support**

- **Linux Server**: Full job isolation and resource management (production)
- **macOS/Darwin Client**: CLI client support for remote job management

### 📊 **Real-Time Monitoring**

- Live job status tracking and updates
- Streaming log output with buffering
- Job history and execution statistics
- Resource usage monitoring and alerts

## 🔍 System Call Isolation in Action

Here's a clear demonstration of how Worker isolates system calls and process visibility:

### **Isolated Job Execution**

```bash
$ worker-cli run ps aux
Job started:
ID: 120
Command: ps aux
Status: RUNNING
StartTime: 2025-06-30T17:34:33Z
Network: host (shared with system)

$ worker-cli log 120   
USER         PID %CPU %MEM    VSZ   RSS TTY      STAT START   TIME COMMAND
root           1  0.0  0.0  10044  1580 ?        R    17:34   0:00 /usr/bin/ps aux
```

### **Direct Host Execution (for comparison)**

```bash
$ ps aux
USER         PID %CPU %MEM    VSZ   RSS TTY      STAT START   TIME COMMAND
root           1  0.0  0.3 167744 13132 ?        Ss   Jun26   0:11 /sbin/init
root           2  0.0  0.0      0     0 ?        S    Jun26   0:00 [kthreadd]
root           3  0.0  0.0      0     0 ?        I<   Jun26   0:00 [rcu_gp]
root           4  0.0  0.0      0     0 ?        I<   Jun26   0:00 [rcu_par_gp]
root           5  0.0  0.0      0     0 ?        I<   Jun26   0:00 [slub_flushwq]
root           6  0.0  0.0      0     0 ?        I<   Jun26   0:00 [netns]
...
worker    544936  0.0  0.0  10072  1608 pts/2    R+   17:37   0:00 ps aux
```

**Key Difference**: The job sees only its own process (PID 1) in an isolated namespace, while the host system sees all
processes. This demonstrates how Worker prevents jobs from accessing or discovering host system information through
system calls.

## 🚀 Quick Start

### Installation

**Linux Server (Required for job execution):**

```bash
# Download the latest release
wget https://github.com/ehsaniara/worker/releases/latest/download/worker_1.0.0_amd64.deb

# Install the package
sudo dpkg -i worker_1.0.0_amd64.deb

# Start the service
sudo systemctl start worker
sudo systemctl enable worker
```

**macOS/Darwin Client (CLI only):**

```bash
# Download CLI binary for macOS
wget https://github.com/ehsaniara/worker/releases/latest/download/worker-cli-darwin-amd64.tar.gz
tar -xzf worker-cli-darwin-amd64.tar.gz
chmod +x worker-cli
sudo mv worker-cli /usr/local/bin/

# Connect to remote Linux server
worker-cli --server your-linux-server.com:50051 run echo "Hello from macOS!"
```

### Initial Setup

```bash
# Generate SSL certificates (one-time setup)
sudo /usr/local/bin/certs_gen.sh

# Check service status
sudo systemctl status worker

# Copy client certificates for CLI access
mkdir -p ~/.worker
sudo cp /opt/worker/certs/admin-client-cert.pem ~/.worker/client-cert.pem
sudo cp /opt/worker/certs/admin-client-key.pem ~/.worker/client-key.pem
sudo cp /opt/worker/certs/ca-cert.pem ~/.worker/ca-cert.pem
```

### First Job

```bash
# Run your first job
worker-cli --server localhost:50051 run echo "Hello, Worker!"

# Check job status
worker-cli list

# Stream job logs
worker-cli log <job-id>
```

## 💻 Usage Examples

### Basic Commands

```bash
# Simple command execution
worker-cli run echo "Hello World"
worker-cli run ls -la /tmp
worker-cli run python3 --version

# Script execution
worker-cli run bash -c "for i in {1..5}; do echo 'Step $i'; sleep 1; done"
worker-cli run python3 script.py --input data.csv

# Package installation and usage
worker-cli run apt update
worker-cli run pip install requests
```

### Resource-Limited Jobs

```bash
# Limit CPU to 50% of one core
worker-cli run --max-cpu=50 stress --cpu 1 --timeout 60s

# Limit memory to 512MB
worker-cli run --max-memory=512 python3 memory_intensive_script.py

# Limit I/O bandwidth
worker-cli run --max-iobps=1000000 dd if=/dev/zero of=/tmp/test bs=1M count=100

# Combined limits
worker-cli run --max-cpu=25 --max-memory=256 --max-iobps=500000 heavy_task.sh
```

### Long-Running Services

```bash
# Web server (accessible on host network)
worker-cli run --max-memory=1024 python3 -m http.server 8080

# Database service
worker-cli run --max-cpu=100 --max-memory=2048 mysqld --datadir=/var/lib/mysql

# Background processing
worker-cli run --max-cpu=50 celery worker -A myapp.celery
```

### Job Management

```bash
# List all jobs with status
worker-cli list

# Get detailed job information
worker-cli status <job-id>

# Stop a running job
worker-cli stop <job-id>

# Stream live job output
worker-cli log -f <job-id>

# Monitor job from start
worker-cli run python3 long_script.py && worker-cli log -f <job-id>
```

## 🔧 Configuration

### Server Configuration

Edit `/opt/worker/config.yml` to customize server behavior:

```yaml
server:
  address: "0.0.0.0"    # Listen on all interfaces
  port: 50051           # gRPC port

worker:
  defaultCpuLimit: 100        # Default CPU limit (%)
  defaultMemoryLimit: 512     # Default memory limit (MB)
  maxConcurrentJobs: 100      # Max simultaneous jobs
  jobTimeout: "1h"            # Maximum job runtime

security:
  minTlsVersion: "1.3"        # Minimum TLS version

cgroup:
  enableControllers: [ "cpu", "memory", "io", "pids" ]

logging:
  level: "INFO"               # DEBUG, INFO, WARN, ERROR
  format: "text"              # text or json
```

### Client Configuration

Create `~/.worker/config` for CLI defaults:

```bash
# Default server connection
export WORKER_SERVER="your-server.com:50051"
export WORKER_CERT_PATH="~/.worker/client-cert.pem"
export WORKER_KEY_PATH="~/.worker/client-key.pem"
export WORKER_CA_PATH="~/.worker/ca-cert.pem"
```

## 🔐 Security & Access Control

### Certificate-Based Authentication

Worker uses mutual TLS with role-based access:

| Role   | Permissions                     | Certificate OU |
|--------|---------------------------------|----------------|
| Admin  | Full access to all operations   | `OU=admin`     |
| Viewer | Read-only access (status, logs) | `OU=viewer`    |

### Setting Up Access Roles

```bash
# Generate certificates with specific roles
sudo /usr/local/bin/certs_gen.sh  # Creates both admin and viewer certs

# Setup admin access
cp /opt/worker/certs/admin-client-cert.pem ~/.worker/admin-cert.pem
cp /opt/worker/certs/admin-client-key.pem ~/.worker/admin-key.pem

# Setup viewer access
cp /opt/worker/certs/viewer-client-cert.pem ~/.worker/viewer-cert.pem
cp /opt/worker/certs/viewer-client-key.pem ~/.worker/viewer-key.pem

# Use specific role
worker-cli --cert ~/.worker/admin-cert.pem --key ~/.worker/admin-key.pem run command
worker-cli --cert ~/.worker/viewer-cert.pem --key ~/.worker/viewer-key.pem list
```

### Job Isolation

Every job runs in an isolated environment that **prevents direct system call access**:

- **Separate Process Tree**: Jobs can't see or interact with host processes
- **Isolated Filesystem**: Jobs have restricted view of host filesystem
- **System Call Filtering**: Jobs cannot directly execute dangerous system calls
- **Resource Boundaries**: CPU, memory, and I/O are strictly controlled
- **Network Isolation**: Jobs use host network but cannot access host networking stack directly

## 📊 Monitoring & Troubleshooting

### Service Health

```bash
# Check service status
sudo systemctl status worker

# View service logs
sudo journalctl -u worker -f

# Check listening ports
sudo netstat -tlnp | grep :50051

# Verify certificates
openssl x509 -in /opt/worker/certs/server-cert.pem -noout -text
```

### Job Monitoring

```bash
# Real-time job monitoring
worker-cli list                    # Show all jobs
worker-cli status <job-id>         # Detailed job info
worker-cli log -f <job-id>        # Follow job output

# Resource usage
worker-cli status <job-id> | grep -E "(CPU|Memory|Status)"
```

### Common Issues

**Connection Refused**

```bash
# Check if service is running
sudo systemctl status worker

# Verify port is open
sudo netstat -tlnp | grep :50051

# Check firewall
sudo ufw status
```

**Certificate Errors**

```bash
# Regenerate certificates
sudo /usr/local/bin/certs_gen.sh

# Verify certificate permissions
ls -la ~/.worker/
```

**Job Failures**

```bash
# Check job logs
worker-cli log <job-id>

# Check system resources
df -h                    # Disk space
free -h                  # Memory usage
sudo systemctl status worker
```

## 🌐 Remote Access

### Connecting from Different Platforms

**Linux to Linux:**

```bash
# Connect to remote Worker instance
worker-cli --server remote-host.com:50051 \
          --cert ~/.worker/client-cert.pem \
          --key ~/.worker/client-key.pem \
          run echo "Remote execution"
```

**macOS to Linux Server:**

```bash
# macOS client connecting to Linux server
worker-cli --server linux-server.com:50051 \
          --cert ~/.worker/client-cert.pem \
          --key ~/.worker/client-key.pem \
          run python3 script.py

# Note: Job execution happens on the Linux server, not locally on macOS
```

**Windows Client (via WSL):**

```bash
# Install Worker CLI in WSL2 Ubuntu
wget https://github.com/ehsaniara/worker/releases/latest/download/worker-cli-linux-amd64.tar.gz
tar -xzf worker-cli-linux-amd64.tar.gz
chmod +x worker-cli
sudo mv worker-cli /usr/local/bin/
```

### Permanent Remote Setup

```bash
# Set up remote access permanently (any client platform)
echo 'export WORKER_SERVER="remote-host.com:50051"' >> ~/.bashrc
echo 'export WORKER_CERT_PATH="~/.worker/client-cert.pem"' >> ~/.bashrc
echo 'export WORKER_KEY_PATH="~/.worker/client-key.pem"' >> ~/.bashrc
```

### Firewall Configuration

```bash
# Open Worker port
sudo ufw allow 50051/tcp

# For specific IP ranges
sudo ufw allow from 192.168.1.0/24 to any port 50051
```

## 📋 CLI Reference

### Command Syntax

```bash
worker-cli [global-options] <command> [command-options] [arguments]
```

### Global Options

```bash
--server=HOST:PORT          # Worker server address
--cert=PATH                 # Client certificate file
--key=PATH                  # Client private key file
--ca=PATH                   # CA certificate file
```

### Commands

```bash
# Job Management
run [options] <command> [args...]    # Execute a new job
status <job-id>                      # Get job status and details
stop <job-id>                        # Stop a running job
list                                 # List all jobs
log [-f] <job-id>                   # Show/follow job logs

# Run Options
--max-cpu=N                          # CPU limit (percentage)
--max-memory=N                       # Memory limit (MB)
--max-iobps=N                        # I/O bandwidth limit (bytes/sec)
```

## 💡 Use Cases

### **Why System Call Isolation Matters**

```bash
# ❌ Without Worker: Direct system access (dangerous)
$ malicious_script.py
# Can see all processes, access sensitive files, modify system

# ✅ With Worker: Isolated execution (safe)
$ worker-cli run malicious_script.py
# Sees only its own process, limited file access, contained environment
```

**Security Benefits:**

- **Process Discovery Prevention**: Jobs cannot enumerate host processes
- **File System Protection**: Limited access to host filesystem
- **System Information Hiding**: Host system details are not exposed
- **Resource Containment**: Jobs cannot exhaust host resources
- **Multi-Tenant Safety**: Jobs cannot interfere with each other

### CI/CD Pipelines

```bash
# Isolated build and test environment
worker-cli run git clone https://github.com/user/repo.git /tmp/build
worker-cli run --max-cpu=200 --max-memory=4096 make -C /tmp/build test
worker-cli run docker build -t myapp /tmp/build  # Isolated from host Docker daemon
```

### Multi-Tenant Environments

```bash
# Each tenant's jobs are isolated from each other and the host
worker-cli run --max-memory=1024 tenant_a_workload.py
worker-cli run --max-cpu=50 tenant_b_processing.sh
```

### Untrusted Code Execution

```bash
# Safe execution of user-provided code
worker-cli run --max-memory=512 --max-cpu=25 user_script.py
worker-cli run --max-iobps=1000000 data_analysis.r
```

## Support

- **Documentation**: [docs/](docs/) directory
- **Issues**: [GitHub Issues](https://github.com/ehsaniara/worker/issues)

## 📝 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

<div align="center">
  <sub>by <a href="https://github.com/ehsaniara">Jay Ehsaniara</a></sub>
</div>