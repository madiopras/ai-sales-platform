-- Phase 7: Orders, order items, inventory reservation (BR-014..BR-015, BR-035..BR-036)

-- Orders: created at checkout confirm with an address snapshot and stock reserved
CREATE TABLE orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_no VARCHAR(32) NOT NULL UNIQUE,
    customer_id UUID NOT NULL REFERENCES customers(id) ON DELETE RESTRICT,
    status VARCHAR(24) NOT NULL DEFAULT 'pending_payment',

    -- Shipping address snapshot (decoupled from customer_addresses)
    recipient_name VARCHAR(255) NOT NULL,
    phone VARCHAR(32) NOT NULL,
    address_line TEXT NOT NULL,
    city VARCHAR(120) NOT NULL,
    district VARCHAR(120) NOT NULL,
    postal_code VARCHAR(16) NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',

    -- Courier fields (populated by shipping phase; stubbed for now)
    courier_code VARCHAR(64) NOT NULL DEFAULT '',
    courier_service VARCHAR(64) NOT NULL DEFAULT '',
    tracking_no VARCHAR(64) NOT NULL DEFAULT '',

    subtotal DECIMAL(12, 2) NOT NULL DEFAULT 0,
    shipping_fee DECIMAL(12, 2) NOT NULL DEFAULT 0,
    total DECIMAL(12, 2) NOT NULL DEFAULT 0,

    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT orders_status_allowed CHECK (status IN (
        'pending_payment', 'paid', 'processing', 'shipped', 'delivered', 'completed', 'cancelled'
    )),
    CONSTRAINT orders_amounts_nonneg CHECK (subtotal >= 0 AND shipping_fee >= 0 AND total >= 0)
);

CREATE INDEX idx_orders_customer_id ON orders(customer_id);
CREATE INDEX idx_orders_status ON orders(status);
CREATE INDEX idx_orders_created_at ON orders(created_at DESC);

-- Order line items with price + name snapshots (independent of catalog mutations)
CREATE TABLE order_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    variant_id UUID NOT NULL REFERENCES product_variants(id) ON DELETE RESTRICT,
    sku VARCHAR(255) NOT NULL DEFAULT '',
    product_name VARCHAR(255) NOT NULL DEFAULT '',
    variant_name VARCHAR(255) NOT NULL DEFAULT '',
    qty INTEGER NOT NULL,
    unit_price DECIMAL(12, 2) NOT NULL,
    subtotal DECIMAL(12, 2) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT order_items_qty_positive CHECK (qty > 0),
    CONSTRAINT order_items_unique_variant UNIQUE (order_id, variant_id)
);

CREATE INDEX idx_order_items_order_id ON order_items(order_id);
CREATE INDEX idx_order_items_variant_id ON order_items(variant_id);
