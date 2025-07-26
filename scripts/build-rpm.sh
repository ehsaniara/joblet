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
cat > "$BUILD_DIR/SPECS/${PACKAGE_NAME}.spec" << 'EOF'
%define _build_id_links none
%global debug_package %{nil}

Name:           joblet
Version:        CLEAN_VERSION
Release:        RELEASE%{?dist}
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

Features:
- Process isolation using Linux namespaces
- Resource limits (CPU, memory, I/O, CPU cores)
- Network isolation with custom networks
- Real-time job monitoring and logging
- Secure mTLS communication with embedded certificates
- Cross-platform CLI client (rnx)

%prep
# No prep needed for pre-built binaries

%build
# No build needed for pre-built binaries

%install
rm -rf %{buildroot}

# Create directory structure
mkdir -p %{buildroot}/opt/joblet
mkdir -p %{buildroot}/opt/joblet/scripts
mkdir -p %{buildroot}/etc/systemd/system
mkdir -p %{buildroot}/usr/local/bin
mkdir -p %{buildroot}/var/lib/joblet
mkdir -p %{buildroot}/etc/modules-load.d

# Copy files (they should be in the current directory)
cp joblet %{buildroot}/opt/joblet/
cp rnx %{buildroot}/opt/joblet/
cp scripts/joblet-config-template.yml %{buildroot}/opt/joblet/scripts/
cp scripts/rnx-config-template.yml %{buildroot}/opt/joblet/scripts/
cp scripts/joblet.service %{buildroot}/etc/systemd/system/
cp scripts/certs_gen_embedded.sh %{buildroot}/usr/local/bin/

%post
# Post-installation script with network setup
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

# Create symlinks for CLI access
if [ ! -L /usr/bin/rnx ]; then
    ln -sf /opt/joblet/rnx /usr/bin/rnx
fi

if [ ! -L /usr/local/bin/rnx ]; then
    ln -sf /opt/joblet/rnx /usr/local/bin/rnx
fi

# Network setup
echo "🔧 Setting up network requirements..."

# Enable IP forwarding
sysctl -w net.ipv4.ip_forward=1 >/dev/null 2>&1 || :

# Make IP forwarding permanent
if ! grep -q "^net.ipv4.ip_forward = 1" /etc/sysctl.conf 2>/dev/null; then
    echo "net.ipv4.ip_forward = 1" >> /etc/sysctl.conf
fi

# Load kernel modules
for module in br_netfilter nf_conntrack nf_nat; do
    modprobe $module 2>/dev/null || :
done

# Auto-load modules on boot
cat > /etc/modules-load.d/joblet.conf << 'MODULES_EOF'
br_netfilter
nf_conntrack
nf_nat
MODULES_EOF

# Create state directory for network configs
mkdir -p /var/lib/joblet
chown root:root /var/lib/joblet
chmod 755 /var/lib/joblet

# Setup default bridge if it doesn't exist
if ! ip link show joblet0 >/dev/null 2>&1; then
    ip link add joblet0 type bridge 2>/dev/null || :
    ip addr add 172.20.0.1/16 dev joblet0 2>/dev/null || :
    ip link set joblet0 up 2>/dev/null || :
    echo "✅ Created default bridge network"
fi

# Configure iptables for NAT
if ! iptables -t nat -C POSTROUTING -s 172.20.0.0/16 -j MASQUERADE 2>/dev/null; then
    iptables -t nat -A POSTROUTING -s 172.20.0.0/16 -j MASQUERADE 2>/dev/null || :
    echo "✅ Configured NAT for bridge network"
fi

# Check and configure FORWARD chain
FORWARD_POLICY=$(iptables -L FORWARD -n | head -1 | grep -oP 'policy \K\w+' || echo "ACCEPT")
if [ "$FORWARD_POLICY" = "DROP" ]; then
    echo "⚠️  iptables FORWARD policy is DROP. Adding ACCEPT rules for joblet..."
    iptables -I FORWARD -i joblet0 -j ACCEPT 2>/dev/null || :
    iptables -I FORWARD -o joblet0 -j ACCEPT 2>/dev/null || :
    iptables -I FORWARD -i viso+ -j ACCEPT 2>/dev/null || :
    iptables -I FORWARD -o viso+ -j ACCEPT 2>/dev/null || :
fi

