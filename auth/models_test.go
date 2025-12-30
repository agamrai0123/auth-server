package auth

import (
	"testing"
	"time"
)

func TestTokenRequestStruct(t *testing.T) {
	tokenReq := TokenRequest{
		GrantType:    "client_credentials",
		ClientID:     "test-client",
		ClientSecret: "test-secret",
	}

	if tokenReq.GrantType != "client_credentials" {
		t.Errorf("Expected GrantType='client_credentials', got %s", tokenReq.GrantType)
	}

	if tokenReq.ClientID != "test-client" {
		t.Errorf("Expected ClientID='test-client', got %s", tokenReq.ClientID)
	}

	if tokenReq.ClientSecret != "test-secret" {
		t.Errorf("Expected ClientSecret='test-secret', got %s", tokenReq.ClientSecret)
	}
}

func TestTokenResponseStruct(t *testing.T) {
	response := TokenResponse{
		AccessToken: "test-token",
		TokenType:   "Bearer",
		ExpiresIn:   3600,
	}

	if response.AccessToken != "test-token" {
		t.Errorf("Expected AccessToken='test-token', got %s", response.AccessToken)
	}

	if response.TokenType != "Bearer" {
		t.Errorf("Expected TokenType='Bearer', got %s", response.TokenType)
	}

	if response.ExpiresIn != 3600 {
		t.Errorf("Expected ExpiresIn=3600, got %d", response.ExpiresIn)
	}
}

func TestErrorResponseStruct(t *testing.T) {
	errResp := ErrorResponse{
		Error:            "invalid_request",
		ErrorDescription: "Missing required field",
	}

	if errResp.Error != "invalid_request" {
		t.Errorf("Expected Error='invalid_request', got %s", errResp.Error)
	}

	if errResp.ErrorDescription != "Missing required field" {
		t.Errorf("Expected ErrorDescription, got %s", errResp.ErrorDescription)
	}
}

func TestTokenValidationResponseStruct(t *testing.T) {
	now := time.Now()
	scopes := []string{"https://api.example.com/users", "https://api.example.com/data"}
	validResp := TokenValidationResponse{
		Valid:     true,
		ClientID:  "test-client",
		ExpiresAt: now,
		Scopes:    scopes,
	}

	if validResp.Valid != true {
		t.Errorf("Expected Valid=true, got %v", validResp.Valid)
	}

	if validResp.ClientID != "test-client" {
		t.Errorf("Expected ClientID='test-client', got %s", validResp.ClientID)
	}

	if validResp.ExpiresAt != now {
		t.Errorf("Expected ExpiresAt=%v, got %v", now, validResp.ExpiresAt)
	}

	if len(validResp.Scopes) != 2 {
		t.Errorf("Expected 2 scopes, got %d", len(validResp.Scopes))
	}
}

func TestClientStruct(t *testing.T) {
	scopes := []string{"read", "write"}
	client := Clients{
		ClientID:       "test-client",
		ClientSecret:   "test-secret",
		Name:           "Test Client",
		AccessTokenTTL: 3600,
		AllowedScopes:  scopes,
	}

	if client.ClientID != "test-client" {
		t.Errorf("Expected ClientID='test-client', got %s", client.ClientID)
	}

	if client.ClientSecret != "test-secret" {
		t.Errorf("Expected ClientSecret='test-secret', got %s", client.ClientSecret)
	}

	if client.Name != "Test Client" {
		t.Errorf("Expected Name='Test Client', got %s", client.Name)
	}

	if client.AccessTokenTTL != 3600 {
		t.Errorf("Expected AccessTokenTTL=3600, got %d", client.AccessTokenTTL)
	}

	if len(client.AllowedScopes) != 2 {
		t.Errorf("Expected 2 scopes, got %d", len(client.AllowedScopes))
	}
}

func TestEndpointsStruct(t *testing.T) {
	endpoint := Endpoints{
		ClientID:    "test-client",
		Scope:       "read",
		Method:      "GET",
		Url:         "https://api.example.com/data",
		Description: "Read data endpoint",
		Active:      true,
	}

	if endpoint.ClientID != "test-client" {
		t.Errorf("Expected ClientID='test-client', got %s", endpoint.ClientID)
	}

	if endpoint.Scope != "read" {
		t.Errorf("Expected Scope='read', got %s", endpoint.Scope)
	}

	if endpoint.Method != "GET" {
		t.Errorf("Expected Method='GET', got %s", endpoint.Method)
	}

	if endpoint.Url != "https://api.example.com/data" {
		t.Errorf("Expected Url, got %s", endpoint.Url)
	}

	if endpoint.Active != true {
		t.Errorf("Expected Active=true, got %v", endpoint.Active)
	}
}

func TestAuthServerStruct(t *testing.T) {
	// Just verify the struct can be instantiated
	secret := []byte("test-secret")
	ctx, cancel := createTestContextFunc()
	defer cancel()

	server := &authServer{
		jwtSecret: secret,
		ctx:       ctx,
		cancel:    cancel,
		httpSrv:   nil,
		db:        nil,
	}

	if server.jwtSecret == nil {
		t.Errorf("Expected jwtSecret to be set")
	}

	if server.ctx == nil {
		t.Errorf("Expected context to be set")
	}
}
