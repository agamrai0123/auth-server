# Auth Server - Configuration & Setup Guide

## Overview

This is a Go-based OAuth2 authentication server with improved configuration management, comprehensive logging, and structured error handling.

## Key Improvements

### 1. **Enhanced Configuration Management** (`auth/config.go`)

- **Multi-path config loading**: Searches multiple directories for `auth-server-config.json`
- **Default values**: Automatic fallback to sensible defaults if config file is missing
- **Comprehensive validation**: Validates all required and optional configuration fields
- **Structured config types**: Separate types for logging, database, and JWT configurations
- **Support for environments**: Development vs. production configuration variations

#### Configuration File (`config/auth-server-config.json`)

```json
{
  "version": "1.0.0",
  "environment": "development",
  "server_port": "8080",
  "metric_port": 9090,
  "logging": {
    "level": -1,
    "path": "./logs/auth-server.log",
    "max_size_mb": 100,
    "max_backups": 10,
    "max_age_days": 14,
    "compress": true
  },
  "database": {
    "host": "localhost",
    "port": 4001,
    "timeout_seconds": 30
  },
  "jwt": {
    "secret_key": "67d81e2c5717548a4ee1bd1e81395746",
    "access_duration_minutes": 15,
    "refresh_duration_hours": 24
  }
}
```

**Configuration Options:**

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `version` | string | Application version | "1.0.0" |
| `environment` | string | Running environment (development/production) | "development" |
| `server_port` | string | HTTP server port | "8080" |
| `metric_port` | int | Metrics/monitoring port | 9090 |
| `logging.level` | int | Log level (-1=debug, 0=info, 1=warn, 2=error) | -1 |
| `logging.path` | string | Path to log file | "./logs/auth-server.log" |
| `logging.max_size_mb` | int | Max log file size before rotation | 100 |
| `logging.max_backups` | int | Number of backup log files to keep | 10 |
| `logging.max_age_days` | int | Max days to keep log files | 14 |
| `logging.compress` | bool | Compress rotated log files | true |
| `database.host` | string | Database host | "localhost" |
| `database.port` | int | Database port | 4001 |
| `database.timeout_seconds` | int | Database connection timeout | 30 |
| `jwt.secret_key` | string | JWT signing secret | "" |
| `jwt.access_duration_minutes` | int | Access token TTL | 15 |
| `jwt.refresh_duration_hours` | int | Refresh token TTL | 24 |

### 2. **Improved Logging** (`auth/logger.go`)

- **Structured logging with Zerolog**: Every log includes context
- **Request-scoped logging**: Each HTTP request gets a unique ID and logger
- **Log rotation**: Automatic log file rotation with compression
- **Environment-aware**: Logs to both stdout and file in development, file-only in production
- **Multiple middlewares**: Dedicated middleware for logging, CORS, and panic recovery

#### Logging Features:

- **Request ID**: Every request gets a UUID for tracking
- **Structured fields**: Method, path, status, duration, IP, user agent
- **Log levels**: Based on HTTP status codes (5xx=error, 4xx=warn, 3xx=debug, 2xx=info)
- **Performance metrics**: Request duration in milliseconds, response size in bytes
- **Panic recovery**: Panics are caught, logged, and converted to error responses

#### Using Request Logging:

```go
// Get the request-specific logger
logger := GetRequestLogger(c)
logger.Info().Msg("Processing request")

// Get the request ID
requestID := GetRequestID(c)
```

### 3. **Structured Error Handling** (`auth/errors.go`)

- **Standardized error codes**: Consistent error responses across the API
- **HTTP status code mapping**: Automatic status code assignment
- **Error context**: Original error stored for logging without exposing to clients
- **Request ID tracking**: Errors include request IDs for debugging

#### Error Codes:

| Code | HTTP Status | Use Case |
|------|-------------|----------|
| `invalid_request` | 400 | Malformed request |
| `invalid_client` | 401 | Invalid client credentials |
| `invalid_grant` | 401 | Invalid grant/token |
| `invalid_scope` | 400 | Invalid scope requested |
| `unauthorized` | 401 | Authentication required |
| `forbidden` | 403 | Insufficient permissions |
| `not_found` | 404 | Resource not found |
| `conflict` | 409 | Resource conflict |
| `validation_failed` | 400 | Request validation failed |
| `internal_server_error` | 500 | Server error |
| `service_unavailable` | 503 | Service temporarily unavailable |
| `database_error` | 500 | Database operation failed |

#### Using Error Handling:

```go
// Create and return a specific error
if err != nil {
    apiErr := ErrBadRequest("Invalid request format").
        WithOriginalError(err).
        WithDetails("Field 'email' is required")
    RespondWithError(c, apiErr)
    return
}

// Or use the generic error creator
apiErr := NewAPIError(
    ErrInvalidClient,
    "Client credentials are invalid",
    http.StatusUnauthorized,
)
RespondWithError(c, apiErr)
```

