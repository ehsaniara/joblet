# Joblet Deployment Guide

This guide covers production deployment of the Joblet distributed job execution system, including server setup,
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

Joblet requires Linux for job execution due to its dependency on Linux-specific features:

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

RNX CLI clients can run on multiple platforms:

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

## Installation Methods

### Method 1: Debian Package (Recommended)

```bash
# Download latest release
wget https://github.com/ehsaniara/joblet/releases/latest/download/joblet_1.0.0_amd64.deb

# Install with dependencies
sudo dpkg -i joblet_1.0.0_amd64.deb

# Fix any dependency issues
sudo apt-get install -f

# Verify installation
systemctl status joblet
rnx --version
```

### Method 2: Manual Binary Installation

```bash
# Create user and directories
sudo useradd -r -s /bin/false -d /opt/joblet joblet
sudo mkdir -p /opt/joblet/{bin,certs,logs}
sudo mkdir -p /var/log/joblet

# Download and install binaries
wget https://github.com/ehsaniara/joblet/releases/latest/download/joblet-linux-amd64.tar.gz
tar -xzf joblet-linux-amd64.tar.gz

sudo mv joblet /opt/joblet/bin/
sudo mv rnx /usr/local/bin/
sudo chmod +x /opt/joblet/bin/joblet /usr/local/bin/rnx

# Set ownership
sudo chown -R joblet:joblet /opt/joblet
```

### Method 3: Build from Source

```bash
# Prerequisites
sudo apt-get install -y golang-go protobuf-compiler make git

# Clone and build
git clone https://github.com/ehsaniara/joblet.git
cd joblet
make build

# Install binaries
sudo cp bin/joblet /opt/joblet/bin/
sudo cp bin/rnx /usr/local/bin/
```

### Automated Deployment

```bash
# Using project Makefile (development to production)
git clone https://github.com/ehsaniara/joblet.git
cd joblet

# Configure target
export REMOTE_HOST=prod-joblet.example.com
export REMOTE_USER=deploy
export REMOTE_DIR=/opt/joblet

# Complete deployment
make setup-remote-passwordless

# Verify deployment
make service-status
make live-log
```

## Service Configuration

### Systemd Service

The Joblet service runs as a systemd daemon with proper cgroup delegation:

```ini
# /etc/systemd/system/joblet.service
[Unit]
Description=Joblet Job Execution Platform
Documentation=https://github.com/ehsaniara/joblet
After=network.target
Wants=network.target

[Service]
Type=simple
User=root
Group=root
WorkingDirectory=/opt/joblet

# Main service binary (single binary architecture)
ExecStart=/opt/joblet/joblet
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
ReadWritePaths=/opt/joblet /var/log/joblet /sys/fs/cgroup /proc /tmp

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
Slice=joblet.slice

# Environment configuration
Environment="JOBLET_MODE=server"
Environment="LOG_LEVEL=INFO"
Environment="JOBLET_CONFIG_PATH=/opt/joblet/config/joblet-config.yml"

# Resource limits for the main service
LimitNOFILE=65536
LimitNPROC=32768
LimitMEMLOCK=infinity

# Process cleanup
KillMode=mixed
KillSignal=SIGTERM
TimeoutStopSec=30s

# Cleanup job cgroups on service stop
ExecStopPost=/bin/bash -c 'find /sys/fs/cgroup/joblet.slice/joblet.service -name "job-*" -type d -exec rmdir {} \; 2>/dev/null || true'

[Install]
WantedBy=multi-user.target
```

### Configuration File

