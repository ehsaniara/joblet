# Python Analytics Examples

Examples demonstrating how to use Joblet for Python-based data science and analytics workflows.

## 📊 Examples Overview

| Example                                                             | Files                                  | Description                                    | Complexity   | Resources |
|---------------------------------------------------------------------|----------------------------------------|------------------------------------------------|--------------|-----------|
| [Sales Analysis](#sales-analysis)                                   | `analyze_sales.py`, `sales_data.csv`   | Pandas DataFrame operations with visualization | Beginner     | 512MB RAM |
| [Customer Segmentation](#customer-segmentation)                     | `train_clustering.py`, `customers.csv` | K-means clustering with scikit-learn           | Intermediate | 2GB RAM   |
| [Distributed Feature Engineering](#distributed-feature-engineering) | `feature_engineering.py`               | Multi-job parallel data processing             | Advanced     | 1GB RAM   |
| [Complete Demo Suite](#complete-demo-suite)                         | `run_demos.sh`                         | Automated execution of all examples            | All Levels   | 4GB RAM   |

## 🚀 Quick Start

### Run All Demos (Recommended)

```bash
# Execute complete demo suite
./run_demos.sh
```

### Prerequisites (handled automatically by demo script)

```bash
# Create volumes for data persistence
rnx volume create analytics-data --size=2GB --type=filesystem
rnx volume create ml-models --size=1GB --type=filesystem
```

### Dependencies

All Python dependencies are listed in `requirements.txt`:

```bash
# Install locally (optional)
pip install -r requirements.txt
```

## 📈 Sales Analysis

Process sales CSV data with Pandas and generate visualizations with Matplotlib.

### Files Included

- **`analyze_sales.py`**: Complete sales analysis script
- **`sales_data.csv`**: Sample sales dataset (30 records)

### Manual Execution

```bash
# Upload data and analysis script
rnx run --upload=sales_data.csv --upload=analyze_sales.py \
       --volume=analytics-data \
       --max-memory=512 \
       python3 analyze_sales.py
```

### Expected Output

- Monthly sales trend analysis
- Sales summary statistics
- Visualization saved as `/volumes/analytics-data/results/sales_trend.png`
- CSV results saved as `/volumes/analytics-data/results/monthly_sales.csv`

## 🤖 Customer Segmentation

Train K-means clustering model with scikit-learn for customer segmentation.

### Files Included

- **`train_clustering.py`**: Complete ML training pipeline
- **`customers.csv`**: Sample customer dataset (50 records)

### Manual Execution

```bash
# Train clustering model with resource limits
rnx run --upload=train_clustering.py \
       --volume=ml-models \
       --volume=analytics-data \
       --max-cpu=75 --max-memory=2048 \
       --env=MODEL_VERSION=v1.2 \
       python3 train_clustering.py
```

### Expected Output

- Optimal cluster count determination using silhouette analysis
- Trained K-means model saved as `.pkl` file
- Customer segmentation results
- Model metadata and performance metrics

**train_clustering.py:**

```python
#!/usr/bin/env python3
import pandas as pd
import numpy as np
from sklearn.cluster import KMeans
from sklearn.preprocessing import StandardScaler
from sklearn.metrics import silhouette_score
import joblib
import os
from datetime import datetime


def train_customer_segmentation():
    print("Starting Customer Segmentation Training")

    # Load customer data
    df = pd.read_csv('/volumes/analytics-data/customers.csv')

    # Feature engineering
    features = ['age', 'income', 'spending_score', 'purchase_frequency']
    X = df[features].copy()

    # Handle missing values
    X = X.fillna(X.mean())

    # Scale features
    scaler = StandardScaler()
    X_scaled = scaler.fit_transform(X)

    # Determine optimal clusters using elbow method
    inertias = []
    silhouette_scores = []
    k_range = range(2, 11)

    print("Finding optimal number of clusters...")
    for k in k_range:
        kmeans = KMeans(n_clusters=k, random_state=42, n_init=10)
        cluster_labels = kmeans.fit_predict(X_scaled)

        inertias.append(kmeans.inertia_)
        sil_score = silhouette_score(X_scaled, cluster_labels)
        silhouette_scores.append(sil_score)
        print(f"k={k}: Silhouette Score = {sil_score:.3f}")

    # Select best k (highest silhouette score)
    best_k = k_range[np.argmax(silhouette_scores)]
    print(f"\\nOptimal clusters: {best_k}")

    # Train final model
    final_model = KMeans(n_clusters=best_k, random_state=42, n_init=10)
    cluster_labels = final_model.fit_predict(X_scaled)

    # Add cluster labels to dataframe
    df['cluster'] = cluster_labels

    # Analyze clusters
    print("\\nCluster Analysis:")
    cluster_summary = df.groupby('cluster')[features].mean()
    print(cluster_summary)

    # Save model and results
    model_dir = '/volumes/ml-models'
    timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
    model_version = os.getenv('MODEL_VERSION', 'v1.0')

    # Save model artifacts
    joblib.dump(final_model, f'{model_dir}/clustering_model_{model_version}_{timestamp}.pkl')
    joblib.dump(scaler, f'{model_dir}/scaler_{model_version}_{timestamp}.pkl')

    # Save results
    df.to_csv(f'{model_dir}/segmented_customers_{timestamp}.csv', index=False)
    cluster_summary.to_csv(f'{model_dir}/cluster_summary_{timestamp}.csv')

    # Save metadata
    metadata = {
        'model_version': model_version,
        'timestamp': timestamp,
        'n_clusters': best_k,
        'silhouette_score': max(silhouette_scores),
        'features': features,
        'n_samples': len(df)
    }

    import json
    with open(f'{model_dir}/model_metadata_{timestamp}.json', 'w') as f:
        json.dump(metadata, f, indent=2)

    print(f"\\nModel saved: clustering_model_{model_version}_{timestamp}.pkl")
    print(f"Silhouette Score: {max(silhouette_scores):.3f}")

    return final_model, scaler, metadata


if __name__ == "__main__":
    train_customer_segmentation()
```

## 🌐 Distributed Feature Engineering

Coordinate multiple Python jobs for parallel data processing.

### Files Included

- **`feature_engineering.py`**: Distributed data processing script

### Automatic Execution

The demo script automatically:

1. Creates sample time-series data chunks
2. Processes 4 chunks in parallel with different workers
3. Applies feature engineering (time-based features, rolling statistics)
4. Saves processed results to `/volumes/analytics-data/processed/`

### Manual Execution

```bash
# Process data chunks in parallel
for chunk in {1..4}; do
    rnx run --upload=feature_engineering.py \
           --volume=analytics-data \
           --max-memory=1024 \
           --env=CHUNK_ID=$chunk \
           --env=TOTAL_CHUNKS=4 \
           --name="features-chunk-$chunk" \
           python3 feature_engineering.py &
done

# Wait for all jobs to complete
wait
echo "All feature engineering jobs completed"
```

### Expected Output

- 4 processed data chunks with engineered features
- Time-based features (hour, day of week, weekend indicator)
- Rolling statistics (7-day mean and standard deviation)
- Normalized numerical features

## 🔄 Complete Demo Suite

Execute all Python analytics examples with a single command.

### Files Included

- **`run_demos.sh`**: Master demo script
- **`requirements.txt`**: All Python dependencies

### What It Runs

1. **Sales Analysis**: Processes sample sales data and generates visualizations
2. **Customer Segmentation**: Trains K-means clustering model on customer data
3. **Distributed Feature Engineering**: Processes data chunks in parallel across multiple jobs

### Execution

```bash
# Run complete demo suite
./run_demos.sh
```

### Demo Flow

1. Creates required volumes automatically
2. Uploads sample customer data to analytics volume
3. Executes sales analysis with resource limits
4. Trains ML model with CPU and memory constraints
5. Runs parallel feature engineering across 4 workers
6. Displays results and locations for inspection

## 📁 Demo Results

After running the demos, check results in the following locations:

### Sales Analysis Results

```bash
# View sales analysis outputs
rnx run --volume=analytics-data ls -la /volumes/analytics-data/results/
rnx run --volume=analytics-data cat /volumes/analytics-data/results/monthly_sales.csv
```

### ML Model Artifacts

```bash
# View trained models and metadata
rnx run --volume=ml-models ls -la /volumes/ml-models/
rnx run --volume=ml-models cat /volumes/ml-models/model_metadata_*.json
```

### Feature Engineering Results

```bash
# View processed data chunks
rnx run --volume=analytics-data ls -la /volumes/analytics-data/processed/
rnx run --volume=analytics-data head /volumes/analytics-data/processed/features_chunk_1.csv
```

## 🎯 Best Practices Demonstrated

### Resource Management

- **Memory Limits**: Examples show 512MB for basic processing, 2GB for ML training
- **CPU Limits**: ML training uses 75% CPU to prevent resource exhaustion
- **Parallel Processing**: Feature engineering distributes work across multiple jobs

### Data Persistence

- **Volumes**: All results saved to persistent filesystem volumes
- **Structured Storage**: Organized directory structure for different data types
- **Metadata**: Model artifacts include comprehensive metadata and metrics

### Production Patterns

- **Error Handling**: Comprehensive logging and error recovery
- **Resource Monitoring**: Scripts include performance metrics collection
- **Reproducibility**: Environment variables and versioning for consistent results

## 🚀 Next Steps

1. **Modify Sample Data**: Replace `sales_data.csv` and `customers.csv` with your real data
2. **Scale Processing**: Increase memory limits and dataset sizes for production workloads
3. **Add More Models**: Extend `train_clustering.py` with different algorithms (Random Forest, XGBoost)
4. **Connect Data Sources**: Modify scripts to load from databases, APIs, or cloud storage
5. **Production Deployment**: Add monitoring, alerting, and automated model retraining

## 📊 Monitoring Your Jobs

```bash
# Monitor job execution in real-time
rnx monitor

# Check job status
rnx list

# View job logs
rnx log <job-id>

# Monitor volume usage
rnx volume list
```

## 📚 Additional Resources

- [Pandas Documentation](https://pandas.pydata.org/docs/)
- [Scikit-learn User Guide](https://scikit-learn.org/stable/user_guide.html)
- [Matplotlib Tutorials](https://matplotlib.org/stable/tutorials/index.html)
- [Python Data Science Handbook](https://jakevdp.github.io/PythonDataScienceHandbook/)
- [Joblet Documentation](../../docs/) - Configuration and advanced usage