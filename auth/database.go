package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "github.com/rqlite/gorqlite/stdlib"
	"github.com/rs/zerolog/log"
)

func newDbClient(url string) (*sql.DB, error) {
	log.Debug().Str("url", url).Msg("Connecting to rqlite database")
	db, err := sql.Open("rqlite", "http://")
	if err != nil {
		log.Error().Err(err).Msg("Failed to open database connection")
		return nil, err
	}
	err = db.Ping()
	if err != nil {
		log.Error().Err(err).Msg("Database ping failed - connection validation error")
		return nil, err
	}

	log.Info().Msg("Database connected successfully")
	return db, nil
}

func (as *authServer) revokeToken(revokedToken RevokedToken) error {
	ctx, cancel := context.WithTimeout(as.ctx, 5*time.Second)
	defer cancel()
	// Begin a Tx for making transaction requests.
	tx, err := as.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	query := "Update tokens set revoked=true, revoked_at=:1 where token_id=:2"
	result, err := tx.ExecContext(ctx, query, revokedToken.RevokedAt, revokedToken.TokenID)
	if err != nil {
		return err
	}

	// Commit the transaction.
	if err = tx.Commit(); err != nil {
		return err
	}
	log.Info().Msgf("token revoked successfully: %v", result)
	return nil
}

func (as *authServer) isTokenRevoked(tokenID string) (bool, error) {
	var isValid bool
	ctx, cancel := context.WithTimeout(as.ctx, 5*time.Second)
	defer cancel()
	query := "SELECT revoked from tokens where token_id=:1"
	row := as.db.QueryRowContext(ctx, query, tokenID)
	if err := row.Scan(&isValid); err != nil {
		if err == sql.ErrNoRows {
			return isValid, fmt.Errorf("tokenID %s: no such tokenID", tokenID)
		}
		return isValid, fmt.Errorf("tokenID %s: %v", tokenID, err)
	}
	return isValid, nil
}

func (as *authServer) insertToken(tokenInfo Token) error {
	ctx, cancel := context.WithTimeout(as.ctx, 5*time.Second)
	defer cancel()
	// Begin a Tx for making transaction requests.
	tx, err := as.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	query := "Insert into tokens(token_id, client_id, issued_at, expires_at) VALUES (:1, :2, :3, :4)"
	result, err := tx.ExecContext(ctx, query, tokenInfo.TokenID, tokenInfo.ClientID, tokenInfo.IssuedAt, tokenInfo.ExpiresAt)
	if err != nil {
		return err
	}

	// Commit the transaction.
	if err = tx.Commit(); err != nil {
		return err
	}
	log.Info().Msgf("token inserted successfully: %v", result)
	return nil
}

// func (as *authServer) getEndpoint(clientID, scope string) (*Endpoints, error) {
// 	ctx, cancel := context.WithTimeout(as.ctx, 5*time.Second)
// 	defer cancel()
// 	var endpoint Endpoints
// 	query := "select client_id, scope, method, endpoint_url, active from endpoints where client_id=:1 and scope=:2"
// 	row := as.db.QueryRowContext(ctx, query, strings.TrimSpace(clientID), strings.TrimSpace(scope))
// 	if err := row.Scan(&endpoint.ClientID, &endpoint.Scope, &endpoint.Method, &endpoint.Url, &endpoint.Active); err != nil {
// 		if err == sql.ErrNoRows {
// 			log.Error().Err(err)
// 			return &endpoint, fmt.Errorf("clientByID %s: no such client", clientID)
// 		}
// 		return &endpoint, fmt.Errorf("clientByID %s: %v", clientID, err)
// 	}
// 	return &endpoint, nil
// }

func (as *authServer) getClientScopes(clientID string) ([]string, error) {
	log.Debug().Str("client_id", clientID).Msg("Fetching client scopes from database")
	ctx, cancel := context.WithTimeout(as.ctx, 5*time.Second)
	defer cancel()

	var scope []string
	var res string
	query := "select allowed_scopes from clients where client_id=:1"
	row := as.db.QueryRowContext(ctx, query, strings.TrimSpace(clientID))

	if err := row.Scan(&res); err != nil {
		if err == sql.ErrNoRows {
			log.Warn().Str("client_id", clientID).Msg("Client not found in database")
			return nil, fmt.Errorf("clientByID %s: no such client", clientID)
		}
		log.Error().Err(err).Str("client_id", clientID).Msg("Database query failed")
		return nil, fmt.Errorf("clientByID %s: %v", clientID, err)
	}

	err := json.Unmarshal([]byte(res), &scope)
	if err != nil {
		log.Error().Err(err).Str("client_id", clientID).Msg("Failed to unmarshal allowed_scopes JSON")
		return nil, fmt.Errorf("clientByID %s: %v", clientID, err)
	}

	log.Debug().Str("client_id", clientID).Strs("scopes", scope).Msg("Client scopes retrieved")
	return scope, nil
}

func (as *authServer) clientByID(clientID string) (*Clients, error) {
	ctx, cancel := context.WithTimeout(as.ctx, 5*time.Second)
	defer cancel()
	log.Debug().Str("client_id", clientID).Msg("Looking up client in database")

	var client Clients
	var scope string
	var err error
	query := "SELECT client_id, client_secret, access_token_ttl, allowed_scopes from clients where client_id=:1"
	row := as.db.QueryRowContext(ctx, query, clientID)

	if err := row.Scan(&client.ClientID, &client.ClientSecret, &client.AccessTokenTTL, &scope); err != nil {
		if err == sql.ErrNoRows {
			log.Warn().Str("client_id", clientID).Msg("Client not found in database")
			return &client, fmt.Errorf("clientByID %s: no such client", clientID)
		}
		log.Error().Err(err).Str("client_id", clientID).Msg("Database query failed")
		return &client, fmt.Errorf("clientByID %s: %v", clientID, err)
	}

	client.AllowedScopes, err = parseStringArray(scope)
	if err != nil {
		log.Error().Err(err).Str("client_id", clientID).Msg("Failed to parse allowed scopes")
		return nil, err
	}

	log.Debug().Str("client_id", clientID).Strs("allowed_scopes", client.AllowedScopes).Msg("Client found and scopes parsed")
	return &client, nil
}

func parseStringArray(s string) ([]string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}

	// Try strict JSON first: '["a","b"]'
	var out []string
	if strings.HasPrefix(s, "[") {
		if err := json.Unmarshal([]byte(s), &out); err == nil {
			return out, nil
		}
		// Try convert single quotes to double quotes and unmarshal
		s2 := strings.ReplaceAll(s, `'`, `"`)
		if err := json.Unmarshal([]byte(s2), &out); err == nil {
			return out, nil
		}
	}

	// Fallback: tolerate forms like [a:1, b:2] or ['a:1','b:2']
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	parts := strings.Split(s, ",")
	out = make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		p = strings.Trim(p, `"'`) // drop surrounding quotes if any
		if p != "" {
			out = append(out, p)
		}
	}
	return out, nil
}
