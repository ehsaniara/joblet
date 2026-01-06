#!/bin/bash
#
# Joblet EC2 User Data Script
# Automatically installs and configures Joblet on EC2 instance launch
#
# This script:
# - Auto-detects the OS (Ubuntu/Debian vs Amazon Linux/RHEL)
# - Gathers EC2 metadata (instance ID, region, IPs)
# - Downloads and installs the appropriate Joblet package
# - Configures Joblet with EC2-specific settings (uses curl for AWS API calls)
# - Optionally enables CloudWatch Logs backend
# - Starts the Joblet service
#
# Usage:
#   Paste into EC2 User Data field when launching an instance
#   Or reference in Terraform/CloudFormation templates
#
# Command-line options (preferred):
#   --storage=TYPE        Storage backend: cloudwatch (default), s3, local
#   --s3-bucket=NAME      S3 bucket name (required for --storage=s3)
#   --s3-prefix=PREFIX    S3 key prefix (default: joblet)
#   --s3-class=CLASS      S3 storage class (default: STANDARD)
#   --version=VERSION     Joblet version (default: latest)
#   --port=PORT           Server port (default: 443)
#   --help                Show this help
#
# Environment variables (alternative, for backward compatibility):
#   PERSIST_BACKEND, S3_BUCKET, S3_PREFIX, S3_STORAGE_CLASS, JOBLET_VERSION, etc.
#

set -e

# ============================================================================
# Default Configuration Values
# ============================================================================

# JOBLET_HOME defines the installation directory (default: /opt/joblet)
JOBLET_HOME="${JOBLET_HOME:-/opt/joblet}"
export JOBLET_HOME

# Joblet version to install (default: latest)
JOBLET_VERSION="${JOBLET_VERSION:-latest}"

# Storage backend configuration
# PERSIST_BACKEND: "local", "cloudwatch", or "s3" (takes precedence over ENABLE_CLOUDWATCH)
# ENABLE_CLOUDWATCH: Legacy variable - "true" maps to cloudwatch, "false" maps to local
PERSIST_BACKEND="${PERSIST_BACKEND:-}"
ENABLE_CLOUDWATCH="${ENABLE_CLOUDWATCH:-true}"
S3_BUCKET="${S3_BUCKET:-}"
S3_PREFIX="${S3_PREFIX:-joblet}"
S3_STORAGE_CLASS="${S3_STORAGE_CLASS:-STANDARD}"

# Joblet server configuration
JOBLET_SERVER_PORT="${JOBLET_SERVER_PORT:-443}"

# Certificate configuration (optional - will use EC2 IPs if not set)
JOBLET_CERT_DOMAIN="${JOBLET_CERT_DOMAIN:-}"

# Log file for installation
LOG_FILE="/var/log/joblet-install.log"

# ============================================================================
# Command-line Argument Parsing
# ============================================================================

show_help() {
    cat << 'EOF'
Joblet EC2 Installation Script

USAGE:
    ./joblet-install.sh [OPTIONS]

OPTIONS:
    --storage=TYPE        Storage backend type
                          cloudwatch  - AWS CloudWatch Logs (default, recommended)
                          s3          - AWS S3 bucket storage
                          local       - Local filesystem (no AWS)

    --s3-bucket=NAME      S3 bucket name (required when --storage=s3)

    --s3-prefix=PREFIX    S3 key prefix for objects (default: jobs/)

    --s3-class=CLASS      S3 storage class (default: STANDARD)
                          Options: STANDARD, STANDARD_IA, ONEZONE_IA, GLACIER, DEEP_ARCHIVE

    --version=VERSION     Joblet version to install (default: latest)

    --port=PORT           Server port (default: 443)

    --help                Show this help message

EXAMPLES:
    # CloudWatch (default)
    ./joblet-install.sh

    # Explicit CloudWatch
    ./joblet-install.sh --storage=cloudwatch

    # S3 storage
    ./joblet-install.sh --storage=s3 --s3-bucket=my-joblet-logs

    # S3 with custom prefix and storage class
    ./joblet-install.sh --storage=s3 --s3-bucket=my-logs --s3-prefix=prod --s3-class=STANDARD_IA

    # Local storage (no AWS)
    ./joblet-install.sh --storage=local

ENVIRONMENT VARIABLES (alternative to command-line):
    PERSIST_BACKEND      Same as --storage
    S3_BUCKET            Same as --s3-bucket
    S3_PREFIX            Same as --s3-prefix
    S3_STORAGE_CLASS     Same as --s3-class
    JOBLET_VERSION       Same as --version
    JOBLET_SERVER_PORT   Same as --port

Note: Command-line options take precedence over environment variables.
EOF
    exit 0
}