### 4. **Improved Main Application** (`main.go`)

- **Graceful shutdown**: Handles OS signals (SIGINT, SIGTERM) for clean shutdown
- **Timeout-based shutdown**: 30-second timeout for graceful shutdown
- **Error handling**: Proper exit codes and error reporting
- **Service initialization**: Proper initialization order and dependency management

#### Server Startup Flow:

1. Load configuration
2. Initialize logger
3. Create auth server instance
4. Start HTTP server
5. Wait for shutdown signal
6. Graceful shutdown with timeout
7. Exit with appropriate code

### 5. **Enhanced Service Management** (`auth/service.go`)

- **Proper middleware ordering**: Logging → CORS → Recovery
- **Server timeouts**: Read/Write timeouts for HTTP server
- **Database initialization**: Proper error handling and logging
- **Graceful shutdown**: Clean resource cleanup
- **Database connection pooling**: Supports connection timeout configuration

## Running the Server

### Prerequisites

- Go 1.25.1 or higher
- Database (rqlite) running on localhost:4001

### Basic Setup

1. Create config directory:
```bash
mkdir config
```

2. Create `config/auth-server-config.json` with your settings

3. Build the application:
```bash
go mod tidy
go build -o auth-server.exe
```

4. Run the server:
```bash
./auth-server.exe
```

### Development Mode

In development mode, logs are written to both stdout and file, making debugging easier:

```json
{
  "environment": "development",
  "logging": {
    "level": -1,
    "path": "./logs/auth-server.log"
  }
}
```

### Production Mode

In production mode, logs go only to file for performance:

```json
{
  "environment": "production",
  "logging": {
    "level": 0,
    "path": "/var/log/auth-server/auth-server.log"
  }
}
```

## API Error Responses

All error responses follow this format:

```json
{
  "error": "error_code",
  "error_description": "Human-readable error message",
  "request_id": "uuid-v4",
  "details": "Additional context (if available)"
}
```

Example:

```json
{
  "error": "invalid_request",
  "error_description": "Invalid JSON format",
  "request_id": "550e8400-e29b-41d4-a716-446655440000",
  "details": "Field 'client_id' is required"
}
```

## Log Format

Log entries include structured fields:

```json
{
  "level": "info",
  "timestamp": "2025-12-30T10:15:30.123456Z",
  "service": "auth_server",
  "version": "1.0.0",
  "environment": "development",
  "request_id": "550e8400-e29b-41d4-a716-446655440000",
  "client_ip": "192.168.1.100",
  "host": "server01",
  "pid": 12345,
  "user_agent": "Mozilla/5.0...",
  "method": "POST",
  "path": "/auth-server/v1/oauth/token",
  "status": 200,
  "response_size_bytes": 1024,
  "duration_ms": 45.5,
  "message": "Request completed"
}
```

## Dependencies

- `github.com/gin-gonic/gin` - Web framework
- `github.com/golang-jwt/jwt/v5` - JWT handling
- `github.com/rs/zerolog` - Structured logging
- `github.com/spf13/viper` - Configuration management
- `gopkg.in/natefinch/lumberjack.v2` - Log rotation
- `github.com/google/uuid` - UUID generation
- `github.com/rqlite/gorqlite` - Database client

## Monitoring and Debugging

### Log Levels

- `-1`: Debug - Detailed diagnostic information
- `0`: Info - General informational messages
- `1`: Warn - Warning messages for potential issues
- `2`: Error - Error messages only
- `3+`: Fatal - Only fatal errors

### Viewing Logs

```bash
# Real-time log tailing
tail -f logs/auth-server.log

# View recent errors
grep "ERROR" logs/auth-server.log | tail -20

# Search by request ID
grep "550e8400-e29b-41d4-a716-446655440000" logs/auth-server.log
```

## Architecture Notes

The improved auth server follows these design principles:

1. **Separation of Concerns**: Configuration, logging, and error handling are separate modules
2. **Structured Logging**: All logs include context for better debugging
3. **Graceful Error Handling**: Errors are handled at the boundary (HTTP handlers)
4. **Timeouts**: All operations have timeouts to prevent hanging
5. **Resource Cleanup**: Proper shutdown sequence ensures clean resource release
6. **Observability**: Request IDs and structured logs enable end-to-end tracing

## Future Enhancements

- [ ] Health check endpoint
- [ ] Metrics export (Prometheus)
- [ ] Distributed tracing (OpenTelemetry)
- [ ] Request rate limiting
- [ ] TLS/HTTPS support
- [ ] Database migrations
- [ ] Configuration hot-reload
