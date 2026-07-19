-- Phase 10: Promo/voucher applied at checkout (BR-037..BR-038)
--
-- The vouchers + audit_logs tables already exist (000007). This migration only
-- wires a redeemed voucher into an order: the code applied and the discount
-- amount snapshotted at checkout so order history is immutable even if the
-- voucher definition later changes.

ALTER TABLE orders
    ADD COLUMN IF NOT EXISTS voucher_code VARCHAR(64) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS discount DECIMAL(12, 2) NOT NULL DEFAULT 0;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'orders_discount_nonneg'
    ) THEN
        ALTER TABLE orders ADD CONSTRAINT orders_discount_nonneg CHECK (discount >= 0);
    END IF;
END$$;


-- Speeds up admin reporting on voucher usage.
CREATE INDEX IF NOT EXISTS idx_orders_voucher_code ON orders(voucher_code) WHERE voucher_code <> '';
