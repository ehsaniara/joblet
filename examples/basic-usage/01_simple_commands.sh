#!/bin/bash
set -e

echo "💻 Joblet Basic Usage: Simple Commands"
echo "======================================"
echo ""
echo "This demo shows how to execute basic commands with Joblet."
echo ""

# Check if rnx is available
if ! command -v rnx &> /dev/null; then
    echo "❌ Error: 'rnx' command not found"
    echo "Please ensure Joblet RNX client is installed"
    exit 1
fi

# Test connection
echo "🔗 Testing connection to Joblet server..."
if ! rnx list &> /dev/null; then
    echo "❌ Error: Cannot connect to Joblet server"
    exit 1
fi
echo "✅ Connected to Joblet server"
echo ""

echo "📋 Demo 1: Basic Command Execution"
echo "----------------------------------"
echo "Running: echo 'Hello, Joblet!'"
rnx run echo "Hello, Joblet!"
echo ""

echo "📋 Demo 2: System Information"
echo "-----------------------------"
echo "Running: uname -a"
rnx run uname -a
echo ""

echo "📋 Demo 3: Directory Listing"
echo "----------------------------"
echo "Running: ls -la"
rnx run ls -la
echo ""

echo "📋 Demo 4: Current Working Directory"
echo "------------------------------------"
echo "Running: pwd"
rnx run pwd
echo ""

echo "📋 Demo 5: Environment Information"
echo "----------------------------------"
echo "Running: env | head -10"
rnx run bash -c "env | head -10"
echo ""

echo "📋 Demo 6: Process Information"
echo "------------------------------"
echo "Running: ps aux | head -5"
rnx run bash -c "ps aux | head -5"
echo ""

echo "📋 Demo 7: Multi-command Execution"
echo "----------------------------------"
echo "Running: Multiple commands in sequence"
rnx run bash -c "echo 'Current time:' && date && echo 'Uptime:' && uptime"
echo ""

echo "📋 Demo 8: Command with Error Handling"
echo "--------------------------------------"
echo "Running: Command that might fail (gracefully handled)"
if rnx run bash -c "echo 'This works' && echo 'Testing error handling' && ls /nonexistent 2>/dev/null || echo 'Error handled gracefully'"; then
    echo "✅ Command completed successfully"
else
    echo "ℹ️  Command completed with expected error handling"
fi
echo ""

echo "✅ Simple Commands Demo Complete!"
echo ""
echo "🎓 What you learned:"
echo "  • How to execute basic shell commands with 'rnx run'"
echo "  • Understanding the job execution environment"
echo "  • Working directory and environment basics"
echo "  • Error handling in command execution"
echo ""
echo "📝 Key takeaways:"
echo "  • Commands run in isolated environments"
echo "  • Each 'rnx run' creates a separate job"
echo "  • Standard shell commands work as expected"
echo "  • Jobs inherit basic environment settings"
echo ""
echo "➡️  Next: Try ./02_file_operations.sh to learn about file handling"