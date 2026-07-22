"""Abandoned-cart recovery sweep (Phase A9).

Runs periodically (scheduled by the caller) and, for each stored session, decides
whether to send a single gentle reminder. A session qualifies when:

- it has a cart reference (``cart_id``) with live items in the backend,
- it has been idle at least ``idle_seconds`` (abandoned, not actively chatting),
- it isn't in human handover (BR-003 — don't cut across a human), and
- it hasn't already received the max number of reminders (anti-spam,
  sales-rules "don't spam").

The sweep is deterministic and idempotent: the reminder count + last-reminded
timestamp are stored on the session context, so re-running the sweep won't
re-nudge a customer within the cooldown. Cart contents are always re-fetched from
the backend before messaging so a cart emptied/checked-out elsewhere doesn't get
a stale reminder (BR-004). Every reminder emits an analytics event (BR-050).

The whole sweep is best-effort: any per-session error is logged and skipped so
one bad session never aborts the run.
"""

from __future__ import annotations

import time

from app.analytics.events import AnalyticsEventType
from app.clients.backend import BackendClient
from app.conversation.models import Session, SessionStatus
from app.conversation.store import SessionStore
from app.platform.errors import BackendError
from app.platform.logging import get_logger
from app.whatsapp.models import OutboundMessage
from app.whatsapp.outbound import OutboundSender

logger = get_logger(__name__)

# Context keys for recovery bookkeeping (kept in ConversationContext.extra so no
# schema change is needed).
_REMINDER_COUNT_KEY = "recovery_reminder_count"
_LAST_REMINDER_KEY = "recovery_last_reminded_at"


def _reminder_text(name: str | None) -> str:
    """Friendly, non-pushy nudge (sales-rules: polite, no spam)."""
    greeting = f"Halo {name}! " if name else "Halo! "
    return (
        f"{greeting}Keranjang belanja kamu masih kami simpan 😊 "
        "Mau dilanjutkan ke pesanan? Ketik saja *lanjut* dan aku bantu prosesnya. "
        "Kalau ada yang mau ditanyakan soal produk atau ongkir, aku siap membantu."
    )


class CartRecoveryService:
    """Scans sessions and sends at-most-N reminders for idle carts."""

    def __init__(
        self,
        *,
        backend: BackendClient,
        sender: OutboundSender,
        sessions: SessionStore,
        analytics=None,
        idle_seconds: int = 3600,
        cooldown_seconds: int = 86400,
        max_reminders: int = 1,
    ):
        self._backend = backend
        self._sender = sender
        self._sessions = sessions
        self._analytics = analytics
        self._idle_seconds = max(1, idle_seconds)
        self._cooldown_seconds = max(1, cooldown_seconds)
        self._max_reminders = max(1, max_reminders)

    async def run_once(self, *, now: int | None = None) -> int:
        """Sweep all sessions once; return the number of reminders sent."""
        now = now if now is not None else int(time.time())
        sent = 0
        async for session in self._sessions.iter_sessions():
            try:
                if await self._maybe_remind(session, now=now):
                    sent += 1
            except Exception:  # noqa: BLE001 — one bad session must not abort the sweep
                logger.warning("recovery_session_error")
        if sent:
            logger.info("recovery_sweep_done", extra={"reminders_sent": sent})
        return sent

    async def _maybe_remind(self, session: Session, *, now: int) -> bool:
        """Send a reminder for one session if it qualifies. Returns True if sent."""
        if not self._is_candidate(session, now=now):
            return False

        # Re-check live cart contents: a cart emptied or checked out elsewhere
        # must not trigger a stale nudge (BR-004).
        if not await self._cart_has_items(session):
            return False

        text = _reminder_text(session.context.name)
        ok = await self._sender.send(OutboundMessage.text_message(session.wa_id, text))
        if not ok:
            logger.warning("recovery_send_failed")
            return False

        self._mark_reminded(session, now=now)
        await self._sessions.save(session)
        if self._analytics is not None:
            await self._analytics.emit(
                AnalyticsEventType.ABANDONED_CART_REMINDER_SENT,
                wa_id=session.wa_id,
                props={"reminder_no": session.context.extra.get(_REMINDER_COUNT_KEY)},
            )
        return True

    def _is_candidate(self, session: Session, *, now: int) -> bool:
        """Deterministic eligibility check (no I/O)."""
        ctx = session.context
        # Must have a cart to recover.
        if not ctx.cart_id:
            return False
        # Never cut across a human handover.
        if session.handover or session.status == SessionStatus.HANDOVER:
            return False
        # Must be idle long enough to count as abandoned.
        if (now - session.last_activity) < self._idle_seconds:
            return False
        # Respect the per-customer reminder cap.
        count = int(ctx.extra.get(_REMINDER_COUNT_KEY, 0) or 0)
        if count >= self._max_reminders:
            return False
        # Respect the cooldown between reminders.
        last = int(ctx.extra.get(_LAST_REMINDER_KEY, 0) or 0)
        if last and (now - last) < self._cooldown_seconds:
            return False
        return True

    async def _cart_has_items(self, session: Session) -> bool:
        """True if the customer's cart still has at least one item."""
        customer_id = session.context.customer_id
        if not customer_id:
            return False
        try:
            cart = await self._backend.get(f"/customers/{customer_id}/cart")
        except BackendError:
            # Backend hiccup: skip this session this round rather than guessing.
            return False
        if not isinstance(cart, dict):
            return False
        items = cart.get("items") or []
        return any(int((i or {}).get("qty") or 0) > 0 for i in items if isinstance(i, dict))

    def _mark_reminded(self, session: Session, *, now: int) -> None:
        count = int(session.context.extra.get(_REMINDER_COUNT_KEY, 0) or 0) + 1
        session.context.extra[_REMINDER_COUNT_KEY] = count
        session.context.extra[_LAST_REMINDER_KEY] = now
