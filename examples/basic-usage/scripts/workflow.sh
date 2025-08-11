#!/bin/bash
# Multi-step workflow demonstration

echo "=== Step 1: Data Generation ==="
for i in {1..10}; do
  echo "Data point $i: $RANDOM" >> /tmp/data.txt
done

echo "=== Step 2: Data Processing ==="
sort -n /tmp/data.txt > /tmp/sorted.txt

echo "=== Step 3: Analysis ==="
echo "Total lines: $(wc -l < /tmp/sorted.txt)"
echo "First value: $(head -1 /tmp/sorted.txt)"
echo "Last value: $(tail -1 /tmp/sorted.txt)"

echo "=== Step 4: Save Results ==="
if [ -d "/volumes/demo-data" ]; then
  cp /tmp/sorted.txt /volumes/demo-data/results_$(date +%s).txt
  echo "Results saved to volume"
else
  echo "No volume mounted, results in temp only"
fi