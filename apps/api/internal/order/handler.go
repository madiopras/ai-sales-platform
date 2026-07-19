package order

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/catalog"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/customer"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/platform/response"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/promo"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

type addressRequest struct {
	RecipientName string `json:"recipient_name"`
	Phone         string `json:"phone"`
	AddressLine   string `json:"address_line"`
	City          string `json:"city"`
	District      string `json:"district"`
	PostalCode    string `json:"postal_code"`
	Notes         string `json:"notes"`
}

type courierRequest struct {
	Code    string  `json:"code"`
	Service string  `json:"service"`
	Fee     float64 `json:"fee"`
}

type checkoutRequest struct {
	CustomerID  string          `json:"customer_id" binding:"required"`
	Address     *addressRequest `json:"address"`
	Courier     courierRequest  `json:"courier"`
	VoucherCode string          `json:"voucher_code"`
}

func (r checkoutRequest) toInput() CheckoutInput {
	input := CheckoutInput{
		CustomerID:  r.CustomerID,
		Courier:     Courier{Code: r.Courier.Code, Service: r.Courier.Service, Fee: r.Courier.Fee},
		VoucherCode: r.VoucherCode,
	}

	if r.Address != nil {
		input.Address = &AddressSnap{
			RecipientName: r.Address.RecipientName,
			Phone:         r.Address.Phone,
			AddressLine:   r.Address.AddressLine,
			City:          r.Address.City,
			District:      r.Address.District,
			PostalCode:    r.Address.PostalCode,
			Notes:         r.Address.Notes,
		}
	}
	return input
}

type updateStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

// PreviewCheckout returns a non-persisted summary of the pending order (internal AI).
func (h *Handler) PreviewCheckout(c *gin.Context) {
	var request checkoutRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.Fail(c, http.StatusBadRequest, "VALIDATION_ERROR", "customer_id is required")
		return
	}
	preview, err := h.service.Preview(c.Request.Context(), request.toInput())
	if failCheckoutError(c, err) {
		return
	}
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, preview)
}

// Checkout confirms the order, reserves stock, and closes the cart (internal AI).
func (h *Handler) Checkout(c *gin.Context) {
	var request checkoutRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.Fail(c, http.StatusBadRequest, "VALIDATION_ERROR", "customer_id is required")
		return
	}
	order, err := h.service.Confirm(c.Request.Context(), request.toInput())
	if failCheckoutError(c, err) {
		return
	}
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.Created(c, order)
}

// ListOrders is an admin endpoint with optional customer_id/status filters.
func (h *Handler) ListOrders(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	orders, err := h.service.ListOrders(c.Request.Context(), ListFilter{
		CustomerID: c.Query("customer_id"),
		Status:     c.Query("status"),
		Limit:      limit,
		Offset:     offset,
	})
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, gin.H{"orders": orders})
}

func (h *Handler) GetOrder(c *gin.Context) {
	order, err := h.service.GetOrder(c.Request.Context(), c.Param("id"))
	if errors.Is(err, ErrOrderNotFound) {
		response.Fail(c, http.StatusNotFound, "ORDER_NOT_FOUND", "order not found")
		return
	}
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, order)
}

// UpdateStatus applies an admin fulfillment transition (BR-026).
func (h *Handler) UpdateStatus(c *gin.Context) {
	var request updateStatusRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.Fail(c, http.StatusBadRequest, "VALIDATION_ERROR", "status is required")
		return
	}
	order, err := h.service.UpdateStatus(c.Request.Context(), c.Param("id"), request.Status)
	switch {
	case errors.Is(err, ErrOrderNotFound):
		response.Fail(c, http.StatusNotFound, "ORDER_NOT_FOUND", "order not found")
		return
	case errors.Is(err, ErrInvalidStatus):
		response.Fail(c, http.StatusBadRequest, "INVALID_STATUS", "invalid order status")
		return
	case errors.Is(err, ErrStatusTransition):
		response.Fail(c, http.StatusConflict, "INVALID_TRANSITION", "invalid order status transition")
		return
	case errors.Is(err, ErrInsufficientStock):
		response.Fail(c, http.StatusConflict, "INSUFFICIENT_STOCK", "insufficient stock to commit")
		return
	}
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, order)
}

func failCheckoutError(c *gin.Context, err error) bool {
	switch {
	case errors.Is(err, customer.ErrCustomerNotFound):
		response.Fail(c, http.StatusNotFound, "CUSTOMER_NOT_FOUND", "customer not found")
	case errors.Is(err, customer.ErrCartNotFound):
		response.Fail(c, http.StatusNotFound, "CART_NOT_FOUND", "open cart not found")
	case errors.Is(err, ErrEmptyCart):
		response.Fail(c, http.StatusBadRequest, "EMPTY_CART", "cart is empty")
	case errors.Is(err, ErrAddressRequired):
		response.Fail(c, http.StatusBadRequest, "ADDRESS_REQUIRED", "shipping address is required")
	case errors.Is(err, ErrCourierRequired):
		response.Fail(c, http.StatusBadRequest, "COURIER_REQUIRED", "courier selection is required")
	case errors.Is(err, catalog.ErrVariantNotFound):
		response.Fail(c, http.StatusNotFound, "VARIANT_NOT_FOUND", "variant not found")
	case errors.Is(err, ErrInsufficientStock):
		response.Fail(c, http.StatusConflict, "INSUFFICIENT_STOCK", "insufficient stock")
	case errors.Is(err, ErrOrderNoConflict):
		response.Fail(c, http.StatusConflict, "ORDER_NO_CONFLICT", "order number conflict, retry")
	case errors.Is(err, promo.ErrVoucherNotFound):
		response.Fail(c, http.StatusNotFound, "VOUCHER_NOT_FOUND", "voucher not found")
	case errors.Is(err, promo.ErrVoucherInactive), errors.Is(err, promo.ErrVoucherNotStarted),
		errors.Is(err, promo.ErrVoucherExpired):
		response.Fail(c, http.StatusUnprocessableEntity, "VOUCHER_NOT_USABLE", "voucher is not usable")
	case errors.Is(err, promo.ErrVoucherExhausted):
		response.Fail(c, http.StatusConflict, "VOUCHER_EXHAUSTED", "voucher quota is exhausted")
	case errors.Is(err, promo.ErrMinOrderNotMet):
		response.Fail(c, http.StatusUnprocessableEntity, "VOUCHER_MIN_NOT_MET", "order does not meet the voucher minimum")
	default:
		return false
	}
	return true
}
