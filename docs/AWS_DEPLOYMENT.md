# Joblet AWS EC2 Deployment Guide

## Overview

This guide shows you how to deploy Joblet on AWS EC2 using the automated user data script. The script automatically
installs Joblet, configures TLS certificates, sets up networking, and optionally enables CloudWatch Logs.

**Time to deploy: ~5 minutes**

**Recommended Setup:**

- Port 443 (HTTPS) instead of default 50051 - better firewall compatibility
- Dedicated EC2 instance (don't run alongside web servers)
- CloudWatch Logs enabled for production workloads
- IAM role attached for CloudWatch permissions

> **Note on CloudWatch Logs**: CloudWatch is **optional**. For simple deployments or development, set
`ENABLE_CLOUDWATCH="false"` in the user data script and skip the IAM setup section entirely. Logs will be stored locally
> on the EC2 instance instead.

## Prerequisites

- AWS account with EC2 launch permissions
- SSH key pair in your target region
- Basic knowledge of EC2 console or AWS CLI

**Supported Operating Systems:**

- Ubuntu 22.04 LTS (recommended)
- Ubuntu 20.04 LTS
- Debian 11/10
- Amazon Linux 2023 (recommended for AL)
- Amazon Linux 2
- RHEL 8+ / CentOS Stream 8+ / Fedora 30+

**Note on SSH Usernames:**
Throughout this guide, `<EC2_USER>` refers to the default SSH user for your AMI:

- Ubuntu/Debian: Use `ubuntu`
- Amazon Linux: Use `ec2-user`
- RHEL: Use `ec2-user`

## Quick Start (AWS Console)

### Step 1: Launch EC2 Instance

1. Go to **EC2 Console** → **Launch Instance**

2. **Configure Instance:**
    - Name: `joblet-server`
    - AMI: Ubuntu 22.04 LTS (or Amazon Linux 2023)
    - Instance type: `t3.medium` (minimum: t3.small)
    - Key pair: Select your SSH key

3. **Network Settings:**
    - VPC: Default or your VPC
    - Auto-assign public IP: Enable
    - Security group: Create new (see Security Group section below)

4. **Storage:**
    - 30 GB gp3 (minimum: 20 GB)

5. **Advanced Details:**
    - **IAM instance profile**: Select `JobletCloudWatchRole` (create first - see IAM Setup below)
    - **User data** - Paste this script:

   ```bash
   #!/bin/bash
   set -e

   # Configuration
   export JOBLET_VERSION="latest"
   export JOBLET_SERVER_PORT="443"
   export ENABLE_CLOUDWATCH="true"
   export JOBLET_CERT_DOMAIN=""  # Optional: your custom domain

   # Download and run installation script
   curl -fsSL https://raw.githubusercontent.com/ehsaniara/joblet/main/scripts/ec2-user-data.sh -o /tmp/joblet-install.sh
   chmod +x /tmp/joblet-install.sh
   /tmp/joblet-install.sh 2>&1 | tee /var/log/joblet-install.log
   ```

6. **Launch** the instance

### Step 2: Wait for Installation

Installation takes 3-5 minutes. You can monitor progress:

```bash
# SSH into instance (use 'ubuntu' for Ubuntu AMI, 'ec2-user' for Amazon Linux)
ssh -i ~/.ssh/your-key.pem <EC2_USER>@<PUBLIC_IP>

# Watch installation log
tail -f /var/log/joblet-install.log

# Check for completion
grep "Installation Complete" /var/log/joblet-install.log
```

### Step 3: Verify Installation

```bash
# On EC2 instance:

# Check service is running
sudo systemctl status joblet

# Check persist subprocess is running
ps aux | grep joblet-persist | grep -v grep

# Check sockets exist
ls -la /opt/joblet/run/

# Run test job
sudo rnx job run echo "Hello from Joblet"
```

## IAM Role Setup

CloudWatch Logs requires an IAM role. Create this **before launching** your instance.

### Create IAM Policy

```bash
# Create policy document
cat > joblet-cloudwatch-policy.json << 'EOF'
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "logs:CreateLogGroup",
        "logs:CreateLogStream",
        "logs:PutLogEvents",
        "logs:PutRetentionPolicy",
        "logs:DescribeLogStreams",
        "logs:GetLogEvents",
        "logs:FilterLogEvents",
        "logs:DeleteLogGroup",
        "logs:DeleteLogStream"
      ],
      "Resource": [
        "arn:aws:logs:*:*:log-group:/joblet/*",
        "arn:aws:logs:*:*:log-group:/joblet/*:*"
      ]
    },
    {
      "Effect": "Allow",
      "Action": [
        "cloudwatch:PutMetricData",
        "cloudwatch:GetMetricStatistics",
        "cloudwatch:ListMetrics"
      ],
      "Resource": "*"
    },
    {
      "Effect": "Allow",
      "Action": [
        "ec2:DescribeRegions"
      ],
      "Resource": "*"
    }
  ]
}
EOF

# Create policy and capture ARN
POLICY_ARN=$(aws iam create-policy \
  --policy-name JobletCloudWatchLogsPolicy \
  --policy-document file://joblet-cloudwatch-policy.json \
  --query 'Policy.Arn' \
  --output text)

echo "Created policy: $POLICY_ARN"
```

### Create IAM Role

```bash
# Create role with EC2 trust policy
aws iam create-role \
  --role-name JobletCloudWatchRole \
  --assume-role-policy-document '{
    "Version": "2012-10-17",
    "Statement": [{
      "Effect": "Allow",
      "Principal": {"Service": "ec2.amazonaws.com"},
      "Action": "sts:AssumeRole"
    }]
  }'

# Attach policy to role
aws iam attach-role-policy \
  --role-name JobletCloudWatchRole \
  --policy-arn $POLICY_ARN

# Create instance profile
aws iam create-instance-profile \
  --instance-profile-name JobletCloudWatchRole

# Add role to instance profile
aws iam add-role-to-instance-profile \
  --instance-profile-name JobletCloudWatchRole \
  --role-name JobletCloudWatchRole

echo "IAM role ready: JobletCloudWatchRole"
```

**Don't want CloudWatch?** Set `ENABLE_CLOUDWATCH="false"` in user data and skip IAM setup.

## Security Group Configuration

Create a security group with these rules:

```bash
# Create security group
SG_ID=$(aws ec2 create-security-group \
  --group-name joblet-server-sg \
  --description "Security group for Joblet server" \
  --vpc-id vpc-xxxxxxxxx \
  --output text)

# Allow SSH from your IP
aws ec2 authorize-security-group-ingress \
  --group-id $SG_ID \
  --protocol tcp \
  --port 22 \
  --cidr YOUR_IP/32

# Allow Joblet gRPC on port 443
aws ec2 authorize-security-group-ingress \
  --group-id $SG_ID \
  --protocol tcp \
  --port 443 \
  --cidr YOUR_IP/32

echo "Security group created: $SG_ID"
```

**Or via Console:**

- Inbound rules:
    - SSH (22): Your IP only
    - Custom TCP (443): Your IP or VPC CIDR
- Outbound rules: Allow all (default)

**Note:** Port 443 is for gRPC over TLS, not HTTP/HTTPS. This is purely for firewall compatibility.

## Post-Deployment

### Download Client Configuration

From your **local machine**:

```bash
# Create config directory
mkdir -p ~/.rnx

# Download config from EC2 instance
scp -i ~/.ssh/your-key.pem <EC2_USER>@<PUBLIC_IP>:/opt/joblet/config/rnx-config.yml ~/.rnx/

# Test connection
rnx job list
```

### Connection Options

The EC2 installation creates certificates with multiple addresses:

- Internal IP: `10.0.1.100` (for VPC access)
- Public IP: `3.15.123.45` (for external access)
- EC2 Public DNS: `ec2-3-15-123-45.us-east-1.compute.amazonaws.com`
- Custom domain: If you set `JOBLET_CERT_DOMAIN`

You can configure multiple nodes in `~/.rnx/rnx-config.yml`:

```yaml
nodes:
  default:
    address: "10.0.1.100:443"  # Internal IP
    cert: |
      -----BEGIN CERTIFICATE-----
      ...

  production:
    address: "ec2-3-15-123-45.us-east-1.compute.amazonaws.com:443"  # Public DNS
    cert: |
      -----BEGIN CERTIFICATE-----
      ...
```

**Usage:**

```bash
rnx job list                      # Uses default node
rnx --node=production job list    # Uses production node
```

### SSH Tunneling (For Private Instances)

If your EC2 instance is in a private subnet or you want to connect through SSH tunneling:

**1. Create SSH tunnel:**

```bash
# Forward local port 50051 to the EC2 instance's internal IP on port 443
ssh -N -v -L 50051:PRIVATE_IP:443 <EC2_USER>@EC2_PUBLIC_IP -i your-key.pem

# Example with actual values (using Ubuntu):
ssh -N -v -L 50051:10.0.1.100:443 ubuntu@3.15.123.45 -i ~/.ssh/joblet-key.pem
```

**Flags explained:**

- `-N`: Don't execute remote commands (tunnel only)
- `-v`: Verbose mode (helps debug connection issues)
- `-L 50051:PRIVATE_IP:443`: Forward local port 50051 to PRIVATE_IP:443 on the remote side
- `-i your-key.pem`: SSH private key

**2. Configure rnx to use localhost:**

Edit `~/.rnx/rnx-config.yml`:

```yaml
nodes:
  tunnel:
    address: "localhost:50051"  # Connect through SSH tunnel
    cert: |
      -----BEGIN CERTIFICATE-----
      ...  # Use the same certificate from the EC2 instance
```

**3. Test connection:**

```bash
# Keep the SSH tunnel running in one terminal
ssh -N -v -L 50051:10.0.1.100:443 ubuntu@3.15.123.45 -i ~/.ssh/joblet-key.pem

# In another terminal, use the tunnel
rnx --node=tunnel job list
```

**Persistent tunnel (optional):**

For a persistent tunnel that reconnects automatically, use `autossh`:

```bash
# Install autossh
sudo apt-get install autossh  # Ubuntu/Debian
sudo yum install autossh      # Amazon Linux/RHEL

# Create persistent tunnel
autossh -M 0 -N -v -L 50051:10.0.1.100:443 ubuntu@3.15.123.45 -i ~/.ssh/joblet-key.pem
```

**Common use cases:**

- EC2 instance in private subnet (no public IP)
- Bastion host/jump server setup
- Additional security layer (no direct gRPC port exposure)
- Testing from environments where only SSH (port 22) is allowed

### Run Test Jobs

```bash
# Submit a job
rnx job run echo "Hello from Joblet on EC2"

# View job list
rnx job list

# View job logs
rnx job log <JOB_ID>

# Check job status
rnx job status <JOB_ID>
```

### Access CloudWatch Logs

If you enabled CloudWatch, view logs in AWS Console:

1. **CloudWatch Logs** → **Log groups**:
    - `/joblet/<NODE_ID>/jobs` - All job logs for this node (grouped by node)
        - Log streams: `<JOB_ID>-stdout`, `<JOB_ID>-stderr`
    - **Default retention**: 7 days (configurable)

2. **CloudWatch Metrics** → **Custom namespaces** → **Joblet/Jobs**:
    - `CPUUsage`, `MemoryUsage`, `GPUUsage`
    - `DiskReadBytes`, `DiskWriteBytes`
    - `NetworkRxBytes`, `NetworkTxBytes`
    - **Retention**: 15 months (automatic)

**Via CLI:**

```bash
# List log groups
aws logs describe-log-groups --log-group-name-prefix /joblet

# Get instance metadata to find NODE_ID
aws ec2 describe-instances --instance-ids i-xxxxxxxxx \
  --query 'Reservations[0].Instances[0].Tags[?Key==`NodeId`].Value' --output text

# Tail job logs (replace NODE_ID and JOB_ID)
aws logs tail /joblet/<NODE_ID>/jobs --follow --filter-pattern <JOB_ID>

# View specific job log stream
aws logs get-log-events \
  --log-group-name /joblet/<NODE_ID>/jobs \
  --log-stream-name <JOB_ID>-stdout

# Query metrics
aws cloudwatch get-metric-statistics \
  --namespace Joblet/Jobs \
  --metric-name CPUUsage \
  --dimensions Name=JobID,Value=<JOB_ID> Name=NodeID,Value=<NODE_ID> \
  --start-time $(date -u -d '1 hour ago' +%Y-%m-%dT%H:%M:%S) \
  --end-time $(date -u +%Y-%m-%dT%H:%M:%S) \
  --period 60 \
  --statistics Average
```

For advanced CloudWatch queries and monitoring, see [MONITORING.md](MONITORING.md).

## Troubleshooting

### Installation Failed

**Check installation log:**

```bash
ssh -i ~/.ssh/your-key.pem <EC2_USER>@<PUBLIC_IP>
cat /var/log/joblet-install.log
grep -i error /var/log/joblet-install.log
```

**Common causes:**

- No internet access: Check VPC has internet gateway or NAT gateway
- Package download failed: Check GitHub is accessible
- Disk full: Use 30GB or larger volume

### Service Won't Start

**Check service status:**

```bash
sudo systemctl status joblet -l
sudo journalctl -u joblet -n 100 --no-pager
```

**Common issues:**

1. **Port 443 already in use:**
   ```bash
   sudo ss -tlnp | grep 443
   # If occupied, use different port: JOBLET_SERVER_PORT="8443"
   ```

2. **Missing /opt/joblet/run directory:**
   ```bash
   sudo mkdir -p /opt/joblet/run
   sudo systemctl restart joblet
   ```

3. **Certificate issues:**
   ```bash
   sudo /usr/local/bin/certs_gen_embedded.sh
   sudo systemctl restart joblet
   ```

### joblet-persist Not Running

**Check if persist subprocess exists:**

```bash
ps aux | grep joblet-persist | grep -v grep
ls -la /opt/joblet/run/
```

**Most common cause: Missing IAM role or wrong CloudWatch config**

**Fix Option 1 - Attach IAM role:**

```bash
# From local machine
aws ec2 associate-iam-instance-profile \
  --instance-id i-xxxxxxxxx \
  --iam-instance-profile Name=JobletCloudWatchRole

# Then restart on EC2 instance
sudo systemctl restart joblet
```

**Fix Option 2 - Disable CloudWatch:**

```bash
# Edit config
sudo nano /opt/joblet/config/joblet-config.yml

# Change:
persist:
  storage:
    type: "local"  # Changed from "cloudwatch"

    local:
      logs:
        directory: "/opt/joblet/logs"
      metrics:
        directory: "/opt/joblet/metrics"

# Save and restart
sudo systemctl restart joblet
```

### Can't Connect from Client

**Check server is listening:**

```bash
ssh -i ~/.ssh/your-key.pem <EC2_USER>@<PUBLIC_IP> "sudo ss -tlnp | grep 443"
```

**Common issues:**

1. **Security group doesn't allow traffic:**
   ```bash
   aws ec2 authorize-security-group-ingress \
     --group-id sg-xxxxxxxxx \
     --protocol tcp \
     --port 443 \
     --cidr YOUR_IP/32
   ```

2. **Wrong IP in client config:**
   ```bash
   # Download fresh config
   scp -i ~/.ssh/your-key.pem <EC2_USER>@<PUBLIC_IP>:/opt/joblet/config/rnx-config.yml ~/.rnx/
   ```

### CloudWatch Logs Not Working

**Check IAM role is attached:**

```bash
# From local machine
aws ec2 describe-instances \
  --instance-ids i-xxxxxxxxx \
  --query 'Reservations[0].Instances[0].IamInstanceProfile'
```

**Check joblet logs for CloudWatch errors:**

```bash
sudo journalctl -u joblet | grep -i cloudwatch
```

**If no role attached, attach it:**

```bash
aws ec2 associate-iam-instance-profile \
  --instance-id i-xxxxxxxxx \
  --iam-instance-profile Name=JobletCloudWatchRole

# Restart joblet
ssh -i ~/.ssh/your-key.pem <EC2_USER>@<PUBLIC_IP> "sudo systemctl restart joblet"
```

## Configuration Options

The user data script accepts these environment variables:

| Variable             | Description                   | Default  | Recommended |
|----------------------|-------------------------------|----------|-------------|
| `JOBLET_VERSION`     | Version to install            | `latest` | `latest`    |
| `JOBLET_SERVER_PORT` | gRPC server port              | `50051`  | `443`       |
| `ENABLE_CLOUDWATCH`  | Enable CloudWatch Logs        | `true`   | `true`      |
| `JOBLET_CERT_DOMAIN` | Custom domain for certificate | (empty)  | (optional)  |

**Example with custom configuration:**

```bash
#!/bin/bash
export JOBLET_VERSION="v1.2.0"
export JOBLET_SERVER_PORT="8443"
export ENABLE_CLOUDWATCH="false"
export JOBLET_CERT_DOMAIN="joblet.example.com"

curl -fsSL https://raw.githubusercontent.com/ehsaniara/joblet/main/scripts/ec2-user-data.sh -o /tmp/joblet-install.sh
chmod +x /tmp/joblet-install.sh
/tmp/joblet-install.sh
```

### CloudWatch Log Retention

After installation, you can configure log retention to control storage costs:

```bash
# SSH into instance
ssh -i ~/.ssh/your-key.pem <EC2_USER>@<PUBLIC_IP>

# Edit config
sudo nano /opt/joblet/config/joblet-config.yml

# Under persist.storage.cloudwatch, add:
persist:
  storage:
    type: cloudwatch
    cloudwatch:
      log_retention_days: 7    # Default (if not set)

      # Other options:
      # log_retention_days: 1    # Development (minimal cost)
      # log_retention_days: 30   # Production (balance)
      # log_retention_days: 365  # Compliance (1 year)
      # log_retention_days: -1   # Never expire (expensive!)

# Restart to apply
sudo systemctl restart joblet
```

**Valid retention values:** 1, 3, 5, 7, 14, 30, 60, 90, 120, 150, 180, 365, 400, 545, 731, 1827, 3653, -1 (never expire)

**Note:** CloudWatch Metrics retention is fixed at 15 months and cannot be configured.

## Quick Start (AWS CLI)

If you prefer AWS CLI:

```bash
# Find latest Ubuntu AMI
AMI_ID=$(aws ec2 describe-images \
  --owners 099720109477 \
  --filters "Name=name,Values=ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-amd64-server-*" \
  --query 'sort_by(Images, &CreationDate)[-1].ImageId' \
  --output text)

# Create user-data.sh (paste user data script from Quick Start section above)

# Launch instance
aws ec2 run-instances \
  --image-id $AMI_ID \
  --instance-type t3.medium \
  --key-name my-key-pair \
  --security-group-ids sg-xxxxxxxxx \
  --subnet-id subnet-xxxxxxxxx \
  --iam-instance-profile Name=JobletCloudWatchRole \
  --user-data file://user-data.sh \
  --block-device-mappings '[{"DeviceName":"/dev/sda1","Ebs":{"VolumeSize":30,"VolumeType":"gp3"}}]' \
  --tag-specifications 'ResourceType=instance,Tags=[{Key=Name,Value=joblet-server}]'
```

## Maintenance

### Update Joblet

```bash
# SSH into instance
ssh -i ~/.ssh/your-key.pem <EC2_USER>@<PUBLIC_IP>

# For Ubuntu/Debian
wget https://github.com/ehsaniara/joblet/releases/download/v1.2.0/joblet_1.2.0_amd64.deb
sudo dpkg -i joblet_1.2.0_amd64.deb
sudo systemctl restart joblet

# For Amazon Linux/RHEL
wget https://github.com/ehsaniara/joblet/releases/download/v1.2.0/joblet-1.2.0-1.x86_64.rpm
sudo yum localinstall -y joblet-1.2.0-1.x86_64.rpm
sudo systemctl restart joblet
```

### Backup Configuration

```bash
# Backup config and data
ssh -i ~/.ssh/your-key.pem <EC2_USER>@<PUBLIC_IP> \
  "sudo tar czf /tmp/joblet-backup.tar.gz /opt/joblet/config /opt/joblet/volumes"

# Download backup
scp -i ~/.ssh/your-key.pem <EC2_USER>@<PUBLIC_IP>:/tmp/joblet-backup.tar.gz ./
```

**For production:** Use EBS snapshots for complete backups.

## Cost Considerations

Key cost factors to consider (prices vary by region):

| Component          | Configuration               | Notes                     |
|--------------------|-----------------------------|---------------------------|
| EC2 Instance       | t3.medium (2 vCPU, 4GB RAM) | Main compute cost         |
| EBS Storage        | 30 GB gp3                   | Storage for jobs and data |
| CloudWatch Logs    | Per GB ingested + stored    | 7-day retention (default) |
| CloudWatch Metrics | Per unique metric           | 9 metrics per job         |

**CloudWatch Logs breakdown (100 jobs/day, 1MB each):**

- Ingestion: 3 GB/day ingested
- Storage (7-day retention): ~21 GB stored
- Note: Ingestion costs typically dominate over storage

**Cost optimization strategies:**

- Use t3.small for development environments
- Stop instance during off-hours (saves ~50%)
- Disable CloudWatch for dev/test environments
- Reduce log retention to 1 day for development
- Use Reserved Instances for production (significant savings)
- Consider Spot Instances for batch/non-critical workloads

## Next Steps

- **Monitoring**: See [MONITORING.md](MONITORING.md) for CloudWatch dashboards, alerts, and queries
- **Persistence**: See [PERSISTENCE.md](PERSISTENCE.md) for CloudWatch backend configuration
- **Security**: See installation notes for security best practices
- **SDK Integration**: See [API.md](API.md) for Python/Go SDK examples

## Support

- **GitHub**: https://github.com/ehsaniara/joblet
- **Issues**: https://github.com/ehsaniara/joblet/issues
- **Installation Notes**:
    - Debian/Ubuntu: [INSTALL_NOTES.md](INSTALL_NOTES.md)
    - RHEL/Amazon Linux: [INSTALL_NOTES_RPM.md](INSTALL_NOTES_RPM.md)
