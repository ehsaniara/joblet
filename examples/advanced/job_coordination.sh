#!/bin/bash
set -e

echo "🔗 Simple Job Coordination Demo"
echo "==============================="
echo ""
echo "This demo shows basic job coordination with volumes"
echo ""

# Check prerequisites
if ! command -v rnx &> /dev/null; then
    echo "❌ Error: 'rnx' command not found"
    exit 1
fi

# Create shared volume
echo "📦 Creating shared volume..."
rnx volume create shared-data --size=100MB --type=filesystem 2>/dev/null || echo "  ✓ Volume 'shared-data' already exists"

echo ""
echo "📝 Job 1: Generate data"
echo "----------------------"
rnx run --volume=shared-data --max-memory=256 \
    python3 -c "
import json
import random

# Generate sample data
data = {
    'records': [
        {'id': i, 'value': random.randint(1, 100)}
        for i in range(100)
    ]
}

# Save to volume
with open('/volumes/shared-data/data.json', 'w') as f:
    json.dump(data, f)

print(f'Generated {len(data[\"records\"])} records')
print('Data saved to /volumes/shared-data/data.json')
"

echo ""
echo "⏳ Waiting for Job 1 to complete..."
sleep 3

echo ""
echo "📊 Job 2: Process data (depends on Job 1)"
echo "-----------------------------------------"
rnx run --volume=shared-data --max-memory=256 \
    python3 -c "
import json
import statistics

# Read data from Job 1
try:
    with open('/volumes/shared-data/data.json', 'r') as f:
        data = json.load(f)
    
    values = [r['value'] for r in data['records']]
    
    # Process the data
    results = {
        'count': len(values),
        'sum': sum(values),
        'mean': statistics.mean(values),
        'median': statistics.median(values),
        'stdev': statistics.stdev(values)
    }
    
    # Save results
    with open('/volumes/shared-data/results.json', 'w') as f:
        json.dump(results, f, indent=2)
    
    print('Analysis complete:')
    for key, value in results.items():
        print(f'  {key}: {value:.2f}' if isinstance(value, float) else f'  {key}: {value}')
    
    print('')
    print('Results saved to /volumes/shared-data/results.json')
    
except FileNotFoundError:
    print('ERROR: Job 1 output not found. Please ensure Job 1 completed successfully.')
    exit(1)
"

echo ""
echo "⏳ Waiting for Job 2 to complete..."
sleep 3

echo ""
echo "✅ Job Coordination Complete!"
echo ""
echo "📊 View results:"
echo "  rnx run --volume=shared-data cat /volumes/shared-data/data.json    # Raw data"
echo "  rnx run --volume=shared-data cat /volumes/shared-data/results.json # Analysis"