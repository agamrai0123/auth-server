package auth

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

func (as *authServer) tokenHandler(c *gin.Context) {
	logger := GetRequestLogger(c)

	if c.Request.Method != http.MethodPost {
		logger.Warn().Str("method", c.Request.Method).Msg("Invalid HTTP method for token endpoint")
		RespondWithError(c, ErrBadRequest("Only POST method is allowed"))
		return
	}

	var tokenReq TokenRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&tokenReq); err != nil {
		logger.Warn().Err(err).Msg("Failed to decode token request JSON")
		RespondWithError(c, ErrBadRequest("Invalid JSON format").WithOriginalError(err))
		return
	}

	logger.Debug().Str("client_id", tokenReq.ClientID).Str("grant_type", tokenReq.GrantType).Msg("Processing token request")

	// ✅ Try cache first (in-memory lookup is <1µs on hit)
	var client *Clients
	var err error
	if cachedClient, found := as.clientCache.Get(tokenReq.ClientID); found {
		client = cachedClient
	} else {
		// Cache miss - query database with timeout
		client, err = as.clientByID(tokenReq.ClientID)
		if err != nil {
			logger.Warn().Err(err).Str("client_id", tokenReq.ClientID).Msg("Client lookup failed")
			RespondWithError(c, ErrInternalServerError("Failed to lookup client").WithOriginalError(err))
			return
		}
		// Store in cache for future requests (only cache valid clients)
		if client != nil {
			as.clientCache.Set(tokenReq.ClientID, client)
		}
	}

	if client == nil || client.ClientSecret != tokenReq.ClientSecret {
		logger.Warn().Str("client_id", tokenReq.ClientID).Msg("Invalid client credentials")
		RespondWithError(c, ErrUnauthorizedError("Invalid client credentials"))
		return
	}

	logger.Debug().Str("client_id", tokenReq.ClientID).Msg("Client credentials validated")

	// Handle client credentials grant
	// Scopes are automatically fetched from the client's configuration
	if tokenReq.GrantType == "client_credentials" {
		token, tokenID, err := as.generateJWT(tokenReq.ClientID)
		if err != nil {
			logger.Error().Err(err).Str("client_id", tokenReq.ClientID).Msg("Failed to generate JWT token")
			RespondWithError(c, ErrInternalServerError("Failed to generate token").WithOriginalError(err))
			return
		}

		logger.Info().Str("client_id", tokenReq.ClientID).Str("token_id", tokenID).Msg("JWT token generated successfully")

		c.Header("Content-Type", "application/json")
		encoder := json.NewEncoder(c.Writer)
		if err := encoder.Encode(TokenResponse{
			AccessToken: token,
			TokenType:   "Bearer",
			ExpiresIn:   2 * 60, // 2 min for testing, use 3600 (1 hour) for production
		}); err != nil {
			logger.Error().Err(err).Msg("Failed to encode token response")
			c.AbortWithError(http.StatusInternalServerError, err)
		}
		return
	}

	logger.Warn().Str("grant_type", tokenReq.GrantType).Msg("Unsupported grant type")
	RespondWithError(c, ErrBadRequest("Unsupported grant type"))
}

