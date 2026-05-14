#!/bin/bash
# Common installation functions for Joblet
# Used by both Debian (.deb) and RPM (.rpm) packages

# JOBLET_HOME defines the installation directory (default: /opt/joblet)
JOBLET_HOME="${JOBLET_HOME:-/opt/joblet}"
export JOBLET_HOME

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

print_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

print_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

print_error() {
    echo -e "${RED}❌ $1${NC}"
}

# Detect Linux distribution and return the appropriate runtime config filename
# Returns: ubuntu, rhel, fedora, or alpine
detect_linux_distro() {
    local distro="ubuntu"  # Default to Ubuntu

    if [ -f /etc/os-release ]; then
        . /etc/os-release

        case "$ID" in
            ubuntu|debian|linuxmint|pop)
                distro="ubuntu"
                ;;
            rhel|centos|rocky|almalinux|ol|scientific)
                # Check if it's a newer version that uses dnf
                if [ -f /etc/dnf/dnf.conf ] || command -v dnf >/dev/null 2>&1; then
                    distro="fedora"  # dnf-based
                else
                    distro="rhel"    # yum-based
                fi
                ;;
            fedora|amzn)
                # Amazon Linux 2023+ uses dnf, Amazon Linux 2 uses yum
                if [ "$ID" = "amzn" ] && [ "${VERSION_ID%%.*}" -lt 2023 ] 2>/dev/null; then
                    distro="rhel"    # Amazon Linux 2 uses yum
                else
                    distro="fedora"  # Fedora and Amazon Linux 2023+ use dnf
                fi
                ;;
            alpine)
                distro="alpine"
                ;;
            *)
                # Try to detect by package manager
                if command -v apt-get >/dev/null 2>&1; then
                    distro="ubuntu"
                elif command -v dnf >/dev/null 2>&1; then
                    distro="fedora"
                elif command -v yum >/dev/null 2>&1; then
                    distro="rhel"
                elif command -v apk >/dev/null 2>&1; then
                    distro="alpine"
                fi
                ;;
        esac
    else
        # Fallback detection by package manager
        if command -v apt-get >/dev/null 2>&1; then
            distro="ubuntu"
        elif command -v dnf >/dev/null 2>&1; then
            distro="fedora"
        elif command -v yum >/dev/null 2>&1; then
            distro="rhel"
        elif command -v apk >/dev/null 2>&1; then
            distro="alpine"
        fi
    fi

    echo "$distro"
}

# Select and copy the appropriate runtime config for this distro
# This function should be called during package installation
select_runtime_config() {
    local scripts_dir="${1:-${JOBLET_HOME}/scripts}"
    local config_dir="${2:-${JOBLET_HOME}/config}"

    local distro=$(detect_linux_distro)
    local runtime_config_src="${scripts_dir}/runtime-config-${distro}.yml"
    local runtime_config_dst="${config_dir}/runtime-config.yml"

    print_info "Detected Linux distribution: ${distro}"

    if [ -f "$runtime_config_src" ]; then
        cp "$runtime_config_src" "$runtime_config_dst"
        chmod 644 "$runtime_config_dst"
        print_success "Installed runtime config for ${distro}: ${runtime_config_dst}"
    else
        print_warning "Runtime config not found: ${runtime_config_src}"
        print_warning "Using built-in defaults for runtime configuration"
    fi
}

detect_internal_ip() {
    local ip=$(ip route get 1.1.1.1 2>/dev/null | grep -oP 'src \K[0-9.]+' | head -1)
    if [ -z "$ip" ]; then
        ip=$(ip -4 addr show | grep -oP '(?<=inet\s)\d+(\.\d+){3}' | grep -v '127.0.0.1' | head -1)
    fi
    echo "${ip:-127.0.0.1}"
}

detect_firewall_backend() {
    # Detect whether system uses nftables, iptables, or firewalld
    if command -v firewall-cmd >/dev/null 2>&1 && systemctl is-active firewalld >/dev/null 2>&1; then
        echo "firewalld"
    elif command -v nft >/dev/null 2>&1 && nft list tables 2>/dev/null | grep -q .; then
        echo "nftables"
    elif command -v iptables >/dev/null 2>&1; then
        echo "iptables"
    else
        echo "none"
    fi
}

