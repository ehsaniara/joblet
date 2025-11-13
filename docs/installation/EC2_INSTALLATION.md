# Installing Joblet on AWS EC2: Complete Guide

*From zero to running distributed jobs in 30 minutes*

> **⚡ Quick Start**: For a faster, automated installation approach, see [AWS_DEPLOYMENT.md](../AWS_DEPLOYMENT.md) which
> uses EC2 user data scripts for one-command deployment with automatic certificate management, CloudWatch integration, and
> support for multi-instance deployments via AWS Secrets Manager.
>
> This guide provides a detailed manual walkthrough for educational purposes and custom setups.

---

## Overview

This guide walks you through setting up Joblet on AWS EC2, including:

- ✅ EC2 instance configuration
- ✅ IAM roles and permissions
- ✅ Security groups and networking
- ✅ Joblet server installation
- ✅ CloudWatch monitoring setup
- ✅ SSH tunnel for secure local access
- ✅ Client configuration on MacBook

**Time to complete:** ~30 minutes

**Prerequisites:**

- AWS Account with EC2 access
- MacBook with terminal access
- SSH key pair for EC2

---

## Architecture Overview

```
┌─────────────┐         SSH Tunnel          ┌─────────────────┐
│   MacBook   │◄────────(localhost:443)───┤   EC2 Instance  │
│   (Local)   │                              │  (Joblet Server)│
│             │                              │                 │
│  rnx CLI    │                              │  joblet daemon  │
└─────────────┘                              └────────┬────────┘
                                                      │
                                                      ▼
                                              ┌──────────────┐
                                              │  CloudWatch  │
                                              │  Logs/Metrics│
                                              └──────────────┘
```

---

## Part 1: EC2 Instance Setup

### 1.1 Launch EC2 Instance

**Instance Specifications:**

| Setting       | Recommendation               | Minimum       |
|---------------|------------------------------|---------------|
| Instance Type | `t3.large` (2 vCPU, 8GB RAM) | `t3.medium`   |
| OS            | Ubuntu 22.04 LTS             | Ubuntu 20.04+ |
| Storage       | 50GB gp3 SSD                 | 30GB          |
| Architecture  | x86_64                       | x86_64        |

**Launch Steps:**

1. Go to AWS Console → EC2 → Launch Instance

2. **Name and Tags:**
   ```
   Name: joblet-server-prod
   Environment: production
   Project: joblet
   ```

3. **Application and OS Images:**
    - Choose: Ubuntu Server 22.04 LTS (HVM), SSD Volume Type
    - Architecture: 64-bit (x86)

4. **Instance Type:**
    - Select: `t3.large` (for production)
    - Or: `t3.medium` (for testing)

5. **Key Pair:**
    - Create new or select existing key pair
    - Download and save as `~/.ssh/joblet-key.pem`
    - Set permissions:
      ```bash
      chmod 400 ~/.ssh/joblet-key.pem
      ```

6. **Network Settings:**
    - VPC: Default or your custom VPC
    - Auto-assign public IP: Enable
    - We'll configure security group in next step

7. **Storage:**
    - Size: 50 GB
    - Volume Type: gp3
    - Delete on termination: Uncheck (for data safety)

8. **Launch Instance**

---

### 1.2 Configure Security Group

Create a security group named `joblet-server-sg`:

**Inbound Rules:**

| Type | Protocol | Port | Source     | Purpose                      |
|------|----------|------|------------|------------------------------|
| SSH  | TCP      | 22   | Your IP/32 | SSH access from your MacBook |

**Important Security Notes:**

- ⚠️ **Never use `0.0.0.0/0`** for Joblet ports
- ✅ Use your MacBook's public IP or VPN IP
- ✅ For production, use VPN or AWS PrivateLink

**AWS Console Steps:**

1. EC2 → Security Groups → Create Security Group

