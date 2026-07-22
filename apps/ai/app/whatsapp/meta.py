"""Meta WhatsApp Cloud API provider.

Implements :class:`~app.whatsapp.provider.WhatsAppProvider` against Meta's Graph
API. References:

- Webhook verification (GET): ``hub.mode=subscribe`` + ``hub.verify_token``
  must match ``WA_VERIFY_TOKEN``; echo back ``hub.challenge``.
- Payload signature (POST): ``X-Hub-Signature-256: sha256=<hmac>`` computed with
  the app secret over the raw request body.
- Send: ``POST {base}/{phone_number_id}/messages`` with a bearer access token.

No domain logic lives here — it only speaks Meta's wire format and maps it to
the normalized models.
"""

from __future__ import annotations

import hashlib
import hmac
from typing import Any

import httpx

from app.config import Settings
from app.platform.logging import get_logger
from app.whatsapp.models import InboundMessage, OutboundMessage
from app.whatsapp.provider import WebhookVerificationError

logger = get_logger(__name__)

_SIGNATURE_PREFIX = "sha256="


class MetaWhatsAppProvider:
    """WhatsApp Cloud API implementation of the provider protocol."""

    name = "meta"

    def __init__(self, settings: Settings, client: httpx.AsyncClient | None = None):
        self._settings = settings
        self._phone_number_id = settings.wa_phone_number_id
        self._verify_token = settings.wa_verify_token
        self._app_secret = settings.wa_app_secret
        base_url = settings.wa_api_base_url.rstrip("/")
        headers = {"Content-Type": "application/json"}
        if settings.wa_access_token:
            headers["Authorization"] = f"Bearer {settings.wa_access_token}"
        self._client = client or httpx.AsyncClient(
            base_url=base_url,
            timeout=settings.backend_timeout_seconds,
            headers=headers,
        )

    async def aclose(self) -> None:
        await self._client.aclose()

    # --- Verification -----------------------------------------------------

    def verify_subscription(
        self, mode: str | None, token: str | None, challenge: str | None
    ) -> str:
        """Validate the GET subscription handshake and return the challenge."""
        if mode == "subscribe" and token and self._verify_token and token == self._verify_token:
            if challenge is None:
                raise WebhookVerificationError("missing challenge")
            return challenge
        raise WebhookVerificationError("verification failed")

    def verify_signature(self, raw_body: bytes, signature_header: str | None) -> bool:
        """Verify ``X-Hub-Signature-256`` against the app secret.

        When no app secret is configured we skip verification (dev only) and
        return True. When a secret is set, a missing or mismatched signature
        returns False.
        """
        if not self._app_secret:
            return True
        if not signature_header or not signature_header.startswith(_SIGNATURE_PREFIX):
            return False
        expected = hmac.new(
            self._app_secret.encode("utf-8"), raw_body, hashlib.sha256
        ).hexdigest()
        provided = signature_header[len(_SIGNATURE_PREFIX):]
        return hmac.compare_digest(expected, provided)

    # --- Inbound parsing --------------------------------------------------

    def parse_inbound(self, payload: dict[str, Any]) -> list[InboundMessage]:
        """Flatten Meta's nested webhook body into normalized messages.

        Structure: ``entry[].changes[].value.messages[]`` with contact profile
        names in ``value.contacts[]``. Status callbacks (no ``messages``) yield
        nothing.
        """
        messages: list[InboundMessage] = []
        for entry in payload.get("entry", []) or []:
            for change in entry.get("changes", []) or []:
                value = change.get("value") or {}
                name_by_wa = self._contact_names(value.get("contacts", []) or [])
                for msg in value.get("messages", []) or []:
                    parsed = self._parse_message(msg, name_by_wa)
                    if parsed is not None:
                        messages.append(parsed)
        return messages

    @staticmethod
    def _contact_names(contacts: list[dict[str, Any]]) -> dict[str, str]:
        names: dict[str, str] = {}
        for contact in contacts:
            wa_id = contact.get("wa_id")
            name = (contact.get("profile") or {}).get("name")
            if wa_id and name:
                names[wa_id] = name
        return names

    def _parse_message(
        self, msg: dict[str, Any], name_by_wa: dict[str, str]
    ) -> InboundMessage | None:
        wa_id = msg.get("from")
        message_id = msg.get("id")
        msg_type = msg.get("type")
        if not wa_id or not message_id or not msg_type:
            return None

        try:
            timestamp = int(msg.get("timestamp", 0))
        except (TypeError, ValueError):
            timestamp = 0

        return InboundMessage(
            wa_id=wa_id,
            message_id=message_id,
            type=msg_type,
            text=self._extract_text(msg, msg_type),
            name=name_by_wa.get(wa_id),
            timestamp=timestamp,
            raw=msg,
        )

    @staticmethod
    def _extract_text(msg: dict[str, Any], msg_type: str) -> str | None:
        """Pull user text from the relevant field for each message type."""
        if msg_type == "text":
            return (msg.get("text") or {}).get("body")
        if msg_type == "button":
            return (msg.get("button") or {}).get("text")
        if msg_type == "interactive":
            interactive = msg.get("interactive") or {}
            reply = interactive.get("button_reply") or interactive.get("list_reply") or {}
            return reply.get("title") or reply.get("id")
        # Media / location / etc: caption when present, else None.
        media = msg.get(msg_type)
        if isinstance(media, dict):
            return media.get("caption")
        return None

    # --- Outbound ---------------------------------------------------------

    async def send(self, message: OutboundMessage) -> dict[str, Any]:
        """Send a message via the Cloud API messages endpoint."""
        body = self._build_send_body(message)
        response = await self._client.post(f"/{self._phone_number_id}/messages", json=body)
        response.raise_for_status()
        try:
            return response.json()
        except ValueError:
            return {}

    def _build_send_body(self, message: OutboundMessage) -> dict[str, Any]:
        base: dict[str, Any] = {
            "messaging_product": "whatsapp",
            "recipient_type": "individual",
            "to": message.to,
        }
        if message.type == "text":
            base["type"] = "text"
            base["text"] = {"body": message.text or ""}
        elif message.type == "image":
            base["type"] = "image"
            image: dict[str, Any] = {"link": message.media_url}
            if message.caption:
                image["caption"] = message.caption
            base["image"] = image
        else:
            raise ValueError(f"unsupported outbound message type: {message.type}")
        return base
