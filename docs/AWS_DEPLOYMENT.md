# Joblet AWS EC2 Deployment Guide

## Overview

Joblet provides an automated EC2 user data script for deploying on AWS EC2 instances. The script automatically:
- Detects the OS (Ubuntu/Debian vs Amazon Linux/RHEL)
- Installs the appropriate Joblet package (.deb or .rpm)
- Configures TLS certificates with EC2 public/private IPs
- Sets up network isolation
- Optionally enables CloudWatch Logs backend
- Starts the Joblet service

**Recommended for EC2**: Use port **443** (HTTPS) instead of the default 50051. Port 443 is typically allowed through corporate firewalls and makes Joblet accessible from restricted networks.

**Important**: Joblet requires a **dedicated EC2 instance** with port 443 available. Do not install Joblet on instances running web servers (nginx, Apache) or other services using port 443. If port 443 is unavailable, use an alternative port like 8443 or 9443.

## Prerequisites

### AWS Account Requirements

- AWS account with EC2 launch permissions
- SSH key pair created in your target region
- VPC and subnet (or use default VPC)
- IAM permissions to create:
  - EC2 instances
  - Security groups
  - Elastic IPs (optional)
  - IAM roles (if using CloudWatch Logs)

### Supported Operating Systems

- Ubuntu 22.04 LTS (recommended)
- Ubuntu 20.04 LTS
- Debian 11 (Bullseye)
- Debian 10 (Buster)
- Amazon Linux 2023 (recommended for AL)
- Amazon Linux 2
- RHEL 8+ / CentOS Stream 8+
- Fedora 30+

## Quick Start

### Option A: Launch via AWS Console

1. **Navigate to EC2 Console** → Launch Instance

2. **Choose AMI**:
   - Ubuntu 22.04 LTS (recommended)
   - Amazon Linux 2023 (for Amazon Linux users)

3. **Choose Instance Type**

4. **Configure Instance Details**:
   - **Network**: Choose VPC and subnet
   - **IAM role**: Select role with CloudWatch Logs permissions (optional, see IAM Role Setup below)
   - **Advanced Details** → **User Data** - Enter the following:

   ```bash
   #!/bin/bash
   set -e

   # Configuration
   export JOBLET_VERSION="latest"
   export JOBLET_SERVER_PORT="443"  # Using 443 (HTTPS) for firewall-friendly access
   export ENABLE_CLOUDWATCH="true"
   export JOBLET_CERT_DOMAIN=""  # Optional: your domain name

   # Download and run installation script
   curl -fsSL https://raw.githubusercontent.com/ehsaniara/joblet/main/scripts/ec2-user-data.sh -o /tmp/joblet-install.sh
   chmod +x /tmp/joblet-install.sh
   /tmp/joblet-install.sh 2>&1 | tee /var/log/joblet-install.log
   ```

5. **Add Storage**: 30GB or more (gp3 recommended)

6. **Configure Security Group**:
   - SSH (22): From your IP address
   - HTTPS (443): From allowed sources (e.g., your VPC CIDR or your office IP)
     - **Note**: Joblet uses port 443 for gRPC over TLS (not HTTP/HTTPS)

7. **Review and Launch**

8. **Wait for Installation**: The installation takes about 3-5 minutes

### Option B: Launch via AWS CLI

**1. Create user data script file:**

Create `user-data.sh`:
```bash
#!/bin/bash
set -e

export JOBLET_VERSION="latest"
export JOBLET_SERVER_PORT="443"  # Using 443 for firewall-friendly access
export ENABLE_CLOUDWATCH="true"
export JOBLET_CERT_DOMAIN=""

curl -fsSL https://raw.githubusercontent.com/ehsaniara/joblet/main/scripts/ec2-user-data.sh -o /tmp/joblet-install.sh
chmod +x /tmp/joblet-install.sh
/tmp/joblet-install.sh 2>&1 | tee /var/log/joblet-install.log
```

**2. Launch instance:**

