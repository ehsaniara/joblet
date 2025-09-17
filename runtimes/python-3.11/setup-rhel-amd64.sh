#!/bin/bash
# Basic Python 3 Runtime Setup for RHEL/CentOS AMD64
# Lightweight runtime using host Python for faster setup

set -e
set -u
set -o pipefail

RUNTIME_NAME="${RUNTIME_SPEC:-python-3.11}"
RUNTIME_BASE_DIR="/opt/joblet/runtimes/$RUNTIME_NAME"
ISOLATED_DIR="$RUNTIME_BASE_DIR/isolated"

echo "Starting Python 3 basic runtime setup..."
echo "Runtime: $RUNTIME_NAME"
echo "Installation path: $RUNTIME_BASE_DIR"

# Create directories
echo "Creating runtime directories..."
mkdir -p "$RUNTIME_BASE_DIR"
cd "$RUNTIME_BASE_DIR"

dirs=(
    bin usr/bin usr/lib usr/local/bin usr/local/lib
    lib lib64 usr/lib64 usr/local/lib/python3.11/site-packages
    etc tmp var/log var/cache
)

for dir in "${dirs[@]}"; do
    mkdir -p "$ISOLATED_DIR/$dir"
done

echo "✓ Directories created"

# Copy system files
echo "Copying system files..."

# Essential binaries
binaries="bash sh ls cat cp mv rm mkdir chmod grep sed awk ps echo tar gzip wget curl git"

for bin in $binaries; do
    for path in /bin /usr/bin /usr/local/bin; do
        [ -f "$path/$bin" ] && cp -P "$path/$bin" "$ISOLATED_DIR/usr/bin/" 2>/dev/null || true
    done
done

# Essential libraries
lib_patterns="libc.so* libdl.so* libpthread.so* libm.so* ld-linux*.so* libz.so* libssl.so* libcrypto.so* libffi.so* libbz2.so* libreadline.so* libsqlite3.so*"

for lib_dir in /lib64 /usr/lib64 /lib /usr/lib; do
    if [ -d "$lib_dir" ]; then
        mkdir -p "$ISOLATED_DIR${lib_dir}"
        for pattern in $lib_patterns; do
            find "$lib_dir" -maxdepth 1 -name "$pattern" -exec cp -P {} "$ISOLATED_DIR${lib_dir}/" \; 2>/dev/null || true
        done
    fi
done

# Dynamic linker
[ -f "/lib64/ld-linux-x86-64.so.2" ] && cp -P "/lib64/ld-linux-x86-64.so.2" "$ISOLATED_DIR/lib64/" 2>/dev/null || true

echo "✓ System files copied"

# Copy Python from host
echo "Installing Python from host..."

# Copy Python binaries
if command -v python3 >/dev/null 2>&1; then
    cp "$(which python3)" "$ISOLATED_DIR/usr/bin/" 2>/dev/null || true
    cp "$(which python3)" "$ISOLATED_DIR/usr/bin/python3.11" 2>/dev/null || true
fi

if command -v pip3 >/dev/null 2>&1; then
    cp "$(which pip3)" "$ISOLATED_DIR/usr/bin/" 2>/dev/null || true
    cp "$(which pip3)" "$ISOLATED_DIR/usr/bin/pip3.11" 2>/dev/null || true
fi

# Create symlinks
cd "$ISOLATED_DIR/usr/bin"
ln -sf python3.11 python 2>/dev/null || true
ln -sf pip3.11 pip 2>/dev/null || true

# Copy Python standard library
python_version=$(python3 -c "import sys; print('.'.join(map(str, sys.version_info[:2])))" 2>/dev/null || echo "3.11")
echo "Detected Python version: $python_version"

