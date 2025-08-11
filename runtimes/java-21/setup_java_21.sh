#!/bin/bash

# Multi-Architecture OpenJDK 21 LTS Runtime Setup Script
# Supports: amd64, arm64 on Ubuntu, Debian, CentOS, RHEL, Fedora, openSUSE, Arch, Alpine
# ⚠️  WARNING: This script installs wget/curl on the host system if missing
# ⚠️  See /opt/joblet/runtimes/CONTAMINATION_WARNING.md for details
# Note: Maven removed as it requires system utilities not available in isolated environment

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RUNTIME_BASE_DIR="/opt/joblet/runtimes"
RUNTIME_DIR="$RUNTIME_BASE_DIR/java/java-21"

# Load system detection library
if [[ -f "$SCRIPT_DIR/../common/detect_system.sh" ]]; then
    source "$SCRIPT_DIR/../common/detect_system.sh"
elif [[ -f "/tmp/common/detect_system.sh" ]]; then
    source "/tmp/common/detect_system.sh"
else
    echo "❌ System detection library not found"
    exit 1
fi

echo "☕ Setting up Multi-Architecture OpenJDK 21 LTS Runtime"
echo "======================================================="
echo "Target: $RUNTIME_DIR"
echo "⚠️  WARNING: This script may install download tools on the host system"
echo "⚠️  Impact: Minimal (~5-10MB) but still modifies host packages"
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
    if [[ -f "$RUNTIME_DIR/runtime.yml" && -d "$RUNTIME_DIR/jdk" ]]; then
        echo "✅ OpenJDK 21 runtime already installed"
        echo "   Location: $RUNTIME_DIR"
        echo "   To reinstall, remove the directory first:"
        echo "   sudo rm -rf '$RUNTIME_DIR'"
        exit 0
    fi
    
    # Get architecture-specific Java download URL
    echo "🔍 Getting Java 21 download URL for $DETECTED_ARCH..."
    JAVA_URL=$(get_java_download_url 21)
    if [[ $? -ne 0 ]]; then
        echo "$JAVA_URL"  # Error message
        show_supported_platforms
        exit 1
    fi
    
    echo "📦 Java 21 URL: $JAVA_URL"
    
    # Download and install OpenJDK 21
    echo "⬇️  Downloading OpenJDK 21 for $DETECTED_ARCH..."
    download_file "$JAVA_URL" "openjdk-21.tar.gz"
    
    echo "📦 Extracting OpenJDK 21..."
    mkdir -p jdk
    tar -xzf openjdk-21.tar.gz -C jdk --strip-components=1
    rm openjdk-21.tar.gz
    
    # Verify Java installation
    echo "🔍 Verifying isolated Java installation..."
    JAVA_BIN="$RUNTIME_DIR/jdk/bin/java"
    JAVAC_BIN="$RUNTIME_DIR/jdk/bin/javac"
    
    if [[ ! -f "$JAVA_BIN" ]]; then
        echo "❌ OpenJDK installation failed!"
        exit 1
    fi
    
    INSTALLED_VERSION=$($JAVA_BIN -version 2>&1 | head -n 1)
    echo "✅ Isolated OpenJDK installed: $INSTALLED_VERSION"
    
    # Create bin directory with symlinks for runtime mount structure
    echo "🔗 Creating bin directory with symlinks for proper runtime mounting..."
    mkdir -p bin
    ln -s ../jdk/bin/java bin/java
    ln -s ../jdk/bin/javac bin/javac  
    ln -s ../jdk/bin/jar bin/jar
    ln -s ../jdk/bin/jshell bin/jshell
    
    # Set correct ownership
    echo "👤 Setting ownership to joblet user..."
    chown -R joblet:joblet "$RUNTIME_DIR" 2>/dev/null || echo "⚠️  joblet user not found, keeping root ownership"
    
    # Create runtime manifest with system information
    cat > "$RUNTIME_DIR/runtime.yml" << EOF
name: "java-21"
version: "21.0.4"
description: "OpenJDK 21 LTS Runtime"
type: "java"
created: "$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
system:
  architecture: "$DETECTED_ARCH"
  os: "$DETECTED_OS"
  distribution: "$DETECTED_DISTRO"
  package_manager: "$DETECTED_PACKAGE_MANAGER"
paths:
  java_home: "$RUNTIME_DIR/jdk"
binaries:
  java: "$RUNTIME_DIR/jdk/bin/java"
  javac: "$RUNTIME_DIR/jdk/bin/javac"
  jar: "$RUNTIME_DIR/jdk/bin/jar"
  jshell: "$RUNTIME_DIR/jdk/bin/jshell"
mounts:
  - source: "jdk/lib"
    target: "/usr/local/lib"
    readonly: true
  - source: "jdk/conf"
    target: "/usr/local/conf"
    readonly: true
  - source: "jdk/bin"
    target: "/usr/local/jdk-bin"
    readonly: true
  - source: "bin"
    target: "/usr/local/bin"
    readonly: true
  - source: "jdk"
    target: "$RUNTIME_DIR/jdk"
    readonly: true
environment:
  JAVA_HOME: "$RUNTIME_DIR/jdk"
  PATH: "$RUNTIME_DIR/jdk/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
features:
  - "Virtual Threads (Java 21)"
  - "Pattern Matching for switch (Java 21)"
  - "Record Patterns (Java 21)"
  - "String Templates (Java 21 Preview)"
  - "Sequenced Collections (Java 21)"
  - "JShell Interactive REPL"
EOF
    
    # Display success information
    echo ""
    echo "🎉 OpenJDK 21 Runtime Installation Complete!"
    echo "=============================================="
    echo "✅ Architecture: $DETECTED_ARCH"
    echo "✅ Distribution: $DETECTED_DISTRO"
    echo "✅ Java Version: $(echo "$INSTALLED_VERSION" | cut -d' ' -f1-3)"
    echo "✅ Installation Path: $RUNTIME_DIR"
    echo "✅ Runtime Manifest: $RUNTIME_DIR/runtime.yml"
    echo ""
    echo "🧪 Test the runtime:"
    echo "   rnx runtime list"
    echo "   rnx runtime info java:21"
    echo "   rnx run --runtime=java:21 java -version"
    echo ""
    echo "📚 Example usage:"
    echo "   cd /opt/joblet/examples/java-21"
    echo "   rnx run --template=jobs.yaml:hello-joblet"
    echo ""
}

# Show usage if requested
if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
    echo "Multi-Architecture OpenJDK 21 LTS Runtime Setup"
    echo ""
    echo "Usage: sudo $0"
    echo ""
    show_supported_platforms
    exit 0
fi

# Run main function
main "$@"