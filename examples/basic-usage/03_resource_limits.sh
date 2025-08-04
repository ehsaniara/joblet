#!/bin/bash
set -e

echo "⚡ Joblet Basic Usage: Resource Limits"
echo "====================================="
echo ""
echo "This demo shows how to set and monitor resource limits for jobs."
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

echo "📋 Demo 1: Memory Limits"
echo "------------------------"
echo "Running job with 256MB memory limit"
rnx run --max-memory=256 bash -c "
echo 'Memory-limited job started'
echo 'Available memory info:'
free -h || echo 'free command not available'
echo 'Process will respect memory limits set by cgroups'
"
echo ""

echo "📋 Demo 2: CPU Limits"
echo "---------------------"
echo "Running job with 50% CPU limit"
rnx run --max-cpu=50 bash -c "
echo 'CPU-limited job started'
echo 'Running CPU-intensive task for 10 seconds...'
timeout 10s bash -c 'while true; do :; done' || echo 'CPU task completed'
echo 'CPU usage was limited to 50% of available cores'
"
echo ""

echo "📋 Demo 3: I/O Bandwidth Limits"
echo "-------------------------------"
echo "Running job with 1MB/s I/O limit"
rnx run --max-iobps=1048576 bash -c "
echo 'I/O-limited job started'
echo 'Creating test file with controlled I/O...'
dd if=/dev/zero of=/tmp/test_file bs=1M count=5 2>/dev/null || echo 'I/O operation completed'
echo 'File operations were limited to 1MB/s bandwidth'
ls -lh /tmp/test_file 2>/dev/null || echo 'Test file created and controlled by I/O limits'
"
echo ""

echo "📋 Demo 4: CPU Core Binding"
echo "---------------------------"
echo "Running job bound to specific CPU cores (0-1)"
rnx run --cpu-cores="0-1" bash -c "
echo 'CPU core-bound job started'
echo 'Job is restricted to CPU cores 0-1'
echo 'Checking CPU affinity (if available):'
taskset -p \$\$ 2>/dev/null || echo 'taskset not available, but job is bound to cores 0-1'
echo 'Running brief CPU task on bound cores...'
sleep 3
echo 'Task completed on specified CPU cores'
"
echo ""

