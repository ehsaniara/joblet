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

# Copy SERVER config file
if [ -f "./config/server-config.yml" ]; then
    cp ./config/server-config.yml "$BUILD_DIR/opt/worker/config/"
    echo "✅ Copied server-config.yml to /opt/worker/config/"
else
    echo "❌ Server config file not found: ./config/server-config.yml"
    exit 1
fi

# Generate CLIENT config file template
cat > "$BUILD_DIR/opt/worker/config/client-config.yml" << 'EOF'
version: "3.0"

nodes:
  default:
    address: "localhost:50051"
    cert: "/opt/worker/certs/client-cert.pem"
    key: "/opt/worker/certs/client-key.pem"
    ca: "/opt/worker/certs/ca-cert.pem"

  production:
    address: "production-server:50051"
    cert: "/opt/worker/certs/client-cert.pem"
    key: "/opt/worker/certs/client-key.pem"
    ca: "/opt/worker/certs/ca-cert.pem"
EOF
echo "✅ Generated client-config.yml template"

# Copy service file
cp ./scripts/worker.service "$BUILD_DIR/etc/systemd/system/"

# Copy certificate generation script
cp ./scripts/certs_gen.sh "$BUILD_DIR/usr/local/bin/"

# Create control file
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

# Copy install scripts
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
echo "  ✅ Interactive installation with server IP configuration"
echo "  ✅ Automatic certificate generation for the specified IP"
echo "  ✅ Both server and client config files included"
echo "  ✅ Debconf support for reconfiguration (dpkg-reconfigure worker)"
echo "  ✅ Environment variable support for automation"
echo "  ✅ Non-interactive mode for CI/CD pipelines"
echo
echo "🚀 Installation methods:"
echo "  Interactive:    sudo dpkg -i $PACKAGE_FILE"
echo "  Pre-configured: WORKER_SERVER_IP='your-ip' sudo -E dpkg -i $PACKAGE_FILE"
echo "  Automated:      DEBIAN_FRONTEND=noninteractive sudo dpkg -i $PACKAGE_FILE"
echo "  Reconfigure:    sudo dpkg-reconfigure worker"