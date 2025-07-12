#!/bin/bash
set -e

ARCH=${1:-x86_64}
VERSION=${2:-1.0.0}
PACKAGE_NAME="joblet"
BUILD_DIR="joblet-rpm-${ARCH}"
RELEASE=${3:-1}

# Clean up version string for RPM package format
CLEAN_VERSION=$(echo "$VERSION" | sed 's/^v//' | sed 's/-[0-9]\+-g[a-f0-9]\+.*//' | sed 's/-[a-f0-9]\+$//')

# Ensure version starts with a digit and is valid
if [[ ! "$CLEAN_VERSION" =~ ^[0-9] ]]; then
    CLEAN_VERSION="1.0.0"
    echo "⚠️  Invalid version format, using default: $CLEAN_VERSION"
else
    echo "📦 Using cleaned version: $CLEAN_VERSION (from $VERSION)"
fi

echo "🔨 Building RPM package for $PACKAGE_NAME v$CLEAN_VERSION-$RELEASE ($ARCH)..."

# Clean and create build directory
rm -rf "$BUILD_DIR"
mkdir -p "$BUILD_DIR"

# Create RPM directory structure
mkdir -p "$BUILD_DIR/BUILD"
mkdir -p "$BUILD_DIR/BUILDROOT"
mkdir -p "$BUILD_DIR/RPMS"
mkdir -p "$BUILD_DIR/SOURCES"
mkdir -p "$BUILD_DIR/SPECS"
mkdir -p "$BUILD_DIR/SRPMS"

# Create buildroot structure
BUILDROOT="$BUILD_DIR/BUILDROOT/${PACKAGE_NAME}-${CLEAN_VERSION}-${RELEASE}.${ARCH}"
mkdir -p "$BUILDROOT"
mkdir -p "$BUILDROOT/opt/joblet"
mkdir -p "$BUILDROOT/opt/joblet/config"
mkdir -p "$BUILDROOT/opt/joblet/scripts"
mkdir -p "$BUILDROOT/etc/systemd/system"
mkdir -p "$BUILDROOT/usr/local/bin"

# Copy binaries
if [ ! -f "./joblet" ]; then
    echo "❌ Joblet binary not found!"
    exit 1
fi
cp ./joblet "$BUILDROOT/opt/joblet/"

if [ ! -f "./rnx" ]; then
    echo "❌ RNX CLI binary not found!"
    exit 1
fi
cp ./rnx "$BUILDROOT/opt/joblet/"

# Copy template files (NOT actual configs with certificates)
if [ -f "./scripts/joblet-config-template.yml" ]; then
    cp ./scripts/joblet-config-template.yml "$BUILDROOT/opt/joblet/scripts/"
    echo "✅ Copied joblet-config-template.yml"
else
    echo "❌ Server config template not found: ./scripts/joblet-config-template.yml"
    exit 1
fi

if [ -f "./scripts/rnx-config-template.yml" ]; then
    cp ./scripts/rnx-config-template.yml "$BUILDROOT/opt/joblet/scripts/"
    echo "✅ Copied rnx-config-template.yml"
else
    echo "❌ Client config template not found: ./scripts/rnx-config-template.yml"
    exit 1
fi

# Copy service file
cp ./scripts/joblet.service "$BUILDROOT/etc/systemd/system/"

# Copy certificate generation script (embedded version)
cp ./scripts/certs_gen_embedded.sh "$BUILDROOT/usr/local/bin/certs_gen_embedded.sh"
chmod +x "$BUILDROOT/usr/local/bin/certs_gen_embedded.sh"

# Create the RPM spec file
cat > "$BUILD_DIR/SPECS/${PACKAGE_NAME}.spec" << EOF
Name:           ${PACKAGE_NAME}
Version:        ${CLEAN_VERSION}
Release:        ${RELEASE}%{?dist}
Summary:        Joblet Job Isolation Platform with Embedded Certificates
License:        MIT
URL:            https://github.com/ehsaniara/joblet
Source0:        %{name}-%{version}.tar.gz
BuildArch:      ${ARCH}

# Dependencies for Amazon Linux 2/2023
Requires:       openssl >= 1.0.2
Requires:       systemd

%description
A job isolation platform that provides secure execution of containerized
workloads with resource management and namespace isolation.

This package includes the joblet daemon, rnx CLI tools, and embedded certificate
management. All certificates are embedded directly in configuration files
for simplified deployment and management.

%prep
# No prep needed for pre-built binaries

%build
# No build needed for pre-built binaries

%install
# Create directory structure in buildroot
mkdir -p %{buildroot}/opt/joblet
mkdir -p %{buildroot}/opt/joblet/scripts
mkdir -p %{buildroot}/etc/systemd/system
mkdir -p %{buildroot}/usr/local/bin

