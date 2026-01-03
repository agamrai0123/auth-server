# Windows Oracle Setup Guide

## The Issue

The Oracle godror driver requires C compilation support. On Windows, you need either:
1. MSVC (Microsoft Visual C++) compiler
2. GCC compiler (MinGW)
3. Or run in WSL2 with Linux environment

## Recommended Solutions

### Option 1: Use WSL2 (Recommended for Windows)

```bash
# 1. Install WSL2 and Ubuntu
# (See: https://learn.microsoft.com/en-us/windows/wsl/install)

# 2. Open WSL2 terminal and run
wsl --install

# 3. Install Go in WSL2
sudo apt update
sudo apt install golang-go

# 4. Install Docker in WSL2
sudo apt install docker.io

# 5. Clone/navigate to project
cd /mnt/d/work-projects/auth-server

# 6. Continue with normal build
go get github.com/godror/godror
go build -o auth-server main.go
```

### Option 2: Install MSVC on Windows

#### Step 1: Download Visual Studio Build Tools
```
https://visualstudio.microsoft.com/visual-cpp-build-tools/
```

#### Step 2: Run the installer
- Select "Desktop development with C++"
- Install C++ build tools

#### Step 3: Set environment variables
```powershell
# In PowerShell (Admin)
setx INCLUDE "C:\Program Files\Microsoft Visual Studio\2022\BuildTools\VC\Tools\MSVC\14.39.33519\include"
setx LIB "C:\Program Files\Microsoft Visual Studio\2022\BuildTools\VC\Tools\MSVC\14.39.33519\lib\x64"
```

#### Step 4: Rebuild
```powershell
cd d:\work-projects\auth-server
go build -o auth-server main.go
```

### Option 3: Use Docker for the Entire App

Since you need Oracle in Docker anyway, run your app in Docker too:

#### Create Dockerfile
```dockerfile
FROM golang:1.21-bullseye

WORKDIR /app
COPY . .

RUN go mod download
RUN go get github.com/godror/godror
RUN go build -o auth-server main.go

EXPOSE 8080

CMD ["./auth-server"]
```

#### Run with Docker Compose
```bash
docker-compose -f docker-compose-full.yml up -d
```

### Option 4: Alternative: SQLite with docker-compose

If Oracle setup is too complex, use SQLite in Docker:

```yaml
version: '3.8'
services:
  sqlite-db:
    image: keinos/sqlite3:latest
    ports:
      - "8765:8000"
    volumes:
      - sqlite-data:/data
  
volumes:
  sqlite-data:
```

But we recommend Oracle for production.

## Quick Start - Assuming You Have C Tools

```bash
# 1. Verify compiler is available
gcc --version
# or
cl.exe  # For MSVC

# 2. Install Oracle driver
go get github.com/godror/godror

# 3. Build app
go build -o auth-server main.go

# 4. Start Oracle in Docker
docker-compose up -d

# 5. Run app (set connection string first)
set DB_URL=oracle://sys:Oracle123!@localhost:1521/XE
go run main.go

# 6. Build load test
go build -o load-test load-test.go

# 7. Run load test
.\load-test -concurrency=10 -requests=100
```

## If You Get cgo Errors

The full error message usually indicates missing C headers. To see the actual error:

```bash
go build -v -x main.go 2>&1 | tail -100
```

Look for paths like:
- `cl.exe not found` → Need MSVC
- `gcc not found` → Need GCC/MinGW
- `oracle.h not found` → Need Oracle C libraries

## Docker-Only Approach (No Windows Build Tools Needed)

```bash
# Just run Docker - build happens inside
docker-compose up -d

# Then access via the container network
# Don't need to build on Windows at all
```

## Verification Without Compilation

You can still verify the schema and database:

```bash
# Start Oracle
docker-compose up -d

# Wait for it to be healthy
docker-compose ps

# Connect and verify
docker exec -it oracle-auth-db sqlplus sys/Oracle123!@localhost:1521/XE as sysdba

# Inside SQL*Plus
select count(*) from clients;
select count(*) from tokens;
exit;
```

## Next Steps

Choose one of these approaches:

1. **WSL2** - Most recommended
   - Seamless Linux environment
   - Full Docker support
   - No Windows tool conflicts

2. **MSVC** - For Windows native
   - Download Visual Studio Build Tools
   - ~3 GB installation
   - Works with native Go tools

3. **Docker Container** - For complete isolation
   - Run everything in Docker
   - No local dependencies
   - Easiest for CI/CD

4. **Verify Database Only** - Start with this
   - Ensure Oracle works in Docker
   - Test manually with SQL*Plus
   - Plan build approach after

Pick the approach that fits your environment best!

