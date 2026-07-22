"""Conversation state models.

These are the neutral, serializable shapes stored in Redis (see
:mod:`app.conversation.store`). They deliberately hold only *references* to
backend-owned data (``customer_id``, ``cart_id``, ``order_id``) plus lightweight
selection context — never authoritative prices/stock/totals, which always come
from the Go backend at call time (BR-004).
"""

from __future__ import annotations

import time
from dataclasses import asdict, dataclass, field
from enum import Enum
from typing import Any


class ConversationState(str, Enum):
    """Stages of the sales conversation (see roadmap Phase A3).

    The state machine guides *which tools are relevant* at each step; the final
    decision is made by the LLM (Phase A4). ``HANDOVER`` is reachable from any
    state (Phase A8) and pauses automated replies.
    """

    GREETING = "greeting"
    DISCOVERY = "discovery"
    PRODUCT = "product"
    CART = "cart"
    CUSTOMER_INFO = "customer_info"
    SHIPPING = "shipping"
    CHECKOUT_CONFIRM = "checkout_confirm"
    PAYMENT = "payment"
    POST_PAYMENT = "post_payment"
    DONE = "done"
    HANDOVER = "handover"


class SessionStatus(str, Enum):
    """Lifecycle status of a session, independent of conversation stage."""

    ACTIVE = "active"
    HANDOVER = "handover"
    EXPIRED = "expired"


# Forward flow used to detect "backward"/re-confirmation transitions. HANDOVER
# and DONE are terminal-ish and handled separately.
_FLOW_ORDER: list[ConversationState] = [
    ConversationState.GREETING,
    ConversationState.DISCOVERY,
    ConversationState.PRODUCT,
    ConversationState.CART,
    ConversationState.CUSTOMER_INFO,
    ConversationState.SHIPPING,
    ConversationState.CHECKOUT_CONFIRM,
    ConversationState.PAYMENT,
    ConversationState.POST_PAYMENT,
    ConversationState.DONE,
]


def flow_index(state: ConversationState) -> int:
    """Position of ``state`` in the forward sales flow, or -1 if off-flow."""
    try:
        return _FLOW_ORDER.index(state)
    except ValueError:
        return -1


@dataclass(slots=True)
class Turn:
    """A single message in the rolling history window."""

    role: str  # "user" | "agent"
    text: str
    ts: int = field(default_factory=lambda: int(time.time()))

    def to_dict(self) -> dict[str, Any]:
        return asdict(self)

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> Turn:
        return cls(role=data["role"], text=data.get("text", ""), ts=int(data.get("ts", 0)))


@dataclass(slots=True)
class ConversationContext:
    """Working memory for the sales flow — references + current selection.

    Mirrors ai-system-prompt "Conversation Context". Everything here is a hint
    or a backend reference; authoritative data is fetched from the backend when
    needed.
    """

    name: str | None = None
    customer_id: str | None = None
    # Current product/variant being discussed (ids resolved against backend).
    product_id: str | None = None
    variant_id: str | None = None
    quantity: int | None = None
    # Backend references established during the flow.
    cart_id: str | None = None
    address_id: str | None = None
    courier: str | None = None
    order_id: str | None = None
    invoice_id: str | None = None
    invoice_status: str | None = None
    # Free-form extras that future phases can stash without a schema change.
    extra: dict[str, Any] = field(default_factory=dict)

    def to_dict(self) -> dict[str, Any]:
        return asdict(self)

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> ConversationContext:
        known = {f for f in cls.__slots__}  # type: ignore[attr-defined]
        return cls(**{k: v for k, v in data.items() if k in known})


@dataclass(slots=True)
class Session:
    """A customer's conversation session, keyed by WhatsApp id (phone)."""

    wa_id: str
    status: SessionStatus = SessionStatus.ACTIVE
    state: ConversationState = ConversationState.GREETING
    handover: bool = False
    created_at: int = field(default_factory=lambda: int(time.time()))
    last_activity: int = field(default_factory=lambda: int(time.time()))
    context: ConversationContext = field(default_factory=ConversationContext)
    history: list[Turn] = field(default_factory=list)

    def touch(self, now: int | None = None) -> None:
        """Mark the session as active as of ``now`` (defaults to current time)."""
        self.last_activity = now if now is not None else int(time.time())

    def add_turn(self, role: str, text: str, *, max_history: int) -> None:
        """Append a message to history, trimming to the configured window."""
        if not text:
            return
        self.history.append(Turn(role=role, text=text))
        if max_history > 0 and len(self.history) > max_history:
            # Keep only the most recent ``max_history`` turns.
            self.history = self.history[-max_history:]

    def to_dict(self) -> dict[str, Any]:
        return {
            "wa_id": self.wa_id,
            "status": self.status.value,
            "state": self.state.value,
            "handover": self.handover,
            "created_at": self.created_at,
            "last_activity": self.last_activity,
            "context": self.context.to_dict(),
            "history": [t.to_dict() for t in self.history],
        }

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> Session:
        return cls(
            wa_id=data["wa_id"],
            status=SessionStatus(data.get("status", SessionStatus.ACTIVE.value)),
            state=ConversationState(data.get("state", ConversationState.GREETING.value)),
            handover=bool(data.get("handover", False)),
            created_at=int(data.get("created_at", 0)),
            last_activity=int(data.get("last_activity", 0)),
            context=ConversationContext.from_dict(data.get("context", {})),
            history=[Turn.from_dict(t) for t in data.get("history", [])],
        )
