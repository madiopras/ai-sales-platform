package order

import (
	"context"
	"testing"

	"github.com/prasdios/ai-sales-platform/apps/api/internal/catalog"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/customer"
)

func newTestService(cart customer.Cart, address customer.Address, variant catalog.Variant) (*Service, *memoryRepo) {
	repo := &memoryRepo{}
	carts := memoryCarts{cart: cart, address: address}
	catalogReader := memoryCatalog{variant: variant, product: catalog.Product{ID: variant.ProductID, Name: "Kaos Polos"}}
	return NewService(repo, carts, catalogReader), repo
}

func fullCart() customer.Cart {
	return customer.Cart{
		ID:         "cart-1",
		CustomerID: "cust-1",
		Status:     customer.CartStatusOpen,
		Items: []customer.CartItem{
			{ID: "ci-1", CartID: "cart-1", VariantID: "var-1", Qty: 2, UnitPrice: 50000},
		},
	}
}

func defaultAddress() customer.Address {
	return customer.Address{
		RecipientName: "Budi", Phone: "628123", AddressLine: "Jl. Mawar 1",
		City: "Jakarta", District: "Menteng",
	}
}

func activeVariant() catalog.Variant {
	return catalog.Variant{ID: "var-1", ProductID: "prod-1", SKU: "SKU-1", Name: "Merah - M", Price: 50000, StockOnHand: 10, IsActive: true}
}

func courier() Courier { return Courier{Code: "jne", Service: "REG", Fee: 15000} }

func TestPreviewComputesTotals(t *testing.T) {
	service, _ := newTestService(fullCart(), defaultAddress(), activeVariant())
	preview, err := service.Preview(context.Background(), CheckoutInput{CustomerID: "cust-1", Courier: courier()})
	if err != nil {
		t.Fatal(err)
	}
	if preview.Subtotal != 100000 {
		t.Fatalf("expected subtotal 100000, got %v", preview.Subtotal)
	}
	if preview.Total != 115000 {
		t.Fatalf("expected total 115000, got %v", preview.Total)
	}
	if len(preview.Items) != 1 || preview.Items[0].ProductName != "Kaos Polos" {
		t.Fatalf("expected snapshotted item, got %+v", preview.Items)
	}
}

func TestPreviewUsesDefaultAddressWhenOmitted(t *testing.T) {
	service, _ := newTestService(fullCart(), defaultAddress(), activeVariant())
	preview, err := service.Preview(context.Background(), CheckoutInput{CustomerID: "cust-1", Courier: courier()})
	if err != nil {
		t.Fatal(err)
	}
	if preview.Address.RecipientName != "Budi" {
		t.Fatalf("expected default address, got %+v", preview.Address)
	}
}

func TestConfirmRejectsEmptyCart(t *testing.T) {
	empty := fullCart()
	empty.Items = nil
	service, _ := newTestService(empty, defaultAddress(), activeVariant())
	_, err := service.Confirm(context.Background(), CheckoutInput{CustomerID: "cust-1", Courier: courier()})
	if err != ErrEmptyCart {
		t.Fatalf("expected ErrEmptyCart, got %v", err)
	}
}

func TestConfirmRequiresCourier(t *testing.T) {
	service, _ := newTestService(fullCart(), defaultAddress(), activeVariant())
	_, err := service.Confirm(context.Background(), CheckoutInput{CustomerID: "cust-1"})
	if err != ErrCourierRequired {
		t.Fatalf("expected ErrCourierRequired, got %v", err)
	}
}

func TestConfirmRequiresCompleteAddress(t *testing.T) {
	service, _ := newTestService(fullCart(), customer.Address{}, activeVariant())
	_, err := service.Confirm(context.Background(), CheckoutInput{CustomerID: "cust-1", Courier: courier()})
	if err != ErrAddressRequired {
		t.Fatalf("expected ErrAddressRequired, got %v", err)
	}
}

func TestConfirmRejectsInsufficientStock(t *testing.T) {
	variant := activeVariant()
	variant.StockOnHand = 3
	variant.StockReserved = 2 // only 1 available, need 2
	service, _ := newTestService(fullCart(), defaultAddress(), variant)
	_, err := service.Confirm(context.Background(), CheckoutInput{CustomerID: "cust-1", Courier: courier()})
	if err != catalog.ErrInsufficientStock {
		t.Fatalf("expected ErrInsufficientStock, got %v", err)
	}
}

