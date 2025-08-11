#!/bin/bash

# Multi-Architecture Python 3.11 Runtime Setup Script
# Supports: amd64, arm64, armhf on Ubuntu, Debian, CentOS, RHEL, Fedora, openSUSE, Arch, Alpine
# ⚠️  WARNING: This script installs build dependencies on the host system
# ⚠️  See /opt/joblet/runtimes/CONTAMINATION_WARNING.md for details

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RUNTIME_BASE_DIR="/opt/joblet/runtimes"
RUNTIME_DIR="$RUNTIME_BASE_DIR/python/python-3.11"
PYTHON_VERSION="3.11.9"

# Load system detection library
if [[ -f "$SCRIPT_DIR/../common/detect_system.sh" ]]; then
    source "$SCRIPT_DIR/../common/detect_system.sh"
elif [[ -f "/tmp/common/detect_system.sh" ]]; then
    source "/tmp/common/detect_system.sh"
else
    echo "❌ System detection library not found"
    exit 1
fi

echo "🐍 Setting up Multi-Architecture Python 3.11 Runtime"
echo "====================================================="
echo "Target: $RUNTIME_DIR"
echo "⚠️  WARNING: This script will install build dependencies on the host system"
echo "⚠️  Impact: ~100-200MB of build tools and packages"
echo


# Check if running as root
check_root() {
    if [[ $EUID -ne 0 ]]; then
        echo "❌ This script must be run as root"
        echo "Usage: sudo $0"
        exit 1
    fi
    echo "✅ Running as root"
}

# Main setup function
main() {
    check_root
    
    # Detect system configuration
    if ! detect_system; then
        show_supported_platforms
        exit 1
    fi
    
    # Create runtime directory structure
    echo "📁 Creating isolated runtime directory..."
    mkdir -p "$RUNTIME_DIR"
    cd "$RUNTIME_DIR"
    
    # Check if already installed
    if [[ -f "$RUNTIME_DIR/runtime.yml" && -d "$RUNTIME_DIR/python-install" ]]; then
        echo "✅ Python 3.11 runtime already installed"
        echo "   Location: $RUNTIME_DIR"
        echo "   To reinstall, remove the directory first:"
        echo "   sudo rm -rf '$RUNTIME_DIR'"
        exit 0
    fi
    
    # Get Python build dependencies for the detected distribution
    echo "📦 Getting Python build dependencies for $DETECTED_DISTRO..."
    PYTHON_DEPS=$(get_python_packages)
    if [[ $? -ne 0 ]]; then
        echo "$PYTHON_DEPS"  # Error message
        show_supported_platforms
        exit 1
    fi
    
    echo "📦 Installing Python build dependencies: $PYTHON_DEPS"
    install_packages "$PYTHON_DEPS"
    
    # Download Python source
    echo "⬇️  Downloading Python $PYTHON_VERSION source for $DETECTED_ARCH..."
    PYTHON_URL="https://www.python.org/ftp/python/$PYTHON_VERSION/Python-$PYTHON_VERSION.tgz"
    
    download_file "$PYTHON_URL" "Python-$PYTHON_VERSION.tgz"
    
    echo "📦 Extracting Python source..."
    tar -xzf "Python-$PYTHON_VERSION.tgz"
    cd "Python-$PYTHON_VERSION"
    
    # Configure Python for the specific architecture
    echo "⚙️  Configuring Python for $DETECTED_ARCH on $DETECTED_DISTRO..."
    CONFIGURE_FLAGS="--prefix=$RUNTIME_DIR/python-install --enable-optimizations --with-ensurepip=install --enable-shared"
    
    # Architecture-specific optimizations
    case "$DETECTED_ARCH" in
        amd64)
            CONFIGURE_FLAGS="$CONFIGURE_FLAGS --enable-optimizations"
            ;;
        arm64)
            CONFIGURE_FLAGS="$CONFIGURE_FLAGS --build=aarch64-linux-gnu"
            ;;
        armhf)
            CONFIGURE_FLAGS="$CONFIGURE_FLAGS --build=arm-linux-gnueabihf --disable-optimizations"
            ;;
    esac
    
    # Set RPATH for shared library loading
    CONFIGURE_FLAGS="$CONFIGURE_FLAGS LDFLAGS=\"-Wl,-rpath=$RUNTIME_DIR/python-install/lib\""
    
    echo "🔧 Configuration: $CONFIGURE_FLAGS"
    eval "./configure $CONFIGURE_FLAGS"
    
    # Compile Python (architecture-optimized)
    echo "🔨 Compiling Python for $DETECTED_ARCH (this may take several minutes)..."
    MAKE_JOBS=$(nproc)
    case "$DETECTED_ARCH" in
        armhf)
            # Use fewer jobs on ARM 32-bit to avoid memory issues
            MAKE_JOBS=$((MAKE_JOBS / 2))
            [[ $MAKE_JOBS -lt 1 ]] && MAKE_JOBS=1
            ;;
    esac
    
    make -j$MAKE_JOBS > /dev/null 2>&1
    
    echo "📦 Installing Python to isolated runtime directory..."
    make install > /dev/null 2>&1
    
    # Clean up source files
    cd "$RUNTIME_DIR"
    rm -rf "Python-$PYTHON_VERSION" "Python-$PYTHON_VERSION.tgz"
    
    # Verify Python installation
    echo "🔍 Verifying isolated Python installation..."
    PYTHON_BIN="$RUNTIME_DIR/python-install/bin/python3"
    
    if [[ ! -f "$PYTHON_BIN" ]]; then
        echo "❌ Python installation failed!"
        exit 1
    fi
    
    INSTALLED_VERSION=$($PYTHON_BIN --version)
    echo "✅ Isolated Python installed: $INSTALLED_VERSION"
    
    # Create basic virtual environment
    echo "🏗️  Creating base virtual environment..."
    $PYTHON_BIN -m venv base-venv
    
    # Upgrade pip in the virtual environment
    source base-venv/bin/activate
    pip install --upgrade pip > /dev/null 2>&1
    
    # Install common packages
    echo "📚 Installing common Python packages..."
    pip install requests urllib3 certifi charset-normalizer idna > /dev/null 2>&1
    
    deactivate
    
    # Set correct ownership
    echo "👤 Setting ownership to joblet user..."
    chown -R joblet:joblet "$RUNTIME_DIR" 2>/dev/null || echo "⚠️  joblet user not found, keeping root ownership"
    
    # Create runtime manifest with system information
    cat > "$RUNTIME_DIR/runtime.yml" << EOF
