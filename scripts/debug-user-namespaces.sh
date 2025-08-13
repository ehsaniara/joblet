#!/bin/bash

echo "🔍 Debugging user namespace configuration on $(hostname)..."

# Kernel configuration
echo "📋 Kernel configuration:"
echo "  /proc/sys/user/max_user_namespaces: $(cat /proc/sys/user/max_user_namespaces 2>/dev/null || echo "not found")"
echo "  /proc/sys/kernel/unprivileged_userns_clone: $(cat /proc/sys/kernel/unprivileged_userns_clone 2>/dev/null || echo "not found")"

# SubUID/SubGID configuration
echo "📋 SubUID/SubGID configuration:"
echo "  /etc/subuid entries:"
cat /etc/subuid 2>/dev/null || echo "  File not found"
echo "  /etc/subgid entries:"
cat /etc/subgid 2>/dev/null || echo "  File not found"

# Joblet user info
echo "📋 Joblet user info:"
id joblet 2>/dev/null || echo "  joblet user not found"

# Service status
echo "📋 Service status:"
sudo systemctl status joblet.service --no-pager --lines=5 2>/dev/null || echo "  Service not found"