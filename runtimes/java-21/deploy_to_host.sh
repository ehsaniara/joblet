#!/bin/bash
# Deploy Java 21 Runtime to Joblet Host

set -e

HOST=${1:-"192.168.1.161"}
USER=${2:-"jay"}
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

log() { echo -e "${GREEN}[$(date +'%H:%M:%S')] $1${NC}"; }
info() { echo -e "${BLUE}[INFO] $1${NC}"; }
warn() { echo -e "${YELLOW}[WARN] $1${NC}"; }

log "☕ Deploying Java 21 Runtime to $USER@$HOST"

# Check if setup script exists
if [[ ! -f "$SCRIPT_DIR/setup_java_21.sh" ]]; then
    echo "❌ Setup script not found: $SCRIPT_DIR/setup_java_21.sh"
    exit 1
fi

# Copy setup script to host
log "📤 Copying setup script to host..."
scp "$SCRIPT_DIR/setup_java_21.sh" "$USER@$HOST:/tmp/"

# Make setup script executable on host
log "🔧 Making setup script executable..."
ssh "$USER@$HOST" 'chmod +x /tmp/setup_java_21.sh'

# Remove existing runtime if it exists
log "🗑️  Removing existing Java 21 runtime if present..."
ssh "$USER@$HOST" 'sudo rm -rf /opt/joblet/runtimes/java/java-21 /tmp/java-21-runtime.tar.gz' || warn "No existing runtime to remove"

# Run setup script on host
log "🏗️  Running fresh setup on host (this will take 5-10 minutes)..."
ssh "$USER@$HOST" 'sudo /tmp/setup_java_21.sh'

# Verify installation
log "🧪 Verifying installation..."
ssh "$USER@$HOST" 'rnx runtime list' || warn "Runtime list failed - joblet might not be running"

# Test basic functionality
log "✅ Testing basic Java functionality..."
ssh "$USER@$HOST" 'rnx run --runtime=java:21 java --version' || warn "Basic test failed"

# Show success message
echo ""
log "🎉 Deployment completed successfully!"
info "Runtime deployed to: $USER@$HOST:/opt/joblet/runtimes/java/java-21/"
info "Package created at: $USER@$HOST:/tmp/java-21-runtime.tar.gz"
echo ""
info "📝 Next steps:"
info "  1. Test: ssh $USER@$HOST 'rnx run --runtime=java:21 java -c \"System.out.println(\\\"Hello!\\\");\"'"
info "  2. Try Maven example from the README"
echo ""