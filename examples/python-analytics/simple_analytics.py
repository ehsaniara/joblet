#!/usr/bin/env python3
"""
Simple Analytics Demo - No External Dependencies
Works with Python 3 standard library only
"""

import csv
import json
import statistics
import os
from datetime import datetime
from collections import defaultdict

def analyze_sales():
    """Simple sales analysis using only Python standard library"""
    print("\n📊 Sales Analysis (Standard Library Only)")
    print("-" * 40)
    
    # Read CSV data
    sales_data = []
    with open('sales_data.csv', 'r') as f:
        reader = csv.DictReader(f)
        for row in reader:
            row['amount'] = float(row['amount'])
            sales_data.append(row)
    
    # Calculate statistics
    amounts = [row['amount'] for row in sales_data]
    
    print(f"Total Sales Records: {len(sales_data)}")
    print(f"Total Revenue: ${sum(amounts):,.2f}")
    print(f"Average Sale: ${statistics.mean(amounts):,.2f}")
    print(f"Median Sale: ${statistics.median(amounts):,.2f}")
    print(f"Std Deviation: ${statistics.stdev(amounts):,.2f}")
    
    # Group by product
    product_sales = defaultdict(list)
    for row in sales_data:
        product_sales[row['product']].append(row['amount'])
    
    print("\nSales by Product:")
    for product, sales in sorted(product_sales.items()):
        total = sum(sales)
        avg = statistics.mean(sales)
        print(f"  {product}: Total=${total:,.2f}, Avg=${avg:,.2f}, Count={len(sales)}")
    
    # Save results
    os.makedirs('/volumes/analytics-data/results', exist_ok=True)
    
    results = {
        'timestamp': datetime.now().isoformat(),
        'total_records': len(sales_data),
        'total_revenue': sum(amounts),
        'average_sale': statistics.mean(amounts),
        'products': {
            product: {
                'total': sum(sales),
                'average': statistics.mean(sales),
                'count': len(sales)
            }
            for product, sales in product_sales.items()
        }
    }
    
    with open('/volumes/analytics-data/results/sales_analysis.json', 'w') as f:
        json.dump(results, f, indent=2)
    
    print("\n✅ Results saved to /volumes/analytics-data/results/sales_analysis.json")