# Copy files from our source directory to buildroot
cp %{_sourcedir}/../../../joblet %{buildroot}/opt/joblet/
cp %{_sourcedir}/../../../rnx %{buildroot}/opt/joblet/
cp %{_sourcedir}/../../../scripts/joblet-config-template.yml %{buildroot}/opt/joblet/scripts/
cp %{_sourcedir}/../../../scripts/rnx-config-template.yml %{buildroot}/opt/joblet/scripts/
cp %{_sourcedir}/../../../scripts/joblet.service %{buildroot}/etc/systemd/system/
cp %{_sourcedir}/../../../scripts/certs_gen_embedded.sh %{buildroot}/usr/local/bin/

%post
# Post-installation script (same as Debian postinst but adapted for RPM)
echo "🔧 Configuring Joblet Service..."

# Set basic permissions
chown -R root:root /opt/joblet
chmod 755 /opt/joblet
chmod 755 /opt/joblet/joblet
chmod 755 /opt/joblet/rnx
chmod 755 /opt/joblet/scripts
chmod 644 /opt/joblet/scripts/joblet-config-template.yml
chmod 644 /opt/joblet/scripts/rnx-config-template.yml
chmod +x /usr/local/bin/certs_gen_embedded.sh

mkdir -p /opt/joblet/config
chmod 700 /opt/joblet/config

# Create symlinks
if [ ! -L /usr/bin/rnx ]; then
    ln -sf /opt/joblet/rnx /usr/bin/rnx
fi

if [ ! -L /usr/local/bin/rnx ]; then
    ln -sf /opt/joblet/rnx /usr/local/bin/rnx
fi

# Auto-detect internal IP
JOBLET_CERT_INTERNAL_IP=\$(ip route get 1.1.1.1 2>/dev/null | grep -oP 'src \K[0-9.]+' | head -1)
if [ -z "\$JOBLET_CERT_INTERNAL_IP" ]; then
    JOBLET_CERT_INTERNAL_IP=\$(ip -4 addr show | grep -oP '(?<=inet\s)\d+(\.\d+){3}' | grep -v '127.0.0.1' | head -1)
fi
JOBLET_CERT_INTERNAL_IP=\${JOBLET_CERT_INTERNAL_IP:-127.0.0.1}

# Set server configuration
JOBLET_SERVER_ADDRESS=\${JOBLET_SERVER_ADDRESS:-0.0.0.0}
JOBLET_SERVER_PORT=\${JOBLET_SERVER_PORT:-50051}
JOBLET_CERT_PRIMARY=\${JOBLET_CERT_PRIMARY:-\$JOBLET_CERT_INTERNAL_IP}

# Build additional names for certificate
JOBLET_ADDITIONAL_NAMES="localhost"
if [ -n "\$JOBLET_CERT_INTERNAL_IP" ] && [ "\$JOBLET_CERT_INTERNAL_IP" != "\$JOBLET_CERT_PRIMARY" ]; then
    JOBLET_ADDITIONAL_NAMES="\$JOBLET_ADDITIONAL_NAMES,\$JOBLET_CERT_INTERNAL_IP"
fi

echo "Configuration Summary:"
echo "  gRPC Server Bind: \$JOBLET_SERVER_ADDRESS:\$JOBLET_SERVER_PORT"
echo "  Certificate Primary IP: \$JOBLET_CERT_PRIMARY"

# Generate certificates and embed them in config files
echo "Generating certificates with embedded configuration..."
export JOBLET_SERVER_ADDRESS="\$JOBLET_CERT_PRIMARY"
export JOBLET_ADDITIONAL_NAMES="\$JOBLET_ADDITIONAL_NAMES"
export JOBLET_MODE="package-install"

if /usr/local/bin/certs_gen_embedded.sh; then
    echo "✅ Certificates generated successfully"

    # Update server configuration
    if [ -f /opt/joblet/config/joblet-config.yml ]; then
        sed -i "s/^  address:.*/  address: \"\$JOBLET_SERVER_ADDRESS\"/" /opt/joblet/config/joblet-config.yml
        sed -i "s/^  port:.*/  port: \$JOBLET_SERVER_PORT/" /opt/joblet/config/joblet-config.yml
        echo "✅ Updated server configuration"
    fi

    # Update client configuration
    if [ -f /opt/joblet/config/rnx-config.yml ]; then
        sed -i "s/address: \"[^:]*:50051\"/address: \"\$JOBLET_CERT_PRIMARY:\$JOBLET_SERVER_PORT\"/" /opt/joblet/config/rnx-config.yml
        echo "✅ Updated client configuration"
    fi

    # Set secure permissions
    chmod 600 /opt/joblet/config/joblet-config.yml
    chmod 600 /opt/joblet/config/rnx-config.yml

    # Create convenience copy for local CLI usage
    if [ -f /opt/joblet/config/rnx-config.yml ]; then
        mkdir -p /etc/joblet
        cp /opt/joblet/config/rnx-config.yml /etc/joblet/rnx-config.yml
        chmod 644 /etc/joblet/rnx-config.yml
    fi
else
    echo "❌ Certificate generation failed"
    exit 1
fi

