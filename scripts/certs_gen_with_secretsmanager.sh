#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

print_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

print_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

print_error() {
    echo -e "${RED}❌ $1${NC}"
}

# Detect if running on AWS EC2 by querying metadata service (IMDSv2 only)
detect_ec2() {
    # Use IMDSv2 (required on modern EC2 instances, IMDSv1 is disabled)
    local token
    token=$(curl -s -m 2 -X PUT "http://169.254.169.254/latest/api/token" \
        -H "X-aws-ec2-metadata-token-ttl-seconds: 21600" 2>/dev/null || echo "")

    if [ -z "$token" ]; then
        return 1  # Not EC2 or IMDS not available
    fi

    local instance_id
    instance_id=$(curl -s -m 2 -H "X-aws-ec2-metadata-token: $token" \
        "http://169.254.169.254/latest/meta-data/instance-id" 2>/dev/null || echo "")

    if [ -n "$instance_id" ] && [[ "$instance_id" == i-* ]]; then
        return 0  # Is EC2
    else
        return 1  # Not EC2
    fi
}

# Get EC2 metadata value (IMDSv2 only)
get_ec2_metadata() {
    local path="$1"
    local token
    token=$(curl -s -m 2 -X PUT "http://169.254.169.254/latest/api/token" \
        -H "X-aws-ec2-metadata-token-ttl-seconds: 21600" 2>/dev/null || echo "")

    if [ -z "$token" ]; then
        echo ""
        return
    fi

    curl -s -m 2 -H "X-aws-ec2-metadata-token: $token" \
        "http://169.254.169.254/latest/meta-data/$path" 2>/dev/null || echo ""
}

# ============================================================================
# AWS Secrets Manager API Functions (using curl, no AWS CLI required)
# ============================================================================

# Cache for IAM credentials
AWS_ACCESS_KEY_ID=""
AWS_SECRET_ACCESS_KEY=""
AWS_SESSION_TOKEN=""
AWS_CREDS_EXPIRY=""

# Get IAM credentials from EC2 instance metadata (IMDSv2)
get_iam_credentials() {
    if [ -n "$AWS_ACCESS_KEY_ID" ] && [ -n "$AWS_CREDS_EXPIRY" ]; then
        # Check if credentials are still valid (with 5 minute buffer)
        local now=$(date +%s)
        local expiry=$(date -d "$AWS_CREDS_EXPIRY" +%s 2>/dev/null || echo 0)
        if [ $now -lt $((expiry - 300)) ]; then
            return 0  # Credentials still valid
        fi
    fi

    # Get IMDS token
    local token=$(curl -s -m 2 -X PUT "http://169.254.169.254/latest/api/token" \
        -H "X-aws-ec2-metadata-token-ttl-seconds: 21600" 2>/dev/null || echo "")

    if [ -z "$token" ]; then
        print_error "Failed to get IMDS token"
        return 1
    fi

    # Get IAM role name
    local role_name=$(curl -s -m 2 -H "X-aws-ec2-metadata-token: $token" \
        "http://169.254.169.254/latest/meta-data/iam/security-credentials/" 2>/dev/null)

    if [ -z "$role_name" ]; then
        print_error "No IAM role attached to this EC2 instance"
        return 1
    fi

    # Get credentials for the role
    local creds_json=$(curl -s -m 5 -H "X-aws-ec2-metadata-token: $token" \
        "http://169.254.169.254/latest/meta-data/iam/security-credentials/$role_name" 2>/dev/null)

    if [ -z "$creds_json" ]; then
        print_error "Failed to get IAM credentials"
        return 1
    fi

    # Parse credentials
    if command -v jq >/dev/null 2>&1; then
        AWS_ACCESS_KEY_ID=$(echo "$creds_json" | jq -r '.AccessKeyId')
        AWS_SECRET_ACCESS_KEY=$(echo "$creds_json" | jq -r '.SecretAccessKey')
        AWS_SESSION_TOKEN=$(echo "$creds_json" | jq -r '.Token')
        AWS_CREDS_EXPIRY=$(echo "$creds_json" | jq -r '.Expiration')
    else
        # Fallback to grep/sed
        AWS_ACCESS_KEY_ID=$(echo "$creds_json" | grep -o '"AccessKeyId" *: *"[^"]*"' | sed 's/.*: *"\([^"]*\)"/\1/')
        AWS_SECRET_ACCESS_KEY=$(echo "$creds_json" | grep -o '"SecretAccessKey" *: *"[^"]*"' | sed 's/.*: *"\([^"]*\)"/\1/')
        AWS_SESSION_TOKEN=$(echo "$creds_json" | grep -o '"Token" *: *"[^"]*"' | sed 's/.*: *"\([^"]*\)"/\1/')
        AWS_CREDS_EXPIRY=$(echo "$creds_json" | grep -o '"Expiration" *: *"[^"]*"' | sed 's/.*: *"\([^"]*\)"/\1/')
    fi

    if [ -z "$AWS_ACCESS_KEY_ID" ] || [ -z "$AWS_SECRET_ACCESS_KEY" ]; then
        print_error "Failed to parse IAM credentials"
        return 1
    fi

    return 0
}

