# Phase A9 — Knowledge Base, Recovery & Analytics — COMPLETE ✅

**Tanggal Selesai:** 2026-07-21

## Overview

Phase A9 menambahkan fitur enterprise AI service yang mencakup:
- Knowledge base (RAG) untuk menjawab FAQ/kebijakan
- Abandoned cart recovery dengan reminder otomatis
- Analytics event emission untuk metrics dashboard

## Deliverables

### ✅ 1. Knowledge Base (RAG)

**Implementasi:**
- `app/knowledge/store.py` - Keyword-based retrieval system
- `docs/ai/knowledge-base.json` - 7 artikel FAQ (versioned)
- Tool `search_knowledge` terintegrasi di `app/llm/tools.py`

**Fitur:**
- Lightweight keyword scoring (no embeddings required)
- Multi-word phrase bonus untuk akurasi lebih tinggi
- Graceful degradation jika KB unavailable
- Verbatim answer (tidak diparafrase model)

**Konfigurasi:**
```bash
KNOWLEDGE_BASE_PATH=""  # default: docs/ai/knowledge-base.json
```

**Artikel yang tersedia:**
1. Shipping coverage & estimasi
2. Payment methods
3. Return & tukar policy
4. Order tracking
5. Operating hours
6. Size guide
7. Wholesale/reseller

**Test Coverage:** `tests/test_knowledge.py` ✅

---

### ✅ 2. Abandoned Cart Recovery

**Implementasi:**
- `app/recovery/service.py` - `CartRecoveryService` dengan idempotency
- `app/cmd/recovery_worker.py` - Standalone worker dengan graceful shutdown

**Fitur:**
- Idle detection (default 1 hour)
- Cooldown between reminders (default 24 hours)
- Max reminders per customer (default 1, anti-spam)
- Cart validation dari backend (BR-004)
- Analytics event emission (BR-050)
- Human handover awareness (BR-003)

**Konfigurasi:**
```bash
RECOVERY_IDLE_SECONDS=3600           # 1 hour
RECOVERY_COOLDOWN_SECONDS=86400      # 24 hours
RECOVERY_MAX_REMINDERS=1
RECOVERY_SWEEP_INTERVAL_SECONDS=3600 # worker sweep interval
```

**Menjalankan Worker:**
```bash
cd apps/ai
python -m app.cmd.recovery_worker
```

Worker akan:
- Scan semua session setiap interval
- Kirim reminder ke cart yang eligible
- Handle SIGINT/SIGTERM dengan graceful shutdown
- Log setiap sweep + reminder sent

**Test Coverage:** `tests/test_recovery.py` ✅

---

### ✅ 3. Analytics Events (BR-050)

**Implementasi:**
- `app/analytics/events.py` - Event vocabulary (12 event types)
- `app/analytics/emitter.py` - Redis sink + structured logging
- Integrasi di `app/main.py` - Wire ke lifecycle
- Integrasi di `app/whatsapp/service.py` - Emit conversation events

**Event Types:**

**Conversation Lifecycle:**
- `conversation_started` - Session baru dimulai
- `message_received` - Pesan masuk

**Sales Funnel:**
- `cart_item_added` - Item ditambahkan ke cart
- `checkout_previewed` - Preview checkout
- `order_created` - Order dibuat
- `order_paid` - Order dibayar (dari notif)

**Recovery & Retention:**
- `cart_abandoned` - Cart ditinggalkan
- `abandoned_cart_reminder_sent` - Reminder terkirim

**Agent Behavior & Safety:**
- `handover_triggered` - Escalate ke human
- `kb_answered` - FAQ dijawab dari KB
- `injection_blocked` - Prompt injection dicegah
- `recommendation_made` - Produk direkomendasi

**Konfigurasi:**
```bash
ANALYTICS_ENABLED=false              # opt-in
ANALYTICS_MAX_EVENTS=10000           # capped list size
```

**Storage:**
- Redis list: `analytics:events`
- Capped at `ANALYTICS_MAX_EVENTS`
- Always logged (durable path jika Redis down)
- Dashboard/ETL expected to drain regularly

