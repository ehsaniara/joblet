# Python Analytics Runtime

Python 3.11 runtime with analytics and visualization packages.

## Quick Start

### 1. Build the Runtime

```bash
# Build the runtime (requires root)
sudo rnx runtime build examples/python-analytics/runtime.yaml

# Or preview without building
rnx runtime build --dry-run examples/python-analytics/runtime.yaml
```

### 2. Verify Installation

```bash
# List available runtimes
rnx runtime list

# Test the runtime
rnx runtime test python-analytics

# Check packages
rnx job run --runtime=python-analytics python3 -c "import pandas; import matplotlib; print('Analytics packages loaded!')"
```

### 3. Run Examples

```bash
# Run the demo script
cd examples/python-analytics
./run_demo.sh

# Or run analytics directly
rnx job run --runtime=python-analytics --upload=examples/python-analytics/simple_analytics.py \
  --upload=examples/python-analytics/sales_data.csv \
  python3 simple_analytics.py

# Run analysis scripts
rnx job run --runtime=python-analytics \
  --upload=examples/python-analytics/scripts/analyze_sales.py \
  --upload=examples/python-analytics/sales_data.csv \
  python3 analyze_sales.py
```

## Pre-installed Packages

| Package | Version | Purpose |
|---------|---------|---------|
| pandas | 2.1.3 | Data manipulation |
| matplotlib | 3.8.2 | Data visualization |
| seaborn | 0.13.0 | Statistical plotting |

## Example Files

### simple_analytics.py

Basic analytics script using pandas for data analysis.

### sales_data.csv / customers.csv

Sample datasets for analytics examples.

### scripts/

Directory containing additional analysis scripts:

- `analyze_sales.py` - Sales data analysis
- `segment_customers.py` - Customer segmentation
- `time_series.py` - Time series processing
- `combine_reports.py` - Report generation
- `run_complete_analytics.sh` - Run full pipeline

## Use Cases

- Data analysis and reporting
- Statistical visualization
- Business analytics
- Data exploration

## Related

- [Python ML Runtime](../python-3.11-ml/README.md) - Full ML stack
- [Python Basic Runtime](../python/README.md) - Lightweight Python
- [Runtime YAML Reference](../../docs/design/RUNTIME_YAML_QUICKREF.md)
