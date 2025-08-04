#!/bin/bash
set -e

echo "🚀 Setting up Python Analytics Demo Environment"
echo "==================================================="

# Check if we're running on a system that supports the demos
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
    echo "Please ensure:"
    echo "  - Joblet daemon is running"
    echo "  - RNX client is properly configured"
    echo "  - Network connectivity to Joblet server"
    exit 1
fi

# Check system resources (basic check)
echo "📊 Checking available system resources..."
if command -v free &> /dev/null; then
    AVAILABLE_RAM=$(free -m | awk 'NR==2{printf "%.0f", $7}')
    if [ "$AVAILABLE_RAM" -lt 2048 ]; then
        echo "⚠️  Warning: Available RAM (${AVAILABLE_RAM}MB) may be insufficient for ML demos"
        echo "   Recommended: 4GB+ available RAM for optimal performance"
    fi
fi

echo "✅ System checks passed"
echo ""

# Create volumes with error handling
echo "📁 Creating volumes..."
if ! rnx volume create analytics-data --size=2GB --type=filesystem 2>/dev/null; then
    echo "ℹ️  Volume 'analytics-data' already exists or creation failed"
    # Check if volume exists
    if ! rnx volume list | grep -q "analytics-data"; then
        echo "❌ Error: Failed to create analytics-data volume"
        echo "Please check Joblet server logs and available disk space"
        exit 1
    fi
fi

if ! rnx volume create ml-models --size=1GB --type=filesystem 2>/dev/null; then
    echo "ℹ️  Volume 'ml-models' already exists or creation failed"
    if ! rnx volume list | grep -q "ml-models"; then
        echo "❌ Error: Failed to create ml-models volume"
        exit 1
    fi
fi

# Upload sample data to volume
# Check if sample data files exist
echo "📊 Checking sample data files..."
if [ ! -f "customers.csv" ]; then
    echo "❌ Error: customers.csv not found"
    echo "Please ensure you're running this script from the python-analytics directory"
    exit 1
fi

if [ ! -f "sales_data.csv" ]; then
    echo "❌ Error: sales_data.csv not found"
    exit 1
fi

echo "📊 Uploading sample data..."
if ! rnx run --volume=analytics-data --upload=customers.csv \
    bash -c "mkdir -p /volumes/analytics-data && cp customers.csv /volumes/analytics-data/"; then
    echo "❌ Error: Failed to upload customer data"
    echo "This may indicate:"
    echo "  - Insufficient resources on Joblet server"
    echo "  - Volume mount issues"
    echo "  - Network connectivity problems"
    exit 1
fi

echo ""
echo "🎬 Demo 1: Basic Sales Analysis"
echo "Running sales analysis with pandas and matplotlib..."
# Check for required Python dependencies in the job environment
echo "🔍 Checking Python environment..."
if ! rnx run --max-memory=256 python3 -c "import pandas, numpy, matplotlib; print('✅ Required packages available')"; then
    echo "❌ Error: Required Python packages not available in Joblet environment"
    echo "The Joblet server environment needs:"
    echo "  - pandas >= 2.0.0"
    echo "  - numpy >= 1.24.0"
    echo "  - matplotlib >= 3.7.0"
    echo ""
    echo "Please install these packages in the Joblet server environment or use a container image that includes them."
    exit 1
fi

if ! rnx run --upload=sales_data.csv --upload=analyze_sales.py \
       --volume=analytics-data \
       --max-memory=512 \
       python3 analyze_sales.py; then
    echo "❌ Error: Sales analysis failed"
    echo "Check job logs with: rnx list (find job ID) then rnx log <job-id>"
    exit 1
fi

echo ""
echo "🤖 Demo 2: Machine Learning - Customer Segmentation"
echo "Training K-means clustering model..."
# Check for ML dependencies
echo "🔍 Checking ML dependencies..."
if ! rnx run --max-memory=256 python3 -c "import sklearn, joblib; print('✅ ML packages available')"; then
    echo "❌ Error: Required ML packages not available"
    echo "The Joblet server environment needs:"
    echo "  - scikit-learn >= 1.3.0"
    echo "  - joblib >= 1.3.0"
    exit 1
fi

