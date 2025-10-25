# Joblet State Persistence

## Overview

Joblet State Persistence provides durable storage of job state information across system restarts. It runs as a dedicated subprocess (`joblet-state`) that communicates with the main joblet service via Unix socket IPC, offering multiple storage backends including in-memory and AWS DynamoDB.

## Architecture

### Process Model

> **⚠️ CRITICAL REQUIREMENT:** Joblet main process **cannot start without** a healthy state service. The startup sequence includes a 30-second health check with retries to ensure state service is ready before accepting job requests.

```
┌─────────────────────────────────────────────────────┐
│                 Joblet Main Process                  │
│                                                     │
│  ┌──────────────────────────────────────────────┐   │
│  │     STARTUP: Wait for State Service          │   │
│  │  • Retry connection: 30 attempts × 1 second   │   │
│  │  • Health check: List operation with timeout │   │
│  │  • PANIC if not available (prevents startup) │   │
│  └──────────────────────────────────────────────┘   │
│                           │                        │
│  ┌──────────────────────────────────────────────┐   │
│  │     Job Execution & State Management          │   │
│  │                                              │   │
│  │  job.Status = RUNNING                         │   │
│  │  jobStoreAdapter.UpdateJob(job) ─────────┐   │   │
│  │                                          │   │   │
│  └──────────────────────────────────────────┼───┘   │
│                                             │       │
│  ┌──────────────────────────────────────────▼───┐   │
│  │         State IPC Client                     │   │
│  │  stateClient.Update(ctx, job)                │   │
│  │  • Async goroutine (fire-and-forget)         │   │
│  │  • 5-second timeout                          │   │
│  │  • JSON encoding over Unix socket            │   │
│  └──────────────────────────────────────────────┘   │
│                           │                        │
└───────────────────────────┼────────────────────────┘
                            │
           Unix Socket: /opt/joblet/run/state-ipc.sock
                            │
┌───────────────────────────▼────────────────────────┐
│            Joblet State Subprocess                 │
│         (MUST be running before joblet starts)     │
│                                                    │
│  ┌─────────────────────────────────────────────┐   │
│  │           IPC Server                        │   │
│  │  • Receives: create/update/delete/get/list  │   │
│  │  • Returns: success/error response          │   │
│  └─────────────────┬───────────────────────────┘   │
│                    │                               │
│  ┌─────────────────▼───────────────────────────┐   │
│  │       Storage Backend Router                │   │
│  │  • Memory Backend (in-memory map)           │   │
│  │  • DynamoDB Backend (AWS SDK)               │   │
│  └─────────────────┬───────────────────────────┘   │
│                    │                               │
└────────────────────┼───────────────────────────────┘
                     │
              ┌──────┴──────┐
              │             │
              ▼             ▼
      ┌──────────┐   ┌──────────────┐
      │  Memory  │   │   DynamoDB   │
      │   Map    │   │   Table      │
      └──────────┘   └──────────────┘
      (In-Process)   (AWS Cloud)
```

### State Flow

#### 1. Job Creation
```
1. Joblet creates new job
2. jobStoreAdapter.CreateNewJob(job)
3. async: stateClient.Create(ctx, job)
4. IPC message → state subprocess
5. backend.Create(ctx, job)
6. → Memory map OR DynamoDB table
```

#### 2. Job Update
```
1. Job status changes (e.g., RUNNING → COMPLETED)
2. jobStoreAdapter.UpdateJob(job)
3. async: stateClient.Update(ctx, job)
4. IPC message → state subprocess
5. backend.Update(ctx, job)
6. → Memory map OR DynamoDB PutItem
```

#### 3. Job Deletion
```
1. Job cleanup triggered
2. stateClient.Delete(ctx, jobID)
3. IPC message → state subprocess
4. backend.Delete(ctx, jobID)
5. → Remove from memory OR DynamoDB DeleteItem
```

## Storage Backends

### Memory Backend (Default)

Simple in-memory storage using Go maps.

