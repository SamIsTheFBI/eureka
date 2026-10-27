package tools

import (
	"encoding/json"
	"fmt"
	"strconv"

	"eureka/services/woo-mcp/internal/mcp"
	"eureka/services/woo-mcp/internal/woo"
)

// Registry holds the five WooCommerce MCP tools and delegates calls to the Woo client.
type Registry struct {
	client *woo.Client
	tools  map[string]mcp.Tool
}

// NewRegistry registers the 5 required tools according to PRD section 6.
func NewRegistry(client *woo.Client) *Registry {
	r := &Registry{
		client: client,
		tools:  make(map[string]mcp.Tool),
	}

	r.tools["list_products"] = mcp.Tool{
		Name:        "list_products",
		Description: "Lists store products with optional search query and status filtering.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"search": map[string]any{
					"type":        "string",
					"description": "Optional search term to filter products by name or SKU.",
				},
				"status": map[string]any{
					"type":        "string",
					"description": "Filter by status (e.g. 'publish', 'draft', 'private', 'pending').",
				},
				"page": map[string]any{
					"type":        "integer",
					"description": "Page number (defaults to 1).",
				},
				"per_page": map[string]any{
					"type":        "integer",
					"description": "Number of items per page (default 20, max 100).",
				},
			},
		},
	}

	r.tools["create_product"] = mcp.Tool{
		Name:        "create_product",
		Description: "Creates a new product in the store with name, price, and description.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "Name of the product.",
				},
				"regular_price": map[string]any{
					"type":        "string",
					"description": "Price as a decimal string (e.g., '19.99').",
				},
				"description": map[string]any{
					"type":        "string",
					"description": "Product description.",
				},
				"status": map[string]any{
					"type":        "string",
					"description": "Publish status ('publish', 'draft', 'pending'). Defaults to 'publish'.",
				},
			},
			"required": []string{"name", "regular_price"},
		},
	}

	r.tools["list_orders"] = mcp.Tool{
		Name:        "list_orders",
		Description: "Lists orders with optional filtering by status and ISO 8601 / RFC 3339 date range.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"status": map[string]any{
					"type":        "string",
					"description": "Filter by order status ('pending', 'processing', 'on-hold', 'completed', 'cancelled', 'refunded', 'failed').",
				},
				"after": map[string]any{
					"type":        "string",
					"description": "Limit results to orders created after a specified ISO 8601 date/time (e.g. '2026-10-12T00:00:00Z').",
				},
				"before": map[string]any{
					"type":        "string",
					"description": "Limit results to orders created before a specified ISO 8601 date/time.",
				},
				"page": map[string]any{
					"type":        "integer",
					"description": "Page number.",
				},
				"per_page": map[string]any{
					"type":        "integer",
					"description": "Number of orders per page (max 100).",
				},
			},
		},
	}

	r.tools["get_order"] = mcp.Tool{
		Name:        "get_order",
		Description: "Fetches complete order details by ID, including line items, billing, and shipping.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id": map[string]any{
					"type":        "integer",
					"description": "The unique WooCommerce order ID.",
				},
			},
			"required": []string{"id"},
		},
	}

	r.tools["update_order_status"] = mcp.Tool{
		Name:        "update_order_status",
		Description: "Updates the status of an existing order.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id": map[string]any{
					"type":        "integer",
					"description": "The unique order ID.",
				},
				"status": map[string]any{
					"type":        "string",
					"description": "New status: 'pending', 'processing', 'on-hold', 'completed', 'cancelled', 'refunded'.",
					"enum":        []string{"pending", "processing", "on-hold", "completed", "cancelled", "refunded"},
				},
			},
			"required": []string{"id", "status"},
		},
	}

	return r
}

// ListTools returns the 5 tools.
func (r *Registry) ListTools() []mcp.Tool {
	list := make([]mcp.Tool, 0, len(r.tools))
	for _, t := range r.tools {
		list = append(list, t)
	}
	return list
}

