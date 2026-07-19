// Package payment implements Phase 8 (BR-016..BR-024, BR-042): create an invoice
// via Xendit after checkout, verify webhook callbacks, transition invoice + order
// state atomically, and release reserved stock when an invoice expires.
package payment

import (
	"errors"
	"time"
)

// Domain errors surface as typed sentinels so the HTTP layer can map them to
// stable error codes without leaking repository/vendor details.
var (
	ErrInvoiceNotFound       = errors.New("invoice not found")
	ErrInvoiceAlreadyExists  = errors.New("invoice already exists for order")
	ErrInvoiceNotPayable     = errors.New("invoice is not in a payable state")
	ErrOrderNotFound         = errors.New("order not found")
	ErrOrderNotAwaitingPay   = errors.New("order is not pending payment")
	ErrInvalidCallbackToken  = errors.New("invalid xendit callback token")
	ErrInvalidWebhookPayload = errors.New("invalid webhook payload")
	ErrXenditFailure         = errors.New("xendit request failed")
)

// Invoice status lifecycle (mirrors the check constraint in migration 000008).
// waiting_payment -> paid | expired | failed
const (
	StatusWaitingPayment = "waiting_payment"
	StatusPaid           = "paid"
	StatusExpired        = "expired"
	StatusFailed         = "failed"
)

// Xendit invoice statuses we react to on webhooks.
const (
	XenditStatusPending = "PENDING"
	XenditStatusPaid    = "PAID"
	XenditStatusSettled = "SETTLED"
	XenditStatusExpired = "EXPIRED"
)

// Invoice is a single payment attempt attached to one order. `order_id` is UNIQUE
// so re-invoicing a cancelled/expired order requires updating the existing row.
type Invoice struct {
	ID                  string     `json:"id"`
	OrderID             string     `json:"order_id"`
	InvoiceNo           string     `json:"invoice_no"`
	Amount              float64    `json:"amount"`
	Status              string     `json:"status"`
	PaymentChannel      string     `json:"payment_channel"`
	XenditInvoiceID     string     `json:"xendit_invoice_id"`
	XenditExternalID    string     `json:"xendit_external_id"`
	XenditPaymentURL    string     `json:"xendit_payment_url"`
	XenditPaymentMethod string     `json:"xendit_payment_method"`
	ExpiredAt           time.Time  `json:"expired_at"`
	PaidAt              *time.Time `json:"paid_at,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

// IsWaitingPayment reports whether the invoice can still be paid.
func (i Invoice) IsWaitingPayment() bool { return i.Status == StatusWaitingPayment }

// IsExpired reports whether the wall-clock expiry has passed. Actual state
// transition still happens through the repository so DB stays authoritative.
func (i Invoice) IsExpired(now time.Time) bool {
	return !i.ExpiredAt.IsZero() && now.After(i.ExpiredAt)
}

// CreateInvoiceInput drives Service.CreateInvoice. The address/customer info is
// picked up from the order so the AI/API caller only needs to pass an order id
// and an optional payer email.
type CreateInvoiceInput struct {
	OrderID    string
	PayerEmail string
}

// WebhookInput is the decoded body of Xendit's invoice callback. We keep only
// the fields Phase 8 acts on; the raw payload is preserved in payment_events.
type WebhookInput struct {
	EventID        string
	ID             string
	ExternalID     string
	Status         string
	PaymentMethod  string
	PaymentChannel string
	Amount         float64
	PaidAt         *time.Time
	RawPayload     []byte
	CallbackToken  string
}

// PaymentEvent is the audit record persisted per webhook call (BR-042).
type PaymentEvent struct {
	ID              string
	InvoiceID       string
	XenditEventID   string
	XenditInvoiceID string
	ExternalID      string
	EventStatus     string
	Payload         []byte
	SignatureValid  bool
	Processed       bool
	ProcessingError string
	CreatedAt       time.Time
}
