#!/usr/bin/env python3
"""
Data analysis script for multi-job workflow test
"""
import json
import os
import sys


def main():
    print("=== Data Analyzer Job ===")

    # Check if input file exists
    if not os.path.exists("processed_data.json"):
        print("✗ Input file not found: processed_data.json")
        return 1

    # Read processed data
    with open("processed_data.json", "r") as f:
        data = json.load(f)

    print(f"✓ Loaded data with {data['statistics']['count']} values")

    # Perform analysis
    stats = data["statistics"]
    analysis = {
        "data_source": data.get("timestamp"),
        "runtime_used": data.get("runtime"),
        "analysis": {
            "range": stats["max"] - stats["min"],
            "variance": sum((x - stats["mean"]) ** 2 for x in data["values"]) / len(data["values"]),
            "above_mean": len([x for x in data["values"] if x > stats["mean"]]),
            "below_mean": len([x for x in data["values"] if x < stats["mean"]])
        }
    }

    # Write analysis results
    with open("analysis_results.json", "w") as f:
        json.dump(analysis, f, indent=2)

    print(f"✓ Analysis completed:")
    print(f"  - Range: {analysis['analysis']['range']}")
    print(f"  - Variance: {analysis['analysis']['variance']:.2f}")
    print(f"  - Values above mean: {analysis['analysis']['above_mean']}")
    print(f"  - Values below mean: {analysis['analysis']['below_mean']}")
    print("✓ Results written to analysis_results.json")

    return 0


if __name__ == "__main__":
    sys.exit(main())
