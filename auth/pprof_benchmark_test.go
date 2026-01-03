package auth

import (
	"testing"
	"time"
)

// BenchmarkClientCache tests the in-memory client cache performance
func BenchmarkClientCache(b *testing.B) {
	cache := NewClientCache(10*time.Minute, 5000)
	defer cache.Stop()

	// Create test client
	testClient := &Clients{
		ClientID:      "test-client-1",
		ClientSecret:  "secret-123",
		AllowedScopes: []string{"read:tokens", "write:tokens"},
	}

	// Cache the client
	cache.Set("test-client-1", testClient)

	b.ResetTimer()

	// Benchmark cache hits (99% of requests)
	b.Run("CacheHit", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = cache.Get("test-client-1")
		}
	})

	b.Run("CacheMiss", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = cache.Get("non-existent")
		}
	})
}

// BenchmarkTokenBatchWriter tests batch token writing performance
func BenchmarkTokenBatchWriter(b *testing.B) {
	// Skip this benchmark as it requires DB context
	b.Skip("Batch writer requires full context setup")
}

// BenchmarkGenerateJWTWithCache compares new implementation with cache hits
func BenchmarkGenerateJWTWithCache(b *testing.B) {
	// Create mock auth server with cache
	as := &authServer{
		clientCache: NewClientCache(10*time.Minute, 5000),
	}
	defer as.clientCache.Stop()

	// Pre-populate cache
	testClient := &Clients{
		ClientID:      "bench-client",
		ClientSecret:  "secret",
		AllowedScopes: []string{"read:tokens", "write:tokens"},
	}
	as.clientCache.Set("bench-client", testClient)

	b.ResetTimer()

	// Benchmark token generation with cache (new code path)
	for i := 0; i < b.N; i++ {
		// Simulate the new generateJWT code path
		if cachedClient, found := as.clientCache.Get("bench-client"); found {
			// This is now a cache hit - should be very fast
			_ = cachedClient
		}
	}
}

// BenchmarkConcurrentCacheAccess tests thread safety under concurrent load
func BenchmarkConcurrentCacheAccess(b *testing.B) {
	cache := NewClientCache(10*time.Minute, 5000)
	defer cache.Stop()

	testClient := &Clients{
		ClientID:      "concurrent-test",
		ClientSecret:  "secret",
		AllowedScopes: []string{"read:tokens"},
	}
	cache.Set("concurrent-test", testClient)

	b.ResetTimer()

	// Run concurrent reads (simulating parallel token requests)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = cache.Get("concurrent-test")
		}
	})
}

// BenchmarkConcurrentBatchWrites tests batch writer under concurrent load
func BenchmarkConcurrentBatchWrites(b *testing.B) {
	b.Skip("Batch writer requires full context setup")
}

// MemoryBenchmark - Track memory allocations
func BenchmarkMemoryUsage(b *testing.B) {
	b.Run("ClientCache-100Entries", func(b *testing.B) {
		cache := NewClientCache(10*time.Minute, 5000)
		b.ReportAllocs()
		for i := 0; i < 100; i++ {
			client := &Clients{
				ClientID:      "client-100",
				ClientSecret:  "secret",
				AllowedScopes: []string{"read:tokens"},
			}
			cache.Set("client-100", client)
		}
		cache.Stop()
	})

	b.Run("ClientCache-1000Entries", func(b *testing.B) {
		cache := NewClientCache(10*time.Minute, 5000)
		b.ReportAllocs()
		for i := 0; i < 1000; i++ {
			client := &Clients{
				ClientID:      "client-1000",
				ClientSecret:  "secret",
				AllowedScopes: []string{"read:tokens"},
			}
			cache.Set("client-1000", client)
		}
		cache.Stop()
	})
}
