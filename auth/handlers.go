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
	if c.Request.Method != http.MethodPost {
		log.Warn().Str("method", c.Request.Method).Msg("Invalid HTTP method for token endpoint")
		c.String(http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var tokenReq TokenRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&tokenReq); err != nil {
		log.Warn().Err(err).Msg("Failed to decode token request JSON")
		c.Header("Content-Type", "application/json")
		c.Status(http.StatusBadRequest)
		encoder := json.NewEncoder(c.Writer)
		if err := encoder.Encode(ErrorResponse{
			Error:            "invalid_request",
			ErrorDescription: "Invalid JSON format",
		}); err != nil {
			log.Error().Err(err).Msg("Failed to encode error response")
			c.AbortWithError(http.StatusBadRequest, err)
		}
		return
	}

	log.Debug().Str("client_id", tokenReq.ClientID).Str("grant_type", tokenReq.GrantType).Msg("Processing token request")

	// Validate client
	client, err := as.clientByID(tokenReq.ClientID)
	if err != nil {
		log.Warn().Err(err).Str("client_id", tokenReq.ClientID).Msg("Client lookup failed")
		c.Header("Content-Type", "application/json")
		c.Status(http.StatusInternalServerError)
		encoder := json.NewEncoder(c.Writer)
		if err := encoder.Encode(ErrorResponse{
			Error:            "server_error",
			ErrorDescription: "Internal server error",
		}); err != nil {
			log.Error().Err(err).Msg("Failed to encode error response")
			c.AbortWithError(http.StatusInternalServerError, err)
		}
		return
	}

	if client == nil || client.ClientSecret != tokenReq.ClientSecret {
		log.Warn().Str("client_id", tokenReq.ClientID).Msg("Invalid client credentials")
		c.Header("Content-Type", "application/json")
		c.Status(http.StatusUnauthorized)
		encoder := json.NewEncoder(c.Writer)
		if err := encoder.Encode(ErrorResponse{
			Error:            "invalid_client",
			ErrorDescription: "Invalid client credentials",
		}); err != nil {
			log.Error().Err(err).Msg("Failed to encode error response")
			c.AbortWithError(http.StatusUnauthorized, err)
		}
		return
	}

	log.Debug().Str("client_id", tokenReq.ClientID).Msg("Client credentials validated")

	// Handle client credentials grant
	// Scopes are automatically fetched from the client's configuration
	if tokenReq.GrantType == "client_credentials" {
		token, tokenID, err := as.generateJWT(tokenReq.ClientID)
		if err != nil {
			log.Error().Err(err).Str("client_id", tokenReq.ClientID).Msg("Failed to generate JWT token")
			c.Header("Content-Type", "application/json")
			c.Status(http.StatusInternalServerError)
			encoder := json.NewEncoder(c.Writer)
			if err := encoder.Encode(ErrorResponse{
				Error:            "server_error",
				ErrorDescription: "Failed to generate token",
			}); err != nil {
				log.Error().Err(err).Msg("Failed to encode error response")
				c.AbortWithError(http.StatusInternalServerError, err)
			}
			return
		}

		log.Info().Str("client_id", tokenReq.ClientID).Str("token_id", tokenID).Msg("JWT token generated successfully")

		c.Header("Content-Type", "application/json")
		encoder := json.NewEncoder(c.Writer)
		if err := encoder.Encode(TokenResponse{
			AccessToken: token,
			TokenType:   "Bearer",
			ExpiresIn:   2 * 60, // 2 min for testing, use 3600 (1 hour) for production
		}); err != nil {
			log.Error().Err(err).Msg("Failed to encode token response")
			c.AbortWithError(http.StatusInternalServerError, err)
		}
		return
	}

	log.Warn().Str("grant_type", tokenReq.GrantType).Msg("Unsupported grant type")
	c.Header("Content-Type", "application/json")
	c.Status(http.StatusBadRequest)
	encoder := json.NewEncoder(c.Writer)
	if err := encoder.Encode(ErrorResponse{
		Error:            "unsupported_grant_type",
		ErrorDescription: "Grant type not supported",
	}); err != nil {
		log.Error().Err(err).Msg("Failed to encode error response")
		c.AbortWithError(http.StatusBadRequest, err)
	}
}

