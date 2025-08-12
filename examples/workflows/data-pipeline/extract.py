#!/usr/bin/env python3
"""
Data extraction script - demonstrates Joblet workflow with file uploads
"""
import json
import os
import sys

def main():
    print("Starting data extraction...")
    
    # Create sample data
    data = {
        "records": [
            {"id": 1, "name": "Alice", "score": 95},
            {"id": 2, "name": "Bob", "score": 87},
            {"id": 3, "name": "Charlie", "score": 92}
        ],
        "timestamp": "2025-08-11T17:45:00Z",
        "source": "joblet-demo"
    }
    
    # Write to shared volume
    volume_path = "/volumes/data-pipeline"
    os.makedirs(volume_path, exist_ok=True)
    with open(f"{volume_path}/raw_data.json", "w") as f:
        json.dump(data, f, indent=2)
    
    print(f"Extracted {len(data['records'])} records to {volume_path}/raw_data.json")
    print("Data extraction completed successfully!")

if __name__ == "__main__":
    main()