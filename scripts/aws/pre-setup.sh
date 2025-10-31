#!/bin/bash
#
# Joblet AWS Pre-Setup Script
# Prepares AWS resources before EC2 instance launch:
#   - Creates IAM role with CloudWatch Logs and DynamoDB permissions
#   - Creates DynamoDB table for job state persistence
#
# Usage: ./pre-setup.sh
# Or: curl -fsSL https://raw.githubusercontent.com/ehsaniara/joblet/main/scripts/aws/pre-setup.sh | bash
#

set -e

echo "=========================================================================="
echo "Joblet AWS Pre-Setup"
echo "=========================================================================="
echo ""
echo "This script will create:"
echo "  • IAM Policy: JobletAWSPolicy"
echo "  • IAM Role: JobletEC2Role"
echo "  • Instance Profile: JobletEC2Role"
echo "  • DynamoDB Table: joblet-jobs"
echo ""
echo "Permissions granted:"
echo "  ✅ CloudWatch Logs - Automatic log aggregation"
echo "  ✅ DynamoDB - Persistent job state"
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
echo "Creating DynamoDB Table"
echo "=========================================================================="
echo ""

# Get region (default to us-east-1)
REGION="${AWS_DEFAULT_REGION:-us-east-1}"

# Check if table already exists
if aws dynamodb describe-table --table-name joblet-jobs --region "$REGION" >/dev/null 2>&1; then
    echo "⚠️  DynamoDB table 'joblet-jobs' already exists in region: $REGION"

    # Check if TTL is enabled
    TTL_STATUS=$(aws dynamodb describe-time-to-live --table-name joblet-jobs --region "$REGION" --query 'TimeToLiveDescription.TimeToLiveStatus' --output text 2>/dev/null || echo "")

    if [ "$TTL_STATUS" = "ENABLED" ]; then
        echo "✅ TTL already enabled on table"
    else
        echo "Enabling TTL for automatic cleanup..."
        if aws dynamodb update-time-to-live \
            --table-name joblet-jobs \
            --time-to-live-specification "Enabled=true,AttributeName=expiresAt" \
            --region "$REGION" >/dev/null 2>&1; then
            echo "✅ TTL enabled - completed jobs will be auto-deleted after 30 days"
        else
            echo "⚠️  Could not enable TTL (may require additional permissions)"
        fi
    fi
else
    # Create DynamoDB table
    echo "Creating DynamoDB table: joblet-jobs in region: $REGION"
    if aws dynamodb create-table \
        --table-name joblet-jobs \
        --attribute-definitions AttributeName=jobId,AttributeType=S \
        --key-schema AttributeName=jobId,KeyType=HASH \
        --billing-mode PAY_PER_REQUEST \
        --region "$REGION" \
        --tags Key=ManagedBy,Value=Joblet Key=Purpose,Value=JobStatePersistence \
        >/dev/null 2>&1; then
        echo "✅ DynamoDB table created successfully"

        # Wait for table to be active
        echo "Waiting for table to become active..."
        if aws dynamodb wait table-exists --table-name joblet-jobs --region "$REGION" 2>/dev/null; then
            echo "✅ Table is now active"

            # Enable TTL
            echo "Enabling TTL for automatic cleanup of old jobs..."
            if aws dynamodb update-time-to-live \
                --table-name joblet-jobs \
                --time-to-live-specification "Enabled=true,AttributeName=expiresAt" \
                --region "$REGION" >/dev/null 2>&1; then
                echo "✅ TTL enabled - completed jobs will be auto-deleted after 30 days"
            else
                echo "⚠️  Could not enable TTL (table created but TTL requires additional permissions)"
            fi
        else
            echo "⚠️  Table created but may still be initializing"
        fi
    else
        echo "❌ Failed to create DynamoDB table"
        echo "You may need to create it manually using the AWS Console"
        echo "Table name: joblet-jobs"
        echo "Region: $REGION"
    fi
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
echo "  • DynamoDB Table: joblet-jobs (region: $REGION)"
echo ""
echo "This enables:"
echo "  ✅ CloudWatch Logs (/joblet log group)"
echo "  ✅ DynamoDB state (joblet-jobs table)"
echo "  ✅ EC2 metadata access"
echo ""
echo "Next step: Launch EC2 instance"
echo "  - From Console: EC2 → Launch Instance"
echo "  - Or run: ./launch-instance.sh"
echo ""
