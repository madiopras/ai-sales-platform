package shipping

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
// State transitions that also touch the order run in a single transaction so a
// consumer never sees a shipment marked shipped/delivered while its order lags
// behind (BR-030..BR-034).
type PostgresRepository struct{ pool *pgxpool.Pool }

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

const shipmentColumns = `id, order_id, courier_code, courier_service, courier_name,
	shipping_fee, status, biteship_order_id, tracking_no, waybill_id, label_url,
	delivered_at, created_at, updated_at`

const selectShipmentQuery = `SELECT ` + shipmentColumns + ` FROM shipments`

func (r *PostgresRepository) GetOrderForShipment(ctx context.Context, orderID string) (OrderSnapshot, error) {
	var snap OrderSnapshot
	err := r.pool.QueryRow(ctx, `SELECT id, order_no, status,
		recipient_name, phone, address_line, city, district, postal_code, notes,
		courier_code, courier_service, shipping_fee
		FROM orders WHERE id = $1`, orderID).
		Scan(&snap.ID, &snap.OrderNo, &snap.Status,
			&snap.RecipientName, &snap.Phone, &snap.AddressLine, &snap.City, &snap.District, &snap.PostalCode, &snap.Notes,
			&snap.CourierCode, &snap.CourierService, &snap.ShippingFee)
	if errors.Is(err, pgx.ErrNoRows) {
		return OrderSnapshot{}, ErrOrderNotFound
	}
	if err != nil {
		return OrderSnapshot{}, err
	}

	rows, err := r.pool.Query(ctx, `SELECT product_name, variant_name, qty, unit_price
		FROM order_items WHERE order_id = $1 ORDER BY created_at ASC`, orderID)
	if err != nil {
		return OrderSnapshot{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var item OrderItemSnapshot
		if err := rows.Scan(&item.ProductName, &item.VariantName, &item.Qty, &item.UnitPrice); err != nil {
			return OrderSnapshot{}, err
		}
		snap.Items = append(snap.Items, item)
	}
	return snap, rows.Err()
}

func (r *PostgresRepository) CreateShipment(ctx context.Context, shipment Shipment) (Shipment, error) {
	err := r.pool.QueryRow(ctx, `INSERT INTO shipments (
		order_id, courier_code, courier_service, shipping_fee, status)
	VALUES ($1, $2, $3, $4, $5)
	RETURNING `+shipmentColumns,
		shipment.OrderID, shipment.CourierCode, shipment.CourierService, shipment.ShippingFee, shipment.Status)
	created, scanErr := scanShipmentRow(err)
	if scanErr != nil {
		var pgErr *pgconn.PgError
		if errors.As(scanErr, &pgErr) && pgErr.Code == "23505" {
			return Shipment{}, ErrShipmentAlreadyExists
		}
		return Shipment{}, scanErr
	}
	return created, nil
}

func (r *PostgresRepository) AttachBiteshipReferences(ctx context.Context, shipmentID string, refs BiteshipReferences) (Shipment, error) {
	status := refs.Status
	if status == "" {
		status = StatusBooked
	}
	row := r.pool.QueryRow(ctx, `UPDATE shipments SET
		biteship_order_id = $2,
		tracking_no = $3,
		waybill_id = $4,
		label_url = $5,
		status = $6,
		updated_at = NOW()
	WHERE id = $1
	RETURNING `+shipmentColumns,
		shipmentID, refs.BiteshipOrderID, refs.TrackingNo, refs.WaybillID, refs.LabelURL, status)
	shipment, err := scanShipment(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Shipment{}, ErrShipmentNotFound
	}
	return shipment, err
}

func (r *PostgresRepository) GetByID(ctx context.Context, id string) (Shipment, error) {
	return r.queryOne(ctx, selectShipmentQuery+` WHERE id = $1`, id)
}

func (r *PostgresRepository) GetByOrderID(ctx context.Context, orderID string) (Shipment, error) {
	return r.queryOne(ctx, selectShipmentQuery+` WHERE order_id = $1`, orderID)
}

func (r *PostgresRepository) GetByBiteshipOrderID(ctx context.Context, biteshipOrderID string) (Shipment, error) {
	return r.queryOne(ctx, selectShipmentQuery+` WHERE biteship_order_id = $1`, biteshipOrderID)
}

func (r *PostgresRepository) GetByTrackingNo(ctx context.Context, trackingNo string) (Shipment, error) {
	return r.queryOne(ctx, selectShipmentQuery+` WHERE tracking_no = $1`, trackingNo)
}

func (r *PostgresRepository) queryOne(ctx context.Context, query string, args ...any) (Shipment, error) {
	shipment, err := scanShipment(r.pool.QueryRow(ctx, query, args...))
	if errors.Is(err, pgx.ErrNoRows) {
		return Shipment{}, ErrShipmentNotFound
	}
	return shipment, err
}

// UpdateStatus stores the latest Biteship status without touching the order.
func (r *PostgresRepository) UpdateStatus(ctx context.Context, shipmentID, status string) (Shipment, error) {
	row := r.pool.QueryRow(ctx, `UPDATE shipments SET status = $2, updated_at = NOW()
		WHERE id = $1
		RETURNING `+shipmentColumns, shipmentID, status)
	shipment, err := scanShipment(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Shipment{}, ErrShipmentNotFound
	}
	return shipment, err
}

// MarkShipped flips the shipment status and advances the order to `shipped`
// (BR-030..BR-031) in one tx. The order transition only fires from paid/processing
// so a later status regression can't drag the order backwards.
func (r *PostgresRepository) MarkShipped(ctx context.Context, shipmentID, status string) (Shipment, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Shipment{}, err
	}
	defer tx.Rollback(ctx)

	row := tx.QueryRow(ctx, `UPDATE shipments SET status = $2, updated_at = NOW()
		WHERE id = $1
		RETURNING `+shipmentColumns, shipmentID, status)
	shipment, err := scanShipment(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Shipment{}, ErrShipmentNotFound
	}
	if err != nil {
		return Shipment{}, err
	}

	if _, err := tx.Exec(ctx, `UPDATE orders SET status = 'shipped', tracking_no = $2, updated_at = NOW()
		WHERE id = $1 AND status IN ('paid', 'processing')`, shipment.OrderID, shipment.TrackingNo); err != nil {
		return Shipment{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Shipment{}, err
	}
	return shipment, nil
}

// MarkDelivered flips the shipment to delivered, stamps delivered_at, and
// advances the order to `delivered` (BR-033) in one tx.
func (r *PostgresRepository) MarkDelivered(ctx context.Context, shipmentID string) (Shipment, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Shipment{}, err
	}
	defer tx.Rollback(ctx)

	row := tx.QueryRow(ctx, `UPDATE shipments SET status = 'delivered', delivered_at = NOW(), updated_at = NOW()
		WHERE id = $1
		RETURNING `+shipmentColumns, shipmentID)
	shipment, err := scanShipment(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Shipment{}, ErrShipmentNotFound
	}
	if err != nil {
		return Shipment{}, err
	}

	if _, err := tx.Exec(ctx, `UPDATE orders SET status = 'delivered', updated_at = NOW()
		WHERE id = $1 AND status IN ('paid', 'processing', 'shipped')`, shipment.OrderID); err != nil {
		return Shipment{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Shipment{}, err
	}
	return shipment, nil
}

// ListCompletable finds delivered shipments past the cutoff whose order is still
// in the `delivered` state (BR-034).
func (r *PostgresRepository) ListCompletable(ctx context.Context, cutoff time.Time, limit int) ([]Shipment, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+shipmentColumns+`
		FROM shipments s
		WHERE s.status = 'delivered' AND s.delivered_at IS NOT NULL AND s.delivered_at <= $1
		  AND EXISTS (SELECT 1 FROM orders o WHERE o.id = s.order_id AND o.status = 'delivered')
		ORDER BY s.delivered_at ASC LIMIT $2`, cutoff, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	shipments := make([]Shipment, 0)
	for rows.Next() {
		shipment, err := scanShipment(rows)
		if err != nil {
			return nil, err
		}
		shipments = append(shipments, shipment)
	}
	return shipments, rows.Err()
}

// CompleteOrder advances the linked order from `delivered` to `completed` (BR-034).
func (r *PostgresRepository) CompleteOrder(ctx context.Context, shipmentID string) error {
	_, err := r.pool.Exec(ctx, `UPDATE orders SET status = 'completed', updated_at = NOW()
		WHERE id = (SELECT order_id FROM shipments WHERE id = $1) AND status = 'delivered'`, shipmentID)
	return err
}

func (r *PostgresRepository) RecordEvent(ctx context.Context, event ShipmentEvent) (ShipmentEvent, bool, error) {
	payload := event.Payload
	if len(payload) == 0 {
		payload = []byte("{}")
	}
	row := r.pool.QueryRow(ctx, `INSERT INTO shipment_events (
		shipment_id, event_key, biteship_order_id, tracking_no,
		event, event_status, payload, signature_valid)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	ON CONFLICT (event_key) WHERE event_key <> ''
	DO NOTHING
	RETURNING id, created_at`,
		nullString(event.ShipmentID), event.EventKey, event.BiteshipOrderID, event.TrackingNo,
		event.Event, event.EventStatus, payload, event.SignatureValid)

	var createdID string
	var createdAt time.Time
	if err := row.Scan(&createdID, &createdAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			existing, err := r.findEvent(ctx, event.EventKey)
			if err != nil {
				return ShipmentEvent{}, true, err
			}
			return existing, true, nil
		}
		return ShipmentEvent{}, false, err
	}
	event.ID = createdID
	event.CreatedAt = createdAt
	return event, false, nil
}

