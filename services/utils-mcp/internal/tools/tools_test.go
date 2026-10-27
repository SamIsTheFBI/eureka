package tools

import (
	"encoding/json"
	"testing"
)

func TestCalculate(t *testing.T) {
	r := NewRegistry()

	tests := []struct {
		expr     string
		expected float64
		hasError bool
	}{
		{"2 + 2", 4, false},
		{"10 - 3 * 2", 4, false},
		{"(10 - 3) * 2", 14, false},
		{"100 / 4 + 5", 30, false},
		{"10 / 0", 0, true},
		{"invalid", 0, true},
	}

	for _, tt := range tests {
		res, err := r.Call("calculate", map[string]any{"expression": tt.expr})
		if err != nil {
			t.Fatalf("unexpected call error: %v", err)
		}
		if tt.hasError {
			if !res.IsError {
				t.Errorf("expected error for expr %q, got success", tt.expr)
			}
		} else {
			if res.IsError {
				t.Errorf("unexpected error for expr %q: %s", tt.expr, res.Content[0].Text)
			}
			var parsed map[string]any
			if err := json.Unmarshal([]byte(res.Content[0].Text), &parsed); err != nil {
				t.Fatalf("failed to parse output: %v", err)
			}
			resultVal := parsed["result"].(float64)
			if resultVal != tt.expected {
				t.Errorf("for expr %q, expected %v, got %v", tt.expr, tt.expected, resultVal)
			}
		}
	}
}

func TestTimeNow(t *testing.T) {
	r := NewRegistry()
	res, err := r.Call("time_now", map[string]any{"timezone": "UTC"})
	if err != nil {
		t.Fatalf("unexpected call error: %v", err)
	}
	if res.IsError {
		t.Fatalf("expected success, got error: %s", res.Content[0].Text)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(res.Content[0].Text), &parsed); err != nil {
		t.Fatalf("failed to parse json: %v", err)
	}
	if parsed["timezone"] != "UTC" {
		t.Errorf("expected timezone UTC, got %v", parsed["timezone"])
	}
}
