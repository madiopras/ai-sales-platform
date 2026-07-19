package shipping

import (
	"context"
	"crypto/subtle"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/prasdios/ai-sales-platform/apps/api/internal/events"
	"github.com/prasdios/ai-sales-platform/apps/api/internal/shipping/biteship"
)

// BiteshipClient is the vendor surface the service depends on. The concrete
// implementation lives in `biteship.Client`; tests substitute a fake.
type BiteshipClient interface {
	GetRates(ctx context.Context, req biteship.RatesRequest) (biteship.RatesResponse, error)
	CreateOrder(ctx context.Context, req biteship.CreateOrderRequest) (biteship.Order, error)
}

// EventPublisher publishes domain events (order.shipped, order.delivered) for
// apps/ai to consume (BR-041). Optional: a nil publisher disables publishing.
type EventPublisher interface {
	Publish(ctx context.Context, event events.Event) error
}

// Config is the runtime configuration the service needs. Loaded once from
// `config.BiteshipConfig` at wire time; keep pure primitives so tests can build it.
type Config struct {
	WebhookToken     string
	DefaultCouriers  string
	DefaultWeight    int // grams; used when an item has no weight
	OriginPostalCode int

	// Origin (warehouse) details used when booking a shipment (BR-028).
	OriginContactName  string
	OriginContactPhone string
	OriginAddress      string
	OriginNote         string
}

// Service orchestrates rate quoting, shipment creation, Biteship calls, and
// webhook handling.
type Service struct {
	repository Repository
	client     BiteshipClient
	config     Config
	publisher  EventPublisher
	now        func() time.Time
}

// NewService returns a Service that uses time.Now() for delivery timestamps.
func NewService(repository Repository, client BiteshipClient, cfg Config) *Service {
	if cfg.DefaultWeight <= 0 {
		cfg.DefaultWeight = 1000 // 1 kg fallback so Biteship always gets a weight
	}
	if strings.TrimSpace(cfg.DefaultCouriers) == "" {
		cfg.DefaultCouriers = "jne,jnt,sicepat,anteraja"
	}
	return &Service{repository: repository, client: client, config: cfg, now: time.Now}
}

// SetEventPublisher wires a domain-event publisher so tracking transitions emit
// order.shipped / order.delivered for apps/ai (BR-041). Optional.
func (s *Service) SetEventPublisher(publisher EventPublisher) { s.publisher = publisher }

// GetRates fetches courier options from Biteship for the AI to present before
// checkout (BR-010..BR-011). On any Biteship failure it returns ErrBiteshipFailure
// so the caller can fall back to "coba lagi / hubungi admin" (BR-013).
func (s *Service) GetRates(ctx context.Context, input RateInput) ([]Rate, error) {
	origin := input.OriginPostalCode
	if origin == 0 {
		origin = s.config.OriginPostalCode
	}
	if origin == 0 || input.DestinationPostalCode == 0 {
		return nil, ErrAddressIncomplete
	}
	couriers := strings.TrimSpace(input.Couriers)
	if couriers == "" {
		couriers = s.config.DefaultCouriers
	}

	items := make([]biteship.Item, 0, len(input.Items))
	for _, item := range input.Items {
		weight := item.Weight
		if weight <= 0 {
			weight = s.config.DefaultWeight
		}
		qty := item.Quantity
		if qty <= 0 {
			qty = 1
		}
		items = append(items, biteship.Item{
			Name:     item.Name,
			Value:    item.Value,
			Quantity: qty,
			Weight:   weight,
		})
	}
	if len(items) == 0 {
		// Biteship requires at least one item; synthesise a default parcel.
		items = append(items, biteship.Item{Name: "Parcel", Value: 0, Quantity: 1, Weight: s.config.DefaultWeight})
	}

	resp, err := s.client.GetRates(ctx, biteship.RatesRequest{
		OriginPostalCode:      origin,
		DestinationPostalCode: input.DestinationPostalCode,
		Couriers:              couriers,
		Items:                 items,
	})
	if err != nil {
		return nil, ErrBiteshipFailure
	}
	if !resp.Success && resp.Error != "" {
		return nil, ErrBiteshipFailure
	}
	if len(resp.Pricing) == 0 {
		return nil, ErrRatesUnavailable
	}

	rates := make([]Rate, 0, len(resp.Pricing))
	for _, p := range resp.Pricing {
		duration := p.Duration
		if duration == "" {
			duration = p.ShipmentDurationRange
		}
		rates = append(rates, Rate{
			CourierCode:    p.CourierCode,
			CourierService: p.CourierServiceCode,
			CourierName:    p.CourierName,
			ServiceName:    p.CourierServiceName,
			Duration:       duration,
			Price:          p.Price,
		})
	}
	return rates, nil
}

