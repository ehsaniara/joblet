#!/bin/bash

# Joblet Uninstall Script
# Removes all joblet components from this host: service, binaries, configs,
# data, network resources, symlinks, and per-user rnx client configs.
#
# Usage:
#   sudo ./uninstall.sh              # Uninstall, keep job logs/volumes
#   sudo ./uninstall.sh --purge      # Uninstall and remove ALL data
#   sudo ./uninstall.sh --purge --all-users   # Also remove every user's ~/.rnx

set -e

JOBLET_HOME="${JOBLET_HOME:-/opt/joblet}"

PURGE=false
ALL_USERS=false
for arg in "$@"; do
    case "$arg" in
        --purge) PURGE=true ;;
        --all-users) ALL_USERS=true ;;
        -h|--help)
            grep '^#' "$0" | sed 's/^# \?//' | head -10
            exit 0
            ;;
        *)
            echo "Unknown option: $arg (use --purge, --all-users, or --help)"
            exit 1
            ;;
    esac
done

if [ "$(id -u)" -ne 0 ]; then
    echo "❌ This script must be run as root (sudo $0)"
    exit 1
fi

echo "🗑️  Uninstalling joblet..."

# ============================================
# Stop and remove the service
# ============================================
if systemctl list-unit-files joblet.service >/dev/null 2>&1; then
    systemctl stop joblet.service 2>/dev/null || true
    systemctl disable joblet.service 2>/dev/null || true
fi

# ============================================
# Clean up network resources
# ============================================
echo "  Cleaning up network resources..."

# Remove NAT rules
iptables -t nat -D POSTROUTING -s 172.20.0.0/16 -j MASQUERADE 2>/dev/null || true
iptables-save 2>/dev/null | grep "POSTROUTING.*10\.255\.255\.2.*MASQUERADE" | \
    sed 's/-A/-D/' | while read -r rule; do
    iptables -t nat $rule 2>/dev/null || true
done

# Remove FORWARD rules
iptables -S FORWARD 2>/dev/null | grep -E "joblet|viso" | sed 's/^-A/-D/' | while read -r rule; do
    iptables $rule 2>/dev/null || true
done

# Remove veth interfaces and bridges
for veth in $(ip link show 2>/dev/null | grep -o 'viso[0-9]*' | grep -v '@'); do
    ip link delete "$veth" 2>/dev/null || true
done
for bridge in $(ip link show type bridge 2>/dev/null | grep -o 'joblet[^ :]*'); do
    ip link delete "$bridge" 2>/dev/null || true
done

# ============================================
# Clean up cgroups
# ============================================
if [ -d "/sys/fs/cgroup/joblet.slice" ]; then
    find /sys/fs/cgroup/joblet.slice -name "job-*" -type d -exec rmdir {} \; 2>/dev/null || true
fi

# ============================================
# Remove the package registration (deb/rpm installs)
# ============================================
if command -v dpkg >/dev/null 2>&1 && dpkg -s joblet >/dev/null 2>&1; then
    echo "  Removing dpkg package..."
    if [ "$PURGE" = true ]; then
        dpkg --purge joblet 2>/dev/null || true
    else
        dpkg --remove joblet 2>/dev/null || true
    fi
elif command -v rpm >/dev/null 2>&1 && rpm -q joblet >/dev/null 2>&1; then
    echo "  Removing rpm package..."
    rpm -e joblet 2>/dev/null || true
fi

# ============================================
# Remove files, symlinks, and service unit
# ============================================
echo "  Removing binaries, symlinks, and service unit..."
rm -f /etc/systemd/system/joblet.service
rm -f /usr/bin/joblet /usr/bin/rnx
rm -f /usr/local/bin/joblet /usr/local/bin/rnx
rm -f /usr/local/bin/certs_gen_embedded.sh /usr/local/bin/certs_gen_with_secretsmanager.sh
rm -rf /etc/joblet
systemctl daemon-reload 2>/dev/null || true

# ============================================
# Remove installation directory and data
# ============================================
if [ "$PURGE" = true ]; then
    echo "  Removing all joblet data..."
    rm -rf "${JOBLET_HOME}"
    rm -rf /var/log/joblet /var/lib/joblet
else
    # Keep job logs and volumes unless purging
    rm -rf "${JOBLET_HOME}/bin" "${JOBLET_HOME}/config" "${JOBLET_HOME}/scripts" "${JOBLET_HOME}/run"
    rm -rf /var/log/joblet
    if [ -d "${JOBLET_HOME}" ] && [ -n "$(ls -A "${JOBLET_HOME}" 2>/dev/null)" ]; then
        echo "  Note: data preserved in ${JOBLET_HOME} (logs, volumes, runtimes)"
        echo "        run with --purge to remove everything"
    else
        rm -rf "${JOBLET_HOME}"
    fi
fi

# ============================================
# Remove per-user rnx client configs (~/.rnx)
# ============================================
remove_user_rnx() {
    local home="$1"
    if [ -n "$home" ] && [ -d "$home/.rnx" ]; then
        rm -rf "$home/.rnx"
        echo "  Removed $home/.rnx"
    fi
}

if [ "$ALL_USERS" = true ]; then
    remove_user_rnx /root
    for home in /home/*; do
        [ -d "$home" ] && remove_user_rnx "$home"
    done
else
    # Remove for root and the user who invoked sudo
    remove_user_rnx /root
    if [ -n "${SUDO_USER:-}" ] && [ "$SUDO_USER" != "root" ]; then
        remove_user_rnx "$(getent passwd "$SUDO_USER" | cut -d: -f6)"
    fi
    # Warn about other users that still have client configs
    for home in /home/*; do
        [ -d "$home/.rnx" ] && echo "  Note: $home/.rnx left in place (use --all-users to remove)"
    done
fi

# ============================================
# Clean up systemd journal entries
# ============================================
echo "  Cleaning up journal logs..."
# Remove a dedicated joblet journal namespace if one exists
systemctl stop systemd-journald@joblet.service 2>/dev/null || true
rm -rf /var/log/journal/*.joblet /run/log/journal/*.joblet 2>/dev/null || true
# Entries in the shared system journal cannot be deleted per-unit; rotate so
# they land in archive files and expire with the normal journald retention
journalctl --rotate 2>/dev/null || true
if [ "$PURGE" = true ]; then
    echo "  Note: joblet entries in the shared system journal expire with journald"
    echo "        retention; deleting them now would wipe other services' logs too"
fi

# ============================================
# Remove the joblet system user
# ============================================
if id joblet >/dev/null 2>&1; then
    userdel joblet 2>/dev/null || true
fi

echo "✅ Joblet uninstalled"
