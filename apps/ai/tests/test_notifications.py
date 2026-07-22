"""Tests for Phase A7: event-driven WhatsApp notifications.

Focus areas:
- Event parsing is defensive: poison/incomplete payloads never raise.
- Templates state only what the event asserts (paid/shipped/delivered), and skip
  events with nothing customer-facing (e.g. order.completed).
- The service resolves the recipient from the backend (order → customer → phone,
  BR-004), dedups redeliveries, and maps outcomes to ack/retry so a transient
  failure is retried while poison/duplicates are dropped.
- order.paid syncs the live session to paid so an in-flight chat stays consistent
  (BR-024). Payment status comes from the event, never asserted by the AI.
"""

from __future__ import annotations

import json
from typing import Any

from app.conversation.models import Session, SessionStatus
from app.notifications import templates
from app.notifications.idempotency import EventDedup
from app.notifications.models import OrderEvent
from app.notifications.service import HandleResult, NotificationService
from app.platform.errors import BackendError
from app.whatsapp.models import OutboundMessage
from app.whatsapp.outbound import OutboundSender

# --- fakes ------------------------------------------------------------------


class FakeRedis:
    """Minimal async Redis supporting SET NX EX + DELETE for dedup tests."""

    def __init__(self, *, fail: bool = False):
        self.store: dict[str, str] = {}
        self.fail = fail

    async def set(self, key: str, value: str, *, nx: bool = False, ex: int | None = None):
        if self.fail:
            raise RuntimeError("redis down")
        if nx and key in self.store:
            return None
        self.store[key] = value
        return True

    async def delete(self, key: str):
        self.store.pop(key, None)
        return 1

    async def get(self, key: str):
        return self.store.get(key)


class RecordingProvider:
    """WhatsApp provider stub that records sent messages (or fails N times)."""

    def __init__(self, fail_times: int = 0):
        self.sent: list[OutboundMessage] = []
        self.fail_times = fail_times

    async def send(self, message: OutboundMessage) -> None:
        if self.fail_times > 0:
            self.fail_times -= 1
            raise RuntimeError("provider boom")
        self.sent.append(message)


class RoutingBackend:
    """Fake backend routing GETs by path prefix; values may be BackendError."""

    def __init__(self, routes: dict[str, Any] | None = None):
        self.routes = routes or {}
        self.calls: list[str] = []

    async def get(self, path: str, *, params: dict | None = None) -> Any:
        self.calls.append(path)
        for prefix, value in self.routes.items():
            if path.startswith(prefix):
                if isinstance(value, BackendError):
                    raise value
                return value
        return None


class FakeSessionStore:
    """In-memory session store with the load/save surface the service uses."""

    def __init__(self, session: Session | None = None):
        self.sessions: dict[str, Session] = {}
        if session is not None:
            self.sessions[session.wa_id] = session
        self.saved: list[Session] = []

    async def load(self, wa_id: str) -> Session | None:
        return self.sessions.get(wa_id)

    async def save(self, session: Session) -> None:
        self.sessions[session.wa_id] = session
        self.saved.append(session)


def _service(
    *,
    backend: RoutingBackend,
    provider: RecordingProvider,
    redis: FakeRedis | None = None,
    sessions: FakeSessionStore | None = None,
) -> NotificationService:
    return NotificationService(
        backend=backend,  # type: ignore[arg-type]
        sender=OutboundSender(provider, max_retries=0),  # type: ignore[arg-type]
        dedup=EventDedup(redis or FakeRedis(), ttl_seconds=60),  # type: ignore[arg-type]
        sessions=sessions,  # type: ignore[arg-type]
    )


def _paid_body(order_id: str = "order-1", occurred_at: str = "2026-01-01T00:00:00Z") -> str:
    return json.dumps(
        {
            "name": "order.paid",
            "occurred_at": occurred_at,
            "aggregate_id": order_id,
            "payload": {"invoice_no": "INV-1", "amount": 118000, "payment_channel": "VA"},
        }
    )


def _shipped_body(order_id: str = "order-1") -> str:
    return json.dumps(
        {
            "name": "order.shipped",
            "occurred_at": "2026-01-02T00:00:00Z",
            "aggregate_id": order_id,
            "payload": {"tracking_no": "JNE123", "courier": "jne", "status": "shipped"},
        }
    )


def _resolves(order_id: str = "order-1", phone: str = "628123") -> RoutingBackend:
    return RoutingBackend(
        {
            f"/orders/{order_id}": {"id": order_id, "customer_id": "cust-1", "status": "paid"},
            "/customers/cust-1": {"id": "cust-1", "phone": phone, "name": "Budi"},
        }
    )


# --- event parsing ----------------------------------------------------------


def test_event_parse_defensive():
    assert OrderEvent.from_bytes("not json") is None
    assert OrderEvent.from_bytes(json.dumps([1, 2])) is None
    assert OrderEvent.from_bytes(json.dumps({"name": "order.paid"})) is None  # no order id
    assert OrderEvent.from_bytes(json.dumps({"aggregate_id": "x"})) is None  # no name

    event = OrderEvent.from_bytes(_paid_body())
    assert event is not None
    assert event.name == "order.paid"
    assert event.order_id == "order-1"
    # Non-dict payloads collapse to {} rather than blowing up.
    weird = OrderEvent.from_bytes(
        json.dumps({"name": "order.paid", "aggregate_id": "o", "payload": 5})
    )
    assert weird is not None and weird.payload == {}


