#!/bin/bash
# Long running job for monitoring demonstration

echo "Starting long-running job..."
for i in {1..30}; do
  echo "Progress: $i/30"
  sleep 2
done
echo "Job completed!"