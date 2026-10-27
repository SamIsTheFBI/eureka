package aggregate

import (
	"strings"
	"testing"
)

func TestNamespaceTool(t *testing.T) {
	tests := []struct {
		slug     string
		tool     string
		expected string
	}{
		{"woo", "list_orders", "woo__list_orders"},
		{"utils", "time_now", "utils__time_now"},
		{"docs", "search", "docs__search"},
	}

	for _, tt := range tests {
		res := NamespaceTool(tt.slug, tt.tool)
		if res != tt.expected {
			t.Errorf("NamespaceTool(%q, %q) = %q; expected %q", tt.slug, tt.tool, res, tt.expected)
		}
		slug, tool, ok := DeconstructTool(res)
		if !ok || slug != tt.slug || tool != tt.tool {
			t.Errorf("DeconstructTool(%q) = (%q, %q, %v); expected (%q, %q, true)", res, slug, tool, ok, tt.slug, tt.tool)
		}
	}
}

func TestNamespaceToolTruncation(t *testing.T) {
	slug := "verylongserverslug"
	tool := strings.Repeat("a", 80)

	namespaced := NamespaceTool(slug, tool)
	if len(namespaced) > 64 {
		t.Errorf("expected max length 64, got %d (%s)", len(namespaced), namespaced)
	}

	slugPart, _, ok := DeconstructTool(namespaced)
	if !ok || slugPart != slug {
		t.Errorf("failed to deconstruct truncated tool %s", namespaced)
	}
}
