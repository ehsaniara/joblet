# Security Guide

Comprehensive security guide for Joblet deployment, covering mTLS authentication, authorization, isolation, and best
practices.

## Table of Contents

- [Security Overview](#security-overview)
- [mTLS Authentication](#mtls-authentication)
- [Authorization and RBAC](#authorization-and-rbac)
- [Process Isolation](#process-isolation)
- [Network Security](#network-security)
- [Data Security](#data-security)
- [Hardening Practices](#hardening-practices)
- [Monitoring and Auditing](#monitoring-and-auditing)
- [Security Compliance](#security-compliance)

## Security Overview

Joblet implements multi-layered security:

```
┌─────────────────────────────────────────┐
│           Transport Layer               │
│         mTLS + Certificate Auth         │
├─────────────────────────────────────────┤
│         Authorization Layer             │
│         RBAC (admin/viewer)             │
├─────────────────────────────────────────┤
│         Process Isolation               │
│    Namespaces + cgroups + chroot        │
├─────────────────────────────────────────┤
│         Network Isolation               │
│       Custom networks + traffic         │
├─────────────────────────────────────────┤
│         Filesystem Isolation            │
│      Per-job workspaces + volumes       │
└─────────────────────────────────────────┘
```

### Key Security Features

- **mTLS**: Mutual TLS with certificate-based authentication
- **RBAC**: Role-based access control (admin/viewer)
- **Process Isolation**: Linux namespaces and cgroups
- **Network Isolation**: Custom networks and traffic control
- **Filesystem Isolation**: Chroot and per-job workspaces
- **Resource Limits**: Prevent resource exhaustion attacks
- **Audit Logging**: Track all operations and access

## mTLS Authentication

### Certificate-Based Security

Joblet uses mutual TLS (mTLS) for secure communication:

```bash
# Generate CA and certificates
export JOBLET_SERVER_ADDRESS='192.168.1.100'
sudo /usr/local/bin/certs_gen_embedded.sh
```

This creates:

- **CA Certificate**: Root certificate authority
- **Server Certificate**: For Joblet daemon
- **Client Certificates**: For RNX clients

### Certificate Structure

```
Certificate Authority (CA)
├── Server Certificate (CN=joblet)
│   ├── Used by Joblet daemon
│   └── Validates server identity
└── Client Certificates
    ├── Admin Certificate (OU=admin)
    │   ├── Full access to all operations
    │   └── Can run, stop, manage jobs
    └── Viewer Certificate (OU=viewer)
        ├── Read-only access
        └── Can list, view status, logs
```

### Manual Certificate Generation

```bash
# 1. Create CA
openssl genrsa -out ca-key.pem 4096
openssl req -new -x509 -key ca-key.pem -out ca-cert.pem -days 3650 \
  -subj "/CN=Joblet CA"

# 2. Server certificate (CN must be "joblet")
openssl genrsa -out server-key.pem 4096
openssl req -new -key server-key.pem -out server.csr \
  -subj "/CN=joblet"
openssl x509 -req -in server.csr -CA ca-cert.pem -CAkey ca-key.pem \
  -out server-cert.pem -days 365 -CAcreateserial \
  -extensions v3_req -extfile <(echo "[v3_req]
subjectAltName = DNS:localhost,DNS:joblet,IP:127.0.0.1,IP:${SERVER_IP}")

# 3. Admin client certificate
openssl genrsa -out admin-key.pem 4096
openssl req -new -key admin-key.pem -out admin.csr \
  -subj "/CN=admin-client/OU=admin"
openssl x509 -req -in admin.csr -CA ca-cert.pem -CAkey ca-key.pem \
  -out admin-cert.pem -days 365 -CAcreateserial

# 4. Viewer client certificate
openssl genrsa -out viewer-key.pem 4096
openssl req -new -key viewer-key.pem -out viewer.csr \
  -subj "/CN=viewer-client/OU=viewer"
openssl x509 -req -in viewer.csr -CA ca-cert.pem -CAkey ca-key.pem \
  -out viewer-cert.pem -days 365 -CAcreateserial
```

### Certificate Rotation

```bash
# 1. Generate new certificates
sudo /usr/local/bin/certs_gen_embedded.sh

# 2. Update server configuration
sudo systemctl restart joblet

# 3. Distribute new client certificates
scp /opt/joblet/config/rnx-config.yml client:~/.rnx/

# 4. Verify new certificates
rnx list  # Should work with new certs
```

### TLS Configuration

```yaml
# Server configuration (joblet-config.yml)
security:
  # TLS settings
  tls:
    enabled: true
    min_version: "1.3"
    cipher_suites:
      - TLS_AES_256_GCM_SHA384
      - TLS_CHACHA20_POLY1305_SHA256

  # Certificate verification
  require_client_cert: true
  verify_client_cert: true

  # Certificate embedded in config
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

## Authorization and RBAC

### Role-Based Access Control

Joblet uses certificate Organization Unit (OU) for roles:

| Role   | OU Value | Permissions                                              |
|--------|----------|----------------------------------------------------------|
| Admin  | `admin`  | Full access: run, stop, list, logs, volumes, networks    |
| Viewer | `viewer` | Read-only: list, status, logs (but cannot run/stop jobs) |

### Admin Role (OU=admin)

```bash
# Full access operations
rnx run echo "Admin can run jobs"
rnx stop <job-id>
rnx volume create admin-vol --size=1GB
rnx network create admin-net --cidr=10.1.0.0/24

# All monitoring operations
rnx monitor
rnx list
rnx status <job-id>
rnx log <job-id>
```

### Viewer Role (OU=viewer)

```bash
# Read-only operations (allowed)
rnx list
rnx status <job-id>
rnx log <job-id>
rnx monitor status

# Write operations (denied)
rnx run echo "test"          # ERROR: Permission denied
rnx stop <job-id>            # ERROR: Permission denied
rnx volume create test       # ERROR: Permission denied
```

### Multi-User Setup

```bash
# Generate certificates for different users
# DevOps team (admin access)
openssl req -new -key devops-key.pem -out devops.csr \
  -subj "/CN=devops-team/OU=admin"

# Developers (viewer access)
openssl req -new -key dev-key.pem -out dev.csr \
  -subj "/CN=developer/OU=viewer"

# QA team (viewer access)
openssl req -new -key qa-key.pem -out qa.csr \
  -subj "/CN=qa-team/OU=viewer"

# Sign all certificates with CA
for cert in devops dev qa; do
  openssl x509 -req -in ${cert}.csr -CA ca-cert.pem -CAkey ca-key.pem \
    -out ${cert}-cert.pem -days 365 -CAcreateserial
done
```

### Fine-Grained Permissions (Future)

Current RBAC is binary (admin/viewer). For more granular control:

```yaml
# Conceptual fine-grained permissions
authorization:
  roles:
    admin:
      - job:*
      - volume:*
      - network:*
      - monitor:*

    developer:
      - job:run
      - job:list
      - job:status
      - job:logs
      - volume:list

    qa:
      - job:list
      - job:status
      - job:logs
      - monitor:status
```

## Process Isolation

### Linux Namespaces

Joblet uses multiple namespaces for isolation:

```bash
# PID namespace - process isolation
rnx run ps aux  # Only sees job processes

# Mount namespace - filesystem isolation
rnx run mount  # Shows only job-specific mounts

# Network namespace - network isolation
rnx run ip addr show  # Shows only job network interface

# IPC namespace - inter-process communication isolation
rnx run ipcs  # No shared memory/semaphores from host

# UTS namespace - hostname isolation
rnx run hostname  # Job-specific hostname

# Cgroup namespace - resource visibility
rnx run cat /proc/cgroups  # Limited cgroup view
```

### Security Context

```bash
# Jobs run as unprivileged user
rnx run id
# Output: uid=65534(nobody) gid=65534(nogroup)

# No sudo/setuid capabilities
rnx run sudo echo "test"  # Command not found

# Limited filesystem access
rnx run ls /root  # Permission denied
rnx run ls /etc/shadow  # Permission denied
```

### Resource Limits (Security)

```bash
# Prevent fork bombs
rnx run --max-cpu=100 :(){ :|:& };:  # Limited by CPU quota

# Prevent memory exhaustion
rnx run --max-memory=512 bash -c 'a=(); while true; do a+=($a); done'  # Killed by OOM

# Prevent I/O attacks
rnx run --max-iobps=1048576 dd if=/dev/zero of=/work/attack bs=1M  # Limited bandwidth
```

## Network Security

### Network Isolation

```bash
# Create isolated networks for different security zones
rnx network create dmz --cidr=10.1.0.0/24           # Public-facing
rnx network create internal --cidr=10.2.0.0/24      # Internal services
rnx network create secure --cidr=10.3.0.0/24        # Sensitive data

# Jobs in different networks cannot communicate
rnx run --network=dmz ping 10.2.0.1        # Will fail
rnx run --network=internal ping 10.3.0.1   # Will fail
```

### Zero-Trust Network Model

```bash
# No network access for sensitive processing
rnx run --network=none --volume=sensitive-data \
  python process_classified.py

# Limited network access
rnx run --network=internal --volume=app-data \
  python internal_service.py

# Full internet access (carefully controlled)
rnx run --network=bridge \
  curl https://api.trusted-service.com
```

### Traffic Control

```bash
# Limit bandwidth to prevent data exfiltration
rnx run --max-iobps=1048576 --network=bridge \
  curl https://malicious-site.com  # Limited to 1MB/s

# Monitor network usage
rnx run --network=monitored iftop
```

## Data Security

### Sensitive Data Handling

```bash
# 1. Create encrypted volume
rnx volume create encrypted-data --size=10GB

# 2. Encrypt data before storage
rnx run --volume=encrypted-data bash -c '
  echo "sensitive information" | \
  openssl enc -aes-256-cbc -k "$ENCRYPTION_KEY" \
  > /volumes/encrypted-data/secret.enc
'

# 3. Decrypt only when needed
rnx run --volume=encrypted-data --env=ENCRYPTION_KEY=xxx bash -c '
  openssl enc -aes-256-cbc -d -k "$ENCRYPTION_KEY" \
  < /volumes/encrypted-data/secret.enc
'
```

### Secrets Management

```bash
# Avoid embedding secrets in commands (BAD)
rnx run curl -H "Authorization: Bearer secret123" api.com

# Use environment variables (BETTER)
rnx run --env=API_TOKEN=secret123 \
  curl -H "Authorization: Bearer \$API_TOKEN" api.com

# Use volume-based secrets (BEST)
echo "secret123" | rnx run --volume=secrets bash -c '
  cat > /volumes/secrets/api-token
  chmod 600 /volumes/secrets/api-token
'

rnx run --volume=secrets bash -c '
  API_TOKEN=$(cat /volumes/secrets/api-token)
  curl -H "Authorization: Bearer $API_TOKEN" api.com
'
```

### Data Classification

```bash
# Public data - no restrictions
rnx run --network=bridge --volume=public-data \
  wget https://public-dataset.com/data.csv

# Internal data - network restrictions
rnx run --network=internal --volume=internal-data \
  python process_internal.py

# Confidential data - maximum isolation
rnx run --network=none --volume=confidential-data \
  python process_confidential.py

# Secret data - encrypted storage
rnx run --network=none --volume=encrypted-secrets \
  --env=DECRYPT_KEY=xxx \
  python process_secrets.py
```

## Hardening Practices

### Server Hardening

```bash
# 1. Minimal server installation
sudo apt install --no-install-recommends joblet

# 2. Disable unnecessary services
sudo systemctl disable apache2 nginx mysql

# 3. Configure firewall
sudo ufw allow 50051/tcp  # Joblet port only
sudo ufw enable

# 4. Regular updates
sudo apt update && sudo apt upgrade

# 5. Secure SSH
# /etc/ssh/sshd_config
PermitRootLogin no
PasswordAuthentication no
ChallengeResponseAuthentication no
```

### Configuration Hardening

```yaml
# Secure server configuration
server:
  tls:
    enabled: true
    min_version: "1.3"

joblet:
  # Validate all commands
  validateCommands: true
  allowedCommands:
    - python3
    - node
    - bash
    - sh

  # Conservative limits
  maxConcurrentJobs: 50
  jobTimeout: "1h"
  defaultMemoryLimit: 1024

security:
  require_client_cert: true
  verify_client_cert: true
  enable_rbac: true

  # Comprehensive auditing
  audit:
    enabled: true
    log_all_operations: true
    log_failed_auth: true

filesystem:
  enable_chroot: true
  readonly_rootfs: true

process:
  default_user: "nobody"
  allow_setuid: false
```

### File Permissions

```bash
# Secure configuration files
sudo chmod 600 /opt/joblet/config/joblet-config.yml
sudo chown root:root /opt/joblet/config/joblet-config.yml

# Secure certificates
sudo chmod 600 /opt/joblet/certs/*.pem
sudo chown root:root /opt/joblet/certs/*.pem

# Secure log files
sudo chmod 640 /var/log/joblet/*.log
sudo chown root:joblet /var/log/joblet/*.log
```

## Monitoring and Auditing

### Security Logging

```yaml
# Enable comprehensive logging
logging:
  level: "info"
  format: "json"

  # Security-focused logging
  outputs:
    - type: "file"
      path: "/var/log/joblet/security.log"
      filter: "security"
    - type: "syslog"
      facility: "auth"

security:
  audit:
    enabled: true
    log_file: "/var/log/joblet/audit.log"
    log_successful_auth: true
    log_failed_auth: true
    log_job_operations: true
    log_admin_operations: true
```

### Security Monitoring

```bash
# Monitor failed authentication attempts
sudo tail -f /var/log/joblet/audit.log | grep "auth_failed"

# Monitor admin operations
sudo tail -f /var/log/joblet/audit.log | grep "admin_operation"

# Monitor unusual job patterns
sudo tail -f /var/log/joblet/audit.log | jq 'select(.job_count > 100)'

# Monitor resource usage spikes
rnx monitor --json | jq 'select(.cpu_usage > 90 or .memory_usage > 90)'
```

### Alerting Setup

```bash
# Create monitoring script
cat > security_monitor.sh << 'EOF'
#!/bin/bash
# Monitor for security events

# Check for multiple failed auth attempts
FAILED_AUTH=$(grep -c "auth_failed" /var/log/joblet/audit.log | tail -100)
if [ $FAILED_AUTH -gt 10 ]; then
  echo "ALERT: Multiple authentication failures detected"
fi

# Check for unusual job patterns
RUNNING_JOBS=$(rnx list --json | jq '[.[] | select(.status == "RUNNING")] | length')
if [ $RUNNING_JOBS -gt 50 ]; then
  echo "ALERT: Unusual number of running jobs: $RUNNING_JOBS"
fi

# Check for resource exhaustion
CPU_USAGE=$(rnx monitor status --json | jq .cpu_usage)
if [ $(echo "$CPU_USAGE > 95" | bc) -eq 1 ]; then
  echo "ALERT: High CPU usage: $CPU_USAGE%"
fi
EOF

# Schedule monitoring
echo "*/5 * * * * /opt/joblet/scripts/security_monitor.sh" | sudo crontab -
```

## Security Compliance

### SOC 2 Compliance

```yaml
# Configuration for SOC 2
security:
  audit:
    enabled: true
    log_all_operations: true
    retention_days: 2555  # 7 years

  access_control:
    require_mfa: true
    session_timeout: "8h"

logging:
  immutable_logs: true
  log_integrity_check: true
```

### HIPAA Compliance

```bash
# HIPAA-compliant setup
# 1. Encrypted storage
rnx volume create phi-data --size=100GB --type=filesystem

# 2. Encrypted transit (already provided by mTLS)

# 3. Access logging
# (Already provided by audit logging)

# 4. Data minimization
rnx run --network=none --volume=phi-data \
  python anonymize_phi.py

# 5. Secure disposal
rnx volume remove phi-data  # Secure deletion
```

### PCI DSS Compliance

```bash
# PCI DSS network segmentation
rnx network create pci-zone --cidr=10.100.0.0/24

# Restricted processing environment
rnx run \
  --network=pci-zone \
  --volume=pci-secure \
  --max-memory=2048 \
  --env=PCI_MODE=true \
  python process_payments.py
```

## Incident Response

### Security Incident Detection

```bash
# Automated threat detection
cat > threat_detection.sh << 'EOF'
#!/bin/bash

# Detect privilege escalation attempts
if grep -q "setuid\|sudo\|su " /var/log/joblet/audit.log; then
  echo "THREAT: Privilege escalation attempt detected"
fi

# Detect network scanning
if rnx list --json | jq '.[] | select(.command | contains("nmap"))' | grep -q .; then
  echo "THREAT: Network scanning detected"
fi

# Detect data exfiltration patterns
if rnx list --json | jq '.[] | select(.command | contains("curl") and .max_iobps == 0)' | grep -q .; then
  echo "THREAT: Potential data exfiltration (unlimited bandwidth)"
fi
EOF
```

### Incident Response Procedures

```bash
# 1. Immediate response
# Stop suspicious jobs
rnx list --json | jq -r '.[] | select(.status == "RUNNING" and (.command | contains("suspicious"))) | .id' | xargs rnx stop

# 2. Isolate affected networks
rnx network delete compromised-network

# 3. Preserve evidence
sudo cp -r /var/log/joblet/ /var/incident-evidence/$(date +%Y%m%d-%H%M%S)

# 4. Reset certificates
sudo /usr/local/bin/certs_gen_embedded.sh
sudo systemctl restart joblet

# 5. Audit all access
sudo grep "auth_success" /var/log/joblet/audit.log | tail -1000
```

## Best Practices Summary

### ✅ Do's

1. **Always use mTLS** - Never disable certificate verification
2. **Implement RBAC** - Use viewer certificates for read-only access
3. **Network isolation** - Use custom networks for sensitive workloads
4. **Resource limits** - Always set appropriate CPU/memory limits
5. **Audit logging** - Enable comprehensive security logging
6. **Regular updates** - Keep Joblet and system updated
7. **Certificate rotation** - Rotate certificates regularly
8. **Principle of least privilege** - Use `--network=none` when possible

### ❌ Don'ts

1. **Don't embed secrets** in job commands
2. **Don't use host network** for untrusted workloads
3. **Don't disable TLS** in production
4. **Don't use unlimited resources** for untrusted jobs
5. **Don't ignore audit logs** - Monitor for anomalies
6. **Don't share admin certificates** - Use viewer certificates for most users
7. **Don't run Joblet as non-root** - It needs privileges for isolation
8. **Don't trust user input** - Validate and sanitize all inputs

## See Also

- [Configuration Guide](./CONFIGURATION.md) - Security configuration
- [Network Management](./NETWORK_MANAGEMENT.md) - Network isolation
- [Installation Guide](./INSTALLATION.md) - Secure installation
- [Troubleshooting](./TROUBLESHOOTING.md) - Security issues