parse_args() {
    while [ $# -gt 0 ]; do
        case "$1" in
            --storage=*)
                PERSIST_BACKEND="${1#*=}"
                ;;
            --s3-bucket=*)
                S3_BUCKET="${1#*=}"
                ;;
            --s3-prefix=*)
                S3_PREFIX="${1#*=}"
                ;;
            --s3-class=*)
                S3_STORAGE_CLASS="${1#*=}"
                ;;
            --version=*)
                JOBLET_VERSION="${1#*=}"
                ;;
            --port=*)
                JOBLET_SERVER_PORT="${1#*=}"
                ;;
            --help|-h)
                show_help
                ;;
            *)
                echo "Unknown option: $1" >&2
                echo "Use --help for usage information" >&2
                exit 1
                ;;
        esac
        shift
    done
}

# Parse command-line arguments (they override environment variables)
parse_args "$@"

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

    # Use IMDSv2 only (required on modern EC2 instances, IMDSv1 is disabled)
    local token=$(curl -s -m 5 -X PUT "http://169.254.169.254/latest/api/token" \
        -H "X-aws-ec2-metadata-token-ttl-seconds: 21600" 2>/dev/null || echo "")

    if [ -z "$token" ]; then
        echo "$default"
        return
    fi

    curl -s -m 5 -H "X-aws-ec2-metadata-token: $token" \
        "http://169.254.169.254/latest/meta-data/$path" 2>/dev/null || echo "$default"
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

