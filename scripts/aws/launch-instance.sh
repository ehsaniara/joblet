#!/bin/bash
#
# Joblet AWS EC2 Instance Launch Script
# Launches EC2 instance with Joblet auto-installation via user data
#
# Usage: ./launch-instance.sh [OPTIONS]
# Or: curl -fsSL https://raw.githubusercontent.com/ehsaniara/joblet/main/scripts/aws/launch-instance.sh | bash
#
# Environment variables:
#   KEY_NAME          - SSH key pair name (required)
#   SG_ID             - Security group ID from setup-security-group.sh (optional, will prompt)
#   REGION            - AWS region (default: us-east-1)
#   INSTANCE_TYPE     - Instance type (default: t3.medium)
#   ENABLE_CLOUDWATCH - Enable CloudWatch Logs (default: true)
#

set -e

echo "=========================================================================="
echo "Joblet AWS EC2 Instance Launch"
echo "=========================================================================="
echo ""

# Check AWS CLI
if ! command -v aws >/dev/null 2>&1; then
    echo "❌ Error: AWS CLI not found"
    echo "Please install: https://aws.amazon.com/cli/"
    exit 1
fi

# Check AWS credentials
if ! aws sts get-caller-identity >/dev/null 2>&1; then
    echo "❌ Error: AWS credentials not configured"
    echo "Please run: aws configure"
    exit 1
fi

# Get configuration from environment or defaults
REGION="${REGION:-us-east-1}"
INSTANCE_TYPE="${INSTANCE_TYPE:-t3.medium}"
ENABLE_CLOUDWATCH="${ENABLE_CLOUDWATCH:-true}"

# Get KEY_NAME if not set
if [ -z "$KEY_NAME" ]; then
    echo "Available SSH key pairs in $REGION:"
    aws ec2 describe-key-pairs --region "$REGION" --query 'KeyPairs[*].KeyName' --output table
    echo ""
    read -p "Enter SSH key pair name: " KEY_NAME
    if [ -z "$KEY_NAME" ]; then
        echo "❌ Error: SSH key pair name is required"
        exit 1
    fi
fi

# Get SG_ID if not set
if [ -z "$SG_ID" ]; then
    echo ""
    echo "Available security groups:"
    aws ec2 describe-security-groups \
      --filters "Name=group-name,Values=joblet-server-sg" \
      --query 'SecurityGroups[*].[GroupId,GroupName,VpcId]' \
      --output table \
      --region "$REGION"
    echo ""
    read -p "Enter security group ID (or press Enter to search): " SG_ID

    if [ -z "$SG_ID" ]; then
        # Try to find joblet-server-sg
        SG_ID=$(aws ec2 describe-security-groups \
          --filters "Name=group-name,Values=joblet-server-sg" \
          --query 'SecurityGroups[0].GroupId' \
          --output text \
          --region "$REGION" 2>/dev/null || echo "")

        if [ -z "$SG_ID" ] || [ "$SG_ID" = "None" ]; then
            echo "❌ Error: Security group not found"
            echo "Please run: ./setup-security-group.sh"
            exit 1
        fi
        echo "Found security group: $SG_ID"
    fi
fi

echo ""
echo "Configuration:"
echo "  Region:        $REGION"
echo "  Instance Type: $INSTANCE_TYPE"
echo "  SSH Key:       $KEY_NAME"
echo "  Security Group: $SG_ID"
echo "  CloudWatch:    $ENABLE_CLOUDWATCH"
echo ""

# Find latest Ubuntu 22.04 AMI
echo "Finding latest Ubuntu 22.04 AMI..."
AMI_ID=$(aws ec2 describe-images \
  --owners 099720109477 \
  --filters "Name=name,Values=ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-amd64-server-*" \
            "Name=state,Values=available" \
  --query 'Images | sort_by(@, &CreationDate) | [-1].ImageId' \
  --output text \
  --region "$REGION")

if [ -z "$AMI_ID" ] || [ "$AMI_ID" = "None" ]; then
    echo "❌ Error: Could not find Ubuntu 22.04 AMI"
    exit 1
fi

echo "Using AMI: $AMI_ID"

# Create user data script
echo "Creating user data script..."
cat > /tmp/joblet-user-data.sh << EOF
#!/bin/bash
set -e

# ============================================================================
# Joblet EC2 Bootstrap - CloudWatch and DynamoDB Integration
# ============================================================================

# Joblet configuration
export JOBLET_VERSION="latest"
export JOBLET_SERVER_PORT="443"
export ENABLE_CLOUDWATCH="$ENABLE_CLOUDWATCH"

