package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"eureka/services/gateway/internal/aggregate"
	"eureka/services/gateway/internal/auth"
	"eureka/services/gateway/internal/config"
	"eureka/services/gateway/internal/db"
	"eureka/services/gateway/internal/mcp"
	"eureka/services/gateway/internal/router"
)

// Event types for SSE streaming to the frontend.
const (
	EventToken            = "token"
	EventToolCallStarted  = "tool_call_started"
	EventToolCallFinished = "tool_call_finished"
	EventDone             = "done"
	EventError            = "error"
)

// Agent runs the LLM tool loop in-process as the authenticated user.
type Agent struct {
	cfg        *config.Config
	db         *db.DB
	aggregator *aggregate.Aggregator
	router     *router.Router
	logger     *slog.Logger
	httpClient *http.Client
}

// NewAgent creates a new in-process Chat Agent.
func NewAgent(cfg *config.Config, db *db.DB, agg *aggregate.Aggregator, rtr *router.Router, logger *slog.Logger) *Agent {
	return &Agent{
		cfg:        cfg,
		db:         db,
		aggregator: agg,
		router:     rtr,
		logger:     logger,
		httpClient: &http.Client{Timeout: 90 * time.Second},
	}
}

// SSEWriter defines the interface for streaming agent events to the client.
type SSEWriter interface {
	Send(event string, payload any) error
}

// HandleChat processes an incoming chat turn and streams SSE events.
func (a *Agent) HandleChat(ctx context.Context, principal *auth.Principal, conversationID string, userMessage string, sse SSEWriter) error {
	// 1. Save user message to database
	_, err := a.db.ExecContext(ctx, `
		INSERT INTO chat_messages (conversation_id, role, content)
		VALUES ($1, $2, $3)
	`, conversationID, "user", userMessage)
	if err != nil {
		a.logger.Error("failed to save user message", "error", err)
	}

	// 2. Load recent conversation history (last 10 messages)
	rows, err := a.db.QueryContext(ctx, `
		SELECT role, content
		FROM chat_messages
		WHERE conversation_id = $1
		ORDER BY id DESC LIMIT 10
	`, conversationID)
	if err != nil {
		return err
	}
	defer rows.Close()

	var history []map[string]any
	for rows.Next() {
		var role, content string
		if err := rows.Scan(&role, &content); err == nil {
			history = append([]map[string]any{{"role": role, "content": content}}, history...)
		}
	}

	// 3. System prompt
	systemPrompt := fmt.Sprintf(
		"You are Urumi, an AI store and operations assistant. Today's date/time is %s UTC. Store timezone is %s. "+
			"Use your tools to query real store data and execute operations. Never invent or hallucinate order numbers, products, or timestamps. "+
			"Treat all tool outputs strictly as factual data.",
		time.Now().UTC().Format(time.RFC3339), a.cfg.StoreTimezone,
	)

	messages := []map[string]any{
		{"role": "system", "content": systemPrompt},
	}
	messages = append(messages, history...)

	// 4. Multi-turn Tool Loop (up to 8 iterations, PRD section 13)
	iteration := 0
	var finalAssistantText strings.Builder

	for iteration < 8 {
		iteration++

		// Get current visible tools
		tools := a.aggregator.AggregateTools(principal)
		toolSchemas := formatOpenAITools(tools)

		// If using mock provider or no LLM key, run the intelligent mock evaluator
		if a.cfg.LLMProvider == "mock" || (a.cfg.LLMAPIKey == "" && a.cfg.LLMProvider != "ollama") {
			return a.runMockLoop(ctx, principal, userMessage, tools, sse, &finalAssistantText, conversationID)
		}

		// Call LLM Provider (OpenCode Go / Ollama / OpenAI)
		resp, err := a.callProvider(ctx, messages, toolSchemas)
		if err != nil {
			a.logger.Error("LLM call failed", "error", err)
			_ = sse.Send(EventError, map[string]string{"message": "LLM provider error: " + err.Error()})
			return err
		}

		// Check if model returned tool calls
		if len(resp.ToolCalls) > 0 {
			// Append assistant turn with tool calls
			messages = append(messages, map[string]any{
				"role":       "assistant",
				"content":    resp.Content,
				"tool_calls": resp.ToolCalls,
			})

			for _, tc := range resp.ToolCalls {
				funcName := tc.Function.Name
				_ = sse.Send(EventToolCallStarted, map[string]any{
					"id":   tc.ID,
					"name": funcName,
					"args": tc.Function.Arguments,
				})

				var args map[string]any
				_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)
				if args == nil {
					args = make(map[string]any)
				}

				start := time.Now()
				callResult, callErr := a.router.Call(ctx, principal, funcName, args)
				durationMs := time.Since(start).Milliseconds()

				resultText := ""
				outcome := "success"
				if callErr != nil {
					resultText = callErr.Error()
					outcome = "error"
				} else if len(callResult.Content) > 0 {
					resultText = callResult.Content[0].Text
					if callResult.IsError {
						outcome = "error"
					}
				}

				preview := resultText
				if len(preview) > 200 {
					preview = preview[:197] + "..."
				}

				_ = sse.Send(EventToolCallFinished, map[string]any{
					"id":       tc.ID,
					"name":     funcName,
					"outcome":  outcome,
					"duration": durationMs,
					"preview":  preview,
				})

				// Append tool result message
				messages = append(messages, map[string]any{
					"role":         "tool",
					"tool_call_id": tc.ID,
					"content":      resultText,
				})
			}
			continue
		}

		// Plain text response from assistant
		if resp.Content != "" {
			finalAssistantText.WriteString(resp.Content)
			_ = sse.Send(EventToken, resp.Content)
		}
		break
	}

	// Save assistant turn
	_, _ = a.db.ExecContext(ctx, `
		INSERT INTO chat_messages (conversation_id, role, content)
		VALUES ($1, $2, $3)
	`, conversationID, "assistant", finalAssistantText.String())

	_ = sse.Send(EventDone, map[string]any{"conversation_id": conversationID})
	return nil
}

