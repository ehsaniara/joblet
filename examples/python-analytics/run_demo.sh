#!/bin/bash
set -e

echo "🚀 Python Analytics Demo"
echo "========================"
echo ""
echo "This demo uses only Python standard library - no external dependencies!"
echo ""

# Check if rnx is available
if ! command -v rnx &> /dev/null; then
    echo "❌ Error: 'rnx' command not found"
    exit 1
fi

# Create volumes
echo "📁 Creating volumes..."
rnx volume create analytics-data --size=1GB --type=filesystem 2>/dev/null || echo "  ✓ Volume 'analytics-data' ready"
rnx volume create ml-models --size=500MB --type=filesystem 2>/dev/null || echo "  ✓ Volume 'ml-models' ready"

echo ""
echo "🚀 Running analytics demo..."

# Run the simple analytics that works with standard library only
rnx run --upload=simple_analytics.py \
        --upload=sales_data.csv \
        --upload=customers.csv \
        --volume=analytics-data \
        --volume=ml-models \
        --max-memory=512 \
        python3 simple_analytics.py

echo ""
echo "✅ Demo complete! Check the results:"
echo ""
echo "📊 View results:"
echo "  rnx run --volume=analytics-data cat /volumes/analytics-data/results/sales_analysis.json"
echo "  rnx run --volume=ml-models cat /volumes/ml-models/clustering_results.json"
echo "  rnx run --volume=analytics-data ls /volumes/analytics-data/processed/"