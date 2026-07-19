package customer

import "time"

type Customer struct {
	ID        string    `json:"id"`
	Phone     string    `json:"phone"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Address struct {
	ID            string    `json:"id"`
	CustomerID    string    `json:"customer_id"`
	Label         string    `json:"label"`
	RecipientName string    `json:"recipient_name"`
	Phone         string    `json:"phone"`
	AddressLine   string    `json:"address_line"`
	City          string    `json:"city"`
	District      string    `json:"district"`
	PostalCode    string    `json:"postal_code"`
	Notes         string    `json:"notes"`
	IsDefault     bool      `json:"is_default"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

const (
	CartStatusOpen       = "open"
	CartStatusCheckedOut = "checked_out"
	CartStatusAbandoned  = "abandoned"
)

type Cart struct {
	ID         string     `json:"id"`
	CustomerID string     `json:"customer_id"`
	Status     string     `json:"status"`
	Items      []CartItem `json:"items,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type CartItem struct {
	ID         string    `json:"id"`
	CartID     string    `json:"cart_id"`
	VariantID  string    `json:"variant_id"`
	Qty        int       `json:"qty"`
	UnitPrice  float64   `json:"unit_price"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
