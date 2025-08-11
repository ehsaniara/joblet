#!/bin/bash
# Volume storage demo script

echo "Writing to persistent volume..."
date > /volumes/demo-data/timestamp.txt
echo "Data saved: $(cat /volumes/demo-data/timestamp.txt)"
echo "Listing volume contents:"
ls -la /volumes/demo-data/