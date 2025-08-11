#!/bin/bash
# Network connectivity test script

echo "Testing network connectivity..."
echo "Hostname: $(hostname)"
echo "IP addresses:"
ip addr show 2>/dev/null || ifconfig 2>/dev/null || echo "No network tools available"
echo "Testing external connectivity:"
ping -c 3 8.8.8.8 2>/dev/null && echo "External network: accessible" || echo "External network: isolated"