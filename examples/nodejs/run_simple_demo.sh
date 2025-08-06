#!/bin/bash
set -e

echo "🚀 Node.js Simple Demo"
echo "======================"
echo ""
echo "This demo uses only Node.js built-in modules - no external dependencies!"
echo ""

# Check if rnx is available
if ! command -v rnx &> /dev/null; then
    echo "❌ Error: 'rnx' command not found"
    exit 1
fi

# Create volume for results
echo "📁 Creating volume for Node.js data..."
rnx volume create nodejs-data --size=100MB --type=filesystem 2>/dev/null || echo "  ✓ Volume 'nodejs-data' ready"

echo ""
echo "🚀 Running Node.js demo..."

# Run the simple Node.js demo
rnx run --upload=simple_nodejs.js \
        --volume=nodejs-data \
        --max-memory=256 \
        node simple_nodejs.js

echo ""
echo "✅ Demo complete! Check the results:"
echo ""
echo "📊 View results:"
echo "  rnx run --volume=nodejs-data cat /volumes/nodejs-data/results.json"