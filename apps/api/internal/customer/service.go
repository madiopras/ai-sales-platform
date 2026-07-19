package customer

import (
	"context"
	"errors"
	"strings"

	"github.com/prasdios/ai-sales-platform/apps/api/internal/catalog"
)

type Service struct {
	repository Repository
	catalog    catalog.Repository
}

type UpsertCustomerInput struct {
	Phone string
	Name  string
}

type CreateAddressInput struct {
	Label         string
	RecipientName string
	Phone         string
	AddressLine   string
	City          string
	District      string
	PostalCode    string
	Notes         string
	IsDefault     bool
}

func NewService(repository Repository, catalogRepo catalog.Repository) *Service {
	return &Service{repository: repository, catalog: catalogRepo}
}

func (s *Service) UpsertCustomerByPhone(ctx context.Context, input UpsertCustomerInput) (Customer, error) {
	return s.repository.UpsertByPhone(ctx, strings.TrimSpace(input.Phone), strings.TrimSpace(input.Name))
}

func (s *Service) GetCustomer(ctx context.Context, id string) (Customer, error) {
	return s.repository.GetByID(ctx, id)
}

func (s *Service) ListCustomers(ctx context.Context, limit, offset int) ([]Customer, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return s.repository.List(ctx, limit, offset)
}

func (s *Service) ListAddresses(ctx context.Context, customerID string) ([]Address, error) {
	return s.repository.ListAddresses(ctx, customerID)
}

func (s *Service) GetDefaultAddress(ctx context.Context, customerID string) (Address, error) {
	return s.repository.GetDefaultAddress(ctx, customerID)
}

func (s *Service) CreateAddress(ctx context.Context, customerID string, input CreateAddressInput) (Address, error) {
	if _, err := s.repository.GetByID(ctx, customerID); err != nil {
		return Address{}, err
	}
	label := strings.TrimSpace(input.Label)
	if label == "" {
		label = "utama"
	}
	return s.repository.CreateAddress(ctx, Address{
		CustomerID:    customerID,
		Label:         label,
		RecipientName: strings.TrimSpace(input.RecipientName),
		Phone:         strings.TrimSpace(input.Phone),
		AddressLine:   strings.TrimSpace(input.AddressLine),
		City:          strings.TrimSpace(input.City),
		District:      strings.TrimSpace(input.District),
		PostalCode:    strings.TrimSpace(input.PostalCode),
		Notes:         strings.TrimSpace(input.Notes),
		IsDefault:     input.IsDefault,
	})
}

func (s *Service) GetOpenCart(ctx context.Context, customerID string) (Cart, error) {
	cart, err := s.getOrCreateOpenCart(ctx, customerID)
	if err != nil {
		return Cart{}, err
	}
	items, err := s.repository.ListCartItems(ctx, cart.ID)
	if err != nil {
		return Cart{}, err
	}
	cart.Items = items
	return cart, nil
}

func (s *Service) AddCartItem(ctx context.Context, customerID, variantID string, qty int) (Cart, error) {
	if qty <= 0 {
		return Cart{}, ErrInvalidQuantity
	}
	variant, err := s.catalog.GetVariant(ctx, variantID)
	if err != nil {
		return Cart{}, err
	}
	if variant.AvailableStock() < qty {
		return Cart{}, catalog.ErrInsufficientStock
	}
	cart, err := s.getOrCreateOpenCart(ctx, customerID)
	if err != nil {
		return Cart{}, err
	}
	if _, err := s.repository.UpsertCartItem(ctx, CartItem{
		CartID:    cart.ID,
		VariantID: variantID,
		Qty:       qty,
		UnitPrice: variant.Price,
	}); err != nil {
		return Cart{}, err
	}
	return s.GetOpenCart(ctx, customerID)
}

func (s *Service) UpdateCartItem(ctx context.Context, customerID, variantID string, qty int) (Cart, error) {
	if qty <= 0 {
		return Cart{}, ErrInvalidQuantity
	}
	variant, err := s.catalog.GetVariant(ctx, variantID)
	if err != nil {
		return Cart{}, err
	}
	if variant.AvailableStock() < qty {
		return Cart{}, catalog.ErrInsufficientStock
	}
	cart, err := s.repository.GetOpenCart(ctx, customerID)
	if err != nil {
		return Cart{}, err
	}
	if _, err := s.repository.UpdateCartItem(ctx, cart.ID, variantID, qty); err != nil {
		return Cart{}, err
	}
	return s.GetOpenCart(ctx, customerID)
}

func (s *Service) RemoveCartItem(ctx context.Context, customerID, variantID string) (Cart, error) {
	cart, err := s.repository.GetOpenCart(ctx, customerID)
	if err != nil {
		return Cart{}, err
	}
	if err := s.repository.RemoveCartItem(ctx, cart.ID, variantID); err != nil {
		return Cart{}, err
	}
	return s.GetOpenCart(ctx, customerID)
}

func (s *Service) getOrCreateOpenCart(ctx context.Context, customerID string) (Cart, error) {
	if _, err := s.repository.GetByID(ctx, customerID); err != nil {
		return Cart{}, err
	}
	cart, err := s.repository.GetOpenCart(ctx, customerID)
	if err == nil {
		return cart, nil
	}
	if !errors.Is(err, ErrCartNotFound) {
		return Cart{}, err
	}
	return s.repository.CreateOpenCart(ctx, customerID)
}
