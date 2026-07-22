"""RabbitMQ transport for order lifecycle events.

Thin by design: it owns only the broker plumbing (connect, declare the topic
exchange + durable queue, bind routing keys, consume) and delegates every event
to :class:`~app.notifications.service.NotificationService`, which decides whether
to ack or retry. All notification logic — parsing, templating, dedup, sending —
lives in the service so it stays broker-free and unit-testable.

``aio-pika`` is imported lazily so the dependency is only needed when
notifications are actually enabled (``NOTIFICATIONS_ENABLED=true``); tests and
offline runs never import it.

Delivery semantics:
- Durable queue + persistent-friendly manual acks: an event is acked only after
  the service reports it handled (or intentionally skipped) it.
- ``HandleResult.RETRY`` → ``nack(requeue=True)`` so transient failures (backend
  down, send failed) are retried rather than lost.
- The service is idempotent (Redis dedup), so at-least-once redelivery is safe.
"""

from __future__ import annotations

import asyncio
from typing import Any

from app.config import Settings
from app.notifications.service import HandleResult, NotificationService
from app.platform.logging import get_logger

logger = get_logger(__name__)


def _routing_keys(settings: Settings) -> list[str]:
    return [k.strip() for k in settings.rabbitmq_routing_keys.split(",") if k.strip()]


class NotificationConsumer:
    """Consumes domain events from RabbitMQ and feeds the notification service."""

    def __init__(self, settings: Settings, service: NotificationService):
        self._settings = settings
        self._service = service
        self._connection: Any = None
        self._channel: Any = None
        self._task: asyncio.Task[None] | None = None

    async def start(self) -> None:
        """Connect, declare topology, and begin consuming in a background task.

        Best-effort: a broker that's unreachable at startup is logged and skipped
        rather than crashing the app — the rest of the service (webhook, chat)
        stays up. A supervisor/restart brings the consumer back.
        """
        try:
            import aio_pika
        except ImportError:
            logger.error("notif_consumer_missing_dep", extra={"dep": "aio-pika"})
            return

        try:
            self._connection = await aio_pika.connect_robust(self._settings.rabbitmq_url)
            self._channel = await self._connection.channel()
            await self._channel.set_qos(prefetch_count=self._settings.rabbitmq_prefetch)

            exchange = await self._channel.declare_exchange(
                self._settings.rabbitmq_exchange,
                type=self._settings.rabbitmq_exchange_type,
                durable=True,
            )
            queue = await self._channel.declare_queue(
                self._settings.rabbitmq_queue, durable=True
            )
            for key in _routing_keys(self._settings):
                await queue.bind(exchange, routing_key=key)

            self._task = asyncio.create_task(queue.consume(self._on_message))
            logger.info(
                "notif_consumer_started",
                extra={
                    "queue": self._settings.rabbitmq_queue,
                    "exchange": self._settings.rabbitmq_exchange,
                },
            )
        except Exception:  # noqa: BLE001 — never let a broker issue crash startup
            logger.exception("notif_consumer_start_failed")
            await self.stop()

    async def _on_message(self, message: Any) -> None:
        """Handle one delivery: run the service, then ack or requeue."""
        result = HandleResult.RETRY
        try:
            result = await self._service.handle(message.body)
        except Exception:  # noqa: BLE001 — service shouldn't raise, but be safe
            logger.exception("notif_consumer_handle_error")
            result = HandleResult.RETRY

        try:
            if result is HandleResult.ACK:
                await message.ack()
            else:
                await message.nack(requeue=True)
        except Exception:  # noqa: BLE001 — ack/nack on a dropped channel
            logger.warning("notif_consumer_ack_failed")

    async def stop(self) -> None:
        """Cancel consumption and close the connection (idempotent)."""
        if self._task is not None:
            self._task.cancel()
            self._task = None
        if self._connection is not None:
            try:
                await self._connection.close()
            except Exception:  # noqa: BLE001
                logger.warning("notif_consumer_close_failed")
            self._connection = None
            self._channel = None
