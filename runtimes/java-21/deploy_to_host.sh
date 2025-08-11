#!/bin/bash
# Deploy Multi-Architecture Java 21 Runtime to Joblet Host with Auto-Detection

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

log "☕ Deploying Multi-Architecture Java 21 Runtime to $USER@$HOST"

# Check if multi-arch setup script exists
SETUP_SCRIPT="setup_java_21.sh"
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
        info "✅ Architecture $TARGET_ARCH fully supported for Java 21"
        ;;
    aarch64|arm64)
        info "✅ Architecture $TARGET_ARCH supported for Java 21"
        ;;
    armv7l|armhf)
        warn "⚠️  Architecture $TARGET_ARCH has limited Java support"
        warn "Java 21 binary packages may not be available for ARM 32-bit"
        ;;
    *)
        error "❌ Unsupported architecture: $TARGET_ARCH"
        error "Java 21 runtime supports: x86_64/amd64, aarch64/arm64"
        exit 1
        ;;
esac

if [[ "$TARGET_OS" != "Linux" ]]; then
    error "❌ Unsupported OS: $TARGET_OS"
    error "Java 21 runtime only supports Linux"
    exit 1
fi

# Check for Amazon Linux specific information
case "$TARGET_DISTRO" in
    amzn)
        info "🌐 Amazon Linux detected - using YUM package manager"
        ;;
    ubuntu|debian)
        info "🌐 Debian-based system detected - using APT package manager"
        ;;
    centos|rhel|rocky|almalinux)
        info "🌐 RHEL-based system detected - using YUM package manager"
        ;;
    fedora)
        info "🌐 Fedora detected - using DNF package manager"
        ;;
esac

# Copy multi-architecture setup script and detection library to host
log "📤 Copying multi-architecture setup components to host..."
scp "$SCRIPT_DIR/$SETUP_SCRIPT" "$USER@$HOST:/tmp/"
ssh "$USER@$HOST" 'mkdir -p /tmp/common'
scp "$SCRIPT_DIR/$DETECT_SCRIPT" "$USER@$HOST:/tmp/common/"

# Make setup script and detection library executable on host
log "🔧 Making setup components executable..."
ssh "$USER@$HOST" 'chmod +x /tmp/setup_java_21.sh'
ssh "$USER@$HOST" 'chmod +x /tmp/common/detect_system.sh'

# Remove existing runtime if it exists
log "🗑️  Removing existing Java 21 runtime if present..."
ssh "$USER@$HOST" 'sudo rm -rf /opt/joblet/runtimes/java/java-21 /tmp/java-21-runtime.tar.gz' || warn "No existing runtime to remove"

# Run multi-architecture setup script on host
log "🏗️  Running multi-architecture setup on host (this will auto-detect and optimize for $TARGET_ARCH)..."
ssh "$USER@$HOST" 'sudo /tmp/setup_java_21.sh'

# Verify installation
log "🧪 Verifying installation..."
ssh "$USER@$HOST" 'rnx runtime list' || warn "Runtime list failed - joblet might not be running"

# Test basic functionality
log "✅ Testing basic Java functionality..."
ssh "$USER@$HOST" 'rnx run --runtime=java:21 java --version' || warn "Basic test failed"

# Show detailed success message with architecture info
echo ""
log "🎉 Multi-Architecture Deployment Completed Successfully!"
info "✅ Target System: $TARGET_ARCH ($TARGET_DISTRO)"
info "✅ Runtime deployed to: $USER@$HOST:/opt/joblet/runtimes/java/java-21/"
info "✅ Optimized for: $TARGET_ARCH architecture"
echo ""
info "📝 Architecture-Specific Information:"
case "$TARGET_ARCH" in
    x86_64|amd64)
        info "  • Full Java 21 feature support with maximum optimization"
        info "  • Virtual Threads, Pattern Matching, String Templates available"
        info "  • JShell interactive REPL with Java 21 features"
        ;;
    aarch64|arm64)
        info "  • Full Java 21 support with ARM64 optimizations"
        info "  • Virtual Threads and modern features fully supported"
        info "  • Native ARM64 binaries for best performance"
        ;;
    armv7l|armhf)
        info "  • Basic Java 21 support (binary availability may vary)"
        info "  • Modern features available if runtime installation succeeds"
        ;;
esac
echo ""
info "📚 Next Steps:"
info "  1. Test: ssh $USER@$HOST 'rnx run --runtime=java:21 java --version'"
info "  2. Try template: ssh $USER@$HOST 'cd /opt/joblet/examples/java-21 && rnx run --template=jobs.yaml:hello-joblet'"
info "  3. Try Virtual Threads: ssh $USER@$HOST 'cd /opt/joblet/examples/java-21 && rnx run --template=jobs.yaml:virtual-threads'"
info "  4. View runtime info: ssh $USER@$HOST 'rnx runtime info java:21'"
echo ""