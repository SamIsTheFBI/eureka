package woo

// ProductSummary represents a trimmed product representation for LLM context efficiency.
type ProductSummary struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	SKU         string `json:"sku,omitempty"`
	Price       string `json:"price"`
	Status      string `json:"status"`
	StockStatus string `json:"stock_status"`
	Permalink   string `json:"permalink"`
}

// OrderItemSummary represents a simplified line item.
type OrderItemSummary struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Quantity int    `json:"quantity"`
	Total    string `json:"total"`
}

// OrderSummary represents a trimmed order summary for listings.
type OrderSummary struct {
	ID       int                `json:"id"`
	Status   string             `json:"status"`
	Total    string             `json:"total"`
	Currency string             `json:"currency"`
	Created  string             `json:"created"`
	Customer string             `json:"customer"`
	Items    []OrderItemSummary `json:"items"`
}

// RawWooProduct models the subset of WooCommerce REST product response we need.
type RawWooProduct struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	SKU         string `json:"sku"`
	Price       string `json:"price"`
	RegularPrice string `json:"regular_price"`
	Status      string `json:"status"`
	StockStatus string `json:"stock_status"`
	Permalink   string `json:"permalink"`
	Description string `json:"description"`
}

// RawWooOrder models the WooCommerce REST order response.
type RawWooOrder struct {
	ID          int    `json:"id"`
	Status      string `json:"status"`
	Total       string `json:"total"`
	Currency    string `json:"currency"`
	DateCreated string `json:"date_created"`
	Billing     struct {
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Email     string `json:"email"`
		Phone     string `json:"phone"`
		City      string `json:"city"`
		Country   string `json:"country"`
	} `json:"billing"`
	Shipping struct {
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Address1  string `json:"address_1"`
		City      string `json:"city"`
		Country   string `json:"country"`
	} `json:"shipping"`
	PaymentMethodTitle string `json:"payment_method_title"`
	LineItems          []struct {
		ID       int    `json:"id"`
		Name     string `json:"name"`
		Quantity int    `json:"quantity"`
		Total    string `json:"total"`
	} `json:"line_items"`
	CustomerNote string `json:"customer_note"`
}
