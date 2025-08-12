#!/bin/bash

# Multi-Architecture Python 3.11 + ML Runtime Setup Script
# Supports: amd64, arm64, armhf on Ubuntu, Debian, CentOS, RHEL, Fedora, openSUSE, Arch, Alpine
# ⚠️  WARNING: This script installs build dependencies on the host system
# ⚠️  See /opt/joblet/runtimes/CONTAMINATION_WARNING.md for details

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RUNTIME_BASE_DIR="/opt/joblet/runtimes"
RUNTIME_DIR="$RUNTIME_BASE_DIR/python/python-3.11-ml"
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

echo "🐍 Setting up Multi-Architecture Python 3.11 + ML Runtime"
echo "=========================================================="
echo "Target: $RUNTIME_DIR"
echo "⚠️  WARNING: This script will install build dependencies on the host system"
echo "⚠️  Impact: ~200-300MB of build tools and ML packages"
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
    
    # Architecture-specific ML package support check
    case "$DETECTED_ARCH" in
        amd64)
            echo "✅ Full ML package support available for $DETECTED_ARCH"
            ML_SUPPORT="full"
            ;;
        arm64)
            echo "✅ Most ML packages supported for $DETECTED_ARCH"
            ML_SUPPORT="most"
            ;;
        armhf)
            echo "⚠️  Limited ML package support for $DETECTED_ARCH"
            echo "   Some binary wheels may not be available"
            ML_SUPPORT="limited"
            ;;
    esac
    
    # Create runtime directory structure
    echo "📁 Creating isolated runtime directory..."
    mkdir -p "$RUNTIME_DIR"
    cd "$RUNTIME_DIR"
    
    # Check if already installed
    if [[ -f "$RUNTIME_DIR/runtime.yml" && -d "$RUNTIME_DIR/ml-venv" ]]; then
        echo "✅ Python 3.11 + ML runtime already installed"
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
    
    # Configure Python for the specific architecture with ML optimizations
    echo "⚙️  Configuring Python for ML workloads on $DETECTED_ARCH..."
    CONFIGURE_FLAGS="--prefix=$RUNTIME_DIR/python-install --with-ensurepip=install --enable-shared"
    
    # Architecture-specific optimizations for ML
    case "$DETECTED_ARCH" in
        amd64)
            CONFIGURE_FLAGS="$CONFIGURE_FLAGS --enable-optimizations --with-lto"
            ;;
        arm64)
            CONFIGURE_FLAGS="$CONFIGURE_FLAGS --enable-optimizations --build=aarch64-linux-gnu"
            ;;
        armhf)
            CONFIGURE_FLAGS="$CONFIGURE_FLAGS --build=arm-linux-gnueabihf --disable-optimizations"
            echo "⚠️  Optimizations disabled for ARM 32-bit compatibility"
            ;;
    esac
    
    # Set RPATH for shared library loading
    CONFIGURE_FLAGS="$CONFIGURE_FLAGS LDFLAGS=\"-Wl,-rpath=$RUNTIME_DIR/python-install/lib\""
    
    echo "🔧 Configuration: $CONFIGURE_FLAGS"
    eval "./configure $CONFIGURE_FLAGS"
    
    # Compile Python (architecture-optimized)
    echo "🔨 Compiling Python for ML workloads on $DETECTED_ARCH (this may take several minutes)..."
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
    
    # Create ML-optimized virtual environment
    echo "🏗️  Creating ML-optimized virtual environment..."
    $PYTHON_BIN -m venv ml-venv
    
    # Install ML packages with architecture-specific handling
    echo "📚 Installing ML packages for $DETECTED_ARCH (ML support: $ML_SUPPORT)..."
    source ml-venv/bin/activate
    
    # Upgrade pip first
    pip install --upgrade pip > /dev/null 2>&1
    
    # Install packages in dependency order with architecture considerations
    case "$ML_SUPPORT" in
        full)
            install_full_ml_packages
            ;;
        most)
            install_arm64_ml_packages
            ;;
        limited)
            install_limited_ml_packages
            ;;
    esac
    
    deactivate
    
    # Set correct ownership
    echo "👤 Setting ownership to joblet user..."
    chown -R joblet:joblet "$RUNTIME_DIR" 2>/dev/null || echo "⚠️  joblet user not found, keeping root ownership"
    
    # Fix Python symlinks for container isolation
    fix_python_symlinks
    
    # Create runtime manifest with system and ML information
    create_runtime_manifest
    
    # Test ML installation
    test_ml_installation
    
    # Display success information
    show_installation_summary
}

