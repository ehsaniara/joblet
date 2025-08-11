#!/bin/bash
# Resource limits testing script

echo "Testing resource limits..."
echo "Memory limit:"
cat /sys/fs/cgroup/memory/memory.limit_in_bytes 2>/dev/null || echo "N/A"
echo "CPU test (5 seconds):"
timeout 5s bash -c 'while true; do :; done' || echo "CPU test completed"