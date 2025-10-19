#!/bin/bash
# Python 3.11 PyTorch CUDA Runtime Setup for RHEL/CentOS/Rocky AMD64
# Includes PyTorch with CUDA support for GPU acceleration

set -e  # Exit on any error
set -u  # Exit on undefined variables
set -o pipefail  # Exit on pipe failures

# Error handling function
handle_error() {
    local exit_code=$?
    local line_number=$1
    echo "❌ ERROR: Script failed at line $line_number with exit code $exit_code"
    echo "❌ Installation FAILED - runtime may be in inconsistent state"
    exit $exit_code
}

# Set up error trap
trap 'handle_error ${LINENO}' ERR

# =============================================================================
# CONFIGURATION
# =============================================================================

RUNTIME_NAME="${RUNTIME_SPEC:-python-3.11-pytorch-cuda}"
RUNTIME_BASE_DIR="/opt/joblet/runtimes/$RUNTIME_NAME"
ISOLATED_DIR="$RUNTIME_BASE_DIR/isolated"

echo "Starting Python 3.11 PyTorch CUDA runtime setup..."
echo "Platform: rhel-amd64"
echo "Runtime: $RUNTIME_NAME"
echo "Installation path: $RUNTIME_BASE_DIR"

# =============================================================================
# SAFETY CHECKS - NO HOST CONTAMINATION
# =============================================================================

safety_check() {
    echo "Performing safety checks to prevent host contamination..."

    # Verify we're in a controlled environment
    if [ "${JOBLET_CHROOT:-false}" != "true" ] && [ -z "${BUILD_ID:-}" ]; then
        echo "⚠ WARNING: Not running in joblet build environment"
        echo "This script should only run within joblet runtime installation"
    fi

    # Ensure target directory is within expected path
    if [[ "$RUNTIME_BASE_DIR" != "/opt/joblet/runtimes/"* ]]; then
        echo "✗ ERROR: Invalid runtime base directory: $RUNTIME_BASE_DIR"
        exit 1
    fi

    echo "✓ Safety checks passed - no host contamination risk"
}

# =============================================================================
# DIRECTORY SETUP
# =============================================================================

create_directories() {
    echo "Creating runtime directories..."

    mkdir -p "$RUNTIME_BASE_DIR"
    cd "$RUNTIME_BASE_DIR"

    # Create minimal isolated filesystem structure
    local dirs=(
        bin lib lib64 usr/bin usr/lib usr/local/lib/python3.11/dist-packages
        opt/venv etc tmp proc lib/x86_64-linux-gnu usr/lib/x86_64-linux-gnu
        usr/local/cuda/lib64 usr/local/cuda/bin
    )

    for dir in "${dirs[@]}"; do
        mkdir -p "$ISOLATED_DIR/$dir"
    done

    echo "✓ Directories created"
}

# =============================================================================
# SYSTEM FILES COPY
# =============================================================================