// CreateShipment books a shipment at Biteship for an order that is ready to ship
// (BR-028..BR-030). The shipment row is created before the Biteship call so we
// always have a persistent audit trail; the biteship_* columns are backfilled
// after the API call succeeds. Idempotent: an order that already has a booked
// shipment returns it without re-booking.
func (s *Service) CreateShipment(ctx context.Context, input CreateShipmentInput) (Shipment, error) {
	input.OrderID = strings.TrimSpace(input.OrderID)
	if input.OrderID == "" {
		return Shipment{}, errors.New("order_id is required")
	}
	order, err := s.repository.GetOrderForShipment(ctx, input.OrderID)
	if err != nil {
		return Shipment{}, err
	}
	// Only orders that are paid or in fulfillment may be shipped (BR-028).
	if !isShippableStatus(order.Status) {
		return Shipment{}, ErrOrderNotShippable
	}
	if order.CourierCode == "" {
		return Shipment{}, ErrOrderNotShippable
	}

	existing, err := s.repository.GetByOrderID(ctx, order.ID)
	switch {
	case err == nil && existing.BiteshipOrderID != "":
		// Already booked; return the existing shipment (idempotent).
		return existing, nil
	case err != nil && !errors.Is(err, ErrShipmentNotFound):
		return Shipment{}, err
	}

	shipment := existing
	if errors.Is(err, ErrShipmentNotFound) {
		shipment, err = s.repository.CreateShipment(ctx, Shipment{
			OrderID:        order.ID,
			CourierCode:    order.CourierCode,
			CourierService: order.CourierService,
			ShippingFee:    order.ShippingFee,
			Status:         StatusPending,
		})
		if err != nil {
			return Shipment{}, err
		}
	}

	biteshipOrder, err := s.client.CreateOrder(ctx, s.buildCreateOrderRequest(order))
	if err != nil {
		return Shipment{}, ErrBiteshipFailure
	}
	if !biteshipOrder.Success && biteshipOrder.Error != "" {
		return Shipment{}, ErrBiteshipFailure
	}

	refreshed, err := s.repository.AttachBiteshipReferences(ctx, shipment.ID, BiteshipReferences{
		BiteshipOrderID: biteshipOrder.ID,
		TrackingNo:      biteshipOrder.Courier.WaybillID,
		WaybillID:       biteshipOrder.Courier.WaybillID,
		LabelURL:        biteshipOrder.Courier.Link,
		Status:          normaliseBookingStatus(biteshipOrder.Status),
	})
	if err != nil {
		return Shipment{}, err
	}
	return refreshed, nil
}

// GetShipment fetches a shipment by internal id.
func (s *Service) GetShipment(ctx context.Context, id string) (Shipment, error) {
	return s.repository.GetByID(ctx, strings.TrimSpace(id))
}

// GetShipmentByOrderID exposes the shipment tied to an order (AI needs this for
// tracking answers, BR-032).
func (s *Service) GetShipmentByOrderID(ctx context.Context, orderID string) (Shipment, error) {
	return s.repository.GetByOrderID(ctx, strings.TrimSpace(orderID))
}

