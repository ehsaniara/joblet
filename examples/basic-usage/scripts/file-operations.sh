#!/bin/bash
# File operations demo script

echo "Working with uploaded files:"
ls -la
cat sample_data.txt
echo "Creating output file..."
echo "Processed at $(date)" > output.txt
cat output.txt