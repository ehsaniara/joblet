#!/bin/bash
# Deploy Multi-Architecture Python 3.11 ML Runtime to Joblet Host with Auto-Detection

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

log "🤖 Deploying Multi-Architecture Python 3.11 ML Runtime to $USER@$HOST"
warn "This is a LARGE runtime (~1-2GB) with ML libraries optimized per architecture"

# Check if multi-arch setup script exists
SETUP_SCRIPT="setup_python_3_11_ml.sh"
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

# Validate system compatibility with ML-specific considerations
case "$TARGET_ARCH" in
    x86_64|amd64)
        info "✅ Architecture $TARGET_ARCH fully supported for Python 3.11 ML"
        info "   All ML packages (NumPy, Pandas, Scikit-learn, etc.) available"
        ML_SUPPORT="full"
        ;;
    aarch64|arm64)
        info "✅ Architecture $TARGET_ARCH supported for Python 3.11 ML"
        info "   Most ML packages available (some may compile from source)"
        ML_SUPPORT="most"
        ;;
    armv7l|armhf)
        warn "⚠️  Architecture $TARGET_ARCH has limited ML package support"
        warn "   Only basic packages (NumPy) may be available for ARM 32-bit"
        warn "   Consider using x86_64 or arm64 for full ML capabilities"
        ML_SUPPORT="limited"
        ;;
    *)
        error "❌ Unsupported architecture: $TARGET_ARCH"
        error "Python 3.11 ML runtime supports: x86_64/amd64, aarch64/arm64, armv7l/armhf"
        exit 1
        ;;
esac

if [[ "$TARGET_OS" != "Linux" ]]; then
    error "❌ Unsupported OS: $TARGET_OS"
    error "Python 3.11 ML runtime only supports Linux"
    exit 1
fi

# Check for distribution-specific ML build requirements
case "$TARGET_DISTRO" in
    amzn)
        info "🌐 Amazon Linux detected - using YUM package manager"
        info "   ML build dependencies will be installed with yum"
        warn "   ML package compilation may take longer on Amazon Linux"
        ;;
    ubuntu|debian)
        info "🌐 Debian-based system detected - using APT package manager"
        info "   ML build dependencies readily available via apt-get"
        ;;
    centos|rhel|rocky|almalinux)
        info "🌐 RHEL-based system detected - using YUM package manager"
        warn "   Some ML packages may require EPEL repository"
        ;;
    fedora)
        info "🌐 Fedora detected - using DNF package manager"
        info "   Good ML package support with DNF"
        ;;
    opensuse|sles)
        info "🌐 openSUSE/SLES detected - using Zypper package manager"
        ;;
    arch|manjaro)
        info "🌐 Arch-based system detected - using Pacman package manager"
        info "   Excellent ML development environment support"
        ;;
    alpine)
        info "🌐 Alpine Linux detected - using APK package manager"
        warn "   ML packages may require additional compilation time"
        ;;
esac

# Copy multi-architecture setup script and detection library to host
log "📤 Copying multi-architecture setup components to host..."
scp "$SCRIPT_DIR/$SETUP_SCRIPT" "$USER@$HOST:/tmp/"
ssh "$USER@$HOST" 'mkdir -p /tmp/common'
scp "$SCRIPT_DIR/$DETECT_SCRIPT" "$USER@$HOST:/tmp/common/"

# Make setup script and detection library executable on host
log "🔧 Making setup components executable..."
ssh "$USER@$HOST" 'chmod +x /tmp/setup_python_3_11_ml.sh'
ssh "$USER@$HOST" 'chmod +x /tmp/common/detect_system.sh'

# Remove existing runtime if it exists
log "🗑️  Removing existing Python 3.11 ML runtime if present..."
ssh "$USER@$HOST" 'sudo rm -rf /opt/joblet/runtimes/python/python-3.11-ml /tmp/python-3.11-ml-runtime.tar.gz' || warn "No existing runtime to remove"

