"""Tests for abandoned cart recovery service (Phase A9)."""

from __future__ import annotations

from unittest.mock import AsyncMock, MagicMock

import pytest

from app.analytics.events import AnalyticsEventType
from app.conversation.models import ConversationContext, Session, SessionStatus
from app.recovery.service import CartRecoveryService


@pytest.fixture
def mock_backend():
    """Mock backend client."""
    backend = MagicMock()
    backend.get = AsyncMock()
    return backend


@pytest.fixture
def mock_sender():
    """Mock outbound sender."""
    sender = MagicMock()
    sender.send = AsyncMock(return_value=True)
    return sender


@pytest.fixture
def mock_sessions():
    """Mock session store."""
    sessions = MagicMock()
    sessions.iter_sessions = AsyncMock()
    sessions.save = AsyncMock()
    return sessions


@pytest.fixture
def mock_analytics():
    """Mock analytics emitter."""
    analytics = MagicMock()
    analytics.emit = AsyncMock()
    return analytics


def make_session(
    *,
    wa_id: str = "6281234567890",
    cart_id: str = "cart-1",
    customer_id: str = "cust-1",
    last_activity: int = 0,
    handover: bool = False,
    status: SessionStatus = SessionStatus.ACTIVE,
    extra: dict | None = None,
) -> Session:
    """Factory for test sessions."""
    context = ConversationContext(
        customer_id=customer_id,
        cart_id=cart_id,
        extra=extra or {},
    )
    return Session(
        wa_id=wa_id,
        last_activity=last_activity,
        handover=handover,
        status=status,
        context=context,
    )


@pytest.mark.asyncio
async def test_recovery_eligible_session(
    mock_backend, mock_sender, mock_sessions, mock_analytics
):
    """An idle cart with no prior reminders gets a reminder."""
    now = 10000
    session = make_session(last_activity=now - 4000)  # idle 4000s
    mock_sessions.iter_sessions.return_value = [session]
    mock_backend.get.return_value = {"items": [{"qty": 1}]}

    service = CartRecoveryService(
        backend=mock_backend,
        sender=mock_sender,
        sessions=mock_sessions,
        analytics=mock_analytics,
        idle_seconds=3600,
        cooldown_seconds=86400,
        max_reminders=1,
    )

    sent = await service.run_once(now=now)
    assert sent == 1
    mock_sender.send.assert_awaited_once()
    mock_sessions.save.assert_awaited_once()
    # Check that reminder count was incremented.
    assert session.context.extra["recovery_reminder_count"] == 1
    assert session.context.extra["recovery_last_reminded_at"] == now
    # Analytics event emitted.
    mock_analytics.emit.assert_awaited_once()
    args = mock_analytics.emit.await_args
    assert args[0][0] == AnalyticsEventType.ABANDONED_CART_REMINDER_SENT


@pytest.mark.asyncio
async def test_recovery_not_idle_enough(
    mock_backend, mock_sender, mock_sessions, mock_analytics
):
    """Session not idle long enough → skip."""
    now = 10000
    session = make_session(last_activity=now - 1000)  # only idle 1000s
    mock_sessions.iter_sessions.return_value = [session]

    service = CartRecoveryService(
        backend=mock_backend,
        sender=mock_sender,
        sessions=mock_sessions,
        analytics=mock_analytics,
        idle_seconds=3600,
    )

    sent = await service.run_once(now=now)
    assert sent == 0
    mock_sender.send.assert_not_awaited()


@pytest.mark.asyncio
async def test_recovery_no_cart(mock_backend, mock_sender, mock_sessions, mock_analytics):
    """Session without a cart_id → skip."""
    now = 10000
    session = make_session(cart_id="", last_activity=now - 4000)
    mock_sessions.iter_sessions.return_value = [session]

    service = CartRecoveryService(
        backend=mock_backend,
        sender=mock_sender,
        sessions=mock_sessions,
        analytics=mock_analytics,
        idle_seconds=3600,
    )

    sent = await service.run_once(now=now)
    assert sent == 0


@pytest.mark.asyncio
async def test_recovery_handover(mock_backend, mock_sender, mock_sessions, mock_analytics):
    """Session in handover → skip."""
    now = 10000
    session = make_session(handover=True, last_activity=now - 4000)
    mock_sessions.iter_sessions.return_value = [session]

    service = CartRecoveryService(
        backend=mock_backend,
        sender=mock_sender,
        sessions=mock_sessions,
        analytics=mock_analytics,
        idle_seconds=3600,
    )

    sent = await service.run_once(now=now)
    assert sent == 0


