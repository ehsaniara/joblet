#!/bin/bash
set -e  # Exit on error

cd /work

echo "🔍 Checking Python environment..."
echo "Python version: $(/usr/bin/python3 --version)"
echo "Current directory: $(pwd)"
echo "Files in directory: $(ls -la)"

# Check if pip is available
if ! /usr/bin/python3 -m pip --version > /dev/null 2>&1; then
    echo "❌ Error: pip is not available in the isolated environment"
    echo "Trying alternative Python locations..."

    # Try to find pip
    if [ -f /usr/bin/pip3 ]; then
        echo "Found pip3 at /usr/bin/pip3"
        PIP_CMD="/usr/bin/pip3"
    elif [ -f /usr/local/bin/pip3 ]; then
        echo "Found pip3 at /usr/local/bin/pip3"
        PIP_CMD="/usr/local/bin/pip3"
    else
        echo "❌ Could not find pip. Please ensure pip is installed and its path is in allowedMounts"
        exit 1
    fi
else
    PIP_CMD="/usr/bin/python3 -m pip"
fi

echo "✅ Using pip: $PIP_CMD"

# Set up local Python environment
echo "🔧 Setting up local Python environment..."
export PYTHONUSERBASE=/work/.local
export PATH="/work/.local/bin:$PATH"

# Get Python version for correct site-packages path
PYTHON_VERSION=$(/usr/bin/python3 -c 'import sys; print(f"{sys.version_info.major}.{sys.version_info.minor}")')
export PYTHONPATH="/work/.local/lib/python${PYTHON_VERSION}/site-packages:$PYTHONPATH"

echo "PYTHONUSERBASE: $PYTHONUSERBASE"
echo "PYTHONPATH: $PYTHONPATH"

# Create necessary directories
mkdir -p /work/.local

# Install packages with proper flags
echo "📦 Installing packages..."
echo "This may take a few minutes on first run..."

# Use eval to properly handle the pip command
eval "$PIP_CMD install --user --no-cache-dir --no-warn-script-location -r requirements.txt"

# Verify installation
echo "✅ Verifying installed packages:"
eval "$PIP_CMD list --user"

# Additional verification - try importing packages
echo "🔍 Testing package imports..."
/usr/bin/python3 -c "
import pandas
import numpy
import sklearn
import matplotlib
import seaborn
print('✅ All packages imported successfully!')
print(f'Pandas version: {pandas.__version__}')
print(f'NumPy version: {numpy.__version__}')
print(f'Scikit-learn version: {sklearn.__version__}')
"

# Set matplotlib backend to non-interactive (since we're in a headless environment)
export MPLBACKEND=Agg

echo "🚀 Running ML analysis..."
/usr/bin/python3 ml_analysis.py

echo "🎉 Job completed successfully!"

# Check if output file was created
if [ -f "ml_analysis_results.png" ]; then
    echo "✅ Output file created: ml_analysis_results.png"
    echo "File size: $(ls -lh ml_analysis_results.png | awk '{print $5}')"
else
    echo "⚠️ Warning: Expected output file ml_analysis_results.png was not created"
fi