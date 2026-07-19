// Package promo implements Phase 10 (BR-037..BR-038): voucher management for the
// admin API and voucher validation/redemption used at checkout. A voucher is
// only usable when it is active, within its start/expiry window, still has quota
// left, and the order meets the minimum spend.
package promo

import (
	"errors"
	"strings"
	"time"
)

// Domain errors surface as typed sentinels so the HTTP layer can map them to
// stable error codes without leaking repository details.
var (
	ErrVoucherNotFound   = errors.New("voucher not found")
	ErrCodeTaken         = errors.New("voucher code already taken")
	ErrVoucherInactive   = errors.New("voucher is not active")
	ErrVoucherNotStarted = errors.New("voucher is not yet active")
	ErrVoucherExpired    = errors.New("voucher has expired")
	ErrVoucherExhausted  = errors.New("voucher quota is exhausted")
	ErrMinOrderNotMet    = errors.New("order does not meet the voucher minimum")
	ErrInvalidDiscount   = errors.New("invalid discount configuration")
	ErrValidation        = errors.New("invalid voucher payload")
)

// Discount types supported (mirrors the check constraint in migration 000007).
const (
	DiscountPercent = "percent" // discount_value is a percentage 0..100
	DiscountFixed   = "fixed"   // discount_value is an absolute amount in IDR
)

// Voucher is the persistent promotion definition managed by admins (BR-048).
type Voucher struct {
	ID             string    `json:"id"`
	Code           string    `json:"code"`
	Description    string    `json:"description"`
	DiscountType   string    `json:"discount_type"`
	DiscountValue  float64   `json:"discount_value"`
	MinOrderAmount float64   `json:"min_order_amount"`
	Quota          int       `json:"quota"`
	UsedCount      int       `json:"used_count"`
	StartsAt       time.Time `json:"starts_at"`
	ExpiresAt      time.Time `json:"expires_at"`
	IsActive       bool      `json:"is_active"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// RemainingQuota returns how many redemptions are left. A quota of 0 means the
// voucher is unlimited (never exhausted).
func (v Voucher) RemainingQuota() int {
	if v.Quota == 0 {
		return -1 // unlimited
	}
	return v.Quota - v.UsedCount
}

// validate applies the usability rules (BR-038) against a candidate order
// subtotal at a given time. It returns the discount amount when valid.
func (v Voucher) validate(subtotal float64, now time.Time) (float64, error) {
	if !v.IsActive {
		return 0, ErrVoucherInactive
	}
	if !v.StartsAt.IsZero() && now.Before(v.StartsAt) {
		return 0, ErrVoucherNotStarted
	}
	if !v.ExpiresAt.IsZero() && now.After(v.ExpiresAt) {
		return 0, ErrVoucherExpired
	}
	if v.Quota > 0 && v.UsedCount >= v.Quota {
		return 0, ErrVoucherExhausted
	}
	if subtotal < v.MinOrderAmount {
		return 0, ErrMinOrderNotMet
	}
	return v.computeDiscount(subtotal), nil
}

// computeDiscount resolves the discount amount for a subtotal, clamped so it can
// never exceed the subtotal (a voucher can zero out a bill but not go negative).
func (v Voucher) computeDiscount(subtotal float64) float64 {
	var discount float64
	switch v.DiscountType {
	case DiscountPercent:
		discount = subtotal * v.DiscountValue / 100
	case DiscountFixed:
		discount = v.DiscountValue
	default:
		return 0
	}
	if discount > subtotal {
		discount = subtotal
	}
	if discount < 0 {
		discount = 0
	}
	return discount
}

// CreateVoucherInput drives Service.CreateVoucher (admin API).
type CreateVoucherInput struct {
	Code           string
	Description    string
	DiscountType   string
	DiscountValue  float64
	MinOrderAmount float64
	Quota          int
	StartsAt       *time.Time
	ExpiresAt      time.Time
	IsActive       *bool
}

// UpdateVoucherInput drives Service.UpdateVoucher (admin API). All fields are
// replaced; UsedCount is never editable through the API.
type UpdateVoucherInput struct {
	Description    string
	DiscountType   string
	DiscountValue  float64
	MinOrderAmount float64
	Quota          int
	StartsAt       *time.Time
	ExpiresAt      time.Time
	IsActive       *bool
}

// ListFilter for admin voucher listing.
type ListFilter struct {
	ActiveOnly bool
	Limit      int
	Offset     int
}

// ApplyResult is the outcome of validating a voucher against an order subtotal.
// It carries the snapshot values checkout persists onto the order.
type ApplyResult struct {
	VoucherID    string  `json:"voucher_id"`
	Code         string  `json:"code"`
	DiscountType string  `json:"discount_type"`
	Discount     float64 `json:"discount"`
	Subtotal     float64 `json:"subtotal"`
	Total        float64 `json:"total"` // subtotal - discount (shipping added by caller)
}

func normaliseCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

func validDiscountType(t string) bool {
	return t == DiscountPercent || t == DiscountFixed
}
