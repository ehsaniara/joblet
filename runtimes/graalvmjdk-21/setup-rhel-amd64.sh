#!/bin/bash
# Self-contained RHEL/CentOS AMD64 GraalVM JDK 21 Runtime Setup
# Downloads and installs GraalVM Community Edition JDK 21


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

PLATFORM="rhel"
ARCHITECTURE="amd64" 
RUNTIME_NAME="graalvmjdk-21"
GRAALVM_VERSION="21.0.1"
GRAALVM_URL="https://github.com/graalvm/graalvm-ce-builds/releases/download/jdk-${GRAALVM_VERSION}/graalvm-community-jdk-${GRAALVM_VERSION}_linux-x64_bin.tar.gz"

echo "Starting GraalVM JDK 21 runtime setup..."
echo "Platform: $PLATFORM"
echo "Architecture: $ARCHITECTURE" 
echo "GraalVM Version: $GRAALVM_VERSION"
echo "Build ID: ${BUILD_ID:-unknown}"
echo "Runtime Spec: ${RUNTIME_SPEC:-unknown}"

# =============================================================================
# EMBEDDED COMMON FUNCTIONS
# =============================================================================

# Setup runtime directory structure (flat structure as per design)
setup_runtime_directories() {
    local runtime_name="${1:-graalvmjdk-21}"
    
    # Use flat directory structure: /opt/joblet/runtimes/graalvmjdk-21
    export RUNTIME_BASE_DIR="/opt/joblet/runtimes/$runtime_name"
    export RUNTIME_YML="$RUNTIME_BASE_DIR/runtime.yml"
    
    echo "Creating runtime directory: $RUNTIME_BASE_DIR"
    mkdir -p "$RUNTIME_BASE_DIR"
    cd "$RUNTIME_BASE_DIR"
    
    echo "Creating isolated structure for runtime mounting..."
    mkdir -p isolated/usr/lib/jvm
    mkdir -p isolated/usr/bin
    mkdir -p isolated/bin
    mkdir -p isolated/usr/sbin
    mkdir -p isolated/usr/share/java
    mkdir -p isolated/etc/ssl/certs
    mkdir -p isolated/etc
    mkdir -p isolated/usr/share/ca-certificates
    mkdir -p isolated/usr/lib64
    mkdir -p isolated/lib64
    mkdir -p isolated/lib
    mkdir -p isolated/usr/lib
    mkdir -p isolated/usr/share/zoneinfo
    mkdir -p isolated/tmp
    
    echo "✓ Runtime directory structure created"
}

# Download and install GraalVM
install_graalvm() {
    echo ""
    echo "📦 Installing GraalVM JDK 21..."
    
    # Download GraalVM
    echo "Downloading GraalVM from: $GRAALVM_URL"
    wget -O graalvm.tar.gz "$GRAALVM_URL" || curl -L -o graalvm.tar.gz "$GRAALVM_URL"
    
    # Extract GraalVM to isolated directory
    echo "Extracting GraalVM..."
    tar -xzf graalvm.tar.gz -C isolated/usr/lib/jvm/
    
    # Find the extracted directory name
    GRAALVM_DIR=$(find isolated/usr/lib/jvm/ -name "graalvm-community-openjdk-*" -type d | head -1)
    if [ -z "$GRAALVM_DIR" ]; then
        echo "ERROR: Could not find extracted GraalVM directory"
        exit 1
    fi
    
    # Create symlink for easier access
    ln -sf "$(basename "$GRAALVM_DIR")" isolated/usr/lib/jvm/graalvm-21
    
    echo "GraalVM installed at: $GRAALVM_DIR"
    
    # Create symlinks in bin directories
    echo "Creating binary symlinks..."
    for binary in java javac jar jarsigner; do
        if [ -f "$GRAALVM_DIR/bin/$binary" ]; then
            ln -sf "../lib/jvm/graalvm-21/bin/$binary" "isolated/usr/bin/$binary"
            ln -sf "../usr/lib/jvm/graalvm-21/bin/$binary" "isolated/bin/$binary"
        fi
    done
    
    # Create GraalVM specific binaries symlinks
    for binary in native-image gu js node npm; do
        if [ -f "$GRAALVM_DIR/bin/$binary" ]; then
            ln -sf "../lib/jvm/graalvm-21/bin/$binary" "isolated/usr/bin/$binary"
            ln -sf "../usr/lib/jvm/graalvm-21/bin/$binary" "isolated/bin/$binary"
        fi
    done
    
    # Clean up download
    rm -f graalvm.tar.gz
    
    echo "✓ GraalVM JDK 21 installed"
}

