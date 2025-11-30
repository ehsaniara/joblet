#!/bin/bash
#
# Joblet AWS Pre-Setup Script
# Prepares AWS resources before EC2 instance launch:
#   - Creates IAM role with CloudWatch Logs and DynamoDB permissions
#   - Creates DynamoDB table for job state persistence
#   - Configures VPC Endpoint for secure DynamoDB access (no internet required)
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
echo "  • VPC Endpoint for DynamoDB (required)"
echo "  • Secrets Manager: CA and client certificates (for horizontal scaling)"
echo ""
echo "Permissions granted:"
echo "  ✅ CloudWatch Logs - Automatic log aggregation"
echo "  ✅ DynamoDB - Persistent job state (via private VPC endpoint)"
echo "  ✅ Secrets Manager - Shared CA/client certificates"
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

# Get region early - this is critical for DynamoDB table location
REGION="${AWS_DEFAULT_REGION:-${AWS_REGION:-us-east-1}}"

echo "=========================================================================="
echo "⚠️  IMPORTANT: Using AWS Region: $REGION"
echo "=========================================================================="
echo ""
echo "The DynamoDB table will be created in this region."
echo "Your EC2 instance MUST be launched in the SAME region ($REGION)."
echo ""
echo "To use a different region, set AWS_DEFAULT_REGION before running:"
echo "  export AWS_DEFAULT_REGION=us-west-2"
echo "  ./pre-setup.sh"
echo ""

echo "Checking for existing IAM resources..."

# Define the policy document (used for both create and update)
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
    },
    {
      "Sid": "SecretsManagerCertAccess",
      "Effect": "Allow",
      "Action": [
        "secretsmanager:GetSecretValue",
        "secretsmanager:DescribeSecret",
        "secretsmanager:CreateSecret",
        "secretsmanager:UpdateSecret",
        "secretsmanager:PutSecretValue",
        "secretsmanager:TagResource"
      ],
      "Resource": "arn:aws:secretsmanager:*:*:secret:joblet/*"
    }
  ]
}
EOF

# Check if policy already exists
EXISTING_POLICY=$(aws iam list-policies --scope Local --query "Policies[?PolicyName=='JobletAWSPolicy'].Arn" --output text)
if [ -n "$EXISTING_POLICY" ]; then
    echo "⚠️  IAM Policy 'JobletAWSPolicy' already exists: $EXISTING_POLICY"
    echo "Replacing with latest permissions..."

    # Detach from JobletEC2Role
    aws iam detach-role-policy --role-name JobletEC2Role --policy-arn "$EXISTING_POLICY" 2>/dev/null || true

    # Delete all non-default policy versions
    for VERSION_ID in $(aws iam list-policy-versions --policy-arn "$EXISTING_POLICY" \
        --query 'Versions[?IsDefaultVersion==`false`].VersionId' --output text 2>/dev/null); do
        aws iam delete-policy-version --policy-arn "$EXISTING_POLICY" --version-id "$VERSION_ID" 2>/dev/null || true
    done

    # Delete the policy
    if ! aws iam delete-policy --policy-arn "$EXISTING_POLICY" 2>&1; then
        echo "❌ Failed to delete policy. It may have other dependencies."
        echo "   Please manually detach the policy from other roles/users/groups and re-run."
        rm -f /tmp/joblet-aws-policy.json
        exit 1
    fi
    echo "   Deleted old policy"
fi

# Create IAM policy
echo "Creating IAM policy..."
POLICY_ARN=$(aws iam create-policy \
  --policy-name JobletAWSPolicy \
  --policy-document file:///tmp/joblet-aws-policy.json \
  --query 'Policy.Arn' \
  --output text)

echo "✅ IAM Policy created: $POLICY_ARN"

rm -f /tmp/joblet-aws-policy.json

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
echo "Secrets Manager Certificates"
echo "=========================================================================="
echo ""
echo "Checking for shared CA and client certificates..."
echo "These certificates enable horizontal scaling (multiple EC2 instances)."
echo ""

SECRETS_PREFIX="joblet"

# Check if CA cert exists
CA_CERT_EXISTS=false
if aws secretsmanager describe-secret --secret-id "$SECRETS_PREFIX/ca-cert" --region "$REGION" >/dev/null 2>&1; then
    echo "✅ CA certificate already exists in Secrets Manager"
    CA_CERT_EXISTS=true
