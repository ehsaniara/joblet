#!/bin/bash
cd "$(dirname "$0")"

# Source the test
source ./tests/11_metrics_gap_test.sh

# Don't run main, just test the function
exit 0
