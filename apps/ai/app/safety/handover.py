"""Human-handover detection + admin notification (BR-003).

Some conversations must leave the bot and go to a human: explicit requests,
complaints/refunds/returns, price negotiation, payment or shipping trouble, bulk
orders, and runtime signals the bot can't handle (low confidence, data
unavailable after tool failure). This module answers two questions:

- *Should this turn hand over, and why?* — :func:`detect_handover` returns a
  :class:`HandoverReason` (or None) from the customer's message.
- *How do we tell a human?* — :class:`AdminNotifier` publishes a handover event
  the admin dashboard (later phase) can consume, so a person is alerted.

Once a session is in handover the WhatsApp service stays quiet (auto-reply off)
until a human resumes it. Detection here is deterministic keyword/phrase based,
mirroring the intent classifier so the two stay consistent.
"""

from __future__ import annotations

import re
import time
from enum import Enum

from app.platform.logging import get_logger

logger = get_logger(__name__)


class HandoverReason(str, Enum):
    """Why a conversation was escalated to a human (for the admin inbox/telemetry)."""

    CUSTOMER_REQUEST = "customer_request"  # asked for admin/human/CS
    COMPLAINT = "complaint"  # complaint / refund / return / damaged
    NEGOTIATION = "negotiation"  # price negotiation / discount haggling
    PAYMENT_ISSUE = "payment_issue"  # payment failed / stuck / double charge
    SHIPPING_ISSUE = "shipping_issue"  # lost / late / wrong item shipment problems
    BULK_ORDER = "bulk_order"  # wholesale / reseller / large quantity
    LOW_CONFIDENCE = "low_confidence"  # bot unsure / data unavailable (runtime)


# Message-driven triggers, ordered most- → least-specific so, e.g., a refund
# complaint isn't swallowed by a generic "admin" mention first. Phrases (not bare
# tokens) keep normal sales chat from tripping escalation.
_TRIGGERS: list[tuple[HandoverReason, tuple[str, ...]]] = [
    (
        HandoverReason.COMPLAINT,
        (
            "komplain",
            "keluhan",
            "refund",
            "pengembalian dana",
            "retur",
            "tukar barang",
            "barang rusak",
            "rusak",
            "cacat",
            "kecewa",
            "menipu",
            "penipuan",
        ),
    ),
    (
        HandoverReason.PAYMENT_ISSUE,
        (
            "sudah bayar tapi",
            "sudah transfer tapi",
            "pembayaran gagal",
            "bayar gagal",
            "double charge",
            "kena charge dua",
            "terpotong dua kali",
            "uang belum masuk",
            "salah transfer",
        ),
    ),
    (
        HandoverReason.SHIPPING_ISSUE,
        (
            "paket hilang",
            "barang belum sampai",
            "belum diterima",
            "salah kirim",
            "salah barang",
            "paket rusak",
            "resi tidak update",
            "resi ga update",
        ),
    ),
    (
        HandoverReason.NEGOTIATION,
        ("nego", "negosiasi", "kurang harganya", "boleh kurang", "minta diskon", "tawar"),
    ),
    (
        HandoverReason.BULK_ORDER,
        ("grosir", "reseller", "dropship", "borongan", "partai besar", "jumlah besar", "lusinan"),
    ),
    (
        HandoverReason.CUSTOMER_REQUEST,
        ("admin", "manusia", "orang asli", "customer service", "cs manusia", "bicara dengan"),
    ),
]


def _matches(keyword: str, text: str) -> bool:
    """Whole-word/phrase match so short tokens don't hit inside longer words."""
    return re.search(rf"(?<!\w){re.escape(keyword)}(?!\w)", text) is not None


def detect_handover(text: str | None) -> HandoverReason | None:
    """Return the :class:`HandoverReason` for ``text``, or None if no trigger.

    Deterministic and side-effect free; the caller decides what to do with the
    reason (flip the session, notify admin). Empty text never triggers.
    """
    if not text:
        return None
    lowered = text.lower()
    for reason, keywords in _TRIGGERS:
        if any(_matches(kw, lowered) for kw in keywords):
            return reason
    return None