// Validate token handler
// This endpoint is called by nginx API gateway before forwarding requests to protected resources.
// Nginx includes the X-Forwarded-For header with the requested resource endpoint URL.
func (as *authServer) validateHandler(c *gin.Context) {
	log.Debug().Msg("Processing validate request")
	if c.Request.Method != http.MethodPost {
		log.Warn().Str("method", c.Request.Method).Msg("Invalid HTTP method for validate endpoint")
		c.String(http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Get the requested resource endpoint from nginx x-forwarded-for header
	requestURL := c.Request.Header.Get("X-Forwarded-For")
	if requestURL == "" {
		log.Warn().Msg("Missing X-Forwarded-For header (resource endpoint)")
		c.Header("Content-Type", "application/json")
		c.Status(http.StatusBadRequest)
		encoder := json.NewEncoder(c.Writer)
		if err := encoder.Encode(TokenValidationResponse{
			Valid: false,
		}); err != nil {
			log.Error().Err(err).Msg("Failed to encode validation response")
			c.AbortWithError(http.StatusBadRequest, err)
		}
		return
	}
	log.Debug().Str("resource", requestURL).Msg("Validating access to resource")

	authHeader := c.Request.Header.Get("Authorization")
	if authHeader == "" {
		log.Warn().Str("resource", requestURL).Msg("Missing Authorization header")
		c.Header("Content-Type", "application/json")
		c.Status(http.StatusUnauthorized)
		encoder := json.NewEncoder(c.Writer)
		if err := encoder.Encode(TokenValidationResponse{
			Valid: false,
		}); err != nil {
			log.Error().Err(err).Msg("Failed to encode validation response")
			c.AbortWithError(http.StatusUnauthorized, err)
		}
		return
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenString == authHeader {
		log.Warn().Str("resource", requestURL).Msg("Invalid Bearer token format")
		c.Header("Content-Type", "application/json")
		c.Status(http.StatusUnauthorized)
		encoder := json.NewEncoder(c.Writer)
		if err := encoder.Encode(TokenValidationResponse{
			Valid: false,
		}); err != nil {
			log.Error().Err(err).Msg("Failed to encode validation response")
			c.AbortWithError(http.StatusUnauthorized, err)
		}
		return
	}

	// Validate token
	claims, err := as.validateJWT(tokenString)
	if err != nil {
		log.Warn().Err(err).Str("resource", requestURL).Msg("JWT token validation failed")
		c.Header("Content-Type", "application/json")
		c.Status(http.StatusUnauthorized)
		encoder := json.NewEncoder(c.Writer)
		if err := encoder.Encode(TokenValidationResponse{
			Valid: false,
		}); err != nil {
			log.Error().Err(err).Msg("Failed to encode validation response")
			c.AbortWithError(http.StatusUnauthorized, err)
		}
		return
	}

	log.Debug().Str("client_id", claims.ClientID).Str("resource", requestURL).Msg("JWT claims extracted")

	// Validate that the requested resource URL is within the client's allowed scopes
	// Scopes represent endpoint URLs that the client is allowed to access
	found := slices.Contains(claims.Scope, requestURL)
	if !found {
		log.Warn().
			Str("client_id", claims.ClientID).
			Str("resource", requestURL).
			Strs("allowed_scopes", claims.Scope).
			Msg("Resource not in token scopes - access denied")
		c.Header("Content-Type", "application/json")
		c.Status(http.StatusForbidden)
		encoder := json.NewEncoder(c.Writer)
		if err := encoder.Encode(TokenValidationResponse{
			Valid: false,
		}); err != nil {
			log.Error().Err(err).Msg("Failed to encode validation response")
			c.AbortWithError(http.StatusForbidden, err)
		}
		return
	}

	log.Info().
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
		log.Error().Err(err).Msg("Failed to encode validation response")
		c.AbortWithError(http.StatusBadRequest, err)
	}
}

// // Revoke token handler
func (as *authServer) revokeHandler(c *gin.Context) {
	if c.Request.Method != http.MethodPost {
		log.Warn().Str("method", c.Request.Method).Msg("Invalid HTTP method for revoke endpoint")
		c.String(http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	authHeader := c.Request.Header.Get("Authorization")
	if authHeader == "" {
		log.Warn().Msg("Missing Authorization header for token revocation")
		c.Header("Content-Type", "application/json")
		c.Status(http.StatusUnauthorized)
		encoder := json.NewEncoder(c.Writer)
		if err := encoder.Encode(ErrorResponse{
			Error:            "missing_token",
			ErrorDescription: "Authorization header required",
		}); err != nil {
			log.Error().Err(err).Msg("Failed to encode error response")
			c.AbortWithError(http.StatusUnauthorized, err)
		}
		return
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenString == authHeader {
		log.Warn().Msg("Invalid Bearer token format for revocation")
		c.Header("Content-Type", "application/json")
		c.Status(http.StatusUnauthorized)
		encoder := json.NewEncoder(c.Writer)
		if err := encoder.Encode(ErrorResponse{
			Error:            "invalid_token_format",
			ErrorDescription: "Bearer token required",
		}); err != nil {
			log.Error().Err(err).Msg("Failed to encode error response")
			c.AbortWithError(http.StatusUnauthorized, err)
		}
		return
	}

	// Validate token first
	claims, err := as.validateJWT(tokenString)
	if err != nil {
		log.Warn().Err(err).Msg("JWT token validation failed during revocation")
		c.Header("Content-Type", "application/json")
		c.Status(http.StatusUnauthorized)
		encoder := json.NewEncoder(c.Writer)
		if err := encoder.Encode(ErrorResponse{
			Error:            "invalid_token",
			ErrorDescription: err.Error(),
		}); err != nil {
			log.Error().Err(err).Msg("Failed to encode error response")
			c.AbortWithError(http.StatusUnauthorized, err)
		}
		return
	}

	log.Debug().Str("client_id", claims.ClientID).Str("token_id", claims.TokenID).Msg("Revoking token")

	// Add to revoked tokens
	revokedToken := RevokedToken{
		ClientID:  claims.ClientID,
		TokenID:   claims.TokenID,
		RevokedAt: time.Now(),
	}

	if err := as.revokeToken(revokedToken); err != nil {
		log.Error().Err(err).Str("client_id", claims.ClientID).Str("token_id", claims.TokenID).Msg("Failed to revoke token")
		c.Header("Content-Type", "application/json")
		c.Status(http.StatusInternalServerError)
		encoder := json.NewEncoder(c.Writer)
		if err := encoder.Encode(ErrorResponse{
			Error:            "server_error",
			ErrorDescription: "Failed to revoke token",
		}); err != nil {
			log.Error().Err(err).Msg("Failed to encode error response")
			c.AbortWithError(http.StatusInternalServerError, err)
		}
		return
	}

	log.Info().Str("client_id", claims.ClientID).Str("token_id", claims.TokenID).Msg("Token revoked successfully")

	c.Header("Content-Type", "application/json")
	encoder := json.NewEncoder(c.Writer)
	if err := encoder.Encode(map[string]string{
		"message": "Token revoked successfully",
	}); err != nil {
		log.Error().Err(err).Msg("Failed to encode revocation response")
		c.AbortWithError(http.StatusBadRequest, err)
	}
}
