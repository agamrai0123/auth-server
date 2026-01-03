# Load Testing Guide

## Overview

The auth server includes a comprehensive load testing tool (`load-test.go`) that tests all endpoints under concurrent load and provides detailed performance metrics.

## Features

- **Multi-endpoint testing**: Tests /token, /validate, and /revoke endpoints
- **Concurrent workers**: Configurable number of parallel requests
- **Detailed metrics**: Latency, throughput, success rate per endpoint
- **Easy configuration**: Command-line flags for all parameters

## Building the Load Test Tool

```bash
cd d:\work-projects\auth-server
go build -o load-test load-test.go
```

## Running Load Tests

### Basic Usage (Default Configuration)

```bash
./load-test
```

Default settings:
- Base URL: http://localhost:8080
- Concurrency: 10 workers
- Requests per worker: 100
- Total requests: 1,000
- Client ID: test-client-1
- Client Secret: secret-key-12345

### Advanced Usage (Custom Configuration)

```bash
# Light load test (100 requests)
./load-test -concurrency=5 -requests=20

# Heavy load test (10,000 requests)
./load-test -concurrency=50 -requests=200

# Custom server URL
./load-test -url=http://192.168.1.100:8080

# Custom credentials
./load-test -client-id=mobile-app -client-secret=mobile-secret-key

# Combination
./load-test \
  -url=http://localhost:8080 \
  -concurrency=25 \
  -requests=100 \
  -client-id=test-client-2 \
  -client-secret=secret-key-67890
```

## Command Line Flags

| Flag | Description | Default |
|------|-------------|---------|
| `-url` | Base URL of auth server | http://localhost:8080 |
| `-concurrency` | Number of concurrent workers | 10 |
| `-requests` | Requests per worker | 100 |
| `-client-id` | Client ID for authentication | test-client-1 |
| `-client-secret` | Client secret for authentication | secret-key-12345 |

## Understanding the Results

### Overall Statistics

```
Total Requests:    1000
Successful:        1000 (100.00%)
Failed:            0 (0.00%)
Total Duration:    12.5s
Requests/sec:      80.00
```

- **Total Requests**: Total number of HTTP requests made
- **Successful**: Requests with HTTP 200 response
- **Failed**: Requests with non-200 response or errors
- **Total Duration**: Wall-clock time for the entire test
- **Requests/sec**: Throughput metric (important for comparing configurations)

### Latency Statistics

```
Min:               5.2ms
Max:               250.1ms
Avg:               45.3ms
```

- **Min**: Fastest single request
- **Max**: Slowest single request
- **Avg**: Average latency across all requests

### Per-Endpoint Statistics

```
Endpoint             Requests   Success   Failed   Avg Latency     Success Rate
POST /token          333        333       0        48.2ms          100.00%
POST /validate       333        333       0        42.1ms          100.00%
POST /revoke         334        334       0        45.8ms          100.00%
```

Each endpoint shows:
- **Requests**: Total requests to this endpoint
- **Success**: Successful requests (HTTP 200)
- **Failed**: Failed requests
- **Avg Latency**: Average response time
- **Success Rate**: Percentage of successful requests

## Test Scenarios

### Scenario 1: Light Load (Baseline)
```bash
./load-test -concurrency=5 -requests=20
# Total: 100 requests
```
Use for basic functionality validation and baseline performance.

### Scenario 2: Normal Load
```bash
./load-test -concurrency=10 -requests=100
# Total: 1,000 requests
```
Good for daily testing and typical usage patterns.

### Scenario 3: Moderate Load
```bash
./load-test -concurrency=25 -requests=100
# Total: 2,500 requests
```
Test system behavior under moderate concurrent usage.

### Scenario 4: Heavy Load
```bash
./load-test -concurrency=50 -requests=200
# Total: 10,000 requests
```
Stress test the system and identify bottlenecks.

### Scenario 5: Sustained Load
```bash
# Run in a loop to test sustained performance
for i in {1..5}; do
  echo "Test run $i"
  ./load-test -concurrency=20 -requests=200
  sleep 5
done
```

## Pre-Test Checklist

Before running load tests:

1. **Verify Oracle Database is Running**
   ```bash
   docker-compose ps
   # Should show "oracle-auth-db" with status "Up (healthy)"
   ```

2. **Start Auth Server**
   ```bash
   go run main.go
   # Or run the compiled binary
   ```

3. **Verify Connectivity**
   ```bash
   curl -X POST http://localhost:8080/token \
     -H "Content-Type: application/json" \
     -d '{
       "grant_type": "client_credentials",
       "client_id": "test-client-1",
       "client_secret": "secret-key-12345"
     }'
   # Should return a token
   ```

