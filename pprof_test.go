package main

import (
	"fmt"
	"math/rand"
	"os"
	"runtime"
	"runtime/pprof"
	"sync"
	"testing"
	"time"
)

// BenchmarkTokenGeneration - Stress test token generation
func BenchmarkTokenGeneration(b *testing.B) {
	b.ReportAllocs()

	// Simulate token generation
	for i := 0; i < b.N; i++ {
		clientID := fmt.Sprintf("client-%d", rand.Intn(100))
		scopes := []string{"scope1", "scope2", "scope3"}

		// Simulate JWT creation
		_ = createMockJWT(clientID, scopes)
	}
}

// BenchmarkTokenValidation - Stress test token validation
func BenchmarkTokenValidation(b *testing.B) {
	b.ReportAllocs()

	// Create mock token
	token := createMockJWT("test-client", []string{"scope1", "scope2"})

	for i := 0; i < b.N; i++ {
		_ = validateMockJWT(token)
	}
}

// BenchmarkDatabaseQuery - Simulate database client lookup
func BenchmarkDatabaseQuery(b *testing.B) {
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		clientID := fmt.Sprintf("client-%d", rand.Intn(1000))
		_ = simulateDatabaseQuery(clientID)
	}
}

// BenchmarkJSONParsing - Parse token requests
func BenchmarkJSONParsing(b *testing.B) {
	b.ReportAllocs()

	jsonData := `{"grant_type":"client_credentials","client_id":"test","client_secret":"secret"}`

	for i := 0; i < b.N; i++ {
		_ = parseJSON(jsonData)
	}
}

// LoadTest - Simulate concurrent load
func LoadTest() {
	fmt.Println("\n=== LOAD TEST: Simulating 1000 Concurrent Requests ===")
	fmt.Println()

	var wg sync.WaitGroup
	numGoroutines := 1000
	requestsPerGoroutine := 100

	start := time.Now()
	successCount := 0
	errorCount := 0
	var mu sync.Mutex

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < requestsPerGoroutine; j++ {
				// Simulate request
				success := simulateRequest(id, j)
				mu.Lock()
				if success {
					successCount++
				} else {
					errorCount++
				}
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)

	totalRequests := numGoroutines * requestsPerGoroutine
	requestsPerSecond := float64(totalRequests) / elapsed.Seconds()
	avgLatency := elapsed.Nanoseconds() / int64(totalRequests)

	fmt.Printf("Total Requests: %d\n", totalRequests)
	fmt.Printf("Successful: %d\n", successCount)
	fmt.Printf("Errors: %d\n", errorCount)
	fmt.Printf("Total Time: %v\n", elapsed)
	fmt.Printf("Requests/Second: %.2f\n", requestsPerSecond)
	fmt.Printf("Avg Latency: %v\n", time.Duration(avgLatency))
	fmt.Printf("Success Rate: %.2f%%\n", float64(successCount)*100/float64(totalRequests))
	fmt.Println()
}

// MemoryProfile - Run with memory profiling
func MemoryProfile() {
	fmt.Println("=== MEMORY PROFILE ===")
	fmt.Println()

	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)
	fmt.Printf("Before Load Test:\n")
	fmt.Printf("  Alloc: %v MB\n", m1.Alloc/1024/1024)
	fmt.Printf("  TotalAlloc: %v MB\n", m1.TotalAlloc/1024/1024)
	fmt.Printf("  Sys: %v MB\n", m1.Sys/1024/1024)
	fmt.Printf("  NumGC: %v\n", m1.NumGC)
	fmt.Println()

	// Write memory profile before
	f, _ := os.Create("memprofile_before.prof")
	pprof.WriteHeapProfile(f)
	f.Close()

	// Run load test
	LoadTest()

	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)
	fmt.Printf("After Load Test:\n")
	fmt.Printf("  Alloc: %v MB\n", m2.Alloc/1024/1024)
	fmt.Printf("  TotalAlloc: %v MB\n", m2.TotalAlloc/1024/1024)
	fmt.Printf("  Sys: %v MB\n", m2.Sys/1024/1024)
	fmt.Printf("  NumGC: %v\n", m2.NumGC)
	fmt.Printf("  GC Pause Avg: %v\n", m2.PauseNs[(m2.NumGC+255)%256])
	fmt.Println()

	// Memory increase
	fmt.Printf("Memory Increase:\n")
	fmt.Printf("  Alloc: +%v MB\n", (m2.Alloc-m1.Alloc)/1024/1024)
	fmt.Printf("  TotalAlloc: +%v MB\n", (m2.TotalAlloc-m1.TotalAlloc)/1024/1024)
	fmt.Printf("  Sys: +%v MB\n", (m2.Sys-m1.Sys)/1024/1024)
	fmt.Println()

	// Write memory profile after
	f, _ = os.Create("memprofile_after.prof")
	pprof.WriteHeapProfile(f)
	f.Close()

	fmt.Println("Memory profiles saved: memprofile_before.prof, memprofile_after.prof")
	fmt.Println("Use: go tool pprof memprofile_after.prof")
	fmt.Println()
}

