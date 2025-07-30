#!/usr/bin/env python3
"""
Simple Sales Data Analysis Script for Joblet (Standard Library Only)
Processes uploaded sales and customer data to generate insights
"""

import csv
import os
import sys
from datetime import datetime
from collections import defaultdict, Counter

def print_separator(title):
    """Print a section separator"""
    print(f"\n{'='*60}")
    print(f" {title}")
    print(f"{'='*60}")

def load_csv_data(filename):
    """Load CSV data from file"""
    data = []
    if not os.path.exists(filename):
        print(f"ERROR: File {filename} not found!")
        return []
    
    try:
        with open(filename, 'r') as f:
            reader = csv.DictReader(f)
            data = list(reader)
        print(f"✓ Loaded {filename}: {len(data)} records")
        return data
    except Exception as e:
        print(f"ERROR loading {filename}: {e}")
        return []

def analyze_sales_summary(sales_data):
    """Generate sales summary statistics"""
    print_separator("SALES SUMMARY")
    
    if not sales_data:
        print("No sales data available")
        return {}
    
    # Calculate totals
    total_revenue = 0
    total_transactions = len(sales_data)
    product_revenue = defaultdict(float)
    
    for sale in sales_data:
        try:
            amount = float(sale.get('amount', 0))
            total_revenue += amount
            product = sale.get('product', 'Unknown')
            product_revenue[product] += amount
        except ValueError:
            continue
    
    avg_transaction = total_revenue / total_transactions if total_transactions > 0 else 0
    
    print(f"Total Revenue: ${total_revenue:,.2f}")
    print(f"Average Transaction: ${avg_transaction:.2f}")
    print(f"Total Transactions: {total_transactions:,}")
    
    # Top products by revenue
    print(f"\nTop 5 Products by Revenue:")
    sorted_products = sorted(product_revenue.items(), key=lambda x: x[1], reverse=True)[:5]
    for product, revenue in sorted_products:
        print(f"  {product}: ${revenue:,.2f}")
    
    return {
        'total_revenue': total_revenue,
        'avg_transaction': avg_transaction,
        'total_transactions': total_transactions
    }

def analyze_customers(customers_data, sales_data):
    """Analyze customer demographics and behavior"""
    print_separator("CUSTOMER ANALYSIS")
    
    if not customers_data:
        print("No customer data available")
        return
    
    # Create customer lookup
    customer_lookup = {c['customer_id']: c for c in customers_data}
    
    # Revenue by region
    region_revenue = defaultdict(float)
    customer_revenue = defaultdict(float)
    
    for sale in sales_data:
        try:
            customer_id = sale.get('customer_id')
            amount = float(sale.get('amount', 0))
            
            if customer_id in customer_lookup:
                region = customer_lookup[customer_id].get('region', 'Unknown')
                region_revenue[region] += amount
                customer_revenue[customer_id] += amount
        except (ValueError, KeyError):
            continue
    
    # Display region analysis
    print("Revenue by Region:")
    sorted_regions = sorted(region_revenue.items(), key=lambda x: x[1], reverse=True)
    for region, revenue in sorted_regions:
        print(f"  {region}: ${revenue:,.2f}")
    
    # Top customers
    print(f"\nTop 5 Customers by Value:")
    sorted_customers = sorted(customer_revenue.items(), key=lambda x: x[1], reverse=True)[:5]
    for customer_id, revenue in sorted_customers:
        customer_name = customer_lookup.get(customer_id, {}).get('name', f'Customer {customer_id}')
        print(f"  {customer_name} (ID: {customer_id}): ${revenue:,.2f}")
    
    # Customer type analysis
    customer_types = Counter(c.get('customer_type', 'Unknown') for c in customers_data)
    print(f"\nCustomers by Type:")
    for ctype, count in customer_types.items():
        print(f"  {ctype}: {count} customers")

