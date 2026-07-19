package payment

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/platform/response"
)

// Handler wires Gin routes to the payment service. It stays thin — request
// decoding, error → HTTP status mapping, and passing raw bytes to the service.
type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

type createInvoiceRequest struct {
	OrderID    string `json:"order_id" binding:"required"`
	PayerEmail string `json:"payer_email"`
}

// CreateInvoice creates a hosted Xendit payment page for a confirmed order.
// Called by the AI after checkout confirmation.
func (h *Handler) CreateInvoice(c *gin.Context) {
	var request createInvoiceRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.Fail(c, http.StatusBadRequest, "VALIDATION_ERROR", "order_id is required")
		return
	}
	invoice, err := h.service.CreateInvoice(c.Request.Context(), CreateInvoiceInput{
		OrderID:    request.OrderID,
		PayerEmail: request.PayerEmail,
	})
	if failInvoiceError(c, err) {
		return
	}
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.Created(c, invoice)
}

// GetInvoice returns an invoice by id.
func (h *Handler) GetInvoice(c *gin.Context) {
	invoice, err := h.service.GetInvoice(c.Request.Context(), c.Param("id"))
	if failInvoiceError(c, err) {
		return
	}
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, invoice)
}

// GetInvoiceByOrder returns the invoice tied to an order id.
func (h *Handler) GetInvoiceByOrder(c *gin.Context) {
	invoice, err := h.service.GetInvoiceByOrderID(c.Request.Context(), c.Param("orderId"))
	if failInvoiceError(c, err) {
		return
	}
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, invoice)
}

// XenditWebhook accepts Xendit invoice callbacks. Signature validation lives in
// the service — the handler forwards the raw payload + `x-callback-token`.
//
// Xendit expects a 2xx within a few seconds; we short-circuit on missing
// headers/payloads with a clear error code so retries don't loop endlessly.
func (h *Handler) XenditWebhook(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "INVALID_PAYLOAD", "unable to read request body")
		return
	}
	if len(body) == 0 {
		response.Fail(c, http.StatusBadRequest, "INVALID_PAYLOAD", "empty payload")
		return
	}

	payload, err := parseXenditPayload(body)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "INVALID_PAYLOAD", "invalid xendit payload")
		return
	}
	payload.RawPayload = body
	payload.CallbackToken = c.GetHeader("x-callback-token")

	invoice, err := h.service.HandleWebhook(c.Request.Context(), payload)
	switch {
	case errors.Is(err, ErrInvalidCallbackToken):
		response.Fail(c, http.StatusUnauthorized, "INVALID_CALLBACK_TOKEN", "invalid callback token")
		return
	case errors.Is(err, ErrInvoiceNotFound):
		// 200 avoids Xendit retry storms for cases we can never reconcile.
		response.OK(c, gin.H{"status": "ignored", "reason": "invoice_not_found"})
		return
	case errors.Is(err, ErrInvalidWebhookPayload):
		response.Fail(c, http.StatusBadRequest, "INVALID_PAYLOAD", "invalid xendit payload")
		return
	}
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, gin.H{"status": "ok", "invoice_status": invoice.Status})
}

// parseXenditPayload decodes the subset of fields we need from an invoice
// callback. Extra fields on the wire are ignored on purpose so Xendit can add
// new attributes without breaking us.
func parseXenditPayload(body []byte) (WebhookInput, error) {
	var raw struct {
		ID             string  `json:"id"`
		ExternalID     string  `json:"external_id"`
		Status         string  `json:"status"`
		PaymentMethod  string  `json:"payment_method"`
		PaymentChannel string  `json:"payment_channel"`
		Amount         float64 `json:"amount"`
		PaidAmount     float64 `json:"paid_amount"`
		PaidAt         string  `json:"paid_at"`
		EventID        string  `json:"event_id"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return WebhookInput{}, err
	}
	if strings.TrimSpace(raw.ID) == "" && strings.TrimSpace(raw.ExternalID) == "" {
		return WebhookInput{}, ErrInvalidWebhookPayload
	}
	// Fall back to Xendit invoice id as dedup key when Xendit doesn't send
	// an event id. status is included so PENDING → PAID both get recorded.
	eventID := raw.EventID
	if eventID == "" {
		eventID = raw.ID + ":" + strings.ToUpper(raw.Status)
	}
	input := WebhookInput{
		EventID:        eventID,
		ID:             raw.ID,
		ExternalID:     raw.ExternalID,
		Status:         raw.Status,
		PaymentMethod:  raw.PaymentMethod,
		PaymentChannel: raw.PaymentChannel,
		Amount:         raw.Amount,
	}
	if raw.PaidAmount > 0 {
		input.Amount = raw.PaidAmount
	}
	if raw.PaidAt != "" {
		if t, err := time.Parse(time.RFC3339, raw.PaidAt); err == nil {
			input.PaidAt = &t
		}
	}
	return input, nil
}

// failInvoiceError maps invoice sentinels to HTTP responses. Returns true when
// a response was written so callers can early-return.
func failInvoiceError(c *gin.Context, err error) bool {
	switch {
	case errors.Is(err, ErrInvoiceNotFound):
		response.Fail(c, http.StatusNotFound, "INVOICE_NOT_FOUND", "invoice not found")
	case errors.Is(err, ErrInvoiceAlreadyExists):
		response.Fail(c, http.StatusConflict, "INVOICE_ALREADY_EXISTS", "invoice already exists for order")
	case errors.Is(err, ErrOrderNotFound):
		response.Fail(c, http.StatusNotFound, "ORDER_NOT_FOUND", "order not found")
	case errors.Is(err, ErrOrderNotAwaitingPay):
		response.Fail(c, http.StatusConflict, "ORDER_NOT_PENDING_PAYMENT", "order is not pending payment")
	case errors.Is(err, ErrXenditFailure):
		response.Fail(c, http.StatusBadGateway, "PAYMENT_GATEWAY_ERROR", "failed to reach payment gateway")
	default:
		return false
	}
	return true
}
