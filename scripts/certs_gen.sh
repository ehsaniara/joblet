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

echo "🔐 Generating certificates for Worker..."

# Determine certificate directory
if [ "$(uname)" = "Linux" ]; then
    CERT_DIR="/opt/worker/certs"
    print_info "Using production cert directory: $CERT_DIR"
else
    CERT_DIR="./certs"
    print_info "Using development cert directory: $CERT_DIR"
fi

mkdir -p "$CERT_DIR"
cd "$CERT_DIR"

# Get configuration from environment variables or defaults
SERVER_ADDRESS="${WORKER_SERVER_ADDRESS:-}"
ADDITIONAL_NAMES="${WORKER_ADDITIONAL_NAMES:-}"

# If no configuration provided, try to detect or use defaults
if [ -z "$SERVER_ADDRESS" ]; then
    # Try to detect current IP
    SERVER_ADDRESS=$(ip route get 1.1.1.1 2>/dev/null | grep -oP 'src \K[0-9.]+' | head -1)
    if [ -z "$SERVER_ADDRESS" ]; then
        SERVER_ADDRESS=$(ip -4 addr show | grep -oP '(?<=inet\s)\d+(\.\d+){3}' | grep -v '127.0.0.1' | head -1)
    fi
    SERVER_ADDRESS=${SERVER_ADDRESS:-127.0.0.1}
    print_warning "No WORKER_SERVER_ADDRESS specified, using detected/default: $SERVER_ADDRESS"
fi

print_info "Certificate configuration:"
echo "  Primary Address: $SERVER_ADDRESS"
echo "  Additional Names: ${ADDITIONAL_NAMES:-none}"

# Auto-detect additional IPs for fallback
print_info "Detecting additional IP addresses for certificate..."
DETECTED_IPS=$(ip -4 addr show 2>/dev/null | grep -oP '(?<=inet\s)\d+(\.\d+){3}' | grep -v '127.0.0.1' | sort -u | tr '\n' ' ')
print_info "Detected IPs: ${DETECTED_IPS:-none}"

# Generate CA certificate
print_info "Generating CA certificate..."
if [ ! -f "ca-key.pem" ] || [ ! -f "ca-cert.pem" ]; then
    openssl genrsa -out ca-key.pem 4096
    openssl req -new -x509 -days 1095 -key ca-key.pem -out ca-cert.pem \
        -subj "/C=US/ST=CA/L=Los Angeles/O=Worker/OU=CA/CN=Worker-CA"
    print_success "CA certificate generated"
else
    print_info "CA certificate already exists, skipping generation"
fi

# Generate server certificate
print_info "Generating server certificate..."
openssl genrsa -out server-key.pem 2048
openssl req -new -key server-key.pem -out server.csr \
    -subj "/C=US/ST=CA/L=Los Angeles/O=Worker/OU=Server/CN=worker-server"

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
DNS.1 = worker
DNS.2 = localhost
DNS.3 = worker-server
IP.1 = 127.0.0.1
IP.2 = 0.0.0.0
EOF

# Initialize counters
DNS_INDEX=4
IP_INDEX=3

# Add hostname variants
HOSTNAME=$(hostname 2>/dev/null || echo "worker-host")
HOSTNAME_FQDN=$(hostname -f 2>/dev/null || echo "$HOSTNAME.local")

echo "DNS.$DNS_INDEX = $HOSTNAME" >> server-ext.cnf
DNS_INDEX=$((DNS_INDEX + 1))

if [ "$HOSTNAME_FQDN" != "$HOSTNAME" ]; then
    echo "DNS.$DNS_INDEX = $HOSTNAME_FQDN" >> server-ext.cnf
    DNS_INDEX=$((DNS_INDEX + 1))
fi

