"""OpenAI-compatible chat model with function calling.

Talks to any endpoint implementing the OpenAI ``/chat/completions`` contract
(OpenAI, Azure OpenAI, or a compatible proxy). It translates our neutral
:class:`~app.llm.types.ChatMessage` list to the wire format, forwards the tool
schemas, and maps the response (including ``tool_calls``) back to a
:class:`ChatMessage`.

No domain logic lives here — the orchestrator runs the tool loop and enforces
guardrails. On any transport/API error we raise :class:`LLMError` so the
orchestrator can degrade to a safe apology instead of leaking internals.
"""

from __future__ import annotations

import json
from typing import Any

import httpx

from app.config import Settings
from app.llm.types import ChatMessage, ToolCall
from app.platform.errors import AppError
from app.platform.logging import get_logger

logger = get_logger(__name__)


class LLMError(AppError):
    """Raised when the LLM provider is unreachable or returns an error."""

    status_code = 502
    code = "LLM_ERROR"


class OpenAIChatModel:
    """Chat model backed by an OpenAI-compatible chat-completions endpoint."""

    name = "openai"

    def __init__(self, settings: Settings, client: httpx.AsyncClient | None = None):
        self._model = settings.llm_model or "gpt-4o-mini"
        self._max_tokens = settings.llm_max_tokens
        self._temperature = settings.llm_temperature
        headers = {
            "Content-Type": "application/json",
            "Authorization": f"Bearer {settings.llm_api_key}",
        }
        self._client = client or httpx.AsyncClient(
            base_url=settings.llm_api_base_url.rstrip("/"),
            timeout=settings.backend_timeout_seconds,
            headers=headers,
        )

    async def aclose(self) -> None:
        await self._client.aclose()

    async def complete(
        self,
        messages: list[ChatMessage],
        *,
        tools: list[dict[str, Any]] | None = None,
    ) -> ChatMessage:
        body: dict[str, Any] = {
            "model": self._model,
            "messages": [self._to_wire(m) for m in messages],
            "temperature": self._temperature,
            "max_tokens": self._max_tokens,
        }
        if tools:
            body["tools"] = tools
            body["tool_choice"] = "auto"

        try:
            response = await self._client.post("/chat/completions", json=body)
        except httpx.RequestError as exc:
            logger.warning("llm_request_error")
            raise LLMError("llm unreachable") from exc

        if response.status_code >= 400:
            # Don't log the body: it may echo the prompt/tool content (BR-044).
            logger.warning("llm_error_response", extra={"status_code": response.status_code})
            raise LLMError("llm request failed")

        try:
            data = response.json()
        except ValueError as exc:
            raise LLMError("llm returned invalid json") from exc

        return self._from_wire(data)

    # --- wire translation ------------------------------------------------

    @staticmethod
    def _to_wire(message: ChatMessage) -> dict[str, Any]:
        wire: dict[str, Any] = {"role": message.role}
        if message.role == "tool":
            wire["content"] = message.content or ""
            wire["tool_call_id"] = message.tool_call_id or ""
            if message.name:
                wire["name"] = message.name
            return wire

        # assistant/user/system
        wire["content"] = message.content or ""
        if message.tool_calls:
            wire["tool_calls"] = [
                {
                    "id": call.id,
                    "type": "function",
                    "function": {
                        "name": call.name,
                        "arguments": json.dumps(call.arguments, ensure_ascii=False),
                    },
                }
                for call in message.tool_calls
            ]
            # OpenAI requires content to be null (not "") when tool_calls present.
            if not message.content:
                wire["content"] = None
        return wire

    @staticmethod
    def _from_wire(data: dict[str, Any]) -> ChatMessage:
        choices = data.get("choices") or []
        if not choices:
            raise LLMError("llm returned no choices")
        message = choices[0].get("message") or {}

        tool_calls: list[ToolCall] = []
        for raw in message.get("tool_calls") or []:
            fn = raw.get("function") or {}
            tool_calls.append(
                ToolCall(
                    id=raw.get("id") or f"call_{len(tool_calls)}",
                    name=fn.get("name") or "",
                    arguments=OpenAIChatModel._parse_args(fn.get("arguments")),
                )
            )

        return ChatMessage.assistant(
            content=message.get("content"),
            tool_calls=tool_calls,
        )

    @staticmethod
    def _parse_args(raw: Any) -> dict[str, Any]:
        if isinstance(raw, dict):
            return raw
        if isinstance(raw, str) and raw.strip():
            try:
                parsed = json.loads(raw)
                return parsed if isinstance(parsed, dict) else {}
            except ValueError:
                return {}
        return {}