```bash
# Find latest Ubuntu 22.04 AMI for your region
AMI_ID=$(aws ec2 describe-images \
  --owners 099720109477 \
  --filters "Name=name,Values=ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-amd64-server-*" \
  --query 'sort_by(Images, &CreationDate)[-1].ImageId' \
  --output text)

# Launch instance
aws ec2 run-instances \
  --image-id $AMI_ID \
  --instance-type t3.medium \
  --key-name my-key-pair \
  --security-group-ids sg-xxxxxxxxx \
  --subnet-id subnet-xxxxxxxxx \
  --iam-instance-profile Name=JobletCloudWatchRole \
  --user-data file://user-data.sh \
  --block-device-mappings '[{
    "DeviceName":"/dev/sda1",
    "Ebs":{
      "VolumeSize":30,
      "VolumeType":"gp3",
      "DeleteOnTermination":true,
      "Encrypted":true
    }
  }]' \
  --tag-specifications 'ResourceType=instance,Tags=[{Key=Name,Value=joblet-server}]'
```

**3. Get instance details:**

```bash
# Get instance ID from launch output
INSTANCE_ID="i-xxxxxxxxx"

# Wait for instance to be running
aws ec2 wait instance-running --instance-ids $INSTANCE_ID

# Get public IP
PUBLIC_IP=$(aws ec2 describe-instances \
  --instance-ids $INSTANCE_ID \
  --query 'Reservations[0].Instances[0].PublicIpAddress' \
  --output text)

echo "Joblet server will be available at: $PUBLIC_IP:443"
echo "SSH: ssh -i ~/.ssh/my-key-pair.pem ubuntu@$PUBLIC_IP"
```

### Option C: Launch via EC2 Launch Template

**1. Create launch template:**

```bash
aws ec2 create-launch-template \
  --launch-template-name joblet-server-template \
  --launch-template-data '{
    "ImageId": "ami-xxxxxxxxx",
    "InstanceType": "t3.medium",
    "KeyName": "my-key-pair",
    "IamInstanceProfile": {
      "Name": "JobletCloudWatchRole"
    },
    "BlockDeviceMappings": [{
      "DeviceName": "/dev/sda1",
      "Ebs": {
        "VolumeSize": 30,
        "VolumeType": "gp3",
        "DeleteOnTermination": true,
        "Encrypted": true
      }
    }],
    "UserData": "'"$(base64 -w0 user-data.sh)"'"
  }'
```

**2. Launch instance from template:**

```bash
aws ec2 run-instances \
  --launch-template LaunchTemplateName=joblet-server-template \
  --subnet-id subnet-xxxxxxxxx \
  --security-group-ids sg-xxxxxxxxx \
  --tag-specifications 'ResourceType=instance,Tags=[{Key=Name,Value=joblet-server}]'
```

## Environment Variables

The user data script accepts these environment variables for customization:

| Variable | Description | Default | Recommended for EC2 | Example |
|----------|-------------|---------|---------------------|---------|
| `JOBLET_VERSION` | Joblet version to install | `latest` | `latest` | `v1.0.0` |
| `JOBLET_SERVER_PORT` | gRPC server port | `50051` | **`443`** | `443` |
| `ENABLE_CLOUDWATCH` | Enable CloudWatch Logs backend | `true` | `true` | `true` or `false` |
| `JOBLET_CERT_DOMAIN` | Optional domain for certificate SAN | (empty) | (empty) | `joblet.example.com` |

**Why port 443 for EC2?**
- Port 443 (HTTPS) is allowed through most corporate firewalls
- Port 50051 is often blocked by restrictive network policies
- Makes Joblet accessible from client machines behind firewalls
- TLS is still used for encryption (gRPC over TLS, not HTTP)

**Example with recommended EC2 configuration:**

```bash
#!/bin/bash
export JOBLET_VERSION="latest"
export JOBLET_SERVER_PORT="443"  # Firewall-friendly
export ENABLE_CLOUDWATCH="true"
export JOBLET_CERT_DOMAIN=""

curl -fsSL https://raw.githubusercontent.com/ehsaniara/joblet/main/scripts/ec2-user-data.sh -o /tmp/joblet-install.sh
chmod +x /tmp/joblet-install.sh
/tmp/joblet-install.sh
```