def test_dedup_id_stable_per_emission():
    a = OrderEvent.from_bytes(_paid_body())
    b = OrderEvent.from_bytes(_paid_body())
    assert a is not None and b is not None
    assert a.dedup_id == b.dedup_id  # same publish → dedups
    later = OrderEvent.from_bytes(_paid_body(occurred_at="2026-02-02T00:00:00Z"))
    assert later is not None and later.dedup_id != a.dedup_id  # new emission differs


# --- templates --------------------------------------------------------------


def test_templates_render_expected_events():
    paid = templates.render(OrderEvent.from_bytes(_paid_body()))  # type: ignore[arg-type]
    assert paid is not None and "INV-1" in paid and "118.000" in paid

    shipped = templates.render(OrderEvent.from_bytes(_shipped_body()))  # type: ignore[arg-type]
    assert shipped is not None and "JNE123" in shipped and "JNE" in shipped


def test_templates_skip_events_with_nothing_to_say():
    completed = OrderEvent(name="order.completed", order_id="o")
    assert templates.render(completed) is None
    unknown = OrderEvent(name="order.whatever", order_id="o")
    assert templates.render(unknown) is None


# --- service flow -----------------------------------------------------------


async def test_handle_paid_resolves_recipient_and_sends():
    backend = _resolves()
    provider = RecordingProvider()
    service = _service(backend=backend, provider=provider)

    result = await service.handle(_paid_body())

    assert result is HandleResult.ACK
    assert len(provider.sent) == 1
    assert provider.sent[0].to == "628123"
    assert "INV-1" in (provider.sent[0].text or "")
    # Resolved via order → customer.
    assert backend.calls == ["/orders/order-1", "/customers/cust-1"]


async def test_handle_dedups_redelivery():
    backend = _resolves()
    provider = RecordingProvider()
    redis = FakeRedis()
    service = _service(backend=backend, provider=provider, redis=redis)

    first = await service.handle(_paid_body())
    second = await service.handle(_paid_body())  # exact redelivery

    assert first is HandleResult.ACK
    assert second is HandleResult.ACK
    assert len(provider.sent) == 1  # sent once


async def test_handle_backend_unreachable_retries_and_releases_claim():
    backend = RoutingBackend({"/orders/order-1": BackendError("down", status_code=502)})
    provider = RecordingProvider()
    redis = FakeRedis()
    service = _service(backend=backend, provider=provider, redis=redis)

    result = await service.handle(_paid_body())

    assert result is HandleResult.RETRY
    assert provider.sent == []
    # Claim was released so a later redelivery can try again.
    assert redis.store == {}


async def test_handle_send_failure_retries_and_releases_claim():
    backend = _resolves()
    provider = RecordingProvider(fail_times=99)  # always fails
    redis = FakeRedis()
    service = _service(backend=backend, provider=provider, redis=redis)

    result = await service.handle(_paid_body())

    assert result is HandleResult.RETRY
    assert redis.store == {}  # claim released for retry


async def test_handle_missing_order_acks_without_send():
    # Order 404 → nothing to notify; acking so we don't retry forever.
    backend = RoutingBackend({"/orders/order-1": BackendError("gone", status_code=404)})
    provider = RecordingProvider()
    service = _service(backend=backend, provider=provider)

    result = await service.handle(_paid_body())

    assert result is HandleResult.ACK
    assert provider.sent == []


async def test_handle_poison_message_acked():
    backend = _resolves()
    provider = RecordingProvider()
    service = _service(backend=backend, provider=provider)

    result = await service.handle("{ not valid json")

    assert result is HandleResult.ACK
    assert provider.sent == []
    assert backend.calls == []  # never even resolved a recipient


async def test_handle_no_message_event_acks_without_resolving():
    backend = _resolves()
    provider = RecordingProvider()
    service = _service(backend=backend, provider=provider)

    body = json.dumps(
        {"name": "order.completed", "aggregate_id": "order-1", "payload": {}}
    )
    result = await service.handle(body)

    assert result is HandleResult.ACK
    assert provider.sent == []
    assert backend.calls == []  # skipped before any backend lookup


async def test_paid_event_syncs_session_to_paid():
    backend = _resolves()
    provider = RecordingProvider()
    session = Session(wa_id="628123")
    session.context.invoice_status = "waiting_payment"
    sessions = FakeSessionStore(session)
    service = _service(backend=backend, provider=provider, sessions=sessions)

    await service.handle(_paid_body())

    saved = sessions.sessions["628123"]
    assert saved.context.invoice_status == "paid"
    assert saved.status is SessionStatus.ACTIVE


async def test_shipped_event_does_not_touch_session():
    backend = _resolves()
    provider = RecordingProvider()
    session = Session(wa_id="628123")
    sessions = FakeSessionStore(session)
    service = _service(backend=backend, provider=provider, sessions=sessions)

    await service.handle(_shipped_body())

    # Only order.paid syncs the session; shipped just notifies.
    assert sessions.saved == []
