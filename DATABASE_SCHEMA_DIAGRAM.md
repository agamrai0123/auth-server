# Database Schema Diagram & Analysis

## 1. Current Database Schema

### ER Diagram (Entity Relationship)

```
┌─────────────────────────────────────────────────────────────────┐
│                         DATABASE SCHEMA                         │
│                                                                 │
│  ┌──────────────────────┐         ┌──────────────────────┐    │
│  │     CLIENTS TABLE    │         │     TOKENS TABLE     │    │
│  ├──────────────────────┤         ├──────────────────────┤    │
│  │ PK client_id (str)   │◄───┐    │ PK token_id (str)    │    │
│  │ client_secret (str)  │    │ FK │ FK client_id (str)   │    │
│  │ name (str)           │    │    │ issued_at (datetime) │    │
│  │ access_token_ttl (int)├────┘    │ expires_at (datetime)│    │
│  │ allowed_scopes (json)│         │ revoked (boolean)    │    │
│  │ created_at (datetime)│         │ revoked_at (datetime)│    │
│  │ updated_at (datetime)│         │ created_at (datetime)│    │
│  └──────────────────────┘         └──────────────────────┘    │
│                                                                 │
│  Relationship: One-to-Many                                    │
│  One Client can have Many Tokens                              │
│  Foreign Key: tokens.client_id → clients.client_id            │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## 2. Table Structure Details

### CLIENTS Table

```sql
CREATE TABLE clients (
  client_id          VARCHAR(255)    PRIMARY KEY,
  client_secret      VARCHAR(255)    NOT NULL,
  name               VARCHAR(255),
  access_token_ttl   INTEGER         DEFAULT 120,
  allowed_scopes     JSON            NOT NULL,
  created_at         DATETIME        DEFAULT CURRENT_TIMESTAMP,
  updated_at         DATETIME        DEFAULT CURRENT_TIMESTAMP
);
```

**Indexes:**
```sql
CREATE INDEX idx_clients_id ON clients(client_id);
CREATE INDEX idx_clients_name ON clients(name);
```

**Sample Data:**
```json
{
  "client_id": "service-auth-a",
  "client_secret": "secret-hash-$2a$12$...",
  "name": "Auth Service A",
  "access_token_ttl": 120,
  "allowed_scopes": [
    "https://api.example.com/users",
    "https://api.example.com/data",
    "https://api.example.com/admin"
  ],
  "created_at": "2025-12-15T10:30:00Z",
  "updated_at": "2025-12-30T14:00:00Z"
}
```

### TOKENS Table

```sql
CREATE TABLE tokens (
  token_id          VARCHAR(255)    PRIMARY KEY,
  client_id         VARCHAR(255)    NOT NULL,
  issued_at         DATETIME        NOT NULL,
  expires_at        DATETIME        NOT NULL,
  revoked           BOOLEAN         DEFAULT FALSE,
  revoked_at        DATETIME        NULL,
  created_at        DATETIME        DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (client_id) REFERENCES clients(client_id)
);
```

**Indexes:**
```sql
CREATE INDEX idx_tokens_client_id ON tokens(client_id);
CREATE INDEX idx_tokens_expires_at ON tokens(expires_at);
CREATE INDEX idx_tokens_revoked ON tokens(revoked);
CREATE INDEX idx_tokens_created_at ON tokens(created_at);
```

**Sample Data:**
```json
{
  "token_id": "5f8d9c2a1e4b6f3a",
  "client_id": "service-auth-a",
  "issued_at": "2025-12-30T14:00:00Z",
  "expires_at": "2025-12-30T14:02:00Z",
  "revoked": false,
  "revoked_at": null,
  "created_at": "2025-12-30T14:00:00Z"
}
```

---

## 3. Data Flow Through System

```
┌──────────────────────────────────────────────────────────────┐
│                    REQUEST FLOW                              │
└──────────────────────────────────────────────────────────────┘

