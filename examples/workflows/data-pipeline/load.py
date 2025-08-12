#!/usr/bin/env python3
"""
Data loading script - demonstrates final stage of Joblet workflow
"""
import json
import sys
import os

def main():
    print("Starting data loading...")
    
    # Read transformed data from shared volume
    volume_path = "/volumes/data-pipeline"
    transformed_data_file = f"{volume_path}/transformed_data.json"
    if not os.path.exists(transformed_data_file):
        print(f"Error: transformed_data.json not found at {transformed_data_file}!")
        sys.exit(1)
    
    with open(transformed_data_file, "r") as f:
        data = json.load(f)
    
    records = data["records"]
    
    # Simulate loading to warehouse (write final output)
    warehouse_data = {
        "table": "student_scores",
        "records": records,
        "load_timestamp": "2025-08-11T17:48:00Z",
        "loaded_count": len(records),
        "load_status": "success"
    }
    
    with open(f"{volume_path}/warehouse_data.json", "w") as f:
        json.dump(warehouse_data, f, indent=2)
    
    # Create load summary
    summary = {
        "pipeline": "student_data_processing",
        "status": "completed",
        "records_processed": len(records),
        "final_output": "data/warehouse_data.json"
    }
    
    with open(f"{volume_path}/load_summary.json", "w") as f:
        json.dump(summary, f, indent=2)
    
    print(f"Loaded {len(records)} records to warehouse")
    print("Data loading completed successfully!")

if __name__ == "__main__":
    main()