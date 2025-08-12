# Python 3.11 ML Multi-Architecture Runtime

Advanced Python 3.11 runtime with machine learning libraries, intelligently optimized for multiple CPU architectures and
Linux distributions.

## 🌐 Multi-Architecture ML Support

### 📊 Platform Compatibility & ML Support

| **Architecture**  | **Support Level**  | **ML Libraries**    | **Performance** | **Package Size** |
|-------------------|--------------------|---------------------|-----------------|------------------|
| **x86_64/amd64**  | ✅ Full ML Stack    | All packages        | Maximum         | ~1.5-2GB         |
| **aarch64/arm64** | ✅ Most ML Packages | Native ARM64 builds | Native ARM64    | ~1.2-1.8GB       |
| **armv7l/armhf**  | ⚠️ Basic ML Only   | NumPy + basics      | Good            | ~500-800MB       |

### 🧠 Machine Learning Package Matrix

| **Package**      | **x86_64**  | **ARM64**  | **ARM32**       | **Features**              |
|------------------|-------------|------------|-----------------|---------------------------|
| **NumPy**        | ✅ Optimized | ✅ Native   | ✅ Compatible    | Linear algebra, arrays    |
| **Pandas**       | ✅ Full      | ✅ Full     | ❌ Limited       | Data analysis, CSV/JSON   |
| **Scikit-learn** | ✅ Full      | ✅ Compiled | ❌ Not available | ML algorithms, pipelines  |
| **Matplotlib**   | ✅ Full      | ✅ Full     | ❌ Limited       | Plotting, visualization   |
| **SciPy**        | ✅ Full      | ✅ Full     | ❌ Limited       | Scientific computing      |
| **Seaborn**      | ✅ Full      | ✅ Full     | ❌ Limited       | Statistical visualization |

### 🌍 Distribution Support

- **Ubuntu/Debian**: Full ML build environment with APT
- **CentOS/RHEL/Amazon Linux**: YUM with development packages (may need EPEL)
- **Fedora**: DNF with excellent ML package support
- **openSUSE/SLES**: Zypper with development libraries
- **Arch/Manjaro**: Pacman with comprehensive ML support
- **Alpine**: APK with extended compilation time for ML packages

## 🚀 Quick Start

### Auto-Detecting Remote Deployment

```bash
# Automatically detects target architecture and deploys ML-optimized runtime
./deploy_to_host.sh user@target-host

# Examples for different ML capabilities:
./deploy_to_host.sh user@x86-server     # Full ML stack (NumPy, Pandas, Scikit-learn, etc.)
./deploy_to_host.sh user@arm64-server   # Most ML packages with ARM64 optimization
./deploy_to_host.sh user@pi-server      # Basic ML (NumPy only) for ARM32
./deploy_to_host.sh user@aws-instance   # Amazon Linux ML stack
```

### Local Multi-Architecture Setup

```bash
# Auto-detects local system and installs ML-optimized Python 3.11
sudo ./setup_python_3_11_ml_multiarch.sh

# View platform compatibility and ML support levels
./setup_python_3_11_ml_multiarch.sh --help
```

### Zero-Contamination Deployment (Production)

```bash
# Step 1: Build on test system (⚠️ installs ML build dependencies)
sudo ./setup_python_3_11_ml_multiarch.sh
# Creates architecture-specific: /tmp/runtime-deployments/python-3.11-ml-runtime.tar.gz

# Step 2: Deploy to production (zero host modification)
scp /tmp/runtime-deployments/python-3.11-ml-runtime.tar.gz admin@prod-host:/tmp/
ssh admin@prod-host 'sudo tar -xzf /tmp/python-3.11-ml-runtime.tar.gz -C /opt/joblet/runtimes/python/'
```

## 📦 What's Included by Architecture

### All Architectures Base

- **Python 3.11.9** (compiled from source)
- **Virtual Environment Support** (`python -m venv`)
- **pip Package Manager** (latest version)
- **requests** for HTTP operations

### x86_64 Full ML Stack

