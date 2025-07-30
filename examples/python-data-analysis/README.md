# Python Data Analysis Example

This example demonstrates how to upload Python scripts and CSV data files to joblet for data analysis tasks. It shows multiple approaches for handling dependencies in the isolated chroot environment.

## What This Example Does

1. Uploads Python scripts and CSV data files to joblet
2. Processes sales and customer data using pandas (or standard library)
3. Generates analysis results, statistics, and visualizations
4. Demonstrates library installation in isolated chroot environment
5. Shows both individual file uploads and directory uploads
6. Outputs results to the job logs for easy viewing

## Files

- `analyze_sales_simple.py` - Standard library version (no dependencies required)
- `analyze_sales.py` - Advanced pandas version with matplotlib visualizations
- `analyze_with_install.py` - Self-installing version that downloads pip and installs dependencies
- `run_with_pandas.sh` - Installs pip/pandas directly and runs full analysis
- `requirements.txt` - Python package dependencies (pandas, numpy, matplotlib)
- `data/sales_data.csv` - Sample sales transaction data (30 records)
- `data/customers.csv` - Sample customer information data (18 records)
- `run_analysis.sh` - Simple version runner (standard library only)

## How to Run

### Option 1: Simple Analysis (No Dependencies)
Uses only Python standard library - works immediately:

```bash
# Upload individual files
rnx run --upload=analyze_sales_simple.py --upload=data/sales_data.csv --upload=data/customers.csv python3 analyze_sales_simple.py

# Or use convenience script
./run_analysis.sh
```

### Option 2: Advanced Analysis with Direct Pip Installation
Installs pip directly without APT and runs pandas analysis:

```bash
# Upload entire directory (demonstrates --upload-dir flag)
rnx run --upload-dir=. bash run_with_pandas.sh
```

### Option 3: Self-Installing Script
Script that installs its own dependencies:

```bash
# Upload directory and run self-installing script
rnx run --upload-dir=. python3 analyze_with_install.py
```

### Option 4: Manual Installation
For direct control over the installation process:

```bash
# Download pip and install packages manually
rnx run --upload-dir=. bash -c "wget https://bootstrap.pypa.io/get-pip.py && python3 get-pip.py --user && python3 -m pip install --user pandas numpy matplotlib && python3 analyze_sales.py"
```

## Key Features Demonstrated

### 🔧 **Chroot Environment & Library Installation**
- Jobs run in isolated chroot environment
- Libraries must be installed within the job
- APT package manager has issues in minimal chroot - use direct pip download instead
- User-space installation (`--user` flag) required for permissions
- Direct pip installation via `get-pip.py` is the most reliable method

### 📁 **File Upload Options**
- `--upload=filename` - Upload individual files
- `--upload-dir=directory` - Upload entire directory
- Files are available in `/work` directory within the job

### 🐍 **Python Dependency Management**
- `requirements.txt` for package specification
- `pip3 install --user` for user-space installation
- Fallback to standard library if packages fail to install
- Environment variable setup for installed packages

## Expected Output

The analysis will output:
- **Sales Summary**: Total revenue, average transaction, transaction count
- **Product Analysis**: Top 5 products by revenue
- **Customer Analysis**: Revenue by region, top customers by value
- **Sales Trends**: Monthly sales patterns and growth rates
- **Data Quality Report**: Missing data analysis and column info
- **Visualizations**: Sales trend charts (if matplotlib available)

## Example Output

```
============================================================
 JOBLET PYTHON DATA ANALYSIS
============================================================
Analysis started at: 2025-07-30 06:43:03.064497
Working directory: /work
Job ID: 3

✓ Loaded sales_data.csv: 30 records
✓ Loaded customers.csv: 18 records

============================================================
 SALES SUMMARY
============================================================
Total Revenue: $8,799.70
Average Transaction: $293.32
Total Transactions: 30

Top 5 Products by Revenue:
  Laptop Pro: $6,499.95
  Office Chair: $899.97
  Headphones: $399.98
...
```

All results are captured in the job logs and can be viewed with `rnx log <job_id>`.