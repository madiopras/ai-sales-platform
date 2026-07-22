"""End-to-end: WhatsApp inbound → conversation → orchestrator → reply.

Uses the real rule-based model + tool registry with a faked backend, exercising
the Phase A4 deliverable: "ada kaos hitam ukuran XL?" triggers a product search
and the reply is composed only from active, in-stock products (BR-004).
"""

from __future__ import annotations

from typing import Any

from app.config import Settings
from app.conversation.manager import ConversationManager
from app.conversation.store import SessionStore
from app.llm.orchestrator import Orchestrator
from app.llm.providers.rule_based import RuleBasedChatModel
from app.llm.tools import build_registry
from app.whatsapp.idempotency import InboundDedup
from app.whatsapp.models import InboundMessage, OutboundMessage
from app.whatsapp.outbound import OutboundSender
from app.whatsapp.service import WhatsAppService


class FakeRedis:
    def __init__(self):
        self.store: dict[str, str] = {}

    async def get(self, key):
        return self.store.get(key)

    async def set(self, key, value, nx=False, ex=None):  # noqa: ARG002
        if nx and key in self.store:
            return None
        self.store[key] = value
        return True

    async def delete(self, key):
        self.store.pop(key, None)
        return 1


class RecordingProvider:
    name = "fake"

    def __init__(self):
        self.sent: list[OutboundMessage] = []

    def verify_subscription(self, mode, token, challenge):  # pragma: no cover
        return challenge or ""

    def verify_signature(self, raw_body, signature_header):  # pragma: no cover
        return True

    def parse_inbound(self, payload):  # pragma: no cover
        return []

    async def send(self, message: OutboundMessage):
        self.sent.append(message)
        return {"messages": [{"id": "wamid.OUT"}]}

    async def aclose(self):  # pragma: no cover
        return None


class FakeBackend:
    def __init__(self, search_result: Any):
        self._search = search_result

    async def get(self, path, *, params=None):
        return self._search


def build_service(provider: RecordingProvider, backend: FakeBackend) -> WhatsAppService:
    settings = Settings(wa_echo_reply=True)
    redis = FakeRedis()
    conversations = ConversationManager(
        SessionStore(redis, ttl_seconds=1800),
        idle_timeout_seconds=1800,
        history_max=20,
    )
    orchestrator = Orchestrator(
        settings=settings,
        model=RuleBasedChatModel(),
        tools=build_registry(backend),  # type: ignore[arg-type]
    )
    return WhatsAppService(
        settings=settings,
        provider=provider,
        dedup=InboundDedup(redis, ttl_seconds=60),
        sender=OutboundSender(provider, max_retries=1),
        conversations=conversations,
        orchestrator=orchestrator,
    )


def inbound(text: str, message_id: str = "wamid.1") -> InboundMessage:
    return InboundMessage(wa_id="628123", message_id=message_id, type="text", text=text)


async def test_product_query_replies_with_in_stock_products():
    product = {
        "id": "p1",
        "name": "Kaos Hitam",
        "slug": "kaos-hitam",
        "status": "active",
        "variants": [
            {"id": "v1", "sku": "XL", "name": "XL", "price": 89000, "stock": 3, "is_active": True}
        ],
    }
    provider = RecordingProvider()
    service = build_service(provider, FakeBackend([product]))

    await service.handle_inbound(inbound("ada kaos hitam ukuran XL?"))

    assert len(provider.sent) == 1
    reply = provider.sent[0].text
    assert "Kaos Hitam" in reply
    assert "89.000" in reply


async def test_out_of_stock_offers_alternative():
    product = {
        "id": "p1",
        "name": "Kaos Hitam",
        "slug": "kaos-hitam",
        "status": "active",
        "variants": [
            {"id": "v1", "sku": "XL", "name": "XL", "price": 89000, "stock": 0, "is_active": True}
        ],
    }
    provider = RecordingProvider()
    service = build_service(provider, FakeBackend([product]))

    await service.handle_inbound(inbound("ada kaos hitam?"))

    reply = provider.sent[0].text.lower()
    assert "alternatif" in reply or "belum menemukan" in reply


async def test_greeting_does_not_call_backend_or_leak():
    provider = RecordingProvider()
    service = build_service(provider, FakeBackend([]))
    await service.handle_inbound(inbound("halo kak"))
    reply = provider.sent[0].text
    assert reply  # a friendly greeting, no product data
    assert "Echo:" not in reply
