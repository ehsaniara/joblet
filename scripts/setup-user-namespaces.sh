#!/bin/bash
set -e

echo "🚀 Setting up user namespace environment on $(hostname)..."

# Create joblet user if not exists
echo "📋 Creating joblet user if not exists..."
if ! id joblet >/dev/null 2>&1; then
    echo "Creating joblet user..."
    sudo useradd -r -s /bin/false joblet
    echo "✅ joblet user created"
else
    echo "✅ joblet user already exists"
fi

# Create subuid/subgid files if needed
echo "📋 Creating subuid/subgid files if needed..."
sudo touch /etc/subuid /etc/subgid

# Set up subuid/subgid ranges
echo "📋 Setting up subuid/subgid ranges..."
if ! grep -q "^joblet:" /etc/subuid 2>/dev/null; then
    echo "joblet:100000:6553600" | sudo tee -a /etc/subuid
    echo "✅ Added subuid entry for joblet"
else
    echo "✅ subuid entry already exists for joblet"
fi

if ! grep -q "^joblet:" /etc/subgid 2>/dev/null; then
    echo "joblet:100000:6553600" | sudo tee -a /etc/subgid
    echo "✅ Added subgid entry for joblet"
else
    echo "✅ subgid entry already exists for joblet"
fi

# Set up cgroup permissions
echo "📋 Setting up cgroup permissions..."
sudo mkdir -p /sys/fs/cgroup
sudo chown joblet:joblet /sys/fs/cgroup 2>/dev/null || echo "Note: Could not change cgroup ownership (may be read-only)"

echo "✅ User namespace environment setup completed!"