# Run multi-architecture setup script on host
log "🏗️  Running multi-architecture ML setup on host (this will auto-detect and optimize for $TARGET_ARCH)..."
case "$ML_SUPPORT" in
    full)
        log "   Note: Full ML stack installation may take 20-30 minutes"
        ;;
    most)
        log "   Note: ARM64 ML installation may take 25-35 minutes (some source compilation)"
        ;;
    limited)
        log "   Note: Limited ML installation for ARM 32-bit (10-15 minutes)"
        ;;
esac
ssh "$USER@$HOST" 'sudo /tmp/setup_python_3_11_ml.sh'

# Verify installation
log "🧪 Verifying installation..."
ssh "$USER@$HOST" 'rnx runtime list' || warn "Runtime list failed - joblet might not be running"

# Test basic functionality with architecture-specific packages
log "✅ Testing Python ML functionality for $TARGET_ARCH..."
case "$ML_SUPPORT" in
    full|most)
        ssh "$USER@$HOST" 'rnx run --runtime=python-3.11-ml python -c "import numpy, pandas; print(f\"NumPy: {numpy.__version__}, Pandas: {pandas.__version__}\")"' || warn "ML test failed"
        ;;
    limited)
        ssh "$USER@$HOST" 'rnx run --runtime=python-3.11-ml python -c "import numpy; print(f\"NumPy: {numpy.__version__} (basic ML support)\")"' || warn "Basic ML test failed"
        ;;
esac

# Show detailed success message with architecture-specific ML info
echo ""
log "🎉 Multi-Architecture ML Deployment Completed Successfully!"
info "✅ Target System: $TARGET_ARCH ($TARGET_DISTRO)"
info "✅ Runtime deployed to: $USER@$HOST:/opt/joblet/runtimes/python/python-3.11-ml/"
info "✅ ML Support Level: $ML_SUPPORT"
info "✅ Optimized for: $TARGET_ARCH architecture"
echo ""
info "📝 Architecture-Specific ML Information:"
case "$TARGET_ARCH" in
    x86_64|amd64)
        info "  • Full ML stack: NumPy, Pandas, Scikit-learn, Matplotlib, SciPy"
        info "  • All binary wheels available for instant loading"
        info "  • Maximum performance optimizations enabled"
        info "  • Runtime size: ~1.5-2GB with all packages"
        ;;
    aarch64|arm64)
        info "  • Most ML packages: NumPy, Pandas, Scikit-learn, Matplotlib"
        info "  • ARM64 native binaries where available"
        info "  • Some packages compiled from source for optimization"
        info "  • Runtime size: ~1.2-1.8GB"
        ;;
    armv7l|armhf)
        info "  • Limited ML support: NumPy and basic packages only"
        info "  • Compiled for ARM 32-bit compatibility"
        info "  • Lighter runtime for resource-constrained systems"
        info "  • Runtime size: ~500-800MB"
        ;;
esac
echo ""
info "📚 Next Steps:"
info "  1. Test: ssh $USER@$HOST 'rnx run --runtime=python-3.11-ml python --version'"
info "  2. Try template: ssh $USER@$HOST 'cd /opt/joblet/examples/python-3.11-ml && rnx run --template=jobs.yaml:ml-analysis'"
if [[ "$ML_SUPPORT" == "full" || "$ML_SUPPORT" == "most" ]]; then
    info "  3. Test ML: ssh $USER@$HOST 'rnx run --runtime=python-3.11-ml python -c \"from sklearn.ensemble import RandomForestClassifier; print(\\\"ML ready!\\\")\"'"
fi
info "  4. View runtime info: ssh $USER@$HOST 'rnx runtime info python-3.11-ml'"
echo ""
if [[ "$ML_SUPPORT" == "limited" ]]; then
    warn "⚠️  ARM 32-bit Limitations:"
    warn "   - Only NumPy and basic packages available"
    warn "   - For full ML stack, consider upgrading to ARM 64-bit or x86_64"
fi