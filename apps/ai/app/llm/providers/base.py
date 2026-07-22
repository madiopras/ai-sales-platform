"""Chat-model protocol.

A :class:`ChatModel` is the pluggable "brain": given the running conversation
(as :class:`~app.llm.types.ChatMessage` objects) and the available tool schemas,
it returns the next assistant message. That message either carries a final reply
(``content``) or one or more ``tool_calls`` the orchestrator must execute before
asking the model again.

Keeping this a narrow protocol means the rule-based default and any real LLM SDK
are interchangeable, and tests can inject a scripted model.
"""

from __future__ import annotations

from typing import Any, Protocol, runtime_checkable

from app.llm.types import ChatMessage


@runtime_checkable
class ChatModel(Protocol):
    """Interface every chat-model provider implements."""

    name: str

    async def complete(
        self,
        messages: list[ChatMessage],
        *,
        tools: list[dict[str, Any]] | None = None,
    ) -> ChatMessage:
        """Return the next assistant message.

        ``tools`` are OpenAI-style function schemas. The returned message has
        either non-empty ``content`` (final answer) or non-empty ``tool_calls``.
        """
        ...

    async def aclose(self) -> None:
        """Release any resources (HTTP client) held by the provider."""
        ...
