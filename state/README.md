# state

**Persistent Job State Management for Joblet**

`state` is a subprocess service that provides persistent storage for job metadata across system restarts. It separates job state persistence from observability data (logs/metrics) handled by `persist`.

## 📋 Overview

### Problem
By default, Joblet stores job states in memory. When the service or host restarts, all job history is lost.

### Solution
`state` provides pluggable persistent storage backends:
- **Memory** (default, no persistence)
- **DynamoDB** (AWS cloud persistence)
- **Redis** (future: distributed caching)

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        joblet (main)                        │
│  ┌──────────────────────────────────────────────────────┐   │
│  │  job_store_adapter (in-memory cache + IPC client)    │   │
│  └────────────────────────┬─────────────────────────────┘   │
└───────────────────────────┼─────────────────────────────────┘
                            │ Unix Socket IPC
                            │ /opt/joblet/run/state-ipc.sock
┌───────────────────────────┼─────────────────────────────────┐
│  ┌────────────────────────▼─────────────────────────────┐   │
│  │       state IPC Server                               │   │
│  └──────────────────────┬───────────────────────────────┘   │
│                         │                                   │
│  ┌──────────────────────▼───────────────────────────────┐   │
│  │  Storage Backend Interface                           │   │
│  │  ┌──────────┐  ┌───────────┐  ┌───────────┐          │   │
│  │  │  Memory  │  │ DynamoDB  │  │   Redis   │          │   │
│  │  │(fallback)│  │(AWS prod) │  │ (future)  │          │   │
│  │  └──────────┘  └───────────┘  └───────────┘          │   │
│  └──────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                            state subprocess
```

## 🚀 Quick Start

### 1. Enable State Persistence

Edit `/opt/joblet/config/joblet-config.yml`:

```yaml
state:
  backend: "dynamodb"  # or "memory" for testing
  socket: "/opt/joblet/run/state-ipc.sock"

  storage:
    dynamodb:
      region: ""  # Auto-detect from EC2 metadata
      table_name: "joblet-jobs"
      ttl_enabled: true
      ttl_days: 30
      read_capacity: 5   # 0 = on-demand
      write_capacity: 5  # 0 = on-demand
```

### 2. Create DynamoDB Table

Use AWS CLI:

```bash
aws dynamodb create-table \
  --table-name joblet-jobs \
  --attribute-definitions \
    AttributeName=jobId,AttributeType=S \
  --key-schema \
    AttributeName=jobId,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST \
  --region us-east-1
```

**Enable TTL (optional):**
```bash
aws dynamodb update-time-to-live \
  --table-name joblet-jobs \
  --time-to-live-specification \
    "Enabled=true, AttributeName=expiresAt"
```

### 3. Build and Deploy

```bash
# Build state binary
cd state
go build -o ../bin/state cmd/state/main.go

# Deploy to production
scp bin/state jay@server:/opt/joblet/bin/
```

### 4. Start Joblet

```bash
# state subprocess starts automatically
sudo systemctl start joblet

# Verify both processes are running
ps aux | grep joblet
# joblet (main process)
# state (subprocess)
```

### 5. Test Persistence

```bash
# Create a job (see docs/RNX_CLI_REFERENCE.md for full command reference)
rnx job run echo "test persistence"

# Restart joblet service
sudo systemctl restart joblet

# Job is still there!
rnx job list

# Check job status
rnx job status <job-id>
```

**Note**: For complete RNX CLI command reference, see [docs/RNX_CLI_REFERENCE.md](../../docs/RNX_CLI_REFERENCE.md)

## 📊 DynamoDB Schema

### Primary Key
- **Partition Key**: `jobId` (String) - Unique job identifier
- **Sort Key**: None (single-item per job)

### Attributes
```
jobId: String         # UUID (e.g., "abc123...")
jobStatus: String     # PENDING, RUNNING, COMPLETED, FAILED
command: String       # Job command
createdAt: String     # RFC3339 timestamp
updatedAt: String     # RFC3339 timestamp
completedAt: String   # RFC3339 timestamp (optional)
exitCode: Number      # Exit code (optional)
nodeId: String        # Node identifier
expiresAt: Number     # Unix timestamp for TTL (auto-cleanup)
```

### Global Secondary Index (Future)
```
status-createdAt-index
  - Partition: status
  - Sort: createdAt
  - Purpose: Efficient queries like "list all RUNNING jobs"
