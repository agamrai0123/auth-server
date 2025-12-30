# Caching Strategies for Auth Server

## Executive Summary

**Problem**: Database is bottleneck for token generation (100K req/sec vs 10K DB writes/sec)  
**Solution**: Implement 3-tier caching strategy  
**Expected Impact**: 95% reduction in database load, 10x throughput increase  
**Implementation Effort**: 1-2 weeks  
**ROI**: Delays database migration by 3+ months  

---

## 1. Three-Tier Caching Strategy

```
┌─────────────────────────────────────────────────────────────────┐
│                    CACHING ARCHITECTURE                         │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌──────────────┐                                              │
│  │ L1 Cache     │  Application Memory (In-Process)            │
│  │ - Clients    │  TTL: 5-10 minutes                           │
│  │ - Revocation │  Size: <100MB                                │
│  │   List       │  Speed: <1µs                                 │
│  └────────┬─────┘                                              │
│           │ Miss                                                │
│           ↓                                                     │
│  ┌──────────────┐                                              │
│  │ L2 Cache     │  Redis (Distributed)                         │
│  │ - Clients    │  TTL: 5-10 minutes                           │
│  │ - Revocation │  Size: Unlimited                             │
│  │   List       │  Speed: <5ms                                 │
│  └────────┬─────┘                                              │
│           │ Miss                                                │
│           ↓                                                     │
│  ┌──────────────┐                                              │
│  │ L3 Cache     │  Database (Primary)                          │
│  │ - Clients    │  Authoritative source                        │
│  │ - Revocation │  Speed: 50-100µs                             │
│  │   List       │  Write-through required                      │
│  └──────────────┘                                              │
│                                                                 │
│  Request → L1 → L2 → L3 → Response                             │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## 2. Cache Layer 1: In-Process (Application Memory)

### Implementation Pattern

```go
type CachedClient struct {
    Client    *Client
    ExpiresAt time.Time
}

type ClientCache struct {
    mu       sync.RWMutex
    cache    map[string]*CachedClient
    maxSize  int
    ttl      time.Duration
}

func (cc *ClientCache) Get(clientID string) (*Client, error) {
    cc.mu.RLock()
    defer cc.mu.RUnlock()
    
    cached, exists := cc.cache[clientID]
    if !exists {
        return nil, ErrCacheMiss
    }
    
    if time.Now().After(cached.ExpiresAt) {
        // Expired, let it be evicted on next write
        return nil, ErrCacheMiss
    }
    
    return cached.Client, nil
}

func (cc *ClientCache) Set(clientID string, client *Client) {
    cc.mu.Lock()
    defer cc.mu.Unlock()
    
    if len(cc.cache) >= cc.maxSize {
        // Evict oldest entry (simplified LRU)
        cc.evictOldest()
    }
    
    cc.cache[clientID] = &CachedClient{
        Client:    client,
        ExpiresAt: time.Now().Add(cc.ttl),
    }
}
```

### Configuration

```go
// In config.go
type CacheConfig struct {
    ClientCacheTTL      time.Duration  // 5-10 minutes
    ClientCacheMaxSize  int            // 1000-10000
    RevocationCacheTTL  time.Duration  // 1-5 minutes
    RevocationCacheSize int            // 10000-100000
}

// Default values
ClientCacheTTL: 10 * time.Minute
ClientCacheMaxSize: 5000
RevocationCacheTTL: 5 * time.Minute
RevocationCacheSize: 50000
```

### Performance Impact

```
Without Cache:
  ├─ clientByID() latency: 50-100µs (database)
  ├─ Max throughput: 100K token gen/sec (DB limited)
  └─ Database load: 100% of token gen requests

With L1 Cache (5-10 min TTL):
  ├─ clientByID() latency: <1µs (memory hit)
  ├─ Hit rate: 99%+ (same clients request repeatedly)
  ├─ Max throughput: 1M+ token gen/sec
  └─ Database load: <1% of requests (only on miss + TTL expiry)