2. **Basic Details:**
   ```
   Name: joblet-server-sg
   Description: Security group for Joblet server
   VPC: (select your VPC)
   ```

3. **Inbound Rules:**
   ```
   # SSH
   Type: SSH
   Port: 22
   Source: My IP (or specify your IP)
   Description: SSH from MacBook

   # That's it! We'll access everything via SSH tunnel
   # No need to expose Joblet or Admin UI ports directly
   ```

4. **Outbound Rules:**
    - Keep default (all traffic allowed)

5. Attach security group to your EC2 instance

---

## Part 2: IAM Configuration

### 2.1 Create IAM Role for EC2 Instance

This allows the EC2 instance to send logs to CloudWatch.

**AWS Console Steps:**

1. IAM → Roles → Create Role

2. **Trusted Entity:**
    - Select: AWS service
    - Use case: EC2

3. **Permissions:**
    - Search and add: `CloudWatchAgentServerPolicy`
    - Search and add: `AmazonSSMManagedInstanceCore` (optional, for Systems Manager)

4. **Role Details:**
   ```
   Role name: JobletServerRole
   Description: IAM role for Joblet EC2 instances
   ```

5. Create Role

### 2.2 Attach Role to EC2 Instance

1. EC2 → Instances → Select your instance
2. Actions → Security → Modify IAM Role
3. Select: `JobletServerRole`
4. Update IAM Role

### 2.3 Create IAM User for Local CLI (Optional)

If you want to manage Joblet via AWS CLI:

1. IAM → Users → Create User
   ```
   User name: joblet-admin
   Access type: Programmatic access
   ```

2. **Attach Policies:**
    - `AmazonEC2ReadOnlyAccess`
    - Custom policy for Joblet (see below)

3. **Custom Policy (Optional):**
   ```json
   {
     "Version": "2012-10-17",
     "Statement": [
       {
         "Effect": "Allow",
         "Action": [
           "ec2:DescribeInstances",
           "ec2:DescribeSecurityGroups",
           "cloudwatch:PutMetricData",
           "logs:CreateLogGroup",
           "logs:CreateLogStream",
           "logs:PutLogEvents"
         ],
         "Resource": "*"
       }
     ]
   }
   ```

4. Save access keys securely

---

## Part 3: Install Joblet on EC2

> **📖 Modern Approach Available**: This guide shows manual installation steps for educational purposes. For production
> deployments, we recommend using the automated EC2 user data script approach documented
> in [AWS_DEPLOYMENT.md](../AWS_DEPLOYMENT.md). The automated approach includes:
> - One-command installation via user data script
> - Automatic certificate generation with embedded certs (single instance) or AWS Secrets Manager (multi-instance)
> - CloudWatch Logs integration
> - DynamoDB state persistence
> - Proper systemd service setup
>
> For multi-instance or auto-scaling deployments on AWS, see:
> - [AWS Secrets Manager Integration](../SECRETS_MANAGER_INTEGRATION.md) - Shared certificates across instances
> - [Scaling on AWS](../SCALING_ON_AWS.md) - Auto-scaling group setup
> - [Certificate Management Comparison](../CERTIFICATE_MANAGEMENT_COMPARISON.md) - Choosing the right approach

### 3.1 Connect to EC2 Instance

From your MacBook:

```bash
# Get instance public DNS from AWS Console
# Should look like: ec2-18-123-45-67.compute-1.amazonaws.com

# SSH into instance
ssh -i ~/.ssh/joblet-key.pem ubuntu@ec2-18-123-45-67.compute-1.amazonaws.com
```

### 3.2 System Updates and Prerequisites

```bash
# Update system
sudo apt-get update
sudo apt-get upgrade -y

# Install prerequisites
sudo apt-get install -y \
    curl \
    wget \
    git \
    build-essential \
    ca-certificates \
    gnupg \
    lsb-release

# Install Docker (required for Joblet)
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh

# Add ubuntu user to docker group
sudo usermod -aG docker ubuntu

# Start Docker
sudo systemctl enable docker
sudo systemctl start docker

# Verify Docker
docker --version
```