**Features:**
- ✅ No external dependencies
- ✅ Ultra-fast operations (<1ms)
- ✅ Simple deployment
- ⚠️ State lost on restart
- ⚠️ Single-node only

**Use Cases:**
- Development environments
- Testing
- VM/local deployments
- Non-critical workloads

**Configuration:**
```yaml
state:
  backend: "memory"
  socket: "/opt/joblet/run/state-ipc.sock"
  buffer_size: 10000
  reconnect_delay: "5s"
```

**Implementation:**
```go
type memoryBackend struct {
    jobs   map[string]*domain.Job
    mu     sync.RWMutex
}

func (m *memoryBackend) Update(ctx context.Context, job *domain.Job) error {
    m.mu.Lock()
    defer m.mu.Unlock()

    if _, exists := m.jobs[job.Uuid]; !exists {
        return ErrJobNotFound
    }

    m.jobs[job.Uuid] = job
    return nil
}
```

### DynamoDB Backend (AWS)

Cloud-native state persistence using AWS DynamoDB.

**Features:**
- ✅ State survives restarts
- ✅ Multi-node support
- ✅ Automatic scaling
- ✅ Built-in TTL for cleanup
- ✅ Strong consistency
- ✅ IAM role authentication
- ⚠️ AWS dependency
- ⚠️ Network latency (5-20ms)

**Use Cases:**
- Production AWS deployments
- Multi-node clusters
- High-availability requirements
- Compliance/audit trails

**Table Schema:**
```
Table: joblet-jobs
├── Primary Key: jobId (String, HASH)
├── Attributes:
│   ├── jobStatus (String)
│   ├── command (String)
│   ├── nodeId (String)
│   ├── startTime (String, RFC3339)
│   ├── endTime (String, RFC3339)
│   ├── scheduledTime (String, RFC3339)
│   ├── exitCode (Number)
│   ├── pid (Number)
│   ├── network (String)
│   ├── runtime (String)
│   └── expiresAt (Number, Unix timestamp) ← TTL attribute
└── TTL Configuration:
    ├── Attribute: expiresAt
    ├── Enabled: true
    └── Auto-delete after expiration
```

**Configuration:**
```yaml
state:
  backend: "dynamodb"
  socket: "/opt/joblet/run/state-ipc.sock"
  buffer_size: 10000
  reconnect_delay: "5s"

  storage:
    dynamodb:
      region: ""  # Empty = auto-detect from EC2 metadata
      table_name: "joblet-jobs"
      ttl_enabled: true
      ttl_days: 30  # Completed/failed jobs auto-deleted after 30 days
```

**Auto-Detection:**
1. **Region Detection:** Queries EC2 metadata service
2. **Credential Detection:** Uses EC2 instance profile (IAM role)
3. **Table Creation:** Automatic via installer on EC2

**Required IAM Permissions:**
```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "dynamodb:CreateTable",
        "dynamodb:DescribeTable",
        "dynamodb:DescribeTimeToLive",
        "dynamodb:UpdateTimeToLive",
        "dynamodb:PutItem",
        "dynamodb:GetItem",
        "dynamodb:UpdateItem",
        "dynamodb:DeleteItem",
        "dynamodb:Scan",
        "dynamodb:Query",
        "dynamodb:BatchWriteItem"
      ],
      "Resource": "arn:aws:dynamodb:*:*:table/joblet-jobs"
    }
  ]
}
```

**DynamoDB Operations:**

| Operation | DynamoDB API | Condition | TTL Behavior |
|-----------|--------------|-----------|--------------|
| Create | PutItem | `attribute_not_exists(jobId)` | No TTL (job running) |
| Update | PutItem | `attribute_exists(jobId)` | TTL set if COMPLETED/FAILED |
| Delete | DeleteItem | None | Immediate deletion |
| Get | GetItem | None | N/A |
| List | Scan | Optional FilterExpression | N/A |
| Sync | BatchWriteItem | None | 25 items per batch |

