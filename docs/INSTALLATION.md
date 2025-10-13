# Joblet Platform Installation Guide

This comprehensive installation guide provides detailed procedures for deploying Joblet across diverse operating systems
and hardware architectures. The guide covers both server-side components for Linux systems and client-side tools for
cross-platform environments.

> **Important**: Joblet leverages native Linux kernel capabilities to deliver enterprise-grade performance, security,
> and resource management through namespaces and cgroups v2. Direct installation on Linux hosts ensures optimal
> performance characteristics with kernel-level process isolation.

## System Requirements

### Server Requirements (Linux Exclusive)

- **Operating System**: Linux distributions with kernel version 3.10 or higher
- **Processor Architecture**: x86_64 (amd64) or ARM64
- **Control Groups**: cgroups v2 recommended (v1 compatibility supported)
- **Access Requirements**: Root privileges or sudo access
- **Memory Requirements**: Minimum 512MB RAM (2GB recommended for production)
- **Storage Requirements**: Minimum 1GB available disk space

### Client Requirements (Cross-Platform)

- **Operating Systems**: Linux, macOS, Windows
- **Processor Architecture**: x86_64, ARM64, Apple Silicon (M1/M2/M3)
- **Network Connectivity**: TCP access to Joblet server (default port: 50051)

## Linux Platform Installation

### Ubuntu/Debian Installation (Version 20.04 and Later)

```bash
# Update package list
sudo apt update

# Install dependencies
sudo apt install -y curl tar make gcc

# Download and install
curl -L https://github.com/ehsaniara/joblet/releases/latest/download/joblet-linux-amd64.tar.gz | tar xz
sudo mv joblet /usr/local/bin/
sudo mv rnx /usr/local/bin/
sudo mv joblet-persist /usr/local/bin/
sudo chmod +x /usr/local/bin/joblet /usr/local/bin/rnx /usr/local/bin/joblet-persist

# Create directories
sudo mkdir -p /opt/joblet/{config,state,certs,jobs,volumes,logs,metrics,run}
sudo mkdir -p /var/log/joblet

# Verify installation
joblet --version
rnx --version
joblet-persist version
```

### Red Hat Enterprise Linux/CentOS/Fedora Installation (Version 8 and Later)

```bash
# Install dependencies
sudo dnf install -y curl tar make gcc

# Download and install
curl -L https://github.com/ehsaniara/joblet/releases/latest/download/joblet-linux-amd64.tar.gz | tar xz
sudo mv joblet /usr/local/bin/
sudo mv rnx /usr/local/bin/
sudo mv joblet-persist /usr/local/bin/
sudo chmod +x /usr/local/bin/joblet /usr/local/bin/rnx /usr/local/bin/joblet-persist

# Create directories
sudo mkdir -p /opt/joblet/{config,state,certs,jobs,volumes,logs,metrics,run}
sudo mkdir -p /var/log/joblet

# Enable cgroups v2 if needed
sudo grubby --update-kernel=ALL --args="systemd.unified_cgroup_hierarchy=1"
# Reboot required after this change
```

### Amazon Linux 2 Installation

```bash
# Install dependencies
sudo yum install -y curl tar make gcc

# Download and install
curl -L https://github.com/ehsaniara/joblet/releases/latest/download/joblet-linux-amd64.tar.gz | tar xz
sudo mv joblet /usr/local/bin/
sudo mv rnx /usr/local/bin/
sudo mv joblet-persist /usr/local/bin/
sudo chmod +x /usr/local/bin/joblet /usr/local/bin/rnx /usr/local/bin/joblet-persist

# Create directories
sudo mkdir -p /opt/joblet/{config,state,certs,jobs,volumes,logs,metrics,run}
sudo mkdir -p /var/log/joblet
```

### Arch Linux Installation

```bash
# Install from AUR (if available)
yay -S joblet

# Or manual installation
sudo pacman -S curl tar make gcc
curl -L https://github.com/ehsaniara/joblet/releases/latest/download/joblet-linux-amd64.tar.gz | tar xz
sudo mv joblet /usr/local/bin/
sudo mv rnx /usr/local/bin/
```

