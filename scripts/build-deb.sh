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
mkdir -p "$BUILD_DIR/etc/systemd/system"
mkdir -p "$BUILD_DIR/usr/local/bin"
mkdir -p "$BUILD_DIR/usr/bin"

# Copy binaries
if [ ! -f "./worker" ]; then
    echo "❌ Worker binary not found!"
    exit 1
fi
cp ./worker "$BUILD_DIR/opt/worker/"

if [ ! -f "./worker-cli" ]; then
    echo "❌ Worker CLI binary not found!"
    exit 1
fi
cp ./worker-cli "$BUILD_DIR/usr/bin/"

# Copy config file with proper handling
if [ -f "./config/config.yml" ]; then
    cp ./config/config.yml "$BUILD_DIR/opt/worker/"
    echo "✅ Copied config/config.yml"
elif [ -f "./config.yaml" ]; then
    cp ./config.yaml "$BUILD_DIR/opt/worker/config.yml"
    echo "✅ Copied config.yaml as config.yml"
elif [ -f "./config/config.yaml" ]; then
    cp ./config/config.yaml "$BUILD_DIR/opt/worker/config.yml"
    echo "✅ Copied config/config.yaml as config.yml"
else
    echo "❌ No config file found!"
    exit 1
fi

# Copy service file
cp ./etc/worker.service "$BUILD_DIR/etc/systemd/system/"

# Copy certificate generation script - FIXED TO USE CERTS_GEN.SH
if [ -f "./etc/certs_gen.sh" ]; then
    cp ./etc/certs_gen.sh "$BUILD_DIR/usr/local/bin/"
    echo "✅ Copied etc/certs_gen.sh"
else
    echo "❌ Certificate generation script not found!"
    echo "🔍 Looking for: ./etc/certs_gen.sh"
    echo "🔍 Contents of ./etc/:"
    ls -la ./etc/ || echo "etc directory not found"
    exit 1
fi

# Create control file
cat > "$BUILD_DIR/DEBIAN/control" << EOF
Package: $PACKAGE_NAME
Version: $CLEAN_VERSION
Section: utils
Priority: optional
Architecture: $ARCH
Depends: openssl (>= 1.1.1), systemd
Maintainer: Jay Ehsaniara <ehsaniara@gmail.com>
Homepage: https://github.com/ehsaniara/worker
Description: Worker Job Isolation Platform
 A job isolation platform that provides secure execution of containerized
 workloads with resource management and namespace isolation.
 .
 This package includes the worker daemon, CLI tools, certificate generation,
 and systemd service configuration.
Installed-Size: $(du -sk $BUILD_DIR | cut -f1)
EOF

# Copy install scripts
cp ./debian/postinst "$BUILD_DIR/DEBIAN/"
cp ./debian/prerm "$BUILD_DIR/DEBIAN/"
cp ./debian/postrm "$BUILD_DIR/DEBIAN/"

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