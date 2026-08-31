# Python 3.11 ML Runtime

Python 3.11 runtime with machine learning and data science packages pre-installed.

## Quick Start

### 1. Build the Runtime

```bash
# Build the runtime (requires root)
sudo rnx runtime build examples/python-3.11-ml/runtime.yaml

# Or preview without building
rnx runtime build --dry-run examples/python-3.11-ml/runtime.yaml
```

### 2. Verify Installation

```bash
# List available runtimes
rnx runtime list

# Test the runtime
rnx runtime test python-3.11-ml

# Check Python version
rnx job run --runtime=python-3.11-ml python3 --version

# Test ML packages
rnx job run --runtime=python-3.11-ml python3 -c "import numpy; import pandas; import sklearn; print('ML packages loaded!')"
```

### 3. Run Examples

```bash
# Run the data analysis example
rnx job run --runtime=python-3.11-ml --upload=examples/python-3.11-ml/example_data_analysis.py \
  python3 example_data_analysis.py

# Quick ML test
rnx job run --runtime=python-3.11-ml python3 -c "
import numpy as np
from sklearn.ensemble import RandomForestClassifier

X = np.random.randn(100, 4)
y = (X[:, 0] > 0).astype(int)
clf = RandomForestClassifier(n_estimators=10)
clf.fit(X, y)
print(f'Model accuracy: {clf.score(X, y):.2f}')
"
```

## Pre-installed Packages

The runtime includes these ML/data science packages:

| Package      | Version | Purpose              |
|--------------|---------|----------------------|
| numpy        | 1.26.2  | Numerical computing  |
| pandas       | 2.1.3   | Data manipulation    |
| scikit-learn | 1.3.2   | Machine learning     |
| matplotlib   | 3.8.2   | Data visualization   |
| seaborn      | 0.13.0  | Statistical plotting |
| joblib       | 1.3.2   | Model persistence    |

## Example Files

### example_data_analysis.py

Complete ML pipeline demonstrating:

- Data generation with NumPy
- Data manipulation with Pandas
- Model training with scikit-learn
- Visualization with Matplotlib

### requirements.txt

Package list for reference or local development.

### setup.sh

Helper script for local development setup.

## Use Cases

- Machine learning model training
- Data analysis and exploration
- Statistical computing
- Data visualization
- Feature engineering

## Related

- [Python Basic Runtime](../python/README.md) - Lightweight Python without ML packages
- [Python Analytics](../python-analytics/README.md) - Analytics-focused examples
- [Runtime System Guide](../../docs/RUNTIME_SYSTEM.md)