```yaml
# /opt/joblet/config/joblet-config.yml
version: "3.0"

server:
  address: "0.0.0.0"
  port: 50051
  mode: "server"
  timeout: "30s"

joblet:
  defaultCpuLimit: 100              # 100% of one core
  defaultMemoryLimit: 512           # 512MB memory limit
  defaultIoLimit: 0                 # Unlimited I/O
  maxConcurrentJobs: 100            # Maximum concurrent jobs
  jobTimeout: "1h"                  # 1-hour job timeout
  cleanupTimeout: "5s"              # Resource cleanup timeout
  validateCommands: true            # Enable command validation

security:
  serverCertPath: "/opt/joblet/certs/server-cert.pem"
  serverKeyPath: "/opt/joblet/certs/server-key.pem"
  caCertPath: "/opt/joblet/certs/ca-cert.pem"
  clientCertPath: "/opt/joblet/certs/client-cert.pem"
  clientKeyPath: "/opt/joblet/certs/client-key.pem"
  minTlsVersion: "1.3"

cgroup:
  baseDir: "/sys/fs/cgroup/joblet.slice/joblet.service"
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
sudo systemctl enable joblet.service
sudo systemctl start joblet.service

# Check service status
sudo systemctl status joblet.service --full

# Monitor logs
sudo journalctl -u joblet.service -f

# Performance monitoring
sudo systemctl show joblet.service --property=CPUUsageNSec
sudo systemctl show joblet.service --property=MemoryCurrent

# Restart service
sudo systemctl restart joblet.service

# Stop service (graceful)
sudo systemctl stop joblet.service
```

## Certificate Management

### Automated Certificate Generation

The Joblet includes comprehensive certificate management:

```bash
# Generate all certificates (CA, server, admin, viewer)
sudo /usr/local/bin/certs_gen_embedded.sh

# Certificate structure created:
# /opt/joblet/config/
# ├── joblet-config.yml           # Server config with embedded certificates
# └── rnx-config.yml             # Client config with embedded certificates
```

### Certificate Configuration

The certificate generation includes proper SAN (Subject Alternative Name) configuration:

```bash
# Server certificate includes multiple SANs
DNS.1 = joblet
DNS.2 = localhost
DNS.3 = joblet-server
IP.1 = 192.168.1.161
IP.2 = 127.0.0.1
IP.3 = 0.0.0.0

# Verify SAN configuration
openssl x509 -in /opt/joblet/certs/server-cert.pem -noout -text | grep -A 10 "Subject Alternative Name"
```

### Certificate Distribution

```bash
# For admin clients
scp server:/opt/joblet/config/rnx-config.yml ~/.rnx/

# For viewer clients (if separate viewer config exists)
scp server:/opt/joblet/config/rnx-config-viewer.yml ~/.rnx/rnx-config.yml

# Set proper permissions
chmod 600 ~/.rnx/rnx-config.yml
```

### Certificate Rotation

```bash
# Automated certificate rotation script
cat > /opt/joblet/scripts/rotate-certs.sh << 'EOF'
#!/bin/bash
set -e

CONFIG_DIR="/opt/joblet/config"
BACKUP_DIR="/opt/joblet/backups/certs-$(date +%Y%m%d-%H%M%S)"

echo "Starting certificate rotation..."

# Create backup
mkdir -p "$BACKUP_DIR"
cp "$CONFIG_DIR"/*.yml "$BACKUP_DIR/"
echo "Certificates backed up to $BACKUP_DIR"

# Generate new certificates
/usr/local/bin/certs_gen_embedded.sh

# Restart service to use new certificates
systemctl restart joblet.service

# Wait for service to start
sleep 5

# Verify service is running
if systemctl is-active --quiet joblet.service; then
    echo "Certificate rotation completed successfully"
    echo "Backup location: $BACKUP_DIR"
else
    echo "Service failed to start with new certificates"
    echo "Restoring from backup..."
    cp "$BACKUP_DIR"/*.yml "$CONFIG_DIR/"
    systemctl restart joblet.service
    exit 1
fi
EOF

chmod +x /opt/joblet/scripts/rotate-certs.sh

# Schedule rotation (monthly)
echo "0 2 1 * * /opt/joblet/scripts/rotate-certs.sh" | sudo crontab -u root -
```

## Monitoring & Observability

### Log Management

```bash
# Configure logrotate for Joblet logs
cat > /etc/logrotate.d/joblet << 'EOF'
/var/log/joblet/*.log {
    daily
    rotate 30
    compress
    delaycompress
    missingok
    notifempty
    create 644 joblet joblet
    postrotate
        systemctl reload joblet.service 2>/dev/null || true
    endscript
}

/opt/joblet/logs/*.log {
    daily
    rotate 7
    compress
    delaycompress
    missingok
    notifempty
    create 644 joblet joblet
}
EOF

# Test logrotate configuration
sudo logrotate -d /etc/logrotate.d/joblet
```

