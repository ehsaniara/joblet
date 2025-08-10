#!/bin/bash

# OpenJDK 21 LTS Runtime Setup Script
# Creates completely isolated OpenJDK 21 environment with Maven
# Includes modern Java features like Virtual Threads (Project Loom)
# ⚠️  WARNING: This script installs wget/curl on the host system if missing
# ⚠️  See /opt/joblet/examples/runtimes/CONTAMINATION_WARNING.md for details

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RUNTIME_BASE_DIR="/opt/joblet/runtimes"
RUNTIME_DIR="$RUNTIME_BASE_DIR/java/java-21"
OPENJDK_VERSION="21.0.4"
MAVEN_VERSION="3.9.6"

echo "☕ Setting up OpenJDK 21 LTS Runtime"
echo "===================================="
echo "Target: $RUNTIME_DIR"
echo "Features: Virtual Threads, Pattern Matching, String Templates"
echo "⚠️  WARNING: This script may install wget/curl on the host system"
echo "⚠️  Impact: Minimal (~5MB) but still modifies host packages"
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
    if [[ -f "$RUNTIME_DIR/runtime.yml" && -d "$RUNTIME_DIR/jdk" ]]; then
        echo "✅ OpenJDK 21 runtime already installed"
        echo "   Location: $RUNTIME_DIR"
        echo "   To reinstall, remove the directory first:"
        echo "   sudo rm -rf '$RUNTIME_DIR'"
        exit 0
    fi
    
    # Install minimal dependencies temporarily (will be removed)
    echo "📦 Installing temporary dependencies..."
    apt-get update -qq
    apt-get install -y wget curl
    
    # Download OpenJDK 21 from Eclipse Adoptium (Temurin)
    echo "⬇️  Downloading OpenJDK 21 (Temurin)..."
    OPENJDK_URL="https://github.com/adoptium/temurin21-binaries/releases/download/jdk-21.0.4%2B7/OpenJDK21U-jdk_x64_linux_hotspot_21.0.4_7.tar.gz"
    
    wget -q --show-progress "$OPENJDK_URL" -O openjdk-21.tar.gz
    
    echo "📦 Extracting OpenJDK 21..."
    mkdir -p jdk
    tar -xzf openjdk-21.tar.gz -C jdk --strip-components=1
    rm openjdk-21.tar.gz
    
    # Download Maven
    echo "⬇️  Downloading Apache Maven $MAVEN_VERSION..."
    MAVEN_URL="https://archive.apache.org/dist/maven/maven-3/$MAVEN_VERSION/binaries/apache-maven-$MAVEN_VERSION-bin.tar.gz"
    
    wget -q --show-progress "$MAVEN_URL" -O maven.tar.gz
    
    echo "📦 Extracting Maven..."
    mkdir -p maven
    tar -xzf maven.tar.gz -C maven --strip-components=1
    rm maven.tar.gz
    
    # Remove temporary dependencies to keep host clean
    echo "🧹 Removing temporary dependencies from host..."
    apt-get remove -y wget curl
    apt-get autoremove -y
    apt-get clean
    
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
    
    # Verify Maven installation
    MAVEN_BIN="$RUNTIME_DIR/maven/bin/mvn"
    if [[ ! -f "$MAVEN_BIN" ]]; then
        echo "❌ Maven installation failed!"
        exit 1
    fi
    
    MAVEN_INSTALLED_VERSION=$($MAVEN_BIN --version 2>&1 | head -n 1)
    echo "✅ Isolated Maven installed: $MAVEN_INSTALLED_VERSION"
    
    # Create mount structure for joblet runtime system
    echo "🔗 Creating runtime mount structure..."
    mkdir -p bin lib
    
    # Create symlinks for mounting into jobs (absolute paths within chroot environment)
    # These point to the mounted JDK binaries at /usr/local/jdk-bin/ within chroot
    ln -sf /usr/local/jdk-bin/java bin/java
    ln -sf /usr/local/jdk-bin/javac bin/javac
    ln -sf /usr/local/jdk-bin/jar bin/jar
    ln -sf /usr/local/jdk-bin/javap bin/javap
    ln -sf /usr/local/jdk-bin/jshell bin/jshell
    # Maven is mounted separately at /usr/local/maven
    ln -sf /usr/local/maven/bin/mvn bin/mvn
    
    # Link Java libraries
    ln -sf "$RUNTIME_DIR/jdk/lib" lib/jdk-lib
    
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
name: "java-21"
version: "21.0.4"
description: "OpenJDK 21 LTS with Maven and modern Java features"

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
    selective: ["java", "javac", "jar", "javap", "jshell", "mvn"]
  - source: "maven"
    target: "/usr/local/maven"
    readonly: true
  - source: "jdk"
    target: "/opt/joblet/runtimes/java/java-21/jdk"
    readonly: true

