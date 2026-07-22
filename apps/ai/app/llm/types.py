"""Neutral types shared across the LLM orchestration layer.

These shapes are provider-agnostic: the orchestrator, tool registry, and every
chat-model implementation speak in terms of :class:`ChatMessage`,
:class:`ToolCall`, and :class:`ToolResult`. Concrete providers translate to/from
their own wire formats (OpenAI, etc.) at the edge.
"""

from __future__ import annotations

import json
from dataclasses import dataclass, field
from typing import Any


@dataclass(slots=True)
class ToolCall:
    """A model's request to invoke a tool.

    - ``id``: opaque id used to correlate the call with its result (providers
      that support parallel tool calls need this; rule-based ones synthesize it)
    - ``name``: the registered tool name
    - ``arguments``: parsed keyword arguments for the tool
    """

    id: str
    name: str
    arguments: dict[str, Any] = field(default_factory=dict)


@dataclass(slots=True)
class ToolResult:
    """The outcome of running a :class:`ToolCall`.

    ``ok`` distinguishes a successful backend call from a handled failure so the
    model can apologize / offer handover instead of inventing data (BR-004).
    ``content`` is always JSON-serializable so it can be fed back to the model.
    """

    call_id: str
    name: str
    ok: bool
    content: Any

    def to_text(self) -> str:
        """Render the result as compact JSON for the model's tool message."""
        payload = {"ok": self.ok, "data" if self.ok else "error": self.content}
        return json.dumps(payload, default=str, ensure_ascii=False)


@dataclass(slots=True)
class ChatMessage:
    """A single message in the model conversation.

    ``role`` is one of ``system|user|assistant|tool``. Assistant messages may
    carry ``tool_calls``; ``tool`` messages carry ``tool_call_id`` + ``name`` to
    answer a specific call.
    """

    role: str
    content: str | None = None
    tool_calls: list[ToolCall] = field(default_factory=list)
    tool_call_id: str | None = None
    name: str | None = None

    @classmethod
    def system(cls, content: str) -> ChatMessage:
        return cls(role="system", content=content)

    @classmethod
    def user(cls, content: str) -> ChatMessage:
        return cls(role="user", content=content)

    @classmethod
    def assistant(
        cls, content: str | None = None, tool_calls: list[ToolCall] | None = None
    ) -> ChatMessage:
        return cls(role="assistant", content=content, tool_calls=tool_calls or [])

    @classmethod
    def tool(cls, result: ToolResult) -> ChatMessage:
        return cls(
            role="tool",
            content=result.to_text(),
            tool_call_id=result.call_id,
            name=result.name,
        )
