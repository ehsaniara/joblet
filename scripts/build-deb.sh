#!/bin/bash
set -e

ARCH=${1:-amd64}
VERSION=${2:-1.0.0}
PACKAGE_NAME="worker"
BUILD_DIR="worker-deb-${ARCH}"

# Clean up version string for Debian package format
CLEAN_VERSION=$(echo "$VERSION" | sed 's/^v//' | sed 's/-[0-9]\+-g[a-f0-9]\+.*//' | sed 's/-[a-f0-9]\+$//')

# Ensure version starts with a digit and is valid
if [[ ! "$CLEAN_VERSION" =~ ^[0-9] ]]; then
    CLEAN_VERSION="1.0.0"
    echo "⚠️  Invalid version format, using default: $CLEAN_VERSION"
else
    echo "📦 Using cleaned version: $CLEAN_VERSION (from $VERSION)"
fi

echo "🔨 Building Debian package for $PACKAGE_NAME v$CLEAN_VERSION ($ARCH)..."

# Clean and create build directory
rm -rf "$BUILD_DIR"
mkdir -p "$BUILD_DIR"

# Create directory structure
mkdir -p "$BUILD_DIR/DEBIAN"
mkdir -p "$BUILD_DIR/opt/worker"
mkdir -p "$BUILD_DIR/opt/worker/config"
mkdir -p "$BUILD_DIR/etc/systemd/system"
mkdir -p "$BUILD_DIR/usr/local/bin"
mkdir -p "$BUILD_DIR/usr/share/debconf/confmodule"  # For debconf support

# Copy binaries - BOTH to /opt/worker/
if [ ! -f "./worker" ]; then
    echo "❌ Worker binary not found!"
    exit 1
fi
cp ./worker "$BUILD_DIR/opt/worker/"

if [ ! -f "./worker-cli" ]; then
    echo "❌ Worker CLI binary not found!"
    exit 1
fi
cp ./worker-cli "$BUILD_DIR/opt/worker/"

# Copy config file (now as template)
if [ -f "./config/config.yml" ]; then
    cp ./config/config.yml "$BUILD_DIR/opt/worker/config/"
    echo "✅ Copied config/config.yml to /opt/worker/config/"
elif [ -f "./config.yaml" ]; then
    cp ./config.yaml "$BUILD_DIR/opt/worker/config/config.yml"
    echo "✅ Copied config.yaml to /opt/worker/config/config.yml"
elif [ -f "./config/config.yaml" ]; then
    cp ./config/config.yaml "$BUILD_DIR/opt/worker/config/config.yml"
    echo "✅ Copied config/config.yaml to /opt/worker/config/config.yml"
else
    echo "❌ No config file found!"
    echo "Looked for: ./config/config.yml, ./config.yaml, ./config/config.yaml"
    exit 1
fi

# Copy service file
cp ./scripts/worker.service "$BUILD_DIR/etc/systemd/system/"

# Copy certificate generation script (enhanced version)
cp ./scripts/certs_gen.sh "$BUILD_DIR/usr/local/bin/"

# Create control file with debconf dependency
cat > "$BUILD_DIR/DEBIAN/control" << EOF
Package: $PACKAGE_NAME
Version: $CLEAN_VERSION
Section: utils
Priority: optional
Architecture: $ARCH
Depends: openssl (>= 1.1.1), systemd, debconf (>= 0.5) | debconf-2.0
Maintainer: Jay Ehsaniara <ehsaniara@gmail.com>
Homepage: https://github.com/ehsaniara/worker
Description: Worker Job Isolation Platform
 A job isolation platform that provides secure execution of containerized
 workloads with resource management and namespace isolation.
 .
 This package includes the worker daemon, CLI tools, certificate generation,
 and systemd service configuration with interactive setup.
Installed-Size: $(du -sk $BUILD_DIR | cut -f1)
EOF

# Copy install scripts (enhanced versions)
cp ./debian/postinst "$BUILD_DIR/DEBIAN/"
cp ./debian/prerm "$BUILD_DIR/DEBIAN/"
cp ./debian/postrm "$BUILD_DIR/DEBIAN/"

# Add debconf config script if it exists
if [ -f "./debian/config" ]; then
    cp ./debian/config "$BUILD_DIR/DEBIAN/"
    chmod 755 "$BUILD_DIR/DEBIAN/config"
    echo "✅ Added debconf config script"
fi

# Add debconf templates if they exist
if [ -f "./debian/templates" ]; then
    cp ./debian/templates "$BUILD_DIR/DEBIAN/"
    echo "✅ Added debconf templates"
fi

# Make scripts executable
chmod 755 "$BUILD_DIR/DEBIAN/postinst"
chmod 755 "$BUILD_DIR/DEBIAN/prerm"
chmod 755 "$BUILD_DIR/DEBIAN/postrm"

# Build the package
PACKAGE_FILE="${PACKAGE_NAME}_${CLEAN_VERSION}_${ARCH}.deb"
dpkg-deb --build "$BUILD_DIR" "$PACKAGE_FILE"

echo "✅ Package built successfully: $PACKAGE_FILE"

# Verify package
echo "📋 Package information:"
dpkg-deb -I "$PACKAGE_FILE"

echo "📁 Package contents:"
dpkg-deb -c "$PACKAGE_FILE"

echo
echo "📦 Package Features:"
echo "  ✅ Interactive installation with network configuration prompts"
echo "  ✅ Support for reverse proxy, load balancer, and NAT scenarios"
echo "  ✅ Automatic certificate generation with custom hostnames/IPs"
echo "  ✅ Debconf support for reconfiguration (dpkg-reconfigure worker)"
echo "  ✅ Environment variable support for automation"
echo "  ✅ Non-interactive mode for CI/CD pipelines"
echo
echo "🚀 Installation methods:"
echo "  Interactive:    sudo dpkg -i $PACKAGE_FILE"
echo "  Pre-configured: WORKER_SERVER_ADDRESS='your-ip' sudo -E dpkg -i $PACKAGE_FILE"
echo "  Automated:      DEBIAN_FRONTEND=noninteractive sudo dpkg -i $PACKAGE_FILE"
echo "  Reconfigure:    sudo dpkg-reconfigure worker"