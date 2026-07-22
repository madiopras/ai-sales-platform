"""Normalized WhatsApp message models.

Provider-specific payloads (Meta Cloud API, other gateways) are parsed down to
these neutral shapes so the rest of the service never depends on a particular
provider's JSON structure.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any


@dataclass(slots=True)
class InboundMessage:
    """A single inbound WhatsApp message, normalized across providers.

    - ``wa_id``: sender's WhatsApp id / phone (E.164 without ``+``, as Meta sends)
    - ``message_id``: provider message id, used for idempotency (dedup)
    - ``type``: normalized message type (``text``, ``image``, ``interactive``, ...)
    - ``text``: extracted text body when applicable (button/list replies included)
    - ``name``: sender profile name when the provider includes it
    - ``timestamp``: provider-supplied unix seconds (0 if absent)
    - ``raw``: the original per-message payload for debugging / future use
    """

    wa_id: str
    message_id: str
    type: str
    text: str | None = None
    name: str | None = None
    timestamp: int = 0
    raw: dict[str, Any] = field(default_factory=dict)

    def to_reply(self) -> str:
        """Recipient id for a reply to this message (the original sender)."""
        return self.wa_id



@dataclass(slots=True)
class OutboundMessage:
    """A message to send back to a WhatsApp user.

    Kept minimal for Phase A2 (text + media). Interactive messages are added
    when the sales flow needs them (later phases).
    """

    to: str
    type: str = "text"
    text: str | None = None
    media_url: str | None = None
    caption: str | None = None

    @classmethod
    def text_message(cls, to: str, text: str) -> OutboundMessage:
        return cls(to=to, type="text", text=text)

    @classmethod
    def media_message(
        cls, to: str, media_url: str, *, caption: str | None = None
    ) -> OutboundMessage:
        return cls(to=to, type="image", media_url=media_url, caption=caption)
