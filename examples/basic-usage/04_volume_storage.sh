#!/bin/bash
set -e

echo "💾 Joblet Basic Usage: Volume Storage"
echo "===================================="
echo ""
echo "This demo shows how to create and use persistent volumes for data storage."
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

# Cleanup any existing demo volumes from previous runs
echo "🧹 Cleaning up any existing demo volumes..."
rnx volume remove demo-data 2>/dev/null || true
rnx volume remove temp-cache 2>/dev/null || true
rnx volume remove shared-workspace 2>/dev/null || true
echo ""

echo "📋 Demo 1: Creating Volumes"
echo "---------------------------"
echo "Creating different types of volumes"

echo "Creating filesystem volume (persistent):"
rnx volume create demo-data --size=100MB --type=filesystem
echo "✅ Created 'demo-data' filesystem volume (100MB)"
echo ""

echo "Creating memory volume (temporary, faster):"
rnx volume create temp-cache --size=50MB --type=memory
echo "✅ Created 'temp-cache' memory volume (50MB)"
echo ""

echo "📋 Demo 2: Listing Volumes"
echo "--------------------------"
echo "Current volumes:"
rnx volume list
echo ""

echo "📋 Demo 3: Using Filesystem Volume"
echo "----------------------------------"
echo "Writing data to persistent filesystem volume"
rnx run --volume=demo-data bash -c "
echo 'Writing to persistent volume at /volumes/demo-data/'
echo 'Volume mount point contents:'
ls -la /volumes/
echo ''
echo 'Creating persistent data file:'
echo 'Hello from Joblet Volume Demo!' > /volumes/demo-data/greeting.txt
echo 'Current timestamp:' \$(date) >> /volumes/demo-data/greeting.txt
echo 'Job ID: \$\$' >> /volumes/demo-data/greeting.txt
echo ''
echo 'File created successfully:'
cat /volumes/demo-data/greeting.txt
echo ''
echo 'Volume directory contents:'
ls -la /volumes/demo-data/
"
echo ""

echo "📋 Demo 4: Data Persistence Verification"
echo "----------------------------------------"
echo "Running new job to verify data persists across job runs"
rnx run --volume=demo-data bash -c "
echo 'New job reading from persistent volume:'
echo ''
echo 'Previous data still exists:'
cat /volumes/demo-data/greeting.txt
echo ''
echo 'Appending new data:'
echo 'Second job ran at:' \$(date) >> /volumes/demo-data/greeting.txt
echo 'Updated file contents:'
cat /volumes/demo-data/greeting.txt
"
echo ""

echo "📋 Demo 5: Using Memory Volume"
echo "------------------------------"
echo "Working with fast memory-based volume"
rnx run --volume=temp-cache bash -c "
echo 'Using memory volume for temporary fast storage:'
echo ''
echo 'Memory volume is mounted at /volumes/temp-cache/'
ls -la /volumes/temp-cache/
echo ''
echo 'Creating temporary cache files:'
for i in {1..5}; do
    echo \"Cache entry \$i: \$(date +%s.%N)\" > /volumes/temp-cache/cache_\$i.txt
done
echo ''
echo 'Cache files created:'
ls -la /volumes/temp-cache/
echo ''
echo 'Cache contents:'
cat /volumes/temp-cache/cache_*.txt
"
echo ""

echo "📋 Demo 6: Multiple Volume Usage"
echo "--------------------------------"
echo "Using both volumes in a single job"
rnx run --volume=demo-data --volume=temp-cache bash -c "
echo 'Job with multiple volumes mounted:'
echo ''
echo 'Available volumes:'
ls -la /volumes/
echo ''
echo 'Reading from persistent volume:'
cat /volumes/demo-data/greeting.txt
echo ''
echo 'Using memory volume for temporary processing:'
echo 'Processing data...' > /volumes/temp-cache/processing.log
echo 'Step 1: Data loaded' >> /volumes/temp-cache/processing.log
echo 'Step 2: Processing started' >> /volumes/temp-cache/processing.log
echo 'Step 3: Results calculated' >> /volumes/temp-cache/processing.log
echo ''
echo 'Processing log:'
cat /volumes/temp-cache/processing.log
echo ''
echo 'Saving results to persistent volume:'
{
    echo 'Processing Results'
    echo '=================='
    echo 'Processed at:' \$(date)
    echo 'Processing steps: 3'
    echo 'Status: Completed'
} > /volumes/demo-data/results.txt
echo 'Results saved to persistent storage:'
cat /volumes/demo-data/results.txt
"
echo ""

