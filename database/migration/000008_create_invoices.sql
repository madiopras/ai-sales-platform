-- Phase 8: Payment (Xendit) — BR-016..BR-024, BR-042
--
-- invoices: satu invoice aktif per order (order_id UNIQUE). Menyimpan referensi
-- Xendit sehingga webhook idempotent bisa mengambil invoice via xendit_invoice_id
-- atau external_id (yang kita set = order_no) tanpa lookup by order_no dua kali.
--
-- payment_events: raw audit log dari webhook Xendit + hasil verifikasi.
-- xendit_event_id (idempotency key) dipakai untuk mencegah pemrosesan ganda
-- ketika Xendit retry callback.

CREATE TABLE IF NOT EXISTS invoices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL UNIQUE REFERENCES orders(id) ON DELETE RESTRICT,
    invoice_no VARCHAR(64) NOT NULL UNIQUE,
    amount DECIMAL(14, 2) NOT NULL,
    status VARCHAR(24) NOT NULL DEFAULT 'waiting_payment',
    payment_channel VARCHAR(64) NOT NULL DEFAULT '',

    -- Xendit references (populated after Xendit /v2/invoices call)
    xendit_invoice_id VARCHAR(64) NOT NULL DEFAULT '',
    xendit_external_id VARCHAR(128) NOT NULL DEFAULT '',
    xendit_payment_url TEXT NOT NULL DEFAULT '',
    xendit_payment_method VARCHAR(64) NOT NULL DEFAULT '',

    expired_at TIMESTAMPTZ NOT NULL,
    paid_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT invoices_status_allowed CHECK (status IN (
        'waiting_payment', 'paid', 'expired', 'failed'
    )),
    CONSTRAINT invoices_amount_positive CHECK (amount > 0)
);

CREATE INDEX IF NOT EXISTS idx_invoices_status ON invoices(status);
CREATE INDEX IF NOT EXISTS idx_invoices_expired_at ON invoices(expired_at);
CREATE INDEX IF NOT EXISTS idx_invoices_xendit_invoice_id ON invoices(xendit_invoice_id)
    WHERE xendit_invoice_id <> '';
CREATE INDEX IF NOT EXISTS idx_invoices_xendit_external_id ON invoices(xendit_external_id)
    WHERE xendit_external_id <> '';

-- Raw webhook log for audit + idempotency (BR-042)
CREATE TABLE IF NOT EXISTS payment_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    invoice_id UUID REFERENCES invoices(id) ON DELETE SET NULL,
    xendit_event_id VARCHAR(128) NOT NULL DEFAULT '',
    xendit_invoice_id VARCHAR(64) NOT NULL DEFAULT '',
    external_id VARCHAR(128) NOT NULL DEFAULT '',
    event_status VARCHAR(64) NOT NULL DEFAULT '',
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    signature_valid BOOLEAN NOT NULL DEFAULT FALSE,
    processed BOOLEAN NOT NULL DEFAULT FALSE,
    processing_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_payment_events_invoice_id ON payment_events(invoice_id);
CREATE INDEX IF NOT EXISTS idx_payment_events_created_at ON payment_events(created_at DESC);
-- Unique per (invoice, event id) so Xendit retries collapse to a single processed row.
CREATE UNIQUE INDEX IF NOT EXISTS idx_payment_events_dedup
    ON payment_events(xendit_invoice_id, xendit_event_id)
    WHERE xendit_event_id <> '';
