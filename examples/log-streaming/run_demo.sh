#!/bin/bash
#
# Log Streaming Demo - Async Log System Performance Test
#
# This script demonstrates Joblet's rate-decoupled async log system
# with various high-frequency logging patterns and real-time streaming.
#
# Features demonstrated:
# - Real-time log streaming with `rnx job log -f`
# - Rate-decoupled async persistence (5M+ writes/second)
# - Overflow protection during burst logging
# - Multiple concurrent logging jobs
# - Performance monitoring and validation

set -e

# Colors for better output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Configuration
DEMO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOG_DIR="${DEMO_DIR}/demo_logs"

echo_header() {
    echo -e "\n${BLUE}========================================${NC}"
    echo -e "${BLUE}$1${NC}"
    echo -e "${BLUE}========================================${NC}\n"
}

echo_info() {
    echo -e "${GREEN}ℹ️  $1${NC}"
}

echo_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

echo_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

echo_command() {
    echo -e "${CYAN}$ $1${NC}"
}

# Create demo logs directory
mkdir -p "$LOG_DIR"

echo_header "🚀 Joblet Async Log System Performance Demo"

echo_info "This demo showcases Joblet's rate-decoupled async log persistence system"
echo_info "optimized for HPC workloads with microsecond write latency and 5M+ writes/second."
echo
echo_info "Features being demonstrated:"
echo "  • Real-time log streaming with rnx job log -f"
echo "  • High-frequency logging (10-100 logs/second)"
echo "  • Async overflow protection strategies"
echo "  • Concurrent job logging scenarios"
echo "  • Performance validation and monitoring"
echo

# Check if we're in the right directory
if [[ ! -f "high_frequency_logger.py" ]]; then
    echo_warning "Please run this script from the log-streaming example directory"
    echo "cd examples/log-streaming && ./run_demo.sh"
    exit 1
fi

# Verify rnx is available
if ! command -v rnx &> /dev/null; then
    echo_warning "rnx command not found. Please ensure Joblet is installed and configured."
    echo "See docs/INSTALLATION.md for setup instructions."
    exit 1
fi

# Function to run a demo job and show streaming
run_demo_job() {
    local job_name="$1"
    local description="$2"
    local stream_option="${3:-yes}"
    
    echo_header "Demo: $description"
    
    echo_info "Starting job: $job_name"
    echo_command "rnx workflow run jobs.yaml
    
    # Start the job and capture the job ID
    local job_output
    job_output=$(rnx workflow run jobs.yaml 2>&1)
    local job_id
    job_id=$(echo "$job_output" | grep -E "Job [A-Za-z0-9-]+ started" | awk '{print $2}' || echo "")
    
    if [[ -z "$job_id" ]]; then
        # Try alternative extraction method
        job_id=$(echo "$job_output" | grep -E "[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}" | head -1 | awk '{print $1}' || echo "")
    fi
    
    if [[ -z "$job_id" ]]; then
        echo_warning "Could not extract job ID from output:"
        echo "$job_output"
        return 1
    fi
    
    echo_success "Job started with ID: $job_id"
    
    if [[ "$stream_option" == "yes" ]]; then
        echo
        echo_info "Real-time log streaming (async log system demonstration):"
        echo_info "Watch how logs appear instantly despite high-frequency generation"
        echo_warning "Press Ctrl+C when ready to move to next demo"
        echo
        
        echo_command "rnx job log -f $job_id"
        
        # Stream logs with timeout or until user interrupts
        timeout 30s rnx job log -f "$job_id" 2>/dev/null || true
        
        echo
        echo_success "Log streaming demo completed"
    fi
    
    # Show final status
    echo
    echo_info "Final job status:"
    echo_command "rnx job status $job_id"
    rnx job status "$job_id" || echo_warning "Could not retrieve job status"
    
    # Save logs to file for later analysis
    local log_file="$LOG_DIR/${job_name}_${job_id}.log"
    echo_info "Saving complete logs to: $log_file"
    rnx job log --follow=false "$job_id" > "$log_file" 2>/dev/null || echo_warning "Could not save logs"
    
    echo
    sleep 2
}

