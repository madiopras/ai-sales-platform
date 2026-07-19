package customer

import (
	"context"
	"errors"
)

var (
	ErrCustomerNotFound   = errors.New("customer not found")
	ErrAddressNotFound    = errors.New("address not found")
	ErrCartNotFound       = errors.New("cart not found")
	ErrCartItemNotFound   = errors.New("cart item not found")
	ErrInvalidQuantity    = errors.New("quantity must be greater than zero")
)

type Repository interface {
	UpsertByPhone(ctx context.Context, phone, name string) (Customer, error)
	GetByID(ctx context.Context, id string) (Customer, error)
	List(ctx context.Context, limit, offset int) ([]Customer, error)

	ListAddresses(ctx context.Context, customerID string) ([]Address, error)
	GetDefaultAddress(ctx context.Context, customerID string) (Address, error)
	CreateAddress(ctx context.Context, address Address) (Address, error)

	GetOpenCart(ctx context.Context, customerID string) (Cart, error)
	CreateOpenCart(ctx context.Context, customerID string) (Cart, error)
	ListCartItems(ctx context.Context, cartID string) ([]CartItem, error)
	UpsertCartItem(ctx context.Context, item CartItem) (CartItem, error)
	UpdateCartItem(ctx context.Context, cartID, variantID string, qty int) (CartItem, error)
	RemoveCartItem(ctx context.Context, cartID, variantID string) error
}
