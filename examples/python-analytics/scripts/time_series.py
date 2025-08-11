#!/usr/bin/env python3
"""
Time series analysis script using Python standard library
"""

import json
import math
import random
import statistics
from datetime import datetime, timedelta
from collections import defaultdict

def analyze_time_series():
    print("=== Time Series Analysis ===")
    
    # Generate sample time series data
    random.seed(42)
    
    # Create synthetic daily sales data
    start_date = datetime(2024, 1, 1)
    data = []
    
    for i in range(365):
        date = start_date + timedelta(days=i)
        # Add trend and seasonality
        trend = i * 0.5
        seasonality = 50 * math.sin(2 * math.pi * i / 365)
        noise = random.gauss(0, 10)
        value = 100 + trend + seasonality + noise
        
        # Weekly pattern (higher on weekends)
        if date.weekday() >= 5:
            value *= 1.3
        
        data.append({
            'date': date.strftime('%Y-%m-%d'),
            'value': max(0, value),
            'day_of_week': date.strftime('%A'),
            'month': date.strftime('%B'),
            'quarter': f"Q{(date.month - 1) // 3 + 1}"
        })
    
    # Analyze patterns
    # 1. Daily statistics
    values = [d['value'] for d in data]
    daily_stats = {
        'mean': round(statistics.mean(values), 2),
        'median': round(statistics.median(values), 2),
        'stdev': round(statistics.stdev(values), 2),
        'min': round(min(values), 2),
        'max': round(max(values), 2)
    }
    
    # 2. Weekly patterns
    weekly_avg = defaultdict(list)
    for d in data:
        weekly_avg[d['day_of_week']].append(d['value'])
    
    weekly_patterns = {
        day: round(statistics.mean(values), 2)
        for day, values in weekly_avg.items()
    }
    
    # 3. Monthly trends
    monthly_avg = defaultdict(list)
    for d in data:
        monthly_avg[d['month']].append(d['value'])
    
    monthly_trends = {
        month: round(statistics.mean(values), 2)
        for month, values in monthly_avg.items()
    }
    
    # 4. Moving averages
    window = 7
    moving_avg = []
    for i in range(window, len(values)):
        avg = statistics.mean(values[i-window:i])
        moving_avg.append({
            'date': data[i]['date'],
            'value': round(data[i]['value'], 2),
            'ma7': round(avg, 2)
        })
    
    # Save analysis
    analysis = {
        'period': f"{data[0]['date']} to {data[-1]['date']}",
        'daily_statistics': daily_stats,
        'weekly_patterns': weekly_patterns,
        'monthly_trends': monthly_trends,
        'sample_moving_average': moving_avg[:30],  # First 30 days
        'generated_at': datetime.now().isoformat()
    }
    
    try:
        with open('/volumes/analytics-data/time_series_analysis.json', 'w') as f:
            json.dump(analysis, f, indent=2, default=str)
        print("Analysis saved to /volumes/analytics-data/time_series_analysis.json")
    except Exception as e:
        print(f"Could not save to volume, saving locally: {e}")
        with open('time_series_analysis.json', 'w') as f:
            json.dump(analysis, f, indent=2, default=str)
        print("Analysis saved to time_series_analysis.json")
    
    # Print summary
    print(f"Analyzed {len(data)} days of data")
    print(f"\nDaily Statistics:")
    print(f"  Mean: ${daily_stats['mean']:.2f}")
    print(f"  Std Dev: ${daily_stats['stdev']:.2f}")
    print(f"  Range: ${daily_stats['min']:.2f} - ${daily_stats['max']:.2f}")
    
    print(f"\nBest Day of Week: {max(weekly_patterns, key=weekly_patterns.get)}")
    print(f"Best Month: {max(monthly_trends, key=monthly_trends.get)}")

if __name__ == "__main__":
    analyze_time_series()