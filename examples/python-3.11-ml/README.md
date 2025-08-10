# Python 3.11 ML Example

This example demonstrates **two different runtime approaches** for the same ML code:

1. **`python:3.11-ml` runtime** - Heavy runtime with ML libraries pre-installed on host
2. **`python:3.11` runtime** - Lightweight runtime with dependencies packaged locally

## 🎯 Runtime Comparison

| Runtime Approach         | Runtime Size | Dependencies     | Upload Size | Total  | Use Case                   |
|--------------------------|--------------|------------------|-------------|--------|----------------------------| 
| **`python:3.11-ml`**     | 500MB+       | Pre-installed    | ~5KB        | 500MB+ | Quick ML jobs, no setup    |
| **`python:3.11` + deps** | 50MB         | Packaged locally | ~150MB      | 200MB  | Custom versions, isolation |

## 🚀 Quick Start

### Option 1: Heavy Runtime (`python:3.11-ml`)

```bash
cd examples/python-3.11-ml

# No setup needed - ML libs pre-installed on host
rnx run --runtime=python:3.11-ml python example_data_analysis.py
```

### Option 2: Packaged Dependencies (`python:3.11`)

```bash
cd examples/python-3.11-ml

# Install dependencies locally first
./setup.sh              # Installs all ML dependencies

# Upload entire project including dependencies
rnx run --runtime=python:3.11 --upload-dir=. python example_data_analysis.py
```

## 📦 Example Included

### `example_data_analysis.py` - Complete ML Pipeline

Works with **both runtime approaches**:

- **Full ML Libraries**:
    - `pandas==2.1.0` - Data manipulation and analysis
    - `numpy==1.24.3` - Numerical computing
    - `scikit-learn==1.3.0` - Machine learning
    - `matplotlib==3.7.2` - Data visualization
    - `seaborn==0.12.2` - Statistical plotting
    - `scipy==1.11.4` - Scientific computing
    - `requests==2.31.0` - HTTP requests

## 🔄 Runtime Benefits Comparison

### `python:3.11-ml` Runtime

✅ **No setup** - ML libraries pre-installed on host  
✅ **Instant deployment** - Just upload your script  
✅ **No gRPC limits** - Upload only your code (~5KB)  
❌ **Fixed versions** - Can't customize library versions  
❌ **Larger host** - 500MB+ runtime footprint

### `python:3.11` + Packaged Dependencies

✅ **Exact versions** - Use precisely the packages you need  
✅ **Perfect reproducibility** - Dependencies travel with code  
✅ **No version conflicts** - Each project has isolated dependencies  
✅ **Lighter host** - 50MB runtime footprint  
❌ **Setup required** - Must package dependencies first  
❌ **Upload size** - Need to upload dependencies (~150MB)

## 📋 Usage Instructions

### For `python:3.11-ml` Runtime (Pre-installed ML)

```bash
cd examples/python-3.11-ml

# No setup needed - run directly
rnx run --runtime=python:3.11-ml python example_data_analysis.py
```

### For `python:3.11` Runtime (Packaged Dependencies)

```bash
cd examples/python-3.11-ml

# 1. Package dependencies locally
./setup.sh              # Installs all ML dependencies

# 2. Test locally (optional)
python3 example_data_analysis.py

# 3. Deploy with dependencies
rnx run --runtime=python:3.11 --upload-dir=. python example_data_analysis.py
```

## 📊 What the Example Does

The `example_data_analysis.py` script demonstrates a complete ML workflow:

### 🔢 Data Generation & Analysis

- Creates synthetic dataset (1000 samples, 5 features)
- Generates binary classification target
- Displays comprehensive dataset statistics using Pandas

### 🤖 Machine Learning Pipeline

- Trains Random Forest classifier with Scikit-learn
- Evaluates model performance with accuracy and classification report
- Analyzes feature importance rankings

### 📈 Data Visualization

- Feature distribution histograms
- Correlation heatmap between features
- Feature importance bar chart
- Target class distribution pie chart
- Saves results as `ml_analysis_results.png`

### 🌐 Network Testing

- Tests HTTP connectivity using Requests library
- Demonstrates external API access capability

## 📁 Project Structure

```
python-3.11-ml/
├── README.md                    # This documentation
├── requirements.txt             # ML dependencies list
├── example_data_analysis.py     # Complete ML example
├── setup.sh                     # Dependency installer
└── lib/                         # Dependencies (after setup.sh)
    ├── pandas/                  # Data manipulation  
    ├── numpy/                   # Numerical computing
    ├── sklearn/                 # Machine learning
    ├── matplotlib/              # Plotting
    ├── seaborn/                 # Statistical plots  
    ├── scipy/                   # Scientific computing
    ├── requests/                # HTTP client
    └── ... (other dependencies)
```

## 🎯 When to Use Each Runtime

### Use `python:3.11-ml` when:

- Quick prototyping and testing
- Standard ML workflows with common packages
- Don't need specific package versions
- Want zero setup time

### Use `python:3.11` + packaged deps when:

- Need specific package versions
- Want perfect reproducibility
- Deploying to production environments
- Multiple projects with different requirements
- Want isolated, controlled environments

This example demonstrates **both approaches working with the same ML code** - choose the runtime that best fits your
workflow!