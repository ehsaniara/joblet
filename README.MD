# Joblet / RNX - Secure Linux Process Execution Platform

[![Tests](https://github.com/ehsaniara/joblet/actions/workflows/ci.yml/badge.svg)](https://github.com/ehsaniara/joblet/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/ehsaniara/joblet)](https://goreportcard.com/report/github.com/ehsaniara/joblet)
[![Go Version](https://img.shields.io/github/go-mod/go-version/ehsaniara/joblet)](https://golang.org/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Release](https://img.shields.io/github/release/ehsaniara/joblet.svg)](https://github.com/ehsaniara/joblet/releases/latest)

<img align="right" width="159px" src="docs/joblet-thumbnail.png">

📝 **Note:** Project recently renamed from _Worker_ to **Joblet** and **RNX** (CLI)

**Joblet** provides **secure, isolated job execution** on Linux systems. Run commands remotely with complete process
isolation, resource limits, and real-time monitoring - protecting your host system from malicious or resource-hungry
processes.

## 🎯 Why Use Joblet?

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
# SAFE: Running commands through Joblet
rnx run --max-memory=512 python3 untrusted-script.py
# ✅ Process isolation - can only see its own processes
# ✅ Filesystem isolation - limited access to host files  
# ✅ Resource limits - memory, CPU, I/O controls
# ✅ Real-time monitoring and logs
```

## 🔧 How It Works

Joblet consists of two components:

1. **Joblet Daemon** - Runs on Linux servers, executes jobs in isolated environments
2. **RNX CLI** - Connects from anywhere (Linux/macOS/Windows) to manage jobs

![joblet-how-it-works.svg](docs/joblet-how-it-works.svg)

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
wget $(curl -s https://api.github.com/repos/ehsaniara/joblet/releases/latest | grep "browser_download_url.*_amd64\.deb" | cut -d '"' -f 4)

# Interactive installation (prompts for server IP)
sudo dpkg -i joblet_*_amd64.deb

# Start the daemon
sudo systemctl start joblet
sudo systemctl enable joblet

# Test locally
rnx --config=/opt/joblet/config/rnx-config.yml list
```

### Manual Installation

#### 1. Build from Source

```bash
git clone https://github.com/ehsaniara/joblet.git
cd joblet

# Build binaries
make all  # Creates: bin/joblet, bin/rnx

# Or download pre-built binaries from releases
```

#### 2. Install Joblet Daemon (Linux Server)

```bash
# Install daemon binary
sudo mkdir -p /opt/joblet/config /opt/joblet/scripts
sudo cp bin/joblet /opt/joblet/
sudo chmod +x /opt/joblet/joblet

# Copy configuration templates
sudo cp scripts/joblet-config-template.yml /opt/joblet/scripts/
sudo cp scripts/joblet.service /etc/systemd/system/

# Generate certificates with embedded configuration
export JOBLET_SERVER_ADDRESS="192.168.1.100"  # Your server IP
sudo ./scripts/certs_gen_embedded.sh

# Start daemon service
sudo systemctl daemon-reload
sudo systemctl enable joblet
sudo systemctl start joblet
```

#### 3. Install RNX CLI (Client Machine)

##### Linux/macOS CLI Setup

```bash
# Install CLI binary
sudo cp bin/rnx /usr/local/bin/rnx
sudo chmod +x /usr/local/bin/rnx

# Download client configuration from daemon server
mkdir -p ~/.rnx
scp your-server:/opt/joblet/config/rnx-config.yml ~/.rnx/

# Test connection
rnx --config ~/.rnx/rnx-config.yml list
```

##### Windows CLI Setup

```powershell
# Download Windows CLI from releases
Invoke-WebRequest -Uri "https://github.com/ehsaniara/joblet/releases/latest/download/rnx-windows-amd64.exe" -OutFile "rnx.exe"

# Copy to PATH
Move-Item rnx.exe "C:\Program Files\rnx.exe"

# Copy client config from daemon server to:
# %USERPROFILE%\.rnx\rnx-config.yml
```

## 🚀 Usage

### Basic Job Management

```bash
# Run simple commands
rnx run echo "Hello World"
rnx run ps aux
rnx run python3 --version

# Run with resource limits
rnx run --max-cpu=50 --max-memory=256 python3 script.py
rnx run --max-iobps=5242880 dd if=/dev/zero of=/tmp/test bs=1M count=100

# Complex commands
rnx run bash -c "curl -s https://api.github.com/users/octocat | jq '.name'"
```

### Job Monitoring

```bash
# List all jobs
rnx list

# Get job details
rnx status <job-id>

# Stream live logs
rnx log <job-id>

# Stop running job
rnx stop <job-id>
```

### Remote Server Connection

```bash
# Connect to remote daemon
rnx --server=production:50051 run python3 script.py

# Use different configurations
rnx --config=prod-rnx-config.yml list
rnx --node=production run deploy.sh

# Set default server
export JOBLET_SERVER=production:50051
rnx list  # Uses production server
```

### Real-World Examples

#### Development Workflow

```bash
# Test on remote development server
rnx --server=dev-server:50051 run --max-memory=1024 pytest tests/

# Build project with resource limits
rnx run --max-cpu=200 --max-memory=2048 make build

# Run security scans safely
rnx run --max-memory=512 trivy scan .
```

#### CI/CD Integration

```bash
# In CI pipeline
export JOBLET_SERVER=build-server:50051
rnx run --max-memory=2048 docker build -t myapp .
rnx run --max-cpu=50 npm test
```

#### Data Processing

```bash
# Process large datasets with limits
rnx --server=data-server:50051 run --max-memory=4096 python process_data.py

# ETL with I/O throttling
rnx run --max-iobps=52428800 spark-submit etl_job.py
```

## ⚙️ Configuration

### Daemon Configuration

The daemon uses embedded certificates generated during installation:

```yaml
# /opt/joblet/config/joblet-config.yml (auto-generated)
version: "3.0"
server:
  address: "192.168.1.100"
  port: 50051
  mode: "server"

joblet:
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

### RNX CLI Configuration

The CLI uses configuration with embedded certificates:

```yaml
# ~/.rnx/rnx-config.yml (auto-generated)
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
export JOBLET_SERVER_ADDRESS="new-server-ip"
sudo /usr/local/bin/certs_gen_embedded.sh

# Restart daemon
sudo systemctl restart joblet

# Update clients with new configuration
scp server:/opt/joblet/config/rnx-config.yml ~/.rnx/
```

### Multiple Environments

```yaml
# ~/.rnx/rnx-config.yml
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
rnx --node=development run pytest
rnx --node=production run deploy.sh
```

### Service Management

```bash
# Daemon management
sudo systemctl start joblet    # Start daemon
sudo systemctl stop joblet     # Stop daemon
sudo systemctl status joblet   # Check status
sudo journalctl -u joblet -f   # View logs

# Configuration management
sudo dpkg-reconfigure joblet   # Reconfigure package
```

## 🚨 Troubleshooting

### Common Issues

**Connection refused**

```bash
# Check daemon status
sudo systemctl status joblet
sudo journalctl -u joblet --lines=20

# Test connectivity
telnet your-server 50051
```

**Certificate errors**

```bash
# Verify certificates
openssl x509 -in ~/.rnx/rnx-config.yml -noout -text

# Regenerate if needed
sudo /usr/local/bin/certs_gen_embedded.sh
```

**CLI not found**

```bash
# Check CLI installation
which rnx
ls -la /usr/local/bin/rnx

# Install if missing
sudo ln -sf /opt/joblet/rnx /usr/local/bin/rnx
```

### Getting Help

- **Configuration Examples**: `rnx config-help`
- **Node Information**: `rnx nodes`
- **GitHub Issues**: [Report bugs](https://github.com/ehsaniara/joblet/issues)
- **Documentation**: Full docs in `/docs` directory

## 📊 System Requirements

### Joblet Daemon (Linux Server)

- **OS**: Linux kernel 4.6+ (Ubuntu 18.04+, CentOS 8+)
- **Memory**: 1GB+ RAM
- **Disk**: 5GB+ available space
- **Network**: Port 50051 (configurable)
- **Privileges**: Root access for namespace operations

### RNX CLI (Client)

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

- [Releases](https://github.com/ehsaniara/joblet/releases) - Download latest version
- [Examples](https://github.com/ehsaniara/joblet/tree/main/examples) - Usage examples
- [Contributing](.github/CONTRIBUTING.md) - Development guide

## 💡 Project Name History

This project was previously known as "Worker" but was renamed to:

- **Joblet** (daemon) - Better describes the job execution platform
- **RNX** (CLI) - Remote eXecution, a concise and memorable name

The rename provides clearer branding and eliminates confusion with other "worker" tools in the ecosystem.