#!/bin/bash
# Setup script for ML project with packaged dependencies
# Installs all ML dependencies to local lib/ directory

set -e

echo "🔧 Setting up ML project with packaged dependencies..."
echo "====================================================="

# Check if we're in the right directory
if [[ ! -f "requirements.txt" ]]; then
    echo "❌ Error: requirements.txt not found in current directory"
    echo "Make sure you're running this from the python-3.11-ml directory"
    exit 1
fi

# Remove existing lib directory
if [[ -d "lib" ]]; then
    echo "🧹 Removing existing lib/ directory..."
    rm -rf lib
fi

# Create lib directory
echo "📁 Creating lib/ directory for dependencies..."
mkdir -p lib

# Install dependencies to lib/
echo "📦 Installing ML dependencies to lib/ (this may take a few minutes)..."
pip3 install -r requirements.txt --target lib/ --quiet

# Clean up unnecessary files to reduce size
echo "🧹 Cleaning up unnecessary files..."
find lib/ -name "*.pyc" -delete 2>/dev/null || true
find lib/ -name "__pycache__" -exec rm -rf {} + 2>/dev/null || true
find lib/ -name "*.pyo" -delete 2>/dev/null || true
find lib/ -name "tests" -type d -exec rm -rf {} + 2>/dev/null || true
find lib/ -name "test" -type d -exec rm -rf {} + 2>/dev/null || true

# Calculate sizes
TOTAL_SIZE=$(du -sh . | cut -f1)
LIB_SIZE=$(du -sh lib/ | cut -f1)
PACKAGE_COUNT=$(ls lib/ | wc -l)

echo ""
echo "✅ Setup Complete!"
echo "==================="
echo "📊 Project Statistics:"
echo "   • Total project size: $TOTAL_SIZE"
echo "   • Dependencies size: $LIB_SIZE" 
echo "   • Packages installed: $PACKAGE_COUNT"
echo ""
echo "📁 Project structure:"
echo "   python-3.11-ml/"
echo "   ├── requirements.txt"
echo "   ├── example_data_analysis.py"
echo "   ├── setup.sh"
echo "   └── lib/              # All dependencies packaged here"
echo "       ├── pandas/"
echo "       ├── numpy/"
echo "       ├── sklearn/"
echo "       └── ... (and more)"
echo ""
echo "🚀 Ready to deploy to Joblet!"
echo ""
echo "💡 Usage examples:"
echo "   # Local test"
echo "   python3 example_data_analysis.py"
echo ""
echo "   # Deploy with packaged dependencies"
echo "   rnx job run --runtime=python-3.11 --upload=. python example_data_analysis.py"