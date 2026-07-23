"""FastAPI application entrypoint.

Wires configuration, logging, the shared backend + Redis clients (managed over
the app lifespan), middleware, exception handlers, and the routers. Phase A2
adds the WhatsApp provider + webhook router. Phase A3/A4 add conversation state
and LLM orchestration; Phase A7 adds the RabbitMQ notification consumer; Phase
A9 adds analytics emitter.
"""

from __future__ import annotations

from contextlib import asynccontextmanager

from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

from app.analytics.emitter import AnalyticsEmitter, NullAnalyticsEmitter
from app.clients.backend import BackendClient
from app.clients.redis import create_redis
from app.config import get_settings
from app.conversation.manager import ConversationManager
from app.conversation.store import SessionStore
from app.health import router as health_router
from app.llm.factory import create_orchestrator
from app.middleware import RequestContextMiddleware
from app.notifications.consumer import NotificationConsumer
from app.notifications.idempotency import EventDedup
from app.notifications.service import NotificationService
from app.platform.errors import register_exception_handlers
from app.platform.logging import configure_logging, get_logger
from app.safety.handover import AdminNotifier
from app.whatsapp.factory import create_provider
from app.whatsapp.idempotency import InboundDedup
from app.whatsapp.outbound import OutboundSender
from app.whatsapp.router import router as whatsapp_router
from app.whatsapp.service import WhatsAppService


@asynccontextmanager
async def lifespan(app: FastAPI):
    settings = get_settings()
    configure_logging(settings.log_level)
    logger = get_logger("app")

    app.state.settings = settings
    app.state.backend = BackendClient(settings)
    app.state.redis = create_redis(settings)

    # Analytics emitter (Phase A9). Opt-in via config so tests/local dev can
    # disable it. The emitter is exposed on app.state so conversation/notif
    # services can emit events (analytics is cross-cutting).
    app.state.analytics = (
        AnalyticsEmitter(app.state.redis, max_events=settings.analytics_max_events)
        if settings.analytics_enabled
        else NullAnalyticsEmitter()
    )

    # Conversation state (Phase A3) + human handover (Phase A8). The admin
    # notifier publishes handover alerts to Redis so the (future) admin
    # dashboard can pick them up and a person can take over.
    conversations = ConversationManager(
        SessionStore(app.state.redis, settings.session_ttl_seconds),
        idle_timeout_seconds=settings.session_ttl_seconds,
        history_max=settings.session_history_max,
        admin_notifier=AdminNotifier(app.state.redis),
    )

    # LLM orchestration (Phase A4): system prompt + tool-calling agent. The
    # chat model is kept so it can be closed on shutdown (real providers hold an
    # HTTP client).

    orchestrator, chat_model = create_orchestrator(settings, app.state.backend)
    app.state.chat_model = chat_model

    # WhatsApp (Phase A2): provider + inbound dedup + outbound sender + service.
    provider = create_provider(settings)
    app.state.whatsapp = WhatsAppService(
        settings=settings,
        provider=provider,
        dedup=InboundDedup(app.state.redis, settings.wa_inbound_dedup_ttl_seconds),
        sender=OutboundSender(provider, settings.wa_send_max_retries),
        conversations=conversations,
        orchestrator=orchestrator,
        analytics=app.state.analytics,
    )

    # Notifications (Phase A7): consume order lifecycle events from RabbitMQ and
    # push WhatsApp updates. Opt-in (NOTIFICATIONS_ENABLED) so tests/offline runs
    # don't require a broker. Best-effort startup: a broker outage won't take
    # down the webhook/chat path.
    app.state.notification_consumer = None
    if settings.notifications_enabled:
        notification_service = NotificationService(
            backend=app.state.backend,
            sender=OutboundSender(provider, settings.wa_send_max_retries),
            dedup=EventDedup(app.state.redis, settings.event_dedup_ttl_seconds),
            sessions=SessionStore(app.state.redis, settings.session_ttl_seconds),
        )
        consumer = NotificationConsumer(settings, notification_service)
        await consumer.start()
        app.state.notification_consumer = consumer

    logger.info(
        "service_started",
        extra={
            "app_name": settings.app_name,
            "env": settings.app_env,
            "llm_provider": chat_model.name,
            "notifications": settings.notifications_enabled,
            "analytics": settings.analytics_enabled,
        },
    )
    try:
        yield
    finally:
        if app.state.notification_consumer is not None:
            await app.state.notification_consumer.stop()
        await app.state.whatsapp.provider.aclose()
        await app.state.chat_model.aclose()
        await app.state.backend.aclose()
        await app.state.redis.aclose()
        logger.info("service_stopped")


def create_app() -> FastAPI:
    settings = get_settings()
    # Configure logging early so import-time / startup logs are structured too.
    configure_logging(settings.log_level)

    app = FastAPI(
        title="AI Sales Agent",
        version="0.1.0",
        docs_url="/docs" if not settings.is_production else None,
        redoc_url=None,
        lifespan=lifespan,
    )

    # CORS — allow the admin frontend (and other trusted origins) to call the
    # AI service directly from the browser (e.g. inbox endpoints).
    allowed_origins = [
        o.strip() for o in settings.cors_allowed_origins.split(",") if o.strip()
    ]
    app.add_middleware(
        CORSMiddleware,
        allow_origins=allowed_origins,
        allow_credentials=True,
        allow_methods=["*"],
        allow_headers=["*"],
        max_age=86400,
    )

    app.add_middleware(RequestContextMiddleware)
    register_exception_handlers(app)
    app.include_router(health_router)
    app.include_router(whatsapp_router)

    return app


app = create_app()
