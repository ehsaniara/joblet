#!/usr/bin/env python3
"""
Advanced Sales Data Analysis Script for Joblet
Processes uploaded sales and customer data to generate insights
Requires pandas, numpy, and matplotlib (installed via requirements.txt)
"""

import pandas as pd
import numpy as np
import os
import sys
from datetime import datetime

# Try to import matplotlib for visualization (optional)
try:
    import matplotlib
    matplotlib.use('Agg')  # Use non-interactive backend
    import matplotlib.pyplot as plt
    HAS_MATPLOTLIB = True
    print("✅ Matplotlib available - will generate visualizations")
except ImportError:
    HAS_MATPLOTLIB = False
    print("⚠️  Matplotlib not available - skipping visualizations")

def print_separator(title):
    """Print a section separator"""
    print(f"\n{'='*60}")
    print(f" {title}")
    print(f"{'='*60}")

def load_data():
    """Load and validate the uploaded CSV files"""
    print("Loading uploaded data files...")
    
    # Check if files exist (try both root and data/ subdirectory)
    sales_file = 'sales_data.csv' if os.path.exists('sales_data.csv') else 'data/sales_data.csv'
    customers_file = 'customers.csv' if os.path.exists('customers.csv') else 'data/customers.csv'
    
    if not os.path.exists(sales_file):
        print(f"ERROR: Required file sales_data.csv not found in current directory or data/ subdirectory!")
        sys.exit(1)
    if not os.path.exists(customers_file):
        print(f"ERROR: Required file customers.csv not found in current directory or data/ subdirectory!")
        sys.exit(1)
    
    try:
        # Load sales data
        sales_df = pd.read_csv(sales_file)
        print(f"✓ Loaded sales data: {len(sales_df)} records")
        
        # Load customer data
        customers_df = pd.read_csv(customers_file)
        print(f"✓ Loaded customer data: {len(customers_df)} records")
        
        return sales_df, customers_df
        
    except Exception as e:
        print(f"ERROR loading data: {e}")
        sys.exit(1)

def analyze_sales_summary(sales_df):
    """Generate sales summary statistics"""
    print_separator("SALES SUMMARY")
    
    # Convert date column if it exists
    if 'date' in sales_df.columns:
        sales_df['date'] = pd.to_datetime(sales_df['date'])
    
    # Basic statistics
    total_revenue = sales_df['amount'].sum()
    avg_transaction = sales_df['amount'].mean()
    total_transactions = len(sales_df)
    
    print(f"Total Revenue: ${total_revenue:,.2f}")
    print(f"Average Transaction: ${avg_transaction:.2f}")
    print(f"Total Transactions: {total_transactions:,}")
    
    # Revenue by product
    if 'product' in sales_df.columns:
        print(f"\nTop 5 Products by Revenue:")
        product_revenue = sales_df.groupby('product')['amount'].sum().sort_values(ascending=False).head()
        for product, revenue in product_revenue.items():
            print(f"  {product}: ${revenue:,.2f}")
    
    return {
        'total_revenue': total_revenue,
        'avg_transaction': avg_transaction,
        'total_transactions': total_transactions
    }

def analyze_customers(customers_df, sales_df):
    """Analyze customer demographics and behavior"""
    print_separator("CUSTOMER ANALYSIS")
    
    # Merge sales with customer data if customer_id exists
    if 'customer_id' in sales_df.columns and 'customer_id' in customers_df.columns:
        merged_df = sales_df.merge(customers_df, on='customer_id', how='left')
        
        # Analysis by region
        if 'region' in customers_df.columns:
            print("Revenue by Region:")
            region_revenue = merged_df.groupby('region')['amount'].sum().sort_values(ascending=False)
            for region, revenue in region_revenue.items():
                print(f"  {region}: ${revenue:,.2f}")
        
        # Customer value analysis
        customer_value = merged_df.groupby('customer_id')['amount'].agg(['sum', 'count']).sort_values('sum', ascending=False)
        print(f"\nTop 5 Customers by Value:")
        for customer_id, (value, transactions) in customer_value.head().iterrows():
            print(f"  Customer {customer_id}: ${value:,.2f} ({transactions} transactions)")
    
    else:
        print("Customer demographic summary:")
        if 'region' in customers_df.columns:
            region_counts = customers_df['region'].value_counts()
            print("Customers by Region:")
            for region, count in region_counts.items():
                print(f"  {region}: {count} customers")