# Install full ML packages (amd64)
install_full_ml_packages() {
    echo "Installing full ML stack for amd64..."
    
    # Core numerical packages (pinned for stability)
    echo "Installing NumPy (foundation - pinned to 1.x)..."
    pip install "numpy>=1.24.3,<2.0" > /dev/null 2>&1
    
    echo "Installing SciPy..."
    pip install "scipy>=1.11.0,<1.12" > /dev/null 2>&1
    
    echo "Installing Pandas..."
    pip install "pandas>=2.0.3,<2.1" > /dev/null 2>&1
    
    echo "Installing Scikit-learn..."
    pip install "scikit-learn>=1.3.0,<1.4" > /dev/null 2>&1
    
    echo "Installing Matplotlib..."
    pip install "matplotlib>=3.7.0,<3.8" > /dev/null 2>&1
    
    echo "Installing Seaborn..."
    pip install "seaborn>=0.12.0,<0.13" > /dev/null 2>&1
    
    echo "Installing additional packages..."
    pip install requests==2.31.0 openpyxl==3.1.2 > /dev/null 2>&1
}

# Install ARM64 ML packages (most packages work)
install_arm64_ml_packages() {
    echo "Installing ML stack for arm64 (most packages supported)..."
    
    # Core packages with ARM64 support
    echo "Installing NumPy for ARM64..."
    pip install "numpy>=1.24.3,<2.0" > /dev/null 2>&1
    
    echo "Installing SciPy for ARM64..."
    pip install "scipy>=1.11.0,<1.12" > /dev/null 2>&1
    
    echo "Installing Pandas for ARM64..."
    pip install "pandas>=2.0.3,<2.1" > /dev/null 2>&1
    
    echo "Installing Scikit-learn for ARM64..."
    pip install "scikit-learn>=1.3.0,<1.4" > /dev/null 2>&1 || {
        echo "⚠️  Scikit-learn binary not available, compiling from source..."
        pip install scikit-learn --no-binary=scikit-learn > /dev/null 2>&1
    }
    
    echo "Installing Matplotlib for ARM64..."
    pip install "matplotlib>=3.7.0,<3.8" > /dev/null 2>&1
    
    echo "Installing Seaborn for ARM64..."
    pip install "seaborn>=0.12.0,<0.13" > /dev/null 2>&1
    
    echo "Installing additional packages..."
    pip install requests==2.31.0 openpyxl==3.1.2 > /dev/null 2>&1
}

# Install limited ML packages (armhf)
install_limited_ml_packages() {
    echo "Installing basic ML packages for armhf (limited support)..."
    
    # Try core packages, fall back to source compilation if needed
    echo "Installing NumPy for ARM 32-bit..."
    pip install "numpy>=1.24.3,<2.0" > /dev/null 2>&1 || {
        echo "⚠️  NumPy binary not available, compiling from source (this will take time)..."
        pip install numpy --no-binary=numpy > /dev/null 2>&1
    }
    
    echo "Installing basic packages..."
    pip install requests==2.31.0 > /dev/null 2>&1
    
    # Skip heavy ML packages that may not compile well on ARM 32-bit
    echo "⚠️  Skipping SciPy, Pandas, Scikit-learn on ARM 32-bit due to compilation complexity"
    echo "   Basic NumPy and standard library available for lightweight ML tasks"
}