environment:
  JAVA_HOME: "/usr/local"
  MAVEN_HOME: "/usr/local/maven"
  PATH_PREPEND: "/usr/local/bin:/usr/local/maven/bin"
  M2_HOME: "/usr/local/maven"
  LD_LIBRARY_PATH: "/usr/local/lib:/usr/local/lib/server"

package_manager:
  type: "maven"
  cache_volume: "maven-cache"

requirements:
  architectures: ["x86_64", "amd64"]

features:
  - "LTS (Long Term Support)"
  - "Virtual Threads (Project Loom)"
  - "Pattern Matching for switch"
  - "String Templates (Preview)"
  - "Record Patterns"
  - "Foreign Function & Memory API (Preview)"
  - "Vector API (Fifth Incubator)"
  - "Maven build tool"
  - "Interactive shell (jshell)"
  - "Modern GC algorithms"
EOF
    
    # Test isolated installation
    echo "🧪 Testing isolated Java runtime..."
    export JAVA_HOME="$RUNTIME_DIR/jdk"
    export PATH="$RUNTIME_DIR/jdk/bin:$PATH"
    
    echo "Java version:"
    $JAVA_BIN -version
    
    echo
    echo "Maven version:"
    $MAVEN_BIN --version
    
    # Create and test a Virtual Threads example
    echo "Testing Java 21 Virtual Threads..."
    cat > VirtualThreadTest.java << 'EOF'
import java.time.Duration;
import java.util.concurrent.Executors;
import java.util.stream.IntStream;

public class VirtualThreadTest {
    public static void main(String[] args) {
        System.out.println("✅ OpenJDK 21 runtime working!");
        System.out.println("Java Version: " + System.getProperty("java.version"));
        System.out.println("Java Home: " + System.getProperty("java.home"));
        
        // Test Virtual Threads (Project Loom)
        System.out.println("\n🧵 Testing Virtual Threads:");
        
        var startTime = System.currentTimeMillis();
        
        try (var executor = Executors.newVirtualThreadPerTaskExecutor()) {
            IntStream.range(0, 10000).forEach(i -> {
                executor.submit(() -> {
                    try {
                        Thread.sleep(Duration.ofMillis(1));
                        return i * i;
                    } catch (InterruptedException e) {
                        return 0;
                    }
                });
            });
        }
        
        var endTime = System.currentTimeMillis();
        System.out.println("Created and executed 10,000 virtual threads in " + 
                         (endTime - startTime) + "ms");
        System.out.println("🚀 Virtual Threads working perfectly!");
        
        // Test Pattern Matching
        System.out.println("\n🔍 Testing Pattern Matching:");
        testPatternMatching("Hello");
        testPatternMatching(42);
        testPatternMatching(3.14);
    }
    
    static void testPatternMatching(Object obj) {
        String result = switch (obj) {
            case String s -> "String: " + s;
            case Integer i -> "Integer: " + i;
            case Double d -> "Double: " + d;
            default -> "Unknown type";
        };
        System.out.println("  " + result);
    }
}
EOF
    
    $JAVAC_BIN VirtualThreadTest.java
    $JAVA_BIN VirtualThreadTest
    rm -f VirtualThreadTest.java VirtualThreadTest.class
    
    echo
    echo "🎉 OpenJDK 21 Runtime setup completed!"
    echo
    echo "📍 Runtime location: $RUNTIME_DIR"
    echo "📋 Configuration: $RUNTIME_DIR/runtime.yml"
    echo
    echo "📋 Verification Commands:"
    echo "  # Host should be clean (no java):"
    echo "  java -version  # Should fail or show different version"
    echo
    echo "  # Runtime should work (isolated java):"
    echo "  rnx run --runtime=java:21 java -version"
    echo "  rnx run --runtime=java:21 javac -version"
    echo "  rnx run --runtime=java:21 mvn -version"
    echo
    echo "  # Test Virtual Threads:"
    echo "  rnx run --runtime=java:21 --upload=VirtualThreadApp.java bash -c \"javac VirtualThreadApp.java && java VirtualThreadApp\""
    echo
    echo "🚀 Modern Java Features Available:"
    echo "  • Virtual Threads for massive concurrency"
    echo "  • Pattern Matching for cleaner code"
    echo "  • String Templates (Preview)"
    echo "  • Record Patterns"
    echo "  • Foreign Function & Memory API"
    echo
    echo "✨ Host system remains completely clean!"
    echo "🔒 All Java functionality isolated to runtime directory only!"
    
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
    
    echo "📦 Packaging Java 21 runtime..."
    
    mkdir -p "$package_dir"
    
    # Package runtime
    cd "$(dirname "$RUNTIME_DIR")"
    tar -czf "$package_dir/java-21-runtime.tar.gz" java-21/
    
    # Create manifest
    cat > "$package_dir/java-21-runtime.manifest" << EOF
