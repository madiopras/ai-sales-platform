package customer

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct{ pool *pgxpool.Pool }

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) UpsertByPhone(ctx context.Context, phone, name string) (Customer, error) {
	var customer Customer
	err := r.pool.QueryRow(ctx, `INSERT INTO customers (phone, name)
VALUES ($1, $2)
ON CONFLICT (phone) DO UPDATE SET name = EXCLUDED.name, updated_at = NOW()
RETURNING id, phone, name, created_at, updated_at`, phone, name).
		Scan(&customer.ID, &customer.Phone, &customer.Name, &customer.CreatedAt, &customer.UpdatedAt)
	return customer, err
}

func (r *PostgresRepository) GetByID(ctx context.Context, id string) (Customer, error) {
	var customer Customer
	err := r.pool.QueryRow(ctx, `SELECT id, phone, name, created_at, updated_at FROM customers WHERE id = $1`, id).
		Scan(&customer.ID, &customer.Phone, &customer.Name, &customer.CreatedAt, &customer.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Customer{}, ErrCustomerNotFound
	}
	return customer, err
}

func (r *PostgresRepository) List(ctx context.Context, limit, offset int) ([]Customer, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, phone, name, created_at, updated_at
FROM customers ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	customers := make([]Customer, 0)
	for rows.Next() {
		var customer Customer
		if err := rows.Scan(&customer.ID, &customer.Phone, &customer.Name, &customer.CreatedAt, &customer.UpdatedAt); err != nil {
			return nil, err
		}
		customers = append(customers, customer)
	}
	return customers, rows.Err()
}

func (r *PostgresRepository) ListAddresses(ctx context.Context, customerID string) ([]Address, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, customer_id, label, recipient_name, phone, address_line, city, district, postal_code, notes, is_default, created_at, updated_at
FROM customer_addresses WHERE customer_id = $1 ORDER BY is_default DESC, created_at ASC`, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAddresses(rows)
}

func (r *PostgresRepository) GetDefaultAddress(ctx context.Context, customerID string) (Address, error) {
	var address Address
	err := r.pool.QueryRow(ctx, `SELECT id, customer_id, label, recipient_name, phone, address_line, city, district, postal_code, notes, is_default, created_at, updated_at
FROM customer_addresses WHERE customer_id = $1 AND is_default = TRUE`, customerID).
		Scan(&address.ID, &address.CustomerID, &address.Label, &address.RecipientName, &address.Phone, &address.AddressLine, &address.City, &address.District, &address.PostalCode, &address.Notes, &address.IsDefault, &address.CreatedAt, &address.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Address{}, ErrAddressNotFound
	}
	return address, err
}

func (r *PostgresRepository) CreateAddress(ctx context.Context, address Address) (Address, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Address{}, err
	}
	defer tx.Rollback(ctx)

	if address.IsDefault {
		if _, err := tx.Exec(ctx, `UPDATE customer_addresses SET is_default = FALSE, updated_at = NOW() WHERE customer_id = $1 AND is_default = TRUE`, address.CustomerID); err != nil {
			return Address{}, err
		}
	}
	err = tx.QueryRow(ctx, `INSERT INTO customer_addresses (customer_id, label, recipient_name, phone, address_line, city, district, postal_code, notes, is_default)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING id, customer_id, label, recipient_name, phone, address_line, city, district, postal_code, notes, is_default, created_at, updated_at`,
		address.CustomerID, address.Label, address.RecipientName, address.Phone, address.AddressLine, address.City, address.District, address.PostalCode, address.Notes, address.IsDefault).
		Scan(&address.ID, &address.CustomerID, &address.Label, &address.RecipientName, &address.Phone, &address.AddressLine, &address.City, &address.District, &address.PostalCode, &address.Notes, &address.IsDefault, &address.CreatedAt, &address.UpdatedAt)
	if err != nil {
		return Address{}, err
	}
	return address, tx.Commit(ctx)
}

func (r *PostgresRepository) GetOpenCart(ctx context.Context, customerID string) (Cart, error) {
	var cart Cart
	err := r.pool.QueryRow(ctx, `SELECT id, customer_id, status, created_at, updated_at FROM carts WHERE customer_id = $1 AND status = 'open'`, customerID).
		Scan(&cart.ID, &cart.CustomerID, &cart.Status, &cart.CreatedAt, &cart.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Cart{}, ErrCartNotFound
	}
	return cart, err
}

func (r *PostgresRepository) CreateOpenCart(ctx context.Context, customerID string) (Cart, error) {
	var cart Cart
	err := r.pool.QueryRow(ctx, `INSERT INTO carts (customer_id, status) VALUES ($1, 'open')
RETURNING id, customer_id, status, created_at, updated_at`, customerID).
		Scan(&cart.ID, &cart.CustomerID, &cart.Status, &cart.CreatedAt, &cart.UpdatedAt)
	return cart, err
}

func (r *PostgresRepository) ListCartItems(ctx context.Context, cartID string) ([]CartItem, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, cart_id, variant_id, qty, unit_price, created_at, updated_at
FROM cart_items WHERE cart_id = $1 ORDER BY created_at ASC`, cartID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]CartItem, 0)
	for rows.Next() {
		var item CartItem
		if err := rows.Scan(&item.ID, &item.CartID, &item.VariantID, &item.Qty, &item.UnitPrice, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) UpsertCartItem(ctx context.Context, item CartItem) (CartItem, error) {
	err := r.pool.QueryRow(ctx, `INSERT INTO cart_items (cart_id, variant_id, qty, unit_price)
VALUES ($1, $2, $3, $4)
ON CONFLICT (cart_id, variant_id) DO UPDATE SET qty = EXCLUDED.qty, unit_price = EXCLUDED.unit_price, updated_at = NOW()
RETURNING id, cart_id, variant_id, qty, unit_price, created_at, updated_at`,
		item.CartID, item.VariantID, item.Qty, item.UnitPrice).
		Scan(&item.ID, &item.CartID, &item.VariantID, &item.Qty, &item.UnitPrice, &item.CreatedAt, &item.UpdatedAt)
	return item, err
}

func (r *PostgresRepository) UpdateCartItem(ctx context.Context, cartID, variantID string, qty int) (CartItem, error) {
	var item CartItem
	err := r.pool.QueryRow(ctx, `UPDATE cart_items SET qty = $3, updated_at = NOW()
WHERE cart_id = $1 AND variant_id = $2
RETURNING id, cart_id, variant_id, qty, unit_price, created_at, updated_at`, cartID, variantID, qty).
		Scan(&item.ID, &item.CartID, &item.VariantID, &item.Qty, &item.UnitPrice, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return CartItem{}, ErrCartItemNotFound
	}
	return item, err
}

func (r *PostgresRepository) RemoveCartItem(ctx context.Context, cartID, variantID string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM cart_items WHERE cart_id = $1 AND variant_id = $2`, cartID, variantID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrCartItemNotFound
	}
	return nil
}

func scanAddresses(rows pgx.Rows) ([]Address, error) {
	addresses := make([]Address, 0)
	for rows.Next() {
		var address Address
		if err := rows.Scan(&address.ID, &address.CustomerID, &address.Label, &address.RecipientName, &address.Phone, &address.AddressLine, &address.City, &address.District, &address.PostalCode, &address.Notes, &address.IsDefault, &address.CreatedAt, &address.UpdatedAt); err != nil {
			return nil, err
		}
		addresses = append(addresses, address)
	}
	return addresses, rows.Err()
}
