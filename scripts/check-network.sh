#!/bin/bash

echo "Checking joblet network requirements..."

# Check IP forwarding
if [ "$(cat /proc/sys/net/ipv4/ip_forward)" != "1" ]; then
    echo "ERROR: IP forwarding is disabled"
    echo "Fix: sudo sysctl -w net.ipv4.ip_forward=1"
    exit 1
fi

# Check iptables
if ! command -v iptables &> /dev/null; then
    echo "ERROR: iptables not installed"
    exit 1
fi

# Check for NAT table
if ! iptables -t nat -L &> /dev/null; then
    echo "ERROR: iptables NAT table not available"
    exit 1
fi

# Check kernel modules
for module in br_netfilter nf_conntrack; do
    if ! lsmod | grep -q "^$module"; then
        echo "WARNING: Kernel module $module not loaded"
    fi
done

# Check joblet bridge
if ip link show joblet0 &> /dev/null; then
    echo "OK: Default bridge network exists"
else
    echo "WARNING: Default bridge network not found"
fi

# Check NAT rules
if iptables -t nat -L POSTROUTING -n | grep -q "172.20.0.0/16.*MASQUERADE"; then
    echo "OK: NAT rules configured"
else
    echo "WARNING: NAT rules not found"
fi

echo "Network check complete"