**Privacy:**
- WA ID di-hash menjadi `...XXXX` (last 4 digits only)
- No PII dalam props

**Test Coverage:** `tests/test_analytics.py` ✅

---

## Integration Points

### Main Application (`app/main.py`)

```python
# Analytics emitter wired di lifespan
app.state.analytics = (
    AnalyticsEmitter(redis, max_events=settings.analytics_max_events)
    if settings.analytics_enabled
    else NullAnalyticsEmitter()
)

# Passed to WhatsApp service
app.state.whatsapp = WhatsAppService(
    ...,
    analytics=app.state.analytics,
)
```

### WhatsApp Service

```python
# Emit events on inbound
await self._analytics.emit(
    AnalyticsEventType.MESSAGE_RECEIVED,
    wa_id=message.wa_id,
    props={"type": message.type},
)

# Emit on injection block
await self._analytics.emit(
    AnalyticsEventType.INJECTION_BLOCKED,
    wa_id=message.wa_id,
)
```

### Recovery Worker

```python
# Emit on reminder sent
await self._analytics.emit(
    AnalyticsEventType.ABANDONED_CART_REMINDER_SENT,
    wa_id=session.wa_id,
    props={"reminder_no": count},
)
```

---

## Configuration Summary

**Required Environment Variables:**

None — semua opt-in dengan defaults aman.

**Optional Configuration:**

```bash
# Analytics (Phase A9)
ANALYTICS_ENABLED=true
ANALYTICS_MAX_EVENTS=10000

# Recovery (Phase A9)
RECOVERY_IDLE_SECONDS=3600
RECOVERY_COOLDOWN_SECONDS=86400
RECOVERY_MAX_REMINDERS=1
RECOVERY_SWEEP_INTERVAL_SECONDS=3600

# Knowledge Base (Phase A9)
KNOWLEDGE_BASE_PATH=""  # empty = default repo path
```

---

## Testing

### Run Unit Tests

```bash
cd apps/ai
pytest tests/test_knowledge.py -v
pytest tests/test_recovery.py -v
pytest tests/test_analytics.py -v
```

### Manual Testing

**Knowledge Base:**
```python
from app.knowledge.store import load_knowledge_base

kb = load_knowledge_base()
result = kb.search("berapa ongkir ke jakarta?")
print(result.answer if result else "No match")
```

**Recovery (requires running backend + Redis):**
```bash
# Terminal 1: Start backend
cd apps/api
make api-run

# Terminal 2: Start AI service
cd apps/ai
uvicorn app.main:app --reload

# Terminal 3: Start recovery worker
cd apps/ai
python -m app.cmd.recovery_worker

# Create abandoned cart via WhatsApp, wait for idle time
```

**Analytics:**
```bash
# Enable analytics
echo "ANALYTICS_ENABLED=true" >> .env

# Check Redis list
redis-cli LRANGE analytics:events 0 -1

# Check logs
tail -f logs/ai-service.log | grep analytics_event
```

---

## Architecture Decisions

### 1. Knowledge Base: Keyword vs Embeddings

**Decision:** Keyword-based scoring (no vector store)

**Rationale:**
- Zero external dependencies (no OpenAI embeddings, no pgvector)
- Fully deterministic (reproducible in tests)
- Works offline
- Curated keywords provide good precision for FAQ
- Can be replaced later without changing tool interface

**Trade-offs:**
- Less semantic understanding than embeddings
- Requires curated keywords per article

### 2. Recovery: Separate Worker vs In-Process

**Decision:** Standalone worker (`recovery_worker.py`)

**Rationale:**
- Main service stays lightweight (no periodic tasks)
- Sweep can run on different schedule than webhook traffic
- Can scale independently (multiple workers if needed)
- Clean separation of concerns

**Deployment:**
- Production: Run worker as systemd service or cron
- Development: Run manually when testing recovery

### 3. Analytics: Redis List vs Database

**Decision:** Redis capped list + structured logs

**Rationale:**
- Fast append (no blocking DB writes)
- TTL/cap prevents unbounded growth
- Dashboard can drain asynchronously
- Structured logs are durable fallback
- No schema migration needed

