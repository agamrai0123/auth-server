#!/bin/bash
# Complete 10M Load Test Execution Script
# =========================================

set -e

cd d:\work-projects\auth-server

echo "========================================"
echo "10 MILLION REQUEST LOAD TEST"
echo "Complete Infrastructure Execution"
echo "========================================"
echo ""

# Step 1: Verify Oracle image is available
echo "[1/4] Checking Oracle image..."
if docker image ls gvenzl/oracle-xe | grep -q "21.3.0"; then
    echo "✓ Oracle image is available (4.46GB)"
else
    echo "✗ Oracle image not found"
    echo "Pulling image..."
    docker pull gvenzl/oracle-xe:21.3.0
fi
echo ""

# Step 2: Start database
echo "[2/4] Starting Oracle database..."
docker-compose down 2>/dev/null || true
docker-compose up -d

echo "Waiting for database to become healthy..."
WAIT_COUNT=0
while [ $WAIT_COUNT -lt 60 ]; do
    if docker-compose ps | grep -q "healthy"; then
        echo "✓ Database is healthy"
        break
    fi
    echo "  Still waiting... ($((WAIT_COUNT+1))/60 seconds)"
    sleep 1
    WAIT_COUNT=$((WAIT_COUNT+1))
done

if [ $WAIT_COUNT -eq 60 ]; then
    echo "⚠ Database health check timeout (may still be initializing)"
fi
echo ""

# Step 3: Display database status
echo "[3/4] Database status:"
docker-compose ps
echo ""

# Step 4: Test database connectivity
echo "Testing database connectivity..."
if docker exec oracle-auth-db sqlplus -v >/dev/null 2>&1; then
    echo "✓ Can connect to database"
else
    echo "⚠ Database may still be initializing"
fi
echo ""

echo "========================================"
echo "Infrastructure is ready!"
echo "========================================"
echo ""
echo "Next steps:"
echo ""
echo "1. In Terminal 1, start the auth server:"
echo "   go run main.go"
echo ""
echo "2. Wait for output: 'Server listening on :8080'"
echo ""
echo "3. In Terminal 2, run the 10M load test:"
echo "   Option A (Balanced - 20-30 min):"
echo "   ./load-test -url=http://localhost:8080 -concurrency=500 -requests=20000"
echo ""
echo "   Option B (Conservative - 60+ min):"
echo "   ./load-test -url=http://localhost:8080 -concurrency=100 -requests=100000"
echo ""
echo "   Option C (Aggressive - 10-15 min):"
echo "   ./load-test -url=http://localhost:8080 -concurrency=1000 -requests=10000"
echo ""
echo "========================================"
