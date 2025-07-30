#!/bin/bash

# Python Data Analysis Example for Joblet
# This script uploads files and runs the analysis, then shows the results

set -e

echo "🚀 Starting Python Data Analysis with Joblet"
echo "============================================="

# Check if rnx is available
if ! command -v rnx &> /dev/null; then
    echo "❌ Error: rnx command not found"
    echo "Please make sure rnx is installed and in your PATH"
    exit 1
fi

# Upload files and run the analysis
echo "📤 Uploading files and starting analysis job..."

JOB_OUTPUT=$(rnx run \
    --upload=analyze_sales_simple.py \
    --upload=data/sales_data.csv \
    --upload=data/customers.csv \
    python3 analyze_sales_simple.py)

# Extract job ID from output
JOB_ID=$(echo "$JOB_OUTPUT" | grep "^ID:" | cut -d' ' -f2)

if [ -z "$JOB_ID" ]; then
    echo "❌ Failed to extract job ID from output:"
    echo "$JOB_OUTPUT"
    exit 1
fi

echo "✅ Job started successfully!"
echo "📋 Job ID: $JOB_ID"
echo ""

# Wait for job to complete
echo "⏳ Waiting for analysis to complete..."
sleep 3

# Show the results
echo "📊 Analysis Results:"
echo "==================="
rnx log "$JOB_ID"

echo ""
echo "🎉 Analysis complete! Job ID: $JOB_ID"
echo ""
echo "💡 You can view the logs again anytime with:"
echo "   rnx log $JOB_ID"