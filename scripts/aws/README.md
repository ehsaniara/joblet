# Joblet AWS Deployment Scripts

Quick deployment scripts for setting up Joblet on AWS EC2 with CloudWatch Logs and DynamoDB state persistence.

## Quick Start (3 Commands)

Run these commands in AWS CloudShell or your terminal with AWS CLI configured:

```bash
# 1. Setup IAM (one-time)
curl -fsSL https://raw.githubusercontent.com/ehsaniara/joblet/main/scripts/aws/setup-iam.sh | bash

# 2. Create Security Group (one-time)
curl -fsSL https://raw.githubusercontent.com/ehsaniara/joblet/main/scripts/aws/setup-security-group.sh | bash

# 3. Launch Instance
export KEY_NAME="your-ssh-key-name"
curl -fsSL https://raw.githubusercontent.com/ehsaniara/joblet/main/scripts/aws/launch-instance.sh | bash
```

**Total time: ~10 minutes**

## What Gets Deployed

✅ **IAM Role** (`JobletEC2Role`)
- CloudWatch Logs permissions
- DynamoDB permissions
- EC2 metadata access

✅ **Security Group** (`joblet-server-sg`)
- SSH (22) from your IP
- gRPC (443) from your IP

✅ **EC2 Instance** (Ubuntu 22.04)
- Joblet server on port 443
- Automatic certificate generation
- systemd service (auto-starts)

✅ **CloudWatch Logs** (`/joblet` log group)
- Real-time job logs
- Automatic retention

✅ **DynamoDB** (`joblet-jobs` table)
- Persistent job state
- Auto-created with TTL

## Script Details

### 1. setup-iam.sh

Creates IAM role with permissions for CloudWatch Logs and DynamoDB.

**Usage:**
```bash
./setup-iam.sh
```

**What it creates:**
- IAM Policy: `JobletAWSPolicy`
- IAM Role: `JobletEC2Role`
- Instance Profile: `JobletEC2Role`

**Idempotent:** Safe to run multiple times.

### 2. setup-security-group.sh

Creates security group with SSH and gRPC access.

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
- gRPC (443): Your IP

**Idempotent:** Safe to run multiple times.

### 3. launch-instance.sh

Launches EC2 instance with Joblet bootstrap.

**Usage:**
```bash
export KEY_NAME="your-ssh-key"
./launch-instance.sh
```

**Environment variables:**
- `KEY_NAME` - SSH key pair name (required)
- `SG_ID` - Security group ID (optional, will prompt)
- `REGION` - AWS region (default: us-east-1)
- `INSTANCE_TYPE` - Instance type (default: t3.medium)
- `ENABLE_CLOUDWATCH` - Enable CloudWatch (default: true)

**What it creates:**
- EC2 instance with user data bootstrap
- 30 GB gp3 EBS volume
- Tags: Name=joblet-server, Application=Joblet

## Using AWS CloudShell

AWS CloudShell is perfect for running these scripts:

1. Open AWS Console → CloudShell (top right toolbar)

2. Run the setup:
```bash
# Setup IAM
curl -fsSL https://raw.githubusercontent.com/ehsaniara/joblet/main/scripts/aws/setup-iam.sh | bash

# Setup Security Group (auto-detects your IP)
curl -fsSL https://raw.githubusercontent.com/ehsaniara/joblet/main/scripts/aws/setup-security-group.sh | bash

# Launch instance (will prompt for SSH key)
curl -fsSL https://raw.githubusercontent.com/ehsaniara/joblet/main/scripts/aws/launch-instance.sh | bash
```

3. Wait 5 minutes for installation

4. Download client config:
```bash
# Get the instance IP from the output above
scp -i ~/.ssh/your-key.pem ubuntu@<PUBLIC_IP>:/opt/joblet/config/rnx-config.yml ~/.rnx/
```

## Verification

Check that everything is working:

```bash
# View CloudWatch Logs
aws logs describe-log-streams --log-group-name /joblet

# View DynamoDB table
aws dynamodb describe-table --table-name joblet-jobs

# SSH to instance
ssh -i ~/.ssh/your-key.pem ubuntu@<PUBLIC_IP>

# Check service status
sudo systemctl status joblet
```

## Manual Download (Alternative)

If you prefer to review scripts before running:

```bash
# Download scripts
wget https://raw.githubusercontent.com/ehsaniara/joblet/main/scripts/aws/setup-iam.sh
wget https://raw.githubusercontent.com/ehsaniara/joblet/main/scripts/aws/setup-security-group.sh
wget https://raw.githubusercontent.com/ehsaniara/joblet/main/scripts/aws/launch-instance.sh

# Make executable
chmod +x setup-iam.sh setup-security-group.sh launch-instance.sh

# Review
cat setup-iam.sh

# Run
./setup-iam.sh
./setup-security-group.sh
./launch-instance.sh
```

## Troubleshooting

**"AWS CLI not found"**
- CloudShell: Already installed
- Local: Install from https://aws.amazon.com/cli/

**"AWS credentials not configured"**
```bash
aws configure
```

**"SSH key not found"**
- Create key pair in EC2 Console → Key Pairs
- Or: `aws ec2 create-key-pair --key-name joblet-key --query 'KeyMaterial' --output text > ~/.ssh/joblet-key.pem`

**"Security group already exists"**
- Scripts are idempotent, safe to re-run
- Or use existing: `export SG_ID=sg-xxxxxxxxx`

## Cost

Approximate monthly costs:
- EC2 t3.medium: ~$30/month (24/7)
- EBS 30GB gp3: ~$2.40/month
- CloudWatch Logs: ~$0.50/GB ingested
- DynamoDB: Pay-per-request (negligible for typical usage)

**Total: ~$35/month for single instance**

## See Also

- [Complete AWS Deployment Guide](../../docs/AWS_DEPLOYMENT.md)
- [Certificate Management](../../docs/CERTIFICATE_MANAGEMENT_COMPARISON.md)
- [Main Documentation](../../docs/README.md)
