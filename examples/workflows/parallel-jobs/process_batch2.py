#!/usr/bin/env python3
"""
Parallel processing script - Batch 2
"""
import json
import time
import os

def main():
    print("Starting batch 2 processing...")
    
    # Simulate processing time
    time.sleep(1.5)
    
    # Process batch 2 data
    batch_data = {
        "batch_id": 2,
        "items": [
            {"id": 4, "value": 40, "processed": True},
            {"id": 5, "value": 50, "processed": True}
        ],
        "processing_time": 1.5,
        "status": "completed"
    }
    
    # Write results
    os.makedirs("results", exist_ok=True)
    with open("results/batch2_results.json", "w") as f:
        json.dump(batch_data, f, indent=2)
    
    print(f"Processed {len(batch_data['items'])} items in batch 2")
    print("Batch 2 processing completed successfully!")

if __name__ == "__main__":
    main()