func (r *PostgresRepository) MarkEventProcessed(ctx context.Context, eventID, processingError string) error {
	_, err := r.pool.Exec(ctx, `UPDATE shipment_events SET processed = TRUE, processing_error = $2
		WHERE id = $1`, eventID, processingError)
	return err
}

func (r *PostgresRepository) findEvent(ctx context.Context, eventKey string) (ShipmentEvent, error) {
	var event ShipmentEvent
	err := r.pool.QueryRow(ctx, `SELECT id, COALESCE(shipment_id::text, ''), event_key, biteship_order_id,
		tracking_no, event, event_status, payload, signature_valid, processed, processing_error, created_at
		FROM shipment_events WHERE event_key = $1`, eventKey).
		Scan(&event.ID, &event.ShipmentID, &event.EventKey, &event.BiteshipOrderID,
			&event.TrackingNo, &event.Event, &event.EventStatus, &event.Payload, &event.SignatureValid,
			&event.Processed, &event.ProcessingError, &event.CreatedAt)
	return event, err
}

func scanShipment(row pgx.Row) (Shipment, error) {
	var s Shipment
	err := row.Scan(&s.ID, &s.OrderID, &s.CourierCode, &s.CourierService, &s.CourierName,
		&s.ShippingFee, &s.Status, &s.BiteshipOrderID, &s.TrackingNo, &s.WaybillID, &s.LabelURL,
		&s.DeliveredAt, &s.CreatedAt, &s.UpdatedAt)
	return s, err
}

// scanShipmentRow adapts a QueryRow returned from an INSERT so we can reuse the
// row scanning while still surfacing the raw error for unique-violation mapping.
func scanShipmentRow(row pgx.Row) (Shipment, error) {
	return scanShipment(row)
}

func nullString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
