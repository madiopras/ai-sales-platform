package payment

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/prasdios/ai-sales-platform/apps/api/internal/payment/xendit"
)

const testCallbackToken = "test-callback-token"

func newTestService(repo *memoryRepo, client XenditClient) *Service {
	return NewService(repo, client, Config{
		CallbackToken:   testCallbackToken,
		InvoiceDuration: 24 * time.Hour,
	})
}

func TestCreateInvoiceRejectsNonPendingOrder(t *testing.T) {
	repo := &memoryRepo{order: OrderSnapshot{ID: "ord-1", OrderNo: "ORD-1", Status: "paid", Total: 100000}}
	service := newTestService(repo, &fakeXendit{})
	_, err := service.CreateInvoice(context.Background(), CreateInvoiceInput{OrderID: "ord-1"})
	if !errors.Is(err, ErrOrderNotAwaitingPay) {
		t.Fatalf("expected ErrOrderNotAwaitingPay, got %v", err)
	}
}

func TestCreateInvoiceCallsXenditAndAttachesReferences(t *testing.T) {
	repo := &memoryRepo{order: OrderSnapshot{ID: "ord-1", OrderNo: "ORD-1", Status: "pending_payment", Total: 100000}}
	client := &fakeXendit{invoice: xendit.Invoice{ID: "xnd-1", ExternalID: "ORD-1", InvoiceURL: "https://pay.example/xnd-1"}}
	service := newTestService(repo, client)

	invoice, err := service.CreateInvoice(context.Background(), CreateInvoiceInput{OrderID: "ord-1", PayerEmail: "a@b.com"})
	if err != nil {
		t.Fatal(err)
	}
	if invoice.XenditInvoiceID != "xnd-1" {
		t.Fatalf("expected xendit id attached, got %q", invoice.XenditInvoiceID)
	}
	if invoice.XenditPaymentURL != "https://pay.example/xnd-1" {
		t.Fatalf("expected payment url attached, got %q", invoice.XenditPaymentURL)
	}
	if client.lastRequest.Amount != 100000 {
		t.Fatalf("expected amount 100000 sent to xendit, got %v", client.lastRequest.Amount)
	}
	if client.lastRequest.ExternalID != "ORD-1" {
		t.Fatalf("expected external_id ORD-1, got %q", client.lastRequest.ExternalID)
	}
}

