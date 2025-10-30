# Joblet AWS Deployment Scripts

Automated scripts for deploying Joblet on AWS EC2 with CloudWatch Logs and DynamoDB state persistence.

## Quick Start

### Recommended: Console + Script (Simplest)

**Step 1**: Setup IAM role (CloudShell, ~30 seconds):
```bash
curl -fsSL https://raw.githubusercontent.com/ehsaniara/joblet/main/scripts/aws/setup-iam.sh | bash
```

**Step 2**: Launch EC2 from AWS Console:
- Go to **EC2 Console → Launch Instance**
- Select **Ubuntu 22.04 LTS**, **t3.medium** (or larger)
- **Create security group** with: SSH (22) and HTTPS (443) from your IP
- Select **IAM instance profile**: `JobletEC2Role`
- Add **user data** (see below)
- **Launch instance**

**User data script**:
```bash
#!/bin/bash
curl -fsSL https://raw.githubusercontent.com/ehsaniara/joblet/main/scripts/aws/ec2-user-data.sh -o /tmp/joblet-install.sh
chmod +x /tmp/joblet-install.sh
ENABLE_CLOUDWATCH=true /tmp/joblet-install.sh 2>&1 | tee /var/log/joblet-install.log
```

### Alternative: Fully Automated CLI

If you prefer full automation from the terminal:

```bash
# Step 1: Setup IAM
curl -fsSL https://raw.githubusercontent.com/ehsaniara/joblet/main/scripts/aws/setup-iam.sh | bash

# Step 2: Launch instance (prompts for security group)
export KEY_NAME="your-ssh-key-name"
curl -fsSL https://raw.githubusercontent.com/ehsaniara/joblet/main/scripts/aws/launch-instance.sh | bash
```

**Total time: ~10 minutes**

---

## What Gets Deployed

### Automatic Setup

When the EC2 instance boots, it automatically:

✅ **Detects EC2 environment** (region, instance ID, metadata)
✅ **Installs Joblet** via Debian/RPM package
✅ **Creates DynamoDB table** `joblet-jobs` (persistent job state)
✅ **Configures CloudWatch Logs** `/joblet` log group (log aggregation)
✅ **Generates TLS certificates** (embedded in config)
✅ **Starts Joblet server** on port 443 (systemd service)

### Resources Created

**IAM Role** (`JobletEC2Role`)
- CloudWatch Logs permissions
- DynamoDB permissions
- EC2 metadata access

**Security Group**
- Create manually in EC2 Console (recommended) or use `setup-security-group.sh`
- Required: SSH (22) and HTTPS (443) from your IP

**EC2 Instance** (Ubuntu 22.04)
- Joblet server on port 443
- Automatic certificate generation
- systemd service (auto-starts)
- 30 GB gp3 EBS volume

**CloudWatch Logs** (`/joblet` log group)
- Real-time job logs
- 7-day retention (default)

**DynamoDB** (`joblet-jobs` table)
- Persistent job state
- Auto-created with TTL (30-day cleanup)
- Pay-per-request billing

---

## Script Details

### setup-iam.sh

Creates IAM role with permissions for CloudWatch Logs and DynamoDB.

**Usage:**
```bash
curl -fsSL https://raw.githubusercontent.com/ehsaniara/joblet/main/scripts/aws/setup-iam.sh | bash
```

**What it creates:**
- IAM Policy: `JobletAWSPolicy`
- IAM Role: `JobletEC2Role`
- Instance Profile: `JobletEC2Role`

**Features:**
- Idempotent (safe to run multiple times)
- Auto-detects AWS credentials
- CloudWatch Logs, DynamoDB, EC2 metadata permissions

### launch-instance.sh

Launches EC2 instance with Joblet bootstrap.

**Usage:**
```bash
export KEY_NAME="your-ssh-key"
export SG_ID="sg-xxxxx"  # Optional, will prompt if not set
./launch-instance.sh
```

**Environment variables:**
- `KEY_NAME` - SSH key pair name (required)
- `SG_ID` - Security group ID (optional, will prompt)
- `REGION` - AWS region (default: us-east-1)
- `INSTANCE_TYPE` - Instance type (default: t3.medium)
- `ENABLE_CLOUDWATCH` - Enable CloudWatch (default: true)

