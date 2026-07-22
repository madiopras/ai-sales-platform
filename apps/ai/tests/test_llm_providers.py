"""Tests for the chat-model providers and their factory selection."""

from __future__ import annotations

import httpx
import pytest

from app.config import Settings
from app.llm.providers import create_chat_model
from app.llm.providers.openai import LLMError, OpenAIChatModel
from app.llm.providers.rule_based import RuleBasedChatModel
from app.llm.types import ChatMessage, ToolResult

# --- factory selection -----------------------------------------------------


def test_factory_defaults_to_rule_based_when_unset():
    assert isinstance(create_chat_model(Settings(llm_provider="")), RuleBasedChatModel)


def test_factory_openai_without_key_falls_back_to_rule_based():
    model = create_chat_model(Settings(llm_provider="openai", llm_api_key=""))
    assert isinstance(model, RuleBasedChatModel)


def test_factory_openai_with_key():
    model = create_chat_model(
        Settings(llm_provider="openai", llm_api_key="sk-test", llm_model="gpt-4o-mini")
    )
    assert isinstance(model, OpenAIChatModel)


def test_factory_unknown_provider_raises():
    with pytest.raises(ValueError):
        create_chat_model(Settings(llm_provider="does-not-exist"))


# --- rule-based behavior ---------------------------------------------------


async def test_rule_based_requests_search_on_product_intent():
    model = RuleBasedChatModel()
    out = await model.complete([ChatMessage.user("ada kaos hitam ukuran XL?")])
    assert len(out.tool_calls) == 1
    assert out.tool_calls[0].name == "search_products"


async def test_rule_based_greeting_has_no_tool_call():
    model = RuleBasedChatModel()
    out = await model.complete([ChatMessage.user("halo kak")])
    assert out.tool_calls == []
    assert out.content


async def test_rule_based_composes_reply_from_tool_result():
    model = RuleBasedChatModel()
    result = ToolResult(
        call_id="c1",
        name="search_products",
        ok=True,
        content={
            "products": [
                {
                    "name": "Kaos Hitam",
                    "variants": [{"name": "XL", "price": 89000, "available_stock": 3}],
                }
            ]
        },
    )
    out = await model.complete([ChatMessage.user("kaos hitam"), ChatMessage.tool(result)])
    assert "Kaos Hitam" in out.content
    assert "89.000" in out.content  # price straight from tool data


async def test_rule_based_empty_result_offers_alternative():
    model = RuleBasedChatModel()
    result = ToolResult("c1", "search_products", ok=True, content={"products": []})
    out = await model.complete([ChatMessage.user("kaos"), ChatMessage.tool(result)])
    assert "alternatif" in out.content.lower() or "belum menemukan" in out.content.lower()


async def test_rule_based_failed_tool_apologizes():
    model = RuleBasedChatModel()
    result = ToolResult("c1", "search_products", ok=False, content={"error": "backend_unavailable"})
    out = await model.complete([ChatMessage.user("kaos"), ChatMessage.tool(result)])
    assert "admin" in out.content.lower() or "coba" in out.content.lower()


# --- OpenAI wire translation ----------------------------------------------


async def test_openai_parses_tool_calls():
    def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(
            200,
            json={
                "choices": [
                    {
                        "message": {
                            "role": "assistant",
                            "content": None,
                            "tool_calls": [
                                {
                                    "id": "call_1",
                                    "type": "function",
                                    "function": {
                                        "name": "search_products",
                                        "arguments": '{"q": "kaos"}',
                                    },
                                }
                            ],
                        }
                    }
                ]
            },
        )

    transport = httpx.MockTransport(handler)
    client = httpx.AsyncClient(transport=transport, base_url="https://api.test/v1")
    model = OpenAIChatModel(Settings(llm_api_key="sk", llm_model="m"), client=client)
    out = await model.complete([ChatMessage.user("cari kaos")], tools=[{"type": "function"}])
    assert out.tool_calls[0].name == "search_products"
    assert out.tool_calls[0].arguments == {"q": "kaos"}
    await model.aclose()


async def test_openai_error_status_raises_llm_error():
    def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(500, json={"error": "boom"})

    transport = httpx.MockTransport(handler)
    client = httpx.AsyncClient(transport=transport, base_url="https://api.test/v1")
    model = OpenAIChatModel(Settings(llm_api_key="sk"), client=client)
    with pytest.raises(LLMError):
        await model.complete([ChatMessage.user("hi")])
    await model.aclose()
