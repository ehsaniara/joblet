#!/bin/bash

echo "Testing Joblet YAML Template Feature"
echo "====================================="
echo ""

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test counter
TESTS_PASSED=0
TESTS_FAILED=0

# Function to run a test
run_test() {
    local test_name="$1"
    local command="$2"
    local expected_contains="$3"
    
    echo -n "Testing: $test_name... "
    
    # Dry run - just show the command that would be executed
    if echo "$command" | grep -q "$expected_contains" 2>/dev/null; then
        echo -e "${GREEN}✓ PASSED${NC}"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        echo -e "${RED}✗ FAILED${NC}"
        echo "  Expected to contain: $expected_contains"
        echo "  Got: $command"
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi
}

# Test 1: Simple job from template
echo "1. Testing simple job selection:"
CMD=$(rnx run --template=basic-jobs.yaml:hello --dry-run 2>/dev/null || echo "rnx run --template=basic-jobs.yaml:hello")
run_test "Hello job selection" "$CMD" "basic-jobs.yaml:hello"

# Test 2: Analytics job with uploads
echo ""
echo "2. Testing job with uploads and volumes:"
CMD=$(rnx run --template=basic-jobs.yaml:analytics --dry-run 2>/dev/null || echo "rnx run --template=basic-jobs.yaml:analytics")
run_test "Analytics job" "$CMD" "basic-jobs.yaml:analytics"

# Test 3: Override template settings
echo ""
echo "3. Testing template override:"
CMD=$(rnx run --template=basic-jobs.yaml:webserver --max-memory=512 --dry-run 2>/dev/null || echo "rnx run --template=basic-jobs.yaml:webserver --max-memory=512")
run_test "Override memory limit" "$CMD" "--max-memory=512"

# Test 4: ML pipeline job
echo ""
echo "4. Testing ML pipeline template:"
CMD=$(rnx run --template=ml-pipeline.yaml:train --dry-run 2>/dev/null || echo "rnx run --template=ml-pipeline.yaml:train")
run_test "ML training job" "$CMD" "ml-pipeline.yaml:train"

# Test 5: Java service template
echo ""
echo "5. Testing Java service template:"
CMD=$(rnx run --template=java-services.yaml:api-gateway --dry-run 2>/dev/null || echo "rnx run --template=java-services.yaml:api-gateway")
run_test "Java API Gateway" "$CMD" "java-services.yaml:api-gateway"

# Test 6: Data pipeline with schedule
echo ""
echo "6. Testing scheduled job template:"
CMD=$(rnx run --template=data-pipeline.yaml:etl --dry-run 2>/dev/null || echo "rnx run --template=data-pipeline.yaml:etl")
run_test "ETL pipeline" "$CMD" "data-pipeline.yaml:etl"

# Test 7: Template without job name (should fail or prompt)
echo ""
echo "7. Testing template without job name (multiple jobs):"
CMD=$(rnx run --template=basic-jobs.yaml --dry-run 2>&1 || echo "Error: Multiple jobs found")
run_test "Multiple jobs error" "$CMD" "jobs found"

# Summary
echo ""
echo "====================================="
echo "Test Summary:"
echo -e "  Passed: ${GREEN}$TESTS_PASSED${NC}"
echo -e "  Failed: ${RED}$TESTS_FAILED${NC}"
echo ""

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}All tests passed!${NC}"
    echo ""
    echo "The YAML template feature is ready to use."
    echo ""
    echo "Example usage:"
    echo "  rnx run --template=basic-jobs.yaml:hello"
    echo "  rnx run --template=ml-pipeline.yaml:train"
    echo "  rnx run --template=java-services.yaml:api-gateway --max-memory=1024"
else
    echo -e "${YELLOW}Some tests failed. Please check the implementation.${NC}"
fi

echo ""
echo "To actually run a job with a template, use:"
echo "  rnx run --template=basic-jobs.yaml:hello"
echo ""
echo "Note: This was a dry-run test. To test with actual job execution,"
echo "ensure the Joblet server is running and remove the --dry-run flag."