fi

CLIENT_CERT_EXISTS=false
if aws secretsmanager describe-secret --secret-id "$SECRETS_PREFIX/client-cert" --region "$REGION" >/dev/null 2>&1; then
    echo "✅ Client certificate already exists in Secrets Manager"
    CLIENT_CERT_EXISTS=true
fi

# Generate and store certs if they don't exist
if [ "$CA_CERT_EXISTS" = false ] || [ "$CLIENT_CERT_EXISTS" = false ]; then
    echo ""
    echo "Generating certificates..."

    # Create temp directory for certificate generation
    CERT_TEMP_DIR=$(mktemp -d)
    trap "rm -rf $CERT_TEMP_DIR" EXIT

    if [ "$CA_CERT_EXISTS" = false ]; then
        echo "   Generating CA certificate (4096-bit RSA, 3 years validity)..."

        # Generate CA private key
        openssl genrsa -out "$CERT_TEMP_DIR/ca.key" 4096 2>/dev/null

        # Generate CA certificate
        openssl req -new -x509 -days 1095 -sha256 \
            -key "$CERT_TEMP_DIR/ca.key" \
            -out "$CERT_TEMP_DIR/ca.crt" \
            -subj "/C=US/ST=Cloud/L=AWS/O=Joblet/OU=CA/CN=Joblet Root CA" 2>/dev/null

        # Store CA cert in Secrets Manager
        if aws secretsmanager create-secret \
            --name "$SECRETS_PREFIX/ca-cert" \
            --description "Joblet Root CA Certificate" \
            --secret-string "$(cat "$CERT_TEMP_DIR/ca.crt")" \
            --region "$REGION" \
            --tags Key=Application,Value=Joblet Key=Type,Value=Certificate >/dev/null 2>&1; then
            echo "   ✅ CA certificate stored in Secrets Manager"
        else
            echo "   ❌ Failed to store CA certificate"
        fi

        # Store CA key in Secrets Manager
        if aws secretsmanager create-secret \
            --name "$SECRETS_PREFIX/ca-key" \
            --description "Joblet Root CA Private Key" \
            --secret-string "$(cat "$CERT_TEMP_DIR/ca.key")" \
            --region "$REGION" \
            --tags Key=Application,Value=Joblet Key=Type,Value=PrivateKey >/dev/null 2>&1; then
            echo "   ✅ CA private key stored in Secrets Manager"
        else
            echo "   ❌ Failed to store CA private key"
        fi

        CA_CERT_EXISTS=true
    else
        # Retrieve CA cert and key for client cert generation
        aws secretsmanager get-secret-value --secret-id "$SECRETS_PREFIX/ca-cert" --region "$REGION" \
            --query 'SecretString' --output text > "$CERT_TEMP_DIR/ca.crt" 2>/dev/null
        aws secretsmanager get-secret-value --secret-id "$SECRETS_PREFIX/ca-key" --region "$REGION" \
            --query 'SecretString' --output text > "$CERT_TEMP_DIR/ca.key" 2>/dev/null
    fi

    if [ "$CLIENT_CERT_EXISTS" = false ]; then
        echo "   Generating client certificate (2048-bit RSA, 1 year validity)..."

        # Generate client private key
        openssl genrsa -out "$CERT_TEMP_DIR/client.key" 2048 2>/dev/null

        # Generate client CSR
        openssl req -new -sha256 \
            -key "$CERT_TEMP_DIR/client.key" \
            -out "$CERT_TEMP_DIR/client.csr" \
            -subj "/C=US/ST=Cloud/L=AWS/O=Joblet/OU=Client/CN=admin" 2>/dev/null

        # Create client extensions file
        cat > "$CERT_TEMP_DIR/client_ext.cnf" << 'EXTEOF'