# Install required system dependencies
install_dependencies() {
    echo ""
    echo "📦 Installing system dependencies..."
    
    # Install essential packages using yum/dnf
    if command -v dnf >/dev/null 2>&1; then
        PKG_MGR="dnf"
    else
        PKG_MGR="yum"
    fi
    
    $PKG_MGR install -y \
        wget \
        ca-certificates \
        glibc \
        libgcc \
        zlib \
        glibc-devel \
        gcc \
        gcc-c++ \
        make
    
    echo "✓ System dependencies installed"
}

# Copy system libraries and configuration
copy_system_files() {
    echo ""
    echo "📂 Copying system libraries and configuration..."
    
    # Copy essential system libraries
    echo "Copying system libraries..."
    
    # Copy standard C library and related files
    for lib_dir in "/lib64" "/usr/lib64"; do
        if [ -d "$lib_dir" ]; then
            echo "Copying libraries from $lib_dir..."
            mkdir -p "isolated$lib_dir"
            
            # Copy all library files including the dynamic linker
            for lib_file in "$lib_dir"/*; do
                if [ -f "$lib_file" ]; then
                    if ! cp "$lib_file" "isolated$lib_dir/" 2>/dev/null; then echo "  ⚠ Failed to copy "$lib_file" "isolated$lib_dir/" (non-critical)"; fi
                fi
            done
        fi
    done
    
    # Copy essential system configuration directories
    echo "Copying system configuration..."
    if [ -d "/etc" ]; then
        echo "Copying /etc directory..."
        # Copy essential files and directories from /etc
        for item in "resolv.conf" "hosts" "nsswitch.conf" "ssl" "pki" "ca-certificates" "passwd" "group"; do
            if [ -e "/etc/$item" ]; then
                if ! cp -r "/etc/$item" "isolated/etc/" 2>/dev/null; then echo "  ⚠ Failed to copy -r "/etc/$item" "isolated/etc/" (non-critical)"; fi
            fi
        done
    fi
    
    # Copy usr/share directory
    if [ -d "/usr/share" ]; then
        echo "Copying essential /usr/share content..."
        for item in "ca-certificates" "zoneinfo"; do
            if [ -d "/usr/share/$item" ]; then
                mkdir -p "isolated/usr/share"
                if ! cp -r "/usr/share/$item" "isolated/usr/share/" 2>/dev/null; then echo "  ⚠ Failed to copy -r "/usr/share/$item" "isolated/usr/share/" (non-critical)"; fi
            fi
        done
    fi
    
    echo "✓ System files copied"
}

# Generate runtime.yml configuration
generate_runtime_config() {
    echo ""
    echo "⚙️  Generating runtime configuration..."
    
    cat > "$RUNTIME_YML" << 'EOF'
name: graalvmjdk-21
version: "21.0.1"
description: "GraalVM Community Edition JDK 21 Runtime (RHEL/CentOS)"

# All mounts from isolated/ - no host dependencies per design
mounts:
  # Essential system binaries
  - source: "isolated/usr/local/bin"
    target: "/usr/local/bin"
    readonly: true
  - source: "isolated/usr/bin"
    target: "/usr/bin"
    readonly: true
  - source: "isolated/bin"
    target: "/bin"
    readonly: true
  - source: "isolated/sbin"
    target: "/sbin"
    readonly: true
  - source: "isolated/usr/sbin"
    target: "/usr/sbin"
    readonly: true
  # Essential libraries (RHEL uses lib64)
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
  # Essential system configuration
  - source: "isolated/etc/ssl"
    target: "/etc/ssl"
    readonly: true
  - source: "isolated/etc/pki"
    target: "/etc/pki"
    readonly: true
  - source: "isolated/etc/ca-certificates"
    target: "/etc/ca-certificates"
    readonly: true
  - source: "isolated/usr/share/ca-certificates"
    target: "/usr/share/ca-certificates"
    readonly: true
  # GraalVM-specific mounts
  - source: "isolated/usr/lib/jvm"
    target: "/usr/lib/jvm"
    readonly: true
  - source: "isolated/usr/share/java"
    target: "/usr/share/java"
    readonly: true
  # Create isolated /tmp directory
  - source: "isolated/tmp"
    target: "/tmp"
    readonly: false  # GraalVM needs write access to temp

environment:
  JAVA_HOME: "/usr/lib/jvm/graalvm-21"
  PATH: "/usr/lib/jvm/graalvm-21/bin:/usr/bin:/bin"
  GRAALVM_HOME: "/usr/lib/jvm/graalvm-21"
  LD_LIBRARY_PATH: "/usr/lib64:/lib64:/usr/lib/jvm/graalvm-21/lib"
EOF

    echo "✓ Runtime configuration generated: $RUNTIME_YML"
}

# Verify installation
verify_installation() {
    echo ""
    echo "🔍 Verifying installation..."
    
    # Check if GraalVM is properly installed
    if [ -x "isolated/usr/bin/java" ]; then
        echo "✓ Java executable found"
        
        # Test Java version (with chroot to test isolated environment)
        if command -v chroot >/dev/null 2>&1; then
            echo "Testing Java version in isolated environment..."
            chroot isolated /usr/bin/java -version || echo "Warning: Could not test in chroot"
        fi
    else
        echo "❌ Java executable not found"
        exit 1
    fi
    
    # Check if native-image is available
    if [ -x "isolated/usr/bin/native-image" ]; then
        echo "✓ native-image executable found"
    else
        echo "❌ native-image executable not found"
        exit 1
    fi
    
    # Check directory structure
    if [ -d "isolated/usr/lib/jvm/graalvm-21" ]; then
        echo "✓ GraalVM installation directory found"
    else
        echo "❌ GraalVM installation directory not found"
        exit 1
    fi
    
    echo "✓ Installation verification complete"
}

# =============================================================================
# MAIN INSTALLATION FLOW  
# =============================================================================

main() {
    echo ""
    echo "🚀 Starting GraalVM JDK 21 Runtime Installation (RHEL/CentOS)"
    echo "============================================================="
    
    # Step 1: Setup directory structure
    setup_runtime_directories "$RUNTIME_NAME"
    
    # Step 2: Install system dependencies
    install_dependencies
    
    # Step 3: Install GraalVM
    install_graalvm
    
    # Step 4: Copy system files
    copy_system_files
    
    # Step 5: Generate runtime configuration
    generate_runtime_config
    
    # Step 6: Verify installation
    verify_installation
    
    echo ""
    echo "🎉 GraalVM JDK 21 Runtime Installation Complete! (RHEL/CentOS)"
    echo "=============================================================="
    echo "Runtime Name: $RUNTIME_NAME"
    echo "Runtime Base: $RUNTIME_BASE_DIR"
    echo "Configuration: $RUNTIME_YML"
    echo ""
    echo "The runtime is now available for use with joblet."
    echo "You can test it with: java -version"
    echo "GraalVM native-image is available for AOT compilation."
}

# Execute main function
main "$@"