**Log out and log back in for docker group changes to take effect:**

```bash
exit
ssh -i ~/.ssh/joblet-key.pem ubuntu@ec2-18-123-45-67.compute-1.amazonaws.com
```

### 3.3 Install Joblet Server

```bash
# Create Joblet directory
sudo mkdir -p /opt/joblet
sudo chown ubuntu:ubuntu /opt/joblet
cd /opt/joblet

# Download Joblet binary (replace VERSION with latest)
JOBLET_VERSION="v1.0.0"  # Check latest release
wget https://github.com/ehsaniara/joblet/releases/download/${JOBLET_VERSION}/joblet-linux-amd64

# Rename and make executable
mv joblet-linux-amd64 joblet
chmod +x joblet

# Verify installation
./joblet version
```

### 3.4 Generate TLS Certificates

> **💡 Note**: The automated installation script (`scripts/ec2-user-data.sh`) handles certificate generation
> automatically. For AWS deployments with multiple instances, it can use AWS Secrets Manager to share CA and client
> certificates across instances while generating unique server certificates per instance.
> See [AWS_DEPLOYMENT.md](../AWS_DEPLOYMENT.md) for details.

Joblet requires TLS for secure communication. Here's the manual approach:

```bash
# Create certs directory
mkdir -p /opt/joblet/certs
cd /opt/joblet/certs

# Generate CA certificate
openssl req -x509 -newkey rsa:4096 -nodes \
  -keyout ca-key.pem -out ca-cert.pem -days 365 \
  -subj "/C=US/ST=State/L=City/O=Joblet/CN=Joblet CA"

# Generate server certificate
openssl req -newkey rsa:4096 -nodes \
  -keyout server-key.pem -out server-req.pem \
  -subj "/C=US/ST=State/L=City/O=Joblet/CN=joblet-server"

# Sign server certificate
openssl x509 -req -in server-req.pem -days 365 \
  -CA ca-cert.pem -CAkey ca-key.pem -CAcreateserial \
  -out server-cert.pem

# Set permissions
chmod 600 *.pem
chmod 644 *-cert.pem

# List generated files
ls -la
```

You should see:

- `ca-cert.pem` (CA certificate)
- `server-cert.pem` (Server certificate)
- `server-key.pem` (Server private key)

### 3.5 Create Joblet Configuration

```bash
# Create config file
cat > /opt/joblet/joblet-config.yml << 'EOF'
# Joblet Server Configuration

server:
  host: "0.0.0.0"
  port: 443  # Using 443 (HTTPS) for firewall compatibility

tls:
  enabled: true
  cert_file: "/opt/joblet/certs/server-cert.pem"
  key_file: "/opt/joblet/certs/server-key.pem"
  ca_file: "/opt/joblet/certs/ca-cert.pem"

storage:
  type: "filesystem"
  path: "/opt/joblet/data"

volumes:
  base_path: "/opt/joblet/volumes"
  max_size: "100GB"

logs:
  level: "info"
  file: "/opt/joblet/logs/joblet.log"
  max_size: 100  # MB
  max_backups: 10
  max_age: 30    # days

metrics:
  enabled: true
  path: "/opt/joblet/metrics"
  interval: 10   # seconds

admin_ui:
  enabled: true
  port: 8080

cloudwatch:
  enabled: true
  region: "us-east-1"  # Change to your region
  log_group: "/aws/joblet/server"
  log_stream: "joblet-server"
EOF
```

### 3.6 Create Required Directories

```bash
sudo mkdir -p /opt/joblet/{data,volumes,logs,metrics}
sudo chown -R ubuntu:ubuntu /opt/joblet
```

### 3.7 Create Systemd Service

Create a systemd service for automatic startup:

```bash
sudo tee /etc/systemd/system/joblet.service > /dev/null << 'EOF'
[Unit]
Description=Joblet Distributed Job Orchestration System
After=network.target docker.service
Requires=docker.service

[Service]
Type=simple
User=ubuntu
Group=ubuntu
WorkingDirectory=/opt/joblet
ExecStart=/opt/joblet/joblet server --config /opt/joblet/joblet-config.yml
Restart=always
RestartSec=10
StandardOutput=append:/opt/joblet/logs/joblet-stdout.log
StandardError=append:/opt/joblet/logs/joblet-stderr.log

# Resource limits
LimitNOFILE=65535
LimitNPROC=4096

[Install]
WantedBy=multi-user.target
EOF
```

### 3.8 Start Joblet Service

```bash
# Reload systemd
sudo systemctl daemon-reload

# Enable Joblet to start on boot
sudo systemctl enable joblet

# Start Joblet
sudo systemctl start joblet

# Check status
sudo systemctl status joblet

# View logs
sudo journalctl -u joblet -f
```

---

## Part 4: CloudWatch Configuration

### 4.1 Install CloudWatch Agent

```bash
# Download CloudWatch agent
wget https://s3.amazonaws.com/amazoncloudwatch-agent/ubuntu/amd64/latest/amazon-cloudwatch-agent.deb

# Install
sudo dpkg -i amazon-cloudwatch-agent.deb

# Verify installation
/opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl \
  -a query -m ec2 -c default -s
```

### 4.2 Configure CloudWatch Agent

```bash
sudo tee /opt/aws/amazon-cloudwatch-agent/etc/cloudwatch-config.json > /dev/null << 'EOF'
{
  "agent": {
    "metrics_collection_interval": 60,
    "run_as_user": "ubuntu"
  },
  "logs": {
    "logs_collected": {
      "files": {
        "collect_list": [
          {
            "file_path": "/opt/joblet/logs/joblet.log",
            "log_group_name": "/aws/joblet/server",
            "log_stream_name": "{instance_id}/joblet",
            "retention_in_days": 7
          },
          {
            "file_path": "/opt/joblet/logs/joblet-stdout.log",
            "log_group_name": "/aws/joblet/server",
            "log_stream_name": "{instance_id}/stdout",
            "retention_in_days": 7
          },
          {
            "file_path": "/opt/joblet/logs/joblet-stderr.log",
            "log_group_name": "/aws/joblet/server",
            "log_stream_name": "{instance_id}/stderr",
            "retention_in_days": 7
          }
        ]
      }
    }
  },
  "metrics": {
    "namespace": "Joblet/Server",
    "metrics_collected": {
      "cpu": {
        "measurement": [
          {
            "name": "cpu_usage_idle",
            "rename": "CPU_IDLE",
            "unit": "Percent"
          }
        ],
        "metrics_collection_interval": 60,
        "totalcpu": false
      },
      "disk": {
        "measurement": [
          {
            "name": "used_percent",
            "rename": "DISK_USED",
            "unit": "Percent"
          }
        ],
        "metrics_collection_interval": 60,
        "resources": [
          "/opt/joblet"
        ]
      },
      "diskio": {
        "measurement": [
          {
            "name": "io_time"
          }
        ],
        "metrics_collection_interval": 60
      },
      "mem": {
        "measurement": [
          {
            "name": "mem_used_percent",
            "rename": "MEMORY_USED",
            "unit": "Percent"
          }
        ],
        "metrics_collection_interval": 60
      },
      "netstat": {
        "measurement": [
          "tcp_established",
          "tcp_time_wait"
        ],
        "metrics_collection_interval": 60
      }
    }
  }
}
EOF
```

### 4.3 Start CloudWatch Agent

```bash
# Start CloudWatch agent
sudo /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl \
  -a fetch-config \
  -m ec2 \
  -s \
  -c file:/opt/aws/amazon-cloudwatch-agent/etc/cloudwatch-config.json

# Verify it's running
sudo /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl \
  -a query -m ec2 -c default -s
```

