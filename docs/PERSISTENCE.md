# Joblet Persistence Service

## Overview

The Joblet Persistence Service (`joblet-persist`) is a dedicated microservice that handles historical storage and querying of job logs and metrics. It runs as a subprocess of the main joblet daemon and provides durable storage with support for multiple storage backends including local filesystem and AWS CloudWatch.

## Architecture

### Components

```
┌─────────────────────────────────────────────────────┐
│                   Joblet Main Service               │
│                                                     │
│  ┌─────────────┐         ┌─────────────────────┐    │
│  │ Job Executor│────────▶│  IPC Client         │    │
│  │             │  logs   │  (Unix Socket)      │    │
│  └─────────────┘  metrics└─────────────────────┘    │
│                                    │                │
└────────────────────────────────────┼────────────────┘
                                     │
                    Unix Socket: /opt/joblet/run/persist-ipc.sock
                                     │
┌────────────────────────────────────▼─────────────────┐
│            Joblet Persistence Service                │
│                                                      │
│  ┌──────────────┐        ┌──────────────────────┐    │
│  │ IPC Server   │───────▶│  Storage Backend     │    │
│  │ (writes)     │        │  Manager             │    │
│  └──────────────┘        └──────────────────────┘    │
│                                   │                  │
│  ┌──────────────┐                 │                  │
│  │ gRPC Server  │◀────────────────┘                  │
│  │ (queries)    │                                    │
│  └──────────────┘                                    │
│         │                                            │
└─────────┼────────────────────────────────────────────┘
          │
          │ Unix Socket: /opt/joblet/run/persist-grpc.sock
          │
┌─────────▼─────────────┐
│  RNX CLI / API Client │
│  (Historical Queries) │
└───────────────────────┘
```

### Communication Channels

1. **IPC Channel (Write Path)**
   - Protocol: Custom binary protocol over Unix socket
   - Socket: `/opt/joblet/run/persist-ipc.sock`
   - Purpose: High-throughput log and metric writes from job executor
   - Async, non-blocking writes

2. **gRPC Channel (Query Path)**
   - Protocol: gRPC over Unix socket
   - Socket: `/opt/joblet/run/persist-grpc.sock`
   - Purpose: Historical queries for logs and metrics
   - Synchronous request-response

## Storage Backends

### Local Backend (Default)

File-based storage using gzipped JSON lines format.

**Features:**
- ✅ No external dependencies
- ✅ Simple deployment
- ✅ Good for single-node setups
- ✅ Works on any Linux system
- ⚠️ Limited scalability
- ⚠️ Manual backup required

**Storage Format:**
```
/opt/joblet/logs/
├── {job-id-1}/
│   ├── stdout.log.gz  # Gzipped JSONL
│   └── stderr.log.gz  # Gzipped JSONL
├── {job-id-2}/
│   ├── stdout.log.gz
│   └── stderr.log.gz
└── ...

/opt/joblet/metrics/
├── {job-id-1}/
│   └── metrics.jsonl.gz
├── {job-id-2}/
│   └── metrics.jsonl.gz
└── ...
```

**Configuration:**
```yaml
persist:
  storage:
    type: "local"
    base_dir: "/opt/joblet"
    local:
      logs:
        directory: "/opt/joblet/logs"
        format: "jsonl.gz"
      metrics:
        directory: "/opt/joblet/metrics"
        format: "jsonl.gz"
```

### CloudWatch Backend (AWS)

Cloud-native storage using AWS CloudWatch Logs for both logs and metrics.

**Features:**
- ✅ Cloud-native, fully managed
- ✅ Multi-node support with nodeID isolation
- ✅ Automatic scaling and durability
- ✅ Integrated with AWS monitoring ecosystem
- ✅ Secure IAM role authentication
- ✅ Auto-region detection on EC2
- ⚠️ AWS dependency
- ⚠️ API rate limits apply

**Log Organization:**
```
CloudWatch Log Groups:
└── {log_group_prefix}/{nodeId}/jobs/{jobId}
    ├── Log Stream: {log_stream_prefix}{jobId}-stdout
    ├── Log Stream: {log_stream_prefix}{jobId}-stderr
    └── Log Stream: {log_stream_prefix}{jobId}-metrics
```