**Example with custom port (if 443 is not available):**

```bash
#!/bin/bash
export JOBLET_VERSION="v1.0.0"
export JOBLET_SERVER_PORT="8443"  # Alternative port
export ENABLE_CLOUDWATCH="false"
export JOBLET_CERT_DOMAIN="joblet.mycompany.com"

curl -fsSL https://raw.githubusercontent.com/ehsaniara/joblet/main/scripts/ec2-user-data.sh -o /tmp/joblet-install.sh
chmod +x /tmp/joblet-install.sh
/tmp/joblet-install.sh
```

## IAM Role Setup

### For CloudWatch Logs (Recommended)

**1. Create IAM policy:**

Create `joblet-cloudwatch-policy.json`:
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
        "logs:FilterLogEvents"
      ],
      "Resource": "arn:aws:logs:*:*:log-group:/joblet/*"
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
```

**2. Create policy and role:**

```bash
# Create policy
aws iam create-policy \
  --policy-name JobletCloudWatchLogsPolicy \
  --policy-document file://joblet-cloudwatch-policy.json

# Create role
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
  --policy-arn arn:aws:iam::123456789012:policy/JobletCloudWatchLogsPolicy

# Create instance profile
aws iam create-instance-profile \
  --instance-profile-name JobletCloudWatchRole

# Add role to instance profile
aws iam add-role-to-instance-profile \
  --instance-profile-name JobletCloudWatchRole \
  --role-name JobletCloudWatchRole
```

**3. Wait a moment for IAM propagation, then use in EC2 launch**

## Security Group Configuration

### Recommended Security Group Rules

**Create security group:**

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
  --cidr YOUR_IP/32 \
  --description "SSH access"

# Allow Joblet gRPC on port 443 (HTTPS) - firewall-friendly
aws ec2 authorize-security-group-ingress \
  --group-id $SG_ID \
  --protocol tcp \
  --port 443 \
  --cidr YOUR_OFFICE_IP/32 \
  --description "Joblet gRPC over TLS (port 443)"

# Or allow from your VPC if clients are within VPC
# aws ec2 authorize-security-group-ingress \
#   --group-id $SG_ID \
#   --protocol tcp \
#   --port 443 \
#   --cidr 10.0.0.0/16 \
#   --description "Joblet gRPC from VPC"

# Allow all outbound (default)
echo "Security group created: $SG_ID"
```

**Note**: Joblet uses port 443 for **gRPC over TLS**, not HTTP/HTTPS. This is purely for firewall compatibility.

### Security Best Practices

1. **Use port 443 instead of 50051**: Better firewall compatibility
2. **Restrict SSH access**: Use your specific IP, not 0.0.0.0/0
3. **Restrict Joblet port**: Use your office IP or VPC CIDR, not 0.0.0.0/0
4. **Use VPC**: Don't deploy in EC2-Classic
5. **Enable VPC Flow Logs**: For network monitoring
6. **Use IMDSv2**: Ensure your AMI supports IMDSv2 (default for modern AMIs)

## Monitoring Installation

### Check Installation Progress

**SSH into the instance:**

```bash
ssh -i ~/.ssh/my-key-pair.pem ubuntu@<PUBLIC_IP>
```

**View installation log:**

```bash
# Follow installation log in real-time
tail -f /var/log/joblet-install.log

# View entire log
cat /var/log/joblet-install.log

# Check for errors
grep ERROR /var/log/joblet-install.log
grep FAILED /var/log/joblet-install.log
```

**Check service status:**

```bash
# Check if Joblet is running
sudo systemctl status joblet

# View service logs
sudo journalctl -u joblet -f

# Check if port is listening (443 if using recommended EC2 setup)
sudo ss -tlnp | grep 443
```

### Verify Installation

```bash
# Check binaries
which rnx
rnx --version

# Check network bridge
ip link show joblet0
ip addr show joblet0

# Test Joblet server
sudo rnx job list

# Run test job
sudo rnx job run echo "Hello from Joblet"
```

