"""Turn a domain event into a WhatsApp notification.

This is the transport-agnostic core of Phase A7 so it can be unit-tested without
a broker. Given a raw message body it:

1. parses the event envelope (drops poison messages),
2. renders the customer copy (skips events with nothing to say),
3. dedups so a redelivery never double-sends,
4. resolves the customer's WhatsApp number from the backend — the event carries
   only ``order_id``, so we look up order → customer → phone (backend is the
   source of truth; BR-004),
5. sends the message, and
6. best-effort syncs the conversation session (e.g. mark paid) so an in-flight
   chat stays consistent (BR-024).

:meth:`NotificationService.handle` returns a :class:`HandleResult` telling the
consumer whether to ack (done / drop) or retry (transient failure). It never
raises, so a bad event can't crash the consumer loop.
"""

from __future__ import annotations

from enum import Enum

from app.clients.backend import BackendClient
from app.conversation.models import SessionStatus
from app.conversation.store import SessionStore
from app.notifications import templates
from app.notifications.idempotency import EventDedup
from app.notifications.models import ORDER_PAID, OrderEvent
from app.platform.errors import BackendError
from app.platform.logging import get_logger
from app.whatsapp.models import OutboundMessage
from app.whatsapp.outbound import OutboundSender

logger = get_logger(__name__)


class HandleResult(str, Enum):
    """Outcome of handling one event, mapped to broker ack/nack by the consumer."""

    # Done or intentionally skipped (duplicate / nothing to send / unresolvable):
    # ack so the broker stops redelivering.
    ACK = "ack"
    # Transient failure (backend unreachable / send failed): nack + requeue so
    # the event is retried later.
    RETRY = "retry"


class NotificationService:
    """Renders + delivers WhatsApp notifications for order lifecycle events."""

    def __init__(
        self,
        *,
        backend: BackendClient,
        sender: OutboundSender,
        dedup: EventDedup,
        sessions: SessionStore | None = None,
    ):
        self._backend = backend
        self._sender = sender
        self._dedup = dedup
        self._sessions = sessions

    async def handle(self, body: bytes | str) -> HandleResult:
        """Process one raw event body. Never raises."""
        event = OrderEvent.from_bytes(body)
        if event is None:
            # Unparseable: acking drops it so we don't hot-loop on poison.
            logger.warning("notif_event_unparseable")
            return HandleResult.ACK

        message_text = templates.render(event)
        if message_text is None:
            # Nothing customer-facing for this event (e.g. order.completed).
            logger.info("notif_event_no_message", extra={"event": event.name})
            return HandleResult.ACK

        # Claim before sending so concurrent/redelivered copies don't both send.
        if not await self._dedup.claim(event.dedup_id):
            logger.info("notif_event_duplicate", extra={"event": event.name})
            return HandleResult.ACK

        try:
            phone = await self._resolve_phone(event.order_id)
        except BackendError:
            # Backend down/unreachable: retry later (release the claim first).
            await self._dedup.release(event.dedup_id)
            logger.warning("notif_resolve_backend_error", extra={"event": event.name})
            return HandleResult.RETRY

        if not phone:
            # Order/customer resolved but no usable number: retrying won't help.
            logger.warning("notif_no_recipient", extra={"event": event.name})
            return HandleResult.ACK

        sent = await self._sender.send(OutboundMessage.text_message(phone, message_text))
        if not sent:
            await self._dedup.release(event.dedup_id)
            logger.warning("notif_send_failed", extra={"event": event.name})
            return HandleResult.RETRY

        logger.info("notif_sent", extra={"event": event.name})
        await self._sync_session(phone, event)
        return HandleResult.ACK

    async def _resolve_phone(self, order_id: str) -> str | None:
        """Resolve the customer's WhatsApp number for an order.

        The event only carries the order id, so: order → customer_id → phone.
        A 404 (order/customer gone) is a normal "nothing to notify" outcome and
        returns None; transport/5xx errors propagate as :class:`BackendError`
        so the caller can retry.
        """
        try:
            order = await self._backend.get(f"/orders/{order_id}")
        except BackendError as exc:
            if exc.status_code == 404:
                return None
            raise
        customer_id = order.get("customer_id") if isinstance(order, dict) else None
        if not customer_id:
            return None
        try:
            customer = await self._backend.get(f"/customers/{customer_id}")
        except BackendError as exc:
            if exc.status_code == 404:
                return None
            raise
        phone = customer.get("phone") if isinstance(customer, dict) else None
        return phone.strip() if isinstance(phone, str) and phone.strip() else None

    async def _sync_session(self, wa_id: str, event: OrderEvent) -> None:
        """Best-effort: reflect the event on the customer's live session.

        Keeps an in-progress chat consistent with reality — e.g. after
        ``order.paid`` the session shouldn't still think payment is pending
        (BR-024). Never fails the notification: session state is a convenience,
        not the source of truth.
        """
        if self._sessions is None or event.name != ORDER_PAID:
            return
        try:
            session = await self._sessions.load(wa_id)
            if session is None:
                return
            session.context.invoice_status = "paid"
            session.status = SessionStatus.ACTIVE
            await self._sessions.save(session)
        except Exception:  # noqa: BLE001 — session sync is best-effort
            logger.warning("notif_session_sync_failed", extra={"event": event.name})
