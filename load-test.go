package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"
)

// LoadTestConfig holds the configuration for load testing
type LoadTestConfig struct {
	BaseURL           string
	Concurrency       int
	RequestsPerWorker int
	ClientID          string
	ClientSecret      string
}

// ResultStats tracks statistics from load test
type ResultStats struct {
	TotalRequests  int64
	SuccessCount   int64
	FailureCount   int64
	TotalDuration  time.Duration
	MinLatency     time.Duration
	MaxLatency     time.Duration
	AvgLatency     time.Duration
	RequestsPerSec float64
	SuccessRate    float64
}

// EndpointStats tracks per-endpoint statistics
type EndpointStats struct {
	Endpoint     string
	Requests     int64
	Successes    int64
	Failures     int64
	TotalLatency time.Duration
	AvgLatency   time.Duration
	MinLatency   time.Duration
	MaxLatency   time.Duration
	SuccessRate  float64
}

// LoadTester handles load testing
type LoadTester struct {
	config        LoadTestConfig
	client        *http.Client
	statsLock     sync.Mutex
	endpointStats map[string]*EndpointStats
	authToken     string
	tokenLock     sync.Mutex
}

// NewLoadTester creates a new load tester
func NewLoadTester(config LoadTestConfig) *LoadTester {
	return &LoadTester{
		config: config,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		endpointStats: make(map[string]*EndpointStats),
	}
}

// GenerateToken generates a JWT token by calling the token endpoint
func (lt *LoadTester) GenerateToken() (string, error) {
	lt.tokenLock.Lock()
	defer lt.tokenLock.Unlock()

	payload := map[string]string{
		"grant_type":    "client_credentials",
		"client_id":     lt.config.ClientID,
		"client_secret": lt.config.ClientSecret,
	}

	body, _ := json.Marshal(payload)
	resp, err := lt.client.Post(
		lt.config.BaseURL+"/token",
		"application/json",
		bytes.NewBuffer(body),
	)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var tokenResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&tokenResp)

	token, ok := tokenResp["access_token"].(string)
	if !ok {
		return "", fmt.Errorf("failed to extract token from response")
	}

	return token, nil
}

