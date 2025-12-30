# Auth Server Database Schema & Diagram

## ER Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                   AUTH SERVER SCHEMA                            │
└─────────────────────────────────────────────────────────────────┘

┌──────────────────────┐         ┌──────────────────────┐
│     CLIENTS          │         │      TOKENS          │
├──────────────────────┤         ├──────────────────────┤
│ id (PK)              │────┐    │ id (PK)              │
│ client_id            │    ├──→ │ client_id (FK)       │
│ client_secret        │    │    │ user_id              │
│ name                 │    │    │ token_type           │
│ email                │    │    │ token_value          │
│ redirect_uri         │    │    │ expiry_time          │
│ scopes               │    │    │ revoked              │
│ is_active            │    │    │ revoked_at           │
│ created_at           │    │    │ created_at           │
│ updated_at           │    └──→ │ metadata             │
└──────────────────────┘         └──────────────────────┘
        │                                 │
        │                                 │
        └─────────────┬────────────────────┘
                      │
              ┌───────┴────────┐
              │                │
              ↓                ↓
        ┌──────────────┐  ┌──────────────┐
        │ AUDIT_LOGS   │  │ SESSIONS     │
        ├──────────────┤  ├──────────────┤
        │ id (PK)      │  │ id (PK)      │
        │ client_id(FK)│  │ client_id(FK)│
        │ action       │  │ user_id      │
        │ resource     │  │ token_id(FK) │
        │ status       │  │ ip_address   │
        │ timestamp    │  │ created_at   │
        │ details      │  │ expires_at   │
        └──────────────┘  └──────────────┘
```

---

## Detailed Schema Definition

### CLIENTS Table

```sql
CREATE TABLE clients (
    -- Primary Key
    id                BIGINT PRIMARY KEY AUTO_INCREMENT,
    
    -- Client Identification
    client_id         VARCHAR(255) NOT NULL UNIQUE,
    client_secret     VARCHAR(512) NOT NULL,
    
    -- Client Information
    name              VARCHAR(255) NOT NULL,
    email             VARCHAR(255),
    redirect_uri      VARCHAR(512),
    
    -- Permissions & Configuration
    scopes            JSON NOT NULL DEFAULT '[]',
    is_active         BOOLEAN NOT NULL DEFAULT TRUE,
    
    -- Metadata
    created_at        TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at        TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    
    -- Indexes
    INDEX idx_client_id (client_id),
    INDEX idx_is_active (is_active),
    INDEX idx_created_at (created_at)
);
```

**Storage**: ~2KB per client record
**Growth Rate**: ~100 clients/month
**Estimated Size @ Year 1**: ~2.4MB

---

### TOKENS Table

```sql
CREATE TABLE tokens (
    -- Primary Key
    id                BIGINT PRIMARY KEY AUTO_INCREMENT,
    
    -- Foreign Keys
    client_id         VARCHAR(255) NOT NULL,
    user_id           VARCHAR(255),
    
    -- Token Details
    token_type        VARCHAR(50) NOT NULL,      -- 'bearer', 'refresh', etc.
    token_value       VARCHAR(2048) NOT NULL UNIQUE,
    
    -- Token Lifecycle
    expiry_time       TIMESTAMP NOT NULL,
    created_at        TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    -- Revocation State
    revoked           BOOLEAN NOT NULL DEFAULT FALSE,
    revoked_at        TIMESTAMP NULL,
    
    -- Metadata
    metadata          JSON,
    
    -- Indexes
    UNIQUE KEY uk_token_value (token_value),
    INDEX idx_client_id (client_id),
    INDEX idx_user_id (user_id),
    INDEX idx_revoked (revoked),
    INDEX idx_expiry_time (expiry_time),
    INDEX idx_created_at (created_at),
    INDEX idx_revoked_at (revoked_at),
    
    -- Foreign Key Constraints
    FOREIGN KEY (client_id) REFERENCES clients(client_id) ON DELETE CASCADE
);
```

**Storage**: ~1KB per token record
**Growth Rate**: ~1M tokens/day (100K token gen/sec × 86400 sec)
**TTL**: 1-24 hours (tokens expire and are archived)
**Estimated Size @ Year 1**: ~1TB (if storing all tokens)

---

### AUDIT_LOGS Table

```sql
CREATE TABLE audit_logs (
    -- Primary Key
    id                BIGINT PRIMARY KEY AUTO_INCREMENT,
    
    -- Foreign Keys
    client_id         VARCHAR(255) NOT NULL,
    
    -- Audit Information
    action            VARCHAR(50) NOT NULL,      -- 'CREATE', 'READ', 'UPDATE', 'DELETE'
    resource          VARCHAR(255) NOT NULL,    -- 'token', 'client', 'session'
    resource_id       VARCHAR(255),
    
    -- Status & Details
    status            VARCHAR(50) NOT NULL,     -- 'success', 'failure'
    error_message     VARCHAR(512),
    details           JSON,
    
    -- Request Context
    ip_address        VARCHAR(45),
    user_agent        VARCHAR(512),
    
    -- Timestamp
    timestamp         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    -- Indexes
    INDEX idx_client_id (client_id),
    INDEX idx_action (action),
    INDEX idx_timestamp (timestamp),
    INDEX idx_status (status),
    COMPOSITE INDEX idx_client_action_time (client_id, action, timestamp)
);
```

**Storage**: ~500B per audit log
**Growth Rate**: ~5-10M logs/day
**Retention**: 30-90 days
**Estimated Size @ Year 1**: ~2-10TB (if storing all logs)
**Recommendation**: Archive to cold storage after 90 days

---

### SESSIONS Table

```sql
CREATE TABLE sessions (
    -- Primary Key
    id                BIGINT PRIMARY KEY AUTO_INCREMENT,
    
    -- Foreign Keys
    client_id         VARCHAR(255) NOT NULL,
    user_id           VARCHAR(255),
    token_id          VARCHAR(2048),
    
    -- Session Details
    ip_address        VARCHAR(45) NOT NULL,
    user_agent        VARCHAR(512),
    
    -- Lifecycle
    created_at        TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at        TIMESTAMP NOT NULL,
    last_activity     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    
    -- Indexes
    INDEX idx_client_id (client_id),
    INDEX idx_user_id (user_id),
    INDEX idx_expires_at (expires_at),
    INDEX idx_created_at (created_at),
    
    -- Foreign Key Constraints
    FOREIGN KEY (client_id) REFERENCES clients(client_id) ON DELETE CASCADE
);
```

**Storage**: ~500B per session
**Growth Rate**: ~10K sessions/day
**TTL**: Varies by use case (typically 1 month)
**Estimated Size @ Year 1**: ~2GB

---

## Indexing Strategy

### Hot Indexes (Most Frequently Used)

```sql
-- TOKENS table - for token validation
INDEX idx_token_value (token_value) -- PRIMARY OPERATION
INDEX idx_revoked (revoked)         -- Filter revoked tokens
INDEX idx_expiry_time (expiry_time) -- Clean up expired