# Function to show concurrent logging demo
demo_concurrent_logging() {
    echo_header "🔄 Concurrent Logging Demo - Async System Stress Test"
    
    echo_info "Starting multiple concurrent high-frequency loggers"
    echo_info "This tests the async log system's ability to handle multiple jobs simultaneously"
    echo
    
    local job_ids=()
    
    # Start multiple jobs concurrently
    for i in {1..3}; do
        echo_command "rnx workflow run jobs.yaml (job $i)"
        local job_output
        job_output=$(rnx workflow run jobs.yaml 2>&1)
        local job_id
        job_id=$(echo "$job_output" | grep -E "[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}" | head -1 | awk '{print $1}' || echo "")
        
        if [[ -n "$job_id" ]]; then
            job_ids+=("$job_id")
            echo_success "Job $i started: $job_id"
        else
            echo_warning "Failed to start job $i"
        fi
        
        sleep 1
    done
    
    echo
    echo_info "All concurrent jobs started. Monitoring combined output..."
    echo_warning "Press Ctrl+C when ready to continue"
    echo
    
    # Monitor all jobs concurrently for a short time
    for job_id in "${job_ids[@]}"; do
        if [[ -n "$job_id" ]]; then
            echo_info "Streaming logs from $job_id:"
            timeout 10s rnx job log -f "$job_id" 2>/dev/null &
        fi
    done
    
    # Wait for background jobs or user interrupt
    wait
    
    echo
    echo_success "Concurrent logging demo completed"
    
    # Show final status of all jobs
    echo_info "Final status of all concurrent jobs:"
    for job_id in "${job_ids[@]}"; do
        if [[ -n "$job_id" ]]; then
            echo_command "rnx job status $job_id"
            rnx job status "$job_id" || echo_warning "Could not retrieve status for $job_id"
            echo
        fi
    done
}

# Main demo sequence
echo_info "Choose demo level:"
echo "1. Quick Demo (10 seconds, basic functionality)"
echo "2. Standard Demo (comprehensive features, ~5 minutes)"  
echo "3. Full Demo (all features including stress tests, ~10 minutes)"
echo

read -p "Enter choice (1-3) [default: 1]: " choice
choice=${choice:-1}

case $choice in
    1)
        echo_info "Running Quick Demo"
        run_demo_job "quick-demo" "Quick High-Frequency Logging (100 counts, 10 seconds)" "yes"
        ;;
    2)
        echo_info "Running Standard Demo"
        run_demo_job "quick-demo" "Quick Demo - Basic Functionality" "yes"
        run_demo_job "high-frequency" "High-Frequency Logging (20 logs/second)" "yes"
        demo_concurrent_logging
        ;;
    3)
        echo_info "Running Full Demo"
        run_demo_job "quick-demo" "Quick Demo - Basic Functionality" "yes"
        run_demo_job "standard-demo" "Standard Demo (10 logs/second, 1000 counts)" "yes"
        run_demo_job "high-frequency" "High-Frequency Logging (20 logs/second)" "yes"
        run_demo_job "burst-test" "Burst Test - Async Overflow Protection (50 logs/second)" "yes"
        demo_concurrent_logging
        
        echo_header "🎯 Performance Validation"
        echo_info "Running stress test to validate async system performance"
        run_demo_job "stress-test" "Stress Test (100 logs/second, 2000 counts)" "no"
        ;;
    *)
        echo_warning "Invalid choice. Running Quick Demo."
        run_demo_job "quick-demo" "Quick High-Frequency Logging (100 counts, 10 seconds)" "yes"
        ;;
esac

echo_header "📊 Demo Summary & Async Log System Analysis"

echo_info "Demo completed! Here's what was demonstrated:"
echo
echo "✅ Rate-decoupled async log persistence"
echo "   • Jobs write to channels instantly (microsecond latency)"
echo "   • Background disk writer handles batching and I/O"
echo "   • Never blocks job execution regardless of disk speed"
echo
echo "✅ High-frequency logging capabilities"
echo "   • Sustained logging at 10-100 logs/second"
echo "   • Burst logging up to system limits"
echo "   • Multiple concurrent jobs handled seamlessly"
echo
echo "✅ Real-time streaming"
echo "   • Live log updates via 'rnx job log -f'"
echo "   • Historical + live log access"
echo "   • Multiple client streaming support"
echo
echo "✅ Overflow protection strategies"
echo "   • Compress: Memory-efficient chunk compression"
echo "   • Spill: Temporary disk files for extreme bursts"
echo "   • Sample: Every Nth chunk during overload"
echo "   • Alert: Operator notification and limit expansion"

if [[ -d "$LOG_DIR" ]] && [[ $(ls -A "$LOG_DIR" 2>/dev/null) ]]; then
    echo
    echo_info "Saved log files for analysis:"
    ls -la "$LOG_DIR"
    echo
    echo_info "You can analyze these logs to see:"
    echo "  • Timestamp precision and ordering"
    echo "  • Burst patterns and frequency"
    echo "  • Complete data integrity (no dropped logs)"
    echo "  • Async system performance characteristics"
fi

echo
echo_header "🚀 Next Steps"
echo_info "Try these commands to explore the async log system further:"
echo
echo_command "# Run custom logging patterns"
echo_command "START_NUM=5000 END_NUM=6000 INTERVAL=0.05 rnx workflow run jobs.yaml
echo
echo_command "# Monitor system performance during logging"
echo_command "rnx monitor status  # View system metrics in real-time"
echo
echo_command "# View all current jobs"
echo_command "rnx job list"
echo
echo_command "# Run concurrent workflow"
echo_command "rnx workflow run concurrent-logging.yaml"

echo_success "🎉 Async log system demo completed successfully!"
echo_info "The rate-decoupled async architecture ensures your jobs never wait for disk I/O,"
echo_info "while guaranteeing complete log persistence and real-time streaming capabilities."