// Package biteship is a thin HTTP client for the Biteship REST API.
//
// It only implements the endpoints the shipping domain currently uses:
//   - POST /v1/rates/couriers  (shipping rate quote before checkout, BR-010..BR-011)
//   - POST /v1/orders          (create a shipment / booking, BR-028..BR-029)
//   - GET  /v1/orders/:id      (optional lookup used by tests/tooling)
//
// The client is intentionally small so tests can swap in an in-memory fake by
// implementing the shipping.BiteshipClient interface. See the API reference at
// https://biteship.com/id/docs/intro.
package biteship

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ErrUnauthorized indicates the API key was rejected by Biteship (HTTP 401).
var ErrUnauthorized = errors.New("biteship: unauthorized (check BITESHIP_API_KEY)")

// Client talks to the Biteship REST API. Biteship authenticates via the raw API
// key in the Authorization header (no "Bearer " prefix).
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// Config configures a Client. BaseURL defaults to https://api.biteship.com.
type Config struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
	Timeout    time.Duration
}

// NewClient constructs a Client. The returned client is safe for concurrent use.
func NewClient(cfg Config) *Client {
	base := cfg.BaseURL
	if base == "" {
		base = "https://api.biteship.com"
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		timeout := cfg.Timeout
		if timeout == 0 {
			timeout = 15 * time.Second
		}
		httpClient = &http.Client{Timeout: timeout}
	}
	return &Client{baseURL: base, apiKey: cfg.APIKey, httpClient: httpClient}
}

// --- Rates ---

// RatesRequest mirrors the subset of Biteship POST /v1/rates/couriers fields we
// send. Origin/destination can be given by postal code or area id; we use
// postal code + address which is enough for BR-010.
// See https://biteship.com/id/docs/api/rates/retrieve.
type RatesRequest struct {
	OriginPostalCode      int    `json:"origin_postal_code,omitempty"`
	DestinationPostalCode int    `json:"destination_postal_code,omitempty"`
	OriginAreaID          string `json:"origin_area_id,omitempty"`
	DestinationAreaID     string `json:"destination_area_id,omitempty"`
	Couriers              string `json:"couriers"` // comma-separated courier codes
	Items                 []Item `json:"items"`
}

// Item is a single package line used for rate + order requests. Weight is grams.
type Item struct {
	Name        string  `json:"name"`
	Description string  `json:"description,omitempty"`
	Value       float64 `json:"value"`
	Quantity    int     `json:"quantity"`
	Weight      int     `json:"weight"`
}

// RatesResponse is the subset of the Biteship rates payload we care about.
type RatesResponse struct {
	Success bool          `json:"success"`
	Error   string        `json:"error"`
	Pricing []PricingRate `json:"pricing"`
}

// PricingRate is a single courier service option returned by the rates API.
type PricingRate struct {
	CourierCode           string  `json:"courier_code"`
	CourierServiceCode    string  `json:"courier_service_code"`
	CourierName           string  `json:"courier_name"`
	CourierServiceName    string  `json:"courier_service_name"`
	Duration              string  `json:"duration"`
	ShipmentDurationRange string  `json:"shipment_duration_range"`
	Price                 float64 `json:"price"`
	Type                  string  `json:"type"`
}

// GetRates calls POST /v1/rates/couriers and returns the available courier rates.
func (c *Client) GetRates(ctx context.Context, req RatesRequest) (RatesResponse, error) {
	var out RatesResponse
	if err := c.do(ctx, http.MethodPost, "/v1/rates/couriers", req, &out); err != nil {
		return RatesResponse{}, err
	}
	return out, nil
}

// --- Orders (shipments) ---

// CreateOrderRequest mirrors the subset of Biteship POST /v1/orders fields we
// send to book a shipment. See https://biteship.com/id/docs/api/orders/create.
type CreateOrderRequest struct {
	ShipperContactName  string `json:"shipper_contact_name,omitempty"`
	ShipperContactPhone string `json:"shipper_contact_phone,omitempty"`

	OriginContactName  string `json:"origin_contact_name"`
	OriginContactPhone string `json:"origin_contact_phone"`
	OriginAddress      string `json:"origin_address"`
	OriginPostalCode   int    `json:"origin_postal_code,omitempty"`
	OriginNote         string `json:"origin_note,omitempty"`

	DestinationContactName  string `json:"destination_contact_name"`
	DestinationContactPhone string `json:"destination_contact_phone"`
	DestinationAddress      string `json:"destination_address"`
	DestinationPostalCode   int    `json:"destination_postal_code,omitempty"`
	DestinationNote         string `json:"destination_note,omitempty"`

	CourierCompany string `json:"courier_company"`
	CourierType    string `json:"courier_type"`
	DeliveryType   string `json:"delivery_type"` // "now" | "scheduled"
	OrderNote      string `json:"order_note,omitempty"`
	ReferenceID    string `json:"reference_id,omitempty"`

	Items []Item `json:"items"`
}

// Order is the subset of the Biteship order payload we persist.
type Order struct {
	Success bool    `json:"success"`
	Error   string  `json:"error"`
	ID      string  `json:"id"`
	Status  string  `json:"status"`
	Price   float64 `json:"price"`
	Courier Courier `json:"courier"`
}

// Courier is the nested courier object on an order response.
type Courier struct {
	TrackingID string `json:"tracking_id"`
	WaybillID  string `json:"waybill_id"`
	Company    string `json:"company"`
	Type       string `json:"type"`
	Link       string `json:"link"`
}

// CreateOrder calls POST /v1/orders and returns the created shipment order.
func (c *Client) CreateOrder(ctx context.Context, req CreateOrderRequest) (Order, error) {
	var out Order
	if err := c.do(ctx, http.MethodPost, "/v1/orders", req, &out); err != nil {
		return Order{}, err
	}
	return out, nil
}

// GetOrder calls GET /v1/orders/:id. Handy for admin lookup + tests.
func (c *Client) GetOrder(ctx context.Context, id string) (Order, error) {
	var out Order
	if err := c.do(ctx, http.MethodGet, "/v1/orders/"+id, nil, &out); err != nil {
		return Order{}, err
	}
	return out, nil
}

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("biteship: marshal body: %w", err)
		}
		reader = bytes.NewReader(buf)
	}
	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return fmt.Errorf("biteship: build request: %w", err)
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	request.Header.Set("Accept", "application/json")
	// Biteship authenticates with the raw API key (no "Bearer " prefix).
	request.Header.Set("Authorization", c.apiKey)

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("biteship: send request: %w", err)
	}
	defer response.Body.Close()

	payload, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("biteship: read response: %w", err)
	}

	if response.StatusCode == http.StatusUnauthorized {
		return ErrUnauthorized
	}
	if response.StatusCode >= 400 {
		return fmt.Errorf("biteship: %s %s -> %d: %s", method, path, response.StatusCode, string(payload))
	}
	if out == nil || len(payload) == 0 {
		return nil
	}
	if err := json.Unmarshal(payload, out); err != nil {
		return fmt.Errorf("biteship: decode response: %w", err)
	}
	return nil
}