# Helper function for HMAC-SHA256 (returns hex)
hmac_sha256() {
    local key="$1"
    local data="$2"
    echo -n "$data" | openssl dgst -sha256 -mac HMAC -macopt "hexkey:$key" | sed 's/^.* //'
}

# Helper function for HMAC-SHA256 with string key (returns hex)
hmac_sha256_string_key() {
    local key="$1"
    local data="$2"
    echo -n "$data" | openssl dgst -sha256 -hmac "$key" | sed 's/^.* //'
}

# AWS Signature Version 4 signing
aws_sign_request() {
    local method="$1"
    local service="$2"
    local region="$3"
    local endpoint="$4"
    local payload="$5"
    local amz_target="$6"

    local host="${service}.${region}.amazonaws.com"
    local date_stamp=$(date -u +%Y%m%d)
    local amz_date=$(date -u +%Y%m%dT%H%M%SZ)
    local content_type="application/x-amz-json-1.1"

    # Create canonical request
    local payload_hash=$(echo -n "$payload" | openssl dgst -sha256 | sed 's/^.* //')

    # Build canonical headers (must be sorted alphabetically and lowercase)
    local canonical_headers="content-type:${content_type}
host:${host}
x-amz-date:${amz_date}
x-amz-security-token:${AWS_SESSION_TOKEN}
x-amz-target:${amz_target}
"
    local signed_headers="content-type;host;x-amz-date;x-amz-security-token;x-amz-target"

    # Build canonical request
    local canonical_request="${method}
/

${canonical_headers}
${signed_headers}
${payload_hash}"

    local canonical_request_hash=$(echo -n "$canonical_request" | openssl dgst -sha256 | sed 's/^.* //')

    # Create string to sign
    local credential_scope="${date_stamp}/${region}/${service}/aws4_request"
    local string_to_sign="AWS4-HMAC-SHA256
${amz_date}
${credential_scope}
${canonical_request_hash}"

    # Calculate signature using proper HMAC chain
    # Step 1: kDate = HMAC("AWS4" + secretKey, dateStamp)
    local k_date=$(hmac_sha256_string_key "AWS4${AWS_SECRET_ACCESS_KEY}" "$date_stamp")
    # Step 2: kRegion = HMAC(kDate, region)
    local k_region=$(hmac_sha256 "$k_date" "$region")
    # Step 3: kService = HMAC(kRegion, service)
    local k_service=$(hmac_sha256 "$k_region" "$service")
    # Step 4: kSigning = HMAC(kService, "aws4_request")
    local k_signing=$(hmac_sha256 "$k_service" "aws4_request")
    # Step 5: signature = HMAC(kSigning, stringToSign)
    local signature=$(hmac_sha256 "$k_signing" "$string_to_sign")

    local authorization="AWS4-HMAC-SHA256 Credential=${AWS_ACCESS_KEY_ID}/${credential_scope}, SignedHeaders=${signed_headers}, Signature=${signature}"

    # Make the request
    curl -s -X "$method" "https://${host}/" \
        -H "Content-Type: ${content_type}" \
        -H "X-Amz-Date: ${amz_date}" \
        -H "X-Amz-Target: ${amz_target}" \
        -H "X-Amz-Security-Token: ${AWS_SESSION_TOKEN}" \
        -H "Authorization: ${authorization}" \
        -d "$payload"
}

# Check if we can access Secrets Manager
check_secrets_manager_access() {
    if ! get_iam_credentials; then
        print_error "Cannot get IAM credentials from instance metadata"
        return 1
    fi
    return 0
}

