package aggregate

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"unicode"

	"eureka/services/gateway/internal/auth"
	"eureka/services/gateway/internal/mcp"
	"eureka/services/gateway/internal/upstream"
)

// Aggregator builds user-visible tool lists from active upstream snapshots.
type Aggregator struct {
	manager *upstream.Manager
}

// NewAggregator creates a new Aggregator.
func NewAggregator(manager *upstream.Manager) *Aggregator {
	return &Aggregator{manager: manager}
}

// AggregateTools compiles namespaced tools for the caller's Principal.
func (a *Aggregator) AggregateTools(p *auth.Principal) []mcp.Tool {
	servers := a.manager.ListServers()
	aggregated := make([]mcp.Tool, 0)

	for _, srv := range servers {
		status, _, _, _ := srv.GetStatus()
		// Only list tools if server is enabled and Connected (PRD section 8)
		if !srv.Enabled || status != upstream.StatusConnected {
			continue
		}

		rawTools := srv.GetTools()
		for _, t := range rawTools {
			namespacedName := NamespaceTool(srv.Slug, t.Name)
			sanitizedDesc := sanitizeDescription(srv.Name, t.Description)

			aggregated = append(aggregated, mcp.Tool{
				Name:        namespacedName,
				Description: sanitizedDesc,
				InputSchema: t.InputSchema,
			})
		}
	}

	return aggregated
}

// NamespaceTool forms `<slug>__<tool>`. Enforces 64-char cap with hash if exceeded.
func NamespaceTool(slug, tool string) string {
	combined := fmt.Sprintf("%s__%s", slug, tool)
	if len(combined) <= 64 {
		return combined
	}

	h := sha256.Sum256([]byte(combined))
	hashSuffix := hex.EncodeToString(h[:])[:6]

	// Truncate tool name to fit: 64 - len(slug) - 2 (__) - 7 (_hash)
	maxToolLen := 64 - len(slug) - 2 - 7
	if maxToolLen < 4 {
		maxToolLen = 4
	}
	trimmedTool := tool
	if len(trimmedTool) > maxToolLen {
		trimmedTool = trimmedTool[:maxToolLen]
	}

	return fmt.Sprintf("%s__%s_%s", slug, trimmedTool, hashSuffix)
}

// DeconstructTool splits namespaced name into (slug, tool).
func DeconstructTool(exposedName string) (slug, tool string, ok bool) {
	idx := strings.Index(exposedName, "__")
	if idx <= 0 || idx+2 >= len(exposedName) {
		return "", "", false
	}
	return exposedName[:idx], exposedName[idx+2:], true
}

func sanitizeDescription(serverName, desc string) string {
	var clean strings.Builder
	for _, r := range desc {
		if !unicode.IsControl(r) || r == '\n' || r == '\t' {
			clean.WriteRune(r)
		}
	}

	res := clean.String()
	if len(res) > 1000 {
		res = res[:997] + "..."
	}

	if serverName != "" {
		res = fmt.Sprintf("[%s] %s", serverName, res)
	}

	return res
}
