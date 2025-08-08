# Python 3.11 + ML Examples

Example applications for the `python-3.11-ml` runtime environment with instant startup (2-3 seconds).

## ⚡ Runtime Features

- **Python Version**: 3.11.9 (compiled for isolation)
- **ML Packages**: 
  - NumPy 1.24.x (pinned to 1.x for stability)
  - Pandas 2.0.x (data analysis)
  - Scikit-learn 1.3.x (machine learning)
  - Matplotlib 3.7.x (visualization)
  - Seaborn 0.12.x (statistical plots)
  - SciPy 1.11.x (scientific computing)
- **Additional**: requests 2.31.0, openpyxl 3.1.2
- **Memory**: 512MB-2GB recommended
- **Startup Time**: 2-3 seconds (vs 5-45 minutes traditional)

## 🚀 Quick Start

### Prerequisites

**Option 1: Deploy Pre-built Package (Recommended)**
```bash
# Copy package from examples/packages/ (226MB)
scp examples/packages/python-3.11-ml-runtime.tar.gz admin@host:/tmp/

# Deploy on target host
ssh admin@host
sudo tar -xzf /tmp/python-3.11-ml-runtime.tar.gz -C /opt/joblet/runtimes/python/
sudo chown -R joblet:joblet /opt/joblet/runtimes/python/python-3.11-ml
```

**Option 2: Build from Setup Script**
```bash
# On Joblet host (as root)
sudo /opt/joblet/examples/runtimes/python-3.11-ml/setup_python_3_11_ml.sh
```

### Running Examples

```bash
# Run the data analysis example
rnx run --runtime=python-3.11-ml --upload=example_data_analysis.py python example_data_analysis.py

# Test ML packages availability
rnx run --runtime=python-3.11-ml python -c "import pandas, numpy, sklearn; print('ML stack ready!')"

# Quick ML demo
rnx run --runtime=python-3.11-ml python -c "
import numpy as np
import pandas as pd
from sklearn.ensemble import RandomForestClassifier
from sklearn.model_selection import train_test_split

# Generate sample data
X = np.random.randn(100, 4)
y = (X[:, 0] + X[:, 1] > 0).astype(int)

# Train model
X_train, X_test, y_train, y_test = train_test_split(X, y, test_size=0.2)
clf = RandomForestClassifier(n_estimators=10)
clf.fit(X_train, y_train)

# Evaluate
accuracy = clf.score(X_test, y_test)
print(f'Model accuracy: {accuracy:.2%}')
print('All ML packages working!')
"

# Interactive data analysis with volume persistence
rnx run --runtime=python-3.11-ml --volume=ml-data --upload-dir=data python analysis.py
```

## 📁 Example Files

### `example_data_analysis.py`
Comprehensive ML package demonstration including:
- Data generation and manipulation with NumPy/Pandas
- Statistical analysis with SciPy
- Machine learning with Scikit-learn
- Visualization with Matplotlib/Seaborn
- Excel file handling with OpenPyXL

## 📋 Common Use Cases

### Data Analysis Pipeline
```python
# analysis.py
import pandas as pd
import numpy as np
import matplotlib.pyplot as plt
import seaborn as sns

# Load data
df = pd.read_csv('data.csv')

# Analysis
print(df.describe())
print(df.corr())

# Visualization
plt.figure(figsize=(10, 6))
sns.heatmap(df.corr(), annot=True)
plt.savefig('/volumes/ml-data/correlation.png')
```

### Machine Learning Workflow
```python
# ml_workflow.py
from sklearn.model_selection import train_test_split, cross_val_score
from sklearn.ensemble import RandomForestClassifier
from sklearn.metrics import classification_report
import joblib

# Train model
model = RandomForestClassifier(n_estimators=100)
scores = cross_val_score(model, X, y, cv=5)
print(f"CV Score: {scores.mean():.3f} (+/- {scores.std() * 2:.3f})")

# Save model
joblib.dump(model, '/volumes/ml-data/model.pkl')
```

## 🌐 Network Capabilities

```bash
# Run HTTP server with data API
rnx run --runtime=python-3.11-ml --network=ml-api --port=8000:8000 python -m http.server 8000

# Make requests from another job
rnx run --runtime=python-3.11-ml --network=ml-api python -c "
import requests
response = requests.get('http://10.200.0.2:8000')
print(response.status_code)
"
```

## 📦 Volume Persistence

```bash
# Create persistent volume
rnx volume create ml-data

# Save analysis results
rnx run --runtime=python-3.11-ml --volume=ml-data python -c "
import pandas as pd
import numpy as np

df = pd.DataFrame(np.random.randn(100, 4), columns=list('ABCD'))
df.to_csv('/volumes/ml-data/results.csv')
print('Data saved to volume')
"

# Access from another job
rnx run --runtime=python-3.11-ml --volume=ml-data python -c "
import pandas as pd
df = pd.read_csv('/volumes/ml-data/results.csv')
print(f'Loaded {len(df)} rows from volume')
"
```

## ⚡ Performance Comparison

| Operation | Traditional | Runtime | Speedup |
|-----------|------------|---------|----------|
| Environment Setup | 5-45 minutes | 0 seconds | ∞ |
| Job Startup | 30-60 seconds | 2-3 seconds | **10-30x** |
| Package Import | 5-10 seconds | <1 second | **5-10x** |
| Total Time to First Result | 5-45 minutes | 2-3 seconds | **100-900x** |

## 🔧 Troubleshooting

### Runtime Not Found
```bash
# Check if runtime is installed
rnx runtime list

# If not listed, deploy the runtime package
sudo tar -xzf python-3.11-ml-runtime.tar.gz -C /opt/joblet/runtimes/python/
```

### Package Import Errors
```bash
# Verify package versions
rnx run --runtime=python-3.11-ml pip list

# Test specific package
rnx run --runtime=python-3.11-ml python -c "import sklearn; print(sklearn.__version__)"
```

### Memory Issues
```bash
# Increase memory limit
rnx run --runtime=python-3.11-ml --max-memory=2048 python memory_intensive.py
```

## 📚 Related Documentation

- [Runtime Setup Guide](../../runtimes/python-3.11-ml/)
- [Package Documentation](../../packages/README.md)
- [Runtime System Overview](../../../docs/RUNTIME_SYSTEM.md)
- [Performance Benchmarks](../../../docs/RUNTIME_PERFORMANCE.md)