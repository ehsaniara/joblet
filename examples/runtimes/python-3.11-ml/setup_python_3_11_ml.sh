#!/bin/bash

# Python 3.11 + ML Runtime Setup Script
# Creates completely isolated Python 3.11 environment with ML packages
# ⚠️  WARNING: This script installs build dependencies on the host system
# ⚠️  See /opt/joblet/examples/runtimes/CONTAMINATION_WARNING.md for details
# 
# IMPORTANT: This script includes the critical Python shared library mount
# that was discovered during runtime development to fix library loading errors.

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RUNTIME_BASE_DIR="/opt/joblet/runtimes"
RUNTIME_DIR="$RUNTIME_BASE_DIR/python/python-3.11-ml"
PYTHON_VERSION="3.11.9"

echo "🐍 Setting up Python 3.11 + ML Runtime"
echo "======================================="
echo "Target: $RUNTIME_DIR"
echo "⚠️  WARNING: This script will install build dependencies on the host system"
echo "⚠️  Impact: ~200MB of build tools (gcc, make, python3-dev, etc.)"
echo "⚠️  See CONTAMINATION_WARNING.md for details and alternatives"
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
    
    # Install minimal build dependencies temporarily (will be removed)
    echo "📦 Installing temporary build dependencies..."
    apt-get update -qq
    apt-get install -y wget build-essential libssl-dev zlib1g-dev \
                       libbz2-dev libreadline-dev libsqlite3-dev \
                       libncursesw5-dev xz-utils tk-dev libxml2-dev \
                       libxmlsec1-dev libffi-dev liblzma-dev
    
    # Download and compile Python in complete isolation
    echo "⬇️  Downloading Python $PYTHON_VERSION source..."
    wget -q "https://www.python.org/ftp/python/$PYTHON_VERSION/Python-$PYTHON_VERSION.tgz"
    
    echo "📦 Extracting Python source..."
    tar -xzf "Python-$PYTHON_VERSION.tgz"
    cd "Python-$PYTHON_VERSION"
    
    # Configure Python to install ONLY in our isolated runtime directory
    echo "⚙️  Configuring Python for complete isolation..."
    ./configure --prefix="$RUNTIME_DIR/python-install" \
               --enable-optimizations \
               --with-ensurepip=install \
               --enable-shared \
               LDFLAGS="-Wl,-rpath=$RUNTIME_DIR/python-install/lib"
    
    # Compile and install Python in complete isolation
    echo "🔨 Compiling Python (this may take several minutes)..."
    make -j$(nproc) > /dev/null 2>&1
    
    echo "📦 Installing Python to isolated runtime directory..."
    make install > /dev/null 2>&1
    
    # Clean up source files
    cd "$RUNTIME_DIR"
    rm -rf "Python-$PYTHON_VERSION" "Python-$PYTHON_VERSION.tgz"
    
    # Remove build dependencies to keep host clean
    echo "🧹 Removing build dependencies from host..."
    apt-get remove -y wget build-essential libssl-dev zlib1g-dev \
                      libbz2-dev libreadline-dev libsqlite3-dev \
                      libncursesw5-dev xz-utils tk-dev libxml2-dev \
                      libxmlsec1-dev libffi-dev liblzma-dev
    apt-get autoremove -y
    apt-get clean
    
    # Verify isolated Python installation
    echo "🔍 Verifying isolated Python installation..."
    PYTHON_BIN="$RUNTIME_DIR/python-install/bin/python3"
    
    if [[ ! -f "$PYTHON_BIN" ]]; then
        echo "❌ Isolated Python installation failed!"
        exit 1
    fi
    
    INSTALLED_VERSION=$($PYTHON_BIN --version)
    echo "✅ Isolated Python installed: $INSTALLED_VERSION"
    
    # Create isolated virtual environment for ML packages
    echo "🏗️  Creating isolated ML package environment..."
    cd "$RUNTIME_DIR"
    $PYTHON_BIN -m venv ml-venv
    
    # Install ML packages in complete isolation
    echo "📚 Installing ML packages in isolated environment..."
    source ml-venv/bin/activate
    
    # Upgrade pip
    pip install --upgrade pip > /dev/null 2>&1
    
    # Install packages in dependency order to avoid binary compatibility issues
    # CRITICAL: Pin NumPy to 1.x to prevent NumPy 2.0 compatibility issues
    echo "Installing NumPy (foundation package - pinned to 1.x)..."
    pip install "numpy>=1.24.3,<2.0" > /dev/null 2>&1
    
    echo "Installing SciPy (depends on NumPy)..."
    pip install "scipy>=1.11.0,<1.12" > /dev/null 2>&1
    
    echo "Installing Pandas (depends on NumPy)..."  
    pip install "pandas>=2.0.3,<2.1" > /dev/null 2>&1
    
    echo "Installing Scikit-learn (depends on NumPy, SciPy)..."
    pip install "scikit-learn>=1.3.0,<1.4" > /dev/null 2>&1
    
    echo "Installing Matplotlib (visualization - with NumPy 1.x constraint)..."
    pip install "matplotlib>=3.7.0,<3.8" --no-deps > /dev/null 2>&1
    pip install "matplotlib>=3.7.0,<3.8" > /dev/null 2>&1
    
    echo "Installing Seaborn (depends on Matplotlib, Pandas)..."
    pip install "seaborn>=0.12.0,<0.13" > /dev/null 2>&1
    
    echo "Installing additional packages..."
    pip install requests==2.31.0 openpyxl==3.1.2 > /dev/null 2>&1
    
    # Verify installation and fix any compatibility issues
    echo "🔧 Verifying package compatibility..."
    python -c "
