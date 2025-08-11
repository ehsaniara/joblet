#!/bin/bash
# Complete analytics workflow script

echo "=== Running Complete Analytics Workflow ==="

echo "Step 1: Sales Analysis..."
python3 scripts/analyze_sales.py
if [ $? -eq 0 ]; then
    echo "✅ Sales analysis completed"
else
    echo "❌ Sales analysis failed"
fi

echo ""
echo "Step 2: Customer Segmentation..."
python3 scripts/segment_customers.py
if [ $? -eq 0 ]; then
    echo "✅ Customer segmentation completed"
else
    echo "❌ Customer segmentation failed"
fi

echo ""
echo "Step 3: Time Series Analysis..."
python3 scripts/time_series.py
if [ $? -eq 0 ]; then
    echo "✅ Time series analysis completed"
else
    echo "❌ Time series analysis failed"
fi

echo ""
echo "Step 4: Combining Reports..."
python3 scripts/combine_reports.py
if [ $? -eq 0 ]; then
    echo "✅ Report combination completed"
else
    echo "❌ Report combination failed"
fi

echo ""
echo "=== Analytics Workflow Complete ==="
echo "Check results in /volumes/analytics-data/"
echo "Combined report: /volumes/analytics-data/combined_analytics_report.json"