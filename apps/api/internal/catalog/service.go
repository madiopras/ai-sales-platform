package catalog

import (
	"context"
	"fmt"
	"strings"
	"unicode"
)

type Service struct{ repository Repository }

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

type CreateCategoryInput struct {
	Name     string `json:"name"`
	Slug     string `json:"slug"`
	IsActive bool   `json:"is_active"`
}

type UpdateCategoryInput struct {
	Name     *string `json:"name"`
	Slug     *string `json:"slug"`
	IsActive *bool   `json:"is_active"`
}

type CreateProductInput struct {
	CategoryID  *string `json:"category_id"`
	Name        string  `json:"name"`
	Slug        string  `json:"slug"`
	Description string  `json:"description"`
	Status      string  `json:"status"`
}

type UpdateProductInput struct {
	CategoryID  *string `json:"category_id"`
	Name        *string `json:"name"`
	Slug        *string `json:"slug"`
	Description *string `json:"description"`
	Status      *string `json:"status"`
}

type CreateVariantInput struct {
	ProductID   string  `json:"product_id"`
	SKU         string  `json:"sku"`
	Name        string  `json:"name"`
	Price       float64 `json:"price"`
	StockOnHand int     `json:"stock_on_hand"`
	IsActive    bool    `json:"is_active"`
}

type UpdateVariantInput struct {
	SKU      *string  `json:"sku"`
	Name     *string  `json:"name"`
	Price    *float64 `json:"price"`
	IsActive *bool    `json:"is_active"`
}

type UpdateStockInput struct {
	StockOnHand int `json:"stock_on_hand"`
}

func (s *Service) CreateCategory(ctx context.Context, input CreateCategoryInput) (Category, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return Category{}, fmt.Errorf("name is required")
	}
	slug := strings.TrimSpace(input.Slug)
	if slug == "" {
		slug = slugify(name)
	}
	return s.repository.CreateCategory(ctx, Category{Name: name, Slug: slug, IsActive: input.IsActive})
}

func (s *Service) UpdateCategory(ctx context.Context, id string, input UpdateCategoryInput) (Category, error) {
	category, err := s.repository.GetCategory(ctx, id)
	if err != nil {
		return Category{}, err
	}
	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" {
			return Category{}, fmt.Errorf("name is required")
		}
		category.Name = name
	}
	if input.Slug != nil {
		slug := strings.TrimSpace(*input.Slug)
		if slug == "" {
			return Category{}, fmt.Errorf("slug is required")
		}
		category.Slug = slug
	}
	if input.IsActive != nil {
		category.IsActive = *input.IsActive
	}
	return s.repository.UpdateCategory(ctx, category)
}

func (s *Service) GetCategory(ctx context.Context, id string) (Category, error) {
	return s.repository.GetCategory(ctx, id)
}

func (s *Service) ListCategories(ctx context.Context) ([]Category, error) {
	return s.repository.ListCategories(ctx)
}

func (s *Service) DeleteCategory(ctx context.Context, id string) error {
	return s.repository.DeleteCategory(ctx, id)
}

func (s *Service) CreateProduct(ctx context.Context, input CreateProductInput) (Product, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return Product{}, fmt.Errorf("name is required")
	}
	if input.CategoryID == nil || *input.CategoryID == "" {
		return Product{}, fmt.Errorf("category_id is required")
	}
	status := strings.TrimSpace(input.Status)
	if status == "" {
		status = "draft"
	}
	if err := validateStatus(status); err != nil {
		return Product{}, err
	}
	slug := strings.TrimSpace(input.Slug)
	if slug == "" {
		slug = slugify(name)
	}
	return s.repository.CreateProduct(ctx, Product{
		CategoryID:  *input.CategoryID,
		Name:        name,
		Slug:        slug,
		Description: strings.TrimSpace(input.Description),
		Status:      status,
	})
}

func (s *Service) UpdateProduct(ctx context.Context, id string, input UpdateProductInput) (Product, error) {
	product, err := s.repository.GetProduct(ctx, id)
	if err != nil {
		return Product{}, err
	}
	if input.CategoryID != nil {
		product.CategoryID = *input.CategoryID
	}
	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" {
			return Product{}, fmt.Errorf("name is required")
		}
		product.Name = name
	}
	if input.Slug != nil {
		slug := strings.TrimSpace(*input.Slug)
		if slug == "" {
			return Product{}, fmt.Errorf("slug is required")
		}
		product.Slug = slug
	}
	if input.Description != nil {
		product.Description = strings.TrimSpace(*input.Description)
	}
	if input.Status != nil {
		if err := validateStatus(strings.TrimSpace(*input.Status)); err != nil {
			return Product{}, err
		}
		product.Status = strings.TrimSpace(*input.Status)
	}
	product.Variants = nil
	return s.repository.UpdateProduct(ctx, product)
}

