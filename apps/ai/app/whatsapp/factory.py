"""Provider factory: build a WhatsAppProvider from settings.

Selecting the gateway is a config decision (``WA_PROVIDER``). Adding a new
gateway means adding an implementation module and one branch here — nothing
else in the service needs to change.
"""

from __future__ import annotations

from app.config import Settings
from app.whatsapp.meta import MetaWhatsAppProvider
from app.whatsapp.provider import WhatsAppProvider


def create_provider(settings: Settings) -> WhatsAppProvider:
    """Instantiate the configured WhatsApp provider.

    Defaults to Meta. Raises ``ValueError`` for an unknown provider name so
    misconfiguration fails fast at startup rather than at first webhook.
    """
    provider = (settings.wa_provider or "meta").lower()
    if provider == "meta":
        return MetaWhatsAppProvider(settings)
    raise ValueError(f"unsupported WA_PROVIDER: {settings.wa_provider!r}")
