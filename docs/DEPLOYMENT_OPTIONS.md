# Joblet Deployment Options

## Overview

Joblet can be deployed in multiple ways depending on your infrastructure and preferences. This guide helps you choose
the right deployment method.

## Deployment Methods Comparison

| Method             | Best For                                  | Complexity | Time to Deploy | Automation |
|--------------------|-------------------------------------------|------------|----------------|------------|
| **Direct Package** | Bare metal, VMs, local dev, non-AWS cloud | Low        | 5 min          | Manual     |
| **EC2 User Data**  | AWS EC2 instances                         | Low        | 10 min         | Automated  |

## Method 1: Direct Package Installation

**Install Joblet directly on any Linux host using .deb or .rpm packages.**

### Supported Platforms

- Ubuntu 20.04+ / Debian 10+
- RHEL 8+ / CentOS Stream 8+
- Fedora 30+
- Amazon Linux 2/2023

### Use Cases

- Bare metal servers
- Virtual machines (VMware, VirtualBox, KVM, Hyper-V)
- Local development environments
- Non-AWS cloud providers (GCP, Azure, DigitalOcean, Linode)
- On-premises data centers
- Any Linux server with internet access

### Quick Start

**Debian/Ubuntu:**

```bash
# Find the latest release version at: https://github.com/ehsaniara/joblet/releases/latest
# Replace VERSION below with the actual version (e.g., 1.0.0)

# Download package
wget https://github.com/ehsaniara/joblet/releases/download/vVERSION/joblet_VERSION_amd64.deb

# Or for arm64:
# wget https://github.com/ehsaniara/joblet/releases/download/vVERSION/joblet_VERSION_arm64.deb

# Install
sudo dpkg -i joblet_VERSION_amd64.deb

# Start service
sudo systemctl start joblet

# Verify
sudo rnx job list
```

**RHEL/CentOS/Fedora/Amazon Linux:**

```bash
# Find the latest release version at: https://github.com/ehsaniara/joblet/releases/latest
# Replace VERSION below with the actual version (e.g., 1.0.0)

# Download package
wget https://github.com/ehsaniara/joblet/releases/download/vVERSION/joblet-VERSION-1.x86_64.rpm

# Or for aarch64:
# wget https://github.com/ehsaniara/joblet/releases/download/vVERSION/joblet-VERSION-1.aarch64.rpm

# Install (RHEL 8+, CentOS Stream 8+, Fedora, Amazon Linux 2023)
sudo dnf localinstall -y joblet-VERSION-1.x86_64.rpm

# Or for RHEL 7, CentOS 7, Amazon Linux 2:
# sudo yum localinstall -y joblet-VERSION-1.x86_64.rpm

# Start service
sudo systemctl start joblet

# Verify
sudo rnx job list
```

### Configuration

The installer supports environment variables for customization:

```bash
# Set configuration before installation
export JOBLET_SERVER_ADDRESS="0.0.0.0"
export JOBLET_SERVER_PORT="50051"
export JOBLET_CERT_INTERNAL_IP="192.168.1.100"
export JOBLET_CERT_PUBLIC_IP="203.0.113.10"
export JOBLET_CERT_DOMAIN="joblet.example.com"

# Then install
sudo dpkg -i joblet_*.deb
# or
sudo yum localinstall -y joblet-*.rpm
```

### Documentation

- **Debian/Ubuntu**: See [INSTALL_NOTES.md](INSTALL_NOTES.md)
- **RHEL/CentOS/Amazon Linux**: See [INSTALL_NOTES_RPM.md](INSTALL_NOTES_RPM.md)

### Pros

- Full control over installation
- Works on any Linux distribution
- No cloud dependencies
- Minimal external dependencies
- Can be used on-premises
- Simple and straightforward

### Cons

- Manual configuration required
- No automatic infrastructure provisioning
- Manual certificate management
- Manual security group / firewall setup
- No built-in CloudWatch integration (unless on EC2)

---

## Method 2: AWS EC2 User Data Script

**Automatically install Joblet when launching EC2 instances.**

### Supported Platforms

- Ubuntu 22.04 LTS (recommended)
- Ubuntu 20.04 LTS
- Debian 11/10
- Amazon Linux 2023 (recommended for AL)
- Amazon Linux 2
- RHEL 8+ / CentOS Stream 8+
- Fedora 30+

### Use Cases

- AWS EC2 deployments
- Quick AWS testing and prototyping
- Manual EC2 launches via Console or CLI
- Integration with existing AWS automation
- AMI baking workflows
- Auto Scaling Groups (with user data)

### Quick Start

**Via AWS Console:**

1. Launch EC2 instance
2. In **User Data** section:

