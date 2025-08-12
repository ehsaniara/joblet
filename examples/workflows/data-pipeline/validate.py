#!/usr/bin/env python3
"""
Data validation script - demonstrates Joblet workflow with shared volumes
"""
import json
import sys
import os

def main():
    print("Starting data validation...")
    
    # Read from shared volume
    volume_path = "/volumes/data-pipeline"
    raw_data_file = f"{volume_path}/raw_data.json"
    if not os.path.exists(raw_data_file):
        print(f"Error: raw_data.json not found at {raw_data_file}!")
        sys.exit(1)
    
    with open(raw_data_file, "r") as f:
        data = json.load(f)
    
    # Validate data structure
    if "records" not in data:
        print("Error: No records found in data!")
        sys.exit(1)
    
    records = data["records"]
    validated_records = []
    
    for record in records:
        if all(key in record for key in ["id", "name", "score"]):
            if 0 <= record["score"] <= 100:
                validated_records.append(record)
            else:
                print(f"Warning: Invalid score for {record['name']}: {record['score']}")
        else:
            print(f"Warning: Missing fields in record: {record}")
    
    # Write validated data
    validated_data = {
        "records": validated_records,
        "validation_timestamp": "2025-08-11T17:46:00Z",
        "total_records": len(validated_records),
        "validation_passed": True
    }
    
    with open(f"{volume_path}/validated_data.json", "w") as f:
        json.dump(validated_data, f, indent=2)
    
    print(f"Validated {len(validated_records)} records")
    print("Data validation completed successfully!")

if __name__ == "__main__":
    main()