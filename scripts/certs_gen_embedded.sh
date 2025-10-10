#!/bin/bash

set -e

# ============================================================================
# Configuration Constants
# ============================================================================

# AWS EC2 Metadata Service (IMDS) Configuration
readonly AWS_IMDS_ENDPOINT="${AWS_IMDS_ENDPOINT:-http://169.254.169.254}"
readonly AWS_IMDS_TOKEN_TTL="${AWS_IMDS_TOKEN_TTL:-21600}"  # 6 hours in seconds
readonly AWS_IMDS_TIMEOUT="${AWS_IMDS_TIMEOUT:-2}"          # Timeout in seconds

# Detection Timeout Configuration
readonly DETECTION_TIMEOUT="${DETECTION_TIMEOUT:-2}"        # General detection timeout in seconds

# System Paths for Cloud Detection
readonly HYPERVISOR_UUID_PATH="${HYPERVISOR_UUID_PATH:-/sys/hypervisor/uuid}"
readonly DMI_VENDOR_PATHS="${DMI_VENDOR_PATHS:-/sys/class/dmi/id/sys_vendor /sys/class/dmi/id/bios_vendor /sys/class/dmi/id/board_vendor}"

# Environment Type Constants
readonly ENV_TYPE_AWS_EC2="AWS_EC2"
readonly ENV_TYPE_ON_PREMISES="ON_PREMISES"

# ============================================================================
# Colors for output
# ============================================================================
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

# Generate viewer client certificate
print_info "Generating viewer client certificate..."
openssl genrsa -out viewer-client-key.pem 2048
openssl req -new -key viewer-client-key.pem -out viewer-client.csr \
    -subj "/C=US/ST=CA/L=Los Angeles/O=Joblet/OU=viewer/CN=viewer-client"
openssl x509 -req -days 365 -in viewer-client.csr -CA ca-cert.pem -CAkey ca-key.pem \
    -CAcreateserial -out viewer-client-cert.pem
print_success "Viewer client certificate generated"

# Function to read and indent certificate content for YAML
read_cert_for_yaml() {
    local file="$1"
    local indent="${2:-      }"  # Default to 6 spaces if not specified
    # Add proper indentation to each line
    while IFS= read -r line; do
        echo "${indent}${line}"
    done < "$file"
}

# AWS EC2 Detection and Auto-Configuration
detect_aws_ec2() {
    print_info "Detecting cloud environment..."

    # Try IMDSv2 (most reliable method for EC2)
    local TOKEN=$(curl -X PUT -H "X-aws-ec2-metadata-token-ttl-seconds: ${AWS_IMDS_TOKEN_TTL}" \
            --max-time ${AWS_IMDS_TIMEOUT} --silent --fail \
            ${AWS_IMDS_ENDPOINT}/latest/api/token 2>/dev/null)

    if [ -n "$TOKEN" ]; then
        local INSTANCE_ID=$(curl -H "X-aws-ec2-metadata-token: $TOKEN" \
                      --max-time ${AWS_IMDS_TIMEOUT} --silent --fail \
                      ${AWS_IMDS_ENDPOINT}/latest/meta-data/instance-id 2>/dev/null)

        if [ -n "$INSTANCE_ID" ]; then
            local REGION=$(curl -H "X-aws-ec2-metadata-token: $TOKEN" \
                          --max-time ${AWS_IMDS_TIMEOUT} --silent --fail \
                          ${AWS_IMDS_ENDPOINT}/latest/meta-data/placement/region 2>/dev/null)

            echo "${ENV_TYPE_AWS_EC2}|$INSTANCE_ID|$REGION"
            return 0
        fi
    fi

    # Fallback: Check system files
    if [ -f "${HYPERVISOR_UUID_PATH}" ] && grep -q "^ec2" "${HYPERVISOR_UUID_PATH}" 2>/dev/null; then
        echo "${ENV_TYPE_AWS_EC2}|unknown|"
        return 0
    fi

    echo "${ENV_TYPE_ON_PREMISES}||"
    return 1
}

# Detect environment
ENV_INFO=$(detect_aws_ec2)
ENV_TYPE=$(echo "$ENV_INFO" | cut -d'|' -f1)
INSTANCE_ID=$(echo "$ENV_INFO" | cut -d'|' -f2)
AWS_REGION=$(echo "$ENV_INFO" | cut -d'|' -f3)

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

    # Auto-configure based on environment
    if [ "$ENV_TYPE" = "${ENV_TYPE_AWS_EC2}" ]; then
        print_success "AWS EC2 environment detected!"
        echo "  Instance ID: ${INSTANCE_ID:-unknown}"
        echo "  Region: ${AWS_REGION:-auto-detect}"

        print_info "Applying cloud-optimized configuration..."

        # Disable local log persistence (optimize EBS usage)
        sed -i 's/^\(\s*\)enabled: true\s*# Enable local disk persistence.*/\1enabled: false                # Disabled for EC2 (using CloudWatch)/' "$SERVER_CONFIG"

        # Enable CloudWatch
        sed -i 's/^\(\s*\)enabled: false\s*# Disabled by default (auto-enabled for EC2.*/\1enabled: true                # Auto-enabled for EC2 deployment/' "$SERVER_CONFIG"

        # Set region if detected
        if [ -n "$AWS_REGION" ]; then
            sed -i "s|^\(\s*\)region: \"\".*# AWS region|\1region: \"$AWS_REGION\"         # Auto-detected from EC2 metadata|" "$SERVER_CONFIG"
        fi

        # Update log group with instance ID
        if [ -n "$INSTANCE_ID" ]; then
            sed -i "s|log_group: \"/aws/joblet/\".*|log_group: \"/aws/joblet/$INSTANCE_ID\" # Instance-specific log group|" "$SERVER_CONFIG"
        fi

        print_success "Cloud-optimized configuration applied:"
        echo "  ✓ Local disk logs: DISABLED (optimize EBS usage)"
        echo "  ✓ CloudWatch Logs: ENABLED"
        echo "  ✓ Region: ${AWS_REGION:-auto-detect at runtime}"
        echo "  ✓ Log Group: /aws/joblet/${INSTANCE_ID:-[instance-id]}"
    else
        print_info "On-premises environment detected"
        print_info "Using standard configuration:"
        echo "  ✓ Local disk logs: ENABLED (/opt/joblet/logs)"
        echo "  ✗ CloudWatch Logs: DISABLED"
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

  viewer:
    address: "$SERVER_ADDRESS:50051"
    nodeId: "$NODE_ID"
    cert: |
$(read_cert_for_yaml viewer-client-cert.pem "      ")
    key: |
$(read_cert_for_yaml viewer-client-key.pem "      ")
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

if openssl verify -CAfile ca-cert.pem viewer-client-cert.pem > /dev/null 2>&1; then
    print_success "Viewer client certificate verified"
else
    print_error "Viewer client certificate verification failed"
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