# Check if secret exists in Secrets Manager
secret_exists() {
    local secret_name="$1"
    local region="$2"

    if ! get_iam_credentials; then
        print_error "secret_exists: Failed to get IAM credentials"
        return 1
    fi

    local payload="{\"SecretId\":\"${secret_name}\"}"
    local response=$(aws_sign_request "POST" "secretsmanager" "$region" "/" "$payload" "secretsmanager.DescribeSecret")

    print_info "DEBUG: secret_exists check for '$secret_name' in region '$region'"

    if echo "$response" | grep -q '"ARN"'; then
        print_info "DEBUG: Secret '$secret_name' exists"
        return 0  # Exists
    else
        # Check for specific error types
        if echo "$response" | grep -q "ResourceNotFoundException"; then
            print_info "DEBUG: Secret '$secret_name' does not exist (ResourceNotFoundException)"
        elif echo "$response" | grep -q "AccessDeniedException"; then
            print_error "DEBUG: Access denied to secret '$secret_name' - check IAM permissions"
            print_error "DEBUG: Response: $response"
        elif echo "$response" | grep -q "Exception"; then
            print_error "DEBUG: Error checking secret '$secret_name': $response"
        else
            print_info "DEBUG: Secret '$secret_name' not found (response: ${response:0:200}...)"
        fi
        return 1  # Does not exist
    fi
}

# Get secret value from Secrets Manager
get_secret() {
    local secret_name="$1"
    local region="$2"

    print_info "DEBUG: get_secret called for '$secret_name' in region '$region'"

    if ! get_iam_credentials; then
        print_error "get_secret: Failed to get IAM credentials"
        echo ""
        return
    fi

    local payload="{\"SecretId\":\"${secret_name}\"}"
    local response=$(aws_sign_request "POST" "secretsmanager" "$region" "/" "$payload" "secretsmanager.GetSecretValue")

    # Check for errors
    if echo "$response" | grep -q '"__type".*Exception'; then
        local error_msg
        if command -v jq >/dev/null 2>&1; then
            error_msg=$(echo "$response" | jq -r '.Message // empty')
        else
            error_msg=$(echo "$response" | grep -o '"Message" *: *"[^"]*"' | head -1 | sed 's/.*: *"\(.*\)"/\1/')
        fi
        print_error "Failed to get secret '$secret_name': $error_msg"
        print_error "DEBUG: Full response: ${response:0:500}"
        echo ""
        return
    fi

    # Extract SecretString from response
    local secret_string
    if command -v jq >/dev/null 2>&1; then
        secret_string=$(echo "$response" | jq -r '.SecretString // empty')
    else
        # Fallback to sed - extract between "SecretString":" and next "
        secret_string=$(echo "$response" | \
            grep -o '"SecretString" *: *"[^"]*"' | \
            sed 's/"SecretString" *: *"//' | \
            sed 's/"$//' | \
            sed 's/\\n/\n/g')
    fi

    if [ -n "$secret_string" ]; then
        print_info "DEBUG: Successfully retrieved secret '$secret_name' (${#secret_string} bytes)"
    else
        print_warning "DEBUG: Secret '$secret_name' retrieved but content is empty"
    fi

    echo "$secret_string"
}

# Escape string for JSON
json_escape() {
    local str="$1"
    if command -v jq >/dev/null 2>&1; then
        # jq -Rs reads input as raw string and outputs as JSON string
        # We strip the surrounding quotes to get just the escaped content
        printf '%s' "$str" | jq -Rs . | sed 's/^"//; s/"$//'
    else
        # Fallback: escape backslashes, quotes, and newlines
        printf '%s' "$str" | sed 's/\\/\\\\/g; s/"/\\"/g' | sed ':a;N;$!ba;s/\n/\\n/g'
    fi
}

