package admin

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"eureka/services/gateway/internal/agent"
	"eureka/services/gateway/internal/aggregate"
	"eureka/services/gateway/internal/auth"
	"eureka/services/gateway/internal/config"
	"eureka/services/gateway/internal/crypto"
	"eureka/services/gateway/internal/db"
	"eureka/services/gateway/internal/mcp"
	"eureka/services/gateway/internal/router"
	"eureka/services/gateway/internal/upstream"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Server implements all admin and dashboard REST APIs.
type Server struct {
	cfg        *config.Config
	db         *db.DB
	authSvc    *auth.Service
	manager    *upstream.Manager
	aggregator *aggregate.Aggregator
	router     *router.Router
	agent      *agent.Agent
	logger     *slog.Logger
}

// NewServer creates a new admin REST API server.
func NewServer(cfg *config.Config, db *db.DB, authSvc *auth.Service, mgr *upstream.Manager, agg *aggregate.Aggregator, rtr *router.Router, ag *agent.Agent, logger *slog.Logger) *Server {
	return &Server{
		cfg:        cfg,
		db:         db,
		authSvc:    authSvc,
		manager:    mgr,
		aggregator: agg,
		router:     rtr,
		agent:      ag,
		logger:     logger,
	}
}

// RegisterRoutes hooks admin endpoints onto a chi.Router.
func (s *Server) RegisterRoutes(r chi.Router) {
	// Public auth routes
	r.Post("/api/auth/login", s.handleLogin)
	r.Post("/api/auth/logout", s.handleLogout)

	// Protected routes (Session or API Key)
	r.Group(func(pr chi.Router) {
		pr.Use(s.authSvc.Middleware)

		pr.Get("/api/auth/me", s.handleMe)
		pr.Get("/api/servers", s.handleListServers)
		pr.Get("/api/keys", s.handleListKeys)
		pr.Post("/api/keys", s.handleCreateKey)
		pr.Delete("/api/keys/{id}", s.handleRevokeKey)
		pr.Get("/api/audit", s.handleListAudit)
		pr.Post("/api/chat", s.handleChat)
		pr.Get("/api/chat/conversations", s.handleListConversations)

		// Admin only routes
		pr.Group(func(ar chi.Router) {
			ar.Use(s.requireAdmin)

			ar.Post("/api/servers", s.handleAddServer)
			ar.Patch("/api/servers/{id}", s.handleUpdateServer)
			ar.Delete("/api/servers/{id}", s.handleDeleteServer)
			ar.Post("/api/servers/test", s.handleTestServer)

			ar.Get("/api/users", s.handleListUsers)
			ar.Post("/api/invites", s.handleCreateInvite)
			ar.Patch("/api/users/{id}", s.handleUpdateUser)
		})
	})
}

