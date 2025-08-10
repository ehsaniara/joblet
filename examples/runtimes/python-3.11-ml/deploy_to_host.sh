#!/bin/bash
# Deploy Python 3.11 ML Runtime to Joblet Host

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

log "🤖 Deploying Python 3.11 ML Runtime to $USER@$HOST"
warn "This is a LARGE runtime (~1.5GB) with pre-installed ML libraries"

# Check if setup script exists
if [[ ! -f "$SCRIPT_DIR/setup_python_3_11_ml.sh" ]]; then
    echo "❌ Setup script not found: $SCRIPT_DIR/setup_python_3_11_ml.sh"
    exit 1
fi

# Copy setup script to host
log "📤 Copying setup script to host..."
scp "$SCRIPT_DIR/setup_python_3_11_ml.sh" "$USER@$HOST:/tmp/"

# Make setup script executable on host
log "🔧 Making setup script executable..."
ssh "$USER@$HOST" 'chmod +x /tmp/setup_python_3_11_ml.sh'

# Remove existing runtime if it exists
log "🗑️  Removing existing Python 3.11 ML runtime if present..."
ssh "$USER@$HOST" 'sudo rm -rf /opt/joblet/runtimes/python/python-3.11-ml /tmp/python-3.11-ml-runtime.tar.gz' || warn "No existing runtime to remove"

# Run setup script on host
log "🏗️  Running fresh setup on host (this will take 20-30 minutes due to ML libraries)..."
ssh "$USER@$HOST" 'sudo /tmp/setup_python_3_11_ml.sh'

# Verify installation
log "🧪 Verifying installation..."
ssh "$USER@$HOST" 'rnx runtime list' || warn "Runtime list failed - joblet might not be running"

# Test basic functionality
log "✅ Testing basic Python ML functionality..."
ssh "$USER@$HOST" 'rnx run --runtime=python:3.11-ml python -c "import numpy, pandas, sklearn; print(\"ML libraries loaded!\")"' || warn "Basic test failed"

# Show success message
echo ""
log "🎉 Deployment completed successfully!"
info "Runtime deployed to: $USER@$HOST:/opt/joblet/runtimes/python/python-3.11-ml/"
info "Package created at: $USER@$HOST:/tmp/python-3.11-ml-runtime.tar.gz"
echo ""
info "📝 Next steps:"
info "  1. Test: ssh $USER@$HOST 'rnx run --runtime=python:3.11-ml python -c \"import numpy; print(numpy.__version__)\"'"
info "  2. Try ML example from the README"
echo ""
warn "Note: This runtime is ~1.5GB with pre-installed ML libraries"
warn "Consider using python:3.11 with packaged dependencies for smaller deployments"