"""Environment-driven settings for the AI service.

All configuration comes from environment variables (or an .env file in local
development). Secrets are never hardcoded and must never be logged (BR-044).
"""

from __future__ import annotations

from functools import lru_cache

from pydantic import Field
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict(
        env_file=".env",
        env_file_encoding="utf-8",
        extra="ignore",
        case_sensitive=False,
    )

    # --- App ---
    app_name: str = Field(default="ai-sales-ai")
    app_env: str = Field(default="development")
    log_level: str = Field(default="INFO")

    # --- HTTP server ---
    http_host: str = Field(default="0.0.0.0")
    http_port: int = Field(default=8090)

    # --- Backend (Go /internal/v1) ---
    backend_base_url: str = Field(default="http://localhost:8081")
    backend_service_token: str = Field(default="")
    backend_timeout_seconds: float = Field(default=10.0)
    backend_max_retries: int = Field(default=2)

    # --- Redis ---
    redis_url: str = Field(default="redis://localhost:6379/0")

    # --- Conversation / session state (Phase A3) ---
    # Idle timeout: a session with no activity for this long expires (Redis TTL).
    session_ttl_seconds: int = Field(default=1800)
    # Max messages kept in the rolling history window (for LLM context budget).
    session_history_max: int = Field(default=20)

    # --- RabbitMQ / event-driven notifications (Phase A7) ---
    rabbitmq_url: str = Field(default="amqp://guest:guest@localhost:5672/")
    # Opt-in: when false the consumer is not started (default so tests/offline
    # runs don't require a broker). Set true in deployments with RabbitMQ.
    notifications_enabled: bool = Field(default=False)
    # Topic exchange the Go backend publishes order lifecycle events to, plus the
    # durable queue this service binds. Event names double as routing keys.
    rabbitmq_exchange: str = Field(default="domain.events")
    rabbitmq_exchange_type: str = Field(default="topic")
    rabbitmq_queue: str = Field(default="ai.notifications")
    rabbitmq_routing_keys: str = Field(
        default="order.paid,order.shipped,order.delivered,order.completed,order.cancelled"
    )
    # Consumer prefetch (unacked messages in flight).
    rabbitmq_prefetch: int = Field(default=10)
    # How long a processed event is remembered so redelivery doesn't double-send.
    event_dedup_ttl_seconds: int = Field(default=86400)

    # --- Analytics (Phase A9) ---
    # Opt-in: when true the analytics emitter pushes events to Redis. Defaults to
    # false so tests/local dev don't require Redis config if not needed.
    analytics_enabled: bool = Field(default=False)
    # Max events kept in the Redis analytics list (capped to prevent unbounded
    # growth). The admin dashboard or ETL job is expected to drain this regularly.
    analytics_max_events: int = Field(default=10000)

    # --- Abandoned cart recovery (Phase A9) ---
    # How long a cart must be idle before qualifying for a reminder (seconds).
    recovery_idle_seconds: int = Field(default=3600)
    # Cooldown between reminders to the same customer (seconds).
    recovery_cooldown_seconds: int = Field(default=86400)
    # Max reminders sent per customer (anti-spam).
    recovery_max_reminders: int = Field(default=1)
    # Interval between recovery worker sweeps (seconds). The worker is a separate
    # process (see app/cmd/recovery_worker.py); this controls how often it runs.
    recovery_sweep_interval_seconds: int = Field(default=3600)

    # --- Knowledge base (Phase A9) ---
    # Optional path to a custom knowledge-base.json. When empty, the loader uses
    # the default from docs/ai/knowledge-base.json (see app/knowledge/store.py).
    knowledge_base_path: str = Field(default="")

    # --- LLM (used from Phase A4) ---
    # Empty provider selects the built-in rule-based orchestrator, which needs
    # no external API key. Set to "openai" (OpenAI-compatible chat completions)
    # to use a real model with function calling.
    llm_provider: str = Field(default="")
    llm_model: str = Field(default="")
    llm_api_key: str = Field(default="")
    llm_max_tokens: int = Field(default=1024)
    llm_temperature: float = Field(default=0.3)
    # Base URL for OpenAI-compatible chat completions (override for Azure/other).
    llm_api_base_url: str = Field(default="https://api.openai.com/v1")
    # Safety cap on tool-call round trips per turn so a misbehaving model can't
    # loop forever calling tools.
    llm_max_tool_iterations: int = Field(default=4)
    # Optional overrides for the versioned prompt sources. When empty the loader
    # resolves them from the repo's docs/ai directory (see app/llm/prompt.py).
    llm_system_prompt_path: str = Field(default="")
    llm_sales_rules_path: str = Field(default="")


    # --- WhatsApp (used from Phase A2) ---
    wa_provider: str = Field(default="meta")
    wa_verify_token: str = Field(default="")
    wa_access_token: str = Field(default="")
    wa_phone_number_id: str = Field(default="")
    wa_api_base_url: str = Field(default="https://graph.facebook.com/v20.0")
    # App secret for verifying inbound webhook signatures (X-Hub-Signature-256).
    # Optional: if empty, signature verification is skipped (dev only).
    wa_app_secret: str = Field(default="")
    # How long a processed inbound message_id is remembered for idempotency.
    wa_inbound_dedup_ttl_seconds: int = Field(default=3600)
    # Retries when the provider send API fails (transport / 5xx).
    wa_send_max_retries: int = Field(default=3)
    # Whether the agent auto-replies with an echo (Phase A2 placeholder until
    # LLM orchestration lands in Phase A4).
    wa_echo_reply: bool = Field(default=True)

    @property
    def internal_base_url(self) -> str:
        """Base URL for the Go internal API surface (`/internal/v1`)."""
        return f"{self.backend_base_url.rstrip('/')}/internal/v1"

    @property
    def is_production(self) -> bool:
        return self.app_env == "production"


@lru_cache
def get_settings() -> Settings:
    """Return a cached Settings instance so env is parsed once per process."""
    return Settings()
