# OAuth 2.0 M2M Authentication Server - Complete Workflow Guide

## Table of Contents
1. [System Architecture](#system-architecture)
2. [Token Generation Flow](#token-generation-flow)
3. [Token Validation Flow](#token-validation-flow)
4. [Token Revocation Flow](#token-revocation-flow)
5. [Logging & Error Handling](#logging--error-handling)
6. [Database Operations](#database-operations)
7. [Security Model](#security-model)
8. [API Reference](#api-reference)

---

## System Architecture

### Overview Diagram
```
┌─────────────────────────────────────────────────────────────────┐
│                    API Request Flow                              │
└─────────────────────────────────────────────────────────────────┘

Client Application
    ↓
    │ 1. POST /token (request access token)
    ↓
┌─────────────────────────┐
│  Auth Server            │
│  - Validate client      │
│  - Generate JWT         │
│  - Store token in DB    │
└─────────────────────────┘
    ↓
    │ 2. Return Bearer token (JWT)
    ↓
Client Application
    ↓
    │ 3. Use token to access protected resources
    ↓
┌──────────────────────────────────────────┐
│  Nginx API Gateway                        │
│  - Intercept request                     │
│  - Extract Authorization header          │
│  - Pass resource URL in X-Forwarded-For │
└──────────────────────────────────────────┘
    ↓
    │ 4. POST /validate (verify token)
    ↓
┌─────────────────────────┐
│  Auth Server            │
│  - Parse JWT token      │
│  - Check token validity │
│  - Verify resource URL  │
│  - Check revocation     │
└─────────────────────────┘
    ↓
    │ 5. Return validation result
    ↓
Nginx API Gateway
    ↓
    ├─ Valid ✓ → Forward to Backend Service → Return Response
    └─ Invalid ✗ → 401/403 Error
```

---

## Token Generation Flow

### Request: POST /token

**Client sends:**
```bash
curl -X POST http://auth-server:8080/token \
  -H "Content-Type: application/json" \
  -d '{
    "grant_type": "client_credentials",
    "client_id": "my-service",
    "client_secret": "supersecret123"
  }'
```

### Step-by-Step Processing

#### 1. **Request Validation**
```
Input: JSON body with grant_type, client_id, client_secret
├─ Parse JSON from request body
│  └─ [❌ Invalid JSON] → 400 Bad Request
│     Log: "Failed to decode token request JSON"
│
├─ Validate HTTP method (must be POST)
│  └─ [❌ Not POST] → 405 Method Not Allowed
│     Log: "Invalid HTTP method for token endpoint"
│
└─ [✓ Valid] → Continue to client validation
   Log: "Processing token request"
```

#### 2. **Client Authentication**
```
Step 1: Lookup client in database by client_id
├─ Query: SELECT * FROM clients WHERE client_id = ?
├─ [❌ Client not found] → 500 Internal Server Error
│  Log: "Client lookup failed"
└─ [✓ Found] → Continue to credential verification

Step 2: Verify client_secret matches
├─ Compare provided secret with stored secret
├─ [❌ Mismatch or null] → 401 Unauthorized
│  Log: "Invalid client credentials"
└─ [✓ Match] → Continue to JWT generation
   Log: "Client credentials validated"
```

#### 3. **Scope Fetching** ⭐ NEW M2M FEATURE
```
Current Step: JWT generation now automatically includes all scopes

Query: SELECT allowed_scopes FROM clients WHERE client_id = ?
├─ Retrieve scopes as JSON string: '["https://api/users", "https://api/data"]'
├─ [❌ No rows] → 500 Error, Log: "Client not found in database"
├─ [❌ JSON parse error] → 500 Error, Log: "Failed to unmarshal allowed_scopes JSON"
│
└─ [✓ Success] → Parse JSON to []string array
   ├─ scopes = ["https://api/users", "https://api/data", "https://api/audit"]
   └─ Log: "Client scopes retrieved"
```

#### 4. **JWT Creation**
```
Create Claims struct:
├─ ClientID:    "my-service"
├─ TokenID:     random 16-byte hex string
├─ Scope:       ["https://api/users", "https://api/data", "https://api/audit"]
├─ IssuedAt:    current timestamp
├─ ExpiresAt:   current + 2 minutes (120 seconds)
├─ Issuer:      "auth-server"
└─ SigningKey:  client_secret (HMAC-SHA256)

Log: "Generating JWT token"
     "Client scopes fetched"
     "JWT token signature valid"
```

JWT Token Payload Example:
```json
{
  "client_id": "my-service",
  "token_id": "a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6",
  "scope": [
    "https://api/users",
    "https://api/data",
    "https://api/audit"
  ],
  "iat": 1735685400,
  "exp": 1735685520,
  "iss": "auth-server"
}
```

#### 5. **Token Storage**
```
Insert token into database:
├─ Query: INSERT INTO tokens (token_id, client_id, issued_at, expires_at) 
│          VALUES (?, ?, ?, ?)
├─ [❌ Insert fails] → 500 Error
│  Log: "Failed to store token in database"
│
└─ [✓ Success] → Token stored
   Log: "Token created and storing in database"
        "Token inserted successfully"
```

### Response: 200 OK
```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJjbGllbnRfaWQiOiJteS1zZXJ2aWNlIiwic2NvcGUiOlsiL2FwaS91c2VycyIsIi9hcGkvZGF0YSJdfQ.SIGNATURE",
  "token_type": "Bearer",
  "expires_in": 120
}
```

**Final Log:**
```
INFO: JWT token generated successfully
      client_id=my-service token_id=a1b2c3d4e5f6g7h8
```

---

## Token Validation Flow

### Request: POST /validate

**Nginx gateway sends:**
```bash
curl -X POST http://auth-server:8080/validate \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..." \
  -H "X-Forwarded-For: https://api/users"
```

### Step-by-Step Processing

#### 1. **Resource Endpoint Extraction** ⭐ NEW M2M FEATURE
```
Header: X-Forwarded-For (passed by nginx API gateway)
├─ Requested resource: "https://api/users"
├─ [❌ Missing header] → 400 Bad Request
│  Log: "Missing X-Forwarded-For header (resource endpoint)"
│
└─ [✓ Present] → Continue to authorization header check
   Log: "Validating access to resource"
        "resource=https://api/users"
```

#### 2. **Authorization Header Validation**
```
Step 1: Extract Authorization header
├─ Header format: "Authorization: Bearer {token}"
├─ [❌ Missing] → 401 Unauthorized
│  Log: "Missing Authorization header"
│
└─ [✓ Present] → Continue to format check

Step 2: Validate Bearer format
├─ Extract token with: TrimPrefix(authHeader, "Bearer ")
├─ [❌ Not "Bearer {token}" format] → 401 Unauthorized
│  Log: "Invalid Bearer token format for validation"
│
└─ [✓ Valid format] → Continue to JWT parsing
   Log: "Processing Bearer token"
```

#### 3. **JWT Token Parsing**
```
Parse JWT with HMAC-SHA256:
├─ Extract payload and verify signature using client_secret
├─ [❌ Invalid signature] → 401 Unauthorized
│  Log: "JWT token parsing failed"
│       "error: crypto/sha256: signature mismatch"
│
├─ [❌ Expired token] → 401 Unauthorized
│  Log: "JWT token parsing failed"
│       "error: token expired"
│
└─ [✓ Valid] → Extract claims and continue
   Log: "JWT token signature valid"
        "client_id=my-service token_id=a1b2c3d4e5f6g7h8"
```

#### 4. **Revocation Check**
```
Query revocation database:
├─ Query: SELECT revoked FROM tokens WHERE token_id = ?
├─ [❌ Token revoked] → 401 Unauthorized
│  Log: "Token has been revoked"
│
└─ [✓ Not revoked] → Continue to scope validation
   Log: "Token is valid and not revoked"
```

#### 5. **Resource Authorization** ⭐ NEW M2M FEATURE - CORE FEATURE
```
Current Step: Verify resource URL is in token's scopes

Token claims contain:
├─ Scope: ["https://api/users", "https://api/data", "https://api/audit"]
│
Requested resource from nginx:
├─ X-Forwarded-For: "https://api/users"
│
Check: Is requested resource in token scopes?
├─ [❌ NOT FOUND] → 403 Forbidden (Resource forbidden)
│  Example: Requesting "https://api/unauthorized" with token for "https://api/users"
│  Log: "Resource not in token scopes - access denied"
│       "client_id=my-service"
│       "resource=https://api/unauthorized"
│       "allowed_scopes=[https://api/users, https://api/data, ...]"
│
└─ [✓ FOUND] → Continue to response
   Log: "Token validated for resource - access granted"
        "client_id=my-service"
        "resource=https://api/users"
        "expires_at=2025-12-30T21:30:00Z"
```

### Response: 200 OK (Valid Token)
```json
{
  "valid": true,
  "client_id": "my-service",
  "expires_at": "2025-12-30T21:30:00Z",
  "scopes": [
    "https://api/users",
    "https://api/data",
    "https://api/audit"
  ]
}
```

### Response: 403 Forbidden (Resource Not in Scopes)
```json
{
  "valid": false,
  "client_id": "",
  "expires_at": "0001-01-01T00:00:00Z",
  "scopes": null
}
```

---

## Token Revocation Flow

### Request: POST /revoke

**Client sends:**
```bash
curl -X POST http://auth-server:8080/revoke \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
```

### Step-by-Step Processing

#### 1. **Token Extraction**
```
Same as validation:
├─ [❌ Missing Authorization] → 401
├─ [❌ Invalid Bearer format] → 401
└─ [✓ Valid format] → Continue to JWT parsing
```

#### 2. **JWT Validation**
```
Same as validation:
├─ [❌ Invalid signature] → 401
├─ [❌ Expired] → 401
└─ [✓ Valid] → Extract client_id and token_id
   Log: "Revoking token"
```

#### 3. **Token Revocation**
```
Update database:
├─ Query: UPDATE tokens SET revoked=true, revoked_at=NOW()
│          WHERE token_id = ?
│
├─ [❌ Update fails] → 500 Internal Server Error
│  Log: "Failed to revoke token"
│       "client_id=my-service token_id=a1b2c3d4e5f6g7h8"
│
└─ [✓ Success] → Token marked as revoked
   Log: "Token revoked successfully"
        "client_id=my-service token_id=a1b2c3d4e5f6g7h8"
```

### Response: 200 OK
```json
{
  "message": "Token revoked successfully"
}
```

Once revoked, any validation request using this token will fail with:
```
Log: "Token has been revoked"
Response: {"valid": false, ...}
```

---

## Logging & Error Handling

### Log Levels

**DEBUG** - Detailed operational information (development)
```
Enabled in development mode, disabled in production
├─ Client scope lookups
├─ JWT token generation details
├─ Token storage operations
└─ Request processing steps
```

**INFO** - General informational messages (production)
```
Suitable for monitoring and audit trails
├─ Successful token generation
├─ Successful token validation
├─ Successful token revocation
├─ Database connection established
└─ Important state changes
```

**WARN** - Warning conditions (potential issues)
```
Abnormal but recoverable situations
├─ Invalid client credentials
├─ Token validation failures
├─ Missing required headers
├─ Unsupported grant types
├─ Token revocation status checks
└─ Client not found scenarios
```

**ERROR** - Error conditions (issues requiring attention)
```
Serious problems that may need investigation
├─ Database connection failures
├─ JWT signing failures
├─ Token storage failures
├─ JSON parsing/encoding failures
├─ Unauthorized access attempts
└─ Server errors during processing
```

### Structured Logging Fields

All logs include context-specific fields:

```
Token Generation Success:
{
  "level": "info",
  "client_id": "my-service",
  "token_id": "a1b2c3d4e5f6g7h8",
  "message": "JWT token generated successfully",
  "time": "2025-12-30T20:48:14+05:30"
}

Token Validation Success:
{
  "level": "info",
  "client_id": "my-service",
  "resource": "https://api/users",
  "expires_at": "2025-12-30T21:30:00Z",
  "message": "Token validated for resource - access granted",
  "time": "2025-12-30T20:48:14+05:30"
}

Authorization Failure:
{
  "level": "warn",
  "client_id": "my-service",
  "resource": "https://api/unauthorized",
  "allowed_scopes": ["https://api/users", "https://api/data"],
  "message": "Resource not in token scopes - access denied",
  "time": "2025-12-30T20:48:14+05:30"
}

Database Error:
{
  "level": "error",
  "error": "connection refused",
  "client_id": "my-service",
  "message": "Client lookup failed",
  "time": "2025-12-30T20:48:14+05:30"
}
```

### Error Response Format

All errors return JSON with OAuth 2.0 standard format:

```json
{
  "error": "invalid_client|invalid_request|server_error|unsupported_grant_type",
  "error_description": "Human readable error message"
}
```

| Error Code | HTTP Status | Description |
|------------|-------------|-------------|
| `invalid_request` | 400 | Malformed request or JSON |
| `invalid_client` | 401 | Client credentials are invalid |
| `unsupported_grant_type` | 400 | Grant type not supported |
| `server_error` | 500 | Internal server error |
| `missing_token` | 401 | Authorization header missing |
| `invalid_token_format` | 401 | Token not in Bearer format |
| `invalid_token` | 401 | Token signature or claims invalid |

---

## Database Operations

### Client Lookup
```
Query: SELECT client_id, client_secret, access_token_ttl, allowed_scopes 
       FROM clients 
       WHERE client_id = ?

Errors:
├─ sql.ErrNoRows → Client not found
└─ Other errors → Database query failed

Log Levels:
├─ DEBUG: "Looking up client in database"
├─ WARN: "Client not found in database" (no rows)
└─ ERROR: "Database query failed" (actual error)
```

### Token Insertion
```
Query: INSERT INTO tokens (token_id, client_id, issued_at, expires_at) 
       VALUES (?, ?, ?, ?)

Errors:
├─ Constraint violations (duplicate key)
└─ Database unavailable

Log Levels:
├─ DEBUG: "Token created and storing in database"
├─ INFO: "Token inserted successfully" (on success)
└─ ERROR: "Failed to store token in database" (on error)
```

### Scope Fetching
```
Query: SELECT allowed_scopes FROM clients WHERE client_id = ?

Parsing:
├─ Input format: '["url1", "url2"]' or '[url1, url2]'
├─ Parse as JSON first
├─ Fallback to CSV parsing
└─ Return []string array

Log Levels:
├─ DEBUG: "Fetching client scopes from database"
├─ DEBUG: "Client scopes retrieved" with scopes array
├─ WARN: "Client not found in database" (no rows)
└─ ERROR: "Failed to unmarshal allowed_scopes JSON" (parse error)
```

---

## Security Model

### 1. **Authentication**
- Client presents `client_id` and `client_secret`
- Secrets are verified against database
- Failed attempts logged as warnings

### 2. **Token Signing**
- JWT tokens signed with HMAC-SHA256
- Client's `client_secret` is the signing key
- Any modification detected on validation

### 3. **Token Expiration**
- Each token has `exp` (expiration) claim
- Expired tokens rejected automatically
- Default: 2 minutes (120 seconds) for testing
- Production: Configure to 3600 (1 hour)

### 4. **Revocation**
- Tokens can be immediately invalidated
- Revoked tokens marked in database
- Validation checks revocation status
- Prevents reuse of leaked tokens

### 5. **Scope-Based Access** ⭐ NEW M2M FEATURE
- Tokens contain resource URLs they can access
- API gateway validates each request
- Token cannot be used outside authorized scopes
- Prevents privilege escalation

### 6. **HTTPS Required** ✅
- All communication must use HTTPS in production
- Tokens are sensitive data
- Environment variable enables SSL

### 7. **Audit Trail**
- All operations logged with context
- Structured logs for parsing
- Timestamps and client info recorded
- Failures logged at WARNING/ERROR level

---

## API Reference

### Endpoint 1: POST /token

**Purpose:** Request an access token for a service

**Request:**
```
Method: POST
Content-Type: application/json

{
  "grant_type": "client_credentials",  [required: must be "client_credentials"]
  "client_id": "service-name",          [required: your service identifier]
  "client_secret": "secret-key"         [required: your service secret]
}
```

**Response (200 OK):**
```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "token_type": "Bearer",
  "expires_in": 120
}
```

**Error Responses:**
- `400` - Invalid JSON or unsupported grant type
- `401` - Invalid client credentials
- `500` - Server error (database issues, JWT signing)

**cURL Example:**
```bash
curl -X POST http://localhost:8080/token \
  -H "Content-Type: application/json" \
  -d '{
    "grant_type": "client_credentials",
    "client_id": "my-app",
    "client_secret": "secret123"
  }'
```

---

### Endpoint 2: POST /validate

**Purpose:** Validate a token for a specific resource (called by nginx)

**Request:**
```
Method: POST
Authorization: Bearer {token}
X-Forwarded-For: {resource-url}
```

**Response (200 OK - Valid):**
```json
{
  "valid": true,
  "client_id": "my-app",
  "expires_at": "2025-12-30T21:30:00Z",
  "scopes": ["https://api/users", "https://api/data"]
}
```

**Response (200 OK - Invalid):**
```json
{
  "valid": false,
  "client_id": "",
  "expires_at": "0001-01-01T00:00:00Z",
  "scopes": null
}
```

**Error Responses:**
- `400` - Missing X-Forwarded-For header
- `401` - Missing/invalid authorization header or token
- `403` - Token valid but resource not in scopes

**cURL Example:**
```bash
curl -X POST http://localhost:8080/validate \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..." \
  -H "X-Forwarded-For: https://api/users"
```

---

### Endpoint 3: POST /revoke

**Purpose:** Revoke (invalidate) a token immediately

**Request:**
```
Method: POST
Authorization: Bearer {token}
```

**Response (200 OK):**
```json
{
  "message": "Token revoked successfully"
}
```

**Error Responses:**
- `401` - Missing/invalid authorization header
- `500` - Database error

**cURL Example:**
```bash
curl -X POST http://localhost:8080/revoke \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
```

---

## Configuration & Deployment

### Environment Variables
```bash
SERVER_PORT=8080              # HTTP port for auth server
JWT_SECRET=your-secret-key    # Signing key (use strong random value)
DATABASE_URL=http://localhost:8848  # rqlite database URL
LOG_LEVEL=info                # debug, info, warn, error
LOG_PATH=./logs/auth.log      # Log file path
ENVIRONMENT=production        # development or production
```

### Production Checklist
- [ ] Change `expires_in` from 120 to 3600 seconds (1 hour)
- [ ] Enable HTTPS on the server
- [ ] Use strong random `JWT_SECRET`
- [ ] Configure `LOG_LEVEL=info` (not debug)
- [ ] Set `ENVIRONMENT=production`
- [ ] Ensure database is backed up and replicated
- [ ] Set up log rotation (lumberjack is configured)
- [ ] Monitor error and warning logs regularly
- [ ] Configure nginx API gateway with validation
- [ ] Test token revocation handling

---

## Performance & Monitoring

### Expected Latency
- Token request: 10-50ms (database + JWT signing)
- Token validation: 5-30ms (database lookup + JWT parsing)
- Token revocation: 5-20ms (database update)

### Monitoring Queries

**Count failed authentications (last hour):**
```
grep "Invalid client credentials" auth.log | wc -l
```

**Count access denials (resource not in scope):**
```
grep "Resource not in token scopes" auth.log | wc -l
```

**Count database errors:**
```
grep "error\|failed" auth.log | grep -i database | wc -l
```

**Extract slow requests (>100ms):**
```
grep "duration_ms" auth.log | awk -F'duration_ms":' '{print $2}' | awk '{if ($1+0 > 100) print}'
```

---

## Troubleshooting

### Client Gets 401 Unauthorized
```
Possible causes:
1. Invalid client_id or client_secret
   → Check credentials against database
   → Review logs: "Invalid client credentials"

2. Token expired
   → Request new token
   → Check ExpiresAt in response

3. Token revoked
   → Request new token
   → Check logs: "Token has been revoked"
```

### Client Gets 403 Forbidden
```
Possible causes:
1. Requesting resource not in token scopes
   → Check requested resource URL
   → Verify it matches a scope in client config
   → Review logs: "Resource not in token scopes"
```

### Client Gets 400 Bad Request
```
Possible causes:
1. Malformed JSON in token request
   → Check JSON syntax
   → Review logs: "Failed to decode token request JSON"

2. Missing X-Forwarded-For header in validation
   → Nginx gateway must pass the header
   → Review logs: "Missing X-Forwarded-For header"
```

### Client Gets 500 Server Error
```
Possible causes:
1. Database connection failure
   → Check database is running
   → Review logs: "Database connected successfully"

2. JWT signing failure
   → Check JWT_SECRET is set and valid
   → Review logs: "Failed to sign JWT token"

3. Token storage failure
   → Check database has tokens table
   → Review logs: "Failed to store token in database"
```

---

## Testing

### Unit Tests (71 total)
```bash
go test -v ./...
```

Coverage includes:
- Configuration loading and validation ✓
- Error handling with all status codes ✓
- Token generation and JWT operations ✓
- Token validation and scope checking ✓
- Middleware and logging ✓
- Handler edge cases ✓

### Integration Test Example
```bash
#!/bin/bash

# 1. Request token
TOKEN=$(curl -s -X POST http://localhost:8080/token \
  -H "Content-Type: application/json" \
  -d '{
    "grant_type": "client_credentials",
    "client_id": "test-client",
    "client_secret": "secret123"
  }' | jq -r '.access_token')

echo "Token: $TOKEN"

# 2. Validate token for allowed resource
curl -X POST http://localhost:8080/validate \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Forwarded-For: https://api/users"

# 3. Validate token for unauthorized resource (should fail)
curl -X POST http://localhost:8080/validate \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Forwarded-For: https://api/unauthorized"

# 4. Revoke token
curl -X POST http://localhost:8080/revoke \
  -H "Authorization: Bearer $TOKEN"

# 5. Try to use revoked token (should fail)
curl -X POST http://localhost:8080/validate \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Forwarded-For: https://api/users"
```

---

## Summary

This OAuth 2.0 M2M auth server implements:

✅ **Client Credentials Flow** - Service-to-service authentication  
✅ **Automatic Scope Management** - No scope in request, auto-fetched  
✅ **Resource-Based Authorization** - Token validates resource URLs  
✅ **Token Revocation** - Immediate token invalidation  
✅ **Comprehensive Logging** - Structured, context-aware logs  
✅ **Error Handling** - Clear error messages and HTTP status codes  
✅ **Database Persistence** - Token storage and revocation tracking  
✅ **Security** - JWT signing, expiration, revocation checks  
✅ **API Gateway Integration** - Nginx-friendly validation endpoint  

Perfect for microservices, API gateways, and service-to-service communication!