// Validate token handler
// This endpoint is called by nginx API gateway before forwarding requests to protected resources.
// Nginx includes the X-Forwarded-For header with the requested resource endpoint URL.
func (as *authServer) validateHandler(c *gin.Context) {
	logger := GetRequestLogger(c)
	logger.Debug().Msg("Processing validate request")

	if c.Request.Method != http.MethodPost {
		logger.Warn().Str("method", c.Request.Method).Msg("Invalid HTTP method for validate endpoint")
		c.String(http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Get the requested resource endpoint from nginx x-forwarded-for header
	requestURL := c.Request.Header.Get("X-Forwarded-For")
	if requestURL == "" {
		logger.Warn().Msg("Missing X-Forwarded-For header (resource endpoint)")
		RespondWithError(c, ErrBadRequest("Missing X-Forwarded-For header (resource endpoint)"))
		return
	}
	logger.Debug().Str("resource", requestURL).Msg("Validating access to resource")

	authHeader := c.Request.Header.Get("Authorization")
	if authHeader == "" {
		logger.Warn().Str("resource", requestURL).Msg("Missing Authorization header")
		RespondWithError(c, ErrUnauthorizedError("Missing Authorization header"))
		return
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenString == authHeader {
		logger.Warn().Str("resource", requestURL).Msg("Invalid Bearer token format")
		RespondWithError(c, ErrUnauthorizedError("Bearer token required"))
		return
	}

	// Validate token
	claims, err := as.validateJWT(tokenString)
	if err != nil {
		logger.Warn().Err(err).Str("resource", requestURL).Msg("JWT token validation failed")
		RespondWithError(c, ErrUnauthorizedError("Invalid or expired token").WithOriginalError(err))
		return
	}

	logger.Debug().Str("client_id", claims.ClientID).Str("resource", requestURL).Msg("JWT claims extracted")

	// Validate that the requested resource URL is within the client's allowed scopes
	// Scopes represent endpoint URLs that the client is allowed to access
	found := slices.Contains(claims.Scope, requestURL)
	if !found {
		logger.Warn().
			Str("client_id", claims.ClientID).
			Str("resource", requestURL).
			Strs("allowed_scopes", claims.Scope).
			Msg("Resource not in token scopes - access denied")
		RespondWithError(c, ErrForbiddenError("Resource not in token scopes"))
		return
	}

	logger.Info().
		Str("client_id", claims.ClientID).
		Str("resource", requestURL).
		Time("expires_at", claims.ExpiresAt.Time).
		Msg("Token validated for resource - access granted")

	c.Header("Content-Type", "application/json")
	encoder := json.NewEncoder(c.Writer)
	if err := encoder.Encode(TokenValidationResponse{
		Valid:     true,
		ClientID:  claims.ClientID,
		ExpiresAt: claims.ExpiresAt.Time,
		Scopes:    claims.Scope,
	}); err != nil {
		logger.Error().Err(err).Msg("Failed to encode validation response")
		c.AbortWithError(http.StatusBadRequest, err)
	}
}

// // Revoke token handler
func (as *authServer) revokeHandler(c *gin.Context) {
	logger := GetRequestLogger(c)

	if c.Request.Method != http.MethodPost {
		logger.Warn().Str("method", c.Request.Method).Msg("Invalid HTTP method for revoke endpoint")
		c.String(http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	authHeader := c.Request.Header.Get("Authorization")
	if authHeader == "" {
		logger.Warn().Msg("Missing Authorization header for token revocation")
		RespondWithError(c, ErrUnauthorizedError("Authorization header required"))
		return
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenString == authHeader {
		logger.Warn().Msg("Invalid Bearer token format for revocation")
		RespondWithError(c, ErrUnauthorizedError("Bearer token required"))
		return
	}

	// Validate token first
	claims, err := as.validateJWT(tokenString)
	if err != nil {
		logger.Warn().Err(err).Msg("JWT token validation failed during revocation")
		RespondWithError(c, ErrUnauthorizedError("Invalid or expired token").WithOriginalError(err))
		return
	}

	logger.Debug().Str("client_id", claims.ClientID).Str("token_id", claims.TokenID).Msg("Revoking token")

	// Add to revoked tokens
	revokedToken := RevokedToken{
		ClientID:  claims.ClientID,
		TokenID:   claims.TokenID,
		RevokedAt: time.Now(),
	}

	if err := as.revokeToken(revokedToken); err != nil {
		logger.Error().Err(err).Str("client_id", claims.ClientID).Str("token_id", claims.TokenID).Msg("Failed to revoke token")
		RespondWithError(c, ErrInternalServerError("Failed to revoke token").WithOriginalError(err))
		return
	}

	logger.Info().Str("client_id", claims.ClientID).Str("token_id", claims.TokenID).Msg("Token revoked successfully")

	c.Header("Content-Type", "application/json")
	encoder := json.NewEncoder(c.Writer)
	if err := encoder.Encode(map[string]string{
		"message": "Token revoked successfully",
	}); err != nil {
		log.Error().Err(err).Msg("Failed to encode revocation response")
		c.AbortWithError(http.StatusBadRequest, err)
	}
}
