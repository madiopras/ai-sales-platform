package payment

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresRepository is the pgx-backed implementation of Repository.
//
// It leans on two transactional invariants to avoid inconsistent state:
//   - MarkPaid commits stock reservations + flips the order to `paid` in one tx
//     (BR-035).
//   - MarkExpired releases the reservation + flips the order to `cancelled` in
//     one tx (BR-036).
type PostgresRepository struct{ pool *pgxpool.Pool }

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

const invoiceColumns = `id, order_id, invoice_no, amount, status, payment_channel,
	xendit_invoice_id, xendit_external_id, xendit_payment_url, xendit_payment_method,
	expired_at, paid_at, created_at, updated_at`

const selectInvoiceQuery = `SELECT ` + invoiceColumns + ` FROM invoices`

func (r *PostgresRepository) GetOrderForInvoice(ctx context.Context, orderID string) (OrderSnapshot, error) {
	var snap OrderSnapshot
	err := r.pool.QueryRow(ctx, `SELECT id, order_no, customer_id, status, total FROM orders WHERE id = $1`, orderID).
		Scan(&snap.ID, &snap.OrderNo, &snap.CustomerID, &snap.Status, &snap.Total)
	if errors.Is(err, pgx.ErrNoRows) {
		return OrderSnapshot{}, ErrOrderNotFound
	}
	return snap, err
}

func (r *PostgresRepository) CreateInvoice(ctx context.Context, invoice Invoice) (Invoice, error) {
	err := r.pool.QueryRow(ctx, `INSERT INTO invoices (
		order_id, invoice_no, amount, status, expired_at)
	VALUES ($1, $2, $3, $4, $5)
	RETURNING `+invoiceColumns,
		invoice.OrderID, invoice.InvoiceNo, invoice.Amount, invoice.Status, invoice.ExpiredAt).
		Scan(&invoice.ID, &invoice.OrderID, &invoice.InvoiceNo, &invoice.Amount, &invoice.Status, &invoice.PaymentChannel,
			&invoice.XenditInvoiceID, &invoice.XenditExternalID, &invoice.XenditPaymentURL, &invoice.XenditPaymentMethod,
			&invoice.ExpiredAt, &invoice.PaidAt, &invoice.CreatedAt, &invoice.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Invoice{}, ErrInvoiceAlreadyExists
		}
		return Invoice{}, err
	}
	return invoice, nil
}

func (r *PostgresRepository) AttachXenditReferences(ctx context.Context, invoiceID string, refs XenditReferences) (Invoice, error) {
	row := r.pool.QueryRow(ctx, `UPDATE invoices SET
		xendit_invoice_id = $2,
		xendit_external_id = $3,
		xendit_payment_url = $4,
		expired_at = COALESCE(NULLIF($5, '0001-01-01 00:00:00'::timestamptz), expired_at),
		updated_at = NOW()
	WHERE id = $1
	RETURNING `+invoiceColumns,
		invoiceID, refs.XenditInvoiceID, refs.XenditExternalID, refs.XenditPaymentURL, refs.ExpiredAt)
	invoice, err := scanInvoice(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Invoice{}, ErrInvoiceNotFound
	}
	return invoice, err
}

func (r *PostgresRepository) GetByID(ctx context.Context, id string) (Invoice, error) {
	return r.queryOne(ctx, selectInvoiceQuery+` WHERE id = $1`, id)
}

func (r *PostgresRepository) GetByOrderID(ctx context.Context, orderID string) (Invoice, error) {
	return r.queryOne(ctx, selectInvoiceQuery+` WHERE order_id = $1`, orderID)
}

func (r *PostgresRepository) GetByXenditInvoiceID(ctx context.Context, xenditInvoiceID string) (Invoice, error) {
	return r.queryOne(ctx, selectInvoiceQuery+` WHERE xendit_invoice_id = $1`, xenditInvoiceID)
}

func (r *PostgresRepository) GetByExternalID(ctx context.Context, externalID string) (Invoice, error) {
	return r.queryOne(ctx, selectInvoiceQuery+` WHERE xendit_external_id = $1`, externalID)
}

func (r *PostgresRepository) queryOne(ctx context.Context, query string, args ...any) (Invoice, error) {
	invoice, err := scanInvoice(r.pool.QueryRow(ctx, query, args...))
	if errors.Is(err, pgx.ErrNoRows) {
		return Invoice{}, ErrInvoiceNotFound
	}
	return invoice, err
}