func TestCreateInvoiceIsIdempotent(t *testing.T) {
	repo := &memoryRepo{order: OrderSnapshot{ID: "ord-1", OrderNo: "ORD-1", Status: "pending_payment", Total: 100000}}
	client := &fakeXendit{invoice: xendit.Invoice{ID: "xnd-1", ExternalID: "ORD-1", InvoiceURL: "url"}}
	service := newTestService(repo, client)

	if _, err := service.CreateInvoice(context.Background(), CreateInvoiceInput{OrderID: "ord-1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.CreateInvoice(context.Background(), CreateInvoiceInput{OrderID: "ord-1"}); err != nil {
		t.Fatal(err)
	}
	if client.calls != 1 {
		t.Fatalf("expected xendit to be called once, got %d", client.calls)
	}
}

func TestCreateInvoiceMarksFailedOnXenditError(t *testing.T) {
	repo := &memoryRepo{order: OrderSnapshot{ID: "ord-1", OrderNo: "ORD-1", Status: "pending_payment", Total: 100000}}
	client := &fakeXendit{err: errors.New("boom")}
	service := newTestService(repo, client)

	_, err := service.CreateInvoice(context.Background(), CreateInvoiceInput{OrderID: "ord-1"})
	if !errors.Is(err, ErrXenditFailure) {
		t.Fatalf("expected ErrXenditFailure, got %v", err)
	}
	if repo.invoice.Status != StatusFailed {
		t.Fatalf("expected invoice marked failed, got %q", repo.invoice.Status)
	}
}

func TestHandleWebhookRejectsInvalidToken(t *testing.T) {
	repo := &memoryRepo{}
	service := newTestService(repo, &fakeXendit{})
	_, err := service.HandleWebhook(context.Background(), WebhookInput{ID: "xnd-1", Status: "PAID", CallbackToken: "wrong"})
	if !errors.Is(err, ErrInvalidCallbackToken) {
		t.Fatalf("expected ErrInvalidCallbackToken, got %v", err)
	}
	if !repo.eventRecorded {
		t.Fatal("expected invalid callback to still be recorded for audit")
	}
}

func TestHandleWebhookMarksPaid(t *testing.T) {
	repo := &memoryRepo{
		invoice: Invoice{ID: "inv-1", OrderID: "ord-1", XenditInvoiceID: "xnd-1", Status: StatusWaitingPayment},
	}
	service := newTestService(repo, &fakeXendit{})
	invoice, err := service.HandleWebhook(context.Background(), WebhookInput{
		EventID: "evt-1", ID: "xnd-1", Status: "PAID", CallbackToken: testCallbackToken,
	})
	if err != nil {
		t.Fatal(err)
	}
	if invoice.Status != StatusPaid {
		t.Fatalf("expected paid, got %q", invoice.Status)
	}
}

func TestHandleWebhookIsIdempotentOnDuplicateEvent(t *testing.T) {
	repo := &memoryRepo{
		invoice:   Invoice{ID: "inv-1", OrderID: "ord-1", XenditInvoiceID: "xnd-1", Status: StatusWaitingPayment},
		duplicate: true,
	}
	service := newTestService(repo, &fakeXendit{})
	_, err := service.HandleWebhook(context.Background(), WebhookInput{
		EventID: "evt-1", ID: "xnd-1", Status: "PAID", CallbackToken: testCallbackToken,
	})
	if err != nil {
		t.Fatal(err)
	}
	if repo.markPaidCalls != 0 {
		t.Fatalf("expected MarkPaid to be skipped for duplicate, got %d calls", repo.markPaidCalls)
	}
}

func TestExpireInvoicesReleasesStock(t *testing.T) {
	repo := &memoryRepo{
		expiring: []Invoice{
			{ID: "inv-1", OrderID: "ord-1", Status: StatusWaitingPayment},
			{ID: "inv-2", OrderID: "ord-2", Status: StatusWaitingPayment},
		},
	}
	service := newTestService(repo, &fakeXendit{})
	count, err := service.ExpireInvoices(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("expected 2 expired, got %d", count)
	}
	if repo.markExpiredCalls != 2 {
		t.Fatalf("expected 2 MarkExpired calls, got %d", repo.markExpiredCalls)
	}
}

// --- test doubles ---

type fakeXendit struct {
	invoice     xendit.Invoice
	err         error
	calls       int
	lastRequest xendit.CreateInvoiceRequest
}

func (f *fakeXendit) CreateInvoice(_ context.Context, req xendit.CreateInvoiceRequest) (xendit.Invoice, error) {
	f.calls++
	f.lastRequest = req
	if f.err != nil {
		return xendit.Invoice{}, f.err
	}
	return f.invoice, nil
}

type memoryRepo struct {
	order            OrderSnapshot
	invoice          Invoice
	expiring         []Invoice
	duplicate        bool
	eventRecorded    bool
	markPaidCalls    int
	markExpiredCalls int
}

func (r *memoryRepo) GetOrderForInvoice(_ context.Context, orderID string) (OrderSnapshot, error) {
	if r.order.ID == orderID {
		return r.order, nil
	}
	return OrderSnapshot{}, ErrOrderNotFound
}

func (r *memoryRepo) CreateInvoice(_ context.Context, invoice Invoice) (Invoice, error) {
	invoice.ID = "inv-new"
	r.invoice = invoice
	return invoice, nil
}

func (r *memoryRepo) AttachXenditReferences(_ context.Context, invoiceID string, refs XenditReferences) (Invoice, error) {
	r.invoice.XenditInvoiceID = refs.XenditInvoiceID
	r.invoice.XenditExternalID = refs.XenditExternalID
	r.invoice.XenditPaymentURL = refs.XenditPaymentURL
	return r.invoice, nil
}

func (r *memoryRepo) GetByID(_ context.Context, id string) (Invoice, error) {
	if r.invoice.ID == id {
		return r.invoice, nil
	}
	return Invoice{}, ErrInvoiceNotFound
}

func (r *memoryRepo) GetByOrderID(_ context.Context, orderID string) (Invoice, error) {
	if r.invoice.OrderID == orderID && r.invoice.ID != "" {
		return r.invoice, nil
	}
	return Invoice{}, ErrInvoiceNotFound
}

func (r *memoryRepo) GetByXenditInvoiceID(_ context.Context, xenditInvoiceID string) (Invoice, error) {
	if r.invoice.XenditInvoiceID == xenditInvoiceID && r.invoice.ID != "" {
		return r.invoice, nil
	}
	return Invoice{}, ErrInvoiceNotFound
}

func (r *memoryRepo) GetByExternalID(_ context.Context, externalID string) (Invoice, error) {
	if r.invoice.XenditExternalID == externalID && r.invoice.ID != "" {
		return r.invoice, nil
	}
	return Invoice{}, ErrInvoiceNotFound
}

func (r *memoryRepo) MarkPaid(_ context.Context, invoiceID string, _ time.Time, _, _ string) (Invoice, error) {
	r.markPaidCalls++
	r.invoice.Status = StatusPaid
	return r.invoice, nil
}

func (r *memoryRepo) MarkExpired(_ context.Context, invoiceID string) (Invoice, error) {
	r.markExpiredCalls++
	r.invoice.Status = StatusExpired
	return Invoice{ID: invoiceID, Status: StatusExpired}, nil
}

func (r *memoryRepo) MarkFailed(_ context.Context, invoiceID, reason string) (Invoice, error) {
	r.invoice.Status = StatusFailed
	return r.invoice, nil
}

func (r *memoryRepo) ListExpiring(_ context.Context, _ time.Time, _ int) ([]Invoice, error) {
	return r.expiring, nil
}

func (r *memoryRepo) RecordEvent(_ context.Context, event PaymentEvent) (PaymentEvent, bool, error) {
	r.eventRecorded = true
	event.ID = "evt-row-1"
	return event, r.duplicate, nil
}

func (r *memoryRepo) MarkEventProcessed(_ context.Context, _ string, _ string) error {
	return nil
}
