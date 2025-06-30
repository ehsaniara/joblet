#!/bin/bash
set -e

ARCH=${1:-amd64}
VERSION=${2:-1.0.0}
PACKAGE_NAME="worker"
BUILD_DIR="worker-deb-${ARCH}"

echo "🔨 Building Debian package for $PACKAGE_NAME v$VERSION ($ARCH)..."

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
cp ./worker-cli "$BUILD_DIR/usr/bin/"
cp ./config/config.yml "$BUILD_DIR/opt/worker/"

# Copy service file
cp ./etc/worker.service "$BUILD_DIR/etc/systemd/system/"

# Copy certificate generation script
cp ./etc/cert_gen.sh "$BUILD_DIR/usr/local/bin/"

# Create control file
cat > "$BUILD_DIR/DEBIAN/control" << EOF
Package: $PACKAGE_NAME
Version: $VERSION
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
PACKAGE_FILE="${PACKAGE_NAME}_${VERSION}_${ARCH}.deb"
dpkg-deb --build "$BUILD_DIR" "$PACKAGE_FILE"

echo "✅ Package built successfully: $PACKAGE_FILE"

# Verify package
echo "📋 Package information:"
dpkg-deb -I "$PACKAGE_FILE"

echo "📁 Package contents:"
dpkg-deb -c "$PACKAGE_FILE"