// CPUProfile - Run with CPU profiling
func CPUProfile() {
	fmt.Println("=== CPU PROFILE ===")
	fmt.Println()

	f, _ := os.Create("cpuprofile.prof")
	defer f.Close()

	pprof.StartCPUProfile(f)
	defer pprof.StopCPUProfile()

	start := time.Now()

	// Run intensive computation
	var total int64
	for i := 0; i < 1000000; i++ {
		clientID := fmt.Sprintf("client-%d", rand.Intn(1000))
		token := createMockJWT(clientID, []string{"scope1", "scope2"})
		if validateMockJWT(token) {
			total++
		}
	}

	elapsed := time.Since(start)

	fmt.Printf("Completed 1,000,000 token operations\n")
	fmt.Printf("Total Time: %v\n", elapsed)
	fmt.Printf("Operations/Second: %.2f\n", float64(1000000)/elapsed.Seconds())
	fmt.Printf("Avg Latency: %v\n", elapsed/1000000)
	fmt.Println()
	fmt.Printf("Successful validations: %d\n", total)
	fmt.Println()
	fmt.Println("CPU profile saved: cpuprofile.prof")
	fmt.Println("Use: go tool pprof cpuprofile.prof")
	fmt.Println()
}

// GoroutineProfile - Check goroutine creation/cleanup
func GoroutineProfile() {
	fmt.Println("=== GOROUTINE PROFILE ===")
	fmt.Println()

	fmt.Printf("Initial Goroutines: %d\n", runtime.NumGoroutine())
	fmt.Println()

	// Create goroutines like handlers would
	var wg sync.WaitGroup
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			// Simulate request processing
			time.Sleep(time.Millisecond * 10)
		}(i)
	}

	fmt.Printf("Goroutines During Load: %d\n", runtime.NumGoroutine())
	wg.Wait()

	fmt.Printf("Goroutines After Cleanup: %d\n", runtime.NumGoroutine())
	fmt.Println()

	// Write goroutine profile
	f, _ := os.Create("goroutineprofile.prof")
	pprof.Lookup("goroutine").WriteTo(f, 0)
	f.Close()

	fmt.Println("Goroutine profile saved: goroutineprofile.prof")
	fmt.Println()
}

// BlockProfile - Identify contention
func BlockProfile() {
	fmt.Println("=== BLOCK PROFILE (Contention Analysis) ===")
	fmt.Println()

	runtime.SetBlockProfileRate(1)
	defer runtime.SetBlockProfileRate(0)

	var mu sync.Mutex
	var wg sync.WaitGroup

	start := time.Now()
	contentionCount := 0

	// Create contention
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				mu.Lock()
				contentionCount++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	elapsed := time.Since(start)

	fmt.Printf("Completed %d operations under contention\n", contentionCount)
	fmt.Printf("Total Time: %v\n", elapsed)
	fmt.Printf("Operations/Second: %.2f\n", float64(contentionCount)/elapsed.Seconds())
	fmt.Println()

	// Write block profile
	f, _ := os.Create("blockprofile.prof")
	pprof.Lookup("block").WriteTo(f, 0)
	f.Close()

	fmt.Println("Block profile saved: blockprofile.prof")
	fmt.Println()
}

// AllocationProfile - Track memory allocations
func AllocationProfile() {
	fmt.Println("=== ALLOCATION PROFILE ===")
	fmt.Println()

	f, _ := os.Create("allocationprofile.prof")
	defer f.Close()

	// Allocate various sizes
	var slices [][]string
	var maps []map[string]interface{}

	start := time.Now()
	allocCount := 0

	for i := 0; i < 100000; i++ {
		// Allocate slice
		slice := make([]string, rand.Intn(10)+1)
		slices = append(slices, slice)
		allocCount++

		// Allocate map
		m := make(map[string]interface{})
		maps = append(maps, m)
		allocCount++

		// Allocate string
		_ = fmt.Sprintf("string-%d", i)
		allocCount++
	}

	elapsed := time.Since(start)

	fmt.Printf("Allocations: %d\n", allocCount)
	fmt.Printf("Total Time: %v\n", elapsed)
	fmt.Printf("Allocations/Second: %.2f\n", float64(allocCount)/elapsed.Seconds())
	fmt.Println()

	// Force GC
	runtime.GC()

	// Write allocation profile
	pprof.WriteHeapProfile(f)

	fmt.Println("Allocation profile saved: allocationprofile.prof")
	fmt.Println()
}

// Helper functions

func createMockJWT(clientID string, scopes []string) string {
	// Simulate JWT creation
	return fmt.Sprintf("eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.%s.%s",
		clientID, time.Now().String())
}

func validateMockJWT(token string) bool {
	// Simulate JWT validation
	return len(token) > 10
}

func simulateDatabaseQuery(clientID string) map[string]interface{} {
	// Simulate database lookup
	return map[string]interface{}{
		"client_id": clientID,
		"secret":    "secret123",
		"scopes":    []string{"scope1", "scope2"},
	}
}

func parseJSON(data string) map[string]string {
	// Simulate JSON parsing
	return map[string]string{
		"grant_type":    "client_credentials",
		"client_id":     "test",
		"client_secret": "secret",
	}
}

func simulateRequest(goroutineID, requestID int) bool {
	// Simulate a request
	rand.Seed(time.Now().UnixNano())

	// Random chance of failure
	if rand.Float64() < 0.01 {
		return false
	}

	// Simulate work
	time.Sleep(time.Microsecond * time.Duration(rand.Intn(100)+10))
	return true
}

func TestMain(m *testing.M) {
	// Run all profiling when tests run
	fmt.Println("\n╔════════════════════════════════════════════════════╗")
	fmt.Println("║         AUTH SERVER - PERFORMANCE PROFILING         ║")
	fmt.Println("╚════════════════════════════════════════════════════╝")
	fmt.Println()

	MemoryProfile()
	CPUProfile()
	GoroutineProfile()
	BlockProfile()
	AllocationProfile()

	fmt.Println("╔════════════════════════════════════════════════════╗")
	fmt.Println("║              PROFILING COMPLETE                    ║")
	fmt.Println("╚════════════════════════════════════════════════════╝")

	os.Exit(m.Run())
}
