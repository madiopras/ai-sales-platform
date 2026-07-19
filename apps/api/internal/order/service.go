package order

import (
	"context"
	"strings"

	"github.com/prasdios/ai-sales-platform/apps/api/internal/catalog"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/customer"
)

// CartReader exposes the customer/cart data checkout needs. Satisfied by *customer.Service.
type CartReader interface {
	GetOpenCart(ctx context.Context, customerID string) (customer.Cart, error)
	GetDefaultAddress(ctx context.Context, customerID string) (customer.Address, error)
}

// CatalogReader resolves variant + product snapshots. Satisfied by catalog.Repository.
type CatalogReader interface {
	GetVariant(ctx context.Context, id string) (catalog.Variant, error)
	GetProduct(ctx context.Context, id string) (catalog.Product, error)
}

// VoucherValidator validates a voucher against a subtotal without consuming it
// (used by Preview), and Redeem consumes one unit of quota (used by Confirm).
// Satisfied by *promo.Service. Optional: a nil validator disables promos.
type VoucherValidator interface {
	Validate(ctx context.Context, code string, subtotal float64) (VoucherResult, error)
	Redeem(ctx context.Context, code string, subtotal float64) (VoucherResult, error)
}

// VoucherResult is the discount outcome the order package consumes. It mirrors
// promo.ApplyResult; an adapter in the container bridges the two so the order
// package doesn't import promo directly.
type VoucherResult struct {
	Code     string
	Discount float64
}

type Service struct {
	repository Repository
	carts      CartReader
	catalog    CatalogReader
	vouchers   VoucherValidator
}

func NewService(repository Repository, carts CartReader, catalogReader CatalogReader) *Service {
	return &Service{repository: repository, carts: carts, catalog: catalogReader}
}

// SetVoucherValidator wires voucher validation/redemption into checkout (BR-037..BR-038).
// Optional; when unset a voucher code on the input is ignored.
func (s *Service) SetVoucherValidator(v VoucherValidator) { s.vouchers = v }

// CheckoutInput drives both preview and confirm. Address is optional; when omitted
// the customer's default address is used. Courier is stubbed until the shipping phase.
// VoucherCode is optional; when set and a validator is wired, the discount is applied.
type CheckoutInput struct {
	CustomerID  string
	Address     *AddressSnap
	Courier     Courier
	VoucherCode string
}

// Preview builds a non-persisted checkout summary: validates the cart, resolves the
// address + courier, and prices items from the current catalog. No stock is reserved.
func (s *Service) Preview(ctx context.Context, input CheckoutInput) (CheckoutPreview, error) {
	cart, address, courier, items, err := s.assemble(ctx, input)
	if err != nil {
		return CheckoutPreview{}, err
	}
	_ = cart
	subtotal := 0.0
	for _, item := range items {
		subtotal += item.Subtotal
	}
	// Apply a voucher for the preview without consuming quota (BR-037..BR-038).
	voucher, err := s.applyVoucher(ctx, input.VoucherCode, subtotal, false)
	if err != nil {
		return CheckoutPreview{}, err
	}
	return CheckoutPreview{
		CustomerID:  input.CustomerID,
		Address:     address,
		Courier:     courier,
		Items:       items,
		VoucherCode: voucher.Code,
		Discount:    voucher.Discount,
		Subtotal:    subtotal,
		ShippingFee: courier.Fee,
		Total:       subtotal - voucher.Discount + courier.Fee,
	}, nil
}

// Confirm creates the order, reserves stock, and closes the cart in one transaction.
func (s *Service) Confirm(ctx context.Context, input CheckoutInput) (Order, error) {
	cart, address, courier, items, err := s.assemble(ctx, input)
	if err != nil {
		return Order{}, err
	}
	subtotal := 0.0
	for _, item := range items {
		subtotal += item.Subtotal
	}
	// Redeem the voucher: consumes one unit of quota and snapshots the discount
	// onto the order (BR-037..BR-038).
	voucher, err := s.applyVoucher(ctx, input.VoucherCode, subtotal, true)
	if err != nil {
		return Order{}, err
	}
	order := Order{
		OrderNo:        generateOrderNo(),
		CustomerID:     input.CustomerID,
		Status:         StatusPendingPayment,
		RecipientName:  address.RecipientName,
		Phone:          address.Phone,
		AddressLine:    address.AddressLine,
		City:           address.City,
		District:       address.District,
		PostalCode:     address.PostalCode,
		Notes:          address.Notes,
		CourierCode:    courier.Code,
		CourierService: courier.Service,
		VoucherCode:    voucher.Code,
		Discount:       voucher.Discount,
		Subtotal:       subtotal,
		ShippingFee:    courier.Fee,
		Total:          subtotal - voucher.Discount + courier.Fee,
		Items:          items,
	}
	return s.repository.CreateFromCheckout(ctx, order, cart.ID)
}

