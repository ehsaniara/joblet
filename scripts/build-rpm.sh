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

# Check if binaries already exist (CI mode)
if [ -f "./joblet" ] && [ -f "./rnx" ] && [ -f "./joblet-persist" ]; then
    echo "📦 Using pre-built binaries from root directory (CI mode)..."
    mkdir -p ./bin
    cp ./joblet ./bin/joblet
    cp ./rnx ./bin/rnx
    cp ./joblet-persist ./bin/joblet-persist
    chmod +x ./bin/joblet ./bin/rnx ./bin/joblet-persist
elif [ ! -f "./bin/joblet" ] || [ ! -f "./bin/rnx" ] || [ ! -f "./bin/joblet-persist" ]; then
    # Build all binaries if they don't exist
    echo "📦 Building all binaries..."
    make all || {
        echo "❌ Build failed!"
        exit 1
    }
else
    echo "📦 Using existing binaries from ./bin/..."
fi

# Get the current date for changelog
CHANGELOG_DATE=$(date '+%a %b %d %Y')

# Clean and create build directory
rm -rf "$BUILD_DIR"
mkdir -p "$BUILD_DIR"/{BUILD,BUILDROOT,RPMS,SOURCES,SPECS,SRPMS}

# Create source directory structure
mkdir -p "$BUILD_DIR/SOURCES/${PACKAGE_NAME}-${CLEAN_VERSION}"

# Copy binaries
if [ ! -f "./bin/joblet" ]; then
    echo "❌ Joblet binary not found!"
    exit 1
fi
cp ./bin/joblet "$BUILD_DIR/SOURCES/${PACKAGE_NAME}-${CLEAN_VERSION}/"

if [ ! -f "./bin/rnx" ]; then
    echo "❌ RNX CLI binary not found!"
    exit 1
fi
cp ./bin/rnx "$BUILD_DIR/SOURCES/${PACKAGE_NAME}-${CLEAN_VERSION}/"

if [ ! -f "./bin/joblet-persist" ]; then
    echo "❌ joblet-persist binary not found!"
    exit 1
fi
cp ./bin/joblet-persist "$BUILD_DIR/SOURCES/${PACKAGE_NAME}-${CLEAN_VERSION}/"

# Copy scripts and configs
cp -r ./scripts "$BUILD_DIR/SOURCES/${PACKAGE_NAME}-${CLEAN_VERSION}/" || {
    echo "⚠️  Scripts directory not found, creating minimal structure"
    mkdir -p "$BUILD_DIR/SOURCES/${PACKAGE_NAME}-${CLEAN_VERSION}/scripts"
}

# Ensure required files exist
for file in joblet-config-template.yml rnx-config-template.yml joblet.service certs_gen_embedded.sh common-install-functions.sh; do
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

# Note: joblet-persist now runs as a subprocess of joblet, no separate service needed

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
mkdir -p \$RPM_BUILD_ROOT/opt/joblet/bin
mkdir -p \$RPM_BUILD_ROOT/opt/joblet/scripts
mkdir -p \$RPM_BUILD_ROOT/etc/systemd/system
mkdir -p \$RPM_BUILD_ROOT/usr/local/bin
mkdir -p \$RPM_BUILD_ROOT/var/lib/joblet
mkdir -p \$RPM_BUILD_ROOT/etc/modules-load.d

# Install binaries to /opt/joblet/bin
cp joblet \$RPM_BUILD_ROOT/opt/joblet/bin/
cp rnx \$RPM_BUILD_ROOT/opt/joblet/bin/
cp joblet-persist \$RPM_BUILD_ROOT/opt/joblet/bin/

# Install config templates and scripts
cp scripts/joblet-config-template.yml \$RPM_BUILD_ROOT/opt/joblet/scripts/
cp scripts/rnx-config-template.yml \$RPM_BUILD_ROOT/opt/joblet/scripts/
cp scripts/common-install-functions.sh \$RPM_BUILD_ROOT/opt/joblet/scripts/

# joblet-persist runs as subprocess, no separate service needed

# Install systemd service
cp scripts/joblet.service \$RPM_BUILD_ROOT/etc/systemd/system/

# Install certificate generation script
cp scripts/certs_gen_embedded.sh \$RPM_BUILD_ROOT/usr/local/bin/
chmod +x \$RPM_BUILD_ROOT/usr/local/bin/certs_gen_embedded.sh

# Create symlinks for system-wide commands
ln -sf /opt/joblet/bin/rnx \$RPM_BUILD_ROOT/usr/local/bin/rnx