- **NumPy** ≥1.24.3, <2.0 (pinned for stability)
- **Pandas** ≥2.0.3, <2.1 (data analysis)
- **Scikit-learn** ≥1.3.0, <1.4 (ML algorithms)
- **Matplotlib** ≥3.7.0, <3.8 (visualization)
- **SciPy** ≥1.11.0, <1.12 (scientific computing)
- **Seaborn** ≥0.12.0, <0.13 (statistical visualization)
- **OpenPyXL** (Excel file support)

### ARM64 Most ML Packages

- **NumPy** (ARM64 native)
- **Pandas** (ARM64 optimized)
- **Scikit-learn** (compiled from source if needed)
- **Matplotlib** (ARM64 compatible)
- **SciPy** (ARM64 builds)
- **Seaborn** (full support)
- **OpenPyXL** (full support)

### ARM32 Basic ML

- **NumPy** (ARM32 compatible build)
- **requests** (HTTP operations)
- Limited additional packages due to compilation constraints

## 🎯 Usage Examples

### Basic ML Testing

```bash
# List available runtimes
rnx runtime list

# View Python 3.11 ML runtime details with architecture info
rnx runtime info python-3.11-ml

# Test ML stack availability
rnx run --runtime=python-3.11-ml python -c "
import sys
print(f'Python {sys.version} on {sys.platform}')

packages = ['numpy', 'pandas', 'sklearn', 'matplotlib', 'scipy']
for pkg in packages:
    try:
        mod = __import__(pkg)
        version = getattr(mod, '__version__', 'unknown')
        print(f'✅ {pkg}: {version}')
    except ImportError:
        print(f'❌ {pkg}: Not available')
"
```

### Data Analysis Example

```bash
# Create a data analysis script
cat > data_analysis_demo.py << 'EOF'
import numpy as np
import pandas as pd
import matplotlib.pyplot as plt
import seaborn as sns

# Create sample data
np.random.seed(42)
data = {
    'x': np.random.normal(0, 1, 1000),
    'y': np.random.normal(0, 1, 1000),
    'category': np.random.choice(['A', 'B', 'C'], 1000)
}
df = pd.DataFrame(data)

# Basic analysis
print("Data Analysis Results:")
print(f"Dataset shape: {df.shape}")
print(f"Mean x: {df['x'].mean():.3f}")
print(f"Mean y: {df['y'].mean():.3f}")
print("\nCategory counts:")
print(df['category'].value_counts())

# Create correlation matrix
correlation = df[['x', 'y']].corr()
print(f"\nCorrelation between x and y: {correlation.iloc[0,1]:.3f}")

print("✅ Data analysis completed successfully!")
EOF

rnx run --runtime=python-3.11-ml --upload=data_analysis_demo.py python data_analysis_demo.py
```

### Machine Learning Example

```bash
# Create ML classification example
cat > ml_classification_demo.py << 'EOF'
import numpy as np
from sklearn.datasets import make_classification
from sklearn.model_selection import train_test_split
from sklearn.ensemble import RandomForestClassifier
from sklearn.metrics import accuracy_score, classification_report

# Generate sample dataset
X, y = make_classification(n_samples=1000, n_features=20, n_classes=3, 
                          n_informative=10, random_state=42)

# Split data
X_train, X_test, y_train, y_test = train_test_split(X, y, test_size=0.2, random_state=42)

# Train model
clf = RandomForestClassifier(n_estimators=100, random_state=42)
clf.fit(X_train, y_train)

# Predictions
y_pred = clf.predict(X_test)
accuracy = accuracy_score(y_test, y_pred)

print("Machine Learning Classification Results:")
print(f"Dataset shape: {X.shape}")
print(f"Training samples: {X_train.shape[0]}")
print(f"Test samples: {X_test.shape[0]}")
print(f"Accuracy: {accuracy:.3f}")

# Feature importance
feature_importance = clf.feature_importances_
top_features = np.argsort(feature_importance)[-5:][::-1]
print(f"\nTop 5 important features: {top_features}")

print("✅ ML classification completed successfully!")
EOF

rnx run --runtime=python-3.11-ml --upload=ml_classification_demo.py python ml_classification_demo.py
```