# Fix Python symlinks for isolated containers
fix_python_symlinks() {
    echo "🔧 Fixing Python symlinks for container isolation..."
    
    # Remove broken symlinks that point to absolute paths
    rm -f "$RUNTIME_DIR/ml-venv/bin/python" "$RUNTIME_DIR/ml-venv/bin/python3"
    
    # Copy the actual Python binary to make it self-contained
    cp "$RUNTIME_DIR/python-install/bin/python3.11" "$RUNTIME_DIR/ml-venv/bin/python"
    
    # Create relative symlinks
    cd "$RUNTIME_DIR/ml-venv/bin"
    ln -s python python3
    cd - > /dev/null
    
    echo "   ✅ Python binary copied and symlinks fixed"
}

# Create runtime manifest
create_runtime_manifest() {
    cat > "$RUNTIME_DIR/runtime.yml" << EOF
name: "python-3.11-ml"
version: "3.11.9"
description: "Python 3.11 with ML packages optimized for $DETECTED_ARCH"
type: "python-ml"
created: "$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
system:
  architecture: "$DETECTED_ARCH"
  os: "$DETECTED_OS"
  distribution: "$DETECTED_DISTRO"
  package_manager: "$DETECTED_PACKAGE_MANAGER"
  ml_support: "$ML_SUPPORT"
paths:
  python_home: "$RUNTIME_DIR/python-install"
  venv_home: "$RUNTIME_DIR/ml-venv"
binaries:
  python: "$RUNTIME_DIR/ml-venv/bin/python"
  python3: "$RUNTIME_DIR/ml-venv/bin/python3"
  pip: "$RUNTIME_DIR/ml-venv/bin/pip"
  pip3: "$RUNTIME_DIR/ml-venv/bin/pip3"
mounts:
  - source: "ml-venv/bin"
    target: "/usr/local/bin"
    readonly: false
  - source: "python-install/lib"
    target: "/usr/local/lib"
    readonly: false
  - source: "ml-venv/lib/python3.11/site-packages"
    target: "/usr/local/lib/python3.11/site-packages"
    readonly: false
environment:
  PYTHON_HOME: "/usr/local"
  PYTHONPATH: "/usr/local/lib/python3.11/site-packages"
  LD_LIBRARY_PATH: "/usr/local/lib"
features:
  - "NumPy for numerical computing"
  - "Pandas for data analysis ($([[ $ML_SUPPORT != "limited" ]] && echo "available" || echo "limited"))"
  - "Scikit-learn for ML ($([[ $ML_SUPPORT == "full" ]] && echo "full support" || [[ $ML_SUPPORT == "most" ]] && echo "ARM64 support" || echo "not available"))"
  - "Matplotlib for visualization ($([[ $ML_SUPPORT != "limited" ]] && echo "available" || echo "limited"))"
  - "SciPy for scientific computing ($([[ $ML_SUPPORT != "limited" ]] && echo "available" || echo "limited"))"
  - "Architecture-optimized build"
packages:
EOF

    # Add architecture-specific package list
    if [[ $ML_SUPPORT == "full" ]]; then
        cat >> "$RUNTIME_DIR/runtime.yml" << EOF
  - "numpy>=1.24.3,<2.0"
  - "pandas>=2.0.3,<2.1"
  - "scikit-learn>=1.3.0,<1.4"
  - "matplotlib>=3.7.0,<3.8"
  - "seaborn>=0.12.0,<0.13"
  - "scipy>=1.11.0,<1.12"
  - "requests==2.31.0"
  - "openpyxl==3.1.2"
EOF
    elif [[ $ML_SUPPORT == "most" ]]; then
        cat >> "$RUNTIME_DIR/runtime.yml" << EOF
  - "numpy>=1.24.3,<2.0"
  - "pandas>=2.0.3,<2.1"
  - "scikit-learn>=1.3.0,<1.4 (ARM64)"
  - "matplotlib>=3.7.0,<3.8"
  - "seaborn>=0.12.0,<0.13"
  - "scipy>=1.11.0,<1.12"
  - "requests==2.31.0"
EOF
    else
        cat >> "$RUNTIME_DIR/runtime.yml" << EOF
  - "numpy>=1.24.3,<2.0 (ARM 32-bit)"
  - "requests==2.31.0"
EOF
    fi
}

