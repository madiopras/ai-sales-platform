"""Analytics event vocabulary + envelope (Phase A9, BR-050).

A deliberately small, stable set of event types covering the sales funnel and
agent behaviour the admin dashboard needs to compute its metrics:

- conversation volume / new sessions,
- funnel steps (cart add, checkout preview, order created, order paid),
- drop-off (abandoned cart reminders sent),
- agent behaviour (handover rate, KB hits, injection blocks).

Keep the enum closed and named after *what happened*, not how it's charted; the
dashboard derives rates (conversion, abandonment, handover rate) from counts.
"""

from __future__ import annotations

import time
from dataclasses import dataclass, field
from enum import Enum
from typing import Any


class AnalyticsEventType(str, Enum):
    """Closed vocabulary of analytics events emitted by the AI service."""

    # conversation lifecycle
    CONVERSATION_STARTED = "conversation_started"
    MESSAGE_RECEIVED = "message_received"
    # sales funnel (mirrors backend truth; AI observes, doesn't compute totals)
    CART_ITEM_ADDED = "cart_item_added"
    CHECKOUT_PREVIEWED = "checkout_previewed"
    ORDER_CREATED = "order_created"
    ORDER_PAID = "order_paid"
    # recovery / retention
    CART_ABANDONED = "cart_abandoned"
    ABANDONED_CART_REMINDER_SENT = "abandoned_cart_reminder_sent"
    # agent behaviour / safety
    HANDOVER_TRIGGERED = "handover_triggered"
    KB_ANSWERED = "kb_answered"
    INJECTION_BLOCKED = "injection_blocked"
    RECOMMENDATION_MADE = "recommendation_made"


@dataclass(slots=True)
class AnalyticsEvent:
    """A single analytics event, serializable for the Redis sink + logs.

    ``wa_hash`` is a coarse, non-reversible marker for the customer (never the
    raw phone). ``props`` holds small, non-PII dimensions (e.g. order_id, amount,
    handover reason) the dashboard can group by.
    """

    type: AnalyticsEventType
    ts: int = field(default_factory=lambda: int(time.time()))
    wa_hash: str | None = None
    props: dict[str, Any] = field(default_factory=dict)

    def to_dict(self) -> dict[str, Any]:
        return {
            "type": self.type.value,
            "ts": self.ts,
            "wa_hash": self.wa_hash,
            "props": self.props,
        }


def hash_wa_id(wa_id: str | None) -> str | None:
    """Reduce a WhatsApp id (phone, PII) to a coarse marker for analytics/logs."""
    if not wa_id:
        return None
    return f"...{wa_id[-4:]}" if len(wa_id) >= 4 else "****"
