package promo

import (
	"context"
	"errors"
	"testing"
	"time"
)

func activeVoucher() Voucher {
	return Voucher{
		ID:            "vch-1",
		Code:          "HEMAT10",
		DiscountType:  DiscountPercent,
		DiscountValue: 10,
		Quota:         100,
		UsedCount:     0,
		StartsAt:      time.Now().Add(-time.Hour),
		ExpiresAt:     time.Now().Add(24 * time.Hour),
		IsActive:      true,
	}
}

func TestCreateVoucherRejectsBadDiscountType(t *testing.T) {
	service := NewService(&memoryRepo{})
	_, err := service.CreateVoucher(context.Background(), CreateVoucherInput{
		Code: "X", DiscountType: "cashback", DiscountValue: 10, ExpiresAt: time.Now().Add(time.Hour),
	})
	if !errors.Is(err, ErrInvalidDiscount) {
		t.Fatalf("expected ErrInvalidDiscount, got %v", err)
	}
}

func TestCreateVoucherRejectsPercentOver100(t *testing.T) {
	service := NewService(&memoryRepo{})
	_, err := service.CreateVoucher(context.Background(), CreateVoucherInput{
		Code: "X", DiscountType: DiscountPercent, DiscountValue: 150, ExpiresAt: time.Now().Add(time.Hour),
	})
	if !errors.Is(err, ErrInvalidDiscount) {
		t.Fatalf("expected ErrInvalidDiscount, got %v", err)
	}
}

func TestCreateVoucherUpperCasesCode(t *testing.T) {
	repo := &memoryRepo{}
	service := NewService(repo)
	v, err := service.CreateVoucher(context.Background(), CreateVoucherInput{
		Code: "hemat10", DiscountType: DiscountFixed, DiscountValue: 5000, ExpiresAt: time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	if v.Code != "HEMAT10" {
		t.Fatalf("expected code HEMAT10, got %q", v.Code)
	}
}

func TestValidatePercentDiscount(t *testing.T) {
	repo := &memoryRepo{voucher: activeVoucher()}
	service := NewService(repo)
	result, err := service.Validate(context.Background(), "hemat10", 100000)
	if err != nil {
		t.Fatal(err)
	}
	if result.Discount != 10000 {
		t.Fatalf("expected discount 10000, got %v", result.Discount)
	}
	if result.Total != 90000 {
		t.Fatalf("expected total 90000, got %v", result.Total)
	}
}

func TestValidateFixedDiscountClampsToSubtotal(t *testing.T) {
	v := activeVoucher()
	v.DiscountType = DiscountFixed
	v.DiscountValue = 200000
	repo := &memoryRepo{voucher: v}
	service := NewService(repo)
	result, err := service.Validate(context.Background(), "hemat10", 100000)
	if err != nil {
		t.Fatal(err)
	}
	if result.Discount != 100000 {
		t.Fatalf("expected discount clamped to 100000, got %v", result.Discount)
	}
	if result.Total != 0 {
		t.Fatalf("expected total 0, got %v", result.Total)
	}
}

func TestValidateRejectsExpired(t *testing.T) {
	v := activeVoucher()
	v.ExpiresAt = time.Now().Add(-time.Hour)
	service := NewService(&memoryRepo{voucher: v})
	_, err := service.Validate(context.Background(), "hemat10", 100000)
	if !errors.Is(err, ErrVoucherExpired) {
		t.Fatalf("expected ErrVoucherExpired, got %v", err)
	}
}

func TestValidateRejectsInactive(t *testing.T) {
	v := activeVoucher()
	v.IsActive = false
	service := NewService(&memoryRepo{voucher: v})
	_, err := service.Validate(context.Background(), "hemat10", 100000)
	if !errors.Is(err, ErrVoucherInactive) {
		t.Fatalf("expected ErrVoucherInactive, got %v", err)
	}
}

func TestValidateRejectsBelowMinimum(t *testing.T) {
	v := activeVoucher()
	v.MinOrderAmount = 150000
	service := NewService(&memoryRepo{voucher: v})
	_, err := service.Validate(context.Background(), "hemat10", 100000)
	if !errors.Is(err, ErrMinOrderNotMet) {
		t.Fatalf("expected ErrMinOrderNotMet, got %v", err)
	}
}

func TestValidateRejectsExhaustedQuota(t *testing.T) {
	v := activeVoucher()
	v.Quota = 5
	v.UsedCount = 5
	service := NewService(&memoryRepo{voucher: v})
	_, err := service.Validate(context.Background(), "hemat10", 100000)
	if !errors.Is(err, ErrVoucherExhausted) {
		t.Fatalf("expected ErrVoucherExhausted, got %v", err)
	}
}

func TestRedeemConsumesQuota(t *testing.T) {
	repo := &memoryRepo{voucher: activeVoucher()}
	service := NewService(repo)
	result, err := service.Redeem(context.Background(), "hemat10", 100000)
	if err != nil {
		t.Fatal(err)
	}
	if result.Discount != 10000 {
		t.Fatalf("expected discount 10000, got %v", result.Discount)
	}
	if repo.redeemCalls != 1 {
		t.Fatalf("expected 1 Redeem call, got %d", repo.redeemCalls)
	}
}

func TestRedeemRejectsInvalidBeforeConsuming(t *testing.T) {
	v := activeVoucher()
	v.ExpiresAt = time.Now().Add(-time.Hour)
	repo := &memoryRepo{voucher: v}
	service := NewService(repo)
	_, err := service.Redeem(context.Background(), "hemat10", 100000)
	if !errors.Is(err, ErrVoucherExpired) {
		t.Fatalf("expected ErrVoucherExpired, got %v", err)
	}
	if repo.redeemCalls != 0 {
		t.Fatalf("expected quota untouched on invalid voucher, got %d calls", repo.redeemCalls)
	}
}

// --- test double ---

type memoryRepo struct {
	voucher     Voucher
	created     Voucher
	redeemCalls int
}

func (r *memoryRepo) Create(_ context.Context, voucher Voucher) (Voucher, error) {
	voucher.ID = "vch-new"
	r.created = voucher
	return voucher, nil
}

func (r *memoryRepo) Update(_ context.Context, id string, voucher Voucher) (Voucher, error) {
	voucher.ID = id
	return voucher, nil
}

func (r *memoryRepo) Delete(_ context.Context, _ string) error { return nil }

func (r *memoryRepo) GetByID(_ context.Context, id string) (Voucher, error) {
	if r.voucher.ID == id && r.voucher.ID != "" {
		return r.voucher, nil
	}
	return Voucher{}, ErrVoucherNotFound
}

func (r *memoryRepo) GetByCode(_ context.Context, code string) (Voucher, error) {
	if r.voucher.Code == normaliseCode(code) && r.voucher.Code != "" {
		return r.voucher, nil
	}
	return Voucher{}, ErrVoucherNotFound
}

func (r *memoryRepo) List(_ context.Context, _ ListFilter) ([]Voucher, error) {
	return []Voucher{r.voucher}, nil
}

func (r *memoryRepo) Redeem(_ context.Context, _ string) (Voucher, error) {
	r.redeemCalls++
	r.voucher.UsedCount++
	return r.voucher, nil
}