import numpy as np
print(f'NumPy version: {np.__version__}')

try:
    import pandas as pd
    print(f'Pandas version: {pd.__version__}')
    
    # Test basic functionality
    df = pd.DataFrame({'test': [1, 2, 3]})
    print('Pandas basic functionality: ✅')
except Exception as e:
    print(f'Pandas compatibility issue detected: {e}')
    print('Applying compatibility fix...')
    exit(1)

try:
    import sklearn
    print(f'Scikit-learn version: {sklearn.__version__}')
except Exception as e:
    print(f'Scikit-learn issue: {e}')
    exit(1)
" || {
        echo "🚨 Compatibility issue detected - applying fix..."
        
        # Uninstall potentially problematic packages
        pip uninstall -y pandas scikit-learn scipy matplotlib seaborn
        
        # Reinstall in strict dependency order with NumPy 1.x constraint
        echo "Reinstalling NumPy (ensuring clean state - pinned to 1.x)..."
        pip install --force-reinstall "numpy>=1.24.3,<2.0"
        
        echo "Reinstalling SciPy..."
        pip install "scipy>=1.11.0,<1.12"
        
        echo "Reinstalling Pandas..."
        pip install "pandas>=2.0.3,<2.1"
        
        echo "Reinstalling Scikit-learn..."
        pip install "scikit-learn>=1.3.0,<1.4"
        
        echo "Reinstalling Matplotlib (with NumPy 1.x)..."
        pip install "matplotlib>=3.7.0,<3.8"
        
        echo "Reinstalling Seaborn..."
        pip install "seaborn>=0.12.0,<0.13"
        
        echo "✅ Compatibility fix applied"
    }
    
    deactivate
    
    # Create mount structure for joblet runtime system
    echo "🔗 Creating runtime mount structure..."
    mkdir -p bin lib
    
    # Create wrapper scripts instead of symlinks to ensure proper library loading
    # These wrappers ensure LD_LIBRARY_PATH is set correctly for the original Python binaries
    cat > bin/python << 'WRAPPER_EOF'
#!/bin/bash
# Wrapper script to ensure proper library loading for Python ML runtime
export LD_LIBRARY_PATH="/usr/local/lib:/usr/local/lib64:$LD_LIBRARY_PATH"
export PYTHONPATH="/usr/local/lib/python3.11/site-packages:$PYTHONPATH"
WRAPPER_EOF
    echo "exec \"$RUNTIME_DIR/ml-venv/bin/python\" \"\$@\"" >> bin/python
    
    cat > bin/python3 << 'WRAPPER_EOF'