**Multi-Node Architecture:**

CloudWatch backend supports distributed deployments with multiple joblet nodes. Each node is identified by a unique `nodeId`, ensuring logs from different nodes are properly isolated:

```
Log Groups:
├── /joblet/node-1/jobs/job-abc      # Node 1's jobs
├── /joblet/node-2/jobs/job-abc      # Node 2's jobs (same jobID, different node)
└── /joblet/node-3/jobs/job-xyz      # Node 3's jobs
```

This allows:
- Multiple nodes to run jobs with the same ID without conflicts
- Easy filtering by node in CloudWatch Logs
- Node-level log retention policies
- Clear attribution of logs to specific nodes

**Authentication:**

CloudWatch backend uses AWS default credential chain (secure, no credentials in config files):

1. **IAM Role / Instance Profile** (recommended for EC2)
   ```bash
   # EC2 instance automatically uses attached IAM role
   # No configuration needed
   ```

2. **Environment Variables**
   ```bash
   export AWS_ACCESS_KEY_ID=xxx
   export AWS_SECRET_ACCESS_KEY=yyy
   export AWS_REGION=us-east-1
   ```

3. **Shared Credentials File**
   ```bash
   ~/.aws/credentials
   ```

**Required IAM Permissions:**
```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "logs:CreateLogGroup",
        "logs:CreateLogStream",
        "logs:PutLogEvents",
        "logs:DescribeLogStreams",
        "logs:GetLogEvents",
        "logs:FilterLogEvents",
        "logs:DeleteLogGroup",
        "logs:DeleteLogStream",
        "ec2:DescribeRegions"
      ],
      "Resource": "*"
    }
  ]
}
```

**Configuration:**
```yaml
server:
  nodeId: "node-1"  # REQUIRED: Unique identifier for this node

persist:
  storage:
    type: "cloudwatch"
    cloudwatch:
      # Region auto-detection (recommended for EC2)
      region: ""  # Empty = auto-detect from EC2 metadata, falls back to us-east-1

      # Or specify explicitly
      # region: "us-east-1"

      # Log group organization (nodeId automatically inserted)
      log_group_prefix: "/joblet"              # Creates: /joblet/{nodeId}/jobs/{jobId}
      log_stream_prefix: "job-"                # Creates: job-{jobId}-stdout, job-{jobId}-stderr

      # Metrics configuration
      metric_namespace: "Joblet/Production"    # CloudWatch Metrics namespace
      metric_dimensions:                       # Additional custom dimensions
        Environment: "production"
        Cluster: "main-cluster"

      # Batch settings (CloudWatch API limits)
      log_batch_size: 100                      # Max: 10,000 events per batch
      metric_batch_size: 20                    # Max: 1,000 data points per batch
```

**Auto-Detection Features:**

1. **Region Detection:**
   - Queries EC2 metadata service: `http://169.254.169.254/latest/meta-data/placement/region`
   - Falls back to `us-east-1` if not on EC2
   - 5-second timeout

2. **Credential Detection:**
   - Automatically uses EC2 instance profile if available
   - No credentials stored in configuration files

**Monitoring:**

View logs in AWS Console:
```
CloudWatch → Log Groups → /joblet/{nodeId}/jobs/{jobId}
```

Query logs using AWS CLI:
```bash
# Get logs for a specific job on node-1
aws logs get-log-events \
  --log-group-name "/joblet/node-1/jobs/my-job-id" \
  --log-stream-name "job-my-job-id-stdout"

# Filter logs across all nodes
aws logs filter-log-events \
  --log-group-name-prefix "/joblet/" \
  --filter-pattern "ERROR"
```

### S3 Backend (Planned)

Object storage for long-term archival (v2.1+).

## Configuration

### Unified Configuration File

The persistence service shares the same configuration file as the main joblet daemon (`/opt/joblet/joblet-config.yml`). Configuration is nested under the `persist:` section:

```yaml
# /opt/joblet/joblet-config.yml

version: "3.0"

# Node identification (shared with main service)
server:
  nodeId: "production-node-1"  # Used by CloudWatch backend
  address: "0.0.0.0"
  port: 50051

# Main joblet configuration
joblet:
  # ... main service config ...

# Persistence service configuration (nested)
persist:
  server:
    grpc_socket: "/opt/joblet/run/persist-grpc.sock"  # Unix socket for queries
    max_connections: 500

  ipc:
    socket: "/opt/joblet/run/persist-ipc.sock"  # Unix socket for writes
    max_connections: 10
    max_message_size: 134217728  # 128MB

  storage:
    type: "cloudwatch"  # or "local", "s3"
    base_dir: "/opt/joblet"

    # Backend-specific configuration
    local:
      # ... local config ...

    cloudwatch:
      # ... cloudwatch config ...

# Logging (inherited by persist service)
logging:
  level: "INFO"
  format: "text"
  output: "stdout"

# Security (inherited by persist service)
security:
  serverCert: "..."
  serverKey: "..."
  caCert: "..."
```

### Configuration Inheritance

The persistence service **inherits** several settings from the parent configuration:

1. **Logging Configuration**
   - `logging.level`
   - `logging.format`
   - `logging.output`

2. **Security Settings**
   - `security.serverCert` (for TLS)
   - `security.serverKey`
   - `security.caCert`

3. **Node Identity**
   - `server.nodeId` (for CloudWatch multi-node support)

4. **Base Paths**
   - `persist.storage.base_dir` defaults to parent's base directory

## Deployment Scenarios

### Single Node (Local Backend)

Best for:
- Development environments
- Single-server deployments
- Limited job throughput

```yaml
persist:
  storage:
    type: "local"
    base_dir: "/opt/joblet"
```

### Single Node (CloudWatch Backend)

Best for:
- AWS EC2 deployments
- Centralized log management
- Integration with AWS monitoring

```yaml
server:
  nodeId: "prod-node-1"

persist:
  storage:
    type: "cloudwatch"
    cloudwatch:
      region: ""  # Auto-detect
      log_group_prefix: "/joblet"
```

### Multi-Node Cluster (CloudWatch Backend)

Best for:
- Distributed job execution
- High-availability setups
- Large-scale deployments

**Node 1 Configuration:**
```yaml
server:
  nodeId: "cluster-node-1"
  address: "10.0.1.10"

persist:
  storage:
    type: "cloudwatch"
    cloudwatch:
      region: "us-east-1"
      log_group_prefix: "/joblet-cluster"
      metric_dimensions:
        Cluster: "production"
        Node: "node-1"
```

**Node 2 Configuration:**
```yaml
server:
  nodeId: "cluster-node-2"
  address: "10.0.1.11"

persist:
  storage:
    type: "cloudwatch"
    cloudwatch:
      region: "us-east-1"
      log_group_prefix: "/joblet-cluster"
      metric_dimensions:
        Cluster: "production"
        Node: "node-2"
```

**Result in CloudWatch:**
```
/joblet-cluster/cluster-node-1/jobs/job-123
/joblet-cluster/cluster-node-2/jobs/job-456
```

## API Reference

### IPC Protocol (Internal)

Used by joblet daemon to write logs/metrics to persistence service.

```protobuf
// Log write message
message LogLine {
  string job_id = 1;
  StreamType stream = 2;  // STDOUT or STDERR
  bytes content = 3;
  int64 timestamp = 4;
  int64 sequence = 5;
}

// Metric write message
message Metric {
  string job_id = 1;
  int64 timestamp = 2;
  double cpu_percent = 3;
  int64 memory_bytes = 4;
  int64 io_read_bytes = 5;
  int64 io_write_bytes = 6;
  // ... additional fields
}
```

### gRPC Query API

Used by RNX CLI and external clients to query historical data.

