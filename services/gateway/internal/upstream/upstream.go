package upstream

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"eureka/services/gateway/internal/mcp"
)

// ServerStatus values defined in PRD Section 8.
const (
	StatusConnected   = "Connected"
	StatusUnreachable = "Unreachable"
	StatusDisabled    = "Disabled"
)

// UpstreamServer represents a single registered upstream MCP provider.
type UpstreamServer struct {
	mu sync.RWMutex

	ID             int
	Slug           string
	Name           string
	URL            string
	Enabled        bool
	AuthKind       string
	AuthHeader     string
	Credential     string
	CallTimeout    time.Duration

	Status         string
	Reason         string
	LastError      string
	LastSuccess    time.Time
	FailedPings    int

	// Circuit Breaker
	CBState        string // "closed", "open", "half-open"
	CBFailures     int
	CBOpenTime     time.Time

	// Bulkhead
	bulkhead       chan struct{}

	// Cached Tool Snapshot
	tools          []mcp.Tool
	toolsMu        sync.RWMutex

	httpClient     *http.Client
	logger         *slog.Logger
}

// NewUpstreamServer creates a new supervised upstream server instance.
func NewUpstreamServer(id int, slug, name, url string, enabled bool, authKind, authHeader, cred string, timeoutMs int, logger *slog.Logger) *UpstreamServer {
	if timeoutMs <= 0 {
		timeoutMs = 30000
	}

	status := StatusConnected
	if !enabled {
		status = StatusDisabled
	}

	return &UpstreamServer{
		ID:          id,
		Slug:        slug,
		Name:        name,
		URL:         url,
		Enabled:     enabled,
		AuthKind:    authKind,
		AuthHeader:  authHeader,
		Credential:  cred,
		CallTimeout: time.Duration(timeoutMs) * time.Millisecond,
		Status:      status,
		CBState:     "closed",
		bulkhead:    make(chan struct{}, 20), // Bulkhead: 20 slots
		tools:       make([]mcp.Tool, 0),
		httpClient: &http.Client{
			Timeout: 3 * time.Second, // Fast connect/ping timeout
		},
		logger: logger.With("upstream", slug),
	}
}

// GetStatus returns the current status and diagnostic reasons.
func (u *UpstreamServer) GetStatus() (status, reason, lastErr string, lastSuccess time.Time) {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.Status, u.Reason, u.LastError, u.LastSuccess
}

// GetTools returns the current cached tool snapshot.
func (u *UpstreamServer) GetTools() []mcp.Tool {
	u.toolsMu.RLock()
	defer u.toolsMu.RUnlock()
	res := make([]mcp.Tool, len(u.tools))
	copy(res, u.tools)
	return res
}

// Ping checks server liveness and refreshes tools.
func (u *UpstreamServer) Ping(ctx context.Context) error {
	u.mu.Lock()
	if !u.Enabled {
		u.Status = StatusDisabled
		u.mu.Unlock()
		return nil
	}
	u.mu.Unlock()

	reqBody := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      "ping-1",
		Method:  "ping",
	}

	resp, err := u.doRPC(ctx, reqBody, 5*time.Second)
	if err != nil {
		u.handlePingFailure(err)
		return err
	}

	if resp.Error != nil {
		u.handlePingFailure(errors.New(resp.Error.Message))
		return errors.New(resp.Error.Message)
	}

	u.handlePingSuccess()

	// Also refresh tools periodically
	_ = u.RefreshTools(ctx)
	return nil
}

// RefreshTools fetches tools/list from upstream and updates the snapshot.
func (u *UpstreamServer) RefreshTools(ctx context.Context) error {
	reqBody := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      "list-1",
		Method:  "tools/list",
	}

	resp, err := u.doRPC(ctx, reqBody, 10*time.Second)
	if err != nil {
		return err
	}

	if resp.Result == nil {
		return errors.New("empty tools/list result")
	}

	var res struct {
		Tools []mcp.Tool `json:"tools"`
	}
	data, _ := json.Marshal(resp.Result)
	if err := json.Unmarshal(data, &res); err != nil {
		return err
	}

	u.toolsMu.Lock()
	u.tools = res.Tools
	u.toolsMu.Unlock()

	return nil
}