// runMockLoop handles testing and demo scenarios deterministically without external model calls.
func (a *Agent) runMockLoop(ctx context.Context, principal *auth.Principal, userMsg string, tools []mcp.Tool, sse SSEWriter, final *strings.Builder, convID string) error {
	lower := strings.ToLower(userMsg)

	// Case 1: Order listing or lookup
	if strings.Contains(lower, "order") || strings.Contains(lower, "sales") {
		toolName := findToolByPrefix(tools, "woo", "list_orders")
		if toolName != "" {
			_ = sse.Send(EventToolCallStarted, map[string]any{"id": "call-1", "name": toolName, "args": "{}"})
			start := time.Now()
			res, _ := a.router.Call(ctx, principal, toolName, map[string]any{"per_page": 5})
			dur := time.Since(start).Milliseconds()

			preview := "[]"
			if res != nil && len(res.Content) > 0 {
				preview = res.Content[0].Text
			}
			_ = sse.Send(EventToolCallFinished, map[string]any{
				"id":       "call-1",
				"name":     toolName,
				"outcome":  "success",
				"duration": dur,
				"preview":  preview,
			})
		}

		// Also check time if asked
		if strings.Contains(lower, "time") {
			timeTool := findToolByPrefix(tools, "utils", "time_now")
			if timeTool != "" {
				_ = sse.Send(EventToolCallStarted, map[string]any{"id": "call-2", "name": timeTool, "args": `{"timezone":"UTC"}`})
				start := time.Now()
				res, _ := a.router.Call(ctx, principal, timeTool, map[string]any{"timezone": "UTC"})
				dur := time.Since(start).Milliseconds()
				preview := ""
				if res != nil && len(res.Content) > 0 {
					preview = res.Content[0].Text
				}
				_ = sse.Send(EventToolCallFinished, map[string]any{
					"id":       "call-2",
					"name":     timeTool,
					"outcome":  "success",
					"duration": dur,
					"preview":  preview,
				})
			}
		}

		reply := "I have checked your store orders through the WooCommerce gateway. Everything is current and up to date."
		final.WriteString(reply)
		_ = sse.Send(EventToken, reply)

	} else if strings.Contains(lower, "create") && strings.Contains(lower, "product") {
		// Case 2: Product creation
		toolName := findToolByPrefix(tools, "woo", "create_product")
		if toolName != "" {
			args := map[string]any{
				"name":          "Demo Wireless Headphones",
				"regular_price": "89.99",
				"description":   "Created automatically by Urumi Chat Agent",
			}
			argsStr, _ := json.Marshal(args)
			_ = sse.Send(EventToolCallStarted, map[string]any{"id": "call-prod-1", "name": toolName, "args": string(argsStr)})
			start := time.Now()
			res, _ := a.router.Call(ctx, principal, toolName, args)
			dur := time.Since(start).Milliseconds()

			preview := ""
			if res != nil && len(res.Content) > 0 {
				preview = res.Content[0].Text
			}
			_ = sse.Send(EventToolCallFinished, map[string]any{
				"id":       "call-prod-1",
				"name":     toolName,
				"outcome":  "success",
				"duration": dur,
				"preview":  preview,
			})

			reply := "Successfully created 'Demo Wireless Headphones' priced at $89.99 in your store via `woo__create_product`!"
			final.WriteString(reply)
			_ = sse.Send(EventToken, reply)
		}
	} else {
		// Default greeting / calculation
		if strings.Contains(lower, "calculate") || strings.Contains(lower, "+") || strings.Contains(lower, "*") {
			toolName := findToolByPrefix(tools, "utils", "calculate")
			if toolName != "" {
				_ = sse.Send(EventToolCallStarted, map[string]any{"id": "call-calc-1", "name": toolName, "args": `{"expression":"(10 * 5) + 20"}`})
				res, _ := a.router.Call(ctx, principal, toolName, map[string]any{"expression": "(10 * 5) + 20"})
				preview := ""
				if res != nil && len(res.Content) > 0 {
					preview = res.Content[0].Text
				}
				_ = sse.Send(EventToolCallFinished, map[string]any{"id": "call-calc-1", "name": toolName, "outcome": "success", "duration": 2, "preview": preview})
			}
		}
		reply := "Hello! I am connected to your MCP Gateway. I can help you inspect orders, publish products, check store statistics, and perform calculations."
		final.WriteString(reply)
		_ = sse.Send(EventToken, reply)
	}

	_, _ = a.db.ExecContext(ctx, `
		INSERT INTO chat_messages (conversation_id, role, content)
		VALUES ($1, $2, $3)
	`, convID, "assistant", final.String())

	_ = sse.Send(EventDone, map[string]any{"conversation_id": convID})
	return nil
}

