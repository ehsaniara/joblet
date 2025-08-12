#!/usr/bin/env python3
"""
Report generation script - demonstrates Joblet workflow final output
"""
import json
import sys
import os

def main():
    print("Starting report generation...")
    
    # Read warehouse data from shared volume
    volume_path = "/volumes/data-pipeline"
    warehouse_data_file = f"{volume_path}/warehouse_data.json"
    if not os.path.exists(warehouse_data_file):
        print(f"Error: warehouse_data.json not found at {warehouse_data_file}!")
        sys.exit(1)
    
    with open(warehouse_data_file, "r") as f:
        data = json.load(f)
    
    records = data["records"]
    
    # Generate summary statistics
    total_records = len(records)
    total_score = sum(r["score"] for r in records)
    avg_score = total_score / total_records if total_records > 0 else 0
    
    grade_counts = {}
    for record in records:
        grade = record["grade"]
        grade_counts[grade] = grade_counts.get(grade, 0) + 1
    
    # Create report
    report = f"""
# Data Processing Pipeline Report

## Summary
- **Total Records Processed**: {total_records}
- **Average Score**: {avg_score:.2f}
- **Processing Status**: {data["load_status"]}
- **Processing Time**: {data["load_timestamp"]}

## Grade Distribution
"""
    
    for grade, count in sorted(grade_counts.items()):
        percentage = (count / total_records) * 100
        report += f"- **Grade {grade}**: {count} students ({percentage:.1f}%)\n"
    
    report += f"""
## Detailed Records
| ID | Name | Score | Grade | Normalized |
|----|------|-------|-------|------------|
"""
    
    for record in records:
        report += f"| {record['id']} | {record['name']} | {record['score']} | {record['grade']} | {record['normalized_score']} |\n"
    
    # Write report
    with open(f"{volume_path}/pipeline_report.md", "w") as f:
        f.write(report)
    
    print(f"Generated report for {total_records} records")
    print(f"Average score: {avg_score:.2f}")
    print(f"Report saved to: {volume_path}/pipeline_report.md")
    print("Report generation completed successfully!")

if __name__ == "__main__":
    main()