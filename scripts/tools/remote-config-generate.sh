#!/bin/bash
set -e

# JOBLET_HOME defines the installation directory (default: /opt/joblet)
JOBLET_HOME="${JOBLET_HOME:-/opt/joblet}"

# Remote configuration generation script
# Usage: ./scripts/remote-config-generate.sh [REMOTE_HOST]

REMOTE_HOST=${1:-$REMOTE_HOST}

if [ -z "$REMOTE_HOST" ]; then
    echo "❌ REMOTE_HOST not specified"
    echo "Usage: $0 [REMOTE_HOST]"
    exit 1
fi

echo "🔐 Generating configuration on $REMOTE_HOST with embedded certificates..."

if [ ! -f ./scripts/certs_gen_embedded.sh ]; then
    echo "❌ ./scripts/certs_gen_embedded.sh script not found"
    exit 1
fi

echo "📤 Uploading certificate generation script..."
scp ./scripts/certs_gen_embedded.sh $REMOTE_USER@$REMOTE_HOST:/tmp/

echo "🏗️  Generating configuration with embedded certificates on remote server..."
echo "⚠️  Note: This requires passwordless sudo to be configured"
ssh $REMOTE_USER@$REMOTE_HOST "
    chmod +x /tmp/certs_gen_embedded.sh
    sudo JOBLET_HOME=${JOBLET_HOME} JOBLET_SERVER_ADDRESS=$REMOTE_HOST /tmp/certs_gen_embedded.sh
    echo ""
    echo "📋 Configuration files created:"
    sudo ls -la ${JOBLET_HOME}/config/ 2>/dev/null || echo "No configuration found"
    rm -f /tmp/certs_gen_embedded.sh
"

echo "✅ Remote configuration generated with embedded certificates!"
