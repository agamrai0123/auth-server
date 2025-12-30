package auth

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestLoggingMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(LoggingMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	// Create request
	req, _ := http.NewRequest("GET", "/test", nil)
	recorder := httptest.NewRecorder()

	// Execute
	router.ServeHTTP(recorder, req)

	// Verify
	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", recorder.Code)
	}

	// Check that request ID was set in context
	// (We can't directly check context in httptest, but we verify the middleware runs)
}

func TestCORSMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(CORSMiddleware())
	router.OPTIONS("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	// Test OPTIONS request
	req, _ := http.NewRequest("OPTIONS", "/test", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != 204 {
		t.Errorf("Expected status 204 for OPTIONS, got %d", recorder.Code)
	}

	// Check CORS headers
	corsOrigin := recorder.Header().Get("Access-Control-Allow-Origin")
	if corsOrigin != "*" {
		t.Errorf("Expected CORS origin *, got %s", corsOrigin)
	}

	corsMethods := recorder.Header().Get("Access-Control-Allow-Methods")
	if corsMethods == "" {
		t.Errorf("Expected CORS methods header to be set")
	}
}

func TestCORSMiddleware_GET(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(CORSMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", recorder.Code)
	}
}

func TestRecoveryMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(RecoveryMiddleware())
	router.GET("/panic", func(c *gin.Context) {
		panic("test panic")
	})

	req, _ := http.NewRequest("GET", "/panic", nil)
	recorder := httptest.NewRecorder()

	// Should not panic
	router.ServeHTTP(recorder, req)

	if recorder.Code != 500 {
		t.Errorf("Expected status 500 for panic recovery, got %d", recorder.Code)
	}
}

func TestGetRequestLogger(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(LoggingMiddleware())
	router.GET("/test", func(c *gin.Context) {
		logger := GetRequestLogger(c)
		// Verify logger is not nil
		if logger.Error() == nil {
			t.Errorf("Expected logger to be set in context")
		}
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", recorder.Code)
	}
}

func TestGetRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(LoggingMiddleware())
	router.GET("/test", func(c *gin.Context) {
		requestID := GetRequestID(c)
		if requestID == "" {
			t.Errorf("Expected request ID to be set")
		}
		c.JSON(http.StatusOK, gin.H{"request_id": requestID})
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", recorder.Code)
	}
}

func TestGetRequestID_NotSet(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		requestID := GetRequestID(c)
		if requestID != "" {
			t.Errorf("Expected empty request ID when not set")
		}
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
}

func TestGetLogger(t *testing.T) {
	// Reset the once to test fresh logger
	onceLog = sync.Once{}

	logger := GetLogger()

	// Verify logger is initialized (not checking nil since zerolog.Logger
	// is a struct and can't be nil, but we can check it's valid)
	logEvent := logger.Info()
	if logEvent == nil {
		t.Errorf("Expected logger to be initialized")
	}
}

func TestMiddlewareOrder(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Verify that middleware can be applied in order
	router := gin.New()
	router.Use(LoggingMiddleware())
	router.Use(CORSMiddleware())
	router.Use(RecoveryMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", recorder.Code)
	}
}

func TestLoggingMiddleware_ErrorStatusCode(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(LoggingMiddleware())
	router.GET("/error", func(c *gin.Context) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "test error"})
	})

	req, _ := http.NewRequest("GET", "/error", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", recorder.Code)
	}
}