```bash
#!/bin/bash
export JOBLET_SERVER_PORT="443"  # Use port 443 (HTTPS) - firewall-friendly!
export ENABLE_CLOUDWATCH="true"
curl -fsSL https://raw.githubusercontent.com/ehsaniara/joblet/main/scripts/ec2-user-data.sh -o /tmp/joblet-install.sh
chmod +x /tmp/joblet-install.sh
/tmp/joblet-install.sh 2>&1 | tee /var/log/joblet-install.log
```

3. Configure security group: Allow SSH (22) and HTTPS (443)
4. Launch instance

**Note**: Port 443 is typically allowed through corporate firewalls, making Joblet accessible from client machines
behind restrictive networks.

**Via AWS CLI:**

```bash
aws ec2 run-instances \
  --image-id ami-xxxxx \
  --instance-type t3.medium \
  --key-name my-key \
  --user-data file://user-data.sh \
  --iam-instance-profile Name=JobletCloudWatchRole
```

### Features

**Automatic Configuration:**

- OS auto-detection (Debian/Ubuntu vs RHEL/Amazon Linux)
- Package selection (.deb vs .rpm)
- EC2 metadata gathering (IPs, region, instance ID)
- TLS certificate generation with EC2 IPs
- CloudWatch Logs backend configuration
- Network isolation setup

**CloudWatch Integration:**

- Automatic IAM role detection
- Auto-configured log groups: `/joblet/job-*`, `/joblet/metrics`, `/joblet/server`
- Region auto-detection from EC2 metadata
- No additional configuration needed

### Environment Variables

```bash
export JOBLET_VERSION="latest"        # Joblet version to install
export JOBLET_SERVER_PORT="443"       # gRPC server port (443 recommended for EC2)
export ENABLE_CLOUDWATCH="true"       # Enable CloudWatch Logs
export JOBLET_CERT_DOMAIN=""          # Optional domain for certificates
```

**Port Selection:**

- **Port 443** (HTTPS): Recommended for EC2 - firewall-friendly, passes through corporate networks
- **Port 50051**: Standard gRPC port - use for internal/VPC-only deployments
- **Port 8443/9443**: Alternative ports if 443 is already in use

**Important**: Joblet requires a **dedicated EC2 instance** with the selected port available. Do not install on
instances running web servers (nginx, Apache) or other services that may conflict with the gRPC port.

### Documentation

See [AWS_DEPLOYMENT.md](AWS_DEPLOYMENT.md)

### Pros

- Simple one-step installation
- Auto-detects OS and installs correct package
- Automatic CloudWatch Logs configuration
- Certificates auto-generated with EC2 IPs
- No external tools required (just AWS Console/CLI)
- Works with Auto Scaling Groups
- IAM role integration
- EC2 metadata integration

### Cons

- AWS-only (not multi-cloud)
- Still requires manual EC2 setup (security groups, VPC, etc.)
- No infrastructure versioning
- Harder to replicate environments consistently
- Manual cleanup required

---

## Decision Tree

### Choose Direct Package If:

- [ ] Deploying on bare metal or VMs
- [ ] Not using AWS
- [ ] Using non-AWS cloud (GCP, Azure, DigitalOcean)
- [ ] On-premises deployment
- [ ] Need full control over installation
- [ ] Don't need infrastructure automation

### Choose EC2 User Data If:

- [ ] Deploying on AWS EC2
- [ ] Want automatic CloudWatch Logs integration
- [ ] Need EC2 metadata integration (IPs, region, etc.)
- [ ] Quick AWS testing/prototyping
- [ ] Using Auto Scaling Groups
- [ ] Don't want to manually configure AWS-specific features

---

## Feature Comparison

| Feature                      | Direct Package | EC2 User Data    |
|------------------------------|----------------|------------------|
| **OS support**               | All Linux      | All Linux on EC2 |
| **Cloud support**            | Any cloud      | AWS only         |
| **Auto EC2 provisioning**    | No             | No               |
| **Security group config**    | Manual         | Manual           |
| **IAM role integration**     | No             | Yes              |
| **CloudWatch auto-config**   | No             | Yes              |
| **Certificate auto-gen**     | Yes            | Yes              |
| **EC2 metadata integration** | No             | Yes              |
| **Version control**          | Manual         | Partial          |
| **Multi-environment**        | Manual         | Manual           |
| **On-premises support**      | Yes            | No               |
| **Learning curve**           | Low            | Low              |
| **Time to deploy**           | 5 min          | 10 min           |

---

## Hybrid Approaches

### Development + Production

**Development**: Direct package on local VM

```bash
# Quick local testing
sudo dpkg -i joblet.deb
```

**Production**: EC2 User Data on AWS

