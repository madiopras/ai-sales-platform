package order

import "context"

// Repository defines order data access. Checkout runs in a single DB transaction
// that persists the order + items, reserves stock, and closes the cart atomically.
type Repository interface {
	// CreateFromCheckout persists a new order with its items, reserves stock for
	// each variant, and marks the source cart checked_out. All within one tx.
	CreateFromCheckout(ctx context.Context, order Order, cartID string) (Order, error)

	GetByID(ctx context.Context, id string) (Order, error)
	List(ctx context.Context, filter ListFilter) ([]Order, error)

	// UpdateStatus transitions an order and, when moving to paid, commits the
	// reserved stock; when cancelling an unpaid order, releases the reservation.
	UpdateStatus(ctx context.Context, id, status string) (Order, error)
}
