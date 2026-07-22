"""Conversation state machine + session lifecycle.

The manager is the single entry point the WhatsApp service uses to turn an
inbound message into an updated session. It:

1. loads (or creates) the session for a WhatsApp id,
2. applies the idle-timeout policy,
3. records the inbound turn in the rolling history,
4. advances the conversation state (the *state machine*), and
5. persists the result with a refreshed TTL.

Handover (Phase A8): detection lives in :mod:`app.safety.handover`. When a turn
trips a handover trigger the session flips to :class:`SessionStatus.HANDOVER`,
the reason is stashed on the context, and (best-effort) an admin is notified.
Once in handover the WhatsApp service stays quiet so a human can take over.

The manager never computes domain data (price/stock/totals) — that always comes
from the backend (BR-004).
"""

from __future__ import annotations

import time

from app.conversation.models import (
    ConversationState,
    Session,
    SessionStatus,
    flow_index,
)
from app.conversation.store import SessionStore
from app.platform.logging import get_logger
from app.safety.handover import AdminNotifier, HandoverReason, detect_handover

logger = get_logger(__name__)

# Very light intent hints → target state. This is a placeholder for Phase A4's
# LLM intent detection; it only needs to be good enough to exercise and test the
# state machine wiring.
_INTENT_KEYWORDS: list[tuple[tuple[str, ...], ConversationState]] = [
    (("bayar", "pembayaran", "va", "qris", "transfer"), ConversationState.PAYMENT),
    (("checkout", "konfirmasi", "lanjut pesan", "order"), ConversationState.CHECKOUT_CONFIRM),
    (("ongkir", "kirim", "kurir", "alamat"), ConversationState.SHIPPING),
    (("keranjang", "cart", "tambah", "beli"), ConversationState.CART),
    (("cari", "produk", "harga", "stok", "ukuran", "warna"), ConversationState.PRODUCT),
]

# Context key where the handover reason is stashed for the reply layer + admin.
HANDOVER_REASON_KEY = "handover_reason"


class ConversationManager:
    """Owns session load → update → persist and the state transitions."""

    def __init__(
        self,
        store: SessionStore,
        *,
        idle_timeout_seconds: int,
        history_max: int,
        admin_notifier: AdminNotifier | None = None,
    ):
        self._store = store
        self._idle_timeout = max(1, idle_timeout_seconds)
        self._history_max = max(0, history_max)
        self._admin_notifier = admin_notifier

    async def get_or_create(self, wa_id: str, *, now: int | None = None) -> Session:
        """Load the session for ``wa_id``, expiring stale ones, else create new."""
        now = now if now is not None else int(time.time())
        session = await self._store.load(wa_id)
        if session is None:
            return Session(wa_id=wa_id, created_at=now, last_activity=now)
        if self._is_expired(session, now):
            # Idle too long: start a clean session but keep the known name so the
            # greeting can stay personal.
            logger.info("session_expired", extra={"wa_id_hash": _hash(wa_id)})
            fresh = Session(wa_id=wa_id, created_at=now, last_activity=now)
            fresh.context.name = session.context.name
            fresh.context.customer_id = session.context.customer_id
            return fresh
        return session

    async def record_inbound(
        self,
        wa_id: str,
        text: str | None,
        *,
        name: str | None = None,
        now: int | None = None,
    ) -> Session:
        """Advance the session for an inbound message and persist it.

        Returns the updated session so the caller (WhatsApp service / LLM
        orchestrator) can decide how to reply based on ``session.state`` and
        ``session.handover``.
        """
        now = now if now is not None else int(time.time())
        session = await self.get_or_create(wa_id, now=now)

        if name and not session.context.name:
            session.context.name = name

        session.add_turn("user", text or "", max_history=self._history_max)

        # Only evaluate handover/transitions while the bot is still driving. Once
        # handed over we stay quiet until a human resumes (see resume_from_handover).
        if session.status != SessionStatus.HANDOVER:
            reason = detect_handover(text or "")
            if reason is not None:
                await self._enter_handover(session, reason)
            else:
                session.state = self._next_state(session, text or "")

        session.status = (
            SessionStatus.HANDOVER if session.handover else SessionStatus.ACTIVE
        )
        session.touch(now)
        await self._store.save(session)
        return session

    async def record_agent_reply(
        self, session: Session, text: str, *, now: int | None = None
    ) -> None:
        """Record the agent's outgoing message in history and persist."""
        session.add_turn("agent", text, max_history=self._history_max)
        session.touch(now if now is not None else int(time.time()))
        await self._store.save(session)

    async def set_handover(
        self,
        wa_id: str,
        *,
        reason: HandoverReason = HandoverReason.CUSTOMER_REQUEST,
        now: int | None = None,
    ) -> Session:
        """Force a session into human-handover (runtime triggers, e.g. low confidence)."""
        session = await self.get_or_create(wa_id, now=now)
        if session.status != SessionStatus.HANDOVER:
            await self._enter_handover(session, reason)
        session.touch(now if now is not None else int(time.time()))
        await self._store.save(session)
        return session

    async def resume_from_handover(self, wa_id: str, *, now: int | None = None) -> Session | None:
        """Return a handed-over session to bot control (admin resolved it).

        Clears the handover flag/reason and drops back to DISCOVERY so the bot
        can help again. Returns None if there's no session to resume.
        """
        session = await self._store.load(wa_id)
        if session is None:
            return None
        session.handover = False
        session.status = SessionStatus.ACTIVE
        session.state = ConversationState.DISCOVERY
        session.context.extra.pop(HANDOVER_REASON_KEY, None)
        session.touch(now if now is not None else int(time.time()))
        await self._store.save(session)
        logger.info("session_resumed", extra={"wa_id_hash": _hash(wa_id)})
        return session

    # --- internals -----------------------------------------------------------

    async def _enter_handover(self, session: Session, reason: HandoverReason) -> None:
        """Flip a session into handover, stash the reason, and notify admin."""
        session.state = ConversationState.HANDOVER
        session.status = SessionStatus.HANDOVER
        session.handover = True
        session.context.extra[HANDOVER_REASON_KEY] = reason.value
        logger.info(
            "session_handover",
            extra={"wa_id_hash": _hash(session.wa_id), "reason": reason.value},
        )
        if self._admin_notifier is not None:
            await self._admin_notifier.notify(session.wa_id, reason)

    def _is_expired(self, session: Session, now: int) -> bool:
        return (now - session.last_activity) >= self._idle_timeout

    def _next_state(self, session: Session, text: str) -> ConversationState:
        """Deterministic transition (Phase A3 placeholder for LLM intent).

        Rules:
        - An explicit intent keyword jumps to that stage, but we never skip
          *backwards* silently past the current stage unless the customer asks
          (keyword match counts as asking).
        - Otherwise nudge one step forward from GREETING → DISCOVERY so a first
          "halo" moves the conversation along.

        Handover is handled by the caller (:meth:`record_inbound`) before this.
        """
        lowered = text.lower()

        for keywords, target in _INTENT_KEYWORDS:
            if any(kw in lowered for kw in keywords):
                return target

        if session.state == ConversationState.GREETING:
            return ConversationState.DISCOVERY

        # No new signal: stay where we are (unless off-flow, then reset sanely).
        return session.state if flow_index(session.state) >= 0 else ConversationState.DISCOVERY


def _hash(wa_id: str) -> str:
    return f"...{wa_id[-4:]}" if len(wa_id) >= 4 else "****"
