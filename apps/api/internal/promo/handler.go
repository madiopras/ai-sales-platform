package promo

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/platform/response"
)

// Handler wires Gin routes to the promo service. It stays thin — request
// decoding, error → HTTP status mapping.
type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

type createVoucherRequest struct {
	Code           string     `json:"code" binding:"required,min=1,max=64"`
	Description    string     `json:"description"`
	DiscountType   string     `json:"discount_type" binding:"required,oneof=percent fixed"`
	DiscountValue  float64    `json:"discount_value" binding:"required,gt=0"`
	MinOrderAmount float64    `json:"min_order_amount"`
	Quota          int        `json:"quota"`
	StartsAt       *time.Time `json:"starts_at"`
	ExpiresAt      time.Time  `json:"expires_at" binding:"required"`
	IsActive       *bool      `json:"is_active"`
}

type updateVoucherRequest struct {
	Description    string     `json:"description"`
	DiscountType   string     `json:"discount_type" binding:"required,oneof=percent fixed"`
	DiscountValue  float64    `json:"discount_value" binding:"required,gt=0"`
	MinOrderAmount float64    `json:"min_order_amount"`
	Quota          int        `json:"quota"`
	StartsAt       *time.Time `json:"starts_at"`
	ExpiresAt      time.Time  `json:"expires_at" binding:"required"`
	IsActive       *bool      `json:"is_active"`
}

type validateVoucherRequest struct {
	Code     string  `json:"code" binding:"required"`
	Subtotal float64 `json:"subtotal" binding:"required,gt=0"`
}

// CreateVoucher persists a new voucher (admin, BR-048).
func (h *Handler) CreateVoucher(c *gin.Context) {
	var request createVoucherRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.Fail(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
		return
	}
	voucher, err := h.service.CreateVoucher(c.Request.Context(), CreateVoucherInput{
		Code:           request.Code,
		Description:    request.Description,
		DiscountType:   request.DiscountType,
		DiscountValue:  request.DiscountValue,
		MinOrderAmount: request.MinOrderAmount,
		Quota:          request.Quota,
		StartsAt:       request.StartsAt,
		ExpiresAt:      request.ExpiresAt,
		IsActive:       request.IsActive,
	})
	if failVoucherError(c, err) {
		return
	}
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.Created(c, voucher)
}

// UpdateVoucher replaces an existing voucher's editable fields (admin).
func (h *Handler) UpdateVoucher(c *gin.Context) {
	var request updateVoucherRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.Fail(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
		return
	}
	voucher, err := h.service.UpdateVoucher(c.Request.Context(), c.Param("id"), UpdateVoucherInput{
		Description:    request.Description,
		DiscountType:   request.DiscountType,
		DiscountValue:  request.DiscountValue,
		MinOrderAmount: request.MinOrderAmount,
		Quota:          request.Quota,
		StartsAt:       request.StartsAt,
		ExpiresAt:      request.ExpiresAt,
		IsActive:       request.IsActive,
	})
	if failVoucherError(c, err) {
		return
	}
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, voucher)
}

// GetVoucher returns a voucher by id (admin).
func (h *Handler) GetVoucher(c *gin.Context) {
	voucher, err := h.service.GetVoucher(c.Request.Context(), c.Param("id"))
	if failVoucherError(c, err) {
		return
	}
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, voucher)
}

// ListVouchers returns vouchers with an optional active_only filter (admin).
func (h *Handler) ListVouchers(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	vouchers, err := h.service.ListVouchers(c.Request.Context(), ListFilter{
		ActiveOnly: c.Query("active_only") == "true",
		Limit:      limit,
		Offset:     offset,
	})
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, gin.H{"vouchers": vouchers})
}

// DeleteVoucher removes a voucher by id (admin).
func (h *Handler) DeleteVoucher(c *gin.Context) {
	if err := h.service.DeleteVoucher(c.Request.Context(), c.Param("id")); err != nil {
		if failVoucherError(c, err) {
			return
		}
		_ = c.Error(err)
		return
	}
	response.OK(c, gin.H{"deleted": true})
}

// ValidateVoucher checks whether a code is usable for a subtotal without
// consuming quota (internal AI, BR-037..BR-038).
func (h *Handler) ValidateVoucher(c *gin.Context) {
	var request validateVoucherRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.Fail(c, http.StatusBadRequest, "VALIDATION_ERROR", "code and subtotal are required")
		return
	}
	result, err := h.service.Validate(c.Request.Context(), request.Code, request.Subtotal)
	if failVoucherError(c, err) {
		return
	}
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, result)
}

// failVoucherError maps promo sentinels to HTTP responses. Returns true when a
// response was written so callers can early-return.
func failVoucherError(c *gin.Context, err error) bool {
	switch {
	case errors.Is(err, ErrVoucherNotFound):
		response.Fail(c, http.StatusNotFound, "VOUCHER_NOT_FOUND", "voucher not found")
	case errors.Is(err, ErrCodeTaken):
		response.Fail(c, http.StatusConflict, "VOUCHER_CODE_TAKEN", "voucher code already taken")
	case errors.Is(err, ErrInvalidDiscount):
		response.Fail(c, http.StatusBadRequest, "INVALID_DISCOUNT", "invalid discount configuration")
	case errors.Is(err, ErrValidation):
		response.Fail(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid voucher payload")
	case errors.Is(err, ErrVoucherInactive):
		response.Fail(c, http.StatusConflict, "VOUCHER_INACTIVE", "voucher is not active")
	case errors.Is(err, ErrVoucherNotStarted):
		response.Fail(c, http.StatusConflict, "VOUCHER_NOT_STARTED", "voucher is not yet active")
	case errors.Is(err, ErrVoucherExpired):
		response.Fail(c, http.StatusConflict, "VOUCHER_EXPIRED", "voucher has expired")
	case errors.Is(err, ErrVoucherExhausted):
		response.Fail(c, http.StatusConflict, "VOUCHER_EXHAUSTED", "voucher quota is exhausted")
	case errors.Is(err, ErrMinOrderNotMet):
		response.Fail(c, http.StatusConflict, "MIN_ORDER_NOT_MET", "order does not meet the voucher minimum")
	default:
		return false
	}
	return true
}
