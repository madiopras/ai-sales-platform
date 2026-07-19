package catalog

import "context"

// Repository defines catalog data access interface
type Repository interface {
	// Category operations
	CreateCategory(ctx context.Context, category Category) (Category, error)
	GetCategory(ctx context.Context, id string) (Category, error)
	ListCategories(ctx context.Context) ([]Category, error)
	UpdateCategory(ctx context.Context, category Category) (Category, error)
	DeleteCategory(ctx context.Context, id string) error

	// Product operations
	CreateProduct(ctx context.Context, product Product) (Product, error)
	GetProduct(ctx context.Context, id string) (Product, error)
	GetProductBySlug(ctx context.Context, slug string) (Product, error)
	ListProducts(ctx context.Context, filter ProductListFilter) ([]Product, error)
	UpdateProduct(ctx context.Context, product Product) (Product, error)
	DeleteProduct(ctx context.Context, id string) error

	// ProductVariant operations
	CreateVariant(ctx context.Context, variant Variant) (Variant, error)
	GetVariant(ctx context.Context, id string) (Variant, error)
	ListVariantsByProduct(ctx context.Context, productID string) ([]Variant, error)
	UpdateVariant(ctx context.Context, variant Variant) (Variant, error)
	DeleteVariant(ctx context.Context, id string) error
	UpdateVariantStock(ctx context.Context, id string, stockOnHand int) (Variant, error)
	ReserveStock(ctx context.Context, variantID string, qty int) error
	ReleaseStock(ctx context.Context, variantID string, qty int) error
	CommitStock(ctx context.Context, variantID string, qty int) error
}