check_network_conflicts() {
    local bridge_network="172.20.0.0/16"
    local bridge_ip="172.20.0.1"

    print_info "Checking for network conflicts..."

    # Check if the network range is already in use
    if ip route | grep -q "172.20."; then
        local conflicting_route=$(ip route | grep "172.20." | head -1)
        print_error "Network conflict detected!"
        print_error "The 172.20.0.0/16 range is already in use: $conflicting_route"
        print_warning "Joblet requires 172.20.0.0/16 for job isolation"
        print_warning "Please remove conflicting network configuration or modify ${JOBLET_HOME}/config/joblet-config.yml"
        print_warning "Continuing anyway, but network isolation may not work correctly..."
        return 1
    fi

    # Check if bridge already exists
    if ip link show joblet0 >/dev/null 2>&1; then
        print_warning "Bridge joblet0 already exists, will reuse it"
        return 0
    fi

    print_success "No network conflicts detected"
    return 0
}

setup_network_requirements() {
    print_warning "⚠️  SYSTEM-WIDE NETWORK CHANGES ⚠️"
    echo "  This installation will modify your system networking:"
    echo "  • Enable IP forwarding (permanent)"
    echo "  • Load kernel modules: br_netfilter, nf_conntrack, nf_nat"
    echo "  • Create bridge network: joblet0 (172.20.0.0/16)"
    echo "  • Add firewall NAT rules for job networking"
    echo ""

    # Check for conflicts first
    check_network_conflicts || true

    print_info "Setting up network requirements for joblet..."

    # Enable IP forwarding
    sysctl -w net.ipv4.ip_forward=1 >/dev/null 2>&1 || true

    # Make IP forwarding permanent (different for RPM vs Debian)
    if [ -d /etc/sysctl.d ]; then
        # Modern systems (systemd)
        echo "net.ipv4.ip_forward = 1" > /etc/sysctl.d/99-joblet.conf
        print_success "Enabled IP forwarding (persistent in /etc/sysctl.d/99-joblet.conf)"
    else
        # Older systems
        if ! grep -q "^net.ipv4.ip_forward = 1" /etc/sysctl.conf 2>/dev/null; then
            echo "net.ipv4.ip_forward = 1" >> /etc/sysctl.conf
            print_success "Enabled IP forwarding (persistent in /etc/sysctl.conf)"
        fi
    fi

    # Load required kernel modules
    for module in br_netfilter nf_conntrack nf_nat; do
        if modprobe $module 2>/dev/null; then
            print_success "Loaded kernel module: $module"
        fi
    done

    # Ensure modules load on boot
    # RPM uses /etc/modules-load.d/, Debian uses /etc/modules
    if [ -d /etc/modules-load.d ]; then
        # Modern systems with systemd
        cat > /etc/modules-load.d/joblet.conf << 'EOF'
# Load modules required for Joblet network isolation
br_netfilter
nf_conntrack
nf_nat
EOF
        print_success "Configured module auto-loading (systemd)"
    else
        # Debian systems with /etc/modules
        for module in br_netfilter nf_conntrack nf_nat; do
            if ! grep -q "^$module$" /etc/modules 2>/dev/null; then
                echo "$module" >> /etc/modules
            fi
        done
        print_success "Configured module auto-loading (/etc/modules)"
    fi

    # Create state directory for network configs
    mkdir -p /var/lib/joblet
    chown root:root /var/lib/joblet
    chmod 755 /var/lib/joblet

    # Setup default bridge if it doesn't exist
    if ! ip link show joblet0 >/dev/null 2>&1; then
        if ip link add joblet0 type bridge 2>/dev/null && \
           ip addr add 172.20.0.1/16 dev joblet0 2>/dev/null && \
           ip link set joblet0 up 2>/dev/null; then
            print_success "Created bridge network: joblet0 (172.20.0.0/16)"
        else
            print_error "Failed to create bridge network"
            print_warning "Job networking may not work correctly"
        fi
    fi

    # Detect and configure firewall backend
    FIREWALL_BACKEND=$(detect_firewall_backend)
    print_info "Detected firewall backend: $FIREWALL_BACKEND"

    case "$FIREWALL_BACKEND" in
        firewalld)
            # Configure firewalld (RHEL/CentOS/Fedora)
            print_info "Configuring firewalld rules for joblet..."

            # Enable masquerading for NAT
            firewall-cmd --permanent --add-masquerade 2>/dev/null || true

            # Add rich rules for joblet traffic
            firewall-cmd --permanent --direct --add-rule ipv4 nat POSTROUTING 0 -s 172.20.0.0/16 -j MASQUERADE 2>/dev/null || true
            firewall-cmd --permanent --direct --add-rule ipv4 filter FORWARD 0 -i joblet0 -j ACCEPT 2>/dev/null || true
            firewall-cmd --permanent --direct --add-rule ipv4 filter FORWARD 0 -o joblet0 -j ACCEPT 2>/dev/null || true
            firewall-cmd --permanent --direct --add-rule ipv4 filter FORWARD 0 -i viso+ -j ACCEPT 2>/dev/null || true
            firewall-cmd --permanent --direct --add-rule ipv4 filter FORWARD 0 -o viso+ -j ACCEPT 2>/dev/null || true

            # Reload to apply
            firewall-cmd --reload 2>/dev/null || true
            print_success "Configured firewalld rules"
            ;;

        nftables)
            # Configure nftables (modern Debian/Ubuntu)
            if ! nft list table inet joblet >/dev/null 2>&1; then
                print_info "Configuring nftables rules for joblet..."
                nft add table inet joblet 2>/dev/null || true
                nft add chain inet joblet postrouting { type nat hook postrouting priority 100 \; } 2>/dev/null || true
                nft add rule inet joblet postrouting ip saddr 172.20.0.0/16 masquerade 2>/dev/null || true

                nft add chain inet joblet forward { type filter hook forward priority 0 \; } 2>/dev/null || true
                nft add rule inet joblet forward iifname "joblet0" accept 2>/dev/null || true
                nft add rule inet joblet forward oifname "joblet0" accept 2>/dev/null || true
                nft add rule inet joblet forward iifname "viso*" accept 2>/dev/null || true
                nft add rule inet joblet forward oifname "viso*" accept 2>/dev/null || true

                print_success "Configured nftables rules"

                # Make rules persistent if nftables.conf exists
                if [ -f /etc/nftables.conf ]; then
                    if ! grep -q "table inet joblet" /etc/nftables.conf 2>/dev/null; then
                        nft list table inet joblet >> /etc/nftables.conf 2>/dev/null || true
                        print_success "Made nftables rules persistent"
                    fi
                fi
            else
                print_info "nftables rules already configured"
            fi
            ;;

        iptables)
            # Configure iptables (older systems)
            # Configure iptables for NAT (idempotent)
            if ! iptables -t nat -C POSTROUTING -s 172.20.0.0/16 -j MASQUERADE 2>/dev/null; then
                iptables -t nat -A POSTROUTING -s 172.20.0.0/16 -j MASQUERADE
                print_success "Configured iptables NAT rule"
            fi

            # Check and configure FORWARD chain
            if iptables -L FORWARD -n | head -1 | grep -q "policy DROP"; then
                print_warning "iptables FORWARD policy is DROP. Adding ACCEPT rules for joblet..."
                iptables -I FORWARD -i joblet0 -j ACCEPT 2>/dev/null || true
                iptables -I FORWARD -o joblet0 -j ACCEPT 2>/dev/null || true
                iptables -I FORWARD -i viso+ -j ACCEPT 2>/dev/null || true
                iptables -I FORWARD -o viso+ -j ACCEPT 2>/dev/null || true
                print_success "Added iptables FORWARD rules"
            fi

            # Save iptables rules if iptables-persistent is installed
            if command -v netfilter-persistent >/dev/null 2>&1; then
                netfilter-persistent save >/dev/null 2>&1 || true
                print_success "Saved iptables rules (persistent)"
            elif [ -d /etc/iptables ]; then
                iptables-save > /etc/iptables/rules.v4 2>/dev/null || true
                print_success "Saved iptables rules"
            elif [ -d /etc/sysconfig ]; then
                # RHEL/CentOS style
                service iptables save 2>/dev/null || true
                print_success "Saved iptables rules (sysconfig)"
            fi
            ;;

        *)
            print_error "No firewall backend detected (iptables, nftables, or firewalld required)"
            print_warning "Network isolation may not work correctly"
            ;;
    esac

    print_success "Network requirements configured"
}

