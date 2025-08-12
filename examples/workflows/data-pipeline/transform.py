#!/usr/bin/env python3
"""
Data transformation script - demonstrates Joblet workflow processing
"""
import json
import sys
import os

def main():
    print("Starting data transformation...")
    
    # Read validated data from shared volume
    volume_path = "/volumes/data-pipeline"
    validated_data_file = f"{volume_path}/validated_data.json"
    if not os.path.exists(validated_data_file):
        print(f"Error: validated_data.json not found at {validated_data_file}!")
        sys.exit(1)
    
    with open(validated_data_file, "r") as f:
        data = json.load(f)
    
    records = data["records"]
    transformed_records = []
    
    # Transform data - add grade and normalize scores
    for record in records:
        score = record["score"]
        
        # Add letter grade
        if score >= 90:
            grade = "A"
        elif score >= 80:
            grade = "B"
        elif score >= 70:
            grade = "C"
        else:
            grade = "D"
        
        transformed_record = {
            "id": record["id"],
            "name": record["name"],
            "score": score,
            "grade": grade,
            "normalized_score": round(score / 100.0, 2)
        }
        
        transformed_records.append(transformed_record)
    
    # Write transformed data
    transformed_data = {
        "records": transformed_records,
        "transform_timestamp": "2025-08-11T17:47:00Z",
        "total_records": len(transformed_records),
        "transformations_applied": ["grade_assignment", "score_normalization"]
    }
    
    with open(f"{volume_path}/transformed_data.json", "w") as f:
        json.dump(transformed_data, f, indent=2)
    
    print(f"Transformed {len(transformed_records)} records")
    print("Data transformation completed successfully!")

if __name__ == "__main__":
    main()