# Create br_netfilter module loading config
cat > \$RPM_BUILD_ROOT/etc/modules-load.d/joblet.conf << 'MODULESEOF'
# Load modules required for Joblet network isolation
br_netfilter
MODULESEOF

%clean
rm -rf \$RPM_BUILD_ROOT

%pre
# Source common installation functions
if [ -f /opt/joblet/scripts/common-install-functions.sh ]; then
    . /opt/joblet/scripts/common-install-functions.sh
fi

# Note: Most setup happens in %post after files are installed

%post
# Source common installation functions (now files are installed)
if [ -f /opt/joblet/scripts/common-install-functions.sh ]; then
    . /opt/joblet/scripts/common-install-functions.sh
else
    echo "ERROR: Common installation functions not found!"
    echo "Installation may be incomplete."
fi

# Display system changes warning
display_system_changes_warning

echo "🔧 Configuring Joblet Service..."
echo ""

# Get configuration from environment variables
get_configuration

# Detect AWS environment
detect_aws_environment

# Display configuration summary
echo ""
echo "Configuration Summary:\$EC2_INFO"
echo "  gRPC Server Bind: \$JOBLET_SERVER_ADDRESS:\$JOBLET_SERVER_PORT"
echo "  Certificate Primary IP: \$JOBLET_CERT_PRIMARY"
if [ -n "\$JOBLET_CERT_PUBLIC_IP" ]; then
    echo "  Certificate Public IP: \$JOBLET_CERT_PUBLIC_IP"
fi
if [ -n "\$JOBLET_CERT_DOMAIN" ]; then
    echo "  Certificate Domain(s): \$JOBLET_CERT_DOMAIN"
fi
echo ""

# Generate certificates
if generate_and_embed_certificates; then
    # Set secure permissions on config files
    chmod 600 /opt/joblet/config/joblet-config.yml 2>/dev/null || true
    chmod 600 /opt/joblet/config/rnx-config.yml 2>/dev/null || true

    # Create convenience copy for local CLI usage
    if [ -f /opt/joblet/config/rnx-config.yml ]; then
        mkdir -p /etc/joblet
        cp /opt/joblet/config/rnx-config.yml /etc/joblet/rnx-config.yml
        chmod 644 /etc/joblet/rnx-config.yml
    fi
fi

# Setup network requirements
setup_network_requirements

# Create runtime directories
mkdir -p /var/log/joblet /opt/joblet/logs /opt/joblet/network /opt/joblet/volumes /opt/joblet/jobs
chown root:root /var/log/joblet /opt/joblet/logs /opt/joblet/network /opt/joblet/volumes /opt/joblet/jobs
chmod 755 /var/log/joblet /opt/joblet/logs /opt/joblet/network /opt/joblet/volumes /opt/joblet/jobs

# Setup cgroup delegation
if [ -d /sys/fs/cgroup ]; then
    mkdir -p /sys/fs/cgroup/joblet.slice
    echo "+cpu +memory +io +pids +cpuset" > /sys/fs/cgroup/joblet.slice/cgroup.subtree_control 2>/dev/null || true
fi

# Reload systemd to pick up the service file
systemctl daemon-reload

# Enable the service (but don't start automatically)
systemctl enable joblet.service 2>/dev/null || true

