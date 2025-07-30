#!/usr/bin/env python3
"""
Self-Installing Sales Data Analysis Script for Joblet
This script installs its own dependencies in the isolated chroot environment
"""

import os
import sys
import subprocess
from datetime import datetime

def install_dependencies():
    """Install required Python packages in the isolated environment"""
    print("🔧 Installing required Python packages in isolated environment...")
    
    # First, try to install pip if it's not available
    try:
        subprocess.run([sys.executable, '-m', 'pip', '--version'], 
                      capture_output=True, check=True)
        print("✅ pip is available")
    except subprocess.CalledProcessError:
        print("📦 Installing pip...")
        try:
            # Download and install pip directly (APT doesn't work in minimal chroot)
            print("📥 Downloading pip installer...")
            import urllib.request
            urllib.request.urlretrieve('https://bootstrap.pypa.io/get-pip.py', 'get-pip.py')
            
            print("🔧 Installing pip...")
            result = subprocess.run([sys.executable, 'get-pip.py', '--user'], 
                         capture_output=True, text=True, timeout=120)
            
            if result.returncode == 0:
                print("✅ pip installed successfully")
                # Add user bin to PATH
                os.environ['PATH'] = f"{os.path.expanduser('~/.local/bin')}:{os.environ.get('PATH', '')}"
            else:
                print(f"❌ Failed to install pip: {result.stderr}")
                return False
                
        except Exception as e:
            print(f"⚠️  Could not install pip: {e}")
            return False
    
    # List of required packages
    packages = ['pandas', 'numpy', 'matplotlib']
    
    for package in packages:
        try:
            print(f"📦 Installing {package}...")
            result = subprocess.run([
                sys.executable, '-m', 'pip', 'install', '--user', package
            ], capture_output=True, text=True, timeout=120)
            
            if result.returncode == 0:
                print(f"✅ {package} installed successfully")
            else:
                print(f"⚠️  {package} installation had issues: {result.stderr}")
                
        except subprocess.TimeoutExpired:
            print(f"⚠️  {package} installation timed out")
        except Exception as e:
            print(f"⚠️  Failed to install {package}: {e}")
    
    # Add user-installed packages to Python path
    import site
    site.main()
    
    print("✅ Package installation complete!")
    return True

def main():
    """Main function that handles both installation and analysis"""
    print("="*60)
    print(" JOBLET SELF-INSTALLING DATA ANALYSIS")
    print("="*60)
    print(f"Started at: {datetime.now()}")
    print(f"Working directory: {os.getcwd()}")
    print(f"Job ID: {os.environ.get('JOB_ID', 'unknown')}")
    print(f"Python version: {sys.version}")
    
    # Install dependencies first
    install_dependencies()
    
    # Now import and run the analysis
    try:
        import pandas as pd
        import numpy as np
        print("✅ Successfully imported pandas and numpy")
        
        # Check for matplotlib
        try:
            import matplotlib
            matplotlib.use('Agg')  # Non-interactive backend
            import matplotlib.pyplot as plt
            has_matplotlib = True
            print("✅ Successfully imported matplotlib")
        except ImportError:
            has_matplotlib = False
            print("⚠️  Matplotlib not available")
        
        # Run the actual data analysis
        run_analysis(pd, np, plt if has_matplotlib else None)
        
    except ImportError as e:
        print(f"❌ Failed to import required packages: {e}")
        print("Falling back to standard library analysis...")
        run_simple_analysis()

