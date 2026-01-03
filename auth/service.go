package auth

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

var JWTsecret = []byte("67d81e2c5717548a4ee1bd1e81395746")

// Start initializes and starts the HTTP server
func (s *authServer) Start() {
	router := gin.New()

	// Apply middleware in the correct order
	router.Use(
		LoggingMiddleware(),  // Log all requests
		CORSMiddleware(),     // Handle CORS
		RecoveryMiddleware(), // Handle panics
	)

	// Register routes
	routes(router, s)

	// Configure HTTP server
	port := AppConfig.ServerPort
	if port == "" {
		port = "8080"
	}
	addr := ":" + port

	s.httpSrv = &http.Server{
		Addr:           addr,
		Handler:        router,
		ReadTimeout:    15 * time.Second,
		WriteTimeout:   15 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1MB
	}

	// Start server in goroutine
	go func() {
		logger := GetLogger()
		logger.Info().
			Str("address", addr).
			Msg("Starting HTTP server")

		err := s.httpSrv.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			logger.Error().
				Err(err).
				Str("address", addr).
				Msg("HTTP server error")
		}
	}()
}

// NewAuthServer creates a new auth server instance with comprehensive initialization
// Returns nil if critical initialization fails
func NewAuthServer() *authServer {
	logger := GetLogger()
	ctx, cancel := context.WithCancel(context.Background())

	// Initialize database connection
	// Oracle DSN format: user/password@host:port/service_name
	dbURL := fmt.Sprintf("sys/Oracle123!@%s:%d/XE", AppConfig.Database.Host, AppConfig.Database.Port)
	db, err := newDbClient(dbURL)
	if err != nil {
		logger.Error().
			Err(err).
			Str("db_url", dbURL).
			Msg("Failed to initialize database client")
		cancel()
		return nil
	}

	logger.Info().
		Str("db_url", dbURL).
		Msg("Database client initialized successfully")

	// Initialize client cache (10 minute TTL, max 5000 clients)
	clientCache := NewClientCache(10*time.Minute, 5000)

	// Create auth server instance
	authServer := &authServer{
		jwtSecret:   JWTsecret,
		ctx:         ctx,
		cancel:      cancel,
		db:          db,
		clientCache: clientCache,
	}

	// Initialize token batch writer (batch 1000 tokens, flush every 5 seconds)
	// This must come after authServer is created since it needs a reference to it
	authServer.tokenBatcher = NewTokenBatchWriter(authServer, 1000, 5*time.Second)

	logger.Info().Msg("Auth server initialized successfully")
	return authServer
}

// Shutdown gracefully shuts down the auth server with proper cleanup order
// Timeout context is used to limit shutdown duration
func (s *authServer) Shutdown(ctx context.Context) error {
	logger := GetLogger()

	// Step 1: Stop accepting new token writes (flush any pending)
	if s.tokenBatcher != nil {
		logger.Info().Msg("Stopping token batch writer...")
		s.tokenBatcher.Stop()
	}

	// Step 2: Stop accepting new cache operations
	if s.clientCache != nil {
		logger.Info().Msg("Stopping client cache...")
		s.clientCache.Stop()
	}

	// Step 3: Close database connection
	if s.db != nil {
		logger.Info().Msg("Closing database connection...")
		if err := s.db.Close(); err != nil {
			logger.Warn().Err(err).Msg("Error closing database connection")
		}
	}

	// Step 4: Cancel main context
	if s.cancel != nil {
		s.cancel()
	}

	// Step 5: Shutdown HTTP server
	if s.httpSrv != nil {
		logger.Info().Msg("Shutting down HTTP server...")
		if err := s.httpSrv.Shutdown(ctx); err != nil {
			logger.Error().Err(err).Msg("HTTP server shutdown error")
			return fmt.Errorf("HTTP server shutdown error: %w", err)
		}
		logger.Info().Msg("HTTP server shutdown complete")
	}

	logger.Info().Msg("Auth server shutdown complete")
	return nil
}

// Stop gracefully stops the auth server (deprecated, use Shutdown instead)
func (s *authServer) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := s.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("Error during shutdown")
	}
}