# Store secret in Secrets Manager
store_secret() {
    local secret_name="$1"
    local secret_value="$2"
    local region="$3"
    local description="$4"

    if ! get_iam_credentials; then
        print_error "Cannot store secret: failed to get IAM credentials"
        return 1
    fi

    # Escape the secret value for JSON
    local escaped_value=$(json_escape "$secret_value")

    if secret_exists "$secret_name" "$region"; then
        # Update existing secret
        local payload="{\"SecretId\":\"${secret_name}\",\"SecretString\":\"${escaped_value}\"}"
        local response=$(aws_sign_request "POST" "secretsmanager" "$region" "/" "$payload" "secretsmanager.UpdateSecret")

        if echo "$response" | grep -q '"ARN"'; then
            print_success "Updated existing secret: $secret_name"
        else
            print_error "Failed to update secret: $secret_name"
            print_error "Response: $response"
            return 1
        fi
    else
        # Create new secret
        local escaped_desc=$(json_escape "$description")
        local payload="{\"Name\":\"${secret_name}\",\"Description\":\"${escaped_desc}\",\"SecretString\":\"${escaped_value}\"}"
        local response=$(aws_sign_request "POST" "secretsmanager" "$region" "/" "$payload" "secretsmanager.CreateSecret")

        if echo "$response" | grep -q '"ARN"'; then
            print_success "Created new secret: $secret_name"
        else
            print_error "Failed to create secret: $secret_name"
            print_error "Response: $response"
            return 1
        fi
    fi
}

# Validate certificate is still valid
validate_cert() {
    local cert_file="$1"
    local days_threshold=30  # Warn if cert expires in less than 30 days

    # Check if certificate is valid
    if ! openssl x509 -in "$cert_file" -noout -checkend 0 >/dev/null 2>&1; then
        return 1  # Certificate expired
    fi

    # Check if certificate expires soon
    if ! openssl x509 -in "$cert_file" -noout -checkend $((days_threshold * 86400)) >/dev/null 2>&1; then
        local expiry_date=$(openssl x509 -in "$cert_file" -noout -enddate | cut -d= -f2)
        print_warning "Certificate expires soon: $expiry_date"
    fi

    return 0  # Valid
}

echo "🔐 Generating certificates with AWS Secrets Manager integration..."

# Configuration
USE_SECRETS_MANAGER="${USE_SECRETS_MANAGER:-auto}"  # auto, true, false
SECRETS_PREFIX="${SECRETS_PREFIX:-joblet}"  # Prefix for secret names
FORCE_REGENERATE="${FORCE_REGENERATE:-false}"  # Force regenerate even if secrets exist

# Determine working directory
if [ "$(uname)" = "Linux" ]; then
    WORK_DIR="/opt/joblet"
    CONFIG_DIR="/opt/joblet/config"
    TEMPLATE_DIR="/opt/joblet/scripts"
    print_info "Using production directories: $WORK_DIR"
else
    WORK_DIR="."
    CONFIG_DIR="./config"
    TEMPLATE_DIR="./scripts"
    print_info "Using development directories: $WORK_DIR"
fi

# Create config directory if it doesn't exist
mkdir -p "$CONFIG_DIR"
chmod 755 "$CONFIG_DIR"  # Directory needs to be accessible for rnx client

# Create temporary directory for certificate generation
TEMP_DIR=$(mktemp -d)
trap "rm -rf $TEMP_DIR" EXIT

cd "$TEMP_DIR"

# Detect AWS environment
# Check if already set by parent process (exported from common-install-functions.sh)
print_info "DEBUG: Initial values - IS_EC2='$IS_EC2', EC2_REGION='$EC2_REGION'"
if [ "$IS_EC2" = "true" ] && [ -n "$EC2_REGION" ]; then
    print_info "Using EC2 environment from parent process (region: $EC2_REGION)"
elif [ -f /tmp/joblet-ec2-info ]; then
    source /tmp/joblet-ec2-info
    print_info "Using EC2 metadata from /tmp/joblet-ec2-info (IS_EC2=$IS_EC2, EC2_REGION=$EC2_REGION)"
elif detect_ec2; then
    IS_EC2="true"
    EC2_REGION=$(get_ec2_metadata "placement/region")
    print_info "EC2 auto-detected via metadata service (region: $EC2_REGION)"
else
    IS_EC2="false"
    EC2_REGION=""
    print_info "Not running on EC2"
fi
print_info "DEBUG: Final values - IS_EC2='$IS_EC2', EC2_REGION='$EC2_REGION'"

# Determine if we should use Secrets Manager
SHOULD_USE_SM="false"
print_info "USE_SECRETS_MANAGER=$USE_SECRETS_MANAGER, IS_EC2=$IS_EC2, EC2_REGION=$EC2_REGION"
if [ "$USE_SECRETS_MANAGER" = "true" ]; then
    SHOULD_USE_SM="true"
    print_info "Secrets Manager explicitly enabled"
