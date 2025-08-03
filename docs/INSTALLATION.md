# Installation Guide

This guide provides detailed instructions for installing Joblet on various operating systems and architectures.

## 📋 System Requirements

### Joblet Server (Linux Only)

- **OS**: Linux with kernel 3.10 or later
- **Architecture**: x86_64 (amd64) or ARM64
- **Cgroups**: v2 support (v1 compatibility available)
- **Permissions**: Root or sudo access
- **Memory**: Minimum 512MB RAM
- **Disk**: Minimum 1GB free space

### RNX Client (Cross-platform)

- **OS**: Linux, macOS, Windows
- **Architecture**: x86_64, ARM64, or Apple Silicon
- **Network**: Access to Joblet server port (default: 50051)

## 🐧 Linux Installation

### Ubuntu/Debian (20.04+)

```bash
# Update package list
sudo apt update

# Install dependencies
sudo apt install -y curl tar make gcc

# Download and install
curl -L https://github.com/ehsaniara/joblet/releases/latest/download/joblet-linux-amd64.tar.gz | tar xz
sudo mv joblet /usr/local/bin/
sudo mv rnx /usr/local/bin/
sudo chmod +x /usr/local/bin/joblet /usr/local/bin/rnx

# Create directories
sudo mkdir -p /opt/joblet/{config,state,certs,jobs,volumes}
sudo mkdir -p /var/log/joblet

# Verify installation
joblet --version
rnx --version
```

### RHEL/CentOS/Fedora (8+)

```bash
# Install dependencies
sudo dnf install -y curl tar make gcc

# Download and install
curl -L https://github.com/ehsaniara/joblet/releases/latest/download/joblet-linux-amd64.tar.gz | tar xz
sudo mv joblet /usr/local/bin/
sudo mv rnx /usr/local/bin/
sudo chmod +x /usr/local/bin/joblet /usr/local/bin/rnx

# Create directories
sudo mkdir -p /opt/joblet/{config,state,certs,jobs,volumes}
sudo mkdir -p /var/log/joblet

# Enable cgroups v2 if needed
sudo grubby --update-kernel=ALL --args="systemd.unified_cgroup_hierarchy=1"
# Reboot required after this change
```

### Amazon Linux 2

```bash
# Install dependencies
sudo yum install -y curl tar make gcc

# Download and install
curl -L https://github.com/ehsaniara/joblet/releases/latest/download/joblet-linux-amd64.tar.gz | tar xz
sudo mv joblet /usr/local/bin/
sudo mv rnx /usr/local/bin/
sudo chmod +x /usr/local/bin/joblet /usr/local/bin/rnx

# Create directories
sudo mkdir -p /opt/joblet/{config,state,certs,jobs,volumes}
sudo mkdir -p /var/log/joblet
```

### Arch Linux

```bash
# Install from AUR (if available)
yay -S joblet

# Or manual installation
sudo pacman -S curl tar make gcc
curl -L https://github.com/ehsaniara/joblet/releases/latest/download/joblet-linux-amd64.tar.gz | tar xz
sudo mv joblet /usr/local/bin/
sudo mv rnx /usr/local/bin/
```

### ARM64 Systems (Raspberry Pi, AWS Graviton)

```bash
# Download ARM64 version
curl -L https://github.com/ehsaniara/joblet/releases/latest/download/joblet-linux-arm64.tar.gz | tar xz
sudo mv joblet /usr/local/bin/
sudo mv rnx /usr/local/bin/
sudo chmod +x /usr/local/bin/joblet /usr/local/bin/rnx
```

## 🍎 macOS Installation (Client Only)

### Using Homebrew (Recommended)

```bash
# Add Joblet tap
brew tap ehsaniara/joblet

# Install RNX client
brew install rnx
```

### Manual Installation

```bash
# Intel Macs
curl -L https://github.com/ehsaniara/joblet/releases/latest/download/rnx-darwin-amd64.tar.gz | tar xz

# Apple Silicon (M1/M2)
curl -L https://github.com/ehsaniara/joblet/releases/latest/download/rnx-darwin-arm64.tar.gz | tar xz

# Install
sudo mv rnx /usr/local/bin/
sudo chmod +x /usr/local/bin/rnx

# Create config directory
mkdir -p ~/.rnx
```

## 🪟 Windows Installation (Client Only)

### Using Scoop

```powershell
# Add Joblet bucket
scoop bucket add joblet https://github.com/ehsaniara/scoop-joblet

# Install RNX
scoop install rnx
```

### Manual Installation

