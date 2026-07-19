// Package xendit is a thin HTTP client for the Xendit Invoice API.
//
// It only implements the endpoints the payment domain currently uses:
//   - POST /v2/invoices  (create hosted payment page)
//   - GET  /v2/invoices/:id (optional lookup used by tests/tooling)
//
// The client is intentionally small so tests can swap in an in-memory fake by
// implementing the payment.XenditClient interface. See the API reference at
// https://docs.xendit.co/apidocs.
package xendit

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ErrUnauthorized indicates the secret key was rejected by Xendit (HTTP 401).
var ErrUnauthorized = errors.New("xendit: unauthorized (check XENDIT_SECRET_KEY)")

// Client talks to the Xendit REST API using Basic auth (secret key + empty password).
type Client struct {
	baseURL    string
	secretKey  string
	httpClient *http.Client
}

// Config configures a Client. BaseURL defaults to https://api.xendit.co.
type Config struct {
	BaseURL    string
	SecretKey  string
	HTTPClient *http.Client
	Timeout    time.Duration
}

// NewClient constructs a Client. The returned client is safe for concurrent use.
func NewClient(cfg Config) *Client {
	base := cfg.BaseURL
	if base == "" {
		base = "https://api.xendit.co"
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		timeout := cfg.Timeout
		if timeout == 0 {
			timeout = 15 * time.Second
		}
		httpClient = &http.Client{Timeout: timeout}
	}
	return &Client{baseURL: base, secretKey: cfg.SecretKey, httpClient: httpClient}
}

// CreateInvoiceRequest mirrors the subset of Xendit /v2/invoices fields we send.
// See https://docs.xendit.co/apidocs/create-invoice for the full schema.
type CreateInvoiceRequest struct {
	ExternalID         string   `json:"external_id"`
	Amount             float64  `json:"amount"`
	Description        string   `json:"description,omitempty"`
	PayerEmail         string   `json:"payer_email,omitempty"`
	InvoiceDuration    int64    `json:"invoice_duration,omitempty"` // seconds
	SuccessRedirectURL string   `json:"success_redirect_url,omitempty"`
	FailureRedirectURL string   `json:"failure_redirect_url,omitempty"`
	Currency           string   `json:"currency,omitempty"`
	Customer           *Payer   `json:"customer,omitempty"`
	Items              []Item   `json:"items,omitempty"`
	PaymentMethods     []string `json:"payment_methods,omitempty"`
}

// Payer is the optional `customer` field on a create-invoice request.
type Payer struct {
	GivenNames   string `json:"given_names,omitempty"`
	Email        string `json:"email,omitempty"`
	MobileNumber string `json:"mobile_number,omitempty"`
}

// Item represents a single line-item on the invoice (Xendit displays these on the hosted page).
type Item struct {
	Name     string  `json:"name"`
	Quantity int     `json:"quantity"`
	Price    float64 `json:"price"`
	Category string  `json:"category,omitempty"`
}

// Invoice is the subset of the Xendit invoice payload we care about.
type Invoice struct {
	ID                 string  `json:"id"`
	ExternalID         string  `json:"external_id"`
	Status             string  `json:"status"`
	Amount             float64 `json:"amount"`
	InvoiceURL         string  `json:"invoice_url"`
	ExpiryDate         string  `json:"expiry_date"`
	PaymentMethod      string  `json:"payment_method"`
	PaymentChannel     string  `json:"payment_channel"`
	Currency           string  `json:"currency"`
	SuccessRedirectURL string  `json:"success_redirect_url"`
	FailureRedirectURL string  `json:"failure_redirect_url"`
}

// CreateInvoice calls POST /v2/invoices and returns the created invoice.
func (c *Client) CreateInvoice(ctx context.Context, req CreateInvoiceRequest) (Invoice, error) {
	var invoice Invoice
	if err := c.do(ctx, http.MethodPost, "/v2/invoices", req, &invoice); err != nil {
		return Invoice{}, err
	}
	return invoice, nil
}

// GetInvoice calls GET /v2/invoices/:id. Handy for admin lookup + tests.
func (c *Client) GetInvoice(ctx context.Context, id string) (Invoice, error) {
	var invoice Invoice
	if err := c.do(ctx, http.MethodGet, "/v2/invoices/"+id, nil, &invoice); err != nil {
		return Invoice{}, err
	}
	return invoice, nil
}

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("xendit: marshal body: %w", err)
		}
		reader = bytes.NewReader(buf)
	}
	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return fmt.Errorf("xendit: build request: %w", err)
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	request.Header.Set("Accept", "application/json")
	// Xendit uses HTTP Basic with secretKey as username and empty password.
	request.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(c.secretKey+":")))

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("xendit: send request: %w", err)
	}
	defer response.Body.Close()

	payload, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("xendit: read response: %w", err)
	}

	if response.StatusCode == http.StatusUnauthorized {
		return ErrUnauthorized
	}
	if response.StatusCode >= 400 {
		return fmt.Errorf("xendit: %s %s -> %d: %s", method, path, response.StatusCode, string(payload))
	}
	if out == nil || len(payload) == 0 {
		return nil
	}
	if err := json.Unmarshal(payload, out); err != nil {
		return fmt.Errorf("xendit: decode response: %w", err)
	}
	return nil
}
