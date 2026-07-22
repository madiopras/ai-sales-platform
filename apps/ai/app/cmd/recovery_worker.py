"""Abandoned cart recovery worker (Phase A9).

Runs a periodic sweep that sends gentle reminders to customers who left items in
their cart. The sweep interval and reminder policy are tuned via environment
config. This worker is opt-in: it only runs when explicitly started (separate
from the main FastAPI service) so deployments control when/how recovery runs.

Usage:
    python -m app.cmd.recovery_worker

The worker connects to the same Redis + backend that the main service uses, so
config comes from the same .env. It runs until interrupted (Ctrl+C) and exits
cleanly on SIGINT/SIGTERM.
"""

from __future__ import annotations

import asyncio
import signal
import sys
from contextlib import asynccontextmanager

from app.analytics.emitter import AnalyticsEmitter, NullAnalyticsEmitter
from app.clients.backend import BackendClient
from app.clients.redis import create_redis
from app.config import get_settings
from app.conversation.store import SessionStore
from app.platform.logging import configure_logging, get_logger
from app.recovery.service import CartRecoveryService
from app.whatsapp.factory import create_provider
from app.whatsapp.outbound import OutboundSender

logger = get_logger(__name__)


class RecoveryWorker:
    """Periodic sweep runner with graceful shutdown."""

    def __init__(
        self,
        *,
        service: CartRecoveryService,
        interval_seconds: int,
    ):
        self._service = service
        self._interval = max(60, interval_seconds)
        self._running = False
        self._task: asyncio.Task | None = None

    async def start(self) -> None:
        """Start the periodic sweep loop."""
        if self._running:
            return
        self._running = True
        self._task = asyncio.create_task(self._loop())
        logger.info("recovery_worker_started", extra={"interval_seconds": self._interval})

    async def stop(self) -> None:
        """Stop the sweep loop gracefully."""
        if not self._running:
            return
        self._running = False
        if self._task is not None:
            self._task.cancel()
            try:
                await self._task
            except asyncio.CancelledError:
                pass
        logger.info("recovery_worker_stopped")

    async def _loop(self) -> None:
        """Run sweeps at the configured interval until stopped."""
        while self._running:
            try:
                sent = await self._service.run_once()
                if sent > 0:
                    logger.info("recovery_sweep_completed", extra={"reminders_sent": sent})
            except Exception:  # noqa: BLE001
                logger.exception("recovery_sweep_error")
            await asyncio.sleep(self._interval)


@asynccontextmanager
async def _lifespan():
    """Wire up the backend, Redis, WhatsApp sender, and recovery service."""
    settings = get_settings()
    configure_logging(settings.log_level)

    backend = BackendClient(settings)
    redis = create_redis(settings)
    provider = create_provider(settings)
    sender = OutboundSender(provider, settings.wa_send_max_retries)
    sessions = SessionStore(redis, settings.session_ttl_seconds)

    # Analytics emitter (opt-in via config, same as main service).
    analytics = (
        AnalyticsEmitter(redis) if settings.analytics_enabled else NullAnalyticsEmitter()
    )

    # Recovery service with configurable policy (idle, cooldown, max reminders).
    service = CartRecoveryService(
        backend=backend,
        sender=sender,
        sessions=sessions,
        analytics=analytics,
        idle_seconds=settings.recovery_idle_seconds,
        cooldown_seconds=settings.recovery_cooldown_seconds,
        max_reminders=settings.recovery_max_reminders,
    )

    # Worker runs the sweep at the configured interval.
    worker = RecoveryWorker(
        service=service,
        interval_seconds=settings.recovery_sweep_interval_seconds,
    )

    try:
        yield worker
    finally:
        await provider.aclose()
        await backend.aclose()
        await redis.aclose()


async def _main() -> None:
    """Run the recovery worker until interrupted."""
    stop_event = asyncio.Event()

    def _signal_handler(signum, frame):  # noqa: ARG001
        logger.info("signal_received", extra={"signal": signum})
        stop_event.set()

    signal.signal(signal.SIGINT, _signal_handler)
    signal.signal(signal.SIGTERM, _signal_handler)

    async with _lifespan() as worker:
        await worker.start()
        await stop_event.wait()
        await worker.stop()


if __name__ == "__main__":
    try:
        asyncio.run(_main())
    except KeyboardInterrupt:
        logger.info("worker_interrupted")
        sys.exit(0)