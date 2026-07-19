// Package shipping implements Phase 9 (BR-010..BR-013, BR-028..BR-034, BR-043):
// fetch courier rates from Biteship before checkout, create a shipment when an
// order is ready to ship, persist tracking references, and process Biteship
// tracking webhooks to advance the order to shipped/delivered/completed.
package shipping

import (
	"errors"
	"time"
)

// Domain errors surface as typed sentinels so the HTTP layer can map them to
// stable error codes without leaking repository/vendor details.
var (
	ErrShipmentNotFound      = errors.New("shipment not found")
	ErrShipmentAlreadyExists = errors.New("shipment already exists for order")
	ErrOrderNotFound         = errors.New("order not found")
	ErrOrderNotShippable     = errors.New("order is not in a shippable state")
	ErrInvalidWebhookPayload = errors.New("invalid webhook payload")
	ErrInvalidSignature      = errors.New("invalid biteship webhook signature")
	ErrBiteshipFailure       = errors.New("biteship request failed")
	ErrRatesUnavailable      = errors.New("no shipping rates available")
	ErrAddressIncomplete     = errors.New("shipping address is incomplete")
)

// Shipment status lifecycle. We store Biteship's raw status but track a small
// set of internal states for the order transition logic.
const (
	StatusPending   = "pending"   // shipment row created, not yet booked at Biteship
	StatusBooked    = "booked"    // Biteship order created, waybill assigned
	StatusShipped   = "shipped"   // courier picked up / in transit
	StatusDelivered = "delivered" // delivered to recipient
	StatusCancelled = "cancelled"
)

// Biteship tracking statuses we react to on webhooks. Biteship sends many
// granular statuses; we normalise them into shipped/delivered buckets.
// See https://biteship.com/id/docs/api/trackings/retrieve.
const (
	BiteshipStatusConfirmed       = "confirmed"
	BiteshipStatusAllocated       = "allocated"
	BiteshipStatusPickingUp       = "picking_up"
	BiteshipStatusPicked          = "picked"
	BiteshipStatusDropping        = "dropping_off"
	BiteshipStatusOnHold          = "on_hold"
	BiteshipStatusDelivered       = "delivered"
	BiteshipStatusReturned        = "returned"
	BiteshipStatusRejected        = "rejected"
	BiteshipStatusCancelled       = "cancelled"
	BiteshipStatusCourierNotFound = "courier_not_found"
)

// Shipment is the persistent booking tied to one order. `order_id` is UNIQUE so
// re-booking a cancelled shipment updates the existing row instead of duplicating.
type Shipment struct {
	ID      string `json:"id"`
	OrderID string `json:"order_id"`

	CourierCode    string `json:"courier_code"`
	CourierService string `json:"courier_service"`
	CourierName    string `json:"courier_name"`

	ShippingFee float64 `json:"shipping_fee"`
	Status      string  `json:"status"`

	BiteshipOrderID string `json:"biteship_order_id"`
	TrackingNo      string `json:"tracking_no"`
	WaybillID       string `json:"waybill_id"`
	LabelURL        string `json:"label_url"`

	DeliveredAt *time.Time `json:"delivered_at,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Rate is a single courier option returned to the AI before checkout (BR-011).
type Rate struct {
	CourierCode    string  `json:"courier_code"`
	CourierService string  `json:"courier_service"`
	CourierName    string  `json:"courier_name"`
	ServiceName    string  `json:"service_name"`
	Duration       string  `json:"duration"`
	Price          float64 `json:"price"`
}

// RateInput drives Service.GetRates. Weight is grams; when zero the service
// falls back to a configured default so the AI can still get a quote.
type RateInput struct {
	OriginPostalCode      int
	DestinationPostalCode int
	Couriers              string // comma-separated courier codes; empty = config default
	Items                 []RateItem
}

// RateItem is one line for the rate request.
type RateItem struct {
	Name     string
	Value    float64
	Quantity int
	Weight   int // grams
}

// CreateShipmentInput drives Service.CreateShipment. The address/courier info is
// picked up from the order so the caller only needs the order id.
type CreateShipmentInput struct {
	OrderID string
}

// BiteshipReferences groups the fields returned by the Biteship POST /v1/orders call.
type BiteshipReferences struct {
	BiteshipOrderID string
	TrackingNo      string
	WaybillID       string
	LabelURL        string
	Status          string
}

// WebhookInput is the decoded body of a Biteship tracking callback. We keep only
// the fields Phase 9 acts on; the raw payload is preserved in shipment_events.
type WebhookInput struct {
	EventKey        string
	Event           string
	BiteshipOrderID string
	TrackingNo      string
	Status          string
	RawPayload      []byte
	Signature       string
}

// ShipmentEvent is the audit record persisted per webhook call (BR-043).
type ShipmentEvent struct {
	ID              string
	ShipmentID      string
	EventKey        string
	BiteshipOrderID string
	TrackingNo      string
	Event           string
	EventStatus     string
	Payload         []byte
	SignatureValid  bool
	Processed       bool
	ProcessingError string
	CreatedAt       time.Time
}