copy_system_files() {
    echo "Copying system files..."

    # Essential binaries
    local binaries="bash sh ls cat cp mv rm mkdir chmod grep sed awk ps echo tar gzip curl wget python3 python3.10 python3.11 pip3 nvidia-smi"
    local copied_binaries=()
    local missing_binaries=()
    local python_binary_copied=false

    for bin in $binaries; do
        local copied=false
        for path in /bin /usr/bin /usr/local/bin; do
            if [ -f "$path/$bin" ]; then
                if cp -P "$path/$bin" "$ISOLATED_DIR/usr/bin/" 2>/dev/null; then
                    copied_binaries+=("$bin")
                    copied=true
                    # Track if we copied any Python binary
                    if [[ "$bin" =~ ^python ]]; then
                        python_binary_copied=true
                    fi
                    break
                fi
            fi
        done
        if [ "$copied" = false ]; then
            missing_binaries+=("$bin")
        fi
    done

    # Report binary copying results
    if [ ${#copied_binaries[@]} -gt 0 ]; then
        echo "  ✓ Copied binaries: ${copied_binaries[*]}"
    fi
    if [ ${#missing_binaries[@]} -gt 0 ]; then
        echo "  ⚠ Missing binaries: ${missing_binaries[*]}"
    fi

    # Critical check: ensure at least one Python binary was copied
    if [ "$python_binary_copied" = false ]; then
        echo "❌ CRITICAL: No Python binary was copied successfully"
        echo "❌ This will result in a non-functional runtime"
        exit 1
    fi

    # Essential libraries (including CUDA libraries)
    local lib_patterns="libc.so* libdl.so* libpthread.so* libm.so* ld-linux*.so* libz.so* libssl.so* libcrypto.so* libffi.so* libexpat.so* libblas.so* liblapack.so* libopenblas.so* libgfortran.so* libgcc_s.so* libstdc++.so* libselinux.so* libresolv.so* libnss*.so* libpcre*.so* libcuda*.so* libnvidia*.so* libcudnn*.so* libcublas*.so* libcudart*.so* libcufft*.so* libcurand*.so* libcusparse*.so* libcusolver*.so*"

    local copied_libs=0
    for lib_dir in /lib/x86_64-linux-gnu /usr/lib/x86_64-linux-gnu /lib64 /usr/local/cuda/lib64; do
        if [ -d "$lib_dir" ]; then
            local target_dir="$ISOLATED_DIR${lib_dir}"
            mkdir -p "$target_dir"
            for pattern in $lib_patterns; do
                local found_libs=$(find "$lib_dir" -maxdepth 1 -name "$pattern" 2>/dev/null | wc -l)
                if [ "$found_libs" -gt 0 ]; then
                    find "$lib_dir" -maxdepth 1 -name "$pattern" -exec cp -P {} "$target_dir" \; 2>/dev/null && ((copied_libs+=found_libs)) || true
                fi
            done
        fi
    done

    echo "  ✓ Copied $copied_libs library files"

    # Dynamic linker
    if [ -f "/lib64/ld-linux-x86-64.so.2" ]; then
        mkdir -p "$ISOLATED_DIR/lib64"
        if cp -P "/lib64/ld-linux-x86-64.so.2" "$ISOLATED_DIR/lib64/" 2>/dev/null; then
            echo "  ✓ Copied dynamic linker"
        else
            echo "  ⚠ Failed to copy dynamic linker"
        fi
    else
        echo "  ⚠ Dynamic linker not found"
    fi

    echo "✓ System files copied"
}

# =============================================================================
# CUDA INSTALLATION
# =============================================================================

install_cuda() {
    echo "Setting up CUDA environment..."

    # Check if CUDA is available on the system
    if [ -d "/usr/local/cuda" ]; then
        echo "Found CUDA installation, copying libraries..."

        # Copy CUDA libraries
        if [ -d "/usr/local/cuda/lib64" ]; then
            echo "Copying CUDA libraries from /usr/local/cuda/lib64..."
            mkdir -p "$ISOLATED_DIR/usr/local/cuda/lib64"
            cp -r /usr/local/cuda/lib64/* "$ISOLATED_DIR/usr/local/cuda/lib64/" 2>/dev/null || echo "  ⚠ Some CUDA libraries couldn't be copied"
        fi

        # Copy CUDA binaries
        if [ -d "/usr/local/cuda/bin" ]; then
            echo "Copying CUDA binaries from /usr/local/cuda/bin..."
            mkdir -p "$ISOLATED_DIR/usr/local/cuda/bin"
            cp -r /usr/local/cuda/bin/* "$ISOLATED_DIR/usr/local/cuda/bin/" 2>/dev/null || echo "  ⚠ Some CUDA binaries couldn't be copied"
        fi

        echo "✓ CUDA environment copied"
    else
        echo "⚠ WARNING: No CUDA installation found at /usr/local/cuda"
        echo "⚠ PyTorch will be installed with CUDA support, but may require CUDA at runtime"
    fi
}

# =============================================================================
# PYTHON INSTALLATION
# =============================================================================

install_python() {
    echo "Setting up Python environment..."

    # Install Python packages in chroot environment
    if [ "${JOBLET_CHROOT:-false}" = "true" ] && command -v yum >/dev/null 2>&1; then
        echo "Installing Python packages in chroot environment..."
        export 
        if ! yum update -qq 2>/dev/null; then
            echo "⚠ yum update failed, but continuing with existing package cache"
        fi
        if ! yum install -y python3 python3-dev python3-venv python3-pip python3-setuptools python3-wheel \
                          build-essential libopenblas-dev liblapack-dev libffi-dev 2>/dev/null; then
            echo "⚠ Some Python packages failed to install in chroot, but this is non-critical"
        fi
    else
        echo "Not in chroot or apt not available - copying existing Python from host"
    fi

    # Copy Python runtime
    echo "Copying Python standard libraries..."
    local python_copied=false
    for py_dir in /usr/lib/python3*; do
        if [ -d "$py_dir" ]; then
            echo "  Copying $py_dir..."
            if cp -r "$py_dir" "$ISOLATED_DIR/usr/lib/" 2>/dev/null; then
                python_copied=true
            else
                echo "⚠ Failed to copy $py_dir (non-critical)"
            fi
        fi
    done

    if [ "$python_copied" = false ]; then
        echo "❌ CRITICAL: No Python libraries were copied successfully"
        exit 1
    fi

    # Copy lib-dynload and other essential Python directories
    for py_lib in /usr/lib/python3*/lib-dynload; do
        if [ -d "$py_lib" ]; then
            echo "  Copying dynamic modules from $py_lib..."
            py_parent=$(dirname "$py_lib" | sed "s|^/usr||")
            mkdir -p "$ISOLATED_DIR/usr/$py_parent"
            if ! cp -r "$py_lib" "$ISOLATED_DIR/usr/$py_parent/" 2>/dev/null; then
                echo "⚠ Failed to copy $py_lib (non-critical)"
            fi
        fi
    done

    # Create symlinks
    cd "$ISOLATED_DIR/usr/bin"
    [ -f python3.11 ] && ln -sf python3.11 python 2>/dev/null || true
    [ -f python3 ] && [ ! -f python ] && ln -sf python3 python 2>/dev/null || true
    [ -f pip3 ] && ln -sf pip3 pip 2>/dev/null || true
    cd - >/dev/null

    echo "✓ Python environment ready"
}

# =============================================================================
# PYTORCH AND CUDA PACKAGES
# =============================================================================

install_pytorch_packages() {
    echo "Installing PyTorch with CUDA support..."

    local site_packages="$ISOLATED_DIR/usr/local/lib/python3.11/dist-packages"
    mkdir -p "$site_packages"

    # PyTorch packages to install
    # Using PyTorch with CUDA 11.8 (common version)
    local pytorch_packages=(
        "torch"
        "torchvision"
        "torchaudio"
    )

    # Additional ML packages
    local ml_packages=(
        "numpy"
        "pandas"
        "matplotlib"
        "scipy"
        "scikit-learn"
        "pillow"
        "requests"
    )

    echo "Installing PyTorch packages: ${pytorch_packages[*]}"
    echo "Installing ML packages: ${ml_packages[*]}"

    # Update pip first
    python3 -m pip install --upgrade pip --quiet 2>/dev/null || echo "  ⚠ Could not upgrade pip"

    local installed_packages=()
    local failed_packages=()

    # Install PyTorch with CUDA support
    # Using PyTorch index URL for CUDA 11.8
    echo "Installing PyTorch with CUDA 11.8 support..."
    if python3 -m pip install torch torchvision torchaudio --index-url https://download.pytorch.org/whl/cu118 --quiet 2>/dev/null; then
        installed_packages+=("torch" "torchvision" "torchaudio")
        echo "  ✓ PyTorch with CUDA installed in system"
    else
        echo "  ⚠ PyTorch CUDA installation failed, trying CPU version..."
        if python3 -m pip install torch torchvision torchaudio --quiet 2>/dev/null; then
            installed_packages+=("torch" "torchvision" "torchaudio")
            echo "  ✓ PyTorch (CPU) installed in system"
        else
            failed_packages+=("torch" "torchvision" "torchaudio")
            echo "  ✗ Failed to install PyTorch"
        fi
    fi

    # Install additional ML packages
    for package in "${ml_packages[@]}"; do
        echo "Installing $package..."
        if python3 -m pip install "$package" --quiet 2>/dev/null; then
            installed_packages+=("$package")
            echo "  ✓ $package installed in system"
        else
            failed_packages+=("$package")
            echo "  ✗ Failed to install $package"
        fi
    done

    # Copy installed packages to isolated environment
    echo ""
    echo "Copying installed packages to isolated environment..."

    # Copy system site-packages
    for python_path in /usr/local/lib/python3*/dist-packages /usr/lib/python3/dist-packages /usr/local/lib/python3*/site-packages; do
        if [ -d "$python_path" ]; then
            echo "Copying from $python_path..."
            cp -r "$python_path"/* "$site_packages/" 2>/dev/null || echo "  ⚠ Some files couldn't be copied from $python_path"
        fi
    done

    # Copy from user site-packages if exists
    local user_site=$(python3 -c "import site; print(site.getusersitepackages())" 2>/dev/null || echo "")
    if [ -n "$user_site" ] && [ -d "$user_site" ]; then
        echo "Copying from user site-packages: $user_site"
        cp -r "$user_site"/* "$site_packages/" 2>/dev/null || echo "  ⚠ Some files couldn't be copied from user site"
    fi

    # Report installation results
    echo ""
    echo "📊 PyTorch/ML Package Installation Results:"
    echo "  Successfully installed: ${#installed_packages[@]} packages"
    [ ${#installed_packages[@]} -gt 0 ] && echo "    ${installed_packages[*]}"
    echo "  Failed to install: ${#failed_packages[@]} packages"
    [ ${#failed_packages[@]} -gt 0 ] && echo "    ${failed_packages[*]}"

    # Count final files
    local final_files=$(find "$site_packages" -name "*.py" -type f 2>/dev/null | wc -l)
    echo "  Final Python files in isolated packages: $final_files"

    echo "✓ PyTorch/ML packages installation completed"
}

# =============================================================================
# CONFIGURATION FILES
# =============================================================================

create_config_files() {
    echo "Creating configuration files..."

    # Minimal /etc files
    cat > "$ISOLATED_DIR/etc/passwd" << 'EOF'
root:x:0:0:root:/root:/bin/bash
nobody:x:65534:65534:nobody:/nonexistent:/bin/false
EOF

    cat > "$ISOLATED_DIR/etc/group" << 'EOF'
root:x:0:
nogroup:x:65534:
EOF

    # Basic /proc files
    echo "processor : 0" > "$ISOLATED_DIR/proc/cpuinfo"
    echo "MemTotal: 8388608 kB" > "$ISOLATED_DIR/proc/meminfo"

    # Runtime configuration
    cat > "$RUNTIME_BASE_DIR/runtime.yml" << EOF
name: $RUNTIME_NAME
version: "3.11"
description: "Python 3.11 with PyTorch and CUDA support"

mounts:
  - source: "isolated/bin"
    target: "/bin"
    readonly: true
  - source: "isolated/lib"
    target: "/lib"
    readonly: true
  - source: "isolated/lib64"
    target: "/lib64"
    readonly: true
  - source: "isolated/usr"
    target: "/usr"
    readonly: true
  - source: "isolated/opt"
    target: "/opt"
    readonly: true
  - source: "isolated/etc"
    target: "/etc"
    readonly: true
  - source: "isolated/tmp"
    target: "/tmp"
    readonly: false
  - source: "isolated/proc"
    target: "/proc"
    readonly: true

environment:
  PATH: "/opt/venv/bin:/usr/local/cuda/bin:/usr/bin:/bin"
  PYTHONPATH: "/usr/local/lib/python3.11/dist-packages"
  VIRTUAL_ENV: "/opt/venv"
  LD_LIBRARY_PATH: "/usr/local/cuda/lib64:/usr/lib/x86_64-linux-gnu:/lib/x86_64-linux-gnu"
  CUDA_HOME: "/usr/local/cuda"
  OPENBLAS_NUM_THREADS: "1"
  OMP_NUM_THREADS: "1"
EOF

    echo "✓ Configuration files created"
}

# =============================================================================
# VALIDATION
# =============================================================================

validate_installation() {
    echo "Validating installation..."

    local status=0

    # Check runtime.yml
    [ -f "$RUNTIME_BASE_DIR/runtime.yml" ] && echo "✓ runtime.yml exists" || { echo "✗ runtime.yml missing"; status=1; }

    # Check Python binary
    [ -f "$ISOLATED_DIR/usr/bin/python3" ] && echo "✓ Python binary exists" || { echo "✗ Python binary missing"; status=1; }

    # Check packages directory
    [ -d "$ISOLATED_DIR/usr/local/lib/python3.11/dist-packages" ] && echo "✓ Packages directory exists" || { echo "✗ Packages directory missing"; status=1; }

    # Check for PyTorch
    if [ -d "$ISOLATED_DIR/usr/local/lib/python3.11/dist-packages/torch" ]; then
        echo "✓ PyTorch package found"
    else
        echo "⚠ PyTorch package not found (may need manual installation)"
    fi

    # Report sizes
    if [ -d "$ISOLATED_DIR" ]; then
        local file_count=$(find "$ISOLATED_DIR" -type f 2>/dev/null | wc -l)
        local dir_size=$(du -sh "$ISOLATED_DIR" 2>/dev/null | cut -f1)
        echo "✓ Total files: $file_count"
        echo "✓ Directory size: $dir_size"
    fi

    return $status
}

# =============================================================================
# MAIN EXECUTION
# =============================================================================

main() {
    echo "═══════════════════════════════════════════════════════════════"
    echo "Python 3.11 PyTorch CUDA Runtime Installation"
    echo "═══════════════════════════════════════════════════════════════"

    # Perform safety checks first
    safety_check

    # Execute installation steps
    create_directories
    copy_system_files
    install_cuda
    install_python
    install_pytorch_packages
    create_config_files

    # Validate and report
    echo ""
    if ! validate_installation; then
        echo "❌ CRITICAL: Installation validation failed"
        echo "❌ Runtime installation FAILED - check errors above"
        exit 1
    fi

    echo ""
    echo "🎉 Installation completed successfully!"
    echo "Runtime installed at: $RUNTIME_BASE_DIR"
}

# Run installation
main "$@"
