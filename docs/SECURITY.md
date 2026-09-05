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

```mermaid
flowchart TD
    N1["Transport Layer<br/>mTLS + Certificate Auth"]
    N2["Authorization Layer<br/>RBAC (admin/maintainer/developer/reader)"]
    N3["Process Isolation<br/>Namespaces + cgroups + chroot"]
    N4["Network Isolation<br/>Custom networks + traffic"]
    N5["Filesystem Isolation<br/>Per-job workspaces + volumes"]
    N1 --- N2 --- N3 --- N4 --- N5
```

### Key Security Features

- **mTLS**: Mutual TLS with certificate-based authentication
- **RBAC**: Role-based access control (admin/maintainer/developer/reader)
- **Service-Based Isolation**: Automatic job routing based on API service
- **Dual Chroot System**: Production isolation (minimal) vs builder isolation (controlled)
- **Runtime Cleanup**: Self-contained runtime isolation preventing host filesystem exposure
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
- **Client Certificates**: One per role (admin, maintainer, developer, reader)

The script writes the operator's `rnx-config.yml` (all roles, admin key included, keep it on the server) and one
`rnx-config-<role>.yml` per role for distribution. Give each party only its role's file. The AWS variant
(`certs_gen_with_secretsmanager.sh`) produces the same files and also stores each role's certificate pair in Secrets
Manager as `joblet/client-cert-<role>` and `joblet/client-key-<role>`; scope each client's IAM policy to its role's
secrets. The unsuffixed `joblet/client-cert` and `joblet/client-key` names hold the admin pair.

### Certificate Structure

```mermaid
flowchart TD
    N1["Certificate Authority (CA)"] --> N2["Server Certificate (CN=joblet)"]
    N1 --> N3["Client Certificates"]
    N2 --> N4["Used by Joblet daemon"]
    N2 --> N5["Validates server identity"]
    N3 --> N6["Admin Certificate (OU=admin)"]
    N3 --> N7["Maintainer Certificate (OU=maintainer)"]
    N3 --> N8["Developer Certificate (OU=developer)"]
    N3 --> N9["Reader Certificate (OU=reader)"]
    N6 --> N10["Every operation, including removals"]
    N7 --> N11["Provision infrastructure + run jobs"]
    N8 --> N12["Run jobs on existing infrastructure"]
    N9 --> N13["Read-only access"]
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

# 3. Client certificates (one per role; the OU field carries the role)
for role in admin maintainer developer reader; do
  openssl genrsa -out ${role}-key.pem 4096
  openssl req -new -key ${role}-key.pem -out ${role}.csr \
    -subj "/CN=${role}-client/OU=${role}"
  openssl x509 -req -in ${role}.csr -CA ca-cert.pem -CAkey ca-key.pem \
    -out ${role}-cert.pem -days 365 -CAcreateserial
done
```

### Certificate Rotation