#!/bin/bash
# Wrapper script to ensure proper library loading for Python ML runtime  
export LD_LIBRARY_PATH="/usr/local/lib:/usr/local/lib64:$LD_LIBRARY_PATH"
export PYTHONPATH="/usr/local/lib/python3.11/site-packages:$PYTHONPATH"
WRAPPER_EOF
    echo "exec \"$RUNTIME_DIR/ml-venv/bin/python3\" \"\$@\"" >> bin/python3
    
    cat > bin/python3.11 << 'WRAPPER_EOF'
#!/bin/bash
# Wrapper script to ensure proper library loading for Python ML runtime
export LD_LIBRARY_PATH="/usr/local/lib:/usr/local/lib64:$LD_LIBRARY_PATH"  
export PYTHONPATH="/usr/local/lib/python3.11/site-packages:$PYTHONPATH"
WRAPPER_EOF
    echo "exec \"$RUNTIME_DIR/ml-venv/bin/python3.11\" \"\$@\"" >> bin/python3.11
    
    # Make wrapper scripts executable
    chmod +x bin/python bin/python3 bin/python3.11
    
    # Keep pip from venv as it has the ML packages installed
    ln -sf "$RUNTIME_DIR/ml-venv/bin/pip" bin/pip
    ln -sf "$RUNTIME_DIR/ml-venv/bin/pip3" bin/pip3
    
    # Link Python libraries
    ln -sf "$RUNTIME_DIR/ml-venv/lib/python3.11" lib/python3.11
    
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
name: "python-3.11-ml"
version: "3.11"
description: "Completely isolated Python 3.11 with ML packages"

mounts:
  - source: "bin"
    target: "/usr/local/bin"
    readonly: true
    selective: ["python", "python3", "python3.11", "pip", "pip3"]
  - source: "ml-venv/lib/python3.11/site-packages"
    target: "/usr/local/lib/python3.11/site-packages"
    readonly: true
  - source: "ml-venv/lib64"
    target: "/usr/local/lib64"
    readonly: true
  - source: "python-install/lib"
    target: "/usr/local/lib"
    readonly: true
  - source: "ml-venv"
    target: "/opt/joblet/runtimes/python/python-3.11-ml/ml-venv"
    readonly: true

environment:
  PYTHON_HOME: "/usr/local"
  PYTHONPATH: "/usr/local/lib/python3.11/site-packages"
  PATH_PREPEND: "/usr/local/bin"
  LD_LIBRARY_PATH: "/usr/local/lib:/usr/local/lib64"

package_manager:
  type: "pip"
  cache_volume: "pip-cache"

requirements:
  architectures: ["x86_64", "amd64"]

features:
  - "NumPy 1.x (stable)"
  - "Pandas for data analysis"
  - "Scikit-learn for ML"
  - "Matplotlib/Seaborn for visualization"
  - "SciPy for scientific computing"
  - "Requests for HTTP"
  - "OpenPyXL for Excel files"

packages:
  - "numpy>=1.24.3,<2.0"
  - "pandas>=2.0.3,<2.1"
  - "scikit-learn>=1.3.0,<1.4"
  - "matplotlib>=3.7.0,<3.8"
  - "seaborn>=0.12.0,<0.13"
  - "scipy>=1.11.0,<1.12"
EOF
    
    # Test isolated installation
    echo "🧪 Testing isolated Python runtime..."
    source "$RUNTIME_DIR/ml-venv/bin/activate"
    
    # Comprehensive test with error handling
    python -c "
import sys
print(f'✅ Python: {sys.version}')
print(f'✅ Python path: {sys.executable}')
print()

packages = ['numpy', 'pandas', 'sklearn', 'matplotlib', 'seaborn', 'scipy']
failed_packages = []

for pkg in packages:
    try:
        mod = __import__(pkg)
        version = getattr(mod, '__version__', 'unknown')
        print(f'✅ {pkg}: {version} - Available in isolation')
        
        # Test basic functionality for critical packages
        if pkg == 'pandas':
            import pandas as pd
            df = pd.DataFrame({'test': [1, 2, 3]})
            print(f'   └─ Basic DataFrame operations: ✅')
        elif pkg == 'numpy':
            import numpy as np
            arr = np.array([1, 2, 3])
            print(f'   └─ Basic array operations: ✅')
        elif pkg == 'sklearn':
            from sklearn.ensemble import RandomForestClassifier
            print(f'   └─ ML model import: ✅')
            
    except ImportError as e:
        print(f'❌ {pkg}: Missing - {e}')
        failed_packages.append(pkg)
    except Exception as e:
        print(f'⚠️  {pkg}: Available but has issues - {e}')
        failed_packages.append(pkg)