### 4.4 Verify CloudWatch Logs

1. Go to AWS Console → CloudWatch → Log Groups
2. Find `/aws/joblet/server`
3. You should see log streams with your instance ID
4. Click on a stream to view logs

---

## Part 5: SSH Tunnel Setup (MacBook)

### 5.1 Why SSH Tunnel?

SSH tunnel provides:

- ✅ Secure encrypted connection
- ✅ No need to expose ports publicly
- ✅ Works from anywhere (coffee shop, home, etc.)
- ✅ Free alternative to VPN

### 5.2 Create SSH Tunnel Script

On your **MacBook**, create a helper script:

```bash
# Create scripts directory
mkdir -p ~/bin

# Create tunnel script
cat > ~/bin/joblet-tunnel.sh << 'EOF'
#!/bin/bash

# Configuration
EC2_HOST="ec2-18-123-45-67.compute-1.amazonaws.com"  # Change this
SSH_KEY="$HOME/.ssh/joblet-key.pem"

echo "🚇 Starting SSH tunnel to Joblet server..."
echo "   Joblet API: localhost:8443"
echo "   Admin UI:   localhost:8080"
echo ""
echo "Press Ctrl+C to stop the tunnel"
echo "─────────────────────────────────────────"

# Start SSH tunnel - forwards both Joblet API and Admin UI
ssh -i "${SSH_KEY}" \
    -N \
    -L 8443:localhost:443 \
    -L 8080:localhost:8080 \
    ubuntu@${EC2_HOST}

# Note: Using 8443 locally for API (443 would need sudo)
# Admin UI on 8080 doesn't need sudo
EOF

# Make executable
chmod +x ~/bin/joblet-tunnel.sh
```

**Edit the script** and change:

- `EC2_HOST` to your instance's public DNS
- `SSH_KEY` path if different

### 5.3 Start the Tunnel

```bash
# Start tunnel (keeps running)
~/bin/joblet-tunnel.sh
```

You should see:

```
🚇 Starting SSH tunnel to Joblet server...
   Local:  localhost:443
   Remote: ec2-18-123-45-67.compute-1.amazonaws.com:443

Press Ctrl+C to stop the tunnel
─────────────────────────────────────────
```

**Leave this terminal open** while using Joblet.

### 5.4 (Optional) Auto-reconnect Tunnel

For production use, create an auto-reconnecting tunnel:

```bash
cat > ~/bin/joblet-tunnel-persistent.sh << 'EOF'
#!/bin/bash

EC2_HOST="ec2-18-123-45-67.compute-1.amazonaws.com"
SSH_KEY="$HOME/.ssh/joblet-key.pem"

while true; do
    echo "$(date): Connecting tunnel..."
    ssh -i "${SSH_KEY}" \
        -N \
        -L 443:localhost:443 \
        -o ServerAliveInterval=30 \
        -o ServerAliveCountMax=3 \
        -o ExitOnForwardFailure=yes \
        ubuntu@${EC2_HOST}

    echo "$(date): Tunnel disconnected. Reconnecting in 5s..."
    sleep 5
done
EOF

chmod +x ~/bin/joblet-tunnel-persistent.sh
```

---

## Part 6: Client Setup (MacBook)

### 6.1 Download Joblet CLI

On your **MacBook**:

```bash
# Download rnx CLI for macOS
JOBLET_VERSION="v1.0.0"  # Check latest release
cd ~/Downloads

# For M1/M2 Macs (ARM)
wget https://github.com/ehsaniara/joblet/releases/download/${JOBLET_VERSION}/rnx-darwin-arm64

# For Intel Macs
wget https://github.com/ehsaniara/joblet/releases/download/${JOBLET_VERSION}/rnx-darwin-amd64

# Rename and move to PATH
# For ARM:
mv rnx-darwin-arm64 rnx
# For Intel:
# mv rnx-darwin-amd64 rnx

chmod +x rnx
sudo mv rnx /usr/local/bin/

# Verify
rnx version
```

