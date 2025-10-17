#!/bin/bash
#
# Joblet EC2 User Data Script
# Automatically installs and configures Joblet on EC2 instance launch
#
# This script:
# - Auto-detects the OS (Ubuntu/Debian vs Amazon Linux/RHEL)
# - Gathers EC2 metadata (instance ID, region, IPs)
# - Downloads and installs the appropriate Joblet package
# - Configures Joblet with EC2-specific settings
# - Optionally enables CloudWatch Logs backend
# - Starts the Joblet service
#
# Usage: Paste this script into EC2 User Data field when launching an instance
#        Or reference it in Terraform/CloudFormation templates
#

set -e

# ============================================================================
# Configuration Variables (can be customized via environment variables)
# ============================================================================

# Joblet version to install (default: latest)
JOBLET_VERSION="${JOBLET_VERSION:-latest}"

# CloudWatch Logs configuration (set to "true" to enable)
ENABLE_CLOUDWATCH="${ENABLE_CLOUDWATCH:-true}"

# Joblet server configuration
JOBLET_SERVER_PORT="${JOBLET_SERVER_PORT:-50051}"

# Certificate configuration (optional - will use EC2 IPs if not set)
JOBLET_CERT_DOMAIN="${JOBLET_CERT_DOMAIN:-}"

# Log file for installation
LOG_FILE="/var/log/joblet-install.log"

# ============================================================================
# Logging Functions
# ============================================================================

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] [INFO] $*" | tee -a "$LOG_FILE"
}

log_error() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] [ERROR] $*" | tee -a "$LOG_FILE" >&2
}

log_success() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] [SUCCESS] $*" | tee -a "$LOG_FILE"
}

# ============================================================================
# EC2 Metadata Functions
# ============================================================================

get_ec2_metadata() {
    local path="$1"
    local default="${2:-}"

    # Use IMDSv2 (more secure)
    local token=$(curl -s -X PUT "http://169.254.169.254/latest/api/token" \
        -H "X-aws-ec2-metadata-token-ttl-seconds: 21600" 2>/dev/null || echo "")

    if [ -n "$token" ]; then
        # IMDSv2
        curl -s -H "X-aws-ec2-metadata-token: $token" \
            "http://169.254.169.254/latest/meta-data/$path" 2>/dev/null || echo "$default"
    else
        # Fallback to IMDSv1
        curl -s "http://169.254.169.254/latest/meta-data/$path" 2>/dev/null || echo "$default"
    fi
}

gather_ec2_info() {
    log "Gathering EC2 instance metadata..."

    EC2_INSTANCE_ID=$(get_ec2_metadata "instance-id")
    EC2_REGION=$(get_ec2_metadata "placement/region")
    EC2_AZ=$(get_ec2_metadata "placement/availability-zone")
    EC2_INTERNAL_IP=$(get_ec2_metadata "local-ipv4" "127.0.0.1")
    EC2_PUBLIC_IP=$(get_ec2_metadata "public-ipv4" "")
    EC2_PUBLIC_DNS=$(get_ec2_metadata "public-hostname" "")
    EC2_INSTANCE_TYPE=$(get_ec2_metadata "instance-type")

    log "EC2 Instance Information:"
    log "  Instance ID: $EC2_INSTANCE_ID"
    log "  Region: $EC2_REGION"
    log "  Availability Zone: $EC2_AZ"
    log "  Instance Type: $EC2_INSTANCE_TYPE"
    log "  Internal IP: $EC2_INTERNAL_IP"
    log "  Public IP: ${EC2_PUBLIC_IP:-none}"
    log "  Public DNS: ${EC2_PUBLIC_DNS:-none}"

    # Create EC2 info file for Joblet installer
    cat > /tmp/joblet-ec2-info << EOF
IS_EC2=true
EC2_INSTANCE_ID="$EC2_INSTANCE_ID"
EC2_REGION="$EC2_REGION"
EC2_AZ="$EC2_AZ"
EC2_INTERNAL_IP="$EC2_INTERNAL_IP"
EC2_PUBLIC_IP="$EC2_PUBLIC_IP"
EC2_PUBLIC_DNS="$EC2_PUBLIC_DNS"
EC2_INSTANCE_TYPE="$EC2_INSTANCE_TYPE"
EOF

    log_success "EC2 metadata gathered successfully"
}

# ============================================================================
# OS Detection
# ============================================================================

