"""LLM orchestration: prompt + history + tool loop + guardrails.

The :class:`Orchestrator` is the agent's decision loop. Given a conversation
session and the latest user text it:

1. builds the message list = system prompt + rolling history + current turn,
2. asks the chat model for the next assistant message,
3. if the model requested tool calls, dispatches them via the registry and feeds
   the (source-of-truth) results back, then loops,
4. stops when the model returns a final text reply or the tool-iteration guard
   trips, and returns the reply text.

Guardrails (roadmap Phase A4):
- Tool results are the only source of truth; the model is instructed (prompt)
  and structurally nudged (tools) never to invent price/stock/shipping (BR-004).
- The loop is bounded by ``LLM_MAX_TOOL_ITERATIONS`` so a misbehaving model can't
  spin forever.
- Any provider failure degrades to a safe apology instead of leaking internals
  (BR-044). A handover session short-circuits before we ever call the model.

The orchestrator is stateless across turns; conversation memory lives in the
Redis-backed session (Phase A3). It never persists anything itself.
"""

from __future__ import annotations

from app.config import Settings
from app.conversation.models import Session, SessionStatus
from app.llm.intent import Intent, detect_intent
from app.llm.prompt import PROMPT_VERSION, build_system_prompt
from app.llm.providers.base import ChatModel
from app.llm.tools import ToolContext, ToolRegistry
from app.llm.types import ChatMessage
from app.platform.logging import get_logger

logger = get_logger(__name__)

# Reply used when the model/provider fails or the loop guard trips. Matches the
# system prompt's Error Handling guidance (apologize, offer retry/handover).
_FALLBACK_REPLY = (
    "Maaf ya, aku lagi ada kendala teknis sebentar. Boleh diulang lagi "
    "pesannya? Kalau masih bermasalah, aku bantu hubungkan ke admin kami."
)

# Reply when a session is already in human handover: the agent stays quiet-ish
# and defers to the human (Phase A8 will refine this).
_HANDOVER_REPLY = (
    "Baik, permintaan kamu sedang kami teruskan ke admin. Mohon tunggu "
    "sebentar ya 🙏"
)


class Orchestrator:
    """Drives the LLM + tool-calling loop for a single inbound turn."""

    def __init__(
        self,
        *,
        settings: Settings,
        model: ChatModel,
        tools: ToolRegistry,
    ):
        self._settings = settings
        self._model = model
        self._tools = tools
        self._max_iterations = max(1, settings.llm_max_tool_iterations)
        self._system_prompt = build_system_prompt(settings)

    async def reply(self, session: Session, user_text: str | None) -> str:
        """Produce a reply for the latest user message on ``session``.

        Does not mutate or persist the session; the caller records history.
        """
        # A human has taken over: don't run the model (BR-003 / Phase A8).
        if session.status == SessionStatus.HANDOVER or session.handover:
            return _HANDOVER_REPLY

        intent = detect_intent(user_text)
        messages = self._build_messages(session, user_text)
        tool_schemas = self._tools.schemas()
        # Server-controlled context: customer identity comes from the session's
        # WhatsApp id, never from model arguments. Tools cache resolved backend
        # references (customer_id/cart_id/address_id) onto the session context,
        # which the caller persists after the reply.
        tool_context = ToolContext(wa_id=session.wa_id, session_context=session.context)

        try:
            return await self._run_loop(messages, tool_schemas, intent, tool_context)
        except Exception:  # noqa: BLE001 — never surface internals to the user
            logger.exception("orchestrator_failed", extra={"intent": intent.value})
            return _FALLBACK_REPLY


    # --- internals -------------------------------------------------------

    def _build_messages(self, session: Session, user_text: str | None) -> list[ChatMessage]:
        """System prompt + rolling history (already trimmed by the session)."""
        messages: list[ChatMessage] = [ChatMessage.system(self._system_prompt)]

        # The session history includes the just-recorded user turn (the WhatsApp
        # service records inbound before calling us), so replay it verbatim and
        # do not append user_text again to avoid duplication.
        replayed_last_user = False
        for turn in session.history:
            role = "assistant" if turn.role == "agent" else "user"
            messages.append(ChatMessage(role=role, content=turn.text))
            if role == "user" and turn.text == (user_text or ""):
                replayed_last_user = True

        # Defensive: if history didn't contain the current turn (e.g. manager not
        # wired), add it so the model still sees the question.
        if user_text and not replayed_last_user:
            messages.append(ChatMessage.user(user_text))

        return messages

    async def _run_loop(
        self,
        messages: list[ChatMessage],
        tool_schemas: list[dict],
        intent: Intent,
        tool_context: ToolContext,
    ) -> str:
        for iteration in range(1, self._max_iterations + 1):
            assistant = await self._model.complete(messages, tools=tool_schemas)
            messages.append(assistant)

            if not assistant.tool_calls:
                content = (assistant.content or "").strip()
                return content or _FALLBACK_REPLY

            # Execute every requested tool call and feed results back as the
            # source of truth before asking the model again (BR-004).
            for call in assistant.tool_calls:
                result = await self._tools.dispatch(call, tool_context)

                logger.info(
                    "tool_called",
                    extra={
                        "tool": call.name,
                        "ok": result.ok,
                        "intent": intent.value,
                        "iteration": iteration,
                        "prompt_version": PROMPT_VERSION,
                    },
                )
                messages.append(ChatMessage.tool(result))

        # Loop guard tripped: ask the model once more for a final answer without
        # offering tools, so it must summarize from what it already has.
        logger.warning("tool_loop_exhausted", extra={"intent": intent.value})
        final = await self._model.complete(messages, tools=None)
        return (final.content or "").strip() or _FALLBACK_REPLY
