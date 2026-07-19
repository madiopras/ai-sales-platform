-- Phase 9: Shipping (Biteship) — BR-010..BR-013, BR-028..BR-034, BR-043
--
-- shipments: satu shipment aktif per order (order_id UNIQUE). Menyimpan referensi
-- Biteship (biteship_order_id, tracking_no, waybill_id) sehingga webhook idempotent
-- bisa mengambil shipment via biteship_order_id atau tracking_no tanpa lookup ganda.
-- status menyimpan status Biteship mentah (allocated, picked, delivered, dst) —
-- tidak diberi CHECK ketat karena Biteship memiliki banyak status yang dapat
-- bertambah; normalisasi ke status order dilakukan di service.
--
-- shipment_events: raw audit log dari webhook Biteship + hasil verifikasi (BR-043).
-- event_key (idempotency key) mencegah pemrosesan ganda saat Biteship retry callback.

CREATE TABLE IF NOT EXISTS shipments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL UNIQUE REFERENCES orders(id) ON DELETE RESTRICT,

    courier_code VARCHAR(64) NOT NULL DEFAULT '',
    courier_service VARCHAR(64) NOT NULL DEFAULT '',
    courier_name VARCHAR(120) NOT NULL DEFAULT '',

    shipping_fee DECIMAL(12, 2) NOT NULL DEFAULT 0,
    status VARCHAR(48) NOT NULL DEFAULT 'pending',

    -- Biteship references (populated after Biteship POST /v1/orders call)
    biteship_order_id VARCHAR(64) NOT NULL DEFAULT '',
    tracking_no VARCHAR(120) NOT NULL DEFAULT '',
    waybill_id VARCHAR(120) NOT NULL DEFAULT '',
    label_url TEXT NOT NULL DEFAULT '',

    delivered_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT shipments_fee_nonneg CHECK (shipping_fee >= 0)
);

CREATE INDEX IF NOT EXISTS idx_shipments_status ON shipments(status);
CREATE INDEX IF NOT EXISTS idx_shipments_delivered_at ON shipments(delivered_at)
    WHERE delivered_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_shipments_biteship_order_id ON shipments(biteship_order_id)
    WHERE biteship_order_id <> '';
CREATE INDEX IF NOT EXISTS idx_shipments_tracking_no ON shipments(tracking_no)
    WHERE tracking_no <> '';

-- Raw webhook log for audit + idempotency (BR-043)
CREATE TABLE IF NOT EXISTS shipment_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    shipment_id UUID REFERENCES shipments(id) ON DELETE SET NULL,
    event_key VARCHAR(160) NOT NULL DEFAULT '',
    biteship_order_id VARCHAR(64) NOT NULL DEFAULT '',
    tracking_no VARCHAR(120) NOT NULL DEFAULT '',
    event VARCHAR(64) NOT NULL DEFAULT '',
    event_status VARCHAR(64) NOT NULL DEFAULT '',
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    signature_valid BOOLEAN NOT NULL DEFAULT FALSE,
    processed BOOLEAN NOT NULL DEFAULT FALSE,
    processing_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_shipment_events_shipment_id ON shipment_events(shipment_id);
CREATE INDEX IF NOT EXISTS idx_shipment_events_created_at ON shipment_events(created_at DESC);
-- Unique per event_key so Biteship retries collapse to a single processed row.
CREATE UNIQUE INDEX IF NOT EXISTS idx_shipment_events_dedup
    ON shipment_events(event_key)
    WHERE event_key <> '';