detect_os() {
    if [ -f /etc/os-release ]; then
        . /etc/os-release
        OS_ID="$ID"
        OS_VERSION_ID="$VERSION_ID"
        OS_NAME="$NAME"
    else
        log_error "Cannot detect OS - /etc/os-release not found"
        exit 1
    fi

    log "Detected OS: $OS_NAME (ID: $OS_ID, Version: $OS_VERSION_ID)"
}

# ============================================================================
# Package Installation Functions
# ============================================================================

install_debian_ubuntu() {
    log "Installing Joblet on Debian/Ubuntu..."

    # Update package list
    log "Updating package list..."
    apt-get update -y

    # Install dependencies
    log "Installing dependencies..."
    apt-get install -y curl wget gnupg lsb-release

    # Determine architecture
    ARCH=$(dpkg --print-architecture)
    log "Architecture: $ARCH"

    # Download Joblet package
    if [ "$JOBLET_VERSION" = "latest" ]; then
        log "Fetching latest Joblet version..."
        JOBLET_VERSION=$(curl -s https://api.github.com/repos/ehsaniara/joblet/releases/latest | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
        log "Latest version: $JOBLET_VERSION"
    fi

    # Clean version string (remove 'v' prefix)
    CLEAN_VERSION=$(echo "$JOBLET_VERSION" | sed 's/^v//')

    PACKAGE_URL="https://github.com/ehsaniara/joblet/releases/download/${JOBLET_VERSION}/joblet_${CLEAN_VERSION}_${ARCH}.deb"
    PACKAGE_FILE="/tmp/joblet_${CLEAN_VERSION}_${ARCH}.deb"

    log "Downloading Joblet package from: $PACKAGE_URL"
    if ! wget -O "$PACKAGE_FILE" "$PACKAGE_URL"; then
        log_error "Failed to download Joblet package"
        exit 1
    fi

    # Set environment variables for installation
    export JOBLET_SERVER_ADDRESS="0.0.0.0"
    export JOBLET_SERVER_PORT="$JOBLET_SERVER_PORT"
    export JOBLET_CERT_INTERNAL_IP="$EC2_INTERNAL_IP"
    export JOBLET_CERT_PUBLIC_IP="$EC2_PUBLIC_IP"

    # Add EC2 public DNS to certificate domain names (comma-separated)
    if [ -n "$EC2_PUBLIC_DNS" ]; then
        if [ -n "$JOBLET_CERT_DOMAIN" ]; then
            export JOBLET_CERT_DOMAIN="$JOBLET_CERT_DOMAIN,$EC2_PUBLIC_DNS"
        else
            export JOBLET_CERT_DOMAIN="$EC2_PUBLIC_DNS"
        fi
    fi

    export DEBIAN_FRONTEND=noninteractive

    # Install the package
    log "Installing Joblet package..."
    if ! dpkg -i "$PACKAGE_FILE"; then
        log "Fixing dependencies..."
        apt-get install -f -y
    fi

    log_success "Joblet installed successfully"
}

install_redhat_amazon() {
    log "Installing Joblet on RedHat/Amazon Linux..."

    # Determine package manager
    if command -v dnf >/dev/null 2>&1; then
        PKG_MGR="dnf"
    else
        PKG_MGR="yum"
    fi

    log "Using package manager: $PKG_MGR"

    # Install dependencies
    log "Installing dependencies..."
    $PKG_MGR install -y curl wget

    # Determine architecture
    ARCH=$(uname -m)
    if [ "$ARCH" = "x86_64" ]; then
        RPM_ARCH="x86_64"
    elif [ "$ARCH" = "aarch64" ]; then
        RPM_ARCH="aarch64"
    else
        log_error "Unsupported architecture: $ARCH"
        exit 1
    fi

    log "Architecture: $RPM_ARCH"

    # Download Joblet package
    if [ "$JOBLET_VERSION" = "latest" ]; then
        log "Fetching latest Joblet version..."
        JOBLET_VERSION=$(curl -s https://api.github.com/repos/ehsaniara/joblet/releases/latest | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
        log "Latest version: $JOBLET_VERSION"
    fi

    # Clean version string (remove 'v' prefix)
    CLEAN_VERSION=$(echo "$JOBLET_VERSION" | sed 's/^v//')

    PACKAGE_URL="https://github.com/ehsaniara/joblet/releases/download/${JOBLET_VERSION}/joblet-${CLEAN_VERSION}-1.${RPM_ARCH}.rpm"
    PACKAGE_FILE="/tmp/joblet-${CLEAN_VERSION}-1.${RPM_ARCH}.rpm"

    log "Downloading Joblet package from: $PACKAGE_URL"
    if ! wget -O "$PACKAGE_FILE" "$PACKAGE_URL"; then
        log_error "Failed to download Joblet package"
        exit 1
    fi

    # Set environment variables for installation
    export JOBLET_SERVER_ADDRESS="0.0.0.0"
    export JOBLET_SERVER_PORT="$JOBLET_SERVER_PORT"
    export JOBLET_CERT_INTERNAL_IP="$EC2_INTERNAL_IP"
    export JOBLET_CERT_PUBLIC_IP="$EC2_PUBLIC_IP"

    # Add EC2 public DNS to certificate domain names (comma-separated)
    if [ -n "$EC2_PUBLIC_DNS" ]; then
        if [ -n "$JOBLET_CERT_DOMAIN" ]; then
            export JOBLET_CERT_DOMAIN="$JOBLET_CERT_DOMAIN,$EC2_PUBLIC_DNS"
        else
            export JOBLET_CERT_DOMAIN="$EC2_PUBLIC_DNS"
        fi
    fi

    # Install the package
    log "Installing Joblet package..."
    if ! $PKG_MGR localinstall -y "$PACKAGE_FILE"; then
        log_error "Failed to install Joblet package"
        exit 1
    fi

    log_success "Joblet installed successfully"
}

# ============================================================================
# Post-Installation Configuration
# ============================================================================

configure_cloudwatch() {
    if [ "$ENABLE_CLOUDWATCH" != "true" ]; then
        log "CloudWatch Logs backend not enabled (set ENABLE_CLOUDWATCH=true to enable)"
        return 0
    fi

    log "Configuring CloudWatch Logs backend..."

    CONFIG_FILE="/opt/joblet/config/joblet-config.yml"

    if [ ! -f "$CONFIG_FILE" ]; then
        log_error "Configuration file not found: $CONFIG_FILE"
        return 1
    fi

    # Check if persist.storage section exists
    if ! grep -q "persist:" "$CONFIG_FILE"; then
        log "Adding persist configuration section..."
        cat >> "$CONFIG_FILE" << 'EOF'

# Persistence service configuration
persist:
  server:
    grpc_socket: "/opt/joblet/run/persist-grpc.sock"

  ipc:
    socket: "/opt/joblet/run/persist-ipc.sock"

  storage:
    type: "cloudwatch"

    cloudwatch:
      region: ""  # Auto-detect from EC2 metadata
      log_group_prefix: "/joblet"
      log_stream_prefix: "job-"
      metric_namespace: "Joblet/Production"

      # Batch settings
      log_batch_size: 100
      metric_batch_size: 20
EOF
    else
        log "Updating persist.storage configuration..."
        # Use sed to update the storage type
        sed -i 's/type: ".*"/type: "cloudwatch"/' "$CONFIG_FILE" || true
    fi

    log_success "CloudWatch Logs backend configured"
}

start_joblet_service() {
    log "Starting Joblet service..."

    # Reload systemd
    systemctl daemon-reload

    # Start the service
    if systemctl start joblet; then
        log_success "Joblet service started successfully"
    else
        log_error "Failed to start Joblet service"
        systemctl status joblet --no-pager -l
        return 1
    fi

    # Enable service to start on boot
    systemctl enable joblet
    log_success "Joblet service enabled for automatic startup"

    # Wait a moment for service to initialize
    sleep 5

    # Check service status
    if systemctl is-active --quiet joblet; then
        log_success "Joblet service is running"
    else
        log_error "Joblet service is not running"
        systemctl status joblet --no-pager -l
        return 1
    fi
}

verify_installation() {
    log "Verifying Joblet installation..."

    # Check binaries
    if command -v rnx >/dev/null 2>&1; then
        RNX_VERSION=$(rnx --version 2>&1 | head -1 || echo "unknown")
        log_success "rnx CLI installed: $RNX_VERSION"
    else
        log_error "rnx CLI not found in PATH"
        return 1
    fi

    # Check service
    if systemctl is-active --quiet joblet; then
        log_success "Joblet service is active"
    else
        log_error "Joblet service is not active"
        return 1
    fi

    # Check network bridge
    if ip link show joblet0 >/dev/null 2>&1; then
        log_success "Bridge network (joblet0) configured"
    else
        log_error "Bridge network not found"
    fi

    # Test basic connectivity
    if timeout 5 rnx job list >/dev/null 2>&1; then
        log_success "Successfully connected to Joblet server"
    else
        log_error "Cannot connect to Joblet server"
    fi

    log_success "Installation verification completed"
}

display_summary() {
    log ""
    log "=========================================================================="
    log "                    Joblet Installation Complete!"
    log "=========================================================================="
    log ""
    log "Instance Information:"
    log "  Instance ID: $EC2_INSTANCE_ID"
    log "  Region: $EC2_REGION"
    log "  Internal IP: $EC2_INTERNAL_IP"
    log "  Public IP: ${EC2_PUBLIC_IP:-none}"
    if [ -n "$EC2_PUBLIC_DNS" ]; then
        log "  Public DNS: $EC2_PUBLIC_DNS"
    fi
    log ""
    log "Joblet Configuration:"
    log "  Server Address: 0.0.0.0:$JOBLET_SERVER_PORT"
    log "  Certificate includes:"
    log "    - Internal IP: $EC2_INTERNAL_IP"
    if [ -n "$EC2_PUBLIC_IP" ]; then
        log "    - Public IP: $EC2_PUBLIC_IP"
    fi
    if [ -n "$EC2_PUBLIC_DNS" ]; then
        log "    - Public DNS: $EC2_PUBLIC_DNS"
    fi
    if [ -n "$JOBLET_CERT_DOMAIN" ] && [ "$JOBLET_CERT_DOMAIN" != "$EC2_PUBLIC_DNS" ]; then
        log "    - Custom Domain: $JOBLET_CERT_DOMAIN"
    fi
    log ""
    if [ "$ENABLE_CLOUDWATCH" = "true" ]; then
        log "CloudWatch Logs:"
        log "  Enabled: Yes"
        log "  Log Group Prefix: /joblet"
        log "  Region: $EC2_REGION (auto-detected)"
        log "  View logs: CloudWatch Console → Logs → /joblet"
        log ""
    fi
    log "Quick Start:"
    log "  Test connection: rnx job list"
    log "  Run a job: rnx job run echo 'Hello from EC2!'"
    log "  View logs: journalctl -u joblet -f"
    log ""
    log "Client Configuration:"
    log "  1. Copy config from server:"
    log "     scp root@$EC2_INTERNAL_IP:/opt/joblet/config/rnx-config.yml ~/.rnx/"
    if [ -n "$EC2_PUBLIC_IP" ]; then
        log "     Or: scp root@$EC2_PUBLIC_IP:/opt/joblet/config/rnx-config.yml ~/.rnx/"
    fi
    log ""
    log "  2. The certificate includes these connection options:"
    log "     - Internal IP: $EC2_INTERNAL_IP:$JOBLET_SERVER_PORT"
    if [ -n "$EC2_PUBLIC_IP" ]; then
        log "     - Public IP: $EC2_PUBLIC_IP:$JOBLET_SERVER_PORT"
    fi
    if [ -n "$EC2_PUBLIC_DNS" ]; then
        log "     - Public DNS: $EC2_PUBLIC_DNS:$JOBLET_SERVER_PORT"
    fi
    log ""
    log "  3. Configure multiple nodes in ~/.rnx/rnx-config.yml if needed:"
    log "     - Edit 'address' field to use different IPs/DNS"
    log "     - Use with: rnx --node=<node_name> job list"
    log ""
    log "=========================================================================="
}

# ============================================================================
# Main Installation Flow
# ============================================================================

main() {
    log "=========================================================================="
    log "        Joblet EC2 Auto-Installation Starting"
    log "=========================================================================="
    log ""

    # Gather EC2 metadata
    gather_ec2_info

    # Detect OS
    detect_os

    # Install based on OS
    case "$OS_ID" in
        ubuntu|debian)
            install_debian_ubuntu
            ;;
        amzn|rhel|centos|fedora)
            install_redhat_amazon
            ;;
        *)
            log_error "Unsupported OS: $OS_ID"
            exit 1
            ;;
    esac

    # Configure CloudWatch if enabled
    configure_cloudwatch

    # Start Joblet service
    start_joblet_service

    # Verify installation
    verify_installation

    # Display summary
    display_summary

    log_success "Joblet EC2 auto-installation completed successfully!"
}

# Run main installation
main 2>&1 | tee -a "$LOG_FILE"