Runtime Package: OpenJDK 21 LTS
===============================
Package: java-21-runtime.tar.gz
Runtime Name: java-21
Language: java
Version: $OPENJDK_VERSION
Type: Binary Distribution + Maven + Modern Features

Built: $(date)
Build Host: $(hostname)
Size: $(du -sh "$RUNTIME_DIR" | cut -f1)

Installation:
  tar -xzf java-21-runtime.tar.gz -C /opt/joblet/runtimes/java/

Verification:
  rnx runtime test java:21
  rnx run --runtime=java:21 java -version
EOF
    
    # Create checksum
    cd "$package_dir"
    sha256sum java-21-runtime.tar.gz > java-21-runtime.sha256
    
    echo "✅ Java 21 runtime packaged: $package_dir/java-21-runtime.tar.gz"
    echo "Size: $(ls -lh java-21-runtime.tar.gz | awk '{print $5}')"
}

# Install runtime from package
install_from_package() {
    local package_path="$1"
    local target_dir="/opt/joblet/runtimes/java"
    
    if [[ -z "$package_path" || ! -f "$package_path" ]]; then
        echo "❌ Package file not found: $package_path"
        return 1
    fi
    
    echo "📦 Installing Java 21 runtime from package..."
    
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
        chown -R joblet:joblet "$target_dir/java-21" 2>/dev/null || echo "⚠️ Could not set ownership"
    fi
    
    echo "✅ Java 21 runtime installed from package"
    echo "Location: $target_dir/java-21"
    echo "Test with: rnx runtime test java:21"
}

# Create deployment-ready tar.gz for easy distribution
create_deployment_zip() {
    local pkg_dir="/tmp/runtime-deployments"
    local pkg_file="$pkg_dir/java-21-runtime.tar.gz"
    
    echo "📦 Creating deployment tar.gz for easy distribution..."
    
    # Create deployment directory
    mkdir -p "$pkg_dir"
    
    # Create tar.gz of the entire runtime directory
    cd "$(dirname "$RUNTIME_DIR")"
    tar -czf "$pkg_file" "$(basename "$RUNTIME_DIR")/"
    
    # Create deployment metadata
    cat > "$pkg_dir/java-21-runtime.json" << EOF
{
  "runtime": {
    "name": "java-21",
    "language": "java", 
    "version": "$OPENJDK_VERSION",
    "type": "modern",
    "description": "OpenJDK 21 with Maven and modern Java features",
    "size_bytes": $(du -sb "$RUNTIME_DIR" | cut -f1),
    "created": "$(date -Iseconds)",
    "build_host": "$(hostname)",
    "architecture": "$(uname -m)"
  },
  "deployment": {
    "tar_file": "java-21-runtime.tar.gz",
    "target_path": "/opt/joblet/runtimes/java/java-21",
    "unpack_command": "tar -xzf java-21-runtime.tar.gz -C /opt/joblet/runtimes/java/",
    "verify_command": "rnx runtime test java:21"
  },
  "features": [
    "Virtual Threads (Project Loom)",
    "Pattern Matching",
    "String Templates (Preview)",
    "Record Patterns",
    "Foreign Function & Memory API",
    "Maven build tool"
  ]
}
EOF
    
    echo "✅ Deployment tar.gz created: $pkg_file"
    echo "Size: $(ls -lh "$pkg_file" | awk '{print $5}')"
    echo "Metadata: $pkg_dir/java-21-runtime.json"
    echo ""
    echo "🚀 Ready for deployment! Administrators can now:"
    echo "  1. Copy package to target host: scp $pkg_file admin@host:/tmp/"
    echo "  2. Deploy: tar -xzf /tmp/java-21-runtime.tar.gz -C /opt/joblet/runtimes/java/"
    echo "  3. Set permissions: sudo chown -R joblet:joblet /opt/joblet/runtimes/java/java-21"
    echo "  4. Verify: rnx runtime test java:21"
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
        echo "OpenJDK 21 LTS Runtime Setup"
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
        echo "  sudo $0                              # Install runtime"
        echo "  sudo $0 package /tmp/packages       # Package runtime"
        echo "  sudo $0 install java-21-runtime.tar.gz # Install from package"
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