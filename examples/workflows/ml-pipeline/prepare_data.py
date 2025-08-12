#!/usr/bin/env python3
"""
ML Data preparation script - demonstrates ML workflow
"""
import json
import os

def main():
    print("Starting ML data preparation...")
    
    # Create sample ML dataset
    dataset = {
        "features": [
            {"id": 1, "feature1": 0.8, "feature2": 0.3, "feature3": 0.9, "label": 1},
            {"id": 2, "feature1": 0.2, "feature2": 0.7, "feature3": 0.4, "label": 0},
            {"id": 3, "feature1": 0.9, "feature2": 0.8, "feature3": 0.6, "label": 1},
            {"id": 4, "feature1": 0.1, "feature2": 0.4, "feature3": 0.2, "label": 0},
            {"id": 5, "feature1": 0.7, "feature2": 0.9, "feature3": 0.8, "label": 1}
        ],
        "metadata": {
            "total_samples": 5,
            "features": ["feature1", "feature2", "feature3"],
            "labels": [0, 1],
            "preparation_timestamp": "2025-08-11T18:00:00Z"
        }
    }
    
    # Write prepared data
    os.makedirs("ml_data", exist_ok=True)
    with open("ml_data/prepared_dataset.json", "w") as f:
        json.dump(dataset, f, indent=2)
    
    print(f"Prepared {len(dataset['features'])} samples")
    print("Data preparation completed successfully!")

if __name__ == "__main__":
    main()