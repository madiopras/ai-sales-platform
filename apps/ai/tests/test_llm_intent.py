"""Tests for the keyword intent classifier."""

from __future__ import annotations

import pytest

from app.llm.intent import Intent, detect_intent


@pytest.mark.parametrize(
    ("text", "expected"),
    [
        ("halo kak", Intent.GREETING),
        ("ada kaos hitam ukuran XL?", Intent.PRODUCT_INQUIRY),
        ("berapa harga celana jeans", Intent.PRICE_INQUIRY),
        ("rekomendasi dong yang cocok buat lari", Intent.RECOMMENDATION),
        ("bandingkan yang A atau B", Intent.COMPARISON),
        ("aku mau checkout sekarang", Intent.CHECKOUT),
        ("tolong hitung ongkir ke Bandung", Intent.SHIPPING),
        ("bayar pakai qris bisa?", Intent.PAYMENT),
        ("pesanan saya dimana ya, minta resi", Intent.ORDER_TRACKING),
        ("barang rusak, aku mau refund", Intent.COMPLAINT),
        ("mau ngobrol sama admin", Intent.HANDOVER),
    ],
)
def test_detect_intent(text, expected):
    assert detect_intent(text) == expected


def test_empty_text_is_general():
    assert detect_intent(None) == Intent.GENERAL
    assert detect_intent("") == Intent.GENERAL


def test_complaint_beats_product_keyword():
    # "produk" appears but complaint keyword must win (ordering guarantee).
    assert detect_intent("produk ini rusak, mau komplain") == Intent.COMPLAINT