```bash
# 1. Generate new certificates
sudo /usr/local/bin/certs_gen_embedded.sh

# 2. Update server configuration
sudo systemctl restart joblet

# 3. Distribute new per-role client configs (never the combined file)
scp /opt/joblet/config/rnx-config-<role>.yml client:~/.rnx/rnx-config.yml

# 4. Verify new certificates
rnx job list  # Should work with new certs
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

Joblet reads the client's role from the certificate Organizational Unit (OU) field (case-insensitive):

| Role       | OU Value     | Permissions                                                                                                                         |
|------------|--------------|-------------------------------------------------------------------------------------------------------------------------------------|
| Admin      | `admin`      | Every operation, including removing runtimes, networks, and volumes                                                                 |
| Maintainer | `maintainer` | Provisions infrastructure (builds runtimes, validates runtime YAML, creates networks and volumes) and everything a developer can do |
| Developer  | `developer`  | Runs, stops, and deletes jobs, tests runtimes, and reads everything. Cannot change infrastructure                                   |
| Reader     | `reader`     | Reads only: jobs, logs, status, resource listings, and metrics. Meant for dashboards and reporting                                  |

Issue `maintainer` certificates to CI/CD service accounts: they can provision what their pipelines need but can never
remove shared infrastructure. Certificates carrying the older `viewer` OU keep working and behave as `reader`. Any
other OU is denied everything, and a certificate carrying more than one role OU gets the least privileged of them.

### Permission Matrix

| Service           | Operation                                  | Admin | Maintainer | Developer | Reader |
|-------------------|--------------------------------------------|:-----:|:----------:|:---------:|:------:|
| JobService        | RunJob                                     |   ✅   |     ✅      |     ✅     |   ❌    |
| JobService        | StopJob                                    |   ✅   |     ✅      |     ✅     |   ❌    |
| JobService        | DeleteJob, DeleteAllJobs                   |   ✅   |     ✅      |     ✅     |   ❌    |
| JobService        | GetJobStatus, ListJobs, log/status streams |   ✅   |     ✅      |     ✅     |   ✅    |
| RuntimeService    | ListRuntimes, GetRuntimeInfo               |   ✅   |     ✅      |     ✅     |   ✅    |
| RuntimeService    | TestRuntime                                |   ✅   |     ✅      |     ✅     |   ❌    |
| RuntimeService    | BuildRuntime, ValidateRuntimeYAML          |   ✅   |     ✅      |     ❌     |   ❌    |
| RuntimeService    | RemoveRuntime                              |   ✅   |     ❌      |     ❌     |   ❌    |
| NetworkService    | ListNetworks                               |   ✅   |     ✅      |     ✅     |   ✅    |
| NetworkService    | CreateNetwork                              |   ✅   |     ✅      |     ❌     |   ❌    |
| NetworkService    | RemoveNetwork                              |   ✅   |     ❌      |     ❌     |   ❌    |
| VolumeService     | ListVolumes                                |   ✅   |     ✅      |     ✅     |   ✅    |
| VolumeService     | CreateVolume                               |   ✅   |     ✅      |     ❌     |   ❌    |
| VolumeService     | RemoveVolume                               |   ✅   |     ❌      |     ❌     |   ❌    |
| MonitoringService | GetSystemStatus, StreamSystemMetrics       |   ✅   |     ✅      |     ✅     |   ✅    |
| Persist           | Query historical logs/metrics              |   ✅   |     ✅      |     ✅     |   ✅    |

Monitoring endpoints go through the same authorization as everything else: a valid certificate alone is not enough;
it must carry a recognized role.

### Admin Role (OU=admin)

```bash
# Every operation, including removal of shared infrastructure
rnx runtime remove openjdk-21
rnx network remove old-net
rnx volume remove old-vol

# Plus everything maintainer, developer, and reader can do
```

### Maintainer Role (OU=maintainer)

```bash
# Infrastructure provisioning (allowed)
rnx runtime build ./examples/java-21/runtime.yaml
rnx runtime validate ./examples/java-21/runtime.yaml
rnx network create ci-net --cidr=10.1.0.0/24
rnx volume create ci-vol --size=1GB

# Plus everything developer can do

# Removal of shared infrastructure (denied)
rnx runtime remove openjdk-21    # ERROR: Permission denied
rnx network remove ci-net        # ERROR: Permission denied
rnx volume remove ci-vol         # ERROR: Permission denied
```

### Developer Role (OU=developer)

```bash
# Job execution on existing infrastructure (allowed)
rnx job run echo "Developers can run jobs"
rnx job stop <job-id>
rnx job delete <job-id>
rnx runtime test openjdk-21

# Plus all read access

# Infrastructure changes (denied)
rnx runtime build ./runtime.yaml # ERROR: Permission denied
rnx network create dev-net       # ERROR: Permission denied
rnx volume create dev-vol        # ERROR: Permission denied
```

### Reader Role (OU=reader)

```bash
# Read-only operations (allowed)
rnx job list
rnx job status <job-id>
rnx job log <job-id>
rnx runtime list
rnx network list
rnx volume list
rnx monitor status

