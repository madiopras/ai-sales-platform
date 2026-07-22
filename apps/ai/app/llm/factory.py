"""Assemble a ready-to-use :class:`Orchestrator` from settings + clients.

Central place that wires the three pluggable pieces together:

- the chat model chosen by ``LLM_PROVIDER`` (:func:`create_chat_model`),
- the tool registry backed by the Go client (:func:`build_registry`), and
- the versioned system prompt (loaded inside the orchestrator).

Keeping this behind one function means ``main.py`` (and tests) construct the
agent in a single call and the chat model's lifecycle can be closed alongside
the other clients.
"""

from __future__ import annotations

from app.clients.backend import BackendClient
from app.config import Settings
from app.llm.orchestrator import Orchestrator
from app.llm.providers import create_chat_model
from app.llm.providers.base import ChatModel
from app.llm.tools import build_registry


def create_orchestrator(
    settings: Settings, backend: BackendClient
) -> tuple[Orchestrator, ChatModel]:
    """Build the orchestrator and return it with its chat model.

    The chat model is returned separately so the caller can ``aclose()`` it on
    shutdown (real providers hold an HTTP client).
    """
    model = create_chat_model(settings)
    tools = build_registry(backend)
    orchestrator = Orchestrator(settings=settings, model=model, tools=tools)
    return orchestrator, model
