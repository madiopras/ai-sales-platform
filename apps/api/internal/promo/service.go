package promo

import (
	"context"
	"strings"
	"time"
)

// Service orchestrates voucher CRUD (admin) and voucher validation/redemption
// used by checkout.
type Service struct {
	repository Repository
	now        func() time.Time
}

// NewService returns a Service that uses time.Now() for window checks.
func NewService(repository Repository) *Service {
	return &Service{repository: repository, now: time.Now}
}

// CreateVoucher persists a new voucher (BR-048). Codes are stored upper-cased and
// must be unique (case-insensitive).
func (s *Service) CreateVoucher(ctx context.Context, input CreateVoucherInput) (Voucher, error) {
	code := normaliseCode(input.Code)
	if code == "" {
		return Voucher{}, ErrValidation
	}
	if !validDiscountType(input.DiscountType) {
		return Voucher{}, ErrInvalidDiscount
	}
	if err := validateDiscountValue(input.DiscountType, input.DiscountValue); err != nil {
		return Voucher{}, err
	}
	if input.ExpiresAt.IsZero() {
		return Voucher{}, ErrValidation
	}

	startsAt := s.now()
	if input.StartsAt != nil {
		startsAt = *input.StartsAt
	}
	isActive := true
	if input.IsActive != nil {
		isActive = *input.IsActive
	}

	voucher := Voucher{
		Code:           code,
		Description:    strings.TrimSpace(input.Description),
		DiscountType:   input.DiscountType,
		DiscountValue:  input.DiscountValue,
		MinOrderAmount: nonNeg(input.MinOrderAmount),
		Quota:          maxInt(input.Quota, 0),
		StartsAt:       startsAt,
		ExpiresAt:      input.ExpiresAt,
		IsActive:       isActive,
	}
	return s.repository.Create(ctx, voucher)
}

// UpdateVoucher replaces the editable fields of an existing voucher. The code and
// used_count are immutable through this path.
func (s *Service) UpdateVoucher(ctx context.Context, id string, input UpdateVoucherInput) (Voucher, error) {
	existing, err := s.repository.GetByID(ctx, id)
	if err != nil {
		return Voucher{}, err
	}
	if !validDiscountType(input.DiscountType) {
		return Voucher{}, ErrInvalidDiscount
	}
	if err := validateDiscountValue(input.DiscountType, input.DiscountValue); err != nil {
		return Voucher{}, err
	}
	if input.ExpiresAt.IsZero() {
		return Voucher{}, ErrValidation
	}

	existing.Description = strings.TrimSpace(input.Description)
	existing.DiscountType = input.DiscountType
	existing.DiscountValue = input.DiscountValue
	existing.MinOrderAmount = nonNeg(input.MinOrderAmount)
	existing.Quota = maxInt(input.Quota, 0)
	if input.StartsAt != nil {
		existing.StartsAt = *input.StartsAt
	}
	existing.ExpiresAt = input.ExpiresAt
	if input.IsActive != nil {
		existing.IsActive = *input.IsActive
	}
	return s.repository.Update(ctx, id, existing)
}

// DeleteVoucher removes a voucher by id.
func (s *Service) DeleteVoucher(ctx context.Context, id string) error {
	return s.repository.Delete(ctx, strings.TrimSpace(id))
}

// GetVoucher fetches a voucher by internal id.
func (s *Service) GetVoucher(ctx context.Context, id string) (Voucher, error) {
	return s.repository.GetByID(ctx, strings.TrimSpace(id))
}

// GetVoucherByCode exposes a case-insensitive lookup (AI may want to describe a
// voucher the customer typed).
func (s *Service) GetVoucherByCode(ctx context.Context, code string) (Voucher, error) {
	return s.repository.GetByCode(ctx, normaliseCode(code))
}

// ListVouchers returns vouchers for the admin API.
func (s *Service) ListVouchers(ctx context.Context, filter ListFilter) ([]Voucher, error) {
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	return s.repository.List(ctx, filter)
}

// Validate checks whether a code is usable for a given subtotal without
// redeeming it (BR-038). Used by checkout preview + the AI before confirmation.
func (s *Service) Validate(ctx context.Context, code string, subtotal float64) (ApplyResult, error) {
	voucher, err := s.repository.GetByCode(ctx, normaliseCode(code))
	if err != nil {
		return ApplyResult{}, err
	}
	discount, err := voucher.validate(subtotal, s.now())
	if err != nil {
		return ApplyResult{}, err
	}
	return ApplyResult{
		VoucherID:    voucher.ID,
		Code:         voucher.Code,
		DiscountType: voucher.DiscountType,
		Discount:     discount,
		Subtotal:     subtotal,
		Total:        subtotal - discount,
	}, nil
}

// Redeem validates then atomically consumes one unit of quota (BR-038). Checkout
// calls this on confirm so a limited voucher can't be over-redeemed under
// concurrency. Returns the same ApplyResult as Validate.
func (s *Service) Redeem(ctx context.Context, code string, subtotal float64) (ApplyResult, error) {
	voucher, err := s.repository.GetByCode(ctx, normaliseCode(code))
	if err != nil {
		return ApplyResult{}, err
	}
	discount, err := voucher.validate(subtotal, s.now())
	if err != nil {
		return ApplyResult{}, err
	}
	// Atomically consume quota; the repository re-checks quota under a row lock
	// so this is safe against concurrent checkouts.
	if _, err := s.repository.Redeem(ctx, voucher.ID); err != nil {
		return ApplyResult{}, err
	}
	return ApplyResult{
		VoucherID:    voucher.ID,
		Code:         voucher.Code,
		DiscountType: voucher.DiscountType,
		Discount:     discount,
		Subtotal:     subtotal,
		Total:        subtotal - discount,
	}, nil
}

func validateDiscountValue(discountType string, value float64) error {
	if value <= 0 {
		return ErrInvalidDiscount
	}
	if discountType == DiscountPercent && value > 100 {
		return ErrInvalidDiscount
	}
	return nil
}

func nonNeg(v float64) float64 {
	if v < 0 {
		return 0
	}
	return v
}

func maxInt(v, floor int) int {
	if v < floor {
		return floor
	}
	return v
}
