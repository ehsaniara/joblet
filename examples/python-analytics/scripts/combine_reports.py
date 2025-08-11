#!/usr/bin/env python3
"""
Combine analytics reports into a comprehensive report
"""

import json
import os
from datetime import datetime

def combine_reports():
    print("=== Generating Comprehensive Analytics Report ===")
    
    volume_path = "/volumes/analytics-data"
    report_files = [
        "sales_report.json",
        "customer_segments.json", 
        "time_series_analysis.json"
    ]
    
    combined_report = {
        "generated_at": datetime.now().isoformat(),
        "reports": {}
    }
    
    # Try volume path first, then local directory
    paths_to_try = [volume_path, "."]
    
    for base_path in paths_to_try:
        found_reports = 0
        for report_file in report_files:
            file_path = os.path.join(base_path, report_file)
            if os.path.exists(file_path):
                try:
                    with open(file_path, 'r') as f:
                        report_name = os.path.splitext(os.path.basename(report_file))[0]
                        combined_report["reports"][report_name] = json.load(f)
                        print(f"Added {report_name} to combined report")
                        found_reports += 1
                except Exception as e:
                    print(f"Error reading {report_file}: {e}")
        
        if found_reports > 0:
            break
    
    if not combined_report["reports"]:
        print("No report files found to combine")
        return
    
    # Save combined report
    try:
        output_path = os.path.join(volume_path, "combined_analytics_report.json")
        with open(output_path, 'w') as f:
            json.dump(combined_report, f, indent=2)
        print(f"Combined report saved to: {output_path}")
    except Exception as e:
        print(f"Could not save to volume, saving locally: {e}")
        with open("combined_analytics_report.json", 'w') as f:
            json.dump(combined_report, f, indent=2)
        print("Combined report saved to: combined_analytics_report.json")
    
    print(f"Total reports included: {len(combined_report['reports'])}")

if __name__ == "__main__":
    combine_reports()