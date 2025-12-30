# Logging & Error Handling Reference

## Quick Log Reference

### Token Generation (/token endpoint)

| Scenario | Log Level | Message | Key Fields |
|----------|-----------|---------|------------|
| Invalid JSON in request | WARN | Failed to decode token request JSON | method, error |
| Invalid HTTP method | WARN | Invalid HTTP method for token endpoint | method |
| Client lookup fails | WARN | Client lookup failed | client_id, error |
| Invalid credentials | WARN | Invalid client credentials | client_id |
| Scope fetch fails | ERROR | Failed to fetch client scopes | client_id, error |
| JWT signing fails | ERROR | Failed to sign JWT token | client_id, error |
| Token storage fails | ERROR | Failed to store token in database | client_id, token_id, error |
| Success | INFO | JWT token generated successfully | client_id, token_id |
| Request processing | DEBUG | Processing token request | client_id, grant_type |
| Client validated | DEBUG | Client credentials validated | client_id |
| Scopes fetched | DEBUG | Client scopes fetched | client_id, scopes |

### Token Validation (/validate endpoint)

| Scenario | Log Level | Message | Key Fields |
|----------|-----------|---------|------------|
| Missing X-Forwarded-For | WARN | Missing X-Forwarded-For header | - |
| Missing Authorization | WARN | Missing Authorization header | resource |
| Invalid Bearer format | WARN | Invalid Bearer token format | resource |
| JWT parse fails | WARN | JWT token parsing failed | resource, error |
| Token is revoked | WARN | Token has been revoked | client_id, token_id |
| Resource not in scope | WARN | Resource not in token scopes | client_id, resource, allowed_scopes |
| Success | INFO | Token validated for resource - access granted | client_id, resource, expires_at |
| Processing request | DEBUG | Processing validate request | - |
| Resource found | DEBUG | Validating access to resource | resource |
| JWT valid | DEBUG | JWT token signature valid | client_id, token_id |
| Token not revoked | DEBUG | Token is valid and not revoked | client_id, token_id |

### Token Revocation (/revoke endpoint)

| Scenario | Log Level | Message | Key Fields |
|----------|-----------|---------|------------|
| Invalid HTTP method | WARN | Invalid HTTP method for revoke endpoint | method |
| Missing Authorization | WARN | Missing Authorization header | - |
| Invalid Bearer format | WARN | Invalid Bearer token format | - |
| JWT validation fails | WARN | JWT token validation failed during revocation | error |
| Database error | ERROR | Failed to revoke token | client_id, token_id, error |
| Success | INFO | Token revoked successfully | client_id, token_id |
| Processing revocation | DEBUG | Revoking token | client_id, token_id |

### Database Operations

| Operation | Log Level | Message | Key Fields |
|-----------|-----------|---------|------------|
| Connection | INFO | Database connected successfully | - |
| Connection fails | ERROR | Failed to open database connection | error |
| Ping fails | ERROR | Database ping failed | error |
| Client lookup | DEBUG | Looking up client in database | client_id |
| Client not found | WARN | Client not found in database | client_id |
| Scope fetch | DEBUG | Fetching client scopes from database | client_id |
| Scope parse error | ERROR | Failed to unmarshal allowed_scopes JSON | client_id, error |
| Token insert | DEBUG | Token created and storing in database | client_id, token_id |
| Insert success | INFO | Token inserted successfully | - |
| Insert failure | ERROR | Failed to store token in database | client_id, token_id, error |
| Revoke check | DEBUG | Checking if token is revoked | token_id |

---

## Structured Log Fields Reference

### Standard Fields in Every Log
```json
{
  "level": "info|debug|warn|error",
  "time": "2025-12-30T20:48:14+05:30"
}
```

### Request-Scoped Fields
```json
{
  "request_id": "uuid-string",
  "method": "POST|GET",
  "path": "/token|/validate|/revoke",
  "client_ip": "127.0.0.1",
  "host": "hostname"
}
```