// Call routes the execution to the appropriate WooCommerce client method.
func (r *Registry) Call(name string, args map[string]any) (*mcp.ToolResult, error) {
	switch name {
	case "list_products":
		search, _ := args["search"].(string)
		status, _ := args["status"].(string)
		page := getInt(args["page"], 1)
		perPage := getInt(args["per_page"], 20)
		products, err := r.client.ListProducts(search, status, page, perPage)
		if err != nil {
			return &mcp.ToolResult{
				Content: []mcp.TextContent{{Type: "text", Text: fmt.Sprintf("Error listing products: %v", err)}},
				IsError: true,
			}, nil
		}
		data, _ := json.Marshal(products)
		return &mcp.ToolResult{
			Content: []mcp.TextContent{{Type: "text", Text: string(data)}},
			IsError: false,
		}, nil

	case "create_product":
		name, _ := args["name"].(string)
		regPrice, _ := args["regular_price"].(string)
		desc, _ := args["description"].(string)
		status, _ := args["status"].(string)
		if name == "" || regPrice == "" {
			return &mcp.ToolResult{
				Content: []mcp.TextContent{{Type: "text", Text: "Missing required fields: 'name' and 'regular_price'"}},
				IsError: true,
			}, nil
		}
		prod, err := r.client.CreateProduct(name, regPrice, desc, status)
		if err != nil {
			return &mcp.ToolResult{
				Content: []mcp.TextContent{{Type: "text", Text: fmt.Sprintf("Error creating product: %v", err)}},
				IsError: true,
			}, nil
		}
		data, _ := json.Marshal(prod)
		return &mcp.ToolResult{
			Content: []mcp.TextContent{{Type: "text", Text: string(data)}},
			IsError: false,
		}, nil

	case "list_orders":
		status, _ := args["status"].(string)
		after, _ := args["after"].(string)
		before, _ := args["before"].(string)
		page := getInt(args["page"], 1)
		perPage := getInt(args["per_page"], 20)
		orders, err := r.client.ListOrders(status, after, before, page, perPage)
		if err != nil {
			return &mcp.ToolResult{
				Content: []mcp.TextContent{{Type: "text", Text: fmt.Sprintf("Error listing orders: %v", err)}},
				IsError: true,
			}, nil
		}
		data, _ := json.Marshal(orders)
		return &mcp.ToolResult{
			Content: []mcp.TextContent{{Type: "text", Text: string(data)}},
			IsError: false,
		}, nil

	case "get_order":
		id := getInt(args["id"], 0)
		if id <= 0 {
			return &mcp.ToolResult{
				Content: []mcp.TextContent{{Type: "text", Text: "Valid 'id' is required"}},
				IsError: true,
			}, nil
		}
		order, err := r.client.GetOrder(id)
		if err != nil {
			return &mcp.ToolResult{
				Content: []mcp.TextContent{{Type: "text", Text: fmt.Sprintf("Error getting order %d: %v", id, err)}},
				IsError: true,
			}, nil
		}
		data, _ := json.Marshal(order)
		return &mcp.ToolResult{
			Content: []mcp.TextContent{{Type: "text", Text: string(data)}},
			IsError: false,
		}, nil

	case "update_order_status":
		id := getInt(args["id"], 0)
		status, _ := args["status"].(string)
		if id <= 0 || status == "" {
			return &mcp.ToolResult{
				Content: []mcp.TextContent{{Type: "text", Text: "Valid 'id' and 'status' are required"}},
				IsError: true,
			}, nil
		}
		order, err := r.client.UpdateOrderStatus(id, status)
		if err != nil {
			return &mcp.ToolResult{
				Content: []mcp.TextContent{{Type: "text", Text: fmt.Sprintf("Error updating order %d: %v", id, err)}},
				IsError: true,
			}, nil
		}
		data, _ := json.Marshal(order)
		return &mcp.ToolResult{
			Content: []mcp.TextContent{{Type: "text", Text: string(data)}},
			IsError: false,
		}, nil

	default:
		return &mcp.ToolResult{
			Content: []mcp.TextContent{{Type: "text", Text: fmt.Sprintf("Tool '%s' not found", name)}},
			IsError: true,
		}, nil
	}
}

func getInt(val any, defaultVal int) int {
	if val == nil {
		return defaultVal
	}
	switch v := val.(type) {
	case int:
		return v
	case float64:
		return int(v)
	case string:
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultVal
}