## Post-Deployment Steps

### 1. Download Client Configuration

**From your local machine:**

```bash
# Create .rnx directory
mkdir -p ~/.rnx

# Download client configuration
scp -i ~/.ssh/my-key-pair.pem ubuntu@<PUBLIC_IP>:/opt/joblet/config/rnx-config.yml ~/.rnx/

# Test connection (uses 'default' node)
rnx job list
```

**Multiple Connection Options:**

The EC2 installation creates certificates with multiple Subject Alternative Names (SANs):
- Internal IP (e.g., `10.0.1.100`)
- Public IP (e.g., `3.15.123.45`)
- EC2 Public DNS (e.g., `ec2-3-15-123-45.us-east-1.compute.amazonaws.com`)
- Custom domain (if `JOBLET_CERT_DOMAIN` is set)

You can configure multiple nodes in `~/.rnx/rnx-config.yml` to connect using different addresses:

```yaml
nodes:
  # Default node - uses internal IP (for EC2 instances in same VPC)
  default:
    address: "10.0.1.100:443"
    cert: |
      -----BEGIN CERTIFICATE-----
      ...
    key: |
      -----BEGIN PRIVATE KEY-----
      ...
    ca: |
      -----BEGIN CERTIFICATE-----
      ...

  # Production node - uses EC2 public DNS (for external access)
  production:
    address: "ec2-3-15-123-45.us-east-1.compute.amazonaws.com:443"
    cert: |
      -----BEGIN CERTIFICATE-----
      ...
    key: |
      -----BEGIN PRIVATE KEY-----
      ...
    ca: |
      -----BEGIN CERTIFICATE-----
      ...

  # Public node - uses public IP (alternative external access)
  public:
    address: "3.15.123.45:443"
    cert: |
      -----BEGIN CERTIFICATE-----
      ...
    key: |
      -----BEGIN PRIVATE KEY-----
      ...
    ca: |
      -----BEGIN CERTIFICATE-----
      ...
```

**Usage with different nodes:**

```bash
# Use default node (internal IP)
rnx job list

# Use production node (EC2 public DNS)
rnx --node=production job list

# Use public node (public IP)
rnx --node=public job list
```

All addresses will work because they're all included in the certificate SANs.

### 2. Run Test Job

```bash
# Submit a test job
rnx job run echo "Hello from Joblet on EC2"

# View job list
rnx job list

# View job logs
rnx job log <JOB_ID>

# Check job status
rnx job status <JOB_ID>
```

### 3. Configure CloudWatch Logs (if enabled)

**View logs in CloudWatch Console:**

1. Navigate to CloudWatch → Logs → Log groups
2. Look for `/joblet` log groups:
   - `/joblet/job-<JOB_ID>` - Individual job logs
   - `/joblet/metrics` - Job metrics
   - `/joblet/server` - Server logs

**View logs via AWS CLI:**

```bash
# List log groups
aws logs describe-log-groups --log-group-name-prefix /joblet

# List log streams for a job
aws logs describe-log-streams \
  --log-group-name /joblet/job-abc123 \
  --order-by LastEventTime

# Tail job logs
aws logs tail /joblet/job-abc123 --follow

# Query with CloudWatch Insights
aws logs start-query \
  --log-group-name /joblet/server \
  --start-time $(date -d '1 hour ago' +%s) \
  --end-time $(date +%s) \
  --query-string 'fields @timestamp, @message | filter @message like /ERROR/ | sort @timestamp desc'
```

### 4. Optional: Attach Elastic IP

**For stable IP address:**

```bash
# Allocate Elastic IP
ALLOCATION_ID=$(aws ec2 allocate-address \
  --domain vpc \
  --query 'AllocationId' \
  --output text)

# Associate with instance
aws ec2 associate-address \
  --instance-id i-xxxxxxxxx \
  --allocation-id $ALLOCATION_ID

# Get new public IP
ELASTIC_IP=$(aws ec2 describe-addresses \
  --allocation-ids $ALLOCATION_ID \
  --query 'Addresses[0].PublicIp' \
  --output text)

echo "New Elastic IP: $ELASTIC_IP"
```

