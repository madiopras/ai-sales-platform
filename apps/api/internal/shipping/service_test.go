package shipping

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/prasdios/ai-sales-platform/apps/api/internal/shipping/biteship"
)

const testWebhookToken = "test-webhook-token"

func newTestService(repo *memoryRepo, client BiteshipClient) *Service {
	return NewService(repo, client, Config{
		WebhookToken:     testWebhookToken,
		DefaultCouriers:  "jne,jnt",
		DefaultWeight:    1000,
		OriginPostalCode: 12345,
	})
}

func TestGetRatesReturnsCourierOptions(t *testing.T) {
	repo := &memoryRepo{}
	client := &fakeBiteship{rates: biteship.RatesResponse{
		Success: true,
		Pricing: []biteship.PricingRate{
			{CourierCode: "jne", CourierServiceCode: "reg", CourierName: "JNE", Price: 18000},
			{CourierCode: "sicepat", CourierServiceCode: "best", CourierName: "SiCepat", Price: 17000},
		},
	}}
	service := newTestService(repo, client)

	rates, err := service.GetRates(context.Background(), RateInput{DestinationPostalCode: 40123})
	if err != nil {
		t.Fatal(err)
	}
	if len(rates) != 2 {
		t.Fatalf("expected 2 rates, got %d", len(rates))
	}
	if rates[0].CourierCode != "jne" || rates[0].Price != 18000 {
		t.Fatalf("unexpected first rate: %+v", rates[0])
	}
	// Origin should fall back to config when not supplied.
	if client.lastRates.OriginPostalCode != 12345 {
		t.Fatalf("expected origin fallback 12345, got %d", client.lastRates.OriginPostalCode)
	}
}

func TestGetRatesRequiresDestination(t *testing.T) {
	service := newTestService(&memoryRepo{}, &fakeBiteship{})
	_, err := service.GetRates(context.Background(), RateInput{})
	if !errors.Is(err, ErrAddressIncomplete) {
		t.Fatalf("expected ErrAddressIncomplete, got %v", err)
	}
}

func TestGetRatesMapsBiteshipFailure(t *testing.T) {
	service := newTestService(&memoryRepo{}, &fakeBiteship{ratesErr: errors.New("boom")})
	_, err := service.GetRates(context.Background(), RateInput{DestinationPostalCode: 40123})
	if !errors.Is(err, ErrBiteshipFailure) {
		t.Fatalf("expected ErrBiteshipFailure, got %v", err)
	}
}

func TestGetRatesEmptyPricingIsUnavailable(t *testing.T) {
	service := newTestService(&memoryRepo{}, &fakeBiteship{rates: biteship.RatesResponse{Success: true}})
	_, err := service.GetRates(context.Background(), RateInput{DestinationPostalCode: 40123})
	if !errors.Is(err, ErrRatesUnavailable) {
		t.Fatalf("expected ErrRatesUnavailable, got %v", err)
	}
}

func TestCreateShipmentRejectsNonShippableOrder(t *testing.T) {
	repo := &memoryRepo{order: OrderSnapshot{ID: "ord-1", Status: "pending_payment", CourierCode: "jne"}}
	service := newTestService(repo, &fakeBiteship{})
	_, err := service.CreateShipment(context.Background(), CreateShipmentInput{OrderID: "ord-1"})
	if !errors.Is(err, ErrOrderNotShippable) {
		t.Fatalf("expected ErrOrderNotShippable, got %v", err)
	}
}

func TestCreateShipmentBooksAtBiteshipAndAttachesReferences(t *testing.T) {
	repo := &memoryRepo{order: OrderSnapshot{
		ID: "ord-1", OrderNo: "ORD-1", Status: "paid",
		CourierCode: "jne", CourierService: "reg", ShippingFee: 18000,
		RecipientName: "Budi", Phone: "0811", AddressLine: "Jl. Merdeka", PostalCode: "40123",
		Items: []OrderItemSnapshot{{ProductName: "Kaos", VariantName: "XL", Qty: 2, UnitPrice: 100000}},
	}}
	client := &fakeBiteship{order: biteship.Order{
		Success: true, ID: "bts-1", Status: "confirmed",
		Courier: biteship.Courier{WaybillID: "JP123", Link: "https://label"},
	}}
	service := newTestService(repo, client)

	shipment, err := service.CreateShipment(context.Background(), CreateShipmentInput{OrderID: "ord-1"})
	if err != nil {
		t.Fatal(err)
	}
	if shipment.BiteshipOrderID != "bts-1" {
		t.Fatalf("expected biteship id attached, got %q", shipment.BiteshipOrderID)
	}
	if shipment.TrackingNo != "JP123" {
		t.Fatalf("expected tracking JP123, got %q", shipment.TrackingNo)
	}
	if client.lastOrder.CourierCompany != "jne" || client.lastOrder.CourierType != "reg" {
		t.Fatalf("unexpected courier sent: %+v", client.lastOrder)
	}
	if client.lastOrder.ReferenceID != "ORD-1" {
		t.Fatalf("expected reference ORD-1, got %q", client.lastOrder.ReferenceID)
	}
}