### 6.2 Copy TLS Certificates from EC2

```bash
# Create certs directory on MacBook
mkdir -p ~/.rnx/certs

# Copy CA certificate from EC2
scp -i ~/.ssh/joblet-key.pem \
    ubuntu@ec2-18-123-45-67.compute-1.amazonaws.com:/opt/joblet/certs/ca-cert.pem \
    ~/.rnx/certs/

# Verify
ls -la ~/.rnx/certs/
```

### 6.3 Configure Joblet CLI

Create configuration file:

```bash
mkdir -p ~/.rnx

cat > ~/.rnx/rnx-config.yml << 'EOF'
# Joblet Client Configuration

default:
  # Connect via SSH tunnel to localhost
  # Use 8443 if your tunnel script uses LOCAL_PORT="8443"
  server: "localhost:8443"

  tls:
    enabled: true
    ca_cert: "~/.rnx/certs/ca-cert.pem"
    skip_verify: false

  # Timeouts
  timeout: 30s

  # Output format (json or text)
  output_format: "text"

# Optional: Direct connection (when on VPN or same network)
# direct:
#   server: "ec2-18-123-45-67.compute-1.amazonaws.com:443"
#   tls:
#     enabled: true
#     ca_cert: "~/.rnx/certs/ca-cert.pem"
#     skip_verify: false
EOF
```

---

## Part 7: Testing and Verification

### 7.1 Test Connection

Open **two terminal windows** on your MacBook:

**Terminal 1** (SSH Tunnel):

```bash
~/bin/joblet-tunnel.sh
```

**Terminal 2** (Joblet CLI):

```bash
# Test connection
rnx version

# List nodes (should show your EC2 instance)
rnx nodes list

# Check server status
rnx job list
```

**Expected Output:**

```bash
$ rnx version
rnx version: v1.0.0
Server version: v1.0.0

$ rnx nodes list
NODE ID                                  STATUS   HOSTNAME        CPU    MEMORY    JOBS
─────────────────────────────────────────────────────────────────────────────────────────
abc123...                                ONLINE   ip-172-31-1-1   2/2    6.5/8GB   0/10
```

### 7.2 Run Test Job

```bash
# Simple test job
rnx job run echo "Hello from Joblet on EC2!"

# Check job status
rnx job list

# View job logs
rnx job log <job-id>
```

### 7.3 Verify CloudWatch Logs

1. AWS Console → CloudWatch → Log Groups
2. Click on `/aws/joblet/server`
3. You should see recent log entries from the test job

### 7.4 Access Admin UI (Optional)

The Admin UI is accessible via SSH tunnel - no need to expose port 8080:

```bash
# In addition to your main tunnel, add port 8080 forwarding
ssh -i ~/.ssh/joblet-key.pem \
    -L 8443:localhost:443 \
    -L 8080:localhost:8080 \
    ubuntu@ec2-18-123-45-67.compute-1.amazonaws.com
```

Then open in your browser: http://localhost:8080

**Note:** Everything stays secure - no ports exposed to the internet except SSH.

---

## Part 8: Production Checklist

### Security

- [ ] Security group restricts access to your IP only
- [ ] SSH key is stored securely (chmod 400)
- [ ] TLS is enabled for Joblet
- [ ] IAM role has minimum required permissions
- [ ] Admin UI is protected or disabled

### Monitoring

- [ ] CloudWatch agent is running
- [ ] Logs are flowing to CloudWatch
- [ ] Metrics are being collected
- [ ] Set up CloudWatch alarms for:
    - High CPU usage
    - High memory usage
    - Disk space usage
    - Joblet service down

### Backup

