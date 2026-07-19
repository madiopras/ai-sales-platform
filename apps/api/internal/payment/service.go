package payment

import (
	"context"
	"crypto/subtle"
	"errors"
	"strings"
	"time"

	"github.com/prasdios/ai-sales-platform/apps/api/internal/events"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/payment/xendit"
)

// XenditClient is the vendor surface the service depends on. The concrete
// implementation lives in `xendit.Client`; tests substitute a fake.
type XenditClient interface {
	CreateInvoice(ctx context.Context, req xendit.CreateInvoiceRequest) (xendit.Invoice, error)
}

// EventPublisher publishes domain events (order.paid) for apps/ai to consume
// (BR-041). Optional: a nil publisher disables publishing.
type EventPublisher interface {
	Publish(ctx context.Context, event events.Event) error
}

// Config is the runtime configuration the service needs. Loaded once from
// `config.XenditConfig` at wire time; keep pure primitives so tests can build it.
type Config struct {
	CallbackToken      string
	InvoiceDuration    time.Duration
	SuccessRedirectURL string
	FailureRedirectURL string
}

// Service orchestrates invoice creation, Xendit calls, and webhook handling.
type Service struct {
	repository Repository
	client     XenditClient
	config     Config
	publisher  EventPublisher
	now        func() time.Time
}

// NewService returns a Service that uses time.Now() for expiry math. The
// injected repository owns transactions across invoices/orders/stock.
func NewService(repository Repository, client XenditClient, cfg Config) *Service {
	if cfg.InvoiceDuration <= 0 {
		cfg.InvoiceDuration = 24 * time.Hour
	}
	return &Service{repository: repository, client: client, config: cfg, now: time.Now}
}

// SetEventPublisher wires a domain-event publisher so a paid invoice emits
// `order.paid` for apps/ai (BR-041). Optional; when unset no events are emitted.
func (s *Service) SetEventPublisher(publisher EventPublisher) { s.publisher = publisher }

// CreateInvoice builds an invoice for the given order and requests a hosted
// payment page from Xendit. The invoice row is created before the Xendit call
// so we always have a persistent audit trail even when Xendit is unreachable;
// the xendit_* columns are backfilled after the API call succeeds.
func (s *Service) CreateInvoice(ctx context.Context, input CreateInvoiceInput) (Invoice, error) {
	input.OrderID = strings.TrimSpace(input.OrderID)
	if input.OrderID == "" {
		return Invoice{}, errors.New("order_id is required")
	}
	order, err := s.repository.GetOrderForInvoice(ctx, input.OrderID)
	if err != nil {
		return Invoice{}, err
	}
	if order.Status != "pending_payment" {
		return Invoice{}, ErrOrderNotAwaitingPay
	}

	existing, err := s.repository.GetByOrderID(ctx, order.ID)
	switch {
	case err == nil && existing.IsWaitingPayment() && !existing.IsExpired(s.now()):
		// Idempotent: same order can be asked for an invoice repeatedly
		// (e.g. AI retry) without spawning duplicates on Xendit.
		return existing, nil
	case err != nil && !errors.Is(err, ErrInvoiceNotFound):
		return Invoice{}, err
	}

	now := s.now()
	invoice := Invoice{
		OrderID:   order.ID,
		InvoiceNo: generateInvoiceNo(),
		Amount:    order.Total,
		Status:    StatusWaitingPayment,
		ExpiredAt: now.Add(s.config.InvoiceDuration),
	}
	created, err := s.repository.CreateInvoice(ctx, invoice)
	if err != nil {
		return Invoice{}, err
	}

	xenditInvoice, err := s.client.CreateInvoice(ctx, xendit.CreateInvoiceRequest{
		ExternalID:         order.OrderNo,
		Amount:             order.Total,
		Description:        "Order " + order.OrderNo,
		PayerEmail:         input.PayerEmail,
		InvoiceDuration:    int64(s.config.InvoiceDuration.Seconds()),
		SuccessRedirectURL: s.config.SuccessRedirectURL,
		FailureRedirectURL: s.config.FailureRedirectURL,
		Currency:           "IDR",
	})
	if err != nil {
		// Best-effort mark failed so we don't leave a zombie waiting_payment row
		// pointing to nothing at Xendit; downstream can re-invoice.
		_, _ = s.repository.MarkFailed(ctx, created.ID, err.Error())
		return Invoice{}, ErrXenditFailure
	}

	expiredAt := parseExpiry(xenditInvoice.ExpiryDate, invoice.ExpiredAt)
	refreshed, err := s.repository.AttachXenditReferences(ctx, created.ID, XenditReferences{
		XenditInvoiceID:  xenditInvoice.ID,
		XenditExternalID: xenditInvoice.ExternalID,
		XenditPaymentURL: xenditInvoice.InvoiceURL,
		ExpiredAt:        expiredAt,
	})
	if err != nil {
		return Invoice{}, err
	}
	return refreshed, nil
}

// GetInvoice fetches an invoice by internal id.
func (s *Service) GetInvoice(ctx context.Context, id string) (Invoice, error) {
	return s.repository.GetByID(ctx, strings.TrimSpace(id))
}

// GetInvoiceByOrderID exposes the invoice tied to an order (AI needs this).
func (s *Service) GetInvoiceByOrderID(ctx context.Context, orderID string) (Invoice, error) {
	return s.repository.GetByOrderID(ctx, strings.TrimSpace(orderID))
}

