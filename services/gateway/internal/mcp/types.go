package mcp

import "encoding/json"

// Standard JSON-RPC 2.0 error codes.
const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603
)

// JSONRPCRequest represents a JSON-RPC 2.0 message.
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response.
type JSONRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      any           `json:"id,omitempty"`
	Result  any           `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
}

// JSONRPCError defines the standard error envelope.
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Tool represents an MCP tool definition.
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"inputSchema"`
}

// TextContent represents text output in an MCP result.
type TextContent struct {
	Type string `json:"type"` // "text"
	Text string `json:"text"`
}

// ToolResult represents the output of a tools/call execution.
type ToolResult struct {
	Content []TextContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

// StructuredError models the error payload defined in PRD Section 8.
type StructuredError struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail describes the error reason for the model.
type ErrorDetail struct {
	Code      string `json:"code"`
	Server    string `json:"server"`
	Tool      string `json:"tool"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}