**Note**: After attaching Elastic IP, you'll need to download a new client config with the updated IP, or you can regenerate certificates with the new IP.

## CloudWatch Logs Integration

### Log Groups Created

When CloudWatch is enabled, Joblet creates these log groups:

| Log Group | Description | Retention |
|-----------|-------------|-----------|
| `/joblet/job-<JOB_ID>` | Individual job stdout/stderr | 7 days |
| `/joblet/metrics` | Job resource usage metrics | 30 days |
| `/joblet/server` | Joblet server logs | 7 days |

### CloudWatch Insights Queries

**Find failed jobs:**

```sql
fields @timestamp, job_id, status, exit_code
| filter status = "FAILED"
| sort @timestamp desc
| limit 20
```

**Jobs by duration:**

```sql
fields @timestamp, job_id, duration
| sort duration desc
| limit 10
```

**Resource usage analysis:**

```sql
fields @timestamp, job_id, cpu_percent, memory_mb
| stats avg(cpu_percent) as avg_cpu, max(memory_mb) as peak_memory by job_id
| sort avg_cpu desc
```

**Error analysis:**

```sql
fields @timestamp, @message
| filter @message like /ERROR|FATAL/
| sort @timestamp desc
| limit 50
```

### Cost Optimization

**CloudWatch Logs costs:**
- Data ingestion: $0.50 per GB
- Storage: $0.03 per GB/month
- Insights queries: $0.005 per GB scanned

**Estimated monthly costs:**
- 100 jobs/day × 1MB logs = ~3GB/month = ~$2/month
- 1000 jobs/day × 1MB logs = ~30GB/month = ~$16/month

**Reduce costs:**

1. **Adjust retention periods:**
   ```bash
   aws logs put-retention-policy \
     --log-group-name /joblet/job-logs \
     --retention-in-days 3
   ```

2. **Disable CloudWatch for dev/test:**
   ```bash
   export ENABLE_CLOUDWATCH="false"
   ```

3. **Use log filtering** to only send important logs

## Troubleshooting

### Installation Failures

**Symptom**: Instance launches but Joblet doesn't install

**Diagnosis:**

```bash
# SSH into instance
ssh -i ~/.ssh/my-key-pair.pem ubuntu@<PUBLIC_IP>

# Check installation log
cat /var/log/joblet-install.log

# Check cloud-init logs
cat /var/log/cloud-init-output.log
cat /var/log/cloud-init.log

# Check for errors
grep -i error /var/log/joblet-install.log
grep -i failed /var/log/joblet-install.log
```

**Common Issues:**

1. **Network timeout downloading packages**
   - **Cause**: Instance has no internet access
   - **Solution**: Ensure subnet has internet gateway or NAT gateway attached
   ```bash
   # Check internet connectivity
   ping -c 3 8.8.8.8
   curl -I https://github.com
   ```

2. **IAM role permissions insufficient**
   - **Cause**: Missing CloudWatch permissions
   - **Solution**: Verify IAM role is attached and has correct permissions
   ```bash
   # Check instance profile
   aws ec2 describe-instances \
     --instance-ids i-xxxxxxxxx \
     --query 'Reservations[0].Instances[0].IamInstanceProfile'
   ```

3. **Port already in use**
   - **Cause**: Another service using port 443 (or your configured port)
   - **Solution**: Choose a different port via `JOBLET_SERVER_PORT` (e.g., 8443, 9443, 50051)
   ```bash
   # Check if port 443 is already in use
   sudo ss -tlnp | grep 443

   # If port 443 is taken, use alternative port
   export JOBLET_SERVER_PORT="8443"
   ```

4. **Disk space insufficient**
   - **Cause**: Root volume too small
   - **Solution**: Use 30GB or larger root volume

### Service Won't Start

**Diagnosis:**

```bash
# Check service status
sudo systemctl status joblet -l

# View detailed logs
sudo journalctl -u joblet -n 100 --no-pager

# Check for common issues
sudo systemctl status joblet | grep -i error
```

**Common Issues:**

