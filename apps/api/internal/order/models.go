package order

import (
	"errors"
	"time"

	"github.com/prasdios/ai-sales-platform/apps/api/internal/catalog"
)

// Domain errors
var (
	ErrOrderNotFound    = errors.New("order not found")
	ErrEmptyCart        = errors.New("cart is empty")
	ErrAddressRequired  = errors.New("shipping address is required")
	ErrCourierRequired  = errors.New("courier selection is required")
	ErrInvalidStatus    = errors.New("invalid order status")
	ErrStatusTransition = errors.New("invalid order status transition")
	ErrOrderNoConflict  = errors.New("order number conflict")
	// ErrInsufficientStock re-exports the catalog sentinel so the whole checkout
	// path (service + repository) reasons about a single stock error.
	ErrInsufficientStock = catalog.ErrInsufficientStock
)

// Order status lifecycle (BR-014..BR-015, BR-026)
const (
	StatusPendingPayment = "pending_payment"
	StatusPaid           = "paid"
	StatusProcessing     = "processing"
	StatusShipped        = "shipped"
	StatusDelivered      = "delivered"
	StatusCompleted      = "completed"
	StatusCancelled      = "cancelled"
)

// Order is a confirmed checkout with an address snapshot and reserved stock.
type Order struct {
	ID         string `json:"id"`
	OrderNo    string `json:"order_no"`
	CustomerID string `json:"customer_id"`
	Status     string `json:"status"`

	// Address snapshot
	RecipientName string `json:"recipient_name"`
	Phone         string `json:"phone"`
	AddressLine   string `json:"address_line"`
	City          string `json:"city"`
	District      string `json:"district"`
	PostalCode    string `json:"postal_code"`
	Notes         string `json:"notes"`

	// Courier (stubbed until shipping phase)
	CourierCode    string `json:"courier_code"`
	CourierService string `json:"courier_service"`
	TrackingNo     string `json:"tracking_no"`

	// Promo (BR-037..BR-038): the voucher applied at checkout + the discount
	// amount snapshotted so order history is immutable.
	VoucherCode string  `json:"voucher_code,omitempty"`
	Discount    float64 `json:"discount"`

	Subtotal    float64 `json:"subtotal"`
	ShippingFee float64 `json:"shipping_fee"`
	Total       float64 `json:"total"`

	Items     []OrderItem `json:"items,omitempty"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}

// OrderItem carries price + name snapshots so catalog changes never mutate history.
type OrderItem struct {
	ID          string    `json:"id"`
	OrderID     string    `json:"order_id"`
	VariantID   string    `json:"variant_id"`
	SKU         string    `json:"sku"`
	ProductName string    `json:"product_name"`
	VariantName string    `json:"variant_name"`
	Qty         int       `json:"qty"`
	UnitPrice   float64   `json:"unit_price"`
	Subtotal    float64   `json:"subtotal"`
	CreatedAt   time.Time `json:"created_at"`
}

// CheckoutPreview is the non-persisted summary returned before confirmation.
type CheckoutPreview struct {
	CustomerID  string      `json:"customer_id"`
	Address     AddressSnap `json:"address"`
	Courier     Courier     `json:"courier"`
	Items       []OrderItem `json:"items"`
	VoucherCode string      `json:"voucher_code,omitempty"`
	Discount    float64     `json:"discount"`
	Subtotal    float64     `json:"subtotal"`
	ShippingFee float64     `json:"shipping_fee"`
	Total       float64     `json:"total"`
}

// AddressSnap is the resolved shipping address used at checkout.
type AddressSnap struct {
	RecipientName string `json:"recipient_name"`
	Phone         string `json:"phone"`
	AddressLine   string `json:"address_line"`
	City          string `json:"city"`
	District      string `json:"district"`
	PostalCode    string `json:"postal_code"`
	Notes         string `json:"notes"`
}

func (a AddressSnap) isComplete() bool {
	return a.RecipientName != "" && a.Phone != "" && a.AddressLine != "" && a.City != "" && a.District != ""
}

// Courier holds the selected courier + fee (stubbed until Biteship phase).
type Courier struct {
	Code    string  `json:"code"`
	Service string  `json:"service"`
	Fee     float64 `json:"fee"`
}

// ListFilter for admin order listing.
type ListFilter struct {
	CustomerID string
	Status     string
	Limit      int
	Offset     int
}

// isValidStatus reports whether s is a known order status.
func isValidStatus(s string) bool {
	switch s {
	case StatusPendingPayment, StatusPaid, StatusProcessing, StatusShipped,
		StatusDelivered, StatusCompleted, StatusCancelled:
		return true
	default:
		return false
	}
}

// allowedTransitions defines the admin-driven fulfillment lifecycle (BR-026).
var allowedTransitions = map[string][]string{
	StatusPendingPayment: {StatusPaid, StatusCancelled},
	StatusPaid:           {StatusProcessing, StatusCancelled},
	StatusProcessing:     {StatusShipped, StatusCancelled},
	StatusShipped:        {StatusDelivered},
	StatusDelivered:      {StatusCompleted},
	StatusCompleted:      {},
	StatusCancelled:      {},
}

func canTransition(from, to string) bool {
	for _, next := range allowedTransitions[from] {
		if next == to {
			return true
		}
	}
	return false
}