### ARM64 Architecture Systems (Raspberry Pi, AWS Graviton)

```bash
# Download ARM64 version
curl -L https://github.com/ehsaniara/joblet/releases/latest/download/joblet-linux-arm64.tar.gz | tar xz
sudo mv joblet /usr/local/bin/
sudo mv rnx /usr/local/bin/
sudo mv joblet-persist /usr/local/bin/
sudo chmod +x /usr/local/bin/joblet /usr/local/bin/rnx /usr/local/bin/joblet-persist
```

## AWS EC2 Deployment with Terraform

### Infrastructure as Code Deployment

The following Terraform configuration deploys Joblet on AWS EC2 instances with production-ready security groups, networking, and automated installation.

#### Prerequisites

- Terraform v1.0+ installed
- AWS CLI configured with appropriate credentials
- An existing AWS VPC and subnet (or use the provided VPC configuration)
- SSH key pair for EC2 access

#### Terraform Configuration

Create a `main.tf` file with the following configuration:

```hcl
# Terraform configuration for Joblet AWS EC2 deployment
terraform {
  required_version = ">= 1.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

# AWS Provider configuration
provider "aws" {
  region = var.aws_region
}

# Variables
variable "aws_region" {
  description = "AWS region for deployment"
  type        = string
  default     = "us-west-2"
}

variable "instance_type" {
  description = "EC2 instance type for Joblet server"
  type        = string
  default     = "t3.medium"
}

variable "key_name" {
  description = "AWS Key Pair name for SSH access"
  type        = string
}

variable "allowed_cidr_blocks" {
  description = "CIDR blocks allowed to access Joblet server"
  type        = list(string)
  default     = ["0.0.0.0/0"]  # Restrict this in production
}

variable "environment" {
  description = "Environment name (e.g., dev, staging, prod)"
  type        = string
  default     = "dev"
}

# Data sources
data "aws_availability_zones" "available" {
  state = "available"
}

data "aws_ami" "ubuntu" {
  most_recent = true
  owners      = ["099720109477"] # Canonical

  filter {
    name   = "name"
    values = ["ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-amd64-server-*"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }
}

# VPC Configuration
resource "aws_vpc" "joblet_vpc" {
  cidr_block           = "10.0.0.0/16"
  enable_dns_hostnames = true
  enable_dns_support   = true

  tags = {
    Name        = "joblet-vpc-${var.environment}"
    Environment = var.environment
    Purpose     = "joblet-infrastructure"
  }
}

# Internet Gateway
resource "aws_internet_gateway" "joblet_igw" {
  vpc_id = aws_vpc.joblet_vpc.id

  tags = {
    Name        = "joblet-igw-${var.environment}"
    Environment = var.environment
  }
}

# Public Subnet
resource "aws_subnet" "joblet_public_subnet" {
  vpc_id                  = aws_vpc.joblet_vpc.id
  cidr_block              = "10.0.1.0/24"
  availability_zone       = data.aws_availability_zones.available.names[0]
  map_public_ip_on_launch = true

  tags = {
    Name        = "joblet-public-subnet-${var.environment}"
    Environment = var.environment
  }
}

# Route Table
resource "aws_route_table" "joblet_public_rt" {
  vpc_id = aws_vpc.joblet_vpc.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.joblet_igw.id
  }

  tags = {
    Name        = "joblet-public-rt-${var.environment}"
    Environment = var.environment
  }
}

# Route Table Association
resource "aws_route_table_association" "joblet_public_rta" {
  subnet_id      = aws_subnet.joblet_public_subnet.id
  route_table_id = aws_route_table.joblet_public_rt.id
}

# Security Group for Joblet Server
resource "aws_security_group" "joblet_server_sg" {
  name_prefix = "joblet-server-${var.environment}-"
  vpc_id      = aws_vpc.joblet_vpc.id
  description = "Security group for Joblet server"

  # SSH access
  ingress {
    description = "SSH"
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = var.allowed_cidr_blocks
  }

  # Joblet gRPC API
  ingress {
    description = "Joblet gRPC API"
    from_port   = 50051
    to_port     = 50051
    protocol    = "tcp"
    cidr_blocks = var.allowed_cidr_blocks
  }

  # Joblet Admin UI (if enabled)
  ingress {
    description = "Joblet Admin UI"
    from_port   = 5173
    to_port     = 5173
    protocol    = "tcp"
    cidr_blocks = var.allowed_cidr_blocks
  }

  # All outbound traffic
  egress {
    description = "All outbound traffic"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name        = "joblet-server-sg-${var.environment}"
    Environment = var.environment
  }
}

# User data script for Joblet installation
locals {
  user_data = base64encode(templatefile("${path.module}/install-joblet.sh", {
    environment = var.environment
  }))
}

# EC2 Instance for Joblet Server
resource "aws_instance" "joblet_server" {
  ami                    = data.aws_ami.ubuntu.id
  instance_type          = var.instance_type
  key_name               = var.key_name
  vpc_security_group_ids = [aws_security_group.joblet_server_sg.id]
  subnet_id              = aws_subnet.joblet_public_subnet.id
  user_data              = local.user_data

  root_block_device {
    volume_type           = "gp3"
    volume_size           = 20
    delete_on_termination = true
    encrypted             = true

    tags = {
      Name        = "joblet-server-root-${var.environment}"
      Environment = var.environment
    }
  }

  tags = {
    Name        = "joblet-server-${var.environment}"
    Environment = var.environment
    Purpose     = "joblet-execution-platform"
  }

  lifecycle {
    create_before_destroy = true
  }
}

# Elastic IP for stable public access
resource "aws_eip" "joblet_server_eip" {
  instance = aws_instance.joblet_server.id
  domain   = "vpc"

  tags = {
    Name        = "joblet-server-eip-${var.environment}"
    Environment = var.environment
  }

  depends_on = [aws_internet_gateway.joblet_igw]
}

# Outputs
output "joblet_server_public_ip" {
  description = "Public IP address of the Joblet server"
  value       = aws_eip.joblet_server_eip.public_ip
}

output "joblet_server_private_ip" {
  description = "Private IP address of the Joblet server"
  value       = aws_instance.joblet_server.private_ip
}

output "joblet_server_public_dns" {
  description = "Public DNS name of the Joblet server"
  value       = aws_eip.joblet_server_eip.public_dns
}

output "ssh_command" {
  description = "SSH command to connect to the Joblet server"
  value       = "ssh -i ~/.ssh/${var.key_name}.pem ubuntu@${aws_eip.joblet_server_eip.public_ip}"
}

output "joblet_api_endpoint" {
  description = "Joblet gRPC API endpoint"
  value       = "${aws_eip.joblet_server_eip.public_ip}:50051"
}
```