def analyze_trends(sales_df):
    """Analyze sales trends over time"""
    print_separator("SALES TRENDS")
    
    if 'date' in sales_df.columns:
        sales_df['date'] = pd.to_datetime(sales_df['date'])
        sales_df['month'] = sales_df['date'].dt.to_period('M')
        
        monthly_sales = sales_df.groupby('month')['amount'].sum()
        print("Monthly Sales:")
        for month, sales in monthly_sales.items():
            print(f"  {month}: ${sales:,.2f}")
            
        # Growth rate
        if len(monthly_sales) > 1:
            growth_rate = ((monthly_sales.iloc[-1] - monthly_sales.iloc[0]) / monthly_sales.iloc[0]) * 100
            print(f"\nOverall Growth Rate: {growth_rate:.1f}%")
            
        # Generate visualization if matplotlib is available
        if HAS_MATPLOTLIB and len(monthly_sales) > 1:
            create_sales_chart(monthly_sales)
    else:
        print("No date column found - cannot analyze trends")

def create_sales_chart(monthly_sales):
    """Create sales trend visualization"""
    try:
        plt.figure(figsize=(10, 6))
        
        # Convert period index to string for plotting
        months = [str(month) for month in monthly_sales.index]
        values = monthly_sales.values
        
        plt.plot(months, values, marker='o', linewidth=2, markersize=8)
        plt.title('Monthly Sales Trend', fontsize=16, fontweight='bold')
        plt.xlabel('Month', fontsize=12)
        plt.ylabel('Sales Amount ($)', fontsize=12)
        plt.grid(True, alpha=0.3)
        
        # Format y-axis to show currency
        plt.gca().yaxis.set_major_formatter(plt.FuncFormatter(lambda x, p: f'${x:,.0f}'))
        
        # Rotate x-axis labels for better readability
        plt.xticks(rotation=45)
        
        # Adjust layout to prevent label cutoff
        plt.tight_layout()
        
        # Save the chart
        plt.savefig('sales_trend.png', dpi=150, bbox_inches='tight')
        print("📊 Sales trend chart saved as 'sales_trend.png'")
        
        plt.close()  # Free memory
        
    except Exception as e:
        print(f"⚠️  Failed to create visualization: {e}")

def data_quality_report(sales_df, customers_df):
    """Generate data quality report"""
    print_separator("DATA QUALITY REPORT")
    
    print("Sales Data Quality:")
    print(f"  Total rows: {len(sales_df)}")
    print(f"  Missing values: {sales_df.isnull().sum().sum()}")
    print(f"  Duplicate rows: {sales_df.duplicated().sum()}")
    
    print(f"\nCustomer Data Quality:")
    print(f"  Total rows: {len(customers_df)}")
    print(f"  Missing values: {customers_df.isnull().sum().sum()}")
    print(f"  Duplicate rows: {customers_df.duplicated().sum()}")
    
    # Column info
    print(f"\nSales Data Columns: {list(sales_df.columns)}")
    print(f"Customer Data Columns: {list(customers_df.columns)}")

def main():
    """Main analysis function"""
    print_separator("JOBLET PYTHON DATA ANALYSIS")
    print(f"Analysis started at: {datetime.now()}")
    print(f"Working directory: {os.getcwd()}")
    print(f"Job ID: {os.environ.get('JOB_ID', 'unknown')}")
    
    # Load data
    sales_df, customers_df = load_data()
    
    # Run analyses
    sales_summary = analyze_sales_summary(sales_df)
    analyze_customers(customers_df, sales_df)
    analyze_trends(sales_df)
    data_quality_report(sales_df, customers_df)
    
    print_separator("ANALYSIS COMPLETE")
    print("✓ All analyses completed successfully!")
    print(f"✓ Processed {len(sales_df)} sales records and {len(customers_df)} customer records")
    print(f"✓ Total revenue analyzed: ${sales_summary['total_revenue']:,.2f}")
    
    # Save summary to file
    with open('analysis_summary.txt', 'w') as f:
        f.write(f"Sales Analysis Summary\n")
        f.write(f"=====================\n")
        f.write(f"Total Revenue: ${sales_summary['total_revenue']:,.2f}\n")
        f.write(f"Average Transaction: ${sales_summary['avg_transaction']:.2f}\n")
        f.write(f"Total Transactions: {sales_summary['total_transactions']:,}\n")
        f.write(f"Analysis completed at: {datetime.now()}\n")
    
    print("✓ Summary saved to analysis_summary.txt")

if __name__ == "__main__":
    main()