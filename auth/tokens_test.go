package auth

import (
	"testing"
	"time"
)

func TestGenerateRandomString(t *testing.T) {
	tests := []struct {
		length int
		name   string
	}{
		{16, "16 bytes"},
		{32, "32 bytes"},
		{64, "64 bytes"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := generateRandomString(test.length)

			if len(result) != test.length*2 {
				t.Errorf("Expected length %d, got %d", test.length*2, len(result))
			}

			// Should be hex string
			for _, c := range result {
				if !isHexChar(c) {
					t.Errorf("Expected hex character, got %c", c)
				}
			}
		})
	}
}

func TestGenerateRandomString_Uniqueness(t *testing.T) {
	str1 := generateRandomString(16)
	str2 := generateRandomString(16)

	if str1 == str2 {
		t.Errorf("Generated strings should be unique")
	}
}

func TestGenerateRandomString_Empty(t *testing.T) {
	result := generateRandomString(0)
	if result != "" {
		t.Errorf("Expected empty string for length 0, got %s", result)
	}
}

func TestTokenStruct(t *testing.T) {
	now := time.Now()
	token := Token{
		TokenID:   "test-token-id",
		ClientID:  "test-client",
		IssuedAt:  now,
		ExpiresAt: now.Add(time.Hour),
		Revoked:   false,
	}

	if token.TokenID != "test-token-id" {
		t.Errorf("Expected TokenID='test-token-id', got %s", token.TokenID)
	}

	if token.ClientID != "test-client" {
		t.Errorf("Expected ClientID='test-client', got %s", token.ClientID)
	}

	if token.Revoked != false {
		t.Errorf("Expected Revoked=false, got %v", token.Revoked)
	}
}

func TestRevokedTokenStruct(t *testing.T) {
	now := time.Now()
	revoked := RevokedToken{
		ClientID:  "test-client",
		TokenID:   "test-token-id",
		RevokedAt: now,
	}

	if revoked.ClientID != "test-client" {
		t.Errorf("Expected ClientID='test-client', got %s", revoked.ClientID)
	}

	if revoked.TokenID != "test-token-id" {
		t.Errorf("Expected TokenID='test-token-id', got %s", revoked.TokenID)
	}

	if revoked.RevokedAt != now {
		t.Errorf("Expected RevokedAt=%v, got %v", now, revoked.RevokedAt)
	}
}

func TestClaimsStruct(t *testing.T) {
	scopes := []string{"read", "write"}
	claims := Claims{
		ClientID: "test-client",
		TokenID:  "test-token",
		Scope:    scopes,
	}

	if claims.ClientID != "test-client" {
		t.Errorf("Expected ClientID='test-client', got %s", claims.ClientID)
	}

	if claims.TokenID != "test-token" {
		t.Errorf("Expected TokenID='test-token', got %s", claims.TokenID)
	}

	if len(claims.Scope) != 2 {
		t.Errorf("Expected 2 scopes, got %d", len(claims.Scope))
	}

	if claims.Scope[0] != "read" {
		t.Errorf("Expected first scope='read', got %s", claims.Scope[0])
	}
}

// Helper function
func isHexChar(c rune) bool {
	return (c >= '0' && c <= '9') ||
		(c >= 'a' && c <= 'f') ||
		(c >= 'A' && c <= 'F')
}
