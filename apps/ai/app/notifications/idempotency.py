"""Event idempotency for the notifications consumer.

RabbitMQ delivery is at-least-once: on redelivery (consumer restart, ack lost,
requeue) the same event can arrive again. Without dedup we'd send the customer
duplicate "payment received" / "shipped" messages. We claim each event's
:attr:`~app.notifications.models.OrderEvent.dedup_id` in Redis with a TTL and
skip anything already seen.

Mirrors the inbound WhatsApp dedup (``SET NX EX``), but fails *closed* here:
if Redis is unavailable we return False (treat as duplicate → skip) so a Redis
blip can't cause a burst of duplicate customer notifications. Missing one
notification on an outage is safer than spamming; the customer can still check
status in-chat.
"""

from __future__ import annotations

from redis.asyncio import Redis

from app.platform.logging import get_logger

logger = get_logger(__name__)

_KEY_PREFIX = "notif:event:"


class EventDedup:
    """Redis-backed one-shot claim on event ids (fails closed on error)."""

    def __init__(self, redis: Redis, ttl_seconds: int):
        self._redis = redis
        self._ttl = max(1, ttl_seconds)

    def _key(self, dedup_id: str) -> str:
        return f"{_KEY_PREFIX}{dedup_id}"

    async def claim(self, dedup_id: str) -> bool:
        """Return True if this event is new (send it), False if a duplicate.

        On Redis failure we fail closed (return False) to avoid duplicate
        outbound notifications during an outage.
        """
        try:
            claimed = await self._redis.set(self._key(dedup_id), "1", nx=True, ex=self._ttl)
        except Exception:  # noqa: BLE001 — never crash the consumer on Redis errors
            logger.warning("event_dedup_unavailable", extra={"dedup_id": dedup_id})
            return False
        return bool(claimed)

    async def release(self, dedup_id: str) -> None:
        """Undo a claim so the event can be retried later.

        Called when sending fails after a successful claim, so a transient send
        failure doesn't permanently suppress the notification on redelivery.
        """
        try:
            await self._redis.delete(self._key(dedup_id))
        except Exception:  # noqa: BLE001
            logger.warning("event_dedup_release_failed", extra={"dedup_id": dedup_id})
