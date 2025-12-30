# Quick Reference: OAuth 2.0 M2M Changes

## What Changed?

### 1. Token Request (Simpler)
```bash
# BEFORE - Need to specify scope
POST /token
{
  "grant_type": "client_credentials",
  "client_id": "my-app",
  "client_secret": "secret123",
  "scope": "read"  ❌ REMOVED
}

# AFTER - Scope auto-fetched from client config
POST /token
{
  "grant_type": "client_credentials",
  "client_id": "my-app",
  "client_secret": "secret123"
}
```

### 2. Validate Endpoint (Resource-Based)
```bash
# BEFORE - Validate scope capability
POST /validate
Authorization: Bearer {token}
Scope: read  ❌ CHANGED

# AFTER - Validate resource access via nginx
POST /validate
Authorization: Bearer {token}
X-Forwarded-For: https://api.example.com/users  ✅ NEW
```

### 3. Response Format (Enhanced)
```json
// BEFORE
{
  "valid": true,
  "client_id": "my-app",
  "expires_at": "2025-12-30T21:00:00Z"
}

// AFTER - Includes scopes
{
  "valid": true,
  "client_id": "my-app",
  "expires_at": "2025-12-30T21:00:00Z",
  "scopes": [                          // ✨ NEW
    "https://api.example.com/users",
    "https://api.example.com/data"
  ]
}
```

---

## Nginx Integration

### Setup
```nginx
location /api/ {
    access_by_lua_block {
        local res = client:request_uri("http://auth-server:8080/validate", {
            method = "POST",
            headers = {
                ["Authorization"] = ngx.var.http_authorization,
                ["X-Forwarded-For"] = ngx.var.request_uri  -- ← KEY CHANGE
            }
        })
    }
    proxy_pass http://backend-service;
}
```

### Flow
```
Request → Nginx → Auth Server
                  (validates token + resource)
              → Backend ✓ (if valid)
              → 403 (if resource not in scope)
```

---

## Token Structure

```json
{
  "client_id": "my-app",
  "token_id": "a1b2c3d4e5f6g7h8",
  "scope": [              // ✨ Array of URLs
    "https://api.example.com/users",
    "https://api.example.com/data",
    "https://api.example.com/audit"
  ],
  "iat": 1735685400,
  "exp": 1735685520,
  "iss": "auth-server"
}
```

---

## Status Codes

| Request | Result | Status |
|---------|--------|--------|
| Valid token + authorized resource | ✅ | 200 |
| Missing X-Forwarded-For | ❌ | 400 |
| Missing Authorization | ❌ | 401 |
| Invalid token | ❌ | 401 |
| Resource not in scope | ❌ | 403 |

---

## Testing

```bash
# Get token
TOKEN=$(curl -s -X POST http://localhost:8080/token \
  -H "Content-Type: application/json" \
  -d '{
    "grant_type": "client_credentials",
    "client_id": "my-app",
    "client_secret": "secret123"
  }' | jq -r .access_token)

# Validate for specific resource
curl -X POST http://localhost:8080/validate \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Forwarded-For: https://api.example.com/users"

# Should return:
# {
#   "valid": true,
#   "client_id": "my-app",
#   "expires_at": "...",
#   "scopes": [...]
# }
```

---

## Key Benefits

✅ **Simpler API** - No scope in requests  
✅ **Secure** - Resource-level validation  
✅ **Automatic** - Scopes fetched from config  
✅ **Gateway-Friendly** - Works with nginx/Envoy  
✅ **Audit Trail** - All access logged  

---

## Files Changed

- [models.go](auth/models.go) - Removed `Scope` from `TokenRequest`
- [handlers.go](auth/handlers.go) - Updated `/validate` endpoint
- [tokens.go](auth/tokens.go) - Auto-fetch scopes in JWT generation
- [handlers_test.go](auth/handlers_test.go) - Updated tests for new behavior

---

## Questions?

See [OAUTH2_M2M_CHANGES.md](OAUTH2_M2M_CHANGES.md) for detailed documentation.

