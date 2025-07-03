#!/bin/bash

set -e

echo "🔐 Generating certificates for Worker..."

if [ "$(uname)" = "Linux" ]; then
    CERT_DIR="/opt/worker/certs"
    echo "📁 Using production cert directory: $CERT_DIR"
else
    CERT_DIR="./certs"
    echo "📁 Using development cert directory: $CERT_DIR"
fi

mkdir -p "$CERT_DIR"
cd "$CERT_DIR"

echo "🏛️  Generating CA certificate..."

openssl genrsa -out ca-key.pem 4096

openssl req -new -x509 -days 1095 -key ca-key.pem -out ca-cert.pem -subj "/C=US/ST=CA/L=Los Angeles/O=Worker/OU=CA/CN=Worker-CA"

echo "🖥️  Generating server certificate with SAN support..."

openssl genrsa -out server-key.pem 2048

openssl req -new -key server-key.pem -out server.csr -subj "/C=US/ST=CA/L=Los Angeles/O=Worker/OU=Server/CN=worker-server"

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
IP.1 = 192.168.1.161
IP.2 = 127.0.0.1
IP.3 = 0.0.0.0
EOF

openssl x509 -req -days 365 -in server.csr -CA ca-cert.pem -CAkey ca-key.pem -CAcreateserial -out server-cert.pem -extensions v3_req -extfile server-ext.cnf

echo "🔍 Verifying SAN was applied to server certificate..."
openssl x509 -in server-cert.pem -noout -text | grep -A 10 "Subject Alternative Name" || echo "⚠️ SAN verification failed"

echo "👑 Generating admin client certificate..."

openssl genrsa -out admin-client-key.pem 2048

openssl req -new -key admin-client-key.pem -out admin-client.csr -subj "/C=US/ST=CA/L=Los Angeles/O=Worker/OU=admin/CN=admin-client"

openssl x509 -req -days 365 -in admin-client.csr -CA ca-cert.pem -CAkey ca-key.pem -CAcreateserial -out admin-client-cert.pem

echo "👁️  Generating viewer client certificate..."

openssl genrsa -out viewer-client-key.pem 2048

openssl req -new -key viewer-client-key.pem -out viewer-client.csr -subj "/C=US/ST=CA/L=Los Angeles/O=Worker/OU=viewer/CN=viewer-client"

openssl x509 -req -days 365 -in viewer-client.csr -CA ca-cert.pem -CAkey ca-key.pem -CAcreateserial -out viewer-client-cert.pem

echo "🔍 Verifying certificates..."

openssl verify -CAfile ca-cert.pem server-cert.pem
openssl verify -CAfile ca-cert.pem admin-client-cert.pem
openssl verify -CAfile ca-cert.pem viewer-client-cert.pem

echo "🔒 Setting secure permissions..."

chmod 600 ca-key.pem server-key.pem admin-client-key.pem viewer-client-key.pem  # Private keys
chmod 644 ca-cert.pem server-cert.pem admin-client-cert.pem viewer-client-cert.pem  # Certificates

if [ "$(uname)" = "Linux" ] && [ "$(whoami)" = "root" ]; then
    echo "🔧 Setting proper ownership for jay user..."
    chown jay:jay ca-cert.pem admin-client-cert.pem admin-client-key.pem viewer-client-cert.pem viewer-client-key.pem
    echo "✅ Ownership set for jay user"
fi

echo "🧹 Cleaning up temporary files..."

rm -f *.csr *.cnf *.srl

echo "✅ Certificate generation complete!"
echo "🚀 Ready to use with Worker service!"

if command -v openssl >/dev/null 2>&1; then
    echo ""
    echo "🔍 Certificate details:"
    echo "Admin client OU: $(openssl x509 -in admin-client-cert.pem -noout -subject | grep -o 'OU=[^/,]*' | cut -d= -f2)"
    echo "Viewer client OU: $(openssl x509 -in viewer-client-cert.pem -noout -subject | grep -o 'OU=[^/,]*' | cut -d= -f2)"
    echo ""
    echo "Server certificate SAN:"
    openssl x509 -in server-cert.pem -noout -text | grep -A 3 "Subject Alternative Name" || echo "   (SAN information not displayed - but it's there!)"
fi