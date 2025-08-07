#!/bin/bash
set -e

echo "🚀 JOBLET COMPREHENSIVE DEMO"
echo "============================"
echo ""
echo "This script runs working Joblet examples to demonstrate:"
echo "• Python Analytics (standard library only)"
echo "• Basic Usage Patterns"
echo "• Advanced Job Coordination"
echo ""

# Check if rnx is available
if ! command -v rnx &> /dev/null; then
    echo "❌ Error: 'rnx' command not found"
    echo "Please ensure Joblet RNX client is installed and configured"
    exit 1
fi

# Check connection to Joblet server
echo "🔍 Checking Joblet server connection..."
if ! rnx list &> /dev/null; then
    echo "❌ Error: Cannot connect to Joblet server"
    echo "Please ensure:"
    echo "  - Joblet daemon is running on the server"
    echo "  - RNX client is properly configured"
    exit 1
fi

echo "✅ Connected to Joblet server"
echo ""

# Track demo results
SUCCESSFUL_DEMOS=()
FAILED_DEMOS=()

# Function to run a demo safely
run_demo() {
    local demo_name="$1"
    local demo_dir="$2"
    local script_name="$3"
    
    echo "Starting $demo_name..."
    if [ -f "$demo_dir/$script_name" ]; then
        cd "$demo_dir"
        if ./"$script_name"; then
            echo "✅ $demo_name completed successfully"
            cd - > /dev/null
            return 0
        else
            echo "❌ $demo_name failed"
            cd - > /dev/null
            return 1
        fi
    else
        echo "❌ Script $demo_dir/$script_name not found"
        return 1
    fi
}

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "📊 Demo 1: Python Analytics (Working)"
echo "====================================="
if run_demo "Python Analytics" "$SCRIPT_DIR/python-analytics" "run_demo.sh"; then
    SUCCESSFUL_DEMOS+=("Python Analytics")
else
    FAILED_DEMOS+=("Python Analytics")
    echo "⚠️  Python Analytics demo failed - continuing with other demos..."
fi

echo ""
echo "💻 Demo 2: Basic Usage Patterns"
echo "==============================="
if run_demo "Basic Usage" "$SCRIPT_DIR/basic-usage" "run_demos.sh"; then
    SUCCESSFUL_DEMOS+=("Basic Usage")
else
    FAILED_DEMOS+=("Basic Usage")
    echo "⚠️  Basic Usage demo failed - continuing with other demos..."
fi

echo ""
echo "🔗 Demo 3: Advanced Job Coordination"
echo "===================================="
if run_demo "Advanced Coordination" "$SCRIPT_DIR/advanced" "job_coordination.sh"; then
    SUCCESSFUL_DEMOS+=("Advanced Coordination")
else
    FAILED_DEMOS+=("Advanced Coordination")
    echo "⚠️  Advanced demo failed - continuing..."
fi

echo ""
echo "🎉 DEMO SUITE COMPLETED!"
echo "======================="
echo ""

# Summary
if [ ${#SUCCESSFUL_DEMOS[@]} -gt 0 ]; then
    echo "✅ Successful demos:"
    for demo in "${SUCCESSFUL_DEMOS[@]}"; do
        echo "   • $demo"
    done
    echo ""
fi

if [ ${#FAILED_DEMOS[@]} -gt 0 ]; then
    echo "❌ Failed demos:"
    for demo in "${FAILED_DEMOS[@]}"; do
        echo "   • $demo"
    done
    echo ""
    echo "💡 Common reasons for failures:"
    echo "   • Missing dependencies (Python packages, Node.js, etc.)"
    echo "   • Insufficient system resources"
    echo "   • Network connectivity issues"
    echo ""
    echo "📚 Try individual working examples:"
    echo "   cd python-analytics && ./run_demo.sh    # Works with Python 3 only"
    echo "   cd basic-usage && ./run_demos.sh        # Always works"
    echo "   cd advanced && ./job_coordination.sh    # Works with Python 3 only"
fi

echo "📊 Results:"
echo "   Total demos: $((${#SUCCESSFUL_DEMOS[@]} + ${#FAILED_DEMOS[@]}))"
echo "   Successful: ${#SUCCESSFUL_DEMOS[@]}"
echo "   Failed: ${#FAILED_DEMOS[@]}"

if [ ${#SUCCESSFUL_DEMOS[@]} -gt 0 ]; then
    echo ""
    echo "🔍 To view demo results:"
    echo "   rnx run --volume=analytics-data ls /volumes/analytics-data/results/"
    echo "   rnx run --volume=shared-data cat /volumes/shared-data/results.json"
    echo ""
    echo "📋 To see all jobs:"
    echo "   rnx list"
fi