#### Installation Script

Create an `install-joblet.sh` file:

```bash
#!/bin/bash
# Joblet installation script for AWS EC2

set -euo pipefail

# Variables
ENVIRONMENT="${environment}"
LOG_FILE="/var/log/joblet-install.log"
JOBLET_VERSION="latest"

# Logging function
log() {
    echo "$(date '+%Y-%m-%d %H:%M:%S') [INFO] $*" | tee -a "$LOG_FILE"
}

log_error() {
    echo "$(date '+%Y-%m-%d %H:%M:%S') [ERROR] $*" | tee -a "$LOG_FILE" >&2
}

# Start installation
log "Starting Joblet installation on AWS EC2 for environment: $ENVIRONMENT"

# Update system
log "Updating system packages"
apt-get update -y
apt-get upgrade -y

# Install dependencies
log "Installing dependencies"
apt-get install -y curl wget unzip jq awscli

# Enable cgroups v2
log "Configuring cgroups v2"
if ! grep -q "systemd.unified_cgroup_hierarchy=1" /etc/default/grub; then
    sed -i 's/GRUB_CMDLINE_LINUX_DEFAULT="/GRUB_CMDLINE_LINUX_DEFAULT="systemd.unified_cgroup_hierarchy=1 /' /etc/default/grub
    update-grub
    log "Configured cgroups v2 - reboot required"
fi

# Prepare AWS EC2 configuration for Debian package installer
log "Preparing AWS EC2 configuration for Joblet installation"

# Get AWS instance metadata
INTERNAL_IP=$(curl -s http://169.254.169.254/latest/meta-data/local-ipv4 2>/dev/null || echo "127.0.0.1")
PUBLIC_IP=$(curl -s http://169.254.169.254/latest/meta-data/public-ipv4 2>/dev/null || echo "")
INSTANCE_ID=$(curl -s http://169.254.169.254/latest/meta-data/instance-id 2>/dev/null || echo "")
REGION=$(curl -s http://169.254.169.254/latest/meta-data/placement/region 2>/dev/null || echo "")

# Create EC2 info file for Debian postinst script
cat > /tmp/joblet-ec2-info << EOF
IS_EC2=true
EC2_INSTANCE_ID="$INSTANCE_ID"
EC2_REGION="$REGION"
EC2_INTERNAL_IP="$INTERNAL_IP"
EC2_PUBLIC_IP="$PUBLIC_IP"
EOF

# Create installation configuration for Debian postinst script
cat > /tmp/joblet-install-config << EOF
JOBLET_SERVER_ADDRESS="0.0.0.0"
JOBLET_SERVER_PORT="50051"
JOBLET_CERT_INTERNAL_IP="$INTERNAL_IP"
JOBLET_CERT_PUBLIC_IP="$PUBLIC_IP"
JOBLET_CERT_PRIMARY="$PUBLIC_IP"
JOBLET_ADDITIONAL_NAMES="localhost,$INTERNAL_IP"
EOF

log "AWS EC2 configuration prepared for Debian package:"
log "  Internal IP: $INTERNAL_IP"
log "  Public IP: $PUBLIC_IP"
log "  Instance ID: $INSTANCE_ID"
log "  Region: $REGION"

# Download and install Joblet Debian package
log "Downloading and installing Joblet Debian package"
cd /tmp

# Determine architecture
ARCH=$(dpkg --print-architecture)
if [ "$ARCH" = "amd64" ]; then
    JOBLET_ARCH="amd64"
elif [ "$ARCH" = "arm64" ]; then
    JOBLET_ARCH="arm64"
else
    log "ERROR: Unsupported architecture: $ARCH"
    exit 1
fi

# Download the latest Debian package
JOBLET_VERSION=$(curl -s https://api.github.com/repos/ehsaniara/joblet/releases/latest | jq -r '.tag_name')
JOBLET_DEB_URL="https://github.com/ehsaniara/joblet/releases/download/${JOBLET_VERSION}/joblet-${JOBLET_VERSION#v}-linux-${JOBLET_ARCH}.deb"

log "Downloading Joblet ${JOBLET_VERSION} for ${JOBLET_ARCH}"
wget -O joblet.deb "$JOBLET_DEB_URL"

# Install the Debian package (this will automatically handle systemd service, certificates, etc.)
log "Installing Joblet Debian package"
DEBIAN_FRONTEND=noninteractive dpkg -i joblet.deb

# Fix any dependency issues
apt-get install -f -y

log "Joblet Debian package installed successfully"
log "The package installer has automatically:"
log "  ✓ Created systemd service with proper configuration"
log "  ✓ Generated TLS certificates for AWS EC2 environment"
log "  ✓ Configured network requirements and bridge networking"
log "  ✓ Set up cgroup delegation for resource management"
log "  ✓ Created all necessary directories and permissions"

# Verify installation
log "Verifying Joblet installation"
if systemctl is-enabled joblet >/dev/null 2>&1; then
    log "✓ Joblet systemd service is enabled"
else
    log "⚠ Joblet service not enabled, enabling now"
    systemctl enable joblet
fi

if [ -f /opt/joblet/config/joblet-config.yml ]; then
    log "✓ Server configuration created"
else
    log "✗ Server configuration missing"
fi

if [ -f /opt/joblet/config/rnx-config.yml ]; then
    log "✓ Client configuration created"
else
    log "✗ Client configuration missing"
fi

# Check if bridge network was created
if ip link show joblet0 >/dev/null 2>&1; then
    log "✓ Bridge network (joblet0) configured"
else
    log "⚠ Bridge network not found"
fi

# Setup log rotation
log "Configuring log rotation"
cat > /etc/logrotate.d/joblet << EOF
/var/log/joblet/*.log {
    daily
    missingok
    rotate 7
    compress
    delaycompress
    notifempty
    create 644 ubuntu ubuntu
    postrotate
        systemctl reload joblet
    endscript
}
EOF

# Install CloudWatch agent (optional)
if command -v aws >/dev/null 2>&1; then
    log "Installing CloudWatch agent"
    wget https://s3.amazonaws.com/amazoncloudwatch-agent/ubuntu/amd64/latest/amazon-cloudwatch-agent.deb
    dpkg -i amazon-cloudwatch-agent.deb
fi

log "Joblet installation completed successfully"
log "AWS EC2 Instance Configuration:"
log "  Internal Address: $INTERNAL_IP:50051"
if [ -n "$PUBLIC_IP" ]; then
    log "  Public Address: $PUBLIC_IP:50051"
fi
log "  Instance ID: $INSTANCE_ID"
log "  Region: $REGION"
log ""
log "Service Management:"
log "  Start service: systemctl start joblet"
log "  Check status: systemctl status joblet"
log "  View logs: journalctl -u joblet -f"
log ""
log "Client Configuration:"
log "  Copy config: scp root@$PUBLIC_IP:/opt/joblet/config/rnx-config.yml ~/.rnx/"
log "  Test connection: rnx --version"

# Start Joblet service
log "Starting Joblet service"
systemctl start joblet

# Wait a moment for service to start
sleep 5

# Check service status
if systemctl is-active joblet >/dev/null 2>&1; then
    log "✓ Joblet service is running"
else
    log "⚠ Joblet service failed to start, checking status"
    systemctl status joblet --no-pager -l
fi
```

