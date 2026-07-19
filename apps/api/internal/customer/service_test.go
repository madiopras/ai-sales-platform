package customer

import (
	"context"
	"testing"

	"github.com/prasdios/ai-sales-platform/apps/api/internal/catalog"
)

func TestGetOpenCartCreatesWhenMissing(t *testing.T) {
	repo := &memoryRepository{
		customer: Customer{ID: "cust-1", Phone: "628123456789", Name: "Budi"},
	}
	service := NewService(repo, memoryCatalog{variant: catalog.Variant{ID: "var-1", Price: 99000, StockOnHand: 10}})
	cart, err := service.GetOpenCart(context.Background(), "cust-1")
	if err != nil {
		t.Fatal(err)
	}
	if cart.ID == "" {
		t.Fatal("expected cart id")
	}
	if cart.Status != CartStatusOpen {
		t.Fatalf("expected open cart, got %q", cart.Status)
	}
	if len(repo.carts) != 1 {
		t.Fatalf("expected one cart, got %d", len(repo.carts))
	}
}

func TestAddCartItemValidatesQuantity(t *testing.T) {
	repo := &memoryRepository{
		customer: Customer{ID: "cust-1", Phone: "628123456789"},
		carts:    map[string]Cart{"cart-1": {ID: "cart-1", CustomerID: "cust-1", Status: CartStatusOpen}},
	}
	service := NewService(repo, memoryCatalog{variant: catalog.Variant{ID: "var-1", Price: 50000, StockOnHand: 5}})
	_, err := service.AddCartItem(context.Background(), "cust-1", "var-1", 0)
	if err != ErrInvalidQuantity {
		t.Fatalf("expected ErrInvalidQuantity, got %v", err)
	}
}