# Test ML installation
test_ml_installation() {
    echo "🧪 Testing ML packages installation..."
    source "$RUNTIME_DIR/ml-venv/bin/activate"
    
    python -c "
import sys
print(f'✅ Python: {sys.version}')
print(f'✅ Architecture: $DETECTED_ARCH')
print()

packages = ['numpy', 'requests']
if '$ML_SUPPORT' != 'limited':
    packages.extend(['pandas', 'matplotlib', 'scipy'])
    if '$ML_SUPPORT' == 'full':
        packages.extend(['sklearn', 'seaborn'])

failed_packages = []

for pkg in packages:
    try:
        if pkg == 'sklearn':
            import sklearn as mod
        else:
            mod = __import__(pkg)
        version = getattr(mod, '__version__', 'unknown')
        print(f'✅ {pkg}: {version}')
        
        # Test basic functionality for critical packages
        if pkg == 'numpy':
            import numpy as np
            arr = np.array([1, 2, 3])
            print(f'   └─ Basic operations: ✅')
            
    except ImportError as e:
        print(f'❌ {pkg}: Missing - {e}')
        failed_packages.append(pkg)
    except Exception as e:
        print(f'⚠️  {pkg}: Available but has issues - {e}')

print()
if failed_packages:
    print(f'⚠️  Some packages had issues: {failed_packages}')
else:
    print('🎉 All expected ML packages working correctly!')
"
    
    deactivate
}

# Show installation summary
show_installation_summary() {
    echo ""
    echo "🎉 Python 3.11 + ML Runtime Installation Complete!"
    echo "=================================================="
    echo "✅ Architecture: $DETECTED_ARCH"
    echo "✅ Distribution: $DETECTED_DISTRO" 
    echo "✅ ML Support Level: $ML_SUPPORT"
    echo "✅ Python Version: $(echo "$INSTALLED_VERSION" | cut -d' ' -f2)"
    echo "✅ Installation Path: $RUNTIME_DIR"
    echo "✅ Runtime Manifest: $RUNTIME_DIR/runtime.yml"
    echo ""
    echo "🧪 Test the runtime:"
    echo "   rnx runtime list"
    echo "   rnx runtime info python-3.11-ml"
    echo "   rnx run --runtime=python-3.11-ml python --version"
    echo ""
    echo "📚 Example usage:"
    echo "   cd /opt/joblet/examples/python-3.11-ml"
    echo "   rnx run --template=jobs.yaml:ml-analysis"
    echo ""
    
    if [[ $ML_SUPPORT == "limited" ]]; then
        echo "⚠️  ARM 32-bit Limitations:"
        echo "   - Only NumPy and basic packages available"
        echo "   - For full ML stack, use ARM 64-bit or x86_64 architecture"
        echo ""
    fi
}

# Show usage if requested
if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
    echo "Multi-Architecture Python 3.11 + ML Runtime Setup"
    echo ""
    echo "Usage: sudo $0"
    echo ""
    echo "ML Package Support by Architecture:"
    echo "  amd64:  ✅ Full ML stack (NumPy, Pandas, Scikit-learn, etc.)"
    echo "  arm64:  ✅ Most ML packages (some may compile from source)"
    echo "  armhf:  ⚠️  Limited support (NumPy + basic packages only)"
    echo ""
    show_supported_platforms
    exit 0
fi

# Run main function
main "$@"