```protobuf
service PersistService {
  // Query logs for a job
  rpc QueryLogs(LogQueryRequest) returns (stream LogLine);

  // Query metrics for a job
  rpc QueryMetrics(MetricQueryRequest) returns (stream Metric);

  // Delete job data
  rpc DeleteJobData(DeleteJobDataRequest) returns (DeleteJobDataResponse);
}

message LogQueryRequest {
  string job_id = 1;
  StreamType stream = 2;  // Optional filter
  int64 start_time = 3;   // Unix timestamp
  int64 end_time = 4;     // Unix timestamp
  int32 limit = 5;
  int32 offset = 6;
  string filter = 7;      // Text search filter
}

message MetricQueryRequest {
  string job_id = 1;
  int64 start_time = 2;
  int64 end_time = 3;
  string aggregation = 4;  // "avg", "min", "max", "sum"
  int32 limit = 5;
  int32 offset = 6;
}
```

## CLI Usage

### Query Logs

```bash
# Get all logs for a job
rnx job log <job-id>

# Get only stderr
rnx job log <job-id> --stream=stderr

# Filter logs
rnx job log <job-id> --filter="ERROR"

# Time range query
rnx job log <job-id> --since="2024-01-01T00:00:00Z"
```

### Query Metrics

```bash
# Get metrics for a job
rnx job metrics <job-id>

# Aggregated metrics
rnx job metrics <job-id> --aggregate=avg

# Time range
rnx job metrics <job-id> --since="1h" --until="now"
```

## Performance Considerations

### Local Backend

**Write Performance:**
- ~10,000 log lines/sec (sequential writes)
- Gzip compression (~5x reduction)
- Async writes (non-blocking)

**Read Performance:**
- ~50,000 log lines/sec (sequential reads)
- Gzip decompression overhead
- File system cache benefits

**Disk Usage:**
```
Typical job with 10,000 log lines:
- Raw JSON: ~5 MB
- Gzipped: ~1 MB
- Storage: ~1 MB per job
```

### CloudWatch Backend

**Write Performance:**
- Batch writes (100 events per request)
- Async, non-blocking
- Automatic retry with backoff
- ~5,000 log lines/sec per node

**Read Performance:**
- CloudWatch API limits apply
- FilterLogEvents: 5 requests/sec per account
- GetLogEvents: 10 requests/sec per account
- Use CloudWatch Insights for complex queries

**Costs:**
```
CloudWatch Logs Pricing (us-east-1, as of 2024):
- Ingestion: $0.50 per GB
- Storage: $0.03 per GB/month
- Query (Insights): $0.005 per GB scanned

Example: 1000 jobs/day × 10 MB logs = 10 GB/day
- Ingestion: $5/day = $150/month
- Storage: 300 GB × $0.03 = $9/month
- Total: ~$160/month
```

**Rate Limiting:**
```yaml
persist:
  storage:
    cloudwatch:
      log_batch_size: 100     # Tune based on log volume
      metric_batch_size: 20   # Tune based on metric frequency
```

## Troubleshooting

### Check Persistence Service Status

```bash
# Check if persist service is running
ps aux | grep joblet-persist

# Check IPC socket
ls -la /opt/joblet/run/persist-ipc.sock

# Check gRPC socket
ls -la /opt/joblet/run/persist-grpc.sock

# View persist logs
journalctl -u joblet -f | grep persist
```

### CloudWatch Backend Issues

**Problem: Logs not appearing in CloudWatch**

```bash
# Check AWS credentials
aws sts get-caller-identity

# Check CloudWatch permissions
aws logs describe-log-groups --log-group-name-prefix="/joblet/"

# Check region configuration
grep -A 5 "cloudwatch:" /opt/joblet/joblet-config.yml

# Enable debug logging
# In joblet-config.yml:
logging:
  level: "DEBUG"
```

**Problem: "Access Denied" errors**

Verify IAM permissions:
```bash
aws iam get-role-policy --role-name joblet-ec2-role --policy-name joblet-logs-policy
```

**Problem: Region auto-detection failed**

Explicitly set region:
```yaml
persist:
  storage:
    cloudwatch:
      region: "us-east-1"  # Explicit instead of ""
```

### Local Backend Issues

**Problem: Disk full**