## Security

### System-Level Security

```bash
# Configure fail2ban for Joblet
cat > /etc/fail2ban/jail.d/joblet.conf << 'EOF'
[joblet]
enabled = true
port = 50051
filter = joblet
logpath = /var/log/joblet/joblet.log
maxretry = 5
bantime = 3600
findtime = 600
EOF

# Joblet fail2ban filter
cat > /etc/fail2ban/filter.d/joblet.conf << 'EOF'
[Definition]
failregex = .*authentication failed.*<HOST>.*
            .*certificate verification failed.*<HOST>.*
            .*unauthorized access attempt.*<HOST>.*
ignoreregex =
EOF

sudo systemctl restart fail2ban
```

### Network Security

```bash
# UFW firewall rules
sudo ufw default deny incoming
sudo ufw default allow outgoing

# Allow SSH (adjust port as needed)
sudo ufw allow 22/tcp

# Allow Joblet from specific networks only
sudo ufw allow from 192.168.0.0/16 to any port 50051
sudo ufw allow from 10.0.0.0/8 to any port 50051

# Enable firewall
sudo ufw --force enable
```

### File System Security

```bash
# Secure certificate permissions (embedded certificates)
sudo chmod 700 /opt/joblet/config
sudo chmod 600 /opt/joblet/config/*.yml

# Secure configuration
sudo chown joblet:joblet /opt/joblet/config/

# Secure log directories
sudo chmod 750 /var/log/joblet
sudo chown joblet:joblet /var/log/joblet

# Set SELinux contexts (RHEL/CentOS)
if command -v semanage >/dev/null 2>&1; then
    sudo semanage fcontext -a -t bin_t "/opt/joblet/joblet"
    sudo restorecon -v /opt/joblet/joblet
fi
```

## Scaling & Performance

```bash
# Optimize for high-concurrency workloads
cat >> /opt/joblet/config/joblet-config.yml << 'EOF'
joblet:
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
systemctl edit joblet.service
# Add:
# [Service]
# LimitNOFILE=1048576
# LimitNPROC=1048576
```

### Performance Monitoring

```bash
# Performance monitoring script
cat > /opt/joblet/scripts/performance-monitor.sh << 'EOF'
#!/bin/bash

echo "=== Joblet Performance Report ==="
echo "Timestamp: $(date)"
echo

# Service status
echo "Service Status:"
systemctl status joblet.service --no-pager -l | head -10

echo
echo "Resource Usage:"
echo "Memory: $(systemctl show joblet.service --property=MemoryCurrent --value | numfmt --to=iec)"
echo "Tasks: $(systemctl show joblet.service --property=TasksCurrent --value)"

echo
echo "Active Jobs:"
ACTIVE_JOBS=$(find /sys/fs/cgroup/joblet.slice/joblet.service -name "job-*" -type d 2>/dev/null | wc -l)
echo "Count: $ACTIVE_JOBS"

if [ "$ACTIVE_JOBS" -gt 0 ]; then
    echo "Job Resource Usage:"
    for job_cgroup in /sys/fs/cgroup/joblet.slice/joblet.service/job-*/; do
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
journalctl -u joblet.service --since "5 minutes ago" --no-pager | tail -5
EOF

chmod +x /opt/joblet/scripts/performance-monitor.sh
```

## Troubleshooting

### Common Issues and Solutions

#### 1. Service Won't Start

```bash
# Check detailed service status
sudo systemctl status joblet.service -l

# Check logs for errors
sudo journalctl -u joblet.service --since "1 hour ago" -f

# Common solutions:
# - Check certificate paths and permissions
# - Verify port availability: sudo netstat -tlnp | grep :50051
# - Check cgroup delegation: cat /sys/fs/cgroup/joblet.slice/cgroup.controllers
# - Verify binary permissions: ls -la /opt/joblet/joblet
```

