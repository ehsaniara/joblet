#!/bin/bash
set -e

echo "🌐 Joblet Basic Usage: Network Basics"
echo "====================================="
echo ""
echo "This demo shows network configuration and isolation concepts in Joblet."
echo ""

# Check prerequisites
if ! command -v rnx &> /dev/null; then
    echo "❌ Error: 'rnx' command not found"
    exit 1
fi

if ! rnx list &> /dev/null; then
    echo "❌ Error: Cannot connect to Joblet server"
    exit 1
fi

echo "✅ Prerequisites checked"
echo ""

echo "📋 Demo 1: Default Network (Bridge Mode)"
echo "----------------------------------------"
echo "Understanding the default bridge network configuration"

echo "Default network connectivity test:"
rnx run bash -c "
echo 'Testing default bridge network:'
echo ''
echo 'Network interfaces:'
ip addr show 2>/dev/null | grep -E '^[0-9]+:' || echo 'ip command not available'
echo ''
echo 'Default route:'
ip route show 2>/dev/null | head -3 || echo 'Routing info not available'
echo ''
echo 'DNS resolution test:'
nslookup google.com 2>/dev/null | head -5 || echo 'nslookup not available'
echo ''
echo 'Connectivity test (ping):'
ping -c 3 google.com 2>/dev/null || echo 'Ping test completed (or not available)'
"
echo ""

echo "📋 Demo 2: Internet Connectivity Test"
echo "-------------------------------------"
echo "Testing outbound internet connectivity"

echo "HTTP connectivity test:"
rnx run bash -c "
echo 'Testing HTTP connectivity:'
echo ''
if command -v curl &> /dev/null; then
    echo 'Using curl for HTTP test:'
    curl -s --connect-timeout 10 https://httpbin.org/ip | head -5 || echo 'HTTP request completed'
    echo ''
    echo 'Testing HTTP headers:'
    curl -s --connect-timeout 10 -I https://www.google.com | head -3 || echo 'Headers retrieved'
elif command -v wget &> /dev/null; then
    echo 'Using wget for HTTP test:'
    wget -q --timeout=10 -O - https://httpbin.org/ip | head -5 || echo 'HTTP request completed'
else
    echo 'Neither curl nor wget available, testing with other methods:'
    nc -z google.com 80 && echo 'Port 80 accessible' || echo 'Port test completed'
fi
"
echo ""

echo "📋 Demo 3: Network Isolation (None Mode)"
echo "----------------------------------------"
echo "Demonstrating network isolation with --network=none"

echo "No network access test:"
rnx run --network=none bash -c "
echo 'Testing isolated network (no network access):'
echo ''
echo 'Network interfaces in isolated mode:'
ip addr show 2>/dev/null | grep -E '^[0-9]+:|inet' || echo 'Network interfaces limited'
echo ''
echo 'Attempting connectivity (should fail):'
ping -c 1 google.com 2>/dev/null && echo 'Unexpected: ping succeeded' || echo '✅ Network isolated - ping failed as expected'
echo ''
echo 'Attempting HTTP request (should fail):'
curl -s --connect-timeout 5 https://google.com 2>/dev/null && echo 'Unexpected: HTTP succeeded' || echo '✅ Network isolated - HTTP failed as expected'
echo ''
echo 'This job runs in complete network isolation'
"
echo ""

echo "📋 Demo 4: Host Network Mode"
echo "----------------------------"
echo "Using host network mode (shares host network stack)"

echo "Host network test:"
rnx run --network=host bash -c "
echo 'Testing host network mode:'
echo ''
echo 'Network interfaces (should match host):'
ip addr show 2>/dev/null | grep -E '^[0-9]+:' | head -5 || echo 'Host network interfaces'
echo ''
echo 'Host network connectivity:'
ping -c 2 google.com 2>/dev/null || echo 'Host network connectivity test completed'
echo ''
echo 'This job shares the host network stack'
echo 'Same IP address and network config as the host system'
"
echo ""

echo "📋 Demo 5: Network Information and Diagnostics"
echo "----------------------------------------------"
echo "Gathering detailed network information"