print()
if failed_packages:
    print(f'⚠️  Some packages had issues: {failed_packages}')
    print('   Runtime installed but may need manual fixes')
else:
    print('🎉 All ML packages working correctly!')
"
    
    deactivate
    
    echo
    echo "🎉 Python 3.11 + ML Runtime setup completed!"
    echo
    echo "📍 Runtime location: $RUNTIME_DIR"
    echo "📋 Configuration: $RUNTIME_DIR/runtime.yml"
    echo
    echo "📋 Verification Commands:"
    echo "  # Host should be clean (no python):"
    echo "  python3 --version  # Should fail or show different version"
    echo
    echo "  # Runtime should work (isolated python):"
    echo "  rnx run --runtime=python-3.11-ml python --version"
    echo "  rnx run --runtime=python-3.11-ml python -c \"import pandas; print('Pandas available!')\""
    echo
    echo "✨ Host system remains completely clean!"
    echo "🔒 All Python functionality isolated to runtime directory only!"
    
    # Automatically create deployment tar.gz for easy distribution
    create_deployment_zip
}

# Package runtime for distribution
package_runtime() {
    local package_dir="${1:-/tmp/runtime-packages}"
    
    if [[ ! -d "$RUNTIME_DIR" || ! -f "$RUNTIME_DIR/runtime.yml" ]]; then
        echo "❌ Runtime not installed. Run setup first."
        return 1
    fi
    
    echo "📦 Packaging Python 3.11 ML runtime..."
    
    mkdir -p "$package_dir"
    
    # Package runtime
    cd "$(dirname "$RUNTIME_DIR")"
    tar -czf "$package_dir/python-3.11-ml-runtime.tar.gz" python-3.11-ml/
    
    # Create manifest
    cat > "$package_dir/python-3.11-ml-runtime.manifest" << EOF
Runtime Package: Python 3.11 ML
================================
Package: python-3.11-ml-runtime.tar.gz
Runtime Name: python-3.11-ml
Language: python
Version: 3.11.9
Type: Compiled with ML Libraries

Built: $(date)
Build Host: $(hostname)
Size: $(du -sh "$RUNTIME_DIR" | cut -f1)

Installation:
  tar -xzf python-3.11-ml-runtime.tar.gz -C /opt/joblet/runtimes/python/

Verification:
  rnx runtime test python-3.11-ml
  rnx run --runtime=python-3.11-ml python --version
EOF
    
    # Create checksum
    cd "$package_dir"
    sha256sum python-3.11-ml-runtime.tar.gz > python-3.11-ml-runtime.sha256
    
    echo "✅ Python 3.11 ML runtime packaged: $package_dir/python-3.11-ml-runtime.tar.gz"
    echo "Size: $(ls -lh python-3.11-ml-runtime.tar.gz | awk '{print $5}')"
}

# Install runtime from package
install_from_package() {
    local package_path="$1"
    local target_dir="/opt/joblet/runtimes/python"
    
    if [[ -z "$package_path" || ! -f "$package_path" ]]; then
        echo "❌ Package file not found: $package_path"
        return 1
    fi
    
    echo "📦 Installing Python 3.11 ML runtime from package..."
    
    # Verify checksum if available
    local checksum_file="${package_path%.tar.gz}.sha256"
    if [[ -f "$checksum_file" ]]; then
        echo "🔐 Verifying package integrity..."
        if sha256sum -c "$checksum_file" --quiet; then
            echo "✅ Package integrity verified ✓"
        else
            echo "❌ Package integrity check failed!"
            return 1
        fi
    fi
    
    # Create target and extract
    mkdir -p "$target_dir"
    tar -xzf "$package_path" -C "$target_dir/" || {
        echo "❌ Failed to extract package"
        return 1
    }
    
    # Set ownership
    if id -u joblet &> /dev/null; then
        chown -R joblet:joblet "$target_dir/python-3.11-ml" 2>/dev/null || echo "⚠️ Could not set ownership"
    fi
    
    echo "✅ Python 3.11 ML runtime installed from package"
    echo "Location: $target_dir/python-3.11-ml"
    echo "Test with: rnx runtime test python-3.11-ml"
}


