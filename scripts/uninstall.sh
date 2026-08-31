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

# rm -rf target below: only /opt/joblet* is ever accepted
case "$JOBLET_HOME" in
    /opt/joblet|/opt/joblet-*) ;;
    *)
        echo "❌ Refusing to uninstall with JOBLET_HOME='$JOBLET_HOME' (must be /opt/joblet or /opt/joblet-*)"
        exit 1
        ;;
esac

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
# Exact-spec rule deletes below; skip when no joblet install is present
if [ -d "${JOBLET_HOME}" ] || [ -e /etc/systemd/system/joblet.service ] || ip link show joblet0 >/dev/null 2>&1; then
echo "  Cleaning up network resources..."

# NAT rules, anchored to joblet's subnets
iptables -t nat -D POSTROUTING -s 172.20.0.0/16 -j MASQUERADE 2>/dev/null || true
iptables-save 2>/dev/null | grep -E "POSTROUTING.*10\.255\.255\.2([^0-9]|$).*MASQUERADE" | \
    sed 's/-A/-D/' | while read -r rule; do
    iptables -t nat $rule 2>/dev/null || true
done

# FORWARD rules: match the interface flag, not a substring ("viso" is also in "supervisor")
iptables -S FORWARD 2>/dev/null | grep -E -- '-(i|o) (joblet[^ ]*|viso[0-9]+)( |$)' | \
    sed 's/^-A/-D/' | while read -r rule; do
    iptables $rule 2>/dev/null || true
done

# Remove veth interfaces and bridges
for veth in $(ip -o link show 2>/dev/null | awk -F': ' '{print $2}' | cut -d@ -f1 | grep -E '^viso[0-9]+$'); do
    ip link delete "$veth" 2>/dev/null || true
done
for bridge in $(ip -o link show type bridge 2>/dev/null | awk -F': ' '{print $2}' | cut -d@ -f1 | grep -E '^joblet'); do
    ip link delete "$bridge" 2>/dev/null || true
done
else
    echo "  No joblet install detected - skipping network cleanup"
fi

# ============================================
# Clean up cgroups
# ============================================
# joblet.slice is a systemd unit (Slice= in joblet.service); stopping it removes the cgroup
systemctl stop joblet.slice 2>/dev/null || true
if [ -d "/sys/fs/cgroup/joblet.slice" ]; then
    find /sys/fs/cgroup/joblet.slice -depth -mindepth 1 -type d -exec rmdir {} \; 2>/dev/null || true
    rmdir /sys/fs/cgroup/joblet.slice 2>/dev/null || true
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
rm -f /usr/bin/joblet
rm -f /usr/local/bin/joblet
# Only symlinks into this install; a standalone rnx must survive
for link in /usr/bin/rnx /usr/local/bin/rnx; do
    case "$(readlink "$link" 2>/dev/null)" in
        /opt/joblet/*) rm -f "$link" ;;
    esac
done
# Marker holds the installed binary's sha256; a mismatch means the user replaced it
if [ -f "${JOBLET_HOME}/.rnx-installed-by-joblet" ]; then
    MARKER_SHA=$(cat "${JOBLET_HOME}/.rnx-installed-by-joblet" 2>/dev/null)
    CURRENT_SHA=$(sha256sum /usr/local/bin/rnx 2>/dev/null | awk '{print $1}')
    if [ -z "$MARKER_SHA" ] || [ "$MARKER_SHA" = "$CURRENT_SHA" ]; then
        rm -f /usr/local/bin/rnx
    else
        echo "  Note: /usr/local/bin/rnx changed since joblet installed it - left in place"
    fi
    rm -f "${JOBLET_HOME}/.rnx-installed-by-joblet"
fi
rm -f /usr/local/bin/certs_gen_embedded.sh /usr/local/bin/certs_gen_with_secretsmanager.sh
rm -rf /etc/joblet
# Installer-written system config (IP forwarding, kernel module loading)
rm -f /etc/sysctl.d/99-joblet.conf
rm -f /etc/modules-load.d/joblet.conf
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
# Remove the joblet system user and group
# ============================================
if id joblet >/dev/null 2>&1; then
    userdel joblet 2>/dev/null || true
fi
if getent group joblet >/dev/null 2>&1; then
    groupdel joblet 2>/dev/null || true
fi

# ============================================
# Verify nothing remains (purge only)
# ============================================
if [ "$PURGE" = true ]; then
    echo "  Verifying no residue remains..."
    RESIDUE=0
    for p in \
        "${JOBLET_HOME}" \
        /etc/joblet \
        /var/log/joblet \
        /var/lib/joblet \
        /etc/systemd/system/joblet.service \
        /etc/sysctl.d/99-joblet.conf \
        /etc/modules-load.d/joblet.conf \
        /usr/bin/joblet \
        /usr/local/bin/joblet \
        /usr/local/bin/certs_gen_embedded.sh \
        /usr/local/bin/certs_gen_with_secretsmanager.sh \
        /sys/fs/cgroup/joblet.slice; do
        if [ -e "$p" ] || [ -L "$p" ]; then
            echo "  ⚠️  leftover: $p"
            RESIDUE=1
        fi
    done
    if ip link show 2>/dev/null | grep -q "joblet"; then
        echo "  ⚠️  leftover: joblet network interfaces ($(ip link show | grep -o 'joblet[^ :]*' | tr '\n' ' '))"
        RESIDUE=1
    fi
    if iptables-save 2>/dev/null | grep -qE "joblet|viso"; then
        echo "  ⚠️  leftover: joblet iptables rules"
        RESIDUE=1
    fi
    if id joblet >/dev/null 2>&1 || getent group joblet >/dev/null 2>&1; then
        echo "  ⚠️  leftover: joblet user or group"
        RESIDUE=1
    fi
    if command -v dpkg >/dev/null 2>&1 && dpkg -s joblet >/dev/null 2>&1; then
        echo "  ⚠️  leftover: dpkg still knows the joblet package"
        RESIDUE=1
    fi
    if [ "$RESIDUE" -eq 0 ]; then
        echo "  ✓ No joblet residue found on this host"
    else
        echo "  ⚠️  Residue found above - uninstall is INCOMPLETE"
        exit 1
    fi
fi

echo "✅ Joblet uninstalled"
