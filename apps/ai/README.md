# apps/ai — AI Sales Agent Service

FastAPI service that acts as the WhatsApp AI sales agent. It is a **consumer**
of the Go backend's internal API (`/internal/v1`), not a source of truth: all
product, cart, order, invoice and shipping data comes from the backend.

Phases delivered so far:

- **Phase A1 — Service foundation**: skeleton app, config, structured logging,
  error envelope, backend + Redis clients, health endpoints.
- **Phase A2 — WhatsApp integration**: provider-agnostic WhatsApp abstraction
  (Meta Cloud API implementation), webhook verify + receive, inbound
  idempotency (Redis), and an outbound sender with retry.
- **Phase A3 — Conversation & session state**: Redis-backed sessions (sliding
  idle timeout), a rolling message-history window, and a deterministic
  conversation state machine (greeting → discovery → … → payment, plus human
  handover). Replies are state-aware placeholders until LLM orchestration lands
  in Phase A4.
- **Phase A4 — LLM orchestration & tool calling**: a versioned system prompt
  (loaded from `docs/ai/*.md`), keyword intent detection, a tool registry that
  wraps the backend product APIs (shaping out inactive/out-of-stock items,
  BR-004), and an orchestrator that runs a bounded prompt → tool-call → reply
  loop with guardrails (tool results are the only source of truth; failures
  degrade to a safe apology; handover sessions never call the model). Pluggable
  chat models: an offline **rule-based** default (no API key) and an
  **OpenAI-compatible** function-calling provider, selected via `LLM_PROVIDER`.
- **Phase A5 — Sales flow tools**: the agent can now drive the catalog → cart →
  customer-data flow through tools: `get_cart`, `add_cart_item`,
  `update_cart_item`, `remove_cart_item`, `save_customer`,
  `get_default_address`, and `create_address` (BR-006, BR-007, BR-008, BR-009).
  Customer identity is resolved from the session's WhatsApp id (phone), never
  from model arguments, and resolved backend references (customer/cart/address
  ids) are cached on the session. Backend business rejections (e.g.
  `INSUFFICIENT_STOCK`) surface as actionable results the model explains;
  cart figures come straight from the backend (BR-004).
- **Phase A6 — Shipping, checkout & payment**: the transaction path via chat —
  `get_shipping_rates`, `validate_voucher`, `preview_checkout`,
  `confirm_checkout`, `create_invoice`, `get_invoice`, `get_order`, and
  `get_shipment` (BR-010..BR-024, BR-032). Guardrails: the AI can only price a
  courier from a rate it actually fetched this session (the authoritative fee is
  cached on the session and matched at checkout, never invented — BR-004);
  preview/confirm pass the backend's own totals through unchanged (BR-015);
  `confirm_checkout` creates a real order and caches the order id, which
  `create_invoice` then uses to return a payment link + status. The AI never
  asserts an order is paid — the paid status arrives via the verified webhook in
  Phase A7. Order/shipment lookups default to the session's order and treat
  "not shipped yet" as an empty, non-error result (BR-032).
- **Phase A7 — Notifications (event-driven WA)**: a RabbitMQ consumer for the
  order lifecycle events the Go backend publishes (`order.paid`, `order.shipped`,
  `order.delivered`, `order.completed`, `order.cancelled` — BR-024, BR-027,
  BR-031, BR-041). Events carry only the order id, so the recipient is resolved
  from the backend (order → customer → phone; the AI never invents a number).
  Each event is deduplicated in Redis so a redelivery never double-sends, and
  the customer copy states only what the event asserts — the verified `order.paid`
  event (from the Xendit webhook in Go) is what confirms payment, closing the
  A6 loop where the AI must never claim an order is paid. Transient failures
  (backend down, send failed) requeue for retry; poison/duplicate/nothing-to-say
  events are acked. On `order.paid` the customer's live session is synced to
  paid so an in-flight chat stays consistent (BR-024). The consumer is opt-in
  (`NOTIFICATIONS_ENABLED`) and imports `aio-pika` lazily, so tests and offline
  runs need no broker.

