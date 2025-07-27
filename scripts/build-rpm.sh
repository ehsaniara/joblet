#!/bin/bash
set -e

ARCH=${1:-x86_64}
VERSION=${2:-1.0.0}
PACKAGE_NAME="joblet"
BUILD_DIR="rpmbuild"
RELEASE="1"

# Map architectures
case $ARCH in
    amd64|x86_64)
        RPM_ARCH="x86_64"
        ;;
    arm64|aarch64)
        RPM_ARCH="aarch64"
        ;;
    386|i386|i686)
        RPM_ARCH="i686"
        ;;
    *)
        echo "❌ Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

# Clean up version string for RPM package format
CLEAN_VERSION=$(echo "$VERSION" | sed 's/^v//' | sed 's/-[0-9]\+-g[a-f0-9]\+.*//' | sed 's/-[a-f0-9]\+$//')

# Ensure version starts with a digit and is valid
if [[ ! "$CLEAN_VERSION" =~ ^[0-9] ]]; then
    CLEAN_VERSION="1.0.0"
    echo "⚠️  Invalid version format, using default: $CLEAN_VERSION"
else
    echo "📦 Using cleaned version: $CLEAN_VERSION (from $VERSION)"
fi

echo "🔨 Building RPM package for $PACKAGE_NAME v$CLEAN_VERSION ($RPM_ARCH)..."

# Get the current date for changelog
CHANGELOG_DATE=$(date '+%a %b %d %Y')

# Clean and create build directory
rm -rf "$BUILD_DIR"
mkdir -p "$BUILD_DIR"/{BUILD,BUILDROOT,RPMS,SOURCES,SPECS,SRPMS}

# Create source directory structure
mkdir -p "$BUILD_DIR/SOURCES/${PACKAGE_NAME}-${CLEAN_VERSION}"

# Copy binaries
if [ ! -f "./joblet" ]; then
    echo "❌ Joblet binary not found!"
    exit 1
fi
cp ./joblet "$BUILD_DIR/SOURCES/${PACKAGE_NAME}-${CLEAN_VERSION}/"

if [ ! -f "./rnx" ]; then
    echo "❌ RNX CLI binary not found!"
    exit 1
fi
cp ./rnx "$BUILD_DIR/SOURCES/${PACKAGE_NAME}-${CLEAN_VERSION}/"

# Copy scripts and configs
cp -r ./scripts "$BUILD_DIR/SOURCES/${PACKAGE_NAME}-${CLEAN_VERSION}/" || {
    echo "⚠️  Scripts directory not found, creating minimal structure"
    mkdir -p "$BUILD_DIR/SOURCES/${PACKAGE_NAME}-${CLEAN_VERSION}/scripts"
}

# Ensure required files exist
for file in joblet-config-template.yml rnx-config-template.yml joblet.service certs_gen_embedded.sh; do
    if [ ! -f "$BUILD_DIR/SOURCES/${PACKAGE_NAME}-${CLEAN_VERSION}/scripts/$file" ]; then
        if [ -f "./scripts/$file" ]; then
            cp "./scripts/$file" "$BUILD_DIR/SOURCES/${PACKAGE_NAME}-${CLEAN_VERSION}/scripts/"
        elif [ -f "./$file" ]; then
            cp "./$file" "$BUILD_DIR/SOURCES/${PACKAGE_NAME}-${CLEAN_VERSION}/scripts/"
        else
            echo "❌ Required file not found: $file"
            exit 1
        fi
    fi
done

# Create the spec file with network support
cat > "$BUILD_DIR/SPECS/${PACKAGE_NAME}.spec" << EOF
%define _build_id_links none
%global debug_package %{nil}

Name:           joblet
Version:        ${CLEAN_VERSION}
Release:        ${RELEASE}%{?dist}
Summary:        Secure job execution platform with network isolation
License:        MIT
URL:            https://github.com/ehsaniara/joblet
Source0:        %{name}-%{version}.tar.gz

# Base requirements
Requires:       systemd
Requires:       openssl >= 1.1.1