echo "📋 Demo 7: Volume Size and Usage"
echo "--------------------------------"
echo "Demonstrating volume space management"
rnx run --volume=demo-data bash -c "
echo 'Checking volume space usage:'
echo ''
echo 'Current volume contents:'
ls -lah /volumes/demo-data/
echo ''
echo 'Volume space usage:'
du -sh /volumes/demo-data/* 2>/dev/null || echo 'Files are small, under 1KB each'
echo ''
echo 'Creating larger test file to demonstrate space usage:'
echo 'This is a larger test file to show volume space usage.' > /volumes/demo-data/large_file.txt
for i in {1..100}; do
    echo \"Line \$i: This is sample data to fill the file with more content.\" >> /volumes/demo-data/large_file.txt
done
echo ''
echo 'Updated volume contents:'
ls -lah /volumes/demo-data/
echo ''
echo 'File sizes:'
du -h /volumes/demo-data/*
"
echo ""

echo "📋 Demo 8: Workspace vs Volume Storage"
echo "--------------------------------------"
echo "Comparing temporary workspace with persistent volume storage"
rnx run --volume=demo-data bash -c "
echo 'Understanding storage locations:'
echo ''
echo '1. Temporary workspace (lost after job):'
echo 'Current directory (workspace):' && pwd
echo 'Creating temporary file in workspace:'
echo 'This file will be lost when job ends' > workspace_temp.txt
ls -la workspace_temp.txt
echo ''
echo '2. Persistent volume (survives job completion):'
echo 'Volume directory:' && ls -la /volumes/demo-data/
echo 'Creating file in persistent volume:'
echo 'This file will persist after job ends' > /volumes/demo-data/persistent_file.txt
echo ''
echo 'Both files exist during job execution:'
echo 'Workspace file:' && cat workspace_temp.txt
echo 'Persistent file:' && cat /volumes/demo-data/persistent_file.txt
"
echo ""

echo "📋 Demo 9: Verifying Persistence"
echo "--------------------------------"
echo "Final verification that volume data persists"
rnx run --volume=demo-data bash -c "
echo 'Final persistence check - new job, same volume:'
echo ''
echo 'Volume contents (should include all previous files):'
ls -la /volumes/demo-data/
echo ''
echo 'Reading files from previous jobs:'
echo '=== greeting.txt ==='
cat /volumes/demo-data/greeting.txt
echo ''
echo '=== results.txt ==='
cat /volumes/demo-data/results.txt
echo ''
echo '=== persistent_file.txt ==='
cat /volumes/demo-data/persistent_file.txt
echo ''
echo 'All files persisted successfully across job runs!'
"
echo ""

echo "📋 Demo 10: Volume Cleanup"
echo "--------------------------"
echo "Demonstrating volume lifecycle management"

echo "Current volumes before cleanup:"
rnx volume list
echo ""

echo "Removing memory volume (temporary cache):"
rnx volume remove temp-cache
echo "✅ Memory volume removed"
echo ""

echo "Keeping filesystem volume for future use:"
echo "💡 The 'demo-data' volume is kept for potential future demonstrations"
echo ""

echo "Final volume list:"
rnx volume list
echo ""

echo "✅ Volume Storage Demo Complete!"
echo ""
echo "🎓 What you learned:"
echo "  • How to create volumes with --size and --type parameters"
echo "  • Difference between filesystem (persistent) and memory (temporary) volumes"
echo "  • How to mount volumes in jobs with --volume"
echo "  • Volume mount points at /volumes/<volume-name>/"
echo "  • Data persistence across multiple job executions"
echo "  • Using multiple volumes in a single job"
echo "  • Volume lifecycle management (create, use, remove)"
echo ""
echo "📝 Key takeaways:"
echo "  • Filesystem volumes persist data across job runs and server restarts"
echo "  • Memory volumes are faster but temporary (cleared when removed)"
echo "  • Volumes are mounted at /volumes/<volume-name>/ inside jobs"
echo "  • Jobs without volumes have limited temporary workspace (1MB)"
echo "  • Multiple volumes can be used in a single job"
echo "  • Volume size should match your data storage needs"
echo ""
echo "💡 Best practices:"
echo "  • Use filesystem volumes for data that must persist"
echo "  • Use memory volumes for temporary high-speed processing"
echo "  • Size volumes appropriately to avoid running out of space"
echo "  • Clean up unused volumes to free resources"
echo "  • Consider volume backup strategies for critical data"
echo ""
echo "➡️  Next: Try ./05_job_monitoring.sh to learn about job management"