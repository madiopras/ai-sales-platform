package promo

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresRepository is the pgx-backed implementation of Repository.
type PostgresRepository struct{ pool *pgxpool.Pool }

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

const voucherColumns = `id, code, description, discount_type, discount_value,
	min_order_amount, quota, used_count, starts_at, expires_at, is_active,
	created_at, updated_at`

const selectVoucherQuery = `SELECT ` + voucherColumns + ` FROM vouchers`

func (r *PostgresRepository) Create(ctx context.Context, voucher Voucher) (Voucher, error) {
	row := r.pool.QueryRow(ctx, `INSERT INTO vouchers (
		code, description, discount_type, discount_value,
		min_order_amount, quota, starts_at, expires_at, is_active)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	RETURNING `+voucherColumns,
		voucher.Code, voucher.Description, voucher.DiscountType, voucher.DiscountValue,
		voucher.MinOrderAmount, voucher.Quota, voucher.StartsAt, voucher.ExpiresAt, voucher.IsActive)
	created, err := scanVoucher(row)
	if err != nil {
		if isUniqueViolation(err) {
			return Voucher{}, ErrCodeTaken
		}
		return Voucher{}, err
	}
	return created, nil
}

func (r *PostgresRepository) Update(ctx context.Context, id string, voucher Voucher) (Voucher, error) {
	row := r.pool.QueryRow(ctx, `UPDATE vouchers SET
		description = $2,
		discount_type = $3,
		discount_value = $4,
		min_order_amount = $5,
		quota = $6,
		starts_at = $7,
		expires_at = $8,
		is_active = $9,
		updated_at = NOW()
	WHERE id = $1
	RETURNING `+voucherColumns,
		id, voucher.Description, voucher.DiscountType, voucher.DiscountValue,
		voucher.MinOrderAmount, voucher.Quota, voucher.StartsAt, voucher.ExpiresAt, voucher.IsActive)
	updated, err := scanVoucher(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Voucher{}, ErrVoucherNotFound
	}
	return updated, err
}

func (r *PostgresRepository) Delete(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM vouchers WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrVoucherNotFound
	}
	return nil
}

func (r *PostgresRepository) GetByID(ctx context.Context, id string) (Voucher, error) {
	return r.queryOne(ctx, selectVoucherQuery+` WHERE id = $1`, id)
}

func (r *PostgresRepository) GetByCode(ctx context.Context, code string) (Voucher, error) {
	return r.queryOne(ctx, selectVoucherQuery+` WHERE lower(code) = lower($1)`, code)
}

func (r *PostgresRepository) List(ctx context.Context, filter ListFilter) ([]Voucher, error) {
	query := strings.Builder{}
	query.WriteString(selectVoucherQuery + ` WHERE 1=1`)
	args := make([]any, 0, 3)
	argNum := 1

	if filter.ActiveOnly {
		query.WriteString(` AND is_active = TRUE AND expires_at > NOW()`)
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

	vouchers := make([]Voucher, 0)
	for rows.Next() {
		v, err := scanVoucher(rows)
		if err != nil {
			return nil, err
		}
		vouchers = append(vouchers, v)
	}
	return vouchers, rows.Err()
}

// Redeem increments used_count while re-checking quota under a row lock so two
// concurrent checkouts can't push a limited voucher past its quota. A quota of 0
// means unlimited.
func (r *PostgresRepository) Redeem(ctx context.Context, id string) (Voucher, error) {
	row := r.pool.QueryRow(ctx, `UPDATE vouchers SET
		used_count = used_count + 1,
		updated_at = NOW()
	WHERE id = $1 AND (quota = 0 OR used_count < quota)
	RETURNING `+voucherColumns, id)
	updated, err := scanVoucher(row)
	if errors.Is(err, pgx.ErrNoRows) {
		// Either the voucher vanished or its quota is now exhausted. Disambiguate
		// so the caller returns the most accurate error.
		if _, getErr := r.GetByID(ctx, id); errors.Is(getErr, ErrVoucherNotFound) {
			return Voucher{}, ErrVoucherNotFound
		}
		return Voucher{}, ErrVoucherExhausted
	}
	return updated, err
}

func (r *PostgresRepository) queryOne(ctx context.Context, query string, args ...any) (Voucher, error) {
	voucher, err := scanVoucher(r.pool.QueryRow(ctx, query, args...))
	if errors.Is(err, pgx.ErrNoRows) {
		return Voucher{}, ErrVoucherNotFound
	}
	return voucher, err
}

func scanVoucher(row pgx.Row) (Voucher, error) {
	var v Voucher
	err := row.Scan(&v.ID, &v.Code, &v.Description, &v.DiscountType, &v.DiscountValue,
		&v.MinOrderAmount, &v.Quota, &v.UsedCount, &v.StartsAt, &v.ExpiresAt, &v.IsActive,
		&v.CreatedAt, &v.UpdatedAt)
	return v, err
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
