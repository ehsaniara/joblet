#!/bin/bash

echo "🔍 Joblet Demo Host Compatibility Check"
echo "======================================="
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Track overall compatibility
COMPATIBILITY_ISSUES=0
WARNINGS=0

echo "🖥️  Host System Information:"
echo "   OS: $(uname -s) $(uname -r)"
echo "   Architecture: $(uname -m)"
echo "   Hostname: $(hostname)"
echo ""

# Check RNX client
echo "🔧 Checking Joblet RNX Client:"
if command -v rnx &> /dev/null; then
    RNX_VERSION=$(rnx --version 2>/dev/null || echo "unknown")
    echo -e "   ${GREEN}✅${NC} RNX client found: $RNX_VERSION"
else
    echo -e "   ${RED}❌${NC} RNX client not found in PATH"
    echo "      Install from: https://github.com/ehsaniara/joblet"
    COMPATIBILITY_ISSUES=$((COMPATIBILITY_ISSUES + 1))
fi

# Check Joblet server connection
echo ""
echo "🌐 Checking Joblet Server Connection:"
if command -v rnx &> /dev/null; then
    if rnx list &> /dev/null; then
        echo -e "   ${GREEN}✅${NC} Connected to Joblet server"
        
        # Get server info if possible
        SERVER_JOBS=$(rnx list 2>/dev/null | wc -l)
        echo "      Active jobs: $((SERVER_JOBS - 1))"  # Subtract header line
        
        # Check volumes
        VOLUME_COUNT=$(rnx volume list 2>/dev/null | wc -l)
        if [ $VOLUME_COUNT -gt 1 ]; then
            echo "      Existing volumes: $((VOLUME_COUNT - 1))"
        fi
    else
        echo -e "   ${RED}❌${NC} Cannot connect to Joblet server"
        echo "      Check: server running, config file, network connectivity"
        COMPATIBILITY_ISSUES=$((COMPATIBILITY_ISSUES + 1))
    fi
else
    echo -e "   ${YELLOW}⚠️${NC} Cannot check server (RNX not available)"
    WARNINGS=$((WARNINGS + 1))
fi

# Check system resources
echo ""
echo "💾 Checking System Resources:"

# Memory check
if command -v free &> /dev/null; then
    TOTAL_RAM=$(free -m | awk 'NR==2{printf "%.0f", $2}')
    AVAILABLE_RAM=$(free -m | awk 'NR==2{printf "%.0f", $7}')
    
    echo "   Memory:"
    echo "      Total: ${TOTAL_RAM}MB"
    echo "      Available: ${AVAILABLE_RAM}MB"
    
    if [ "$AVAILABLE_RAM" -ge 8192 ]; then
        echo -e "      ${GREEN}✅${NC} Sufficient for all demos (8GB+ recommended)"
    elif [ "$AVAILABLE_RAM" -ge 4096 ]; then
        echo -e "      ${YELLOW}⚠️${NC} Sufficient for individual demos (8GB+ recommended for all)"
        WARNINGS=$((WARNINGS + 1))
    else
        echo -e "      ${RED}❌${NC} Insufficient memory (4GB+ required, 8GB+ recommended)"
        COMPATIBILITY_ISSUES=$((COMPATIBILITY_ISSUES + 1))
    fi
elif command -v vm_stat &> /dev/null; then
    # macOS memory check
    FREE_PAGES=$(vm_stat | grep "Pages free" | awk '{print $3}' | sed 's/\.//')
    FREE_MB=$((FREE_PAGES * 4096 / 1024 / 1024))
    echo "   Memory:"
    echo "      Available: ~${FREE_MB}MB (macOS estimate)"
    
    if [ "$FREE_MB" -ge 4096 ]; then
        echo -e "      ${GREEN}✅${NC} Sufficient memory available"
    else
        echo -e "      ${YELLOW}⚠️${NC} Memory may be limited for larger demos"
        WARNINGS=$((WARNINGS + 1))
    fi
else
    echo -e "   ${YELLOW}⚠️${NC} Cannot check memory (platform not supported)"
    WARNINGS=$((WARNINGS + 1))
fi

# Disk space check
if command -v df &> /dev/null; then
    AVAILABLE_DISK=$(df -m . | awk 'NR==2 {print $4}')
    echo "   Disk Space:"
    echo "      Available: ${AVAILABLE_DISK}MB"
    
    if [ "$AVAILABLE_DISK" -ge 10240 ]; then
        echo -e "      ${GREEN}✅${NC} Sufficient disk space (10GB+ recommended)"
    elif [ "$AVAILABLE_DISK" -ge 5120 ]; then
        echo -e "      ${YELLOW}⚠️${NC} Limited disk space (10GB+ recommended)"
        WARNINGS=$((WARNINGS + 1))
    else
        echo -e "      ${RED}❌${NC} Insufficient disk space (5GB+ required)"
        COMPATIBILITY_ISSUES=$((COMPATIBILITY_ISSUES + 1))
    fi
fi

# Check for required tools
echo ""
echo "🛠️  Checking Required Tools:"

