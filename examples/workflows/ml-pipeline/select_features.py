#!/usr/bin/env python3
"""
ML Feature selection script
"""
import json
import sys
import os

def main():
    print("Starting feature selection...")
    
    # Read prepared data
    if not os.path.exists("/volumes/ml-pipeline/prepared_dataset.json"):
        print("Error: prepared_dataset.json not found!")
        sys.exit(1)
    
    with open("/volumes/ml-pipeline/prepared_dataset.json", "r") as f:
        dataset = json.load(f)
    
    # Simple feature selection - select best features based on variance
    features = dataset["features"]
    selected_features = []
    
    for sample in features:
        # Select only feature1 and feature3 (simulate feature selection)
        selected_sample = {
            "id": sample["id"],
            "feature1": sample["feature1"],
            "feature3": sample["feature3"],
            "label": sample["label"]
        }
        selected_features.append(selected_sample)
    
    # Create selected dataset
    selected_dataset = {
        "features": selected_features,
        "metadata": {
            "total_samples": len(selected_features),
            "selected_features": ["feature1", "feature3"],
            "labels": [0, 1],
            "selection_timestamp": "2025-08-11T18:01:00Z",
            "selection_method": "variance_threshold"
        }
    }
    
    with open("/volumes/ml-pipeline/selected_features.json", "w") as f:
        json.dump(selected_dataset, f, indent=2)
    
    print(f"Selected 2 features from {len(dataset['metadata']['features'])} original features")
    print("Feature selection completed successfully!")

if __name__ == "__main__":
    main()