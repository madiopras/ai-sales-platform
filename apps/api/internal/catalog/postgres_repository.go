package catalog

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

func (r *PostgresRepository) CreateCategory(ctx context.Context, category Category) (Category, error) {
	err := r.pool.QueryRow(ctx, `INSERT INTO categories (name, slug, is_active)
VALUES ($1, $2, $3)
RETURNING id, name, slug, is_active, created_at, updated_at`, category.Name, category.Slug, category.IsActive).
		Scan(&category.ID, &category.Name, &category.Slug, &category.IsActive, &category.CreatedAt, &category.UpdatedAt)
	return category, mapUniqueViolation(err)
}

func (r *PostgresRepository) UpdateCategory(ctx context.Context, category Category) (Category, error) {
	err := r.pool.QueryRow(ctx, `UPDATE categories
SET name = $2, slug = $3, is_active = $4, updated_at = NOW()
WHERE id = $1
RETURNING id, name, slug, is_active, created_at, updated_at`, category.ID, category.Name, category.Slug, category.IsActive).
		Scan(&category.ID, &category.Name, &category.Slug, &category.IsActive, &category.CreatedAt, &category.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Category{}, ErrCategoryNotFound
	}
	return category, mapUniqueViolation(err)
}

func (r *PostgresRepository) GetCategory(ctx context.Context, id string) (Category, error) {
	var category Category
	err := r.pool.QueryRow(ctx, `SELECT id, name, slug, is_active, created_at, updated_at FROM categories WHERE id = $1`, id).
		Scan(&category.ID, &category.Name, &category.Slug, &category.IsActive, &category.CreatedAt, &category.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Category{}, ErrCategoryNotFound
	}
	return category, err
}

func (r *PostgresRepository) ListCategories(ctx context.Context) ([]Category, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, name, slug, is_active, created_at, updated_at FROM categories ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	categories := make([]Category, 0)
	for rows.Next() {
		var category Category
		if err := rows.Scan(&category.ID, &category.Name, &category.Slug, &category.IsActive, &category.CreatedAt, &category.UpdatedAt); err != nil {
			return nil, err
		}
		categories = append(categories, category)
	}
	return categories, rows.Err()
}

func (r *PostgresRepository) DeleteCategory(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM categories WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrCategoryNotFound
	}
	return nil
}

func (r *PostgresRepository) CreateProduct(ctx context.Context, product Product) (Product, error) {
	err := r.pool.QueryRow(ctx, `INSERT INTO products (category_id, name, slug, description, status)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, category_id, name, slug, description, status, created_at, updated_at`, product.CategoryID, product.Name, product.Slug, product.Description, product.Status).
		Scan(&product.ID, &product.CategoryID, &product.Name, &product.Slug, &product.Description, &product.Status, &product.CreatedAt, &product.UpdatedAt)
	return product, mapUniqueViolation(err)
}

func (r *PostgresRepository) UpdateProduct(ctx context.Context, product Product) (Product, error) {
	err := r.pool.QueryRow(ctx, `UPDATE products
SET category_id = $2, name = $3, slug = $4, description = $5, status = $6, updated_at = NOW()
WHERE id = $1
RETURNING id, category_id, name, slug, description, status, created_at, updated_at`, product.ID, product.CategoryID, product.Name, product.Slug, product.Description, product.Status).
		Scan(&product.ID, &product.CategoryID, &product.Name, &product.Slug, &product.Description, &product.Status, &product.CreatedAt, &product.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Product{}, ErrProductNotFound
	}
	return product, mapUniqueViolation(err)
}