TOKEN GENERATION REQUEST:
  ↓
Client sends: {client_id, client_secret}
  ↓
Handler queries: SELECT * FROM clients WHERE client_id = ?
  ↓
Parse client_secret + allowed_scopes from result
  ↓
Validate credentials
  ↓
Generate JWT with scopes
  ↓
INSERT into tokens table: {token_id, client_id, issued_at, expires_at}
  ↓
Return: access_token + expiration
  ↓
Database now has token for validation


TOKEN VALIDATION REQUEST:
  ↓
Client sends: Authorization header + X-Forwarded-For header
  ↓
Extract token from header
  ↓
Verify JWT signature (no DB required, uses secret from config)
  ↓
Query: SELECT revoked FROM tokens WHERE token_id = ?
  ↓
Check: token not revoked AND not expired
  ↓
Return: 200 OK + scopes OR 401/403 error


TOKEN REVOCATION REQUEST:
  ↓
Client sends: Authorization header
  ↓
Verify JWT valid
  ↓
UPDATE tokens SET revoked=true, revoked_at=NOW() WHERE token_id = ?
  ↓
Return: 200 OK revocation confirmed
```

---

## 4. Query Patterns & Performance

### High-Frequency Queries

```
Query 1: Get Client by ID (During Token Generation)
  SQL: SELECT * FROM clients WHERE client_id = ?
  Frequency: ~100K per second in peak load
  Latency: 50-100µs (database bound)
  Cacheable: YES (client info rarely changes)
  
Query 2: Check Token Revoked (During Token Validation)
  SQL: SELECT revoked FROM tokens WHERE token_id = ?
  Frequency: ~1.3M per second in peak load
  Latency: 1-10µs (memory bound after JWT validation)
  Cacheable: YES (tokens expire anyway)
  
Query 3: Insert Token (During Generation)
  SQL: INSERT INTO tokens (...)
  Frequency: ~100K per second in peak load
  Latency: 50-100µs (database bound)
  Cacheable: NO (must persist)

Query 4: Update Token Revocation (During Revocation)
  SQL: UPDATE tokens SET revoked=true, revoked_at=? WHERE token_id = ?
  Frequency: ~1-100 per second (rare)
  Latency: 50-100µs (database bound)
  Cacheable: NO (must persist)
```

### Query Optimization Opportunities

```
Current Bottleneck: clientByID() query
  ├─ Frequency: 100K/sec
  ├─ Latency: 50-100µs each
  ├─ Database time: 5-10 seconds per second of load
  └─ Solution: Cache with 5-10 minute TTL
  
Secondary Bottleneck: isTokenRevoked() query
  ├─ Frequency: 1.3M/sec (but JWT validation is faster)
  ├─ Actual queries: Only if JWT valid AND revocation check needed
  ├─ Real frequency: 10-100K/sec
  └─ Solution: Cache revocation with 1-5 minute TTL