func TestAddCartItemUsesVariantPrice(t *testing.T) {
	repo := &memoryRepository{
		customer: Customer{ID: "cust-1", Phone: "628123456789"},
		carts:    map[string]Cart{"cart-1": {ID: "cart-1", CustomerID: "cust-1", Status: CartStatusOpen}},
	}
	service := NewService(repo, memoryCatalog{variant: catalog.Variant{ID: "var-1", Price: 75000, StockOnHand: 5}})
	cart, err := service.AddCartItem(context.Background(), "cust-1", "var-1", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(cart.Items) != 1 {
		t.Fatalf("expected one item, got %d", len(cart.Items))
	}
	if cart.Items[0].UnitPrice != 75000 {
		t.Fatalf("expected unit price 75000, got %v", cart.Items[0].UnitPrice)
	}
}

func TestAddCartItemRejectsInsufficientStock(t *testing.T) {
	repo := &memoryRepository{
		customer: Customer{ID: "cust-1", Phone: "628123456789"},
		carts:    map[string]Cart{"cart-1": {ID: "cart-1", CustomerID: "cust-1", Status: CartStatusOpen}},
	}
	// Only 1 available (on_hand 3 - reserved 2) but requesting 2
	service := NewService(repo, memoryCatalog{variant: catalog.Variant{ID: "var-1", Price: 50000, StockOnHand: 3, StockReserved: 2}})
	_, err := service.AddCartItem(context.Background(), "cust-1", "var-1", 2)
	if err != catalog.ErrInsufficientStock {
		t.Fatalf("expected ErrInsufficientStock, got %v", err)
	}
}

func TestCreateAddressDefaultsLabel(t *testing.T) {
	repo := &memoryRepository{customer: Customer{ID: "cust-1", Phone: "628123456789"}}
	service := NewService(repo, memoryCatalog{})
	address, err := service.CreateAddress(context.Background(), "cust-1", CreateAddressInput{
		RecipientName: "Budi", Phone: "628123456789", AddressLine: "Jl. Mawar 1",
		City: "Jakarta", District: "Menteng", IsDefault: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if address.Label != "utama" {
		t.Fatalf("expected default label 'utama', got %q", address.Label)
	}
	if !address.IsDefault {
		t.Fatal("expected address to be default")
	}
}

func TestCreateAddressUnknownCustomer(t *testing.T) {
	repo := &memoryRepository{customer: Customer{ID: "cust-1", Phone: "628123456789"}}
	service := NewService(repo, memoryCatalog{})
	_, err := service.CreateAddress(context.Background(), "missing", CreateAddressInput{
		RecipientName: "Budi", Phone: "628", AddressLine: "x", City: "y", District: "z",
	})
	if err != ErrCustomerNotFound {
		t.Fatalf("expected ErrCustomerNotFound, got %v", err)
	}
}

type memoryRepository struct {
	customer Customer
	carts    map[string]Cart
	items    map[string][]CartItem
}

func (r *memoryRepository) UpsertByPhone(_ context.Context, phone, name string) (Customer, error) {
	r.customer = Customer{ID: "cust-1", Phone: phone, Name: name}
	return r.customer, nil
}

func (r *memoryRepository) GetByID(_ context.Context, id string) (Customer, error) {
	if r.customer.ID == id {
		return r.customer, nil
	}
	return Customer{}, ErrCustomerNotFound
}

func (r *memoryRepository) List(_ context.Context, _, _ int) ([]Customer, error) {
	return []Customer{r.customer}, nil
}

func (r *memoryRepository) ListAddresses(context.Context, string) ([]Address, error) {
	return nil, nil
}

func (r *memoryRepository) GetDefaultAddress(context.Context, string) (Address, error) {
	return Address{}, ErrAddressNotFound
}

func (r *memoryRepository) CreateAddress(_ context.Context, address Address) (Address, error) {
	address.ID = "addr-1"
	return address, nil
}

func (r *memoryRepository) GetOpenCart(_ context.Context, customerID string) (Cart, error) {
	if r.carts == nil {
		return Cart{}, ErrCartNotFound
	}
	for _, cart := range r.carts {
		if cart.CustomerID == customerID && cart.Status == CartStatusOpen {
			return cart, nil
		}
	}
	return Cart{}, ErrCartNotFound
}

func (r *memoryRepository) CreateOpenCart(_ context.Context, customerID string) (Cart, error) {
	if r.carts == nil {
		r.carts = make(map[string]Cart)
	}
	cart := Cart{ID: "cart-new", CustomerID: customerID, Status: CartStatusOpen}
	r.carts[cart.ID] = cart
	return cart, nil
}

func (r *memoryRepository) ListCartItems(_ context.Context, cartID string) ([]CartItem, error) {
	if r.items == nil {
		return nil, nil
	}
	return r.items[cartID], nil
}

func (r *memoryRepository) UpsertCartItem(_ context.Context, item CartItem) (CartItem, error) {
	if r.items == nil {
		r.items = make(map[string][]CartItem)
	}
	item.ID = "item-1"
	r.items[item.CartID] = []CartItem{item}
	return item, nil
}

func (r *memoryRepository) UpdateCartItem(_ context.Context, cartID, variantID string, qty int) (CartItem, error) {
	items := r.items[cartID]
	for i, item := range items {
		if item.VariantID == variantID {
			items[i].Qty = qty
			return items[i], nil
		}
	}
	return CartItem{}, ErrCartItemNotFound
}

func (r *memoryRepository) RemoveCartItem(_ context.Context, cartID, variantID string) error {
	items := r.items[cartID]
	for i, item := range items {
		if item.VariantID == variantID {
			r.items[cartID] = append(items[:i], items[i+1:]...)
			return nil
		}
	}
	return ErrCartItemNotFound
}

type memoryCatalog struct{ variant catalog.Variant }

func (c memoryCatalog) GetVariant(_ context.Context, id string) (catalog.Variant, error) {
	if c.variant.ID == id {
		return c.variant, nil
	}
	return catalog.Variant{}, catalog.ErrVariantNotFound
}

func (memoryCatalog) CreateCategory(context.Context, catalog.Category) (catalog.Category, error) {
	return catalog.Category{}, nil
}
func (memoryCatalog) UpdateCategory(context.Context, catalog.Category) (catalog.Category, error) {
	return catalog.Category{}, nil
}
func (memoryCatalog) GetCategory(context.Context, string) (catalog.Category, error) {
	return catalog.Category{}, nil
}
func (memoryCatalog) ListCategories(context.Context) ([]catalog.Category, error) { return nil, nil }
func (memoryCatalog) DeleteCategory(context.Context, string) error               { return nil }
func (memoryCatalog) CreateProduct(context.Context, catalog.Product) (catalog.Product, error) {
	return catalog.Product{}, nil
}
func (memoryCatalog) UpdateProduct(context.Context, catalog.Product) (catalog.Product, error) {
	return catalog.Product{}, nil
}
func (memoryCatalog) GetProduct(context.Context, string) (catalog.Product, error) {
	return catalog.Product{}, nil
}
func (memoryCatalog) GetProductBySlug(context.Context, string) (catalog.Product, error) {
	return catalog.Product{}, nil
}
func (memoryCatalog) ListProducts(context.Context, catalog.ProductListFilter) ([]catalog.Product, error) {
	return nil, nil
}
func (memoryCatalog) DeleteProduct(context.Context, string) error { return nil }
func (memoryCatalog) CreateVariant(context.Context, catalog.Variant) (catalog.Variant, error) {
	return catalog.Variant{}, nil
}
func (memoryCatalog) UpdateVariant(context.Context, catalog.Variant) (catalog.Variant, error) {
	return catalog.Variant{}, nil
}
func (memoryCatalog) ListVariantsByProduct(context.Context, string) ([]catalog.Variant, error) {
	return nil, nil
}
func (memoryCatalog) DeleteVariant(context.Context, string) error { return nil }
func (memoryCatalog) UpdateVariantStock(context.Context, string, int) (catalog.Variant, error) {
	return catalog.Variant{}, nil
}
func (memoryCatalog) ReserveStock(context.Context, string, int) error { return nil }
func (memoryCatalog) ReleaseStock(context.Context, string, int) error { return nil }
func (memoryCatalog) CommitStock(context.Context, string, int) error  { return nil }