```bash
# Automated AWS deployment with CloudWatch (using port 443)
export JOBLET_SERVER_PORT="443"
export ENABLE_CLOUDWATCH="true"
curl -fsSL https://raw.githubusercontent.com/ehsaniara/joblet/main/scripts/ec2-user-data.sh | bash
```

### Multi-Cloud

**AWS**: EC2 User Data

```bash
# AWS with CloudWatch integration (port 443 for firewall compatibility)
export JOBLET_SERVER_PORT="443"
export ENABLE_CLOUDWATCH="true"
curl -fsSL https://raw.githubusercontent.com/ehsaniara/joblet/main/scripts/ec2-user-data.sh | bash
```

**GCP/Azure/On-prem**: Direct package

```bash
# Manual installation on other clouds
sudo dpkg -i joblet.deb
```

---

## Installation System Improvements

Both deployment methods benefit from recent installation improvements:

### Cross-Platform Support

- **Shared installation functions**: Single codebase for all platforms
- **Firewall support**: iptables, nftables, firewalld all supported
- **Network conflict detection**: Warns if 172.20.0.0/16 is in use
- **OS auto-detection**: Automatically selects .deb or .rpm

### Security

- **TLS mutual authentication**: Automatic certificate generation
- **Encrypted volumes**: EBS encryption on EC2
- **IAM roles**: CloudWatch Logs with least-privilege permissions
- **Network isolation**: Linux namespaces and bridge networking

### Documentation

- **Comprehensive guides**: Platform-specific installation docs
- **Troubleshooting**: Common issues with solutions
- **Security best practices**: Prominent warnings about system modifications
- **Distribution support**: Ubuntu, Debian, RHEL, CentOS, Fedora, Amazon Linux

See [INSTALLATION_IMPROVEMENTS.md](INSTALLATION_IMPROVEMENTS.md) for details.

---

## Distribution Support Matrix

| Distribution      | Package | Direct Install | EC2 User Data | Firewall           |
|-------------------|---------|----------------|---------------|--------------------|
| Ubuntu 20.04      | .deb    | Yes            | Yes           | iptables           |
| Ubuntu 22.04+     | .deb    | Yes            | Yes           | nftables           |
| Debian 10         | .deb    | Yes            | Yes           | iptables           |
| Debian 11+        | .deb    | Yes            | Yes           | nftables           |
| RHEL 7            | .rpm    | Yes            | Yes           | iptables           |
| RHEL 8+           | .rpm    | Yes            | Yes           | firewalld          |
| CentOS Stream 8+  | .rpm    | Yes            | Yes           | firewalld          |
| Fedora 30+        | .rpm    | Yes            | Yes           | firewalld/nftables |
| Amazon Linux 2    | .rpm    | Yes            | Yes           | iptables           |
| Amazon Linux 2023 | .rpm    | Yes            | Yes           | firewalld          |

All distributions benefit from the same improvements through shared installation functions.

---

## Cost Considerations

### Direct Package

- **Infrastructure cost**: Varies (your hardware or cloud provider)
- **Operational cost**: Higher (manual operations)
- **CloudWatch Logs**: Not available (unless on EC2)

### EC2 User Data

- **Infrastructure cost**: AWS EC2 rates (varies by instance type)
- **Operational cost**: Lower (automated installation)
- **CloudWatch Logs**: Per GB ingested + per GB/month stored

**Example resource usage (EC2):**

- t3.small instance: Suitable for dev/test
- t3.medium instance: Suitable for small production
- CloudWatch Logs (100 jobs/day × 1MB): ~3 GB/day ingestion, ~21 GB stored (7-day retention)

---

## Future Deployment Methods

The following methods are planned for future releases:

### Terraform Module (Future)

- Infrastructure as Code for AWS EC2
- Complete automation (EC2, security groups, IAM, Elastic IP)
- Multi-environment support
- State management for teams

### CloudFormation Template (Future)

- AWS-native Infrastructure as Code
- One-click deployment
- Service Catalog integration
- StackSets for multi-account

These will be introduced in a separate release. Star the GitHub repo to be notified!

---

## Next Steps

### For Direct Package Installation

→ See [INSTALL_NOTES.md](INSTALL_NOTES.md) (Debian/Ubuntu)
→ See [INSTALL_NOTES_RPM.md](INSTALL_NOTES_RPM.md) (RHEL/CentOS/Fedora/Amazon Linux)

### For AWS EC2 Deployments

→ See [AWS_DEPLOYMENT.md](AWS_DEPLOYMENT.md)

### For Installation System Details

→ See [INSTALLATION_IMPROVEMENTS.md](INSTALLATION_IMPROVEMENTS.md)

---

## Support

- **GitHub**: https://github.com/ehsaniara/joblet
- **Issues**: https://github.com/ehsaniara/joblet/issues
- **Documentation**: `/opt/joblet/docs/` (on installed instances)
