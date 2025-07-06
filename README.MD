# Worker - Secure Job Execution Platform

[![Tests](https://github.com/ehsaniara/job-worker/actions/workflows/ci.yml/badge.svg)](https://github.com/ehsaniara/job-worker/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/ehsaniara/job-worker)](https://goreportcard.com/report/github.com/ehsaniara/job-worker)
[![Go Version](https://img.shields.io/github/go-mod/go-version/ehsaniara/job-worker)](https://golang.org/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Release](https://img.shields.io/github/release/ehsaniara/job-worker.svg)](https://github.com/ehsaniara/job-worker/releases/latest)

<img align="right" width="159px" src="docs/worker-thum.png">

**Worker** provides **secure, isolated job execution** on Linux systems. Run commands remotely with complete process isolation, resource limits, and real-time monitoring - protecting your host system from malicious or resource-hungry processes.

## 🎯 Why Use Worker?

### The Problem: Unsafe Remote Execution
```bash
# DANGEROUS: Running commands directly on your server
ssh production-server "python3 untrusted-script.py"
# ❌ Can see all processes, access sensitive files, consume unlimited resources
# ❌ Can interfere with other applications
# ❌ No resource controls or monitoring
```

### The Solution: Isolated Job Execution
```bash
# SAFE: Running commands through Worker
worker-cli --server=production-server:50051 run --max-memory=512 python3 untrusted-script.py
# ✅ Process isolation - can only see its own processes
# ✅ Filesystem isolation - limited access to host files  
# ✅ Resource limits - memory, CPU, I/O controls
# ✅ Real-time monitoring and logs
```

## 🔧 How It Works

Worker consists of two components:

1. **Worker Daemon** - Runs on Linux servers, executes jobs in isolated environments
2. **Worker CLI** - Connects from anywhere (Linux/macOS/Windows) to manage jobs

```
┌─────────────────┐    gRPC/mTLS     ┌───────────────────┐
│   Worker CLI    │ ───────────────► │  Worker Daemon    │
│                 │                  │                   │
│ • Linux/macOS   │                  │ • Linux only      │
│ • Windows       │                  │ • Systemd service │
│ • Remote client │                  │ • Job isolation   │
└─────────────────┘                  └───────────────────┘
```

### Security & Isolation Features

- **Process Isolation**: Jobs run in separate PID namespaces
- **Filesystem Isolation**: Chroot environments with limited host access
- **Resource Limits**: CPU, memory, and I/O bandwidth controls
- **Network Security**: mTLS encryption with certificate-based authentication
- **Role-Based Access**: Admin (full control) and Viewer (read-only) roles

## 📦 Installation

### Quick Start - Debian Package (Recommended)

Install both daemon and CLI with embedded certificates:

```bash
# Download latest release
wget $(curl -s https://api.github.com/repos/ehsaniara/worker/releases/latest | grep "browser_download_url.*_amd64\.deb" | cut -d '"' -f 4)

# Interactive installation (prompts for server IP)
sudo dpkg -i worker_*_amd64.deb

# Or automated installation
WORKER_SERVER_IP="192.168.1.100" sudo -E dpkg -i worker_*_amd64.deb

# Start the daemon
sudo systemctl start worker
sudo systemctl enable worker

# Test locally
worker-cli list
```

### Manual Installation

#### 1. Build from Source

```bash
git clone https://github.com/ehsaniara/worker.git
cd worker

# Build binaries
make all  # Creates: bin/worker, bin/cli

# Or download pre-built binaries from releases
```

#### 2. Install Worker Daemon (Linux Server)

```bash
# Install daemon binary
sudo mkdir -p /opt/worker/config /opt/worker/scripts
sudo cp bin/worker /opt/worker/
sudo chmod +x /opt/worker/worker

# Copy configuration templates
sudo cp scripts/server-config-template.yml /opt/worker/scripts/
sudo cp scripts/worker.service /etc/systemd/system/

# Generate certificates with embedded configuration
export WORKER_SERVER_ADDRESS="192.168.1.100"  # Your server IP
sudo ./scripts/certs_gen_embedded.sh

# Start daemon service
sudo systemctl daemon-reload
sudo systemctl enable worker
sudo systemctl start worker
```

#### 3. Install Worker CLI (Client Machine)

##### Linux/macOS CLI Setup
```bash
# Install CLI binary
sudo cp bin/cli /usr/local/bin/worker-cli
sudo chmod +x /usr/local/bin/worker-cli

# Download client configuration from daemon server
mkdir -p ~/.worker
scp your-server:/opt/worker/config/client-config.yml ~/.worker/

# Test connection
worker-cli --config ~/.worker/client-config.yml list
```

##### Windows CLI Setup
```powershell
# Download Windows CLI from releases
Invoke-WebRequest -Uri "https://github.com/ehsaniara/worker/releases/latest/download/worker-cli-windows-amd64.exe" -OutFile "worker-cli.exe"

# Copy to PATH
Move-Item worker-cli.exe "C:\Program Files\worker-cli.exe"

# Copy client config from daemon server to:
# %USERPROFILE%\.worker\client-config.yml
```

## 🚀 Usage

### Basic Job Management

```bash
# Run simple commands
worker-cli run echo "Hello World"
worker-cli run ps aux
worker-cli run python3 --version

# Run with resource limits
worker-cli run --max-cpu=50 --max-memory=256 python3 script.py
worker-cli run --max-iobps=5242880 dd if=/dev/zero of=/tmp/test bs=1M count=100

# Complex commands
worker-cli run bash -c "curl -s https://api.github.com/users/octocat | jq '.name'"
```

### Job Monitoring

```bash
# List all jobs
worker-cli list

# Get job details
worker-cli status <job-id>

# Stream live logs
worker-cli log <job-id>

# Stop running job
worker-cli stop <job-id>
```

### Remote Server Connection

```bash
# Connect to remote daemon
worker-cli --server=production:50051 run python3 script.py

# Use different configurations
worker-cli --config=prod-config.yml list
worker-cli --node=production run deploy.sh

# Set default server
export WORKER_SERVER=production:50051
worker-cli list  # Uses production server
```

### Real-World Examples

#### Development Workflow
```bash
# Test on remote development server
worker-cli --server=dev-server:50051 run --max-memory=1024 pytest tests/

# Build project with resource limits
worker-cli run --max-cpu=200 --max-memory=2048 make build

# Run security scans safely
worker-cli run --max-memory=512 trivy scan .
```

#### CI/CD Integration
```bash
# In CI pipeline
export WORKER_SERVER=build-server:50051
worker-cli run --max-memory=2048 docker build -t myapp .
worker-cli run --max-cpu=50 npm test
```

#### Data Processing
```bash
# Process large datasets with limits
worker-cli --server=data-server:50051 run --max-memory=4096 python process_data.py

# ETL with I/O throttling
worker-cli run --max-iobps=52428800 spark-submit etl_job.py
```

## ⚙️ Configuration

### Daemon Configuration

The daemon uses embedded certificates generated during installation:

```yaml
# /opt/worker/config/server-config.yml (auto-generated)
version: "3.0"
server:
  address: "192.168.1.100"
  port: 50051
  mode: "server"

worker:
  defaultCpuLimit: 50
  defaultMemoryLimit: 256
  maxConcurrentJobs: 100

security:
  # Certificates embedded here automatically
  serverCert: |
    -----BEGIN CERTIFICATE-----
    ...
  serverKey: |
    -----BEGIN PRIVATE KEY-----
    ...
  caCert: |
    -----BEGIN CERTIFICATE-----
    ...
```

### CLI Configuration

The CLI uses configuration with embedded certificates:

```yaml
# ~/.worker/client-config.yml (auto-generated)
version: "3.0"
nodes:
  default:
    address: "192.168.1.100:50051"
    cert: |
      -----BEGIN CERTIFICATE-----
      ...
    key: |
      -----BEGIN PRIVATE KEY-----
      ...
    ca: |
      -----BEGIN CERTIFICATE-----
      ...
  
  production:
    address: "prod-server:50051"
    # ... additional certificates for production
```

## 🔒 Security Model

### Certificate-Based Authentication

- **Automatic Generation**: Certificates embedded in configuration during installation
- **Role-Based Access**: Admin (full access) vs Viewer (read-only) certificates
- **mTLS Security**: All communication encrypted with TLS 1.3+

### Job Isolation

- **Process Isolation**: Each job runs in separate PID namespace (can only see its own processes)
- **Filesystem Isolation**: Jobs run in chroot with limited host access
- **Resource Isolation**: CPU, memory, and I/O limits prevent resource exhaustion
- **Network Isolation**: Optional network namespace isolation

## 🛠️ Advanced Configuration

### Regenerate Certificates

```bash
# On daemon server - regenerate with new IP
export WORKER_SERVER_ADDRESS="new-server-ip"
sudo /usr/local/bin/certs_gen.sh

# Restart daemon
sudo systemctl restart worker

# Update clients with new configuration
scp server:/opt/worker/config/client-config.yml ~/.worker/
```

### Multiple Environments

```yaml
# ~/.worker/client-config.yml
nodes:
  development:
    address: "dev.company.com:50051"
    # ... dev certificates
  
  staging:
    address: "staging.company.com:50051"
    # ... staging certificates
    
  production:
    address: "prod.company.com:50051"
    # ... production certificates
```

```bash
# Use specific environment
worker-cli --node=development run pytest
worker-cli --node=production run deploy.sh
```

### Service Management

```bash
# Daemon management
sudo systemctl start worker    # Start daemon
sudo systemctl stop worker     # Stop daemon
sudo systemctl status worker   # Check status
sudo journalctl -u worker -f   # View logs

# Configuration management
sudo dpkg-reconfigure worker   # Reconfigure package
```

## 🚨 Troubleshooting

### Common Issues

**Connection refused**
```bash
# Check daemon status
sudo systemctl status worker
sudo journalctl -u worker --lines=20

# Test connectivity
telnet your-server 50051
```

**Certificate errors**
```bash
# Verify certificates
openssl x509 -in ~/.worker/client-config.yml -noout -text

# Regenerate if needed
sudo /usr/local/bin/certs_gen.sh
```

**CLI not found**
```bash
# Check CLI installation
which worker-cli
ls -la /usr/local/bin/worker-cli

# Install if missing
sudo ln -sf /opt/worker/worker-cli /usr/local/bin/worker-cli
```

### Getting Help

- **Configuration Examples**: `worker-cli config-help`
- **Node Information**: `worker-cli nodes`
- **GitHub Issues**: [Report bugs](https://github.com/ehsaniara/worker/issues)
- **Documentation**: Full docs in `/docs` directory

## 📊 System Requirements

### Worker Daemon (Linux Server)
- **OS**: Linux kernel 4.6+ (Ubuntu 18.04+, CentOS 8+)
- **Memory**: 1GB+ RAM
- **Disk**: 5GB+ available space
- **Network**: Port 50051 (configurable)
- **Privileges**: Root access for namespace operations

### Worker CLI (Client)
- **OS**: Linux, macOS 10.15+, Windows 10+
- **Memory**: 50MB+ RAM
- **Network**: Access to daemon port 50051

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
- [Releases](https://github.com/ehsaniara/worker/releases) - Download latest version
- [Examples](https://github.com/ehsaniara/worker/tree/main/examples) - Usage examples
- [Contributing](.github/CONTRIBUTING.md) - Development guide