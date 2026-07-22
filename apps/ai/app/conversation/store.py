"""Redis persistence for conversation sessions.

Sessions are stored as a single JSON blob per WhatsApp id with a sliding TTL:
every save refreshes the expiry, so an idle conversation naturally expires after
``session_ttl_seconds`` (the Phase A3 idle timeout). This keeps the store simple
and lets Redis, not a scheduler, garbage-collect abandoned sessions.

On Redis failure we fail soft (return ``None`` / swallow) so a transient blip
degrades to a fresh session rather than crashing message handling — consistent
with the inbound dedup policy.
"""

from __future__ import annotations

import json

from redis.asyncio import Redis

from app.conversation.models import Session
from app.platform.logging import get_logger

logger = get_logger(__name__)

_KEY_PREFIX = "conv:session:"


class SessionStore:
    """Load/save/delete :class:`Session` objects in Redis with a sliding TTL."""

    def __init__(self, redis: Redis, ttl_seconds: int):
        self._redis = redis
        self._ttl = max(1, ttl_seconds)

    def _key(self, wa_id: str) -> str:
        return f"{_KEY_PREFIX}{wa_id}"

    async def load(self, wa_id: str) -> Session | None:
        """Return the stored session for ``wa_id``, or None if absent/unreadable."""
        try:
            raw = await self._redis.get(self._key(wa_id))
        except Exception:  # noqa: BLE001 — state must not break message handling
            logger.warning("session_load_unavailable", extra={"wa_id_hash": _hash(wa_id)})
            return None
        if not raw:
            return None
        try:
            return Session.from_dict(json.loads(raw))
        except (ValueError, KeyError):
            # Corrupt/incompatible blob: drop it and start fresh.
            logger.warning("session_corrupt", extra={"wa_id_hash": _hash(wa_id)})
            return None

    async def save(self, session: Session) -> None:
        """Persist a session and refresh its TTL (sliding idle timeout)."""
        try:
            await self._redis.set(
                self._key(session.wa_id),
                json.dumps(session.to_dict()),
                ex=self._ttl,
            )
        except Exception:  # noqa: BLE001 — best-effort persistence
            logger.warning("session_save_unavailable", extra={"wa_id_hash": _hash(session.wa_id)})

    async def delete(self, wa_id: str) -> None:
        """Remove a session (e.g. after the flow completes)."""
        try:
            await self._redis.delete(self._key(wa_id))
        except Exception:  # noqa: BLE001
            logger.warning("session_delete_unavailable", extra={"wa_id_hash": _hash(wa_id)})

    async def iter_sessions(self):
        """Yield every stored session (best-effort; used by background sweeps).

        Uses ``SCAN`` so it never blocks Redis on a large keyspace. Unreadable or
        corrupt blobs are skipped. Intended for infrequent maintenance jobs like
        the abandoned-cart recovery sweep (Phase A9), not the hot path.
        """
        try:
            async for key in self._redis.scan_iter(match=f"{_KEY_PREFIX}*"):
                wa_id = _wa_id_from_key(key)
                if not wa_id:
                    continue
                session = await self.load(wa_id)
                if session is not None:
                    yield session
        except Exception:  # noqa: BLE001 — sweeps must not crash on Redis errors
            logger.warning("session_scan_unavailable")
            return



def _wa_id_from_key(key: str | bytes) -> str | None:
    """Extract the wa_id from a session key (handles bytes from Redis)."""
    if isinstance(key, bytes):
        key = key.decode("utf-8", "ignore")
    if not key.startswith(_KEY_PREFIX):
        return None
    return key[len(_KEY_PREFIX):] or None


def _hash(wa_id: str) -> str:
    """Coarse, non-reversible marker for logs so we never log the phone (PII)."""
    return f"...{wa_id[-4:]}" if len(wa_id) >= 4 else "****"