get_configuration() {
    # Configuration precedence (highest to lowest):
    # 1. Environment variables (for automated deployments)
    # 2. Auto-detection (fallback)

    print_info "Loading configuration..."

    # === Priority 1: Environment Variables ===
    # Check for environment variables first (highest priority)
    if [ -n "$JOBLET_SERVER_ADDRESS" ] || [ -n "$JOBLET_CERT_INTERNAL_IP" ]; then
        print_info "Configuration source: Environment variables"

        # Standard environment variables
        JOBLET_SERVER_ADDRESS="${JOBLET_SERVER_ADDRESS:-0.0.0.0}"
        JOBLET_SERVER_PORT="${JOBLET_SERVER_PORT:-50051}"
        # JOBLET_CERT_INTERNAL_IP, JOBLET_CERT_PUBLIC_IP, JOBLET_CERT_DOMAIN
        # are used directly if set

    # === Priority 2: Defaults ===
    else
        print_info "Configuration source: Defaults with auto-detection"
        JOBLET_SERVER_ADDRESS="0.0.0.0"
        JOBLET_SERVER_PORT="50051"
    fi

    # === Auto-detect internal IP if not set (all paths) ===
    if [ -z "$JOBLET_CERT_INTERNAL_IP" ]; then
        JOBLET_CERT_INTERNAL_IP=$(detect_internal_ip)
        print_info "Auto-detected internal IP: $JOBLET_CERT_INTERNAL_IP"
    fi

    # === Set primary certificate address (used for CN) ===
    JOBLET_CERT_PRIMARY=${JOBLET_CERT_PRIMARY:-$JOBLET_CERT_INTERNAL_IP}

    # === Build SAN list for certificate ===
    if [ -z "$JOBLET_ADDITIONAL_NAMES" ]; then
        JOBLET_ADDITIONAL_NAMES="localhost"

        # Add internal IP if different from primary
        if [ -n "$JOBLET_CERT_INTERNAL_IP" ] && [ "$JOBLET_CERT_INTERNAL_IP" != "$JOBLET_CERT_PRIMARY" ]; then
            JOBLET_ADDITIONAL_NAMES="$JOBLET_ADDITIONAL_NAMES,$JOBLET_CERT_INTERNAL_IP"
        fi

        # Add public IP if configured
        if [ -n "$JOBLET_CERT_PUBLIC_IP" ]; then
            JOBLET_ADDITIONAL_NAMES="$JOBLET_ADDITIONAL_NAMES,$JOBLET_CERT_PUBLIC_IP"
        fi

        # Add domain(s) if configured
        if [ -n "$JOBLET_CERT_DOMAIN" ]; then
            JOBLET_ADDITIONAL_NAMES="$JOBLET_ADDITIONAL_NAMES,$JOBLET_CERT_DOMAIN"
        fi
    fi

    print_success "Configuration loaded successfully"
}

