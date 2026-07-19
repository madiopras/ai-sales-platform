package shipping

import (
	"context"
	"time"
)

// OrderSnapshot is the minimal view of an order needed to create a shipment.
// It comes from the repository (single query) to avoid a circular dep on the
// order service.
type OrderSnapshot struct {
	ID      string
	OrderNo string
	Status  string

	// Address snapshot taken at checkout.
	RecipientName string
	Phone         string
	AddressLine   string
	City          string
	District      string
	PostalCode    string
	Notes         string

	// Courier selected at checkout.
	CourierCode    string
	CourierService string
	ShippingFee    float64

	Items []OrderItemSnapshot
}

// OrderItemSnapshot is one line of the order used to describe the parcel to Biteship.
type OrderItemSnapshot struct {
	ProductName string
	VariantName string
	Qty         int
	UnitPrice   float64
}

// Repository is the persistence boundary for the shipping domain.
//
// Contracts:
//   - GetOrderForShipment: read-only order + items fetch (used to validate
//     status and build the Biteship parcel payload).
//   - CreateShipment: insert a fresh shipment row for an order. Returns
//     ErrShipmentAlreadyExists when order_id already has one.
//   - AttachBiteshipReferences: update biteship_* columns after the API call.
//   - MarkShipped / MarkDelivered: state transitions that also advance the
//     linked order's status inside a single transaction.
//   - RecordEvent: persist raw webhook payload for audit + idempotency; if the
//     event_key already exists, returns `duplicate=true`.
type Repository interface {
	GetOrderForShipment(ctx context.Context, orderID string) (OrderSnapshot, error)

	CreateShipment(ctx context.Context, shipment Shipment) (Shipment, error)
	AttachBiteshipReferences(ctx context.Context, shipmentID string, refs BiteshipReferences) (Shipment, error)

	GetByID(ctx context.Context, id string) (Shipment, error)
	GetByOrderID(ctx context.Context, orderID string) (Shipment, error)
	GetByBiteshipOrderID(ctx context.Context, biteshipOrderID string) (Shipment, error)
	GetByTrackingNo(ctx context.Context, trackingNo string) (Shipment, error)

	// UpdateStatus stores the latest Biteship status without an order transition.
	UpdateStatus(ctx context.Context, shipmentID, status string) (Shipment, error)
	// MarkShipped advances the order to `shipped` (BR-030..BR-031) in one tx.
	MarkShipped(ctx context.Context, shipmentID, status string) (Shipment, error)
	// MarkDelivered advances the order to `delivered` (BR-033) in one tx.
	MarkDelivered(ctx context.Context, shipmentID string) (Shipment, error)

	// ListCompletable returns shipments delivered before `cutoff` whose order is
	// still `delivered` (used by the worker for BR-034 auto-complete).
	ListCompletable(ctx context.Context, cutoff time.Time, limit int) ([]Shipment, error)
	// CompleteOrder advances the linked order from `delivered` to `completed` (BR-034).
	CompleteOrder(ctx context.Context, shipmentID string) error

	RecordEvent(ctx context.Context, event ShipmentEvent) (recorded ShipmentEvent, duplicate bool, err error)
	MarkEventProcessed(ctx context.Context, eventID string, processingError string) error
}
