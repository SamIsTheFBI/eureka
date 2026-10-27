package tools

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"eureka/services/utils-mcp/internal/mcp"
)

// Registry holds available tools and handles their execution.
type Registry struct {
	tools map[string]mcp.Tool
}

// NewRegistry initializes the tool registry with time_now and calculate.
func NewRegistry() *Registry {
	r := &Registry{
		tools: make(map[string]mcp.Tool),
	}

	r.tools["time_now"] = mcp.Tool{
		Name:        "time_now",
		Description: "Returns the current time in UTC and an optionally specified timezone.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"timezone": map[string]any{
					"type":        "string",
					"description": "Optional IANA timezone name (e.g., 'UTC', 'America/New_York', 'Asia/Kolkata', 'Europe/London'). Defaults to UTC.",
				},
			},
		},
	}

	r.tools["calculate"] = mcp.Tool{
		Name:        "calculate",
		Description: "Safely evaluates an arithmetic expression (+, -, *, /, parentheses) and returns the numeric result.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"expression": map[string]any{
					"type":        "string",
					"description": "Mathematical expression to evaluate, e.g. '(15 * 4) + 120 / 3'.",
				},
			},
			"required": []string{"expression"},
		},
	}

	return r
}

// ListTools returns the list of all registered tools.
func (r *Registry) ListTools() []mcp.Tool {
	list := make([]mcp.Tool, 0, len(r.tools))
	for _, t := range r.tools {
		list = append(list, t)
	}
	return list
}

// Call executes the named tool with the provided arguments.
func (r *Registry) Call(name string, args map[string]any) (*mcp.ToolResult, error) {
	switch name {
	case "time_now":
		return r.callTimeNow(args)
	case "calculate":
		return r.callCalculate(args)
	default:
		return &mcp.ToolResult{
			Content: []mcp.TextContent{{Type: "text", Text: fmt.Sprintf("Tool '%s' not found", name)}},
			IsError: true,
		}, nil
	}
}

func (r *Registry) callTimeNow(args map[string]any) (*mcp.ToolResult, error) {
	now := time.Now().UTC()
	tzStr, _ := args["timezone"].(string)
	if tzStr == "" {
		tzStr = "UTC"
	}

	loc, err := time.LoadLocation(tzStr)
	if err != nil {
		return &mcp.ToolResult{
			Content: []mcp.TextContent{{Type: "text", Text: fmt.Sprintf("Invalid timezone '%s': %v", tzStr, err)}},
			IsError: true,
		}, nil
	}

	localTime := now.In(loc)
	res := map[string]any{
		"utc":        now.Format(time.RFC3339),
		"timezone":   tzStr,
		"local_time": localTime.Format("2006-01-02 15:04:05 MST"),
		"unix":       now.Unix(),
	}

	data, _ := json.Marshal(res)
	return &mcp.ToolResult{
		Content: []mcp.TextContent{{Type: "text", Text: string(data)}},
		IsError: false,
	}, nil
}

func (r *Registry) callCalculate(args map[string]any) (*mcp.ToolResult, error) {
	expr, _ := args["expression"].(string)
	if strings.TrimSpace(expr) == "" {
		return &mcp.ToolResult{
			Content: []mcp.TextContent{{Type: "text", Text: "Missing 'expression' argument"}},
			IsError: true,
		}, nil
	}

	val, err := evaluateExpression(expr)
	if err != nil {
		return &mcp.ToolResult{
			Content: []mcp.TextContent{{Type: "text", Text: fmt.Sprintf("Evaluation error: %v", err)}},
			IsError: true,
		}, nil
	}

	res := map[string]any{
		"expression": expr,
		"result":     val,
	}
	data, _ := json.Marshal(res)
	return &mcp.ToolResult{
		Content: []mcp.TextContent{{Type: "text", Text: string(data)}},
		IsError: false,
	}, nil
}

// evaluateExpression evaluates basic arithmetic expressions safely using Shunting-Yard algorithm.
func evaluateExpression(expr string) (float64, error) {
	tokens, err := tokenize(expr)
	if err != nil {
		return 0, err
	}
	return parseTokens(tokens)
}

func tokenize(s string) ([]string, error) {
	var tokens []string
	var current strings.Builder

	for i := 0; i < len(s); i++ {
		ch := s[i]
		if unicode.IsSpace(rune(ch)) {
			continue
		}
		if unicode.IsDigit(rune(ch)) || ch == '.' {
			current.WriteByte(ch)
			for i+1 < len(s) && (unicode.IsDigit(rune(s[i+1])) || s[i+1] == '.') {
				i++
				current.WriteByte(s[i])
			}
			tokens = append(tokens, current.String())
			current.Reset()
		} else if strings.ContainsRune("+-*/()", rune(ch)) {
			tokens = append(tokens, string(ch))
		} else {
			return nil, fmt.Errorf("unexpected character '%c'", ch)
		}
	}
	return tokens, nil
}

func parseTokens(tokens []string) (float64, error) {
	var values []float64
	var ops []string

	applyOp := func() error {
		if len(ops) == 0 || len(values) < 2 {
			return fmt.Errorf("malformed expression")
		}
		op := ops[len(ops)-1]
		ops = ops[:len(ops)-1]
		b := values[len(values)-1]
		values = values[:len(values)-1]
		a := values[len(values)-1]
		values = values[:len(values)-1]

		var res float64
		switch op {
		case "+":
			res = a + b
		case "-":
			res = a - b
		case "*":
			res = a * b
		case "/":
			if b == 0 {
				return fmt.Errorf("division by zero")
			}
			res = a / b
		default:
			return fmt.Errorf("unknown operator %s", op)
		}
		values = append(values, res)
		return nil
	}

	precedence := func(op string) int {
		if op == "+" || op == "-" {
			return 1
		}
		if op == "*" || op == "/" {
			return 2
		}
		return 0
	}

	for _, token := range tokens {
		if val, err := strconv.ParseFloat(token, 64); err == nil {
			values = append(values, val)
		} else if token == "(" {
			ops = append(ops, token)
		} else if token == ")" {
			for len(ops) > 0 && ops[len(ops)-1] != "(" {
				if err := applyOp(); err != nil {
					return 0, err
				}
			}
			if len(ops) == 0 {
				return 0, fmt.Errorf("mismatched parentheses")
			}
			ops = ops[:len(ops)-1] // pop '('
		} else {
			// Operator
			for len(ops) > 0 && precedence(ops[len(ops)-1]) >= precedence(token) {
				if err := applyOp(); err != nil {
					return 0, err
				}
			}
			ops = append(ops, token)
		}
	}

	for len(ops) > 0 {
		if ops[len(ops)-1] == "(" {
			return 0, fmt.Errorf("mismatched parentheses")
		}
		if err := applyOp(); err != nil {
			return 0, err
		}
	}

	if len(values) != 1 {
		return 0, fmt.Errorf("invalid expression")
	}
	return values[0], nil
}