func TestCreateShipmentIsIdempotent(t *testing.T) {
	repo := &memoryRepo{
		order:    OrderSnapshot{ID: "ord-1", OrderNo: "ORD-1", Status: "paid", CourierCode: "jne"},
		shipment: Shipment{ID: "shp-1", OrderID: "ord-1", BiteshipOrderID: "bts-1", TrackingNo: "JP123", Status: StatusBooked},
	}
	client := &fakeBiteship{}
	service := newTestService(repo, client)

	shipment, err := service.CreateShipment(context.Background(), CreateShipmentInput{OrderID: "ord-1"})
	if err != nil {
		t.Fatal(err)
	}
	if shipment.BiteshipOrderID != "bts-1" {
		t.Fatalf("expected existing shipment returned, got %q", shipment.BiteshipOrderID)
	}
	if client.orderCalls != 0 {
		t.Fatalf("expected Biteship not to be called for already-booked shipment, got %d", client.orderCalls)
	}
}

func TestCreateShipmentMapsBiteshipFailure(t *testing.T) {
	repo := &memoryRepo{order: OrderSnapshot{ID: "ord-1", OrderNo: "ORD-1", Status: "paid", CourierCode: "jne"}}
	client := &fakeBiteship{orderErr: errors.New("boom")}
	service := newTestService(repo, client)

	_, err := service.CreateShipment(context.Background(), CreateShipmentInput{OrderID: "ord-1"})
	if !errors.Is(err, ErrBiteshipFailure) {
		t.Fatalf("expected ErrBiteshipFailure, got %v", err)
	}
}

func TestHandleWebhookRejectsInvalidToken(t *testing.T) {
	repo := &memoryRepo{}
	service := newTestService(repo, &fakeBiteship{})
	_, err := service.HandleWebhook(context.Background(), WebhookInput{
		BiteshipOrderID: "bts-1", Status: "delivered", Signature: "wrong",
	})
	if !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("expected ErrInvalidSignature, got %v", err)
	}
	if !repo.eventRecorded {
		t.Fatal("expected invalid callback to still be recorded for audit")
	}
}

func TestHandleWebhookMarksShipped(t *testing.T) {
	repo := &memoryRepo{shipment: Shipment{ID: "shp-1", OrderID: "ord-1", BiteshipOrderID: "bts-1", Status: StatusBooked}}
	service := newTestService(repo, &fakeBiteship{})
	shipment, err := service.HandleWebhook(context.Background(), WebhookInput{
		EventKey: "bts-1:picked", BiteshipOrderID: "bts-1", Status: BiteshipStatusPicked, Signature: testWebhookToken,
	})
	if err != nil {
		t.Fatal(err)
	}
	if shipment.Status != StatusShipped {
		t.Fatalf("expected shipped, got %q", shipment.Status)
	}
	if repo.markShippedCalls != 1 {
		t.Fatalf("expected 1 MarkShipped call, got %d", repo.markShippedCalls)
	}
}

func TestHandleWebhookMarksDelivered(t *testing.T) {
	repo := &memoryRepo{shipment: Shipment{ID: "shp-1", OrderID: "ord-1", BiteshipOrderID: "bts-1", Status: StatusShipped}}
	service := newTestService(repo, &fakeBiteship{})
	shipment, err := service.HandleWebhook(context.Background(), WebhookInput{
		EventKey: "bts-1:delivered", BiteshipOrderID: "bts-1", Status: BiteshipStatusDelivered, Signature: testWebhookToken,
	})
	if err != nil {
		t.Fatal(err)
	}
	if shipment.Status != StatusDelivered {
		t.Fatalf("expected delivered, got %q", shipment.Status)
	}
}

func TestHandleWebhookIsIdempotentOnDuplicateEvent(t *testing.T) {
	repo := &memoryRepo{
		shipment:  Shipment{ID: "shp-1", OrderID: "ord-1", BiteshipOrderID: "bts-1", Status: StatusBooked},
		duplicate: true,
	}
	service := newTestService(repo, &fakeBiteship{})
	_, err := service.HandleWebhook(context.Background(), WebhookInput{
		EventKey: "bts-1:picked", BiteshipOrderID: "bts-1", Status: BiteshipStatusPicked, Signature: testWebhookToken,
	})
	if err != nil {
		t.Fatal(err)
	}
	if repo.markShippedCalls != 0 {
		t.Fatalf("expected MarkShipped skipped for duplicate, got %d calls", repo.markShippedCalls)
	}
}

