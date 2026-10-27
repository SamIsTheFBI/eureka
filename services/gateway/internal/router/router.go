package router

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"eureka/services/gateway/internal/aggregate"
	"eureka/services/gateway/internal/audit"
	"eureka/services/gateway/internal/auth"
	"eureka/services/gateway/internal/mcp"
	"eureka/services/gateway/internal/upstream"
	"github.com/google/uuid"
)

// Router dispatches tools/call invocations to upstream providers with audit and resilience.
type Router struct {
	manager *upstream.Manager
	audit   *audit.Writer
	logger  *slog.Logger
}

// NewRouter creates a new Router.
func NewRouter(manager *upstream.Manager, auditWriter *audit.Writer, logger *slog.Logger) *Router {
	return &Router{
		manager: manager,
		audit:   auditWriter,
		logger:  logger,
	}
}

// Call routes a tool call, applies policies, and audits the result.
func (r *Router) Call(ctx context.Context, principal *auth.Principal, exposedName string, args map[string]any) (*mcp.ToolResult, error) {
	start := time.Now()
	requestID := uuid.New().String()

	argsBytes, _ := json.Marshal(args)
	argsHash := sha256.Sum256(argsBytes)
	argsHashHex := hex.EncodeToString(argsHash[:])

	slug, originalTool, ok := aggregate.DeconstructTool(exposedName)
	if !ok {
		r.auditCall(principal, requestID, "", exposedName, exposedName, "denied", "tool_not_found", start, argsHashHex, len(argsBytes))
		return r.structuredError("tool_not_found", "", exposedName, "Invalid namespaced tool name format", false), nil
	}

	srv, exists := r.manager.GetServer(slug)
	if !exists {
		r.auditCall(principal, requestID, slug, originalTool, exposedName, "denied", "tool_not_found", start, argsHashHex, len(argsBytes))
		return r.structuredError("tool_not_found", slug, originalTool, fmt.Sprintf("Upstream server '%s' not registered", slug), false), nil
	}

	if !srv.Enabled {
		r.auditCall(principal, requestID, slug, originalTool, exposedName, "denied", "server_disabled", start, argsHashHex, len(argsBytes))
		return r.structuredError("server_disabled", slug, originalTool, fmt.Sprintf("Upstream server '%s' is disabled", slug), false), nil
	}

	status, reason, _, _ := srv.GetStatus()
	if status == upstream.StatusUnreachable {
		r.auditCall(principal, requestID, slug, originalTool, exposedName, "unreachable", "upstream_unreachable", start, argsHashHex, len(argsBytes))
		return r.structuredError("upstream_unreachable", slug, originalTool, fmt.Sprintf("Upstream server '%s' is unreachable (%s)", slug, reason), true), nil
	}

	// Verify tool presence in snapshot to prevent arbitrary string forwarding
	cachedTools := srv.GetTools()
	found := false
	for _, t := range cachedTools {
		if t.Name == originalTool {
			found = true
			break
		}
	}
	if !found && len(cachedTools) > 0 {
		r.auditCall(principal, requestID, slug, originalTool, exposedName, "denied", "tool_not_found", start, argsHashHex, len(argsBytes))
		return r.structuredError("tool_not_found", slug, originalTool, fmt.Sprintf("Tool '%s' not advertised by upstream '%s'", originalTool, slug), false), nil
	}

	// Execute call against upstream
	result, err := srv.Call(ctx, originalTool, args)
	durationMs := time.Since(start).Milliseconds()

	if err != nil {
		errStr := err.Error()
		outcome := "tool_error"
		code := "upstream_error"
		retryable := false

		if strings.Contains(errStr, "timeout") {
			outcome = "timeout"
			code = "upstream_timeout"
			retryable = true
		} else if strings.Contains(errStr, "circuit_breaker") || strings.Contains(errStr, "connection_refused") {
			outcome = "unreachable"
			code = "upstream_unreachable"
			retryable = true
		} else if strings.Contains(errStr, "server_disabled") {
			outcome = "denied"
			code = "server_disabled"
		}

		r.auditCall(principal, requestID, slug, originalTool, exposedName, outcome, code, start, argsHashHex, len(argsBytes))
		return r.structuredError(code, slug, originalTool, errStr, retryable), nil
	}

	outcome := "success"
	if result.IsError {
		outcome = "tool_error"
	}

	r.audit.Log(audit.Record{
		TS:          start,
		UserID:      principal.UserID,
		UserEmail:   principal.UserEmail,
		KeyID:       principal.KeyID,
		Channel:     principal.Channel,
		ServerSlug:  slug,
		Tool:        originalTool,
		ExposedName: exposedName,
		Outcome:     outcome,
		ErrorCode:   "",
		DurationMs:  durationMs,
		RequestID:   requestID,
		ArgsHash:    argsHashHex,
		ArgsBytes:   len(argsBytes),
	})

	return result, nil
}

func (r *Router) structuredError(code, server, tool, message string, retryable bool) *mcp.ToolResult {
	errPayload := mcp.StructuredError{
		Error: mcp.ErrorDetail{
			Code:      code,
			Server:    server,
			Tool:      tool,
			Message:   message,
			Retryable: retryable,
		},
	}
	data, _ := json.Marshal(errPayload)
	return &mcp.ToolResult{
		Content: []mcp.TextContent{
			{Type: "text", Text: string(data)},
		},
		IsError: true,
	}
}

func (r *Router) auditCall(p *auth.Principal, reqID, slug, tool, exposed, outcome, code string, start time.Time, argsHash string, argsLen int) {
	channel := "mcp"
	email := "anonymous"
	uid := 0
	var kid *int

	if p != nil {
		channel = p.Channel
		email = p.UserEmail
		uid = p.UserID
		kid = p.KeyID
	}

	r.audit.Log(audit.Record{
		TS:          start,
		UserID:      uid,
		UserEmail:   email,
		KeyID:       kid,
		Channel:     channel,
		ServerSlug:  slug,
		Tool:        tool,
		ExposedName: exposed,
		Outcome:     outcome,
		ErrorCode:   code,
		DurationMs:  time.Since(start).Milliseconds(),
		RequestID:   reqID,
		ArgsHash:    argsHash,
		ArgsBytes:   argsLen,
	})
}
