#!/bin/bash
set -e

echo "🚀 JOBLET COMPREHENSIVE DEMO"
echo "============================"
echo ""
echo "This script runs all Joblet examples to demonstrate:"
echo "• Python Analytics & Machine Learning"
echo "• Node.js Applications & Microservices"  
echo "• Agentic AI Foundations"
echo ""

# Check if rnx is available
if ! command -v rnx &> /dev/null; then
    echo "❌ Error: 'rnx' command not found"
    echo "Please ensure Joblet RNX client is installed and configured"
    echo ""
    echo "Installation help:"
    echo "  - Check if RNX is in your PATH"
    echo "  - Verify Joblet installation"
    echo "  - See: https://github.com/ehsaniara/joblet"
    exit 1
fi

# Check connection to Joblet server
echo "🔍 Checking Joblet server connection..."
if ! rnx list &> /dev/null; then
    echo "❌ Error: Cannot connect to Joblet server"
    echo "Please ensure:"
    echo "  - Joblet daemon is running on the server"
    echo "  - RNX client is properly configured (rnx-config.yml)"
    echo "  - Network connectivity to Joblet server"
    echo "  - Valid certificates if using mTLS"
    echo ""
    echo "Debug steps:"
    echo "  1. Check server status: systemctl status joblet"
    echo "  2. Verify config: rnx nodes"
    echo "  3. Test connectivity: ping <server-host>"
    exit 1
fi

# Check system resources
echo "📊 Checking system resources..."
if command -v free &> /dev/null; then
    TOTAL_RAM=$(free -m | awk 'NR==2{printf "%.0f", $2}')
    AVAILABLE_RAM=$(free -m | awk 'NR==2{printf "%.0f", $7}')
    echo "System RAM: ${TOTAL_RAM}MB total, ${AVAILABLE_RAM}MB available"
    
    if [ "$AVAILABLE_RAM" -lt 8192 ]; then
        echo "⚠️  Warning: Available RAM (${AVAILABLE_RAM}MB) may be insufficient"
        echo "   Recommended: 8GB+ available RAM for all demos"
        echo "   Consider running individual demo suites instead"
        echo ""
        read -p "Continue anyway? (y/N): " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            echo "Demo cancelled. Try individual demo scripts:"
            echo "  cd python-analytics && ./run_demos.sh"
            echo "  cd nodejs && ./run_demos.sh"
            echo "  cd agentic-ai && ./run_demos.sh"
            exit 1
        fi
    fi
fi

# Check disk space for volumes
if command -v df &> /dev/null; then
    AVAILABLE_DISK=$(df -m . | awk 'NR==2 {print $4}')
    if [ "$AVAILABLE_DISK" -lt 10240 ]; then
        echo "⚠️  Warning: Available disk space (${AVAILABLE_DISK}MB) may be insufficient"
        echo "   Demos will create volumes totaling ~10GB"
    fi
fi

echo "✅ Connected to Joblet server"
echo ""

# Function to run demo with comprehensive error handling
run_demo() {
    local demo_name="$1"
    local demo_path="$2"
    local demo_script="$3"
    
    echo "🎬 Starting: $demo_name"
    echo "----------------------------------------"
    
    # Check if demo directory exists
    if [ ! -d "$demo_path" ]; then
        echo "❌ Error: Demo directory not found: $demo_path"
        echo "Please ensure you're running this script from the examples/ directory"
        return 1
    fi
    
    cd "$demo_path"
    
    # Check if demo script exists and is executable
    if [ ! -f "$demo_script" ]; then
        echo "❌ Demo script not found: $demo_script"
        cd - > /dev/null
        return 1
    fi
    
    if [ ! -x "$demo_script" ]; then
        echo "🔧 Making demo script executable..."
        chmod +x "$demo_script" || {
            echo "❌ Error: Cannot make script executable"
            cd - > /dev/null
            return 1
        }
    fi
    
    # Run the demo with timeout to prevent hanging
    local start_time=$(date +%s)
    echo "🕰️ Started at: $(date)"
    
    if timeout 1800 bash "$demo_script"; then  # 30-minute timeout
        local end_time=$(date +%s)
        local duration=$((end_time - start_time))
        echo "✅ $demo_name completed successfully in ${duration}s"
    else
        local exit_code=$?
        local end_time=$(date +%s)
        local duration=$((end_time - start_time))
        
        if [ $exit_code -eq 124 ]; then
            echo "⏰ $demo_name timed out after ${duration}s"
            echo "   This may indicate resource constraints or hanging processes"
        else
            echo "❌ $demo_name failed after ${duration}s (exit code: $exit_code)"
            echo "   Check demo logs for specific error details"
        fi
        
        cd - > /dev/null
        return 1
    fi
    
    echo ""
    cd - > /dev/null
    return 0
}

# Get the directory of this script
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Track demo results
DEMO_RESULTS=()
FAILED_DEMOS=()
SUCCESSFUL_DEMOS=()

echo "📊 Demo 1: Python Analytics & Machine Learning"
echo "==============================================="
if run_demo "Python Analytics" "$SCRIPT_DIR/python-analytics" "run_demos.sh"; then
    SUCCESSFUL_DEMOS+=("Python Analytics")