```

## ⚙️ Configuration Reference

### Performance

All state operations use async fire-and-forget pattern for maximum performance:
- Create/Update/Delete operations run in goroutines with 5s timeout
- Non-blocking - joblet continues immediately
- High-throughput regardless of number of jobs
- Automatic reconnection if state service restarts

### Backend Options

#### Memory (Development/Testing)
```yaml
state:
  backend: "memory"
```
- RAM-only persistence (lost on restart)
- Fast, suitable for testing

#### DynamoDB (Production)
```yaml
state:
  backend: "dynamodb"
  storage:
    dynamodb:
      region: "us-east-1"  # or "" for auto-detect
      table_name: "joblet-jobs"
      ttl_enabled: true
      ttl_days: 30
```

#### Redis (Future)
```yaml
state:
  backend: "redis"
  storage:
    redis:
      endpoint: "localhost:6379"
      password: ""
      db: 0
      ttl_days: 30
```

## 🔧 IPC Protocol

### Message Format

**Request:**
```json
{
  "op": "create",
  "jobId": "abc123",
  "job": {
    "uuid": "abc123",
    "status": "RUNNING",
    "command": "echo test",
    "createdAt": "2025-10-26T00:00:00Z"
  },
  "requestId": "req-001",
  "timestamp": 1698432000
}
```

**Response:**
```json
{
  "requestId": "req-001",
  "success": true,
  "job": { ... },
  "error": ""
}
```

### Operations
- `create` - Create new job state
- `update` - Update existing job state
- `delete` - Delete job state
- `get` - Retrieve single job
- `list` - List jobs with filters
- `sync` - Bulk reconciliation

## 🔍 Monitoring

### Health Check
```bash
# Check if state is running
systemctl status joblet | grep state

# Check IPC socket
ls -la /opt/joblet/run/state-ipc.sock
```

### Logs
```bash
# View state logs (prefixed with [STATE])
journalctl -u joblet -f | grep STATE
```

### DynamoDB Metrics
Monitor in AWS CloudWatch:
- `ConsumedReadCapacityUnits`
- `ConsumedWriteCapacityUnits`
- `ThrottledRequests`
- `UserErrors`

## 🚨 Troubleshooting

### State subprocess not starting
```bash
# Check if binary exists
ls -la /opt/joblet/bin/state

# Check configuration
grep -A 10 "^state:" /opt/joblet/config/joblet-config.yml

# Check logs
journalctl -u joblet | grep -i "state"
```

### DynamoDB connection issues
```bash
# Verify IAM permissions
aws dynamodb describe-table --table-name joblet-jobs

# Check EC2 instance profile
curl http://169.254.169.254/latest/meta-data/iam/security-credentials/

# Test region detection
aws ec2 describe-availability-zones --query 'AvailabilityZones[0].RegionName'
```

### IPC socket errors
```bash
# Check socket permissions
ls -la /opt/joblet/run/state-ipc.sock

# Remove stale socket
sudo rm /opt/joblet/run/state-ipc.sock
sudo systemctl restart joblet
```

## 📖 Development

### Build
```bash
cd state
go mod download
go build -o ../bin/state cmd/state/main.go
```

### Test Locally
```bash
# Use memory backend for testing
echo "state:
  backend: memory
" > test-config.yml

JOBLET_CONFIG_PATH=test-config.yml ./bin/state
```

### Run Unit Tests
```bash
go test ./internal/storage/...
go test ./internal/ipc/...
```

## 🔐 Security

### IAM Policy (DynamoDB)
```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
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

### EC2 Instance Profile
Attach the IAM role with the above policy to your EC2 instance.

## 🎯 Future Enhancements

- [ ] Redis backend for distributed caching
- [ ] PostgreSQL backend for self-hosted
- [ ] Global Secondary Indexes for efficient queries
- [ ] Compression for large job metadata
- [ ] Encryption at rest
- [ ] Cross-region replication (DynamoDB Global Tables)

## 📄 License

Same as Joblet main project

## 🤝 Contributing

See main Joblet repository for contribution guidelines
