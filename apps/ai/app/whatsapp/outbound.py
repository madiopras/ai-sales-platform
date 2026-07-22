"""Outbound WhatsApp sender with light retry.

Replies are sent as FastAPI background tasks (see the router) so the webhook can
return 200 quickly and the provider does not retry on our processing latency.
This module owns the retry policy around the provider's ``send`` call so a
transient provider hiccup doesn't drop a reply.
"""

from __future__ import annotations

import asyncio

from app.platform.logging import get_logger
from app.whatsapp.models import OutboundMessage
from app.whatsapp.provider import WhatsAppProvider

logger = get_logger(__name__)


class OutboundSender:
    """Sends outbound messages via a provider, retrying on failure."""

    def __init__(self, provider: WhatsAppProvider, max_retries: int = 3):
        self._provider = provider
        self._max_retries = max(0, max_retries)

    async def send(self, message: OutboundMessage) -> bool:
        """Send a message, retrying with backoff. Returns True on success.

        Never raises: outbound sending typically runs in a background task where
        an exception would be swallowed anyway, so we log and return a bool the
        caller can act on if it awaits directly.
        """
        attempts = self._max_retries + 1
        for attempt in range(1, attempts + 1):
            try:
                await self._provider.send(message)
                logger.info(
                    "wa_send_ok",
                    extra={"to": message.to, "type": message.type, "attempt": attempt},
                )
                return True
            except Exception:  # noqa: BLE001 — log + retry, never crash the task
                logger.warning(
                    "wa_send_failed",
                    extra={"to": message.to, "type": message.type, "attempt": attempt},
                )
                if attempt < attempts:
                    await asyncio.sleep(0.2 * attempt)

        logger.error("wa_send_exhausted", extra={"to": message.to, "type": message.type})
        return False