### Security/Auth Fields
```json
{
  "client_id": "service-name",
  "token_id": "a1b2c3d4...",
  "resource": "https://api/users",
  "allowed_scopes": ["https://api/users", "https://api/data"],
  "error": "error description"
}
```

### Temporal Fields
```json
{
  "expires_at": "2025-12-30T21:30:00Z",
  "issued_at": "2025-12-30T20:30:00Z",
  "duration_ms": 15.5
}
```

---

## Error Response Format

### Invalid Request (400)
```json
{
  "error": "invalid_request",
  "error_description": "Invalid JSON format"
}
```
Log: `Failed to decode token request JSON`

### Invalid Client (401)
```json
{
  "error": "invalid_client",
  "error_description": "Invalid client credentials"
}
```
Log: `Invalid client credentials`

### Missing Token (401)
```json
{
  "error": "missing_token",
  "error_description": "Authorization header required"
}
```
Log: `Missing Authorization header`

### Invalid Token (401)
```json
{
  "error": "invalid_token",
  "error_description": "Token validation failed"
}
```
Log: `JWT token validation failed`

### Unsupported Grant Type (400)
```json
{
  "error": "unsupported_grant_type",
  "error_description": "Grant type not supported"
}
```
Log: `Unsupported grant type`

### Server Error (500)
```json
{
  "error": "server_error",
  "error_description": "Internal server error"
}
```
Log: `Database connection failed` or similar

### Forbidden - Resource Not in Scope (403)
```json
{
  "valid": false
}
```
Log: `Resource not in token scopes - access denied`

---

## Monitoring Queries

### Find All Errors in Last Hour
```bash
grep '"level":"error"' auth.log | tail -100
```

### Find Failed Authentication Attempts
```bash
grep 'Invalid client credentials' auth.log | wc -l
```

### Find Unauthorized Resource Access Attempts
```bash
grep 'Resource not in token scopes' auth.log
```

### Find Database Errors
```bash
grep 'Database\|database' auth.log | grep '"level":"error"'
```

### List All Revoked Tokens
```bash
grep 'Token revoked successfully' auth.log | jq '.token_id'
```

### Get Average Token Generation Time
```bash
grep 'JWT token generated successfully' auth.log | jq '.duration_ms' | awk '{sum+=$1; count++} END {print sum/count}'
```

### Find Tokens Expiring Soon (< 30 seconds)
```bash
grep 'Token validated' auth.log | jq 'select(.expires_at < now + 30)'
```

---

## Configuration

### Log Level Settings

**Development (debug level - verbose)**
```bash
LOG_LEVEL=debug
```
Use this to see:
- All client lookups
- Scope fetching details
- JWT generation steps
- Request processing flow

**Production (info level - normal)**
```bash
LOG_LEVEL=info
```
Use this to see:
- Successful operations only
- Critical errors
- Authentication failures
- Token revocations

**Quiet (warn level)**
```bash
LOG_LEVEL=warn
```
Use this to see:
- Warnings and errors only
- Failed attempts
- Invalid requests

### Log Output

**Console Output**
```
Development: Color-coded, readable format
Production: JSON format for parsing/monitoring
```

**File Output**
```
Path: LOG_PATH (default: ./logs/auth.log)
Rotation: When file reaches 100MB
Backup: Keep last 3 files
Compression: gz compressed backups
```

### Example Production Config
```bash
export LOG_LEVEL=info
export LOG_PATH=/var/log/auth-server/auth.log
export ENVIRONMENT=production
```

---

## Best Practices

### 1. Monitor These Metrics
- ✓ Count of invalid credentials (potential brute force)
- ✓ Count of resource access denials (scope misconfigurations)
- ✓ Database error rate (infrastructure issues)
- ✓ Token generation latency (performance)
- ✓ Token validation latency (API gateway slowdowns)