# Setup cgroup delegation
if [ -d /sys/fs/cgroup ]; then
    echo "Setting up cgroup delegation..."
    mkdir -p /sys/fs/cgroup/joblet.slice
    echo "+cpu +memory +io +pids +cpuset" > /sys/fs/cgroup/joblet.slice/cgroup.subtree_control 2>/dev/null || true
fi

# Create log directory
mkdir -p /var/log/joblet
chown root:root /var/log/joblet
chmod 755 /var/log/joblet

# Enable systemd service
systemctl daemon-reload
systemctl enable joblet.service

echo
echo "✅ Joblet service installed successfully!"
echo
echo "🚀 Quick Start:"
echo "  sudo systemctl start joblet    # Start the service"
echo "  sudo rnx list                  # Test local connection"
echo
echo "📱 Remote Access:"
echo "  Clients can connect using: \$JOBLET_CERT_PRIMARY:\$JOBLET_SERVER_PORT"
echo
echo "📋 Client Configuration:"
echo "  Copy /opt/joblet/config/rnx-config.yml to client machines"
echo

%preun
# Pre-uninstallation script
if [ \$1 -eq 0 ]; then
    # Stop and disable service on complete removal
    systemctl stop joblet.service || true
    systemctl disable joblet.service || true
fi

%postun
# Post-uninstallation script
if [ \$1 -eq 0 ]; then
    # Complete removal
    # Clean up cgroup directories
    if [ -d "/sys/fs/cgroup/joblet.slice" ]; then
        find /sys/fs/cgroup/joblet.slice -name "job-*" -type d -exec rmdir {} \; 2>/dev/null || true
    fi

    # Remove symlinks
    rm -f /usr/bin/rnx
    rm -f /usr/local/bin/rnx

    # Remove user-accessible certificate symlinks
    rm -rf /etc/joblet

    echo "Joblet service removed successfully!"
fi

if [ \$1 -eq 0 ]; then
    # Purge (complete removal)
    # Remove all joblet files
    rm -rf /opt/joblet
    rm -rf /var/log/joblet
    rm -rf /etc/joblet

    echo "Joblet service purged successfully!"
fi

%files
%defattr(-,root,root,-)
/opt/joblet/joblet
/opt/joblet/rnx
/opt/joblet/scripts/joblet-config-template.yml
/opt/joblet/scripts/rnx-config-template.yml
/etc/systemd/system/joblet.service
/usr/local/bin/certs_gen_embedded.sh

%dir /opt/joblet
%dir /opt/joblet/scripts

%changelog
* $(date '+%a %b %d %Y') Joblet Build System <build@joblet.dev> - ${CLEAN_VERSION}-${RELEASE}
- RPM package for Amazon Linux
- Embedded certificate management
- Equivalent to Debian package functionality

EOF

# Create source tarball (RPM expects this even for binary packages)
tar -czf "$BUILD_DIR/SOURCES/${PACKAGE_NAME}-${CLEAN_VERSION}.tar.gz" -C "$BUILD_DIR" --exclude="SOURCES" --exclude="SPECS" --exclude="BUILD*" --exclude="RPMS" --exclude="SRPMS" .

# Build the RPM package
cd "$BUILD_DIR"
rpmbuild --define "_topdir $(pwd)" \
         --define "_builddir $(pwd)/BUILD" \
         --define "_buildrootdir $(pwd)/BUILDROOT" \
         --define "_rpmdir $(pwd)/RPMS" \
         --define "_sourcedir $(pwd)/SOURCES" \
         --define "_specdir $(pwd)/SPECS" \
         --define "_srcrpmdir $(pwd)/SRPMS" \
         -bb SPECS/${PACKAGE_NAME}.spec

cd ..

# Move the built RPM to the main directory
PACKAGE_FILE="${PACKAGE_NAME}-${CLEAN_VERSION}-${RELEASE}.${ARCH}.rpm"
if [ -f "$BUILD_DIR/RPMS/${ARCH}/${PACKAGE_FILE}" ]; then
    cp "$BUILD_DIR/RPMS/${ARCH}/${PACKAGE_FILE}" .
    echo "✅ Package built successfully: $PACKAGE_FILE"
else
    echo "❌ Package build failed - RPM not found"
    exit 1
fi

# Verify package
echo "📋 Package information:"
rpm -qip "$PACKAGE_FILE"

echo "📁 Package contents:"
rpm -qlp "$PACKAGE_FILE"

echo
echo "🚀 Installation methods:"
echo "  Basic:              sudo yum install -y $PACKAGE_FILE"
echo "  DNF (AL2023):       sudo dnf install -y $PACKAGE_FILE"
echo "  With EC2 auto:      sudo yum install -y $PACKAGE_FILE  # Auto-detects EC2 metadata"
echo "  Custom IP:          JOBLET_SERVER_IP='your-ip' sudo -E yum install -y $PACKAGE_FILE"
echo "  Verification:       rpm -V joblet"
echo "  Service:            sudo systemctl start joblet && sudo systemctl enable joblet"