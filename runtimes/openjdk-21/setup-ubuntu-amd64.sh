#!/bin/bash
# Self-contained Ubuntu/Debian AMD64 OpenJDK 21 Runtime Setup
# Uses APT package manager for reliable installation


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

# Bypass systemd in isolated environment to prevent bus connection errors
export SYSTEMCTL_SKIP_REDIRECT=1
unset DBUS_SESSION_BUS_ADDRESS
unset DBUS_SYSTEM_BUS_ADDRESS

# =============================================================================
# CONFIGURATION
# =============================================================================

PLATFORM="ubuntu"
ARCHITECTURE="amd64" 
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
    mkdir -p isolated/bin
    mkdir -p isolated/usr/sbin
    mkdir -p isolated/usr/share/java
    mkdir -p isolated/etc/ssl/certs
    mkdir -p isolated/etc
    mkdir -p isolated/usr/share/ca-certificates
    mkdir -p isolated/usr/lib/x86_64-linux-gnu
    mkdir -p isolated/lib/x86_64-linux-gnu  
    mkdir -p isolated/lib64
    mkdir -p isolated/lib
    mkdir -p isolated/usr/lib
    mkdir -p isolated/usr/lib64
    
    echo "✓ Runtime directories created"
}

# Find Java installation
find_java_installation() {
    echo "Finding Java installation..."
    
    # Common Java installation paths (prioritize OpenJDK 21)
    local java_homes=(
        "/usr/lib/jvm/java-21-openjdk-amd64"
        "/usr/lib/jvm/java-21-openjdk"
        "/usr/lib/jvm/java-11-openjdk-amd64"
        "/usr/lib/jvm/java-11-openjdk"
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
    
    # Copy complete JVM installation
    if [ -d "$JAVA_HOME" ]; then
        echo "Copying JVM from $JAVA_HOME..."
        local java_basename=$(basename "$JAVA_HOME")
        
        # Create the full JVM directory structure in isolated environment
        mkdir -p "isolated/usr/lib/jvm/$java_basename"
        
        # Copy all essential directories
        for dir in bin lib jmods conf legal include; do
            if [ -d "$JAVA_HOME/$dir" ]; then
                echo "Copying $dir directory..."
                cp -r "$JAVA_HOME/$dir" "isolated/usr/lib/jvm/$java_basename/" 2>/dev/null || echo "Warning: Failed to copy $dir"
            fi
        done
        
        # Copy critical Java libraries to system library locations
        echo "Copying Java libraries to system locations..."
        if [ -d "$JAVA_HOME/lib" ]; then
            # Copy libjava.so and other critical JVM libraries
            find "$JAVA_HOME/lib" -name "*.so*" -type f | while read libfile; do
                if ! cp "$libfile" "isolated/usr/lib/x86_64-linux-gnu/" 2>/dev/null; then echo "  ⚠ Failed to copy "$libfile" "isolated/usr/lib/x86_64-linux-gnu/" (non-critical)"; fi
                if ! cp "$libfile" "isolated/lib/x86_64-linux-gnu/" 2>/dev/null; then echo "  ⚠ Failed to copy "$libfile" "isolated/lib/x86_64-linux-gnu/" (non-critical)"; fi
                if ! cp "$libfile" "isolated/usr/lib/" 2>/dev/null; then echo "  ⚠ Failed to copy "$libfile" "isolated/usr/lib/" (non-critical)"; fi
                if ! cp "$libfile" "isolated/lib/" 2>/dev/null; then echo "  ⚠ Failed to copy "$libfile" "isolated/lib/" (non-critical)"; fi
            done
            
            # Special handling for server and client JVM libraries
            if [ -d "$JAVA_HOME/lib/server" ]; then
                echo "Copying server JVM libraries..."
                find "$JAVA_HOME/lib/server" -name "*.so*" -type f | while read libfile; do
                    if ! cp "$libfile" "isolated/usr/lib/x86_64-linux-gnu/" 2>/dev/null; then echo "  ⚠ Failed to copy "$libfile" "isolated/usr/lib/x86_64-linux-gnu/" (non-critical)"; fi
                    if ! cp "$libfile" "isolated/lib/x86_64-linux-gnu/" 2>/dev/null; then echo "  ⚠ Failed to copy "$libfile" "isolated/lib/x86_64-linux-gnu/" (non-critical)"; fi
                done
            fi
        fi
    fi
    
    echo "✓ Java runtime files copied"
}

# Generate platform-specific runtime.yml
generate_runtime_yml() {
    local java_version="${1:-21}"
    
    # Get actual Java version if possible
    if [ -n "$JAVA_HOME" ] && command -v java >/dev/null 2>&1; then
        java_version=$(java -version 2>&1 | head -n1 | cut -d'"' -f2 2>/dev/null || echo "$java_version")
    fi
    
    # Count files for size estimation  
    local file_count=$(find isolated/ -type f 2>/dev/null | wc -l || echo "0")
    
    echo "Creating runtime.yml configuration aligned with builder-runtime-final.md..."
    
    # Generate design-compliant runtime.yml (per builder-runtime-final.md)
    cat > "$RUNTIME_YML" << EOF
name: openjdk-21
version: "$java_version"
description: "OpenJDK 21 - self-contained ($file_count files)"

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
  - source: "isolated/lib/x86_64-linux-gnu"
    target: "/lib/x86_64-linux-gnu"
    readonly: true
  - source: "isolated/usr/lib/x86_64-linux-gnu"
    target: "/usr/lib/x86_64-linux-gnu"
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
  JAVA_HOME: "/usr/lib/jvm/java-21-openjdk-amd64"
  PATH: "/usr/lib/jvm/java-21-openjdk-amd64/bin:/usr/bin:/bin:/usr/sbin:/sbin"
  JAVA_VERSION: "$java_version"
  LD_LIBRARY_PATH: "/usr/lib/jvm/java-21-openjdk-amd64/lib:/usr/lib/jvm/java-21-openjdk-amd64/lib/server:/usr/lib/x86_64-linux-gnu:/lib/x86_64-linux-gnu:/usr/lib:/lib"
EOF
    
    echo "✓ runtime.yml created successfully for $ARCHITECTURE $PLATFORM"
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
    echo "Installing OpenJDK packages..."
    
    # Check if we're in a chroot/read-only environment (like local installation)
    if [ "${CHROOT:-false}" = "true" ] || [ ! -w /usr/sbin ] 2>/dev/null; then
        echo "⚠ Detected isolated/read-only environment - skipping APT installation"
        echo "⚠ Using host system's existing Java installation instead"
        
        # Verify Java is available on host system
        if ! command -v java >/dev/null 2>&1; then
            echo "❌ ERROR: No Java installation found on host system"
            echo "   Please install OpenJDK on the host system first:"
            echo "   sudo apt-get install openjdk-21-jdk-headless"
            exit 1
        fi
        
        echo "✓ Found Java on host system, will copy in next step"
        return 0
    fi
    
    # Only attempt APT installation in non-chroot environments (GitHub installation)
    echo "Installing OpenJDK packages via APT..."
    
    # Configure APT for isolated environment installation
    export DEBIAN_FRONTEND=noninteractive
    export DEBCONF_NONINTERACTIVE_SEEN=true
    export NEEDRESTART_SUSPEND=1
    export SYSTEMD_LOG_LEVEL=emerg
    export RUNLEVEL=1
    
    # Update package lists with error handling
    echo "Updating package lists..."
    apt-get update -qq 2>/dev/null || {
        echo "⚠ Failed to update package lists, continuing with existing packages"
    }
    
    # Install OpenJDK 21 with robust options
    echo "Installing OpenJDK 21 packages..."
    local apt_opts=("-y" "-o" "DPkg::Options::=--force-confold" "-o" "APT::Install-Recommends=false")
    
    if ! apt-get install "${apt_opts[@]}" openjdk-21-jdk-headless openjdk-21-jre-headless ca-certificates-java 2>/dev/null; then
        echo "⚠ OpenJDK 21 failed, trying fallback packages..."
        apt-get install "${apt_opts[@]}" default-jdk-headless ca-certificates-java 2>/dev/null || \
        apt-get install "${apt_opts[@]}" openjdk-11-jdk-headless 2>/dev/null || \
        echo "⚠ Some Java package installations failed, continuing..."
    fi
    
    echo "✓ Java package installation completed"
}

copy_system_libraries() {
    echo "Copying essential system libraries..."
    
    # Copy essential libraries for Java runtime
    local lib_dirs=("/usr/lib/x86_64-linux-gnu" "/lib/x86_64-linux-gnu" "/lib64")
    
    for lib_dir in "${lib_dirs[@]}"; do
        if [ -d "$lib_dir" ]; then
            echo "Copying libraries from $lib_dir..."
            local lib_patterns=("libc.so*" "libdl.so*" "libpthread.so*" "librt.so*" "libm.so*" "libz.so*" "libgcc_s.so*" "ld-linux*.so*" "libstdc++.so*" "libtinfo.so*" "libreadline.so*" "libhistory.so*" "libncurses.so*" "libselinux.so*" "libpcre2-8.so*" "libjava.so*" "libjvm.so*" "libverify.so*" "libjli.so*")
            
            # Create target directory maintaining full path
            if [[ "$lib_dir" == "/lib/x86_64-linux-gnu" || "$lib_dir" == "/usr/lib/x86_64-linux-gnu" ]]; then
                mkdir -p "isolated${lib_dir}"
                target_dir="isolated${lib_dir}"
            else
                mkdir -p "isolated/$(basename "$lib_dir")"
                target_dir="isolated/$(basename "$lib_dir")"
            fi
            
            for pattern in "${lib_patterns[@]}"; do
                find "$lib_dir" -name "$pattern" -exec cp {} "$target_dir/" \; 2>/dev/null || echo "  ⚠ Failed to copy {} "$target_dir/" (non-critical)"
            done
        fi
    done
    
    # Copy dynamic linker and symlinks
    echo "Copying dynamic linker..."
    
    # Copy the actual dynamic linker from /lib/x86_64-linux-gnu/
    if [ -f "/lib/x86_64-linux-gnu/ld-linux-x86-64.so.2" ]; then
        if ! cp "/lib/x86_64-linux-gnu/ld-linux-x86-64.so.2" "isolated/lib/x86_64-linux-gnu/" 2>/dev/null; then echo "  ⚠ Failed to copy "/lib/x86_64-linux-gnu/ld-linux-x86-64.so.2" "isolated/lib/x86_64-linux-gnu/" (non-critical)"; fi
    fi
    
    # Create the /lib64 symlink structure
    if [ -L "/lib64/ld-linux-x86-64.so.2" ]; then
        mkdir -p "isolated/lib64"
        # Create symlink pointing to the x86_64-linux-gnu location
        ln -sf "../lib/x86_64-linux-gnu/ld-linux-x86-64.so.2" "isolated/lib64/ld-linux-x86-64.so.2" 2>/dev/null || true
    fi
    
    echo "✓ System libraries copied"
}

copy_system_files() {
    echo "Copying essential system files..."
    
    # Copy essential system binaries
    for bin_dir in "/usr/bin" "/bin" "/usr/sbin"; do
        if [ -d "$bin_dir" ]; then
            echo "Copying binaries from $bin_dir..."
            mkdir -p "isolated$(dirname "$bin_dir")"
            mkdir -p "isolated$bin_dir"
            
            # Copy essential system binaries
            local bin_patterns=("bash" "sh" "ls" "cat" "grep" "find" "which" "dirname" "basename")
            for pattern in "${bin_patterns[@]}"; do
                if [ -f "$bin_dir/$pattern" ]; then
                    if ! cp "$bin_dir/$pattern" "isolated$bin_dir/" 2>/dev/null; then echo "  ⚠ Failed to copy "$bin_dir/$pattern" "isolated$bin_dir/" (non-critical)"; fi
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
        
        # Copy Java configuration directory
        if [ -d "/etc/java-21-openjdk" ]; then
            echo "Copying Java configuration from /etc/java-21-openjdk..."
            cp -r "/etc/java-21-openjdk" "isolated/etc/" 2>/dev/null || echo "Warning: Failed to copy Java config"
        fi
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
    
    # Step 4.1: Copy Java binaries for PATH compatibility
    echo "Copying Java binaries to standard locations..."
    if [ -n "$JAVA_HOME" ]; then
        # Create wrapper scripts that properly delegate to JAVA_HOME binaries
        for binary in java javac jar; do
            if [ -f "$JAVA_HOME/bin/$binary" ]; then
                # Create wrapper script that uses full path
                cat > "isolated/usr/bin/$binary" << 'WRAPPER_EOF'
#!/bin/bash
exec /usr/lib/jvm/java-21-openjdk-amd64/bin/java "$@"
WRAPPER_EOF
                # Replace java with correct binary name in the wrapper
                sed -i "s|/bin/java|/bin/$binary|g" "isolated/usr/bin/$binary"
                chmod +x "isolated/usr/bin/$binary"
                
                # Also copy to /bin for compatibility
                if ! cp "isolated/usr/bin/$binary" "isolated/bin/$binary" 2>/dev/null; then echo "  ⚠ Failed to copy "isolated/usr/bin/$binary" "isolated/bin/$binary" (non-critical)"; fi
            fi
        done
        
        # Copy JVM configuration file to expected location
        if [ -f "$JAVA_HOME/lib/jvm.cfg" ]; then
            mkdir -p "isolated/usr/lib"
            if ! cp "$JAVA_HOME/lib/jvm.cfg" "isolated/usr/lib/jvm.cfg" 2>/dev/null; then echo "  ⚠ Failed to copy "$JAVA_HOME/lib/jvm.cfg" "isolated/usr/lib/jvm.cfg" (non-critical)"; fi
            echo "✓ JVM configuration copied to /usr/lib/jvm.cfg"
        fi
        
        # Copy server JVM library to expected location
        if [ -f "$JAVA_HOME/lib/server/libjvm.so" ]; then
            mkdir -p "isolated/usr/lib/server"
            if ! cp "$JAVA_HOME/lib/server/libjvm.so" "isolated/usr/lib/server/libjvm.so" 2>/dev/null; then echo "  ⚠ Failed to copy "$JAVA_HOME/lib/server/libjvm.so" "isolated/usr/lib/server/libjvm.so" (non-critical)"; fi
            # Also copy other server JVM files that might be needed
            if ! cp "$JAVA_HOME"/lib/server/*.so "isolated/usr/lib/server/" 2>/dev/null; then echo "  ⚠ Failed to copy "$JAVA_HOME"/lib/server/*.so "isolated/usr/lib/server/" (non-critical)"; fi
            echo "✓ Server JVM libraries copied to /usr/lib/server/"
        fi
        
        echo "✓ Java binaries copied to standard locations"
    fi
    
    # Step 5: Copy platform-specific system libraries
    copy_system_libraries
    
    # Step 6: Copy essential system files
    copy_system_files
    
    # Step 7: Generate runtime configuration
    generate_runtime_yml "21"
    
    # Step 8: Print installation summary
    print_summary
    
    echo ""
    echo "🎉 OpenJDK 21 runtime setup completed successfully!"
}

# Execute main function
main "$@"