```bash
# Check disk usage
du -sh /opt/joblet/logs
du -sh /opt/joblet/metrics

# Clean up old jobs
find /opt/joblet/logs -type d -mtime +7 -exec rm -rf {} \;
```

**Problem: Permission errors**

```bash
# Fix ownership
sudo chown -R joblet:joblet /opt/joblet/logs
sudo chown -R joblet:joblet /opt/joblet/metrics

# Fix permissions
sudo chmod -R 755 /opt/joblet/logs
sudo chmod -R 755 /opt/joblet/metrics
```

## Migration Between Backends

### Local to CloudWatch

```bash
# 1. Stop joblet
sudo systemctl stop joblet

# 2. Update configuration
sudo vi /opt/joblet/joblet-config.yml
# Change: type: "local" → type: "cloudwatch"

# 3. Start joblet
sudo systemctl start joblet

# 4. (Optional) Migrate old logs
# Use custom script to read local logs and push to CloudWatch
```

### CloudWatch to Local

```bash
# 1. Stop joblet
sudo systemctl stop joblet

# 2. Update configuration
sudo vi /opt/joblet/joblet-config.yml
# Change: type: "cloudwatch" → type: "local"

# 3. Create directories
sudo mkdir -p /opt/joblet/logs /opt/joblet/metrics
sudo chown joblet:joblet /opt/joblet/logs /opt/joblet/metrics

# 4. Start joblet
sudo systemctl start joblet
```

## Security Best Practices

### Local Backend

1. **File Permissions:**
   ```bash
   chmod 700 /opt/joblet/logs
   chmod 700 /opt/joblet/metrics
   ```

2. **Disk Encryption:**
   - Use LUKS or dm-crypt for log directories
   - Encrypt entire `/opt/joblet` partition

3. **Log Rotation:**
   - Implement log rotation to prevent disk exhaustion
   - Use `logrotate` or custom cleanup scripts

### CloudWatch Backend

1. **IAM Roles:**
   - Use EC2 instance profiles (never hardcode credentials)
   - Follow principle of least privilege
   - Separate roles for different environments

2. **Log Group Permissions:**
   - Restrict CloudWatch Logs access via IAM
   - Use resource-based policies for cross-account access

3. **Encryption:**
   - Enable CloudWatch Logs encryption at rest (KMS)
   ```bash
   aws logs associate-kms-key \
     --log-group-name "/joblet" \
     --kms-key-id "arn:aws:kms:region:account:key/xxx"
   ```

4. **VPC Endpoints:**
   - Use VPC endpoints for CloudWatch API calls
   - Avoid public internet for log traffic

## Monitoring

### Local Backend Metrics

- Disk usage: `/opt/joblet/logs`, `/opt/joblet/metrics`
- I/O throughput: `iostat -x 1`
- File handle count: `lsof | grep /opt/joblet | wc -l`

### CloudWatch Backend Metrics

Built-in CloudWatch metrics:
- `IncomingBytes`: Log ingestion volume
- `IncomingLogEvents`: Number of log events
- `ThrottledRequests`: Rate limiting

Custom metrics (via CloudWatch Metrics):
- Job execution count
- Average job duration
- Resource usage per job

## Future Enhancements

### Planned for v2.1

- **S3 Backend**: Long-term archival storage
- **Elasticsearch Backend**: Full-text search and analytics
- **Compression Options**: Configurable compression levels
- **Retention Policies**: Automatic data lifecycle management

### Under Consideration

- **Distributed Tracing**: OpenTelemetry integration
- **Log Streaming**: Real-time log tailing via WebSocket
- **Query DSL**: Advanced query language for log filtering
- **Multi-Region**: Cross-region log replication

## References

- [AWS CloudWatch Logs Documentation](https://docs.aws.amazon.com/cloudwatch/index.html)
- [CloudWatch Logs Pricing](https://aws.amazon.com/cloudwatch/pricing/)
- [IAM Best Practices](https://docs.aws.amazon.com/IAM/latest/UserGuide/best-practices.html)
- [Joblet Architecture](./ARCHITECTURE.md)
- [Joblet Configuration](./CONFIGURATION.md)