else
    FAILED_DEMOS+=("Python Analytics")
    echo "⚠️  Python Analytics demo failed - continuing with other demos..."
fi

echo "🟨 Demo 2: Node.js Applications & Microservices"
echo "==============================================="
if run_demo "Node.js Applications" "$SCRIPT_DIR/nodejs" "run_demos.sh"; then
    SUCCESSFUL_DEMOS+=("Node.js Applications")
else
    FAILED_DEMOS+=("Node.js Applications")
    echo "⚠️  Node.js Applications demo failed - continuing with other demos..."
fi

echo "🤖 Demo 3: Agentic AI Foundations"
echo "================================="
if run_demo "Agentic AI" "$SCRIPT_DIR/agentic-ai" "run_demos.sh"; then
    SUCCESSFUL_DEMOS+=("Agentic AI")
else
    FAILED_DEMOS+=("Agentic AI")
    echo "⚠️  Agentic AI demo failed - this is expected if system resources are limited..."
fi

echo ""
echo "🎉 DEMO SUITE COMPLETED!"
echo "======================"
echo ""
echo "📊 Results Summary:"
echo "  ✅ Successful: ${#SUCCESSFUL_DEMOS[@]} demos"
echo "  ❌ Failed: ${#FAILED_DEMOS[@]} demos"
echo ""

if [ ${#SUCCESSFUL_DEMOS[@]} -gt 0 ]; then
    echo "✅ Successful demos:"
    for demo in "${SUCCESSFUL_DEMOS[@]}"; do
        echo "    - $demo"
    done
    echo ""
fi

if [ ${#FAILED_DEMOS[@]} -gt 0 ]; then
    echo "❌ Failed demos:"
    for demo in "${FAILED_DEMOS[@]}"; do
        echo "    - $demo"
    done
    echo ""
    echo "💡 Troubleshooting failed demos:"
    echo "  1. Check system resources (RAM, disk space)"
    echo "  2. Verify Joblet server environment has required packages"
    echo "  3. Run individual demo scripts for detailed error messages"
    echo "  4. Check 'rnx list' and 'rnx log <job-id>' for job-specific errors"
    echo ""
fi

if [ ${#SUCCESSFUL_DEMOS[@]} -eq 3 ]; then
    echo "🎆 ALL DEMOS COMPLETED SUCCESSFULLY!"
else
    echo "⚠️  Some demos failed - see details above"
fi
echo ""
echo "📋 Summary of what was demonstrated:"
echo ""
echo "Python Analytics:"
echo "  ✓ Sales data analysis with Pandas & Matplotlib"
echo "  ✓ Machine learning customer segmentation with scikit-learn"
echo "  ✓ Distributed feature engineering across multiple jobs"
echo ""
echo "Node.js Applications:"
echo "  ✓ API testing suite with comprehensive test coverage"
echo "  ✓ Microservice deployment with Express.js"
echo "  ✓ Data processing with CSV streams and transformations"
echo "  ✓ Complete CI/CD build pipeline"
echo "  ✓ Real-time event processing simulation"
echo ""
echo "Agentic AI:"
echo "  ✓ LLM inference service with caching and metrics"
echo "  ✓ Multi-agent coordination system with specialized agents"
echo "  ✓ RAG (Retrieval-Augmented Generation) with vector database"
echo "  ✓ Distributed AI model training simulation"
echo "  ✓ End-to-end AI pipeline orchestration"
echo ""
echo "📁 Results Location:"
echo "All demo outputs, metrics, and artifacts are stored in Joblet volumes:"
echo ""
echo "  📈 Analytics Results:"
echo "    rnx run --volume=analytics-data ls -la /volumes/analytics-data/results/"
echo "    rnx run --volume=ml-models ls -la /volumes/ml-models/"
echo ""
echo "  🟨 Node.js Results:"
echo "    rnx run --volume=nodejs-projects ls -la /volumes/nodejs-projects/"
echo ""
echo "  🤖 AI Results:"
echo "    rnx run --volume=ai-outputs ls -la /volumes/ai-outputs/"
echo "    rnx run --volume=ai-metrics ls -la /volumes/ai-metrics/"
echo ""
echo "🔍 Inspect Specific Results:"
echo "  rnx run --volume=analytics-data cat /volumes/analytics-data/results/monthly_sales.csv"
echo "  rnx run --volume=nodejs-projects cat /volumes/nodejs-projects/reports/test-report.txt"
echo "  rnx run --volume=ai-outputs cat /volumes/ai-outputs/inference_results_*.json"
echo ""
echo "📊 Monitor System:"
echo "  rnx list                    # View all jobs"
echo "  rnx monitor                 # Real-time system monitoring"
echo "  rnx volume list             # View all volumes and usage"
echo "  rnx network list            # View network configurations"
echo ""
echo "🎯 These examples demonstrate Joblet's capabilities for:"
echo "  • Secure job isolation with resource limits"
echo "  • Persistent data storage with volumes"
echo "  • Scalable distributed processing"
echo "  • Real-time log streaming and monitoring"
echo "  • Integration with modern AI/ML workflows"
echo ""
echo "Ready for production use in agentic AI foundations! 🚀"