func TestCompleteDeliveredOrders(t *testing.T) {
	repo := &memoryRepo{completable: []Shipment{
		{ID: "shp-1", OrderID: "ord-1"},
		{ID: "shp-2", OrderID: "ord-2"},
	}}
	service := newTestService(repo, &fakeBiteship{})
	count, err := service.CompleteDeliveredOrders(context.Background(), 72*time.Hour, 10)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("expected 2 completed, got %d", count)
	}
	if repo.completeCalls != 2 {
		t.Fatalf("expected 2 CompleteOrder calls, got %d", repo.completeCalls)
	}
}

// --- test doubles ---

type fakeBiteship struct {
	rates      biteship.RatesResponse
	ratesErr   error
	order      biteship.Order
	orderErr   error
	orderCalls int
	lastRates  biteship.RatesRequest
	lastOrder  biteship.CreateOrderRequest
}

func (f *fakeBiteship) GetRates(_ context.Context, req biteship.RatesRequest) (biteship.RatesResponse, error) {
	f.lastRates = req
	if f.ratesErr != nil {
		return biteship.RatesResponse{}, f.ratesErr
	}
	return f.rates, nil
}

func (f *fakeBiteship) CreateOrder(_ context.Context, req biteship.CreateOrderRequest) (biteship.Order, error) {
	f.orderCalls++
	f.lastOrder = req
	if f.orderErr != nil {
		return biteship.Order{}, f.orderErr
	}
	return f.order, nil
}

type memoryRepo struct {
	order    OrderSnapshot
	shipment Shipment

	completable []Shipment
	duplicate   bool

	eventRecorded    bool
	markShippedCalls int
	markDeliveredCls int
	completeCalls    int
}

func (r *memoryRepo) GetOrderForShipment(_ context.Context, orderID string) (OrderSnapshot, error) {
	if r.order.ID == orderID {
		return r.order, nil
	}
	return OrderSnapshot{}, ErrOrderNotFound
}

func (r *memoryRepo) CreateShipment(_ context.Context, shipment Shipment) (Shipment, error) {
	shipment.ID = "shp-new"
	r.shipment = shipment
	return shipment, nil
}

func (r *memoryRepo) AttachBiteshipReferences(_ context.Context, shipmentID string, refs BiteshipReferences) (Shipment, error) {
	r.shipment.ID = shipmentID
	r.shipment.BiteshipOrderID = refs.BiteshipOrderID
	r.shipment.TrackingNo = refs.TrackingNo
	r.shipment.WaybillID = refs.WaybillID
	r.shipment.LabelURL = refs.LabelURL
	r.shipment.Status = refs.Status
	return r.shipment, nil
}

func (r *memoryRepo) GetByID(_ context.Context, id string) (Shipment, error) {
	if r.shipment.ID == id && r.shipment.ID != "" {
		return r.shipment, nil
	}
	return Shipment{}, ErrShipmentNotFound
}

func (r *memoryRepo) GetByOrderID(_ context.Context, orderID string) (Shipment, error) {
	if r.shipment.OrderID == orderID && r.shipment.ID != "" {
		return r.shipment, nil
	}
	return Shipment{}, ErrShipmentNotFound
}

func (r *memoryRepo) GetByBiteshipOrderID(_ context.Context, biteshipOrderID string) (Shipment, error) {
	if r.shipment.BiteshipOrderID == biteshipOrderID && r.shipment.ID != "" {
		return r.shipment, nil
	}
	return Shipment{}, ErrShipmentNotFound
}

func (r *memoryRepo) GetByTrackingNo(_ context.Context, trackingNo string) (Shipment, error) {
	if r.shipment.TrackingNo == trackingNo && r.shipment.ID != "" {
		return r.shipment, nil
	}
	return Shipment{}, ErrShipmentNotFound
}

func (r *memoryRepo) UpdateStatus(_ context.Context, _ string, status string) (Shipment, error) {
	r.shipment.Status = status
	return r.shipment, nil
}

func (r *memoryRepo) MarkShipped(_ context.Context, _ string, status string) (Shipment, error) {
	r.markShippedCalls++
	r.shipment.Status = status
	return r.shipment, nil
}

func (r *memoryRepo) MarkDelivered(_ context.Context, _ string) (Shipment, error) {
	r.markDeliveredCls++
	r.shipment.Status = StatusDelivered
	return r.shipment, nil
}

func (r *memoryRepo) ListCompletable(_ context.Context, _ time.Time, _ int) ([]Shipment, error) {
	return r.completable, nil
}

func (r *memoryRepo) CompleteOrder(_ context.Context, _ string) error {
	r.completeCalls++
	return nil
}

func (r *memoryRepo) RecordEvent(_ context.Context, event ShipmentEvent) (ShipmentEvent, bool, error) {
	r.eventRecorded = true
	event.ID = "evt-row-1"
	return event, r.duplicate, nil
}

func (r *memoryRepo) MarkEventProcessed(_ context.Context, _ string, _ string) error {
	return nil
}