func (r *PostgresRepository) GetProduct(ctx context.Context, id string) (Product, error) {
	product, err := r.scanProduct(r.pool.QueryRow(ctx, `SELECT id, category_id, name, slug, description, status, created_at, updated_at FROM products WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Product{}, ErrProductNotFound
	}
	if err != nil {
		return Product{}, err
	}
	product.Variants, err = r.listVariantsByProduct(ctx, product.ID, false)
	return product, err
}

func (r *PostgresRepository) GetProductBySlug(ctx context.Context, slug string) (Product, error) {
	product, err := r.scanProduct(r.pool.QueryRow(ctx, `SELECT id, category_id, name, slug, description, status, created_at, updated_at FROM products WHERE slug = $1`, slug))
	if errors.Is(err, pgx.ErrNoRows) {
		return Product{}, ErrProductNotFound
	}
	if err != nil {
		return Product{}, err
	}
	product.Variants, err = r.listVariantsByProduct(ctx, product.ID, false)
	return product, err
}

func (r *PostgresRepository) ListProducts(ctx context.Context, filter ProductListFilter) ([]Product, error) {
	query := strings.Builder{}
	query.WriteString(`SELECT id, category_id, name, slug, description, status, created_at, updated_at FROM products WHERE 1=1`)

	args := make([]any, 0, 4)
	argNum := 1

	if filter.ActiveOnly {
		query.WriteString(` AND status = 'active'`)
	} else if filter.Status != "" {
		fmt.Fprintf(&query, ` AND status = $%d`, argNum)
		args = append(args, filter.Status)
		argNum++
	}
	if filter.CategoryID != "" {
		fmt.Fprintf(&query, ` AND category_id = $%d`, argNum)
		args = append(args, filter.CategoryID)
		argNum++
	}
	if filter.Query != "" {
		fmt.Fprintf(&query, ` AND name ILIKE $%d`, argNum)
		args = append(args, "%"+filter.Query+"%")
		argNum++
	}

	query.WriteString(` ORDER BY name`)
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

	products := make([]Product, 0)
	for rows.Next() {
		product, err := r.scanProduct(rows)
		if err != nil {
			return nil, err
		}
		if filter.ActiveOnly {
			product.Variants, err = r.listVariantsByProduct(ctx, product.ID, true)
			if err != nil {
				return nil, err
			}
		}
		products = append(products, product)
	}
	return products, rows.Err()
}

func (r *PostgresRepository) DeleteProduct(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM products WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrProductNotFound
	}
	return nil
}

func (r *PostgresRepository) CreateVariant(ctx context.Context, variant Variant) (Variant, error) {
	err := r.pool.QueryRow(ctx, `INSERT INTO product_variants (product_id, sku, name, price, stock_on_hand, stock_reserved, is_active)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, product_id, sku, name, price, stock_on_hand, stock_reserved, is_active, created_at, updated_at`, variant.ProductID, variant.SKU, variant.Name, variant.Price, variant.StockOnHand, variant.StockReserved, variant.IsActive).
		Scan(&variant.ID, &variant.ProductID, &variant.SKU, &variant.Name, &variant.Price, &variant.StockOnHand, &variant.StockReserved, &variant.IsActive, &variant.CreatedAt, &variant.UpdatedAt)
	return variant, mapUniqueViolation(err)
}

func (r *PostgresRepository) UpdateVariant(ctx context.Context, variant Variant) (Variant, error) {
	err := r.pool.QueryRow(ctx, `UPDATE product_variants
SET sku = $2, name = $3, price = $4, is_active = $5, updated_at = NOW()
WHERE id = $1
RETURNING id, product_id, sku, name, price, stock_on_hand, stock_reserved, is_active, created_at, updated_at`, variant.ID, variant.SKU, variant.Name, variant.Price, variant.IsActive).
		Scan(&variant.ID, &variant.ProductID, &variant.SKU, &variant.Name, &variant.Price, &variant.StockOnHand, &variant.StockReserved, &variant.IsActive, &variant.CreatedAt, &variant.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Variant{}, ErrVariantNotFound
	}
	return variant, mapUniqueViolation(err)
}

func (r *PostgresRepository) GetVariant(ctx context.Context, id string) (Variant, error) {
	return r.scanVariant(r.pool.QueryRow(ctx, `SELECT id, product_id, sku, name, price, stock_on_hand, stock_reserved, is_active, created_at, updated_at FROM product_variants WHERE id = $1`, id))
}

func (r *PostgresRepository) ListVariantsByProduct(ctx context.Context, productID string) ([]Variant, error) {
	return r.listVariantsByProduct(ctx, productID, false)
}

func (r *PostgresRepository) DeleteVariant(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM product_variants WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrVariantNotFound
	}
	return nil
}

func (r *PostgresRepository) UpdateVariantStock(ctx context.Context, id string, stockOnHand int) (Variant, error) {
	variant, err := r.scanVariant(r.pool.QueryRow(ctx, `UPDATE product_variants
SET stock_on_hand = $2, updated_at = NOW()
WHERE id = $1
RETURNING id, product_id, sku, name, price, stock_on_hand, stock_reserved, is_active, created_at, updated_at`, id, stockOnHand))
	if errors.Is(err, pgx.ErrNoRows) {
		return Variant{}, ErrVariantNotFound
	}
	return variant, err
}

func (r *PostgresRepository) ReserveStock(ctx context.Context, variantID string, qty int) error {
	tag, err := r.pool.Exec(ctx, `UPDATE product_variants
SET stock_reserved = stock_reserved + $2, updated_at = NOW()
WHERE id = $1 AND (stock_on_hand - stock_reserved) >= $2`, variantID, qty)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrInsufficientStock
	}
	return nil
}

func (r *PostgresRepository) ReleaseStock(ctx context.Context, variantID string, qty int) error {
	tag, err := r.pool.Exec(ctx, `UPDATE product_variants
SET stock_reserved = GREATEST(stock_reserved - $2, 0), updated_at = NOW()
WHERE id = $1`, variantID, qty)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrVariantNotFound
	}
	return nil
}

func (r *PostgresRepository) CommitStock(ctx context.Context, variantID string, qty int) error {
	tag, err := r.pool.Exec(ctx, `UPDATE product_variants
SET stock_on_hand = stock_on_hand - $2,
    stock_reserved = stock_reserved - $2,
    updated_at = NOW()
WHERE id = $1 AND stock_on_hand >= $2 AND stock_reserved >= $2`, variantID, qty)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrInsufficientStock
	}
	return nil
}

func (r *PostgresRepository) scanProduct(row pgx.Row) (Product, error) {
	var product Product
	err := row.Scan(&product.ID, &product.CategoryID, &product.Name, &product.Slug, &product.Description, &product.Status, &product.CreatedAt, &product.UpdatedAt)
	return product, err
}

func (r *PostgresRepository) scanVariant(row pgx.Row) (Variant, error) {
	var variant Variant
	err := row.Scan(&variant.ID, &variant.ProductID, &variant.SKU, &variant.Name, &variant.Price, &variant.StockOnHand, &variant.StockReserved, &variant.IsActive, &variant.CreatedAt, &variant.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Variant{}, ErrVariantNotFound
	}
	return variant, err
}

func (r *PostgresRepository) listVariantsByProduct(ctx context.Context, productID string, activeOnly bool) ([]Variant, error) {
	query := `SELECT id, product_id, sku, name, price, stock_on_hand, stock_reserved, is_active, created_at, updated_at
FROM product_variants
WHERE product_id = $1`
	if activeOnly {
		query += ` AND is_active = TRUE AND (stock_on_hand - stock_reserved) > 0`
	}
	query += ` ORDER BY name`

	rows, err := r.pool.Query(ctx, query, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	variants := make([]Variant, 0)
	for rows.Next() {
		variant, err := r.scanVariant(rows)
		if err != nil {
			return nil, err
		}
		variants = append(variants, variant)
	}
	return variants, rows.Err()
}

func mapUniqueViolation(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		constraint := strings.ToLower(pgErr.ConstraintName)
		if strings.Contains(constraint, "slug") {
			return ErrSlugTaken
		}
		if strings.Contains(constraint, "sku") {
			return ErrSKUTaken
		}
	}
	return err
}
