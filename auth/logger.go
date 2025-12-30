package auth

import (
	"io"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/rs/zerolog/pkgerrors"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	onceLog sync.Once
	// MultiWriter allows writing to both file and stdout
	logWriter io.Writer
)

// GetLogger returns the configured logger instance
func GetLogger() zerolog.Logger {
	onceLog.Do(func() {
		// Configure zerolog settings
		zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
		zerolog.TimeFieldFormat = time.RFC3339Nano

		// Set up rotating file logger
		rotatingLog := &lumberjack.Logger{
			Filename:   AppConfig.Logging.Path,
			MaxSize:    AppConfig.Logging.MaxSizeMB,
			MaxBackups: AppConfig.Logging.MaxBackups,
			MaxAge:     AppConfig.Logging.MaxAgeDays,
			Compress:   AppConfig.Logging.Compress,
		}

		// In development, write to both stdout and file. In production, just file.
		if AppConfig.Environment == "development" {
			logWriter = io.MultiWriter(os.Stdout, rotatingLog)
		} else {
			logWriter = rotatingLog
		}

		// Create logger with context
		logger := zerolog.New(logWriter).
			Level(zerolog.Level(AppConfig.Logging.Level)).
			With().
			Timestamp().
			Str("service", "auth_server").
			Str("version", AppConfig.Version).
			Str("environment", AppConfig.Environment).
			Logger()

		log.Logger = logger
		log.Info().
			Str("log_path", AppConfig.Logging.Path).
			Int("log_level", AppConfig.Logging.Level).
			Msg("Logger initialized for auth_server")
	})

	return log.Logger
}

// LoggingMiddleware creates a Gin middleware that logs all HTTP requests
func LoggingMiddleware() gin.HandlerFunc {
	hostname, _ := os.Hostname()
	processID := os.Getpid()

	return func(c *gin.Context) {
		start := time.Now()
		requestID := uuid.New().String()

		// Create request-specific logger
		logger := log.With().
			Str("request_id", requestID).
			Str("client_ip", c.ClientIP()).
			Str("host", hostname).
			Int("pid", processID).
			Str("user_agent", c.Request.UserAgent()).
			Logger()

		c.Set("logger", logger)
		c.Set("request_id", requestID)

		// Log incoming request
		logger.Debug().
			Str("method", c.Request.Method).
			Str("path", c.Request.URL.Path).
			Str("query", c.Request.URL.RawQuery).
			Msg("Incoming request")

		// Continue with next middleware
		c.Next()

		// Log request completion
		duration := time.Since(start)
		statusCode := c.Writer.Status()
		responseSize := c.Writer.Size()

		// Determine log level based on status code
		logLevel := getLogLevelForStatus(statusCode)

		logLevel().
			Str("method", c.Request.Method).
			Str("path", c.Request.URL.Path).
			Int("status", statusCode).
			Int("response_size_bytes", responseSize).
			Float64("duration_ms", float64(duration.Microseconds())/1000).
			Msg("Request completed")
	}
}

// getLogLevelForStatus returns appropriate log level based on HTTP status code
func getLogLevelForStatus(statusCode int) func() *zerolog.Event {
	switch {
	case statusCode >= 500:
		return log.Error
	case statusCode >= 400:
		return log.Warn
	case statusCode >= 300:
		return log.Debug
	default:
		return log.Info
	}
}

// CORSMiddleware adds CORS headers to responses
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// RecoveryMiddleware handles panics and logs them
func RecoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				logger, ok := c.Get("logger")
				if !ok {
					logger = log.Logger
				}

				requestLogger := logger.(zerolog.Logger)
				requestLogger.Error().
					Interface("panic", err).
					Str("path", c.Request.URL.Path).
					Str("method", c.Request.Method).
					Msg("Request panic recovered")

				c.JSON(500, gin.H{
					"error": "Internal server error",
				})
			}
		}()
		c.Next()
	}
}

// GetRequestLogger retrieves the request-specific logger from context
func GetRequestLogger(c *gin.Context) zerolog.Logger {
	logger, ok := c.Get("logger")
	if !ok {
		return log.Logger
	}
	return logger.(zerolog.Logger)
}

// GetRequestID retrieves the request ID from context
func GetRequestID(c *gin.Context) string {
	requestID, ok := c.Get("request_id")
	if !ok {
		return ""
	}
	return requestID.(string)
}