detect_aws_environment() {
    # Detect if running on AWS EC2 and load configuration
    # Sets: EC2_INFO, EC2_CLOUDWATCH_CONFIGURED, EC2_DYNAMODB_CONFIGURED, EC2_INSTANCE_ID, EC2_REGION

    EC2_INFO=""
    EC2_CLOUDWATCH_CONFIGURED=false
    EC2_DYNAMODB_CONFIGURED=false

    if [ -f /tmp/joblet-ec2-info ]; then
        source /tmp/joblet-ec2-info
        if [ "$IS_EC2" = "true" ]; then
            EC2_INFO=" (AWS EC2 Instance)"
            EC2_CLOUDWATCH_CONFIGURED=true
            EC2_DYNAMODB_CONFIGURED=true

            print_info "🌩️  AWS EC2 Environment Detected"
            if [ -n "$EC2_INSTANCE_ID" ]; then
                echo "  Instance ID: $EC2_INSTANCE_ID"
            fi
            if [ -n "$EC2_REGION" ]; then
                echo "  Region: $EC2_REGION"
            fi
            echo ""
            print_info "📊  CloudWatch Logs backend will be enabled"
            echo "  Log storage: AWS CloudWatch Logs"
            echo "  Log group format: /joblet/{nodeId}/jobs/{jobId}"
            echo ""
            print_info "💾  DynamoDB State Persistence will be enabled"
            echo "  State storage: AWS DynamoDB"
            echo "  Table: joblet-jobs"
            echo "  Features: Job state survives restarts, auto-cleanup with TTL"
            echo ""
            print_warning "📋  Required IAM Permissions:"
            echo "  Ensure EC2 instance has IAM role with permissions:"
            echo "    • CloudWatch Logs: logs:CreateLogGroup, logs:CreateLogStream, logs:PutLogEvents"
            echo "    • DynamoDB: dynamodb:CreateTable, dynamodb:PutItem, dynamodb:GetItem,"
            echo "                dynamodb:UpdateItem, dynamodb:DeleteItem, dynamodb:Scan, dynamodb:Query"
            echo "                dynamodb:DescribeTable, dynamodb:UpdateTimeToLive"
            echo ""

            print_info "💾  DynamoDB State Persistence:"
            echo "  Table 'joblet-jobs' should be created via pre-setup.sh script"
            echo "  If not created, Joblet will use in-memory state (not persistent)"
            echo ""

            return 0
        fi
    fi

    # Not running on EC2 - this is not an error, just detection
    return 0
}

