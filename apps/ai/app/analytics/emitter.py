"""Analytics emitter: append to a Redis sink + mirror to structured logs.

The emitter is the single entry point conversation/notification code uses to
record an :class:`AnalyticsEvent`. It:

- pushes the JSON event onto a capped Redis list (``analytics:events``) that a
  later dashboard/ETL job drains, and
- always logs the event (so metrics survive even if Redis is down).

Emission is strictly best-effort and must never affect the customer experience:
every failure is swallowed and logged. :class:`NullAnalyticsEmitter` is a no-op
used in tests / when analytics is disabled, so callers never branch on config.
"""

from __future__ import annotations

import json

from app.analytics.events import AnalyticsEvent, AnalyticsEventType, hash_wa_id
from app.platform.logging import get_logger

logger = get_logger(__name__)


class NullAnalyticsEmitter:
    """No-op emitter (analytics disabled / tests). Safe to call anywhere."""

    async def emit(
        self,
        event_type: AnalyticsEventType,
        *,
        wa_id: str | None = None,
        props: dict | None = None,
    ) -> None:
        return None


class AnalyticsEmitter:
    """Best-effort analytics sink backed by a capped Redis list."""

    _KEY = "analytics:events"
    _MAX = 10000

    def __init__(self, redis, *, max_events: int = _MAX):
        self._redis = redis
        self._max = max(1, max_events)

    async def emit(
        self,
        event_type: AnalyticsEventType,
        *,
        wa_id: str | None = None,
        props: dict | None = None,
    ) -> None:
        """Record an event. Never raises; failures are logged and dropped."""
        event = AnalyticsEvent(
            type=event_type, wa_hash=hash_wa_id(wa_id), props=props or {}
        )
        payload = event.to_dict()
        # Always log — this is the durable path if Redis is unavailable.
        log_extra = {"event": event_type.value}
        log_extra.update({f"p_{k}": v for k, v in payload["props"].items()})
        logger.info("analytics_event", extra=log_extra)
        try:

            await self._redis.lpush(self._KEY, json.dumps(payload))
            await self._redis.ltrim(self._KEY, 0, self._max - 1)
        except Exception:  # noqa: BLE001 — analytics must never break the flow
            logger.warning("analytics_emit_failed", extra={"event": event_type.value})
