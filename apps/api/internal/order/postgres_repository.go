package order

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct{ pool *pgxpool.Pool }

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// CreateFromCheckout persists the order + items, reserves stock for each variant,
// and marks the source cart checked_out — all inside a single transaction so a
// failure anywhere (e.g. insufficient stock) rolls the whole checkout back.
func (r *PostgresRepository) CreateFromCheckout(ctx context.Context, order Order, cartID string) (Order, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Order{}, err
	}
	defer tx.Rollback(ctx)

	err = tx.QueryRow(ctx, `INSERT INTO orders (
		order_no, customer_id, status,
		recipient_name, phone, address_line, city, district, postal_code, notes,
		courier_code, courier_service,
		voucher_code, discount,
		subtotal, shipping_fee, total)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
	RETURNING id, created_at, updated_at`,
		order.OrderNo, order.CustomerID, order.Status,
		order.RecipientName, order.Phone, order.AddressLine, order.City, order.District, order.PostalCode, order.Notes,
		order.CourierCode, order.CourierService,
		order.VoucherCode, order.Discount,
		order.Subtotal, order.ShippingFee, order.Total).
		Scan(&order.ID, &order.CreatedAt, &order.UpdatedAt)

	if err != nil {
		return Order{}, mapOrderNoConflict(err)
	}

	for i := range order.Items {
		item := &order.Items[i]
		item.OrderID = order.ID
		err = tx.QueryRow(ctx, `INSERT INTO order_items (
			order_id, variant_id, sku, product_name, variant_name, qty, unit_price, subtotal)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at`,
			item.OrderID, item.VariantID, item.SKU, item.ProductName, item.VariantName, item.Qty, item.UnitPrice, item.Subtotal).
			Scan(&item.ID, &item.CreatedAt)
		if err != nil {
			return Order{}, err
		}

		// Reserve stock atomically; guard against oversell within the tx.
		tag, err := tx.Exec(ctx, `UPDATE product_variants
			SET stock_reserved = stock_reserved + $2, updated_at = NOW()
			WHERE id = $1 AND (stock_on_hand - stock_reserved) >= $2`, item.VariantID, item.Qty)
		if err != nil {
			return Order{}, err
		}
		if tag.RowsAffected() == 0 {
			return Order{}, ErrInsufficientStock
		}
	}

	// Close the cart so a new one is created for the customer's next session.
	if cartID != "" {
		if _, err := tx.Exec(ctx, `UPDATE carts SET status = 'checked_out', updated_at = NOW()
			WHERE id = $1 AND status = 'open'`, cartID); err != nil {
			return Order{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return Order{}, err
	}
	return order, nil
}

func (r *PostgresRepository) GetByID(ctx context.Context, id string) (Order, error) {
	order, err := scanOrder(r.pool.QueryRow(ctx, selectOrderColumns+` WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Order{}, ErrOrderNotFound
	}
	if err != nil {
		return Order{}, err
	}
	order.Items, err = r.listItems(ctx, order.ID)
	return order, err
}

func (r *PostgresRepository) List(ctx context.Context, filter ListFilter) ([]Order, error) {
	query := strings.Builder{}
	query.WriteString(selectOrderColumns + ` WHERE 1=1`)
	args := make([]any, 0, 4)
	argNum := 1

	if filter.CustomerID != "" {
		fmt.Fprintf(&query, ` AND customer_id = $%d`, argNum)
		args = append(args, filter.CustomerID)
		argNum++
	}
	if filter.Status != "" {
		fmt.Fprintf(&query, ` AND status = $%d`, argNum)
		args = append(args, filter.Status)
		argNum++
	}
	query.WriteString(` ORDER BY created_at DESC`)
	if filter.Limit > 0 {
		fmt.Fprintf(&query, ` LIMIT $%d`, argNum)
		args = append(args, filter.Limit)
		argNum++
	}
	if filter.Offset > 0 {
		fmt.Fprintf(&query, ` OFFSET $%d`, argNum)
		args = append(args, filter.Offset)
	}

	rows, err := r.pool.Query(ctx, query.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	orders := make([]Order, 0)
	for rows.Next() {
		o, err := scanOrder(rows)
		if err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}
	return orders, rows.Err()
}

// UpdateStatus transitions an order and adjusts inventory as a side effect:
//   - paid: commit reservation (decrement on_hand + reserved) (BR-035)
//   - cancelled from an unpaid state: release the reservation
//
// The status change + stock adjustment happen in one transaction.
func (r *PostgresRepository) UpdateStatus(ctx context.Context, id, status string) (Order, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Order{}, err
	}
	defer tx.Rollback(ctx)

	current, err := scanOrder(tx.QueryRow(ctx, selectOrderColumns+` WHERE id = $1 FOR UPDATE`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Order{}, ErrOrderNotFound
	}
	if err != nil {
		return Order{}, err
	}

	items, err := r.listItemsTx(ctx, tx, id)
	if err != nil {
		return Order{}, err
	}

	switch {
	case status == StatusPaid && current.Status == StatusPendingPayment:
		if err := commitStock(ctx, tx, items); err != nil {
			return Order{}, err
		}
	case status == StatusCancelled && current.Status == StatusPendingPayment:
		if err := releaseStock(ctx, tx, items); err != nil {
			return Order{}, err
		}
	}

	updated, err := scanOrder(tx.QueryRow(ctx, `UPDATE orders SET status = $2, updated_at = NOW()
		WHERE id = $1
		RETURNING `+orderColumns, id, status))
	if err != nil {
		return Order{}, err
	}
	updated.Items = items
	if err := tx.Commit(ctx); err != nil {
		return Order{}, err
	}
	return updated, nil
}

func commitStock(ctx context.Context, tx pgx.Tx, items []OrderItem) error {
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
			return ErrInsufficientStock
		}
	}
	return nil
}

func releaseStock(ctx context.Context, tx pgx.Tx, items []OrderItem) error {
	for _, item := range items {
		if _, err := tx.Exec(ctx, `UPDATE product_variants
			SET stock_reserved = GREATEST(stock_reserved - $2, 0), updated_at = NOW()
			WHERE id = $1`, item.VariantID, item.Qty); err != nil {
			return err
		}
	}
	return nil
}

func (r *PostgresRepository) listItems(ctx context.Context, orderID string) ([]OrderItem, error) {
	rows, err := r.pool.Query(ctx, selectItemColumns+` WHERE order_id = $1 ORDER BY created_at ASC`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanItems(rows)
}

func (r *PostgresRepository) listItemsTx(ctx context.Context, tx pgx.Tx, orderID string) ([]OrderItem, error) {
	rows, err := tx.Query(ctx, selectItemColumns+` WHERE order_id = $1 ORDER BY created_at ASC`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanItems(rows)
}

const orderColumns = `id, order_no, customer_id, status,
	recipient_name, phone, address_line, city, district, postal_code, notes,
	courier_code, courier_service, tracking_no,
	voucher_code, discount,
	subtotal, shipping_fee, total, created_at, updated_at`

const selectOrderColumns = `SELECT ` + orderColumns + ` FROM orders`

const selectItemColumns = `SELECT id, order_id, variant_id, sku, product_name, variant_name, qty, unit_price, subtotal, created_at FROM order_items`

func scanOrder(row pgx.Row) (Order, error) {
	var o Order
	err := row.Scan(&o.ID, &o.OrderNo, &o.CustomerID, &o.Status,
		&o.RecipientName, &o.Phone, &o.AddressLine, &o.City, &o.District, &o.PostalCode, &o.Notes,
		&o.CourierCode, &o.CourierService, &o.TrackingNo,
		&o.VoucherCode, &o.Discount,
		&o.Subtotal, &o.ShippingFee, &o.Total, &o.CreatedAt, &o.UpdatedAt)

	return o, err
}

func scanItems(rows pgx.Rows) ([]OrderItem, error) {
	items := make([]OrderItem, 0)
	for rows.Next() {
		var item OrderItem
		if err := rows.Scan(&item.ID, &item.OrderID, &item.VariantID, &item.SKU, &item.ProductName, &item.VariantName, &item.Qty, &item.UnitPrice, &item.Subtotal, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func mapOrderNoConflict(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && strings.Contains(strings.ToLower(pgErr.ConstraintName), "order_no") {
		return ErrOrderNoConflict
	}
	return err
}
