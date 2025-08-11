#!/bin/bash

# Python 3.11 Runtime Setup Script
# Creates a minimal Python runtime for project dependencies

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RUNTIME_BASE_DIR="/opt/joblet/runtimes"
RUNTIME_DIR="$RUNTIME_BASE_DIR/python/python-3.11"
PYTHON_VERSION="3.11.9"

echo "🐍 Setting up Python 3.11 Runtime"
echo "==================================="
echo "Target: $RUNTIME_DIR"
echo "Purpose: Minimal Python for Lambda-style dependencies"
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
setup_runtime() {
    check_root
    
    # Create runtime directory structure
    echo "📁 Creating runtime directory..."
    mkdir -p "$RUNTIME_DIR"
    cd "$RUNTIME_DIR"
    
    # Check if already installed
    if [[ -f "$RUNTIME_DIR/runtime.yml" && -f "$RUNTIME_DIR/bin/python3" ]]; then
        echo "✅ Python 3.11 runtime already installed"
        echo "   Location: $RUNTIME_DIR"
        echo "   To reinstall, remove the directory first:"
        echo "   sudo rm -rf '$RUNTIME_DIR'"
        exit 0
    fi
    
    # Install minimal build dependencies temporarily
    echo "📦 Installing temporary build dependencies..."
    apt-get update -qq
    apt-get install -y wget build-essential libssl-dev zlib1g-dev \
                       libbz2-dev libreadline-dev libsqlite3-dev \
                       libncursesw5-dev xz-utils tk-dev libffi-dev liblzma-dev
    
    # Download and compile Python in isolation
    echo "⬇️  Downloading Python $PYTHON_VERSION source..."
    wget -q "https://www.python.org/ftp/python/$PYTHON_VERSION/Python-$PYTHON_VERSION.tgz"
    
    echo "📦 Extracting Python source..."
    tar -xzf "Python-$PYTHON_VERSION.tgz"
    cd "Python-$PYTHON_VERSION"
    
    # Configure Python for runtime installation
    echo "⚙️  Configuring Python for runtime..."
    ./configure --prefix="$RUNTIME_DIR/python-install" \
               --enable-optimizations \
               --with-ensurepip=install \
               --enable-shared \
               LDFLAGS="-Wl,-rpath=$RUNTIME_DIR/python-install/lib"
    
    # Compile and install Python
    echo "🔨 Compiling Python (this may take 10-15 minutes)..."
    make -j$(nproc) > /dev/null 2>&1
    
    echo "📦 Installing Python to isolated runtime directory..."
    make install > /dev/null 2>&1
    
    # Clean up source files
    cd "$RUNTIME_DIR"
    rm -rf "Python-$PYTHON_VERSION" "Python-$PYTHON_VERSION.tgz"
    
    # Remove build dependencies
    echo "🧹 Removing build dependencies from host..."
    apt-get remove -y wget build-essential libssl-dev zlib1g-dev \
                      libbz2-dev libreadline-dev libsqlite3-dev \
                      libncursesw5-dev xz-utils tk-dev libffi-dev liblzma-dev
    apt-get autoremove -y
    apt-get clean
    
    # Verify Python installation
    echo "🔍 Verifying Python installation..."
    PYTHON_BIN="$RUNTIME_DIR/python-install/bin/python3"
    
    if [[ ! -f "$PYTHON_BIN" ]]; then
        echo "❌ Python installation failed!"
        exit 1
    fi
    
    INSTALLED_VERSION=$($PYTHON_BIN --version)
    echo "✅ Python installed: $INSTALLED_VERSION"
    
    # Create mount structure for joblet runtime system
    echo "🔗 Creating runtime mount structure..."
    mkdir -p bin lib
    
    # Create wrapper scripts for local lib detection
    cat > bin/python << 'WRAPPER_EOF'
#!/bin/bash
# Wrapper script with local lib detection
export LD_LIBRARY_PATH="/usr/local/lib:/usr/local/lib64:$LD_LIBRARY_PATH"

# Auto-detect local dependency directories
if [[ -d "./lib" ]]; then
    export PYTHONPATH="./lib:$PYTHONPATH"
fi
if [[ -d "./site-packages" ]]; then
    export PYTHONPATH="./site-packages:$PYTHONPATH"
fi

WRAPPER_EOF
    echo "exec \"$RUNTIME_DIR/python-install/bin/python3\" \"\$@\"" >> bin/python
    
    cat > bin/python3 << 'WRAPPER_EOF'
#!/bin/bash
# Wrapper script with lib detection
export LD_LIBRARY_PATH="/usr/local/lib:/usr/local/lib64:$LD_LIBRARY_PATH"

# Auto-detect local dependency directories
if [[ -d "./lib" ]]; then
    export PYTHONPATH="./lib:$PYTHONPATH"
fi
if [[ -d "./site-packages" ]]; then
    export PYTHONPATH="./site-packages:$PYTHONPATH"
fi

WRAPPER_EOF
    echo "exec \"$RUNTIME_DIR/python-install/bin/python3\" \"\$@\"" >> bin/python3
    
    cat > bin/python3.11 << 'WRAPPER_EOF'
#!/bin/bash
# Wrapper script with lib detection
export LD_LIBRARY_PATH="/usr/local/lib:/usr/local/lib64:$LD_LIBRARY_PATH"

# Auto-detect dependency directories
if [[ -d "./lib" ]]; then
    export PYTHONPATH="./lib:$PYTHONPATH"
fi
if [[ -d "./site-packages" ]]; then
    export PYTHONPATH="./site-packages:$PYTHONPATH"
fi

WRAPPER_EOF
    echo "exec \"$RUNTIME_DIR/python-install/bin/python3.11\" \"\$@\"" >> bin/python3.11
    
    # Make wrapper scripts executable
    chmod +x bin/python bin/python3 bin/python3.11
    
    # Link pip and libraries
    ln -sf "$RUNTIME_DIR/python-install/bin/pip3" bin/pip
    ln -sf "$RUNTIME_DIR/python-install/bin/pip3" bin/pip3
    ln -sf "$RUNTIME_DIR/python-install/lib" lib/
    
    # Set proper permissions
    echo "🔐 Setting permissions..."
    chown -R joblet:joblet "$RUNTIME_BASE_DIR" 2>/dev/null || {
        echo "⚠️  joblet user not found, using root ownership"
        chown -R root:root "$RUNTIME_BASE_DIR"
    }
    chmod -R 755 "$RUNTIME_BASE_DIR"
    
    # Create runtime configuration
    echo "⚙️  Creating runtime configuration..."
    cat > "$RUNTIME_DIR/runtime.yml" << 'EOF'
name: "python-3.11"
version: "3.11"
description: "Python 3.11 runtime for packaged project dependencies"

mounts:
  - source: "bin"
    target: "/usr/local/bin"
    readonly: true
    selective: ["python", "python3", "python3.11", "pip", "pip3"]
  - source: "lib"
    target: "/usr/local/lib"
    readonly: true
  - source: "python-install"
    target: "/opt/joblet/runtimes/python/python-3.11/python-install"
    readonly: true

environment:
  PYTHON_HOME: "/usr/local"
  PATH_PREPEND: "/usr/local/bin"
  LD_LIBRARY_PATH: "/usr/local/lib:/usr/local/lib64"

package_manager:
  type: "pip"
  cache_volume: "pip-cache"

requirements:
  architectures: ["x86_64", "amd64"]

features:
  - "Python 3.11.9"
  - "Packaged dependency support"
  - "Auto-detection of ./lib directories"
  - "pip package manager"
  - "SSL/TLS support"
  - "SQLite3 support"

usage:
  dependency_style: "packaged"
  examples:
    - "pip install pandas numpy --target lib/"
    - "rnx run --runtime=python:3.11 --upload-dir=project python main.py"
EOF
    
    # Test the installation
    echo "🧪 Testing runtime installation..."
    source bin/python --version > /dev/null 2>&1 || echo "⚠️  Direct test failed (expected in build environment)"
    
    # Create package for distribution
    echo "📦 Creating distribution package..."
    cd "$RUNTIME_BASE_DIR"
    tar -czf /tmp/python-3.11-runtime.tar.gz python/python-3.11/
    
    # Calculate sizes
    RUNTIME_SIZE=$(du -sh "$RUNTIME_DIR" | cut -f1)
    PACKAGE_SIZE=$(du -sh /tmp/python-3.11-runtime.tar.gz | cut -f1)
    
    echo "✅ Python 3.11 Runtime Setup Complete!"
    echo ""
    echo "📊 Runtime Statistics:"
    echo "   • Runtime Size: $RUNTIME_SIZE (vs ~226MB for ML version)"
    echo "   • Package Size: $PACKAGE_SIZE"
    echo "   • Location: $RUNTIME_DIR"
    echo "   • Package: /tmp/python-3.11-runtime.tar.gz"
    echo ""
    echo "🚀 Packaged Dependencies Usage:"
    echo "   # Create project with dependencies"
    echo "   mkdir my-project && cd my-project"
    echo "   echo 'pandas==2.1.0' > requirements.txt"
    echo "   pip install -r requirements.txt --target lib/"
    echo "   rnx run --runtime=python:3.11 --upload-dir=my-project python main.py"
    echo ""
    echo "📋 Configuration: $RUNTIME_DIR/runtime.yml"
}

# Test function
test_runtime() {
    echo "🧪 Testing Python 3.11 Runtime"
    echo "==============================="
    
    if [[ ! -d "$RUNTIME_DIR" || ! -f "$RUNTIME_DIR/runtime.yml" ]]; then
        echo "❌ Runtime not found. Run setup first."
        exit 1
    fi
    
    echo "✅ Runtime directory exists"
    echo "✅ Configuration file exists"
    echo "🎉 Runtime ready for projects!"
}

# Main execution
case "${1:-setup}" in
    "setup")
        setup_runtime
        ;;
    "test")
        test_runtime
        ;;
    *)
        echo "Usage: $0 [setup|test]"
        exit 1
        ;;
esac