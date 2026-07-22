"""Integration: WhatsAppService driven by the ConversationManager (Phase A3)."""

from __future__ import annotations

from app.config import Settings
from app.conversation.manager import ConversationManager
from app.conversation.store import SessionStore
from app.whatsapp.idempotency import InboundDedup
from app.whatsapp.models import InboundMessage
from app.whatsapp.outbound import OutboundSender
from app.whatsapp.service import WhatsAppService
from tests.test_conversation_manager import FakeRedis as SessionFakeRedis
from tests.test_whatsapp_service import FakeRedis as DedupFakeRedis
from tests.test_whatsapp_service import RecordingProvider


def make_service(
    *, echo: bool = True
) -> tuple[WhatsAppService, RecordingProvider, ConversationManager]:
    provider = RecordingProvider()

    settings = Settings(wa_echo_reply=echo)
    store = SessionStore(SessionFakeRedis(), ttl_seconds=1800)
    manager = ConversationManager(store, idle_timeout_seconds=1800, history_max=20)
    service = WhatsAppService(
        settings=settings,
        provider=provider,
        dedup=InboundDedup(DedupFakeRedis(), ttl_seconds=60),
        sender=OutboundSender(provider, max_retries=0),
        conversations=manager,
    )
    return service, provider, manager


def inbound(text: str, message_id: str = "wamid.1", name: str | None = None) -> InboundMessage:
    return InboundMessage(wa_id="628123", message_id=message_id, type="text", text=text, name=name)


async def test_state_aware_reply_replaces_echo():
    service, provider, _ = make_service()
    await service.handle_inbound(inbound("halo", name="Budi"))
    assert len(provider.sent) == 1
    # Not the old "Echo: ..." — it's the state-aware, personalized reply.
    assert not provider.sent[0].text.startswith("Echo:")
    assert "Budi" in provider.sent[0].text


async def test_handover_sends_one_ack_then_stays_silent():
    service, provider, manager = make_service()
    # First handover-triggering message: the customer gets a single warm ack so
    # they know a human is coming (BR-003), then the session is in handover.
    await service.handle_inbound(inbound("saya mau komplain", message_id="wamid.h1"))
    assert len(provider.sent) == 1
    session = await manager.get_or_create("628123")
    assert session.handover is True

    # Subsequent messages while handed over get no auto-reply — the bot stays
    # quiet so it doesn't talk over the human now handling the conversation.
    await service.handle_inbound(inbound("halo? ada orang?", message_id="wamid.h2"))
    assert len(provider.sent) == 1



async def test_multi_turn_persists_and_records_agent_reply():
    service, provider, manager = make_service()
    await service.handle_inbound(inbound("halo", message_id="wamid.1"))
    await service.handle_inbound(inbound("cari kaos hitam", message_id="wamid.2"))
    session = await manager.get_or_create("628123")
    # user + agent turns from both inbound messages are recorded.
    roles = [t.role for t in session.history]
    assert roles.count("user") == 2
    assert roles.count("agent") == 2
    assert len(provider.sent) == 2