1. **Certificate issues**
   ```bash
   # Verify certificates exist
   ls -la /opt/joblet/config/

   # Regenerate if needed
   sudo /usr/local/bin/certs_gen_embedded.sh
   sudo systemctl restart joblet
   ```

2. **Cgroup delegation issues**
   ```bash
   # Check cgroup controllers
   cat /sys/fs/cgroup/joblet.slice/cgroup.controllers

   # Should show: cpuset cpu io memory pids
   ```

3. **Network bridge issues**
   ```bash
   # Check bridge exists
   ip link show joblet0

   # Check IP forwarding
   cat /proc/sys/net/ipv4/ip_forward  # Should be 1

   # Recreate bridge if needed
   sudo systemctl restart joblet
   ```

### Connection Refused from Clients

**Symptom**: `rnx` cannot connect to server

**Diagnosis:**

```bash
# Check server is listening (port 443 if using recommended EC2 setup)
ssh -i ~/.ssh/my-key-pair.pem ubuntu@<PUBLIC_IP> "sudo ss -tlnp | grep 443"

# Or check your custom port
ssh -i ~/.ssh/my-key-pair.pem ubuntu@<PUBLIC_IP> "sudo ss -tlnp | grep LISTEN"

# Test from server itself
ssh -i ~/.ssh/my-key-pair.pem ubuntu@<PUBLIC_IP> "sudo rnx job list"
```

**Solutions:**

1. **Security group not allowing traffic**
   ```bash
   # Add your IP to security group (port 443 for recommended setup)
   aws ec2 authorize-security-group-ingress \
     --group-id sg-xxxxxxxxx \
     --protocol tcp \
     --port 443 \
     --cidr YOUR_IP/32

   # Or for custom port
   aws ec2 authorize-security-group-ingress \
     --group-id sg-xxxxxxxxx \
     --protocol tcp \
     --port YOUR_PORT \
     --cidr YOUR_IP/32
   ```

2. **Wrong IP in client config**
   ```bash
   # Download fresh config
   scp -i ~/.ssh/my-key-pair.pem \
     ubuntu@<PUBLIC_IP>:/opt/joblet/config/rnx-config.yml ~/.rnx/
   ```

3. **Certificate mismatch**
   ```bash
   # Regenerate certificates with correct IP/domain
   ssh -i ~/.ssh/my-key-pair.pem ubuntu@<PUBLIC_IP>
   sudo /usr/local/bin/certs_gen_embedded.sh
   sudo systemctl restart joblet

   # Download new config
   scp -i ~/.ssh/my-key-pair.pem \
     ubuntu@<PUBLIC_IP>:/opt/joblet/config/rnx-config.yml ~/.rnx/
   ```

### CloudWatch Logs Not Working

**Diagnosis:**

```bash
# Check if IAM role is attached
aws ec2 describe-instances \
  --instance-ids i-xxxxxxxxx \
  --query 'Reservations[0].Instances[0].IamInstanceProfile'

# Check CloudWatch Logs configuration
ssh -i ~/.ssh/my-key-pair.pem ubuntu@<PUBLIC_IP>
grep -A 10 "persist:" /opt/joblet/config/joblet-config.yml

# Check for CloudWatch errors in logs
sudo journalctl -u joblet | grep -i cloudwatch
```

**Solutions:**

1. **IAM role not attached**
   ```bash
   # Attach IAM role to running instance
   aws ec2 associate-iam-instance-profile \
     --instance-id i-xxxxxxxxx \
     --iam-instance-profile Name=JobletCloudWatchRole

   # Restart Joblet
   ssh -i ~/.ssh/my-key-pair.pem ubuntu@<PUBLIC_IP> \
     "sudo systemctl restart joblet"
   ```

2. **IAM permissions insufficient**
   - Verify policy includes `logs:CreateLogGroup`, `logs:CreateLogStream`, `logs:PutLogEvents`

3. **Wrong region configuration**
   - CloudWatch auto-detects region from EC2 metadata
   - Verify instance metadata is accessible:
   ```bash
   curl http://169.254.169.254/latest/meta-data/placement/region
   ```