def run_analysis(pd, np, plt):
    """Run pandas-based data analysis"""
    print("\n" + "="*60)
    print(" PANDAS DATA ANALYSIS")
    print("="*60)
    
    # Load data (check both root and data/ subdirectory)
    try:
        sales_file = 'sales_data.csv' if os.path.exists('sales_data.csv') else 'data/sales_data.csv'
        customers_file = 'customers.csv' if os.path.exists('customers.csv') else 'data/customers.csv'
        
        sales_df = pd.read_csv(sales_file)
        customers_df = pd.read_csv(customers_file)
        print(f"✅ Loaded {len(sales_df)} sales records and {len(customers_df)} customer records")
    except Exception as e:
        print(f"❌ Failed to load data: {e}")
        return
    
    # Sales summary
    print(f"\n📊 SALES SUMMARY:")
    total_revenue = sales_df['amount'].sum()
    avg_transaction = sales_df['amount'].mean()
    print(f"Total Revenue: ${total_revenue:,.2f}")
    print(f"Average Transaction: ${avg_transaction:.2f}")
    print(f"Total Transactions: {len(sales_df):,}")
    
    # Top products
    print(f"\n🏆 TOP PRODUCTS:")
    product_revenue = sales_df.groupby('product')['amount'].sum().sort_values(ascending=False).head()
    for product, revenue in product_revenue.items():
        print(f"  {product}: ${revenue:,.2f}")
    
    # Customer analysis
    print(f"\n👥 CUSTOMER ANALYSIS:")
    merged = sales_df.merge(customers_df, on='customer_id', how='left')
    region_revenue = merged.groupby('region')['amount'].sum().sort_values(ascending=False)
    for region, revenue in region_revenue.items():
        print(f"  {region}: ${revenue:,.2f}")
    
    # Monthly trends
    print(f"\n📈 MONTHLY TRENDS:")
    sales_df['date'] = pd.to_datetime(sales_df['date'])
    monthly = sales_df.groupby(sales_df['date'].dt.to_period('M'))['amount'].sum()
    for month, amount in monthly.items():
        print(f"  {month}: ${amount:,.2f}")
    
    # Create visualization if matplotlib is available
    if plt is not None:
        try:
            fig, (ax1, ax2) = plt.subplots(1, 2, figsize=(15, 6))
            
            # Monthly sales trend
            monthly.plot(kind='line', marker='o', ax=ax1)
            ax1.set_title('Monthly Sales Trend')
            ax1.set_ylabel('Sales ($)')
            ax1.grid(True, alpha=0.3)
            
            # Revenue by region
            region_revenue.plot(kind='bar', ax=ax2, color='skyblue')
            ax2.set_title('Revenue by Region')
            ax2.set_ylabel('Revenue ($)')
            ax2.tick_params(axis='x', rotation=45)
            
            plt.tight_layout()
            plt.savefig('analysis_charts.png', dpi=150, bbox_inches='tight')
            print("📊 Charts saved as 'analysis_charts.png'")
        except Exception as e:
            print(f"⚠️  Visualization failed: {e}")
    
    print("\n✅ Analysis complete!")

def run_simple_analysis():
    """Fallback analysis using only standard library"""
    print("\n" + "="*60)
    print(" SIMPLE DATA ANALYSIS (Standard Library)")
    print("="*60)
    
    import csv
    from collections import defaultdict
    
    # Load sales data (check both root and data/ subdirectory)
    sales_data = []
    try:
        sales_file = 'sales_data.csv' if os.path.exists('sales_data.csv') else 'data/sales_data.csv'
        with open(sales_file, 'r') as f:
            reader = csv.DictReader(f)
            sales_data = list(reader)
        print(f"✅ Loaded {len(sales_data)} sales records")
    except Exception as e:
        print(f"❌ Failed to load sales data: {e}")
        return
    
    # Basic analysis
    total_revenue = sum(float(row['amount']) for row in sales_data)
    print(f"Total Revenue: ${total_revenue:,.2f}")
    print(f"Total Transactions: {len(sales_data)}")
    
    # Product revenue
    product_revenue = defaultdict(float)
    for row in sales_data:
        product_revenue[row['product']] += float(row['amount'])
    
    print(f"\nTop Products:")
    for product, revenue in sorted(product_revenue.items(), key=lambda x: x[1], reverse=True)[:5]:
        print(f"  {product}: ${revenue:,.2f}")
    
    print("\n✅ Simple analysis complete!")

if __name__ == "__main__":
    main()