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

Create the service file at `/etc/systemd/system/worker.service`:

```bash
  sudo nano /etc/systemd/system/worker.service
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
ExecStart=/opt/worker/worker
ExecReload=/bin/kill -HUP $MAINPID
ExecStopPost=/bin/bash -c 'for cg in $(find /sys/fs/cgroup -name "worker-*" -type d 2>/dev/null); do rmdir "$cg" 2>/dev/null || true; done'

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
WorkingDirectory=/opt/worker
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
SyslogIdentifier=worker

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
    sudo systemctl enable worker.service  
    sudo systemctl start worker.service  
    sudo systemctl status worker.service
```

---

## Service Management

### Basic Service Operations

```bash
    sudo systemctl start worker
```

```bash    
    sudo systemctl stop worker
```

```bash    
    sudo systemctl restart worker
```

Reload configuration without restart

```bash    
    sudo systemctl reload worker
```

```bash    
    sudo systemctl status worker
```

Check if service is enabled

```bash    
    sudo systemctl is-enabled worker
```

Disable service from starting on boot

```bash    
    sudo systemctl disable worker
```

### Service Health Checks

Check service active status

```bash
    systemctl is-active worker
```

Check service failed status

```bash    
    systemctl is-failed worker
```

List all failed services

```bash    
    systemctl --failed
```

Show service dependencies

```bash    
    systemctl list-dependencies worker
```

---

## Logging and Monitoring

### Viewing Logs with `journalctl`

View all logs for the service

```bash
    sudo journalctl -u worker
 ```

Follow logs in real-time

```bash
    sudo journalctl -u worker -f
 ```

View logs from last boot

```bash    
    sudo journalctl -u worker -b
 ```

View logs from specific time period

```bash    
    sudo journalctl -u worker --since "2024-01-01" --until "2024-01-02"
 ```

View last N lines

```bash    
    sudo journalctl -u worker -n 50
 ```

View logs with specific priority (error, warning, info, debug)

```bash    
    sudo journalctl -u worker -p err
 ```

Export logs to file

```bash    
    sudo journalctl -u worker > /tmp/worker-logs.txt
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
    sudo systemctl edit --full worker.service
```

Create override file (recommended for modifications)

```bash    
    sudo systemctl edit worker.service
```

After editing, always reload and restart

```bash   
    sudo systemctl daemon-reload
    sudo systemctl restart worker.service
```

---

## Deployment Process

### Prerequisites

- New binary built and available in `/tmp/worker/build/`
- Service currently running
- Administrative privileges

### Deployment Steps

Deployment script - save as `deploy-worker.sh`

```bash
    #!/bin/bash
    
    
    set -e  # Exit on any error
    
    SERVICE_NAME="worker"
    TEMP_BINARY="/tmp/worker/build/worker"
    TARGET_BINARY="/opt/worker/worker"
    BACKUP_DIR="/opt/worker/backups"
    
    echo "Starting deployment process..."
    
    sudo mkdir -p "$BACKUP_DIR"
    
    # Create backup of current binary
    if [ -f "$TARGET_BINARY" ]; then
        BACKUP_NAME="worker.$(date +%Y%m%d_%H%M%S).backup"
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
    sudo systemctl stop worker.service
```

Verify service is stopped

```bash
    sudo systemctl is-active worker.service
```

Copy new binary

```bash    
    sudo cp /tmp/worker/build/worker /opt/worker/worker
```

Set proper permissions

```bash    
    sudo chmod +x /opt/worker/worker
```

Start the service

```bash    
    sudo systemctl start worker.service
```

Monitor startup logs

```bash    
    sudo journalctl -u worker.service -f
```

### Rollback Procedure

If deployment fails, rollback to previous version

```bash
    sudo systemctl stop worker.service
    sudo cp /opt/worker/backups/worker.YYYYMMDD_HHMMSS.backup /opt/worker/worker
    sudo systemctl start worker.service
    sudo systemctl status worker.service
```

---

## Troubleshooting

### Common Issues and Solutions

#### Service Won't Start

Check service status and recent logs

```bash
    sudo systemctl status worker.service
    sudo journalctl -u worker.service -n 20
    
    # Check if binary exists and is executable
    ls -la /opt/worker/worker
    file /opt/worker/worker
    
    sudo systemd-analyze verify /etc/systemd/system/worker.service
```

Check for configuration syntax errors

#### Service Keeps Restarting

Check restart count and failure reason

```bash
    sudo systemctl status worker.service
```

Check for resource limits

```bash
    sudo systemctl show worker.service | grep -E "(Limit|Memory|CPU)"
```

Monitor resource usage

```bash    
    top -p $(pgrep worker)
```

#### Permission Issues

Check file ownership and permissions

```bash
    ls -la /opt/worker/
    sudo namei -l /opt/worker/worker
    
    # Fix ownership if needed
    sudo chown -R root:root /opt/worker/
    sudo chmod 755 /opt/worker/worker
```

#### Network/Port Issues

Check listening ports

```bash
    
    sudo netstat -tlnp | grep worker
    sudo ss -tlnp | grep worker
    
    # Check if process is running
    ps aux | grep worker
```

### Diagnostic Commands

System information

```bash
    uname -a
    systemctl --version
```

Service configuration

```bash    
    sudo systemctl cat worker.service
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
    sudo chmod 755 /opt/worker/worker
    sudo chown root:root /opt/worker/worker
```

Secure the service file

```bash    
    sudo chmod 644 /etc/systemd/system/worker.service
    sudo chown root:root /etc/systemd/system/worker.service
```

### Service Hardening

Options to the service file:

```ini
[Service]
# Additional security options
PrivateTmp=yes
ProtectSystem=strict
ProtectHome=yes
ReadWritePaths=/opt/worker
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
    sudo journalctl -u worker.service -p err -f
```

Monitor service uptime

```bash    
    systemctl show worker.service --property=ActiveEnterTimestamp
```

Check service failures

```bash    
    systemctl show worker.service --property=NRestarts
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
    sudo systemctl status worker.service
```

Update binary permissions after system updates

```bash    
    sudo chmod +x /opt/worker/worker
```

Restart service monthly for fresh state

```bash    
    sudo systemctl restart worker.service
```

### Backup Strategy

Backup current binary before updates

```bash
    sudo cp /opt/worker/worker /opt/worker/backups/worker.$(date +%Y%m%d).backup
```

Backup service configuration

```bash    
    sudo cp /etc/systemd/system/worker.service /opt/worker/backups/
```

