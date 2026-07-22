"""Event-driven WhatsApp notifications (Phase A7).

The Go backend is publish-only: it emits order lifecycle events
(``order.paid``, ``order.shipped``, ...) to RabbitMQ. This package *consumes*
them and pushes a WhatsApp message to the customer, keeping the AI out of the
transaction path (status truth stays in the backend, BR-024, BR-041).

Split so the domain logic is testable without a broker:

- :mod:`app.notifications.models` — the neutral event envelope + parsing.
- :mod:`app.notifications.templates` — event → customer-facing message copy.
- :mod:`app.notifications.idempotency` — Redis dedup so a redelivered event is
  never sent twice.
- :mod:`app.notifications.service` — resolve the customer's number, render the
  message, send it, and sync the conversation session.
- :mod:`app.notifications.consumer` — the RabbitMQ transport that feeds the
  service (thin; all logic lives in the service).
"""
