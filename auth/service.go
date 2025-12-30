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

// NewAuthServer creates a new auth server instance with proper error handling
func NewAuthServer() *authServer {
	logger := GetLogger()
	ctx, cancel := context.WithCancel(context.Background())

	// Initialize database connection
	dbURL := fmt.Sprintf("http://%s:%d", AppConfig.Database.Host, AppConfig.Database.Port)
	db, err := newDbClient(dbURL)
	if err != nil {
		logger.Error().
			Err(err).
			Str("db_url", dbURL).
			Msg("Failed to initialize database client")
		// Don't fatal here, allow graceful degradation
		cancel()
		return nil
	}

	logger.Info().
		Str("db_url", dbURL).
		Msg("Database client initialized successfully")

	return &authServer{
		jwtSecret: JWTsecret,
		ctx:       ctx,
		cancel:    cancel,
		db:        db,
	}
}

// Shutdown gracefully shuts down the auth server
func (s *authServer) Shutdown(ctx context.Context) error {
	logger := GetLogger()

	// Close database connection
	if s.db != nil {
		if err := s.db.Close(); err != nil {
			logger.Warn().Err(err).Msg("Error closing database connection")
		}
	}

	// Cancel context
	if s.cancel != nil {
		s.cancel()
	}

	// Shutdown HTTP server
	if s.httpSrv != nil {
		logger.Info().Msg("Shutting down HTTP server")
		if err := s.httpSrv.Shutdown(ctx); err != nil {
			return fmt.Errorf("HTTP server shutdown error: %w", err)
		}
		logger.Info().Msg("HTTP server shutdown complete")
	}

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