# Download and run installation script
curl -fsSL https://raw.githubusercontent.com/ehsaniara/joblet/main/scripts/ec2-user-data.sh -o /tmp/joblet-install.sh
chmod +x /tmp/joblet-install.sh
/tmp/joblet-install.sh 2>&1 | tee /var/log/joblet-install.log

echo ""
echo "=============================================================================="
echo "✅ Joblet Installation Complete"
echo "=============================================================================="
echo "Enabled features:"
echo "  ✅ CloudWatch Logs: /joblet (log group)"
echo "  ✅ DynamoDB State: joblet-jobs (auto-created table)"
echo "  ✅ Certificates: Embedded (auto-generated)"
echo "  ✅ gRPC Server: Port 443"
echo ""
echo "Next steps:"
echo "  1. Download client config: scp ubuntu@<IP>:/opt/joblet/config/rnx-config.yml ~/.rnx/"
echo "  2. Test connection: rnx job list"
echo "=============================================================================="
EOF

# Launch instance
echo ""
echo "Launching EC2 instance..."
INSTANCE_ID=$(aws ec2 run-instances \
  --image-id "$AMI_ID" \
  --instance-type "$INSTANCE_TYPE" \
  --key-name "$KEY_NAME" \
  --security-group-ids "$SG_ID" \
  --iam-instance-profile Name=JobletEC2Role \
  --user-data file:///tmp/joblet-user-data.sh \
  --block-device-mappings '[{"DeviceName":"/dev/sda1","Ebs":{"VolumeSize":30,"VolumeType":"gp3","DeleteOnTermination":false}}]' \
  --tag-specifications 'ResourceType=instance,Tags=[{Key=Name,Value=joblet-server},{Key=Application,Value=Joblet}]' \
  --query 'Instances[0].InstanceId' \
  --output text \
  --region "$REGION")

if [ -z "$INSTANCE_ID" ]; then
    echo "❌ Error: Failed to launch instance"
    rm /tmp/joblet-user-data.sh
    exit 1
fi

echo "✅ Instance launched: $INSTANCE_ID"
rm /tmp/joblet-user-data.sh

echo ""
echo "Waiting for instance to be running..."
aws ec2 wait instance-running --instance-ids "$INSTANCE_ID" --region "$REGION"

# Get instance details
INSTANCE_INFO=$(aws ec2 describe-instances \
  --instance-ids "$INSTANCE_ID" \
  --query 'Reservations[0].Instances[0].[PublicIpAddress,PrivateIpAddress,PublicDnsName]' \
  --output text \
  --region "$REGION")

PUBLIC_IP=$(echo "$INSTANCE_INFO" | cut -f1)
PRIVATE_IP=$(echo "$INSTANCE_INFO" | cut -f2)
PUBLIC_DNS=$(echo "$INSTANCE_INFO" | cut -f3)

echo ""
echo "=========================================================================="
echo "✅ Joblet EC2 Instance Ready"
echo "=========================================================================="
echo ""
echo "Instance Details:"
echo "  Instance ID:  $INSTANCE_ID"
echo "  Public IP:    $PUBLIC_IP"
echo "  Private IP:   $PRIVATE_IP"
echo "  Public DNS:   $PUBLIC_DNS"
echo "  Region:       $REGION"
echo ""
echo "SSH Access:"
echo "  ssh -i ~/.ssh/${KEY_NAME}.pem ubuntu@${PUBLIC_IP}"
echo ""
echo "Installation Status:"
echo "  Installation is running (takes ~5 minutes)..."
echo "  Monitor progress:"
echo "    ssh -i ~/.ssh/${KEY_NAME}.pem ubuntu@${PUBLIC_IP}"
echo "    tail -f /var/log/joblet-install.log"
echo ""
echo "When installation completes:"
echo ""
echo "  1. Download client configuration:"
echo "     scp -i ~/.ssh/${KEY_NAME}.pem ubuntu@${PUBLIC_IP}:/opt/joblet/config/rnx-config.yml ~/.rnx/"
echo ""
echo "  2. Test connection:"
echo "     rnx job list"
echo ""
echo "  3. Run your first job:"
echo "     rnx job run echo 'Hello from Joblet on AWS!'"
echo ""
echo "CloudWatch Logs:"
echo "  Log Group: /joblet"
echo "  Region: $REGION"
echo "  View logs: https://console.aws.amazon.com/cloudwatch/home?region=${REGION}#logsV2:log-groups/log-group//joblet"
echo ""
echo "DynamoDB:"
echo "  Table: joblet-jobs"
echo "  Region: $REGION"
echo "  View table: https://console.aws.amazon.com/dynamodbv2/home?region=${REGION}#table?name=joblet-jobs"
echo ""
echo "=========================================================================="
