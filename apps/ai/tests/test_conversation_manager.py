"""Tests for the conversation manager: transitions, timeout, handover, store."""

from __future__ import annotations

from app.conversation.manager import ConversationManager
from app.conversation.models import ConversationState, Session, SessionStatus
from app.conversation.store import SessionStore


class FakeRedis:
    """In-memory Redis stub supporting get/set(ex)/delete used by the store."""

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


def make_manager(*, ttl: int = 1800, history: int = 20) -> tuple[ConversationManager, FakeRedis]:
    redis = FakeRedis()
    store = SessionStore(redis, ttl_seconds=ttl)
    manager = ConversationManager(store, idle_timeout_seconds=ttl, history_max=history)
    return manager, redis


async def test_first_message_creates_session_and_advances_from_greeting():
    manager, _ = make_manager()
    session = await manager.record_inbound("628123", "halo", name="Budi")
    assert session.wa_id == "628123"
    assert session.state == ConversationState.DISCOVERY
    assert session.context.name == "Budi"
    assert session.status == SessionStatus.ACTIVE
    assert [t.text for t in session.history] == ["halo"]


async def test_session_persists_across_messages():
    manager, _ = make_manager()
    await manager.record_inbound("628123", "halo")
    session = await manager.record_inbound("628123", "cari kaos hitam")
    assert session.state == ConversationState.PRODUCT
    # History accumulates across turns.
    assert [t.text for t in session.history] == ["halo", "cari kaos hitam"]


async def test_intent_keyword_jumps_to_shipping():
    manager, _ = make_manager()
    session = await manager.record_inbound("628123", "tolong hitung ongkir ke Jakarta")
    assert session.state == ConversationState.SHIPPING


async def test_handover_keyword_sets_handover_and_status():
    manager, _ = make_manager()
    session = await manager.record_inbound("628123", "saya mau komplain barang rusak")
    assert session.state == ConversationState.HANDOVER
    assert session.status == SessionStatus.HANDOVER
    assert session.handover is True


async def test_handover_session_stops_advancing():
    manager, _ = make_manager()
    await manager.record_inbound("628123", "mau refund dong")
    # A follow-up product-ish message must not pull it back out of handover.
    session = await manager.record_inbound("628123", "cari produk lain")
    assert session.status == SessionStatus.HANDOVER
    assert session.state == ConversationState.HANDOVER


async def test_idle_timeout_starts_fresh_but_keeps_name():
    manager, _ = make_manager(ttl=100)
    first = await manager.record_inbound("628123", "cari kaos", name="Budi", now=1000)
    assert first.state == ConversationState.PRODUCT
    # Far in the future → previous session considered expired.
    session = await manager.get_or_create("628123", now=1000 + 200)
    assert session.state == ConversationState.GREETING
    assert session.history == []
    assert session.context.name == "Budi"  # personalization retained


async def test_agent_reply_recorded_in_history():
    manager, _ = make_manager()
    session = await manager.record_inbound("628123", "halo")
    await manager.record_agent_reply(session, "Halo Budi!")
    reloaded = await manager.get_or_create("628123")
    assert reloaded.history[-1].role == "agent"
    assert reloaded.history[-1].text == "Halo Budi!"


async def test_set_handover_forces_handover():
    manager, _ = make_manager()
    session = await manager.set_handover("628123")
    assert session.handover is True
    assert session.status == SessionStatus.HANDOVER


async def test_store_delete_removes_session():
    manager, redis = make_manager()
    await manager.record_inbound("628123", "halo")
    store = SessionStore(redis, ttl_seconds=1800)
    await store.delete("628123")
    assert await store.load("628123") is None


async def test_store_corrupt_blob_returns_none():
    redis = FakeRedis()
    redis.store["conv:session:628123"] = "{not valid json"
    store = SessionStore(redis, ttl_seconds=1800)
    assert await store.load("628123") is None


async def test_store_save_load_roundtrip():
    redis = FakeRedis()
    store = SessionStore(redis, ttl_seconds=1800)
    session = Session(wa_id="628123")
    session.state = ConversationState.CART
    await store.save(session)
    loaded = await store.load("628123")
    assert loaded is not None
    assert loaded.state == ConversationState.CART