```

---

## 5. Database Size Estimation

### Storage Requirements

```
CLIENTS Table:
  Per record: ~1KB (with JSON scopes)
  Expected records: 1,000-10,000 clients
  Storage: 1-10 MB
  Growth: Slow (clients don't change often)

TOKENS Table:
  Per record: ~500 bytes
  Tokens created per second: 100K (peak)
  Average token TTL: 2 minutes (120 seconds)
  Active tokens at any time: 100K × 120 = 12M tokens
  Storage: 12M × 500 bytes = ~6GB
  
  Recommendation:
    ├─ Archive tokens older than 7 days: -2GB
    ├─ Delete revoked tokens after 1 hour: -500MB
    ├─ Delete expired tokens after 24 hours: -1GB
    └─ Keep live: ~2.5GB maximum

Total Database Size: 3-5 GB (manageable)
```

### Cleanup Strategy

```sql
-- Run daily: Delete expired tokens older than 24 hours
DELETE FROM tokens 
WHERE expires_at < NOW() - INTERVAL '24 hours'
  AND revoked = false;

-- Run hourly: Delete revoked tokens older than 1 hour
DELETE FROM tokens
WHERE revoked = true
  AND revoked_at < NOW() - INTERVAL '1 hour';

-- Run weekly: Archive old audit records
INSERT INTO tokens_archive 
SELECT * FROM tokens 
WHERE created_at < NOW() - INTERVAL '7 days';

DELETE FROM tokens
WHERE created_at < NOW() - INTERVAL '7 days';
```

---

## 6. Connection Pool Configuration

### Current Setup

```go
// From database.go
db.SetMaxOpenConns(25)      // Max connections
db.SetMaxIdleConns(5)       // Idle connections
db.SetConnMaxLifetime(0)    // Connection lifetime
```

### Recommended Configuration

```go
// For 100K concurrent clients + 1M requests/sec
db.SetMaxOpenConns(50)      // Can handle 50 concurrent queries
db.SetMaxIdleConns(10)      // Keep 10 warm connections
db.SetConnMaxLifetime(time.Hour * 1)  // Refresh hourly
db.SetConnMaxIdleTime(time.Minute * 5) // Close idle after 5min
```

### Pool Performance

```
Current Config (25 max):
  ├─ Concurrent queries: ~25 at peak
  ├─ Queue time: 5-50ms under load
  ├─ Connection overhead: Acceptable
  └─ Bottleneck: Database itself (rqlite)

Recommended Config (50 max):
  ├─ Concurrent queries: ~50 at peak
  ├─ Queue time: <5ms under load
  ├─ Connection overhead: Acceptable
  └─ Benefit: Better performance with 5+ servers
```

---

## 7. Scaling Considerations

### Current Limitations

```
rqlite (SQLite-based):
  ├─ Write throughput: ~10K queries/second
  ├─ Read throughput: ~100K queries/second
  ├─ Single file: Serialized writes
  └─ Verdict: Bottleneck for 100K+ token generations/sec

Recommendation Timeline:
  ├─ Now: Implement caching (95% DB load reduction)
  ├─ Month 1: Stay with rqlite + caching
  ├─ Month 3: Migrate to PostgreSQL
  └─ Month 6: Multi-database replication for HA
```

### Migration Path to PostgreSQL

```
Step 1: Setup PostgreSQL replica
  ├─ Create PostgreSQL instance with same schema
  ├─ Replicate data from rqlite
  └─ Test with read-only queries

Step 2: Gradual migration
  ├─ Week 1: 10% write traffic to PostgreSQL
  ├─ Week 2: 25% write traffic to PostgreSQL
  ├─ Week 3: 50% write traffic to PostgreSQL
  ├─ Week 4: 100% write traffic to PostgreSQL

Step 3: Full cutover
  ├─ Stop using rqlite
  ├─ Keep rqlite as backup
  └─ Monitor PostgreSQL performance

PostgreSQL Benefits:
  ├─ Write throughput: 500K+ queries/second
  ├─ Read throughput: 1M+ queries/second
  ├─ Connection pooling: pgBouncer
  ├─ Replication: Built-in
  └─ Cost: Slightly higher infrastructure
```

---

## 8. Schema Evolution Strategy

### Planned Future Tables

```
AUDIT_LOG Table (Optional):
  CREATE TABLE audit_log (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    client_id VARCHAR(255),
    event_type VARCHAR(50),
    event_details JSON,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
  );
  
  Purpose: Track authentication events for compliance
  Frequency: ~1.3M per second
  Retention: 90 days
  Storage: Large (~100GB for high volume)
  
  Recommendation: Store in separate database or data warehouse

CLIENT_QUOTA Table (Optional):
  CREATE TABLE client_quota (
    client_id VARCHAR(255) PRIMARY KEY,
    requests_per_second INT,
    tokens_per_day INT,
    created_at DATETIME
  );
  
  Purpose: Rate limiting and quota enforcement
  Usage: Check before generating token
  Impact: Small overhead

SESSION_REVOCATION Table (Optional):
  CREATE TABLE session_revocation (
    revocation_id VARCHAR(255) PRIMARY KEY,
    client_id VARCHAR(255),
    revoked_all_tokens BOOLEAN,
    revoked_at DATETIME,
    reason VARCHAR(255)
  );
  
  Purpose: Batch revocation (revoke all tokens for a client)
  Usage: During security incidents
  Impact: Reduces need to revoke individually
```

---

## 9. Backup & Recovery Strategy

### Backup Frequency

```
rqlite (current):
  ├─ Full backup: Daily at 2 AM
  ├─ Incremental: Every 6 hours
  ├─ Transaction logs: Continuous
  └─ Retention: 30 days

PostgreSQL (future):
  ├─ Full backup: Daily
  ├─ WAL (Write-Ahead Logging): Continuous
  ├─ Point-in-time recovery: Yes
  └─ Retention: 30 days
```

### Disaster Recovery RTO/RPO

```
Current SLA:
  ├─ RTO (Recovery Time Objective): 15 minutes
  ├─ RPO (Recovery Point Objective): 1 hour
  └─ Method: Restore from latest backup

Enhanced SLA (with caching):
  ├─ RTO (Recovery Time Objective): 5 minutes
  ├─ RPO (Recovery Point Objective): 15 minutes
  └─ Method: Cache serves during recovery
  
High Availability SLA (with replication):
  ├─ RTO (Recovery Time Objective): <1 minute
  ├─ RPO (Recovery Point Objective): 0 seconds
  └─ Method: Automatic failover to replica
```

---

## 10. Monitoring & Alerting

### Key Metrics to Monitor

```
CLIENTS Table:
  ├─ Row count: Should be stable (1K-10K)
  ├─ Size: Should grow slowly
  ├─ Query latency: Should be <100ms
  └─ Index efficiency: Should use index for lookups

TOKENS Table:
  ├─ Row count: 12M typical (varies with TTL)
  ├─ Size: 6GB typical
  ├─ Query latency: Should be <50ms
  ├─ Expired rows: Should be cleaned up regularly
  └─ Revoked rows: Should be cleaned up regularly

Database Performance:
  ├─ Connection count: Monitor pool usage
  ├─ Query queue time: Should be <10ms
  ├─ Disk I/O: Should not hit 100%
  ├─ Memory usage: Should not hit limits
  └─ CPU usage: Should not exceed 80%
```

### Alert Thresholds

```
CRITICAL (Page on-call):
  ├─ Database down: Immediate
  ├─ Query latency > 500ms: Immediate
  ├─ Connection pool exhausted: Immediate
  └─ Disk usage > 90%: Within 15 minutes

HIGH (Email alert):
  ├─ Query latency > 100ms: Within 1 hour
  ├─ Connection pool > 80% used: Within 1 hour
  ├─ Disk usage > 80%: Within 1 hour
  └─ Table bloat detected: Within 1 hour

MEDIUM (Dashboard):
  ├─ Slow queries detected: Monitor
  ├─ Missing indexes: Monitor
  └─ Query performance degradation: Monitor
```

---

## Summary

**Current Schema**: Simple, normalized (2 tables, 1-to-many relationship)  
**Current Database**: rqlite (SQLite), suitable for development/small production  
**Current Bottleneck**: Database write throughput (10K/sec vs 100K+ needed)  
**Immediate Solution**: Implement caching (95% load reduction)  
**Long-term Solution**: Migrate to PostgreSQL (500K+ throughput)  

**Action Items**:
1. ✅ Current schema is well-designed
2. ⚠️ Add caching layer (implement within 1 month)
3. 📅 Plan PostgreSQL migration (implement within 3 months)
4. 🔄 Implement token cleanup (reduce storage bloat)
5. 📊 Setup monitoring and alerting (immediate)
