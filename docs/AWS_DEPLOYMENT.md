# Joblet AWS EC2 Deployment Guide

Deploy Joblet on AWS EC2 in **2 simple steps** (~10 minutes total).

## Quick Start

### Step 1: AWS Pre-Setup (CloudShell - 1 minute)

Open **AWS Console → CloudShell** (top-right toolbar icon) and run ONE of the following based on your storage preference:

<details>
<summary><b>CloudWatch (Recommended)</b></summary>

```bash
curl -fsSL https://raw.githubusercontent.com/ehsaniara/joblet/main/scripts/aws/pre-setup.sh | bash -s -- --storage=cloudwatch
```
</details>

<details>
<summary><b>S3 Storage</b></summary>

```bash
curl -fsSL https://raw.githubusercontent.com/ehsaniara/joblet/main/scripts/aws/pre-setup.sh | bash -s -- --storage=s3
```

The script will prompt you to enter your S3 bucket name. Create the bucket first if it doesn't exist:
```bash
aws s3 mb s3://your-bucket-name
```
</details>

<details>
<summary><b>Local Storage</b></summary>

```bash
curl -fsSL https://raw.githubusercontent.com/ehsaniara/joblet/main/scripts/aws/pre-setup.sh | bash -s -- --storage=local
```
</details>

This interactive script creates:

- `JobletEC2Role` IAM role with permissions based on your storage selection:
  - CloudWatch: CloudWatch Logs + DynamoDB + Secrets Manager
  - S3: S3 bucket access + DynamoDB + Secrets Manager
  - Local: DynamoDB + Secrets Manager only
- `joblet-jobs` DynamoDB table for job state persistence
- **DynamoDB VPC Endpoint** (required) for secure access to DynamoDB
- **S3 VPC Endpoint** (when using S3 storage) with bucket-specific policy
- **CA and client certificates** in AWS Secrets Manager (for horizontal scaling)

The script will:

1. Ask you to select a VPC for the EC2 instance
2. Check if a DynamoDB VPC Endpoint exists in that VPC
3. Let you select an existing endpoint or create a new one
4. If using S3 storage: Configure S3 VPC Endpoint with bucket-specific policy
5. Generate and store shared CA/client certificates in Secrets Manager

**Note the VPC ID** shown at the end - you'll need it in Step 2.

### Step 2: Launch EC2 Instance (Console - 5 minutes)

1. Go to **EC2 Console → Launch Instance**

2. **Configure the instance**:
    - **Name**: `joblet-server`
    - **AMI**: Ubuntu Server 24.04 LTS
    - **Instance type**: `t3.medium` (or larger)
    - **Key pair**: Select or create your SSH key pair
    - **Network**: Select the VPC from Step 1 ⬅️ Important!
    - **Security Group**: Create new or select existing with:
        - **SSH (22)** from your IP
        - **HTTPS (443)** from your IP (for gRPC)
    - **IAM Instance Profile**: Select `JobletEC2Role` ⬅️ Created in Step 1
    - **Storage**: 30 GB gp3 (default)

