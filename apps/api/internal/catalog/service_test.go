package catalog

import (
	"context"
	"strings"
	"testing"
)

func TestCreateProductAndSearchActive(t *testing.T) {
	repo := &memoryRepository{products: make(map[string]Product)}
	service := NewService(repo)

	categoryID := "category-1"
	created, err := service.CreateProduct(context.Background(), CreateProductInput{
		CategoryID: &categoryID,
		Name:       "Sample Product",
		Status:     "active",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Slug != "sample-product" {
		t.Fatalf("expected slug sample-product, got %q", created.Slug)
	}

	variant, err := service.CreateVariant(context.Background(), CreateVariantInput{
		ProductID:   created.ID,
		SKU:         "SKU-1",
		Name:        "Default",
		Price:       10000,
		StockOnHand: 5,
		IsActive:    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	repo.variants[created.ID] = []Variant{variant}

	results, err := service.SearchActive(context.Background(), "sample", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 active product, got %d", len(results))
	}
	if len(results[0].Variants) != 1 {
		t.Fatalf("expected 1 active variant, got %d", len(results[0].Variants))
	}
}

type memoryRepository struct {
	products map[string]Product
	variants map[string][]Variant
	nextID   int
}

func (r *memoryRepository) next() string {
	r.nextID++
	return "id-" + strings.TrimSpace(strings.Repeat("0", 1)) + string(rune('0'+r.nextID))
}

func (r *memoryRepository) CreateCategory(context.Context, Category) (Category, error) {
	return Category{}, nil
}
func (r *memoryRepository) UpdateCategory(context.Context, Category) (Category, error) {
	return Category{}, nil
}
func (r *memoryRepository) GetCategory(context.Context, string) (Category, error) {
	return Category{}, ErrCategoryNotFound
}
func (r *memoryRepository) ListCategories(context.Context) ([]Category, error) { return nil, nil }
func (r *memoryRepository) DeleteCategory(context.Context, string) error       { return nil }

func (r *memoryRepository) CreateProduct(_ context.Context, product Product) (Product, error) {
	if r.products == nil {
		r.products = make(map[string]Product)
	}
	product.ID = r.next()
	r.products[product.ID] = product
	return product, nil
}

func (r *memoryRepository) UpdateProduct(_ context.Context, product Product) (Product, error) {
	r.products[product.ID] = product
	return product, nil
}

func (r *memoryRepository) GetProduct(_ context.Context, id string) (Product, error) {
	product, ok := r.products[id]
	if !ok {
		return Product{}, ErrProductNotFound
	}
	product.Variants = r.variants[id]
	return product, nil
}

func (r *memoryRepository) GetProductBySlug(_ context.Context, slug string) (Product, error) {
	for _, product := range r.products {
		if product.Slug == slug {
			product.Variants = r.variants[product.ID]
			return product, nil
		}
	}
	return Product{}, ErrProductNotFound
}

func (r *memoryRepository) ListProducts(_ context.Context, filter ProductListFilter) ([]Product, error) {
	results := make([]Product, 0)
	for _, product := range r.products {
		if filter.ActiveOnly && product.Status != "active" {
			continue
		}
		if filter.Status != "" && product.Status != filter.Status {
			continue
		}
		if filter.Query != "" && !strings.Contains(strings.ToLower(product.Name), strings.ToLower(filter.Query)) {
			continue
		}
		if filter.ActiveOnly {
			product.Variants = filterActiveVariants(r.variants[product.ID])
		}
		results = append(results, product)
	}
	return results, nil
}

func (r *memoryRepository) DeleteProduct(context.Context, string) error { return nil }

func (r *memoryRepository) CreateVariant(_ context.Context, variant Variant) (Variant, error) {
	if r.variants == nil {
		r.variants = make(map[string][]Variant)
	}
	variant.ID = r.next()
	r.variants[variant.ProductID] = append(r.variants[variant.ProductID], variant)
	return variant, nil
}

func (r *memoryRepository) UpdateVariant(context.Context, Variant) (Variant, error) {
	return Variant{}, nil
}
func (r *memoryRepository) GetVariant(context.Context, string) (Variant, error) {
	return Variant{}, ErrVariantNotFound
}
func (r *memoryRepository) ListVariantsByProduct(_ context.Context, productID string) ([]Variant, error) {
	return r.variants[productID], nil
}
func (r *memoryRepository) DeleteVariant(_ context.Context, id string) error {
	for productID, variants := range r.variants {
		for i, variant := range variants {
			if variant.ID == id {
				r.variants[productID] = append(variants[:i], variants[i+1:]...)
				return nil
			}
		}
	}
	return ErrVariantNotFound
}

func (r *memoryRepository) UpdateVariantStock(_ context.Context, id string, stockOnHand int) (Variant, error) {
	for productID, variants := range r.variants {
		for i, variant := range variants {
			if variant.ID == id {
				variant.StockOnHand = stockOnHand
				r.variants[productID][i] = variant
				return variant, nil
			}
		}
	}
	return Variant{}, ErrVariantNotFound
}
func (r *memoryRepository) ReserveStock(context.Context, string, int) error { return nil }
func (r *memoryRepository) ReleaseStock(context.Context, string, int) error { return nil }
func (r *memoryRepository) CommitStock(context.Context, string, int) error  { return nil }
