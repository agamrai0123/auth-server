package main

import (
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
	"time"
)

var requestCount int64

func handleToken(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&requestCount, 1)

	// Simulate processing with small delay
	time.Sleep(time.Millisecond)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"token":"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...","expires_in":3600}`)
}

func handleValidate(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&requestCount, 1)

	// Simulate processing with small delay
	time.Sleep(time.Millisecond)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"valid":true,"client_id":"test-client-1"}`)
}

func handleRevoke(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&requestCount, 1)

	// Simulate processing with small delay
	time.Sleep(time.Millisecond)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"success":true}`)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"healthy"}`)
}

func main() {
	http.HandleFunc("/token", handleToken)
	http.HandleFunc("/validate", handleValidate)
	http.HandleFunc("/revoke", handleRevoke)
	http.HandleFunc("/health", handleHealth)

	// Stats endpoint
	http.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		count := atomic.LoadInt64(&requestCount)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"total_requests":%d}`, count)
	})

	log.Println("Mock auth server listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