# Handle firewalld if present
if systemctl is-active firewalld >/dev/null 2>&1; then
    echo "🔥 Configuring firewalld for joblet..."
    firewall-cmd --permanent --direct --add-rule ipv4 filter FORWARD 0 -i joblet0 -j ACCEPT 2>/dev/null || :
    firewall-cmd --permanent --direct --add-rule ipv4 filter FORWARD 0 -o joblet0 -j ACCEPT 2>/dev/null || :
    firewall-cmd --permanent --direct --add-rule ipv4 filter FORWARD 0 -i viso+ -j ACCEPT 2>/dev/null || :
    firewall-cmd --permanent --direct --add-rule ipv4 filter FORWARD 0 -o viso+ -j ACCEPT 2>/dev/null || :
    firewall-cmd --permanent --zone=trusted --add-source=172.20.0.0/16 2>/dev/null || :
    firewall-cmd --reload 2>/dev/null || :
fi

# Save iptables rules for persistence
if command -v iptables-save >/dev/null 2>&1; then
    # For RHEL/CentOS 7 and older
    if [ -f /etc/sysconfig/iptables ]; then
        iptables-save > /etc/sysconfig/iptables 2>/dev/null || :
    fi
    # For newer systems with iptables-services
    if systemctl is-enabled iptables.service >/dev/null 2>&1; then
        service iptables save 2>/dev/null || :
    fi
fi

# Auto-detect internal IP
JOBLET_CERT_INTERNAL_IP=$(ip route get 1.1.1.1 2>/dev/null | grep -oP 'src \K[0-9.]+' | head -1)
if [ -z "$JOBLET_CERT_INTERNAL_IP" ]; then
    JOBLET_CERT_INTERNAL_IP=$(ip -4 addr show | grep -oP '(?<=inet\s)\d+(\.\d+){3}' | grep -v '127.0.0.1' | head -1)
fi
JOBLET_CERT_INTERNAL_IP=${JOBLET_CERT_INTERNAL_IP:-127.0.0.1}

# Get configuration from environment or use defaults
JOBLET_SERVER_ADDRESS=${JOBLET_SERVER_ADDRESS:-0.0.0.0}
JOBLET_SERVER_PORT=${JOBLET_SERVER_PORT:-50051}
JOBLET_CERT_EXTERNAL_IP=${JOBLET_CERT_EXTERNAL_IP}
JOBLET_CERT_DOMAIN=${JOBLET_CERT_DOMAIN}

# Legacy support
if [ -n "$JOBLET_SERVER_IP" ]; then
    JOBLET_CERT_PRIMARY="$JOBLET_SERVER_IP"
    JOBLET_CERT_INTERNAL_IP="$JOBLET_SERVER_IP"
fi

# Determine primary IP (prefer internal unless external is explicitly set)
JOBLET_CERT_PRIMARY=${JOBLET_CERT_PRIMARY:-$JOBLET_CERT_INTERNAL_IP}

echo "🔒 Generating embedded certificates..."
echo "   Primary IP: $JOBLET_CERT_PRIMARY"
echo "   Internal IP: $JOBLET_CERT_INTERNAL_IP"
[ -n "$JOBLET_CERT_EXTERNAL_IP" ] && echo "   External IP: $JOBLET_CERT_EXTERNAL_IP"
[ -n "$JOBLET_CERT_DOMAIN" ] && echo "   Domain: $JOBLET_CERT_DOMAIN"

# Generate embedded certificates
cd /opt/joblet
/usr/local/bin/certs_gen_embedded.sh \
    "$JOBLET_CERT_PRIMARY" \
    "$JOBLET_CERT_INTERNAL_IP" \
    ${JOBLET_CERT_EXTERNAL_IP:+"$JOBLET_CERT_EXTERNAL_IP"} \
    ${JOBLET_CERT_DOMAIN:+"$JOBLET_CERT_DOMAIN"} || {
    echo "❌ Failed to generate certificates"
    exit 1
}

# Generate configuration files from templates
sed -e "s|{{SERVER_ADDRESS}}|$JOBLET_SERVER_ADDRESS|g" \
    -e "s|{{SERVER_PORT}}|$JOBLET_SERVER_PORT|g" \
    /opt/joblet/scripts/joblet-config-template.yml > /opt/joblet/config/joblet-config.yml

sed -e "s|{{RNX_SERVER_ADDRESS}}|$JOBLET_CERT_PRIMARY|g" \
    -e "s|{{RNX_SERVER_PORT}}|$JOBLET_SERVER_PORT|g" \
    /opt/joblet/scripts/rnx-config-template.yml > /opt/joblet/config/rnx-config.yml