# Add primary server address
if [[ "$SERVER_ADDRESS" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    # It's an IP address
    if [ "$SERVER_ADDRESS" != "127.0.0.1" ] && [ "$SERVER_ADDRESS" != "0.0.0.0" ]; then
        echo "IP.$IP_INDEX = $SERVER_ADDRESS" >> server-ext.cnf
        IP_INDEX=$((IP_INDEX + 1))
    fi
else
    # It's a hostname
    echo "DNS.$DNS_INDEX = $SERVER_ADDRESS" >> server-ext.cnf
    DNS_INDEX=$((DNS_INDEX + 1))
fi

# Add additional names from configuration
if [ -n "$ADDITIONAL_NAMES" ]; then
    print_info "Adding additional names from configuration..."
    IFS=',' read -ra NAMES <<< "$ADDITIONAL_NAMES"
    for name in "${NAMES[@]}"; do
        # Trim whitespace
        name=$(echo "$name" | xargs)
        if [ -n "$name" ]; then
            if [[ "$name" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
                # It's an IP address
                echo "IP.$IP_INDEX = $name" >> server-ext.cnf
                IP_INDEX=$((IP_INDEX + 1))
            else
                # It's a hostname
                echo "DNS.$DNS_INDEX = $name" >> server-ext.cnf
                DNS_INDEX=$((DNS_INDEX + 1))
            fi
        fi
    done
fi

# Add detected IPs as fallback (excluding already added ones)
for ip in $DETECTED_IPS; do
    if [ "$ip" != "$SERVER_ADDRESS" ] && ! grep -q "$ip" server-ext.cnf; then
        echo "IP.$IP_INDEX = $ip" >> server-ext.cnf
        IP_INDEX=$((IP_INDEX + 1))
    fi
done

# Add common private network IPs as fallback
FALLBACK_IPS="192.168.1.1 10.0.0.1 172.16.0.1"
for ip in $FALLBACK_IPS; do
    if ! grep -q "$ip" server-ext.cnf; then
        echo "IP.$IP_INDEX = $ip" >> server-ext.cnf
        IP_INDEX=$((IP_INDEX + 1))
    fi
done

print_info "Certificate will include:"
print_info "🌐 DNS Names:"
grep "DNS\." server-ext.cnf | while read line; do
    echo "   $(echo $line | cut -d'=' -f2 | xargs)"
done
print_info "🔢 IP Addresses:"
grep "IP\." server-ext.cnf | while read line; do
    echo "   $(echo $line | cut -d'=' -f2 | xargs)"
done

# Generate server certificate
openssl x509 -req -days 365 -in server.csr -CA ca-cert.pem -CAkey ca-key.pem \
    -CAcreateserial -out server-cert.pem -extensions v3_req -extfile server-ext.cnf

print_info "Verifying server certificate..."
if openssl x509 -in server-cert.pem -noout -text | grep -A 10 "Subject Alternative Name" > /dev/null; then
    print_success "Server certificate contains Subject Alternative Names"
else
    print_warning "SAN verification failed - certificate may not work with all hostnames"
fi

# Generate admin client certificate
print_info "Generating admin client certificate..."
if [ ! -f "admin-client-key.pem" ] || [ ! -f "admin-client-cert.pem" ]; then
    openssl genrsa -out admin-client-key.pem 2048
    openssl req -new -key admin-client-key.pem -out admin-client.csr \
        -subj "/C=US/ST=CA/L=Los Angeles/O=Worker/OU=admin/CN=admin-client"
    openssl x509 -req -days 365 -in admin-client.csr -CA ca-cert.pem -CAkey ca-key.pem \
        -CAcreateserial -out admin-client-cert.pem
    print_success "Admin client certificate generated"
else
    print_info "Admin client certificate already exists, skipping generation"
fi

# Generate viewer client certificate
print_info "Generating viewer client certificate..."
if [ ! -f "viewer-client-key.pem" ] || [ ! -f "viewer-client-cert.pem" ]; then
    openssl genrsa -out viewer-client-key.pem 2048
    openssl req -new -key viewer-client-key.pem -out viewer-client.csr \
        -subj "/C=US/ST=CA/L=Los Angeles/O/Worker/OU=viewer/CN=viewer-client"
    openssl x509 -req -days 365 -in viewer-client.csr -CA ca-cert.pem -CAkey ca-key.pem \
        -CAcreateserial -out viewer-client-cert.pem
    print_success "Viewer client certificate generated"
else
    print_info "Viewer client certificate already exists, skipping generation"
fi

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

# Set secure permissions
print_info "Setting secure permissions..."
chmod 600 ca-key.pem server-key.pem admin-client-key.pem viewer-client-key.pem  # Private keys
chmod 644 ca-cert.pem server-cert.pem admin-client-cert.pem viewer-client-cert.pem  # Certificates

# Create client certificate symlinks for easier access
print_info "Creating convenient symlinks..."
ln -sf admin-client-cert.pem client-cert.pem
ln -sf admin-client-key.pem client-key.pem
chmod 644 client-cert.pem
chmod 600 client-key.pem

# Set ownership if running as root on Linux
if [ "$(uname)" = "Linux" ] && [ "$(whoami)" = "root" ]; then
    print_info "Setting proper ownership..."

    # Try to set ownership to a non-root user if they exist
    for user in jay ubuntu worker; do
        if id "$user" >/dev/null 2>&1; then
            chown $user:$user ca-cert.pem admin-client-cert.pem admin-client-key.pem \
                viewer-client-cert.pem viewer-client-key.pem client-cert.pem client-key.pem 2>/dev/null || true
            print_success "Ownership set for $user user"
            break
        fi
    done
fi

# Clean up temporary files
print_info "Cleaning up temporary files..."
rm -f *.csr *.cnf *.srl

# Final status
echo
if [ $CERT_ERRORS -eq 0 ]; then
    print_success "Certificate generation completed successfully!"
else
    print_error "Certificate generation completed with $CERT_ERRORS errors"
fi

echo
print_info "📋 Generated certificates:"
echo "  🏛️  CA Certificate: ca-cert.pem"
echo "  🖥️  Server Certificate: server-cert.pem"
echo "  🔑 Server Private Key: server-key.pem"
echo "  👑 Admin Client Certificate: admin-client-cert.pem"
echo "  🔑 Admin Client Private Key: admin-client-key.pem"
echo "  👁️  Viewer Client Certificate: viewer-client-cert.pem"
echo "  🔑 Viewer Client Private Key: viewer-client-key.pem"
echo "  🔗 Client Certificate Symlink: client-cert.pem → admin-client-cert.pem"
echo "  🔗 Client Key Symlink: client-key.pem → admin-client-key.pem"
echo

print_info "🚀 Usage:"
echo "  Server uses: server-cert.pem, server-key.pem, ca-cert.pem"
echo "  Admin client uses: admin-client-cert.pem, admin-client-key.pem, ca-cert.pem"
echo "  Viewer client uses: viewer-client-cert.pem, viewer-client-key.pem, ca-cert.pem"
echo "  CLI convenience: client-cert.pem, client-key.pem, ca-cert.pem"
echo

print_info "🔧 To regenerate certificates:"
echo "  WORKER_SERVER_ADDRESS='your-server' WORKER_ADDITIONAL_NAMES='name1,name2' $0"
echo

# Display certificate details if OpenSSL is available
if command -v openssl >/dev/null 2>&1; then
    echo
    print_info "🔍 Certificate Details:"
    echo "Server Certificate Valid For:"
    openssl x509 -in server-cert.pem -noout -text | grep -A 20 "Subject Alternative Name" | grep -E "(DNS|IP Address)" | head -10 | sed 's/^[ \t]*/  /' || echo "  (SAN details not displayed)"
    echo
    echo "Client Certificate Types:"
    echo "  Admin: $(openssl x509 -in admin-client-cert.pem -noout -subject | grep -o 'OU=[^/,]*' | cut -d= -f2)"
    echo "  Viewer: $(openssl x509 -in viewer-client-cert.pem -noout -subject | grep -o 'OU=[^/,]*' | cut -d= -f2)"
    echo
    echo "Certificate Validity:"
    echo "  Server: $(openssl x509 -in server-cert.pem -noout -dates | grep notAfter | cut -d= -f2)"
    echo "  CA: $(openssl x509 -in ca-cert.pem -noout -dates | grep notAfter | cut -d= -f2)"
fi

print_success "Ready to use with Worker service!"

# Exit with error code if there were certificate errors
exit $CERT_ERRORS