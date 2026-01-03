# Use official Go image with build tools
FROM golang:1.23-bullseye AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o auth-server main.go

# Final stage - runtime with Oracle client
FROM debian:bullseye-slim

RUN apt-get update && apt-get install -y \
    ca-certificates \
    libglib2.0-0 \
    libaio1 \
    curl \
    wget \
    && rm -rf /var/lib/apt/lists/*

# Download and install Oracle Instant Client
RUN cd /tmp && \
    wget -q https://download.oracle.com/otn_software/linux/instantclient/instantclient-basiclite-linuxx64.zip && \
    unzip -q instantclient-basiclite-linuxx64.zip && \
    mkdir -p /opt/oracle && \
    mv instantclient_* /opt/oracle/instantclient && \
    echo /opt/oracle/instantclient > /etc/ld.so.conf.d/oracle.conf && \
    ldconfig && \
    rm -f /tmp/instantclient-basiclite-linuxx64.zip

ENV LD_LIBRARY_PATH=/opt/oracle/instantclient:$LD_LIBRARY_PATH

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/auth-server .

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=10s --timeout=5s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

# Run the application
CMD ["./auth-server"]