### 2. Alert on These Conditions
- Error count > 10 per minute
- Database connection failures
- JWT signing failures
- Token storage failures
- Revoked token reuse attempts (security incident)

### 3. Audit Trail Compliance
- All authentication attempts are logged
- Include client_id, timestamp, and result
- Store logs for compliance period (e.g., 90 days)
- Protect logs with access controls
- Encrypt logs in transit and at rest

### 4. Privacy Considerations
- Never log full token values in logs
- Only log token_id (first 16 chars of random ID)
- Don't log client_secret
- Include request_id for tracing
- Allow users to track their token usage

---

## Troubleshooting via Logs

### Problem: Clients Getting 401 Errors

**Check logs for:**
```bash
grep '"level":"warn"' auth.log | grep -E 'Invalid|JWT|revoked'
```

**Common causes:**
1. "Invalid client credentials" → Client/secret mismatch
2. "JWT token parsing failed" → Signature issue or expiration
3. "Token has been revoked" → Token was revoked

### Problem: Clients Getting 403 Errors

**Check logs for:**
```bash
grep 'Resource not in token scopes' auth.log
```

**Common causes:**
1. Requested resource not in client's allowed_scopes
2. Resource URL format mismatch (https vs http, trailing slash)
3. Client configuration hasn't been updated

### Problem: High Latency

**Check logs for:**
```bash
grep 'duration_ms' auth.log | jq '.duration_ms' | sort -n | tail -10
```

**Common causes:**
1. Database query slow (check DB performance)
2. Network latency (check connectivity)
3. JWT signing slow (rare - check CPU)

### Problem: Database Connection Failed

**Check logs for:**
```bash
grep '"level":"error"' auth.log | grep -i 'database\|connection'
```

**Common causes:**
1. Database service down
2. Wrong connection URL
3. Database credentials wrong
4. Network connectivity issue

---

## Log Analysis Examples

### Count by Status/Level
```bash
jq '.level' auth.log | sort | uniq -c | sort -rn
```

### Find Slowest Operations
```bash
jq 'select(.duration_ms) | {message, duration_ms}' auth.log | sort -k3 -rn | head -20
```

### Find All Failed Operations
```bash
jq 'select(.level == "error" or .level == "warn")' auth.log | jq '.message'
```

### Timeline of Client Activity
```bash
jq 'select(.client_id == "my-service") | {time, message, level}' auth.log
```

### Resource Access Summary
```bash
jq 'select(.resource) | {resource, valid: .valid}' auth.log | sort | uniq -c
```

---

## Integration with Monitoring Tools

### Prometheus Metrics
```
auth_server_token_generated_total{client_id="x"} 100
auth_server_token_validation_total{valid="true"} 950
auth_server_token_validation_total{valid="false"} 50
auth_server_token_revoked_total{client_id="x"} 5
auth_server_errors_total{type="invalid_client"} 10
auth_server_request_duration_seconds 0.015
```

### ELK Stack Integration
```
index: auth-server-logs-*
timestamp: @timestamp
fields: level, client_id, resource, error, duration_ms
```

### CloudWatch Integration
```
log_group: /auth-server/api
log_stream: {environment}/{instance-id}
```

---

## Compliance & Retention

### Recommended Retention Periods
- **All logs**: 30 days (operational troubleshooting)
- **Error logs**: 90 days (incident investigation)
- **Auth logs**: 365 days (compliance/audit)
- **Failed auth attempts**: 365 days (security)

### Compliance Frameworks
- **GDPR**: Hash client IDs, don't log PII
- **SOC 2**: Maintain immutable audit logs
- **HIPAA**: Encrypt logs, restrict access
- **PCI-DSS**: Log all authentication attempts

---

This comprehensive logging and error handling system ensures:
✅ Security incidents are captured
✅ Performance issues are detectable
✅ Operational problems are debuggable
✅ Compliance requirements are met
✅ User support can trace issues