display_aws_quickstart() {
    # Display AWS-specific quickstart information based on configured backend
    # Check PERSIST_BACKEND first, then fall back to legacy detection

    local effective_backend="local"
    if [ -n "$PERSIST_BACKEND" ]; then
        effective_backend="$PERSIST_BACKEND"
    elif [ "$EC2_CLOUDWATCH_CONFIGURED" = "true" ]; then
        effective_backend="cloudwatch"
    fi

    case "$effective_backend" in
        cloudwatch)
            echo ""
            print_info "🌩️  AWS CloudWatch Logs Configuration:"
            echo "  View logs: AWS Console → CloudWatch → Logs → /joblet"
            echo "  Query logs: aws logs filter-log-events --log-group-name-prefix '/joblet/'"
            echo "  Config file: ${JOBLET_HOME}/config/joblet-config.yml"
            echo "  Storage type: persist.storage.type = cloudwatch"
            echo "  Documentation: https://docs.aws.amazon.com/cloudwatch/"
            echo ""
            ;;
        s3)
            echo ""
            print_info "📦  AWS S3 Storage Configuration:"
            echo "  Bucket: ${S3_BUCKET:-<not set>}"
            echo "  Key Prefix: ${S3_PREFIX:-jobs/}"
            echo "  Storage Class: ${S3_STORAGE_CLASS:-STANDARD}"
            echo "  View logs: AWS Console → S3 → ${S3_BUCKET:-<bucket>} → ${S3_PREFIX:-jobs/}"
            echo "  List objects: aws s3 ls s3://${S3_BUCKET:-<bucket>}/${S3_PREFIX:-jobs/}"
            echo "  Config file: ${JOBLET_HOME}/config/joblet-config.yml"
            echo "  Storage type: persist.storage.type = s3"
            echo ""
            print_info "💡  S3 Tips:"
            echo "  • Configure lifecycle rules on S3 bucket for automatic archival/deletion"
            echo "  • Use GLACIER or DEEP_ARCHIVE storage class for long-term retention"
            echo "  • Logs are stored as gzipped JSONL files"
            echo ""
            ;;
    esac

    # DynamoDB is used by both cloudwatch and s3 backends
    if [ "$effective_backend" = "cloudwatch" ] || [ "$effective_backend" = "s3" ]; then
        echo ""
        print_info "💾  AWS DynamoDB State Persistence Configuration:"
        echo "  Table: joblet-jobs"
        echo "  View jobs: AWS Console → DynamoDB → Tables → joblet-jobs"
        echo "  Query jobs: aws dynamodb scan --table-name joblet-jobs --region ${EC2_REGION:-us-east-1}"
        echo "  Config file: ${JOBLET_HOME}/config/joblet-config.yml"
        echo "  Backend type: state.backend = dynamodb"
        echo "  Features:"
        echo "    • Job state persists across restarts"
        echo "    • Auto-cleanup with TTL (30 days for completed jobs)"
        echo "    • Pay-per-request billing (< $0.05/month for 100 jobs/day)"
        echo "  Documentation: https://docs.aws.amazon.com/dynamodb/"
        echo ""
    fi
}

