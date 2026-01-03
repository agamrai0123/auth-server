package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "github.com/godror/godror"
	"github.com/rs/zerolog/log"
)

// newDbClient creates a new Oracle database connection
// Connection string format: user/password@hostname:port/sid
// Example: sys/Oracle123@localhost:1521/XE (with as sysdba)
func newDbClient(url string) (*sql.DB, error) {
	log.Debug().Str("url", url).Msg("Connecting to Oracle database")

	// url format: oracle://username:password@host:port/service_name
	db, err := sql.Open("oracle", url)
	if err != nil {
		log.Error().Err(err).Msg("Failed to open Oracle database connection")
		return nil, err
	}

	// Ping to validate connection
	err = db.Ping()
	if err != nil {
		log.Error().Err(err).Msg("Database ping failed - connection validation error")
		db.Close()
		return nil, err
	}

	// Configure connection pool for optimal performance
	db.SetMaxOpenConns(25)                 // Max concurrent DB connections
	db.SetMaxIdleConns(5)                  // Keep 5 idle for reuse (reduces overhead)
	db.SetConnMaxLifetime(5 * time.Minute) // Recycle old connections

	log.Info().Msg("Oracle database connected successfully")
	return db, nil
}

func (as *authServer) revokeToken(revokedToken RevokedToken) error {
	ctx, cancel := context.WithTimeout(as.ctx, 5*time.Second)
	defer cancel()

	// Begin a Tx for making transaction requests.
	tx, err := as.db.BeginTx(ctx, nil)
	if err != nil {
		log.Error().Err(err).Msg("Failed to begin transaction for token revocation")
		return err
	}
	defer tx.Rollback()

	// Oracle uses ? placeholders or named parameters like :name
	// Using named parameters for clarity
	query := "UPDATE tokens SET revoked = 1, revoked_at = :revoked_at WHERE token_id = :token_id"
	result, err := tx.ExecContext(ctx, query,
		sql.Named("revoked_at", revokedToken.RevokedAt),
		sql.Named("token_id", revokedToken.TokenID))
	if err != nil {
		log.Error().Err(err).Str("token_id", revokedToken.TokenID).Msg("Failed to revoke token")
		return err
	}

	// Commit the transaction.
	if err = tx.Commit(); err != nil {
		log.Error().Err(err).Msg("Failed to commit token revocation transaction")
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	log.Info().Int64("rows_affected", rowsAffected).Str("token_id", revokedToken.TokenID).Msg("Token revoked successfully")
	return nil
}

func (as *authServer) isTokenRevoked(tokenID string) (bool, error) {
	var revoked int
	ctx, cancel := context.WithTimeout(as.ctx, 5*time.Second)
	defer cancel()

	query := "SELECT revoked FROM tokens WHERE token_id = :token_id"
	row := as.db.QueryRowContext(ctx, query, sql.Named("token_id", tokenID))

	if err := row.Scan(&revoked); err != nil {
		if err == sql.ErrNoRows {
			log.Warn().Str("token_id", tokenID).Msg("Token not found in database")
			return false, fmt.Errorf("tokenID %s: no such token", tokenID)
		}
		log.Error().Err(err).Str("token_id", tokenID).Msg("Database query failed")
		return false, fmt.Errorf("tokenID %s: %v", tokenID, err)
	}

	return revoked == 1, nil
}

func (as *authServer) insertToken(tokenInfo Token) error {
	ctx, cancel := context.WithTimeout(as.ctx, 5*time.Second)
	defer cancel()

	// Begin a Tx for making transaction requests.
	tx, err := as.db.BeginTx(ctx, nil)
	if err != nil {
		log.Error().Err(err).Msg("Failed to begin transaction for token insertion")
		return err
	}
	defer tx.Rollback()

	query := "INSERT INTO tokens(token_id, client_id, issued_at, expires_at) VALUES (:token_id, :client_id, :issued_at, :expires_at)"
	_, err = tx.ExecContext(ctx, query,
		sql.Named("token_id", tokenInfo.TokenID),
		sql.Named("client_id", tokenInfo.ClientID),
		sql.Named("issued_at", tokenInfo.IssuedAt),
		sql.Named("expires_at", tokenInfo.ExpiresAt))
	if err != nil {
		log.Error().Err(err).Str("token_id", tokenInfo.TokenID).Msg("Failed to insert token")
		return err
	}

	// Commit the transaction.
	if err = tx.Commit(); err != nil {
		log.Error().Err(err).Msg("Failed to commit token insertion transaction")
		return err
	}

	log.Debug().Str("token_id", tokenInfo.TokenID).Str("client_id", tokenInfo.ClientID).Msg("Token inserted successfully")
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
	query := "SELECT allowed_scopes FROM clients WHERE client_id = :client_id"
	row := as.db.QueryRowContext(ctx, query, sql.Named("client_id", strings.TrimSpace(clientID)))

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
	query := "SELECT client_id, client_secret, access_token_ttl, allowed_scopes FROM clients WHERE client_id = :client_id"
	row := as.db.QueryRowContext(ctx, query, sql.Named("client_id", clientID))

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

// insertTokenBatch performs batch insertion of multiple tokens in a single transaction
// This is much more efficient than inserting one at a time
func (as *authServer) insertTokenBatch(tokens []Token) error {
	if len(tokens) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(as.ctx, 10*time.Second)
	defer cancel()

	// Begin transaction for atomic batch insert
	tx, err := as.db.BeginTx(ctx, nil)
	if err != nil {
		log.Error().
			Err(err).
			Int("batch_size", len(tokens)).
			Msg("Failed to begin transaction for batch insert")
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Prepare statement for batch insert (reused for all tokens in batch)
	stmt, err := tx.PrepareContext(ctx, "INSERT INTO tokens(token_id, client_id, issued_at, expires_at) VALUES (:token_id, :client_id, :issued_at, :expires_at)")
	if err != nil {
		log.Error().
			Err(err).
			Int("batch_size", len(tokens)).
			Msg("Failed to prepare batch insert statement")
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	// Execute insert for each token in batch
	inserted := 0
	for i, token := range tokens {
		_, err := stmt.ExecContext(ctx,
			sql.Named("token_id", token.TokenID),
			sql.Named("client_id", token.ClientID),
			sql.Named("issued_at", token.IssuedAt),
			sql.Named("expires_at", token.ExpiresAt))
		if err != nil {
			log.Error().
				Err(err).
				Str("token_id", token.TokenID).
				Str("client_id", token.ClientID).
				Int("position", i).
				Int("batch_size", len(tokens)).
				Msg("Failed to insert token in batch")
			return fmt.Errorf("failed to insert token at position %d: %w", i, err)
		}
		inserted++
	}

	// Commit transaction (atomicity ensures all or nothing)
	if err := tx.Commit(); err != nil {
		log.Error().
			Err(err).
			Int("inserted", inserted).
			Int("batch_size", len(tokens)).
			Msg("Failed to commit batch insert transaction")
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Debug().
		Int("count", len(tokens)).
		Msg("Token batch inserted successfully")
	return nil
}
