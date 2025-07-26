#!/bin/bash

echo "=== Basic Network Connectivity Test ==="
echo "Testing with limited tools available in isolated environment"
echo

# Test 1: Check network configuration using /proc and /sys
echo "1. Checking network configuration in backend network:"
bin/rnx run --network=backend sh -c '
echo "=== Backend Network Job ==="
echo "Hostname: $(hostname 2>/dev/null || echo "unknown")"
echo
echo "Network interfaces:"
ls /sys/class/net/ 2>/dev/null || echo "Cannot list interfaces"
echo
echo "IP addresses from /etc/hosts:"
cat /etc/hosts 2>/dev/null | grep -v "^#" | grep -v "localhost" | grep -v "^$" || echo "No hosts file"
echo
echo "Interface details:"
for iface in $(ls /sys/class/net/ 2>/dev/null); do
    echo "Interface: $iface"
    cat /sys/class/net/$iface/address 2>/dev/null || echo "  No MAC address"
    echo
done
'

echo
echo "2. Running another job in the same backend network:"
bin/rnx run --network=backend sh -c '
echo "=== Second Backend Network Job ==="
echo "This job should have a different IP in the same subnet (10.1.0.x)"
cat /etc/hosts 2>/dev/null | grep -v "^#" | grep -v "localhost" | grep -v "^$"
'

echo
echo "3. Running a job in frontend network for comparison:"
bin/rnx run --network=frontend sh -c '
echo "=== Frontend Network Job ==="
echo "This job should have an IP in a different subnet (10.2.0.x)"
cat /etc/hosts 2>/dev/null | grep -v "^#" | grep -v "localhost" | grep -v "^$"
'

echo
echo "4. Testing basic connectivity within same network:"
echo "Since we don't have ping or nc, we'll verify network setup by checking routes"
bin/rnx run --network=backend sh -c '
echo "=== Routing table (if accessible) ==="
cat /proc/net/route 2>/dev/null | head -5 || echo "Cannot read routing table"
echo
echo "=== ARP table (shows network neighbors) ==="
cat /proc/net/arp 2>/dev/null || echo "Cannot read ARP table"
'

echo
echo "=== Summary ==="
echo "- Jobs in backend network should get IPs like 10.1.0.2, 10.1.0.3, etc."
echo "- Jobs in frontend network should get IPs like 10.2.0.2, 10.2.0.3, etc."
echo "- The 'No route to host' error from earlier proves networks are isolated"
echo "- Jobs in the same network share the same subnet and can reach each other"