elif [ "$USE_SECRETS_MANAGER" = "auto" ] && [ "$IS_EC2" = "true" ]; then
    SHOULD_USE_SM="true"
    print_info "Secrets Manager auto-enabled (detected EC2 environment)"
elif [ "$USE_SECRETS_MANAGER" = "false" ]; then
    SHOULD_USE_SM="false"
    print_info "Secrets Manager explicitly disabled"
else
    print_warning "Secrets Manager NOT enabled (USE_SECRETS_MANAGER=$USE_SECRETS_MANAGER, IS_EC2=$IS_EC2)"
fi

# Check Secrets Manager access if using Secrets Manager
if [ "$SHOULD_USE_SM" = "true" ]; then
    if [ -z "$EC2_REGION" ]; then
        print_error "Cannot determine AWS region"
        print_warning "Set EC2_REGION environment variable or disable Secrets Manager"
        SHOULD_USE_SM="false"
    elif ! check_secrets_manager_access; then
        print_warning "Cannot access AWS Secrets Manager, falling back to embedded certificates"
        SHOULD_USE_SM="false"
    fi
fi

# Get configuration from environment variables or defaults
SERVER_ADDRESS="${JOBLET_SERVER_ADDRESS:-}"
ADDITIONAL_NAMES="${JOBLET_ADDITIONAL_NAMES:-}"

# If no configuration provided, try to detect or use defaults
if [ -z "$SERVER_ADDRESS" ]; then
    # Try to detect current IP
    SERVER_ADDRESS=$(ip route get 1.1.1.1 2>/dev/null | grep -oP 'src \K[0-9.]+' | head -1)
    if [ -z "$SERVER_ADDRESS" ]; then
        SERVER_ADDRESS=$(ip -4 addr show | grep -oP '(?<=inet\s)\d+(\.\d+){3}' | grep -v '127.0.0.1' | head -1)
    fi
    SERVER_ADDRESS=${SERVER_ADDRESS:-127.0.0.1}
    print_warning "No JOBLET_SERVER_ADDRESS specified, using detected/default: $SERVER_ADDRESS"
fi

print_info "Certificate configuration:"
echo "  Primary Address: $SERVER_ADDRESS"
echo "  Additional Names: ${ADDITIONAL_NAMES:-none}"
echo "  Use Secrets Manager: $SHOULD_USE_SM"
if [ "$SHOULD_USE_SM" = "true" ]; then
    echo "  Secrets Manager Region: $EC2_REGION"
    echo "  Secrets Prefix: $SECRETS_PREFIX"
fi

# ============================================================================
# CA and Client Certificate Management
# ============================================================================

CA_FROM_SM="false"
CLIENT_FROM_SM="false"