# Network requirements
Requires:       iptables
Requires:       iproute
Requires:       bridge-utils
Requires:       procps-ng

# Distribution-specific dependencies
%if 0%{?rhel} >= 8 || 0%{?fedora} >= 30
Requires:       iptables-services
%endif

%if 0%{?suse_version}
Requires:       iproute2
%else
Requires:       iproute
%endif

BuildRoot:      %{_tmppath}/%{name}-%{version}-%{release}-root

%description
Joblet is a job isolation platform that provides secure execution of containerized
workloads with resource management, namespace isolation, and network isolation.

This package includes the joblet daemon, rnx CLI tools, embedded certificate
management, and network isolation features (bridge, isolated, and custom networks).

%prep
%setup -q

%build
# No build required - using pre-built binaries

%install
rm -rf \$RPM_BUILD_ROOT

# Create directories
mkdir -p \$RPM_BUILD_ROOT/opt/joblet
mkdir -p \$RPM_BUILD_ROOT/opt/joblet/scripts
mkdir -p \$RPM_BUILD_ROOT/etc/systemd/system
mkdir -p \$RPM_BUILD_ROOT/usr/local/bin
mkdir -p \$RPM_BUILD_ROOT/var/lib/joblet
mkdir -p \$RPM_BUILD_ROOT/etc/modules-load.d

# Install binaries
cp joblet \$RPM_BUILD_ROOT/opt/joblet/
cp rnx \$RPM_BUILD_ROOT/opt/joblet/

# Install config templates and scripts
cp scripts/joblet-config-template.yml \$RPM_BUILD_ROOT/opt/joblet/scripts/
cp scripts/rnx-config-template.yml \$RPM_BUILD_ROOT/opt/joblet/scripts/

# Install systemd service
cp scripts/joblet.service \$RPM_BUILD_ROOT/etc/systemd/system/

# Install certificate generation script
cp scripts/certs_gen_embedded.sh \$RPM_BUILD_ROOT/usr/local/bin/
chmod +x \$RPM_BUILD_ROOT/usr/local/bin/certs_gen_embedded.sh

# Create symlinks for system-wide commands
ln -sf /opt/joblet/rnx \$RPM_BUILD_ROOT/usr/local/bin/rnx

# Create br_netfilter module loading config
cat > \$RPM_BUILD_ROOT/etc/modules-load.d/joblet.conf << 'MODULESEOF'
# Load modules required for Joblet network isolation
br_netfilter
MODULESEOF

%clean
rm -rf \$RPM_BUILD_ROOT

%pre
# Create joblet user if it doesn't exist
if ! id joblet >/dev/null 2>&1; then
    useradd -r -s /sbin/nologin -d /var/lib/joblet -c "Joblet Service User" joblet
fi

# Load required kernel modules
modprobe br_netfilter 2>/dev/null || true

# Enable IP forwarding
sysctl -w net.ipv4.ip_forward=1 >/dev/null 2>&1 || true
echo "net.ipv4.ip_forward=1" > /etc/sysctl.d/99-joblet.conf

%post
# Reload systemd to pick up the service file
systemctl daemon-reload

# Enable and start the service
echo "To start Joblet service, run:"
echo "  sudo systemctl start joblet"
echo "  sudo systemctl enable joblet"
echo ""
echo "Network features are now available:"
echo "  Bridge network: rnx run --network=bridge <command>"
echo "  Isolated network: rnx run --network=isolated <command>"
echo "  Create custom network: rnx network create <name> --cidr=<cidr>"

# Ensure kernel modules are loaded
systemctl restart systemd-modules-load.service >/dev/null 2>&1 || true

# Configure firewall if available
if command -v firewall-cmd >/dev/null 2>&1; then
    firewall-cmd --permanent --add-masquerade >/dev/null 2>&1 || true
    firewall-cmd --reload >/dev/null 2>&1 || true
fi

%preun
# Only stop on actual uninstall, not upgrade
if [ \$1 -eq 0 ]; then
    # Stop and disable the service
    systemctl stop joblet || true
    systemctl disable joblet || true
fi