if ! rnx run --upload=train_clustering.py \
       --volume=ml-models \
       --volume=analytics-data \
       --max-cpu=75 --max-memory=2048 \
       --env=MODEL_VERSION=v1.2 \
       python3 train_clustering.py; then
    echo "❌ Error: ML training failed"
    echo "This could be due to:"
    echo "  - Insufficient CPU/memory resources"
    echo "  - Missing customer data from previous step"
    echo "  - Package version incompatibilities"
    exit 1
fi

echo ""
echo "📈 Demo 3: Feature Engineering"
echo "Processing data chunks in parallel..."

# Create sample data chunks with error handling
echo "Creating sample data chunks..."
if ! rnx run --volume=analytics-data --max-memory=512 \
    python3 -c "
import pandas as pd
import numpy as np
from datetime import datetime, timedelta
import os

# Create sample time-series data
np.random.seed(42)
start_date = datetime(2024, 1, 1)
dates = [start_date + timedelta(days=i) for i in range(365)]

for chunk_id in range(1, 5):
    data = []
    for user_id in range(chunk_id*10, (chunk_id+1)*10):
        for i, date in enumerate(dates[::7]):  # Weekly data
            value = np.random.normal(100 + user_id, 20) + np.sin(i/10) * 10
            data.append({
                'user_id': user_id,
                'timestamp': date.isoformat(),
                'value': max(0, value)
            })
    
    df = pd.DataFrame(data)
    os.makedirs('/volumes/analytics-data/raw', exist_ok=True)
    df.to_csv(f'/volumes/analytics-data/raw/data_chunk_{chunk_id}.csv', index=False)

print('Sample data chunks created')
"; then
    echo "❌ Error: Failed to create sample data chunks"
    echo "This may indicate memory or volume issues"
    exit 1
fi

# Process chunks in parallel with error handling
echo "🔄 Starting parallel feature engineering jobs..."
JOB_PIDS=()
FAILED_JOBS=()

for chunk in {1..4}; do
    echo "Starting chunk $chunk processing..."
    if rnx run --upload=feature_engineering.py \
           --volume=analytics-data \
           --max-memory=1024 \
           --env=CHUNK_ID=$chunk \
           --env=TOTAL_CHUNKS=4 \
           --name="features-chunk-$chunk" \
           python3 feature_engineering.py & then
        JOB_PIDS+=($!)
    else
        echo "⚠️  Warning: Failed to start chunk $chunk processing"
        FAILED_JOBS+=("chunk-$chunk")
    fi
done

# Wait for all jobs to complete
echo "⏳ Waiting for parallel jobs to complete..."
for pid in "${JOB_PIDS[@]}"; do
    if ! wait $pid; then
        echo "⚠️  Warning: One of the parallel jobs failed"
    fi
done

if [ ${#FAILED_JOBS[@]} -eq 0 ]; then
    echo "✅ All feature engineering jobs completed successfully"
else
    echo "⚠️  Some feature engineering jobs failed: ${FAILED_JOBS[*]}"
    echo "   This may be due to resource constraints or concurrent job limits"
fi

echo ""
echo "✅ Python Analytics Demo Complete!"
echo "================================="
echo ""
echo "📋 Results Summary:"
echo "  📊 Sales Analysis: Trend charts and monthly summaries"
echo "  🤖 ML Models: Customer segmentation with K-means clustering"
echo "  🔄 Feature Engineering: Processed data chunks with time-based features"
echo ""
echo "🔍 Inspect results:"
echo "  # Sales analysis results"
echo "  rnx run --volume=analytics-data ls -la /volumes/analytics-data/results/"
echo ""
echo "  # ML model artifacts"
echo "  rnx run --volume=ml-models ls -la /volumes/ml-models/"
echo ""
echo "  # Feature engineering output"
echo "  rnx run --volume=analytics-data ls -la /volumes/analytics-data/processed/"
echo ""
echo "📊 Monitor system:"
echo "  rnx list                    # View all jobs"
echo "  rnx monitor                 # Real-time monitoring"
echo "  rnx volume list             # Check volume usage"
echo ""
echo "💡 Tips:"
echo "  - Resource requirements scale with data size"
echo "  - Use 'rnx log <job-id>' to debug any failures"
echo "  - Consider increasing memory limits for larger datasets"