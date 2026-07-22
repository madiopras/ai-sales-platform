"""Inbound idempotency for WhatsApp webhooks.

WhatsApp providers retry webhook delivery until they get a 200. Without dedup
we would process (and reply to) the same message multiple times. We record each
processed ``message_id`` in Redis with a TTL and skip anything we've already
seen.

Uses ``SET key value NX EX ttl`` so claiming a message id is atomic: the first
caller gets ``True`` (proceed), retries get ``False`` (skip).
"""

from __future__ import annotations

from redis.asyncio import Redis

from app.platform.logging import get_logger

logger = get_logger(__name__)

_KEY_PREFIX = "wa:inbound:"


class InboundDedup:
    """Redis-backed one-shot claim on inbound message ids."""

    def __init__(self, redis: Redis, ttl_seconds: int):
        self._redis = redis
        self._ttl = max(1, ttl_seconds)

    def _key(self, message_id: str) -> str:
        return f"{_KEY_PREFIX}{message_id}"

    async def claim(self, message_id: str) -> bool:
        """Atomically claim a message id for processing.

        Returns True if this is the first time we've seen ``message_id`` (caller
        should process it), False if it was already claimed (duplicate — skip).

        On Redis failure we fail open (return True) so a transient Redis blip
        doesn't drop customer messages; at-least-once is safer than at-most-once
        for inbound sales conversations.
        """
        try:
            claimed = await self._redis.set(self._key(message_id), "1", nx=True, ex=self._ttl)
        except Exception:  # noqa: BLE001 — dedup must not break message handling
            logger.warning("inbound_dedup_unavailable", extra={"message_id": message_id})
            return True
        return bool(claimed)