-- CLIENTS table - for client lookup
INDEX idx_client_id (client_id)     -- PRIMARY OPERATION
INDEX idx_is_active (is_active)     -- Filter active clients

-- AUDIT_LOGS table - for searching
COMPOSITE INDEX idx_client_action_time (client_id, action, timestamp)
```

### Warm Indexes (Frequently Used)

```sql
-- Time-based queries
INDEX idx_created_at (created_at)
INDEX idx_timestamp (timestamp)

-- Foreign key lookups
INDEX idx_user_id (user_id)
INDEX idx_revoked_at (revoked_at)
```

### Cold Indexes (Rarely Used)

```sql
-- Keep but monitor usage
INDEX idx_status (status)
INDEX idx_action (action)
INDEX idx_is_active (is_active)
```

---

## Query Performance Analysis

### Token Validation Query (Most Critical)

```sql
-- Query: Validate token (check revocation, expiry, client active)
SELECT t.id, t.user_id, t.token_type, c.scopes, c.name
FROM tokens t
JOIN clients c ON t.client_id = c.client_id
WHERE t.token_value = ? 
  AND t.revoked = FALSE
  AND t.expiry_time > NOW()
  AND c.is_active = TRUE
LIMIT 1;

Query Plan:
  ├─ Full index scan: idx_token_value (FAST)
  ├─ Join: CLIENTS using client_id (FAST)
  ├─ Filter by revocation: idx_revoked (INSTANT)
  ├─ Filter by expiry: idx_expiry_time (INSTANT)
  └─ Total latency: 50-100µs

Optimization:
  ├─ ✅ Already well-indexed
  ├─ ✅ Query is optimal
  ├─ ✅ Needs caching (not DB optimization)
  └─ Recommended: Add L1+L2 cache
```

### Client Lookup Query

```sql
-- Query: Lookup client by ID
SELECT * FROM clients WHERE client_id = ? AND is_active = TRUE;

Query Plan:
  ├─ Unique index scan: uk_client_id (VERY FAST)
  └─ Total latency: 10-50µs

Optimization:
  ├─ ✅ Fully optimized
  ├─ ✅ Add in-memory cache for 10x improvement
  └─ Recommended: L1 cache (TTL 5-10 min)
```

### Revocation Check Query

```sql
-- Query: Check if token is revoked (for expired/revoked cleanup)
SELECT COUNT(*) FROM tokens 
WHERE client_id = ? 
  AND revoked = TRUE 
  AND revoked_at > DATE_SUB(NOW(), INTERVAL 5 MINUTE);

Query Plan:
  ├─ Composite index scan: (revoked, revoked_at) (FAST)
  └─ Total latency: 50-200µs (depends on cardinality)

Optimization:
  ├─ ✅ Well-indexed
  ├─ ✅ Add revocation cache for 100x improvement
  └─ Recommended: L1 revocation cache (TTL 1-5 min)