%postun
# Only run on actual uninstall, not upgrade
if [ \$1 -eq 0 ]; then
    # Clean up network resources
    if command -v ip >/dev/null 2>&1; then
        # Remove any leftover joblet bridges
        for bridge in \$(ip link show type bridge | grep -E "joblet[0-9-]+" | awk -F: '{print \$2}' | tr -d ' '); do
            ip link delete \$bridge 2>/dev/null || true
        done

        # Clean up any leftover veth interfaces
        for veth in \$(ip link show | grep -E "veth[0-9]+" | awk -F: '{print \$2}' | tr -d ' '); do
            ip link delete \$veth 2>/dev/null || true
        done
    fi

    # Clean up firewall rules if applicable
    if command -v firewall-cmd >/dev/null 2>&1; then
        firewall-cmd --permanent --remove-masquerade >/dev/null 2>&1 || true
        firewall-cmd --reload >/dev/null 2>&1 || true
    fi

    # Remove sysctl config
    rm -f /etc/sysctl.d/99-joblet.conf

    # Reload systemd
    systemctl daemon-reload
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
%dir /var/lib/joblet

%changelog
* ${CHANGELOG_DATE} Joblet Build System <build@joblet.dev> - ${CLEAN_VERSION}-${RELEASE}
- Network isolation features (bridge, isolated, custom networks)
- Automatic network setup during installation
- IP forwarding and NAT configuration
- Support for multiple firewall systems
- Enhanced cleanup on removal
EOF

# Create source tarball
tar -czf "$BUILD_DIR/SOURCES/${PACKAGE_NAME}-${CLEAN_VERSION}.tar.gz" \
    -C "$BUILD_DIR/SOURCES" \
    "${PACKAGE_NAME}-${CLEAN_VERSION}"

# Build the RPM
echo "📦 Building RPM package..."
rpmbuild --define "_topdir $(pwd)/$BUILD_DIR" \
         --define "_arch $RPM_ARCH" \
         -bb "$BUILD_DIR/SPECS/${PACKAGE_NAME}.spec"

# Find the built package
PACKAGE_FILE=$(find "$BUILD_DIR/RPMS" -name "*.rpm" -type f | head -1)

if [ -f "$PACKAGE_FILE" ]; then
    # Move to current directory
    mv "$PACKAGE_FILE" .
    PACKAGE_FILE=$(basename "$PACKAGE_FILE")
    echo "✅ Package built successfully: $PACKAGE_FILE"
else
    echo "❌ Package build failed - RPM not found"
    ls -la "$BUILD_DIR/RPMS/"
    if [ -d "$BUILD_DIR/RPMS/${RPM_ARCH}" ]; then
        ls -la "$BUILD_DIR/RPMS/${RPM_ARCH}/"
    fi
    exit 1
fi

echo "📋 Package information:"
rpm -qip "$PACKAGE_FILE"

echo "📁 Package contents:"
rpm -qlp "$PACKAGE_FILE"

echo
echo "🚀 Installation methods:"
echo "  Amazon Linux 2:     sudo yum localinstall -y $PACKAGE_FILE"
echo "  Amazon Linux 2023:  sudo dnf localinstall -y $PACKAGE_FILE"
echo "  RHEL/CentOS 8+:     sudo dnf localinstall -y $PACKAGE_FILE"
echo "  RHEL/CentOS 7:      sudo yum localinstall -y $PACKAGE_FILE"
echo "  Fedora:             sudo dnf localinstall -y $PACKAGE_FILE"
echo "  SUSE/openSUSE:      sudo zypper install -y $PACKAGE_FILE"
echo ""
echo "🔧 Network features:"
echo "  - Bridge network:   rnx run --network=bridge <command>"
echo "  - Isolated network: rnx run --network=isolated <command>"
echo "  - Custom networks:  rnx network create <name> --cidr=<cidr>"
echo ""
echo "  With custom IP:     JOBLET_SERVER_IP='your-ip' sudo -E yum localinstall -y $PACKAGE_FILE"
echo "  Verification:       rpm -V joblet"
echo "  Service:            sudo systemctl start joblet && sudo systemctl enable joblet"