package auth

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// type Storage interface {
// 	// Client operations
// 	GetClients() ([]Client, error)
// 	SaveClients(clients []Client) error
// 	GetClientByID(clientID string) (*Client, error)

// 	// Token operations
// 	GetTokens() ([]Token, error)
// 	SaveTokens(tokens []Token) error
// 	SaveToken(token Token) error
// 	GetTokenByID(tokenID string) (*Token, error)

// 	// Revoked token operations
// 	GetRevokedTokens() ([]RevokedToken, error)
// 	SaveRevokedTokens(tokens []RevokedToken) error
// 	AddRevokedToken(token RevokedToken) error
// 	IsTokenRevoked(tokenID string) (bool, error)

// 	// Endpoints
// 	GetEndpoints() ([]Endpoint, error)
// 	GetEndpoint(clientId, scope string) (*Endpoint, error)
// }

// // JSONStorage implements Storage interface using JSON files
// type JSONStorage struct {
// 	clientsFile       string
// 	tokensFile        string
// 	revokedTokensFile string
// 	endpointsFile     string
// }

type authServer struct {
	// storage   Storage
	jwtSecret []byte
	ctx       context.Context
	cancel    context.CancelFunc
	httpSrv   *http.Server
	db        *sql.DB
}

type Clients struct {
	ClientID       string
	ClientSecret   string
	Name           string
	AccessTokenTTL int32
	AllowedScopes  []string
}

type Endpoints struct {
	ClientID    string `json:"client_id"`
	Scope       string `json:"scope"`
	Method      string `json:"method"`
	Url         string `json:"api_url"`
	Description string `json:"description"`
	Active      bool   `json:"active"`
}

type Token struct {
	TokenID   string    `json:"token_id"`
	ClientID  string    `json:"client_id"`
	IssuedAt  time.Time `json:"issued_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Revoked   bool      `json:"revoked"`
	RevokedAt time.Time
}

type RevokedToken struct {
	ClientID  string    `json:"client_id"`
	TokenID   string    `json:"token_id"`
	RevokedAt time.Time `json:"revoked_at"`
}

// JWT Claims
type Claims struct {
	ClientID string   `json:"client_id"`
	TokenID  string   `json:"token_id"`
	Scope    []string `json:"scope"`
	jwt.RegisteredClaims
}

type TokenRequest struct {
	GrantType    string `json:"grant_type"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"`
	// AuthCode string `json:"auth_code"`
	// Method       string `json:"method"`
	// Scope        string `json:"scope"`
	// Audience     string `json:"aud"`
	// RefreshToken string `json:"refresh_token,omitempty"`
}

type ErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

type TokenValidationResponse struct {
	Valid     bool      `json:"valid"`
	ClientID  string    `json:"client_id"`
	ExpiresAt time.Time `json:"expires_at"`
	Scopes    []string  `json:"scopes"`
}
