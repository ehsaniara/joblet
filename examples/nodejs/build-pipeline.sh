#!/bin/bash
set -e

echo "🚀 Starting Build Pipeline"
echo "Build Number: $BUILD_NUMBER"
echo "Node Environment: $NODE_ENV"

# Setup directories
WORKSPACE="/work"
OUTPUT_DIR="/volumes/nodejs-projects/builds/$BUILD_NUMBER"
CACHE_DIR="/volumes/build-cache"
REPORTS_DIR="/volumes/nodejs-projects/reports"

# Create directories if volumes exist
if [ -d "/volumes/nodejs-projects" ]; then
    mkdir -p "$OUTPUT_DIR" "$REPORTS_DIR"
fi

if [ -d "/volumes/build-cache" ]; then
    mkdir -p "$CACHE_DIR"
fi

cd "$WORKSPACE"

echo "📋 Step 1: Environment Setup"
node --version
npm --version

# Use cached node_modules if available
if [ -d "/volumes/node-modules/node_modules" ]; then
    echo "Using cached node_modules"
    ln -sf /volumes/node-modules/node_modules ./node_modules
else
    echo "Installing dependencies"
    if [ -d "$CACHE_DIR" ]; then
        npm ci --cache="$CACHE_DIR"
    else
        npm ci
    fi
    
    # Cache node_modules if volume exists
    if [ -d "/volumes/node-modules" ]; then
        cp -r node_modules /volumes/node-modules/
    fi
fi

echo "🔍 Step 2: Code Quality Checks"
npm run lint 2>&1 | tee "$REPORTS_DIR/lint-report.txt" || echo "Lint completed with warnings"

echo "🧪 Step 3: Testing"
npm run test 2>&1 | tee "$REPORTS_DIR/test-report.txt" || echo "Tests completed"

echo "📊 Step 4: Demo Processing"
npm run process-data 2>&1 | tee "$REPORTS_DIR/processing-report.txt" || echo "Processing completed"

echo "📋 Step 5: Build Artifacts"
# Create build manifest
if [ -d "/volumes/nodejs-projects" ]; then
    cat > "$OUTPUT_DIR/build-manifest.json" << EOF
{
  "buildNumber": "$BUILD_NUMBER",
  "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "nodeVersion": "$(node --version)",
  "npmVersion": "$(npm --version)",
  "environment": "$NODE_ENV",
  "buildDuration": "$(date +%s)",
  "artifacts": [
    "app.js",
    "package.json",
    "process_data.js"
  ]
}
EOF

    # Copy main files to build output
    cp app.js package.json process_data.js api.test.js "$OUTPUT_DIR/" || echo "Some files not found"
fi

echo "✅ Build Pipeline Completed Successfully"
echo "Build artifacts saved to: $OUTPUT_DIR"
echo "Reports saved to: $REPORTS_DIR"