basicConstraints = CA:FALSE
keyUsage = digitalSignature, keyEncipherment
extendedKeyUsage = clientAuth
EXTEOF

        # Sign client certificate with CA
        openssl x509 -req -days 365 -sha256 \
            -in "$CERT_TEMP_DIR/client.csr" \
            -CA "$CERT_TEMP_DIR/ca.crt" \
            -CAkey "$CERT_TEMP_DIR/ca.key" \
            -CAcreateserial \
            -out "$CERT_TEMP_DIR/client.crt" \
            -extfile "$CERT_TEMP_DIR/client_ext.cnf" 2>/dev/null

        # Store client cert in Secrets Manager
        if aws secretsmanager create-secret \
            --name "$SECRETS_PREFIX/client-cert" \
            --description "Joblet Admin Client Certificate" \
            --secret-string "$(cat "$CERT_TEMP_DIR/client.crt")" \
            --region "$REGION" \
            --tags Key=Application,Value=Joblet Key=Type,Value=Certificate >/dev/null 2>&1; then
            echo "   ✅ Client certificate stored in Secrets Manager"
        else
            echo "   ❌ Failed to store client certificate"
        fi

        # Store client key in Secrets Manager
        if aws secretsmanager create-secret \
            --name "$SECRETS_PREFIX/client-key" \
            --description "Joblet Admin Client Private Key" \
            --secret-string "$(cat "$CERT_TEMP_DIR/client.key")" \
            --region "$REGION" \
            --tags Key=Application,Value=Joblet Key=Type,Value=PrivateKey >/dev/null 2>&1; then
            echo "   ✅ Client private key stored in Secrets Manager"
        else
            echo "   ❌ Failed to store client private key"
        fi
    fi

    echo ""
    echo "✅ Certificates created and stored in Secrets Manager"
    echo "   Secret prefix: $SECRETS_PREFIX"
    echo "   Secrets created:"
    echo "     - $SECRETS_PREFIX/ca-cert"
    echo "     - $SECRETS_PREFIX/ca-key"
    echo "     - $SECRETS_PREFIX/client-cert"
    echo "     - $SECRETS_PREFIX/client-key"
else
    echo ""
    echo "✅ All certificates already exist in Secrets Manager"
fi

echo ""
echo "=========================================================================="
echo "VPC Selection"
echo "=========================================================================="
echo ""

# List VPCs with their names
VPC_LIST=$(aws ec2 describe-vpcs --region "$REGION" \
    --query 'Vpcs[*].[VpcId,Tags[?Key==`Name`].Value|[0],CidrBlock,IsDefault]' \
    --output text 2>/dev/null || echo "")

if [ -z "$VPC_LIST" ]; then
    echo "❌ No VPCs found in region $REGION"
    echo "   You need to create a VPC before launching EC2."
    VPC_ID=""
    ENDPOINT_STATUS="none"
