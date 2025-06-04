#!/bin/bash

set -e

echo "🔐 Generating certificates for Job Worker..."

if [ "$(uname)" = "Linux" ]; then
    CERT_DIR="/opt/job-worker/certs"
    echo "📁 Using production cert directory: $CERT_DIR"
else
    CERT_DIR="./certs"
    echo "📁 Using development cert directory: $CERT_DIR"
fi

mkdir -p "$CERT_DIR"
cd "$CERT_DIR"

echo "🏛️  Generating CA certificate..."

openssl genrsa -out ca-key.pem 4096

openssl req -new -x509 -days 1095 -key ca-key.pem -out ca-cert.pem -subj "/C=US/ST=CA/L=Los Angeles/O=JobWorker/OU=CA/CN=JobWorker-CA"

echo "🖥️  Generating server certificate..."

openssl genrsa -out server-key.pem 2048

openssl req -new -key server-key.pem -out server.csr -subj "/C=US/ST=CA/L=Los Angeles/O=JobWorker/OU=Server/CN=job-worker-server"

openssl x509 -req -days 365 -in server.csr -CA ca-cert.pem -CAkey ca-key.pem -CAcreateserial -out server-cert.pem

echo "👑 Generating admin client certificate..."

openssl genrsa -out admin-client-key.pem 2048

openssl req -new -key admin-client-key.pem -out admin-client.csr -subj "/C=US/ST=CA/L=Los Angeles/O=JobWorker/OU=admin/CN=admin-client"

openssl x509 -req -days 365 -in admin-client.csr -CA ca-cert.pem -CAkey ca-key.pem -CAcreateserial -out admin-client-cert.pem

echo "👁️  Generating viewer client certificate..."

openssl genrsa -out viewer-client-key.pem 2048

openssl req -new -key viewer-client-key.pem -out viewer-client.csr -subj "/C=US/ST=CA/L=Los Angeles/O=JobWorker/OU=viewer/CN=viewer-client"

openssl x509 -req -days 365 -in viewer-client.csr -CA ca-cert.pem -CAkey ca-key.pem -CAcreateserial -out viewer-client-cert.pem

echo "🔍 Verifying certificates..."

openssl verify -CAfile ca-cert.pem server-cert.pem
openssl verify -CAfile ca-cert.pem admin-client-cert.pem
openssl verify -CAfile ca-cert.pem viewer-client-cert.pem

echo "🔒 Setting secure permissions..."

chmod 600 *.pem  # All private keys and certs readable only by owner
chmod 644 ca-cert.pem server-cert.pem admin-client-cert.pem viewer-client-cert.pem  # Certs can be world-readable

echo "🧹 Cleaning up temporary files..."

rm -f *.csr *.srl

echo "✅ Certificate generation complete!"
echo ""
echo "📋 Generated files:"
echo "   ca-cert.pem           - CA certificate (public)"
echo "   ca-key.pem            - CA private key (keep secure!)"
echo "   server-cert.pem       - Server certificate"
echo "   server-key.pem        - Server private key"
echo "   admin-client-cert.pem - Admin client certificate (OU=admin)"
echo "   admin-client-key.pem  - Admin client private key"
echo "   viewer-client-cert.pem- Viewer client certificate (OU=viewer)"
echo "   viewer-client-key.pem - Viewer client private key"
echo ""
echo "🚀 Ready to use with Job Worker service!"

if command -v openssl >/dev/null 2>&1; then
    echo ""
    echo "🔍 Certificate details:"
    echo "Admin client OU: $(openssl x509 -in admin-client-cert.pem -noout -subject | grep -o 'OU=[^/]*' | cut -d= -f2)"
    echo "Viewer client OU: $(openssl x509 -in viewer-client-cert.pem -noout -subject | grep -o 'OU=[^/]*' | cut -d= -f2)"
fi