// MarkPaid commits the stock reservation and flips both invoice + order to
// `paid`. All three writes happen in one tx so a partial failure rolls back.
func (r *PostgresRepository) MarkPaid(ctx context.Context, invoiceID string, paidAt time.Time, paymentMethod, paymentChannel string) (Invoice, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Invoice{}, err
	}
	defer tx.Rollback(ctx)

	row := tx.QueryRow(ctx, `UPDATE invoices SET
		status = 'paid',
		paid_at = $2,
		xendit_payment_method = COALESCE(NULLIF($3, ''), xendit_payment_method),
		payment_channel = COALESCE(NULLIF($4, ''), payment_channel),
		updated_at = NOW()
	WHERE id = $1
	RETURNING `+invoiceColumns, invoiceID, paidAt, paymentMethod, paymentChannel)
	invoice, err := scanInvoice(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Invoice{}, ErrInvoiceNotFound
	}
	if err != nil {
		return Invoice{}, err
	}

	if err := commitOrderStock(ctx, tx, invoice.OrderID); err != nil {
		return Invoice{}, err
	}

	if _, err := tx.Exec(ctx, `UPDATE orders SET status = 'paid', updated_at = NOW()
		WHERE id = $1 AND status = 'pending_payment'`, invoice.OrderID); err != nil {
		return Invoice{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Invoice{}, err
	}
	return invoice, nil
}

// MarkExpired releases the reservation and moves the order to `cancelled` when
// it was still pending payment. If the order already advanced (e.g. admin
// cancelled it manually), we only update the invoice status.
func (r *PostgresRepository) MarkExpired(ctx context.Context, invoiceID string) (Invoice, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Invoice{}, err
	}
	defer tx.Rollback(ctx)

	row := tx.QueryRow(ctx, `UPDATE invoices SET status = 'expired', updated_at = NOW()
		WHERE id = $1 AND status = 'waiting_payment'
		RETURNING `+invoiceColumns, invoiceID)
	invoice, err := scanInvoice(row)
	if errors.Is(err, pgx.ErrNoRows) {
		// Either the invoice doesn't exist, or it already left waiting_payment
		// (paid/expired/failed). Return the current row so callers see truth.
		return r.GetByID(ctx, invoiceID)
	}
	if err != nil {
		return Invoice{}, err
	}

	if err := releaseOrderStock(ctx, tx, invoice.OrderID); err != nil {
		return Invoice{}, err
	}

	if _, err := tx.Exec(ctx, `UPDATE orders SET status = 'cancelled', updated_at = NOW()
		WHERE id = $1 AND status = 'pending_payment'`, invoice.OrderID); err != nil {
		return Invoice{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Invoice{}, err
	}
	return invoice, nil
}

func (r *PostgresRepository) MarkFailed(ctx context.Context, invoiceID, reason string) (Invoice, error) {
	row := r.pool.QueryRow(ctx, `UPDATE invoices SET status = 'failed', updated_at = NOW()
		WHERE id = $1
		RETURNING `+invoiceColumns, invoiceID)
	invoice, err := scanInvoice(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Invoice{}, ErrInvoiceNotFound
	}
	if err != nil {
		return Invoice{}, err
	}
	// Reason is best-effort logged as a payment_event so we keep the invoice
	// table lean; ignore errors here so failing the invoice is never blocked
	// by the audit write.
	_, _ = r.pool.Exec(ctx, `INSERT INTO payment_events (
		invoice_id, event_status, processed, processing_error, payload)
		VALUES ($1, 'failed', TRUE, $2, '{}'::jsonb)`, invoice.ID, reason)
	return invoice, nil
}

func (r *PostgresRepository) ListExpiring(ctx context.Context, now time.Time, limit int) ([]Invoice, error) {
	rows, err := r.pool.Query(ctx, selectInvoiceQuery+` WHERE status = 'waiting_payment' AND expired_at <= $1
		ORDER BY expired_at ASC LIMIT $2`, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	invoices := make([]Invoice, 0)
	for rows.Next() {
		invoice, err := scanInvoice(rows)
		if err != nil {
			return nil, err
		}
		invoices = append(invoices, invoice)
	}
	return invoices, rows.Err()
}

func (r *PostgresRepository) RecordEvent(ctx context.Context, event PaymentEvent) (PaymentEvent, bool, error) {
	// Dedup uses the unique index on (xendit_invoice_id, xendit_event_id) so a
	// second delivery of the same webhook collapses to the original row.
	payload := event.Payload
	if len(payload) == 0 {
		payload = []byte("{}")
	}
	row := r.pool.QueryRow(ctx, `INSERT INTO payment_events (
		invoice_id, xendit_event_id, xendit_invoice_id, external_id,
		event_status, payload, signature_valid)
	VALUES ($1, $2, $3, $4, $5, $6, $7)
	ON CONFLICT (xendit_invoice_id, xendit_event_id) WHERE xendit_event_id <> ''
	DO NOTHING
	RETURNING id, created_at`,
		nullString(event.InvoiceID), event.XenditEventID, event.XenditInvoiceID,
		event.ExternalID, event.EventStatus, payload, event.SignatureValid)

	var createdID string
	var createdAt time.Time
	if err := row.Scan(&createdID, &createdAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Duplicate: pull the existing row so callers can reference it.
			existing, err := r.findEvent(ctx, event.XenditInvoiceID, event.XenditEventID)
			if err != nil {
				return PaymentEvent{}, true, err
			}
			return existing, true, nil
		}
		return PaymentEvent{}, false, err
	}
	event.ID = createdID
	event.CreatedAt = createdAt
	return event, false, nil
}

