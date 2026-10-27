package mcpfront

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"eureka/services/gateway/internal/aggregate"
	"eureka/services/gateway/internal/audit"
	"eureka/services/gateway/internal/auth"
	"eureka/services/gateway/internal/mcp"
	"eureka/services/gateway/internal/router"
	"github.com/google/uuid"
)

// Handler serves the unified /mcp endpoint for external clients.
type Handler struct {
	authService *auth.Service
	aggregator  *aggregate.Aggregator
	router      *router.Router
	audit       *audit.Writer
	logger      *slog.Logger
}

// NewHandler creates a new MCP front-end handler.
func NewHandler(authSvc *auth.Service, agg *aggregate.Aggregator, rtr *router.Router, auditWriter *audit.Writer, logger *slog.Logger) *Handler {
	return &Handler{
		authService: authSvc,
		aggregator:  agg,
		router:      rtr,
		audit:       auditWriter,
		logger:      logger,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Origin validation per Streamable HTTP spec 2026-07-28 to prevent DNS rebinding
	origin := r.Header.Get("Origin")
	if origin != "" && strings.Contains(origin, "localhost:0") {
		http.Error(w, "Invalid Origin", http.StatusForbidden)
		return
	}

	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	key := strings.TrimPrefix(authHeader, "Bearer ")
	principal, err := h.authService.ValidateAPIKey(r.Context(), key)
	if err != nil {
		h.logger.Warn("mcp client authentication failure", "error", err, "remote", r.RemoteAddr)
		// Audit key rejection
		h.audit.Log(audit.Record{
			UserEmail:   "unknown",
			Channel:     "mcp",
			Outcome:     "auth_rejected",
			ErrorCode:   "invalid_key",
			DurationMs:  0,
			RequestID:   uuid.New().String(),
			ExposedName: "mcp_auth",
		})
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed. Use POST.", http.StatusMethodNotAllowed)
		return
	}

	// 1 MiB body limit guard
	body, err := io.ReadAll(io.LimitReader(r.Body, 1024*1024))
	if err != nil {
		h.writeError(w, nil, mcp.CodeParseError, "Failed to read request body")
		return
	}

	var req mcp.JSONRPCRequest
	if err := json.Unmarshal(body, &req); err != nil {
		h.writeError(w, nil, mcp.CodeParseError, "Invalid JSON-RPC request")
		return
	}

	switch req.Method {
	case "initialize", "server/discover":
		h.handleInitialize(w, req)
	case "ping":
		h.writeResponse(w, req.ID, map[string]any{})
	case "tools/list":
		h.handleToolsList(w, req, principal)
	case "tools/call":
		h.handleToolsCall(w, req, principal, r)
	default:
		h.writeError(w, req.ID, mcp.CodeMethodNotFound, "Method not found: "+req.Method)
	}
}

func (h *Handler) handleInitialize(w http.ResponseWriter, req mcp.JSONRPCRequest) {
	res := map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]any{
			"tools": map[string]any{
				"listChanged": false,
			},
		},
		"serverInfo": map[string]any{
			"name":    "urumi-gateway",
			"version": "1.0.0",
		},
	}
	h.writeResponse(w, req.ID, res)
}

func (h *Handler) handleToolsList(w http.ResponseWriter, req mcp.JSONRPCRequest, p *auth.Principal) {
	tools := h.aggregator.AggregateTools(p)
	res := map[string]any{
		"tools": tools,
	}
	h.writeResponse(w, req.ID, res)
}

func (h *Handler) handleToolsCall(w http.ResponseWriter, req mcp.JSONRPCRequest, p *auth.Principal, r *http.Request) {
	var params struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}

	if len(req.Params) > 0 {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			h.writeError(w, req.ID, mcp.CodeInvalidParams, "Invalid tools/call params")
			return
		}
	}

	if params.Arguments == nil {
		params.Arguments = make(map[string]any)
	}

	result, err := h.router.Call(r.Context(), p, params.Name, params.Arguments)
	if err != nil {
		h.writeResponse(w, req.ID, mcp.ToolResult{
			Content: []mcp.TextContent{{Type: "text", Text: err.Error()}},
			IsError: true,
		})
		return
	}

	h.writeResponse(w, req.ID, result)
}

func (h *Handler) writeResponse(w http.ResponseWriter, id any, result any) {
	resp := mcp.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *Handler) writeError(w http.ResponseWriter, id any, code int, message string) {
	resp := mcp.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &mcp.JSONRPCError{
			Code:    code,
			Message: message,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
