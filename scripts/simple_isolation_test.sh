#!/bin/bash

# Simplest Isolation Test - Uses CLI defaults
set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

CLI_CMD="./bin/cli"

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[PASS]${NC} $1"; }
log_error() { echo -e "${RED}[FAIL]${NC} $1"; }
log_test() { echo -e "${YELLOW}[TEST]${NC} $1"; }

echo "=================================="
echo "   Simplest Isolation Tests"
echo "=================================="
echo "Using CLI defaults (should connect to 192.168.1.161:50051)"
echo

# Test 1: Basic job with process listing
log_test "Test 1: Basic process isolation"
log_info "Running 'ps aux' to see what processes are visible..."

output=$($CLI_CMD run ps aux 2>&1)
echo "Job creation output:"
echo "$output"
echo

job_id=$(echo "$output" | grep "ID:" | awk '{print $2}')
if [ -z "$job_id" ]; then
    log_error "Could not extract job ID"
    exit 1
fi

log_info "Job ID: $job_id"
log_info "Waiting for job to complete and getting logs..."
sleep 3

echo "=== JOB LOGS ==="
$CLI_CMD log $job_id || true
echo "=== END LOGS ==="
echo

# Test 2: PID isolation
log_test "Test 2: PID namespace isolation"
log_info "Testing what PID the job sees itself as..."

output2=$($CLI_CMD run sh -c "echo 'Process ID: $$'; ps aux" 2>&1)
job_id2=$(echo "$output2" | grep "ID:" | awk '{print $2}')

if [ -z "$job_id2" ]; then
    log_error "Could not extract job ID for PID test"
    exit 1
fi

log_info "PID test job ID: $job_id2"
sleep 3

echo "=== PID TEST LOGS ==="
$CLI_CMD log $job_id2 || true
echo "=== END PID TEST ==="
echo

# Test 3: Concurrent jobs isolation
log_test "Test 3: Concurrent jobs isolation"
log_info "Starting a long-running job..."

long_output=$($CLI_CMD run sh -c "echo 'Long job started'; sleep 15; echo 'Long job done'" 2>&1)
long_job_id=$(echo "$long_output" | grep "ID:" | awk '{print $2}')

if [ -z "$long_job_id" ]; then
    log_error "Could not start long job"
    exit 1
fi

log_info "Long job ID: $long_job_id (running in background)"
sleep 2

log_info "Starting a second job to list processes while first job runs..."
ps_output=$($CLI_CMD run sh -c "echo 'Second job - checking processes:'; ps aux" 2>&1)
ps_job_id=$(echo "$ps_output" | grep "ID:" | awk '{print $2}')

if [ -z "$ps_job_id" ]; then
    log_error "Could not start ps job"
    $CLI_CMD stop $long_job_id 2>/dev/null || true
    exit 1
fi

log_info "Process listing job ID: $ps_job_id"
sleep 3

echo "=== PROCESS ISOLATION TEST LOGS ==="
$CLI_CMD log $ps_job_id || true
echo "=== END ISOLATION TEST ==="

# Clean up
log_info "Stopping long-running job..."
$CLI_CMD stop $long_job_id 2>/dev/null || true

echo
log_info "Current jobs:"
$CLI_CMD list

echo
echo "=================================="
echo "Analysis of Results:"
echo "=================================="
echo "✅ If jobs show 'PID 1' - PID namespace isolation is working"
echo "✅ If jobs only see 1-3 processes - Process isolation is working"
echo "✅ If concurrent jobs don't see each other - Isolation is working"
echo "✅ If you see job-init or similar processes - Container setup is working"
echo "=================================="