name: "python:3.11"
version: "3.11.9"
description: "Python 3.11 with virtual environment support"
type: "python"
created: "$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
system:
  architecture: "$DETECTED_ARCH"
  os: "$DETECTED_OS"
  distribution: "$DETECTED_DISTRO"
  package_manager: "$DETECTED_PACKAGE_MANAGER"
paths:
  python_home: "$RUNTIME_DIR/python-install"
  venv_home: "$RUNTIME_DIR/base-venv"
binaries:
  python: "$RUNTIME_DIR/base-venv/bin/python"
  python3: "$RUNTIME_DIR/base-venv/bin/python3"
  pip: "$RUNTIME_DIR/base-venv/bin/pip"
  pip3: "$RUNTIME_DIR/base-venv/bin/pip3"
environment:
  PYTHON_HOME: "$RUNTIME_DIR/python-install"
  PYTHONPATH: "$RUNTIME_DIR/base-venv/lib/python3.11/site-packages"
  PATH: "$RUNTIME_DIR/base-venv/bin:\$PATH"
  LD_LIBRARY_PATH: "$RUNTIME_DIR/python-install/lib"
features:
  - "Pattern Matching (Python 3.10+)"
  - "Structural Pattern Matching"
  - "Exception Groups (Python 3.11+)"
  - "Task Groups (asyncio)"
  - "TOML support (tomllib)"
  - "Virtual Environment Support"
packages:
  - "requests"
  - "urllib3"
  - "certifi"
EOF
    
    # Display success information
    echo ""
    echo "🎉 Python 3.11 Runtime Installation Complete!"
    echo "=============================================="
    echo "✅ Architecture: $DETECTED_ARCH"
    echo "✅ Distribution: $DETECTED_DISTRO"
    echo "✅ Python Version: $(echo "$INSTALLED_VERSION" | cut -d' ' -f2)"
    echo "✅ Installation Path: $RUNTIME_DIR"
    echo "✅ Runtime Manifest: $RUNTIME_DIR/runtime.yml"
    echo ""
    echo "🧪 Test the runtime:"
    echo "   rnx runtime list"
    echo "   rnx runtime info python:3.11"
    echo "   rnx run --runtime=python:3.11 python --version"
    echo ""
    echo "📚 Example usage:"
    echo "   cd /opt/joblet/examples/python-analytics"
    echo "   rnx run --template=jobs.yaml:sales-analysis"
    echo ""
}

# Show usage if requested
if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
    echo "Multi-Architecture Python 3.11 Runtime Setup"
    echo ""
    echo "Usage: sudo $0"
    echo ""
    show_supported_platforms
    exit 0
fi

# Run main function
main "$@"