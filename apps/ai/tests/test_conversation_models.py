"""Tests for conversation state models: serialization + history trimming."""

from __future__ import annotations

from app.conversation.models import (
    ConversationState,
    Session,
    SessionStatus,
    flow_index,
)


def test_session_roundtrip_serialization():
    session = Session(wa_id="628123")
    session.context.name = "Budi"
    session.context.customer_id = "cust-1"
    session.state = ConversationState.CART
    session.add_turn("user", "halo", max_history=10)
    session.add_turn("agent", "hai", max_history=10)

    restored = Session.from_dict(session.to_dict())

    assert restored.wa_id == "628123"
    assert restored.state == ConversationState.CART
    assert restored.status == SessionStatus.ACTIVE
    assert restored.context.name == "Budi"
    assert restored.context.customer_id == "cust-1"
    assert [t.text for t in restored.history] == ["halo", "hai"]


def test_add_turn_trims_to_history_max():
    session = Session(wa_id="628123")
    for i in range(10):
        session.add_turn("user", f"msg{i}", max_history=3)
    assert len(session.history) == 3
    assert [t.text for t in session.history] == ["msg7", "msg8", "msg9"]


def test_add_turn_ignores_empty_text():
    session = Session(wa_id="628123")
    session.add_turn("user", "", max_history=5)
    assert session.history == []


def test_flow_index_orders_forward_states():
    assert flow_index(ConversationState.GREETING) == 0
    assert flow_index(ConversationState.PAYMENT) > flow_index(ConversationState.CART)
    # HANDOVER is off the linear flow.
    assert flow_index(ConversationState.HANDOVER) == -1