**TTL Logic:**
```go
// Only set TTL for completed jobs
if ttlDays > 0 && (job.Status == "COMPLETED" || job.Status == "FAILED") {
    expiresAt := time.Now().Add(time.Duration(ttlDays) * 24 * time.Hour).Unix()
    item["expiresAt"] = &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", expiresAt)}
}
```

**Cost Considerations:**
```
DynamoDB Pricing (PAY_PER_REQUEST mode):

Write Requests:
- $1.25 per million write requests
- Job creation: 1 write
- Job updates (status changes): ~3-5 writes per job
- Total: ~5 writes per job

Storage:
- $0.25 per GB-month
- Job state: ~1-2 KB per job
- 100,000 jobs: ~200 MB = $0.05/month

Example: 100 jobs/day
- Writes: 500 writes/day × 30 days = 15,000 writes/month
- Cost: 15,000 / 1,000,000 × $1.25 = $0.02/month
- Storage: < $0.05/month (TTL cleanup keeps it bounded)
- Total: < $0.10/month

TTL Savings:
- Without TTL: Unbounded growth
- With TTL (30 days): Max 3,000 jobs stored (100/day × 30 days)
- Storage cost stays constant
```

## IPC Protocol

### Message Format

**Request:**
```json
{
  "op": "create" | "update" | "delete" | "get" | "list" | "sync",
  "jobId": "abc-123-...",
  "job": {
    "uuid": "abc-123-...",
    "status": "RUNNING",
    "command": "echo test",
    "nodeId": "node-1",
    ...
  },
  "jobs": [...],  // For sync operation
  "filter": {...},  // For list operation
  "requestId": "req-123456789",
  "timestamp": 1698765432
}
```

**Response:**
```json
{
  "requestId": "req-123456789",
  "success": true | false,
  "job": {...},      // For get operation
  "jobs": [...],     // For list operation
  "error": "error message"
}
```

### Operations

#### Create
```go
// Client
msg := Message{
    Operation: "create",
    Job:       job,
    RequestID: c.nextRequestID(),
    Timestamp: time.Now().Unix(),
}
c.sendMessage(ctx, msg)

// Server
if err := backend.Create(ctx, msg.Job); err != nil {
    return &Response{Success: false, Error: err.Error()}
}
return &Response{Success: true, Job: msg.Job}
```

#### Update
```go
// Client
msg := Message{
    Operation: "update",
    Job:       job,
    RequestID: c.nextRequestID(),
    Timestamp: time.Now().Unix(),
}
c.sendMessage(ctx, msg)

// Server (DynamoDB example)
item := jobToItem(msg.Job, ttlDays)
input := &dynamodb.PutItemInput{
    TableName:           aws.String(tableName),
    Item:                item,
    ConditionExpression: aws.String("attribute_exists(jobId)"),
}
_, err := client.PutItem(ctx, input)
```

#### Get
```go
// Client
msg := Message{
    Operation: "get",
    JobID:     jobID,
    RequestID: c.nextRequestID(),
    Timestamp: time.Now().Unix(),
}
response, err := c.sendMessageWithResponse(ctx, msg)
return response.Job, err
```

#### List
```go
// Client
msg := Message{
    Operation: "list",
    Filter: &Filter{
        Status: "RUNNING",
        Limit:  100,
    },
    RequestID: c.nextRequestID(),
    Timestamp: time.Now().Unix(),
}
response, err := c.sendMessageWithResponse(ctx, msg)
return response.Jobs, err
```

#### Sync (Bulk Update)
```go
// Client - used for reconciliation after restart
msg := Message{
    Operation: "sync",
    Jobs:      allJobs,  // Batch up to 25 jobs
    RequestID: c.nextRequestID(),
    Timestamp: time.Now().Unix(),
}
c.sendMessage(ctx, msg)

// Server (DynamoDB example)
// Batches 25 items per BatchWriteItem call
for i := 0; i < len(jobs); i += 25 {
    batch := jobs[i:min(i+25, len(jobs))]
    backend.writeBatch(ctx, batch)
}
```

## Configuration

### Full Configuration Example