if [ "$SHOULD_USE_SM" = "true" ] && [ "$FORCE_REGENERATE" != "true" ]; then
    print_info "Checking AWS Secrets Manager for existing CA and client certificates..."
    print_info "Looking for secrets with prefix: ${SECRETS_PREFIX} in region: ${EC2_REGION}"

    # Try to retrieve CA from Secrets Manager
    if secret_exists "${SECRETS_PREFIX}/ca-cert" "$EC2_REGION" && \
       secret_exists "${SECRETS_PREFIX}/ca-key" "$EC2_REGION"; then
        print_info "Found CA certificates in Secrets Manager, retrieving..."

        CA_CERT=$(get_secret "${SECRETS_PREFIX}/ca-cert" "$EC2_REGION")
        CA_KEY=$(get_secret "${SECRETS_PREFIX}/ca-key" "$EC2_REGION")

        if [ -n "$CA_CERT" ] && [ -n "$CA_KEY" ]; then
            echo "$CA_CERT" > ca-cert.pem
            echo "$CA_KEY" > ca-key.pem

            # Validate CA certificate
            if validate_cert ca-cert.pem; then
                print_success "Retrieved valid CA certificate from Secrets Manager"
                CA_FROM_SM="true"
            else
                print_warning "CA certificate from Secrets Manager is expired or invalid"
                rm -f ca-cert.pem ca-key.pem
            fi
        else
            print_warning "CA certificate content is empty from Secrets Manager"
        fi
    else
        print_warning "CA certificates not found in Secrets Manager (looked for ${SECRETS_PREFIX}/ca-cert and ${SECRETS_PREFIX}/ca-key)"
    fi

    # Try to retrieve client certificate from Secrets Manager
    if secret_exists "${SECRETS_PREFIX}/client-cert" "$EC2_REGION" && \
       secret_exists "${SECRETS_PREFIX}/client-key" "$EC2_REGION"; then
        print_info "Found client certificates in Secrets Manager, retrieving..."

        CLIENT_CERT=$(get_secret "${SECRETS_PREFIX}/client-cert" "$EC2_REGION")
        CLIENT_KEY=$(get_secret "${SECRETS_PREFIX}/client-key" "$EC2_REGION")

        if [ -n "$CLIENT_CERT" ] && [ -n "$CLIENT_KEY" ]; then
            echo "$CLIENT_CERT" > admin-client-cert.pem
            echo "$CLIENT_KEY" > admin-client-key.pem

            # Validate client certificate
            if validate_cert admin-client-cert.pem; then
                print_success "Retrieved valid client certificate from Secrets Manager"
                CLIENT_FROM_SM="true"
            else
                print_warning "Client certificate from Secrets Manager is expired or invalid"
                rm -f admin-client-cert.pem admin-client-key.pem
            fi
        else
            print_warning "Client certificate content is empty from Secrets Manager"
        fi
    else
        print_warning "Client certificates not found in Secrets Manager (looked for ${SECRETS_PREFIX}/client-cert and ${SECRETS_PREFIX}/client-key)"
    fi
fi

# Generate CA if not retrieved from Secrets Manager
if [ "$CA_FROM_SM" != "true" ]; then
    print_info "Generating new CA certificate..."
    openssl genrsa -out ca-key.pem 4096
    openssl req -new -x509 -days 1095 -key ca-key.pem -out ca-cert.pem \
        -subj "/C=US/ST=CA/L=Los Angeles/O=Joblet/OU=CA/CN=Joblet-CA"
    print_success "CA certificate generated"

    # Store in Secrets Manager if enabled
    if [ "$SHOULD_USE_SM" = "true" ]; then
        print_info "Storing CA certificate in Secrets Manager..."
        store_secret "${SECRETS_PREFIX}/ca-cert" "$(cat ca-cert.pem)" "$EC2_REGION" \
            "Joblet Root CA Certificate - shared across all instances"
        store_secret "${SECRETS_PREFIX}/ca-key" "$(cat ca-key.pem)" "$EC2_REGION" \
            "Joblet Root CA Private Key - shared across all instances"
    fi
else
    print_info "Using existing CA certificate from Secrets Manager"
fi

# Generate client certificate if not retrieved from Secrets Manager
if [ "$CLIENT_FROM_SM" != "true" ]; then
    print_info "Generating new admin client certificate..."
    openssl genrsa -out admin-client-key.pem 2048
    openssl req -new -key admin-client-key.pem -out admin-client.csr \
        -subj "/C=US/ST=CA/L=Los Angeles/O=Joblet/OU=admin/CN=admin-client"
    openssl x509 -req -days 365 -in admin-client.csr -CA ca-cert.pem -CAkey ca-key.pem \
        -CAcreateserial -out admin-client-cert.pem
    print_success "Admin client certificate generated"

    # Store in Secrets Manager if enabled
    if [ "$SHOULD_USE_SM" = "true" ]; then
        print_info "Storing client certificate in Secrets Manager..."
        store_secret "${SECRETS_PREFIX}/client-cert" "$(cat admin-client-cert.pem)" "$EC2_REGION" \
            "Joblet Admin Client Certificate - shared across all clients"
        store_secret "${SECRETS_PREFIX}/client-key" "$(cat admin-client-key.pem)" "$EC2_REGION" \
            "Joblet Admin Client Private Key - shared across all clients"
    fi
else
    print_info "Using existing client certificate from Secrets Manager"
fi

# ============================================================================
# Server Certificate Generation (Always Generated Per-Instance)
# ============================================================================

print_info "Generating server certificate (instance-specific)..."
openssl genrsa -out server-key.pem 2048
openssl req -new -key server-key.pem -out server.csr \
    -subj "/C=US/ST=CA/L=Los Angeles/O=Joblet/OU=Server/CN=joblet-server"

