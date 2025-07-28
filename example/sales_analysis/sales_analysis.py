#!/usr/bin/env python3
import pandas as pd
import numpy as np
import json
import sys
from datetime import datetime

print(f"Starting sales analysis at {datetime.now()}")
print("=" * 50)

# Generate sample sales data
np.random.seed(42)
dates = pd.date_range(start='2024-01-01', end='2024-12-31', freq='D')
sales_data = {
    'date': dates,
    'revenue': np.random.normal(10000, 2000, len(dates)),
    'units_sold': np.random.poisson(50, len(dates)),
    'region': np.random.choice(['North', 'South', 'East', 'West'], len(dates))
}

df = pd.DataFrame(sales_data)

# Perform analysis
print("\n📊 Sales Analysis Results:")
print(f"Total Revenue: ${df['revenue'].sum():,.2f}")
print(f"Average Daily Revenue: ${df['revenue'].mean():,.2f}")
print(f"Total Units Sold: {df['units_sold'].sum():,}")

# Regional breakdown
print("\n📍 Regional Performance:")
regional_stats = df.groupby('region').agg({
    'revenue': ['sum', 'mean'],
    'units_sold': 'sum'
}).round(2)
print(regional_stats)

# Monthly trends
df['month'] = df['date'].dt.to_period('M')
monthly_stats = df.groupby('month').agg({
    'revenue': 'sum',
    'units_sold': 'sum'
})

print("\n📈 Monthly Trends:")
for month, row in monthly_stats.iterrows():
    print(f"{month}: Revenue=${row['revenue']:,.2f}, Units={row['units_sold']:,}")

# Save results
results = {
    'analysis_date': datetime.now().isoformat(),
    'total_revenue': float(df['revenue'].sum()),
    'avg_daily_revenue': float(df['revenue'].mean()),
    'total_units': int(df['units_sold'].sum()),
    'regional_summary': regional_stats.to_dict()
}

with open('analysis_results.json', 'w') as f:
    json.dump(results, f, indent=2)

print("\n✅ Analysis complete! Results saved to analysis_results.json")
print(f"Finished at {datetime.now()}")