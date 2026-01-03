#!/bin/bash
# 10 Million Request Load Test Script
# ====================================
# This script runs the complete 10M load test with proper orchestration

set -e

PROJECT_DIR="d:\work-projects\auth-server"
cd "$PROJECT_DIR"

echo "=========================================="
echo "10 MILLION REQUEST LOAD TEST"
echo "=========================================="
echo ""

# Configuration
CONCURRENCY=${1:-500}
REQUESTS_PER_WORKER=${2:-20000}
TOTAL_REQUESTS=$((CONCURRENCY * REQUESTS_PER_WORKER))
SERVER_URL="http://localhost:8080"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
RESULTS_FILE="LOAD_TEST_10M_RESULTS_${TIMESTAMP}.txt"

echo "Configuration:"
echo "  Concurrency: $CONCURRENCY workers"
echo "  Requests/worker: $REQUESTS_PER_WORKER"
echo "  Total requests: $TOTAL_REQUESTS"
echo "  Server URL: $SERVER_URL"
echo "  Results file: $RESULTS_FILE"
echo ""

# Check if load-test executable exists
if [ ! -f "./load-test" ]; then
    echo "ERROR: load-test executable not found"
    echo "Building load-test..."
    go build -o load-test load-test.go
fi

# Verify load-test is ready
echo "Verifying load-test executable..."
ls -lh load-test
echo ""

# Check if server is reachable
echo "Checking if server is reachable at $SERVER_URL..."
if curl -s "$SERVER_URL/health" > /dev/null 2>&1; then
    echo "✓ Server is reachable"
else
    echo "⚠ Server might not be running at $SERVER_URL"
    echo "  Make sure to run: go run main.go"
    echo "  Continuing anyway (will attempt test)..."
fi
echo ""

# Run the load test
echo "=========================================="
echo "STARTING 10M REQUEST LOAD TEST"
echo "=========================================="
echo "Start time: $(date)"
echo ""

START_TIME=$(date +%s)

# Run load test with all output captured
./load-test \
    -url="$SERVER_URL" \
    -concurrency=$CONCURRENCY \
    -requests=$REQUESTS_PER_WORKER | tee "$RESULTS_FILE"

END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))

# Summary
echo ""
echo "=========================================="
echo "LOAD TEST COMPLETED"
echo "=========================================="
echo "End time: $(date)"
echo "Total duration: ${DURATION}s ($(($DURATION / 60)) minutes)"
echo "Results saved to: $RESULTS_FILE"
echo ""

# Calculate throughput
if [ $DURATION -gt 0 ]; then
    THROUGHPUT=$((TOTAL_REQUESTS / DURATION))
    echo "Average throughput: $THROUGHPUT req/sec"
fi

echo ""
echo "Next steps:"
echo "1. Review results in: $RESULTS_FILE"
echo "2. Check success rate"
echo "3. Review latency statistics"
echo "4. Check per-endpoint performance"
echo ""