#### 2. Certificate Issues

```bash
# Verify embedded certificates in config
grep -A 5 "BEGIN CERTIFICATE" /opt/joblet/config/joblet-config.yml

# Test TLS connection
rnx --config=/opt/joblet/config/rnx-config.yml list

# Regenerate certificates if needed
sudo /usr/local/bin/certs_gen_embedded.sh
sudo systemctl restart joblet.service
```

#### 3. Job Execution Issues

```bash
# Check cgroup delegation
cat /sys/fs/cgroup/joblet.slice/cgroup.controllers
cat /sys/fs/cgroup/joblet.slice/cgroup.subtree_control

# Verify namespace support
unshare --help | grep -E "(pid|mount|ipc|uts)"

# Check resource limits
cat /proc/sys/kernel/pid_max
cat /proc/sys/vm/max_map_count

# Debug job isolation
rnx run ps aux  # Should show limited process tree
rnx run mount   # Should show isolated mount namespace
```

#### 4. Performance Issues

```bash
# Monitor resource usage
top -p $(pgrep joblet)
sudo iotop -p $(pgrep joblet)

# Check job resource consumption
for job in /sys/fs/cgroup/joblet.slice/joblet.service/job-*/; do
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
sudo systemctl edit joblet.service
# Add:
# [Service]
# Environment=JOBLET_LOG_LEVEL=DEBUG

sudo systemctl restart joblet.service

# Monitor debug logs
sudo journalctl -u joblet.service -f | grep DEBUG
```

### Recovery Procedures

#### Emergency Service Recovery

```bash
# Stop all processes
sudo pkill -f joblet
sudo systemctl stop joblet.service

# Clean up cgroups
sudo find /sys/fs/cgroup/joblet.slice -name "job-*" -type d -exec rmdir {} \; 2>/dev/null

# Reset service state
sudo systemctl reset-failed joblet.service

# Restart service
sudo systemctl start joblet.service
```

## Backup & Recovery

### Configuration Backup

```bash
# Create backup script
cat > /opt/joblet/scripts/backup-config.sh << 'EOF'
#!/bin/bash
set -e

BACKUP_DIR="/opt/joblet/backups/$(date +%Y%m%d-%H%M%S)"
mkdir -p "$BACKUP_DIR"

# Backup configuration files with embedded certificates
cp /opt/joblet/config/*.yml "$BACKUP_DIR/"

# Backup service file
cp /etc/systemd/system/joblet.service "$BACKUP_DIR/"

# Create archive
tar -czf "/opt/joblet/backups/joblet-backup-$(date +%Y%m%d-%H%M%S).tar.gz" -C "$BACKUP_DIR" .

echo "Backup completed: $BACKUP_DIR"
EOF

chmod +x /opt/joblet/scripts/backup-config.sh

# Schedule daily backups
echo "0 3 * * * /opt/joblet/scripts/backup-config.sh" | sudo crontab -u root -
```

### Disaster Recovery

```bash
# Recovery script
cat > /opt/joblet/scripts/restore-config.sh << 'EOF'
#!/bin/bash
set -e

BACKUP_FILE="$1"
if [ -z "$BACKUP_FILE" ] || [ ! -f "$BACKUP_FILE" ]; then
    echo "Usage: $0 <backup-file.tar.gz>"
    exit 1
fi

RESTORE_DIR="/tmp/joblet-restore-$"
mkdir -p "$RESTORE_DIR"

# Extract backup
tar -xzf "$BACKUP_FILE" -C "$RESTORE_DIR"

# Stop service
systemctl stop joblet.service

# Restore configuration
cp "$RESTORE_DIR"/*.yml /opt/joblet/config/
cp "$RESTORE_DIR"/joblet.service /etc/systemd/system/

# Reload systemd and restart service
systemctl daemon-reload
systemctl start joblet.service

# Cleanup
rm -rf "$RESTORE_DIR"

echo "Configuration restored from $BACKUP_FILE"
EOF

chmod +x /opt/joblet/scripts/restore-config.sh
```