# Basic shell tools
TOOLS=("bash" "curl" "grep" "awk" "sed" "timeout")
for tool in "${TOOLS[@]}"; do
    if command -v $tool &> /dev/null; then
        echo -e "   ${GREEN}✅${NC} $tool"
    else
        echo -e "   ${RED}❌${NC} $tool (required for demo scripts)"
        COMPATIBILITY_ISSUES=$((COMPATIBILITY_ISSUES + 1))
    fi
done

# Check Joblet server environment (if connected)
if command -v rnx &> /dev/null && rnx list &> /dev/null; then
    echo ""
    echo "🐍 Checking Joblet Server Environment:"
    
    # Python availability
    if rnx run --max-memory=128 python3 --version &> /dev/null; then
        PYTHON_VERSION=$(rnx run --max-memory=128 python3 --version 2>&1 | cut -d' ' -f2)
        echo -e "   ${GREEN}✅${NC} Python 3: $PYTHON_VERSION"
        
        # Check Python packages
        if rnx run --max-memory=128 python3 -c "import pandas, numpy, matplotlib, sklearn" &> /dev/null; then
            echo -e "   ${GREEN}✅${NC} Python ML packages (pandas, numpy, matplotlib, sklearn)"
        else
            echo -e "   ${YELLOW}⚠️${NC} Python ML packages missing (required for Python analytics demos)"
            WARNINGS=$((WARNINGS + 1))
        fi
    else
        echo -e "   ${RED}❌${NC} Python 3 not available in Joblet server environment"
        COMPATIBILITY_ISSUES=$((COMPATIBILITY_ISSUES + 1))
    fi
    
    # Node.js availability
    if rnx run --max-memory=128 node --version &> /dev/null; then
        NODE_VERSION=$(rnx run --max-memory=128 node --version 2>&1)
        echo -e "   ${GREEN}✅${NC} Node.js: $NODE_VERSION"
        
        if rnx run --max-memory=128 npm --version &> /dev/null; then
            NPM_VERSION=$(rnx run --max-memory=128 npm --version 2>&1)
            echo -e "   ${GREEN}✅${NC} npm: $NPM_VERSION"
        else
            echo -e "   ${YELLOW}⚠️${NC} npm not available (required for Node.js demos)"
            WARNINGS=$((WARNINGS + 1))
        fi
    else
        echo -e "   ${RED}❌${NC} Node.js not available in Joblet server environment"
        COMPATIBILITY_ISSUES=$((COMPATIBILITY_ISSUES + 1))
    fi
    
    # Basic shell commands in server
    if rnx run --max-memory=128 bash -c "echo 'Shell available'" &> /dev/null; then
        echo -e "   ${GREEN}✅${NC} Shell (bash) available in server"
    else
        echo -e "   ${RED}❌${NC} Shell not available in Joblet server environment"
        COMPATIBILITY_ISSUES=$((COMPATIBILITY_ISSUES + 1))
    fi
fi

# Check network connectivity (if we can reach common sites)
echo ""
echo "🌍 Checking Network Connectivity:"
if command -v curl &> /dev/null; then
    if curl -s --connect-timeout 5 https://www.google.com > /dev/null; then
        echo -e "   ${GREEN}✅${NC} Internet connectivity available"
    else
        echo -e "   ${YELLOW}⚠️${NC} Limited internet connectivity (may affect some demos)"
        WARNINGS=$((WARNINGS + 1))
    fi
else
    echo -e "   ${YELLOW}⚠️${NC} Cannot check network connectivity (curl not available)"
    WARNINGS=$((WARNINGS + 1))
fi

# Final compatibility report
echo ""
echo "📊 Compatibility Summary:"
echo "========================="

if [ $COMPATIBILITY_ISSUES -eq 0 ]; then
    if [ $WARNINGS -eq 0 ]; then
        echo -e "${GREEN}🎉 FULLY COMPATIBLE${NC}"
        echo "   All demos should run successfully!"
        echo ""
        echo "✨ Ready to run:"
        echo "   ./run_all_demos.sh              # All demos"
        echo "   cd python-analytics && ./run_demos.sh  # Python only"
        echo "   cd nodejs && ./run_demos.sh     # Node.js only"
        echo "   cd agentic-ai && ./run_demos.sh # AI only"
    else
        echo -e "${YELLOW}⚠️  MOSTLY COMPATIBLE${NC}"
        echo "   Demos should run with $WARNINGS minor issues"
        echo "   Consider addressing warnings for optimal experience"
    fi
else
    echo -e "${RED}❌ COMPATIBILITY ISSUES FOUND${NC}"
    echo "   $COMPATIBILITY_ISSUES critical issues must be resolved"
    if [ $WARNINGS -gt 0 ]; then
        echo "   $WARNINGS additional warnings"
    fi
    echo ""
    echo "🔧 Required actions:"
    echo "   1. Install missing required tools"
    echo "   2. Ensure Joblet server is running with required environments"
    echo "   3. Verify system has sufficient resources"
    echo "   4. Re-run this check after fixes"
fi

echo ""
echo "📚 Need help?"
echo "   • Joblet Documentation: https://github.com/ehsaniara/joblet"
echo "   • Demo Setup Guide: ./DEMO_SETUP.md"
echo "   • Individual demo READMEs in each subdirectory"

# Exit with appropriate code
if [ $COMPATIBILITY_ISSUES -gt 0 ]; then
    exit 1
else
    exit 0
fi