else
    echo "Available VPCs in $REGION:"
    echo ""

    # Display VPC list with numbers for selection
    VPC_COUNT=0
    echo "$VPC_LIST" | while IFS=$'\t' read -r vpc_id name cidr is_default; do
        VPC_COUNT=$((VPC_COUNT + 1))
        default_marker=""
        if [ "$is_default" = "True" ]; then
            default_marker=" (default)"
        fi
        printf "  %d) %-22s %-20s %-15s%s\n" "$VPC_COUNT" "$vpc_id" "${name:-<no name>}" "$cidr" "$default_marker"
    done

    echo ""

    # Get default VPC ID for suggestion
    DEFAULT_VPC=$(aws ec2 describe-vpcs --region "$REGION" \
        --filters "Name=is-default,Values=true" \
        --query 'Vpcs[0].VpcId' --output text 2>/dev/null || echo "None")

    if [ "$DEFAULT_VPC" != "None" ] && [ -n "$DEFAULT_VPC" ]; then
        read -p "Enter VPC ID for EC2 instance [default: $DEFAULT_VPC]: " VPC_ID </dev/tty
        if [ -z "$VPC_ID" ]; then
            VPC_ID="$DEFAULT_VPC"
        fi
    else
        read -p "Enter VPC ID for EC2 instance: " VPC_ID </dev/tty
    fi

    # Validate VPC ID exists
    if ! echo "$VPC_LIST" | grep -q "^$VPC_ID"; then
        echo "❌ Invalid VPC ID: $VPC_ID"
        exit 1
    fi

    echo ""
    echo "✅ Selected VPC: $VPC_ID"

    echo ""
    echo "=========================================================================="
    echo "VPC Endpoint Configuration (Required)"
    echo "=========================================================================="
    echo ""
    echo "A DynamoDB VPC Endpoint is required for Joblet to access DynamoDB."
    echo ""
    echo "Checking for existing DynamoDB VPC Endpoints in $VPC_ID..."

    # Check if DynamoDB endpoint already exists in this VPC (any state, not just 'available')
    EXISTING_ENDPOINTS=$(aws ec2 describe-vpc-endpoints --region "$REGION" \
        --filters "Name=vpc-id,Values=$VPC_ID" "Name=service-name,Values=com.amazonaws.$REGION.dynamodb" \
        --query 'VpcEndpoints[*].[VpcEndpointId,State,Tags[?Key==`Name`].Value|[0]]' \
        --output text 2>/dev/null || echo "")

    if [ -n "$EXISTING_ENDPOINTS" ] && [ "$EXISTING_ENDPOINTS" != "None" ]; then
        echo ""
        echo "Found existing DynamoDB VPC Endpoint(s):"
        echo ""

        # Display endpoints with numbers
        ENDPOINT_NUM=0
        echo "$EXISTING_ENDPOINTS" | while IFS=$'\t' read -r endpoint_id state name; do
            ENDPOINT_NUM=$((ENDPOINT_NUM + 1))
            printf "  %d) %s [%s] %s\n" "$ENDPOINT_NUM" "$endpoint_id" "$state" "${name:+($name)}"
        done
        echo "  N) Create new endpoint"
        echo ""

        read -p "Select endpoint [1]: " ENDPOINT_CHOICE </dev/tty
        ENDPOINT_CHOICE="${ENDPOINT_CHOICE:-1}"

        if [ "$ENDPOINT_CHOICE" = "N" ] || [ "$ENDPOINT_CHOICE" = "n" ]; then
            # Create new endpoint
            echo ""
            echo "Creating new DynamoDB VPC Endpoint..."

            ROUTE_TABLE_IDS=$(aws ec2 describe-route-tables --region "$REGION" \
                --filters "Name=vpc-id,Values=$VPC_ID" \
                --query 'RouteTables[*].RouteTableId' --output text 2>/dev/null | tr '\t' ' ')

            if [ -z "$ROUTE_TABLE_IDS" ] || [ "$ROUTE_TABLE_IDS" = "None" ]; then
                echo "❌ No route tables found for VPC $VPC_ID"
                echo "   Cannot create VPC Endpoint."
                exit 1
            fi

            echo "   Route tables: $ROUTE_TABLE_IDS"

            if ENDPOINT_ID=$(aws ec2 create-vpc-endpoint --region "$REGION" \
                --vpc-id "$VPC_ID" \
                --service-name "com.amazonaws.$REGION.dynamodb" \
                --route-table-ids $ROUTE_TABLE_IDS \
                --vpc-endpoint-type Gateway \
                --tag-specifications "ResourceType=vpc-endpoint,Tags=[{Key=Name,Value=joblet-dynamodb-endpoint},{Key=ManagedBy,Value=Joblet}]" \
                --query 'VpcEndpoint.VpcEndpointId' --output text 2>&1); then
                echo "✅ VPC Endpoint created: $ENDPOINT_ID"
                ENDPOINT_STATUS="created"
            else
                echo "❌ Failed to create VPC Endpoint: $ENDPOINT_ID"
                exit 1
            fi
        else
            # Use existing endpoint (select by number)
            ENDPOINT_ID=$(echo "$EXISTING_ENDPOINTS" | sed -n "${ENDPOINT_CHOICE}p" | cut -f1)
            if [ -z "$ENDPOINT_ID" ]; then
                # Default to first if invalid selection
                ENDPOINT_ID=$(echo "$EXISTING_ENDPOINTS" | head -1 | cut -f1)
            fi
            echo ""
            echo "✅ Using existing VPC Endpoint: $ENDPOINT_ID"
            ENDPOINT_STATUS="existing"
        fi
    else
        echo ""
        echo "No existing DynamoDB VPC Endpoint found in query results."
        echo ""

        # Get route tables for this VPC
        ROUTE_TABLE_IDS=$(aws ec2 describe-route-tables --region "$REGION" \
            --filters "Name=vpc-id,Values=$VPC_ID" \
            --query 'RouteTables[*].RouteTableId' --output text 2>/dev/null | tr '\t' ' ')

        # Check if DynamoDB routes already exist in any route table
        # DynamoDB uses prefix list (pl-*) routes for Gateway endpoints
        DYNAMODB_PREFIX_LIST=$(aws ec2 describe-prefix-lists --region "$REGION" \
            --filters "Name=prefix-list-name,Values=com.amazonaws.$REGION.dynamodb" \
            --query 'PrefixLists[0].PrefixListId' --output text 2>/dev/null || echo "")

        if [ -n "$DYNAMODB_PREFIX_LIST" ] && [ "$DYNAMODB_PREFIX_LIST" != "None" ]; then
            # Check if any route table has this prefix list
            for RT_ID in $ROUTE_TABLE_IDS; do
                EXISTING_ROUTE=$(aws ec2 describe-route-tables --region "$REGION" \
                    --route-table-ids "$RT_ID" \
                    --query "RouteTables[0].Routes[?DestinationPrefixListId=='$DYNAMODB_PREFIX_LIST'].DestinationPrefixListId" \
                    --output text 2>/dev/null || echo "")

                if [ -n "$EXISTING_ROUTE" ] && [ "$EXISTING_ROUTE" != "None" ]; then
                    echo "✅ DynamoDB route already exists in route table $RT_ID"
                    echo "   A DynamoDB VPC Endpoint is already configured for this VPC."
                    echo ""

                    # Try to find the endpoint ID
                    ENDPOINT_ID=$(aws ec2 describe-vpc-endpoints --region "$REGION" \
                        --filters "Name=vpc-id,Values=$VPC_ID" "Name=service-name,Values=com.amazonaws.$REGION.dynamodb" \
                        --query 'VpcEndpoints[0].VpcEndpointId' --output text 2>/dev/null || echo "")

                    if [ -n "$ENDPOINT_ID" ] && [ "$ENDPOINT_ID" != "None" ]; then
                        echo "   Endpoint ID: $ENDPOINT_ID"
                    fi

                    ENDPOINT_STATUS="existing"
                    break
                fi
            done
        fi

        # If we found existing route, skip creation
        if [ "$ENDPOINT_STATUS" = "existing" ]; then
            echo ""
            echo "Skipping VPC Endpoint creation - already configured."
        else
            # No existing route found, proceed with creation
            echo "Creating new DynamoDB VPC Endpoint..."
            echo ""

            if [ -z "$ROUTE_TABLE_IDS" ] || [ "$ROUTE_TABLE_IDS" = "None" ]; then
                echo "❌ No route tables found for VPC $VPC_ID"
                echo "   Cannot create VPC Endpoint. Please create route tables first."
                exit 1
            fi

            echo "   Route tables: $ROUTE_TABLE_IDS"

            # Try to create endpoint, handle RouteAlreadyExists error
            CREATE_OUTPUT=$(aws ec2 create-vpc-endpoint --region "$REGION" \
                --vpc-id "$VPC_ID" \
                --service-name "com.amazonaws.$REGION.dynamodb" \
                --route-table-ids $ROUTE_TABLE_IDS \
                --vpc-endpoint-type Gateway \
                --tag-specifications "ResourceType=vpc-endpoint,Tags=[{Key=Name,Value=joblet-dynamodb-endpoint},{Key=ManagedBy,Value=Joblet}]" \
                --query 'VpcEndpoint.VpcEndpointId' --output text 2>&1)
            CREATE_STATUS=$?

            if [ $CREATE_STATUS -eq 0 ]; then
                ENDPOINT_ID="$CREATE_OUTPUT"
                echo "✅ VPC Endpoint created: $ENDPOINT_ID"
                ENDPOINT_STATUS="created"
            elif echo "$CREATE_OUTPUT" | grep -q "RouteAlreadyExists"; then
                # Route exists - there's already a DynamoDB endpoint for this VPC
                echo ""
                echo "⚠️  DynamoDB route already exists in route table."
                echo "   This means a DynamoDB VPC Endpoint is already configured."
                echo ""

                # Try to find the existing endpoint (including all states)
                ENDPOINT_ID=$(aws ec2 describe-vpc-endpoints --region "$REGION" \
                    --filters "Name=vpc-id,Values=$VPC_ID" "Name=service-name,Values=com.amazonaws.$REGION.dynamodb" \
                    --query 'VpcEndpoints[0].VpcEndpointId' --output text 2>/dev/null || echo "")

                if [ -n "$ENDPOINT_ID" ] && [ "$ENDPOINT_ID" != "None" ]; then
                    echo "✅ Found existing VPC Endpoint: $ENDPOINT_ID"
                    ENDPOINT_STATUS="existing"
                else
                    echo "✅ DynamoDB access is already configured via existing VPC Endpoint."
                    ENDPOINT_STATUS="existing"
                fi
            else
                echo "❌ Failed to create VPC Endpoint:"
                echo "   $CREATE_OUTPUT"
                exit 1
            fi
        fi
    fi

    # Update VPC Endpoint policy to allow DynamoDB access
    # This is needed because default policies may be restrictive
    if [ -n "$ENDPOINT_ID" ] && [ "$ENDPOINT_ID" != "None" ]; then
        echo ""
        echo "Updating VPC Endpoint policy for DynamoDB access..."

        # Create policy with only required DynamoDB actions (least privilege)
        # Get AWS account ID for resource ARN
        AWS_ACCOUNT_ID=$(aws sts get-caller-identity --query 'Account' --output text 2>/dev/null || echo "*")

        cat > /tmp/dynamodb-endpoint-policy.json << POLICY_EOF
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Principal": "*",
            "Action": [
                "dynamodb:DescribeTable",
                "dynamodb:CreateTable",
                "dynamodb:PutItem",
                "dynamodb:GetItem",
                "dynamodb:UpdateItem",
                "dynamodb:DeleteItem",
                "dynamodb:Query",
                "dynamodb:Scan",
                "dynamodb:BatchWriteItem",
                "dynamodb:UpdateTimeToLive",
                "dynamodb:DescribeTimeToLive"
            ],
            "Resource": "arn:aws:dynamodb:${REGION}:${AWS_ACCOUNT_ID}:table/joblet-jobs"
        }
    ]
}
POLICY_EOF

        if aws ec2 modify-vpc-endpoint --region "$REGION" \
            --vpc-endpoint-id "$ENDPOINT_ID" \
            --policy-document file:///tmp/dynamodb-endpoint-policy.json 2>/dev/null; then
            echo "✅ VPC Endpoint policy updated (full DynamoDB access)"
        else
            echo "⚠️  Could not update VPC Endpoint policy"
            echo "   You may need to manually update the policy to allow DynamoDB access."
            echo "   See: AWS Console → VPC → Endpoints → $ENDPOINT_ID → Policy"
        fi

        rm -f /tmp/dynamodb-endpoint-policy.json
    fi
