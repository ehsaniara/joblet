# Joblet AWS EC2 Deployment Guide

Deploy Joblet on AWS EC2 in **2 simple steps** (~10 minutes total).

## Quick Start

### Step 1: Setup IAM Role (CloudShell - 30 seconds)

Open **AWS Console → CloudShell** (top-right toolbar icon) and run:

```bash
curl -fsSL https://raw.githubusercontent.com/ehsaniara/joblet/main/scripts/aws/setup-iam.sh | bash
```

This creates the `JobletEC2Role` IAM role with permissions for CloudWatch Logs and DynamoDB.

### Step 2: Launch EC2 Instance (Console - 5 minutes)

1. Go to **EC2 Console → Launch Instance**

2. **Configure the instance**:
   - **Name**: `joblet-server`
   - **AMI**: Ubuntu Server 22.04 LTS (latest)
   - **Instance type**: `t3.medium` (or larger)
   - **Key pair**: Select or create your SSH key pair
   - **Network**: Default VPC
   - **Security Group**: Create new or select existing with:
     - **SSH (22)** from your IP
     - **HTTPS (443)** from your IP (for gRPC)
   - **IAM Instance Profile**: Select `JobletEC2Role` ⬅️ Created in Step 1
   - **Storage**: 30 GB gp3 (default)

3. **Expand "Advanced Details" → Scroll to "User data"** and paste:

```bash
#!/bin/bash
curl -fsSL https://raw.githubusercontent.com/ehsaniara/joblet/main/scripts/aws/ec2-user-data.sh -o /tmp/joblet-install.sh
chmod +x /tmp/joblet-install.sh
ENABLE_CLOUDWATCH=true /tmp/joblet-install.sh 2>&1 | tee /var/log/joblet-install.log
```

4. Click **Launch instance**

### What Gets Deployed Automatically

When the instance boots, the user data script automatically:

✅ **Detects EC2 environment** (region, instance ID, metadata)
✅ **Installs Joblet** via Debian/RPM package
✅ **Creates DynamoDB table** `joblet-jobs` (for persistent job state)
✅ **Configures CloudWatch Logs** `/joblet` log group (for log aggregation)
✅ **Generates TLS certificates** (embedded in config)
✅ **Starts Joblet server** on port 443 (systemd service)

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

---

## Advanced

### Alternative: Fully Automated CLI Deployment

If you prefer command-line automation instead of the Console:

```bash
# Step 1: Setup IAM
curl -fsSL https://raw.githubusercontent.com/ehsaniara/joblet/main/scripts/aws/setup-iam.sh | bash

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
curl -fsSL https://raw.githubusercontent.com/ehsaniara/joblet/main/scripts/aws/ec2-user-data.sh -o /tmp/joblet-install.sh
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

### Cost Estimate

Approximate monthly costs (us-east-1, 24/7 operation):

| Service | Cost |
|---------|------|
| EC2 t3.medium (on-demand) | ~$30/month |
| EBS 30GB gp3 | ~$2.40/month |
| CloudWatch Logs (10GB ingestion) | ~$5/month |
| DynamoDB (pay-per-request, light usage) | ~$0.50/month |
| **Total** | **~$38/month** |

**Cost savings tips:**
- Use Reserved Instances or Savings Plans (~40% discount)
- Stop instance when not in use (pay only for EBS storage)
- Disable CloudWatch for dev/test environments

---

## Troubleshooting

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
┌─────────────────────────────────────────────────────────────┐
│                        AWS Account                          │
│                                                             │
│  ┌──────────────────────────────────────────────────────┐  │
│  │ EC2 Instance (Ubuntu 22.04)                          │  │
│  │                                                      │  │
│  │  ┌────────────────────────────────────────────┐     │  │
│  │  │ Joblet Server (port 443)                   │     │  │
│  │  │  - Job execution                           │     │  │
│  │  │  - gRPC API                                │     │  │
│  │  │  - TLS certificates (embedded)             │     │  │
│  │  └────────────────────────────────────────────┘     │  │
│  │                    ↓         ↓                       │  │
│  └────────────────────┼─────────┼───────────────────────┘  │
│                       ↓         ↓                          │
│         ┌─────────────────┐  ┌──────────────────────┐     │
│         │ CloudWatch Logs │  │ DynamoDB             │     │
│         │                 │  │                      │     │
│         │ /joblet/...     │  │ Table: joblet-jobs   │     │
│         │ (job logs)      │  │ (job state)          │     │
│         └─────────────────┘  └──────────────────────┘     │
│                                                             │
└─────────────────────────────────────────────────────────────┘
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
2. **Joblet → DynamoDB**: Job state persistence (Create, Update, Get, List)
3. **Joblet → CloudWatch**: Real-time log streaming (PutLogEvents)
4. **Client ← CloudWatch**: Historical log queries via `rnx job log` (GetLogEvents)

---

## See Also

- [EC2 Installation Guide](installation/EC2_INSTALLATION.md) - Manual installation steps
- [Certificate Management](CERTIFICATE_MANAGEMENT_COMPARISON.md) - Certificate options
- [Main Documentation](README.md) - Complete Joblet documentation
- [AWS Scripts](../scripts/aws/README.md) - Deployment script details
