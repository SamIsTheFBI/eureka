package woo

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Client executes authenticated REST calls against WooCommerce /wp-json/wc/v3.
type Client struct {
	baseURL        string
	consumerKey    string
	consumerSecret string
	hostHeader     string
	httpClient     *http.Client
}

// NewClient initializes a new WooCommerce REST client.
func NewClient(baseURL, consumerKey, consumerSecret, hostHeader string) *Client {
	baseURL = strings.TrimSuffix(baseURL, "/")
	return &Client{
		baseURL:        baseURL,
		consumerKey:    consumerKey,
		consumerSecret: consumerSecret,
		hostHeader:     hostHeader,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *Client) newRequest(method, path string, query url.Values, body any) (*http.Request, error) {
	fullURL := fmt.Sprintf("%s/wp-json/wc/v3%s", c.baseURL, path)
	if len(query) > 0 {
		fullURL += "?" + query.Encode()
	}

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to encode request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, fullURL, bodyReader)
	if err != nil {
		return nil, err
	}

	// CRITICAL WooCommerce Requirements:
	// 1. Basic auth with consumer key and consumer secret
	auth := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", c.consumerKey, c.consumerSecret)))
	req.Header.Set("Authorization", "Basic "+auth)

	// 2. Pass X-Forwarded-Proto: https so WooCommerce enables key-based Basic auth without OAuth 1.0a
	req.Header.Set("X-Forwarded-Proto", "https")

	// 3. Set Host header to match the canonical store hostname to avoid WordPress redirecting
	if c.hostHeader != "" {
		req.Host = c.hostHeader
		req.Header.Set("Host", c.hostHeader)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return req, nil
}

func (c *Client) do(req *http.Request) ([]byte, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("store network error: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read store response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("store returned status %d: %s", resp.StatusCode, string(respBytes))
	}

	return respBytes, nil
}

// ListProducts retrieves a list of products and trims to essentials.
func (c *Client) ListProducts(search, status string, page, perPage int) ([]ProductSummary, error) {
	if perPage <= 0 || perPage > 100 {
		perPage = 20
	}
	if page <= 0 {
		page = 1
	}

	q := url.Values{}
	q.Set("page", strconv.Itoa(page))
	q.Set("per_page", strconv.Itoa(perPage))
	if search != "" {
		q.Set("search", search)
	}
	if status != "" {
		q.Set("status", status)
	}

	req, err := c.newRequest(http.MethodGet, "/products", q, nil)
	if err != nil {
		return nil, err
	}

	data, err := c.do(req)
	if err != nil {
		return nil, err
	}

	var raw []RawWooProduct
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to unmarshal products: %w", err)
	}

	summaries := make([]ProductSummary, len(raw))
	for i, p := range raw {
		summaries[i] = ProductSummary{
			ID:          p.ID,
			Name:        p.Name,
			SKU:         p.SKU,
			Price:       p.Price,
			Status:      p.Status,
			StockStatus: p.StockStatus,
			Permalink:   p.Permalink,
		}
	}

	return summaries, nil
}

// CreateProduct creates a new product in WooCommerce.
func (c *Client) CreateProduct(name, regularPrice, description, status string) (*ProductSummary, error) {
	if status == "" {
		status = "publish"
	}

	payload := map[string]any{
		"name":          name,
		"regular_price": regularPrice,
		"description":   description,
		"status":        status,
	}

	req, err := c.newRequest(http.MethodPost, "/products", nil, payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(req)
	if err != nil {
		return nil, err
	}

	var p RawWooProduct
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("failed to unmarshal created product: %w", err)
	}

	return &ProductSummary{
		ID:          p.ID,
		Name:        p.Name,
		SKU:         p.SKU,
		Price:       p.Price,
		Status:      p.Status,
		StockStatus: p.StockStatus,
		Permalink:   p.Permalink,
	}, nil
}

// ListOrders lists orders with optional filtering by status and date range.
func (c *Client) ListOrders(status, after, before string, page, perPage int) ([]OrderSummary, error) {
	if perPage <= 0 || perPage > 100 {
		perPage = 20
	}
	if page <= 0 {
		page = 1
	}

	q := url.Values{}
	q.Set("page", strconv.Itoa(page))
	q.Set("per_page", strconv.Itoa(perPage))
	if status != "" {
		q.Set("status", status)
	}
	if after != "" {
		q.Set("after", after)
	}
	if before != "" {
		q.Set("before", before)
	}

	req, err := c.newRequest(http.MethodGet, "/orders", q, nil)
	if err != nil {
		return nil, err
	}

	data, err := c.do(req)
	if err != nil {
		return nil, err
	}

	var raw []RawWooOrder
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to unmarshal orders: %w", err)
	}

	summaries := make([]OrderSummary, len(raw))
	for i, o := range raw {
		items := make([]OrderItemSummary, len(o.LineItems))
		for j, it := range o.LineItems {
			items[j] = OrderItemSummary{
				ID:       it.ID,
				Name:     it.Name,
				Quantity: it.Quantity,
				Total:    it.Total,
			}
		}

		custName := strings.TrimSpace(o.Billing.FirstName + " " + o.Billing.LastName)
		if custName == "" {
			custName = o.Billing.Email
		}

		summaries[i] = OrderSummary{
			ID:       o.ID,
			Status:   o.Status,
			Total:    o.Total,
			Currency: o.Currency,
			Created:  o.DateCreated,
			Customer: custName,
			Items:    items,
		}
	}

	return summaries, nil
}

// GetOrder fetches a single order in full detail.
func (c *Client) GetOrder(id int) (*RawWooOrder, error) {
	req, err := c.newRequest(http.MethodGet, fmt.Sprintf("/orders/%d", id), nil, nil)
	if err != nil {
		return nil, err
	}

	data, err := c.do(req)
	if err != nil {
		return nil, err
	}

	var o RawWooOrder
	if err := json.Unmarshal(data, &o); err != nil {
		return nil, fmt.Errorf("failed to unmarshal order: %w", err)
	}

	return &o, nil
}

// UpdateOrderStatus updates an order's status.
func (c *Client) UpdateOrderStatus(id int, status string) (*OrderSummary, error) {
	payload := map[string]any{
		"status": status,
	}

	req, err := c.newRequest(http.MethodPut, fmt.Sprintf("/orders/%d", id), nil, payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(req)
	if err != nil {
		return nil, err
	}

	var o RawWooOrder
	if err := json.Unmarshal(data, &o); err != nil {
		return nil, fmt.Errorf("failed to unmarshal updated order: %w", err)
	}

	custName := strings.TrimSpace(o.Billing.FirstName + " " + o.Billing.LastName)
	return &OrderSummary{
		ID:       o.ID,
		Status:   o.Status,
		Total:    o.Total,
		Currency: o.Currency,
		Created:  o.DateCreated,
		Customer: custName,
	}, nil
}
