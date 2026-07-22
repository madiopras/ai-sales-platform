"""Provider abstraction for WhatsApp gateways.

A ``WhatsAppProvider`` hides the differences between concrete gateways (Meta
Cloud API, third-party gateways) behind a stable interface. The webhook,
idempotency, and outbound layers depend only on this protocol, so swapping the
gateway is a config change (``WA_PROVIDER``) plus a new implementation file.
"""

from __future__ import annotations

from typing import Any, Protocol, runtime_checkable

from app.whatsapp.models import InboundMessage, OutboundMessage


class WebhookVerificationError(Exception):
    """Raised when webhook verification (challenge or signature) fails."""


@runtime_checkable
class WhatsAppProvider(Protocol):
    """Interface every WhatsApp gateway implementation must satisfy."""

    name: str

    def verify_subscription(
        self, mode: str | None, token: str | None, challenge: str | None
    ) -> str:
        """Handle the provider's webhook subscription handshake (GET).

        For Meta this is the hub.mode / hub.verify_token / hub.challenge
        exchange; return the challenge string to echo back on success. Raise
        :class:`WebhookVerificationError` when verification fails.
        """
        ...

    def verify_signature(self, raw_body: bytes, signature_header: str | None) -> bool:
        """Verify an inbound webhook payload's authenticity (POST).

        Returns True when the signature is valid or verification is disabled
        (no secret configured). Returns False when a configured signature check
        fails, so the caller can reject the request.
        """
        ...

    def parse_inbound(self, payload: dict[str, Any]) -> list[InboundMessage]:
        """Parse a raw webhook body into zero or more normalized messages.

        Non-message events (delivery/read status callbacks) yield an empty list.
        """
        ...

    async def send(self, message: OutboundMessage) -> dict[str, Any]:
        """Send an outbound message via the gateway; return the raw response.

        Raises on transport / API errors so the outbound layer can retry.
        """
        ...

    async def aclose(self) -> None:
        """Release any resources (HTTP client) held by the provider."""
        ...