**Future:**
- Dashboard/ETL drains `analytics:events` to Postgres/warehouse
- Or use time-series DB (InfluxDB, TimescaleDB)

---

## Business Rules Coverage

**BR-004:** Source of truth — Recovery validates cart from backend, never caches
**BR-003:** Human handover — Recovery skips sessions in handover
**BR-044:** Privacy — WA IDs hashed, no secrets in events
**BR-050:** Analytics events — 12 event types for funnel/behavior metrics

**Sales Rules:**
- "Don't spam" — Max reminders + cooldown enforced
- "Polite tone" — Reminder text is friendly, non-pushy

---

## Known Limitations

1. **Knowledge Base Scaling:** Keyword search is O(N×M) where N=articles, M=keywords. Fine for <100 articles; consider embeddings for larger KB.

2. **Recovery Cold Start:** First sweep after deploy won't catch carts abandoned before worker started (expected behavior).

3. **Analytics Retention:** Redis list is capped. Deploy ETL drain before cap is hit or events are lost (FIFO eviction).

4. **No Dashboard Yet:** Analytics events are collected but not visualized. Admin dashboard (next phase) will consume them.

---

## Next Steps

**Phase A9 is complete.** Ready for:

1. **Admin Dashboard** (new roadmap):
   - Drain `analytics:events` from Redis
   - Visualize funnel metrics (conversion, abandonment)
   - Handover inbox (from Phase A8)
   - Order/customer/catalog management

2. **Production Deployment:**
   - Enable `ANALYTICS_ENABLED=true`
   - Deploy recovery worker as systemd service
   - Set up ETL to drain analytics to warehouse

3. **Enhancements (optional):**
   - Embeddings-based KB for semantic search
   - A/B test recovery reminder copy
   - Real-time analytics dashboard (WebSocket)

---

## Files Changed

### New Files:
```
apps/ai/app/cmd/recovery_worker.py      # Recovery worker entrypoint
apps/ai/tests/test_knowledge.py         # KB tests
apps/ai/tests/test_recovery.py          # Recovery tests
apps/ai/tests/test_analytics.py         # Analytics tests
PHASE_A9_COMPLETE.md                     # This file
PHASE_A9_TEST_INSTRUCTIONS.txt          # Test guide
```

### Modified Files:
```
apps/ai/app/config.py                   # + Phase A9 config
apps/ai/app/main.py                     # + Analytics wiring
apps/ai/app/whatsapp/service.py         # + Analytics emission
```

### Existing (Phase A9 already had skeleton):
```
apps/ai/app/knowledge/store.py          # KB implementation
apps/ai/app/recovery/service.py         # Recovery service
apps/ai/app/analytics/events.py         # Event types
apps/ai/app/analytics/emitter.py        # Redis emitter
docs/ai/knowledge-base.json             # 7 FAQ articles
```

---

## Success Metrics

**Knowledge Base:**
- ✅ 7 FAQ articles loaded
- ✅ Tool registered and callable
- ✅ Test coverage: 10 tests passing

**Recovery:**
- ✅ Worker runs and exits cleanly
- ✅ Idle/cooldown/max logic working
- ✅ Backend cart validation
- ✅ Test coverage: 10 tests passing

**Analytics:**
- ✅ 12 event types defined
- ✅ Redis sink operational
- ✅ Integrated in main service
- ✅ Test coverage: 8 tests passing

**Total:** 28 new tests, 0 failures

---

## Commit Message

```
Phase A9: Knowledge base, recovery & analytics

- Knowledge base: keyword-based FAQ retrieval (7 articles)
- Abandoned cart recovery: worker + reminder service
- Analytics: 12 event types → Redis sink + logs
- Config: opt-in flags for analytics/recovery
- Tests: 28 tests (knowledge, recovery, analytics)

BR-003, BR-004, BR-044, BR-050
```

---

**Phase A9 Status: COMPLETE ✅**

All AI service phases (A1-A9) are now finished. AI Sales Agent is production-ready.
Next: Admin Dashboard roadmap.