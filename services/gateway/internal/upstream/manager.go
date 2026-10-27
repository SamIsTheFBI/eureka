package upstream

import (
	"context"
	"database/sql"
	"log/slog"
	"sync"
	"time"

	"eureka/services/gateway/internal/crypto"
	"eureka/services/gateway/internal/db"
)

// Manager supervises all active upstream MCP connections.
type Manager struct {
	mu        sync.RWMutex
	servers   map[string]*UpstreamServer
	db        *db.DB
	masterKey []byte
	logger    *slog.Logger
	cancel    context.CancelFunc
}

// NewManager initializes the upstream manager.
func NewManager(db *db.DB, masterKey []byte, logger *slog.Logger) *Manager {
	return &Manager{
		servers:   make(map[string]*UpstreamServer),
		db:        db,
		masterKey: masterKey,
		logger:    logger,
	}
}

// Start begins upstream supervision and periodic health checking.
func (m *Manager) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	m.cancel = cancel

	if err := m.Reload(ctx); err != nil {
		m.logger.Error("failed to load initial upstreams from database", "error", err)
	}

	go m.superviseLoop(ctx)
	return nil
}

// Stop shuts down the manager.
func (m *Manager) Stop() {
	if m.cancel != nil {
		m.cancel()
	}
}

// GetServer retrieves an upstream server by its slug.
func (m *Manager) GetServer(slug string) (*UpstreamServer, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.servers[slug]
	return s, ok
}

// ListServers returns all registered upstream servers.
func (m *Manager) ListServers() []*UpstreamServer {
	m.mu.RLock()
	defer m.mu.RUnlock()
	list := make([]*UpstreamServer, 0, len(m.servers))
	for _, s := range m.servers {
		list = append(list, s)
	}
	return list
}

// Reload reads upstream definitions from the database and updates running instances.
func (m *Manager) Reload(ctx context.Context) error {
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, slug, name, url, enabled, auth_kind, auth_header, credential_enc, call_timeout_ms
		FROM mcp_servers
		ORDER BY id ASC
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	newMap := make(map[string]*UpstreamServer)

	for rows.Next() {
		var (
			id, timeoutMs               int
			slug, name, urlStr, authKind string
			authHeader, credEnc         sql.NullString
			enabled                     bool
		)
		if err := rows.Scan(&id, &slug, &name, &urlStr, &enabled, &authKind, &authHeader, &credEnc, &timeoutMs); err != nil {
			m.logger.Warn("failed to scan upstream row", "error", err)
			continue
		}

		decryptedCred := ""
		if credEnc.Valid && credEnc.String != "" {
			if dec, err := crypto.DecryptCredential(m.masterKey, credEnc.String, slug); err == nil {
				decryptedCred = dec
			} else {
				m.logger.Error("failed to decrypt upstream credential", "slug", slug, "error", err)
			}
		}

		m.mu.RLock()
		existing, ok := m.servers[slug]
		m.mu.RUnlock()

		if ok && existing.URL == urlStr && existing.Enabled == enabled && existing.Credential == decryptedCred {
			newMap[slug] = existing
		} else {
			srv := NewUpstreamServer(id, slug, name, urlStr, enabled, authKind, authHeader.String, decryptedCred, timeoutMs, m.logger)
			newMap[slug] = srv
			// Probe initial connection in background
			go func(s *UpstreamServer) {
				_ = s.Ping(context.Background())
			}(srv)
		}
	}

	m.mu.Lock()
	m.servers = newMap
	m.mu.Unlock()

	return nil
}

func (m *Manager) superviseLoop(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			servers := m.ListServers()
			for _, srv := range servers {
				go func(s *UpstreamServer) {
					pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
					defer cancel()
					_ = s.Ping(pingCtx)
				}(srv)
			}
		}
	}
}
