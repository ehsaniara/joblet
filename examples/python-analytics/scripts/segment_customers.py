#!/usr/bin/env python3
"""
Customer segmentation script using Python standard library
"""

import csv
import json
from collections import defaultdict
from datetime import datetime


def segment_customers():
    print("=== Customer Segmentation Analysis ===")

    # Read customer data
    customers = []
    try:
        with open('customers.csv', 'r') as f:
            reader = csv.DictReader(f)
            for row in reader:
                row['age'] = int(row['age'])
                row['purchases'] = int(row['purchases'])
                row['total_spent'] = float(row['total_spent'])
                customers.append(row)
    except FileNotFoundError:
        print("Error: customers.csv not found")
        return
    except ValueError as e:
        print(f"Error parsing customer data: {e}")
        return

    if not customers:
        print("No customer data found")
        return

    # Calculate RFM-like scores
    def calculate_score(value, thresholds):
        for i, threshold in enumerate(thresholds):
            if value <= threshold:
                return i + 1
        return len(thresholds) + 1

    # Calculate spending percentiles
    spending_values = sorted([c['total_spent'] for c in customers])
    n = len(spending_values)
    spending_thresholds = [
        spending_values[int(n * 0.2)],
        spending_values[int(n * 0.4)],
        spending_values[int(n * 0.6)],
        spending_values[int(n * 0.8)]
    ]

    # Segment customers
    segments = defaultdict(list)
    for customer in customers:
        spending_score = calculate_score(customer['total_spent'], spending_thresholds)

        if spending_score >= 4:
            segment = 'VIP'
        elif spending_score >= 3:
            segment = 'Regular'
        elif customer['purchases'] > 0:
            segment = 'Occasional'
        else:
            segment = 'New'

        segments[segment].append(customer)
        customer['segment'] = segment
        customer['spending_score'] = spending_score

    # Calculate segment statistics
    segment_stats = {}
    for segment_name, segment_customers in segments.items():
        if segment_customers:
            avg_age = sum(c['age'] for c in segment_customers) / len(segment_customers)
            avg_purchases = sum(c['purchases'] for c in segment_customers) / len(segment_customers)
            avg_spent = sum(c['total_spent'] for c in segment_customers) / len(segment_customers)

            segment_stats[segment_name] = {
                'count': len(segment_customers),
                'avg_age': round(avg_age, 1),
                'avg_purchases': round(avg_purchases, 1),
                'avg_spent': round(avg_spent, 2),
                'percentage': round(len(segment_customers) / len(customers) * 100, 1)
            }

    # Save results
    results = {
        'total_customers': len(customers),
        'segments': segment_stats,
        'thresholds': {
            'spending': spending_thresholds
        },
        'generated_at': datetime.now().isoformat()
    }

    try:
        with open('/volumes/analytics-data/customer_segments.json', 'w') as f:
            json.dump(results, f, indent=2)
        print("Segmentation saved to /volumes/analytics-data/customer_segments.json")
    except Exception as e:
        print(f"Could not save to volume, saving locally: {e}")
        with open('customer_segments.json', 'w') as f:
            json.dump(results, f, indent=2)
        print("Segmentation saved to customer_segments.json")

    # Print summary
    print(f"Total Customers: {len(customers)}")
    print("\nCustomer Segments:")
    for segment, stats in segment_stats.items():
        print(f"\n{segment} Customers:")
        print(f"  Count: {stats['count']} ({stats['percentage']}%)")
        print(f"  Avg Age: {stats['avg_age']}")
        print(f"  Avg Purchases: {stats['avg_purchases']}")
        print(f"  Avg Spent: ${stats['avg_spent']:.2f}")


if __name__ == "__main__":
    segment_customers()