// HandleWebhook is the entry point for `POST /webhooks/biteship`. It validates
// the webhook token (BR-043), records the raw payload for audit, and applies the
// tracking status transition. Idempotent: replaying the same event is a no-op.
func (s *Service) HandleWebhook(ctx context.Context, input WebhookInput) (Shipment, error) {
	if !s.verifyToken(input.Signature) {
		// Even invalid callbacks are recorded so we can investigate abuse.
		_, _, _ = s.repository.RecordEvent(ctx, ShipmentEvent{
			EventKey:        input.EventKey,
			BiteshipOrderID: input.BiteshipOrderID,
			TrackingNo:      input.TrackingNo,
			Event:           input.Event,
			EventStatus:     input.Status,
			Payload:         input.RawPayload,
			SignatureValid:  false,
		})
		return Shipment{}, ErrInvalidSignature
	}

	shipment, err := s.lookupShipment(ctx, input)
	if err != nil {
		return Shipment{}, err
	}

	event, duplicate, err := s.repository.RecordEvent(ctx, ShipmentEvent{
		ShipmentID:      shipment.ID,
		EventKey:        input.EventKey,
		BiteshipOrderID: input.BiteshipOrderID,
		TrackingNo:      input.TrackingNo,
		Event:           input.Event,
		EventStatus:     input.Status,
		Payload:         input.RawPayload,
		SignatureValid:  true,
	})
	if err != nil {
		return Shipment{}, err
	}
	if duplicate {
		// Biteship retries deliver the same event; short-circuit.
		return shipment, nil
	}

	updated, processErr := s.applyWebhook(ctx, shipment, input)
	processingError := ""
	if processErr != nil {
		processingError = processErr.Error()
	}
	if err := s.repository.MarkEventProcessed(ctx, event.ID, processingError); err != nil {
		return updated, err
	}
	if processErr != nil {
		return Shipment{}, processErr
	}
	return updated, nil
}

