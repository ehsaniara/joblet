#!/usr/bin/env python3
"""
Parallel processing script - Batch 1
"""
import json
import time
import os

def main():
    print("Starting batch 1 processing...")
    
    # Simulate processing time
    time.sleep(2)
    
    # Process batch 1 data
    batch_data = {
        "batch_id": 1,
        "items": [
            {"id": 1, "value": 10, "processed": True},
            {"id": 2, "value": 20, "processed": True},
            {"id": 3, "value": 30, "processed": True}
        ],
        "processing_time": 2.0,
        "status": "completed"
    }
    
    # Write results
    os.makedirs("results", exist_ok=True)
    with open("results/batch1_results.json", "w") as f:
        json.dump(batch_data, f, indent=2)
    
    print(f"Processed {len(batch_data['items'])} items in batch 1")
    print("Batch 1 processing completed successfully!")

if __name__ == "__main__":
    main()