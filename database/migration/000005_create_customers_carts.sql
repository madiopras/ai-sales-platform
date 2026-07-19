-- Phase 6: Customers, addresses, carts, cart items (BR-006..BR-009)

-- Customers keyed by unique phone (WhatsApp identity)
CREATE TABLE customers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    phone VARCHAR(32) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_customers_phone ON customers(phone);

-- Customer shipping addresses; at most one default per customer (BR-009)
CREATE TABLE customer_addresses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id UUID NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
    label VARCHAR(64) NOT NULL DEFAULT 'utama',
    recipient_name VARCHAR(255) NOT NULL,
    phone VARCHAR(32) NOT NULL,
    address_line TEXT NOT NULL,
    city VARCHAR(120) NOT NULL,
    district VARCHAR(120) NOT NULL,
    postal_code VARCHAR(16) NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_customer_addresses_customer_id ON customer_addresses(customer_id);
-- Enforce a single default address per customer
CREATE UNIQUE INDEX idx_customer_addresses_default
    ON customer_addresses(customer_id)
    WHERE is_default = TRUE;

-- Carts: one open cart per customer at a time
CREATE TABLE carts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id UUID NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
    status VARCHAR(16) NOT NULL DEFAULT 'open',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT carts_status_allowed CHECK (status IN ('open', 'checked_out', 'abandoned'))
);

CREATE INDEX idx_carts_customer_id ON carts(customer_id);
-- At most one open cart per customer
CREATE UNIQUE INDEX idx_carts_one_open_per_customer
    ON carts(customer_id)
    WHERE status = 'open';

-- Cart items with unit price snapshot; unique per (cart, variant) for upsert
CREATE TABLE cart_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cart_id UUID NOT NULL REFERENCES carts(id) ON DELETE CASCADE,
    variant_id UUID NOT NULL REFERENCES product_variants(id) ON DELETE RESTRICT,
    qty INTEGER NOT NULL,
    unit_price DECIMAL(10, 2) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT cart_items_qty_positive CHECK (qty > 0),
    CONSTRAINT cart_items_unique_variant UNIQUE (cart_id, variant_id)
);

CREATE INDEX idx_cart_items_cart_id ON cart_items(cart_id);
CREATE INDEX idx_cart_items_variant_id ON cart_items(variant_id);
