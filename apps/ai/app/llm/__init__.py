"""LLM orchestration package (Phase A4).

Turns an inbound message + conversation session into a reply by combining:

- a versioned system prompt (``prompt.py``) built from the docs sources,
- lightweight intent detection (``intent.py``),
- a tool registry that wraps the Go backend as callable functions
  (``tools.py``), and
- a pluggable chat model (``providers/``) that may request tool calls.

The :class:`~app.llm.orchestrator.Orchestrator` ties these together and enforces
the guardrails from the roadmap: tool results are the source of truth, the model
never invents price/stock/shipping, and failures degrade to a safe apology or a
handover hint (BR-004, BR-044).
"""

from __future__ import annotations

from app.llm.intent import Intent, detect_intent
from app.llm.orchestrator import Orchestrator
from app.llm.types import ChatMessage, ToolCall, ToolResult

__all__ = [
    "ChatMessage",
    "Intent",
    "Orchestrator",
    "ToolCall",
    "ToolResult",
    "detect_intent",
]