### Template-Based Usage

```bash
# Use YAML templates for ML workflows (if available)
cd /opt/joblet/examples/python-3.11-ml
rnx run --template=jobs.yaml:ml-analysis
rnx run --template=jobs.yaml:data-visualization
rnx run --template=jobs.yaml:model-training
```

## ⚡ Architecture-Specific Performance

### ML Performance Benefits

| **Architecture** | **Traditional Setup**      | **Runtime Startup** | **Speedup**          | **ML Capabilities** |
|------------------|----------------------------|---------------------|----------------------|---------------------|
| **x86_64**       | 20-60 min (ML packages)    | 2-3 seconds         | **400-1200x faster** | Full ML ecosystem   |
| **ARM64**        | 30-90 min (compilation)    | 3-6 seconds         | **600-1800x faster** | Most ML packages    |
| **ARM32**        | 60-180 min (limited build) | 8-15 seconds        | **450-1350x faster** | Basic ML only       |

### Package Installation Performance

**x86_64 Benefits:**

- **Binary wheels** available for all packages
- **Instant imports** - no compilation delays
- **Optimized BLAS/LAPACK** integration

**ARM64 Benefits:**

- **Native ARM64** compiled packages
- **NEON SIMD** optimizations where available
- **64-bit performance** advantages

**ARM32 Limitations:**

- **Source compilation** may be required
- **Limited memory** for complex models
- **Reduced package availability**

## 🔧 Architecture-Specific Troubleshooting

### x86_64/amd64 Issues

```bash
# Should work out-of-box with full ML stack
# Test all ML packages
rnx run --runtime=python-3.11-ml python -c "
import numpy as np
import pandas as pd  
import sklearn
print('✅ Full ML stack working')
print(f'NumPy: {np.__version__}')
print(f'Pandas: {pd.__version__}')
print(f'Scikit-learn: {sklearn.__version__}')
"
```

### ARM64/aarch64 Issues

```bash
# Verify ARM64 ML packages
rnx run --runtime=python-3.11-ml python -c "
import numpy as np
print(f'NumPy on ARM64: {np.__version__}')
print(f'BLAS info: {np.show_config()}')
"

# If scikit-learn compilation issues
rnx run --runtime=python-3.11-ml pip install --no-binary=scikit-learn scikit-learn
```

### ARM32/armhf Issues

```bash
# Check available packages on ARM32
rnx run --runtime=python-3.11-ml pip list

# ARM32 typically only has NumPy
rnx run --runtime=python-3.11-ml python -c "
import numpy as np
print(f'NumPy ARM32: {np.__version__}')
print('ARM32 ML runtime ready for basic numerical computing')
"

# For additional packages on ARM32, try source installation
rnx run --runtime=python-3.11-ml pip install --no-binary=pandas pandas
```

### ML Package Compatibility Issues

```bash
# NumPy 2.0 compatibility check (runtime uses NumPy 1.x)
rnx run --runtime=python-3.11-ml python -c "
import numpy as np
print(f'NumPy version: {np.__version__}')
assert np.__version__.startswith('1.'), 'Using stable NumPy 1.x branch'
"

# Pandas compatibility verification
rnx run --runtime=python-3.11-ml python -c "
import pandas as pd
import numpy as np
df = pd.DataFrame({'test': [1, 2, 3]})
print('✅ Pandas-NumPy compatibility verified')
"
```

### Distribution-Specific ML Issues

**Amazon Linux:**

```bash
# May need EPEL for some ML dependencies
sudo yum install epel-release
sudo yum install blas-devel lapack-devel
```

**Alpine Linux:**

```bash
# Extended compilation time for ML packages
sudo apk add gfortran openblas-dev lapack-dev
```

**CentOS/RHEL:**

```bash
# May need development tools
sudo yum groupinstall "Development Tools"
sudo yum install blas-devel lapack-devel
```

## 📊 Runtime Manifest