# Create dynamic SAN configuration
cat > server-ext.cnf << 'EOF'
[req]
req_extensions = v3_req
distinguished_name = req_distinguished_name

[req_distinguished_name]

[v3_req]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names

[alt_names]
DNS.1 = joblet
DNS.2 = localhost
DNS.3 = joblet-server
IP.1 = 127.0.0.1
IP.2 = 0.0.0.0
EOF

# Add server address and additional names
DNS_INDEX=4
IP_INDEX=3

# Add primary server address
if [[ "$SERVER_ADDRESS" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    if [ "$SERVER_ADDRESS" != "127.0.0.1" ] && [ "$SERVER_ADDRESS" != "0.0.0.0" ]; then
        echo "IP.$IP_INDEX = $SERVER_ADDRESS" >> server-ext.cnf
        IP_INDEX=$((IP_INDEX + 1))
    fi
else
    echo "DNS.$DNS_INDEX = $SERVER_ADDRESS" >> server-ext.cnf
    DNS_INDEX=$((DNS_INDEX + 1))
fi

# Add additional names
if [ -n "$ADDITIONAL_NAMES" ]; then
    IFS=',' read -ra NAMES <<< "$ADDITIONAL_NAMES"
    for name in "${NAMES[@]}"; do
        name=$(echo "$name" | xargs)
        if [ -n "$name" ]; then
            if [[ "$name" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
                echo "IP.$IP_INDEX = $name" >> server-ext.cnf
                IP_INDEX=$((IP_INDEX + 1))
            else
                echo "DNS.$DNS_INDEX = $name" >> server-ext.cnf
                DNS_INDEX=$((DNS_INDEX + 1))
            fi
        fi
    done
fi

# Generate server certificate signed by CA
openssl x509 -req -days 365 -in server.csr -CA ca-cert.pem -CAkey ca-key.pem \
    -CAcreateserial -out server-cert.pem -extensions v3_req -extfile server-ext.cnf

print_success "Server certificate generated (instance-specific)"

# Note: Server certificates are NOT stored in Secrets Manager
# Each instance has its own unique server certificate

# Function to read and indent certificate content for YAML
read_cert_for_yaml() {
    local file="$1"
    local indent="${2:-      }"  # Default to 6 spaces if not specified
    # Add proper indentation to each line
    while IFS= read -r line; do
        echo "${indent}${line}"
    done < "$file"
}

# Update server configuration with embedded certificates
print_info "Updating server configuration with embedded certificates..."
SERVER_TEMPLATE="$TEMPLATE_DIR/joblet-config-template.yml"
SERVER_CONFIG="$CONFIG_DIR/joblet-config.yml"

if [ -f "$SERVER_TEMPLATE" ]; then
    # Copy template
    cp "$SERVER_TEMPLATE" "$SERVER_CONFIG"

    # Generate unique nodeId UUID
    NODE_ID=$(uuidgen 2>/dev/null || python3 -c "import uuid; print(uuid.uuid4())" 2>/dev/null || openssl rand -hex 16 | sed 's/\(..\)/\1-/g; s/.\{8\}-\(.\{4\}\)-\(.\{4\}\)-\(.\{4\}\)-/&/; s/-$//')
    print_info "Generated nodeId: $NODE_ID"

    # Update server address and nodeId in the config
    sed -i "s/address: \".*\"/address: \"$SERVER_ADDRESS\"/" "$SERVER_CONFIG"
    sed -i "s/nodeId: \"\"/nodeId: \"$NODE_ID\"/" "$SERVER_CONFIG"

    # Configure AWS backends if running on EC2
    # Note: IS_EC2 and EC2_REGION are already set at the beginning of the script
    if [ "$IS_EC2" = "true" ]; then
        print_info "AWS EC2 detected - configuring CloudWatch and DynamoDB backends"

        # Update persist storage to CloudWatch
        sed -i 's/type: "local"/type: "cloudwatch"/' "$SERVER_CONFIG"

        # Update state backend to DynamoDB
        sed -i 's/backend: "memory"/backend: "dynamodb"/' "$SERVER_CONFIG"

        print_success "Set persist=cloudwatch, state=dynamodb"
    fi

    # Append security section with embedded certificates
    cat >> "$SERVER_CONFIG" << EOF

# Security configuration with embedded certificates
security:
  serverCert: |
$(read_cert_for_yaml server-cert.pem "    ")
  serverKey: |
$(read_cert_for_yaml server-key.pem "    ")
  caCert: |
$(read_cert_for_yaml ca-cert.pem "    ")
EOF

    print_success "Server configuration updated with embedded certificates"
else
    print_error "Server template not found: $SERVER_TEMPLATE"
fi

# Update client configuration with embedded certificates
print_info "Updating client configuration with embedded certificates..."
CLIENT_CONFIG="$CONFIG_DIR/rnx-config.yml"

# Create client configuration with embedded certificates
cat > "$CLIENT_CONFIG" << EOF
version: "3.0"

nodes:
  default:
    address: "$SERVER_ADDRESS:50051"
    nodeId: "$NODE_ID"
    cert: |
$(read_cert_for_yaml admin-client-cert.pem "      ")
    key: |
$(read_cert_for_yaml admin-client-key.pem "      ")
    ca: |
$(read_cert_for_yaml ca-cert.pem "      ")
EOF

print_success "Client configuration created with embedded certificates"

# Verify all certificates
print_info "Verifying all certificates..."
CERT_ERRORS=0

if openssl verify -CAfile ca-cert.pem server-cert.pem > /dev/null 2>&1; then
    print_success "Server certificate verified"
else
    print_error "Server certificate verification failed"
    CERT_ERRORS=$((CERT_ERRORS + 1))
fi

if openssl verify -CAfile ca-cert.pem admin-client-cert.pem > /dev/null 2>&1; then
    print_success "Admin client certificate verified"
else
    print_error "Admin client certificate verification failed"
    CERT_ERRORS=$((CERT_ERRORS + 1))
fi

# Set secure permissions on config files
print_info "Setting secure permissions on configuration files..."
chmod 600 "$SERVER_CONFIG" 2>/dev/null || true  # Server config contains private keys
chmod 600 "$CLIENT_CONFIG" 2>/dev/null || true  # Client config contains private keys

# Final status
echo
if [ $CERT_ERRORS -eq 0 ]; then
    print_success "Certificate generation and embedding completed successfully!"
else
    print_error "Certificate generation completed with $CERT_ERRORS errors"
fi

echo
print_info "📋 Configuration files updated:"
echo "  🖥️  Server Config: $SERVER_CONFIG"
echo "  📱 Client Config: $CLIENT_CONFIG"
echo "  🔐 All certificates are now embedded in configuration files"
if [ "$SHOULD_USE_SM" = "true" ]; then
    echo ""
    print_info "🔑 AWS Secrets Manager Integration:"
    if [ "$CA_FROM_SM" = "true" ]; then
        echo "  ✅ CA Certificate: Retrieved from Secrets Manager (shared)"
    else
        echo "  ✨ CA Certificate: Generated and stored in Secrets Manager (shared)"
    fi
    if [ "$CLIENT_FROM_SM" = "true" ]; then
        echo "  ✅ Client Certificate: Retrieved from Secrets Manager (shared)"
    else
        echo "  ✨ Client Certificate: Generated and stored in Secrets Manager (shared)"
    fi
    echo "  🆕 Server Certificate: Generated locally (instance-specific)"
    echo ""
    print_info "📊 Scaling Benefits:"
    echo "  • Additional instances will reuse the same CA and client certificates"
    echo "  • Clients only need one config file to connect to all instances"
    echo "  • Each server gets its own certificate for security"
fi
echo

print_info "🚀 Usage:"
echo "  Server: systemctl start joblet  # Uses embedded certs from joblet-config.yml"
echo "  CLI: rnx --config=$CLIENT_CONFIG list  # Uses embedded certs"
echo

print_info "🔧 To regenerate certificates:"
if [ "$SHOULD_USE_SM" = "true" ]; then
    echo "  # Reuse existing CA/client (default):"
    echo "  USE_SECRETS_MANAGER=true JOBLET_SERVER_ADDRESS='your-server' $0"
    echo ""
    echo "  # Force regenerate everything:"
    echo "  USE_SECRETS_MANAGER=true FORCE_REGENERATE=true JOBLET_SERVER_ADDRESS='your-server' $0"
else
    echo "  JOBLET_SERVER_ADDRESS='your-server' $0"
fi
echo

print_success "Ready to use with embedded certificates!"

# Exit with error code if there were certificate errors
exit $CERT_ERRORS