4. **Check Database Tables**
   ```bash
   docker exec -it oracle-auth-db sqlplus sys/Oracle123!@localhost:1521/XE as sysdba << EOF
   SELECT COUNT(*) FROM clients;
   SELECT COUNT(*) FROM tokens;
   EXIT;
   EOF
   ```

## Performance Benchmarks

### Recommended Performance Targets

| Metric | Target | Good | Excellent |
|--------|--------|------|-----------|
| Requests/sec | >50 | >100 | >200 |
| Avg Latency | <100ms | <50ms | <20ms |
| Success Rate | 100% | 100% | 100% |
| Max Latency | <500ms | <300ms | <100ms |

### Expected Results with Oracle + Docker

With typical hardware:
- **Throughput**: 80-150 req/sec
- **Avg Latency**: 40-80ms
- **Success Rate**: 100%

### Results with In-Memory Cache

The auth server includes in-memory caching which significantly improves performance:
- **Cache Hit**: ~21ns per lookup
- **Cache Miss**: ~17ns per lookup
- **Overall Throughput**: 100-200+ req/sec

## Monitoring During Load Test

### Open New Terminal Windows

**Terminal 1: Monitor Database**
```bash
watch -n 1 'docker exec -it oracle-auth-db sqlplus sys/Oracle123!@localhost:1521/XE as sysdba << EOF
SELECT COUNT(*) as token_count FROM tokens;
SELECT COUNT(*) as revoked_count FROM revoked_tokens;
EXIT;
EOF'
```

**Terminal 2: Monitor System Resources**
```bash
# Linux/Mac
watch -n 1 'docker stats oracle-auth-db'

# Windows PowerShell
while ($true) {
  docker stats oracle-auth-db --no-stream
  Start-Sleep -Seconds 1
}
```

**Terminal 3: Monitor Auth Server Logs**
```bash
# If running with: go run main.go
# Logs will show in the same terminal
```

## Analyzing Results

### Looking for Bottlenecks

1. **High Latency on /token endpoint**
   - Usually indicates cache miss or slow JWT generation
   - Check database response times

2. **Failed /validate requests**
   - May indicate token validation issues
   - Check JWT signature and cache hit rate

3. **Failed /revoke requests**
   - Suggests database transaction issues
   - Check database logs

### Comparing Test Runs

Save results for comparison:

```bash
# Run and save output
./load-test -concurrency=10 -requests=100 | tee results-10-100.txt

# Compare multiple runs
diff results-10-100.txt results-20-100.txt
```

## Troubleshooting

### "Connection refused" Error
```
Error: connection refused
```
**Solution**: Ensure auth server is running on localhost:8080
```bash
go run main.go
```

### "Failed to generate token" Error
```
Error: failed to generate token: invalid client credentials
```
**Solution**: Check client credentials match database
```bash
./load-test -client-id=test-client-1 -client-secret=secret-key-12345
```

### "Database connection error"
```
Error: failed to connect to database
```
**Solution**: Verify Oracle database is running
```bash
docker-compose ps
docker-compose logs oracle-db
```

### Timeout Errors During Heavy Load
```
Error: context deadline exceeded
```
**Solution**: Reduce concurrency or increase server resources
```bash
./load-test -concurrency=20 -requests=200  # Reduced from 50
```

## Advanced Testing

### Creating Custom Load Patterns

Edit `load-test.go` to implement custom patterns:

```go
// Example: Test only /token endpoint
func (lt *LoadTester) CustomTestPattern() {
    for i := 0; i < 1000; i++ {
        lt.TestTokenEndpoint()
    }
}
```

### Load Test with Different Client Credentials

```bash
# Test with each client
for client in "test-client-1:secret-key-12345" \
              "test-client-2:secret-key-67890" \
              "mobile-app:mobile-secret-key"; do
  IFS=':' read -r id secret <<< "$client"
  echo "Testing with $id"
  ./load-test -client-id="$id" -client-secret="$secret" -concurrency=5 -requests=50
done
```

### Continuous Load Testing

```bash
# Run tests continuously (useful for stability testing)
while true; do
  echo "Test run at $(date)"
  ./load-test -concurrency=10 -requests=100
  sleep 60
done
```

## Performance Optimization Tips

1. **Increase Cache TTL** in cache.go for better hit rates
2. **Batch size** tuning for token batch writer
3. **Connection pool** settings in database.go
4. **Network latency** - use localhost testing for best results
5. **CPU/Memory** - ensure sufficient resources allocated to Docker

## Next Steps

1. Run baseline load test with default settings
2. Identify bottlenecks from results
3. Tune configuration parameters
4. Compare results with benchmarks
5. Document your findings

