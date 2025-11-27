# Joblet AWS EC2 Deployment Guide

Deploy Joblet on AWS EC2 in **2 simple steps** (~10 minutes total).

## Quick Start

### Step 1: AWS Pre-Setup (CloudShell - 1 minute)

Open **AWS Console → CloudShell** (top-right toolbar icon) and run:

```bash
curl -fsSL https://raw.githubusercontent.com/ehsaniara/joblet/main/scripts/aws/pre-setup.sh | bash
```

This interactive script creates:

- `JobletEC2Role` IAM role with permissions for CloudWatch Logs and DynamoDB
- `joblet-jobs` DynamoDB table for job state persistence
- **DynamoDB VPC Endpoint** (required) for secure access to DynamoDB

The script will:
1. Ask you to select a VPC for the EC2 instance
2. Check if a DynamoDB VPC Endpoint exists in that VPC
3. Let you select an existing endpoint or create a new one

**Note the VPC ID** shown at the end - you'll need it in Step 2.

### Step 2: Launch EC2 Instance (Console - 5 minutes)

1. Go to **EC2 Console → Launch Instance**

2. **Configure the instance**:
    - **Name**: `joblet-server`
    - **AMI**: Ubuntu Server 22.04 LTS (latest)
    - **Instance type**: `t3.medium` (or larger)
    - **Key pair**: Select or create your SSH key pair
    - **Network**: Select the VPC from Step 1 ⬅️ Important!
    - **Security Group**: Create new or select existing with:
        - **SSH (22)** from your IP
        - **HTTPS (443)** from your IP (for gRPC)
    - **IAM Instance Profile**: Select `JobletEC2Role` ⬅️ Created in Step 1
    - **Storage**: 30 GB gp3 (default)

3. **Expand "Advanced Details" → Scroll to "User data"** and paste:

```bash
#!/bin/bash
curl -fsSL https://raw.githubusercontent.com/ehsaniara/joblet/main/scripts/ec2-user-data.sh -o /tmp/joblet-install.sh
chmod +x /tmp/joblet-install.sh
ENABLE_CLOUDWATCH=true /tmp/joblet-install.sh 2>&1 | tee /var/log/joblet-install.log
```

4. Click **Launch instance**

### What Gets Deployed Automatically

When the instance boots, the user data script automatically:

✅ **Detects EC2 environment** (region, instance ID, metadata)
✅ **Installs Joblet** via Debian/RPM package
✅ **Configures CloudWatch Logs** `/joblet` log group (for log aggregation)
✅ **Generates TLS certificates** (embedded in config)
✅ **Starts Joblet server** on port 443 (systemd service)

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

# View job logs (stored in CloudWatch)
rnx job log <job-id>

# Check job status
rnx job status <job-id>
```

### 3. Verify AWS Integration

```bash
# View CloudWatch Logs
aws logs describe-log-streams --log-group-name /joblet

# View DynamoDB table
aws dynamodb describe-table --table-name joblet-jobs

# SSH to instance
ssh -i ~/.ssh/your-key.pem ubuntu@${PUBLIC_IP}

# Check Joblet service status
sudo systemctl status joblet
```

---

## What You Get

### IAM Role (`JobletEC2Role`)

- **CloudWatch Logs** permissions (CreateLogGroup, CreateLogStream, PutLogEvents)
- **DynamoDB** permissions (CreateTable, PutItem, GetItem, UpdateItem, DeleteItem, Scan, Query)
- **EC2 Metadata** access (region detection)

### EC2 Instance

- **Ubuntu 22.04** LTS
- **Joblet server** running on port 443 (gRPC)
- **Auto-starts on boot** (systemd service)
- **30GB gp3 EBS** volume

### CloudWatch Logs (`/joblet` log group)

- **Real-time job logs** aggregated from all jobs
- **Searchable and filterable** via AWS Console or CLI
- **7-day retention** (default, configurable)
- **Log format**: `/joblet/{nodeId}/jobs/{jobId}`

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

### Graceful Fallback (Resilience)

Joblet is designed to **always start**, even if AWS services are unavailable:

| Service | Primary | Fallback | Behavior |
|---------|---------|----------|----------|
| **State** | DynamoDB | In-memory | Jobs work, but state lost on restart |
| **Persist** | CloudWatch | Local disk | Logs stored at `/opt/joblet/logs` |

When running in fallback mode, Joblet logs **prominent warnings** so you know AWS integration is not working. This ensures Joblet remains functional for development/testing even without proper AWS setup.

---

## Advanced

### Alternative: Fully Automated CLI Deployment

If you prefer command-line automation instead of the Console:

```bash
# Step 1: AWS Pre-Setup (IAM + DynamoDB)
curl -fsSL https://raw.githubusercontent.com/ehsaniara/joblet/main/scripts/aws/pre-setup.sh | bash

