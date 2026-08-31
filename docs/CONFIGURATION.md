# Configuration Guide

Comprehensive guide to configuring Joblet server and RNX client.

## Table of Contents

- [Server Configuration](#server-configuration)
    - [Basic Configuration](#basic-configuration)
    - [Resource Limits](#resource-limits)
    - [Network Configuration](#network-configuration)
    - [Volume Configuration](#volume-configuration)
    - [Security Settings](#security-settings)
    - [Buffer Configuration](#buffer-configuration)
    - [Persistence Configuration](#persistence-configuration)
    - [Telemetry Configuration](#telemetry-configuration)
    - [State Persistence Configuration](#state-persistence-configuration)
    - [Logging Configuration](#logging-configuration)
- [Client Configuration](#client-configuration)
    - [Single Node Setup](#single-node-setup)
    - [Multi-Node Setup](#multi-node-setup)
    - [Authentication Roles](#authentication-roles)
- [Environment Variables](#environment-variables)
- [Configuration Examples](#configuration-examples)

## Server Configuration

Joblet uses a **split configuration architecture** for cross-distribution compatibility:

| File                 | Purpose                                          | Location              |
|----------------------|--------------------------------------------------|-----------------------|
| `joblet-config.yml`  | Core joblet config (server, IPC, persist, state) | `/opt/joblet/config/` |
| `runtime-config.yml` | Distro-specific runtime settings                 | `/opt/joblet/config/` |

### Configuration Files

**Main config:** `/opt/joblet/config/joblet-config.yml`

- Server settings, IPC, persistence, state, security
- Distro-agnostic settings

**Runtime config:** `/opt/joblet/config/runtime-config.yml`

- Package manager paths (apt/yum/dnf/apk)
- Library paths for the specific Linux distribution
- Automatically selected during installation based on OS detection

### Automatic Distro Detection

During installation, Joblet automatically detects your Linux distribution and installs the appropriate runtime config:

| Distribution                   | Runtime Config Selected     |
|--------------------------------|-----------------------------|
| Ubuntu, Debian, Linux Mint     | `runtime-config-ubuntu.yml` |
| RHEL, CentOS, Rocky, AlmaLinux | `runtime-config-rhel.yml`   |
| Fedora, Amazon Linux 2023+     | `runtime-config-fedora.yml` |
| Alpine Linux                   | `runtime-config-alpine.yml` |

The detection uses `/etc/os-release` and falls back to package manager detection.

### Basic Configuration

```yaml
version: "3.0"

server:
  mode: "server"                    # Always "server" for daemon mode
  address: "0.0.0.0"               # Listen address
  port: 50051                      # gRPC port
  nodeId: ""                       # Unique node identifier (UUID, auto-generated during setup)
```

The `server:` section holds only these four fields. TLS is always required
(mTLS) and is configured via embedded certificates under [`security:`](#security-settings);
gRPC message sizes and keepalive settings live under the [`grpc:`](#grpc-configuration)
section.

### Node Identification

Joblet supports unique node identification for distributed deployments:

```yaml
server:
  nodeId: "8f94c5b2-1234-5678-9abc-def012345678"  # Unique UUID for this node
```

**Key Features:**

- **Automatic Generation**: During Joblet setup, a unique UUID is automatically generated and stored in the
  configuration
- **Job Tracking**: All jobs executed on a node are tagged with the node's UUID for tracking and debugging
- **Distributed Telematics**: In multi-node deployments, you can identify which node executed specific jobs
- **CLI Display**: The node ID is displayed in `rnx job list` and `rnx job status` commands

**Setup Process:**

The `nodeId` is automatically populated during Joblet installation via the `certs_gen_embedded.sh` script:

```bash
# Generates a UUID and updates the configuration
NODE_ID=$(uuidgen)
sed -i "s/nodeId: \"\"/nodeId: \"$NODE_ID\"/" /opt/joblet/config/joblet-config.yml
```

**Manual Configuration:**

If needed, you can manually set a custom node ID:

```yaml
server:
  nodeId: "custom-node-identifier-uuid"
```

**Note**: The nodeId should be a valid UUID format for consistency with the system's expectations.

### Resource Limits

```yaml
joblet:
  # Default resource limits for jobs
  defaultCpuLimit: 100            # Default CPU limit (100 = 1 core)
  defaultMemoryLimit: 512         # Default memory limit in MB
  defaultIoLimit: 0               # Default I/O limit in bytes/sec (0 = unlimited)

  # Job execution settings
  maxConcurrentJobs: 100          # Maximum concurrent jobs
  jobTimeout: "1h"                # Maximum job runtime (0 = unlimited). Can be overridden per-job with --timeout flag

  # Cleanup settings
  cleanupTimeout: "5s"            # Timeout for cleanup operations
```

These six fields are the entire `joblet:` section. Job isolation (chroot,
namespaces, OverlayFS runtime isolation) is applied automatically and is not
configurable here.

### Network Configuration

```yaml
network:
  enabled: true                    # Enable network management
  state_dir: "/opt/joblet/network" # Network state directory

  # Default network settings
  default_network: "bridge"        # Default network for jobs
  allow_custom_networks: true      # Allow custom network creation

  # Persistent network state storage
  storage:
    path: "/opt/joblet/network"    # Where network state is persisted

  # Predefined networks. Each network has only these two fields.
  networks:
    bridge:
      cidr: "172.20.0.0/16"        # Bridge network CIDR
      bridge_name: "joblet0"       # Bridge interface name
```

The built-in `host` and `none` networks (host namespace / no network) are
handled internally and are not declared in this section. Each entry under
`networks:` accepts only `cidr` and `bridge_name`.

### Volume Configuration

```yaml
volumes:
  base_path: "/opt/joblet/volumes"      # Volume storage path
  default_disk_quota_bytes: 1048576     # Default per-volume disk quota in bytes (1MB)
```

The `volumes:` section (note the plural key) has only these two fields.

### Runtime Configuration

Runtime configuration is stored in a **separate file** (`runtime-config.yml`) for cross-distribution compatibility.
The appropriate config is automatically selected during installation based on your Linux distribution.

**File location:** `/opt/joblet/config/runtime-config.yml`

```yaml
# Example: runtime-config-ubuntu.yml (auto-selected for Ubuntu/Debian)
runtime:
  base_path: "/opt/joblet/runtimes"

  common_paths:
    - "/usr/local/bin"
    - "/usr/local/lib"
    - "/usr/lib/jvm"
    - "/usr/local/node"
    - "/usr/local/go"

  # Note: Runtime builds use OverlayFS-based isolation (see pkg/builder/isolation.go)
  # The entire host filesystem is mounted read-only as the lower layer,
  # and all package installations write to an ephemeral upper layer.
  # No additional configuration is needed for runtime builds.

  # Paths mounted read-only into job sandbox (for job execution, not builds)
  allowed_mounts:
    - "/usr/bin"
    - "/bin"
    - "/usr/sbin"
    - "/lib"
    - "/lib64"
    - "/usr/lib"
    - "/usr/lib64"
    - "/etc/resolv.conf"
    - "/etc/hosts"
    - "/etc/nsswitch.conf"
    - "/etc/ssl"
    - "/etc/pki"
    - "/etc/ca-certificates"
    - "/usr/share/ca-certificates"
```

### Security Settings

```yaml
security:
  # Embedded certificates (generated by certs_gen_embedded.sh)
  serverCert: |
    -----BEGIN CERTIFICATE-----
    MIIFKzCCAxOgAwIBAgIUY8Z9...
    -----END CERTIFICATE-----

  serverKey: |
    -----BEGIN PRIVATE KEY-----
    MIIJQwIBADANBgkqhkiG9w0BAQ...
    -----END PRIVATE KEY-----

  caCert: |
    -----BEGIN CERTIFICATE-----
    MIIFazCCA1OgAwIBAgIUX...
    -----END CERTIFICATE-----
```

The `security:` section holds only these three embedded PEM fields:
`serverCert`, `serverKey`, and `caCert`. There are no toggles for client-cert
verification or RBAC - mTLS is always enforced (the server requires and
verifies client certificates against `caCert`), and role-based access control
is always on. A client's role is derived from the OU field of its certificate:
`admin`, `maintainer`, `developer`, or `reader` (the old `viewer` still works
and maps to `reader`). Any other OU is rejected.

### Buffer Configuration

```yaml
buffers:
  # Pub-sub configuration for job events and log streaming
  pubsub_buffer_size: 10000      # Pub-sub channel buffer for high-throughput (default: 10000)
  chunk_size: 1048576            # 1MB chunks for optimal streaming performance (default: 1MB)
```

**Buffer System Tuning:**

- `pubsub_buffer_size`: Channel buffer size for job event streaming (default: 10000)
- `chunk_size`: Chunk size for upload/download streaming operations (default: 1MB)

### Persistence Configuration

**⚠️ IMPORTANT: `ipc.enabled` controls BOTH persistence AND in-memory buffering behavior.**

```yaml
# IPC configuration for persist integration (joblet -> persist communication)
# This setting controls BOTH persistence AND buffering:
#   enabled: true  - Logs/metrics buffered in memory + forwarded to persist (gap prevention enabled)
#   enabled: false - NO buffering (live streaming only, no persistence, no historical data)
#
# NOTE: The socket path here is the SINGLE SOURCE OF TRUTH - persist.ipc inherits it automatically
ipc:
  enabled: true                                   # Enable IPC to persist service + in-memory buffering
  socket: "/opt/joblet/run/persist-ipc.sock"      # Unix socket path (shared with persist.ipc)
  buffer_size: 10000                              # Client: message buffer size
  reconnect_delay: "5s"                           # Client: reconnection retry delay
  max_reconnects: 0                               # Client: max reconnection attempts (0 = infinite)

# Persistence service configuration (only used when ipc.enabled: true)
persist:
  server:
    grpc_socket: "/opt/joblet/run/persist-grpc.sock"  # Unix socket for queries
    max_connections: 500

  ipc:
    # socket: inherited from top-level ipc.socket (single source of truth)
    max_message_size: 134217728  # 128MB

  storage:
    type: "local"  # Options: "local", "cloudwatch", "s3"

    local:
      logs:
        directory: "/opt/joblet/logs"
      metrics:
        directory: "/opt/joblet/metrics"
      events:
        directory: "/opt/joblet/events"  # eBPF events storage

    # CloudWatch configuration (when type: "cloudwatch")
    cloudwatch:
      region: "us-west-2"           # AWS region
      log_group_prefix: "/joblet"   # CloudWatch log group prefix
      # Log streams created per job:
      # - {job_uuid}-logs           (stdout/stderr)
      # - {job_uuid}-metrics        (resource metrics)
      # - {job_uuid}-exec-events    (eBPF process execution)
      # - {job_uuid}-connect-events (eBPF network connections)

    # S3 configuration (when type: "s3")
    s3:
      region: "us-east-1"              # Required: AWS region
      bucket: "my-joblet-data"         # Required: S3 bucket name
      key_prefix: "jobs/"              # Optional: Object key prefix (default: "jobs/")
      flush_interval: 30               # Seconds between flushes (default: 30)
      flush_threshold: 5242880         # Bytes before flush (default: 5MB)
      max_buffer_size: 52428800        # Max buffer before blocking (default: 50MB)
      storage_class: "STANDARD"        # S3 storage class (default: STANDARD)
      sse: "AES256"                    # Server-side encryption: "", "AES256", or "aws:kms"
      kms_key_id: ""                   # KMS key ID if sse="aws:kms"
```

### Telemetry Configuration

Configure resource metrics collection and eBPF-based activity tracking:

```yaml
telemetry:
  # Resource metrics collection interval (cgroups v2)
  # How often to sample CPU, memory, disk I/O, and network metrics
  metrics_interval: "5s"     # Default: 5 seconds (minimum: 1s)

  # eBPF activity tracking (Linux 5.8+ required)
  ebpf_enabled: true         # Enable eBPF telematics (default: true)

  # List of enabled event types (omit or leave empty for all)
  # Valid values: exec, connect, accept, mmap, mprotect, file, socket_data
  event_types:
    - exec                   # Process execution events
    - connect                # Outbound network connections
    - accept                 # Inbound network connections
    # - mmap                 # Memory mappings - HIGH VOLUME
    # - mprotect             # Memory protection changes
    # - file                 # File operations
    # - socket_data          # Socket send/recv - HIGH VOLUME
```

**Metrics Interval Tuning:**

| Interval | Use Case                   | Trade-off                      |
|----------|----------------------------|--------------------------------|
| `1s`     | High-resolution debugging  | Higher CPU overhead, more data |
| `5s`     | Default, balanced          | Good for most workloads        |
| `10s`    | Long-running jobs          | Lower overhead, less granular  |
| `30s`    | Cost-sensitive/high-volume | Minimal overhead, coarse data  |

**eBPF Event Types:**

| Event         | Description                                    | Use Case                            |
|---------------|------------------------------------------------|-------------------------------------|
| `exec`        | Process execution (fork/exec syscalls)         | Debug what binaries jobs run        |
| `connect`     | Outgoing network connections (connect syscall) | Track external service dependencies |
| `accept`      | Incoming network connections (accept syscall)  | Monitor server connections          |
| `socket_data` | Socket data transfers (sendto/recvfrom)        | Monitor data flow                   |
| `mmap`        | Memory mappings with exec permissions          | Detect code loading                 |
| `mprotect`    | Memory protection changes adding exec          | Detect JIT compilation              |
| `file`        | File access (open/read/write)                  | Audit data access (high volume)     |

**Performance Tuning - Disabling High-Volume Events:**

If you experience performance issues with eBPF telematics, list only the events you need:

```yaml
# Performance-optimized configuration (minimal overhead)
# Only list the events you want - omit high-volume ones
telemetry:
  ebpf_enabled: true
  # Valid: exec, connect, accept, mmap, mprotect, file, socket_data
  event_types:
    - exec      # Keep - low volume, high value
    - connect   # Keep - low volume, high value
    - accept    # Keep - low volume, high value
    # High-volume events omitted: mmap, mprotect, file, socket_data
```

**Recommended profiles:**

| Profile        | Events                      | Config                                                       |
|----------------|-----------------------------|--------------------------------------------------------------|
| Minimal        | `exec`, `connect`, `accept` | `event_types: [exec, connect, accept]`                       |
| Standard       | All except `socket_data`    | `event_types: [exec, connect, accept, mmap, mprotect, file]` |
| Full (default) | All events                  | Omit `event_types` or leave empty                            |

**Requirements:**

- Linux kernel 5.8+ (for eBPF ring buffer)
- `CAP_BPF` and `CAP_PERFMON` capabilities (joblet runs as root)

**CloudWatch Integration:**

When using CloudWatch storage backend, eBPF events are shipped to dedicated log streams:

```text
Log Group: /joblet/{node_id}
  {job_uuid}-exec-events     # Process execution events (JSON)
  {job_uuid}-connect-events  # Network connection events (JSON)
```

Query eBPF events with CloudWatch Insights:

```sql
-- Find all network connections to a specific host
fields @timestamp, job_uuid, pid, dst_addr, dst_port
| filter dst_addr = "10.0.1.50"
| sort @timestamp desc
```

**When to enable persistence (`ipc.enabled: true`):**

- Production environments requiring audit trails
- Long-running jobs where historical data is needed
- Multi-user environments where users connect at different times
- Compliance requirements for log retention

**When to disable persistence (`ipc.enabled: false`):**

- Development and testing environments
- Real-time monitoring where history is not needed
- Resource-constrained environments
- Temporary jobs where logs are consumed immediately

**Memory Impact:**

- **Persist enabled**: Bounded memory (~1000 log chunks + 100 metric samples per job)
- **Persist disabled**: No buffering at all (live streaming only)

See [PERSISTENCE.md](PERSISTENCE.md) for detailed persistence configuration.

### State Persistence Configuration

Job state persistence ensures job metadata survives system restarts. Unlike persist (which stores logs/metrics), the
state service stores job status, exit codes, and metadata.

```yaml
state:
  backend: "memory"  # Options: "memory", "dynamodb", "local"
  socket: "/opt/joblet/run/state-ipc.sock"      # Unix socket for state operations
  buffer_size: 10000                             # Message buffer size
  reconnect_delay: "5s"                          # Reconnection retry delay

  # Connection pool configuration (for high-concurrency scenarios with 1000+ jobs)
  pool:
    size: 20                      # Max connections in pool (default: 20)
    read_timeout: "10s"           # Timeout for read operations (default: 10s)
    dial_timeout: "5s"            # Timeout for establishing new connections (default: 5s)
    max_idle_time: "30s"          # Max idle time before health check (default: 30s)
    health_check_timeout: "500ms" # Timeout for connection health checks (default: 500ms)
    shutdown_timeout: "5s"        # Max time to wait for graceful shutdown (default: 5s)

  # Client retry configuration (for transient failures)
  client:
    max_retries: 3                # Max retry attempts for transient failures (default: 3)
    retry_base_delay: "100ms"     # Initial delay between retries, doubles each attempt (default: 100ms)
    retry_max_delay: "2s"         # Maximum delay between retries (default: 2s)
    connect_timeout: "5s"         # Timeout for initial connection test (default: 5s)

  # Local storage configuration (when backend: "local")
  local:
    directory: "/opt/joblet/state"  # Directory for local state storage
    sync_interval: "5s"             # How often to sync to disk (default: 5s)

  storage:
    # DynamoDB configuration (when backend: "dynamodb")
    dynamodb:
      region: ""  # AWS region (empty = auto-detect from EC2 metadata)
      table_name: "joblet-jobs"
      ttl_enabled: true
      ttl_attribute: "expiresAt"
      ttl_days: 30  # Auto-delete completed jobs after 30 days
      read_capacity: 5   # 0 for on-demand pricing
      write_capacity: 5  # 0 for on-demand pricing
      batch_size: 25
      batch_interval: "100ms"
```

**Backend Options:**

- **memory**: Jobs persist in RAM only (default, lost on restart)
- **local**: Jobs persist to local filesystem (survives restarts, single-node)
- **dynamodb**: Jobs persist in AWS DynamoDB (EC2 only, production, survives restarts)

**When to use DynamoDB state persistence:**

✅ Production AWS deployments where jobs must survive restarts
✅ Auto-scaling EC2 fleets where instances may be replaced
✅ Disaster recovery scenarios requiring durable state
✅ Multi-node distributed deployments

❌ Development/testing environments
❌ Single-node setups where restarts are infrequent
❌ Cost-sensitive deployments with short-lived jobs

**Performance characteristics:**

All state operations use async fire-and-forget pattern with connection pooling:

- Non-blocking create/update/delete operations
- Configurable timeout per operation (default: 10s via `pool.read_timeout`)
- Connection pool handles 1000+ concurrent jobs efficiently
- Automatic reconnection if state service restarts
- High-throughput regardless of job count (200x faster than previous implementation)
- Automatic retry with exponential backoff for transient failures

**Pool Size Recommendations:**

- < 100 jobs: Default (20) is sufficient
- 100-1000 jobs: Default (20) handles well
- 1000-2500 jobs: Consider 30-50 via `pool.size`
- > 2500 jobs: 50-100+ depending on workload

See [STATE_PERSISTENCE.md](./STATE_PERSISTENCE.md) for detailed state persistence documentation including performance
characteristics, DynamoDB setup, monitoring, and troubleshooting.

### Logging Configuration

```yaml
logging:
  level: "INFO"                  # Log level: DEBUG, INFO, WARN, ERROR (case-insensitive)
  format: "text"                 # Log format: text or json
  output: "stdout"               # Single output destination (e.g. "stdout")
```

The `logging:` section has exactly three fields: `level`, `format`, and a
single `output` string. There is no multi-output list or per-component log
configuration.

### Advanced Settings

```yaml
# Cgroup configuration (cgroups v2 only)
cgroup:
  baseDir: "/sys/fs/cgroup/joblet.slice/joblet.service" # Cgroup hierarchy path
  namespaceMount: "/sys/fs/cgroup"                       # Cgroup namespace mount point

  # Controllers to enable
  enableControllers:
    - cpu
    - memory
    - io
    - pids
    - cpuset
    - devices

  cleanupTimeout: "5s"           # Timeout for cgroup cleanup operations

# Filesystem isolation
filesystem:
  baseDir: "/opt/joblet/jobs"    # Base directory for job workspaces
  tmpDir: "/tmp/job-{JOB_ID}"    # Per-job temporary directory template
  workspaceDir: "/work"          # Workspace mount point inside the job

# Monitoring configuration (system-level host metrics)
monitoring:
  system_interval: "10s"         # How often to sample host system metrics
  cloud_detection: true          # Detect cloud provider/instance metadata
```

The `cgroup:`, `filesystem:`, and `monitoring:` sections have only the fields
shown above. Chroot isolation and job namespaces are applied automatically and
are not configurable. There is no separate `process:` section.

### gRPC Configuration

gRPC message sizes and keepalive/timeout settings live in the `grpc:` section:

```yaml
grpc:
  maxRecvMsgSize: 134217728        # Max received message size (default: 128MB)
  maxSendMsgSize: 134217728        # Max sent message size (default: 128MB)
  maxHeaderListSize: 16777216      # Max header list size (default: 16MB)
  keepAliveTime: "10s"             # Keepalive ping interval
  keepAliveTimeout: "3s"           # Keepalive ping timeout
  maxConcurrentStreams: 1000       # Max concurrent streams per connection
  connectionTimeout: "10s"         # Connection establishment timeout
```

## Client Configuration

The RNX client configuration file is typically located at `~/.rnx/rnx-config.yml`.

### Single Node Setup

```yaml
version: "3.0"

nodes:
  admin:
    address: "joblet-server:50051"
    nodeId: "8f94c5b2-1234-5678-9abc-def012345678"  # Optional: Joblet node identifier
    isDefault: true  # Used when --node is not specified; only one node may set this

    # Embedded certificates
    cert: |
      -----BEGIN CERTIFICATE-----
      MIIFLDCCAxSgAwIBAgIUd...
      -----END CERTIFICATE-----

    key: |
      -----BEGIN PRIVATE KEY-----
      MIIJQgIBADANBgkqhkiG9w0BAQ...
      -----END PRIVATE KEY-----

    ca: |
      -----BEGIN CERTIFICATE-----
      MIIFazCCA1OgAwIBAgIUX...
      -----END CERTIFICATE-----

    # Connection settings
    timeout: "30s"
    keepalive: "120s"

    # Retry configuration
    retry:
      enabled: true
      max_attempts: 3
      backoff: "1s"
```

### Multi-Node Setup

```yaml
version: "3.0"

default_node: "production"

# Global settings
global:
  timeout: "30s"
  keepalive: "120s"

nodes:
  production:
    address: "prod.joblet.company.com:50051"
    nodeId: "a1b2c3d4-5678-9abc-def0-123456789012"  # Production node identifier
    cert: |
      -----BEGIN CERTIFICATE-----
      # Production admin certificate
      -----END CERTIFICATE-----
    key: |
      -----BEGIN PRIVATE KEY-----
      # Production admin key
      -----END PRIVATE KEY-----
    ca: |
      -----BEGIN CERTIFICATE-----
      # Company CA certificate
      -----END CERTIFICATE-----

  staging:
    address: "staging.joblet.company.com:50051"
    nodeId: "b2c3d4e5-6789-abcd-ef01-23456789abcd"  # Staging node identifier
    cert: |
      -----BEGIN CERTIFICATE-----
      # Staging admin certificate
      -----END CERTIFICATE-----
    # ... rest of credentials

  development:
    address: "dev.joblet.company.com:50051"
    nodeId: "c3d4e5f6-789a-bcde-f012-3456789abcde"  # Development node identifier
    cert: |
      -----BEGIN CERTIFICATE-----
      # Dev admin certificate
      -----END CERTIFICATE-----
    # ... rest of credentials

  reader:
    address: "prod.joblet.company.com:50051"
    nodeId: "a1b2c3d4-5678-9abc-def0-123456789012"  # Same as production (read-only access)
    cert: |
      -----BEGIN CERTIFICATE-----
      # Reader certificate (OU=reader)
      -----END CERTIFICATE-----
    # ... rest of credentials

# Client preferences
preferences:
  output_format: "table"         # Default output format
  color_output: true            # Enable colored output
  confirm_destructive: true     # Confirm before destructive operations

  # Upload settings
  upload:
    chunk_size: 1048576         # Upload chunk size (1MB)
    compression: true           # Compress uploads
    show_progress: true         # Show upload progress
```

### Node Identification

The `nodeId` field in client configuration provides display information about which Joblet node is being connected to:

**Key Features:**

- **Optional Field**: The `nodeId` is optional and used only for display purposes in `rnx nodes` command
- **Automatic Population**: When using `certs_gen_embedded.sh`, the nodeId is automatically populated from the server's
  nodeId
- **Multi-Node Tracking**: Helps identify which physical Joblet server each configuration entry connects to
- **Job Correlation**: Can be used to correlate job execution with specific nodes when viewing job status

**Usage:**

```bash
# View configured nodes with their nodeId information
rnx nodes

# Example output shows node identifiers:
# * default
#    Address: localhost:50051
#    Node ID: 8f94c5b2-1234-5678-9abc-def012345678
#    Cert:    ***
#    Key:     ***
#    CA:      ***
```

**Manual Configuration:**

You can manually add nodeId to existing configurations:

```yaml
nodes:
  my-server:
    address: "server.example.com:50051"
    nodeId: "server-node-uuid-here"  # Add this line
    cert: |
      # ... existing certificate
```

**Note**: The nodeId should match the server's nodeId (configured in `joblet-config.yml`) for accurate tracking.

### Authentication Roles

A client's role comes from the OU field of its certificate (case doesn't matter). Certificates without one of these
OUs are rejected on every request.

| Role         | Access                                                                                                                                    |
|--------------|-------------------------------------------------------------------------------------------------------------------------------------------|
| `admin`      | Everything, including removing runtimes, networks, and volumes                                                                            |
| `maintainer` | Developer access plus building runtimes, validating runtime YAML, and creating networks and volumes; intended for CI/CD. No removals     |
| `developer`  | Run, stop, and delete jobs; test runtimes; read everything. No infrastructure changes                                                     |
| `reader`     | Read-only: jobs, logs, telemetry, and resource listings, for dashboards and reporting                                                    |

Certificates issued with the old `viewer` OU keep working; they are treated as `reader`.

```yaml
# Certificate subjects per role
# /CN=admin-client/OU=admin
# /CN=maintainer-client/OU=maintainer
# /CN=developer-client/OU=developer
# /CN=reader-client/OU=reader
```

Both certificate scripts (`scripts/certs_gen_embedded.sh` and the AWS variant
`scripts/certs_gen_with_secretsmanager.sh`) generate one client certificate per role and write two kinds of client
config:

- `rnx-config.yml`: the operator's copy, with one node per role; the `admin` node is marked `isDefault: true`. It
  contains the admin key, so it stays on the server. On the server, select a role with `rnx --node <role> ...`.
- `rnx-config-<role>.yml`: one self-contained file per role, with that role's node marked `isDefault: true`. Hand each party the
  file for its role and nothing else; a developer holding `rnx-config-developer.yml` never sees the admin key.

The AWS variant additionally stores each role's certificate pair in Secrets Manager
(`joblet/client-cert-<role>` and `joblet/client-key-<role>`; the unsuffixed `joblet/client-cert` and
`joblet/client-key` are the admin pair, kept under their original names). Scope each client's IAM policy to only its
role's secrets so clients can fetch their own credentials without any file distribution.

Generate role-specific certificates manually:

```bash
# Admin certificate
openssl req -new -key client-key.pem -out admin.csr \
  -subj "/CN=admin-client/OU=admin"

# Maintainer certificate
openssl req -new -key client-key.pem -out maintainer.csr \
  -subj "/CN=maintainer-client/OU=maintainer"

# Developer certificate
openssl req -new -key client-key.pem -out developer.csr \
  -subj "/CN=developer-client/OU=developer"

# Reader certificate
openssl req -new -key client-key.pem -out reader.csr \
  -subj "/CN=reader-client/OU=reader"
```

## Environment Variables

### Server Environment Variables

| Variable                     | Description                        | Default                                 |
|------------------------------|------------------------------------|-----------------------------------------|
| `JOBLET_CONFIG_PATH`         | Path to main configuration file    | searches standard locations             |
| `JOBLET_RUNTIME_CONFIG_PATH` | Path to runtime configuration file | searches standard locations             |
| `JOBLET_SERVER_ADDRESS`      | Server address override            | from config                             |
| `JOBLET_MODE`                | Server mode override (`server`/`init`) | from config                         |
| `JOBLET_LOG_LEVEL`           | Log level override                 | from config                             |
| `JOBLET_LOG_FORMAT`          | Log format override (`text`/`json`) | from config                            |

### Client Environment Variables

| Variable     | Description                | Default                     |
|--------------|----------------------------|-----------------------------|
| `RNX_CONFIG` | Path to configuration file | searches standard locations |

> **Note**: Use `--config`, `--node`, and `--json` flags for node selection and output format control.

## Configuration Examples

### High-Security Production Setup

TLS (mTLS) is always enforced via the embedded certificates under `security:`,
so there are no TLS or RBAC toggles to set here.

```yaml
version: "3.0"

server:
  address: "0.0.0.0"
  port: 50051

joblet:
  maxConcurrentJobs: 50
  jobTimeout: "1h"

security:
  serverCert: |
    -----BEGIN CERTIFICATE-----
    ...
    -----END CERTIFICATE-----
  serverKey: |
    -----BEGIN PRIVATE KEY-----
    ...
    -----END PRIVATE KEY-----
  caCert: |
    -----BEGIN CERTIFICATE-----
    ...
    -----END CERTIFICATE-----
```

### Development Environment Setup

```yaml
version: "3.0"

server:
  address: "0.0.0.0"
  port: 50051

joblet:
  defaultCpuLimit: 0      # No limits in dev
  defaultMemoryLimit: 0
  defaultIoLimit: 0

logging:
  level: "DEBUG"
  format: "text"

network:
  networks:
    bridge:
      cidr: "172.30.0.0/16"
      bridge_name: "joblet0"

volumes:
  base_path: "/opt/joblet/volumes"
  default_disk_quota_bytes: 1048576
```

### CI/CD Optimized Setup

```yaml
version: "3.0"

server:
  address: "0.0.0.0"
  port: 50051

joblet:
  maxConcurrentJobs: 200
  jobTimeout: "30m"
  cleanupTimeout: "5s"

logging:
  level: "WARN"        # Reduce log volume
  format: "json"       # Structured logs for CI
  output: "stdout"
```

## Best Practices

1. **Security First**: Always use TLS and client certificates in production
2. **Resource Limits**: Set appropriate defaults to prevent resource exhaustion
3. **Monitoring**: Enable metrics collection for production environments
4. **Logging**: Use JSON format for easier log parsing
5. **State Persistence**: Use the `local` or `dynamodb` state backend so jobs survive restarts
6. **Access Control**: Issue per-role client certificates (OU = admin/maintainer/developer/reader); RBAC is enforced automatically via mTLS
7. **Backup**: Keep configuration file backups

## Configuration Validation

Validate your configuration:

```bash
# Server configuration
joblet --config=/opt/joblet/config/joblet-config.yml --validate

# Client configuration
rnx --config=~/.rnx/rnx-config.yml nodes
```

## Troubleshooting

See [Troubleshooting Guide](./TROUBLESHOOTING.md) for configuration-related issues.