```yaml
version: "3.0"

server:
  nodeId: "production-node-1"
  address: "0.0.0.0"
  port: 50051

# State persistence configuration
state:
  # Backend type: "memory" or "dynamodb"
  backend: "dynamodb"

  # IPC socket for communication
  socket: "/opt/joblet/run/state-ipc.sock"

  # Buffer size for IPC messages
  buffer_size: 10000

  # Reconnect delay if state subprocess crashes
  reconnect_delay: "5s"

  # Backend-specific configuration
  storage:
    # DynamoDB configuration (ignored if backend: "memory")
    dynamodb:
      # AWS region (empty = auto-detect from EC2 metadata)
      region: ""

      # DynamoDB table name
      table_name: "joblet-jobs"

      # TTL configuration
      ttl_enabled: true
      ttl_days: 30  # Auto-delete completed jobs after 30 days
```

### Environment-Specific Configurations

#### Development (Memory Backend)
```yaml
state:
  backend: "memory"
  socket: "/opt/joblet/run/state-ipc.sock"
```

#### AWS EC2 (DynamoDB Backend)
```yaml
state:
  backend: "dynamodb"
  socket: "/opt/joblet/run/state-ipc.sock"
  storage:
    dynamodb:
      region: ""  # Auto-detect
      table_name: "joblet-jobs"
      ttl_enabled: true
      ttl_days: 30
```

## Deployment

### VM/Local Deployment (Memory Backend)

```bash
# Install joblet
sudo dpkg -i joblet_*.deb  # or rpm -i joblet-*.rpm

# Configuration is auto-generated with memory backend
cat /opt/joblet/config/joblet-config.yml | grep -A 5 "^state:"
# Output:
# state:
#   backend: "memory"
#   socket: "/opt/joblet/run/state-ipc.sock"

# Start joblet
sudo systemctl start joblet

# Verify state subprocess
ps aux | grep joblet-state
# Expected: /opt/joblet/joblet-state (running as subprocess)
```

### AWS EC2 Deployment (DynamoDB Backend)

**Option 1: Automatic Setup (Recommended)**

The installer automatically detects EC2 and configures DynamoDB:

```bash
# 1. Launch EC2 instance with IAM role (DynamoDB permissions)

# 2. Install joblet
sudo dpkg -i joblet_*.deb

# Installer automatically:
# - Detects EC2 environment
# - Creates DynamoDB table "joblet-jobs"
# - Configures state backend = "dynamodb"
# - Enables TTL on expiresAt attribute

# 3. Verify configuration
cat /opt/joblet/config/joblet-config.yml | grep -A 10 "^state:"
# Expected:
# state:
#   backend: "dynamodb"
#   storage:
#     dynamodb:
#       region: "us-east-1"  # Detected region
#       table_name: "joblet-jobs"
#       ttl_enabled: true
#       ttl_days: 30

# 4. Start joblet
sudo systemctl start joblet

# 5. Verify DynamoDB table
aws dynamodb describe-table --table-name joblet-jobs
```

**Option 2: Manual Setup**

```bash
# 1. Create DynamoDB table
aws dynamodb create-table \
  --table-name joblet-jobs \
  --attribute-definitions AttributeName=jobId,AttributeType=S \
  --key-schema AttributeName=jobId,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST \
  --region us-east-1

# 2. Enable TTL
aws dynamodb update-time-to-live \
  --table-name joblet-jobs \
  --time-to-live-specification "Enabled=true,AttributeName=expiresAt" \
  --region us-east-1

# 3. Update joblet configuration
sudo vi /opt/joblet/config/joblet-config.yml
# Set: state.backend = "dynamodb"

# 4. Restart joblet
sudo systemctl restart joblet
```

## Monitoring

### Memory Backend

```bash
# Check state subprocess status
sudo systemctl status joblet | grep -A 5 "state"

# View state subprocess logs
journalctl -u joblet -f | grep state

# Check IPC socket
ls -la /opt/joblet/run/state-ipc.sock
# Expected: srwxrwxrwx ... state-ipc.sock
```

### DynamoDB Backend

