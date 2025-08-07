# Python Analytics Examples

Examples demonstrating how to use Joblet for Python-based data analytics workflows using only the Python standard library.

## 📊 Overview

These examples show how to perform data analytics tasks in Joblet's isolated environment without requiring external dependencies. All examples use only Python 3's standard library.

| Example | Files | Description | Resources |
|---------|-------|-------------|-----------|
| Sales Analysis | `simple_analytics.py`, `sales_data.csv` | Statistical analysis of sales data | 512MB RAM |
| Customer Segmentation | `simple_analytics.py`, `customers.csv` | K-means clustering from scratch | 512MB RAM |
| Time Series Processing | `simple_analytics.py` | Generate and process time series data | 512MB RAM |

## 🚀 Quick Start

```bash
# Run the complete analytics demo
./run_demo.sh
```

This will:
1. Create persistent volumes for data storage
2. Upload sample data and Python scripts
3. Execute analytics in an isolated environment
4. Save results to volumes for inspection

## 📋 Prerequisites

- Joblet server with Python 3 installed
- RNX client configured and connected  
- 512MB RAM available for jobs

No external Python packages required - everything runs with the standard library!

## 📈 Features

### Sales Analysis
- Calculate total revenue, average sale, median, and standard deviation
- Group sales by product category
- Export results as JSON for further processing

### Customer Segmentation  
- Implement K-means clustering from scratch
- Segment customers based on age, income, and spending patterns
- Analyze cluster characteristics

### Time Series Processing
- Generate synthetic time series data
- Calculate moving averages
- Process data in chunks for scalability

## 🏃 Running the Examples

### Automated Execution

```bash
# Run all examples with one command
./run_demo.sh
```

### Manual Execution

```bash
# Create volumes first
rnx volume create analytics-data --size=1GB --type=filesystem
rnx volume create ml-models --size=500MB --type=filesystem

# Run the analytics
rnx run --upload=simple_analytics.py \
        --upload=sales_data.csv \
        --upload=customers.csv \
        --volume=analytics-data \
        --volume=ml-models \
        --max-memory=512 \
        python3 simple_analytics.py
```

## 📊 Viewing Results

After running the demo, inspect the results:

```bash
# View sales analysis results
rnx run --volume=analytics-data cat /volumes/analytics-data/results/sales_analysis.json

# List processed time series files
rnx run --volume=analytics-data ls /volumes/analytics-data/processed/

# View a specific processed chunk
rnx run --volume=analytics-data cat /volumes/analytics-data/processed/chunk_1.json
```

## 📁 File Structure

```
python-analytics/
├── README.md              # This file
├── run_demo.sh           # Main demo script
├── simple_analytics.py   # Python analytics implementation
├── sales_data.csv        # Sample sales dataset (30 records)
└── customers.csv         # Sample customer dataset (50 records)
```

## 💡 Key Concepts Demonstrated

### Resource Management
- Jobs run with memory limits (512MB)
- Isolated execution environment
- No dependency conflicts

### Data Persistence
- Results saved to persistent volumes
- Data accessible across job runs
- JSON format for easy inspection

### Pure Python Implementation
- Statistical analysis with `statistics` module
- K-means clustering implemented from scratch
- CSV processing with `csv` module
- JSON output with `json` module

## 🔍 Sample Output

```
==================================================
Python Analytics Demo - Standard Library Only
==================================================

📊 Sales Analysis (Standard Library Only)
----------------------------------------
Total Sales Records: 30
Total Revenue: $5,496.43
Average Sale: $183.21
Median Sale: $156.78
Std Deviation: $114.77

Sales by Product:
  keyboard: Total=$550.68, Avg=$91.78, Count=6
  laptop: Total=$2,800.44, Avg=$280.04, Count=10
  monitor: Total=$1,671.43, Avg=$208.93, Count=8
  mouse: Total=$473.88, Avg=$78.98, Count=6

✅ Results saved to /volumes/analytics-data/results/sales_analysis.json

🤖 Customer Segmentation (From Scratch)
----------------------------------------
Clustered 50 customers into 3 segments:

Cluster 1:
  Size: 22 customers
  Avg Age: 59.5
  Avg Income: $31,000.00
  Avg Spending Score: 53.5

[... more clusters ...]

📈 Time Series Processing
----------------------------------------
Processed chunk 1: 720 records
Processed chunk 2: 720 records
Processed chunk 3: 720 records
Processed chunk 4: 720 records

✅ Processed data saved to /volumes/analytics-data/processed/
```

## 🚀 Next Steps

1. **Modify the data**: Replace sample CSV files with your own data
2. **Extend the analytics**: Add more statistical calculations or algorithms
3. **Scale up**: Process larger datasets by adjusting memory limits
4. **Add visualization**: Generate ASCII charts or export data for external plotting

## 📚 Additional Resources

- [Python Statistics Module](https://docs.python.org/3/library/statistics.html)
- [Python CSV Module](https://docs.python.org/3/library/csv.html)
- [Python JSON Module](https://docs.python.org/3/library/json.html)
- [Joblet Documentation](../../docs/)