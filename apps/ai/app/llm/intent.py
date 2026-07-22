"""Lightweight intent detection.

Maps an inbound message to one of the intents from ``sales-rules.md`` ("Customer
Intent Detection"). This is a fast, deterministic keyword classifier used to:

- hint the orchestrator which tools are most relevant for the turn, and
- give the rule-based provider (no external LLM) enough signal to act.

When a real LLM provider is configured it does its own understanding via
function calling; this classifier stays useful as a cheap pre-signal and for
telemetry (Phase A9). It intentionally does not compute or claim any domain
data (BR-004).
"""

from __future__ import annotations

import re
from enum import Enum


class Intent(str, Enum):
    GREETING = "greeting"
    PRODUCT_INQUIRY = "product_inquiry"
    PRICE_INQUIRY = "price_inquiry"
    COMPARISON = "comparison"
    RECOMMENDATION = "recommendation"
    CHECKOUT = "checkout"
    SHIPPING = "shipping"
    PAYMENT = "payment"
    ORDER_TRACKING = "order_tracking"
    COMPLAINT = "complaint"
    HANDOVER = "handover"
    GENERAL = "general"


# Ordered most-specific → least-specific. First matching group wins so that,
# e.g., "komplain" routes to COMPLAINT before a stray "produk" pulls it to
# PRODUCT_INQUIRY.
_KEYWORDS: list[tuple[Intent, tuple[str, ...]]] = [
    (Intent.HANDOVER, ("admin", "manusia", "cs", "customer service")),
    (Intent.COMPLAINT, ("komplain", "keluhan", "refund", "retur", "tukar", "rusak", "kecewa")),
    (
        Intent.ORDER_TRACKING,
        ("resi", "tracking", "lacak", "pesanan saya", "dimana pesanan", "status pesanan"),
    ),
    (Intent.PAYMENT, ("bayar", "pembayaran", "va", "qris", "transfer", "e-wallet", "ewallet")),
    (Intent.CHECKOUT, ("checkout", "konfirmasi", "lanjut pesan", "jadi order", "pesan sekarang")),
    (Intent.SHIPPING, ("ongkir", "ongkos kirim", "kurir", "pengiriman", "kirim ke", "alamat")),
    (Intent.COMPARISON, ("bandingkan", "beda", "vs", "lebih bagus", "atau")),
    (Intent.RECOMMENDATION, ("rekomendasi", "saran", "cocok", "bagusan mana", "yang mana")),
    (Intent.PRICE_INQUIRY, ("harga", "berapa", "diskon", "promo", "murah")),
    (
        Intent.PRODUCT_INQUIRY,
        ("produk", "cari", "stok", "ukuran", "warna", "varian", "ada", "punya"),
    ),
    (Intent.GREETING, ("halo", "hai", "hi", "pagi", "siang", "sore", "malam", "assalamualaikum")),
]


def _matches(keyword: str, text: str) -> bool:
    """Whole-word/phrase match so short tokens don't hit inside longer words.

    Substring matching wrongly classifies "kaos hitam" as a GREETING because
    "hi" is inside "hitam". Anchoring on word boundaries avoids that while still
    supporting multi-word phrases ("kirim ke") and hyphenated tokens ("e-wallet").
    """
    return re.search(rf"(?<!\w){re.escape(keyword)}(?!\w)", text) is not None


def detect_intent(text: str | None) -> Intent:
    """Classify ``text`` into an :class:`Intent`.

    Empty/None text (e.g. an image with no caption) is treated as GENERAL so the
    agent asks a clarifying question rather than guessing.
    """
    if not text:
        return Intent.GENERAL

    lowered = text.lower()
    for intent, keywords in _KEYWORDS:
        if any(_matches(kw, lowered) for kw in keywords):
            return intent
    return Intent.GENERAL