# Set proper permissions
chmod 600 /opt/joblet/config/*.yml
chmod 600 /opt/joblet/config/*.json

# Enable and start service
%systemd_post joblet.service
systemctl daemon-reload
systemctl enable joblet.service

# Start the service
if systemctl start joblet.service; then
    echo "✅ Joblet service started successfully!"
else
    echo "⚠️  Failed to start Joblet service. Check: journalctl -u joblet"
fi

echo ""
echo "✅ Joblet installation completed!"
echo ""
echo "🔧 Network features enabled:"
echo "   - Default bridge network: joblet0 (172.20.0.0/16)"
echo "   - Isolated network mode available"
echo "   - Custom networks can be created with: rnx network create"
echo ""
echo "📘 Usage:"
echo "   Service status: systemctl status joblet"
echo "   View logs: journalctl -u joblet -f"
echo "   Run a job: rnx run echo 'Hello from Joblet!'"
echo "   With network: rnx run --network=isolated ping google.com"
echo "   Create network: rnx network create mynet --cidr=10.1.0.0/24"
echo ""
echo "📝 Configuration:"
echo "   Server: /opt/joblet/config/joblet-config.yml"
echo "   Client: /opt/joblet/config/rnx-config.yml"
echo "   Reconfigure: rpm -e joblet && JOBLET_SERVER_IP=<new-ip> rpm -i joblet*.rpm"

%preun
%systemd_preun joblet.service

# Clean up network on removal
if [ $1 -eq 0 ]; then
    # Complete removal - clean up network resources
    echo "🧹 Cleaning up joblet network resources..."

    # Stop the service first
    systemctl stop joblet.service 2>/dev/null || :

    # Remove iptables rules
    iptables -t nat -D POSTROUTING -s 172.20.0.0/16 -j MASQUERADE 2>/dev/null || :

    # Remove any isolated network NAT rules
    iptables-save 2>/dev/null | grep "POSTROUTING.*10\.255\.255\.2.*MASQUERADE" | \
        sed 's/-A/-D/' | while read rule; do
        eval "iptables -t nat $rule" 2>/dev/null || :
    done

    # Remove FORWARD rules
    for proto in viso joblet; do
        iptables -S FORWARD 2>/dev/null | grep "$proto" | sed 's/^-A/-D/' | while read rule; do
            eval "iptables $rule" 2>/dev/null || :
        done
    done

    # Remove firewalld rules if present
    if systemctl is-active firewalld >/dev/null 2>&1; then
        firewall-cmd --permanent --direct --remove-rule ipv4 filter FORWARD 0 -i joblet0 -j ACCEPT 2>/dev/null || :
        firewall-cmd --permanent --direct --remove-rule ipv4 filter FORWARD 0 -o joblet0 -j ACCEPT 2>/dev/null || :
        firewall-cmd --permanent --direct --remove-rule ipv4 filter FORWARD 0 -i viso+ -j ACCEPT 2>/dev/null || :
        firewall-cmd --permanent --direct --remove-rule ipv4 filter FORWARD 0 -o viso+ -j ACCEPT 2>/dev/null || :
        firewall-cmd --permanent --zone=trusted --remove-source=172.20.0.0/16 2>/dev/null || :
        firewall-cmd --reload 2>/dev/null || :
    fi

    # Clean up veth interfaces
    for veth in $(ip link show 2>/dev/null | grep -o 'viso[0-9]*' | grep -v '@'); do
        ip link delete $veth 2>/dev/null || :
    done

    # Save cleaned iptables rules
    if command -v iptables-save >/dev/null 2>&1; then
        if [ -f /etc/sysconfig/iptables ]; then
            iptables-save > /etc/sysconfig/iptables 2>/dev/null || :
        fi
        if systemctl is-enabled iptables.service >/dev/null 2>&1; then
            service iptables save 2>/dev/null || :
        fi
    fi
fi

%postun
%systemd_postun_with_restart joblet.service

if [ $1 -eq 0 ]; then
    # Complete removal - clean up everything

    # Remove bridges
    for bridge in $(ip link show type bridge 2>/dev/null | grep -o 'joblet[^ :]*'); do
        ip link delete $bridge 2>/dev/null || :
    done

    # Remove kernel module config
    rm -f /etc/modules-load.d/joblet.conf

    # Remove symlinks
    rm -f /usr/bin/rnx /usr/local/bin/rnx

    # Remove directories
    rm -rf /opt/joblet
    rm -rf /var/lib/joblet
    rm -rf /var/log/joblet

    echo "✅ Joblet service removed successfully!"
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
* $(date '+%a %b %d %Y') Joblet Build System <build@joblet.dev> - CLEAN_VERSION-RELEASE
- Network isolation features (bridge, isolated, custom networks)
- Automatic network setup during installation
- IP forwarding and NAT configuration
- Support for multiple firewall systems
- Enhanced cleanup on removal
EOF

# Replace placeholders in spec file
sed -i "s/CLEAN_VERSION/${CLEAN_VERSION}/g" "$BUILD_DIR/SPECS/${PACKAGE_NAME}.spec"
sed -i "s/RELEASE/${RELEASE}/g" "$BUILD_DIR/SPECS/${PACKAGE_NAME}.spec"

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