func (r *PostgresRepository) MarkEventProcessed(ctx context.Context, eventID, processingError string) error {
	_, err := r.pool.Exec(ctx, `UPDATE payment_events SET processed = TRUE, processing_error = $2
		WHERE id = $1`, eventID, processingError)
	return err
}

func (r *PostgresRepository) findEvent(ctx context.Context, xenditInvoiceID, eventID string) (PaymentEvent, error) {
	var event PaymentEvent
	err := r.pool.QueryRow(ctx, `SELECT id, COALESCE(invoice_id::text, ''), xendit_event_id, xendit_invoice_id,
		external_id, event_status, payload, signature_valid, processed, processing_error, created_at
		FROM payment_events WHERE xendit_invoice_id = $1 AND xendit_event_id = $2`,
		xenditInvoiceID, eventID).
		Scan(&event.ID, &event.InvoiceID, &event.XenditEventID, &event.XenditInvoiceID,
			&event.ExternalID, &event.EventStatus, &event.Payload, &event.SignatureValid,
			&event.Processed, &event.ProcessingError, &event.CreatedAt)
	return event, err
}

// commitOrderStock decrements on_hand + reserved for each item on the order.
// It refuses to underflow so a bad reservation surfaces as an error instead of
// silently corrupting stock counters.
func commitOrderStock(ctx context.Context, tx pgx.Tx, orderID string) error {
	rows, err := tx.Query(ctx, `SELECT variant_id, qty FROM order_items WHERE order_id = $1`, orderID)
	if err != nil {
		return err
	}
	items, err := collectStockUpdates(rows)
	if err != nil {
		return err
	}
	for _, item := range items {
		tag, err := tx.Exec(ctx, `UPDATE product_variants
			SET stock_on_hand = stock_on_hand - $2,
			    stock_reserved = stock_reserved - $2,
			    updated_at = NOW()
			WHERE id = $1 AND stock_on_hand >= $2 AND stock_reserved >= $2`, item.VariantID, item.Qty)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return errors.New("payment: stock commit underflow")
		}
	}
	return nil
}

// releaseOrderStock rolls back the reservation without touching on_hand so an
// expired invoice makes the units bookable again (BR-036).
func releaseOrderStock(ctx context.Context, tx pgx.Tx, orderID string) error {
	rows, err := tx.Query(ctx, `SELECT variant_id, qty FROM order_items WHERE order_id = $1`, orderID)
	if err != nil {
		return err
	}
	items, err := collectStockUpdates(rows)
	if err != nil {
		return err
	}
	for _, item := range items {
		if _, err := tx.Exec(ctx, `UPDATE product_variants
			SET stock_reserved = GREATEST(stock_reserved - $2, 0), updated_at = NOW()
			WHERE id = $1`, item.VariantID, item.Qty); err != nil {
			return err
		}
	}
	return nil
}

type stockUpdate struct {
	VariantID string
	Qty       int
}

func collectStockUpdates(rows pgx.Rows) ([]stockUpdate, error) {
	defer rows.Close()
	items := make([]stockUpdate, 0)
	for rows.Next() {
		var item stockUpdate
		if err := rows.Scan(&item.VariantID, &item.Qty); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func scanInvoice(row pgx.Row) (Invoice, error) {
	var invoice Invoice
	err := row.Scan(&invoice.ID, &invoice.OrderID, &invoice.InvoiceNo, &invoice.Amount, &invoice.Status,
		&invoice.PaymentChannel, &invoice.XenditInvoiceID, &invoice.XenditExternalID,
		&invoice.XenditPaymentURL, &invoice.XenditPaymentMethod,
		&invoice.ExpiredAt, &invoice.PaidAt, &invoice.CreatedAt, &invoice.UpdatedAt)
	return invoice, err
}

func nullString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
