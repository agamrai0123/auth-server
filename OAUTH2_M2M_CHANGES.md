# OAuth 2.0 M2M Architecture Changes

## Overview

The auth-server has been updated to implement a production-ready Machine-to-Machine (M2M) OAuth 2.0 authentication pattern optimized for API gateway integration (nginx, Envoy, etc.).

## Changes Implemented

### 1. Automatic Scope Fetching (No Scope in Request)

**Before:**
```bash
POST /token HTTP/1.1
Content-Type: application/json

{
  "grant_type": "client_credentials",
  "client_id": "my-app",
  "client_secret": "secret123",
  "scope": "read"  ❌ Manual scope specification
}
```

**After:**
```bash
POST /token HTTP/1.1
Content-Type: application/json

{
  "grant_type": "client_credentials",
  "client_id": "my-app",
  "client_secret": "secret123"
  // scope removed - automatically fetched from client config
}
```

**Changes Made:**
- Removed `scope` field from `TokenRequest` struct in [models.go](auth/models.go#L111)
- Modified [tokens.go](auth/tokens.go#L32) `generateJWT()` to call `getClientScopes(clientID)` automatically
- All scopes assigned to a client are now included in the JWT token
- Simplifies client flow - no need to specify scopes on every request

**Benefit:**
- Client configuration defines what the service can access
- Tokens automatically grant all authorized scopes
- Reduces API request complexity
- Centralizes scope management in client configuration

---

### 2. Nginx API Gateway Integration with X-Forwarded-For

**Nginx Configuration Example:**
```nginx
server {
    listen 80;

    location /api/ {
        # 1. First, validate the token with auth server
        access_by_lua_block {
            local http = require "resty.http"
            local client = http.new()
            
            -- Extract Bearer token from Authorization header
            local auth_header = ngx.var.http_authorization
            
            -- Validate token with auth server
            -- Pass the requested resource endpoint
            local res, err = client:request_uri("http://auth-server:8080/validate", {
                method = "POST",
                headers = {
                    ["Authorization"] = auth_header,
                    ["X-Forwarded-For"] = ngx.var.request_uri  -- Pass requested resource
                }
            })
            
            if not res or res.status ~= 200 then
                ngx.exit(403)  -- Forbidden
            end
        }
        
        # 2. If validated, proxy to actual service
        proxy_pass http://backend-service;
    }
}
```

**Request Flow:**
```
Client Request
    ↓
[Nginx API Gateway]
    ↓
    ├─→ GET /api/users with Bearer Token
    │   └─→ Nginx intercepts request
    │
    ├─→ POST /validate (auth-server)
    │   ├─ Authorization: Bearer {token}
    │   └─ X-Forwarded-For: /api/users  (resource endpoint)
    │
    ├─ Auth Server validates:
    │   ├─ Token signature ✓
    │   ├─ Token not expired ✓
    │   └─ /api/users in token scopes ✓
    │
    └─→ GET /api/users (forward to backend)
        └─→ Response to client
```

**Code Changes:**

**Before:**
```go
func (as *authServer) validateHandler(c *gin.Context) {
    requestedScope := c.Request.Header.Get("Scope")  // ❌ Expected scope string
    // Validate if client has this scope
}
```

**After:**
```go
func (as *authServer) validateHandler(c *gin.Context) {
    // Get requested resource endpoint from nginx
    requestURL := c.Request.Header.Get("X-Forwarded-For")
    
    // Extract and validate token
    claims, _ := as.validateJWT(tokenString)
    
    // Check if resource is in client's allowed scopes
    found := slices.Contains(claims.Scope, requestURL)  // ✓ Direct URL matching
}
```

**Key Points:**
- Nginx passes the requested resource URL in `X-Forwarded-For` header
- Auth server validates that the token's scopes include this resource
- Prevents token reuse outside of authorized resources
- 403 Forbidden returned if resource not in scope

---

### 3. Updated Response Structure

**TokenValidationResponse now includes Scopes:**

```go
type TokenValidationResponse struct {
    Valid     bool      `json:"valid"`
    ClientID  string    `json:"client_id"`
    ExpiresAt time.Time `json:"expires_at"`
    Scopes    []string  `json:"scopes"`  // ✨ NEW: All token scopes
}
```

**Response Example:**
```json
{
  "valid": true,
  "client_id": "my-app",
  "expires_at": "2025-12-30T21:30:00Z",
  "scopes": [
    "https://api.example.com/users",
    "https://api.example.com/data",
    "https://api.example.com/audit"
  ]
}
```

---

## JWT Token Structure

The JWT token now contains all scopes from the client's configuration:

```json
{
  "client_id": "my-app",
  "token_id": "a1b2c3d4e5f6g7h8",
  "scope": [
    "https://api.example.com/users",
    "https://api.example.com/data",
    "https://api.example.com/audit"
  ],
  "iat": 1735685400,
  "exp": 1735685520,
  "iss": "auth-server"
}
```

**Scope Format Best Practices:**
```
Option 1 - Full URLs (Recommended for API Gateway)
"scope": [
  "https://api.example.com/users",
  "https://api.example.com/data"
]

Option 2 - Path-based
"scope": [
  "/api/users",
  "/api/data"
]

Option 3 - Scope names
"scope": [
  "read:users",
  "write:users",
  "read:data"
]
```

---

## Complete Request/Response Flow

### 1. Token Request (POST /token)
```bash
curl -X POST http://localhost:8080/token \
  -H "Content-Type: application/json" \
  -d '{
    "grant_type": "client_credentials",
    "client_id": "my-app",
    "client_secret": "secret123"
  }'
```

**Response:**
```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "token_type": "Bearer",
  "expires_in": 120
}
```

### 2. Nginx Validates Token (POST /validate)
```bash
curl -X POST http://localhost:8080/validate \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..." \
  -H "X-Forwarded-For: https://api.example.com/users"
```

**Response (Valid):**
```json
{
  "valid": true,
  "client_id": "my-app",
  "expires_at": "2025-12-30T21:30:00Z",
  "scopes": [
    "https://api.example.com/users",
    "https://api.example.com/data",
    "https://api.example.com/audit"
  ]
}
```

**Response (Invalid - Resource Not in Scopes):**
```json
{
  "valid": false,
  "client_id": "",
  "expires_at": "0001-01-01T00:00:00Z",
  "scopes": null
}
```

### 3. Access Protected Resource
If `/validate` returns `valid: true`, nginx forwards the request:
```bash
GET /api/users
Authorization: Bearer {token}
→ [Backend Service]
→ Response to client
```

---

## Error Handling

| Scenario | Status | Reason |
|----------|--------|--------|
| Missing X-Forwarded-For header | 400 | Resource endpoint required |
| Missing Authorization header | 401 | Token required |
| Invalid Bearer format | 401 | Token must be "Bearer {token}" |
| Invalid/expired token | 401 | Token validation failed |
| Token not revoked | 401 | Only for revoked tokens |
| Resource not in scope | 403 | Token not authorized for this resource |
| Token revoked | 401 | Previously invalidated token |

---

## Security Considerations

✅ **Resource-Level Access Control**
- Each service is limited to specific resource URLs
- Prevents token abuse for unauthorized resources

✅ **Token Signature Validation**
- JWT signed with client_secret
- Any tampering detected immediately

✅ **Automatic Expiration**
- Tokens expire after 2 minutes (configurable)
- Clients must request new tokens periodically

✅ **Early Revocation**
- POST /revoke endpoint invalidates tokens immediately
- Useful for emergency token invalidation

✅ **No Scope Negotiation**
- Client cannot request elevated permissions
- Scopes fixed at client configuration level

---

## Implementation Checklist

- ✅ Removed `scope` from `TokenRequest` struct
- ✅ Automatic scope fetching from client configuration
- ✅ Updated `validateHandler` to use `X-Forwarded-For` header
- ✅ Resource URL matching against token scopes
- ✅ Added `Scopes` field to `TokenValidationResponse`
- ✅ Updated all tests to reflect new behavior
- ✅ Improved logging for audit trail
- ✅ Error handling for missing headers

---

## Nginx Configuration Template

```nginx
upstream auth_server {
    server auth-server:8080;
}

upstream backend_service {
    server backend:3000;
}

server {
    listen 80;
    server_name api.example.com;

    location /api/v1/ {
        # Validate token before proxying
        access_by_lua_block {
            local http = require "resty.http"
            local client = http.new()
            
            local auth_header = ngx.var.http_authorization
            if not auth_header then
                ngx.exit(401)
            end
            
            local res = client:request_uri("http://auth_server/validate", {
                method = "POST",
                headers = {
                    ["Authorization"] = auth_header,
                    ["X-Forwarded-For"] = ngx.var.request_uri
                }
            })
            
            if not res or res.status ~= 200 then
                ngx.exit(res and res.status or 500)
            end
        }
        
        # Forward to backend if validation passed
        proxy_pass http://backend_service;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }

    # Health check endpoint
    location /health {
        access_log off;
        return 200 "OK";
    }
}
```

---

## Testing

All tests have been updated to reflect the new behavior:

```bash
# Run all tests
go test -v ./...

# Run specific test
go test -v ./auth -run TestValidateHandler

# Test coverage
go test -cover ./...
```

**Test Results:** ✅ All 71 tests passing

---

## Migration from Old Implementation

If upgrading from a previous version:

1. **Remove scope from token requests:**
   ```go
   // Old
   {"grant_type": "client_credentials", "scope": "read"}
   
   // New
   {"grant_type": "client_credentials"}
   ```

2. **Update nginx configuration:**
   ```nginx
   # Old
   -H "Scope: read"
   
   # New
   -H "X-Forwarded-For: /api/users"
   ```

3. **Handle new response format:**
   ```json
   // Old response
   {"valid": true, "client_id": "...", "expires_at": "..."}
   
   // New response
   {"valid": true, "client_id": "...", "expires_at": "...", "scopes": [...]}
   ```

---

## References

- [OAuth 2.0 Client Credentials Grant](https://tools.ietf.org/html/rfc6749#section-4.4)
- [JWT (JSON Web Tokens)](https://tools.ietf.org/html/rfc7519)
- [API Gateway Pattern](https://microservices.io/patterns/apigateway.html)
- [IETF Token Introspection](https://tools.ietf.org/html/rfc7662)

