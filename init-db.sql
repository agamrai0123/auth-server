-- Oracle Database Initialization Script for Auth Server
-- This script creates the necessary tables and indexes for the authentication server

-- Create CLIENTS table
CREATE TABLE clients (
    client_id VARCHAR2(100) PRIMARY KEY,
    client_secret VARCHAR2(255) NOT NULL,
    client_name VARCHAR2(255),
    access_token_ttl NUMBER(10) DEFAULT 3600,
    allowed_scopes CLOB,
    created_at TIMESTAMP DEFAULT SYSTIMESTAMP,
    updated_at TIMESTAMP DEFAULT SYSTIMESTAMP,
    active NUMBER(1) DEFAULT 1
);

-- Create TOKENS table
CREATE TABLE tokens (
    token_id VARCHAR2(255) PRIMARY KEY,
    client_id VARCHAR2(100) NOT NULL,
    issued_at TIMESTAMP DEFAULT SYSTIMESTAMP,
    expires_at TIMESTAMP NOT NULL,
    revoked NUMBER(1) DEFAULT 0,
    revoked_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT SYSTIMESTAMP,
    CONSTRAINT fk_tokens_client FOREIGN KEY (client_id) REFERENCES clients(client_id)
);

-- Create REVOKED_TOKENS table
CREATE TABLE revoked_tokens (
    id NUMBER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    token_id VARCHAR2(255) NOT NULL,
    client_id VARCHAR2(100) NOT NULL,
    revoked_at TIMESTAMP DEFAULT SYSTIMESTAMP,
    CONSTRAINT fk_revoked_tokens_client FOREIGN KEY (client_id) REFERENCES clients(client_id)
);

-- Create ENDPOINTS table
CREATE TABLE endpoints (
    id NUMBER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    client_id VARCHAR2(100) NOT NULL,
    scope VARCHAR2(255) NOT NULL,
    method VARCHAR2(10) NOT NULL,
    endpoint_url VARCHAR2(500) NOT NULL,
    description VARCHAR2(500),
    active NUMBER(1) DEFAULT 1,
    created_at TIMESTAMP DEFAULT SYSTIMESTAMP,
    CONSTRAINT fk_endpoints_client FOREIGN KEY (client_id) REFERENCES clients(client_id)
);

-- Create indexes for performance
CREATE INDEX idx_tokens_client_id ON tokens(client_id);
CREATE INDEX idx_tokens_expires_at ON tokens(expires_at);
CREATE INDEX idx_tokens_revoked ON tokens(revoked);
CREATE INDEX idx_revoked_tokens_token_id ON revoked_tokens(token_id);
CREATE INDEX idx_revoked_tokens_client_id ON revoked_tokens(client_id);
CREATE INDEX idx_endpoints_client_id ON endpoints(client_id);

-- Insert sample test data
INSERT INTO clients (client_id, client_secret, client_name, access_token_ttl, allowed_scopes) 
VALUES (
    'test-client-1',
    'secret-key-12345',
    'Test Client 1',
    3600,
    '["http://localhost:3000/api/users", "http://localhost:3000/api/posts"]'
);

INSERT INTO clients (client_id, client_secret, client_name, access_token_ttl, allowed_scopes) 
VALUES (
    'test-client-2',
    'secret-key-67890',
    'Test Client 2',
    7200,
    '["http://localhost:3000/api/admin", "http://localhost:3000/api/reports"]'
);

INSERT INTO clients (client_id, client_secret, client_name, access_token_ttl, allowed_scopes) 
VALUES (
    'mobile-app',
    'mobile-secret-key',
    'Mobile Application',
    1800,
    '["http://localhost:3000/api/auth", "http://localhost:3000/api/profile"]'
);

-- Commit changes
COMMIT;

-- Display table information
SELECT table_name FROM user_tables WHERE table_name IN ('CLIENTS', 'TOKENS', 'REVOKED_TOKENS', 'ENDPOINTS');