1. Download the Windows binary:
    - [rnx-windows-amd64.zip](https://github.com/ehsaniara/joblet/releases/latest/download/rnx-windows-amd64.zip)

2. Extract to a directory (e.g., `C:\Program Files\Joblet`)

3. Add to PATH:
   ```powershell
   [Environment]::SetEnvironmentVariable("Path", $env:Path + ";C:\Program Files\Joblet", "User")
   ```

4. Create config directory:
   ```powershell
   mkdir $env:USERPROFILE\.rnx
   ```

## 🔨 Building from Source

### Prerequisites

- Go 1.21 or later
- Git
- Make (optional but recommended)
- GCC (for CGO dependencies)

### Build Steps

```bash
# Clone repository
git clone https://github.com/ehsaniara/joblet.git
cd joblet

# Build all binaries
make build

# Or build manually
go build -o joblet ./cmd/joblet
go build -o rnx ./cmd/rnx

# Run tests
make test

# Install binaries
sudo make install
```

### Cross-compilation

```bash
# Build for Linux AMD64
GOOS=linux GOARCH=amd64 go build -o joblet-linux-amd64 ./cmd/joblet

# Build for macOS ARM64
GOOS=darwin GOARCH=arm64 go build -o rnx-darwin-arm64 ./cmd/rnx

# Build for Windows
GOOS=windows GOARCH=amd64 go build -o rnx.exe ./cmd/rnx
```

## 🔐 Certificate Generation

### Automatic Generation

```bash
# Set server address (REQUIRED)
export JOBLET_SERVER_ADDRESS='192.168.1.100'  # Use your server's IP

# Generate certificates with embedded configuration
sudo /usr/local/bin/certs_gen_embedded.sh
```

This creates:

- `/opt/joblet/config/joblet-config.yml` - Server config with embedded certificates
- `/opt/joblet/config/rnx-config.yml` - Client config with embedded certificates

### Manual Certificate Generation

```bash
# Create CA
openssl genrsa -out ca-key.pem 4096
openssl req -new -x509 -key ca-key.pem -out ca-cert.pem -days 3650 \
  -subj "/CN=Joblet CA"

# Create server certificate
openssl genrsa -out server-key.pem 4096
openssl req -new -key server-key.pem -out server.csr \
  -subj "/CN=joblet"
openssl x509 -req -in server.csr -CA ca-cert.pem -CAkey ca-key.pem \
  -out server-cert.pem -days 365 -CAcreateserial \
  -extensions v3_req -extfile <(echo "[v3_req]
subjectAltName = DNS:localhost,DNS:joblet,IP:127.0.0.1,IP:${JOBLET_SERVER_ADDRESS}")

# Create client certificate
openssl genrsa -out client-key.pem 4096
openssl req -new -key client-key.pem -out client.csr \
  -subj "/CN=rnx-client/OU=admin"
openssl x509 -req -in client.csr -CA ca-cert.pem -CAkey ca-key.pem \
  -out client-cert.pem -days 365 -CAcreateserial
```

## 🚀 Systemd Service Setup

### Create Service File

```bash
sudo tee /etc/systemd/system/joblet.service > /dev/null <<EOF
[Unit]
Description=Joblet Job Execution Service
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
Group=root
ExecStart=/usr/local/bin/joblet
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=joblet
Environment="JOBLET_CONFIG_PATH=/opt/joblet/config/joblet-config.yml"

# Security settings
NoNewPrivileges=false
PrivateTmp=false
ProtectSystem=false
ProtectHome=false

[Install]
WantedBy=multi-user.target
EOF
```

### Enable and Start Service

```bash
# Reload systemd
sudo systemctl daemon-reload

# Enable service
sudo systemctl enable joblet

# Start service
sudo systemctl start joblet

# Check status
sudo systemctl status joblet

# View logs
sudo journalctl -u joblet -f
```

## 🐳 Docker Installation

### Running Joblet in Docker

```bash
# Create Docker network
docker network create joblet

# Run Joblet server
docker run -d \
  --name joblet \
  --network joblet \
  --privileged \
  -v /sys/fs/cgroup:/sys/fs/cgroup:rw \
  -v joblet-config:/opt/joblet/config \
  -v joblet-state:/opt/joblet/state \
  -v joblet-jobs:/opt/joblet/jobs \
  -p 50051:50051 \
  ehsaniara/joblet:latest
```

### Docker Compose

```yaml
version: '3.8'

services:
  joblet:
    image: ehsaniara/joblet:latest
    container_name: joblet
    privileged: true
    network_mode: host
    volumes:
      - /sys/fs/cgroup:/sys/fs/cgroup:rw
      - ./config:/opt/joblet/config
      - joblet-state:/opt/joblet/state
      - joblet-jobs:/opt/joblet/jobs
      - joblet-volumes:/opt/joblet/volumes
    environment:
      - JOBLET_CONFIG_PATH=/opt/joblet/config/joblet-config.yml
    restart: unless-stopped

volumes:
  joblet-state:
  joblet-jobs:
  joblet-volumes:
```

## ☸️ Kubernetes Installation

### Helm Chart

```bash
# Add Helm repository
helm repo add joblet https://ehsaniara.github.io/joblet-helm
helm repo update

# Install Joblet
helm install joblet joblet/joblet \
  --set server.address=0.0.0.0 \
  --set service.type=LoadBalancer
```

### Manual Kubernetes Deployment

See [Kubernetes deployment example](./examples/kubernetes/) for detailed YAML files.

## ✅ Post-Installation Verification

### Server Health Check

```bash
# Check if server is running
sudo systemctl is-active joblet

# Test server locally
sudo joblet --version

# Check listening port
sudo ss -tlnp | grep 50051
```

### Client Connectivity Test

```bash
# Copy client config from server
scp server:/opt/joblet/config/rnx-config.yml ~/.rnx/

# Test connection
rnx list

# Run test job
rnx run echo "Installation successful!"
```

## 🔧 Troubleshooting Installation

### Common Issues

1. **Permission Denied**
   ```bash
   sudo chmod +x /usr/local/bin/joblet /usr/local/bin/rnx
   ```

2. **Cgroups v2 Not Available**
   ```bash
   # Check cgroups version
   mount | grep cgroup
   
   # Enable cgroups v2 (requires reboot)
   sudo grubby --update-kernel=ALL --args="systemd.unified_cgroup_hierarchy=1"
   ```

3. **Port Already in Use**
   ```bash
   # Find process using port
   sudo lsof -i :50051
   
   # Change port in config
   # Edit /opt/joblet/config/joblet-config.yml
   ```

4. **Certificate Issues**
   ```bash
   # Regenerate certificates
   sudo rm -rf /opt/joblet/config/*.yml
   sudo /usr/local/bin/certs_gen_embedded.sh
   ```

## 📚 Next Steps

- [Configuration Guide](./CONFIGURATION.md) - Customize your setup
- [Quick Start Guide](./QUICKSTART.md) - Start using Joblet
- [Security Guide](./SECURITY.md) - Secure your installation