func (s *Server) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := auth.GetPrincipal(r.Context())
		if !ok || p.Role != "admin" {
			http.Error(w, `{"error":"Forbidden: admin role required"}`, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// --- Auth Handlers ---

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"Invalid request"}`, http.StatusBadRequest)
		return
	}

	var (
		id           int
		email, hash  string
		role         string
		active       bool
	)
	err := s.db.QueryRowContext(r.Context(), "SELECT id, email, password_hash, role, active FROM users WHERE email = $1", strings.ToLower(req.Email)).
		Scan(&id, &email, &hash, &role, &active)
	if err != nil || !active || !crypto.CheckPassword(req.Password, hash) {
		http.Error(w, `{"error":"Invalid credentials"}`, http.StatusUnauthorized)
		return
	}

	// Create session (256-bit random)
	sessionBytes := make([]byte, 32)
	_, _ = rand.Read(sessionBytes)
	sessionID := hex.EncodeToString(sessionBytes)
	sessionHash := crypto.HashString(sessionID)

	expiresAt := time.Now().Add(12 * time.Hour)
	_, err = s.db.ExecContext(r.Context(), "INSERT INTO sessions (id_hash, user_id, expires_at) VALUES ($1, $2, $3)",
		sessionHash, id, expiresAt)
	if err != nil {
		http.Error(w, `{"error":"Failed to create session"}`, http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "urumi_session",
		Value:    sessionID,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		// Secure flag is disabled for local test HTTP, enabled on prod TLS
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"id":    id,
		"email": email,
		"role":  role,
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("urumi_session"); err == nil && cookie.Value != "" {
		hash := crypto.HashString(cookie.Value)
		_, _ = s.db.ExecContext(r.Context(), "DELETE FROM sessions WHERE id_hash = $1", hash)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "urumi_session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	p, ok := auth.GetPrincipal(r.Context())
	if !ok {
		http.Error(w, `{"error":"Unauthorized"}`, http.StatusUnauthorized)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":    p.UserID,
		"email": p.UserEmail,
		"role":  p.Role,
	})
}

// --- Upstream Server Handlers ---

func (s *Server) handleListServers(w http.ResponseWriter, r *http.Request) {
	p, _ := auth.GetPrincipal(r.Context())
	isAdmin := p != nil && p.Role == "admin"

	servers := s.manager.ListServers()
	type serverDTO struct {
		ID            int        `json:"id"`
		Slug          string     `json:"slug"`
		Name          string     `json:"name"`
		URL           string     `json:"url,omitempty"`
		Enabled       bool       `json:"enabled"`
		Status        string     `json:"status"`
		Reason        string     `json:"reason,omitempty"`
		LastError     string     `json:"last_error,omitempty"`
		LastSuccess   *time.Time `json:"last_success,omitempty"`
		HasCredential bool       `json:"has_credential"`
		Tools         []mcp.Tool `json:"tools"`
	}

	resp := make([]serverDTO, 0, len(servers))
	for _, srv := range servers {
		status, reason, lastErr, lastSucc := srv.GetStatus()
		dto := serverDTO{
			ID:            srv.ID,
			Slug:          srv.Slug,
			Name:          srv.Name,
			Enabled:       srv.Enabled,
			Status:        status,
			Reason:        reason,
			LastError:     lastErr,
			HasCredential: srv.Credential != "",
			Tools:         srv.GetTools(),
		}
		if !lastSucc.IsZero() {
			dto.LastSuccess = &lastSucc
		}
		if isAdmin {
			dto.URL = srv.URL
		}
		resp = append(resp, dto)
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleAddServer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Slug          string `json:"slug"`
		Name          string `json:"name"`
		URL           string `json:"url"`
		AuthKind      string `json:"auth_kind"`
		AuthHeader    string `json:"auth_header"`
		Credential    string `json:"credential"`
		CallTimeoutMs int    `json:"call_timeout_ms"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"Invalid request"}`, http.StatusBadRequest)
		return
	}

	req.Slug = strings.ToLower(strings.TrimSpace(req.Slug))
	if req.Slug == "" || req.URL == "" {
		http.Error(w, `{"error":"Slug and URL are required"}`, http.StatusBadRequest)
		return
	}
	if req.CallTimeoutMs <= 0 {
		req.CallTimeoutMs = 30000
	}

	credEnc := ""
	if req.Credential != "" {
		enc, err := crypto.EncryptCredential(s.cfg.MasterKey, req.Credential, req.Slug)
		if err != nil {
			http.Error(w, `{"error":"Failed to encrypt credential"}`, http.StatusInternalServerError)
			return
		}
		credEnc = enc
	}

	var newID int
	query := `
		INSERT INTO mcp_servers (slug, name, url, enabled, auth_kind, auth_header, credential_enc, call_timeout_ms)
		VALUES ($1, $2, $3, true, $4, $5, $6, $7)
		RETURNING id
	`
	err := s.db.QueryRowContext(r.Context(), query, req.Slug, req.Name, req.URL, req.AuthKind, req.AuthHeader, credEnc, req.CallTimeoutMs).Scan(&newID)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Failed to insert server: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	_ = s.manager.Reload(r.Context())
	writeJSON(w, http.StatusCreated, map[string]any{"id": newID, "slug": req.Slug})
}

func (s *Server) handleUpdateServer(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, _ := strconv.Atoi(idStr)

	var req struct {
		Enabled       *bool   `json:"enabled"`
		Name          *string `json:"name"`
		URL           *string `json:"url"`
		Credential    *string `json:"credential"`
		CallTimeoutMs *int    `json:"call_timeout_ms"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"Invalid request"}`, http.StatusBadRequest)
		return
	}

	if req.Enabled != nil {
		_, _ = s.db.ExecContext(r.Context(), "UPDATE mcp_servers SET enabled = $1 WHERE id = $2", *req.Enabled, id)
	}
	if req.Name != nil {
		_, _ = s.db.ExecContext(r.Context(), "UPDATE mcp_servers SET name = $1 WHERE id = $2", *req.Name, id)
	}
	if req.URL != nil {
		_, _ = s.db.ExecContext(r.Context(), "UPDATE mcp_servers SET url = $1 WHERE id = $2", *req.URL, id)
	}
	if req.CallTimeoutMs != nil {
		_, _ = s.db.ExecContext(r.Context(), "UPDATE mcp_servers SET call_timeout_ms = $1 WHERE id = $2", *req.CallTimeoutMs, id)
	}

	_ = s.manager.Reload(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleDeleteServer(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, _ := strconv.Atoi(idStr)

	_, err := s.db.ExecContext(r.Context(), "DELETE FROM mcp_servers WHERE id = $1", id)
	if err != nil {
		http.Error(w, `{"error":"Failed to delete"}`, http.StatusInternalServerError)
		return
	}

	_ = s.manager.Reload(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleTestServer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL        string `json:"url"`
		AuthKind   string `json:"auth_kind"`
		AuthHeader string `json:"auth_header"`
		Credential string `json:"credential"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"Invalid request"}`, http.StatusBadRequest)
		return
	}

	testSrv := upstream.NewUpstreamServer(0, "test", "Test", req.URL, true, req.AuthKind, req.AuthHeader, req.Credential, 5000, s.logger)
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := testSrv.Ping(ctx); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "tools_count": len(testSrv.GetTools())})
}

// --- API Key Handlers ---

func (s *Server) handleListKeys(w http.ResponseWriter, r *http.Request) {
	p, ok := auth.GetPrincipal(r.Context())
	if !ok {
		http.Error(w, `{"error":"Unauthorized"}`, http.StatusUnauthorized)
		return
	}

	var rows *sql.Rows
	var err error
	if p.Role == "admin" {
		rows, err = s.db.QueryContext(r.Context(), `
			SELECT k.id, k.name, k.prefix, k.created_at, k.last_used_at, k.revoked_at, u.email
			FROM api_keys k
			JOIN users u ON k.user_id = u.id
			ORDER BY k.id DESC
		`)
	} else {
		rows, err = s.db.QueryContext(r.Context(), `
			SELECT k.id, k.name, k.prefix, k.created_at, k.last_used_at, k.revoked_at, u.email
			FROM api_keys k
			JOIN users u ON k.user_id = u.id
			WHERE k.user_id = $1
			ORDER BY k.id DESC
		`, p.UserID)
	}

	if err != nil {
		http.Error(w, `{"error":"Database error"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type keyDTO struct {
		ID         int        `json:"id"`
		Name       string     `json:"name"`
		Prefix     string     `json:"prefix"`
		UserEmail  string     `json:"user_email"`
		CreatedAt  time.Time  `json:"created_at"`
		LastUsedAt *time.Time `json:"last_used_at,omitempty"`
		RevokedAt  *time.Time `json:"revoked_at,omitempty"`
	}

	list := make([]keyDTO, 0)
	for rows.Next() {
		var dto keyDTO
		var lastUsed, revoked sql.NullTime
		if err := rows.Scan(&dto.ID, &dto.Name, &dto.Prefix, &dto.CreatedAt, &lastUsed, &revoked, &dto.UserEmail); err == nil {
			if lastUsed.Valid {
				dto.LastUsedAt = &lastUsed.Time
			}
			if revoked.Valid {
				dto.RevokedAt = &revoked.Time
			}
			list = append(list, dto)
		}
	}

	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleCreateKey(w http.ResponseWriter, r *http.Request) {
	p, ok := auth.GetPrincipal(r.Context())
	if !ok {
		http.Error(w, `{"error":"Unauthorized"}`, http.StatusUnauthorized)
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.Name == "" {
		req.Name = "Default Key"
	}

	fullKey, prefix, keyHash, err := crypto.GenerateAPIKey()
	if err != nil {
		http.Error(w, `{"error":"Failed to generate key"}`, http.StatusInternalServerError)
		return
	}

	var newID int
	query := `
		INSERT INTO api_keys (user_id, name, prefix, key_hash)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`
	err = s.db.QueryRowContext(r.Context(), query, p.UserID, req.Name, prefix, keyHash).Scan(&newID)
	if err != nil {
		http.Error(w, `{"error":"Database error"}`, http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":      newID,
		"name":    req.Name,
		"prefix":  prefix,
		"api_key": fullKey, // ONLY shown once at creation!
	})
}

func (s *Server) handleRevokeKey(w http.ResponseWriter, r *http.Request) {
	p, ok := auth.GetPrincipal(r.Context())
	if !ok {
		http.Error(w, `{"error":"Unauthorized"}`, http.StatusUnauthorized)
		return
	}

	idStr := chi.URLParam(r, "id")
	id, _ := strconv.Atoi(idStr)

	var err error
	if p.Role == "admin" {
		_, err = s.db.ExecContext(r.Context(), "UPDATE api_keys SET revoked_at = NOW() WHERE id = $1", id)
	} else {
		_, err = s.db.ExecContext(r.Context(), "UPDATE api_keys SET revoked_at = NOW() WHERE id = $1 AND user_id = $2", id, p.UserID)
	}

	if err != nil {
		http.Error(w, `{"error":"Failed to revoke"}`, http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// --- Audit Log Handler ---

func (s *Server) handleListAudit(w http.ResponseWriter, r *http.Request) {
	p, ok := auth.GetPrincipal(r.Context())
	if !ok {
		http.Error(w, `{"error":"Unauthorized"}`, http.StatusUnauthorized)
		return
	}

	serverFilter := r.URL.Query().Get("server")
	toolFilter := r.URL.Query().Get("tool")
	outcomeFilter := r.URL.Query().Get("outcome")

	query := `
		SELECT id, ts, user_email, channel, server_slug, tool, exposed_name, outcome, error_code, duration_ms, request_id
		FROM audit_log
		WHERE 1=1
	`
	args := []any{}
	idx := 1

	if p.Role != "admin" {
		query += fmt.Sprintf(" AND user_id = $%d", idx)
		args = append(args, p.UserID)
		idx++
	}
	if serverFilter != "" {
		query += fmt.Sprintf(" AND server_slug = $%d", idx)
		args = append(args, serverFilter)
		idx++
	}
	if toolFilter != "" {
		query += fmt.Sprintf(" AND tool = $%d", idx)
		args = append(args, toolFilter)
		idx++
	}
	if outcomeFilter != "" {
		query += fmt.Sprintf(" AND outcome = $%d", idx)
		args = append(args, outcomeFilter)
		idx++
	}

	query += " ORDER BY ts DESC LIMIT 100"

	rows, err := s.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		http.Error(w, `{"error":"Database error"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type auditRow struct {
		ID          int64     `json:"id"`
		TS          time.Time `json:"ts"`
		UserEmail   string    `json:"user_email"`
		Channel     string    `json:"channel"`
		ServerSlug  string    `json:"server_slug"`
		Tool        string    `json:"tool"`
		ExposedName string    `json:"exposed_name"`
		Outcome     string    `json:"outcome"`
		ErrorCode   string    `json:"error_code,omitempty"`
		DurationMs  int64     `json:"duration_ms"`
		RequestID   string    `json:"request_id"`
	}

	records := make([]auditRow, 0)
	for rows.Next() {
		var row auditRow
		var errCode sql.NullString
		if err := rows.Scan(&row.ID, &row.TS, &row.UserEmail, &row.Channel, &row.ServerSlug, &row.Tool, &row.ExposedName, &row.Outcome, &errCode, &row.DurationMs, &row.RequestID); err == nil {
			if errCode.Valid {
				row.ErrorCode = errCode.String
			}
			records = append(records, row)
		}
	}

	writeJSON(w, http.StatusOK, records)
}

// --- Users & Invites ---

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.QueryContext(r.Context(), `
		SELECT u.id, u.email, u.role, u.active, u.created_at, COUNT(k.id) as key_count
		FROM users u
		LEFT JOIN api_keys k ON u.id = k.user_id AND k.revoked_at IS NULL
		GROUP BY u.id
		ORDER BY u.id ASC
	`)
	if err != nil {
		http.Error(w, `{"error":"Database error"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type userDTO struct {
		ID        int       `json:"id"`
		Email     string    `json:"email"`
		Role      string    `json:"role"`
		Active    bool      `json:"active"`
		CreatedAt time.Time `json:"created_at"`
		KeyCount  int       `json:"key_count"`
	}

	users := make([]userDTO, 0)
	for rows.Next() {
		var u userDTO
		if err := rows.Scan(&u.ID, &u.Email, &u.Role, &u.Active, &u.CreatedAt, &u.KeyCount); err == nil {
			users = append(users, u)
		}
	}

	writeJSON(w, http.StatusOK, users)
}

func (s *Server) handleCreateInvite(w http.ResponseWriter, r *http.Request) {
	p, _ := auth.GetPrincipal(r.Context())

	var req struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"Invalid request"}`, http.StatusBadRequest)
		return
	}

	tokenBytes := make([]byte, 16)
	_, _ = rand.Read(tokenBytes)
	token := hex.EncodeToString(tokenBytes)
	tokenHash := crypto.HashString(token)

	expiresAt := time.Now().Add(72 * time.Hour)
	_, err := s.db.ExecContext(r.Context(), `
		INSERT INTO invites (email, role, token_hash, expires_at, created_by)
		VALUES ($1, $2, $3, $4, $5)
	`, req.Email, req.Role, tokenHash, expiresAt, p.UserID)
	if err != nil {
		http.Error(w, `{"error":"Database error"}`, http.StatusInternalServerError)
		return
	}

	inviteLink := fmt.Sprintf("http://app.localtest.me/invite?token=%s", token)
	writeJSON(w, http.StatusCreated, map[string]any{
		"email":       req.Email,
		"invite_link": inviteLink,
		"expires_at":  expiresAt,
	})
}

func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, _ := strconv.Atoi(idStr)

	var req struct {
		Active *bool   `json:"active"`
		Role   *string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"Invalid request"}`, http.StatusBadRequest)
		return
	}

	if req.Active != nil {
		_, _ = s.db.ExecContext(r.Context(), "UPDATE users SET active = $1 WHERE id = $2", *req.Active, id)
		if !*req.Active {
			// Deactivation immediately revokes all keys and sessions (PRD section 9)
			_, _ = s.db.ExecContext(r.Context(), "UPDATE api_keys SET revoked_at = NOW() WHERE user_id = $1 AND revoked_at IS NULL", id)
			_, _ = s.db.ExecContext(r.Context(), "DELETE FROM sessions WHERE user_id = $1", id)
		}
	}
	if req.Role != nil {
		_, _ = s.db.ExecContext(r.Context(), "UPDATE users SET role = $1 WHERE id = $2", *req.Role, id)
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// --- Chat & SSE Handlers ---

type sseWriter struct {
	w http.ResponseWriter
	f http.Flusher
}

func (s *sseWriter) Send(event string, payload any) error {
	data, _ := json.Marshal(payload)
	_, err := fmt.Fprintf(s.w, "event: %s\ndata: %s\n\n", event, string(data))
	if err == nil {
		s.f.Flush()
	}
	return err
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	p, ok := auth.GetPrincipal(r.Context())
	if !ok {
		http.Error(w, `{"error":"Unauthorized"}`, http.StatusUnauthorized)
		return
	}

	var req struct {
		ConversationID string `json:"conversation_id"`
		Message        string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"Invalid request"}`, http.StatusBadRequest)
		return
	}

	if req.ConversationID == "" {
		req.ConversationID = uuid.New().String()
		_, _ = s.db.ExecContext(r.Context(), `
			INSERT INTO chat_conversations (id, user_id, title)
			VALUES ($1, $2, $3)
		`, req.ConversationID, p.UserID, "New Conversation")
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	writer := &sseWriter{w: w, f: flusher}
	_ = s.agent.HandleChat(r.Context(), p, req.ConversationID, req.Message, writer)
}

func (s *Server) handleListConversations(w http.ResponseWriter, r *http.Request) {
	p, ok := auth.GetPrincipal(r.Context())
	if !ok {
		http.Error(w, `{"error":"Unauthorized"}`, http.StatusUnauthorized)
		return
	}

	rows, err := s.db.QueryContext(r.Context(), `
		SELECT id, title, created_at
		FROM chat_conversations
		WHERE user_id = $1
		ORDER BY created_at DESC LIMIT 20
	`, p.UserID)
	if err != nil {
		http.Error(w, `{"error":"Database error"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type convDTO struct {
		ID        string    `json:"id"`
		Title     string    `json:"title"`
		CreatedAt time.Time `json:"created_at"`
	}

	list := make([]convDTO, 0)
	for rows.Next() {
		var c convDTO
		if err := rows.Scan(&c.ID, &c.Title, &c.CreatedAt); err == nil {
			list = append(list, c)
		}
	}

	writeJSON(w, http.StatusOK, list)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
