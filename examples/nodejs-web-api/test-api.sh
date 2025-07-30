#!/bin/bash

# Test script for the Node.js API
# This script demonstrates various API endpoints

set -e

API_URL="http://api:3000"

echo "🧪 Testing Node.js API Endpoints"
echo "================================"

# Wait for API to be ready
echo ""
echo "⏳ Waiting for API to be ready..."
for i in {1..10}; do
    if curl -s "$API_URL/health" > /dev/null 2>&1; then
        echo "✅ API is ready!"
        break
    fi
    echo "   Attempt $i/10..."
    sleep 2
done

# Health check
echo ""
echo "1. Health Check"
echo "---------------"
curl -s "$API_URL/health" | python3 -m json.tool || curl -s "$API_URL/health"

# Create some tasks
echo ""
echo ""
echo "2. Creating Tasks"
echo "-----------------"

echo "Creating data processing task..."
curl -s -X POST "$API_URL/api/tasks" \
    -H "Content-Type: application/json" \
    -d '{"title": "Process sales data", "type": "data-processing"}' \
    | python3 -m json.tool || echo "Task created"

echo ""
echo "Creating email notification task..."
curl -s -X POST "$API_URL/api/tasks" \
    -H "Content-Type: application/json" \
    -d '{"title": "Send weekly newsletter", "type": "email-notification"}' \
    | python3 -m json.tool || echo "Task created"

echo ""
echo "Creating report generation task..."
curl -s -X POST "$API_URL/api/tasks" \
    -H "Content-Type: application/json" \
    -d '{"title": "Generate monthly report", "type": "report-generation"}' \
    | python3 -m json.tool || echo "Task created"

# List all tasks
echo ""
echo ""
echo "3. Listing All Tasks"
echo "--------------------"
curl -s "$API_URL/api/tasks" | python3 -m json.tool || curl -s "$API_URL/api/tasks"

# Get statistics
echo ""
echo ""
echo "4. API Statistics"
echo "-----------------"
curl -s "$API_URL/api/stats" | python3 -m json.tool || curl -s "$API_URL/api/stats"

# Update a task
echo ""
echo ""
echo "5. Updating Task Status"
echo "-----------------------"
echo "Marking task 1 as completed..."
curl -s -X PUT "$API_URL/api/tasks/1" \
    -H "Content-Type: application/json" \
    -d '{"status": "completed", "result": "Processed 1000 records"}' \
    | python3 -m json.tool || echo "Task updated"

# Get specific task
echo ""
echo ""
echo "6. Getting Specific Task"
echo "------------------------"
curl -s "$API_URL/api/tasks/1" | python3 -m json.tool || curl -s "$API_URL/api/tasks/1"

# List pending tasks
echo ""
echo ""
echo "7. Listing Pending Tasks"
echo "------------------------"
curl -s "$API_URL/api/tasks?status=pending" | python3 -m json.tool || curl -s "$API_URL/api/tasks?status=pending"

# Final statistics
echo ""
echo ""
echo "8. Final Statistics"
echo "-------------------"
curl -s "$API_URL/api/stats" | python3 -m json.tool || curl -s "$API_URL/api/stats"

echo ""
echo ""
echo "✅ API tests completed!"
echo ""
echo "💡 To test with a worker running:"
echo "   1. Start a worker in another terminal:"
echo "      rnx run --upload-dir=. --network=myapp node worker.js"
echo "   2. Watch tasks being processed automatically"
echo "   3. Check statistics to see processing metrics"