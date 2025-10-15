#!/usr/bin/env python3
"""
Data processing script for multi-job workflow test
"""
import json
import sys


def main():
    print("=== Data Processor Job ===")

    # Create some test data
    data = {
        "timestamp": "2025-08-27T07:45:00Z",
        "values": [10, 20, 30, 40, 50],
        "status": "processed",
        "runtime": "python-3.11-ml"
    }

    # Calculate basic statistics
    values = data["values"]
    data["statistics"] = {
        "count": len(values),
        "sum": sum(values),
        "mean": sum(values) / len(values),
        "min": min(values),
        "max": max(values)
    }

    # Write data to output file
    with open("processed_data.json", "w") as f:
        json.dump(data, f, indent=2)

    print("✓ Data processed successfully")
    print(f"✓ Statistics calculated: mean={data['statistics']['mean']}")
    print("✓ Output written to processed_data.json")

    return 0


if __name__ == "__main__":
    sys.exit(main())