def analyze_trends(sales_data):
    """Analyze sales trends over time"""
    print_separator("SALES TRENDS")
    
    if not sales_data:
        print("No sales data available")
        return
    
    # Group by month
    monthly_sales = defaultdict(float)
    
    for sale in sales_data:
        try:
            date_str = sale.get('date', '')
            amount = float(sale.get('amount', 0))
            
            # Extract year-month from date (assuming YYYY-MM-DD format)
            if len(date_str) >= 7:
                year_month = date_str[:7]  # YYYY-MM
                monthly_sales[year_month] += amount
        except ValueError:
            continue
    
    if monthly_sales:
        print("Monthly Sales:")
        sorted_months = sorted(monthly_sales.items())
        for month, sales in sorted_months:
            print(f"  {month}: ${sales:,.2f}")
        
        # Calculate growth if we have multiple months
        if len(sorted_months) > 1:
            first_month_sales = sorted_months[0][1]
            last_month_sales = sorted_months[-1][1]
            if first_month_sales > 0:
                growth_rate = ((last_month_sales - first_month_sales) / first_month_sales) * 100
                print(f"\nOverall Growth Rate: {growth_rate:.1f}%")
    else:
        print("No valid date information found for trend analysis")

def data_quality_report(sales_data, customers_data):
    """Generate data quality report"""
    print_separator("DATA QUALITY REPORT")
    
    # Sales data quality
    print("Sales Data Quality:")
    print(f"  Total rows: {len(sales_data)}")
    
    sales_missing = 0
    for sale in sales_data:
        for key in ['date', 'customer_id', 'product', 'amount']:
            if not sale.get(key) or sale[key].strip() == '':
                sales_missing += 1
                break
    
    print(f"  Rows with missing key fields: {sales_missing}")
    
    # Customer data quality
    print(f"\nCustomer Data Quality:")
    print(f"  Total rows: {len(customers_data)}")
    
    customers_missing = 0
    for customer in customers_data:
        for key in ['customer_id', 'name', 'region']:
            if not customer.get(key) or customer[key].strip() == '':
                customers_missing += 1
                break
    
    print(f"  Rows with missing key fields: {customers_missing}")
    
    # Show available columns
    if sales_data:
        print(f"\nSales Data Columns: {list(sales_data[0].keys())}")
    if customers_data:
        print(f"Customer Data Columns: {list(customers_data[0].keys())}")

def main():
    """Main analysis function"""
    print_separator("JOBLET PYTHON DATA ANALYSIS")
    print(f"Analysis started at: {datetime.now()}")
    print(f"Working directory: {os.getcwd()}")
    print(f"Job ID: {os.environ.get('JOB_ID', 'unknown')}")
    
    # List available files
    print(f"\nAvailable files:")
    for filename in os.listdir('.'):
        if os.path.isfile(filename):
            size = os.path.getsize(filename)
            print(f"  {filename} ({size} bytes)")
    
    # Load data
    sales_data = load_csv_data('sales_data.csv')
    customers_data = load_csv_data('customers.csv')
    
    if not sales_data and not customers_data:
        print("❌ No data files found or loaded successfully")
        sys.exit(1)
    
    # Run analyses
    sales_summary = analyze_sales_summary(sales_data)
    analyze_customers(customers_data, sales_data)
    analyze_trends(sales_data)
    data_quality_report(sales_data, customers_data)
    
    print_separator("ANALYSIS COMPLETE")
    print("✅ All analyses completed successfully!")
    print(f"✅ Processed {len(sales_data)} sales records and {len(customers_data)} customer records")
    if sales_summary.get('total_revenue'):
        print(f"✅ Total revenue analyzed: ${sales_summary['total_revenue']:,.2f}")
    
    # Save summary to file
    with open('analysis_summary.txt', 'w') as f:
        f.write(f"Sales Analysis Summary\n")
        f.write(f"=====================\n")
        f.write(f"Total Revenue: ${sales_summary.get('total_revenue', 0):,.2f}\n")
        f.write(f"Average Transaction: ${sales_summary.get('avg_transaction', 0):.2f}\n")
        f.write(f"Total Transactions: {sales_summary.get('total_transactions', 0):,}\n")
        f.write(f"Analysis completed at: {datetime.now()}\n")
    
    print("✅ Summary saved to analysis_summary.txt")

if __name__ == "__main__":
    main()