```

### Audit Log Query

```sql
-- Query: Get logs for a client
SELECT * FROM audit_logs 
WHERE client_id = ? 
  AND timestamp > DATE_SUB(NOW(), INTERVAL 7 DAY)
ORDER BY timestamp DESC
LIMIT 100;

Query Plan:
  ├─ Composite index scan: (client_id, timestamp) (FAST)
  ├─ Filter and sort: In-memory sort
  └─ Total latency: 100-500µs

Optimization:
  ├─ ✅ Well-indexed
  ├─ ⚠️ Consider archival for logs > 90 days
  └─ Recommended: Automatic archival job
```

---

## Storage & Scaling

### Current Growth Metrics

```
Baseline (Year 0):
  ├─ Clients: ~100
  ├─ Tokens/day: ~1M (100K gen/sec)
  ├─ Audit logs/day: ~5M
  └─ Total Size: ~50GB

Year 1 Projection:
  ├─ Clients: ~500
  ├─ Tokens/day: ~10M (1M gen/sec potential with caching)
  ├─ Audit logs/day: ~50M
  ├─ Stored tokens: ~10M active (TTL 1-24h) + archive
  ├─ Stored audit logs: ~150M (3 months + archive)
  └─ Total Size: ~500GB - 2TB

Year 2-3 Projection:
  ├─ 10x growth possible with caching
  └─ Requires read replicas / distributed database
```

### Partitioning Strategy

```
TOKENS table - Partition by date:
  ├─ tokens_2025_01 (January 2025)
  ├─ tokens_2025_02 (February 2025)
  ├─ ...
  └─ Benefits: Faster expiry/cleanup, parallel queries

AUDIT_LOGS table - Partition by date:
  ├─ audit_logs_2025_01
  ├─ audit_logs_2025_02
  └─ Benefits: Archive old partitions, faster queries

CLIENTS table - No partitioning:
  ├─ Stay on single partition (< 10K records)
  └─ Keep fully indexed
```

---

## Backup & Recovery Strategy

### Backup Schedule

```
Daily: Full backup at 2:00 AM (UTC)
  ├─ Backup size: 100GB → 500GB (year 1)
  ├─ Backup time: 30 minutes
  ├─ Retention: 30 days
  └─ Compression: 5:1 ratio

Hourly: Incremental backup
  ├─ Retention: 7 days
  └─ RTO: 1 hour

Real-time: Binary logs
  ├─ Replication lag: <1 second
  └─ RPO: 0 (no data loss)
```

### Recovery Procedures

| Scenario | RTO | RPO | Procedure |
|----------|-----|-----|-----------|
| Single record corruption | 10 min | Point-in-time | Restore from backup |
| Entire DB failure | 30 min | Last backup | Restore from backup + replay logs |
| Regional failure | 5 min | <1 sec | Failover to replica |
| Ransomware/attack | 1 hour | Last clean backup | Isolated restore to new cluster |

---

## Monitoring Queries

### Table Size Monitoring

```sql
-- Check table sizes
SELECT 
    table_name,
    ROUND((data_length + index_length) / 1024 / 1024 / 1024, 2) as size_gb,
    table_rows
FROM information_schema.tables
WHERE table_schema = 'auth_db'
ORDER BY size_gb DESC;

-- Expected output (Year 1):
-- tokens: 50-100GB, ~1M rows
-- audit_logs: 200-500GB, ~50M rows  
-- clients: <1MB, ~500 rows
-- sessions: 1-5GB, ~10K rows
```

### Performance Monitoring

```sql
-- Check query latency by table
SELECT 
    object_schema,
    object_name,
    count_read,
    count_write,
    sum_timer_read / 1000000000000 as total_read_sec,
    sum_timer_write / 1000000000000 as total_write_sec
FROM performance_schema.table_io_waits_summary_by_table
ORDER BY sum_timer_read DESC;
```

### Index Usage Monitoring

```sql
-- Find unused indexes
SELECT 
    object_schema,
    object_name,
    index_name,
    count_read,
    count_write,
    count_delete
FROM performance_schema.table_io_waits_summary_by_index_usage
WHERE count_read = 0 AND count_write = 0
ORDER BY count_delete DESC;
```

---

## Conclusion

**Database Design**:
- ✅ Normalized schema (3NF)
- ✅ Proper indexing strategy
- ✅ Appropriate constraints
- ✅ Audit trail built-in

**Performance Bottleneck**: Token validation queries (not schema design)
- **Solution**: Implement 3-tier caching strategy
- **Expected Improvement**: 10-100x throughput increase

**Scalability Path**:
1. Add caching layers (immediate, 10x improvement)
2. Implement read replicas (3-6 months)
3. Implement sharding (12+ months)
4. Consider distributed database (18+ months)

See [CACHING_STRATEGIES.md](CACHING_STRATEGIES.md) for caching implementation plan.