```bash
# Check table status
aws dynamodb describe-table \
  --table-name joblet-jobs \
  --query 'Table.[TableName,TableStatus,ItemCount]' \
  --output table

# Check TTL status
aws dynamodb describe-time-to-live \
  --table-name joblet-jobs \
  --query 'TimeToLiveDescription.[TimeToLiveStatus,AttributeName]' \
  --output table

# View recent jobs
aws dynamodb scan \
  --table-name joblet-jobs \
  --limit 10 \
  --output table

# Query by status
aws dynamodb scan \
  --table-name joblet-jobs \
  --filter-expression "jobStatus = :status" \
  --expression-attribute-values '{":status":{"S":"RUNNING"}}' \
  --output table

# Monitor CloudWatch metrics
# AWS Console → DynamoDB → joblet-jobs → Metrics
# - ReadCapacityUnits
# - WriteCapacityUnits
# - ConsumedReadCapacityUnits
# - ConsumedWriteCapacityUnits
```

## Troubleshooting

### Joblet Won't Start: "State service is not available"

**Symptom:**
```
FATAL: state service is not available - joblet cannot start
ensure joblet-state subprocess is running and healthy
panic: state service required but not available
```

**Cause:** Joblet main process **requires** a healthy state service before it can start. It waits up to 30 seconds with retries.

**Solution:**

```bash
# 1. Check if state subprocess is running
ps aux | grep joblet-state | grep -v grep

# 2. Check state socket exists
ls -la /opt/joblet/run/state-ipc.sock

# 3. Check state service logs
journalctl -u joblet -f | grep "state"

# 4. Verify state configuration
grep -A 20 "^state:" /opt/joblet/config/joblet-config.yml

# 5. If using DynamoDB backend, verify table exists
aws dynamodb describe-table --table-name joblet-jobs

# 6. If using DynamoDB, verify IAM permissions
aws sts get-caller-identity
```

**Common Issues:**

1. **State subprocess crashed during startup**
   ```bash
   # Check for crash logs
   journalctl -u joblet --since "5 minutes ago" | grep -i "panic\|fatal\|error"

   # Restart joblet service
   sudo systemctl restart joblet
   ```

2. **DynamoDB table missing (EC2 with DynamoDB backend)**
   ```bash
   # Check if table exists
   aws dynamodb list-tables | grep joblet-jobs

   # Create table manually if needed
   # See "DynamoDB Backend" section for commands
   ```

3. **Socket permission issues**
   ```bash
   # Fix socket directory permissions
   sudo chmod 755 /opt/joblet/run
   sudo chown joblet:joblet /opt/joblet/run
   ```

### State Subprocess Not Starting

```bash
# Check logs
journalctl -u joblet -f | grep "state subprocess"

# Common issues:
# 1. Socket permission denied
ls -la /opt/joblet/run/
sudo chmod 755 /opt/joblet/run

# 2. Backend configuration error
grep -A 20 "^state:" /opt/joblet/config/joblet-config.yml

# 3. DynamoDB table doesn't exist
aws dynamodb describe-table --table-name joblet-jobs
```

### DynamoDB Connection Issues

```bash
# Check AWS credentials
aws sts get-caller-identity

# Check IAM permissions
aws dynamodb describe-table --table-name joblet-jobs
# Should return table details

# Check region
aws configure get region
# Should match state.storage.dynamodb.region

# Enable debug logging
# Edit /opt/joblet/config/joblet-config.yml:
logging:
  level: "DEBUG"

# Restart and check logs
sudo systemctl restart joblet
journalctl -u joblet -f | grep -i dynamodb
```

### State Sync Issues

```bash
# Verify IPC socket connection
sudo netstat -anp | grep state-ipc.sock

# Check for IPC errors
journalctl -u joblet -f | grep "state client"

# Manually trigger sync (not implemented, future feature)
# For now, restart triggers sync:
sudo systemctl restart joblet
```

## Migration

### Memory → DynamoDB