The runtime creates a detailed manifest at `/opt/joblet/runtimes/python/python-3.11-ml/runtime.yml`:

```yaml
name: "python-3.11-ml"
version: "3.11.9"
description: "Python 3.11 with ML packages optimized for amd64"
type: "python-ml"
system:
  architecture: "amd64"  # Detected architecture
  os: "Linux"
  distribution: "ubuntu"  # Detected distribution
  ml_support: "full"     # full/most/limited based on architecture
paths:
  python_home: "/opt/joblet/runtimes/python/python-3.11-ml/python-install"
  venv_home: "/opt/joblet/runtimes/python/python-3.11-ml/ml-venv"
binaries:
  python: "/opt/joblet/runtimes/python/python-3.11-ml/ml-venv/bin/python"
  pip: "/opt/joblet/runtimes/python/python-3.11-ml/ml-venv/bin/pip"
features:
  - "NumPy for numerical computing"
  - "Pandas for data analysis (available)"
  - "Scikit-learn for ML (full support)"
  - "Matplotlib for visualization (available)"
  - "SciPy for scientific computing (available)"
  - "Architecture-optimized build"
packages:
  - "numpy>=1.24.3,<2.0"
  - "pandas>=2.0.3,<2.1"
  - "scikit-learn>=1.3.0,<1.4"
  - "matplotlib>=3.7.0,<3.8"
  - "seaborn>=0.12.0,<0.13"
  - "scipy>=1.11.0,<1.12"
```

## 🌟 ML Feature Examples

### Advanced Data Analysis

```python
# advanced_ml_analysis.py
import numpy as np
import pandas as pd
import matplotlib.pyplot as plt
import seaborn as sns
from sklearn.preprocessing import StandardScaler
from sklearn.decomposition import PCA
from sklearn.cluster import KMeans

# Generate complex dataset
np.random.seed(42)
n_samples = 500
n_features = 10

# Create multi-dimensional data
data = np.random.randn(n_samples, n_features)
data[:n_samples//2, :3] += 3  # Create clusters

# Convert to DataFrame
feature_names = [f'feature_{i}' for i in range(n_features)]
df = pd.DataFrame(data, columns=feature_names)

# Standardize features
scaler = StandardScaler()
data_scaled = scaler.fit_transform(df)

# Apply PCA
pca = PCA(n_components=2)
data_pca = pca.fit_transform(data_scaled)

# K-means clustering
kmeans = KMeans(n_clusters=3, random_state=42)
clusters = kmeans.fit_predict(data_scaled)

# Results
print("Advanced ML Analysis Results:")
print(f"Original data shape: {df.shape}")
print(f"PCA explained variance ratio: {pca.explained_variance_ratio_}")
print(f"Number of clusters found: {len(np.unique(clusters))}")

# Add results to dataframe
df['cluster'] = clusters
df['pca1'] = data_pca[:, 0]
df['pca2'] = data_pca[:, 1]

print("✅ Advanced ML analysis completed!")
```

### Time Series Analysis

```python
# time_series_ml.py  
import numpy as np
import pandas as pd
from sklearn.linear_model import LinearRegression
from sklearn.metrics import mean_squared_error, r2_score

# Generate time series data
dates = pd.date_range('2023-01-01', periods=365, freq='D')
trend = np.linspace(100, 150, 365)
seasonal = 10 * np.sin(2 * np.pi * np.arange(365) / 365.25)
noise = np.random.normal(0, 5, 365)
values = trend + seasonal + noise

# Create DataFrame
ts_df = pd.DataFrame({
    'date': dates,
    'value': values,
    'trend': trend,
    'seasonal': seasonal
})

# Feature engineering
ts_df['day_of_year'] = ts_df['date'].dt.dayofyear
ts_df['month'] = ts_df['date'].dt.month
ts_df['quarter'] = ts_df['date'].dt.quarter

# Simple ML prediction
X = ts_df[['day_of_year', 'month', 'quarter']].values
y = ts_df['value'].values

# Train/test split
split_point = int(0.8 * len(X))
X_train, X_test = X[:split_point], X[split_point:]
y_train, y_test = y[:split_point], y[split_point:]

# Train model
model = LinearRegression()
model.fit(X_train, y_train)

# Predictions
y_pred = model.predict(X_test)
mse = mean_squared_error(y_test, y_pred)
r2 = r2_score(y_test, y_pred)

print("Time Series ML Results:")
print(f"Dataset period: {dates[0]} to {dates[-1]}")
print(f"Total data points: {len(ts_df)}")
print(f"Training samples: {len(X_train)}")
print(f"Test samples: {len(X_test)}")
print(f"Mean Squared Error: {mse:.2f}")
print(f"R² Score: {r2:.3f}")

print("✅ Time series ML analysis completed!")
```

