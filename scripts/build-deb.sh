#!/bin/bash
set -e

ARCH=${1:-amd64}
VERSION=${2:-1.0.0}
PACKAGE_NAME="worker"
BUILD_DIR="worker-deb-${ARCH}"

# Clean up version string for Debian package format
# Remove 'v' prefix and git commit suffixes
CLEAN_VERSION=$(echo "$VERSION" | sed 's/^v//' | sed 's/-[0-9]\+-g[a-f0-9]\+.*//' | sed 's/-[a-f0-9]\+$//')

# Ensure version starts with a digit and is valid
if [[ ! "$CLEAN_VERSION" =~ ^[0-9] ]]; then
    CLEAN_VERSION="1.0.0"
    echo "⚠️  Invalid version format, using default: $CLEAN_VERSION"
else
    echo "📦 Using cleaned version: $CLEAN_VERSION (from $VERSION)"
fi

echo "🔨 Building Debian package for $PACKAGE_NAME v$CLEAN_VERSION ($ARCH)..."

# Debug: Show current directory and contents
echo "🔍 Current directory: $(pwd)"
echo "🔍 Repository contents:"
ls -la

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
    echo "🔍 Looking for: ./worker"
    echo "🔍 Current directory contents:"
    ls -la
    exit 1
fi

echo "✅ Found worker binary"
cp ./worker "$BUILD_DIR/opt/worker/"

if [ ! -f "./worker-cli" ]; then
    echo "❌ Worker CLI binary not found!"
    echo "🔍 Looking for: ./worker-cli"
    exit 1
fi

echo "✅ Found worker-cli binary"
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
    echo "🔍 Looked for: ./config/config.yml, ./config.yaml, ./config/config.yaml"
    echo "🔍 Directory structure:"
    find . -name "*.yml" -o -name "*.yaml" | head -10
    exit 1
fi

# Copy service file
if [ -f "./etc/worker.service" ]; then
    cp ./etc/worker.service "$BUILD_DIR/etc/systemd/system/"
    echo "✅ Copied etc/worker.service"
else
    echo "❌ Service file not found!"
    echo "🔍 Looking for: ./etc/worker.service"
    echo "🔍 Contents of ./etc/:"
    ls -la ./etc/ || echo "etc directory not found"
    exit 1
fi

# Copy certificate generation script
if [ -f "./etc/certs_gen.sh" ]; then
    cp ./etc/certs_gen.sh "$BUILD_DIR/usr/local/bin/"
    echo "✅ Copied etc/certs_gen.sh"
else
    echo "❌ Certificate generation script not found!"
    echo "🔍 Looking for: ./etc/certs_gen.sh"
    echo "🔍 Contents of ./etc/:"
    ls -la ./etc/ || echo "etc directory not found"
    echo "🔍 All .sh files in repository:"
    find . -name "*.sh" | head -10
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
Maintainer: Ehsan Iara <ehsan@ehsaniara.com>
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
if [ -f "./debian/postinst" ]; then
    cp ./debian/postinst "$BUILD_DIR/DEBIAN/"
    echo "✅ Copied debian/postinst"
else
    echo "❌ debian/postinst not found!"
    exit 1
fi

if [ -f "./debian/prerm" ]; then
    cp ./debian/prerm "$BUILD_DIR/DEBIAN/"
    echo "✅ Copied debian/prerm"
else
    echo "❌ debian/prerm not found!"
    exit 1
fi

if [ -f "./debian/postrm" ]; then
    cp ./debian/postrm "$BUILD_DIR/DEBIAN/"
    echo "✅ Copied debian/postrm"
else
    echo "❌ debian/postrm not found!"
    exit 1
fi

# Make scripts executable
chmod 755 "$BUILD_DIR/DEBIAN/postinst"
chmod 755 "$BUILD_DIR/DEBIAN/prerm"
chmod 755 "$BUILD_DIR/DEBIAN/postrm"

echo "✅ All files copied successfully"

# Debug: Show what we're about to package
echo "🔍 Package contents:"
find "$BUILD_DIR" -type f | sort

# Build the package
PACKAGE_FILE="${PACKAGE_NAME}_${CLEAN_VERSION}_${ARCH}.deb"
echo "📦 Building package: $PACKAGE_FILE"

dpkg-deb --build "$BUILD_DIR" "$PACKAGE_FILE"

echo "✅ Package built successfully: $PACKAGE_FILE"

# Verify package
echo "📋 Package information:"
dpkg-deb -I "$PACKAGE_FILE"

echo "📁 Package contents:"
dpkg-deb -c "$PACKAGE_FILE"