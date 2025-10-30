#!/bin/bash
#
# Joblet AWS IAM Setup Script
# Creates IAM role with permissions for CloudWatch Logs and DynamoDB state persistence
#
# Usage: ./setup-iam.sh
# Or: curl -fsSL https://raw.githubusercontent.com/ehsaniara/joblet/main/scripts/aws/setup-iam.sh | bash
#

set -e

echo "=========================================================================="
echo "Joblet AWS IAM Setup"
echo "=========================================================================="
echo ""
echo "This script will create:"
echo "  • IAM Policy: JobletAWSPolicy"
echo "  • IAM Role: JobletEC2Role"
echo "  • Instance Profile: JobletEC2Role"
echo ""
echo "Permissions granted:"
echo "  ✅ CloudWatch Logs - Automatic log aggregation"
echo "  ✅ DynamoDB - Persistent job state (auto-created table)"
echo "  ✅ EC2 Metadata - Region detection"
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

echo "Checking for existing IAM resources..."

# Check if policy already exists
EXISTING_POLICY=$(aws iam list-policies --scope Local --query "Policies[?PolicyName=='JobletAWSPolicy'].Arn" --output text)
if [ -n "$EXISTING_POLICY" ]; then
    echo "⚠️  IAM Policy 'JobletAWSPolicy' already exists: $EXISTING_POLICY"
    POLICY_ARN="$EXISTING_POLICY"
else
    # Create IAM policy
    echo "Creating IAM policy..."
    cat > /tmp/joblet-aws-policy.json << 'EOF'
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "CloudWatchLogsAccess",
      "Effect": "Allow",
      "Action": [
        "logs:CreateLogGroup",
        "logs:CreateLogStream",
        "logs:PutLogEvents",
        "logs:PutRetentionPolicy",
        "logs:DescribeLogStreams",
        "logs:GetLogEvents",
        "logs:FilterLogEvents"
      ],
      "Resource": [
        "arn:aws:logs:*:*:log-group:/joblet/*",
        "arn:aws:logs:*:*:log-group:/joblet/*:*"
      ]
    },
    {
      "Sid": "CloudWatchMetricsAccess",
      "Effect": "Allow",
      "Action": [
        "cloudwatch:PutMetricData",
        "cloudwatch:GetMetricStatistics",
        "cloudwatch:ListMetrics"
      ],
      "Resource": "*"
    },
    {
      "Sid": "DynamoDBStateAccess",
      "Effect": "Allow",
      "Action": [
        "dynamodb:CreateTable",
        "dynamodb:DescribeTable",
        "dynamodb:DescribeTimeToLive",
        "dynamodb:UpdateTimeToLive",
        "dynamodb:PutItem",
        "dynamodb:GetItem",
        "dynamodb:UpdateItem",
        "dynamodb:DeleteItem",
        "dynamodb:Scan",
        "dynamodb:Query",
        "dynamodb:BatchWriteItem"
      ],
      "Resource": "arn:aws:dynamodb:*:*:table/joblet-jobs"
    },
    {
      "Sid": "EC2MetadataAccess",
      "Effect": "Allow",
      "Action": [
        "ec2:DescribeRegions"
      ],
      "Resource": "*"
    }
  ]
}
EOF

    POLICY_ARN=$(aws iam create-policy \
      --policy-name JobletAWSPolicy \
      --policy-document file:///tmp/joblet-aws-policy.json \
      --query 'Policy.Arn' \
      --output text)

    echo "✅ IAM Policy created: $POLICY_ARN"
    rm /tmp/joblet-aws-policy.json
fi

# Check if role already exists
if aws iam get-role --role-name JobletEC2Role >/dev/null 2>&1; then
    echo "⚠️  IAM Role 'JobletEC2Role' already exists"

    # Ensure policy is attached
    echo "Ensuring policy is attached to role..."
    aws iam attach-role-policy \
      --role-name JobletEC2Role \
      --policy-arn "$POLICY_ARN" 2>/dev/null || echo "   (Policy already attached)"
else
    # Create IAM role
    echo "Creating IAM role..."
    aws iam create-role \
      --role-name JobletEC2Role \
      --assume-role-policy-document '{
        "Version": "2012-10-17",
        "Statement": [{
          "Effect": "Allow",
          "Principal": {"Service": "ec2.amazonaws.com"},
          "Action": "sts:AssumeRole"
        }]
      }' >/dev/null

    echo "✅ IAM Role created: JobletEC2Role"

    # Attach policy to role
    echo "Attaching policy to role..."
    aws iam attach-role-policy \
      --role-name JobletEC2Role \
      --policy-arn "$POLICY_ARN"

    echo "✅ Policy attached to role"
fi

# Check if instance profile exists
if aws iam get-instance-profile --instance-profile-name JobletEC2Role >/dev/null 2>&1; then
    echo "⚠️  Instance Profile 'JobletEC2Role' already exists"
else
    # Create instance profile
    echo "Creating instance profile..."
    aws iam create-instance-profile \
      --instance-profile-name JobletEC2Role >/dev/null

    echo "✅ Instance Profile created: JobletEC2Role"

    # Add role to instance profile
    echo "Adding role to instance profile..."
    aws iam add-role-to-instance-profile \
      --instance-profile-name JobletEC2Role \
      --role-name JobletEC2Role

    echo "✅ Role added to instance profile"
fi

echo ""
echo "=========================================================================="
echo "✅ IAM Setup Complete"
echo "=========================================================================="
echo ""
echo "Resources created:"
echo "  • Policy ARN: $POLICY_ARN"
echo "  • IAM Role: JobletEC2Role"
echo "  • Instance Profile: JobletEC2Role"
echo ""
echo "This enables:"
echo "  ✅ CloudWatch Logs (/joblet log group)"
echo "  ✅ DynamoDB state (joblet-jobs table, auto-created)"
echo "  ✅ EC2 metadata access"
echo ""
echo "Next step: Create security group"
echo "  Run: ./setup-security-group.sh"
echo ""
