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

> **Note**: The `rnx` client CLI is released separately from the
> [joblet-rnx repository](https://github.com/ehsaniara/joblet-rnx/releases/latest)
> and is not part of the Joblet server packages or release archives. The .deb
> and .rpm installers download the latest rnx to `/usr/local/bin/rnx` on the
> server host (set `JOBLET_SKIP_RNX_INSTALL=1` to skip). For other machines see
> [Client Installation](#client-installation-rnx-cli) below.

### Ubuntu/Debian Installation (Version 20.04 and Later)

```bash
# Download the latest .deb for this machine's architecture (amd64 or arm64)
ARCH=$(dpkg --print-architecture)
wget $(curl -s https://api.github.com/repos/ehsaniara/joblet/releases/latest | grep "browser_download_url.*_${ARCH}\.deb" | cut -d '"' -f 4)

# Install (the installer generates certificates and configs, sets up networking,
# and installs the latest rnx client)
sudo dpkg -i joblet_*_${ARCH}.deb
sudo systemctl enable --now joblet

# Verify installation
systemctl status joblet
rnx job list
```

### Red Hat Enterprise Linux/CentOS/Fedora Installation (Version 8 and Later)

```bash
# Download the latest .rpm for this machine's architecture (x86_64 or aarch64)
ARCH=$(uname -m)
wget $(curl -s https://api.github.com/repos/ehsaniara/joblet/releases/latest | grep "browser_download_url.*\.${ARCH}\.rpm" | cut -d '"' -f 4)

# Install (resolves dependencies) and start
sudo dnf install -y ./joblet-*.${ARCH}.rpm
sudo systemctl enable --now joblet

# Enable cgroups v2 if needed
sudo grubby --update-kernel=ALL --args="systemd.unified_cgroup_hierarchy=1"
# Reboot required after this change
```

### Amazon Linux 2 Installation

```bash
ARCH=$(uname -m)
wget $(curl -s https://api.github.com/repos/ehsaniara/joblet/releases/latest | grep "browser_download_url.*\.${ARCH}\.rpm" | cut -d '"' -f 4)
sudo yum install -y ./joblet-*.${ARCH}.rpm
sudo systemctl enable --now joblet
```

For EC2 the user-data script in [AWS_DEPLOYMENT.md](AWS_DEPLOYMENT.md)
automates the same steps.

### Arch Linux Installation

No native Arch package is published. Build from source (see Building from
Source below) or install the .deb/.rpm through a converter of your choice.

### ARM64 Architecture Systems (Raspberry Pi, AWS Graviton)

Every release ships arm64 (.deb) and aarch64 (.rpm) packages; the commands
above pick the right one automatically.

## AWS EC2 Deployment with Terraform

> **💡 Quick Start**: For simpler EC2 deployment without Terraform, see [AWS_DEPLOYMENT.md](AWS_DEPLOYMENT.md) for
> ready-to-use bash scripts.

### Infrastructure as Code Deployment

The following Terraform configuration deploys Joblet on AWS EC2 instances with production-ready security groups,
networking, and automated installation.

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

# Get AWS instance metadata using IMDSv2 (required, IMDSv1 is disabled)
IMDS_TOKEN=$(curl -s -m 5 -X PUT "http://169.254.169.254/latest/api/token" -H "X-aws-ec2-metadata-token-ttl-seconds: 300" 2>/dev/null)
INTERNAL_IP=$(curl -s -m 5 -H "X-aws-ec2-metadata-token: $IMDS_TOKEN" http://169.254.169.254/latest/meta-data/local-ipv4 2>/dev/null || echo "127.0.0.1")
PUBLIC_IP=$(curl -s -m 5 -H "X-aws-ec2-metadata-token: $IMDS_TOKEN" http://169.254.169.254/latest/meta-data/public-ipv4 2>/dev/null || echo "")
INSTANCE_ID=$(curl -s -m 5 -H "X-aws-ec2-metadata-token: $IMDS_TOKEN" http://169.254.169.254/latest/meta-data/instance-id 2>/dev/null || echo "")
REGION=$(curl -s -m 5 -H "X-aws-ec2-metadata-token: $IMDS_TOKEN" http://169.254.169.254/latest/meta-data/placement/region 2>/dev/null || echo "")

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
JOBLET_DEB_URL="https://github.com/ehsaniara/joblet/releases/download/${JOBLET_VERSION}/joblet_${JOBLET_VERSION#v}_${JOBLET_ARCH}.deb"

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
log "  Copy config: scp root@$PUBLIC_IP:/opt/joblet/config/rnx-config-<role>.yml ~/.rnx/rnx-config.yml"
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

# Configure RNX client with your role's config (config files are root-only)
mkdir -p ~/.rnx
ssh -i ~/.ssh/your-key.pem ubuntu@$(terraform output -raw joblet_server_public_ip) "sudo cat /opt/joblet/config/rnx-config-<role>.yml" > ~/.rnx/rnx-config.yml

# Test connection
rnx --version
rnx job run echo "Hello from AWS EC2!"
```

## Client Installation (rnx CLI)

The `rnx` client is developed and released in its own repository:
[joblet-rnx](https://github.com/ehsaniara/joblet-rnx). Binaries for Linux,
macOS, and Windows are published on its
[releases page](https://github.com/ehsaniara/joblet-rnx/releases/latest).
Any rnx release works with any Joblet release that speaks the same
joblet-proto contract (see the joblet-rnx compatibility table).

On the server host itself the package installer already installs the latest
rnx (`/usr/local/bin/rnx`, skipped if an rnx is already on PATH or
`JOBLET_SKIP_RNX_INSTALL=1` is set, never fatal to the install). The methods
below are for client machines and for hosts without network access.

### Homebrew (macOS and Linux)

```bash
brew tap ehsaniara/rnx https://github.com/ehsaniara/joblet-rnx
brew install rnx
```

### Linux / macOS (manual)

```bash
# Pick the tarball for your platform from the releases page, e.g.:
#   rnx-<version>-linux-amd64.tar.gz
#   rnx-<version>-linux-arm64.tar.gz
#   rnx-<version>-darwin-amd64.tar.gz   (Intel Macs)
#   rnx-<version>-darwin-arm64.tar.gz   (Apple Silicon)
VERSION=v6.0.0  # substitute the latest release tag
OS=linux ARCH=amd64
curl -L https://github.com/ehsaniara/joblet-rnx/releases/download/${VERSION}/rnx-${VERSION}-${OS}-${ARCH}.tar.gz | tar xz
sudo install -m 0755 rnx-${OS}-${ARCH} /usr/local/bin/rnx

# Create config directory
mkdir -p ~/.rnx
```

### Windows

1. Download `rnx-<version>-windows-amd64.zip` from the
   [joblet-rnx releases page](https://github.com/ehsaniara/joblet-rnx/releases/latest)

2. Extract to a directory (e.g., `C:\Program Files\Joblet`) and rename the
   binary to `rnx.exe`

3. Add to PATH:
   ```powershell
   [Environment]::SetEnvironmentVariable("Path", $env:Path + ";C:\Program Files\Joblet", "User")
   ```

4. Create config directory:
   ```powershell
   mkdir $env:USERPROFILE\.rnx
   ```

### Joblet Admin UI (Standalone Package)

The Admin UI is now available as a separate repository. After installing the RNX CLI, you can optionally install the
admin interface:

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

# Build all server binaries
make all

# Or build manually
go build -o bin/joblet ./cmd/joblet
cd persist && go build -o ../bin/persist ./cmd/persist

# Run tests
make test

# Purge any existing install, build a .deb from the working tree, install it (uses sudo)
make fresh-install
```

To build the `rnx` client from source, clone
[joblet-rnx](https://github.com/ehsaniara/joblet-rnx) and run `make build`
there.

### Verify Installation

After installation, verify both client and server versions (with `rnx`
installed from joblet-rnx as described above):

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

# Build for Linux ARM64
GOOS=linux GOARCH=arm64 go build -o joblet-linux-arm64 ./cmd/joblet
```

The joblet server binaries are Linux-only; cross-platform builds of the
`rnx` client are done in the joblet-rnx repository.

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

# Create client certificate. The OU carries the role; repeat with
# OU=maintainer, OU=developer, or OU=reader for the other roles
openssl genrsa -out client-key.pem 4096
openssl req -new -key client-key.pem -out client.csr \
  -subj "/CN=admin-client/OU=admin"
openssl x509 -req -in client.csr -CA ca-cert.pem -CAkey ca-key.pem \
  -out client-cert.pem -days 365 -CAcreateserial
```

## 🚀 Systemd Service Setup

Joblet uses a single systemd service. The persistence layer (persist) runs as an embedded subprocess.

### Create Joblet Service File

**Note:** persist now runs as a subprocess of joblet. Only one service is needed.

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

# Install locally for development (builds and installs a .deb, uses sudo)
make fresh-install
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
- `rnx runtime build` - Build runtime environments from YAML specifications

## ✅ Post-Installation Verification

### Server Health Check

```bash
# Check if both services are running
sudo systemctl is-active persist
sudo systemctl is-active joblet

# Test binaries locally
sudo joblet --version
sudo persist version

# Check listening ports
sudo ss -tlnp | grep 50051  # Main joblet service
sudo ss -tlnp | grep 50052  # Persist service (optional gRPC)

# Verify Unix socket for IPC
ls -la /opt/joblet/run/persist.sock
```

### Client Connectivity Test

```bash
# Copy your role's client config from the server (files are root-only)
ssh server "sudo cat /opt/joblet/config/rnx-config-<role>.yml" > ~/.rnx/rnx-config.yml

# Test connection
rnx job list

# Run test job
rnx job run echo "Installation successful!"
```

## Uninstalling

```bash
sudo /opt/joblet/scripts/uninstall.sh              # keep job logs and volumes
sudo /opt/joblet/scripts/uninstall.sh --purge      # remove everything
sudo /opt/joblet/scripts/uninstall.sh --purge --all-users   # also every user's ~/.rnx
```

Removed: the service and its `joblet.slice` cgroup, the package registration,
`/opt/joblet`, `/etc/joblet`, `/var/log/joblet`, `/var/lib/joblet`, the
installer-written `/etc/sysctl.d/99-joblet.conf` and
`/etc/modules-load.d/joblet.conf`, joblet symlinks and cert scripts in
`/usr/bin` and `/usr/local/bin`, joblet bridges, veths and iptables rules,
the joblet user and group, and `~/.rnx` for root and the invoking user.

Kept on purpose:

- A user-managed `/usr/local/bin/rnx`. Only an rnx that the installer
  downloaded is removed, and only if its sha256 still matches the one the
  installer recorded; a replaced binary stays.
- Loaded kernel modules (`br_netfilter`, `nf_conntrack`, `nf_nat`) until the
  next reboot; their persistent load config is removed.
- joblet entries in the shared systemd journal, which expire with journald
  retention (deleting them would also delete other services' logs).
- Other users' `~/.rnx` unless `--all-users` is given.

Safety: the script refuses any `JOBLET_HOME` outside `/opt/joblet*`, matches
iptables rules by interface flag rather than substring, skips network cleanup
when no joblet install is present, and with `--purge` ends by scanning the
host for leftovers, failing loudly if anything remains.

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