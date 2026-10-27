package auth

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"eureka/services/gateway/internal/crypto"
	"eureka/services/gateway/internal/db"
)

// Principal represents the authenticated user context for any request.
type Principal struct {
	UserID    int
	UserEmail string
	Role      string // "admin", "member"
	KeyID     *int   // nil if dashboard session
	Channel   string // "mcp" or "chat"
}

type contextKey string

const principalContextKey = contextKey("urumi_principal")

// WithPrincipal adds the Principal to the request context.
func WithPrincipal(ctx context.Context, p *Principal) context.Context {
	return context.WithValue(ctx, principalContextKey, p)
}

// GetPrincipal extracts the Principal from context if present.
func GetPrincipal(ctx context.Context) (*Principal, bool) {
	p, ok := ctx.Value(principalContextKey).(*Principal)
	return p, ok
}

// Service handles authentication and session verification.
type Service struct {
	db *db.DB
}

// NewService creates a new authentication service.
func NewService(db *db.DB) *Service {
	return &Service{db: db}
}

// ValidateAPIKey verifies an incoming bearer key against the database.
func (s *Service) ValidateAPIKey(ctx context.Context, fullKey string) (*Principal, error) {
	parts := strings.SplitN(fullKey, "_", 3)
	if len(parts) != 3 || parts[0] != "urmk" || len(parts[1]) != 8 {
		return nil, errors.New("invalid key format")
	}

	prefix := parts[1]
	providedHash := crypto.HashKey(fullKey)

	query := `
		SELECT k.id, k.key_hash, k.revoked_at, k.expires_at, u.id, u.email, u.role, u.active
		FROM api_keys k
		JOIN users u ON k.user_id = u.id
		WHERE k.prefix = $1
	`
	rows, err := s.db.QueryContext(ctx, query, prefix)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			keyID       int
			storedHash  string
			revokedAt   sql.NullTime
			expiresAt   sql.NullTime
			userID      int
			email, role string
			active      bool
		)
		if err := rows.Scan(&keyID, &storedHash, &revokedAt, &expiresAt, &userID, &email, &role, &active); err != nil {
			continue
		}

		// Constant-time check on hash
		if subtle.ConstantTimeCompare([]byte(providedHash), []byte(storedHash)) == 1 {
			if revokedAt.Valid {
				return nil, errors.New("key revoked")
			}
			if expiresAt.Valid && expiresAt.Time.Before(time.Now()) {
				return nil, errors.New("key expired")
			}
			if !active {
				return nil, errors.New("user deactivated")
			}

			// Fire-and-forget last_used update
			go func(kID int) {
				_, _ = s.db.Exec("UPDATE api_keys SET last_used_at = NOW() WHERE id = $1", kID)
			}(keyID)

			return &Principal{
				UserID:    userID,
				UserEmail: email,
				Role:      role,
				KeyID:     &keyID,
				Channel:   "mcp",
			}, nil
		}
	}

	return nil, errors.New("key not found or invalid")
}

// ValidateSession verifies a dashboard session cookie.
func (s *Service) ValidateSession(ctx context.Context, sessionID string) (*Principal, error) {
	if sessionID == "" {
		return nil, errors.New("missing session")
	}

	sessionHash := crypto.HashString(sessionID)
	query := `
		SELECT s.expires_at, u.id, u.email, u.role, u.active
		FROM sessions s
		JOIN users u ON s.user_id = u.id
		WHERE s.id_hash = $1
	`
	var (
		expiresAt   time.Time
		userID      int
		email, role string
		active      bool
	)
	err := s.db.QueryRowContext(ctx, query, sessionHash).Scan(&expiresAt, &userID, &email, &role, &active)
	if err != nil {
		return nil, errors.New("session not found")
	}

	if expiresAt.Before(time.Now()) {
		return nil, errors.New("session expired")
	}
	if !active {
		return nil, errors.New("user deactivated")
	}

	return &Principal{
		UserID:    userID,
		UserEmail: email,
		Role:      role,
		KeyID:     nil,
		Channel:   "chat",
	}, nil
}

// Middleware returns an HTTP middleware that populates the Principal from Session or API Key.
func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var principal *Principal
		var err error

		// 1. Try Bearer token in Authorization header
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			key := strings.TrimPrefix(authHeader, "Bearer ")
			principal, err = s.ValidateAPIKey(r.Context(), key)
		}

		// 2. Try Cookie if no valid Bearer token
		if principal == nil {
			if cookie, cErr := r.Cookie("urumi_session"); cErr == nil && cookie.Value != "" {
				principal, err = s.ValidateSession(r.Context(), cookie.Value)
			}
		}

		if err == nil && principal != nil {
			r = r.WithContext(WithPrincipal(r.Context(), principal))
		}

		next.ServeHTTP(w, r)
	})
}