# Create, execute, and delete operations (denied)
rnx job run echo "test"          # ERROR: Permission denied
rnx job stop <job-id>            # ERROR: Permission denied
rnx volume create test           # ERROR: Permission denied
```

### Multi-User Setup

```bash
# Generate certificates for different users
# DevOps team (admin access)
openssl req -new -key devops-key.pem -out devops.csr \
  -subj "/CN=devops-team/OU=admin"

# CI/CD service account (maintainer access)
openssl req -new -key cicd-key.pem -out cicd.csr \
  -subj "/CN=cicd-pipeline/OU=maintainer"

# Developers (developer access)
openssl req -new -key dev-key.pem -out dev.csr \
  -subj "/CN=developer/OU=developer"

# Reporting/observability (reader access)
openssl req -new -key report-key.pem -out report.csr \
  -subj "/CN=reporting/OU=reader"

# Sign all certificates with CA
for cert in devops cicd dev report; do
  openssl x509 -req -in ${cert}.csr -CA ca-cert.pem -CAkey ca-key.pem \
    -out ${cert}-cert.pem -days 365 -CAcreateserial
done
```

## Process Isolation

### Service-Based Isolation Architecture

Joblet implements automatic isolation based on which API service initiates jobs:

```bash
# Production Jobs (JobService API) - Minimal Chroot
rnx job run echo "Hello World"           # Uses minimal chroot isolation
rnx job run --runtime=java:21 java App  # Secure runtime mounting

# Runtime Build Jobs (RuntimeService API) - Builder Chroot
rnx runtime build ./examples/java-21/runtime.yaml  # Uses builder chroot with host OS access
```

**Isolation Routing:**

```mermaid
flowchart TD
    N1["JobService API"] --> N2["JobType: standard"]
    N2 --> N3["Minimal Chroot<br/>- Production isolation<br/>- Secure runtime mounts<br/>- No package managers"]
    N4["RuntimeService API"] --> N5["JobType: runtime-build"]
    N5 --> N6["Builder Chroot<br/>- Controlled host access<br/>- Runtime building tools<br/>- Temporary modifications"]
```

### Dual Chroot System

#### Production Jobs (Minimal Chroot)

```bash
# Minimal filesystem access
rnx job run ls /                    # Limited directories
rnx job run which apt              # Command not found
rnx job run ls /opt/joblet         # No access to joblet internals
```

#### Runtime Builds (Builder Chroot)

```bash
# Controlled host OS access (ONLY during runtime building)
# - Full host filesystem via OverlayFS (host root is a read-only lower layer)
# - Writes are captured in an upper layer; the host filesystem is never modified
# - Package managers available (runtime builds share host networking)
# - Automatic cleanup creates isolated runtime structure
```

### Linux Namespaces

Both job types use identical namespace isolation:

```bash
# PID namespace - process isolation
rnx job run ps aux  # Only sees job processes

# Mount namespace - filesystem isolation
rnx job run mount  # Shows only job-specific mounts

# Network namespace - network isolation
rnx job run ip addr show  # Shows only job network interface

# IPC namespace - inter-process communication isolation
rnx job run ipcs  # No shared memory/semaphores from host

# UTS namespace - hostname isolation
rnx job run hostname  # Job-specific hostname

# Cgroup namespace - resource telematics
rnx job run cat /proc/cgroups  # Limited cgroup view
```

### Runtime Isolation Security

Joblet prevents host filesystem exposure through runtime cleanup:

```bash
# BEFORE cleanup (INSECURE): Runtime mounts point to host OS paths
# runtime.yml contained:
# - source: "usr/lib/jvm/java-21-openjdk-amd64"  # HOST PATH!

# AFTER cleanup (SECURE): Runtime mounts point to isolated copies  
# runtime.yml contains:
# - source: "isolated/usr/lib/jvm/java-21-openjdk-amd64"  # ISOLATED COPY

