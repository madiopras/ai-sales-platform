"""Tests for the orchestrator tool loop, guardrails, and handover behavior."""

from __future__ import annotations

from typing import Any

from app.config import Settings
from app.conversation.models import ConversationState, Session, SessionStatus
from app.llm.orchestrator import Orchestrator
from app.llm.tools import ToolRegistry, build_registry
from app.llm.types import ChatMessage, ToolCall


class ScriptedModel:
    """Returns a queued list of assistant messages, one per complete() call."""

    name = "scripted"

    def __init__(self, script: list[ChatMessage]):
        self._script = list(script)
        self.calls: list[list[ChatMessage]] = []
        self.tools_seen: list[Any] = []

    async def complete(self, messages, *, tools=None):
        self.calls.append(list(messages))
        self.tools_seen.append(tools)
        return self._script.pop(0)

    async def aclose(self):
        return None


class FakeBackend:
    def __init__(self, search_result: Any):
        self._search = search_result
        self.calls: list[str] = []

    async def get(self, path, *, params=None):
        self.calls.append(path)
        return self._search


class ScopedBackend:
    """Fake backend for the cart-scoping test: upsert customer + return cart."""

    def __init__(self) -> None:
        self.calls: list[tuple[str, str]] = []

    async def get(self, path, *, params=None):
        self.calls.append(("GET", path))
        return {"id": "cart-1", "customer_id": "cust-1", "status": "open", "items": []}

    async def post(self, path, *, json=None):
        self.calls.append(("POST", path))
        return {"id": "cust-1", "phone": json.get("phone"), "name": json.get("name")}



def make_orchestrator(model, *, registry: ToolRegistry | None = None) -> Orchestrator:
    settings = Settings(llm_max_tool_iterations=3)
    tools = registry or ToolRegistry()
    return Orchestrator(settings=settings, model=model, tools=tools)


def session_with_user(text: str) -> Session:
    s = Session(wa_id="628123")
    s.add_turn("user", text, max_history=20)
    return s


async def test_direct_reply_without_tools():
    model = ScriptedModel([ChatMessage.assistant(content="Halo! Ada yang bisa dibantu?")])
    orch = make_orchestrator(model)
    reply = await orch.reply(session_with_user("halo"), "halo")
    assert reply == "Halo! Ada yang bisa dibantu?"
    # System prompt is always the first message.
    assert model.calls[0][0].role == "system"


async def test_tool_call_then_final_reply():
    product = {
        "id": "p1",
        "name": "Kaos Hitam",
        "slug": "kaos-hitam",
        "status": "active",
        "variants": [
            {"id": "v1", "sku": "XL", "name": "XL", "price": 89000, "stock": 3, "is_active": True}
        ],
    }
    backend = FakeBackend([product])
    registry = build_registry(backend)  # type: ignore[arg-type]

    model = ScriptedModel(
        [
            ChatMessage.assistant(
                tool_calls=[
                    ToolCall(id="c1", name="search_products", arguments={"q": "kaos hitam"})
                ]
            ),
            ChatMessage.assistant(content="Ada Kaos Hitam XL seharga Rp89.000."),
        ]

    )
    orch = make_orchestrator(model, registry=registry)
    reply = await orch.reply(session_with_user("ada kaos hitam XL?"), "ada kaos hitam XL?")

    assert reply == "Ada Kaos Hitam XL seharga Rp89.000."
    assert backend.calls == ["/products/search"]
    # On the second model call the tool result was appended as a tool message.
    second_call_msgs = model.calls[1]
    assert second_call_msgs[-1].role == "tool"


async def test_handover_session_short_circuits_model():
    model = ScriptedModel([ChatMessage.assistant(content="should not be used")])
    orch = make_orchestrator(model)
    session = Session(wa_id="628123", status=SessionStatus.HANDOVER, handover=True)
    reply = await orch.reply(session, "mau komplain")
    assert "admin" in reply.lower()
    assert model.calls == []  # model never invoked


async def test_model_failure_degrades_to_apology():
    class BoomModel:
        name = "boom"

        async def complete(self, messages, *, tools=None):
            raise RuntimeError("upstream down")

        async def aclose(self):
            return None

    orch = make_orchestrator(BoomModel())
    reply = await orch.reply(session_with_user("halo"), "halo")
    assert "kendala" in reply.lower() or "admin" in reply.lower()


async def test_tool_loop_guard_forces_final_answer():
    # Model always asks for a tool → loop must trip the guard and ask once more.
    always_tool = ChatMessage.assistant(
        tool_calls=[ToolCall(id="c", name="search_products", arguments={"q": "x"})]
    )
    backend = FakeBackend([])
    registry = build_registry(backend)  # type: ignore[arg-type]
    # 3 iterations of tool calls + 1 final (no-tools) summarizing call.
    final = ChatMessage.assistant(content="Ringkasan akhir.")
    script = [always_tool, always_tool, always_tool, final]
    model = ScriptedModel(script)

    orch = make_orchestrator(model, registry=registry)
    reply = await orch.reply(session_with_user("cari"), "cari")
    assert reply == "Ringkasan akhir."
    # Final call was made without tools.
    assert model.tools_seen[-1] is None


async def test_history_replayed_without_duplicating_current_turn():
    model = ScriptedModel([ChatMessage.assistant(content="ok")])
    orch = make_orchestrator(model)
    session = Session(wa_id="628123", state=ConversationState.DISCOVERY)
    session.add_turn("user", "halo", max_history=20)
    session.add_turn("agent", "Halo!", max_history=20)
    session.add_turn("user", "cari kaos", max_history=20)

    await orch.reply(session, "cari kaos")

    sent = model.calls[0]
    user_texts = [m.content for m in sent if m.role == "user"]
    # "cari kaos" appears exactly once (replayed from history, not re-appended).
    assert user_texts.count("cari kaos") == 1


async def test_cart_tool_scoped_to_session_customer_and_writes_back_ids():
    # The model asks to see the cart; the orchestrator must supply customer
    # identity from the session (phone = wa_id), not from the model.
    backend = ScopedBackend()
    registry = build_registry(backend)  # type: ignore[arg-type]
    model = ScriptedModel(
        [
            ChatMessage.assistant(
                tool_calls=[ToolCall(id="c1", name="get_cart", arguments={})]
            ),
            ChatMessage.assistant(content="Keranjang kamu masih kosong."),
        ]
    )
    orch = make_orchestrator(model, registry=registry)
    session = session_with_user("lihat keranjang")

    reply = await orch.reply(session, "lihat keranjang")

    assert reply == "Keranjang kamu masih kosong."
    # Customer was upserted using the session's wa_id, then the cart fetched.
    assert ("POST", "/customers") in backend.calls
    assert ("GET", "/customers/cust-1/cart") in backend.calls
    # Resolved ids were written back onto the session context for persistence.
    assert session.context.customer_id == "cust-1"
    assert session.context.cart_id == "cart-1"

