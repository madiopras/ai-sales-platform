package shipping

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/platform/response"
)

// Handler wires Gin routes to the shipping service. It stays thin — request
// decoding, error → HTTP status mapping, and passing raw bytes to the service.
type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

type rateRequest struct {
	OriginPostalCode      int    `json:"origin_postal_code"`
	DestinationPostalCode int    `json:"destination_postal_code" binding:"required"`
	Couriers              string `json:"couriers"`
	Items                 []struct {
		Name     string  `json:"name"`
		Value    float64 `json:"value"`
		Quantity int     `json:"quantity"`
		Weight   int     `json:"weight"`
	} `json:"items"`
}

// GetRates returns courier options for a destination (BR-010..BR-011). Called by
// the AI before checkout so the customer can pick a service.
func (h *Handler) GetRates(c *gin.Context) {
	var request rateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.Fail(c, http.StatusBadRequest, "VALIDATION_ERROR", "destination_postal_code is required")
		return
	}
	input := RateInput{
		OriginPostalCode:      request.OriginPostalCode,
		DestinationPostalCode: request.DestinationPostalCode,
		Couriers:              request.Couriers,
	}
	for _, item := range request.Items {
		input.Items = append(input.Items, RateItem{
			Name:     item.Name,
			Value:    item.Value,
			Quantity: item.Quantity,
			Weight:   item.Weight,
		})
	}
	rates, err := h.service.GetRates(c.Request.Context(), input)
	if failShippingError(c, err) {
		return
	}
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, gin.H{"rates": rates})
}

type createShipmentRequest struct {
	OrderID string `json:"order_id" binding:"required"`
}

// CreateShipment books a shipment at Biteship for an order that is ready to ship
// (BR-028). Called by the admin API or automatically after an order is marked
// ready.
func (h *Handler) CreateShipment(c *gin.Context) {
	var request createShipmentRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.Fail(c, http.StatusBadRequest, "VALIDATION_ERROR", "order_id is required")
		return
	}
	shipment, err := h.service.CreateShipment(c.Request.Context(), CreateShipmentInput{OrderID: request.OrderID})
	if failShippingError(c, err) {
		return
	}
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.Created(c, shipment)
}

// GetShipment returns a shipment by id.
func (h *Handler) GetShipment(c *gin.Context) {
	shipment, err := h.service.GetShipment(c.Request.Context(), c.Param("id"))
	if failShippingError(c, err) {
		return
	}
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, shipment)
}

// GetShipmentByOrder returns the shipment tied to an order id (BR-032).
func (h *Handler) GetShipmentByOrder(c *gin.Context) {
	shipment, err := h.service.GetShipmentByOrderID(c.Request.Context(), c.Param("id"))
	if failShippingError(c, err) {
		return
	}
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, shipment)
}

// BiteshipWebhook accepts Biteship tracking callbacks. Signature validation lives
// in the service — the handler forwards the raw payload + the auth token header.
//
// Biteship expects a 2xx quickly; we short-circuit on missing headers/payloads
// with a clear error code so retries don't loop endlessly.
func (h *Handler) BiteshipWebhook(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "INVALID_PAYLOAD", "unable to read request body")
		return
	}
	if len(body) == 0 {
		response.Fail(c, http.StatusBadRequest, "INVALID_PAYLOAD", "empty payload")
		return
	}

	payload, err := parseBiteshipPayload(body)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "INVALID_PAYLOAD", "invalid biteship payload")
		return
	}
	payload.RawPayload = body
	// Biteship lets you configure a webhook auth token; it arrives either in the
	// Authorization header or a custom token header depending on setup.
	payload.Signature = firstNonEmpty(
		c.GetHeader("Authorization"),
		c.GetHeader("X-Biteship-Token"),
		c.GetHeader("biteship-token"),
	)

	shipment, err := h.service.HandleWebhook(c.Request.Context(), payload)
	switch {
	case errors.Is(err, ErrInvalidSignature):
		response.Fail(c, http.StatusUnauthorized, "INVALID_SIGNATURE", "invalid webhook signature")
		return
	case errors.Is(err, ErrShipmentNotFound):
		// 200 avoids Biteship retry storms for cases we can never reconcile.
		response.OK(c, gin.H{"status": "ignored", "reason": "shipment_not_found"})
		return
	case errors.Is(err, ErrInvalidWebhookPayload):
		response.Fail(c, http.StatusBadRequest, "INVALID_PAYLOAD", "invalid biteship payload")
		return
	}
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, gin.H{"status": "ok", "shipment_status": shipment.Status})
}

// parseBiteshipPayload decodes the subset of fields we need from a tracking
// callback. Extra fields on the wire are ignored so Biteship can add new
// attributes without breaking us. See https://biteship.com/id/docs/api/trackings/webhook.
func parseBiteshipPayload(body []byte) (WebhookInput, error) {
	var raw struct {
		Event             string `json:"event"`
		OrderID           string `json:"order_id"`
		CourierWaybillID  string `json:"courier_waybill_id"`
		CourierTrackingID string `json:"courier_tracking_id"`
		Status            string `json:"status"`
		// Some Biteship payloads nest the tracking status under `courier`.
		Courier struct {
			WaybillID  string `json:"waybill_id"`
			TrackingID string `json:"tracking_id"`
		} `json:"courier"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return WebhookInput{}, err
	}

	trackingNo := firstNonEmpty(raw.CourierWaybillID, raw.CourierTrackingID, raw.Courier.WaybillID, raw.Courier.TrackingID)
	if strings.TrimSpace(raw.OrderID) == "" && strings.TrimSpace(trackingNo) == "" {
		return WebhookInput{}, ErrInvalidWebhookPayload
	}

	// Biteship doesn't always send a unique event id; derive an idempotency key
	// from the order id + status so a status change is processed once.
	eventKey := firstNonEmpty(raw.OrderID, trackingNo) + ":" + strings.ToLower(strings.TrimSpace(raw.Status))

	return WebhookInput{
		EventKey:        eventKey,
		Event:           raw.Event,
		BiteshipOrderID: raw.OrderID,
		TrackingNo:      trackingNo,
		Status:          raw.Status,
	}, nil
}

// failShippingError maps shipping sentinels to HTTP responses. Returns true when
// a response was written so callers can early-return.
func failShippingError(c *gin.Context, err error) bool {
	switch {
	case errors.Is(err, ErrShipmentNotFound):
		response.Fail(c, http.StatusNotFound, "SHIPMENT_NOT_FOUND", "shipment not found")
	case errors.Is(err, ErrShipmentAlreadyExists):
		response.Fail(c, http.StatusConflict, "SHIPMENT_ALREADY_EXISTS", "shipment already exists for order")
	case errors.Is(err, ErrOrderNotFound):
		response.Fail(c, http.StatusNotFound, "ORDER_NOT_FOUND", "order not found")
	case errors.Is(err, ErrOrderNotShippable):
		response.Fail(c, http.StatusConflict, "ORDER_NOT_SHIPPABLE", "order is not in a shippable state")
	case errors.Is(err, ErrAddressIncomplete):
		response.Fail(c, http.StatusBadRequest, "ADDRESS_INCOMPLETE", "origin and destination postal codes are required")
	case errors.Is(err, ErrRatesUnavailable):
		response.Fail(c, http.StatusNotFound, "RATES_UNAVAILABLE", "no shipping rates available for this route")
	case errors.Is(err, ErrBiteshipFailure):
		response.Fail(c, http.StatusBadGateway, "SHIPPING_GATEWAY_ERROR", "failed to reach shipping provider")
	default:
		return false
	}
	return true
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
