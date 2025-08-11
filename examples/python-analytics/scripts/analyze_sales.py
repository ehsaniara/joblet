#!/usr/bin/env python3
"""
Sales data analysis script using Python standard library
"""

import csv
import json
import statistics
from datetime import datetime
from collections import defaultdict

def analyze_sales():
    print("=== Sales Data Analysis ===")
    
    # Read sales data
    sales = []
    try:
        with open('sales_data.csv', 'r') as f:
            reader = csv.DictReader(f)
            for row in reader:
                row['amount'] = float(row['amount'])
                row['quantity'] = int(row['quantity'])
                sales.append(row)
    except FileNotFoundError:
        print("Error: sales_data.csv not found")
        return
    except ValueError as e:
        print(f"Error parsing data: {e}")
        return
    
    if not sales:
        print("No sales data found")
        return
    
    # Calculate metrics
    total_revenue = sum(s['amount'] for s in sales)
    total_quantity = sum(s['quantity'] for s in sales)
    avg_order_value = total_revenue / len(sales) if sales else 0
    
    # Product analysis
    product_sales = defaultdict(lambda: {'quantity': 0, 'revenue': 0})
    for sale in sales:
        product = sale['product']
        product_sales[product]['quantity'] += sale['quantity']
        product_sales[product]['revenue'] += sale['amount']
    
    # Top products
    top_products = sorted(product_sales.items(), 
                        key=lambda x: x[1]['revenue'], 
                        reverse=True)[:5]
    
    # Monthly trends (if date field exists)
    monthly_sales = defaultdict(float)
    for sale in sales:
        if 'date' in sale:
            month = sale['date'][:7]  # YYYY-MM
            monthly_sales[month] += sale['amount']
    
    # Prepare report
    report = {
        'summary': {
            'total_revenue': round(total_revenue, 2),
            'total_quantity': total_quantity,
            'total_orders': len(sales),
            'avg_order_value': round(avg_order_value, 2)
        },
        'top_products': [
            {
                'product': name,
                'quantity': data['quantity'],
                'revenue': round(data['revenue'], 2)
            }
            for name, data in top_products
        ],
        'monthly_trends': {
            month: round(revenue, 2)
            for month, revenue in sorted(monthly_sales.items())
        },
        'generated_at': datetime.now().isoformat()
    }
    
    # Save report
    try:
        with open('/volumes/analytics-data/sales_report.json', 'w') as f:
            json.dump(report, f, indent=2)
        print("Report saved to /volumes/analytics-data/sales_report.json")
    except Exception as e:
        print(f"Could not save to volume, saving locally: {e}")
        with open('sales_report.json', 'w') as f:
            json.dump(report, f, indent=2)
        print("Report saved to sales_report.json")
    
    # Print summary
    print(f"Total Revenue: ${total_revenue:,.2f}")
    print(f"Total Orders: {len(sales)}")
    print(f"Average Order Value: ${avg_order_value:.2f}")
    print(f"\nTop 3 Products by Revenue:")
    for product, data in top_products[:3]:
        print(f"  - {product}: ${data['revenue']:,.2f}")

if __name__ == "__main__":
    analyze_sales()