package auth

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
)

// ClientCache provides in-memory caching for client credentials with thread-safe operations
type ClientCache struct {
	mu            sync.RWMutex
	cache         map[string]*CachedClient
	ttl           time.Duration
	maxSize       int
	stats         CacheStatsAtomic
	cleanupTicker *time.Ticker
	done          chan struct{}
}

// CachedClient stores client data with expiration metadata
type CachedClient struct {
	Client    *Clients
	ExpiresAt time.Time
	CreatedAt time.Time
}

// CacheStatsAtomic tracks cache performance metrics with atomic operations (thread-safe)
type CacheStatsAtomic struct {
	Hits    atomic.Int64
	Misses  atomic.Int64
	Evicted atomic.Int64
}

// CacheStats is a snapshot of cache statistics for reporting
type CacheStats struct {
	Hits    int64
	Misses  int64
	Evicted int64
}

// NewClientCache creates a new client cache instance with validation
// Parameters: ttl - time-to-live for cached entries, maxSize - maximum number of clients to cache
func NewClientCache(ttl time.Duration, maxSize int) *ClientCache {
	// Validate and fix invalid parameters
	if ttl <= 0 {
		log.Warn().Dur("ttl", ttl).Msg("Invalid TTL, using default 10 minutes")
		ttl = 10 * time.Minute
	}
	if maxSize <= 0 {
		log.Warn().Int("max_size", maxSize).Msg("Invalid maxSize, using default 5000")
		maxSize = 5000
	}

	cc := &ClientCache{
		cache:   make(map[string]*CachedClient),
		ttl:     ttl,
		maxSize: maxSize,
		done:    make(chan struct{}),
	}

	// Calculate cleanup interval (TTL/2, minimum 1 minute)
	cleanupInterval := ttl / 2
	if cleanupInterval < 1*time.Minute {
		cleanupInterval = 1 * time.Minute
	}

	// Start background cleanup goroutine
	cc.cleanupTicker = time.NewTicker(cleanupInterval)
	go cc.cleanupExpired()

	log.Info().
		Int("max_size", maxSize).
		Str("ttl", ttl.String()).
		Str("cleanup_interval", cleanupInterval.String()).
		Msg("Client cache initialized")

	return cc
}

// Get retrieves a client from cache if it exists and hasn't expired
// Returns the cached client and true if found and not expired, nil and false otherwise
func (cc *ClientCache) Get(clientID string) (*Clients, bool) {
	cc.mu.RLock()
	cached, exists := cc.cache[clientID]
	cc.mu.RUnlock()

	if !exists {
		cc.stats.Misses.Add(1)
		return nil, false
	}

	// Check if expired (do this outside lock to minimize lock contention)
	if time.Now().After(cached.ExpiresAt) {
		cc.stats.Misses.Add(1)
		return nil, false
	}

	cc.stats.Hits.Add(1)
	return cached.Client, true
}

// Set stores a client in cache, evicting oldest entry if cache is full
func (cc *ClientCache) Set(clientID string, client *Clients) {
	if client == nil {
		log.Warn().Str("client_id", clientID).Msg("Attempted to cache nil client, skipping")
		return
	}

	cc.mu.Lock()
	defer cc.mu.Unlock()

	// Evict oldest entry if cache is at max size (and entry doesn't already exist)
	if len(cc.cache) >= cc.maxSize && cc.cache[clientID] == nil {
		cc.evictOldestLocked()
	}

	cc.cache[clientID] = &CachedClient{
		Client:    client,
		ExpiresAt: time.Now().Add(cc.ttl),
		CreatedAt: time.Now(),
	}
}

// Invalidate removes a specific client from cache (useful for forced updates)
func (cc *ClientCache) Invalidate(clientID string) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	if _, exists := cc.cache[clientID]; exists {
		delete(cc.cache, clientID)
		log.Debug().Str("client_id", clientID).Msg("Client cache entry invalidated")
	}
}

// Clear removes all clients from cache (e.g., during shutdown or restart)
func (cc *ClientCache) Clear() {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	oldSize := len(cc.cache)
	cc.cache = make(map[string]*CachedClient)
	log.Info().Int("cleared_entries", oldSize).Msg("Client cache cleared")
}

// GetStats returns a snapshot of cache statistics (thread-safe)
func (cc *ClientCache) GetStats() CacheStats {
	return CacheStats{
		Hits:    cc.stats.Hits.Load(),
		Misses:  cc.stats.Misses.Load(),
		Evicted: cc.stats.Evicted.Load(),
	}
}

// GetSize returns current number of entries in cache
func (cc *ClientCache) GetSize() int {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	return len(cc.cache)
}

// GetHitRate returns cache hit rate percentage (0-100)
func (cc *ClientCache) GetHitRate() float64 {
	hits := cc.stats.Hits.Load()
	misses := cc.stats.Misses.Load()
	total := hits + misses

	if total == 0 {
		return 0
	}

	return float64(hits) / float64(total) * 100
}

// cleanupExpired removes expired entries periodically (runs in background goroutine)
func (cc *ClientCache) cleanupExpired() {
	for {
		select {
		case <-cc.done:
			cc.cleanupTicker.Stop()
			log.Debug().Msg("Cache cleanup goroutine stopped")
			return
		case <-cc.cleanupTicker.C:
			cc.removeExpired()
		}
	}
}