```

### Trade-offs

```
Pros:
  ✅ Ultra-fast (sub-microsecond)
  ✅ No network latency
  ✅ No external dependencies
  ✅ Simple to implement

Cons:
  ⚠️ Not shared between instances
  ⚠️ Memory overhead (up to 100MB)
  ⚠️ Stale data for 5-10 minutes after change
  ⚠️ Requires manual invalidation on updates

Use When:
  ├─ Single server deployment
  ├─ Client changes are rare
  ├─ Acceptable staleness: 5-10 minutes
  └─ Memory is abundant
```

---

## 3. Cache Layer 2: Distributed (Redis)

### Implementation Pattern

```go
import "github.com/redis/go-redis/v9"

type RedisCacheClient struct {
    redis *redis.Client
    ttl   time.Duration
}

func (rc *RedisCacheClient) GetClient(ctx context.Context, clientID string) (*Client, error) {
    // Try Redis
    val, err := rc.redis.Get(ctx, "client:"+clientID).Result()
    if err == nil {
        // Cache hit - deserialize and return
        var client Client
        json.Unmarshal([]byte(val), &client)
        return &client, nil
    }
    
    if err != redis.Nil {
        // Redis error (not critical, fall through to DB)
        log.Warn().Err(err).Msg("Redis error, falling back to database")
    }
    
    // Cache miss or error - query database
    client, err := dbClient.clientByID(ctx, clientID)
    if err != nil {
        return nil, err
    }
    
    // Update cache
    data, _ := json.Marshal(client)
    rc.redis.Set(ctx, "client:"+clientID, data, rc.ttl)
    
    return client, nil
}

func (rc *RedisCacheClient) InvalidateClient(ctx context.Context, clientID string) error {
    return rc.redis.Del(ctx, "client:"+clientID).Err()
}
```

### Configuration

```yaml
# Docker compose example
redis:
  image: redis:7-alpine
  ports:
    - "6379:6379"
  volumes:
    - redis-data:/data
  command: redis-server --maxmemory 500mb --maxmemory-policy allkeys-lru

# Environment variables
REDIS_URL: redis://localhost:6379
REDIS_CLIENT_CACHE_TTL: 300s (5 minutes)
REDIS_REVOCATION_CACHE_TTL: 60s (1 minute)
```

### Performance Characteristics

```
Hit Rate: 95-99%
  ├─ Redis latency: <5ms
  ├─ Throughput: 100K-500K queries/sec
  └─ Database load: 5-50 req/sec (only misses)

Memory Usage: 
  ├─ 1000 clients × 1KB each: 1MB
  ├─ 100K revoked tokens: 10MB
  └─ Overhead: ~20MB total
  └─ Max config: 500MB

Advantages:
  ✅ Shared across multiple servers
  ✅ Persistent storage (optional)
  ✅ Rich data structures
  ✅ Built-in expiration
  ✅ Easy monitoring

Disadvantages:
  ⚠️ Network latency (5ms)
  ⚠️ External service to manage
  ⚠️ Additional infrastructure cost
  ⚠️ Requires Redis expertise
```

### Cache Key Design

```
Pattern 1: Clients Table
  Key: "client:{client_id}"
  Value: JSON(Client)
  TTL: 5-10 minutes
  Example: "client:service-auth-a"

Pattern 2: Revocation List
  Key: "revoked_tokens"
  Value: Set of token IDs
  TTL: 1-5 minutes
  Example: "revoked_tokens": {"token1", "token2", "token3"}

Pattern 3: Client Quota
  Key: "quota:{client_id}"
  Value: JSON(quota_data)
  TTL: 1 minute
  Example: "quota:service-auth-a"

Pattern 4: Rate Limiting
  Key: "ratelimit:{client_id}:{minute}"
  Value: Counter
  TTL: 2 minutes
  Example: "ratelimit:service-auth-a:202512301400"