## Maintenance

### Updating Joblet

**Update to new version:**

```bash
# SSH into instance
ssh -i ~/.ssh/my-key-pair.pem ubuntu@<PUBLIC_IP>

# For Ubuntu/Debian
wget https://github.com/ehsaniara/joblet/releases/download/v1.2.0/joblet_1.2.0_amd64.deb
sudo dpkg -i joblet_1.2.0_amd64.deb
sudo systemctl restart joblet

# For Amazon Linux/RHEL
wget https://github.com/ehsaniara/joblet/releases/download/v1.2.0/joblet-1.2.0-1.x86_64.rpm
sudo yum localinstall -y joblet-1.2.0-1.x86_64.rpm
sudo systemctl restart joblet

# Verify new version
rnx --version
```

### Backup and Restore

**Create backup:**

```bash
# Backup configuration and data
ssh -i ~/.ssh/my-key-pair.pem ubuntu@<PUBLIC_IP> \
  "sudo tar czf /tmp/joblet-backup-$(date +%Y%m%d).tar.gz \
    /opt/joblet/config \
    /opt/joblet/volumes \
    /opt/joblet/logs"

# Download backup
scp -i ~/.ssh/my-key-pair.pem \
  ubuntu@<PUBLIC_IP>:/tmp/joblet-backup-*.tar.gz ./
```

**Restore to new instance:**

```bash
# Upload backup
scp -i ~/.ssh/my-key-pair.pem \
  joblet-backup-*.tar.gz ubuntu@<NEW_IP>:/tmp/

# Restore
ssh -i ~/.ssh/my-key-pair.pem ubuntu@<NEW_IP> \
  "sudo systemctl stop joblet && \
   sudo tar xzf /tmp/joblet-backup-*.tar.gz -C / && \
   sudo systemctl start joblet"
```

**EBS Snapshots (recommended for production):**

```bash
# Find volume ID
VOLUME_ID=$(aws ec2 describe-instances \
  --instance-ids i-xxxxxxxxx \
  --query 'Reservations[0].Instances[0].BlockDeviceMappings[0].Ebs.VolumeId' \
  --output text)

# Create snapshot
SNAPSHOT_ID=$(aws ec2 create-snapshot \
  --volume-id $VOLUME_ID \
  --description "Joblet backup $(date +%Y-%m-%d)" \
  --tag-specifications 'ResourceType=snapshot,Tags=[{Key=Name,Value=joblet-backup}]' \
  --query 'SnapshotId' \
  --output text)

echo "Snapshot created: $SNAPSHOT_ID"
```

### Certificate Rotation

**Regenerate certificates:**

```bash
# SSH into instance
ssh -i ~/.ssh/my-key-pair.pem ubuntu@<PUBLIC_IP>

# Regenerate certificates
sudo /usr/local/bin/certs_gen_embedded.sh

# Restart Joblet
sudo systemctl restart joblet

# Download new client config
exit
scp -i ~/.ssh/my-key-pair.pem \
  ubuntu@<PUBLIC_IP>:/opt/joblet/config/rnx-config.yml ~/.rnx/

# Test connection
rnx job list
```

## Cost Optimization

### Instance Sizing

Choose instance size based on workload:

| Workload | Instance Type | vCPUs | RAM | Cost/month (us-east-1)* |
|----------|---------------|-------|-----|------------------------|
| Development | t3.small | 2 | 2GB | ~$15 |
| Small Production | t3.medium | 2 | 4GB | ~$30 |
| Medium Production | t3.large | 2 | 8GB | ~$60 |
| Large Production | t3.xlarge | 4 | 16GB | ~$120 |

*Approximate On-Demand pricing, subject to change

### Cost Saving Strategies

1. **Reserved Instances (1-year or 3-year commitment)**
   - Save up to 72% vs On-Demand
   - Best for stable production workloads

2. **Savings Plans**
   - More flexible than Reserved Instances
   - Save up to 66% vs On-Demand