**What it does:**
- Finds latest Ubuntu 22.04 AMI
- Prompts for security group selection
- Launches EC2 instance with user data
- Attaches `JobletEC2Role` IAM profile
- Outputs instance details (IP, DNS, etc.)

### setup-security-group.sh (Optional)

Creates security group with SSH and gRPC access. You can skip this and create the security group manually in the EC2 Console during instance launch.

**Usage:**
```bash
# Auto-detect your IP
./setup-security-group.sh

# Or specify IP
./setup-security-group.sh 203.0.113.45/32
```

**What it creates:**
- Security Group: `joblet-server-sg`
- SSH (22): Your IP
- HTTPS (443): Your IP

**Features:**
- Idempotent (safe to run multiple times)
- Auto-detects your public IP
- Uses default VPC

---

## Using AWS CloudShell

AWS CloudShell is perfect for running the IAM setup script (no local AWS CLI needed):

1. Open **AWS Console → CloudShell** (top-right toolbar)

2. Setup IAM:
```bash
curl -fsSL https://raw.githubusercontent.com/ehsaniara/joblet/main/scripts/aws/setup-iam.sh | bash
```

3. Then **launch EC2 from the Console** (easier than CLI):
   - Go to **EC2 → Launch Instance**
   - Create **security group** with SSH (22) and HTTPS (443)
   - Select **IAM instance profile**: `JobletEC2Role`
   - Add **user data** script (see Quick Start above)

4. Wait ~5 minutes for installation

5. Download client config:
```bash
# Get the instance IP from EC2 Console
scp -i ~/.ssh/your-key.pem ubuntu@<PUBLIC_IP>:/opt/joblet/config/rnx-config.yml ~/.rnx/
```

---

## Verification

Check that everything is working:

```bash
# Test Joblet client
rnx job list

# View CloudWatch Logs
aws logs describe-log-streams --log-group-name /joblet

# View DynamoDB table
aws dynamodb describe-table --table-name joblet-jobs

# SSH to instance
ssh -i ~/.ssh/your-key.pem ubuntu@<PUBLIC_IP>

# Check service status
sudo systemctl status joblet
```

---

## Troubleshooting

**AWS CLI not found**
- CloudShell: Already installed
- Local: Install from https://aws.amazon.com/cli/

**AWS credentials not configured**
```bash
aws configure
```

**SSH key not found**
- Create key pair in EC2 Console → Key Pairs
- Or: `aws ec2 create-key-pair --key-name joblet-key --query 'KeyMaterial' --output text > ~/.ssh/joblet-key.pem`

**Security group already exists**
- Scripts are idempotent, safe to re-run
- Or use existing: `export SG_ID=sg-xxxxxxxxx`

**DynamoDB table not created**
- Check IAM role is attached to instance
- Check IAM permissions: `aws iam list-attached-role-policies --role-name JobletEC2Role`
- Table is created automatically during package installation

**CloudWatch logs not appearing**
- Check IAM role has CloudWatch permissions
- Verify `ENABLE_CLOUDWATCH=true` in user data
- Check log group: `aws logs describe-log-groups --log-group-name-prefix /joblet`

---

## Cost Estimate

Approximate monthly costs (us-east-1, 24/7 operation):

- EC2 t3.medium: ~$30/month
- EBS 30GB gp3: ~$2.40/month
- CloudWatch Logs (10GB): ~$5/month
- DynamoDB (pay-per-request): ~$0.50/month

**Total: ~$38/month**

**Cost savings:**
- Use Reserved Instances (~40% discount)
- Stop instance when not in use
- Disable CloudWatch for dev/test

---

## See Also

- [AWS Deployment Guide](../../docs/AWS_DEPLOYMENT.md) - Complete documentation
- [EC2 Installation Guide](../../docs/installation/EC2_INSTALLATION.md) - Manual steps
- [Certificate Management](../../docs/CERTIFICATE_MANAGEMENT_COMPARISON.md) - Certificate options
- [Main Documentation](../../docs/README.md) - All Joblet docs
