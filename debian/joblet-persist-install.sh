#!/bin/bash
# This script can be sourced or run standalone
# It installs joblet-persist from GitHub releases

# Define helper functions if not already defined
if ! command -v print_info &> /dev/null; then
    print_info() { echo "ℹ️  $1"; }
    print_success() { echo "✅ $1"; }
    print_error() { echo "❌ $1"; }
    print_warning() { echo "⚠️  $1"; }
fi

install_joblet_persist() {
    print_info "📦 Installing joblet-persist from GitHub..."

    # Configuration
    PERSIST_VERSION="${JOBLET_PERSIST_VERSION:-latest}"
    PERSIST_REPO="ehsaniara/joblet-persist"
    PERSIST_BINARY="/opt/joblet/bin/joblet-persist"

    # Create directories
    mkdir -p /opt/joblet/bin
    mkdir -p /opt/joblet/run
    mkdir -p /opt/joblet/metrics

    # Determine architecture
    ARCH=$(uname -m)
    case "$ARCH" in
        x86_64)
            BINARY_ARCH="amd64"
            ;;
        aarch64|arm64)
            BINARY_ARCH="arm64"
            ;;
        *)
            print_error "Unsupported architecture: $ARCH"
            return 1
            ;;
    esac

    # Get latest release if version not specified
    if [ "$PERSIST_VERSION" = "latest" ]; then
        print_info "Fetching latest joblet-persist release..."
        PERSIST_VERSION=$(curl -s "https://api.github.com/repos/$PERSIST_REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

        if [ -z "$PERSIST_VERSION" ]; then
            print_error "Failed to fetch latest version from GitHub"
            return 1
        fi

        print_success "Latest version: $PERSIST_VERSION"
    fi

    # Construct download URL
    DOWNLOAD_URL="https://github.com/$PERSIST_REPO/releases/download/$PERSIST_VERSION/joblet-persist-linux-${BINARY_ARCH}"

    print_info "Downloading joblet-persist from: $DOWNLOAD_URL"

    # Download binary
    if curl -L -f -o "$PERSIST_BINARY" "$DOWNLOAD_URL"; then
        chmod +x "$PERSIST_BINARY"
        print_success "joblet-persist binary downloaded successfully"
    else
        print_error "Failed to download joblet-persist from GitHub"
        print_warning "Please manually install joblet-persist to $PERSIST_BINARY"
        return 1
    fi

    # Verify binary
    if [ -x "$PERSIST_BINARY" ]; then
        VERSION_OUTPUT=$("$PERSIST_BINARY" --version 2>&1 || echo "unknown")
        print_success "joblet-persist installed: $VERSION_OUTPUT"
    else
        print_error "joblet-persist binary is not executable"
        return 1
    fi

    # Create joblet user if it doesn't exist
    if ! id "joblet" &>/dev/null; then
        print_info "Creating joblet user..."
        useradd --system --no-create-home --shell /bin/false joblet
        print_success "joblet user created"
    fi

    # Set ownership and permissions
    chown -R joblet:joblet /opt/joblet/logs
    chown -R joblet:joblet /opt/joblet/metrics
    chown -R joblet:joblet /opt/joblet/run
    chmod 755 /opt/joblet/logs
    chmod 755 /opt/joblet/metrics
    chmod 755 /opt/joblet/run

    # Copy systemd service file
    if [ -f /opt/joblet/scripts/joblet-persist.service ]; then
        cp /opt/joblet/scripts/joblet-persist.service /etc/systemd/system/
        systemctl daemon-reload
        systemctl enable joblet-persist.service
        print_success "joblet-persist systemd service installed"
    else
        print_warning "joblet-persist.service file not found in /opt/joblet/scripts/"
    fi

    # Create default config if it doesn't exist
    if [ ! -f /opt/joblet/config/joblet-persist-config.yml ]; then
        cat > /opt/joblet/config/joblet-persist-config.yml << 'EOF'
# joblet-persist configuration
server:
  host: "0.0.0.0"
  port: 50052

ipc:
  socket_path: "/opt/joblet/run/persist.sock"
  buffer_size: 10000

storage:
  logs_dir: "/opt/joblet/logs"
  metrics_dir: "/opt/joblet/metrics"

retention:
  days: 30
  cleanup_interval: "24h"

logging:
  level: "info"
  format: "json"
EOF
        chown root:root /opt/joblet/config/joblet-persist-config.yml
        chmod 644 /opt/joblet/config/joblet-persist-config.yml
        print_success "Created default joblet-persist config"
    fi

    return 0
}

# Allow script to be run standalone (for RPM %post or manual execution)
# If sourced, the function is available but not auto-executed
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    # Script is being executed directly, not sourced
    install_joblet_persist
    exit $?
fi

# This function should be called from the main postinst script
# after certificate generation and before starting services