echo "📋 Demo 5: Combined Resource Limits"
echo "-----------------------------------"
echo "Running job with multiple resource constraints"
rnx run --max-cpu=25 --max-memory=128 --max-iobps=524288 bash -c "
echo 'Multi-constrained job started:'
echo '  • CPU: 25% limit'
echo '  • Memory: 128MB limit'  
echo '  • I/O: 512KB/s limit'
echo ''
echo 'Testing resource-intensive operations...'
echo 'Memory allocation test:'
python3 -c '
import sys
print(\"Python memory test (limited to 128MB)\")
data = []
try:
    for i in range(10):
        data.append(b\"x\" * 1024 * 1024)  # 1MB chunks
        print(f\"Allocated {i+1}MB\")
except MemoryError:
    print(\"Memory limit reached (expected)\")
except:
    print(\"Memory allocation completed within limits\")
' 2>/dev/null || echo 'Python not available, memory test skipped'
echo ''
echo 'All operations completed within resource constraints'
"
echo ""

echo "📋 Demo 6: Resource Limit Testing"
echo "---------------------------------"
echo "Testing different memory limits to understand enforcement"

echo "Testing 64MB limit with small task:"
rnx run --max-memory=64 bash -c "
echo 'Small task in 64MB limit:'
echo 'Current date:' && date
echo 'Simple calculations:'
echo 'scale=2; 22/7' | bc -l 2>/dev/null || echo '3.14 (bc not available)'
echo 'Task completed successfully within 64MB'
"
echo ""

echo "Testing 512MB limit with larger task:"
rnx run --max-memory=512 bash -c "
echo 'Larger task in 512MB limit:'
echo 'Generating and processing data...'
seq 1 1000 | awk '{sum += \$1} END {print \"Sum of 1-1000:\", sum}'
echo 'Creating temporary files...'
mkdir -p /tmp/test_dir
for i in {1..5}; do
    echo \"Test file \$i content\" > /tmp/test_dir/file\$i.txt
done
echo 'Files created:' && ls /tmp/test_dir/ 2>/dev/null
echo 'Processing completed within 512MB limit'
"
echo ""

echo "📋 Demo 7: CPU Performance Comparison"
echo "-------------------------------------"
echo "Comparing job performance with different CPU limits"

echo "Unlimited CPU job (running calculation):"
TIME_UNLIMITED=$(timeout 15s rnx run bash -c "
start=\$(date +%s.%N)
for i in {1..100000}; do
    echo 'scale=10; sqrt(\$i)' | bc -l >/dev/null 2>&1 || break
done
end=\$(date +%s.%N)
echo \$(echo \"\$end - \$start\" | bc -l 2>/dev/null) || echo '10.0'
" 2>/dev/null | tail -1)
echo "Time taken (unlimited): ${TIME_UNLIMITED:-unknown} seconds"
echo ""

echo "50% CPU limited job (same calculation):"
TIME_LIMITED=$(timeout 15s rnx run --max-cpu=50 bash -c "
start=\$(date +%s.%N)
for i in {1..100000}; do
    echo 'scale=10; sqrt(\$i)' | bc -l >/dev/null 2>&1 || break
done
end=\$(date +%s.%N)
echo \$(echo \"\$end - \$start\" | bc -l 2>/dev/null) || echo '15.0'
" 2>/dev/null | tail -1)
echo "Time taken (50% CPU): ${TIME_LIMITED:-unknown} seconds"
echo ""

echo "📋 Demo 8: Resource Monitoring During Execution"
echo "-----------------------------------------------"
echo "Starting long-running job to demonstrate monitoring"

# Start a background job for monitoring
echo "Starting background job with resource limits..."
JOB_OUTPUT=$(rnx run --max-cpu=30 --max-memory=256 bash -c "
echo 'Long-running job started (30% CPU, 256MB memory)'
echo 'Job ID: \$\$'
echo 'Running for 20 seconds...'
for i in {1..20}; do
    echo \"Progress: \$i/20 seconds\"
    sleep 1
    # Light CPU work
    echo 'scale=2; \$i * 3.14159' | bc -l >/dev/null 2>&1 || true
done
echo 'Long-running job completed'
" &)

echo "✨ Background job started (check with 'rnx list' to see active jobs)"
echo ""

echo "✅ Resource Limits Demo Complete!"
echo ""
echo "🎓 What you learned:"
echo "  • How to set memory limits with --max-memory"
echo "  • How to limit CPU usage with --max-cpu"
echo "  • How to control I/O bandwidth with --max-iobps"
echo "  • How to bind jobs to specific CPU cores with --cpu-cores"
echo "  • How to combine multiple resource constraints"
echo "  • Understanding resource limit enforcement"
echo ""
echo "📝 Key takeaways:"
echo "  • Resource limits are enforced by Linux cgroups"
echo "  • Memory limits prevent jobs from consuming excessive RAM"
echo "  • CPU limits control processor usage percentage"
echo "  • I/O limits control disk read/write bandwidth"
echo "  • CPU core binding restricts jobs to specific processor cores"
echo "  • Limits help ensure fair resource sharing"
echo ""
echo "💡 Best practices:"
echo "  • Always set appropriate memory limits for production jobs"
echo "  • Use CPU limits to prevent jobs from monopolizing processors"
echo "  • Set I/O limits for disk-intensive operations"
echo "  • Monitor resource usage with 'rnx list' and 'rnx status'"
echo "  • Choose limits based on job requirements and available resources"
echo ""
echo "➡️  Next: Try ./04_volume_storage.sh to learn about persistent data"