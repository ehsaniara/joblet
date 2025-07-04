# Worker Deployment Guide

This guide covers production deployment of the Worker distributed job execution system, including server setup,
certificate management, systemd service configuration, and operational procedures.

## Table of Contents

- [System Requirements](#system-requirements)
- [Architecture Deployment](#architecture-deployment)
- [Installation Methods](#installation-methods)
- [Service Configuration](#service-configuration)
- [Certificate Management](#certificate-management)
- [Monitoring & Observability](#monitoring--observability)
- [Security Hardening](#security-hardening)
- [Scaling & Performance](#scaling--performance)
- [Troubleshooting](#troubleshooting)
- [Backup & Recovery](#backup--recovery)

## System Requirements

### Server Requirements (Linux Only)

Worker requires Linux for job execution due to its dependency on Linux-specific features:

| Component            | Requirement                               | Notes                                      |
|----------------------|-------------------------------------------|--------------------------------------------|
| **Operating System** | Linux (Ubuntu 20.04+, CentOS 8+, RHEL 8+) | Kernel 4.6+ required for cgroup namespaces |
| **Kernel Features**  | cgroups v2, namespaces, systemd           | `CONFIG_CGROUPS=y`, `CONFIG_NAMESPACES=y`  |
| **Architecture**     | x86_64 (amd64) or ARM64                   | Single binary supports both                |
| **Memory**           | 2GB+ RAM (scales with concurrent jobs)    | ~2MB overhead per job                      |
| **Storage**          | 20GB+ available space                     | Logs, certificates, temporary files        |
| **Network**          | Port 50051 accessible                     | gRPC service port (configurable)           |
| **Privileges**       | Root access required                      | For cgroup and namespace management        |

### Client Requirements (Cross-Platform)

Worker CLI clients can run on multiple platforms:

| Platform | Status               | Installation Method       |
|----------|----------------------|---------------------------|
| Linux    | ✅ Full Support       | Package manager or binary |
| macOS    | ✅ CLI Only           | Binary download           |
| Windows  | ✅ CLI Only (via WSL) | WSL2 + Linux binary       |

### Kernel Verification

```bash
# Verify cgroups v2 support
mount | grep cgroup2
# Expected: cgroup2 on /sys/fs/cgroup type cgroup2

# Check namespace support
ls /proc/self/ns/
# Expected: cgroup, ipc, mnt, net, pid, user, uts

# Verify kernel version (4.6+ required)
uname -r
# Expected: 4.6.0 or higher

# Check systemd version (required for cgroup delegation)
systemctl --version
# Expected: systemd 219 or higher
```

## Architecture Deployment

### Single-Node Deployment

```
┌─────────────────────────────────────┐
│           Linux Server              │
├─────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐   │
│  │   Worker    │  │  Job Proc   │   │
│  │   Server    │  │  (init mode)│   │
│  │             │  │             │   │
│  │ • gRPC API  │  │ • Namespaces│   │
│  │ • Job Mgmt  │  │ • Cgroups   │   │
│  │ • Auth      │  │ • Isolation │   │
│  └─────────────┘  └─────────────┘   │
├─────────────────────────────────────┤
│         Linux Kernel                │
│  • cgroups v2  • namespaces         │
└─────────────────────────────────────┘

┌─────────────────┐    ┌─────────────────┐
│  Client (Any)   │    │  Client (Any)   │
│  • CLI Tool     │    │  • CLI Tool     │
│  • TLS Certs    │◄──►│  • TLS Certs    │
│  • gRPC Client  │    │  • gRPC Client  │
└─────────────────┘    └─────────────────┘
```

## Installation Methods

### Method 1: Debian Package (Recommended)

```bash
# Download latest release
wget https://github.com/ehsaniara/worker/releases/latest/download/worker_1.0.0_amd64.deb

# Install with dependencies
sudo dpkg -i worker_1.0.0_amd64.deb

# Fix any dependency issues
sudo apt-get install -f

# Verify installation
systemctl status worker
worker-cli --version
```

### Method 2: Manual Binary Installation

```bash
# Create user and directories
sudo useradd -r -s /bin/false -d /opt/worker worker
sudo mkdir -p /opt/worker/{bin,certs,logs}
sudo mkdir -p /var/log/worker

# Download and install binaries
wget https://github.com/ehsaniara/worker/releases/latest/download/worker-linux-amd64.tar.gz
tar -xzf worker-linux-amd64.tar.gz

sudo mv worker /opt/worker/bin/
sudo mv worker-cli /usr/local/bin/
sudo chmod +x /opt/worker/bin/worker /usr/local/bin/worker-cli

# Set ownership
sudo chown -R worker:worker /opt/worker
```

### Method 3: Build from Source

```bash
# Prerequisites
sudo apt-get install -y golang-go protobuf-compiler make git

# Clone and build
git clone https://github.com/ehsaniara/worker.git
cd worker
make build

# Install binaries
sudo cp bin/worker /opt/worker/bin/
sudo cp bin/worker-cli /usr/local/bin/
```

### Automated Deployment

```bash
# Using project Makefile (development to production)
git clone https://github.com/ehsaniara/worker.git
cd worker

# Configure target
export REMOTE_HOST=prod-worker.example.com
export REMOTE_USER=deploy
export REMOTE_DIR=/opt/worker

# Complete deployment
make setup-remote-passwordless

# Verify deployment
make service-status
make live-log
```

## Service Configuration

### Systemd Service

The Worker service runs as a systemd daemon with proper cgroup delegation:

```ini
# /etc/systemd/system/worker.service
[Unit]
Description=Worker Job Execution Platform
Documentation=https://github.com/ehsaniara/worker
After=network.target
Wants=network.target

[Service]
Type=simple
User=root
Group=root
WorkingDirectory=/opt/worker

# Main service binary (single binary architecture)
ExecStart=/opt/worker/worker
ExecReload=/bin/kill -HUP $MAINPID

# Process management
Restart=always
RestartSec=10s
TimeoutStartSec=30s
TimeoutStopSec=30s

# CRITICAL: Allow new privileges for namespace operations
NoNewPrivileges=no

# Security hardening while maintaining isolation capabilities
PrivateTmp=yes
ProtectHome=yes
ReadWritePaths=/opt/worker /var/log/worker /sys/fs/cgroup /proc /tmp

# CRITICAL: Disable protections that block namespace operations
ProtectSystem=no
PrivateDevices=no
ProtectKernelTunables=no
ProtectControlGroups=no
RestrictRealtime=no
RestrictSUIDSGID=no
MemoryDenyWriteExecute=no

# CRITICAL: Cgroup delegation for job resource management
Delegate=yes
DelegateControllers=cpu memory io pids
CPUAccounting=yes
MemoryAccounting=yes
IOAccounting=yes
TasksAccounting=yes
Slice=worker.slice

# Environment configuration
Environment="WORKER_MODE=server"
Environment="LOG_LEVEL=INFO"
Environment="WORKER_CONFIG_PATH=/opt/worker/config.yml"

# Resource limits for the main service
LimitNOFILE=65536
LimitNPROC=32768
LimitMEMLOCK=infinity

# Process cleanup
KillMode=mixed
KillSignal=SIGTERM
TimeoutStopSec=30s

# Cleanup job cgroups on service stop
ExecStopPost=/bin/bash -c 'find /sys/fs/cgroup/worker.slice/worker.service -name "job-*" -type d -exec rmdir {} \; 2>/dev/null || true'

[Install]
WantedBy=multi-user.target
```

### Configuration File

```yaml
# /opt/worker/server-config.yml
version: "3.0"

server:
  address: "0.0.0.0"
  port: 50051
  mode: "server"
  timeout: "30s"

worker:
  defaultCpuLimit: 100              # 100% of one core
  defaultMemoryLimit: 512           # 512MB memory limit
  defaultIoLimit: 0                 # Unlimited I/O
  maxConcurrentJobs: 100            # Maximum concurrent jobs
  jobTimeout: "1h"                  # 1-hour job timeout
  cleanupTimeout: "5s"              # Resource cleanup timeout
  validateCommands: true            # Enable command validation

security:
  serverCertPath: "/opt/worker/certs/server-cert.pem"
  serverKeyPath: "/opt/worker/certs/server-key.pem"
  caCertPath: "/opt/worker/certs/ca-cert.pem"
  clientCertPath: "/opt/worker/certs/client-cert.pem"
  clientKeyPath: "/opt/worker/certs/client-key.pem"
  minTlsVersion: "1.3"

cgroup:
  baseDir: "/sys/fs/cgroup/worker.slice/worker.service"
  namespaceMount: "/sys/fs/cgroup"
  enableControllers: [ "cpu", "memory", "io", "pids" ]
  cleanupTimeout: "5s"

grpc:
  maxRecvMsgSize: 524288            # 512KB
  maxSendMsgSize: 4194304           # 4MB
  maxHeaderListSize: 1048576        # 1MB
  keepAliveTime: "30s"
  keepAliveTimeout: "5s"

logging:
  level: "INFO"                     # DEBUG, INFO, WARN, ERROR
  format: "text"                    # text or json
  output: "stdout"                  # stdout, stderr, or file path
```

### Service Management

```bash
# Enable and start service
sudo systemctl daemon-reload
sudo systemctl enable worker.service
sudo systemctl start worker.service

# Check service status
sudo systemctl status worker.service --full

# Monitor logs
sudo journalctl -u worker.service -f

# Performance monitoring
sudo systemctl show worker.service --property=CPUUsageNSec
sudo systemctl show worker.service --property=MemoryCurrent

# Restart service
sudo systemctl restart worker.service

# Stop service (graceful)
sudo systemctl stop worker.service
```

## Certificate Management

### Automated Certificate Generation

The Worker includes comprehensive certificate management:

```bash
# Generate all certificates (CA, server, admin, viewer)
sudo /usr/local/bin/certs_gen.sh

# Certificate structure created:
# /opt/worker/certs/
# ├── ca-cert.pem              # Certificate Authority
# ├── ca-key.pem               # CA private key (secure)
# ├── server-cert.pem          # Server certificate with SAN
# ├── server-key.pem           # Server private key (secure)
# ├── admin-client-cert.pem    # Admin client certificate (OU=admin)
# ├── admin-client-key.pem     # Admin client private key (secure)
# ├── viewer-client-cert.pem   # Viewer client certificate (OU=viewer)
# └── viewer-client-key.pem    # Viewer client private key (secure)
```

### Certificate Configuration

The certificate generation includes proper SAN (Subject Alternative Name) configuration:

```bash
# Server certificate includes multiple SANs
DNS.1 = worker
DNS.2 = localhost
DNS.3 = worker-server
IP.1 = 192.168.1.161
IP.2 = 127.0.0.1
IP.3 = 0.0.0.0

# Verify SAN configuration
openssl x509 -in /opt/worker/certs/server-cert.pem -noout -text | grep -A 10 "Subject Alternative Name"
```

### Certificate Distribution

```bash
# For admin clients
scp server:/opt/worker/certs/ca-cert.pem ~/.worker/
scp server:/opt/worker/certs/admin-client-cert.pem ~/.worker/client-cert.pem
scp server:/opt/worker/certs/admin-client-key.pem ~/.worker/client-key.pem

# For viewer clients
scp server:/opt/worker/certs/ca-cert.pem ~/.worker/
scp server:/opt/worker/certs/viewer-client-cert.pem ~/.worker/client-cert.pem
scp server:/opt/worker/certs/viewer-client-key.pem ~/.worker/client-key.pem

# Set proper permissions
chmod 600 ~/.worker/client-key.pem
chmod 644 ~/.worker/client-cert.pem ~/.worker/ca-cert.pem
```

### Certificate Rotation

```bash
# Automated certificate rotation script
cat > /opt/worker/scripts/rotate-certs.sh << 'EOF'
#!/bin/bash
set -e

CERT_DIR="/opt/worker/certs"
BACKUP_DIR="/opt/worker/backups/certs-$(date +%Y%m%d-%H%M%S)"

echo "Starting certificate rotation..."

# Create backup
mkdir -p "$BACKUP_DIR"
cp "$CERT_DIR"/*.pem "$BACKUP_DIR/"
echo "Certificates backed up to $BACKUP_DIR"

# Generate new certificates
/usr/local/bin/certs_gen.sh

# Verify new certificates
openssl verify -CAfile "$CERT_DIR/ca-cert.pem" "$CERT_DIR/server-cert.pem"
openssl verify -CAfile "$CERT_DIR/ca-cert.pem" "$CERT_DIR/admin-client-cert.pem"
openssl verify -CAfile "$CERT_DIR/ca-cert.pem" "$CERT_DIR/viewer-client-cert.pem"

# Restart service to use new certificates
systemctl restart worker.service

# Wait for service to start
sleep 5

# Verify service is running
if systemctl is-active --quiet worker.service; then
    echo "Certificate rotation completed successfully"
    echo "Backup location: $BACKUP_DIR"
else
    echo "Service failed to start with new certificates"
    echo "Restoring from backup..."
    cp "$BACKUP_DIR"/*.pem "$CERT_DIR/"
    systemctl restart worker.service
    exit 1
fi
EOF

chmod +x /opt/worker/scripts/rotate-certs.sh

# Schedule rotation (monthly)
echo "0 2 1 * * /opt/worker/scripts/rotate-certs.sh" | sudo crontab -u root -
```

## Monitoring & Observability

### Log Management

optional

```bash
# Configure logrotate for Worker logs
cat > /etc/logrotate.d/worker << 'EOF'
/var/log/worker/*.log {
    daily
    rotate 30
    compress
    delaycompress
    missingok
    notifempty
    create 644 worker worker
    postrotate
        systemctl reload worker.service 2>/dev/null || true
    endscript
}

/opt/worker/logs/*.log {
    daily
    rotate 7
    compress
    delaycompress
    missingok
    notifempty
    create 644 worker worker
}
EOF

# Test logrotate configuration
sudo logrotate -d /etc/logrotate.d/worker
```

## Security

### System-Level Security

optional

```bash
# Configure fail2ban for Worker
cat > /etc/fail2ban/jail.d/worker.conf << 'EOF'
[worker]
enabled = true
port = 50051
filter = worker
logpath = /var/log/worker/worker.log
maxretry = 5
bantime = 3600
findtime = 600
EOF

# Worker fail2ban filter
cat > /etc/fail2ban/filter.d/worker.conf << 'EOF'
[Definition]
failregex = .*authentication failed.*<HOST>.*
            .*certificate verification failed.*<HOST>.*
            .*unauthorized access attempt.*<HOST>.*
ignoreregex =
EOF

sudo systemctl restart fail2ban
```

### Network Security

example

```bash
# UFW firewall rules
sudo ufw default deny incoming
sudo ufw default allow outgoing

# Allow SSH (adjust port as needed)
sudo ufw allow 22/tcp

# Allow Worker from specific networks only
sudo ufw allow from 192.168.0.0/16 to any port 50051
sudo ufw allow from 10.0.0.0/8 to any port 50051

# Enable firewall
sudo ufw --force enable
```

### File System Security

```bash
# Secure certificate permissions
sudo chmod 700 /opt/worker/certs
sudo chmod 600 /opt/worker/certs/*-key.pem
sudo chmod 644 /opt/worker/certs/*-cert.pem

# Secure configuration
sudo chmod 600 /opt/worker/server-config.yml
sudo chown worker:worker /opt/worker/server-config.yml

# Secure log directories
sudo chmod 750 /var/log/worker
sudo chown worker:worker /var/log/worker

# Set SELinux contexts (RHEL/CentOS)
if command -v semanage >/dev/null 2>&1; then
    sudo semanage fcontext -a -t bin_t "/opt/worker/worker"
    sudo restorecon -v /opt/worker/worker
fi
```

## Scaling & Performance

```bash
# Optimize for high-concurrency workloads
cat >> /opt/worker/server-config.yml << 'EOF'
worker:
  maxConcurrentJobs: 500          # Increase concurrent job limit
  jobTimeout: "30m"               # Shorter timeout for faster turnover

grpc:
  maxRecvMsgSize: 1048576         # 1MB
  maxSendMsgSize: 8388608         # 8MB
  keepAliveTime: "10s"            # More frequent keep-alives

cgroup:
  cleanupTimeout: "2s"            # Faster cleanup
EOF

# System-level optimizations
echo 'net.core.somaxconn = 65535' >> /etc/sysctl.conf
echo 'fs.file-max = 2097152' >> /etc/sysctl.conf
echo 'kernel.pid_max = 4194304' >> /etc/sysctl.conf
sysctl -p

# Service-level optimizations
systemctl edit worker.service
# Add:
# [Service]
# LimitNOFILE=1048576
# LimitNPROC=1048576
```

### Performance Monitoring

```bash
# Performance monitoring script
cat > /opt/worker/scripts/performance-monitor.sh << 'EOF'
#!/bin/bash

echo "=== Worker Performance Report ==="
echo "Timestamp: $(date)"
echo

# Service status
echo "Service Status:"
systemctl status worker.service --no-pager -l | head -10

echo
echo "Resource Usage:"
echo "Memory: $(systemctl show worker.service --property=MemoryCurrent --value | numfmt --to=iec)"
echo "Tasks: $(systemctl show worker.service --property=TasksCurrent --value)"

echo
echo "Active Jobs:"
ACTIVE_JOBS=$(find /sys/fs/cgroup/worker.slice/worker.service -name "job-*" -type d 2>/dev/null | wc -l)
echo "Count: $ACTIVE_JOBS"

if [ "$ACTIVE_JOBS" -gt 0 ]; then
    echo "Job Resource Usage:"
    for job_cgroup in /sys/fs/cgroup/worker.slice/worker.service/job-*/; do
        if [ -d "$job_cgroup" ]; then
            job_id=$(basename "$job_cgroup")
            memory=$(cat "$job_cgroup/memory.current" 2>/dev/null | numfmt --to=iec)
            echo "  $job_id: Memory=$memory"
        fi
    done
fi

echo
echo "Network Connections:"
ss -tlnp | grep :50051

echo
echo "Recent Log Entries:"
journalctl -u worker.service --since "5 minutes ago" --no-pager | tail -5
EOF

chmod +x /opt/worker/scripts/performance-monitor.sh
```

## Troubleshooting

### Common Issues and Solutions

#### 1. Service Won't Start

```bash
# Check detailed service status
sudo systemctl status worker.service -l

# Check logs for errors
sudo journalctl -u worker.service --since "1 hour ago" -f

# Common solutions:
# - Check certificate paths and permissions
# - Verify port availability: sudo netstat -tlnp | grep :50051
# - Check cgroup delegation: cat /sys/fs/cgroup/worker.slice/cgroup.controllers
# - Verify binary permissions: ls -la /opt/worker/worker
```

#### 2. Certificate Issues

```bash
# Verify certificate validity
openssl x509 -in /opt/worker/certs/server-cert.pem -noout -text
openssl verify -CAfile /opt/worker/certs/ca-cert.pem /opt/worker/certs/server-cert.pem

# Check certificate expiration
openssl x509 -in /opt/worker/certs/server-cert.pem -noout -dates

# Test TLS connection
openssl s_client -connect localhost:50051 -cert /opt/worker/certs/admin-client-cert.pem -key /opt/worker/certs/admin-client-key.pem

# Regenerate certificates if needed
sudo /usr/local/bin/certs_gen.sh
sudo systemctl restart worker.service
```

#### 3. Job Execution Issues

```bash
# Check cgroup delegation
cat /sys/fs/cgroup/worker.slice/cgroup.controllers
cat /sys/fs/cgroup/worker.slice/cgroup.subtree_control

# Verify namespace support
unshare --help | grep -E "(pid|mount|ipc|uts)"

# Check resource limits
cat /proc/sys/kernel/pid_max
cat /proc/sys/vm/max_map_count

# Debug job isolation
worker-cli run ps aux  # Should show limited process tree
worker-cli run mount   # Should show isolated mount namespace
```

#### 4. Performance Issues

```bash
# Monitor resource usage
top -p $(pgrep worker)
sudo iotop -p $(pgrep worker)

# Check job resource consumption
for job in /sys/fs/cgroup/worker.slice/worker.service/job-*/; do
    echo "Job: $(basename $job)"
    echo "  Memory: $(cat $job/memory.current 2>/dev/null | numfmt --to=iec)"
    echo "  CPU: $(cat $job/cpu.stat 2>/dev/null | grep usage_usec)"
done

# Optimize configuration
# - Reduce job timeout
# - Increase cleanup timeout
# - Adjust concurrent job limits
```

### Debug Mode

```bash
# Enable debug logging
sudo systemctl edit worker.service
# Add:
# [Service]
# Environment=LOG_LEVEL=DEBUG

sudo systemctl restart worker.service

# Monitor debug logs
sudo journalctl -u worker.service -f | grep DEBUG
```

### Recovery Procedures

#### Emergency Service Recovery

```bash
# Stop all processes
sudo pkill -f worker
sudo systemctl stop worker.service

# Clean up cgroups
sudo find /sys/fs/cgroup/worker.slice -name "job-*" -type d -exec rmdir {} \; 2>/dev/null

# Reset service state
sudo systemctl reset-failed worker.service

# Restart service
sudo systemctl start worker.service
```
