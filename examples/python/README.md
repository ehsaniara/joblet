# Python 3.11 Basic Runtime

Lightweight Python 3.11 runtime for scripts and utilities.

## Quick Start

### 1. Build the Runtime

```bash
# Build the runtime (requires root)
sudo rnx runtime build examples/python/runtime.yaml

# Or preview without building
rnx runtime build --dry-run examples/python/runtime.yaml
```

### 2. Verify Installation

```bash
# List available runtimes
rnx runtime list

# Test the runtime
rnx runtime test python-3.11

# Check Python version
rnx job run --runtime=python-3.11 python3 --version
```

### 3. Run Examples

```bash
# Run the comprehensive test
rnx job run --runtime=python-3.11 \
  --upload=examples/python/comprehensive-python-test.py \
  python3 comprehensive-python-test.py

# Run data processing workflow
rnx job run --runtime=python-3.11 \
  --upload=examples/python/data-processor.py \
  python3 data-processor.py
```

## Runtime Features

- **Python Version**: 3.11
- **Pre-installed**: Standard library only
- **Fast startup**: Minimal dependencies
- **Low memory**: Lightweight footprint

## Example Files

### comprehensive-python-test.py

Tests the Python environment including:

- Python version and executable path
- Environment variables
- Package imports (stdlib only)
- File operations
- JSON processing

### data-processor.py

Data processing script that:

- Creates sample data
- Calculates statistics (count, sum, mean, min, max)
- Writes JSON output

### data-analyzer.py

Analysis script that:

- Reads processed data
- Calculates range and variance
- Counts values above/below mean

## Multi-Job Workflow

These scripts demonstrate a multi-job workflow:

```bash
# Step 1: Process data
rnx job run --runtime=python-3.11 --volume=workflow \
  --upload=examples/python/data-processor.py \
  python3 data-processor.py

# Step 2: Analyze results
rnx job run --runtime=python-3.11 --volume=workflow \
  --upload=examples/python/data-analyzer.py \
  python3 data-analyzer.py
```

## Use Cases

- Lightweight scripting
- File processing and JSON handling
- HTTP operations (with urllib)
- Multi-job data workflows
- Fast startup required

## Related

- [Python ML Runtime](../python-3.11-ml/README.md) - NumPy, Pandas, scikit-learn
- [Python Analytics Runtime](../python-analytics/README.md) - Pandas and visualization
- [Runtime YAML Reference](../../docs/design/RUNTIME_YAML_QUICKREF.md)