// TestTokenEndpoint tests the /token endpoint
func (lt *LoadTester) TestTokenEndpoint() error {
	payload := map[string]string{
		"grant_type":    "client_credentials",
		"client_id":     lt.config.ClientID,
		"client_secret": lt.config.ClientSecret,
	}

	body, _ := json.Marshal(payload)
	start := time.Now()

	resp, err := lt.client.Post(
		lt.config.BaseURL+"/token",
		"application/json",
		bytes.NewBuffer(body),
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	duration := time.Since(start)
	lt.recordStat("POST /token", resp.StatusCode == 200, duration)

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// TestValidateEndpoint tests the /validate endpoint
func (lt *LoadTester) TestValidateEndpoint(token string) error {
	// Generate a token if not provided
	if token == "" {
		t, err := lt.GenerateToken()
		if err != nil {
			return fmt.Errorf("failed to generate token: %v", err)
		}
		token = t
	}

	req, _ := http.NewRequest("POST", lt.config.BaseURL+"/validate", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Forwarded-For", "http://localhost:3000/api/users")

	start := time.Now()
	resp, err := lt.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	duration := time.Since(start)
	lt.recordStat("POST /validate", resp.StatusCode == 200, duration)

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("validate endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// TestRevokeEndpoint tests the /revoke endpoint
func (lt *LoadTester) TestRevokeEndpoint(token string) error {
	// Generate a fresh token for revocation
	if token == "" {
		t, err := lt.GenerateToken()
		if err != nil {
			return fmt.Errorf("failed to generate token: %v", err)
		}
		token = t
	}

	req, _ := http.NewRequest("POST", lt.config.BaseURL+"/revoke", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	start := time.Now()
	resp, err := lt.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	duration := time.Since(start)
	lt.recordStat("POST /revoke", resp.StatusCode == 200, duration)

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("revoke endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// recordStat records statistics for an endpoint
func (lt *LoadTester) recordStat(endpoint string, success bool, duration time.Duration) {
	lt.statsLock.Lock()
	defer lt.statsLock.Unlock()

	if _, ok := lt.endpointStats[endpoint]; !ok {
		lt.endpointStats[endpoint] = &EndpointStats{
			Endpoint:   endpoint,
			MinLatency: duration,
			MaxLatency: duration,
		}
	}

	stat := lt.endpointStats[endpoint]
	stat.Requests++
	stat.TotalLatency += duration

	if duration < stat.MinLatency {
		stat.MinLatency = duration
	}
	if duration > stat.MaxLatency {
		stat.MaxLatency = duration
	}

	if success {
		stat.Successes++
	} else {
		stat.Failures++
	}

	stat.AvgLatency = stat.TotalLatency / time.Duration(stat.Requests)
	stat.SuccessRate = float64(stat.Successes) / float64(stat.Requests) * 100
}

// RunLoadTest executes the load test
func (lt *LoadTester) RunLoadTest() ResultStats {
	var wg sync.WaitGroup
	startTime := time.Now()

	// Generate initial token
	_, _ = lt.GenerateToken()

	// Distribute work across workers
	for i := 0; i < lt.config.Concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for j := 0; j < lt.config.RequestsPerWorker; j++ {
				// Cycle through endpoints
				switch j % 3 {
				case 0:
					lt.TestTokenEndpoint()
				case 1:
					newToken, _ := lt.GenerateToken()
					lt.TestValidateEndpoint(newToken)
				case 2:
					newToken, _ := lt.GenerateToken()
					lt.TestRevokeEndpoint(newToken)
				}

				// Small delay to avoid overwhelming the server
				time.Sleep(time.Millisecond * time.Duration(rand.Intn(50)))
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(startTime)

	// Calculate aggregate statistics
	totalRequests := int64(0)
	successCount := int64(0)
	minLatency := time.Duration(1<<63 - 1)
	maxLatency := time.Duration(0)
	totalLatency := time.Duration(0)

	for _, stat := range lt.endpointStats {
		totalRequests += stat.Requests
		successCount += stat.Successes
		if stat.MinLatency < minLatency {
			minLatency = stat.MinLatency
		}
		if stat.MaxLatency > maxLatency {
			maxLatency = stat.MaxLatency
		}
		totalLatency += stat.TotalLatency
	}

	avgLatency := time.Duration(0)
	if totalRequests > 0 {
		avgLatency = totalLatency / time.Duration(totalRequests)
	}

	return ResultStats{
		TotalRequests:  totalRequests,
		SuccessCount:   successCount,
		FailureCount:   totalRequests - successCount,
		TotalDuration:  duration,
		MinLatency:     minLatency,
		MaxLatency:     maxLatency,
		AvgLatency:     avgLatency,
		RequestsPerSec: float64(totalRequests) / duration.Seconds(),
		SuccessRate:    float64(successCount) / float64(totalRequests) * 100,
	}
}

// PrintResults prints load test results
func (lt *LoadTester) PrintResults(results ResultStats) {
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("LOAD TEST RESULTS")
	fmt.Println(strings.Repeat("=", 80))

	fmt.Printf("\nOverall Statistics:\n")
	fmt.Printf("  Total Requests:    %d\n", results.TotalRequests)
	fmt.Printf("  Successful:        %d (%.2f%%)\n", results.SuccessCount, results.SuccessRate)
	fmt.Printf("  Failed:            %d (%.2f%%)\n", results.FailureCount, 100-results.SuccessRate)
	fmt.Printf("  Total Duration:    %v\n", results.TotalDuration)
	fmt.Printf("  Requests/sec:      %.2f\n", results.RequestsPerSec)

	fmt.Printf("\nLatency Statistics:\n")
	fmt.Printf("  Min:               %v\n", results.MinLatency)
	fmt.Printf("  Max:               %v\n", results.MaxLatency)
	fmt.Printf("  Avg:               %v\n", results.AvgLatency)

	fmt.Printf("\nPer-Endpoint Statistics:\n")
	fmt.Printf("%-20s %-10s %-10s %-10s %-15s %-15s\n", "Endpoint", "Requests", "Success", "Failed", "Avg Latency", "Success Rate")
	fmt.Printf(strings.Repeat("-", 90) + "\n")

	for _, stat := range lt.endpointStats {
		fmt.Printf("%-20s %-10d %-10d %-10d %-15v %-15.2f%%\n",
			stat.Endpoint,
			stat.Requests,
			stat.Successes,
			stat.Failures,
			stat.AvgLatency,
			stat.SuccessRate,
		)
	}

	fmt.Println(strings.Repeat("=", 80) + "\n")
}

func main() {
	// Parse command line flags
	baseURL := flag.String("url", "http://localhost:8080", "Base URL of the auth server")
	concurrency := flag.Int("concurrency", 10, "Number of concurrent workers")
	requests := flag.Int("requests", 100, "Requests per worker")
	clientID := flag.String("client-id", "test-client-1", "Client ID for testing")
	clientSecret := flag.String("client-secret", "secret-key-12345", "Client secret for testing")

	flag.Parse()

	config := LoadTestConfig{
		BaseURL:           *baseURL,
		Concurrency:       *concurrency,
		RequestsPerWorker: *requests,
		ClientID:          *clientID,
		ClientSecret:      *clientSecret,
	}

	fmt.Printf("\n%s\n", strings.Repeat("=", 80))
	fmt.Println("AUTH SERVER LOAD TEST")
	fmt.Printf("%s\n\n", strings.Repeat("=", 80))
	fmt.Printf("Configuration:\n")
	fmt.Printf("  Base URL:          %s\n", config.BaseURL)
	fmt.Printf("  Concurrency:       %d workers\n", config.Concurrency)
	fmt.Printf("  Requests/worker:   %d\n", config.RequestsPerWorker)
	fmt.Printf("  Total requests:    %d\n", config.Concurrency*config.RequestsPerWorker)
	fmt.Printf("  Client ID:         %s\n", config.ClientID)
	fmt.Printf("\n")

	tester := NewLoadTester(config)

	// Run load test
	fmt.Println("Starting load test...")
	start := time.Now()
	results := tester.RunLoadTest()

	// Print results
	tester.PrintResults(results)

	fmt.Printf("Load test completed in %v\n", time.Since(start))
}
