#!/bin/bash
#
# Joblet AWS Security Group Setup Script
# Creates security group with SSH and gRPC access
#
# Usage: ./setup-security-group.sh [YOUR_IP/32]
# Or: curl -fsSL https://raw.githubusercontent.com/ehsaniara/joblet/main/scripts/aws/setup-security-group.sh | bash -s YOUR_IP/32
#
# Example: ./setup-security-group.sh 203.0.113.45/32
#

set -e

echo "=========================================================================="
echo "Joblet AWS Security Group Setup"
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

# Get IP address from argument or auto-detect
if [ -n "$1" ]; then
    MY_IP="$1"
    echo "Using provided IP: $MY_IP"
else
    echo "Auto-detecting your public IP address..."
    MY_IP=$(curl -s https://checkip.amazonaws.com)
    if [ -n "$MY_IP" ]; then
        MY_IP="${MY_IP}/32"
        echo "Detected IP: $MY_IP"
    else
        echo ""
        echo "❌ Error: Could not auto-detect IP address"
        echo ""
        echo "Usage: $0 YOUR_IP/32"
        echo "Example: $0 203.0.113.45/32"
        echo ""
        echo "Or set MY_IP environment variable:"
        echo "  export MY_IP=203.0.113.45/32"
        echo "  $0"
        exit 1
    fi
fi

echo ""
echo "This script will create:"
echo "  • Security Group: joblet-server-sg"
echo "  • Inbound: SSH (22) from $MY_IP"
echo "  • Inbound: gRPC (443) from $MY_IP"
echo "  • Outbound: All traffic (default)"
echo ""

# Get default VPC
echo "Finding default VPC..."
VPC_ID=$(aws ec2 describe-vpcs \
  --filters "Name=is-default,Values=true" \
  --query 'Vpcs[0].VpcId' \
  --output text)

if [ -z "$VPC_ID" ] || [ "$VPC_ID" = "None" ]; then
    echo "❌ Error: No default VPC found"
    echo "Please specify VPC ID:"
    echo "  export VPC_ID=vpc-xxxxxxxxx"
    echo "  $0"
    exit 1
fi

echo "Using VPC: $VPC_ID"

# Check if security group already exists
EXISTING_SG=$(aws ec2 describe-security-groups \
  --filters "Name=group-name,Values=joblet-server-sg" "Name=vpc-id,Values=$VPC_ID" \
  --query 'SecurityGroups[0].GroupId' \
  --output text 2>/dev/null || echo "")

if [ -n "$EXISTING_SG" ] && [ "$EXISTING_SG" != "None" ]; then
    echo "⚠️  Security group 'joblet-server-sg' already exists: $EXISTING_SG"
    SG_ID="$EXISTING_SG"

    # Update rules (this will fail silently if rules already exist)
    echo "Ensuring rules are configured..."
    aws ec2 authorize-security-group-ingress \
      --group-id "$SG_ID" \
      --protocol tcp \
      --port 22 \
      --cidr "$MY_IP" 2>/dev/null || echo "   (SSH rule already exists)"

    aws ec2 authorize-security-group-ingress \
      --group-id "$SG_ID" \
      --protocol tcp \
      --port 443 \
      --cidr "$MY_IP" 2>/dev/null || echo "   (gRPC rule already exists)"
else
    # Create security group
    echo "Creating security group..."
    SG_ID=$(aws ec2 create-security-group \
      --group-name joblet-server-sg \
      --description "Security group for Joblet server - SSH and gRPC access" \
      --vpc-id "$VPC_ID" \
      --output text)

    echo "✅ Security group created: $SG_ID"

    # Add SSH rule
    echo "Adding SSH rule (port 22)..."
    aws ec2 authorize-security-group-ingress \
      --group-id "$SG_ID" \
      --protocol tcp \
      --port 22 \
      --cidr "$MY_IP" >/dev/null

    echo "✅ SSH access configured: $MY_IP"

    # Add gRPC rule
    echo "Adding gRPC rule (port 443)..."
    aws ec2 authorize-security-group-ingress \
      --group-id "$SG_ID" \
      --protocol tcp \
      --port 443 \
      --cidr "$MY_IP" >/dev/null

    echo "✅ gRPC access configured: $MY_IP"
fi

echo ""
echo "=========================================================================="
echo "✅ Security Group Setup Complete"
echo "=========================================================================="
echo ""
echo "Security Group ID: $SG_ID"
echo "VPC: $VPC_ID"
echo ""
echo "Inbound rules:"
echo "  • SSH (22):  $MY_IP"
echo "  • gRPC (443): $MY_IP"
echo ""
echo "⚠️  Security Note:"
echo "  Access is restricted to: $MY_IP"
echo "  If your IP changes, update the security group or re-run this script"
echo ""
echo "Next step: Launch EC2 instance"
echo "  Run: ./launch-instance.sh"
echo "  Or set: export SG_ID=$SG_ID"
echo ""