// applyVoucher resolves the discount for a voucher code. When redeem is true it
// consumes one unit of quota (Confirm); otherwise it only validates (Preview).
// An empty code or an unwired validator is a no-op returning a zero discount.
func (s *Service) applyVoucher(ctx context.Context, code string, subtotal float64, redeem bool) (VoucherResult, error) {
	code = strings.TrimSpace(code)
	if code == "" || s.vouchers == nil {
		return VoucherResult{}, nil
	}
	if redeem {
		return s.vouchers.Redeem(ctx, code, subtotal)
	}
	return s.vouchers.Validate(ctx, code, subtotal)
}

func (s *Service) GetOrder(ctx context.Context, id string) (Order, error) {
	return s.repository.GetByID(ctx, id)
}

func (s *Service) ListOrders(ctx context.Context, filter ListFilter) ([]Order, error) {
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	return s.repository.List(ctx, filter)
}

// UpdateStatus applies an admin-driven fulfillment transition (BR-026).
func (s *Service) UpdateStatus(ctx context.Context, id, status string) (Order, error) {
	status = strings.TrimSpace(status)
	if !isValidStatus(status) {
		return Order{}, ErrInvalidStatus
	}
	current, err := s.repository.GetByID(ctx, id)
	if err != nil {
		return Order{}, err
	}
	if current.Status == status {
		return current, nil
	}
	if !canTransition(current.Status, status) {
		return Order{}, ErrStatusTransition
	}
	return s.repository.UpdateStatus(ctx, id, status)
}

// assemble validates the cart, resolves address + courier, and builds priced items.
func (s *Service) assemble(ctx context.Context, input CheckoutInput) (customer.Cart, AddressSnap, Courier, []OrderItem, error) {
	cart, err := s.carts.GetOpenCart(ctx, input.CustomerID)
	if err != nil {
		return customer.Cart{}, AddressSnap{}, Courier{}, nil, err
	}
	if len(cart.Items) == 0 {
		return customer.Cart{}, AddressSnap{}, Courier{}, nil, ErrEmptyCart
	}

	address, err := s.resolveAddress(ctx, input)
	if err != nil {
		return customer.Cart{}, AddressSnap{}, Courier{}, nil, err
	}
	if !address.isComplete() {
		return customer.Cart{}, AddressSnap{}, Courier{}, nil, ErrAddressRequired
	}

	courier := input.Courier
	courier.Code = strings.TrimSpace(courier.Code)
	courier.Service = strings.TrimSpace(courier.Service)
	if courier.Code == "" {
		return customer.Cart{}, AddressSnap{}, Courier{}, nil, ErrCourierRequired
	}
	if courier.Fee < 0 {
		courier.Fee = 0
	}

	items, err := s.buildItems(ctx, cart.Items)
	if err != nil {
		return customer.Cart{}, AddressSnap{}, Courier{}, nil, err
	}
	return cart, address, courier, items, nil
}

func (s *Service) resolveAddress(ctx context.Context, input CheckoutInput) (AddressSnap, error) {
	if input.Address != nil {
		a := *input.Address
		a.RecipientName = strings.TrimSpace(a.RecipientName)
		a.Phone = strings.TrimSpace(a.Phone)
		a.AddressLine = strings.TrimSpace(a.AddressLine)
		a.City = strings.TrimSpace(a.City)
		a.District = strings.TrimSpace(a.District)
		a.PostalCode = strings.TrimSpace(a.PostalCode)
		a.Notes = strings.TrimSpace(a.Notes)
		return a, nil
	}
	addr, err := s.carts.GetDefaultAddress(ctx, input.CustomerID)
	if err != nil {
		return AddressSnap{}, ErrAddressRequired
	}
	return AddressSnap{
		RecipientName: addr.RecipientName,
		Phone:         addr.Phone,
		AddressLine:   addr.AddressLine,
		City:          addr.City,
		District:      addr.District,
		PostalCode:    addr.PostalCode,
		Notes:         addr.Notes,
	}, nil
}

// buildItems re-prices each cart line from the current catalog and validates
// availability. Product/variant names are snapshotted for order history.
func (s *Service) buildItems(ctx context.Context, cartItems []customer.CartItem) ([]OrderItem, error) {
	items := make([]OrderItem, 0, len(cartItems))
	for _, ci := range cartItems {
		variant, err := s.catalog.GetVariant(ctx, ci.VariantID)
		if err != nil {
			return nil, err
		}
		if !variant.IsActive || variant.AvailableStock() < ci.Qty {
			return nil, catalog.ErrInsufficientStock
		}
		productName := ""
		if product, err := s.catalog.GetProduct(ctx, variant.ProductID); err == nil {
			productName = product.Name
		}
		items = append(items, OrderItem{
			VariantID:   variant.ID,
			SKU:         variant.SKU,
			ProductName: productName,
			VariantName: variant.Name,
			Qty:         ci.Qty,
			UnitPrice:   variant.Price,
			Subtotal:    variant.Price * float64(ci.Qty),
		})
	}
	return items, nil
}