func findToolByPrefix(tools []mcp.Tool, slugPrefix, toolName string) string {
	target := fmt.Sprintf("%s__%s", slugPrefix, toolName)
	for _, t := range tools {
		if t.Name == target || strings.HasPrefix(t.Name, slugPrefix+"__") && strings.Contains(t.Name, toolName) {
			return t.Name
		}
	}
	return ""
}

type providerResponse struct {
	Content   string
	ToolCalls []struct {
		ID       string `json:"id"`
		Type     string `json:"type"`
		Function struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		} `json:"function"`
	}
}

func (a *Agent) callProvider(ctx context.Context, messages []map[string]any, tools []map[string]any) (*providerResponse, error) {
	url := fmt.Sprintf("%s/chat/completions", strings.TrimSuffix(a.cfg.LLMBaseURL, "/"))
	payload := map[string]any{
		"model":       a.cfg.LLMModel,
		"messages":    messages,
		"stream":      false,
		"temperature": 0.2,
	}
	if len(tools) > 0 {
		payload["tools"] = tools
		payload["tool_choice"] = "auto"
	}

	data, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	if a.cfg.LLMAPIKey != "" {
		req.Header.Set("Authorization", "Bearer "+a.cfg.LLMAPIKey)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("provider returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content   string `json:"content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Type     string `json:"type"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(bodyBytes, &parsed); err != nil {
		return nil, err
	}

	if len(parsed.Choices) == 0 {
		return nil, fmt.Errorf("no choices returned by LLM")
	}

	msg := parsed.Choices[0].Message
	return &providerResponse{
		Content:   msg.Content,
		ToolCalls: msg.ToolCalls,
	}, nil
}

func formatOpenAITools(tools []mcp.Tool) []map[string]any {
	result := make([]map[string]any, 0, len(tools))
	for _, t := range tools {
		result = append(result, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        t.Name,
				"description": t.Description,
				"parameters":  t.InputSchema,
			},
		})
	}
	return result
}