3. **Expand "Advanced Details" → Scroll to "User data"** and paste ONE of the following scripts based on your storage preference:

   #### Choose Your Storage Backend:

   <details>
   <summary><b>CloudWatch Logs (Recommended)</b> - Real-time monitoring, AWS-native integration</summary>

   ```bash
   #!/bin/bash
   curl -fsSL https://raw.githubusercontent.com/ehsaniara/joblet/main/scripts/ec2-user-data.sh -o /tmp/joblet-install.sh
   chmod +x /tmp/joblet-install.sh
   /tmp/joblet-install.sh --storage=cloudwatch 2>&1 | tee /var/log/joblet-install.log
   ```

   **Features**: Real-time log streaming, CloudWatch Insights queries, 7-day default retention
   </details>

   <details>
   <summary><b>S3 Storage</b> - Cost-effective long-term archival</summary>

   ```bash
   #!/bin/bash
   curl -fsSL https://raw.githubusercontent.com/ehsaniara/joblet/main/scripts/ec2-user-data.sh -o /tmp/joblet-install.sh
   chmod +x /tmp/joblet-install.sh
   /tmp/joblet-install.sh --storage=s3 --s3-bucket=your-bucket-name 2>&1 | tee /var/log/joblet-install.log
   ```

   **Prerequisites**: Create an S3 bucket first. Run `pre-setup.sh --storage=s3 --s3-bucket=your-bucket-name` in Step 1 to configure IAM permissions.

   **Features**: Low cost, unlimited storage, lifecycle policies for archival

   **Additional options**: `--s3-prefix=PREFIX` (default: jobs/), `--s3-class=CLASS` (default: STANDARD)
   </details>

   <details>
   <summary><b>Local Storage</b> - Development/testing, no AWS dependencies</summary>

   ```bash
   #!/bin/bash
   curl -fsSL https://raw.githubusercontent.com/ehsaniara/joblet/main/scripts/ec2-user-data.sh -o /tmp/joblet-install.sh
   chmod +x /tmp/joblet-install.sh
   /tmp/joblet-install.sh --storage=local 2>&1 | tee /var/log/joblet-install.log
   ```

   **Features**: No AWS permissions needed, logs stored at `/opt/joblet/logs/`

   **Note**: Logs are lost if instance is terminated. Best for development/testing only.
   </details>

   > See [Storage Backend Options](#storage-backend-options) for detailed configuration options.

4. Click **Launch instance**

### What Gets Deployed Automatically

When the instance boots, the user data script automatically:

- **Detects EC2 environment** (region, instance ID, metadata)
- **Installs Joblet** via Debian/RPM package
- **Configures storage backend** based on your selection:
  - CloudWatch: Creates `/joblet` log group for real-time aggregation
  - S3: Configures bucket and prefix for batch uploads
  - Local: Uses `/opt/joblet/logs/` on disk
- **Fetches CA/client certificates** from Secrets Manager (shared across instances)
- **Generates server TLS certificate** (instance-specific, embedded in config)
- **Starts Joblet server** on port 443 (systemd service)

**Note**: DynamoDB table is created in Step 1 (pre-setup), not on EC2 instance.

**Total time: ~5 minutes** after launch

---

## Post-Deployment

### 1. Download Client Configuration

After the instance is running (wait ~5 minutes for installation):

```bash
# Get the public IP from EC2 Console
PUBLIC_IP="x.x.x.x"  # Replace with actual IP

# Download client config
mkdir -p ~/.rnx
scp -i ~/.ssh/your-key.pem ubuntu@${PUBLIC_IP}:/opt/joblet/config/rnx-config.yml ~/.rnx/
```

### 2. Test Connection

```bash
# List jobs (should return empty list)
rnx job list

# Run first job
rnx job run echo "Hello from Joblet on AWS!"

# View job logs (fetched from configured storage backend)
rnx job log <job-id>

# Check job status
rnx job status <job-id>
```

### 3. Verify AWS Integration

```bash
# SSH to instance
ssh -i ~/.ssh/your-key.pem ubuntu@${PUBLIC_IP}

# Check Joblet service status
sudo systemctl status joblet

# View DynamoDB table (job state)
aws dynamodb describe-table --table-name joblet-jobs
```

**Verify storage backend (based on your selection):**

```bash
# If using CloudWatch:
aws logs describe-log-streams --log-group-name /joblet

# If using S3:
aws s3 ls s3://your-bucket-name/joblet/

# If using Local:
ssh ubuntu@${PUBLIC_IP} "ls -la /opt/joblet/logs/"
```

---

## What You Get

### IAM Role (`JobletEC2Role`)

- **CloudWatch Logs** permissions (CreateLogGroup, CreateLogStream, PutLogEvents)
- **DynamoDB** permissions (CreateTable, PutItem, GetItem, UpdateItem, DeleteItem, Scan, Query)
- **Secrets Manager** permissions (GetSecretValue, DescribeSecret, CreateSecret, UpdateSecret)
- **EC2 Metadata** access (region detection)

### EC2 Instance

- **Ubuntu Server 24.04 LTS** (recommended)
- **Joblet server** running on port 443 (gRPC)
- **Auto-starts on boot** (systemd service)
- **30GB gp3 EBS** volume

### CloudWatch Logs (`/joblet` log group) - Default Backend

- **Real-time job logs** aggregated from all jobs
- **Searchable and filterable** via AWS Console or CLI
- **7-day retention** (default, configurable)
- **Log format**: `/joblet/{nodeId}/jobs/{job_uuid}`

### S3 Storage (Alternative Backend)

- **Cost-effective** long-term storage for high-volume logs
- **Configurable storage classes** (STANDARD, GLACIER, DEEP_ARCHIVE)
- **Lifecycle policies** for automatic archival and deletion
- **Object format**: `{prefix}/{nodeId}/{jobId}/{type}/{timestamp}.jsonl.gz`
- **Enable with**: `PERSIST_BACKEND=s3 S3_BUCKET=your-bucket`

### DynamoDB (`joblet-jobs` table)

- **Persistent job state** (survives restarts)
- **Auto-cleanup** with TTL (30 days for completed jobs)
- **Pay-per-request** billing (no upfront costs)
- **Automatic creation** during installation

### VPC Endpoint (DynamoDB Gateway) - Required

- **Required for Joblet** to access DynamoDB from EC2
- **Secure DynamoDB access** without internet exposure
- **Traffic stays within AWS** network (never goes to public internet)
- **No NAT Gateway required** for DynamoDB access
- **Created automatically** by the pre-setup script if not exists

### VPC Endpoint (S3 Gateway) - For S3 Storage

When using S3 storage backend, the pre-setup script also configures:

- **S3 VPC Endpoint** for EC2 instances without internet access
- **Bucket-specific policy** allowing only your Joblet S3 bucket
- **Actions allowed**: PutObject, GetObject, DeleteObject, ListBucket
- **Created/updated automatically** by `pre-setup.sh --storage=s3`

**Note**: If the S3 VPC Endpoint policy doesn't allow access to your bucket, you'll see `AccessDenied` errors in the persist service logs. The pre-setup script updates the endpoint policy automatically to allow access to the specified bucket.

### Secrets Manager (TLS Certificates)

- **Shared CA certificate** (`joblet/ca-cert`, `joblet/ca-key`) - Same across all instances
- **Shared client certificate** (`joblet/client-cert`, `joblet/client-key`) - Same for all clients
- **Instance-specific server certificate** - Generated fresh on each EC2 instance
- **Enables horizontal scaling** - Multiple Joblet instances share the same CA/client trust
- **Created automatically** by the pre-setup script

### Graceful Fallback (Resilience)

Joblet is designed to **always start**, even if AWS services are unavailable:

| Service     | Primary         | Fallback   | Behavior                             |
|-------------|-----------------|------------|--------------------------------------|
| **State**   | DynamoDB        | In-memory  | Jobs work, but state lost on restart |
| **Persist** | CloudWatch / S3 | Local disk | Logs stored at `/opt/joblet/logs`    |

When running in fallback mode, Joblet logs **prominent warnings** so you know AWS integration is not working. This
ensures Joblet remains functional for development/testing even without proper AWS setup.

---

## Advanced

### Storage Backend Options

Joblet supports three storage backends for logs and metrics. Select your preferred backend in [Step 2](#step-2-launch-ec2-instance-console---5-minutes) when configuring User Data.

| Backend | Best For | Pros | Cons |
|---------|----------|------|------|
| **CloudWatch** (default) | Real-time monitoring, AWS integration | Native AWS integration, real-time queries, CloudWatch Insights | Higher cost for high-volume logs |
| **S3** | Long-term archival, cost optimization | Very low cost, unlimited storage, lifecycle policies | Not real-time, requires S3 bucket setup |
| **Local** | Development, testing, VMs | No AWS dependencies, zero cost | Lost on instance termination, no aggregation |

#### Command-Line Options (Preferred)

The install script supports command-line options for cleaner User Data:

```bash
/tmp/joblet-install.sh [OPTIONS]

Options:
  --storage=TYPE      cloudwatch (default), s3, or local
  --s3-bucket=NAME    S3 bucket name (required for s3)
  --s3-prefix=PREFIX  S3 key prefix (default: jobs/)
  --s3-class=CLASS    STANDARD, STANDARD_IA, GLACIER, etc.
  --version=VERSION   Joblet version (default: latest)
  --port=PORT         Server port (default: 443)
  --help              Show all options
```

#### Option 1: CloudWatch Logs (Default)

No additional configuration needed. CloudWatch log group `/joblet` is created automatically.

```bash
/tmp/joblet-install.sh --storage=cloudwatch
```

**Environment variables (alternative):**

| Variable | Default | Description |
|----------|---------|-------------|
| `PERSIST_BACKEND` | `cloudwatch` | Explicitly set backend |
| `CLOUDWATCH_LOG_GROUP` | `/joblet` | Custom log group name |
| `CLOUDWATCH_RETENTION_DAYS` | `7` | Log retention period |

#### Option 2: S3 Storage

```bash
/tmp/joblet-install.sh --storage=s3 --s3-bucket=my-bucket --s3-prefix=joblet --s3-class=STANDARD
```

**Environment variables (alternative):**

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `PERSIST_BACKEND` | Yes | - | Must be `s3` |
| `S3_BUCKET` | Yes | - | Your S3 bucket name |
| `S3_PREFIX` | No | `joblet` | Key prefix for all objects |
| `S3_STORAGE_CLASS` | No | `STANDARD` | Storage class (see below) |

**S3 Storage Classes:**

- `STANDARD` - Frequently accessed (default)
- `STANDARD_IA` - Infrequent access, lower cost
- `ONEZONE_IA` - Single AZ, cheapest for non-critical data
- `GLACIER` - Archive, minutes to hours retrieval
- `DEEP_ARCHIVE` - Long-term archive, 12+ hours retrieval

**S3 IAM Permissions:**

When you run `pre-setup.sh --storage=s3 --s3-bucket=YOUR_BUCKET`, the IAM policy is automatically configured with these permissions scoped to your bucket:

```json
{
    "Effect": "Allow",
    "Action": [
        "s3:PutObject",
        "s3:GetObject",
        "s3:ListBucket",
        "s3:DeleteObject"
    ],
    "Resource": [
        "arn:aws:s3:::YOUR_BUCKET",
        "arn:aws:s3:::YOUR_BUCKET/*"
    ]
}
```

> **Note**: If you ran `pre-setup.sh` without `--storage=s3`, you'll need to manually add S3 permissions to `JobletEC2Role` or re-run the pre-setup with the correct storage option.

**S3 Lifecycle Rules (Recommended):**

Configure lifecycle rules on your S3 bucket for automatic cost optimization:

```bash
aws s3api put-bucket-lifecycle-configuration --bucket my-joblet-logs \
  --lifecycle-configuration '{
    "Rules": [{
      "ID": "JobletArchiveRule",
      "Status": "Enabled",
      "Filter": {"Prefix": "joblet/"},
      "Transitions": [{"Days": 30, "StorageClass": "GLACIER"}],
      "Expiration": {"Days": 365}
    }]
  }'
```

#### Option 3: Local Storage (No AWS Services)

For development, testing, or VM deployments without AWS. No S3 bucket or CloudWatch permissions needed.

```bash
/tmp/joblet-install.sh --storage=local
```

**Local Storage Behavior:**
- Logs stored in `/opt/joblet/logs/`
- Metrics stored in `/opt/joblet/metrics/`
- Job state is in-memory (lost on restart)
- No AWS permissions required

**Environment variables (alternative):**

| Variable | Default | Description |
|----------|---------|-------------|
| `PERSIST_BACKEND` | - | Must be `local` |
| `LOCAL_LOG_DIR` | `/opt/joblet/logs` | Custom log directory |
| `LOCAL_METRICS_DIR` | `/opt/joblet/metrics` | Custom metrics directory |

### Alternative: Fully Automated CLI Deployment

If you prefer command-line automation instead of the Console:

```bash
# Step 1: AWS Pre-Setup (IAM + DynamoDB)
# For CloudWatch (default):
curl -fsSL https://raw.githubusercontent.com/ehsaniara/joblet/main/scripts/aws/pre-setup.sh | bash -s -- --storage=cloudwatch

# For S3 (interactive - will prompt for bucket name):
curl -fsSL https://raw.githubusercontent.com/ehsaniara/joblet/main/scripts/aws/pre-setup.sh | bash -s -- --storage=s3

# For S3 (non-interactive - for scripts/automation):
curl -fsSL https://raw.githubusercontent.com/ehsaniara/joblet/main/scripts/aws/pre-setup.sh | bash -s -- --storage=s3 --s3-bucket=my-bucket

# Step 2: Launch instance (will prompt for security group)
export KEY_NAME="your-ssh-key-name"
curl -fsSL https://raw.githubusercontent.com/ehsaniara/joblet/main/scripts/aws/launch-instance.sh | bash
```

The `launch-instance.sh` script will:

- Find the latest Ubuntu 24.04 LTS AMI
- Prompt for security group selection (or create one)
- Launch EC2 instance with user data
- Output instance details (IP, DNS, etc.)

### Local-Only Mode (No AWS Services)

To deploy without CloudWatch or DynamoDB:

1. **Skip Step 1** (IAM role not needed)
2. In Step 2, select the **💾 Local Storage** option

> **Note**: `ENABLE_CLOUDWATCH=false` is still supported for backward compatibility, but `PERSIST_BACKEND=local` is preferred.

This results in:
- Logs stored locally at `/opt/joblet/logs/`
- Job state in-memory (lost on restart)
- No AWS permissions required
- Fully functional for job execution

### Monitor Installation Progress

SSH to the instance and watch the installation:

```bash
ssh -i ~/.ssh/your-key.pem ubuntu@${PUBLIC_IP}

# Watch installation log
tail -f /var/log/joblet-install.log

# Check if Joblet is running
sudo systemctl status joblet

# View Joblet logs
sudo journalctl -u joblet -f
```

### Security Group Configuration

**Required Rules:**

- **SSH (22)**: Your IP address (for management)
- **HTTPS (443)**: Your IP address or CIDR range (for gRPC client connections)

**Optional Rules:**

- **HTTPS (443)**: `0.0.0.0/0` (if you want to allow connections from anywhere - not recommended for production)

**Important**: Always restrict SSH and gRPC access to your IP or VPC CIDR range for security.

### SSH Tunneling (For Private Instances)

If your EC2 instance is in a private subnet without public IP:

```bash
# Create SSH tunnel through bastion (from your workstation)
# Replace <PRIVATE_IP> with the Joblet EC2 instance's private IP (e.g., 172.31.16.50)
ssh -i ~/.ssh/your-key.pem -L 50051:<PRIVATE_IP>:443 ubuntu@<BASTION_IP>

# Configure client to use localhost
# Edit ~/.rnx/rnx-config.yml:
#   address: localhost:50051
```

## Troubleshooting

### Joblet Running in Fallback Mode

If you see warnings like these in the logs (`sudo journalctl -u joblet`):

```
========================================================================
[STATE] WARNING: Running with IN-MEMORY backend (fallback mode)
[STATE] Job state will NOT persist across restarts!
[STATE] Reason: failed to connect to dynamodb
[STATE] To fix: Check VPC Endpoint, IAM role, and DynamoDB table
========================================================================
```

Or:

```
========================================================================
[PERSIST] WARNING: Running with LOCAL storage backend (fallback mode)
[PERSIST] Logs will be stored on disk at /opt/joblet/logs
[PERSIST] CloudWatch Logs integration is DISABLED
[PERSIST] Reason: failed to connect to cloudwatch
[PERSIST] To fix: Check IAM role and CloudWatch permissions
========================================================================
```

**This means Joblet is running but with reduced functionality:**

| Service     | Normal Mode | Fallback Mode | Impact                              |
|-------------|-------------|---------------|-------------------------------------|
| **State**   | DynamoDB    | In-memory     | Job state lost on restart           |
| **Persist** | CloudWatch  | Local disk    | Logs only on EC2, not in CloudWatch |

**Common causes and fixes:**

1. **DynamoDB fallback** (State service):
    - VPC Endpoint not configured → Run pre-setup script or create manually
    - VPC Endpoint in wrong VPC → Ensure EC2 is in the same VPC
    - IAM role not attached → Attach `JobletEC2Role` to EC2 instance
    - DynamoDB table doesn't exist → Table should be created by pre-setup script

2. **CloudWatch fallback** (Persist service):
    - IAM role missing CloudWatch permissions → Check `JobletEC2Role` policy
    - IAM role not attached → Attach `JobletEC2Role` to EC2 instance

**To verify and fix:**

```bash
# Check if IAM role is attached (using IMDSv2)
TOKEN=$(curl -s -X PUT "http://169.254.169.254/latest/api/token" -H "X-aws-ec2-metadata-token-ttl-seconds: 300")
curl -s -H "X-aws-ec2-metadata-token: $TOKEN" http://169.254.169.254/latest/meta-data/iam/security-credentials/

# Check VPC Endpoint exists (from CloudShell or local AWS CLI)
aws ec2 describe-vpc-endpoints --filters "Name=service-name,Values=com.amazonaws.REGION.dynamodb"

# Check DynamoDB table exists
aws dynamodb describe-table --table-name joblet-jobs --region REGION

# Restart Joblet after fixing
sudo systemctl restart joblet
```

### Installation Failed

**Check installation log:**

```bash
ssh -i ~/.ssh/your-key.pem ubuntu@${PUBLIC_IP}
cat /var/log/joblet-install.log
```

**Common issues:**

- IAM role not attached → Go to EC2 Console → Instance → Actions → Security → Modify IAM role
- AWS CLI not installed → User data script installs it automatically (wait longer)
- Region mismatch → DynamoDB table created in wrong region (check IAM permissions)

### Joblet Not Starting

```bash
# Check service status
sudo systemctl status joblet

# View logs
sudo journalctl -u joblet -n 50

# Verify configuration
cat /opt/joblet/config/joblet-config.yml
```

### DynamoDB Table Not Created

```bash
# Check if IAM role has permissions
aws iam get-role --role-name JobletEC2Role
aws iam list-attached-role-policies --role-name JobletEC2Role

# Manually create table (if needed)
aws dynamodb create-table \
  --table-name joblet-jobs \
  --attribute-definitions AttributeName=job_uuid,AttributeType=S \
  --key-schema AttributeName=job_uuid,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST \
  --region us-east-1
```

### CloudWatch Logs Not Appearing

```bash
# Check IAM permissions
aws iam get-role-policy --role-name JobletEC2Role --policy-name JobletAWSPolicy

# Verify CloudWatch agent/config
cat /opt/joblet/config/joblet-config.yml | grep -A 5 "persist:"

# Check log group exists
aws logs describe-log-groups --log-group-name-prefix /joblet
```

### S3 Storage Access Denied

If you see errors like:
```
AccessDenied: User is not authorized to perform: s3:PutObject on resource
because no VPC endpoint policy allows the s3:PutObject action
```

**Cause**: The S3 VPC Endpoint policy doesn't allow access to your bucket.

**Fix 1**: Re-run pre-setup with your bucket name:
```bash
./pre-setup.sh --storage=s3 --s3-bucket=your-bucket-name
```

**Fix 2**: Manually update the S3 VPC Endpoint policy:
```bash
# Find the S3 VPC Endpoint ID
aws ec2 describe-vpc-endpoints --filters "Name=service-name,Values=com.amazonaws.REGION.s3" \
  --query 'VpcEndpoints[*].[VpcEndpointId,VpcId]' --output table

# Update the policy
aws ec2 modify-vpc-endpoint --vpc-endpoint-id vpce-xxxx \
  --policy-document '{
    "Version":"2012-10-17",
    "Statement":[{
      "Effect":"Allow",
      "Principal":"*",
      "Action":"s3:*",
      "Resource":["arn:aws:s3:::your-bucket","arn:aws:s3:::your-bucket/*"]
    }]
  }'
```

**Fix 3**: Via AWS Console:
1. Go to **VPC → Endpoints**
2. Find the S3 Gateway endpoint for your VPC
3. Click **Actions → Manage policy**
4. Update the policy to allow access to your bucket

### Client Cannot Connect

**Check security group:**

```bash
# From EC2 Console or CLI
aws ec2 describe-security-groups --group-ids sg-xxxxx
```

**Verify port 443 is open from your IP**

**Test connectivity:**

```bash
# From your workstation
telnet ${PUBLIC_IP} 443

# Or with openssl
openssl s_client -connect ${PUBLIC_IP}:443
```

### Connection Refused

**Verify Joblet is listening:**

```bash
ssh -i ~/.ssh/your-key.pem ubuntu@${PUBLIC_IP}
sudo netstat -tulpn | grep 443
```

**Check if certificates were generated:**

```bash
cat /opt/joblet/config/joblet-config.yml | grep -A 20 "certificates:"
```

---

## Architecture

### Components

```
┌───────────────────────────────────────────────────────────────────┐
│                          AWS Account                               │
│                                                                    │
│  ┌─────────────────────────────────────────────────────────────┐  │
│  │                           VPC                                │  │
│  │                                                              │  │
│  │  ┌────────────────────────────────────────────────────┐     │  │
│  │  │ EC2 Instance (Ubuntu 24.04 LTS)                     │     │  │
│  │  │                                                    │     │  │
│  │  │  ┌──────────────────────────────────────────┐     │     │  │
│  │  │  │ Joblet Server (port 443)                 │     │     │  │
│  │  │  │  - Job execution                         │     │     │  │
│  │  │  │  - gRPC API                              │     │     │  │
│  │  │  │  - TLS certificates (embedded)           │     │     │  │
│  │  │  └──────────────────────────────────────────┘     │     │  │
│  │  │                    │                               │     │  │
│  │  └────────────────────┼───────────────────────────────┘     │  │
│  │                       │                                      │  │
│  │                       │                                      │  │
│  │           ┌───────────┴───────────┐                         │  │
│  │           ▼                       ▼                         │  │
│  │  ┌───────────────────┐  ┌───────────────────┐              │  │
│  │  │ VPC Endpoint      │  │ VPC Endpoint      │              │  │
│  │  │ (DynamoDB Gateway)│  │ (S3 Gateway)*     │              │  │
│  │  └─────────┬─────────┘  └─────────┬─────────┘              │  │
│  │            │                      │                         │  │
│  └────────────┼──────────────────────┼─────────────────────────┘  │
│               │                      │                            │
│               ▼                      ▼                            │
│  ┌──────────────────────┐  ┌─────────────────┐                   │
│  │ DynamoDB             │  │ CloudWatch Logs │                   │
│  │                      │  │  OR S3 Bucket   │                   │
│  │ Table: joblet-jobs   │  │ /joblet/...     │                   │
│  │ (job state)          │  │ (job logs)      │                   │
│  └──────────────────────┘  └─────────────────┘                   │
│                                                                   │
│  * S3 VPC Endpoint created when using --storage=s3               │
│                                                                    │
└───────────────────────────────────────────────────────────────────┘
                              ↑
                              │ gRPC (port 443)
                              │
                       ┌──────────────┐
                       │ rnx Client   │
                       │ (your laptop)│
                       └──────────────┘
```

### Data Flow

1. **Client → Joblet Server**: gRPC requests over TLS (port 443)
2. **Joblet → VPC Endpoint → DynamoDB**: Job state persistence (private, no internet)
3. **Joblet → CloudWatch OR S3**: Log/metrics storage
   - CloudWatch: Real-time log streaming (PutLogEvents)
   - S3: Batch uploads via VPC Endpoint as gzipped JSONL files (PutObject)
4. **Client ← CloudWatch/S3**: Historical log queries via `rnx job log`

---

## See Also

- [EC2 Installation Guide](installation/EC2_INSTALLATION.md) - Manual installation steps
- [Certificate Management](CERTIFICATE_MANAGEMENT_COMPARISON.md) - Certificate options
- [Main Documentation](README.md) - Complete Joblet documentation
- [AWS Scripts](../scripts/aws/README.md) - Deployment script details
