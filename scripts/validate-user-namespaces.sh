#!/bin/bash
set -e

echo "🔍 Validating user namespace support on $(hostname)..."

# Check kernel support
echo "📋 Checking kernel support..."
if [ ! -f /proc/self/ns/user ]; then
    echo "❌ User namespaces not supported by kernel"
    exit 1
else
    echo "✅ User namespace kernel support detected"
fi

# Check user namespace limits
echo "📋 Checking user namespace limits..."
if [ -f /proc/sys/user/max_user_namespaces ]; then
    MAX_NS=$(cat /proc/sys/user/max_user_namespaces)
    if [ "$MAX_NS" = "0" ]; then
        echo "❌ User namespaces disabled (max_user_namespaces=0)"
        exit 1
    else
        echo "✅ User namespaces enabled (max: $MAX_NS)"
    fi
fi

# Check cgroup namespace support
echo "📋 Checking cgroup namespace support..."
if [ ! -f /proc/self/ns/cgroup ]; then
    echo "❌ Cgroup namespaces not supported by kernel"
    exit 1
else
    echo "✅ Cgroup namespace kernel support detected"
fi

# Check cgroups v2
echo "📋 Checking cgroups v2..."
if [ ! -f /sys/fs/cgroup/cgroup.controllers ]; then
    echo "❌ Cgroups v2 not available"
    exit 1
else
    echo "✅ Cgroups v2 detected"
fi

# Check subuid/subgid files
echo "📋 Checking subuid/subgid files..."
if [ ! -f /etc/subuid ]; then
    echo "❌ /etc/subuid not found"
    exit 1
fi
if [ ! -f /etc/subgid ]; then
    echo "❌ /etc/subgid not found"
    exit 1
fi

# Check joblet user configuration
echo "📋 Checking joblet user configuration..."
if ! grep -q "joblet:" /etc/subuid; then
    echo "❌ joblet not configured in /etc/subuid"
    exit 1
fi
if ! grep -q "joblet:" /etc/subgid; then
    echo "❌ joblet not configured in /etc/subgid"
    exit 1
fi

echo "✅ All user namespace requirements validated successfully!"