// CompleteDeliveredOrders advances orders that have been `delivered` for at
// least `after` to `completed` (BR-034). Intended to be called from a periodic
// worker. Returns the number of orders completed.
func (s *Service) CompleteDeliveredOrders(ctx context.Context, after time.Duration, limit int) (int, error) {
	if after <= 0 {
		after = 3 * 24 * time.Hour
	}
	if limit <= 0 {
		limit = 50
	}
	cutoff := s.now().Add(-after)
	shipments, err := s.repository.ListCompletable(ctx, cutoff, limit)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, shipment := range shipments {
		if err := s.repository.CompleteOrder(ctx, shipment.ID); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

// verifyToken uses constant-time compare so we don't leak byte-by-byte timing
// info about the configured secret. Biteship signs webhooks with a configurable
// token; when no token is configured we fail-closed (BR-043).

func (s *Service) verifyToken(received string) bool {
	if s.config.WebhookToken == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(received), []byte(s.config.WebhookToken)) == 1
}

func (s *Service) lookupShipment(ctx context.Context, input WebhookInput) (Shipment, error) {
	if strings.TrimSpace(input.BiteshipOrderID) != "" {
		shipment, err := s.repository.GetByBiteshipOrderID(ctx, input.BiteshipOrderID)
		if err == nil {
			return shipment, nil
		}
		if !errors.Is(err, ErrShipmentNotFound) {
			return Shipment{}, err
		}
	}
	if strings.TrimSpace(input.TrackingNo) != "" {
		return s.repository.GetByTrackingNo(ctx, input.TrackingNo)
	}
	return Shipment{}, ErrInvalidWebhookPayload
}

// applyWebhook maps a Biteship tracking status to a shipment/order transition.
//   - delivered → order delivered (BR-033); Completed happens later via worker (BR-034)
//   - picked/allocated/dropping/etc → order shipped (BR-030..BR-031)
//   - everything else → just store the raw status
func (s *Service) applyWebhook(ctx context.Context, shipment Shipment, input WebhookInput) (Shipment, error) {
	status := strings.ToLower(strings.TrimSpace(input.Status))
	switch status {
	case BiteshipStatusDelivered:
		if shipment.Status == StatusDelivered {
			return shipment, nil
		}
		delivered, err := s.repository.MarkDelivered(ctx, shipment.ID)
		if err != nil {
			return Shipment{}, err
		}
		s.publishOrderEvent(ctx, events.OrderDelivered, delivered)
		return delivered, nil
	case BiteshipStatusAllocated, BiteshipStatusPickingUp, BiteshipStatusPicked,
		BiteshipStatusDropping, BiteshipStatusOnHold:
		wasShipped := shipment.Status == StatusShipped
		shipped, err := s.repository.MarkShipped(ctx, shipment.ID, StatusShipped)
		if err != nil {
			return Shipment{}, err
		}
		// Only emit order.shipped on the first transition into shipped.
		if !wasShipped {
			s.publishOrderEvent(ctx, events.OrderShipped, shipped)
		}
		return shipped, nil

	case BiteshipStatusCancelled, BiteshipStatusRejected, BiteshipStatusReturned,
		BiteshipStatusCourierNotFound:
		return s.repository.UpdateStatus(ctx, shipment.ID, status)
	default:
		// Unknown/intermediate status — just record it without an order transition.
		if status == "" {
			return shipment, nil
		}
		return s.repository.UpdateStatus(ctx, shipment.ID, status)
	}
}

// publishOrderEvent emits an order lifecycle event. Best-effort: publishing is
// never allowed to fail the surrounding transition.
func (s *Service) publishOrderEvent(ctx context.Context, name string, shipment Shipment) {
	if s.publisher == nil {
		return
	}
	_ = s.publisher.Publish(ctx, events.NewOrderEvent(name, shipment.OrderID, map[string]any{
		"shipment_id": shipment.ID,
		"tracking_no": shipment.TrackingNo,
		"courier":     shipment.CourierCode,
		"status":      shipment.Status,
	}))
}

func (s *Service) buildCreateOrderRequest(order OrderSnapshot) biteship.CreateOrderRequest {

	items := make([]biteship.Item, 0, len(order.Items))
	for _, item := range order.Items {
		name := strings.TrimSpace(item.ProductName + " " + item.VariantName)
		if name == "" {
			name = "Item"
		}
		qty := item.Qty
		if qty <= 0 {
			qty = 1
		}
		items = append(items, biteship.Item{
			Name:     name,
			Value:    item.UnitPrice,
			Quantity: qty,
			Weight:   s.config.DefaultWeight,
		})
	}
	if len(items) == 0 {
		items = append(items, biteship.Item{Name: "Parcel", Value: 0, Quantity: 1, Weight: s.config.DefaultWeight})
	}

	return biteship.CreateOrderRequest{
		ShipperContactName:  s.config.OriginContactName,
		ShipperContactPhone: s.config.OriginContactPhone,

		OriginContactName:  s.config.OriginContactName,
		OriginContactPhone: s.config.OriginContactPhone,
		OriginAddress:      s.config.OriginAddress,
		OriginPostalCode:   s.config.OriginPostalCode,
		OriginNote:         s.config.OriginNote,

		DestinationContactName:  order.RecipientName,
		DestinationContactPhone: order.Phone,
		DestinationAddress:      order.AddressLine,
		DestinationPostalCode:   parsePostalCode(order.PostalCode),
		DestinationNote:         order.Notes,

		CourierCompany: order.CourierCode,
		CourierType:    order.CourierService,
		DeliveryType:   "now",
		OrderNote:      "Order " + order.OrderNo,
		ReferenceID:    order.OrderNo,

		Items: items,
	}
}

func isShippableStatus(status string) bool {
	switch status {
	case "paid", "processing", "shipped":
		return true
	default:
		return false
	}
}

// normaliseBookingStatus maps the status returned on a fresh Biteship order to
// our internal shipment status. A newly booked order is at least `booked`.
func normaliseBookingStatus(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "confirmed", "allocated":
		return StatusBooked
	case BiteshipStatusDelivered:
		return StatusDelivered
	default:
		return StatusBooked
	}
}

func parsePostalCode(raw string) int {
	code, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0
	}
	return code
}
