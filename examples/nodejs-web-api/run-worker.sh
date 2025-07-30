#!/bin/bash

# Setup and run script for Node.js background worker
# This script handles npm installation and starts the worker

set -e

echo "🔧 Setting up Node.js worker environment..."
echo "=========================================="

# Check if node is available
if ! command -v node &> /dev/null; then
    echo "❌ Node.js is not available in the container"
    echo "You may need to use a container image with Node.js pre-installed"
    exit 1
fi

echo "✅ Node.js version: $(node --version)"

# Install dependencies if node_modules doesn't exist
if [ ! -d "node_modules" ]; then
    echo "📦 Installing Node.js dependencies..."
    
    # Create a local npm prefix to avoid permission issues
    export NPM_CONFIG_PREFIX="$HOME/.npm-global"
    export PATH="$NPM_CONFIG_PREFIX/bin:$PATH"
    
    npm install --production
    echo "✅ Dependencies installed successfully"
else
    echo "✅ Using existing node_modules"
fi

echo ""
echo "🚀 Starting background worker..."
echo "================================"

# Start the worker
exec node worker.js