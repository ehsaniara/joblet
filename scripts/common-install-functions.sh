#!/bin/bash
# Common installation functions for Joblet
# Used by both Debian (.deb) and RPM (.rpm) packages

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
        print_warning "Please remove conflicting network configuration or modify /opt/joblet/config/joblet-config.yml"
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
    if [ -n "$JOBLET_SERVER_ADDRESS" ] || [ -n "$JOBLET_CERT_INTERNAL_IP" ] || [ -n "$JOBLET_SERVER_IP" ]; then
        print_info "Configuration source: Environment variables"

        # Legacy support: JOBLET_SERVER_IP sets the certificate primary IP
        if [ -n "$JOBLET_SERVER_IP" ]; then
            JOBLET_CERT_PRIMARY="${JOBLET_CERT_PRIMARY:-$JOBLET_SERVER_IP}"
            JOBLET_CERT_INTERNAL_IP="${JOBLET_CERT_INTERNAL_IP:-$JOBLET_SERVER_IP}"
            print_info "Using JOBLET_SERVER_IP (legacy): $JOBLET_SERVER_IP"
        fi

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

setup_dynamodb_table() {
    # Create DynamoDB table for job state persistence
    # Uses AWS CLI to create table if it doesn't exist

    local TABLE_NAME="${DYNAMODB_TABLE_NAME:-joblet-jobs}"
    local AWS_REGION="${EC2_REGION:-us-east-1}"

    print_info "Setting up DynamoDB table for state persistence..."

    # Check if AWS CLI is available
    if ! command -v aws >/dev/null 2>&1; then
        print_warning "AWS CLI not found - skipping table creation"
        print_warning "Install AWS CLI: https://aws.amazon.com/cli/"
        print_warning "Or create table manually using the AWS Console"
        return 1
    fi

    # Check if table already exists
    if aws dynamodb describe-table --table-name "$TABLE_NAME" --region "$AWS_REGION" >/dev/null 2>&1; then
        print_success "DynamoDB table '$TABLE_NAME' already exists"

        # Check if TTL is enabled
        local ttl_status=$(aws dynamodb describe-time-to-live --table-name "$TABLE_NAME" --region "$AWS_REGION" --query 'TimeToLiveDescription.TimeToLiveStatus' --output text 2>/dev/null || echo "")
        if [ "$ttl_status" != "ENABLED" ]; then
            print_info "Enabling TTL on existing table..."
            if aws dynamodb update-time-to-live \
                --table-name "$TABLE_NAME" \
                --time-to-live-specification "Enabled=true,AttributeName=expiresAt" \
                --region "$AWS_REGION" >/dev/null 2>&1; then
                print_success "TTL enabled on table '$TABLE_NAME'"
            else
                print_warning "Could not enable TTL (may require additional permissions)"
            fi
        else
            print_success "TTL already enabled on table '$TABLE_NAME'"
        fi
        return 0
    fi

    # Create the table
    print_info "Creating DynamoDB table: $TABLE_NAME"
    if aws dynamodb create-table \
        --table-name "$TABLE_NAME" \
        --attribute-definitions AttributeName=jobId,AttributeType=S \
        --key-schema AttributeName=jobId,KeyType=HASH \
        --billing-mode PAY_PER_REQUEST \
        --region "$AWS_REGION" \
        --tags Key=ManagedBy,Value=Joblet Key=Purpose,Value=JobStatePersistence \
        >/dev/null 2>&1; then
        print_success "DynamoDB table created successfully"

        # Wait for table to be active
        print_info "Waiting for table to become active..."
        if aws dynamodb wait table-exists --table-name "$TABLE_NAME" --region "$AWS_REGION" 2>/dev/null; then
            print_success "Table is now active"

            # Enable TTL
            print_info "Enabling TTL for automatic cleanup of old jobs..."
            if aws dynamodb update-time-to-live \
                --table-name "$TABLE_NAME" \
                --time-to-live-specification "Enabled=true,AttributeName=expiresAt" \
                --region "$AWS_REGION" >/dev/null 2>&1; then
                print_success "TTL enabled - completed jobs will be auto-deleted after 30 days"
            else
                print_warning "Could not enable TTL (table created but TTL requires additional permissions)"
            fi
        else
            print_warning "Table created but may still be initializing"
        fi
        return 0
    else
        print_error "Failed to create DynamoDB table"
        print_warning "You may need to create it manually or check IAM permissions"
        print_warning "See: https://docs.aws.amazon.com/cli/latest/reference/dynamodb/create-table.html"
        return 1
    fi
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

            # Setup DynamoDB table
            setup_dynamodb_table

            return 0
        fi
    fi

    return 1
}