```

---

## 4. Cache Layer 3: Database (Primary)

### Read-Through Pattern

```go
func (as *authServer) getClientCached(ctx context.Context, clientID string) (*Client, error) {
    // L1: In-process cache
    if client, err := as.l1Cache.Get(clientID); err == nil {
        log.Debug().Str("client_id", clientID).Msg("Cache hit: L1")
        return client, nil
    }
    
    // L2: Redis cache
    if client, err := as.l2Cache.GetClient(ctx, clientID); err == nil {
        log.Debug().Str("client_id", clientID).Msg("Cache hit: L2")
        as.l1Cache.Set(clientID, client) // Populate L1
        return client, nil
    }
    
    // L3: Database (authoritative)
    client, err := as.clientByID(clientID)
    if err != nil {
        log.Error().Err(err).Str("client_id", clientID).Msg("Database lookup failed")
        return nil, err
    }
    
    log.Debug().Str("client_id", clientID).Msg("Cache miss: L3 (DB)")
    
    // Populate caches
    as.l1Cache.Set(clientID, client)
    as.l2Cache.SetClient(ctx, clientID, client)
    
    return client, nil
}
```

### Write-Through Pattern

```go
func (as *authServer) updateClient(ctx context.Context, client *Client) error {
    // Write to database first (authoritative)
    if err := as.db.UpdateClient(ctx, client); err != nil {
        log.Error().Err(err).Str("client_id", client.ClientID).Msg("Failed to update client in database")
        return err
    }
    
    // Invalidate caches
    as.l1Cache.Delete(client.ClientID)
    as.l2Cache.InvalidateClient(ctx, client.ClientID)
    
    log.Info().Str("client_id", client.ClientID).Msg("Client updated and caches invalidated")
    
    return nil
}
```

---

## 5. Revocation Cache Strategy

### Token Revocation Tracking

```go
type RevocationCache struct {
    mu               sync.RWMutex
    revokedTokens    map[string]time.Time  // token_id → revoked_at
    revocationTTL    time.Duration
    lastSync         time.Time
}

func (rc *RevocationCache) IsRevoked(tokenID string) bool {
    rc.mu.RLock()
    defer rc.mu.RUnlock()
    
    revokedAt, exists := rc.revokedTokens[tokenID]
    if !exists {
        return false
    }
    
    // Check if still in cache window
    if time.Since(revokedAt) > rc.revocationTTL {
        return false // Assume not revoked (expired from cache)
    }
    
    return true
}

func (rc *RevocationCache) RevokeToken(tokenID string) {
    rc.mu.Lock()
    defer rc.mu.Unlock()
    rc.revokedTokens[tokenID] = time.Now()
}

func (rc *RevocationCache) Sync(ctx context.Context, db *sql.DB) error {
    // Periodically sync with database
    // Query: SELECT token_id FROM tokens WHERE revoked=true AND revoked_at > ?
    query := "SELECT token_id FROM tokens WHERE revoked=true AND revoked_at > ?"
    rows, err := db.QueryContext(ctx, query, time.Now().Add(-rc.revocationTTL))
    if err != nil {
        return err
    }
    defer rows.Close()
    
    rc.mu.Lock()
    for rows.Next() {
        var tokenID string
        if err := rows.Scan(&tokenID); err != nil {
            continue
        }
        rc.revokedTokens[tokenID] = time.Now()
    }
    rc.mu.Unlock()
    
    rc.lastSync = time.Now()
    return nil
}
```

### Configuration

```yaml
Revocation Cache:
  TTL: 1-5 minutes
  Sync Interval: 30-60 seconds
  Max Size: 100K tokens
  
  Tradeoff:
    ├─ Short TTL (1 min): Faster revocation (1 min max delay)
    ├─ Long TTL (5 min): Fewer syncs, more cache hits
    └─ Recommended: 2-3 minutes