// HandleWebhook is the entry point for `POST /webhooks/xendit`. It validates
// the callback token, records the raw payload for audit (BR-042), and applies
// the state transition based on Xendit's `status` field. The transition is
// idempotent: replaying the same event is a no-op.
func (s *Service) HandleWebhook(ctx context.Context, input WebhookInput) (Invoice, error) {
	if !s.verifyCallbackToken(input.CallbackToken) {
		// Even invalid callbacks are recorded so we can investigate abuse.
		_, _, _ = s.repository.RecordEvent(ctx, PaymentEvent{
			XenditEventID:   input.EventID,
			XenditInvoiceID: input.ID,
			ExternalID:      input.ExternalID,
			EventStatus:     input.Status,
			Payload:         input.RawPayload,
			SignatureValid:  false,
		})
		return Invoice{}, ErrInvalidCallbackToken
	}

	invoice, err := s.lookupInvoice(ctx, input)
	if err != nil {
		return Invoice{}, err
	}

	event, duplicate, err := s.repository.RecordEvent(ctx, PaymentEvent{
		InvoiceID:       invoice.ID,
		XenditEventID:   input.EventID,
		XenditInvoiceID: input.ID,
		ExternalID:      input.ExternalID,
		EventStatus:     input.Status,
		Payload:         input.RawPayload,
		SignatureValid:  true,
	})
	if err != nil {
		return Invoice{}, err
	}
	if duplicate {
		// Xendit retries deliver the same event id; short-circuit without
		// touching the order/stock again.
		return invoice, nil
	}

	updated, processErr := s.applyWebhook(ctx, invoice, input)
	processingError := ""
	if processErr != nil {
		processingError = processErr.Error()
	}
	// Always record whether processing succeeded so we can alert on failures.
	if err := s.repository.MarkEventProcessed(ctx, event.ID, processingError); err != nil {
		return updated, err
	}
	if processErr != nil {
		return Invoice{}, processErr
	}
	return updated, nil
}

// ExpireInvoices scans invoices whose expired_at is in the past and transitions
// them to `expired`, releasing the reserved stock (BR-036). Intended to be
// called from a periodic worker.
func (s *Service) ExpireInvoices(ctx context.Context, limit int) (int, error) {
	if limit <= 0 {
		limit = 50
	}
	invoices, err := s.repository.ListExpiring(ctx, s.now(), limit)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, invoice := range invoices {
		if _, err := s.repository.MarkExpired(ctx, invoice.ID); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

// verifyCallbackToken uses constant-time compare so we don't leak byte-by-byte
// timing info about the configured secret.
func (s *Service) verifyCallbackToken(received string) bool {
	if s.config.CallbackToken == "" {
		// Fail-closed: a missing config is a misconfiguration, not "allow all".
		return false
	}
	if subtle.ConstantTimeCompare([]byte(received), []byte(s.config.CallbackToken)) == 1 {
		return true
	}
	return false
}

func (s *Service) lookupInvoice(ctx context.Context, input WebhookInput) (Invoice, error) {
	if strings.TrimSpace(input.ID) != "" {
		invoice, err := s.repository.GetByXenditInvoiceID(ctx, input.ID)
		if err == nil {
			return invoice, nil
		}
		if !errors.Is(err, ErrInvoiceNotFound) {
			return Invoice{}, err
		}
	}
	if strings.TrimSpace(input.ExternalID) != "" {
		return s.repository.GetByExternalID(ctx, input.ExternalID)
	}
	return Invoice{}, ErrInvalidWebhookPayload
}

func (s *Service) applyWebhook(ctx context.Context, invoice Invoice, input WebhookInput) (Invoice, error) {
	status := strings.ToUpper(strings.TrimSpace(input.Status))
	switch status {
	case XenditStatusPaid, XenditStatusSettled:
		if invoice.Status == StatusPaid {
			return invoice, nil
		}
		paidAt := s.now()
		if input.PaidAt != nil {
			paidAt = *input.PaidAt
		}
		paid, err := s.repository.MarkPaid(ctx, invoice.ID, paidAt, input.PaymentMethod, input.PaymentChannel)
		if err != nil {
			return Invoice{}, err
		}
		// Publish order.paid for apps/ai to notify the customer (BR-041). Best-effort:
		// a publish failure must not roll back the paid transition.
		s.publishOrderPaid(ctx, paid)
		return paid, nil

	case XenditStatusExpired:
		if invoice.Status == StatusExpired {
			return invoice, nil
		}
		return s.repository.MarkExpired(ctx, invoice.ID)
	case XenditStatusPending, "":
		// Nothing to do — invoice is already waiting_payment.
		return invoice, nil
	default:
		return invoice, nil
	}
}

// publishOrderPaid emits the order.paid domain event. Best-effort: publishing is
// never allowed to fail the surrounding transition, so errors are swallowed here
// (the log publisher surfaces them at the transport layer).
func (s *Service) publishOrderPaid(ctx context.Context, invoice Invoice) {
	if s.publisher == nil {
		return
	}
	_ = s.publisher.Publish(ctx, events.NewOrderEvent(events.OrderPaid, invoice.OrderID, map[string]any{
		"invoice_id":      invoice.ID,
		"invoice_no":      invoice.InvoiceNo,
		"amount":          invoice.Amount,
		"payment_channel": invoice.PaymentChannel,
	}))
}

func parseExpiry(raw string, fallback time.Time) time.Time {

	if raw == "" {
		return fallback
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t
	}
	if t, err := time.Parse("2006-01-02T15:04:05.000Z", raw); err == nil {
		return t
	}
	return fallback
}