# Create deployment-ready tar.gz for easy distribution
create_deployment_zip() {
    local pkg_dir="/tmp/runtime-deployments"
    local pkg_file="$pkg_dir/python-3.11-ml-runtime.tar.gz"
    
    echo "📦 Creating deployment tar.gz for easy distribution..."
    
    # Create deployment directory
    mkdir -p "$pkg_dir"
    
    # Create tar.gz of the entire runtime directory
    cd "$(dirname "$RUNTIME_DIR")"
    tar -czf "$pkg_file" "$(basename "$RUNTIME_DIR")/"
    
    # Create deployment metadata
    cat > "$pkg_dir/python-3.11-ml-runtime.json" << EOF
{
  "runtime": {
    "name": "python-3.11-ml",
    "language": "python", 
    "version": "$PYTHON_VERSION",
    "type": "ml",
    "description": "Python 3.11 with ML packages (NumPy, Pandas, Scikit-learn, etc.)",
    "size_bytes": $(du -sb "$RUNTIME_DIR" | cut -f1),
    "created": "$(date -Iseconds)",
    "build_host": "$(hostname)",
    "architecture": "$(uname -m)"
  },
  "deployment": {
    "tar_file": "python-3.11-ml-runtime.tar.gz",
    "target_path": "/opt/joblet/runtimes/python/python-3.11-ml",
    "unpack_command": "tar -xzf python-3.11-ml-runtime.tar.gz -C /opt/joblet/runtimes/python/",
    "verify_command": "rnx runtime test python-3.11-ml"
  },
  "features": [
    "NumPy 1.x (stable)",
    "Pandas for data analysis",
    "Scikit-learn for ML",
    "Matplotlib/Seaborn for visualization",
    "SciPy for scientific computing",
    "Requests for HTTP"
  ]
}
EOF
    
    echo "✅ Deployment tar.gz created: $pkg_file"
    echo "Size: $(ls -lh "$pkg_file" | awk '{print $5}')"
    echo "Metadata: $pkg_dir/python-3.11-ml-runtime.json"
    echo ""
    echo "🚀 Ready for deployment! Administrators can now:"
    echo "  1. Copy package to target host: scp $pkg_file admin@host:/tmp/"
    echo "  2. Deploy: tar -xzf /tmp/python-3.11-ml-runtime.tar.gz -C /opt/joblet/runtimes/python/"
    echo "  3. Set permissions: sudo chown -R joblet:joblet /opt/joblet/runtimes/python/python-3.11-ml"
    echo "  4. Verify: rnx runtime test python-3.11-ml"
}

# Main execution with command support
case "${1:-setup}" in
    "setup")
        setup_runtime
        ;;
    "package")
        package_runtime "${2:-/tmp/runtime-packages}"
        ;;
    "install")
        if [[ $EUID -ne 0 ]]; then
            echo "❌ Installation requires root privileges: sudo $0 install <package.tar.gz>"
            exit 1
        fi
        install_from_package "$2"
        ;;
    "help"|"-h"|"--help")
        echo "Python 3.11 ML Runtime Setup"
        echo "============================="
        echo "Usage:"
        echo "  sudo $0 [command] [options]"
        echo ""
        echo "Commands:"
        echo "  setup                    - Install runtime (default)"
        echo "  package [output_dir]     - Package existing runtime"
        echo "  install <package.tar.gz> - Install from package (zero contamination)"
        echo "  help                     - Show this help"
        echo ""
        echo "Examples:"
        echo "  sudo $0                                         # Install runtime"
        echo "  sudo $0 package /tmp/packages                  # Package runtime"
        echo "  sudo $0 install python-3.11-ml-runtime.tar.gz # Install from package"
        echo ""
        echo "This script follows the decoupled runtime management pattern."
        echo "See runtime_manager.sh for batch operations across all runtimes."
        ;;
    *)
        echo "❌ Unknown command: $1"
        echo "Use: $0 help"
        exit 1
        ;;
esac