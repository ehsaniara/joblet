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

# Detect if running on AWS EC2 by querying metadata service
detect_ec2() {
    # Try to get EC2 instance ID with short timeout
    # Use IMDSv2 (more secure) with fallback to IMDSv1
    local token
    token=$(curl -s -m 2 -X PUT "http://169.254.169.254/latest/api/token" \
        -H "X-aws-ec2-metadata-token-ttl-seconds: 21600" 2>/dev/null || echo "")

    local instance_id
    if [ -n "$token" ]; then
        # IMDSv2
        instance_id=$(curl -s -m 2 -H "X-aws-ec2-metadata-token: $token" \
            "http://169.254.169.254/latest/meta-data/instance-id" 2>/dev/null || echo "")
    else
        # Fallback to IMDSv1
        instance_id=$(curl -s -m 2 "http://169.254.169.254/latest/meta-data/instance-id" 2>/dev/null || echo "")
    fi

    if [ -n "$instance_id" ] && [ "$instance_id" != "" ]; then
        return 0  # Is EC2
    else
        return 1  # Not EC2
    fi
}

# Get EC2 metadata value
get_ec2_metadata() {
    local path="$1"
    local token
    token=$(curl -s -m 2 -X PUT "http://169.254.169.254/latest/api/token" \
        -H "X-aws-ec2-metadata-token-ttl-seconds: 21600" 2>/dev/null || echo "")

    if [ -n "$token" ]; then
        # IMDSv2
        curl -s -m 2 -H "X-aws-ec2-metadata-token: $token" \
            "http://169.254.169.254/latest/meta-data/$path" 2>/dev/null || echo ""
    else
        # Fallback to IMDSv1
        curl -s -m 2 "http://169.254.169.254/latest/meta-data/$path" 2>/dev/null || echo ""
    fi
}

echo "🔐 Generating certificates and embedding them in config files..."

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

# Create temporary directory for certificate generation
TEMP_DIR=$(mktemp -d)
trap "rm -rf $TEMP_DIR" EXIT

cd "$TEMP_DIR"

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

# Generate CA certificate
print_info "Generating CA certificate..."
openssl genrsa -out ca-key.pem 4096
openssl req -new -x509 -days 1095 -key ca-key.pem -out ca-cert.pem \
    -subj "/C=US/ST=CA/L=Los Angeles/O=Joblet/OU=CA/CN=Joblet-CA"
print_success "CA certificate generated"

# Generate server certificate
print_info "Generating server certificate..."
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

# Generate server certificate
openssl x509 -req -days 365 -in server.csr -CA ca-cert.pem -CAkey ca-key.pem \
    -CAcreateserial -out server-cert.pem -extensions v3_req -extfile server-ext.cnf

print_success "Server certificate generated"

# Generate admin client certificate
print_info "Generating admin client certificate..."
openssl genrsa -out admin-client-key.pem 2048
openssl req -new -key admin-client-key.pem -out admin-client.csr \
    -subj "/C=US/ST=CA/L=Los Angeles/O=Joblet/OU=admin/CN=admin-client"
openssl x509 -req -days 365 -in admin-client.csr -CA ca-cert.pem -CAkey ca-key.pem \
    -CAcreateserial -out admin-client-cert.pem
print_success "Admin client certificate generated"

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

    # Check if running on AWS EC2 and configure CloudWatch backend automatically
    IS_EC2="false"
    EC2_REGION=""
    EC2_INSTANCE_ID=""

    # Method 1: Check for pre-created EC2 info file (from ec2-user-data.sh)
    if [ -f /tmp/joblet-ec2-info ]; then
        source /tmp/joblet-ec2-info
        print_info "Using EC2 metadata from /tmp/joblet-ec2-info"
    # Method 2: Direct EC2 detection (for manual package installations on EC2)
    elif detect_ec2; then
        IS_EC2="true"
        EC2_REGION=$(get_ec2_metadata "placement/region")
        EC2_INSTANCE_ID=$(get_ec2_metadata "instance-id")
        print_info "EC2 auto-detected via metadata service"
    fi

    # Configure CloudWatch if running on EC2
    if [ "$IS_EC2" = "true" ]; then
        print_info "🌩️  AWS EC2 detected - configuring CloudWatch backend"
        if [ -n "$EC2_INSTANCE_ID" ]; then
            echo "  Instance ID: $EC2_INSTANCE_ID"
        fi
        if [ -n "$EC2_REGION" ]; then
            echo "  Region: $EC2_REGION"
        fi

        # Update persist storage type to cloudwatch
        sed -i '/persist:/,/^  storage:/ {
            /type: "local"/ s/type: "local"/type: "cloudwatch"/
        }' "$SERVER_CONFIG"

        # Comment out local storage configuration (lines between "local:" and next top-level key)
        awk '
        /^    local:/ { in_local=1; print "    # local:"; next }
        in_local && /^    [a-z]/ && !/^      / { in_local=0 }
        in_local { print "    #" $0; next }
        { print }
        ' "$SERVER_CONFIG" > "${SERVER_CONFIG}.tmp" && mv "${SERVER_CONFIG}.tmp" "$SERVER_CONFIG"

        # Set region if detected
        if [ -n "$EC2_REGION" ]; then
            sed -i "/region: \"\"/s/region: \"\"/region: \"$EC2_REGION\"/" "$SERVER_CONFIG"
        fi

        print_success "CloudWatch backend configured (region: ${EC2_REGION:-auto-detect})"
        print_info "Logs will be stored in CloudWatch Logs under /joblet/$NODE_ID/jobs/"

        # Update state backend to DynamoDB
        print_info "💾  Configuring DynamoDB backend for state persistence"
        sed -i '/^state:/,/^[^ ]/ {
            /backend: "memory"/ s/backend: "memory"/backend: "dynamodb"/
        }' "$SERVER_CONFIG"

        # Add DynamoDB storage configuration if not present
        if ! grep -q "storage:" "$SERVER_CONFIG" | grep -A 20 "^state:" | grep -q "dynamodb:"; then
            # Add storage section under state
            sed -i '/^state:/a\
  storage:\
    dynamodb:\
      region: ""  # Auto-detect from EC2 metadata\
      table_name: "joblet-jobs"\
      ttl_enabled: true\
      ttl_days: 30' "$SERVER_CONFIG"
        fi

        # Set region in DynamoDB config if detected
        if [ -n "$EC2_REGION" ]; then
            sed -i "/^state:/,/^[^ ]/ {
                /region: \"\"/ s/region: \"\"/region: \"$EC2_REGION\"/
            }" "$SERVER_CONFIG"
        fi

        print_success "DynamoDB backend configured (region: ${EC2_REGION:-auto-detect}, table: joblet-jobs)"
        print_info "Job state will persist in DynamoDB and survive restarts"
        print_warning "Ensure IAM role with CloudWatch Logs + DynamoDB permissions is attached to this EC2 instance"
    else
        print_info "Not running on AWS EC2 - using local filesystem storage"
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
CLIENT_TEMPLATE="$TEMPLATE_DIR/rnx-config-template.yml"
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
echo "  🗑️  No separate certificate files needed"
echo

print_info "🚀 Usage:"
echo "  Server: systemctl start joblet  # Uses embedded certs from joblet-config.yml"
echo "  CLI: rnx --config=$CLIENT_CONFIG list  # Uses embedded certs"
echo

print_info "🔧 To regenerate certificates:"
echo "  JOBLET_SERVER_ADDRESS='your-server' $0"
echo

print_success "Ready to use with embedded certificates!"

# Exit with error code if there were certificate errors
exit $CERT_ERRORS