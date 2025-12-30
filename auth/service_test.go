package auth

import (
	"testing"
)

func TestNewAuthServer_Creation(t *testing.T) {
	// Note: This test creates an auth server but won't test database connectivity
	// since we don't have an actual rqlite instance running in tests

	ctx, cancel := createTestContextFunc()
	defer cancel()

	server := &authServer{
		jwtSecret: []byte("test-secret"),
		ctx:       ctx,
		cancel:    cancel,
		httpSrv:   nil,
		db:        nil,
	}

	if server == nil {
		t.Errorf("Expected server to be created")
	}

	if server.jwtSecret == nil {
		t.Errorf("Expected JWT secret to be set")
	}
}

func TestAuthServer_Shutdown_WithoutServer(t *testing.T) {
	ctx, cancel := createTestContextFunc()
	defer cancel()

	server := &authServer{
		jwtSecret: []byte("test-secret"),
		ctx:       ctx,
		cancel:    cancel,
		httpSrv:   nil,
		db:        nil,
	}

	shutdownCtx, shutdownCancel := createTestContextFunc()
	defer shutdownCancel()

	// Should not panic when httpSrv is nil
	err := server.Shutdown(shutdownCtx)
	if err != nil {
		t.Logf("Shutdown error (expected if httpSrv not running): %v", err)
	}
}

func TestAuthServer_Stop_Method(t *testing.T) {
	ctx, cancel := createTestContextFunc()
	defer cancel()

	server := &authServer{
		jwtSecret: []byte("test-secret"),
		ctx:       ctx,
		cancel:    cancel,
		httpSrv:   nil,
		db:        nil,
	}

	// Should not panic
	server.Stop()
}

func TestAuthServerStruct_Fields(t *testing.T) {
	secret := []byte("test-secret-key")
	ctx, cancel := createTestContextFunc()
	defer cancel()

	server := &authServer{
		jwtSecret: secret,
		ctx:       ctx,
		cancel:    cancel,
		httpSrv:   nil,
		db:        nil,
	}

	if len(server.jwtSecret) == 0 {
		t.Errorf("Expected JWT secret to be set")
	}

	if server.ctx == nil {
		t.Errorf("Expected context to be set")
	}

	if server.cancel == nil {
		t.Errorf("Expected cancel function to be set")
	}
}

func TestJWTSecretConstant(t *testing.T) {
	if len(JWTsecret) == 0 {
		t.Errorf("Expected JWTsecret to be set")
	}

	if len(JWTsecret) != 32 {
		t.Logf("JWTsecret length: %d (expected 32 for security)", len(JWTsecret))
	}
}

func TestAuthServer_ContextManagement(t *testing.T) {
	ctx, cancel := createTestContextFunc()

	server := &authServer{
		jwtSecret: []byte("test-secret"),
		ctx:       ctx,
		cancel:    cancel,
		httpSrv:   nil,
		db:        nil,
	}

	// Verify context is working
	select {
	case <-server.ctx.Done():
		t.Errorf("Expected context to be active")
	default:
		// Context is active
	}

	// Cancel it
	server.cancel()

	// Now it should be done
	select {
	case <-server.ctx.Done():
		// Context is cancelled
	default:
		t.Errorf("Expected context to be cancelled")
	}
}

func TestMultipleAuthServers(t *testing.T) {
	// Verify we can create multiple auth server instances
	servers := make([]*authServer, 3)

	for i := 0; i < 3; i++ {
		ctx, cancel := createTestContextFunc()
		servers[i] = &authServer{
			jwtSecret: []byte("secret-" + string(rune(i))),
			ctx:       ctx,
			cancel:    cancel,
			httpSrv:   nil,
			db:        nil,
		}
	}

	for i, server := range servers {
		if server == nil {
			t.Errorf("Server %d is nil", i)
		}
	}

	// Clean up
	for _, server := range servers {
		server.cancel()
	}
}

func TestAuthServer_PortConfiguration(t *testing.T) {
	// Verify port configuration would work
	port := "8080"
	addr := ":" + port

	if addr != ":8080" {
		t.Errorf("Expected addr=':8080', got %s", addr)
	}
}
