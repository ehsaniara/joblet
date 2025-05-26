# Developer and Administration Documentation

## Table of Contents

- [SSH Authentication Setup](#ssh-authentication-setup)
- [Service Configuration](#service-configuration)
- [Service Management](#service-management)
- [Logging and Monitoring](#logging-and-monitoring)
- [Deployment Process](#deployment-process)
- [Troubleshooting](#troubleshooting)
- [Security Considerations](#security-considerations)

---

## SSH Authentication Setup

### Prerequisites

- Mac/Linux local machine with SSH client
- Ubuntu remote server with SSH server installed
- Administrative privileges on both machines

### 1. Check for Existing SSH Key

First, verify if you already have an SSH key pair on your local machine:

```bash
  ls ~/.ssh/id_rsa.pub
```

If the file doesn't exist, generate a new SSH key pair:

```bash
  ssh-keygen -t rsa -b 4096 -C "your_email@example.com"
```

**Note:** Press Enter to accept the default file location and optionally set a passphrase for additional security.

### 2. Copy Public Key to Remote Server

Use `ssh-copy-id` to automatically copy your public key to the remote server:

```bash
  ssh-copy-id username@remote-host-ip
```

**Alternative method** (if `ssh-copy-id` is not available):

```bash
  cat ~/.ssh/id_rsa.pub | ssh username@remote-host-ip 'mkdir -p ~/.ssh && cat >> ~/.ssh/authorized_keys'
```

### 3. Test SSH Connection

Verify that key-based authentication works:

```bash
  ssh username@remote-host-ip
```

You should be able to log in without entering a password.

**Note:** not for production

---

## Service Configuration

### Creating the Systemd Service File

Create the service file at `/etc/systemd/system/job-worker.service`:

```bash
  sudo nano /etc/systemd/system/job-worker.service
```

**Service Configuration:**

```ini
[Unit]
Description=Job Worker Service - Background job processing daemon
Documentation=https://your-docs-url.com
After=network.target network-online.target
Wants=network-online.target
StartLimitIntervalSec=60s
StartLimitBurst=5

[Service]
Type=simple
ExecStart=/opt/job-worker/job-worker
ExecReload=/bin/kill -HUP $MAINPID
ExecStopPost=/bin/bash -c 'for cg in $(find /sys/fs/cgroup -name "job-*" -type d 2>/dev/null); do rmdir "$cg" 2>/dev/null || true; done'

# Process management
Restart=always
RestartSec=10s
TimeoutStartSec=30s
TimeoutStopSec=30s
KillMode=mixed
KillSignal=SIGTERM

# User and permissions
User=root
Group=root
UMask=0022

# Working directory and environment
WorkingDirectory=/opt/job-worker
Environment=PATH=/usr/bin:/usr/local/bin:/bin
Environment=GODEBUG=gctrace=1,madvdontneed=1
Environment=JOB_WORKER_LOG_LEVEL=debug

# Resource limits
LimitNOFILE=4096
LimitNPROC=4096
MemoryMax=2G

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=job-worker

# Security and capabilities
NoNewPrivileges=yes
AmbientCapabilities=CAP_SYS_ADMIN
SecureBits=keep-caps

# Terminal settings
TTYReset=yes
TTYVHangup=yes

[Install]
WantedBy=multi-user.target
```

### Enable and Start the Service

```bash
    sudo systemctl daemon-reload
    sudo systemctl enable job-worker.service  
    sudo systemctl start job-worker.service  
    sudo systemctl status job-worker.service
```

---

## Service Management

### Basic Service Operations

```bash
    sudo systemctl start job-worker
```

```bash    
    sudo systemctl stop job-worker
```

```bash    
    sudo systemctl restart job-worker
```

Reload configuration without restart

```bash    
    sudo systemctl reload job-worker
```

```bash    
    sudo systemctl status job-worker
```

Check if service is enabled

```bash    
    sudo systemctl is-enabled job-worker
```

Disable service from starting on boot

```bash    
    sudo systemctl disable job-worker
```

### Service Health Checks

Check service active status

```bash
    systemctl is-active job-worker
```

Check service failed status

```bash    
    systemctl is-failed job-worker
```

List all failed services

```bash    
    systemctl --failed
```

Show service dependencies

```bash    
    systemctl list-dependencies job-worker
```

---

## Logging and Monitoring

### Viewing Logs with `journalctl`

View all logs for the service

```bash
    sudo journalctl -u job-worker
 ```

Follow logs in real-time

```bash
    sudo journalctl -u job-worker -f
 ```

View logs from last boot

```bash    
    sudo journalctl -u job-worker -b
 ```

View logs from specific time period

```bash    
    sudo journalctl -u job-worker --since "2024-01-01" --until "2024-01-02"
 ```

View last N lines

```bash    
    sudo journalctl -u job-worker -n 50
 ```

View logs with specific priority (error, warning, info, debug)

```bash    
    sudo journalctl -u job-worker -p err
 ```

Export logs to file

```bash    
    sudo journalctl -u job-worker > /tmp/job-worker-logs.txt
```

### Log Management

Check journal disk usage

```bash
    sudo journalctl --disk-usage
```

Clean old logs (keep last 7 days)

```bash    
    sudo journalctl --vacuum-time=7d
```

Clean old logs (keep only 100MB)

```bash   
    sudo journalctl --vacuum-size=100M
```

Rotate logs manually

```bash    
    sudo systemctl kill --kill-who=main --signal=SIGUSR2 systemd-journald.service
```

### Editing Service Configuration

Edit service file

```bash
    sudo systemctl edit --full job-worker.service
```

Create override file (recommended for modifications)

```bash    
    sudo systemctl edit job-worker.service
```

After editing, always reload and restart

```bash   
    sudo systemctl daemon-reload
    sudo systemctl restart job-worker.service
```

---

## Deployment Process

### Prerequisites

- New binary built and available in `/tmp/job-worker/build/`
- Service currently running
- Administrative privileges

### Deployment Steps

Deployment script - save as `deploy-job-worker.sh`

```bash
    #!/bin/bash
    
    
    set -e  # Exit on any error
    
    SERVICE_NAME="job-worker"
    TEMP_BINARY="/tmp/job-worker/build/job-worker"
    TARGET_BINARY="/opt/job-worker/job-worker"
    BACKUP_DIR="/opt/job-worker/backups"
    
    echo "Starting deployment process..."
    
    sudo mkdir -p "$BACKUP_DIR"
    
    # Create backup of current binary
    if [ -f "$TARGET_BINARY" ]; then
        BACKUP_NAME="job-worker.$(date +%Y%m%d_%H%M%S).backup"
        echo "Creating backup: $BACKUP_NAME"
        sudo cp "$TARGET_BINARY" "$BACKUP_DIR/$BACKUP_NAME"
    fi
    
    # Verify new binary exists
    if [ ! -f "$TEMP_BINARY" ]; then
        echo "Error: New binary not found at $TEMP_BINARY"
        exit 1
    fi
    
    sudo chmod +x "$TEMP_BINARY"
    
    echo "Stopping $SERVICE_NAME service..."
    sudo systemctl stop "$SERVICE_NAME.service"
    
    sleep 2
    
    echo "Deploying new binary..."
    sudo cp "$TEMP_BINARY" "$TARGET_BINARY"
    
    sudo chown root:root "$TARGET_BINARY"
    sudo chmod 755 "$TARGET_BINARY"
    
    echo "Starting $SERVICE_NAME service..."
    sudo systemctl start "$SERVICE_NAME.service"
    
    sleep 3
    
    if sudo systemctl is-active --quiet "$SERVICE_NAME"; then
        echo "Deployment successful! Service is running."
        sudo systemctl status "$SERVICE_NAME.service" --no-pager -l
    else
        echo "Deployment failed! Service is not running."
        echo "Checking logs..."
        sudo journalctl -u "$SERVICE_NAME.service" -n 20 --no-pager
        exit 1
    fi
    
    echo "Following logs (Ctrl+C to exit)..."
    sudo journalctl -u "$SERVICE_NAME.service" -f
```

### Manual Deployment (Original Process)

Stop the service

```bash
    sudo systemctl stop job-worker.service
```

Verify service is stopped

```bash
    sudo systemctl is-active job-worker.service
```

Copy new binary

```bash    
    sudo cp /tmp/job-worker/build/job-worker /opt/job-worker/job-worker
```

Set proper permissions

```bash    
    sudo chmod +x /opt/job-worker/job-worker
```

Start the service

```bash    
    sudo systemctl start job-worker.service
```

Monitor startup logs

```bash    
    sudo journalctl -u job-worker.service -f
```

### Rollback Procedure

If deployment fails, rollback to previous version

```bash
    sudo systemctl stop job-worker.service
    sudo cp /opt/job-worker/backups/job-worker.YYYYMMDD_HHMMSS.backup /opt/job-worker/job-worker
    sudo systemctl start job-worker.service
    sudo systemctl status job-worker.service
```

---

## Troubleshooting

### Common Issues and Solutions

#### Service Won't Start

Check service status and recent logs

```bash
    sudo systemctl status job-worker.service
    sudo journalctl -u job-worker.service -n 20
    
    # Check if binary exists and is executable
    ls -la /opt/job-worker/job-worker
    file /opt/job-worker/job-worker
    
    sudo systemd-analyze verify /etc/systemd/system/job-worker.service
```

Check for configuration syntax errors

#### Service Keeps Restarting

Check restart count and failure reason

```bash
    sudo systemctl status job-worker.service
```

Check for resource limits

```bash
    sudo systemctl show job-worker.service | grep -E "(Limit|Memory|CPU)"
```

Monitor resource usage

```bash    
    top -p $(pgrep job-worker)
```

#### Permission Issues

Check file ownership and permissions

```bash
    ls -la /opt/job-worker/
    sudo namei -l /opt/job-worker/job-worker
    
    # Fix ownership if needed
    sudo chown -R root:root /opt/job-worker/
    sudo chmod 755 /opt/job-worker/job-worker
```

#### Network/Port Issues

Check listening ports

```bash
    
    sudo netstat -tlnp | grep job-worker
    sudo ss -tlnp | grep job-worker
    
    # Check if process is running
    ps aux | grep job-worker
```

### Diagnostic Commands

System information

```bash
    uname -a
    systemctl --version
```

Service configuration

```bash    
    sudo systemctl cat job-worker.service
```

Service environment

```bash    
    sudo systemctl show-environment
```

System resources

```bash    
    free -h
    df -h
    top -bn1 | head -20
```

---

## Security Considerations

### File Permissions

Secure the service binary

```bash
    sudo chmod 755 /opt/job-worker/job-worker
    sudo chown root:root /opt/job-worker/job-worker
```

Secure the service file

```bash    
    sudo chmod 644 /etc/systemd/system/job-worker.service
    sudo chown root:root /etc/systemd/system/job-worker.service
```

### Service Hardening

Options to the service file:

```ini
[Service]
# Additional security options
PrivateTmp=yes
ProtectSystem=strict
ProtectHome=yes
ReadWritePaths=/opt/job-worker
NoNewPrivileges=yes
ProtectKernelTunables=yes
ProtectKernelModules=yes
ProtectControlGroups=yes
```

### SSH Security (NO FOR PRODUCTION)

Disable password authentication (after key-based auth is working)

```bash
    sudo nano /etc/ssh/sshd_config
    # Set: PasswordAuthentication no
    # Set: ChallengeResponseAuthentication no
    # Set: UsePAM no
```

Then Restart SSH service

```bash    
    sudo systemctl restart sshd
```

### Monitoring and Alerts

Set up log monitoring for errors

```bash
    sudo journalctl -u job-worker.service -p err -f
```

Monitor service uptime

```bash    
    systemctl show job-worker.service --property=ActiveEnterTimestamp
```

Check service failures

```bash    
    systemctl show job-worker.service --property=NRestarts
```

---

## Maintenance Tasks

### Regular Maintenance

Weekly log cleanup

```bash
    sudo journalctl --vacuum-time=7d
```

Check service health

```bash    
    sudo systemctl status job-worker.service
```

Update binary permissions after system updates

```bash    
    sudo chmod +x /opt/job-worker/job-worker
```

Restart service monthly for fresh state

```bash    
    sudo systemctl restart job-worker.service
```

### Backup Strategy

Backup current binary before updates

```bash
    sudo cp /opt/job-worker/job-worker /opt/job-worker/backups/job-worker.$(date +%Y%m%d).backup
```

Backup service configuration

```bash    
    sudo cp /etc/systemd/system/job-worker.service /opt/job-worker/backups/
```