# Customer-facing acknowledgement per reason. Warm, sets the expectation that a
# human will follow up; never promises an outcome (BR-003, no false assurances).
_ACK_REPLIES: dict[HandoverReason, str] = {
    HandoverReason.CUSTOMER_REQUEST: (
        "Baik, aku hubungkan kamu ke admin kami ya. Mohon tunggu sebentar 🙏"
    ),
    HandoverReason.COMPLAINT: (
        "Maaf banget atas kendalanya 🙏 Aku teruskan ke admin kami supaya "
        "dibantu langsung ya. Mohon tunggu sebentar."
    ),
    HandoverReason.NEGOTIATION: (
        "Untuk penawaran harga khusus, aku hubungkan ke admin kami ya. "
        "Mohon tunggu sebentar 🙏"
    ),
    HandoverReason.PAYMENT_ISSUE: (
        "Maaf ada kendala pembayaran 🙏 Aku teruskan ke admin kami untuk dicek "
        "langsung ya. Mohon tunggu sebentar."
    ),
    HandoverReason.SHIPPING_ISSUE: (
        "Maaf soal pengirimannya 🙏 Aku teruskan ke admin kami supaya ditelusuri "
        "ya. Mohon tunggu sebentar."
    ),
    HandoverReason.BULK_ORDER: (
        "Untuk pesanan jumlah besar, aku hubungkan ke admin kami ya biar dibantu "
        "penawaran terbaik. Mohon tunggu sebentar 🙏"
    ),
    HandoverReason.LOW_CONFIDENCE: (
        "Biar lebih pasti, aku hubungkan kamu ke admin kami ya. "
        "Mohon tunggu sebentar 🙏"
    ),
}

# Fallback used if a new reason is added without copy.
_DEFAULT_ACK = "Baik, aku teruskan ke admin kami ya. Mohon tunggu sebentar 🙏"


def acknowledgement(reason: HandoverReason) -> str:
    """Customer-facing reply to send when handing over for ``reason``."""
    return _ACK_REPLIES.get(reason, _DEFAULT_ACK)


class AdminNotifier:
    """Publishes handover alerts so a human is notified (BR-003).

    Backed by Redis: a per-session flag (so the dashboard can list open
    handovers) plus a capped list of recent handover events for an inbox view.
    Best-effort — a Redis hiccup must never block the customer-facing handover,
    so failures are logged and swallowed. A later phase replaces/augments the
    consumer (admin dashboard); the write contract stays stable.
    """

    _QUEUE_KEY = "handover:queue"
    _FLAG_PREFIX = "handover:open:"
    _MAX_QUEUE = 500

    def __init__(self, redis, *, flag_ttl_seconds: int = 604800):
        self._redis = redis
        self._flag_ttl = max(1, flag_ttl_seconds)

    async def notify(self, wa_id: str, reason: HandoverReason, *, note: str = "") -> None:
        """Record a handover for ``wa_id`` with ``reason`` (never raises)."""
        entry = {
            "wa_id": wa_id,
            "reason": reason.value,
            "note": note,
            "ts": int(time.time()),
        }
        try:
            import json

            await self._redis.lpush(self._QUEUE_KEY, json.dumps(entry))
            await self._redis.ltrim(self._QUEUE_KEY, 0, self._MAX_QUEUE - 1)
            await self._redis.set(
                f"{self._FLAG_PREFIX}{wa_id}", reason.value, ex=self._flag_ttl
            )
        except Exception:  # noqa: BLE001 — notification is best-effort
            logger.warning("handover_notify_failed", extra={"reason": reason.value})
        # Always log so the escalation is visible even if Redis is down. wa_id is
        # PII (phone); log a coarse marker only.
        logger.info(
            "handover_triggered",
            extra={"reason": reason.value, "wa_id_hash": _hash(wa_id)},
        )


def _hash(wa_id: str) -> str:
    return f"...{wa_id[-4:]}" if len(wa_id) >= 4 else "****"