configure_storage_backends() {
    # Configure storage backends based on detected environment and user preferences
    # Must be called after detect_aws_environment() and after config file exists
    #
    # Environment variables:
    #   PERSIST_BACKEND: "local", "cloudwatch", or "s3" (takes precedence)
    #   ENABLE_CLOUDWATCH: Legacy - "true" maps to cloudwatch (for backward compatibility)
    #   S3_BUCKET: Required when PERSIST_BACKEND=s3
    #   S3_PREFIX: Optional S3 key prefix (default: "joblet")
    #   S3_STORAGE_CLASS: Optional storage class (default: "STANDARD")

    CONFIG_FILE="${JOBLET_HOME}/config/joblet-config.yml"

    if [ ! -f "$CONFIG_FILE" ]; then
        return 1
    fi

    # Determine effective backend
    # Priority: PERSIST_BACKEND > EC2_CLOUDWATCH_CONFIGURED/ENABLE_CLOUDWATCH > local
    local effective_backend="local"

    if [ -n "$PERSIST_BACKEND" ]; then
        effective_backend="$PERSIST_BACKEND"
        print_info "Using PERSIST_BACKEND=$effective_backend"
    elif [ "$EC2_CLOUDWATCH_CONFIGURED" = "true" ]; then
        effective_backend="cloudwatch"
        print_info "EC2 detected with CloudWatch enabled"
    fi

    case "$effective_backend" in
        cloudwatch)
            print_info "Configuring CloudWatch storage backend..."

            # Update persist storage to CloudWatch
            sed -i 's/type: "local"/type: "cloudwatch"/' "$CONFIG_FILE"

            # Set CloudWatch region (required by persist)
            if [ -n "$EC2_REGION" ]; then
                sed -i "s/region: \"\"/region: \"$EC2_REGION\"/" "$CONFIG_FILE"
                print_success "Set CloudWatch region: $EC2_REGION"
            fi

            # Update state backend to DynamoDB
            sed -i 's/backend: "memory"/backend: "dynamodb"/' "$CONFIG_FILE"

            print_success "Set persist=cloudwatch, state=dynamodb"
            ;;

        s3)
            print_info "Configuring S3 storage backend..."

            # Validate S3_BUCKET is set
            if [ -z "$S3_BUCKET" ]; then
                print_error "S3_BUCKET is required when PERSIST_BACKEND=s3"
                print_error "Set S3_BUCKET environment variable to your S3 bucket name"
                return 1
            fi

            # Update persist storage to S3
            sed -i 's/type: "local"/type: "s3"/' "$CONFIG_FILE"

            # Set S3 region
            if [ -n "$EC2_REGION" ]; then
                sed -i "s/region: \"\"/region: \"$EC2_REGION\"/" "$CONFIG_FILE"
                print_success "Set S3 region: $EC2_REGION"
            fi

            # Set S3 bucket
            sed -i "s/bucket: \"\"/bucket: \"$S3_BUCKET\"/" "$CONFIG_FILE"
            print_success "Set S3 bucket: $S3_BUCKET"

            # Set S3 key prefix (optional)
            if [ -n "$S3_PREFIX" ]; then
                sed -i "s|key_prefix: \"jobs/\"|key_prefix: \"$S3_PREFIX\"|" "$CONFIG_FILE"
                print_success "Set S3 key_prefix: $S3_PREFIX"
            fi

            # Set S3 storage class (optional)
            if [ -n "$S3_STORAGE_CLASS" ]; then
                sed -i "s/storage_class: \"STANDARD\"/storage_class: \"$S3_STORAGE_CLASS\"/" "$CONFIG_FILE"
                print_success "Set S3 storage class: $S3_STORAGE_CLASS"
            fi

            # Update state backend to DynamoDB (S3 also uses DynamoDB for state)
            sed -i 's/backend: "memory"/backend: "dynamodb"/' "$CONFIG_FILE"

            print_success "Set persist=s3 (bucket=$S3_BUCKET), state=dynamodb"
            ;;

        local)
            print_info "Using local storage backend"
            print_info "  Logs stored in: ${JOBLET_HOME}/logs/"
            print_info "  State: in-memory (not persistent across restarts)"
            # Config already defaults to local, no changes needed
            ;;

        *)
            print_error "Unknown PERSIST_BACKEND: $effective_backend"
            print_error "Valid options: local, cloudwatch, s3"
            return 1
            ;;
    esac

    return 0
}