display_aws_quickstart() {
    # Display AWS-specific quickstart information
    if [ "$EC2_CLOUDWATCH_CONFIGURED" = "true" ]; then
        echo ""
        print_info "🌩️  AWS CloudWatch Logs Configuration:"
        echo "  View logs: AWS Console → CloudWatch → Logs → /joblet"
        echo "  Query logs: aws logs filter-log-events --log-group-name-prefix '/joblet/'"
        echo "  Config file: /opt/joblet/config/joblet-config.yml"
        echo "  Storage type: persist.storage.type = cloudwatch"
        echo "  Documentation: https://docs.aws.amazon.com/cloudwatch/"
        echo ""
    fi

    if [ "$EC2_DYNAMODB_CONFIGURED" = "true" ]; then
        echo ""
        print_info "💾  AWS DynamoDB State Persistence Configuration:"
        echo "  Table: joblet-jobs"
        echo "  View jobs: AWS Console → DynamoDB → Tables → joblet-jobs"
        echo "  Query jobs: aws dynamodb scan --table-name joblet-jobs --region ${EC2_REGION:-us-east-1}"
        echo "  Config file: /opt/joblet/config/joblet-config.yml"
        echo "  Backend type: state.backend = dynamodb"
        echo "  Features:"
        echo "    • Job state persists across restarts"
        echo "    • Auto-cleanup with TTL (30 days for completed jobs)"
        echo "    • Pay-per-request billing (< $0.05/month for 100 jobs/day)"
        echo "  Documentation: https://docs.aws.amazon.com/dynamodb/"
        echo ""
    fi
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

    # Run the embedded certificate generation script
    if [ -x /usr/local/bin/certs_gen_embedded.sh ]; then
        if /usr/local/bin/certs_gen_embedded.sh; then
            print_success "Certificates generated successfully"

            # Update the server configuration with the actual bind address and port
            if [ -f /opt/joblet/config/joblet-config.yml ]; then
                # Update server bind address and port in the config
                sed -i "s/^  address:.*/  address: \"$JOBLET_SERVER_ADDRESS\"/" /opt/joblet/config/joblet-config.yml
                sed -i "s/^  port:.*/  port: $JOBLET_SERVER_PORT/" /opt/joblet/config/joblet-config.yml
                print_success "Updated server configuration: $JOBLET_SERVER_ADDRESS:$JOBLET_SERVER_PORT"
            fi

            # Update client configuration files with all valid connection endpoints
            if [ -f /opt/joblet/config/rnx-config.yml ]; then
                # For each node in the client config, we need to update the address
                # The address in rnx-config.yml should be how clients connect, not the bind address
                # Use the certificate primary address as it's what clients should connect to
                sed -i "s/address: \"[^:]*:50051\"/address: \"$JOBLET_CERT_PRIMARY:$JOBLET_SERVER_PORT\"/" /opt/joblet/config/rnx-config.yml
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
    echo "   • Config files will be stored in /opt/joblet/config/ (chmod 600)"
    echo "   • Network isolation uses Linux namespaces and bridge networking"
    echo ""
    echo "📁 FILES AND DIRECTORIES CREATED:"
    echo "   • /opt/joblet/                 - Main installation directory"
    echo "   • /opt/joblet/config/          - Configuration and certificates"
    echo "   • /opt/joblet/logs/            - Job logs and output"
    echo "   • /opt/joblet/volumes/         - Persistent job volumes"
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
    echo "  Copy /opt/joblet/config/rnx-config.yml to client machines"
    echo "  Or use: scp root@$JOBLET_CERT_PRIMARY:/opt/joblet/config/rnx-config.yml ~/.rnx/"
    echo ""

    # Display AWS-specific information
    display_aws_quickstart
}
