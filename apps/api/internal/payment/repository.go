package payment

import (
	"context"
	"time"
)

// OrderSnapshot is the minimal view of an order needed to create an invoice.
// It comes from the repository (single query) to avoid a circular dep on the
// order service.
type OrderSnapshot struct {
	ID         string
	OrderNo    string
	CustomerID string
	Status     string
	Total      float64
}

// Repository is the persistence boundary for the payment domain.
//
// Contracts:
//   - GetOrderForInvoice: read-only order fetch (used by service to validate
//     status before hitting Xendit).
//   - CreateInvoice: insert a fresh invoice row for an order. Returns
//     ErrInvoiceAlreadyExists when order_id already has one.
//   - AttachXenditReferences: update xendit_* columns after the API call succeeds.
//   - MarkPaid / MarkExpired / MarkFailed: state transitions that also flip the
//     linked order's status inside a single transaction so callers cannot
//     observe an invoice = paid with an order still pending_payment.
//   - RecordEvent: persist raw webhook payload for audit + idempotency; if the
//     (xendit_invoice_id, xendit_event_id) pair already exists, returns
//     `duplicate=true` and the caller short-circuits.
type Repository interface {
	GetOrderForInvoice(ctx context.Context, orderID string) (OrderSnapshot, error)

	CreateInvoice(ctx context.Context, invoice Invoice) (Invoice, error)
	AttachXenditReferences(ctx context.Context, invoiceID string, refs XenditReferences) (Invoice, error)

	GetByID(ctx context.Context, id string) (Invoice, error)
	GetByOrderID(ctx context.Context, orderID string) (Invoice, error)
	GetByXenditInvoiceID(ctx context.Context, xenditInvoiceID string) (Invoice, error)
	GetByExternalID(ctx context.Context, externalID string) (Invoice, error)

	MarkPaid(ctx context.Context, invoiceID string, paidAt time.Time, paymentMethod, paymentChannel string) (Invoice, error)
	MarkExpired(ctx context.Context, invoiceID string) (Invoice, error)
	MarkFailed(ctx context.Context, invoiceID string, reason string) (Invoice, error)

	ListExpiring(ctx context.Context, now time.Time, limit int) ([]Invoice, error)

	RecordEvent(ctx context.Context, event PaymentEvent) (recorded PaymentEvent, duplicate bool, err error)
	MarkEventProcessed(ctx context.Context, eventID string, processingError string) error
}

// XenditReferences groups the fields returned by the Xendit /v2/invoices call.
type XenditReferences struct {
	XenditInvoiceID  string
	XenditExternalID string
	XenditPaymentURL string
	ExpiredAt        time.Time
}