# Display quickstart information
display_quickstart_info "rpm"

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
    echo "🧹 Cleaning up Joblet resources..."

    # Clean up cgroup directories
    if [ -d "/sys/fs/cgroup/joblet.slice" ]; then
        find /sys/fs/cgroup/joblet.slice -name "job-*" -type d -exec rmdir {} \; 2>/dev/null || true
    fi

    # Clean up network resources
    if command -v ip >/dev/null 2>&1; then
        # Remove joblet bridge
        ip link delete joblet0 2>/dev/null || true

        # Clean up any leftover veth interfaces
        for veth in \$(ip link show | grep -oE "viso[0-9]+" | grep -v '@'); do
            ip link delete \$veth 2>/dev/null || true
        done
    fi

    # Detect firewall backend and clean up
    FIREWALL_BACKEND=""
    if command -v firewall-cmd >/dev/null 2>&1 && systemctl is-active firewalld >/dev/null 2>&1; then
        FIREWALL_BACKEND="firewalld"
    elif command -v nft >/dev/null 2>&1 && nft list tables 2>/dev/null | grep -q .; then
        FIREWALL_BACKEND="nftables"
    elif command -v iptables >/dev/null 2>&1; then
        FIREWALL_BACKEND="iptables"
    fi

    case "\$FIREWALL_BACKEND" in
        firewalld)
            # Clean up firewalld rules
            firewall-cmd --permanent --remove-masquerade >/dev/null 2>&1 || true
            firewall-cmd --permanent --direct --remove-rule ipv4 nat POSTROUTING 0 -s 172.20.0.0/16 -j MASQUERADE 2>/dev/null || true
            firewall-cmd --permanent --direct --remove-rule ipv4 filter FORWARD 0 -i joblet0 -j ACCEPT 2>/dev/null || true
            firewall-cmd --permanent --direct --remove-rule ipv4 filter FORWARD 0 -o joblet0 -j ACCEPT 2>/dev/null || true
            firewall-cmd --reload >/dev/null 2>&1 || true
            ;;
        nftables)
            # Remove nftables table
            nft delete table inet joblet 2>/dev/null || true
            # Remove from nftables.conf if it exists
            if [ -f /etc/nftables.conf ]; then
                sed -i '/^table inet joblet/,/^}/d' /etc/nftables.conf 2>/dev/null || true
            fi
            ;;
        iptables)
            # Remove iptables rules
            iptables -t nat -D POSTROUTING -s 172.20.0.0/16 -j MASQUERADE 2>/dev/null || true
            iptables -D FORWARD -i joblet0 -j ACCEPT 2>/dev/null || true
            iptables -D FORWARD -o joblet0 -j ACCEPT 2>/dev/null || true
            iptables -D FORWARD -i viso+ -j ACCEPT 2>/dev/null || true
            iptables -D FORWARD -o viso+ -j ACCEPT 2>/dev/null || true
            # Save if possible
            if command -v service >/dev/null 2>&1; then
                service iptables save 2>/dev/null || true
            fi
            ;;
    esac

    # Remove sysctl config
    rm -f /etc/sysctl.d/99-joblet.conf

    # Remove module loading config
    rm -f /etc/modules-load.d/joblet.conf

    # Remove systemd log directory
    rm -rf /var/log/joblet

    # Remove convenience config copy
    rm -rf /etc/joblet

    # Note: Job logs in /opt/joblet/logs are preserved
    if [ -d "/opt/joblet/logs" ] && [ "\$(ls -A /opt/joblet/logs 2>/dev/null)" ]; then
        echo "ℹ️  Job logs preserved in /opt/joblet/logs"
        echo "ℹ️  To remove all data: rm -rf /opt/joblet"
    fi

    # Reload systemd
    systemctl daemon-reload

    echo "✅ Joblet uninstalled successfully"
fi

%files
%defattr(-,root,root,-)
/opt/joblet/bin/joblet
/opt/joblet/bin/rnx
/opt/joblet/bin/joblet-persist
/opt/joblet/scripts/joblet-config-template.yml
/opt/joblet/scripts/rnx-config-template.yml
/opt/joblet/scripts/common-install-functions.sh
/etc/systemd/system/joblet.service
/usr/local/bin/certs_gen_embedded.sh
/usr/local/bin/rnx
/etc/modules-load.d/joblet.conf

%dir /opt/joblet
%dir /opt/joblet/bin
%dir /opt/joblet/scripts
%dir /var/lib/joblet
%dir /etc/modules-load.d

%changelog
* ${CHANGELOG_DATE} Joblet Build System <build@joblet.dev> - ${CLEAN_VERSION}-${RELEASE}
- Network isolation features (bridge, isolated, custom networks)
- Automatic network setup with conflict detection
- IP forwarding and NAT configuration
- Support for multiple firewall systems (iptables, nftables, firewalld)
- AWS EC2 environment detection and CloudWatch Logs support
- Enhanced error handling and user feedback
- Prominent system modification warnings
- Improved certificate generation during installation
- Comprehensive cleanup on removal
- Shared installation functions for consistency
EOF

# Create source tarball
tar -czf "$BUILD_DIR/SOURCES/${PACKAGE_NAME}-${CLEAN_VERSION}.tar.gz" \
    -C "$BUILD_DIR/SOURCES" \
    "${PACKAGE_NAME}-${CLEAN_VERSION}"

# Build the RPM
echo "📦 Building RPM package..."
rpmbuild --define "_topdir $(pwd)/$BUILD_DIR" \
         --define "_arch $RPM_ARCH" \
         --target "$RPM_ARCH" \
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
echo "  - Bridge network:   rnx job run --network=bridge <command>"
echo "  - Isolated network: rnx job run --network=isolated <command>"
echo "  - Custom networks:  rnx network create <name> --cidr=<cidr>"
echo ""
echo "  With custom IP:     JOBLET_SERVER_IP='your-ip' sudo -E yum localinstall -y $PACKAGE_FILE"
echo "  Verification:       rpm -V joblet"
echo "  Service:            sudo systemctl start joblet && sudo systemctl enable joblet"