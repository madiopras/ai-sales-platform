"""Analytics event emission (Phase A9, BR-050).

The AI is where the funnel actually happens (conversation → cart → checkout →
paid), so it is the natural place to emit the business metrics the admin
dashboard will later chart: conversation volume, conversions, cart abandonment,
handover rate, response signals, etc.

This package keeps emission decoupled and non-blocking:

- :mod:`app.analytics.events` defines the small, stable event vocabulary and the
  serializable :class:`AnalyticsEvent` envelope.
- :mod:`app.analytics.emitter` writes events to a capped Redis list (an append
  sink the dashboard/ETL can drain) and always mirrors them to the structured
  log. Emission is strictly best-effort — it never raises into the conversation
  path and never carries a raw phone number (PII is reduced to a coarse hash).

Analytics is fire-and-forget telemetry, never a source of truth: it observes the
flow, it does not drive it.
"""

from app.analytics.emitter import AnalyticsEmitter, NullAnalyticsEmitter
from app.analytics.events import AnalyticsEvent, AnalyticsEventType

__all__ = [
    "AnalyticsEmitter",
    "AnalyticsEvent",
    "AnalyticsEventType",
    "NullAnalyticsEmitter",
]