```

---

## 6. Cache Invalidation Strategy

### Event-Based Invalidation

```
Scenario 1: Client Secret Changed
  ├─ Trigger: Admin updates client credentials
  ├─ Action: Delete "client:{client_id}" from all caches
  ├─ Impact: New token requests query database (5 sec max)
  └─ Risk: Previous tokens still valid (JWT doesn't check secret)

Scenario 2: Client Scopes Updated
  ├─ Trigger: Admin modifies allowed resources
  ├─ Action: Delete "client:{client_id}" from all caches
  ├─ Impact: New tokens get new scopes
  └─ Risk: Old tokens retain old scopes (2-10 min max)

Scenario 3: Token Revoked
  ├─ Trigger: User/admin revokes specific token
  ├─ Action: Add to revocation list cache
  ├─ Impact: Immediate revocation
  └─ Risk: None (revocation is enforced)

Scenario 4: Client Disabled
  ├─ Trigger: Admin disables client account
  ├─ Action: Delete from cache + block in code
  ├─ Impact: No new tokens generated
  └─ Risk: Old tokens still valid (until expiration)
```

### TTL-Based Invalidation

```
Automatic Expiration (Recommended):
  ├─ Client Cache: 5-10 minutes
  │  └─ Risk: Stale for up to 10 minutes after change
  │
  ├─ Revocation Cache: 1-5 minutes
  │  └─ Risk: Revocation delay up to 5 minutes
  │
  └─ Acceptable for most use cases
```

### Manual Invalidation Endpoints

```go
// Optional: Add admin endpoints for cache management
POST /admin/cache/invalidate-client/{clientId}
  ├─ Purpose: Force refresh of client data
  ├─ Auth: Admin only
  └─ Response: 200 OK

POST /admin/cache/invalidate-revocations
  ├─ Purpose: Refresh entire revocation list
  ├─ Auth: Admin only
  └─ Response: 200 OK

GET /admin/cache/stats
  ├─ Purpose: View cache hit rates and sizes
  ├─ Auth: Admin only
  └─ Response: JSON with metrics
```

---

## 7. Caching Roadmap

### Phase 1: L1 In-Process Cache (Week 1)

```
Implementation Steps:
  1. Create ClientCache struct with sync.RWMutex
  2. Add cache to authServer initialization
  3. Update clientByID() to use cache
  4. Add cache stats/metrics
  5. Test with unit tests
  
Expected Result:
  ├─ Database load: -50%
  ├─ Latency: <1µs for hits
  ├─ Memory: +50MB
  ├─ Complexity: Low
  └─ Time: 4-8 hours
```

### Phase 2: L2 Redis Cache (Week 2-3)

```
Implementation Steps:
  1. Add Redis dependency (go-redis/v9)
  2. Create RedisCacheClient
  3. Implement get/set/invalidate methods
  4. Add fallback for Redis failures
  5. Setup Redis connection pooling
  6. Add metrics for cache hits/misses
  
Expected Result:
  ├─ Database load: -90%
  ├─ Shared across servers: Yes
  ├─ External dependency: Yes (Redis)
  ├─ Complexity: Medium
  └─ Time: 8-16 hours
```

### Phase 3: Revocation Cache (Week 3)

```
Implementation Steps:
  1. Create RevocationCache with periodic sync
  2. Update validateJWT() to use cache first
  3. Add background sync goroutine
  4. Handle cache failures gracefully
  5. Add metrics and monitoring
  
Expected Result:
  ├─ Revocation check latency: <1µs
  ├─ Database read load: -99%
  ├─ Revocation delay: <5 minutes (acceptable)
  ├─ Complexity: Low
  └─ Time: 6-12 hours
```

### Phase 4: Monitoring & Optimization (Week 4)

```
Tasks:
  1. Add cache hit/miss metrics
  2. Monitor cache size growth
  3. Track cache eviction rate
  4. Setup alerts for stale data
  5. Performance testing and tuning
  
Metrics to Track:
  ├─ Cache hit rate (target: >95%)
  ├─ Cache size (target: <100MB)
  ├─ Eviction rate (target: <1/sec)
  ├─ Database query reduction
  └─ End-to-end latency improvement
```

---

## 8. Implementation Checklist

### Code Changes Required

- [ ] Create `cache.go` with ClientCache struct
- [ ] Add cache initialization to `service.go`
- [ ] Update `handlers.go` to use cached client lookup
- [ ] Create `redis_cache.go` for distributed cache
- [ ] Update `tokens.go` revocation check with cache
- [ ] Add cache metrics/observability
- [ ] Add cache invalidation endpoints (optional)
- [ ] Update tests to cover cache scenarios
- [ ] Add configuration for cache TTLs
- [ ] Documentation and runbooks

### Configuration Updates

- [ ] Add cache TTL settings to config
- [ ] Add Redis connection settings
- [ ] Add cache size limits
- [ ] Add cache sync intervals
- [ ] Environment variables for cache

### Testing Requirements

- [ ] Unit tests for cache get/set/invalidate
- [ ] Integration tests with database
- [ ] Load tests to verify performance
- [ ] Cache miss/failure scenarios
- [ ] TTL expiration handling
- [ ] Concurrent access patterns

### Monitoring Setup

- [ ] Add Prometheus metrics
- [ ] Cache hit/miss rate dashboard
- [ ] Cache size monitoring
- [ ] Database query reduction metrics
- [ ] Latency comparison (before/after cache)

---

## 9. Performance Comparison

### Before Caching

```
Load: 100K token generation requests/second
  ├─ Database hit: 100%
  ├─ Database queries: 100K SELECT + 100K INSERT = 200K/sec
  ├─ Database latency: 50-100µs per query
  ├─ Bottleneck: Database (~10K writes/sec limit)
  └─ Practical max: 50-100K token gen/sec
```

### After L1 + L2 Caching

```
Load: 100K token generation requests/second
  ├─ Cache hits: 99%
  ├─ L1 cache hits: 95% (sub-microsecond)
  ├─ L2 cache hits: 4% (<5ms)
  ├─ Database hits: 1% (<50µs)
  ├─ Total database queries: 1K SELECT + 100K INSERT = 101K/sec
  ├─ Database load: 10x reduction in reads
  └─ Practical max: 1M+ token gen/sec
```

### ROI Analysis

```
Cost:
  ├─ Development time: 40-60 hours
  ├─ Redis infrastructure: $5-20/month
  └─ Maintenance overhead: Low

Benefit:
  ├─ Delay database migration: 3+ months
  ├─ Database cost savings: $1000-5000
  ├─ Performance improvement: 10x
  ├─ Server scalability: 1 server → 10+ servers
  └─ User satisfaction: Improved response time

ROI Payback: < 2 weeks
```

---

## 10. Caching Best Practices

### Do's ✅

- ✅ Cache read-heavy data (clients, revocations)
- ✅ Use appropriate TTLs for different data types
- ✅ Implement graceful degradation (DB fallback)
- ✅ Monitor cache effectiveness
- ✅ Set cache size limits to prevent OOM
- ✅ Use write-through for critical updates
- ✅ Log cache hits/misses for debugging
- ✅ Test cache failures explicitly

### Don'ts ❌

- ❌ Cache sensitive data without encryption
- ❌ Use infinite TTLs (always set expiration)
- ❌ Ignore cache failures (always have fallback)
- ❌ Cache write operations
- ❌ Trust cache for authorization decisions alone
- ❌ Forget to invalidate on updates
- ❌ Leave cache unbounded (set max size)
- ❌ Use cache as primary storage

---

## Conclusion

**Recommended Strategy**: Implement L1 + L2 caching gradually

| Phase | Timeline | Effort | Impact |
|-------|----------|--------|--------|
| **L1 In-Process** | 1 week | Low (8h) | +50% throughput, -50% DB load |
| **L2 Redis** | 2-3 weeks | Medium (16h) | +10x throughput, -90% DB load |
| **L3 Revocation** | 1 week | Low (10h) | +1% improvement, -99% revocation reads |

**Total Effort**: 40-60 hours over 4 weeks  
**Expected Benefit**: 10x throughput increase, enables 10+ server cluster without DB migration

Start with L1 now, plan L2 for next sprint.