# Production jobs using runtimes are completely isolated
rnx job run --runtime=java:21 find /usr -type f | head -5
# Only shows isolated runtime files, not host OS files
```

**Runtime Directory Structure:**

```text
/opt/joblet/runtimes/openjdk-21/1.0.0/
├── isolated/                    # Self-contained runtime files
│   ├── usr/lib/jvm/            # Copied from host during build
│   ├── usr/bin/                # Runtime binaries (isolated)  
│   └── etc/ssl/certs/          # Certificates (isolated)
├── runtime.yml                 # Uses isolated/ paths only
└── runtime.yml.original        # Backup for audit
```

### Security Context

```bash
# Jobs run as unprivileged user
rnx job run id
# Output: uid=65534(nobody) gid=65534(nogroup)

# No sudo/setuid capabilities
rnx job run sudo echo "test"  # Command not found

# Limited filesystem access
rnx job run ls /root  # Permission denied
rnx job run ls /etc/shadow  # Permission denied
```

**Note**: Runtime-build jobs (e.g., `rnx runtime build`) are an exception and run as root to allow package
installation via `apt`. Standard jobs submitted via `rnx job run` always run as the unprivileged `nobody` user.

### Resource Limits (Security)

```bash
# Prevent fork bombs
rnx job run --max-cpu=100 :(){ :|:& };:  # Limited by CPU quota

# Prevent memory exhaustion
rnx job run --max-memory=512 bash -c 'a=(); while true; do a+=($a); done'  # Killed by OOM

# Prevent I/O attacks
rnx job run --max-iobps=1048576 dd if=/dev/zero of=/work/attack bs=1M  # Limited bandwidth
```

## Network Security

### Network Isolation

```bash
# Create isolated networks for different security zones
rnx network create dmz --cidr=10.1.0.0/24           # Public-facing
rnx network create internal --cidr=10.2.0.0/24      # Internal services
rnx network create secure --cidr=10.3.0.0/24        # Sensitive data

# Jobs in different networks cannot communicate
rnx job run --network=dmz ping 10.2.0.1        # Will fail
rnx job run --network=internal ping 10.3.0.1   # Will fail
```

### Zero-Trust Network Model

```bash
# No network access for sensitive processing
rnx job run --network=none --volume=sensitive-data \
  python process_classified.py

# Limited network access
rnx job run --network=internal --volume=app-data \
  python internal_service.py

# Full internet access (carefully controlled)
rnx job run --network=bridge \
  curl https://api.trusted-service.com
```

### Traffic Control

```bash
# Limit bandwidth to prevent data exfiltration
rnx job run --max-iobps=1048576 --network=bridge \
  curl https://malicious-site.com  # Limited to 1MB/s

# Monitor network usage
rnx job run --network=monitored iftop
```

## Data Security

### Sensitive Data Handling

```bash
# 1. Create encrypted volume
rnx volume create encrypted-data --size=10GB

# 2. Encrypt data before storage
rnx job run --volume=encrypted-data bash -c '
  echo "sensitive information" | \
  openssl enc -aes-256-cbc -k "$ENCRYPTION_KEY" \
  > /volumes/encrypted-data/secret.enc
'

# 3. Decrypt only when needed
rnx job run --volume=encrypted-data --env=ENCRYPTION_KEY=xxx bash -c '
  openssl enc -aes-256-cbc -d -k "$ENCRYPTION_KEY" \
  < /volumes/encrypted-data/secret.enc
'
```

### Secrets Management

```bash
# Avoid embedding secrets in commands (BAD)
rnx job run curl -H "Authorization: Bearer secret123" api.com

# Use environment variables (BETTER)
rnx job run --env=API_TOKEN=secret123 \
  curl -H "Authorization: Bearer \$API_TOKEN" api.com

# Use volume-based secrets (BEST)
echo "secret123" | rnx job run --volume=secrets bash -c '
  cat > /volumes/secrets/api-token
  chmod 600 /volumes/secrets/api-token
'

rnx job run --volume=secrets bash -c '
  API_TOKEN=$(cat /volumes/secrets/api-token)
  curl -H "Authorization: Bearer $API_TOKEN" api.com
