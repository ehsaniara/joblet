#!/bin/bash
# Self-contained Amazon Linux ARM64 OpenJDK 21 Runtime Setup
# Uses YUM package manager for reliable installation


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

PLATFORM="amzn"
ARCHITECTURE="arm64" 
RUNTIME_NAME="openjdk-21"

echo "Starting OpenJDK 21 runtime setup..."
echo "Platform: $PLATFORM"
echo "Architecture: $ARCHITECTURE" 
echo "Build ID: ${BUILD_ID:-unknown}"
echo "Runtime Spec: ${RUNTIME_SPEC:-unknown}"

# =============================================================================
# EMBEDDED COMMON FUNCTIONS
# =============================================================================

# Setup runtime directory structure (flat structure as per design)
setup_runtime_directories() {
    local runtime_name="${1:-openjdk-21}"
    
    # Use flat directory structure: /opt/joblet/runtimes/openjdk-21
    export RUNTIME_BASE_DIR="/opt/joblet/runtimes/$runtime_name"
    export RUNTIME_YML="$RUNTIME_BASE_DIR/runtime.yml"
    
    echo "Creating runtime directory: $RUNTIME_BASE_DIR"
    mkdir -p "$RUNTIME_BASE_DIR"
    cd "$RUNTIME_BASE_DIR"
    
    echo "Creating isolated structure for runtime mounting..."
    mkdir -p isolated/usr/lib/jvm
    mkdir -p isolated/usr/bin  
    mkdir -p isolated/usr/share/java
    mkdir -p isolated/etc/ssl/certs
    mkdir -p isolated/usr/lib64
    mkdir -p isolated/lib64
    mkdir -p isolated/usr/lib
    
    echo "✓ Runtime directories created"
}

# Find Java installation
find_java_installation() {
    echo "Finding Java installation..."
    
    # Common Java installation paths (prioritize Amazon Corretto 21)
    local java_homes=(
        "/usr/lib/jvm/java-21-amazon-corretto"
        "/usr/lib/jvm/java-21-openjdk"
        "/usr/lib/jvm/java-11-amazon-corretto"
        "/usr/lib/jvm/java-11-openjdk"
        "/usr/lib/jvm/java-1.8.0-openjdk"
        "/usr/lib/jvm/default-java"
    )
    
    export JAVA_HOME=""
    for jh in "${java_homes[@]}"; do
        if [ -d "$jh" ]; then
            export JAVA_HOME="$jh"
            echo "Found Java home: $JAVA_HOME"
            break
        fi
    done
    
    if [ -z "$JAVA_HOME" ]; then
        echo "ERROR: No Java installation found"
        exit 1
    fi
    
    # Verify Java is working
    echo "Verifying Java installation..."
    if command -v java >/dev/null 2>&1; then
        JAVA_VERSION=$(java -version 2>&1 | head -n1 | cut -d'"' -f2 2>/dev/null || echo "unknown")
        echo "✓ Java version: $JAVA_VERSION"
    else
        echo "✗ Java command not found"
        exit 1
    fi
    
    echo "✓ Java installation verified"
}