# Find and copy Python libraries (RHEL paths)
for lib_path in "/usr/lib/python$python_version" "/usr/lib64/python$python_version" "/usr/local/lib/python$python_version" "/usr/local/lib64/python$python_version"; do
    if [ -d "$lib_path" ]; then
        echo "Copying Python library from $lib_path..."
        target_dir=$(dirname "$ISOLATED_DIR$lib_path")
        mkdir -p "$target_dir/python$python_version"
        cp -r "$lib_path"/* "$target_dir/python$python_version/" 2>/dev/null || true

        # Also create a symlink for python3.11 compatibility
        if [ "$python_version" != "3.11" ]; then
            mkdir -p "$target_dir/python3.11"
            ln -sf "../python$python_version"/* "$target_dir/python3.11/" 2>/dev/null || true
        fi
        break
    fi
done

echo "✓ Python installation completed"

# Install basic packages
echo "Installing basic Python packages..."

# Install basic packages in system first
basic_packages=("requests" "urllib3" "certifi" "charset-normalizer" "idna")

for package in "${basic_packages[@]}"; do
    echo "Installing $package..."
    python3 -m pip install "$package" --quiet 2>/dev/null || echo "  ⚠ Failed to install $package"
done

# Copy packages to isolated environment
site_packages="$ISOLATED_DIR/usr/local/lib/python3.11/site-packages"
mkdir -p "$site_packages"

echo "Copying packages to isolated environment..."
for search_path in /usr/local/lib/python*/site-packages /usr/local/lib64/python*/site-packages /usr/lib/python*/site-packages /usr/lib64/python*/site-packages ~/.local/lib/python*/site-packages; do
    if [ -d "$search_path" ]; then
        for pkg_pattern in requests* urllib3* certifi* charset* idna* six* packaging* setuptools* wheel*; do
            for pkg_dir in "$search_path"/$pkg_pattern; do
                if [ -d "$pkg_dir" ]; then
                    pkg_name=$(basename "$pkg_dir")
                    if [ ! -d "$site_packages/$pkg_name" ]; then
                        echo "  Copying: $pkg_name"
                        cp -r "$pkg_dir" "$site_packages/"
                    fi
                fi
            done
        done
    fi
done

echo "✓ Package installation completed"

# Create config files
echo "Creating configuration files..."

# Basic /etc files
cat > "$ISOLATED_DIR/etc/passwd" << 'EOF'
root:x:0:0:root:/root:/bin/bash
python:x:1000:1000:python:/home/python:/bin/bash
EOF

cat > "$ISOLATED_DIR/etc/group" << 'EOF'
root:x:0:
python:x:1000:
EOF

# Runtime configuration
pythonpath="/usr/local/lib/python3.11/site-packages:/usr/lib/python$python_version/site-packages:/usr/lib64/python$python_version/site-packages"
cat > "$RUNTIME_BASE_DIR/runtime.yml" << EOF
name: $RUNTIME_NAME
version: "3.11"
description: "Basic Python runtime with essential packages"
type: "language-runtime"

mounts:
  - source: "isolated/bin"
    target: "/bin"
    readonly: true
  - source: "isolated/usr/bin"
    target: "/usr/bin"
    readonly: true
  - source: "isolated/usr/local/bin"
    target: "/usr/local/bin"
    readonly: true
  - source: "isolated/lib"
    target: "/lib"
    readonly: true
  - source: "isolated/lib64"
    target: "/lib64"
    readonly: true
  - source: "isolated/usr/lib"
    target: "/usr/lib"
    readonly: true
  - source: "isolated/usr/lib64"
    target: "/usr/lib64"
    readonly: true
  - source: "isolated/usr/local/lib"
    target: "/usr/local/lib"
    readonly: true
  - source: "isolated/etc"
    target: "/etc"
    readonly: true
  - source: "isolated/tmp"
    target: "/tmp"
    readonly: false
  - source: "isolated/var/log"
    target: "/var/log"
    readonly: false
  - source: "isolated/var/cache"
    target: "/var/cache"
    readonly: false

environment:
  PYTHON_HOME: "/usr/local"
  PATH: "/usr/local/bin:/usr/bin:/bin"
  PYTHONPATH: "$pythonpath"
  PYTHONUNBUFFERED: "1"
EOF

echo "✓ Configuration files created"

# Validation
echo "Validating installation..."

status=0

[ -f "$RUNTIME_BASE_DIR/runtime.yml" ] && echo "✓ runtime.yml exists" || { echo "✗ runtime.yml missing"; status=1; }
[ -f "$ISOLATED_DIR/usr/bin/python3.11" ] && echo "✓ Python 3.11 binary exists" || { echo "✗ Python 3.11 binary missing"; status=1; }
[ -f "$ISOLATED_DIR/usr/bin/pip3.11" ] && echo "✓ pip binary exists" || { echo "✗ pip binary missing"; status=1; }

# Report sizes
if [ -d "$ISOLATED_DIR" ]; then
    file_count=$(find "$ISOLATED_DIR" -type f 2>/dev/null | wc -l)
    dir_size=$(du -sh "$ISOLATED_DIR" 2>/dev/null | cut -f1)
    echo "✓ Total files: $file_count"
    echo "✓ Directory size: $dir_size"
fi

if [ $status -eq 0 ]; then
    echo ""
    echo "🎉 Python basic runtime installation completed successfully!"
    echo "Runtime installed at: $RUNTIME_BASE_DIR"
    echo ""
    echo "Usage examples:"
    echo "  rnx job run --runtime=python-3.11 python --version"
    echo "  rnx job run --runtime=python-3.11 python -c 'import sys; print(sys.version)'"
    echo "  rnx job run --runtime=python-3.11 python -c 'import requests; print(\"requests available\")'"
else
    echo ""
    echo "⚠ Installation completed with warnings"
fi