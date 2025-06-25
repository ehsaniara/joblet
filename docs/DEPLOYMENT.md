# Job Worker Deployment Guide

This guide covers production deployment of the Job Worker system, including server setup, certificate management,
systemd service configuration, and operational procedures.

## Table of Contents

- [System Requirements](#system-requirements)
- [Pre-deployment Setup](#pre-deployment-setup)
- [Automated Deployment](#automated-deployment)
- [Manual Deployment](#manual-deployment)
- [Service Configuration](#service-configuration)
- [Certificate Management](#certificate-management)
- [Monitoring & Maintenance](#monitoring--maintenance)
- [Security Considerations](#security-considerations)
- [Troubleshooting](#troubleshooting)
- [Backup & Recovery](#backup--recovery)

## System Requirements

### Server Requirements

| Component            | Requirement                               | Notes                            |
|----------------------|-------------------------------------------|----------------------------------|
| **Operating System** | Linux (Ubuntu 20.04+, CentOS 8+, RHEL 8+) | Requires systemd                 |
| **Kernel Version**   | 4.5+                                      | For cgroups v2 support           |
| **Architecture**     | x86_64 (amd64) or ARM64                   |                                  |
| **Memory**           | 2GB+ RAM                                  | Scales with concurrent jobs      |
| **Storage**          | 10GB+ available space                     | For binaries, logs, certificates |
| **Network**          | Port 50051 accessible                     | gRPC service port                |
| **Privileges**       | Root access required                      | For cgroup management            |

### Software Dependencies

```bash
# Ubuntu/Debian
sudo apt update
sudo apt install -y openssl systemd

# CentOS/RHEL
sudo yum install -y openssl systemd

# Verify cgroups v2 support
mount | grep cgroup2
# Should show: cgroup2 on /sys/fs/cgroup type cgroup2
```

### Network Requirements

```bash
# Firewall configuration
sudo ufw allow 50051/tcp  # Ubuntu
sudo firewall-cmd --permanent --add-port=50051/tcp  # CentOS/RHEL
sudo firewall-cmd --reload
```

## Pre-deployment Setup

### 1. Create System User

```bash
# Create dedicated user for job-worker
sudo useradd -r -s /bin/false -d /opt/job-worker job-worker
sudo mkdir -p /opt/job-worker
sudo chown job-worker:job-worker /opt/job-worker
```

### 2. Directory Structure

```bash
# Create required directories
sudo mkdir -p /opt/job-worker/{bin,certs,logs}
sudo mkdir -p /var/log/job-worker
sudo mkdir -p /etc/job-worker

# Set permissions
sudo chown -R job-worker:job-worker /opt/job-worker
sudo chown job-worker:job-worker /var/log/job-worker
```

### 3. Configure SSH Access

For automated deployment, configure passwordless SSH and sudo:

```bash
# On deployment machine, generate SSH key if needed
ssh-keygen -t ed25519 -C "job-worker-deployment"

# Copy public key to server
ssh-copy-id user@your-server.com

# Configure passwordless sudo on server
echo "your-username ALL=(ALL) NOPASSWD: ALL" | sudo tee /etc/sudoers.d/job-worker-deploy
```

## Automated Deployment

### Quick Setup (Recommended)

Use the Makefile for automated deployment:

```bash
# Clone repository
git clone https://github.com/ehsaniara/job-worker.git
cd job-worker

# Configure deployment target
export REMOTE_HOST=your-server.com
export REMOTE_USER=your-username
export REMOTE_DIR=/opt/job-worker

# Complete automated setup
make setup-remote-passwordless
```

This will:

1. Generate TLS certificates on the server
2. Build and deploy binaries
3. Configure systemd service
4. Start the service

### Step-by-Step Automated Deployment

```bash
# 1. Build binaries locally
make worker init

# 2. Generate certificates on remote server
make certs-remote-passwordless

# 3. Deploy binaries
make deploy-passwordless

# 4. Verify deployment
make service-status
make live-log
```

### Safe Deployment (With Password Prompts)

For production environments where passwordless sudo is not configured:

```bash
# Deploy with password prompts
make deploy-safe REMOTE_HOST=prod.example.com

# Download admin certificates for CLI access
make certs-download-admin-simple
```

## Manual Deployment

### 1. Build Binaries

```bash
# On development machine
git clone https://github.com/ehsaniara/job-worker.git
cd job-worker

# Build Linux binaries
make worker init
# Creates: bin/job-worker, bin/job-init
```

### 2. Transfer Binaries

```bash
# Copy binaries to server
scp bin/job-worker user@server:/tmp/
scp bin/job-init user@server:/tmp/

# On server, install binaries
sudo cp /tmp/job-worker /opt/job-worker/
sudo cp /tmp/job-init /opt/job-worker/
sudo chmod +x /opt/job-worker/job-worker /opt/job-worker/job-init
sudo chown job-worker:job-worker /opt/job-worker/job-*
```

### 3. Generate Certificates

```bash
# Copy certificate generation script
scp etc/certs_gen.sh user@server:/tmp/

# On server, generate certificates
sudo /tmp/certs_gen.sh
sudo chown -R job-worker:job-worker /opt/job-worker/certs
```

## Service Configuration

### 1. Create Systemd Service

Create `/etc/systemd/system/job-worker.service`:

```ini
[Unit]
Description=Job Worker Service
Documentation=https://github.com/ehsaniara/job-worker
After=network.target
Wants=network.target

[Service]
Type=simple
User=root
Group=root
WorkingDirectory=/opt/job-worker
ExecStart=/opt/job-worker/job-worker
ExecReload=/bin/kill -HUP $MAINPID
Restart=always
RestartSec=5
TimeoutStopSec=30

# Security settings
NoNewPrivileges=false
PrivateTmp=true
ProtectSystem=strict
ReadWritePaths=/opt/job-worker /var/log/job-worker /sys/fs/cgroup

# Environment
Environment=WORKER_ADDR=0.0.0.0:50051
Environment=WORKER_CERT_PATH=/opt/job-worker/certs
Environment=WORKER_LOG_LEVEL=info

# Resource limits
LimitNOFILE=65536
LimitNPROC=32768

# Cgroup settings
Delegate=yes
KillMode=mixed

[Install]
WantedBy=multi-user.target
```

### 2. Enable and Start Service

```bash
# Reload systemd configuration
sudo systemctl daemon-reload

# Enable service to start on boot
sudo systemctl enable worker.service

# Start service
sudo systemctl start worker.service

# Check status
sudo systemctl status worker.service
```

### 3. Configure Logging

Create `/etc/rsyslog.d/job-worker.conf`:

```bash
# Job Worker logging configuration
if $programname == 'job-worker' then /var/log/job-worker/job-worker.log
& stop
```

Restart rsyslog:

```bash
sudo systemctl restart rsyslog
```

## Certificate Management

### Automated Certificate Generation

The project includes an automated certificate generation script:

```bash
# Generate all certificates (server + clients)
sudo /opt/job-worker/etc/certs_gen.sh

# Certificate files created:
# /opt/job-worker/certs/ca-cert.pem       # Certificate Authority
# /opt/job-worker/certs/ca-key.pem        # CA private key
# /opt/job-worker/certs/server-cert.pem   # Server certificate
# /opt/job-worker/certs/server-key.pem    # Server private key
# /opt/job-worker/certs/admin-client-cert.pem   # Admin client cert
# /opt/job-worker/certs/admin-client-key.pem    # Admin client key
# /opt/job-worker/certs/viewer-client-cert.pem  # Viewer client cert
# /opt/job-worker/certs/viewer-client-key.pem   # Viewer client key
```

### Certificate Rotation

For production environments, implement regular certificate rotation:

```bash
# Create certificate rotation script
cat > /opt/job-worker/scripts/rotate-certs.sh << 'EOF'
#!/bin/bash
# Certificate rotation script

CERT_DIR="/opt/job-worker/certs"
BACKUP_DIR="/opt/job-worker/backups/certs-$(date +%Y%m%d)"

# Backup existing certificates
mkdir -p "$BACKUP_DIR"
cp "$CERT_DIR"/*.pem "$BACKUP_DIR/"

# Generate new certificates
/opt/job-worker/etc/certs_gen.sh

# Restart service
systemctl restart job-worker.service

echo "Certificate rotation completed: $(date)"
EOF

chmod +x /opt/job-worker/scripts/rotate-certs.sh
```

### Certificate Validation

```bash
# Validate certificate chain
openssl verify -CAfile /opt/job-worker/certs/ca-cert.pem \
  /opt/job-worker/certs/server-cert.pem

# Check certificate expiration
openssl x509 -in /opt/job-worker/certs/server-cert.pem -noout -dates

# Test TLS connection
openssl s_client -connect localhost:50051 -verify_return_error
```

## Monitoring & Maintenance

### Health Checks

```bash
# Service status
sudo systemctl is-active worker.service

# Process check
pgrep -f job-worker

# Port check
netstat -tlnp | grep :50051

# Log tail
sudo journalctl -u worker.service -f
```

### Log Management

```bash
# Configure log rotation
cat > /etc/logrotate.d/job-worker << 'EOF'
/var/log/job-worker/*.log {
    daily
    rotate 30
    compress
    delaycompress
    missingok
    notifempty
    create 644 job-worker job-worker
    postrotate
        systemctl reload job-worker.service
    endscript
}
EOF
```

### Performance Monitoring

```bash
# Monitor resource usage
top -p $(pgrep job-worker)

# Check cgroup usage
cat /sys/fs/cgroup/job-*/memory.current
cat /sys/fs/cgroup/job-*/cpu.stat

# Network connections
ss -tlnp | grep :50051
```

### Backup Script

```bash
cat > /opt/job-worker/scripts/backup.sh << 'EOF'
#!/bin/bash
# Job Worker backup script

BACKUP_DIR="/opt/job-worker/backups/$(date +%Y%m%d-%H%M%S)"
mkdir -p "$BACKUP_DIR"

# Backup configuration and certificates
cp -r /opt/job-worker/certs "$BACKUP_DIR/"
cp /etc/systemd/system/job-worker.service "$BACKUP_DIR/"

# Backup logs (last 7 days)
find /var/log/job-worker -name "*.log" -mtime -7 -exec cp {} "$BACKUP_DIR/" \;

# Create tarball
tar -czf "$BACKUP_DIR.tar.gz" -C /opt/job-worker/backups "$(basename $BACKUP_DIR)"
rm -rf "$BACKUP_DIR"

echo "Backup created: $BACKUP_DIR.tar.gz"
EOF

chmod +x /opt/job-worker/scripts/backup.sh
```

## Security Considerations

### Firewall Configuration

```bash
# UFW (Ubuntu)
sudo ufw allow from trusted.client.ip to any port 50051
sudo ufw deny 50051

# iptables
sudo iptables -A INPUT -p tcp --dport 50051 -s trusted.client.ip -j ACCEPT
sudo iptables -A INPUT -p tcp --dport 50051 -j DROP
```

### File Permissions

```bash
# Secure certificate permissions
sudo chmod 600 /opt/job-worker/certs/*-key.pem
sudo chmod 644 /opt/job-worker/certs/*-cert.pem
sudo chown job-worker:job-worker /opt/job-worker/certs/*
```

### SELinux Configuration (RHEL/CentOS)

```bash
# Configure SELinux for job-worker
sudo setsebool -P container_manage_cgroup true
sudo semanage fcontext -a -t bin_t "/opt/job-worker/job-worker"
sudo restorecon -v /opt/job-worker/job-worker
```

### Audit Logging

```bash
# Enable audit logging for job-worker
echo "-w /opt/job-worker/ -p wa -k job-worker" >> /etc/audit/rules.d/job-worker.rules
sudo systemctl restart auditd
```

## Troubleshooting

### Common Issues

#### Service Won't Start

```bash
# Check systemd status
sudo systemctl status worker.service -l

# Check logs
sudo journalctl -u worker.service --since "1 hour ago"

# Common causes:
# 1. Missing certificates
# 2. Port already in use
# 3. Permission issues
# 4. Cgroups not available
```

#### Certificate Issues

```bash
# Verify certificate chain
make verify-cert-chain

# Check certificate expiration
make examine-certs

# Regenerate certificates
make certs-remote-passwordless
```

#### Connection Issues

```bash
# Test network connectivity
telnet your-server.com 50051

# Test TLS connection
make test-tls

# Check firewall
sudo ufw status
sudo iptables -L
```

#### Performance Issues

```bash
# Check system resources
htop
df -h
free -h

# Check cgroup limits
cat /sys/fs/cgroup/memory.stat
cat /sys/fs/cgroup/cpu.stat

# Monitor job processes
ps aux | grep job-
```

### Debug Mode

Enable debug logging for troubleshooting:

```bash
# Edit systemd service
sudo systemctl edit worker.service

# Add debug environment
[Service]
Environment=WORKER_LOG_LEVEL=debug

# Restart service
sudo systemctl restart worker.service
```

### Emergency Procedures

#### Service Recovery

```bash
# Stop all job processes
sudo pkill -f job-worker
sudo systemctl stop worker.service

# Clean up cgroups
sudo find /sys/fs/cgroup -name "job-*" -type d -exec rmdir {} \; 2>/dev/null

# Restart service
sudo systemctl start worker.service
```

#### Certificate Recovery

```bash
# Backup corrupted certificates
sudo mv /opt/job-worker/certs /opt/job-worker/certs.backup

# Regenerate certificates
sudo /opt/job-worker/etc/certs_gen.sh

# Restart service
sudo systemctl restart worker.service
```

## Backup & Recovery

### Automated Backup

Set up daily backups with cron:

```bash
# Add to crontab
echo "0 2 * * * /opt/job-worker/scripts/backup.sh" | sudo crontab -u job-worker -
```

### Disaster Recovery

```bash
# 1. Stop service
sudo systemctl stop worker.service

# 2. Restore from backup
sudo tar -xzf backup.tar.gz -C /opt/job-worker/

# 3. Fix permissions
sudo chown -R job-worker:job-worker /opt/job-worker
sudo chmod 600 /opt/job-worker/certs/*-key.pem

# 4. Start service
sudo systemctl start worker.service
```

### Migration to New Server

```bash
# 1. On old server - create backup
/opt/job-worker/scripts/backup.sh

# 2. Set up new server
# Follow standard deployment process

# 3. Transfer backup
scp backup.tar.gz user@new-server:/tmp/

# 4. On new server - restore
sudo tar -xzf /tmp/backup.tar.gz -C /opt/job-worker/
sudo systemctl start worker.service
```

## Environment-Specific Configurations

### Development Environment

```bash
# Use make setup-dev for local development
make setup-dev

# Start local server
./bin/job-worker

# Test with local CLI
./bin/cli --cert certs/admin-client-cert.pem --key certs/admin-client-key.pem create echo "test"
```

### Staging Environment

```bash
# Deploy to staging
make deploy-safe REMOTE_HOST=staging.example.com

# Use staging certificates
make certs-download-admin REMOTE_HOST=staging.example.com
```

### Production Environment

```bash
# Production deployment with proper certificates
make setup-remote-passwordless REMOTE_HOST=prod.example.com

# Enable monitoring and alerting
# Set up log aggregation
# Configure backup automation
# Implement certificate rotation
```

## Scaling Considerations

### Horizontal Scaling

For high-availability deployments:

1. **Load Balancer**: Deploy multiple job-worker instances behind a load balancer
2. **Shared Storage**: Use shared storage for certificate distribution
3. **Service Discovery**: Implement service discovery for dynamic scaling
4. **Health Checks**: Configure load balancer health checks

### Resource Planning

| Concurrent Jobs | Recommended RAM | Recommended CPU | Storage |
|-----------------|-----------------|-----------------|---------|
| 1-10            | 2GB             | 2 cores         | 20GB    |
| 10-50           | 4GB             | 4 cores         | 50GB    |
| 50-100          | 8GB             | 8 cores         | 100GB   |
| 100+            | 16GB+           | 16 cores+       | 200GB+  |

## Maintenance Schedule

### Daily

- Monitor service health
- Check log files for errors
- Verify certificate expiration dates

### Weekly

- Review job execution patterns
- Analyze resource usage
- Update system packages

### Monthly

- Rotate certificates (if needed)
- Backup configuration and logs
- Review security configurations

### Quarterly

- Update job-worker binaries
- Review and update documentation
- Conduct disaster recovery tests