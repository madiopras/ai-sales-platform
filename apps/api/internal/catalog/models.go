package catalog

import (
	"errors"
	"time"
)

// Error types
var (
	ErrCategoryNotFound  = errors.New("category not found")
	ErrProductNotFound   = errors.New("product not found")
	ErrVariantNotFound   = errors.New("variant not found")
	ErrSlugTaken         = errors.New("slug already taken")
	ErrSKUTaken          = errors.New("sku already taken")
	ErrInsufficientStock = errors.New("insufficient stock")
)

type Category struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Product struct {
	ID          string    `json:"id"`
	CategoryID  string    `json:"category_id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description"`
	Status      string    `json:"status"` // active, inactive, draft
	Variants    []Variant `json:"variants"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ProductWithVariants struct {
	Product  `json:"inline"`
	Variants []ProductVariant `json:"variants"`
}

type ProductVariant struct {
	ID        string    `json:"id"`
	ProductID string    `json:"product_id"`
	SKU       string    `json:"sku"`
	Name      string    `json:"name"`
	Price     float64   `json:"price"`
	Stock     int       `json:"stock"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Request DTOs
type CreateCategoryRequest struct {
	Name     string `json:"name" binding:"required,min=1,max=255"`
	Slug     string `json:"slug" binding:"required,min=1,max=255"`
	IsActive *bool  `json:"is_active"`
}

type UpdateCategoryRequest struct {
	Name     string `json:"name" binding:"required,min=1,max=255"`
	Slug     string `json:"slug" binding:"required,min=1,max=255"`
	IsActive *bool  `json:"is_active"`
}

type CreateProductRequest struct {
	CategoryID  string `json:"category_id" binding:"required"`
	Name        string `json:"name" binding:"required,min=1,max=255"`
	Slug        string `json:"slug" binding:"required,min=1,max=255"`
	Description string `json:"description"`
	Status      string `json:"status" binding:"required,oneof=active inactive draft"`
}

type UpdateProductRequest struct {
	CategoryID  string `json:"category_id" binding:"required"`
	Name        string `json:"name" binding:"required,min=1,max=255"`
	Slug        string `json:"slug" binding:"required,min=1,max=255"`
	Description string `json:"description"`
	Status      string `json:"status" binding:"required,oneof=active inactive draft"`
}

type CreateVariantRequest struct {
	SKU      string  `json:"sku" binding:"required,min=1,max=255"`
	Name     string  `json:"name" binding:"required,min=1,max=255"`
	Price    float64 `json:"price" binding:"required,gt=0"`
	Stock    int     `json:"stock" binding:"required,gte=0"`
	IsActive *bool   `json:"is_active"`
}

type UpdateVariantRequest struct {
	SKU      string  `json:"sku" binding:"required,min=1,max=255"`
	Name     string  `json:"name" binding:"required,min=1,max=255"`
	Price    float64 `json:"price" binding:"required,gt=0"`
	Stock    int     `json:"stock" binding:"required,gte=0"`
	IsActive *bool   `json:"is_active"`
}

type SearchProductsQuery struct {
	Query  string
	Limit  int
	Offset int
}

// Variant type (for stock management)
type Variant struct {
	ID            string    `json:"id"`
	ProductID     string    `json:"product_id"`
	SKU           string    `json:"sku"`
	Name          string    `json:"name"`
	Price         float64   `json:"price"`
	StockOnHand   int       `json:"stock_on_hand"`
	StockReserved int       `json:"stock_reserved"`
	IsActive      bool      `json:"is_active"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// AvailableStock returns stock available for purchase
func (v *Variant) AvailableStock() int {
	return v.StockOnHand - v.StockReserved
}

// ProductListFilter for querying products
type ProductListFilter struct {
	Status     string
	CategoryID string
	Query      string
	ActiveOnly bool
	Limit      int
	Offset     int
}