func (s *Service) GetProduct(ctx context.Context, id string) (Product, error) {
	return s.repository.GetProduct(ctx, id)
}

func (s *Service) ListProducts(ctx context.Context, filter ProductListFilter) ([]Product, error) {
	return s.repository.ListProducts(ctx, filter)
}

func (s *Service) DeleteProduct(ctx context.Context, id string) error {
	return s.repository.DeleteProduct(ctx, id)
}

func (s *Service) CreateVariant(ctx context.Context, input CreateVariantInput) (Variant, error) {
	sku := strings.TrimSpace(input.SKU)
	name := strings.TrimSpace(input.Name)
	if input.ProductID == "" {
		return Variant{}, fmt.Errorf("product_id is required")
	}
	if sku == "" {
		return Variant{}, fmt.Errorf("sku is required")
	}
	if name == "" {
		return Variant{}, fmt.Errorf("name is required")
	}
	if input.Price < 0 {
		return Variant{}, fmt.Errorf("price must be zero or greater")
	}
	if input.StockOnHand < 0 {
		return Variant{}, fmt.Errorf("stock_on_hand must be zero or greater")
	}
	return s.repository.CreateVariant(ctx, Variant{
		ProductID:   input.ProductID,
		SKU:         sku,
		Name:        name,
		Price:       input.Price,
		StockOnHand: input.StockOnHand,
		IsActive:    input.IsActive,
	})
}

func (s *Service) UpdateVariant(ctx context.Context, id string, input UpdateVariantInput) (Variant, error) {
	variant, err := s.repository.GetVariant(ctx, id)
	if err != nil {
		return Variant{}, err
	}
	if input.SKU != nil {
		sku := strings.TrimSpace(*input.SKU)
		if sku == "" {
			return Variant{}, fmt.Errorf("sku is required")
		}
		variant.SKU = sku
	}
	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" {
			return Variant{}, fmt.Errorf("name is required")
		}
		variant.Name = name
	}
	if input.Price != nil {
		if *input.Price < 0 {
			return Variant{}, fmt.Errorf("price must be zero or greater")
		}
		variant.Price = *input.Price
	}
	if input.IsActive != nil {
		variant.IsActive = *input.IsActive
	}
	return s.repository.UpdateVariant(ctx, variant)
}

func (s *Service) GetVariant(ctx context.Context, id string) (Variant, error) {
	return s.repository.GetVariant(ctx, id)
}

func (s *Service) ListVariantsByProduct(ctx context.Context, productID string) ([]Variant, error) {
	return s.repository.ListVariantsByProduct(ctx, productID)
}

func (s *Service) DeleteVariant(ctx context.Context, id string) error {
	return s.repository.DeleteVariant(ctx, id)
}

func (s *Service) UpdateStock(ctx context.Context, id string, input UpdateStockInput) (Variant, error) {
	if input.StockOnHand < 0 {
		return Variant{}, fmt.Errorf("stock_on_hand must be zero or greater")
	}
	return s.repository.UpdateVariantStock(ctx, id, input.StockOnHand)
}

func (s *Service) SearchActive(ctx context.Context, query string, limit, offset int) ([]Product, error) {
	products, err := s.repository.ListProducts(ctx, ProductListFilter{
		Query:      strings.TrimSpace(query),
		ActiveOnly: true,
		Limit:      limit,
		Offset:     offset,
	})
	if err != nil {
		return nil, err
	}
	filtered := make([]Product, 0, len(products))
	for _, product := range products {
		if len(product.Variants) > 0 {
			filtered = append(filtered, product)
		}
	}
	return filtered, nil
}

func (s *Service) GetActiveByID(ctx context.Context, id string) (Product, error) {
	product, err := s.repository.GetProduct(ctx, id)
	if err != nil {
		return Product{}, err
	}
	if product.Status != "active" {
		return Product{}, ErrProductNotFound
	}
	product.Variants = filterActiveVariants(product.Variants)
	return product, nil
}

func (s *Service) GetActiveBySlug(ctx context.Context, slug string) (Product, error) {
	product, err := s.repository.GetProductBySlug(ctx, strings.TrimSpace(slug))
	if err != nil {
		return Product{}, err
	}
	if product.Status != "active" {
		return Product{}, ErrProductNotFound
	}
	product.Variants = filterActiveVariants(product.Variants)
	return product, nil
}

func slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.ReplaceAll(s, " ", "-")
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func validateStatus(status string) error {
	switch status {
	case "active", "inactive", "draft":
		return nil
	default:
		return fmt.Errorf("status must be active, inactive, or draft")
	}
}

func filterActiveVariants(variants []Variant) []Variant {
	filtered := make([]Variant, 0, len(variants))
	for _, variant := range variants {
		if variant.IsActive && variant.AvailableStock() > 0 {
			filtered = append(filtered, variant)
		}
	}
	return filtered
}