3. **Auto-shutdown for development**
   ```bash
   # Stop instance during off-hours (saves ~50% for dev)
   aws ec2 stop-instances --instance-ids i-xxxxxxxxx

   # Start when needed
   aws ec2 start-instances --instance-ids i-xxxxxxxxx
   ```

4. **Right-sizing**
   - Monitor actual usage with CloudWatch
   - Downgrade if underutilized

5. **gp3 EBS volumes**
   - Already configured in examples
   - ~20% cheaper than gp2

## Security Best Practices

### Network Security

1. **Use private subnets for production**
   ```bash
   # Access via bastion host or VPN
   # Don't expose Joblet directly to internet
   ```

2. **Implement security group layering**
   ```bash
   # Separate SG for SSH vs Joblet traffic
   # Reference other security groups instead of CIDR
   ```

3. **Enable VPC Flow Logs**
   ```bash
   aws ec2 create-flow-logs \
     --resource-type VPC \
     --resource-ids vpc-xxxxxxxxx \
     --traffic-type ALL \
     --log-destination-type cloud-watch-logs \
     --log-group-name /aws/vpc/flowlogs
   ```

### IAM Best Practices

1. **Use least-privilege policies**
   - Only grant necessary CloudWatch permissions
   - Scope resources to `/joblet/*` log groups

2. **Enable CloudTrail**
   - Monitor API calls to Joblet instances
   - Audit IAM role usage

3. **Rotate credentials**
   - Regularly rotate SSH keys
   - Regenerate TLS certificates periodically

### Monitoring and Alerting

**CloudWatch Alarms:**

```bash
# CPU alarm
aws cloudwatch put-metric-alarm \
  --alarm-name joblet-high-cpu \
  --alarm-description "Joblet CPU > 80%" \
  --metric-name CPUUtilization \
  --namespace AWS/EC2 \
  --statistic Average \
  --period 300 \
  --threshold 80 \
  --comparison-operator GreaterThanThreshold \
  --evaluation-periods 2 \
  --dimensions Name=InstanceId,Value=i-xxxxxxxxx

# Status check alarm
aws cloudwatch put-metric-alarm \
  --alarm-name joblet-status-check \
  --alarm-description "Instance status check failed" \
  --metric-name StatusCheckFailed \
  --namespace AWS/EC2 \
  --statistic Maximum \
  --period 60 \
  --threshold 0 \
  --comparison-operator GreaterThanThreshold \
  --evaluation-periods 2 \
  --dimensions Name=InstanceId,Value=i-xxxxxxxxx
```

## Support and Resources

- **GitHub**: https://github.com/ehsaniara/joblet
- **Issues**: https://github.com/ehsaniara/joblet/issues
- **Installation Notes**:
  - Debian/Ubuntu: See `INSTALL_NOTES.md`
  - RHEL/CentOS/Amazon Linux: See `INSTALL_NOTES_RPM.md`
- **User Data Script**: `scripts/ec2-user-data.sh`

## Quick Reference

### Essential Commands

**Instance Management:**
```bash
# SSH into instance
ssh -i ~/.ssh/KEY.pem ubuntu@PUBLIC_IP

# Check installation log
tail -f /var/log/joblet-install.log

# Check service status
sudo systemctl status joblet

# View logs
sudo journalctl -u joblet -f
```

**Joblet Operations:**
```bash
# List jobs
rnx job list

# Run job
rnx job run echo "test"

# View job logs
rnx job log JOB_ID

# Check job status
rnx job status JOB_ID
```

**Network Troubleshooting:**
```bash
# Check if Joblet is listening (port 443 for EC2)
sudo ss -tlnp | grep 443

# Or check all listening ports
sudo ss -tlnp | grep LISTEN

# Check bridge network
ip link show joblet0

# Test from server
sudo rnx job list
```

**CloudWatch Logs:**
```bash
# List log groups
aws logs describe-log-groups --log-group-name-prefix /joblet

# Tail logs
aws logs tail /joblet/server --follow

# Query logs
aws logs start-query --log-group-name /joblet/server --start-time TIMESTAMP --end-time TIMESTAMP --query-string 'QUERY'
```
