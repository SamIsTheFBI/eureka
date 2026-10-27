package db

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	_ "github.com/lib/pq"
)

// DB wraps the sql.DB connection pool.
type DB struct {
	*sql.DB
	logger *slog.Logger
}

// Connect initializes the database pool and verifies connectivity.
func Connect(dsn string, logger *slog.Logger) (*DB, error) {
	conn, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	conn.SetMaxOpenConns(25)
	conn.SetMaxIdleConns(10)
	conn.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := conn.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{DB: conn, logger: logger}, nil
}

// Migrate runs idempotent DDL migrations to set up the gateway schema.
func (db *DB) Migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			email VARCHAR(255) UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			role VARCHAR(32) NOT NULL DEFAULT 'member',
			active BOOLEAN NOT NULL DEFAULT true,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`,

		`CREATE TABLE IF NOT EXISTS invites (
			id SERIAL PRIMARY KEY,
			email VARCHAR(255) NOT NULL,
			role VARCHAR(32) NOT NULL DEFAULT 'member',
			token_hash VARCHAR(64) UNIQUE NOT NULL,
			expires_at TIMESTAMPTZ NOT NULL,
			used_at TIMESTAMPTZ,
			created_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`,

		`CREATE TABLE IF NOT EXISTS sessions (
			id_hash VARCHAR(64) PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			expires_at TIMESTAMPTZ NOT NULL,
			last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`,

		`CREATE TABLE IF NOT EXISTS api_keys (
			id SERIAL PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			name VARCHAR(255) NOT NULL,
			prefix VARCHAR(16) NOT NULL,
			key_hash VARCHAR(64) UNIQUE NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			last_used_at TIMESTAMPTZ,
			expires_at TIMESTAMPTZ,
			revoked_at TIMESTAMPTZ
		);
		CREATE INDEX IF NOT EXISTS idx_api_keys_prefix ON api_keys(prefix);`,

		`CREATE TABLE IF NOT EXISTS mcp_servers (
			id SERIAL PRIMARY KEY,
			slug VARCHAR(32) UNIQUE NOT NULL,
			name VARCHAR(255) NOT NULL,
			url TEXT NOT NULL,
			enabled BOOLEAN NOT NULL DEFAULT true,
			auth_kind VARCHAR(32) NOT NULL DEFAULT 'none',
			auth_header VARCHAR(255),
			credential_enc TEXT,
			key_version INTEGER NOT NULL DEFAULT 1,
			call_timeout_ms INTEGER NOT NULL DEFAULT 30000,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`,

		`CREATE TABLE IF NOT EXISTS audit_log (
			id BIGSERIAL PRIMARY KEY,
			ts TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
			user_email VARCHAR(255) NOT NULL,
			key_id INTEGER REFERENCES api_keys(id) ON DELETE SET NULL,
			channel VARCHAR(16) NOT NULL,
			server_slug VARCHAR(32) NOT NULL,
			tool VARCHAR(64) NOT NULL,
			exposed_name VARCHAR(128) NOT NULL,
			outcome VARCHAR(32) NOT NULL,
			error_code VARCHAR(64),
			duration_ms BIGINT NOT NULL,
			request_id VARCHAR(64) NOT NULL,
			args_hash VARCHAR(64),
			args_bytes INTEGER DEFAULT 0
		);
		CREATE INDEX IF NOT EXISTS idx_audit_ts ON audit_log(ts DESC);
		CREATE INDEX IF NOT EXISTS idx_audit_user_ts ON audit_log(user_id, ts DESC);
		CREATE INDEX IF NOT EXISTS idx_audit_server_ts ON audit_log(server_slug, ts DESC);
		CREATE INDEX IF NOT EXISTS idx_audit_outcome_ts ON audit_log(outcome, ts DESC);`,

		`CREATE TABLE IF NOT EXISTS admin_events (
			id SERIAL PRIMARY KEY,
			ts TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			actor_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
			action VARCHAR(64) NOT NULL,
			target VARCHAR(255) NOT NULL,
			detail JSONB
		);`,

		`CREATE TABLE IF NOT EXISTS chat_conversations (
			id VARCHAR(64) PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			title VARCHAR(255) NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`,

		`CREATE TABLE IF NOT EXISTS chat_messages (
			id SERIAL PRIMARY KEY,
			conversation_id VARCHAR(64) NOT NULL REFERENCES chat_conversations(id) ON DELETE CASCADE,
			role VARCHAR(32) NOT NULL,
			content TEXT NOT NULL,
			tool_calls JSONB,
			reasoning TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE INDEX IF NOT EXISTS idx_chat_msg_conv ON chat_messages(conversation_id, id ASC);`,
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return fmt.Errorf("migration query failed: %w (query: %s)", err, q)
		}
	}

	db.logger.Info("database schema migrations applied successfully")
	return nil
}
