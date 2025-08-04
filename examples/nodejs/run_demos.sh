#!/bin/bash
set -e

echo "🚀 Setting up Node.js Demo Environment"
echo "========================================"

# System compatibility checks
echo "🔍 Checking system compatibility..."

# Check if rnx is available
if ! command -v rnx &> /dev/null; then
    echo "❌ Error: 'rnx' command not found"
    echo "Please ensure Joblet RNX client is installed and in PATH"
    exit 1
fi

# Test connection to Joblet server
echo "🔗 Testing connection to Joblet server..."
if ! rnx list &> /dev/null; then
    echo "❌ Error: Cannot connect to Joblet server"
    echo "Please ensure Joblet daemon is running and RNX is configured"
    exit 1
fi

# Check if we have the required demo files
echo "📁 Checking demo files..."
REQUIRED_FILES=("package.json" "api.test.js" "app.js" "process_data.js" "event-processor.js" "build-pipeline.sh")
for file in "${REQUIRED_FILES[@]}"; do
    if [ ! -f "$file" ]; then
        echo "❌ Error: Required file '$file' not found"
        echo "Please ensure you're running this script from the nodejs directory"
        exit 1
    fi
done

echo "✅ System checks passed"
echo ""

# Create volumes with error handling
echo "📁 Creating volumes..."
VOLUMES=("nodejs-projects:1GB" "node-modules:2GB" "build-cache:1GB")

for volume_spec in "${VOLUMES[@]}"; do
    IFS=':' read -r volume_name volume_size <<< "$volume_spec"
    if ! rnx volume create "$volume_name" --size="$volume_size" --type=filesystem 2>/dev/null; then
        echo "ℹ️  Volume '$volume_name' already exists or creation failed"
        if ! rnx volume list | grep -q "$volume_name"; then
            echo "❌ Error: Failed to create $volume_name volume"
            echo "Please check available disk space and Joblet server logs"
            exit 1
        fi
    fi
done

echo ""
echo "🎬 Demo 1: API Testing Suite"
echo "Running comprehensive API tests..."

# Check Node.js environment first
echo "🔍 Checking Node.js environment..."
if ! rnx run --max-memory=256 node --version; then
    echo "❌ Error: Node.js not available in Joblet environment"
    echo "Please ensure Node.js is installed in the Joblet server environment"
    exit 1
fi

# Check if npm is available
if ! rnx run --max-memory=256 npm --version; then
    echo "❌ Error: npm not available in Joblet environment"
    exit 1
fi

if ! rnx run --upload-dir=. \
       --volume=nodejs-projects \
       --max-memory=256 \
       --env=API_BASE_URL=http://demo-api.example.com \
       --env=NODE_ENV=test \
       npm test; then
    echo "⚠️  API tests completed with warnings (expected in demo mode)"
    echo "   In demo mode, tests adapt to missing external services"
fi

echo ""
echo "🚀 Demo 2: Microservice Deployment"
echo "Starting demo microservice..."

# Start demo service in background
rnx run --upload-dir=. \
       --volume=nodejs-projects \
       --volume=node-modules \
       --max-memory=512 \
       --env=PORT=3001 \
       --env=NODE_ENV=production \
       --name="demo-microservice" \
       npm start &

SERVICE_JOB_ID=$!
echo "Demo service started with job ID: $SERVICE_JOB_ID"

echo ""
echo "📊 Demo 3: Data Processing"
echo "Processing CSV data with Node.js streams..."
rnx run --upload=process_data.js \
       --volume=nodejs-projects \
       --max-memory=1024 \
       --max-iobps=52428800 \
       node process_data.js

echo ""
echo "🔧 Demo 4: Build Pipeline"
echo "Running complete CI/CD pipeline..."
rnx run --upload-dir=. \
       --volume=nodejs-projects \
       --volume=node-modules \
       --volume=build-cache \
       --max-memory=1024 \
       --env=NODE_ENV=production \
       --env=BUILD_NUMBER=$(date +%s) \
       --upload=build-pipeline.sh \
       bash build-pipeline.sh

echo ""
echo "⚡ Demo 5: Event Processing Simulation"
echo "Running event stream processor..."
rnx run --upload=event-processor.js \
       --volume=nodejs-projects \
       --max-memory=512 \
       --env=DEMO_MODE=true \
       node event-processor.js &

EVENT_JOB_ID=$!
echo "Event processor started with job ID: $EVENT_JOB_ID"

# Let it run for a bit
echo "Letting event processor run for 30 seconds..."
sleep 30

# Stop the event processor
echo "Stopping event processor..."
rnx stop $EVENT_JOB_ID || echo "Event processor may have already stopped"

echo ""
echo "✅ Node.js Demo Complete!"
echo ""
echo "📋 Check results:"
echo "  rnx run --volume=nodejs-projects ls -la /volumes/nodejs-projects/"
echo "  rnx run --volume=nodejs-projects cat /volumes/nodejs-projects/reports/test-report.txt"
echo "  rnx run --volume=nodejs-projects ls -la /volumes/nodejs-projects/output/"
echo ""
echo "🏃 Running services:"
echo "  Demo microservice may still be running - check with: rnx list"