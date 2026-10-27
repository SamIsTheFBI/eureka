package mcp

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

// ToolProvider defines the interface required to serve tools.
type ToolProvider interface {
	ListTools() []Tool
	Call(name string, args map[string]any) (*ToolResult, error)
}

// Handler implements the MCP Streamable HTTP endpoint for woo-mcp.
type Handler struct {
	provider   ToolProvider
	bearerToken string
	logger     *slog.Logger
}

// NewHandler creates a new MCP HTTP handler.
func NewHandler(provider ToolProvider, bearerToken string, logger *slog.Logger) *Handler {
	return &Handler{
		provider:    provider,
		bearerToken: bearerToken,
		logger:      logger,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Enforce bearer token authentication if configured
	if h.bearerToken != "" {
		auth := r.Header.Get("Authorization")
		token := ""
		if strings.HasPrefix(auth, "Bearer ") {
			token = strings.TrimPrefix(auth, "Bearer ")
		}
		if token != h.bearerToken {
			h.logger.Warn("unauthorized request to woo-mcp", "remote", r.RemoteAddr)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed. Use POST.", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.writeError(w, nil, CodeParseError, "Failed to read request body")
		return
	}

	var req JSONRPCRequest
	if err := json.Unmarshal(body, &req); err != nil {
		h.writeError(w, nil, CodeParseError, "Invalid JSON-RPC request")
		return
	}

	h.logger.Debug("received woo-mcp request", "method", req.Method, "id", req.ID)

	switch req.Method {
	case "initialize", "server/discover":
		h.handleInitialize(w, req)
	case "ping":
		h.writeResponse(w, req.ID, map[string]any{})
	case "tools/list":
		h.handleToolsList(w, req)
	case "tools/call":
		h.handleToolsCall(w, req)
	default:
		h.writeError(w, req.ID, CodeMethodNotFound, "Method not found: "+req.Method)
	}
}

func (h *Handler) handleInitialize(w http.ResponseWriter, req JSONRPCRequest) {
	res := map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]any{
			"tools": map[string]any{
				"listChanged": false,
			},
		},
		"serverInfo": map[string]any{
			"name":    "woo-mcp",
			"version": "1.0.0",
		},
	}
	h.writeResponse(w, req.ID, res)
}

func (h *Handler) handleToolsList(w http.ResponseWriter, req JSONRPCRequest) {
	tools := h.provider.ListTools()
	res := map[string]any{
		"tools": tools,
	}
	h.writeResponse(w, req.ID, res)
}

func (h *Handler) handleToolsCall(w http.ResponseWriter, req JSONRPCRequest) {
	var params ToolCallParams
	if len(req.Params) > 0 {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			h.writeError(w, req.ID, CodeInvalidParams, "Invalid tools/call params")
			return
		}
	}

	result, err := h.provider.Call(params.Name, params.Arguments)
	if err != nil {
		h.logger.Error("woo tool execution error", "tool", params.Name, "error", err)
		h.writeResponse(w, req.ID, ToolResult{
			Content: []TextContent{{Type: "text", Text: err.Error()}},
			IsError: true,
		})
		return
	}

	h.writeResponse(w, req.ID, result)
}

func (h *Handler) writeResponse(w http.ResponseWriter, id any, result any) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *Handler) writeError(w http.ResponseWriter, id any, code int, message string) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &JSONRPCError{
			Code:    code,
			Message: message,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
