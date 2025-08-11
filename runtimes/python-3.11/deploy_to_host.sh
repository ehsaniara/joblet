#!/bin/bash
# Deploy Multi-Architecture Python 3.11 Runtime to Joblet Host with Auto-Detection

set -e

HOST=${1:-"192.168.1.161"}
USER=${2:-"jay"}
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

log() { echo -e "${GREEN}[$(date +'%H:%M:%S')] $1${NC}"; }
info() { echo -e "${BLUE}[INFO] $1${NC}"; }
warn() { echo -e "${YELLOW}[WARN] $1${NC}"; }
error() { echo -e "${RED}[ERROR] $1${NC}"; }

log "🐍 Deploying Multi-Architecture Python 3.11 Runtime to $USER@$HOST"

# Check if multi-arch setup script exists
SETUP_SCRIPT="setup_python_3_11.sh"
if [[ ! -f "$SCRIPT_DIR/$SETUP_SCRIPT" ]]; then
    error "Multi-architecture setup script not found: $SCRIPT_DIR/$SETUP_SCRIPT"
    info "Please ensure the multi-architecture setup script is available"
    exit 1
fi

# Check if detection library exists
DETECT_SCRIPT="../common/detect_system.sh"
if [[ ! -f "$SCRIPT_DIR/$DETECT_SCRIPT" ]]; then
    error "System detection library not found: $SCRIPT_DIR/$DETECT_SCRIPT"
    info "Please ensure the common detection library is available"
    exit 1
fi

# Auto-detect target system before deployment
log "🔍 Auto-detecting target system architecture and distribution..."
REMOTE_DETECTION=$(ssh "$USER@$HOST" 'uname -m && uname -s && (cat /etc/os-release | grep "^ID=" | cut -d= -f2 | tr -d "\"" || echo "unknown")')
TARGET_ARCH=$(echo "$REMOTE_DETECTION" | sed -n '1p')
TARGET_OS=$(echo "$REMOTE_DETECTION" | sed -n '2p')  
TARGET_DISTRO=$(echo "$REMOTE_DETECTION" | sed -n '3p')

info "Target System Detection:"
info "  Architecture: $TARGET_ARCH"
info "  OS: $TARGET_OS"
info "  Distribution: $TARGET_DISTRO"

# Validate system compatibility
case "$TARGET_ARCH" in
    x86_64|amd64)
        info "✅ Architecture $TARGET_ARCH fully supported for Python 3.11"
        ;;
    aarch64|arm64)
        info "✅ Architecture $TARGET_ARCH supported for Python 3.11"
        ;;
    armv7l|armhf)
        info "✅ Architecture $TARGET_ARCH supported for Python 3.11 (basic features)"
        warn "Some optimizations may be disabled for ARM 32-bit compatibility"
        ;;
    *)
        error "❌ Unsupported architecture: $TARGET_ARCH"
        error "Python 3.11 runtime supports: x86_64/amd64, aarch64/arm64, armv7l/armhf"
        exit 1
        ;;
esac

if [[ "$TARGET_OS" != "Linux" ]]; then
    error "❌ Unsupported OS: $TARGET_OS"
    error "Python 3.11 runtime only supports Linux"
    exit 1
fi

# Check for distribution-specific information
case "$TARGET_DISTRO" in
    amzn)
        info "🌐 Amazon Linux detected - using YUM package manager"
        info "   Python build dependencies will be installed with yum"
        ;;
    ubuntu|debian)
        info "🌐 Debian-based system detected - using APT package manager"
        info "   Python build dependencies available via apt-get"
        ;;
    centos|rhel|rocky|almalinux)
        info "🌐 RHEL-based system detected - using YUM package manager"
        ;;
    fedora)
        info "🌐 Fedora detected - using DNF package manager"
        ;;
    opensuse|sles)
        info "🌐 openSUSE/SLES detected - using Zypper package manager"
        ;;
    arch|manjaro)
        info "🌐 Arch-based system detected - using Pacman package manager"
        ;;
    alpine)
        info "🌐 Alpine Linux detected - using APK package manager"
        ;;
esac

# Copy multi-architecture setup script and detection library to host
log "📤 Copying multi-architecture setup components to host..."
scp "$SCRIPT_DIR/$SETUP_SCRIPT" "$USER@$HOST:/tmp/"
ssh "$USER@$HOST" 'mkdir -p /tmp/common'
scp "$SCRIPT_DIR/$DETECT_SCRIPT" "$USER@$HOST:/tmp/common/"

# Make setup script and detection library executable on host
log "🔧 Making setup components executable..."
ssh "$USER@$HOST" 'chmod +x /tmp/setup_python_3_11.sh'
ssh "$USER@$HOST" 'chmod +x /tmp/common/detect_system.sh'

# Remove existing runtime if it exists
log "🗑️  Removing existing Python 3.11 runtime if present..."
ssh "$USER@$HOST" 'sudo rm -rf /opt/joblet/runtimes/python/python-3.11 /tmp/python-3.11-runtime.tar.gz' || warn "No existing runtime to remove"

# Run multi-architecture setup script on host
log "🏗️  Running multi-architecture setup on host (this will auto-detect and optimize for $TARGET_ARCH)..."
log "   Note: Python compilation may take 10-15 minutes on slower systems"
ssh "$USER@$HOST" 'sudo /tmp/setup_python_3_11.sh'

# Verify installation
log "🧪 Verifying installation..."
ssh "$USER@$HOST" 'rnx runtime list' || warn "Runtime list failed - joblet might not be running"

# Test basic functionality
log "✅ Testing basic Python functionality..."
ssh "$USER@$HOST" 'rnx run --runtime=python:3.11 python --version' || warn "Basic test failed"

# Show detailed success message with architecture info
echo ""
log "🎉 Multi-Architecture Deployment Completed Successfully!"
info "✅ Target System: $TARGET_ARCH ($TARGET_DISTRO)"
info "✅ Runtime deployed to: $USER@$HOST:/opt/joblet/runtimes/python/python-3.11/"
info "✅ Optimized for: $TARGET_ARCH architecture"
echo ""
info "📝 Architecture-Specific Information:"
case "$TARGET_ARCH" in
    x86_64|amd64)
        info "  • Full Python 3.11 features with maximum optimization"
        info "  • All compilation optimizations enabled"
        info "  • Complete package ecosystem support"
        ;;
    aarch64|arm64)
        info "  • Full Python 3.11 support with ARM64 optimizations"
        info "  • Native ARM64 compilation for best performance"
        info "  • Full package compatibility"
        ;;
    armv7l|armhf)
        info "  • Python 3.11 support with ARM 32-bit compatibility"
        info "  • Some optimizations disabled for stability"
        info "  • Basic package support (some may require source compilation)"
        ;;
esac
echo ""
info "📚 Next Steps:"
info "  1. Test: ssh $USER@$HOST 'rnx run --runtime=python:3.11 python --version'"
info "  2. Try template: ssh $USER@$HOST 'cd /opt/joblet/examples/python-analytics && rnx run --template=jobs.yaml:sales-analysis'"
info "  3. Install packages: ssh $USER@$HOST 'rnx run --runtime=python:3.11 pip install requests'"
info "  4. View runtime info: ssh $USER@$HOST 'rnx runtime info python:3.11'"
echo ""