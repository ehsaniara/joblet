#!/bin/bash

# Setup and run script for Node.js API server
# This script handles npm installation and starts the server

set -e

echo "🔧 Setting up Node.js environment..."
echo "===================================="

# Check if node is available
if ! command -v node &> /dev/null; then
    echo "❌ Node.js is not available in the container"
    echo "You may need to use a container image with Node.js pre-installed"
    exit 1
fi

echo "✅ Node.js version: $(node --version)"

# Check if npm is available
if ! command -v npm &> /dev/null; then
    echo "📦 Installing npm..."
    # Try to install npm via apt if available
    if command -v apt-get &> /dev/null; then
        apt-get update -qq
        apt-get install -y npm
    else
        echo "❌ npm is not available and cannot be installed"
        exit 1
    fi
fi

echo "✅ npm version: $(npm --version)"

# Install dependencies
echo ""
echo "📦 Installing Node.js dependencies..."
echo "===================================="

# Create a local npm prefix to avoid permission issues
export NPM_CONFIG_PREFIX="$HOME/.npm-global"
export PATH="$NPM_CONFIG_PREFIX/bin:$PATH"

# Install dependencies
npm install --production

echo "✅ Dependencies installed successfully"

# Show installed packages
echo ""
echo "📋 Installed packages:"
npm list --depth=0

echo ""
echo "🚀 Starting API server..."
echo "========================"

# Start the server
exec node server.js