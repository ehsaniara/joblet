#!/bin/bash

# Direct pip installation script for minimal chroot environments
# This bypasses APT entirely and installs pip directly

set -e

echo "🔧 Installing pip directly without APT..."
echo "========================================"

# Check if pip is already available
if python3 -m pip --version 2>/dev/null; then
    echo "✅ pip is already available"
else
    echo "📦 Downloading and installing pip directly..."
    
    # Download get-pip.py
    if command -v wget >/dev/null 2>&1; then
        wget -q https://bootstrap.pypa.io/get-pip.py -O get-pip.py
    elif command -v curl >/dev/null 2>&1; then
        curl -sS https://bootstrap.pypa.io/get-pip.py -o get-pip.py
    else
        echo "❌ Neither wget nor curl available, cannot download pip installer"
        exit 1
    fi
    
    # Install pip to user directory
    python3 get-pip.py --user
    
    # Add to PATH
    export PATH="$HOME/.local/bin:$PATH"
    
    echo "✅ pip installed successfully"
fi

# Verify pip installation
echo "🔍 Verifying pip installation..."
python3 -m pip --version

# Install required packages
echo "📚 Installing Python packages..."
python3 -m pip install --user pandas numpy matplotlib

echo "✅ All packages installed successfully!"

# Test imports
echo "🧪 Testing package imports..."
python3 -c "import pandas; print('✅ pandas imported successfully')"
python3 -c "import numpy; print('✅ numpy imported successfully')"
python3 -c "import matplotlib; print('✅ matplotlib imported successfully')"

echo "🚀 Running data analysis..."
python3 analyze_sales.py