fi

echo ""
echo "=========================================================================="
echo "✅ Pre-Setup Complete"
echo "=========================================================================="
echo ""
echo "Resources ready:"
echo "  • IAM Policy: $POLICY_ARN"
echo "  • IAM Role: JobletEC2Role"
echo "  • Instance Profile: JobletEC2Role"
echo "  • DynamoDB Table: joblet-jobs (region: $REGION)"
echo "  • Secrets Manager: CA and client certificates"
if [ -n "$VPC_ID" ]; then
    echo "  • VPC: $VPC_ID"
    if [ "$ENDPOINT_STATUS" = "created" ]; then
        echo "  • VPC Endpoint: Created (DynamoDB Gateway)"
    else
        echo "  • VPC Endpoint: Configured (DynamoDB Gateway)"
    fi
fi
echo ""
echo "=========================================================================="
echo "Next Step: Launch EC2 Instance"
echo "=========================================================================="
echo ""
echo "Launch your EC2 instance with these settings:"
echo ""
echo "  Region:           $REGION"
if [ -n "$VPC_ID" ]; then
    echo "  VPC:              $VPC_ID"
fi
echo "  IAM Role:         JobletEC2Role"
echo "  AMI:              Ubuntu Server 22.04 LTS"
echo "  Instance Type:    t3.medium (or larger)"
echo ""
echo "From AWS Console: EC2 → Launch Instance"
echo "  1. Verify region is '$REGION' (top-right corner)"
if [ -n "$VPC_ID" ]; then
    echo "  2. Network settings → Select VPC: $VPC_ID"
fi
echo "  3. Advanced details → IAM instance profile: JobletEC2Role"
echo ""
