-- Create product_variants table
CREATE TABLE product_variants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    sku VARCHAR(255) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    price DECIMAL(10, 2) NOT NULL,
    stock_on_hand INTEGER NOT NULL DEFAULT 0,
    stock_reserved INTEGER NOT NULL DEFAULT 0,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT variants_stock_on_hand_nonneg CHECK (stock_on_hand >= 0),
    CONSTRAINT variants_stock_reserved_nonneg CHECK (stock_reserved >= 0),
    CONSTRAINT variants_stock_reserved_lte_on_hand CHECK (stock_reserved <= stock_on_hand)
);

-- Create indexes
CREATE INDEX idx_variants_product_id ON product_variants(product_id);
CREATE INDEX idx_variants_sku ON product_variants(sku);
CREATE INDEX idx_variants_is_active ON product_variants(is_active);
-- Index available stock (on_hand - reserved) for AI availability queries (BR-004)
CREATE INDEX idx_variants_available ON product_variants((stock_on_hand - stock_reserved)) WHERE is_active = true;

