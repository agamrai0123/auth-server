package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/zerolog/log"
)

// Generate random string
func generateRandomString(length int) string {
	bytes := make([]byte, length)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// // Find client by ID
// func (as *authServer) findClient(clientID string) (*Client, error) {
// 	log.Info().Msg("in findClient")
// 	return as.storage.GetClientByID(clientID)
// }

// // Check if token is revoked
// func (as *authServer) isTokenRevoked(tokenID string) (bool, error) {
// 	log.Info().Msg("in isTokenRevoked")
// 	return as.storage.IsTokenRevoked(tokenID)
// }

// Generate JWT token with cached client scopes and async token persistence
func (as *authServer) generateJWT(clientID string) (string, string, error) {
	// ✅ Try cache first for client scopes (avoids DB query ~99% of time)
	var scopes []string
	if client, found := as.clientCache.Get(clientID); found {
		if client == nil {
			log.Error().Str("client_id", clientID).Msg("Cache returned nil client")
			return "", "", fmt.Errorf("cached client is nil")
		}
		scopes = client.AllowedScopes
	} else {
		// Cache miss - fetch from database
		var err error
		scopes, err = as.getClientScopes(clientID)
		if err != nil {
			log.Error().Err(err).Str("client_id", clientID).Msg("Failed to fetch client scopes")
			return "", "", fmt.Errorf("failed to fetch scopes: %w", err)
		}
	}

	tokenID := generateRandomString(16)
	now := time.Now()
	expiresAt := now.Add(time.Minute * 2)

	claims := Claims{
		ClientID: clientID,
		TokenID:  tokenID,
		Scope:    scopes,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "auth-server",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(as.jwtSecret)
	if err != nil {
		log.Error().Err(err).Str("client_id", clientID).Msg("Failed to sign JWT token")
		return "", "", err
	}

	// Store token info for async batch insertion
	tokenInfo := Token{
		TokenID:   tokenID,
		ClientID:  clientID,
		IssuedAt:  now,
		ExpiresAt: expiresAt,
		Revoked:   false,
	}

	// ✅ Queue token for batch insertion (non-blocking)
	as.tokenBatcher.Add(tokenInfo)

	return tokenString, tokenID, nil
}

// Validate JWT token
func (as *authServer) validateJWT(tokenString string) (*Claims, error) {
	log.Debug().Msg("Validating JWT token signature and claims")
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return as.jwtSecret, nil
	})

	if err != nil {
		log.Warn().Err(err).Msg("JWT token parsing failed")
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		log.Debug().Str("client_id", claims.ClientID).Str("token_id", claims.TokenID).Msg("JWT token signature valid")

		// Check if token is revoked
		revoked, err := as.isTokenRevoked(claims.TokenID)
		if err != nil {
			log.Warn().Err(err).Str("token_id", claims.TokenID).Msg("Failed to check token revocation status")
			return nil, fmt.Errorf("error checking token revocation: %v", err)
		}
		if revoked {
			log.Warn().Str("client_id", claims.ClientID).Str("token_id", claims.TokenID).Msg("Token has been revoked")
			return nil, fmt.Errorf("token has been revoked")
		}
		log.Debug().Str("client_id", claims.ClientID).Str("token_id", claims.TokenID).Msg("Token is valid and not revoked")
		return claims, nil
	}

	log.Warn().Msg("JWT token validation failed - invalid token")
	return nil, fmt.Errorf("invalid token")
}