generate_and_embed_certificates() {
    print_info "Generating certificates with configured IPs and domains..."

    # Export variables for the certificate generation script
    export JOBLET_SERVER_ADDRESS="$JOBLET_CERT_PRIMARY"  # Primary address for certificate CN
    export JOBLET_ADDITIONAL_NAMES="$JOBLET_ADDITIONAL_NAMES"
    export JOBLET_MODE="package-install"

    # Show what will be in the certificate
    print_info "Certificate will be valid for:"
    echo "  Primary: $JOBLET_CERT_PRIMARY"
    if [ -n "$JOBLET_ADDITIONAL_NAMES" ]; then
        echo "  Additional: $JOBLET_ADDITIONAL_NAMES"
    fi

    # Determine which certificate generation script to use
    # On EC2 with Secrets Manager available, use secretsmanager version to fetch shared CA/client certs
    CERT_SCRIPT="/usr/local/bin/certs_gen_embedded.sh"

    # Detect EC2 via metadata service if not already set (supports IMDSv2)
    if [ "$IS_EC2" != "true" ]; then
        # Try IMDSv2 (required on modern instances)
        IMDS_TOKEN=$(curl -s -m 2 -X PUT "http://169.254.169.254/latest/api/token" -H "X-aws-ec2-metadata-token-ttl-seconds: 60" 2>/dev/null)
        if [ -n "$IMDS_TOKEN" ]; then
            if curl -s -m 2 -H "X-aws-ec2-metadata-token: $IMDS_TOKEN" http://169.254.169.254/latest/meta-data/instance-id >/dev/null 2>&1; then
                IS_EC2="true"
                # Also get the region if not set
                if [ -z "$EC2_REGION" ]; then
                    EC2_REGION=$(curl -s -m 2 -H "X-aws-ec2-metadata-token: $IMDS_TOKEN" http://169.254.169.254/latest/meta-data/placement/region 2>/dev/null)
                fi
            fi
        fi
    fi

    if [ "$IS_EC2" = "true" ] && [ -x /usr/local/bin/certs_gen_with_secretsmanager.sh ]; then
        print_info "EC2 detected - using Secrets Manager for shared CA/client certificates"
        print_info "EC2 Region: $EC2_REGION"
        CERT_SCRIPT="/usr/local/bin/certs_gen_with_secretsmanager.sh"
        # Export for the certificate generation script subprocess
        export IS_EC2
        export EC2_REGION
    fi

    # Run the certificate generation script
    if [ -x "$CERT_SCRIPT" ]; then
        if "$CERT_SCRIPT"; then
            print_success "Certificates generated successfully"

            # Update the server configuration with the actual bind address and port
            if [ -f ${JOBLET_HOME}/config/joblet-config.yml ]; then
                # Update server bind address and port in the config
                sed -i "s/^  address:.*/  address: \"$JOBLET_SERVER_ADDRESS\"/" ${JOBLET_HOME}/config/joblet-config.yml
                sed -i "s/^  port:.*/  port: $JOBLET_SERVER_PORT/" ${JOBLET_HOME}/config/joblet-config.yml
                print_success "Updated server configuration: $JOBLET_SERVER_ADDRESS:$JOBLET_SERVER_PORT"
            fi

            # Update client configuration files with all valid connection endpoints
            if [ -f ${JOBLET_HOME}/config/rnx-config.yml ]; then
                # For each node in the client config, we need to update the address
                # The address in rnx-config.yml should be how clients connect, not the bind address
                # Use the certificate primary address as it's what clients should connect to
                sed -i "s/address: \"[^:]*:50051\"/address: \"$JOBLET_CERT_PRIMARY:$JOBLET_SERVER_PORT\"/" ${JOBLET_HOME}/config/rnx-config.yml
                print_success "Updated client configuration with connection endpoint: $JOBLET_CERT_PRIMARY:$JOBLET_SERVER_PORT"
            fi

            return 0
        else
            print_error "Certificate generation failed"
            return 1
        fi
    else
        print_error "Certificate generation script not found or not executable"
        return 1
    fi
}

display_system_changes_warning() {
    echo ""
    echo "=========================================================================="
    print_warning "⚠️  IMPORTANT: SYSTEM MODIFICATIONS AND SECURITY NOTICE ⚠️"
    echo "=========================================================================="
    echo ""
    echo "This installation will make the following PERMANENT system changes:"
    echo ""
    echo "📡 NETWORK MODIFICATIONS:"
    echo "   • IP forwarding will be ENABLED in /etc/sysctl.d/99-joblet.conf"
    echo "   • Bridge network 'joblet0' will be created (172.20.0.0/16)"
    echo "   • Firewall rules will be added for NAT and forwarding"
    echo "   • Kernel modules will be loaded: br_netfilter, nf_conntrack, nf_nat"
    echo ""
    echo "🔐 SECURITY CONSIDERATIONS:"
    echo "   • Joblet service runs as ROOT (required for namespaces/cgroups)"
    echo "   • TLS certificates with private keys will be embedded in config files"
    echo "   • Config files will be stored in ${JOBLET_HOME}/config/ (chmod 600)"
    echo "   • Network isolation uses Linux namespaces and bridge networking"
    echo ""
    echo "📁 FILES AND DIRECTORIES CREATED:"
    echo "   • ${JOBLET_HOME}/                 - Main installation directory"
    echo "   • ${JOBLET_HOME}/config/          - Configuration and certificates"
    echo "   • ${JOBLET_HOME}/logs/            - Job logs and output"
    echo "   • ${JOBLET_HOME}/volumes/         - Persistent job volumes"
    echo "   • /var/log/joblet/             - System logs"
    echo "   • /etc/systemd/system/joblet.service - Systemd service"
    echo ""
    echo "🔄 TO FULLY REMOVE JOBLET:"
    echo "   • Debian/Ubuntu: apt purge joblet"
    echo "   • RHEL/CentOS/Fedora: yum remove joblet (or dnf remove joblet)"
    echo "   • Manually remove: /etc/sysctl.d/99-joblet.conf, firewall rules"
    echo "   • Manually remove bridge: ip link delete joblet0"
    echo ""
    echo "=========================================================================="
    echo ""
}

display_quickstart_info() {
    local PACKAGE_TYPE="${1:-generic}"

    echo ""
    print_success "Joblet service installed successfully!"
    echo ""
    print_info "🚀 Quick Start:"
    echo "  sudo systemctl start joblet    # Start the service"
    echo "  sudo rnx job list              # Test local connection"
    if [ "$EC2_CLOUDWATCH_CONFIGURED" = "true" ]; then
        echo "  sudo rnx job run echo 'Hello CloudWatch'  # Test job with CloudWatch logging"
    fi
    echo ""
    print_info "📱 Remote Access:"
    echo "  The service accepts connections on: $JOBLET_SERVER_ADDRESS:$JOBLET_SERVER_PORT"
    echo "  Clients can connect using any of these addresses:"
    echo "    - $JOBLET_CERT_PRIMARY:$JOBLET_SERVER_PORT (Internal network)"
    if [ -n "$JOBLET_CERT_PUBLIC_IP" ]; then
        echo "    - $JOBLET_CERT_PUBLIC_IP:$JOBLET_SERVER_PORT (Internet)"
    fi
    if [ -n "$JOBLET_CERT_DOMAIN" ]; then
        # Split domains by comma and display each
        IFS=',' read -ra DOMAINS <<< "$JOBLET_CERT_DOMAIN"
        for domain in "${DOMAINS[@]}"; do
            echo "    - ${domain}:$JOBLET_SERVER_PORT"
        done
    fi
    echo ""
    print_info "📋 Client Configuration:"
    echo "  Copy ${JOBLET_HOME}/config/rnx-config.yml to client machines"
    echo "  Or use: scp root@$JOBLET_CERT_PRIMARY:${JOBLET_HOME}/config/rnx-config.yml ~/.rnx/"
    echo ""

    # Display AWS-specific information
    display_aws_quickstart
}