// Call executes tools/call against the upstream server.
func (u *UpstreamServer) Call(ctx context.Context, tool string, args map[string]any) (*mcp.ToolResult, error) {
	u.mu.Lock()
	if !u.Enabled {
		u.mu.Unlock()
		return nil, errors.New("server_disabled")
	}

	// Circuit breaker check
	now := time.Now()
	if u.CBState == "open" {
		if now.Sub(u.CBOpenTime) > 15*time.Second {
			u.CBState = "half-open"
		} else {
			u.mu.Unlock()
			return nil, errors.New("circuit_breaker_open")
		}
	}
	u.mu.Unlock()

	// Bulkhead slot acquisition (concurrency control)
	select {
	case u.bulkhead <- struct{}{}:
		defer func() { <-u.bulkhead }()
	default:
		return nil, errors.New("bulkhead_full")
	}

	timeout := u.CallTimeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	reqBody := mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      fmt.Sprintf("call-%d", time.Now().UnixNano()),
		Method:  "tools/call",
		Params: mustMarshal(map[string]any{
			"name":      tool,
			"arguments": args,
		}),
	}

	resp, err := u.doRPC(ctx, reqBody, timeout)
	if err != nil {
		u.recordCallFailure()
		return nil, err
	}

	if resp.Error != nil {
		u.recordCallFailure()
		return nil, fmt.Errorf("upstream error: %s", resp.Error.Message)
	}

	u.recordCallSuccess()

	var result mcp.ToolResult
	data, _ := json.Marshal(resp.Result)
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse tool result: %w", err)
	}

	return &result, nil
}

func (u *UpstreamServer) doRPC(ctx context.Context, reqBody mcp.JSONRPCRequest, timeout time.Duration) (*mcp.JSONRPCResponse, error) {
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(callCtx, http.MethodPost, u.URL, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	// Attach private upstream credentials (never leaked to clients)
	if u.Credential != "" {
		if u.AuthKind == "bearer" || u.AuthHeader == "" {
			req.Header.Set("Authorization", "Bearer "+u.Credential)
		} else {
			req.Header.Set(u.AuthHeader, u.Credential)
		}
	}

	resp, err := u.httpClient.Do(req)
	if err != nil {
		if errors.Is(callCtx.Err(), context.DeadlineExceeded) {
			return nil, errors.New("timeout")
		}
		if strings.Contains(err.Error(), "connection refused") {
			return nil, errors.New("connection_refused")
		}
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, errors.New("auth_failed")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad_response_%d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Handle SSE formatted responses (Streamable HTTP spec 2026-07-28: event: message\ndata: {...})
	bodyStr := strings.TrimSpace(string(bodyBytes))
	if strings.Contains(bodyStr, "data: ") {
		for _, line := range strings.Split(bodyStr, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "data: ") {
				bodyBytes = []byte(strings.TrimPrefix(line, "data: "))
				break
			}
		}
	}

	var jsonResp mcp.JSONRPCResponse
	if err := json.Unmarshal(bodyBytes, &jsonResp); err != nil {
		return nil, fmt.Errorf("protocol_error: %w", err)
	}

	return &jsonResp, nil
}

func (u *UpstreamServer) handlePingSuccess() {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.Status = StatusConnected
	u.FailedPings = 0
	u.LastSuccess = time.Now()
	u.LastError = ""
	u.Reason = ""
}

func (u *UpstreamServer) handlePingFailure(err error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.FailedPings++
	u.LastError = err.Error()

	errStr := err.Error()
	if strings.Contains(errStr, "connection_refused") {
		u.Reason = "connection_refused"
	} else if strings.Contains(errStr, "timeout") {
		u.Reason = "timeout"
	} else if strings.Contains(errStr, "auth_failed") {
		u.Reason = "auth_failed"
	} else {
		u.Reason = "unreachable"
	}

	if u.FailedPings >= 3 {
		u.Status = StatusUnreachable
	}
}

func (u *UpstreamServer) recordCallFailure() {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.CBFailures++
	if u.CBFailures >= 5 {
		u.CBState = "open"
		u.CBOpenTime = time.Now()
	}
}

func (u *UpstreamServer) recordCallSuccess() {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.CBFailures = 0
	u.CBState = "closed"
}

func mustMarshal(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