#### Deployment Commands

```bash
# Initialize Terraform
terraform init

# Plan deployment
terraform plan -var="key_name=your-ssh-key-name"

# Apply configuration
terraform apply -var="key_name=your-ssh-key-name"

# Get outputs
terraform output
```

#### Production Considerations

1. **Security Groups**: Restrict `allowed_cidr_blocks` to your organization's IP ranges
2. **TLS Certificates**: Replace the placeholder certificate generation with proper CA-signed certificates
3. **Monitoring**: Enable CloudWatch monitoring and set up alerts
4. **Backup**: Configure automated snapshots for the EBS volume
5. **High Availability**: Consider multi-AZ deployment for production workloads
6. **Instance Size**: Adjust `instance_type` based on expected workload requirements

#### Connecting to Your Joblet Instance

After deployment:

```bash
# SSH to the instance
ssh -i ~/.ssh/your-key.pem ubuntu@$(terraform output -raw joblet_server_public_ip)

# Check Joblet service status
sudo systemctl status joblet

# View Joblet logs
sudo journalctl -u joblet -f

# Configure RNX client
mkdir -p ~/.rnx
scp -i ~/.ssh/your-key.pem ubuntu@$(terraform output -raw joblet_server_public_ip):/opt/joblet/config/rnx-config.yml ~/.rnx/

# Test connection
rnx --version
rnx job run echo "Hello from AWS EC2!"
```