echo "Comprehensive network diagnostics:"
rnx run bash -c "
echo '=== Network Diagnostics ==='
echo ''
echo '1. Hostname and system info:'
hostname 2>/dev/null || echo 'Hostname: unavailable'
uname -n 2>/dev/null || echo 'System name: unavailable'
echo ''

echo '2. Network interface summary:'
if command -v ip &> /dev/null; then
    ip addr show | grep -E '^[0-9]+:|inet ' | head -10
elif command -v ifconfig &> /dev/null; then
    ifconfig | grep -E '^[a-z]+|inet ' | head -10
else
    echo 'Network interface tools not available'
fi
echo ''

echo '3. Routing information:'
ip route 2>/dev/null | head -5 || echo 'Routing info not available'
echo ''

echo '4. DNS configuration:'
cat /etc/resolv.conf 2>/dev/null | head -5 || echo 'DNS config not accessible'
echo ''

echo '5. Port connectivity tests:'
for port in 80 443 53; do
    nc -z google.com \$port 2>/dev/null && echo \"Port \$port: accessible\" || echo \"Port \$port: not accessible/timeout\"
done
"
echo ""

echo "📋 Demo 6: Network Performance Test"
echo "-----------------------------------"
echo "Basic network performance testing"

echo "Network performance test:"
rnx run bash -c "
echo 'Network Performance Test:'
echo ''
echo 'Testing DNS lookup speed:'
time nslookup google.com >/dev/null 2>&1 || echo 'DNS lookup completed'
echo ''

echo 'Testing HTTP response time:'
if command -v curl &> /dev/null; then
    echo 'HTTP response timing:'
    curl -w 'Total time: %{time_total}s\\nConnect time: %{time_connect}s\\n' -s -o /dev/null https://www.google.com 2>/dev/null || echo 'HTTP timing test completed'
else
    echo 'curl not available for timing test'
fi
echo ''

echo 'Basic bandwidth test (downloading small file):'
if command -v wget &> /dev/null; then
    time wget -q --timeout=10 -O /dev/null https://httpbin.org/bytes/10240 2>/dev/null || echo 'Bandwidth test completed'
elif command -v curl &> /dev/null; then
    time curl -s --connect-timeout 10 https://httpbin.org/bytes/10240 > /dev/null 2>/dev/null || echo 'Bandwidth test completed'
else
    echo 'No tools available for bandwidth test'
fi
"
echo ""

echo "📋 Demo 7: Network Security Concepts"
echo "------------------------------------"
echo "Understanding network security implications"

echo "Network security demonstration:"
rnx run bash -c "
echo 'Network Security Concepts:'
echo ''
echo '1. Default network isolation:'
echo '   • Each job runs in its own network namespace'
echo '   • Jobs cannot see other jobs network traffic'
echo '   • Network isolation provides security boundaries'
echo ''

echo '2. Available network modes:'
echo '   • bridge (default): Isolated with internet access'
echo '   • none: Complete network isolation'
echo '   • host: Shares host network (less isolated)'
echo '   • custom: User-defined networks (if supported)'
echo ''

echo '3. Security implications:'
echo '   • bridge mode: Best balance of access and isolation'
echo '   • none mode: Maximum security, no network access'
echo '   • host mode: Less isolation, inherits host network'
echo ''

echo '4. Current network mode analysis:'
if ping -c 1 google.com >/dev/null 2>&1; then
    echo '   ✅ Internet access available (bridge or host mode)'
else
    echo '   🔒 No internet access (none mode or restricted)'
fi

echo ''
echo 'Network isolation helps prevent:'
echo '   • Cross-job network interference'
echo '   • Unauthorized network access'
echo '   • Network-based attacks between jobs'
"
echo ""

echo "📋 Demo 8: Custom Network Concepts"
echo "----------------------------------"
echo "Understanding custom network capabilities"

echo "Custom network concepts (demonstration only):"
rnx run bash -c "
echo 'Custom Network Concepts:'
echo ''
echo 'Joblet supports custom networks for advanced use cases:'
echo ''
echo '1. Custom Network Creation:'
echo '   rnx network create mynet --cidr=10.10.0.0/24'
echo '   • Creates isolated network with specified IP range'
echo '   • Jobs on same custom network can communicate'
echo '   • Different custom networks are isolated from each other'
echo ''

