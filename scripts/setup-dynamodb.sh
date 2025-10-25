#!/bin/bash
#
# Setup DynamoDB table for state persistence
# Usage: ./setup-dynamodb.sh [region] [table-name]
#

set -e

REGION="${1:-us-east-1}"
TABLE_NAME="${2:-joblet-jobs}"

echo "=== Setting up DynamoDB table for Joblet State Persistence ==="
echo "Region: $REGION"
echo "Table: $TABLE_NAME"
echo ""

# Create DynamoDB table
echo "Creating DynamoDB table..."
aws dynamodb create-table \
  --region "$REGION" \
  --table-name "$TABLE_NAME" \
  --attribute-definitions \
    AttributeName=jobId,AttributeType=S \
  --key-schema \
    AttributeName=jobId,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST \
  --tags \
    Key=Application,Value=Joblet \
    Key=Component,Value=JobState \
    Key=ManagedBy,Value=Script

echo "✓ Table created successfully"
echo ""

# Wait for table to become active
echo "Waiting for table to become active..."
aws dynamodb wait table-exists \
  --region "$REGION" \
  --table-name "$TABLE_NAME"

echo "✓ Table is active"
echo ""

# Enable TTL for automatic cleanup of completed jobs
echo "Enabling TTL attribute 'expiresAt'..."
aws dynamodb update-time-to-live \
  --region "$REGION" \
  --table-name "$TABLE_NAME" \
  --time-to-live-specification \
    "Enabled=true,AttributeName=expiresAt"

echo "✓ TTL enabled successfully"
echo ""

# Describe table
echo "Table details:"
aws dynamodb describe-table \
  --region "$REGION" \
  --table-name "$TABLE_NAME" \
  --query 'Table.[TableName,TableStatus,BillingModeSummary.BillingMode,TimeToLiveDescription.TimeToLiveStatus]' \
  --output table

echo ""
echo "=== Setup Complete ==="
echo ""
echo "Next steps:"
echo "1. Update /opt/joblet/config/joblet-config.yml:"
echo "   state:"
echo "     enabled: true"
echo "     backend: dynamodb"
echo "     storage:"
echo "       dynamodb:"
echo "         region: \"$REGION\""
echo "         table_name: \"$TABLE_NAME\""
echo ""
echo "2. Restart joblet service:"
echo "   sudo systemctl restart joblet"
echo ""
echo "3. Verify state subprocess is running:"
echo "   ps aux | grep state"
echo ""
