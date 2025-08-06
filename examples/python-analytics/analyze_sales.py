#!/usr/bin/env python3
import pandas as pd
import numpy as np
import matplotlib.pyplot as plt
import os

def analyze_sales():
    # Load data
    df = pd.read_csv('sales_data.csv')
    print(f"Loaded {len(df)} sales records")
    
    # Basic statistics
    print("\nSales Summary:")
    print(df.describe())
    
    # Monthly analysis
    df['date'] = pd.to_datetime(df['date'])
    monthly_sales = df.groupby(df['date'].dt.to_period('M'))['amount'].sum()
    
    # Save results to persistent volume
    results_dir = '/volumes/analytics-data/results'
    os.makedirs(results_dir, exist_ok=True)
    
    monthly_sales.to_csv(f'{results_dir}/monthly_sales.csv')
    
    # Generate visualization
    plt.figure(figsize=(12, 6))
    monthly_sales.plot(kind='bar')
    plt.title('Monthly Sales Trends')
    plt.xticks(rotation=45)
    plt.tight_layout()
    plt.savefig(f'{results_dir}/sales_trend.png')
    
    print(f"\nResults saved to {results_dir}")
    return monthly_sales

if __name__ == "__main__":
    analyze_sales()