configure_storage_backend() {
    CONFIG_FILE="${JOBLET_HOME}/config/joblet-config.yml"

    if [ ! -f "$CONFIG_FILE" ]; then
        log_error "Configuration file not found: $CONFIG_FILE"
        return 1
    fi

    # Determine effective backend (PERSIST_BACKEND takes precedence over ENABLE_CLOUDWATCH)
    local effective_backend=""
    if [ -n "$PERSIST_BACKEND" ]; then
        effective_backend="$PERSIST_BACKEND"
        log "Using PERSIST_BACKEND=$effective_backend"
    elif [ "$ENABLE_CLOUDWATCH" = "true" ]; then
        effective_backend="cloudwatch"
        log "Using ENABLE_CLOUDWATCH=true (legacy) -> cloudwatch backend"
    else
        effective_backend="local"
        log "Using local storage backend"
    fi

    case "$effective_backend" in
        cloudwatch)
            log "Configuring CloudWatch storage backend..."

            # Update persist storage to CloudWatch (handle both quoted and unquoted values)
            sed -i 's/type: "local"/type: "cloudwatch"/' "$CONFIG_FILE"
            sed -i 's/type: local/type: "cloudwatch"/' "$CONFIG_FILE"
            sed -i 's/type: "s3"/type: "cloudwatch"/' "$CONFIG_FILE"
            sed -i 's/type: s3/type: "cloudwatch"/' "$CONFIG_FILE"

            # Set CloudWatch region (required by persist)
            if [ -n "$EC2_REGION" ]; then
                sed -i "s/region: ''/region: '$EC2_REGION'/" "$CONFIG_FILE"
                sed -i "s/region: \"\"/region: '$EC2_REGION'/" "$CONFIG_FILE"
                log_success "Set CloudWatch region: $EC2_REGION"
            fi

            # Update state backend to DynamoDB
            sed -i 's/backend: "memory"/backend: dynamodb/' "$CONFIG_FILE"
            sed -i 's/backend: memory/backend: dynamodb/' "$CONFIG_FILE"

            log_success "Set persist=cloudwatch, state=dynamodb"
            ;;

        s3)
            log "Configuring S3 storage backend..."

            # Validate S3_BUCKET is set
            if [ -z "$S3_BUCKET" ]; then
                log_error "S3_BUCKET is required when PERSIST_BACKEND=s3"
                log_error "Set S3_BUCKET environment variable to your S3 bucket name"
                return 1
            fi

            # Update persist storage to S3 (handle both quoted and unquoted values)
            sed -i 's/type: "local"/type: "s3"/' "$CONFIG_FILE"
            sed -i 's/type: local/type: "s3"/' "$CONFIG_FILE"
            sed -i 's/type: "cloudwatch"/type: "s3"/' "$CONFIG_FILE"
            sed -i 's/type: cloudwatch/type: "s3"/' "$CONFIG_FILE"

            # Set S3 region
            if [ -n "$EC2_REGION" ]; then
                sed -i "s/region: ''/region: '$EC2_REGION'/" "$CONFIG_FILE"
                sed -i "s/region: \"\"/region: '$EC2_REGION'/" "$CONFIG_FILE"
                log_success "Set S3 region: $EC2_REGION"
            fi

            # Set S3 configuration
            # Update bucket (handle both quoted and unquoted empty values)
            sed -i "s/bucket: ''/bucket: '$S3_BUCKET'/" "$CONFIG_FILE"
            sed -i "s/bucket: \"\"/bucket: '$S3_BUCKET'/" "$CONFIG_FILE"

            # Update key_prefix if specified (default in template is "jobs/")
            if [ -n "$S3_PREFIX" ]; then
                sed -i "s|key_prefix: \"jobs/\"|key_prefix: '$S3_PREFIX'|" "$CONFIG_FILE"
                sed -i "s|key_prefix: 'jobs/'|key_prefix: '$S3_PREFIX'|" "$CONFIG_FILE"
                log_success "Set S3 key_prefix: $S3_PREFIX"
            fi

            # Update storage_class if specified (default in template is "STANDARD")
            if [ -n "$S3_STORAGE_CLASS" ] && [ "$S3_STORAGE_CLASS" != "STANDARD" ]; then
                sed -i "s/storage_class: \"STANDARD\"/storage_class: '$S3_STORAGE_CLASS'/" "$CONFIG_FILE"
                sed -i "s/storage_class: 'STANDARD'/storage_class: '$S3_STORAGE_CLASS'/" "$CONFIG_FILE"
                log_success "Set S3 storage_class: $S3_STORAGE_CLASS"
            fi

            log_success "Set S3 bucket: $S3_BUCKET"

            # Update state backend to DynamoDB (S3 also uses DynamoDB for state)
            sed -i 's/backend: "memory"/backend: dynamodb/' "$CONFIG_FILE"
            sed -i 's/backend: memory/backend: dynamodb/' "$CONFIG_FILE"

            log_success "Set persist=s3 (bucket=$S3_BUCKET), state=dynamodb"
            ;;

        local)
            log "Configuring local storage backend..."

            # Update persist storage to local (handle both quoted and unquoted values)
            sed -i 's/type: "cloudwatch"/type: "local"/' "$CONFIG_FILE"
            sed -i 's/type: cloudwatch/type: "local"/' "$CONFIG_FILE"
            sed -i 's/type: "s3"/type: "local"/' "$CONFIG_FILE"
            sed -i 's/type: s3/type: "local"/' "$CONFIG_FILE"

            log "  Logs stored in: ${JOBLET_HOME}/logs/"
            log "  State: in-memory (not persistent across restarts)"
            log_success "Set persist=local, state=memory"
            ;;

        *)
            log_error "Unknown PERSIST_BACKEND: $effective_backend"
            log_error "Valid options: local, cloudwatch, s3"
            return 1
            ;;
    esac

    return 0
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
    # Determine effective backend for display
    local effective_backend=""
    if [ -n "$PERSIST_BACKEND" ]; then
        effective_backend="$PERSIST_BACKEND"
    elif [ "$ENABLE_CLOUDWATCH" = "true" ]; then
        effective_backend="cloudwatch"
    else
        effective_backend="local"
    fi

    log "Storage Backend: $effective_backend"
    case "$effective_backend" in
        cloudwatch)
            log "  Log Storage: AWS CloudWatch Logs"
            log "  Log Group Prefix: /joblet"
            log "  Region: $EC2_REGION"
            log "  State Storage: AWS DynamoDB (joblet-jobs table)"
            log "  View logs: CloudWatch Console → Logs → /joblet"
            ;;
        s3)
            log "  Log Storage: AWS S3"
            log "  Bucket: $S3_BUCKET"
            log "  Key Prefix: ${S3_PREFIX:-jobs/}"
            log "  Storage Class: ${S3_STORAGE_CLASS:-STANDARD}"
            log "  Region: $EC2_REGION"
            log "  State Storage: AWS DynamoDB (joblet-jobs table)"
            log "  View logs: aws s3 ls s3://$S3_BUCKET/${S3_PREFIX:-jobs/}"
            ;;
        local)
            log "  Log Storage: Local filesystem (${JOBLET_HOME}/logs/)"
            log "  State Storage: In-memory (not persistent)"
            ;;
    esac
    log ""
    log "Quick Start:"
    log "  Test connection: rnx job list"
    log "  Run a job: rnx job run echo 'Hello from EC2!'"
    log "  View logs: journalctl -u joblet -f"
    log ""
    log "Client Configuration:"
    log "  1. Copy config from server:"
    log "     scp <EC2_USER>@$EC2_INTERNAL_IP:${JOBLET_HOME}/config/rnx-config.yml ~/.rnx/"
    if [ -n "$EC2_PUBLIC_IP" ]; then
        log "     Or: scp <EC2_USER>@$EC2_PUBLIC_IP:${JOBLET_HOME}/config/rnx-config.yml ~/.rnx/"
    fi
    log "     (Use 'ubuntu' for Ubuntu, 'ec2-user' for Amazon Linux)"
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

    # Configure storage backend (cloudwatch, s3, or local)
    configure_storage_backend

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
