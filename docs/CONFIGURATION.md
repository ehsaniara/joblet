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
    - [State Persistence Configuration](#state-persistence-configuration)
    - [Logging Configuration](#logging-configuration)
- [Client Configuration](#client-configuration)
    - [Single Node Setup](#single-node-setup)
    - [Multi-Node Setup](#multi-node-setup)
    - [Authentication Roles](#authentication-roles)
- [Environment Variables](#environment-variables)
- [Configuration Examples](#configuration-examples)

## Server Configuration

The Joblet server configuration file is typically located at `/opt/joblet/config/joblet-config.yml`.

### Basic Configuration

```yaml
version: "3.0"

server:
  mode: "server"                    # Always "server" for daemon mode
  address: "0.0.0.0"               # Listen address
  port: 50051                      # gRPC port
  nodeId: ""                       # Unique node identifier (UUID, auto-generated during setup)

  # TLS configuration
  tls:
    enabled: true                  # Enable TLS (recommended)
    min_version: "1.3"            # Minimum TLS version

  # Connection settings
  max_message_size: 104857600     # Max gRPC message size (100MB)
  keepalive:
    time: 120s                    # Keepalive time
    timeout: 20s                  # Keepalive timeout
```

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
- **Distributed Visibility**: In multi-node deployments, you can identify which node executed specific jobs
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
  defaultIoLimit: 10485760        # Default I/O limit in bytes/sec (10MB/s)

  # Job execution settings
  maxConcurrentJobs: 100          # Maximum concurrent jobs
  jobTimeout: "24h"               # Maximum job runtime

  # Command validation
  validateCommands: true          # Validate commands before execution

  # Cleanup settings
  cleanupTimeout: "30s"          # Timeout for cleanup operations

  # Isolation configuration
  isolation:
    service_based_routing: true   # Enable automatic service-based job routing

    # Production jobs (JobService API)
    production:
      type: "minimal_chroot"      # Minimal chroot isolation
      allowed_mounts: # Read-only host directories
        - "/bin"
        - "/usr/bin"
        - "/lib"
        - "/usr/lib"
        - "/lib64"
        - "/usr/lib64"
      runtime_isolation: true     # Use isolated runtime copies

    # Runtime build jobs (RuntimeService API) 
    builder:
      type: "builder_chroot"      # Builder chroot with controlled host access
      host_access: "readonly"     # Host filesystem access level
      runtime_cleanup: true       # Automatic runtime cleanup after build
      cleanup_on_completion: true # Clean up builder environment
```

### Network Configuration

```yaml
network:
  enabled: true                   # Enable network management
  state_dir: "/opt/joblet/network" # Network state directory

  # Default network settings
  default_network: "bridge"       # Default network for jobs
  allow_custom_networks: true     # Allow custom network creation
  max_custom_networks: 50         # Maximum custom networks

  # Predefined networks
  networks:
    bridge:
      cidr: "172.20.0.0/16"      # Bridge network CIDR
      bridge_name: "joblet0"      # Bridge interface name
      enable_nat: true            # Enable NAT for internet access
      enable_icc: true            # Inter-container communication

    host:
      type: "host"                # Use host network namespace

    none:
      type: "none"                # No network access

  # DNS configuration
  dns:
    servers:
      - "8.8.8.8"
      - "8.8.4.4"
    search:
      - "local"
    options:
      - "ndots:1"

  # Traffic control
  traffic_control:
    enabled: true                 # Enable bandwidth limiting
    default_ingress: 0            # Default ingress limit (0 = unlimited)
    default_egress: 0             # Default egress limit
```

### Volume Configuration

```yaml
volume:
  enabled: true                   # Enable volume management
  state_dir: "/opt/joblet/state"  # Volume state directory
  base_path: "/opt/joblet/volumes" # Volume storage path

  # Volume limits
  max_volumes: 100                # Maximum number of volumes
  max_size: "100GB"              # Maximum total volume size
  default_size: "1GB"            # Default volume size

  # Volume types configuration
  filesystem:
    enabled: true
    default_fs: "ext4"           # Default filesystem type
    mount_options: "noatime,nodiratime"

  memory:
    enabled: true
    max_memory_volumes: 10       # Maximum memory volumes
    max_memory_usage: "10GB"     # Maximum total memory usage

  # Cleanup settings
  auto_cleanup: false            # Auto-remove unused volumes
  cleanup_interval: "24h"        # Cleanup check interval
```

### Runtime Configuration

```yaml
runtime:
  enabled: true                   # Enable runtime system
  base_path: "/opt/joblet/runtimes" # Runtime storage path

  # Runtime isolation settings
  isolation:
    cleanup_enabled: true         # Enable automatic runtime cleanup
    create_isolated_copies: true  # Create isolated runtime structures
    verify_isolation: true        # Verify runtime isolation after cleanup

  # Runtime installation
  installation:
    timeout: "3600s"             # Maximum runtime installation time (1 hour)
    cleanup_temp_files: true     # Clean up temporary installation files

  # Runtime security
  security:
    scan_for_host_dependencies: true  # Scan for insecure host dependencies
    enforce_isolated_paths: true      # Enforce that runtimes use isolated paths
    backup_original_configs: true     # Backup original runtime configs

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

  # Authentication settings
  require_client_cert: true       # Require client certificates
  verify_client_cert: true        # Verify client certificates

  # Authorization
  enable_rbac: true              # Enable role-based access control
  default_role: "viewer"         # Default role for unknown OUs

  # Audit logging
  audit:
    enabled: true
    log_file: "/var/log/joblet/audit.log"
    log_successful_auth: true
    log_failed_auth: true
    log_job_operations: true
```

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
# IPC configuration for persist integration
# This setting controls BOTH persistence AND buffering:
#   enabled: true  - Logs/metrics buffered in memory + forwarded to persist (gap prevention enabled)
#   enabled: false - NO buffering (live streaming only, no persistence, no historical data)
ipc:
  enabled: true                                   # Enable IPC to persist service + in-memory buffering
  socket: "/opt/joblet/run/persist-ipc.sock"      # Unix socket for log/metric writes
  buffer_size: 10000                              # Message buffer size
  reconnect_delay: "5s"                           # Reconnection retry delay
  max_reconnects: 0                               # Max reconnection attempts (0 = infinite)

# Persistence service configuration (only used when ipc.enabled: true)
persist:
  server:
    grpc_socket: "/opt/joblet/run/persist-grpc.sock"  # Unix socket for queries
    max_connections: 500

  ipc:
    socket: "/opt/joblet/run/persist-ipc.sock"  # Must match main ipc.socket
    max_connections: 10
    max_message_size: 134217728  # 128MB

  storage:
    type: "local"  # Options: "local", "cloudwatch"

    local:
      logs:
        directory: "/opt/joblet/logs"
        format: "jsonl"
      metrics:
        directory: "/opt/joblet/metrics"
        format: "jsonl.gz"
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
  backend: "memory"  # Options: "memory", "dynamodb" (EC2 only)
  socket: "/opt/joblet/run/state-ipc.sock"      # Unix socket for state operations
  buffer_size: 10000                             # Message buffer size
  reconnect_delay: "5s"                          # Reconnection retry delay
  pool_size: 20                                  # Connection pool size for high concurrency (default: 20)

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
- 10-second timeout per operation (configurable)
- Connection pool handles 1000+ concurrent jobs efficiently
- Automatic reconnection if state service restarts
- High-throughput regardless of job count (200x faster than previous implementation)
- Pool size configurable via `pool_size` (default: 20 connections)

**Pool Size Recommendations:**

- < 100 jobs: Default (20) is sufficient
- 100-1000 jobs: Default (20) handles well
- 1000-2500 jobs: Consider 30-50 for headroom
- > 2500 jobs: 50-100+ depending on workload

See [STATE_PERSISTENCE.md](./STATE_PERSISTENCE.md) for detailed state persistence documentation including performance
characteristics, DynamoDB setup, monitoring, and troubleshooting.

### Logging Configuration

```yaml
logging:
  level: "info"                  # Log level: debug, info, warn, error
  format: "json"                 # Log format: json or text

  # Output configuration
  outputs:
    - type: "file"
      path: "/var/log/joblet/joblet.log"
      rotate: true
      max_size: "100MB"
      max_backups: 10
      max_age: 30

    - type: "stdout"
      format: "text"             # Override format for stdout

  # Component-specific logging
  components:
    grpc: "warn"
    cgroup: "info"
    network: "info"
    volume: "info"
    auth: "info"
```

### Advanced Settings

```yaml
# Cgroup configuration
cgroup:
  baseDir: "/sys/fs/cgroup/joblet.slice" # Cgroup hierarchy path
  version: "v2"                          # Cgroup version (v1 or v2)

  # Controllers to enable
  enableControllers:
    - memory
    - cpu
    - io
    - pids
    - cpuset

  # Resource accounting
  accounting:
    enabled: true
    interval: "10s"              # Metrics collection interval

# Filesystem isolation
filesystem:
  baseDir: "/opt/joblet/jobs"    # Base directory for job workspaces
  tmpDir: "/opt/joblet/tmp"      # Temporary directory

  # Workspace settings
  workspace:
    default_quota: "1MB"         # Default workspace size
    cleanup_on_exit: true        # Clean workspace after job
    preserve_on_failure: true    # Keep workspace on failure

  # Security
  enable_chroot: true            # Use chroot isolation
  readonly_rootfs: false         # Make root filesystem read-only

# Process management
process:
  default_user: "nobody"         # Default user for jobs
  default_group: "nogroup"       # Default group for jobs
  allow_setuid: false           # Allow setuid in jobs

  # Namespace configuration
  namespaces:
    - pid                       # Process isolation
    - mount                     # Filesystem isolation
    - network                   # Network isolation
    - ipc                       # IPC isolation
    - uts                       # Hostname isolation
    - cgroup                    # Cgroup isolation

# Monitoring configuration
monitoring:
  enabled: true
  bind_address: "127.0.0.1:9090" # Prometheus metrics endpoint

  collection:
    system_interval: "15s"       # System metrics interval
    process_interval: "30s"      # Process metrics interval

  # Metrics to collect
  metrics:
    - cpu
    - memory
    - disk
    - network
    - processes
```

## Client Configuration

The RNX client configuration file is typically located at `~/.rnx/rnx-config.yml`.

### Single Node Setup

```yaml
version: "3.0"

# Default node configuration
default_node: "default"

nodes:
  default:
    address: "joblet-server:50051"
    nodeId: "8f94c5b2-1234-5678-9abc-def012345678"  # Optional: Joblet node identifier

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

  viewer:
    address: "prod.joblet.company.com:50051"
    nodeId: "a1b2c3d4-5678-9abc-def0-123456789012"  # Same as production (viewer access)
    cert: |
      -----BEGIN CERTIFICATE-----
      # Viewer certificate (OU=viewer)
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

Joblet uses certificate Organization Units (OU) for role-based access:

```yaml
# Admin role certificate (full access)
# Certificate subject: /CN=admin-client/OU=admin

# Viewer role certificate (read-only)
# Certificate subject: /CN=viewer-client/OU=viewer
```

Generate role-specific certificates:

```bash
# Admin certificate
openssl req -new -key client-key.pem -out admin.csr \
  -subj "/CN=admin-client/OU=admin"

# Viewer certificate  
openssl req -new -key client-key.pem -out viewer.csr \
  -subj "/CN=viewer-client/OU=viewer"
```

## Environment Variables

### Server Environment Variables

| Variable                | Description                        | Default                                |
|-------------------------|------------------------------------|----------------------------------------|
| `JOBLET_CONFIG_PATH`    | Path to configuration file         | `/opt/joblet/config/joblet-config.yml` |
| `JOBLET_LOG_LEVEL`      | Log level override                 | from config                            |
| `JOBLET_SERVER_ADDRESS` | Server address override            | from config                            |
| `JOBLET_SERVER_PORT`    | Server port override               | from config                            |
| `JOBLET_NODE_ID`        | Node identifier override           | from config                            |
| `JOBLET_MAX_JOBS`       | Maximum concurrent jobs            | from config                            |
| `JOBLET_CI_MODE`        | Enable CI mode (relaxed isolation) | `false`                                |

### Client Environment Variables

| Variable            | Description                | Default                     |
|---------------------|----------------------------|-----------------------------|
| `RNX_CONFIG`        | Path to configuration file | searches standard locations |
| `RNX_NODE`          | Default node to use        | `default`                   |
| `RNX_OUTPUT_FORMAT` | Output format (table/json) | `table`                     |
| `RNX_NO_COLOR`      | Disable colored output     | `false`                     |
| `RNX_TIMEOUT`       | Request timeout            | `30s`                       |

## Configuration Examples

### High-Security Production Setup

```yaml
version: "3.0"

server:
  address: "0.0.0.0"
  port: 50051
  tls:
    enabled: true
    min_version: "1.3"
    cipher_suites:
      - TLS_AES_256_GCM_SHA384
      - TLS_CHACHA20_POLY1305_SHA256

joblet:
  validateCommands: true
  allowedCommands:
    - python3
    - node
  maxConcurrentJobs: 50
  jobTimeout: "1h"

security:
  require_client_cert: true
  verify_client_cert: true
  enable_rbac: true
  audit:
    enabled: true
    log_all_operations: true

filesystem:
  enable_chroot: true
  readonly_rootfs: true

process:
  default_user: "nobody"
  allow_setuid: false
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
  validateCommands: false # Allow any command

logging:
  level: "debug"
  format: "text"

network:
  networks:
    bridge:
      cidr: "172.30.0.0/16"
      enable_nat: true

volume:
  max_volumes: 1000
  max_size: "1TB"
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
  preserveFailedJobs: false

filesystem:
  workspace:
    cleanup_on_exit: true
    preserve_on_failure: false

cgroup:
  accounting:
    enabled: false      # Reduce overhead

logging:
  level: "warn"        # Reduce log volume
  outputs:
    - type: "stdout"
      format: "json"   # Structured logs for CI
```

## Best Practices

1. **Security First**: Always use TLS and client certificates in production
2. **Resource Limits**: Set appropriate defaults to prevent resource exhaustion
3. **Monitoring**: Enable metrics collection for production environments
4. **Logging**: Use JSON format for easier log parsing
5. **Cleanup**: Configure automatic cleanup to prevent disk space issues
6. **Validation**: Enable command validation in production
7. **Audit**: Enable audit logging for compliance
8. **Backup**: Keep configuration file backups

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