// removeExpired removes all expired entries from cache
func (cc *ClientCache) removeExpired() {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	now := time.Now()
	removed := 0

	for clientID, cached := range cc.cache {
		if now.After(cached.ExpiresAt) {
			delete(cc.cache, clientID)
			removed++
		}
	}

	if removed > 0 {
		log.Debug().
			Int("removed", removed).
			Int("cache_size", len(cc.cache)).
			Msg("Expired cache entries cleaned up")
	}
}

// evictOldestLocked removes the oldest entry when cache is full (assumes lock is held)
func (cc *ClientCache) evictOldestLocked() {
	if len(cc.cache) == 0 {
		return
	}

	var oldestID string
	var oldestTime time.Time = time.Now()
	firstEntry := true

	// Find the oldest entry by creation time
	for clientID, cached := range cc.cache {
		if firstEntry || cached.CreatedAt.Before(oldestTime) {
			oldestTime = cached.CreatedAt
			oldestID = clientID
			firstEntry = false
		}
	}

	if oldestID != "" {
		delete(cc.cache, oldestID)
		cc.stats.Evicted.Add(1)
		log.Debug().
			Str("client_id", oldestID).
			Int("cache_size", len(cc.cache)).
			Msg("Cache entry evicted (max size reached)")
	}
}

// Stop gracefully stops the cache cleanup goroutine
func (cc *ClientCache) Stop() {
	close(cc.done)
	log.Info().Msg("Client cache stopped")
}

// TokenBatchWriter handles asynchronous batch insertion of tokens to reduce DB load
type TokenBatchWriter struct {
	mu         sync.Mutex
	tokens     []Token
	maxBatch   int
	flushTick  *time.Ticker
	done       chan struct{}
	authServer *authServer
}

// NewTokenBatchWriter creates a new token batch writer with specified parameters
// Parameters: authServer - server instance for DB access, maxBatch - size before auto-flush, flushInterval - max time before flush
func NewTokenBatchWriter(as *authServer, maxBatch int, flushInterval time.Duration) *TokenBatchWriter {
	if maxBatch <= 0 {
		log.Warn().Int("max_batch", maxBatch).Msg("Invalid maxBatch, using default 1000")
		maxBatch = 1000
	}
	if flushInterval <= 0 {
		log.Warn().Dur("flush_interval", flushInterval).Msg("Invalid flushInterval, using default 5 seconds")
		flushInterval = 5 * time.Second
	}

	tbw := &TokenBatchWriter{
		tokens:     make([]Token, 0, maxBatch),
		maxBatch:   maxBatch,
		done:       make(chan struct{}),
		authServer: as,
		flushTick:  time.NewTicker(flushInterval),
	}

	// Start background flush goroutine
	go tbw.backgroundFlush()

	log.Info().
		Int("max_batch", maxBatch).
		Str("flush_interval", flushInterval.String()).
		Msg("Token batch writer initialized")

	return tbw
}

// Add queues a token for batch insertion (non-blocking)
func (tbw *TokenBatchWriter) Add(token Token) {
	if token.TokenID == "" || token.ClientID == "" {
		log.Warn().Msg("Attempted to add invalid token (missing TokenID or ClientID)")
		return
	}

	tbw.mu.Lock()
	defer tbw.mu.Unlock()

	tbw.tokens = append(tbw.tokens, token)

	// Flush immediately if batch is full
	if len(tbw.tokens) >= tbw.maxBatch {
		tbw.flushLockedAsync()
	}
}

// Flush immediately writes pending tokens to database (blocking)
func (tbw *TokenBatchWriter) Flush() {
	tbw.mu.Lock()
	defer tbw.mu.Unlock()

	if len(tbw.tokens) > 0 {
		tbw.flushLockedAsync()
	}
}

// flushLockedAsync flushes tokens asynchronously without acquiring lock (assumes lock is held)
func (tbw *TokenBatchWriter) flushLockedAsync() {
	if len(tbw.tokens) == 0 {
		return
	}

	// Copy tokens and reset buffer (prevents holding lock during DB operation)
	batch := make([]Token, len(tbw.tokens))
	copy(batch, tbw.tokens)
	tbw.tokens = tbw.tokens[:0]

	// Write to database asynchronously in separate goroutine
	go func() {
		if err := tbw.authServer.insertTokenBatch(batch); err != nil {
			log.Error().
				Err(err).
				Int("batch_size", len(batch)).
				Msg("Failed to insert token batch")
		} else {
			log.Debug().
				Int("batch_size", len(batch)).
				Msg("Token batch inserted successfully")
		}
	}()
}

// backgroundFlush flushes tokens periodically or on shutdown (runs in background goroutine)
func (tbw *TokenBatchWriter) backgroundFlush() {
	for {
		select {
		case <-tbw.done:
			tbw.flushTick.Stop()
			// Final flush before shutdown
			tbw.Flush()
			log.Debug().Msg("Token batch writer background flush stopped")
			return
		case <-tbw.flushTick.C:
			tbw.Flush()
		}
	}
}

// Stop gracefully stops the batch writer and flushes any pending tokens
func (tbw *TokenBatchWriter) Stop() {
	close(tbw.done)
	log.Info().Msg("Token batch writer stopped")
}

// GetPendingCount returns number of tokens currently waiting for flush
func (tbw *TokenBatchWriter) GetPendingCount() int {
	tbw.mu.Lock()
	defer tbw.mu.Unlock()
	return len(tbw.tokens)
}