```bash
# 1. Stop joblet
sudo systemctl stop joblet

# 2. Create DynamoDB table (if not exists)
aws dynamodb create-table \
  --table-name joblet-jobs \
  --attribute-definitions AttributeName=jobId,AttributeType=S \
  --key-schema AttributeName=jobId,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST

# 3. Enable TTL
aws dynamodb update-time-to-live \
  --table-name joblet-jobs \
  --time-to-live-specification "Enabled=true,AttributeName=expiresAt"

# 4. Update configuration
sudo vi /opt/joblet/config/joblet-config.yml
# Change: backend: "memory" → backend: "dynamodb"

# 5. Start joblet
sudo systemctl start joblet

# Note: In-memory state is lost during migration
# Only new jobs will be persisted to DynamoDB
```

### DynamoDB → Memory

```bash
# 1. Stop joblet
sudo systemctl stop joblet

# 2. Update configuration
sudo vi /opt/joblet/config/joblet-config.yml
# Change: backend: "dynamodb" → backend: "memory"

# 3. Start joblet
sudo systemctl start joblet

# Note: DynamoDB table remains (optional cleanup):
# aws dynamodb delete-table --table-name joblet-jobs
```

## Performance

### Memory Backend
- **Create**: < 1ms
- **Update**: < 1ms
- **Get**: < 1ms
- **List**: < 10ms (1000 jobs)

### DynamoDB Backend
- **Create**: 5-20ms (network latency)
- **Update**: 5-20ms (network latency)
- **Get**: 5-15ms (strongly consistent)
- **List**: 50-200ms (scan, 1000 jobs)
- **Batch Sync**: 25 jobs/request

**Optimization Tips:**
- Use eventual consistency for reads (not implemented)
- Batch operations where possible
- Consider GSI for status-based queries (future)

## Security

### Unix Socket Permissions
```bash
# State IPC socket should be world-writable (joblet main process needs access)
ls -la /opt/joblet/run/state-ipc.sock
# Expected: srwxrwxrwx ... state-ipc.sock

# Socket directory should be restricted
ls -la /opt/joblet/run/
# Expected: drwxr-xr-x ... joblet joblet ... run/
```

### DynamoDB Security
- ✅ IAM role-based authentication (no credentials in config)
- ✅ Encryption at rest (enabled by default)
- ✅ Encryption in transit (TLS)
- ✅ VPC endpoints (optional, for private subnets)

**Recommended IAM Policy:**
```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "JobletStateAccess",
      "Effect": "Allow",
      "Action": [
        "dynamodb:PutItem",
        "dynamodb:GetItem",
        "dynamodb:UpdateItem",
        "dynamodb:DeleteItem",
        "dynamodb:Scan",
        "dynamodb:Query"
      ],
      "Resource": "arn:aws:dynamodb:*:*:table/joblet-jobs"
    },
    {
      "Sid": "JobletTableManagement",
      "Effect": "Allow",
      "Action": [
        "dynamodb:DescribeTable",
        "dynamodb:DescribeTimeToLive",
        "dynamodb:UpdateTimeToLive"
      ],
      "Resource": "arn:aws:dynamodb:*:*:table/joblet-jobs"
    }
  ]
}
```

## Future Enhancements

### Planned (v2.1+)
- **PostgreSQL Backend**: SQL-based state storage
- **Redis Backend**: High-performance caching layer
- **State Reconciliation**: Automatic sync after network partitions
- **Query Optimization**: GSI for status-based queries
- **Backup & Restore**: Point-in-time state snapshots

### Under Consideration
- **Multi-Region**: Cross-region state replication
- **Compression**: Gzip job metadata to reduce storage
- **Encryption**: Client-side encryption for sensitive data
- **Audit Trail**: Immutable state change history

## References

- [AWS DynamoDB Documentation](https://docs.aws.amazon.com/dynamodb/)
- [DynamoDB TTL](https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/TTL.html)
- [DynamoDB Best Practices](https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/best-practices.html)
- [Joblet Architecture](./ARCHITECTURE.md)
- [Joblet Configuration](./CONFIGURATION.md)
- [AWS Deployment](./AWS_DEPLOYMENT.md)