- **Phase A8 — Human handover & safety**: two deterministic guardrails that run
  before the model on every turn. A prompt-injection guard refuses attempts to
  extract the system prompt / rules / secrets or to "forget" instructions
  (EN + ID wordings), returning a friendly refusal without ever calling the LLM
  so no crafted input reaches its tools (BR-044). Handover detection escalates a
  conversation to a human on the BR-003 triggers — explicit request, complaint /
  refund / return, price negotiation, payment or shipping trouble, bulk order
  (plus a runtime low-confidence path). On escalation the session flips to
  handover, the reason is stashed, and an admin is notified via a Redis-backed
  queue + open-handover flag (best-effort — a Redis blip never blocks the
  customer-facing handover). The bot then sends one warm acknowledgement keyed
  to the reason and goes quiet so it doesn't talk over the human; an admin can
  resume the session back to bot control.

Later phases add the knowledge base, cart recovery, and analytics. See
[`docs/ai/ai_service_roadmap_9f2c1a7d.plan.md`](../../docs/ai/ai_service_roadmap_9f2c1a7d.plan.md).




## Layout

```
app/
  main.py            # FastAPI app + lifespan (wires clients)
  config.py          # env settings (pydantic-settings)
  health.py          # /health/live, /health/ready
  middleware.py      # request id + structured access log
  clients/
    backend.py       # httpx client → /internal/v1 + service token
    redis.py         # async redis connection
  platform/
    logging.py       # structured JSON logging
    response.py      # response envelope (matches Go backend)
    errors.py        # exception handlers → envelope
  whatsapp/
    provider.py      # WhatsAppProvider protocol + verification error
    meta.py          # Meta Cloud API implementation
    factory.py       # build a provider from WA_PROVIDER
    models.py        # normalized Inbound/Outbound message types
    idempotency.py   # Redis inbound message_id dedup
    outbound.py      # async sender with retry
    service.py       # orchestrates dedup → conversation → reply
    router.py        # /webhooks/whatsapp (GET verify, POST receive)
  conversation/
    models.py        # session/context/state shapes (serializable)
    store.py         # Redis session persistence (sliding TTL)
    manager.py       # state machine: load → transition → persist
  llm/
    types.py         # ChatMessage / ToolCall / ToolResult shapes
    prompt.py        # versioned system prompt loader (docs/ai/*.md)
    intent.py        # keyword intent classifier
    tools.py         # tool registry + product tool wrappers (BR-004 shaping)
    orchestrator.py  # prompt + history + bounded tool loop + guardrails
    factory.py       # build orchestrator (model + tools) from settings
    providers/
      base.py        # ChatModel protocol
      rule_based.py  # offline default model (no API key)
      openai.py      # OpenAI-compatible function-calling model
  notifications/
    models.py        # domain-event envelope + defensive parsing
    templates.py     # event → customer WhatsApp copy (states only facts)
    idempotency.py   # Redis event dedup (fails closed to avoid dupes)
    service.py       # parse → render → dedup → resolve phone → send → sync
    consumer.py      # RabbitMQ transport (lazy aio-pika; ack/nack)
  safety/
    injection.py     # prompt-injection / extraction guard (refuse in-place)
    handover.py      # handover detection + reason + admin notifier (BR-003)
tests/               # unit tests (backend + provider + redis mocked)



```


## Setup

From the repo root, the Makefile has AI targets:

```bash
make ai-setup   # create .venv, install deps, copy .env.example → .env
make ai-run     # run with autoreload on :8090
make ai-test    # run pytest
make ai-lint    # run ruff
make ai-clean   # remove .venv and caches
```

Or manually:

```bash
cd apps/ai
python -m venv .venv && source .venv/bin/activate
pip install -e ".[dev]"      # or: pip install -r requirements.txt
cp .env.example .env         # set BACKEND_SERVICE_TOKEN to match apps/api
uvicorn app.main:app --reload --port 8090
```

`BACKEND_SERVICE_TOKEN` must equal `INTERNAL_SERVICE_TOKEN` in `apps/api/.env`;
it is sent as the `X-Service-Token` header on every internal API call.

Once running:

- `GET /health/live` → `{"data": {"status": "ok"}}`
- `GET /health/ready` → pings the Go backend + Redis; 503 if either is down.

## Conventions

- Config via env only; never hardcode secrets and never log tokens (BR-044).
- All backend calls go through `clients/backend.py` (one place for auth, retry,
  error mapping).
- No domain logic here: prices, stock, shipping and totals always come from the
  backend (BR-004).