echo '2. Using Custom Networks:'
echo '   rnx run --network=mynet command'
echo '   • Job joins the specified custom network'
echo '   • Gets IP address from network CIDR range'
echo '   • Can communicate with other jobs on same network'
echo ''

echo '3. Network Management:'
echo '   rnx network list          # List available networks'
echo '   rnx network delete mynet  # Remove custom network'
echo ''

echo '4. Use Cases:'
echo '   • Multi-tier applications requiring job communication'
echo '   • Distributed processing with job coordination'
echo '   • Development environments with service dependencies'
echo '   • Testing network-based applications'
echo ''

echo 'Note: Custom networks require appropriate server permissions'
echo 'and may not be available in all Joblet configurations.'
"
echo ""

echo "📋 Demo 9: Network Troubleshooting"
echo "----------------------------------"
echo "Common network troubleshooting techniques"

echo "Network troubleshooting guide:"
rnx run bash -c "
echo 'Network Troubleshooting Checklist:'
echo ''
echo '1. Basic connectivity test:'
ping -c 1 8.8.8.8 >/dev/null 2>&1 && echo '   ✅ Basic IP connectivity works' || echo '   ❌ Basic IP connectivity failed'
echo ''

echo '2. DNS resolution test:'
nslookup google.com >/dev/null 2>&1 && echo '   ✅ DNS resolution works' || echo '   ❌ DNS resolution failed'
echo ''

echo '3. HTTP connectivity test:'
if command -v curl &> /dev/null; then
    curl -s --connect-timeout 5 https://www.google.com >/dev/null 2>&1 && echo '   ✅ HTTPS connectivity works' || echo '   ❌ HTTPS connectivity failed'
else
    echo '   ⚠️  curl not available for HTTPS test'
fi
echo ''

echo '4. Common issues and solutions:'
echo '   • No connectivity: Check network mode (--network=none blocks all)'
echo '   • DNS issues: Verify /etc/resolv.conf has valid nameservers'
echo '   • Timeout errors: Check firewall rules and server network config'
echo '   • Custom network issues: Verify network exists and has capacity'
echo ''

echo '5. Diagnostic commands:'
echo '   ip addr show           # Show network interfaces'
echo '   ip route show          # Show routing table'
echo '   cat /etc/resolv.conf   # Show DNS configuration'
echo '   ping <host>            # Test connectivity'
echo '   nslookup <domain>      # Test DNS resolution'
echo '   curl -I <url>          # Test HTTP connectivity'
"
echo ""

echo "✅ Network Basics Demo Complete!"
echo ""
echo "🎓 What you learned:"
echo "  • Default bridge network provides isolated internet access"
echo "  • --network=none completely isolates jobs from network"
echo "  • --network=host shares the host system's network stack"
echo "  • Network isolation provides security between jobs"
echo "  • Custom networks enable job-to-job communication"
echo "  • Network diagnostics help troubleshoot connectivity issues"
echo ""
echo "📝 Key takeaways:"
echo "  • Each job runs in its own network namespace by default"
echo "  • Network modes control the level of isolation and access"
echo "  • Bridge mode balances isolation with internet connectivity"
echo "  • Network isolation is a key security feature"
echo "  • Custom networks enable advanced networking scenarios"
echo ""
echo "💡 Best practices:"
echo "  • Use default bridge mode for most jobs requiring internet access"
echo "  • Use --network=none for jobs that don't need network access"
echo "  • Use --network=host sparingly (reduces isolation)"
echo "  • Test network connectivity early in job execution"
echo "  • Consider network requirements when designing job workflows"
echo "  • Use custom networks for jobs that need to communicate"
echo ""
echo "🔧 Network mode selection guide:"
echo "  • bridge: Default, isolated with internet access"
echo "  • none: Maximum security, no network at all"
echo "  • host: Shares host network, less isolation"
echo "  • custom: Job-to-job communication on shared network"
echo ""
echo "🔒 Security considerations:"
echo "  • Network isolation prevents cross-job interference"
echo "  • Bridge mode provides good security with internet access"
echo "  • Host mode reduces isolation - use with caution"
echo "  • Custom networks create communication boundaries"
echo ""
echo "➡️  Next: Try ./run_demos.sh to run all basic usage examples together"