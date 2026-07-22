"""Tests for analytics emitter and events (Phase A9)."""

from __future__ import annotations

import json
from unittest.mock import AsyncMock, MagicMock

import pytest

from app.analytics.emitter import AnalyticsEmitter, NullAnalyticsEmitter
from app.analytics.events import AnalyticsEvent, AnalyticsEventType, hash_wa_id


def test_hash_wa_id():
    """Hash WA ID to a non-reversible marker."""
    # Last 4 digits.
    assert hash_wa_id("6281234567890") == "...7890"
    assert hash_wa_id("123") == "****"
    assert hash_wa_id("") is None
    assert hash_wa_id(None) is None


def test_analytics_event_to_dict():
    """Event serializes to a dict."""
    event = AnalyticsEvent(
        type=AnalyticsEventType.CONVERSATION_STARTED,
        ts=1234567890,
        wa_hash="...1234",
        props={"key": "value"},
    )
    result = event.to_dict()
    assert result["type"] == "conversation_started"
    assert result["ts"] == 1234567890
    assert result["wa_hash"] == "...1234"
    assert result["props"] == {"key": "value"}


@pytest.mark.asyncio
async def test_null_emitter():
    """NullAnalyticsEmitter is a safe no-op."""
    emitter = NullAnalyticsEmitter()
    # Should not raise.
    await emitter.emit(AnalyticsEventType.MESSAGE_RECEIVED, wa_id="test")


@pytest.mark.asyncio
async def test_emitter_push_to_redis():
    """AnalyticsEmitter pushes events to a Redis list."""
    redis = MagicMock()
    redis.lpush = AsyncMock()
    redis.ltrim = AsyncMock()

    emitter = AnalyticsEmitter(redis, max_events=100)
    await emitter.emit(
        AnalyticsEventType.CART_ITEM_ADDED,
        wa_id="6281234567890",
        props={"variant_id": "var-1", "qty": 2},
    )

    # Check lpush was called.
    redis.lpush.assert_awaited_once()
    args = redis.lpush.await_args[0]
    assert args[0] == "analytics:events"
    payload = json.loads(args[1])
    assert payload["type"] == "cart_item_added"
    assert payload["wa_hash"] == "...7890"
    assert payload["props"]["variant_id"] == "var-1"

    # Check ltrim was called to cap the list.
    redis.ltrim.assert_awaited_once_with("analytics:events", 0, 99)


@pytest.mark.asyncio
async def test_emitter_redis_failure():
    """Redis failure is swallowed; never raises."""
    redis = MagicMock()
    redis.lpush = AsyncMock(side_effect=Exception("Redis down"))
    redis.ltrim = AsyncMock()

    emitter = AnalyticsEmitter(redis)
    # Should not raise.
    await emitter.emit(AnalyticsEventType.ORDER_PAID, wa_id="test")


@pytest.mark.asyncio
async def test_emitter_no_wa_id():
    """Event without wa_id still works."""
    redis = MagicMock()
    redis.lpush = AsyncMock()
    redis.ltrim = AsyncMock()

    emitter = AnalyticsEmitter(redis)
    await emitter.emit(AnalyticsEventType.HANDOVER_TRIGGERED)

    args = redis.lpush.await_args[0]
    payload = json.loads(args[1])
    assert payload["wa_hash"] is None


@pytest.mark.asyncio
async def test_emitter_custom_max_events():
    """Custom max_events is respected in ltrim."""
    redis = MagicMock()
    redis.lpush = AsyncMock()
    redis.ltrim = AsyncMock()

    emitter = AnalyticsEmitter(redis, max_events=500)
    await emitter.emit(AnalyticsEventType.MESSAGE_RECEIVED)

    redis.ltrim.assert_awaited_once_with("analytics:events", 0, 499)


def test_event_types_coverage():
    """All documented event types are present."""
    expected = {
        "conversation_started",
        "message_received",
        "cart_item_added",
        "checkout_previewed",
        "order_created",
        "order_paid",
        "cart_abandoned",
        "abandoned_cart_reminder_sent",
        "handover_triggered",
        "kb_answered",
        "injection_blocked",
        "recommendation_made",
    }
    actual = {e.value for e in AnalyticsEventType}
    assert actual == expected