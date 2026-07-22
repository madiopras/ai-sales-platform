"""Async Redis connection factory.

Redis backs conversation/session state (Phase A3) and inbound/event
idempotency (Phase A2/A7). This module only owns the connection lifecycle;
key schemas live with the features that use them.
"""

from __future__ import annotations

from redis.asyncio import Redis

from app.config import Settings


def create_redis(settings: Settings) -> Redis:
    """Create an async Redis client from the configured URL.

    ``decode_responses=True`` so callers work with ``str`` rather than bytes.
    The connection is lazy; readiness is verified via :func:`ping`.
    """
    return Redis.from_url(settings.redis_url, decode_responses=True)


async def ping(redis: Redis) -> bool:
    """Return True if Redis responds to PING, False otherwise."""
    try:
        return bool(await redis.ping())
    except Exception:  # noqa: BLE001 — readiness check must never raise
        return False