## 🏗️ Manual Installation Steps

If you need to understand what the scripts do:

### 1. System Detection and ML Dependencies

```bash
# Detect architecture for ML capability assessment
arch=$(uname -m)
case "$arch" in
    x86_64) ML_SUPPORT="full" ;;
    aarch64) ML_SUPPORT="most" ;;
    armv7l) ML_SUPPORT="limited" ;;
esac

# Install ML build dependencies (varies by distribution)
# Ubuntu/Debian:
sudo apt-get install python3-dev build-essential libssl-dev libffi-dev \
                     libblas-dev liblapack-dev gfortran

# CentOS/RHEL/Amazon Linux:
sudo yum install python3-devel gcc gcc-gfortran openssl-devel libffi-devel \
                 blas-devel lapack-devel
```

### 2. Architecture-Specific Python Build

```bash
# Configure Python with ML optimizations
./configure --prefix=/opt/joblet/runtimes/python/python-3.11-ml/python-install \
           --enable-optimizations \
           --with-ensurepip=install \
           --enable-shared \
           LDFLAGS="-Wl,-rpath=/opt/joblet/runtimes/python/python-3.11-ml/python-install/lib"

# Architecture-specific compilation
case "$arch" in
    x86_64) make -j$(nproc) ;;                    # Full parallel build
    aarch64) make -j$(nproc) ;;                   # ARM64 parallel build
    armv7l) make -j$(($(nproc)/2)) ;;             # Reduced parallelism for ARM32
esac
```

### 3. ML Package Installation by Architecture

```bash
# Full ML stack (x86_64)
pip install "numpy>=1.24.3,<2.0" "pandas>=2.0.3,<2.1" "scikit-learn>=1.3.0,<1.4" \
            "matplotlib>=3.7.0,<3.8" "seaborn>=0.12.0,<0.13" "scipy>=1.11.0,<1.12"

# Most packages (ARM64) - with fallback compilation
pip install "numpy>=1.24.3,<2.0" "pandas>=2.0.3,<2.1" || \
pip install --no-binary=pandas "pandas>=2.0.3,<2.1"

# Basic packages (ARM32)
pip install "numpy>=1.24.3,<2.0" || \
pip install --no-binary=numpy "numpy>=1.24.3,<2.0"
```

## 📚 Related Documentation

- **[Multi-Arch Main README](../README.md)**: Complete multi-architecture system overview
- **[System Detection](../common/detect_system.sh)**: Architecture detection library
- **[Python Base Runtime](../python-3.11/README.md)**: Non-ML version
- **[Example Usage](/opt/joblet/examples/python-3.11-ml/)**: ML examples and templates

## 🎉 Summary

The Python 3.11 ML multi-architecture runtime provides:

- **🧠 Smart ML Support**: Full/Most/Basic packages based on architecture capabilities
- **🌐 Universal Linux Support**: Optimized ML stack for x86_64, ARM64, and ARM32
- **🚀 Instant ML Ready**: 400-1800x faster than traditional ML environment setup
- **🔒 Zero Production Impact**: Portable packaging for clean deployments
- **📊 Production ML**: NumPy 1.x stability with modern Python 3.11 features
- **🎯 Auto-Optimization**: Architecture-aware package selection and compilation

**Perfect for machine learning development, data analysis, and AI workloads across any Linux architecture!** 🐍🧠⚡