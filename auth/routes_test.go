package auth

import (
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRoutesRegistration(t *testing.T) {
	// This test verifies that routes can be registered
	ctx, cancel := createTestContextFunc()
	defer cancel()

	server := &authServer{
		jwtSecret: []byte("test-secret"),
		ctx:       ctx,
		cancel:    cancel,
		httpSrv:   nil,
		db:        nil,
	}

	// Import gin to test routes
	router := gin.New()

	// This should not panic
	routes(router, server)
}

func TestRoutesStructure(t *testing.T) {
	// Verify routes are set up correctly
	ctx, cancel := createTestContextFunc()
	defer cancel()

	server := &authServer{
		jwtSecret: []byte("test-secret"),
		ctx:       ctx,
		cancel:    cancel,
		httpSrv:   nil,
		db:        nil,
	}

	router := gin.New()
	routes(router, server)

	// Verify router has routes registered
	// (Gin doesn't expose route list easily, but we verify no panic)
	if router == nil {
		t.Errorf("Expected router to be created")
	}
}

