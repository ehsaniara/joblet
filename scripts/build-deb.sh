#!/bin/bash
set -e

ARCH=${1:-amd64}
VERSION=${2:-1.0.0}
PACKAGE_NAME="joblet"
BUILD_DIR="joblet-deb-${ARCH}"

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

# Build all binaries first
echo "📦 Building all binaries..."
make all || {
    echo "❌ Build failed!"
    exit 1
}

# Clean and create build directory
rm -rf "$BUILD_DIR"
mkdir -p "$BUILD_DIR"

# Create directory structure
mkdir -p "$BUILD_DIR/DEBIAN"
mkdir -p "$BUILD_DIR/opt/joblet/bin"
mkdir -p "$BUILD_DIR/opt/joblet/config"
mkdir -p "$BUILD_DIR/opt/joblet/scripts"
mkdir -p "$BUILD_DIR/etc/systemd/system"
mkdir -p "$BUILD_DIR/usr/local/bin"

# Copy binaries to /opt/joblet/bin
if [ ! -f "./bin/joblet" ]; then
    echo "❌ Joblet binary not found!"
    exit 1
fi
cp ./bin/joblet "$BUILD_DIR/opt/joblet/bin/"

if [ ! -f "./bin/rnx" ]; then
    echo "❌ RNX CLI binary not found!"
    exit 1
fi
cp ./bin/rnx "$BUILD_DIR/opt/joblet/bin/"

if [ ! -f "./bin/joblet-persist" ]; then
    echo "❌ joblet-persist binary not found!"
    exit 1
fi
cp ./bin/joblet-persist "$BUILD_DIR/opt/joblet/bin/"

# Copy template files (NOT actual configs with certificates)
if [ -f "./scripts/joblet-config-template.yml" ]; then
    cp ./scripts/joblet-config-template.yml "$BUILD_DIR/opt/joblet/scripts/"
    echo "✅ Copied joblet-config-template.yml"
else
    echo "❌ Server config template not found: ./scripts/joblet-config-template.yml"
    exit 1
fi

if [ -f "./scripts/rnx-config-template.yml" ]; then
    cp ./scripts/rnx-config-template.yml "$BUILD_DIR/opt/joblet/scripts/"
    echo "✅ Copied rnx-config-template.yml"
else
    echo "❌ Client config template not found: ./scripts/rnx-config-template.yml"
    exit 1
fi

# Copy joblet-persist installation script
if [ -f "./debian/joblet-persist-install.sh" ]; then
    cp ./debian/joblet-persist-install.sh "$BUILD_DIR/opt/joblet/scripts/"
    chmod +x "$BUILD_DIR/opt/joblet/scripts/joblet-persist-install.sh"
    echo "✅ Copied joblet-persist-install.sh"
else
    echo "⚠️  Warning: joblet-persist-install.sh not found - persistence layer won't be installed"
fi

# Copy service files
cp ./scripts/joblet.service "$BUILD_DIR/etc/systemd/system/"

# Copy joblet-persist service file if it exists
if [ -f "./scripts/joblet-persist.service" ]; then
    cp ./scripts/joblet-persist.service "$BUILD_DIR/opt/joblet/scripts/"
    echo "✅ Copied joblet-persist.service"
fi

# Copy certificate generation script (embedded version)
cp ./scripts/certs_gen_embedded.sh "$BUILD_DIR/usr/local/bin/certs_gen_embedded.sh"
chmod +x "$BUILD_DIR/usr/local/bin/certs_gen_embedded.sh"

# Create control file
cat > "$BUILD_DIR/DEBIAN/control" << EOF
Package: $PACKAGE_NAME
Version: $CLEAN_VERSION
Section: utils
Priority: optional
Architecture: $ARCH
Depends: openssl (>= 1.1.1), systemd, debconf (>= 0.5) | debconf-2.0
Maintainer: Jay Ehsaniara <ehsaniara@gmail.com>
Homepage: https://github.com/ehsaniara/joblet
Description: Joblet Job Isolation Platform with Embedded Certificates
 A job isolation platform that provides secure execution of containerized
 workloads with resource management and namespace isolation.
 .
 This package includes the joblet daemon, rnx CLI tools, and embedded certificate
 management. All certificates are embedded directly in configuration files
 for simplified deployment and management.
Installed-Size: $(du -sk $BUILD_DIR | cut -f1)
EOF

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
echo "🚀 Installation methods:"
echo "  Interactive:    sudo dpkg -i $PACKAGE_FILE"
echo "  Pre-configured: JOBLET_SERVER_IP='your-ip' sudo -E dpkg -i $PACKAGE_FILE"
echo "  Automated:      DEBIAN_FRONTEND=noninteractive sudo dpkg -i $PACKAGE_FILE"
echo "  Reconfigure:    sudo dpkg-reconfigure joblet"