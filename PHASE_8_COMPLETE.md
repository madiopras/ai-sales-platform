# Phase 8 — Payment (Xendit) — Complete

Menggantikan seluruh rules bisnis pembayaran dari **Espay → Xendit**, lalu
mengimplementasikan domain `payment` sesuai roadmap (BR-016..BR-024, BR-035,
BR-036, BR-042). Referensi API: <https://docs.xendit.co/apidocs>.

## Perubahan ESPAY → Xendit

| File | Perubahan |
|------|-----------|
| `docs/ai/business-rules.md` | Section 7 "Payment (Espay)" → "Payment (Xendit)", BR-019/BR-022/BR-042, dan diagram end-to-end flow |
| `docs/be/go_backend_roadmap_e471211b.plan.md` | Todo `phase-8-espay` → `phase-8-xendit`, Phase 8 detail, mermaid diagram, tabel urutan |
| `apps/api/internal/config/config.go` | `XenditConfig` (bukan `ESPAY_*`) |
| `apps/api/.env.example` | Block `XENDIT_*` |

## Yang dibuat

### Database
- `database/migration/000008_create_invoices.sql`
  - `invoices` — `order_id` UNIQUE, `invoice_no`, `amount`, `status`
    (`waiting_payment|paid|expired|failed`), `expired_at`, `paid_at`, referensi
    Xendit (`xendit_invoice_id`, `xendit_external_id`, `xendit_payment_url`,
    `xendit_payment_method`).
  - `payment_events` — raw webhook log untuk audit + idempotency via unique
    index `(xendit_invoice_id, xendit_event_id)`.

### Domain `internal/payment/`
- `models.go` — sentinel errors, konstanta status, `Invoice`, `WebhookInput`, `PaymentEvent`.
- `repository.go` — interface persistence + `OrderSnapshot`, `XenditReferences`.
- `postgres_repository.go` — implementasi pgx. `MarkPaid` commit reservasi stok +
  set order `paid` dalam 1 transaksi (BR-035). `MarkExpired` release reservasi +
  set order `cancelled` (BR-036). `RecordEvent` idempotent (ON CONFLICT DO NOTHING).
- `service.go` — orkestrasi: buat invoice → panggil Xendit → attach refs.
  Idempotent (invoice aktif dipakai ulang). Verifikasi callback token dengan
  `subtle.ConstantTimeCompare` dan fail-closed. `ExpireInvoices` untuk worker.
- `handler.go` — `CreateInvoice`, `GetInvoice`, `GetInvoiceByOrder`, `XenditWebhook`
  (baca raw body + header `x-callback-token`).
- `invoiceno.go` — generator `INV-YYYYMMDD-XXXXXX`.
- `xendit/client.go` — HTTP client tipis untuk `POST /v2/invoices` (Basic auth
  secret key), plus `GetInvoice`.
- `service_test.go` — 7 test (reject non-pending order, create+attach refs,
  idempotency, mark failed on error, reject invalid token, mark paid, dedup event,
  expire releases stock).

### Wiring
- `container.go` — `PaymentService` + `xendit.Client`.
- `bootstrap/app.go` + `router.go` — handler + route baru.
- `cmd/worker/main.go` — CLI expiry job (`-loop`, `-interval`, `-batch`).

## Endpoint baru

Internal AI (`/internal/v1`):
- `POST /payments/invoices` — buat invoice untuk order (BR-016..BR-019)
- `GET  /payments/invoices/:id`
- `GET  /orders/:id/invoice`

Admin (`/api/v1`, JWT + role admin/manager):
- `GET /invoices/:id`
- `GET /orders/:id/invoice`

Webhook (token diverifikasi di service, BR-042):
- `POST /webhooks/xendit`

## Config baru (`XENDIT_*`)

```
XENDIT_SECRET_KEY=          # Basic auth ke Xendit API
XENDIT_CALLBACK_TOKEN=      # dibandingkan dengan header x-callback-token
XENDIT_BASE_URL=https://api.xendit.co
XENDIT_INVOICE_DURATION=24h
XENDIT_SUCCESS_REDIRECT_URL=
XENDIT_FAILURE_REDIRECT_URL=
XENDIT_HTTP_TIMEOUT=15s
```

## Verifikasi

```
cd apps/api
go build ./...          # BUILD_EXIT=0
go vet ./...            # VET_EXIT=0
go test ./...           # TEST_EXIT=0 (payment + semua paket lolos)
```

## Catatan keamanan

- Webhook `POST /webhooks/xendit` tidak dilindungi JWT (Xendit tidak mengirim
  bearer token); autentikasi dilakukan lewat header `x-callback-token` dengan
  constant-time compare, dan **fail-closed** bila `XENDIT_CALLBACK_TOKEN` kosong.
- Setiap callback (valid maupun invalid) dicatat di `payment_events` untuk audit.
- Sebelum production, `XENDIT_SECRET_KEY` + `XENDIT_CALLBACK_TOKEN` wajib diisi dari
  dashboard Xendit dan disimpan sebagai secret (bukan di repo).

## Langkah manual berikutnya

1. Jalankan migration `000008_create_invoices.sql`.
2. Isi kredensial `XENDIT_*` di environment.
3. Daftarkan URL webhook `https://<host>/webhooks/xendit` di dashboard Xendit.
4. Jadwalkan `worker -loop` (atau cron `worker`) untuk expiry invoice.
