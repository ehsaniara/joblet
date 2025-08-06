#!/bin/bash
set -e

echo "📊 Joblet Basic Usage: Job Monitoring"
echo "====================================="
echo ""
echo "This demo shows how to monitor and manage jobs in Joblet."
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

echo "📋 Demo 1: Current Job Status"
echo "-----------------------------"
echo "Checking current jobs on the server"
echo "Current jobs:"
rnx list
echo ""

echo "📋 Demo 2: Starting Jobs for Monitoring"
echo "---------------------------------------"
echo "Starting several jobs to demonstrate monitoring capabilities"

# Start a quick job
echo "Starting quick job (5 seconds)..."
QUICK_JOB=$(rnx run bash -c "
echo 'Quick job started'
for i in {1..5}; do
    echo \"Quick job progress: \$i/5\"
    sleep 1
done
echo 'Quick job completed'
" 2>&1 | tail -1 | grep -o '[0-9a-f-]*' | head -1 || echo "quick-job")

# Start a medium job
echo "Starting medium job (15 seconds)..."
MEDIUM_JOB=$(timeout 20s rnx run bash -c "
echo 'Medium job started'
for i in {1..15}; do
    echo \"Medium job progress: \$i/15\"
    sleep 1
done  
echo 'Medium job completed'
" &>/dev/null & echo $!)

# Start a long job
echo "Starting long job (30 seconds)..."  
LONG_JOB=$(timeout 35s rnx run bash -c "
echo 'Long job started - this will run for 30 seconds'
for i in {1..30}; do
    echo \"Long job progress: \$i/30 - \$(date)\"
    sleep 1
done
echo 'Long job completed'
" &>/dev/null & echo $!)

echo "✅ Started demonstration jobs"
echo ""

# Give jobs a moment to start
sleep 2

echo "📋 Demo 3: Listing Active Jobs"
echo "------------------------------"
echo "Current active jobs:"
rnx list
echo ""

echo "📋 Demo 4: Job Status Details"
echo "-----------------------------"
echo "Getting detailed status for running jobs"
echo ""

# Function to safely get job status
get_job_status() {
    local job_id=$1
    local job_name=$2
    echo "Status of $job_name:"
    if rnx status "$job_id" 2>/dev/null; then
        echo "✅ Status retrieved successfully"
    else
        echo "ℹ️  Job may have completed or ID not found"
    fi
    echo ""
}

# Note: In a real demo, we'd use actual job IDs returned from rnx run
echo "💡 Note: Job IDs would be shown here in a real demo"
echo "    Use 'rnx list' to get current job IDs, then 'rnx status <job-id>'"
echo ""

echo "📋 Demo 5: Real-time Job Monitoring"
echo "-----------------------------------"
echo "Starting a job and monitoring it in real-time"

echo "Starting monitored job (10 seconds with detailed output)..."
rnx run bash -c "
echo '=== Monitored Job Started ==='
echo 'Job ID: \$\$'
echo 'Start time:' \$(date)
echo ''

for i in {1..10}; do
    echo \"[\$(date '+%H:%M:%S')] Progress: \$i/10\"
    echo \"  - CPU usage: simulated\"
    echo \"  - Memory usage: \$(free -m 2>/dev/null | awk 'NR==2{printf \"%.1f%%\", \$3*100/\$2}' || echo 'unknown')\"
    echo \"  - Status: Running step \$i\"
    sleep 1
done

echo ''
echo '=== Monitored Job Completed ==='
echo 'End time:' \$(date)
echo 'Total duration: 10 seconds'
"
echo ""

echo "📋 Demo 6: Job Log Streaming"
echo "----------------------------"
echo "Demonstrating log streaming for running jobs"

echo "Starting job that produces logs over time..."
JOB_CMD="bash -c \"
echo 'Log streaming job started'
for i in {1..8}; do
    echo \\\"[\\\$(date '+%H:%M:%S')] Log entry \\\$i: Processing data batch \\\$i\\\"
    sleep 2
    echo \\\"[\\\$(date '+%H:%M:%S')] Batch \\\$i completed successfully\\\"
done
echo 'Log streaming job finished'
\""

# Run job and capture output
echo "Job output (streaming in real-time):"
rnx run $JOB_CMD
echo ""

echo "📋 Demo 7: Multiple Job Monitoring"
echo "----------------------------------"
echo "Running multiple jobs simultaneously and monitoring them"

echo "Starting 3 parallel jobs..."

# Start parallel jobs
rnx run bash -c "
echo 'Parallel Job A started'
for i in {1..6}; do
    echo \"Job A: Step \$i/6\"
    sleep 1
done
echo 'Parallel Job A completed'
" &

rnx run bash -c "
echo 'Parallel Job B started'  
for i in {1..6}; do
    echo \"Job B: Task \$i/6\"
    sleep 1
done
echo 'Parallel Job B completed'
" &

rnx run bash -c "
echo 'Parallel Job C started'
for i in {1..6}; do  
    echo \"Job C: Operation \$i/6\"
    sleep 1
done
echo 'Parallel Job C completed'
" &

echo "✅ Parallel jobs started"
echo ""

# Wait a moment for jobs to start
sleep 2

echo "Current job list during parallel execution:"
rnx list
echo ""

# Wait for jobs to complete
sleep 8

echo "📋 Demo 8: Job Completion Status"
echo "--------------------------------"
echo "Checking final status after jobs complete"
echo "Final job list:"
rnx list
echo ""

echo "📋 Demo 9: Job Management Commands"
echo "----------------------------------"
echo "Demonstrating job management capabilities"

echo "Starting a long-running job for management demo..."
MANAGE_JOB=$(timeout 25s rnx run bash -c "
echo 'Manageable job started (will run for 20 seconds)'
for i in {1..20}; do
    echo \"Manageable job: \$i/20 - \$(date '+%H:%M:%S')\"
    sleep 1
done
echo 'Manageable job completed (if not stopped)'
" &>/dev/null & echo $!)

echo "✅ Long-running job started in background"
echo ""

# Give it a moment to start
sleep 3

echo "Current jobs (showing our long-running job):"
rnx list
echo ""

echo "💡 Job management commands available:"
echo "   rnx list                    # List all jobs"
echo "   rnx status <job-id>         # Get job details"
echo "   rnx log <job-id>            # View job logs"
echo "   rnx log -f <job-id>         # Follow logs in real-time"
echo "   rnx stop <job-id>           # Stop a running job"
echo ""

# Wait for background job to finish
sleep 18

echo "📋 Demo 10: Job History and Cleanup"
echo "-----------------------------------"
echo "Understanding job lifecycle and cleanup"

echo "Final job status check:"
rnx list
echo ""

echo "Job lifecycle states you might see:"
echo "  • INITIALIZING - Job is being set up"
echo "  • RUNNING - Job is actively executing"
echo "  • COMPLETED - Job finished successfully"
echo "  • FAILED - Job encountered an error"  
echo "  • STOPPED - Job was manually stopped"
echo ""

echo "✅ Job Monitoring Demo Complete!"
echo ""
echo "🎓 What you learned:"
echo "  • How to list jobs with 'rnx list'"
echo "  • How to get job details with 'rnx status'"
echo "  • How to view job logs with 'rnx log'"
echo "  • Real-time log streaming capabilities"
echo "  • Managing multiple concurrent jobs"
echo "  • Job lifecycle states and monitoring"
echo "  • Background job execution and monitoring"
echo ""
echo "📝 Key takeaways:"
echo "  • Jobs have unique IDs for management and tracking"
echo "  • Real-time log streaming helps monitor job progress"
echo "  • Multiple jobs can run concurrently with proper resource limits"
echo "  • Job states provide insight into execution status"
echo "  • Logs are preserved for completed jobs (until cleanup)"
echo ""
echo "💡 Best practices:"
echo "  • Use meaningful output in your jobs for better monitoring"
echo "  • Include timestamps in job output for better tracking"
echo "  • Monitor resource usage to optimize job performance"
echo "  • Use appropriate resource limits to prevent system overload"
echo "  • Clean up completed jobs periodically"
echo "  • Use job naming (when available) for better organization"
echo ""
echo "🔧 Useful monitoring commands:"
echo "  rnx list                    # Quick overview of all jobs"
echo "  rnx status <job-id>         # Detailed job information"
echo "  rnx log <job-id>            # View complete job logs"
echo "  rnx log -f <job-id>         # Follow live job output"
echo "  rnx stop <job-id>           # Terminate running job"
echo ""
echo "➡️  Next: Try ./06_environment.sh to learn about environment variables"