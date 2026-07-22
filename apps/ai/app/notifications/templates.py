"""Event → customer-facing WhatsApp copy.

Messages are short, in Indonesian, and match the sales agent's voice. They state
only what the backend event asserts (BR-024, BR-027, BR-031); we never claim a
status the event didn't carry. Amounts/resi come from the payload as-is — the AI
does not compute or reformat money beyond thousands separators.

A template returns ``None`` when there's nothing worth notifying for that event,
so the service can ack-and-skip without sending noise.
"""

from __future__ import annotations

from typing import Any

from app.notifications.models import (
    ORDER_CANCELLED,
    ORDER_DELIVERED,
    ORDER_PAID,
    ORDER_SHIPPED,
    OrderEvent,
)


def _order_ref(event: OrderEvent) -> str:
    """Prefer a human order/invoice number from the payload, else the id tail."""
    for key in ("order_no", "invoice_no"):
        value = event.payload.get(key)
        if value:
            return str(value)
    return f"#{event.order_id[-6:]}" if event.order_id else ""


def _format_amount(value: Any) -> str | None:
    """Format a rupiah amount with thousands separators, e.g. ``118.000``."""
    try:
        number = float(value)
    except (TypeError, ValueError):
        return None
    if number <= 0:
        return None
    return f"{int(round(number)):,}".replace(",", ".")


def render(event: OrderEvent) -> str | None:
    """Render the WhatsApp message body for an event, or None to skip."""
    ref = _order_ref(event)

    if event.name == ORDER_PAID:
        amount = _format_amount(event.payload.get("amount"))
        lines = [f"Pembayaran untuk pesanan {ref} berhasil kami terima. ✅"]
        if amount:
            lines.append(f"Total dibayar: Rp{amount}.")
        lines.append("Pesanan kamu sedang kami proses ya. Terima kasih! 🙏")
        return "\n".join(lines)

    if event.name == ORDER_SHIPPED:
        tracking = str(event.payload.get("tracking_no") or "").strip()
        courier = str(event.payload.get("courier") or "").strip()
        lines = [f"Pesanan {ref} sudah dikirim! 📦"]
        if courier:
            lines.append(f"Kurir: {courier.upper()}")
        if tracking:
            lines.append(f"No. Resi: {tracking}")
        lines.append("Kamu bisa pantau pengiriman lewat nomor resi di atas.")
        return "\n".join(lines)

    if event.name == ORDER_DELIVERED:
        return (
            f"Pesanan {ref} sudah sampai di tujuan. 🎉\n"
            "Semoga suka dengan pesanannya! Kalau ada kendala, balas chat ini ya."
        )

    if event.name == ORDER_CANCELLED:
        return (
            f"Pesanan {ref} telah dibatalkan.\n"
            "Kalau ini di luar dugaan kamu atau butuh bantuan, balas chat ini ya."
        )

    # order.completed and any unknown event: nothing customer-facing to send.
    return None