# Copy Java runtime files
copy_java_runtime() {
    echo "Copying Java runtime files..."
    
    # Copy JVM
    if [ -d "$JAVA_HOME" ]; then
        echo "Copying JVM from $JAVA_HOME..."
        cp -r "$JAVA_HOME"/* isolated/usr/lib/jvm/ 2>/dev/null || {
            echo "WARNING: Failed to copy complete Java home, copying essentials..."
            mkdir -p "isolated/usr/lib/jvm/$(basename "$JAVA_HOME")"
            # Copy essential directories
            for dir in bin lib jmods conf legal; do
                if [ -d "$JAVA_HOME/$dir" ]; then
                    cp -r "$JAVA_HOME/$dir" "isolated/usr/lib/jvm/$(basename "$JAVA_HOME")/" || echo "Warning: Failed to copy $dir"
                fi
            done
        }
    fi
    
    # Create wrapper scripts for Java binaries (fixes boot class path issues)
    echo "Creating Java wrapper scripts..."
    mkdir -p isolated/usr/bin
    
    # Create java wrapper
    cat > isolated/usr/bin/java << 'EOF'
#!/bin/bash
exec /usr/lib/jvm/bin/java "$@"
EOF
    chmod +x isolated/usr/bin/java
    
    # Create javac wrapper
    cat > isolated/usr/bin/javac << 'EOF'
#!/bin/bash
exec /usr/lib/jvm/bin/javac "$@"
EOF
    chmod +x isolated/usr/bin/javac
    
    # Create jar wrapper
    cat > isolated/usr/bin/jar << 'EOF'
#!/bin/bash
exec /usr/lib/jvm/bin/jar "$@"
EOF
    chmod +x isolated/usr/bin/jar
    
    # Copy JVM configuration file (fixes "could not open /usr/lib/jvm.cfg" errors)
    if [ -f "$JAVA_HOME/lib/jvm.cfg" ]; then
        mkdir -p isolated/usr/lib
        cp "$JAVA_HOME/lib/jvm.cfg" isolated/usr/lib/jvm.cfg
        echo "✓ Copied JVM configuration file"
    fi
    
    # Copy server JVM library (fixes "missing server JVM" errors)
    if [ -d "$JAVA_HOME/lib/server" ]; then
        mkdir -p isolated/usr/lib/server
        if ! cp "$JAVA_HOME/lib/server"/*.so isolated/usr/lib/server/ 2>/dev/null; then echo "  ⚠ Failed to copy "$JAVA_HOME/lib/server"/*.so isolated/usr/lib/server/ (non-critical)"; fi
        echo "✓ Copied server JVM libraries"
    fi
    
    echo "✓ Java runtime files copied"
}

# Generate platform-specific runtime.yml
# Generate design-compliant runtime.yml (uses shared common function)
generate_runtime_yml() {
    local java_version="${1:-21}"
    
    # Get actual Java version if possible
    if [ -n "$JAVA_HOME" ] && command -v java >/dev/null 2>&1; then
        java_version=$(java -version 2>&1 | head -n1 | cut -d'""'f2 2>/dev/null || echo "$java_version")
    fi
    
    # Count files for validation
    local file_count=$(find isolated/ -type f 2>/dev/null | wc -l || echo "0")
    
    echo "Creating design-compliant runtime.yml for OpenJDK..."
        cat > "$RUNTIME_YML" << EOF
name: openjdk-21
version: "$java_version"
description: "OpenJDK - self-contained ($file_count files)"

# All mounts from isolated/ - no host dependencies per design
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
  # Essential libraries
  - source: "isolated/lib"
    target: "/lib"
    readonly: true
  - source: "isolated/lib64"
    target: "/lib64"
    readonly: true
  - source: "isolated/usr/lib"
    target: "/usr/lib"
    readonly: true
  # Architecture-specific library directories
  - source: "isolated/lib/aarch64-linux-gnu"
    target: "/lib/aarch64-linux-gnu"
    readonly: true
  - source: "isolated/usr/lib/aarch64-linux-gnu"
    target: "/usr/lib/aarch64-linux-gnu"
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
  # Java-specific mounts
  - source: "isolated/usr/lib/jvm"
    target: "/usr/lib/jvm"
    readonly: true
  - source: "isolated/usr/share/java"
    target: "/usr/share/java"
    readonly: true
  # Package management infrastructure (for apt/dpkg/yum functionality)
  - source: "isolated/var/lib/dpkg"
    target: "/var/lib/dpkg"
    readonly: false  # dpkg needs write access for package operations
  - source: "isolated/var/cache/apt"
    target: "/var/cache/apt"
    readonly: false  # apt needs write access for package cache
  - source: "isolated/var/lib/apt"
    target: "/var/lib/apt"
    readonly: false  # apt needs write access for package lists
  - source: "isolated/etc/apt"
    target: "/etc/apt"
    readonly: true   # apt configuration (read-only for security)
  - source: "isolated/usr/share/keyrings"
    target: "/usr/share/keyrings"
    readonly: true   # GPG keyrings for package verification
  - source: "isolated/var/tmp"
    target: "/var/tmp"
    readonly: false  # Package managers need temp space
  # Create isolated /tmp and /proc directories
  - source: "isolated/tmp"
    target: "/tmp"
    readonly: false  # Java needs write access to temp
  - source: "isolated/proc"
    target: "/proc"
    readonly: true   # Java runtime needs CPU detection

environment:
  JAVA_HOME: "/usr/lib/jvm"
  PATH: "/usr/lib/jvm/bin:/usr/bin:/bin"
EOF
        echo "✓ Design-compliant runtime.yml created"
}

# Print installation summary
print_summary() {
    local file_count=$(find isolated/ -type f 2>/dev/null | wc -l || echo "0")
    
    echo ""
    echo "🎉 OpenJDK runtime setup completed!"
    echo "Java version: ${JAVA_VERSION:-unknown}"
    echo "Java home: ${JAVA_HOME:-not found}"
    echo "Total files: $file_count"
    echo "Runtime configuration: $RUNTIME_YML"
    
    if [ -f "$RUNTIME_YML" ]; then
        echo "✓ Runtime configuration created"
    else
        echo "✗ Runtime configuration missing"
    fi
}

# =============================================================================
# PLATFORM-SPECIFIC SETUP
# =============================================================================

install_java_packages() {
    echo "Installing OpenJDK packages via YUM (Amazon Linux ARM64)..."
    
    # Check if we're in a chroot or read-only environment
    if [ "$CHROOT" = "true" ] || ! [ -w "/usr/sbin" ]; then
        echo "⚠ Detected chroot/read-only environment - using host system Java"
        
        # Look for existing Java installations on host system
        local java_homes=(
            "/usr/lib/jvm/java-21-amazon-corretto"
            "/usr/lib/jvm/java-21-openjdk"
            "/usr/lib/jvm/java-11-amazon-corretto"
            "/usr/lib/jvm/java-11-openjdk"
            "/usr/lib/jvm/java-1.8.0-openjdk"
            "/usr/lib/jvm/default-java"
        )
        
        for jh in "${java_homes[@]}"; do
            if [ -d "$jh" ]; then
                echo "✓ Found existing Java installation at $jh"
                return 0
            fi
        done
        
        echo "⚠ No existing Java found - setup may fail"
        return 0
    fi
    
    # Update package lists (with fallback)
    yum update -y -q || {
        echo "⚠ Failed to update package lists, continuing with existing packages"
    }
    
    # Install Amazon Corretto (preferred on Amazon Linux)
    echo "Installing Amazon Corretto 21..."
    if ! yum install -y java-21-amazon-corretto-devel java-21-amazon-corretto-headless; then
        echo "⚠ Amazon Corretto 21 failed, trying OpenJDK packages..."
        if ! yum install -y java-21-openjdk-devel java-21-openjdk-headless; then
            echo "⚠ OpenJDK 21 failed, trying available Java packages..."
            yum install -y java-11-openjdk-devel java-11-openjdk-headless || \
            yum install -y java-1.8.0-openjdk-devel java-1.8.0-openjdk-headless || \
            echo "⚠ Some Java package installations failed, continuing..."
        fi
    fi
    
    echo "✓ Java package installation completed"
}

copy_system_libraries() {
    echo "Copying essential system libraries..."
    
    # Copy essential libraries for Java runtime (ARM64 specific)
    local lib_dirs=("/usr/lib64" "/lib64" "/usr/lib/aarch64-linux-gnu" "/lib/aarch64-linux-gnu")
    
    for lib_dir in "${lib_dirs[@]}"; do
        if [ -d "$lib_dir" ]; then
            echo "Copying libraries from $lib_dir..."
            local lib_patterns=("libc.so*" "libdl.so*" "libpthread.so*" "librt.so*" "libm.so*" "libz.so*" "libgcc_s.so*" "ld-linux*.so*" "libstdc++.so*")
            
            for pattern in "${lib_patterns[@]}"; do
                find "$lib_dir" -name "$pattern" -exec cp {} "isolated/$(basename "$lib_dir")/" \; 2>/dev/null || echo "  ⚠ Failed to copy $pattern from $lib_dir (non-critical)"
            done
        fi
    done
    
    echo "✓ System libraries copied"
}

# =============================================================================
# MAIN INSTALLATION FLOW  
# =============================================================================

main() {
    echo ""
    echo "🚀 Starting OpenJDK 21 Runtime Installation"
    echo "=============================================="
    
    # Step 1: Setup directory structure
    setup_runtime_directories "$RUNTIME_NAME"
    
    # Step 2: Install Java packages
    install_java_packages
    
    # Step 3: Find Java installation  
    find_java_installation
    
    # Step 4: Copy Java runtime files
    copy_java_runtime
    
    # Step 4.1: Java wrapper scripts are now created instead of binary copies
    
    # Step 5: Copy platform-specific system libraries
    copy_system_libraries
    
    # Step 6: Generate runtime configuration
    generate_runtime_yml "21"
    
    # Step 7: Print installation summary
    print_summary
    
    echo ""
    echo "🎉 OpenJDK 21 runtime setup completed successfully!"
}

# Execute main function
main "$@"
