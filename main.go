package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"auth-server/auth"

	_ "github.com/godror/godror" // Register Oracle driver
)

const (
	gracefulShutdownTimeout = 30 * time.Second
)

func main() {
	// Load configuration
	if err := auth.ReadConfiguration(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	logger := auth.GetLogger()

	logger.Info().
		Str("version", auth.AppConfig.Version).
		Str("server_port", auth.AppConfig.ServerPort).
		Msg("Starting auth server")

	// Create auth server instance
	authServer := auth.NewAuthServer()
	if authServer == nil {
		logger.Fatal().Msg("Failed to create auth server instance")
	}

	// Start the server
	authServer.Start()

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for shutdown signal
	sig := <-sigChan
	logger.Info().Str("signal", sig.String()).Msg("Received shutdown signal")

	// Graceful shutdown with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), gracefulShutdownTimeout)
	defer cancel()

	if err := authServer.Shutdown(shutdownCtx); err != nil {
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error().Err(err).Msg("Error during server shutdown")
			os.Exit(1)
		}
	}

	logger.Info().Msg("Auth server stopped gracefully")
}