def simple_clustering():
    """Simple customer segmentation using k-means (from scratch)"""
    print("\n🤖 Customer Segmentation (From Scratch)")
    print("-" * 40)
    
    # Read customer data
    customers = []
    with open('customers.csv', 'r') as f:
        reader = csv.DictReader(f)
        for row in reader:
            customers.append({
                'id': int(row['customer_id']),
                'age': float(row['age']),
                'income': float(row['income']),
                'spending': float(row['spending_score'])
            })
    
    # Normalize data (simple min-max scaling)
    def normalize(values):
        min_val = min(values)
        max_val = max(values)
        return [(v - min_val) / (max_val - min_val) for v in values]
    
    ages = normalize([c['age'] for c in customers])
    incomes = normalize([c['income'] for c in customers])
    spendings = normalize([c['spending'] for c in customers])
    
    # Simple k-means with k=3
    k = 3
    points = list(zip(ages, incomes, spendings))
    
    # Initialize centroids (first k points)
    centroids = points[:k]
    
    # Run 10 iterations
    for iteration in range(10):
        # Assign points to clusters
        clusters = [[] for _ in range(k)]
        for point in points:
            distances = []
            for centroid in centroids:
                # Euclidean distance
                dist = sum((p - c) ** 2 for p, c in zip(point, centroid)) ** 0.5
                distances.append(dist)
            nearest = distances.index(min(distances))
            clusters[nearest].append(point)
        
        # Update centroids
        new_centroids = []
        for cluster in clusters:
            if cluster:
                # Mean of all points in cluster
                centroid = tuple(
                    sum(point[i] for point in cluster) / len(cluster)
                    for i in range(3)
                )
                new_centroids.append(centroid)
            else:
                # Keep old centroid if cluster is empty
                new_centroids.append(centroids[len(new_centroids)])
        
        centroids = new_centroids
    
    # Final cluster assignment
    cluster_assignments = []
    for i, point in enumerate(points):
        distances = []
        for centroid in centroids:
            dist = sum((p - c) ** 2 for p, c in zip(point, centroid)) ** 0.5
            distances.append(dist)
        cluster_assignments.append(distances.index(min(distances)))
    
    # Analyze clusters
    cluster_stats = defaultdict(lambda: {'count': 0, 'ages': [], 'incomes': [], 'spendings': []})
    for i, cluster_id in enumerate(cluster_assignments):
        cluster_stats[cluster_id]['count'] += 1
        cluster_stats[cluster_id]['ages'].append(customers[i]['age'])
        cluster_stats[cluster_id]['incomes'].append(customers[i]['income'])
        cluster_stats[cluster_id]['spendings'].append(customers[i]['spending'])
    
    print(f"Clustered {len(customers)} customers into {k} segments:")
    for cluster_id, stats in sorted(cluster_stats.items()):
        print(f"\nCluster {cluster_id + 1}:")
        print(f"  Size: {stats['count']} customers")
        print(f"  Avg Age: {statistics.mean(stats['ages']):.1f}")
        print(f"  Avg Income: ${statistics.mean(stats['incomes']):,.2f}")
        print(f"  Avg Spending Score: {statistics.mean(stats['spendings']):.1f}")
    
    # Save results
    os.makedirs('/volumes/ml-models', exist_ok=True)
    
    results = {
        'timestamp': datetime.now().isoformat(),
        'n_clusters': k,
        'n_customers': len(customers),
        'clusters': {
            f'cluster_{i+1}': {
                'size': stats['count'],
                'avg_age': statistics.mean(stats['ages']),
                'avg_income': statistics.mean(stats['incomes']),
                'avg_spending': statistics.mean(stats['spendings'])
            }
            for i, stats in sorted(cluster_stats.items())
        }
    }
    
    with open('/volumes/ml-models/clustering_results.json', 'w') as f:
        json.dump(results, f, indent=2)
    
    print("\n✅ Results saved to /volumes/ml-models/clustering_results.json")

def process_time_series():
    """Simple time series processing"""
    print("\n📈 Time Series Processing")
    print("-" * 40)
    
    # Generate sample time series data
    import random
    random.seed(42)
    
    os.makedirs('/volumes/analytics-data/processed', exist_ok=True)
    
    for chunk_id in range(1, 5):
        data = []
        for day in range(30):
            for hour in range(24):
                value = 100 + random.gauss(0, 20) + 10 * (1 if 9 <= hour <= 17 else 0)
                data.append({
                    'chunk': chunk_id,
                    'day': day,
                    'hour': hour,
                    'value': max(0, value),
                    'is_business_hours': 9 <= hour <= 17
                })
        
        # Calculate rolling averages (simple 7-point moving average)
        values = [d['value'] for d in data]
        moving_avg = []
        window = 7
        for i in range(len(values)):
            start = max(0, i - window + 1)
            moving_avg.append(sum(values[start:i+1]) / (i - start + 1))
        
        # Add moving average to data
        for i, d in enumerate(data):
            d['moving_avg'] = moving_avg[i]
        
        # Save processed chunk
        filename = f'/volumes/analytics-data/processed/chunk_{chunk_id}.json'
        with open(filename, 'w') as f:
            json.dump(data[:100], f, indent=2)  # Save first 100 records
        
        print(f"Processed chunk {chunk_id}: {len(data)} records")
    
    print("\n✅ Processed data saved to /volumes/analytics-data/processed/")

if __name__ == "__main__":
    print("=" * 50)
    print("Python Analytics Demo - Standard Library Only")
    print("=" * 50)
    
    try:
        analyze_sales()
    except Exception as e:
        print(f"⚠️  Sales analysis skipped: {e}")
    
    try:
        simple_clustering()
    except Exception as e:
        print(f"⚠️  Customer clustering skipped: {e}")
    
    try:
        process_time_series()
    except Exception as e:
        print(f"⚠️  Time series processing skipped: {e}")
    
    print("\n" + "=" * 50)
    print("✅ Demo Complete!")
    print("=" * 50)