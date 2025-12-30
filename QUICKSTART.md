# Quick Start Guide

## 1. Build the Application

```bash
cd d:\work-projects\auth-server
go mod tidy
go build -o auth-server.exe
```

## 2. Create Config Directory

```bash
mkdir config
```

## 3. Configure the Application

Create `config/auth-server-config.json`:

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

## 4. Run the Server

```bash
./auth-server.exe
```

Expected output:
```
{"level":"info","timestamp":"2025-12-30T...","service":"auth_server","version":"1.0.0","environment":"development","log_path":"./logs/auth-server.log","log_level":-1,"message":"Logger initialized for auth_server"}
{"level":"info","timestamp":"2025-12-30T...","service":"auth_server","version":"1.0.0","environment":"development","db_url":"http://localhost:4001","message":"Database client initialized successfully"}
{"level":"info","timestamp":"2025-12-30T...","service":"auth_server","version":"1.0.0","environment":"development","address":":8080","message":"Starting HTTP server"}
```

## 5. Test the Server

```bash
# Test the health endpoint
curl -X GET http://localhost:8080/auth-server/v1/oauth/

# Response:
# ok
```

## Configuration Options

### Server Port
Change server port in config:
```json
"server_port": "9000"
```

### Logging Level
- `-1` = Debug (verbose)
- `0` = Info (default)
- `1` = Warn
- `2` = Error

### Development vs Production

**Development** (logs to stdout + file):
```json
"environment": "development",
"logging": {
  "level": -1,
  "path": "./logs/auth-server.log"
}
```

**Production** (logs to file only):
```json
"environment": "production",
"logging": {
  "level": 0,
  "path": "/var/log/auth-server/auth-server.log"
}
```

## Graceful Shutdown

The server gracefully shuts down on:
- `Ctrl+C` (SIGINT)
- Kill signal (SIGTERM)

Shutdown timeout: 30 seconds

## View Logs

```bash
# Real-time log tailing
tail -f logs/auth-server.log

# View recent entries
tail -20 logs/auth-server.log

# Search for errors
grep "ERROR" logs/auth-server.log
```

## Environment Variables

Optional: Use `.env` file instead of JSON config:

```bash
# Copy the example
cp .env.example .env

# Edit .env with your values
nano .env
```

## Troubleshooting

### Configuration file not found
The application will use defaults. Create `config/auth-server-config.json` for custom values.

### Port already in use
Change `server_port` in the configuration file to an available port.

### Database connection failed
Ensure rqlite is running on `localhost:4001` or update `database.host` and `database.port` in config.

### Logs not appearing
Check `logging.path` in configuration. The logs directory will be created automatically.

## Documentation

- Full documentation: See [README.md](README.md)
- Implementation details: See [IMPLEMENTATION_SUMMARY.md](IMPLEMENTATION_SUMMARY.md)
- Configuration guide: See [README.md#configuration-file](README.md#configuration-file-configauth-server-configjson)