## macOS Client Installation

### Installation via Homebrew Package Manager (Recommended)

```bash
# Add Joblet tap
brew tap ehsaniara/joblet https://github.com/ehsaniara/joblet

# Install RNX client
brew install rnx
```

The Homebrew installation includes:
- **RNX CLI**: Command-line interface for job management

### Joblet Admin UI (Standalone Package)

The Admin UI is now available as a separate repository. After installing the RNX CLI, you can optionally install the admin interface:

```bash
# Clone the joblet-admin repository
git clone https://github.com/ehsaniara/joblet-admin
cd joblet-admin

# Install dependencies
npm install

# Start the admin interface
npm run dev

# Access at http://localhost:3000
```

**Requirements for Admin UI:**
- Node.js 18+ required
- Requires configured RNX client with valid connection to Joblet server
- Connects directly to Joblet server via gRPC

**Learn more**: See the [Admin UI Documentation](./ADMIN_UI.md) for complete setup and usage instructions.

### Manual Binary Installation

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

## Windows Client Installation

### Installation via Scoop Package Manager

```powershell
# Add Joblet bucket
scoop bucket add joblet https://github.com/ehsaniara/scoop-joblet

# Install RNX
scoop install rnx
```

### Manual Binary Installation

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
make all

# Or build manually
go build -o bin/joblet ./cmd/joblet
go build -o bin/rnx ./cmd/rnx
cd persist && go build -o ../bin/joblet-persist ./cmd/joblet-persist

