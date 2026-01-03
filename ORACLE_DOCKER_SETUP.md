# Oracle Database Docker Setup Guide

## Prerequisites

- Docker Desktop installed and running
- Docker Compose installed (usually included with Docker Desktop)
- Go 1.21+ installed
- 4GB RAM available for Oracle database container

## Step 1: Pull Oracle Database Image (Optional)

The docker-compose.yml will automatically pull the image, but you can pre-pull it:

```bash
docker pull container-registry.oracle.com/database/express:21.3.0
```

## Step 2: Start Oracle Database in Docker

Navigate to the project root directory and run:

```bash
# Start Oracle database container
docker-compose up -d

# Check status
docker-compose ps
```

Output should show:
```
NAME                    STATUS
oracle-auth-db          Up (healthy)
```

## Step 3: Wait for Database to Be Ready

The container has a health check. Wait for the status to show "healthy":

```bash
# Monitor health status
docker-compose ps

# Or check logs
docker-compose logs oracle-db
```

The database is ready when you see:
```
Oracle Database initialized and ready to accept connections
```

This typically takes 2-3 minutes on first startup.

## Step 4: Verify Database Connection

Test the connection using SQL*Plus (if installed) or through Go:

```bash
# Option A: Using sqlplus (if installed)
sqlplus sys/Oracle123!@localhost:1521/XE as sysdba

# Option B: Using the Go app (it will auto-connect)
go run main.go
```

## Database Connection String

The connection string used in the Go app:

```
oracle://sys:Oracle123!@localhost:1521/XE
```

Format: `oracle://username:password@host:port/service_name`

**Default credentials for Oracle Express Edition:**
- Username: `sys`
- Password: `Oracle123!`
- Port: `1521`
- SID: `XE`

## Step 5: Update Environment Variable

In your application config or environment, set:

```bash
# Linux/Mac
export DB_URL="oracle://sys:Oracle123!@localhost:1521/XE"

# Windows PowerShell
$env:DB_URL = "oracle://sys:Oracle123!@localhost:1521/XE"

# Windows CMD
set DB_URL=oracle://sys:Oracle123!@localhost:1521/XE
```

## Step 6: Initialize Database (Optional)

The `init-db.sql` script is automatically executed when the container starts. It creates:

- CLIENTS table (client credentials)
- TOKENS table (issued tokens)
- REVOKED_TOKENS table (revoked tokens)
- ENDPOINTS table (API endpoints)
- Indexes for performance
- Sample test data (3 test clients)

## Testing Database Connection

```go
package main

import (
	"database/sql"
	_ "github.com/godror/godror"
)

func main() {
	db, err := sql.Open("oracle", "oracle://sys:Oracle123!@localhost:1521/XE")
	if err != nil {
		panic(err)
	}
	defer db.Close()
	
	err = db.Ping()
	if err != nil {
		panic(err)
	}
	
	println("Connected to Oracle database!")
}
```

## Common Commands

### View Database Logs
```bash
docker-compose logs -f oracle-db
```

### Access Oracle Database Shell
```bash
docker exec -it oracle-auth-db sqlplus sys/Oracle123!@localhost:1521/XE as sysdba
```

### Stop Database
```bash
docker-compose down
```

### Stop Database and Remove Data (Full Reset)
```bash
docker-compose down -v
```

### View Tables in Database
```bash
docker exec -it oracle-auth-db sqlplus sys/Oracle123!@localhost:1521/XE as sysdba << EOF
SELECT table_name FROM user_tables;
EXIT;
EOF
```

### Check Available Test Data
```bash
docker exec -it oracle-auth-db sqlplus sys/Oracle123!@localhost:1521/XE as sysdba << EOF
SELECT * FROM clients;
EXIT;
EOF
```

## Sample Test Data Inserted

Three test clients are automatically inserted:

```
1. test-client-1
   Secret: secret-key-12345
   Scopes: ["/api/users", "/api/posts"]

2. test-client-2
   Secret: secret-key-67890
   Scopes: ["/api/admin", "/api/reports"]

3. mobile-app
   Secret: mobile-secret-key
   Scopes: ["/api/auth", "/api/profile"]
```

## Troubleshooting

### Container Won't Start
```bash
# Check logs
docker-compose logs oracle-db

# Restart containers
docker-compose restart

# Clean restart (removes old container)
docker-compose down
docker-compose up -d
```

### Port Already in Use
If port 1521 is already in use:

1. Edit docker-compose.yml
2. Change `"1521:1521"` to `"1522:1521"` (or another available port)
3. Update connection string in app: `oracle://sys:Oracle123!@localhost:1522/XE`

### Memory Issues
If Docker container runs out of memory:

1. Increase Docker Desktop memory limit
2. Or reduce Oracle SGA size (advanced)

### Health Check Failing
The health check may take 3-5 minutes to pass on first startup. This is normal.

## Database Schema

### CLIENTS Table
```
client_id (VARCHAR2, PRIMARY KEY)
client_secret (VARCHAR2, NOT NULL)
client_name (VARCHAR2)
access_token_ttl (NUMBER, DEFAULT 3600)
allowed_scopes (CLOB) - JSON array
created_at (TIMESTAMP)
updated_at (TIMESTAMP)
active (NUMBER, DEFAULT 1)
```

### TOKENS Table
```
token_id (VARCHAR2, PRIMARY KEY)
client_id (VARCHAR2, FOREIGN KEY)
issued_at (TIMESTAMP)
expires_at (TIMESTAMP)
revoked (NUMBER, DEFAULT 0)
revoked_at (TIMESTAMP)
created_at (TIMESTAMP)
```

### REVOKED_TOKENS Table
```
id (NUMBER, AUTO INCREMENT PRIMARY KEY)
token_id (VARCHAR2)
client_id (VARCHAR2, FOREIGN KEY)
revoked_at (TIMESTAMP)
```

### ENDPOINTS Table
```
id (NUMBER, AUTO INCREMENT PRIMARY KEY)
client_id (VARCHAR2, FOREIGN KEY)
scope (VARCHAR2)
method (VARCHAR2)
endpoint_url (VARCHAR2)
description (VARCHAR2)
active (NUMBER, DEFAULT 1)
created_at (TIMESTAMP)
```

## Build and Run Application

```bash
# Install Oracle driver
go get github.com/godror/godror

# Download dependencies
go mod download

# Build
go build -o auth-server main.go

# Run
./auth-server
```

The application will automatically connect to the Oracle database using the connection string from the environment variable or config file.

## Docker Cleanup

### Remove Unused Images
```bash
docker image prune
```

### Remove All Oracle Containers
```bash
docker ps -a | grep oracle-auth-db | awk '{print $1}' | xargs docker rm
```

### Remove Oracle Volumes
```bash
docker volume rm auth-server_oracle-data
```

## Performance Notes

The database is configured with:
- Connection pool: Max 25 connections, 5 idle connections
- Indexes on: client_id, expires_at, revoked, token_id
- Transaction support for atomic batch operations
- Context timeouts: 5-10 seconds per operation

These settings are optimized for the auth server workload.

