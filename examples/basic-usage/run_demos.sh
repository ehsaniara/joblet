#!/bin/bash

echo "🚀 Joblet Basic Usage - Complete Demo Suite"
echo "============================================"
echo ""
echo "This script runs all basic usage examples in sequence to demonstrate"
echo "Joblet's core features and capabilities."
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Demo tracking
DEMO_COUNT=0
DEMO_SUCCESS=0
DEMO_ERRORS=0
START_TIME=$(date +%s)

# Function to run demo with error handling
run_demo() {
    local demo_script=$1
    local demo_name=$2
    
    echo -e "${CYAN}🎯 Starting: $demo_name${NC}"
    echo "========================================================"
    
    DEMO_COUNT=$((DEMO_COUNT + 1))
    
    if [ -f "$demo_script" ]; then
        if bash "$demo_script"; then
            echo -e "${GREEN}✅ $demo_name completed successfully${NC}"
            DEMO_SUCCESS=$((DEMO_SUCCESS + 1))
        else
            echo -e "${RED}❌ $demo_name failed${NC}"
            DEMO_ERRORS=$((DEMO_ERRORS + 1))
        fi
    else
        echo -e "${RED}❌ Demo script not found: $demo_script${NC}"
        DEMO_ERRORS=$((DEMO_ERRORS + 1))
    fi
    
    echo ""
    echo -e "${BLUE}Press Enter to continue to next demo (or Ctrl+C to exit)${NC}"
    read -r
    echo ""
}

# Check prerequisites
echo "🔍 Checking Prerequisites"
echo "========================="

# Check if we're in the right directory
if [ ! -f "01_simple_commands.sh" ]; then
    echo -e "${RED}❌ Error: Please run this script from the basic-usage directory${NC}"
    echo "   Expected files: 01_simple_commands.sh, 02_file_operations.sh, etc."
    exit 1
fi

# Check RNX client
if ! command -v rnx &> /dev/null; then
    echo -e "${RED}❌ Error: 'rnx' command not found${NC}"
    echo "   Please install Joblet RNX client first"
    exit 1
fi

# Check server connection
if ! rnx list &> /dev/null; then
    echo -e "${RED}❌ Error: Cannot connect to Joblet server${NC}"
    echo "   Please ensure Joblet server is running and accessible"
    exit 1
fi

echo -e "${GREEN}✅ All prerequisites met${NC}"
echo ""

# System information
echo "🖥️  System Information"
echo "====================="
echo "   OS: $(uname -s) $(uname -r)"
echo "   Architecture: $(uname -m)"
echo "   Shell: $SHELL"
echo "   Date: $(date)"
echo ""

# Server information
echo "🌐 Joblet Server Information"
echo "============================"
echo "   Active jobs: $(rnx list 2>/dev/null | wc -l) (including header)"
echo "   Volumes: $(rnx volume list 2>/dev/null | wc -l) (including header)"
echo ""

echo -e "${YELLOW}🎬 Starting Basic Usage Demo Suite${NC}"
echo "==================================="
echo ""
echo "This demo will walk through 7 core areas of Joblet functionality:"
echo "1. Simple Commands - Basic command execution"
echo "2. File Operations - File upload and workspace usage"
echo "3. Resource Limits - CPU, memory, and I/O management"
echo "4. Volume Storage - Persistent data storage"
echo "5. Job Monitoring - Status tracking and log viewing"
echo "6. Environment Variables - Configuration management"
echo "7. Network Basics - Network isolation and connectivity"
echo ""
echo "Each demo section is interactive and educational."
echo ""
echo -e "${BLUE}Press Enter to start the demo suite (or Ctrl+C to exit)${NC}"
read -r
echo ""

# Run all demos in sequence
run_demo "01_simple_commands.sh" "Simple Commands"
run_demo "02_file_operations.sh" "File Operations"
run_demo "03_resource_limits.sh" "Resource Management"
run_demo "04_volume_storage.sh" "Volume Storage"
run_demo "05_job_monitoring.sh" "Job Monitoring"
run_demo "06_environment.sh" "Environment Variables"
run_demo "07_network_basics.sh" "Network Basics"

# Calculate completion time
END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))
MINUTES=$((DURATION / 60))
SECONDS=$((DURATION % 60))

# Final summary
echo "🎉 Basic Usage Demo Suite Complete!"
echo "====================================="
echo ""
echo "📊 Demo Summary:"
echo "   Total demos run: $DEMO_COUNT"
echo "   Successful: $DEMO_SUCCESS"
echo "   Errors: $DEMO_ERRORS"
echo "   Duration: ${MINUTES}m ${SECONDS}s"
echo ""

if [ $DEMO_ERRORS -eq 0 ]; then
    echo -e "${GREEN}🎊 All demos completed successfully!${NC}"
    echo ""
    echo "🎓 You've learned the fundamentals of Joblet:"
    echo "   ✅ Basic command execution with job isolation"
    echo "   ✅ File upload and workspace management"
    echo "   ✅ Resource limits and performance control"
    echo "   ✅ Persistent data storage with volumes"
    echo "   ✅ Job monitoring and lifecycle management"
    echo "   ✅ Configuration with environment variables"
    echo "   ✅ Network isolation and connectivity options"
else
    echo -e "${YELLOW}⚠️  Demo completed with $DEMO_ERRORS issues${NC}"
    echo ""
    echo "Some demos may have encountered expected limitations based on:"
    echo "   • Available system resources"
    echo "   • Joblet server configuration"
    echo "   • Network connectivity"
    echo "   • Missing tools in server environment"
fi

echo ""
echo "🚀 Next Steps:"
echo "=============="
echo ""
echo "1. 📚 Explore Advanced Examples:"
echo "   cd ../python-analytics/    # Data science workflows"
echo "   cd ../nodejs/              # API testing and microservices"
echo "   cd ../agentic-ai/          # AI agent systems and LLM inference"
echo ""
echo "2. 🔧 Try Your Own Jobs:"
echo "   rnx run echo \"Hello, I understand Joblet now!\""
echo "   rnx run --max-memory=256 --upload=myfile.txt python3 process.py"
echo "   rnx volume create mydata --size=1GB --type=filesystem"
echo ""
echo "3. 📖 Learn More:"
echo "   • Check the main project README.md"
echo "   • Review individual demo scripts for detailed examples"
echo "   • Explore Joblet documentation and configuration options"
echo ""
echo "4. 🏗️  Production Usage:"
echo "   • Set appropriate resource limits for your workloads"
echo "   • Use volumes for data that must persist"
echo "   • Consider network security requirements"
echo "   • Monitor job performance and resource usage"
echo ""

# Cleanup check
echo "🧹 Cleanup Options:"
echo "==================="
echo ""
echo "The demos may have created some persistent resources:"
echo ""
echo "Check volumes:"
rnx volume list
echo ""
echo "To clean up demo volumes (optional):"
echo "   rnx volume remove demo-data    # Remove persistent demo volume"
echo ""
echo "Check current jobs:"
rnx list
echo ""

# Final message
echo "🎖️  Congratulations!"
echo "==================="
echo ""
echo "You've successfully completed the Joblet Basic Usage demo suite!"
echo ""
echo "You now have hands-on experience with:"
echo "   • Job execution and isolation"
echo "   • Resource management and limits"
echo "   • File handling and persistent storage"
echo "   • Job monitoring and configuration"
echo "   • Network security and isolation"
echo ""
echo "You're ready to use Joblet for your own workloads!"
echo ""
echo -e "${GREEN}🚀 Happy job processing with Joblet! 🚀${NC}"