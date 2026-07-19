// Package events implements Phase 10 domain-event publishing. Go is a publisher
// only: it emits order lifecycle events (order.paid, order.shipped, ...) that
// apps/ai consumes to send WhatsApp notifications (BR-041). The transport is
// RabbitMQ in production, but the domain code depends only on the Publisher
// interface so tests and offline runs use a log-backed no-op.
package events

import (
	"context"
	"encoding/json"
	"time"

	"go.uber.org/zap"
)

// Event names for the order lifecycle. Keep these stable — apps/ai binds queues
// to them.
const (
	OrderPaid      = "order.paid"
	OrderShipped   = "order.shipped"
	OrderDelivered = "order.delivered"
	OrderCompleted = "order.completed"
	OrderCancelled = "order.cancelled"
)

// Event is the envelope published to the message bus. Payload carries the
// event-specific data (e.g. order id, order no, customer id).
type Event struct {
	Name        string         `json:"name"`
	OccurredAt  time.Time      `json:"occurred_at"`
	AggregateID string         `json:"aggregate_id"`
	Payload     map[string]any `json:"payload"`
}

// Publisher publishes domain events. Implementations must be safe for concurrent
// use. Publishing is best-effort: a failure must never roll back the business
// action that produced the event.
type Publisher interface {
	Publish(ctx context.Context, event Event) error
	Close() error
}

// NewOrderEvent is a small constructor that stamps the occurrence time.
func NewOrderEvent(name, orderID string, payload map[string]any) Event {
	if payload == nil {
		payload = map[string]any{}
	}
	return Event{
		Name:        name,
		OccurredAt:  time.Now().UTC(),
		AggregateID: orderID,
		Payload:     payload,
	}
}

// LogPublisher is the default Publisher used when RabbitMQ is not configured. It
// records events to the application log so they're observable in development and
// nothing silently disappears. Swap for a real broker publisher in production.
type LogPublisher struct{ log *zap.Logger }

// NewLogPublisher returns a Publisher that logs events instead of enqueuing them.
func NewLogPublisher(log *zap.Logger) *LogPublisher { return &LogPublisher{log: log} }

// Publish records the event to the log at info level.
func (p *LogPublisher) Publish(_ context.Context, event Event) error {
	if p.log == nil {
		return nil
	}
	body, _ := json.Marshal(event.Payload)
	p.log.Info("domain event published",
		zap.String("event", event.Name),
		zap.String("aggregate_id", event.AggregateID),
		zap.Time("occurred_at", event.OccurredAt),
		zap.ByteString("payload", body),
	)
	return nil
}

// Close is a no-op for the log publisher.
func (p *LogPublisher) Close() error { return nil }
