package promo

import "context"

// Repository is the persistence boundary for the promo domain.
//
// Contracts:
//   - Create / Update / Delete / GetByID / List: admin CRUD (BR-048).
//   - GetByCode: case-insensitive lookup used by checkout validation (BR-038).
//   - Redeem: atomically increments used_count while re-checking quota so two
//     concurrent checkouts can't push a limited voucher past its quota. Returns
//     ErrVoucherExhausted when no quota remains.
type Repository interface {
	Create(ctx context.Context, voucher Voucher) (Voucher, error)
	Update(ctx context.Context, id string, voucher Voucher) (Voucher, error)
	Delete(ctx context.Context, id string) error

	GetByID(ctx context.Context, id string) (Voucher, error)
	GetByCode(ctx context.Context, code string) (Voucher, error)
	List(ctx context.Context, filter ListFilter) ([]Voucher, error)

	Redeem(ctx context.Context, id string) (Voucher, error)
}
