"""Chat-model providers for the orchestrator.

A provider turns a list of :class:`~app.llm.types.ChatMessage` (plus the tool
schemas) into an assistant message that either contains a final reply or one or
more tool calls. The orchestrator drives the tool-call loop; providers stay
stateless.

- :class:`ChatModel` — the protocol every provider implements.
- :class:`RuleBasedChatModel` — a no-external-dependency default that satisfies
  the Phase A4 deliverable (search on product intent) without an API key.
- :class:`OpenAIChatModel` — an OpenAI-compatible function-calling client.

:func:`create_chat_model` selects one from settings (``LLM_PROVIDER``).
"""

from __future__ import annotations

from app.config import Settings
from app.llm.providers.base import ChatModel
from app.llm.providers.openai import OpenAIChatModel
from app.llm.providers.rule_based import RuleBasedChatModel


def create_chat_model(settings: Settings) -> ChatModel:
    """Instantiate the configured chat model.

    Defaults to the rule-based model when no provider (or no API key) is set so
    the service runs end-to-end in development without external credentials.
    """
    provider = (settings.llm_provider or "").strip().lower()
    if provider in ("", "rule", "rule_based", "rulebased", "none"):
        return RuleBasedChatModel()
    if provider in ("openai", "azure", "openai_compatible"):
        if not settings.llm_api_key:
            # Fail safe rather than making unauthenticated calls that 401.
            return RuleBasedChatModel()
        return OpenAIChatModel(settings)
    raise ValueError(f"unsupported LLM_PROVIDER: {settings.llm_provider!r}")


__all__ = [
    "ChatModel",
    "OpenAIChatModel",
    "RuleBasedChatModel",
    "create_chat_model",
]