# Step 2: Launch instance (will prompt for security group)
export KEY_NAME="your-ssh-key-name"
curl -fsSL https://raw.githubusercontent.com/ehsaniara/joblet/main/scripts/aws/launch-instance.sh | bash
```

The `launch-instance.sh` script will:

- Find the latest Ubuntu 22.04 AMI
- Prompt for security group selection (or create one)
- Launch EC2 instance with user data
- Output instance details (IP, DNS, etc.)

### Disable CloudWatch/DynamoDB (Local-Only Mode)

If you don't want AWS CloudWatch or DynamoDB integration:

1. **Skip Step 1** (don't create IAM role)
2. In Step 2, use this user data instead:

```bash
#!/bin/bash
curl -fsSL https://raw.githubusercontent.com/ehsaniara/joblet/main/scripts/ec2-user-data.sh -o /tmp/joblet-install.sh
chmod +x /tmp/joblet-install.sh
ENABLE_CLOUDWATCH=false /tmp/joblet-install.sh 2>&1 | tee /var/log/joblet-install.log
```

This deploys Joblet with:

- ❌ No CloudWatch Logs (logs stored in `/opt/joblet/logs/` on instance)
- ❌ No DynamoDB (job state stored in memory, lost on restart)
- ✅ Still fully functional for job execution

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
# Create SSH tunnel (from your workstation)
ssh -i ~/.ssh/your-key.pem -L 50051:localhost:443 ubuntu@<BASTION_IP>

# Configure client to use localhost
# Edit ~/.rnx/rnx-config.yml:
#   server_address: localhost:50051
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

| Service | Normal Mode | Fallback Mode | Impact |
|---------|-------------|---------------|--------|
| **State** | DynamoDB | In-memory | Job state lost on restart |
| **Persist** | CloudWatch | Local disk | Logs only on EC2, not in CloudWatch |

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
# Check if IAM role is attached
curl -s http://169.254.169.254/latest/meta-data/iam/security-credentials/

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
  --attribute-definitions AttributeName=jobId,AttributeType=S \
  --key-schema AttributeName=jobId,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST \
  --region us-east-1
```

### DynamoDB VPC Endpoint Policy Error

If you see an error like:

```
AccessDeniedException: User is not authorized to perform: dynamodb:DescribeTable
because no VPC endpoint policy allows the dynamodb:DescribeTable action
```

This means the VPC Endpoint has a restrictive policy that doesn't allow DynamoDB access.

**Solution: Update VPC Endpoint Policy**

1. Go to **VPC Console → Endpoints**
2. Find the DynamoDB endpoint (type: Gateway)
3. Edit the **Policy** and select "Full Access" or add specific permissions:

```json
{
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": "*",
      "Action": [
        "dynamodb:DescribeTable",
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

**Note:** VPC Endpoints created by the pre-setup script use "Full Access" policy by default, which allows all DynamoDB operations.

### CloudWatch Logs Not Appearing

```bash
# Check IAM permissions
aws iam get-role-policy --role-name JobletEC2Role --policy-name JobletAWSPolicy

# Verify CloudWatch agent/config
cat /opt/joblet/config/joblet-config.yml | grep -A 5 "persist:"

# Check log group exists
aws logs describe-log-groups --log-group-name-prefix /joblet
```

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
│  │  │ EC2 Instance (Ubuntu 22.04)                        │     │  │
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
│  │                       ▼                                      │  │
│  │           ┌───────────────────────┐                         │  │
│  │           │ VPC Endpoint          │                         │  │
│  │           │ (DynamoDB Gateway)    │                         │  │
│  │           └───────────┬───────────┘                         │  │
│  │                       │                                      │  │
│  └───────────────────────┼──────────────────────────────────────┘  │
│                          │                                         │
│            ┌─────────────┴─────────────┐                          │
│            ▼                           ▼                          │
│  ┌─────────────────┐         ┌──────────────────────┐            │
│  │ CloudWatch Logs │         │ DynamoDB             │            │
│  │                 │         │                      │            │
│  │ /joblet/...     │         │ Table: joblet-jobs   │            │
│  │ (job logs)      │         │ (job state)          │            │
│  └─────────────────┘         └──────────────────────┘            │
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
3. **Joblet → CloudWatch**: Real-time log streaming (PutLogEvents)
4. **Client ← CloudWatch**: Historical log queries via `rnx job log` (GetLogEvents)

---

## See Also

- [EC2 Installation Guide](installation/EC2_INSTALLATION.md) - Manual installation steps
- [Certificate Management](CERTIFICATE_MANAGEMENT_COMPARISON.md) - Certificate options
- [Main Documentation](README.md) - Complete Joblet documentation
- [AWS Scripts](../scripts/aws/README.md) - Deployment script details
