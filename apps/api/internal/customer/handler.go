package customer

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/catalog"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/platform/response"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

type upsertCustomerRequest struct {
	Phone string `json:"phone" binding:"required"`
	Name  string `json:"name"`
}

type createAddressRequest struct {
	Label         string `json:"label"`
	RecipientName string `json:"recipient_name" binding:"required"`
	Phone         string `json:"phone" binding:"required"`
	AddressLine   string `json:"address_line" binding:"required"`
	City          string `json:"city" binding:"required"`
	District      string `json:"district" binding:"required"`
	PostalCode    string `json:"postal_code"`
	Notes         string `json:"notes"`
	IsDefault     bool   `json:"is_default"`
}

type cartItemRequest struct {
	VariantID string `json:"variant_id" binding:"required"`
	Qty       int    `json:"qty" binding:"required"`
}

func (h *Handler) ListCustomers(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	customers, err := h.service.ListCustomers(c.Request.Context(), limit, offset)
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, gin.H{"customers": customers})
}

func (h *Handler) GetCustomer(c *gin.Context) {
	customer, err := h.service.GetCustomer(c.Request.Context(), c.Param("id"))
	if errors.Is(err, ErrCustomerNotFound) {
		response.Fail(c, http.StatusNotFound, "CUSTOMER_NOT_FOUND", "customer not found")
		return
	}
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, customer)
}

func (h *Handler) UpsertCustomer(c *gin.Context) {
	var request upsertCustomerRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.Fail(c, http.StatusBadRequest, "VALIDATION_ERROR", "phone is required")
		return
	}
	customer, err := h.service.UpsertCustomerByPhone(c.Request.Context(), UpsertCustomerInput{Phone: request.Phone, Name: request.Name})
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, customer)
}

func (h *Handler) GetOpenCart(c *gin.Context) {
	cart, err := h.service.GetOpenCart(c.Request.Context(), c.Param("id"))
	if errors.Is(err, ErrCustomerNotFound) {
		response.Fail(c, http.StatusNotFound, "CUSTOMER_NOT_FOUND", "customer not found")
		return
	}
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, cart)
}

func (h *Handler) AddCartItem(c *gin.Context) {
	var request cartItemRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.Fail(c, http.StatusBadRequest, "VALIDATION_ERROR", "variant_id and qty are required")
		return
	}
	cart, err := h.service.AddCartItem(c.Request.Context(), c.Param("id"), request.VariantID, request.Qty)
	if failCartError(c, err) {
		return
	}
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, cart)
}

func (h *Handler) UpdateCartItem(c *gin.Context) {
	var request cartItemRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.Fail(c, http.StatusBadRequest, "VALIDATION_ERROR", "qty is required")
		return
	}
	variantID := c.Param("variantId")
	if request.VariantID != "" && request.VariantID != variantID {
		response.Fail(c, http.StatusBadRequest, "VALIDATION_ERROR", "variant_id mismatch")
		return
	}
	cart, err := h.service.UpdateCartItem(c.Request.Context(), c.Param("id"), variantID, request.Qty)
	if failCartError(c, err) {
		return
	}
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, cart)
}

func (h *Handler) RemoveCartItem(c *gin.Context) {
	cart, err := h.service.RemoveCartItem(c.Request.Context(), c.Param("id"), c.Param("variantId"))
	if failCartError(c, err) {
		return
	}
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, cart)
}

func (h *Handler) ListAddresses(c *gin.Context) {
	addresses, err := h.service.ListAddresses(c.Request.Context(), c.Param("id"))
	if errors.Is(err, ErrCustomerNotFound) {
		response.Fail(c, http.StatusNotFound, "CUSTOMER_NOT_FOUND", "customer not found")
		return
	}
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, gin.H{"addresses": addresses})
}

func (h *Handler) GetDefaultAddress(c *gin.Context) {
	address, err := h.service.GetDefaultAddress(c.Request.Context(), c.Param("id"))
	if errors.Is(err, ErrAddressNotFound) {
		response.Fail(c, http.StatusNotFound, "ADDRESS_NOT_FOUND", "default address not found")
		return
	}
	if errors.Is(err, ErrCustomerNotFound) {
		response.Fail(c, http.StatusNotFound, "CUSTOMER_NOT_FOUND", "customer not found")
		return
	}
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, address)
}

func (h *Handler) CreateAddress(c *gin.Context) {
	var request createAddressRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.Fail(c, http.StatusBadRequest, "VALIDATION_ERROR", "address fields are required")
		return
	}
	address, err := h.service.CreateAddress(c.Request.Context(), c.Param("id"), CreateAddressInput{
		Label: request.Label, RecipientName: request.RecipientName, Phone: request.Phone,
		AddressLine: request.AddressLine, City: request.City, District: request.District,
		PostalCode: request.PostalCode, Notes: request.Notes, IsDefault: request.IsDefault,
	})
	if errors.Is(err, ErrCustomerNotFound) {
		response.Fail(c, http.StatusNotFound, "CUSTOMER_NOT_FOUND", "customer not found")
		return
	}
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.Created(c, address)
}

func failCartError(c *gin.Context, err error) bool {
	switch {
	case errors.Is(err, ErrCustomerNotFound):
		response.Fail(c, http.StatusNotFound, "CUSTOMER_NOT_FOUND", "customer not found")
	case errors.Is(err, ErrCartNotFound):
		response.Fail(c, http.StatusNotFound, "CART_NOT_FOUND", "open cart not found")
	case errors.Is(err, ErrCartItemNotFound):
		response.Fail(c, http.StatusNotFound, "CART_ITEM_NOT_FOUND", "cart item not found")
	case errors.Is(err, ErrInvalidQuantity):
		response.Fail(c, http.StatusBadRequest, "VALIDATION_ERROR", "quantity must be greater than zero")
	case errors.Is(err, catalog.ErrVariantNotFound):
		response.Fail(c, http.StatusNotFound, "VARIANT_NOT_FOUND", "variant not found")
	case errors.Is(err, catalog.ErrInsufficientStock):
		response.Fail(c, http.StatusConflict, "INSUFFICIENT_STOCK", "insufficient stock")
	default:
		return false
	}
	return true
}