func TestConfirmPersistsOrder(t *testing.T) {
	service, repo := newTestService(fullCart(), defaultAddress(), activeVariant())
	order, err := service.Confirm(context.Background(), CheckoutInput{CustomerID: "cust-1", Courier: courier()})
	if err != nil {
		t.Fatal(err)
	}
	if order.Status != StatusPendingPayment {
		t.Fatalf("expected pending_payment, got %q", order.Status)
	}
	if order.Total != 115000 {
		t.Fatalf("expected total 115000, got %v", order.Total)
	}
	if repo.closedCartID != "cart-1" {
		t.Fatalf("expected cart-1 to be closed, got %q", repo.closedCartID)
	}
	if len(order.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(order.Items))
	}
}

func TestUpdateStatusValidatesTransition(t *testing.T) {
	service, repo := newTestService(fullCart(), defaultAddress(), activeVariant())
	repo.orders = map[string]Order{"ord-1": {ID: "ord-1", Status: StatusPendingPayment}}
	// pending_payment -> shipped is not allowed
	_, err := service.UpdateStatus(context.Background(), "ord-1", StatusShipped)
	if err != ErrStatusTransition {
		t.Fatalf("expected ErrStatusTransition, got %v", err)
	}
}

func TestUpdateStatusRejectsUnknownStatus(t *testing.T) {
	service, repo := newTestService(fullCart(), defaultAddress(), activeVariant())
	repo.orders = map[string]Order{"ord-1": {ID: "ord-1", Status: StatusPendingPayment}}
	_, err := service.UpdateStatus(context.Background(), "ord-1", "banana")
	if err != ErrInvalidStatus {
		t.Fatalf("expected ErrInvalidStatus, got %v", err)
	}
}

func TestUpdateStatusAllowsValidTransition(t *testing.T) {
	service, repo := newTestService(fullCart(), defaultAddress(), activeVariant())
	repo.orders = map[string]Order{"ord-1": {ID: "ord-1", Status: StatusPendingPayment}}
	updated, err := service.UpdateStatus(context.Background(), "ord-1", StatusPaid)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != StatusPaid {
		t.Fatalf("expected paid, got %q", updated.Status)
	}
}

// --- test doubles ---

type memoryRepo struct {
	orders       map[string]Order
	closedCartID string
	lastOrder    Order
}

func (r *memoryRepo) CreateFromCheckout(_ context.Context, o Order, cartID string) (Order, error) {
	o.ID = "ord-new"
	r.closedCartID = cartID
	r.lastOrder = o
	if r.orders == nil {
		r.orders = make(map[string]Order)
	}
	r.orders[o.ID] = o
	return o, nil
}

func (r *memoryRepo) GetByID(_ context.Context, id string) (Order, error) {
	if o, ok := r.orders[id]; ok {
		return o, nil
	}
	return Order{}, ErrOrderNotFound
}

func (r *memoryRepo) List(context.Context, ListFilter) ([]Order, error) { return nil, nil }

func (r *memoryRepo) UpdateStatus(_ context.Context, id, status string) (Order, error) {
	o := r.orders[id]
	o.Status = status
	r.orders[id] = o
	return o, nil
}

type memoryCarts struct {
	cart    customer.Cart
	address customer.Address
}

func (c memoryCarts) GetOpenCart(_ context.Context, customerID string) (customer.Cart, error) {
	if c.cart.CustomerID == customerID {
		return c.cart, nil
	}
	return customer.Cart{}, customer.ErrCartNotFound
}

func (c memoryCarts) GetDefaultAddress(_ context.Context, _ string) (customer.Address, error) {
	if c.address.RecipientName == "" {
		return customer.Address{}, customer.ErrAddressNotFound
	}
	return c.address, nil
}

type memoryCatalog struct {
	variant catalog.Variant
	product catalog.Product
}

func (c memoryCatalog) GetVariant(_ context.Context, id string) (catalog.Variant, error) {
	if c.variant.ID == id {
		return c.variant, nil
	}
	return catalog.Variant{}, catalog.ErrVariantNotFound
}

func (c memoryCatalog) GetProduct(_ context.Context, id string) (catalog.Product, error) {
	if c.product.ID == id {
		return c.product, nil
	}
	return catalog.Product{}, catalog.ErrProductNotFound
}