@pytest.mark.asyncio
async def test_recovery_max_reminders_reached(
    mock_backend, mock_sender, mock_sessions, mock_analytics
):
    """Already sent max reminders → skip."""
    now = 10000
    session = make_session(
        last_activity=now - 4000,
        extra={"recovery_reminder_count": 1},
    )
    mock_sessions.iter_sessions.return_value = [session]

    service = CartRecoveryService(
        backend=mock_backend,
        sender=mock_sender,
        sessions=mock_sessions,
        analytics=mock_analytics,
        idle_seconds=3600,
        max_reminders=1,
    )

    sent = await service.run_once(now=now)
    assert sent == 0


@pytest.mark.asyncio
async def test_recovery_cooldown_active(
    mock_backend, mock_sender, mock_sessions, mock_analytics
):
    """Within cooldown period → skip."""
    now = 10000
    session = make_session(
        last_activity=now - 4000,
        extra={
            "recovery_reminder_count": 0,
            "recovery_last_reminded_at": now - 1000,  # reminded 1000s ago
        },
    )
    mock_sessions.iter_sessions.return_value = [session]

    service = CartRecoveryService(
        backend=mock_backend,
        sender=mock_sender,
        sessions=mock_sessions,
        analytics=mock_analytics,
        idle_seconds=3600,
        cooldown_seconds=86400,  # 24h cooldown
    )

    sent = await service.run_once(now=now)
    assert sent == 0


@pytest.mark.asyncio
async def test_recovery_empty_cart(
    mock_backend, mock_sender, mock_sessions, mock_analytics
):
    """Cart has no items → skip."""
    now = 10000
    session = make_session(last_activity=now - 4000)
    mock_sessions.iter_sessions.return_value = [session]
    mock_backend.get.return_value = {"items": []}

    service = CartRecoveryService(
        backend=mock_backend,
        sender=mock_sender,
        sessions=mock_sessions,
        analytics=mock_analytics,
        idle_seconds=3600,
    )

    sent = await service.run_once(now=now)
    assert sent == 0


@pytest.mark.asyncio
async def test_recovery_backend_error(
    mock_backend, mock_sender, mock_sessions, mock_analytics
):
    """Backend error fetching cart → skip gracefully."""
    now = 10000
    session = make_session(last_activity=now - 4000)
    mock_sessions.iter_sessions.return_value = [session]
    mock_backend.get.side_effect = Exception("Backend down")

    service = CartRecoveryService(
        backend=mock_backend,
        sender=mock_sender,
        sessions=mock_sessions,
        analytics=mock_analytics,
        idle_seconds=3600,
    )

    sent = await service.run_once(now=now)
    assert sent == 0


@pytest.mark.asyncio
async def test_recovery_send_failed(
    mock_backend, mock_sender, mock_sessions, mock_analytics
):
    """Send fails → don't mark as reminded."""
    now = 10000
    session = make_session(last_activity=now - 4000)
    mock_sessions.iter_sessions.return_value = [session]
    mock_backend.get.return_value = {"items": [{"qty": 1}]}
    mock_sender.send.return_value = False  # send failed

    service = CartRecoveryService(
        backend=mock_backend,
        sender=mock_sender,
        sessions=mock_sessions,
        analytics=mock_analytics,
        idle_seconds=3600,
    )

    sent = await service.run_once(now=now)
    assert sent == 0
    # Session wasn't saved because send failed.
    mock_sessions.save.assert_not_awaited()


@pytest.mark.asyncio
async def test_recovery_multiple_sessions(
    mock_backend, mock_sender, mock_sessions, mock_analytics
):
    """Multiple sessions: send to eligible ones only."""
    now = 10000
    session1 = make_session(wa_id="111", last_activity=now - 4000)  # eligible
    session2 = make_session(wa_id="222", last_activity=now - 1000)  # not idle enough
    session3 = make_session(wa_id="333", last_activity=now - 5000)  # eligible
    mock_sessions.iter_sessions.return_value = [session1, session2, session3]
    mock_backend.get.return_value = {"items": [{"qty": 1}]}

    service = CartRecoveryService(
        backend=mock_backend,
        sender=mock_sender,
        sessions=mock_sessions,
        analytics=mock_analytics,
        idle_seconds=3600,
    )

    sent = await service.run_once(now=now)
    assert sent == 2
    assert mock_sender.send.await_count == 2