# Run tests
make test

# Install binaries
sudo make install
```

### Verify Installation

After installation, verify both client and server versions:

```bash
# Check RNX client version
rnx --version

# Output should show both client and server versions:
# RNX Client:
# rnx version v4.3.3 (abc1234)
# Built: 2025-09-14T05:17:17Z
# ...
#
# Joblet Server (default):
# joblet version v4.3.3 (abc1234)
# Built: 2025-09-14T05:18:24Z
# ...

# If server is not reachable, you'll see:
# Joblet Server: failed to connect to server: <error>

# Test basic functionality
rnx job list  # Should connect to server and list jobs
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

Joblet uses a single systemd service. The persistence layer (joblet-persist) runs as an embedded subprocess.

### Create Joblet Service File

**Note:** joblet-persist now runs as a subprocess of joblet. Only one service is needed.

```bash
sudo tee /etc/systemd/system/joblet.service > /dev/null <<EOF
[Unit]
Description=Joblet Job Execution Service with Embedded Persistence
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

# Enable and start joblet service
sudo systemctl enable joblet
sudo systemctl start joblet

# Check status
sudo systemctl status joblet

# View logs (includes both joblet and persist subprocess logs)
sudo journalctl -u joblet -f

# View only persist subprocess logs
sudo journalctl -u joblet -f | grep '\[PERSIST\]'
```

## 🖥️ Development Environment Setup

### Local Development

Joblet provides superior isolation, performance, and resource control through Linux namespaces and cgroups v2.

```bash
# Set up development environment on Linux
# Requires Linux host (VM, WSL2, or native Linux)

# Install development dependencies
sudo apt update
sudo apt install -y build-essential git protobuf-compiler

# Clone and build
git clone https://github.com/ehsaniara/joblet.git
cd joblet
make all

# Run tests
make test

# Install locally for development
sudo make install
```

### Native Process Isolation

Joblet provides native Linux process isolation with:

- **Better Performance**: Direct Linux namespace execution (no container overhead)
- **Superior Resource Control**: cgroups v2 with precise CPU, memory, and I/O limits
- **Enhanced Security**: Process isolation without container escape vulnerabilities
- **Simplified Deployment**: Single binary installation vs container orchestration complexity
- **Instant Startup**: 2-3 second job execution vs container pull/start overhead

**Joblet Commands:**

- `rnx job run` - Execute isolated processes
- `rnx job run --workflow=workflow.yaml` - Run complex workflows
- `rnx runtime install` - Install pre-built runtime environments

## ✅ Post-Installation Verification

### Server Health Check

```bash
# Check if both services are running
sudo systemctl is-active joblet-persist
sudo systemctl is-active joblet

# Test binaries locally
sudo joblet --version
sudo joblet-persist version

# Check listening ports
sudo ss -tlnp | grep 50051  # Main joblet service
sudo ss -tlnp | grep 50052  # Persist service (optional gRPC)

# Verify Unix socket for IPC
ls -la /opt/joblet/run/persist.sock
```

### Client Connectivity Test

```bash
# Copy client config from server
scp server:/opt/joblet/config/rnx-config.yml ~/.rnx/

# Test connection
rnx job list

# Run test job
rnx job run echo "Installation successful!"
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