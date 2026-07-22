"""WhatsApp integration package (Phase A2).

Provides a provider-agnostic abstraction over a WhatsApp Business API so the
concrete gateway (Meta Cloud API today, another gateway tomorrow) can be swapped
without touching the webhook, idempotency, or outbound-send logic.

Layout:

- ``models``      — normalized ``InboundMessage`` / ``OutboundMessage`` types
- ``provider``    — the ``WhatsAppProvider`` protocol
- ``meta``        — Meta WhatsApp Cloud API implementation
- ``factory``     — builds a provider from settings
- ``idempotency`` — Redis-backed inbound ``message_id`` dedup
- ``outbound``    — async sender with retry
- ``service``     — orchestrates verify → parse → dedup → reply
- ``router``      — the ``/webhooks/whatsapp`` FastAPI routes
"""

from __future__ import annotations

from app.whatsapp.models import InboundMessage, OutboundMessage
from app.whatsapp.provider import WhatsAppProvider

__all__ = ["InboundMessage", "OutboundMessage", "WhatsAppProvider"]