- [ ] EBS volume snapshots enabled
- [ ] Backup `/opt/joblet/data`
- [ ] Backup `/opt/joblet/volumes`
- [ ] Backup configuration files

### High Availability

- [ ] Consider multiple EC2 instances
- [ ] Use Auto Scaling Group
- [ ] Set up Load Balancer
- [ ] Use EFS for shared volumes (multi-node)

---

## Troubleshooting

### Can't Connect to EC2

```bash
# Check security group allows your IP
# Check SSH key permissions
chmod 400 ~/.ssh/joblet-key.pem

# Test SSH connection
ssh -v -i ~/.ssh/joblet-key.pem ubuntu@<ec2-host>
```

### Joblet Service Won't Start

```bash
# Check logs
sudo journalctl -u joblet -n 50

# Check configuration
cat /opt/joblet/joblet-config.yml

# Check ports
sudo netstat -tlnp | grep 443

# Restart service
sudo systemctl restart joblet
```

### SSH Tunnel Issues

```bash
# Check if tunnel is running
ps aux | grep ssh

# Kill existing tunnels
pkill -f "ssh.*443"

# Restart tunnel
~/bin/joblet-tunnel.sh
```

### TLS Certificate Errors

```bash
# Verify certificates exist
ls -la ~/.rnx/certs/

# Check certificate validity
openssl x509 -in ~/.rnx/certs/ca-cert.pem -text -noout

# Regenerate if needed (on EC2)
cd /opt/joblet/certs
# Re-run certificate generation commands
```

### CloudWatch Logs Not Appearing

```bash
# Check CloudWatch agent status
sudo /opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl \
  -a query -m ec2 -c default -s

# Check IAM role is attached
aws ec2 describe-instances --instance-ids <instance-id> \
  --query 'Reservations[0].Instances[0].IamInstanceProfile'

# Restart CloudWatch agent
sudo systemctl restart amazon-cloudwatch-agent
```

---

## Cost Optimization

### EC2 Instance

- **t3.medium**: ~$30/month (24/7)
- **t3.large**: ~$60/month (24/7)

**Savings:**

- Use Reserved Instances (1-3 year): Save up to 60%
- Use Spot Instances (non-prod): Save up to 90%
- Stop instance when not in use

### Storage

- **50GB EBS gp3**: ~$4/month
- **CloudWatch Logs**: ~$0.50/GB ingested

### Data Transfer

- Inbound: Free
- Outbound: First 100GB/month free, then $0.09/GB

**Monthly Total**: ~$35-65 for small workloads

---

## Next Steps

Now that Joblet is running on EC2:

1. ✅ **Install Runtimes** - Get Python, Java, etc.
   ```bash
   rnx runtime install python-3.11-ml
   ```

2. ✅ **Create Volumes** - For data sharing
   ```bash
   rnx volume create my-data --size=10GB
   ```

3. ✅ **Run the ML Demo** - See Joblet in action
    - [Continue to ML Demo Tutorial →](../../examples/ml-demo/README.md)

4. ✅ **Build Your Workflows** - Start with simple jobs
   ```bash
   rnx job run echo "My first job!"
   ```

---

## Summary

You've successfully:

✅ Launched and configured an EC2 instance
✅ Set up IAM roles and security groups
✅ Installed Joblet server with TLS
✅ Configured CloudWatch monitoring
✅ Set up SSH tunnel for secure access
✅ Configured Joblet CLI on MacBook
✅ Verified everything works

**Your Joblet cluster is ready for production workloads!** 🚀

---

## Additional Resources

- [Joblet Documentation](https://github.com/ehsaniara/joblet)
- [AWS EC2 Best Practices](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-best-practices.html)
- [CloudWatch Agent Guide](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/Install-CloudWatch-Agent.html)
- [SSH Tunneling Guide](https://www.ssh.com/academy/ssh/tunneling)

---

*Have questions? Open an issue on [GitHub](https://github.com/ehsaniara/joblet/issues)*
