"""Domain-event envelope, mirrored from the Go publisher.

The backend publishes ``events.Event`` (see ``apps/api/internal/events/events.go``)
as JSON::

    {
      "name": "order.paid",
      "occurred_at": "2026-01-01T00:00:00Z",
      "aggregate_id": "<order id>",
      "payload": { ... event-specific ... }
    }

We keep the shape neutral and defensive: unknown/missing fields never raise, so
a malformed or future-versioned message degrades gracefully instead of crashing
the consumer. ``aggregate_id`` is the order id for every order.* event.
"""

from __future__ import annotations

import json
from dataclasses import dataclass, field
from typing import Any

# Event names — must match events.go constants (they double as routing keys).
ORDER_PAID = "order.paid"
ORDER_SHIPPED = "order.shipped"
ORDER_DELIVERED = "order.delivered"
ORDER_COMPLETED = "order.completed"
ORDER_CANCELLED = "order.cancelled"


@dataclass(slots=True)
class OrderEvent:
    """A parsed domain event. ``order_id`` aliases the envelope aggregate id."""

    name: str
    order_id: str
    occurred_at: str | None = None
    payload: dict[str, Any] = field(default_factory=dict)

    @property
    def dedup_id(self) -> str:
        """Stable key for idempotency.

        The envelope has no event id, so we derive one from the fields that make
        a given emission unique: name + order + occurrence time. Redelivery of
        the same publish repeats all three, so it dedups; a genuinely new event
        (e.g. a later status change) differs on ``occurred_at``.
        """
        return f"{self.name}:{self.order_id}:{self.occurred_at or ''}"

    @classmethod
    def from_bytes(cls, body: bytes | str) -> OrderEvent | None:
        """Parse a raw message body into an event, or None if unusable.

        Returns None (rather than raising) for non-JSON, non-object, or
        nameless/order-less payloads so the consumer can ack-and-drop poison
        messages instead of hot-looping on them.
        """
        try:
            data = json.loads(body)
        except (ValueError, TypeError):
            return None
        if not isinstance(data, dict):
            return None
        name = str(data.get("name") or "").strip()
        order_id = str(data.get("aggregate_id") or "").strip()
        if not name or not order_id:
            return None
        payload = data.get("payload")
        if not isinstance(payload, dict):
            payload = {}
        occurred_at = data.get("occurred_at")
        return cls(
            name=name,
            order_id=order_id,
            occurred_at=str(occurred_at) if occurred_at else None,
            payload=payload,
        )