'
```

### Data Classification

```bash
# Public data - no restrictions
rnx job run --network=bridge --volume=public-data \
  wget https://public-dataset.com/data.csv

# Internal data - network restrictions
rnx job run --network=internal --volume=internal-data \
  python process_internal.py

# Confidential data - maximum isolation
rnx job run --network=none --volume=confidential-data \
  python process_confidential.py

# Secret data - encrypted storage
rnx job run --network=none --volume=encrypted-secrets \
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
```

**Note on what is and isn't configurable:**

- **RBAC is always on and has no toggle.** Authorization is enforced from the client certificate's OU field on every
  request (see [Authorization and RBAC](#authorization-and-rbac)); there is no `security.enable_rbac` setting to turn
  it off.
- **setuid escalation is mitigated by the privilege drop, not by mount flags.** Standard jobs drop to uid/gid 65534
  (`nobody`) before the command executes, so a setuid binary in the chroot cannot elevate them. Joblet does not
  currently apply `nosuid`/`nodev`/`noexec` mount flags (the constants are defined in the codebase but not used in any
  mount call), and there is no `process.allow_setuid`, `process.default_user`, or `filesystem.readonly_rootfs` setting.
- **There is no built-in audit-log subsystem to configure.** The `security.audit.*` keys shown in earlier revisions of
  this guide are not implemented. Use standard logging plus your host's audit tooling instead.

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

> **Illustrative.** The `security.audit.*` block below is **not** a supported configuration subsystem - Joblet does not
> read these keys. Joblet writes operational logs (the daemon logs authorization decisions and job lifecycle events);
> route those to a file or syslog with your host's logging stack rather than expecting a built-in audit config.

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

# NOTE: the security.audit.* keys below are aspirational and NOT read by Joblet
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
RUNNING_JOBS=$(rnx job list --json | jq '[.[] | select(.status == "RUNNING")] | length')
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

> **Illustrative / aspirational - NOT currently supported.** The keys below (`security.audit.*`,
> `access_control.require_mfa`, `session_timeout`, `logging.immutable_logs`, `log_integrity_check`) are **not**
> implemented in Joblet and setting them has no effect. They are shown only to sketch the controls a SOC 2 program
> typically expects; implement them at the host/platform layer (external audit log shipping, MFA on the operator's
> access path, immutable log storage). RBAC and mTLS, which Joblet does enforce, are covered above.

```yaml
# Aspirational sketch - these keys are NOT read by Joblet
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
rnx job run --network=none --volume=phi-data \
  python anonymize_phi.py

# 5. Secure disposal
rnx volume remove phi-data  # Secure deletion
```

### PCI DSS Compliance

```bash
# PCI DSS network segmentation
rnx network create pci-zone --cidr=10.100.0.0/24

# Restricted processing environment
rnx job run \
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
if rnx job list --json | jq '.[] | select(.command | contains("nmap"))' | grep -q .; then
  echo "THREAT: Network scanning detected"
fi

# Detect data exfiltration patterns
if rnx job list --json | jq '.[] | select(.command | contains("curl") and .max_iobps == 0)' | grep -q .; then
  echo "THREAT: Potential data exfiltration (unlimited bandwidth)"
fi
EOF
```

### Incident Response Procedures

```bash
# 1. Immediate response
# Stop suspicious jobs
rnx job list --json | jq -r '.[] | select(.status == "RUNNING" and (.command | contains("suspicious"))) | .id' | xargs rnx job stop

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
2. **Implement RBAC** - Give each user the least-privileged role (reader for read-only access)
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
6. **Don't share admin certificates** - Use maintainer, developer, or reader certificates for most users
7. **Don't run Joblet as non-root** - It needs privileges for isolation
8. **Don't trust user input** - Validate and sanitize all inputs

## See Also

- [Configuration Guide](./CONFIGURATION.md) - Security configuration
- [Network Management](./NETWORK_MANAGEMENT.md) - Network isolation
- [Installation Guide](./INSTALLATION.md) - Secure installation
- [Troubleshooting](./TROUBLESHOOTING.md) - Security issues