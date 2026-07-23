# Enhancement Technical Implementation Plan — Tenant Configuration & Multi-Tenant Architecture

**Status:** Draft — Ready for Review  
**Date:** 2026-07-23  
**Related:**
- [ENHANCEMENT_BUSINESS_RULES_TENANT_CONFIG.md](./ENHANCEMENT_BUSINESS_RULES_TENANT_CONFIG.md) — Business rules (EBR-001..EBR-120)
- [ENHANCEMENT_UI_DESIGN_TENANT_CONFIG.md](./ENHANCEMENT_UI_DESIGN_TENANT_CONFIG.md) — UI design specification

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Architecture Overview](#2-architecture-overview)
3. [Database Design — New Tables & Migrations](#3-database-design--new-tables--migrations)
4. [Backend Implementation — Go API](#4-backend-implementation--go-api)
5. [AI Service Changes — Python](#5-ai-service-changes--python)
6. [Admin Frontend Implementation — Next.js](#6-admin-frontend-implementation--nextjs)
7. [Payment Provider Recommendation](#7-payment-provider-recommendation)
8. [Migration Strategy — From Single-Tenant to Multi-Tenant](#8-migration-strategy--from-single-tenant-to-multi-tenant)
9. [Security Considerations](#9-security-considerations)
10. [Implementation Phases & Timeline](#10-implementation-phases--timeline)
11. [Testing Strategy](#11-testing-strategy)
12. [Deployment Checklist](#12-deployment-checklist)

---

## 1. Executive Summary

### Current State

The AI Sales Platform currently operates as a **single-tenant** system:
- One hardcoded WhatsApp number
- One hardcoded LLM provider (OpenAI/Anthropic via env vars)
- One hardcoded Biteship API key
- One hardcoded Xendit payment configuration
- All accounts share the same product catalog, orders, customers

### Target State

Transform into a **multi-tenant SaaS platform** where:
- Each registered account can configure their own business profile
- Each account connects exactly 1 WhatsApp channel (Meta Cloud API or third-party)
- Each account chooses exactly 1 LLM provider (Anthropic or OpenAI) with their own API key
- Each account inputs their own Biteship API key for shipping
- Each account configures their own payment provider
- All data (products, orders, customers, conversations) is isolated per account
- AI agent operates within the tenant's business context

### Key Technical Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Tenant isolation strategy | `account_id` column on all tenant-scoped tables | Simple, proven, no complex row-level security needed |
| Credential storage | AES-256-GCM encrypted in `tenant_credentials` table | Industry standard; keys never in plaintext at rest |
| WhatsApp multi-provider | Abstraction interface + provider-specific adapters | Allows adding new providers without changing core logic |
| LLM multi-provider | Existing `app/llm/providers.py` extended with per-tenant config | Already supports OpenAI + Anthropic; just needs dynamic config source |
| Payment recommendation | **Midtrans** (Indonesian market) | See Section 7 for detailed analysis |
| AI tenant context | `account_id` in Redis session → propagated to all backend calls | Minimal changes to existing AI orchestration |

---

## 2. Architecture Overview

### 2.1 High-Level Multi-Tenant Flow

```
┌─────────────────────────────────────────────────────────────────────┐
│                        ADMIN DASHBOARD (Next.js)                      │
│  /settings/business  /settings/channel  /settings/llm               │
│  /settings/shipping  /settings/payment                               │
│  → All requests include JWT (which contains account_id)              │
└──────────────────────────────┬──────────────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────────┐
│                     BACKEND API (Go — Gin)                            │
│                                                                       │
│  ┌──────────────────────────────────────────────────────────────┐   │
│  │  NEW: Tenant Configuration Endpoints                          │   │
│  │  /api/v1/settings/business     CRUD Business Profile          │   │
│  │  /api/v1/settings/channel      CRUD WhatsApp Channel          │   │
│  │  /api/v1/settings/llm          CRUD LLM Configuration         │   │
│  │  /api/v1/settings/shipping     CRUD Biteship Configuration    │   │
│  │  /api/v1/settings/payment      CRUD Payment Configuration     │   │
│  └──────────────────────────────────────────────────────────────┘   │
│                                                                       │
│  ┌──────────────────────────────────────────────────────────────┐   │
│  │  MODIFIED: All existing endpoints                             │   │
│  │  - Add account_id filter to all queries                       │   │
│  │  - Validate tenant owns resource before mutation              │   │
│  │  - Internal endpoints accept account_id parameter             │   │
│  └──────────────────────────────────────────────────────────────┘   │
│                                                                       │
│  ┌──────────────────────────────────────────────────────────────┐   │
│  │  NEW: Tenant Middleware                                       │   │
│  │  - Extract account_id from JWT                                │   │
│  │  - Inject into request context                                │   │
│  │  - Validate tenant is active (not suspended)                  │   │
│  └──────────────────────────────────────────────────────────────┘   │
│                                                                       │
│  ┌──────────────────────────────────────────────────────────────┐   │
│  │  NEW: Credential Encryption Service                           │   │
│  │  - Encrypt before storing                                     │   │
│  │  - Decrypt when provider needs credential                     │   │
│  │  - Key rotation support                                       │   │
│  └──────────────────────────────────────────────────────────────┘   │
└──────────────────────────────┬──────────────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────────┐
│                     AI SERVICE (Python — FastAPI)                      │
│                                                                       │
│  ┌──────────────────────────────────────────────────────────────┐   │
│  │  MODIFIED: WhatsApp Service                                   │   │
│  │  - Route inbound messages to correct tenant                   │   │
│  │  - Use tenant's WhatsApp credential                           │   │
│  │  - Support Meta Cloud API + third-party adapters              │   │
│  └──────────────────────────────────────────────────────────────┘   │
│                                                                       │
│  ┌──────────────────────────────────────────────────────────────┐   │
│  │  MODIFIED: LLM Orchestrator                                   │   │
│  │  - Load tenant's LLM provider + API key                       │   │
│  │  - Load tenant's business profile for system prompt           │   │
│  │  - Load tenant's knowledge base                               │   │
│  └──────────────────────────────────────────────────────────────┘   │
│                                                                       │
│  ┌──────────────────────────────────────────────────────────────┐   │
│  │  MODIFIED: Backend Client                                     │   │
│  │  - Pass account_id in all internal API calls                  │   │
│  │  - Use tenant's shipping/payment credentials                  │   │
│  └──────────────────────────────────────────────────────────────┘   │
│                                                                       │
│  ┌──────────────────────────────────────────────────────────────┐   │
│  │  NEW: Tenant Resolver                                         │   │
│  │  - Map inbound phone number → account_id                      │   │
│  │  - Cache mapping in Redis                                     │   │
│  └──────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────┘
```

### 2.2 Tenant Context Propagation

```
Inbound WhatsApp Message
  │
  ├─ WhatsApp Service receives webhook
  ├─ Extract from_number (customer WA ID)
  ├─ Lookup Redis: wa_account:{from_number} → account_id
  │   (populated when admin registers WhatsApp channel)
  ├─ Load tenant config from DB (or Redis cache)
  │   ├─ LLM provider + API key
  │   ├─ Business profile
  │   ├─ Knowledge base
  │   └─ Biteship/Xendit credentials
  ├─ Set tenant context on Redis session
  └─ All subsequent backend calls include account_id header
```

---

## 3. Database Design — New Tables & Migrations

### 3.1 Migration Files Required

```
database/migration/
├── 000011_create_business_profiles.sql
├── 000012_create_whatsapp_channels.sql
├── 000013_create_tenant_credentials.sql
├── 000014_create_llm_configurations.sql
├── 000015_create_shipping_configurations.sql
├── 000016_create_payment_configurations.sql
├── 000017_add_account_id_to_existing_tables.sql
└── 000018_create_tenant_audit_log.sql
```

### 3.2 Table: `business_profiles`

```sql
-- 000011_create_business_profiles.sql
CREATE TABLE business_profiles (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id      UUID NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    
    -- Business identity
    business_name       VARCHAR(255) NOT NULL,
    business_category   VARCHAR(100) NOT NULL,
    business_description TEXT,
    
    -- Contact
    contact_phone       VARCHAR(30) NOT NULL,
    contact_email       VARCHAR(255),
    
    -- Origin address (for shipping rate calculation)
    origin_address      TEXT NOT NULL,
    origin_city         VARCHAR(100) NOT NULL,
    origin_district     VARCHAR(100) NOT NULL,
    origin_postal_code  VARCHAR(10) NOT NULL,
    origin_province     VARCHAR(100),
    origin_country      VARCHAR(100) DEFAULT 'Indonesia',
    
    -- Operational
    timezone            VARCHAR(50) NOT NULL DEFAULT 'Asia/Jakarta',
    default_language    VARCHAR(10) NOT NULL DEFAULT 'id',
    operating_hours     JSONB NOT NULL DEFAULT '{
        "monday":    {"open": "08:00", "close": "17:00"},
        "tuesday":   {"open": "08:00", "close": "17:00"},
        "wednesday": {"open": "08:00", "close": "17:00"},
        "thursday":  {"open": "08:00", "close": "17:00"},
        "friday":    {"open": "08:00", "close": "17:00"},
        "saturday":  {"open": "08:00", "close": "14:00"},
        "sunday":    {"open": null,   "close": null}
    }',
    
    -- Policies (editable by user)
    shipping_policy     TEXT,
    return_policy       TEXT,
    payment_policy      TEXT,
    
    -- AI behavior
    tone_of_voice       VARCHAR(50) DEFAULT 'friendly_professional',
    -- friendly_professional | casual_friendly | formal_business | humorous
    
    greeting_template   TEXT DEFAULT 'Halo! Selamat datang di {business_name}. Ada yang bisa kami bantu? 😊',
    
    -- Status
    status              VARCHAR(30) NOT NULL DEFAULT 'incomplete'
                        CHECK (status IN ('incomplete', 'active', 'suspended', 'inactive')),
    -- incomplete: required fields not filled
    -- active: ready for AI operations
    -- suspended: admin/platform suspended
    -- inactive: user manually deactivated
    
    suspended_reason    TEXT,
    
    -- Timestamps
    created_at          TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at          TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_business_profiles_account ON business_profiles(account_id);
CREATE INDEX idx_business_profiles_status ON business_profiles(status);
```

### 3.3 Table: `whatsapp_channels`

```sql
-- 000012_create_whatsapp_channels.sql
CREATE TABLE whatsapp_channels (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    
    -- Provider selection
    provider        VARCHAR(50) NOT NULL
                    CHECK (provider IN ('meta_cloud_api', 'third_party_gateway')),
    -- meta_cloud_api: WhatsApp Business Cloud API langsung
    -- third_party_gateway: Provider pihak ketiga (WATI, Qontak, dll)
    
    third_party_name VARCHAR(100),  -- Nama provider jika third_party_gateway
    
    -- Channel identity
    phone_number    VARCHAR(30) NOT NULL,       -- Nomor WA bisnis
    phone_number_id VARCHAR(100),               -- Meta: phone_number_id
    waba_id         VARCHAR(100),               -- Meta: WhatsApp Business Account ID
    business_account_id VARCHAR(100),           -- Meta: business account ID
    
    -- Webhook configuration
    webhook_verify_token VARCHAR(255),          -- Meta: verify token
    webhook_url     VARCHAR(500),               -- URL webhook terdaftar
    
    -- Status lifecycle
    status          VARCHAR(30) NOT NULL DEFAULT 'draft'
                    CHECK (status IN (
                        'draft',
                        'pending_verification',
                        'active',
                        'inactive',
                        'failed',
                        'revoked'
                    )),
    
    -- Verification
    verified_at     TIMESTAMP,
    verification_error TEXT,
    
    -- Rate limiting & quota
    daily_message_limit INTEGER DEFAULT 1000,
    messages_sent_today INTEGER DEFAULT 0,
    last_reset_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    -- Metadata
    notes           TEXT,
    
    -- Timestamps
    created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    -- Constraint: only one active channel per account
    CONSTRAINT uq_one_active_per_account UNIQUE (account_id, status)
    -- Note: This partial unique constraint ensures only one row can have
    -- status='active' per account_id. Implement via partial unique index:
    -- CREATE UNIQUE INDEX idx_one_active_channel ON whatsapp_channels(account_id) WHERE status = 'active';
);

CREATE INDEX idx_whatsapp_channels_account ON whatsapp_channels(account_id);
CREATE INDEX idx_whatsapp_channels_phone ON whatsapp_channels(phone_number);
CREATE UNIQUE INDEX idx_one_active_channel_per_account ON whatsapp_channels(account_id) WHERE status = 'active';
```

### 3.4 Table: `tenant_credentials` (Encrypted Storage)

```sql
-- 000013_create_tenant_credentials.sql
CREATE TABLE tenant_credentials (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    
    -- What credential is this for
    credential_type VARCHAR(50) NOT NULL
                    CHECK (credential_type IN (
                        'whatsapp_meta',
                        'whatsapp_third_party',
                        'llm_openai',
                        'llm_anthropic',
                        'biteship',
                        'payment_midtrans',
                        'payment_xendit',
                        'payment_stripe'
                    )),
    
    -- Encrypted credential data (AES-256-GCM)
    -- Plaintext structure depends on credential_type:
    --
    -- whatsapp_meta:
    --   {"access_token": "...", "phone_number_id": "...", "waba_id": "...",
    --    "webhook_verify_token": "...", "app_id": "...", "app_secret": "..."}
    --
    -- whatsapp_third_party:
    --   {"api_key": "...", "api_secret": "...", "sender_id": "...",
    --    "base_url": "...", "instance_id": "..."}
    --
    -- llm_openai:
    --   {"api_key": "sk-..."}
    --
    -- llm_anthropic:
    --   {"api_key": "sk-ant-..."}
    --
    -- biteship:
    --   {"api_key": "..."}
    --
    -- payment_*:
    --   {"server_key": "...", "client_key": "...", "merchant_id": "..."}
    
    encrypted_data  BYTEA NOT NULL,
    
    -- Encryption metadata
    encryption_key_id   VARCHAR(100) NOT NULL,  -- Which master key encrypted this
    encryption_algorithm VARCHAR(50) NOT NULL DEFAULT 'AES-256-GCM',
    encrypted_at        TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    -- Validation
    last_validated_at   TIMESTAMP,
    validation_status   VARCHAR(30) DEFAULT 'not_validated'
                        CHECK (validation_status IN ('not_validated', 'valid', 'invalid', 'expired')),
    validation_error    TEXT,
    
    -- Status
    is_active       BOOLEAN DEFAULT true,
    
    -- For display only (masked)
    display_hint    VARCHAR(100),  -- e.g., "sk-...abc123" (masked)
    
    -- Timestamps
    created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_tenant_credentials_account ON tenant_credentials(account_id);
CREATE INDEX idx_tenant_credentials_type ON tenant_credentials(account_id, credential_type);
CREATE UNIQUE INDEX idx_one_active_credential_per_type ON tenant_credentials(account_id, credential_type) WHERE is_active = true;
```

### 3.5 Table: `llm_configurations`

```sql
-- 000014_create_llm_configurations.sql
CREATE TABLE llm_configurations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    
    -- Provider selection
    provider        VARCHAR(50) NOT NULL
                    CHECK (provider IN ('openai', 'anthropic')),
    
    -- Model selection
    model           VARCHAR(100) NOT NULL,
    -- Examples: gpt-4o, gpt-4o-mini, claude-sonnet-4-20250514, claude-opus-4-20250514
    
    -- System prompt customization (optional override)
    custom_system_prompt TEXT,
    -- If NULL, use default system prompt + business profile
    
    -- LLM parameters
    temperature     DECIMAL(3,2) DEFAULT 0.7,
    max_tokens      INTEGER DEFAULT 4096,
    
    -- Status
    is_active       BOOLEAN DEFAULT true,
    
    -- Timestamps
    created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT uq_one_active_llm_per_account UNIQUE (account_id, is_active)
    -- Implement via partial unique index:
    -- CREATE UNIQUE INDEX idx_one_active_llm ON llm_configurations(account_id) WHERE is_active = true;
);

CREATE INDEX idx_llm_config_account ON llm_configurations(account_id);
CREATE UNIQUE INDEX idx_one_active_llm_per_account ON llm_configurations(account_id) WHERE is_active = true;
```

### 3.6 Table: `shipping_configurations`

```sql
-- 000015_create_shipping_configurations.sql
CREATE TABLE shipping_configurations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    
    -- Provider (currently only Biteship, extensible)
    provider        VARCHAR(50) NOT NULL DEFAULT 'biteship'
                    CHECK (provider IN ('biteship')),
    
    -- Origin defaults (can override business profile)
    origin_postal_code  VARCHAR(10),
    origin_address      TEXT,
    origin_city         VARCHAR(100),
    origin_district     VARCHAR(100),
    
    -- Courier preferences
    default_couriers    VARCHAR(200) DEFAULT 'jne,jnt,sicepat',
    -- Comma-separated courier codes
    
    -- Auto-complete settings
    auto_complete_after_hours INTEGER DEFAULT 72,
    
    -- Status
    is_active       BOOLEAN DEFAULT true,
    
    -- Timestamps
    created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT uq_one_active_shipping_per_account UNIQUE (account_id, is_active)
);

CREATE INDEX idx_shipping_config_account ON shipping_configurations(account_id);
CREATE UNIQUE INDEX idx_one_active_shipping_per_account ON shipping_configurations(account_id) WHERE is_active = true;
```

### 3.7 Table: `payment_configurations`

```sql
-- 000016_create_payment_configurations.sql
CREATE TABLE payment_configurations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    
    -- Provider selection
    provider        VARCHAR(50) NOT NULL
                    CHECK (provider IN ('midtrans', 'xendit', 'stripe')),
    
    -- Payment methods enabled
    enabled_methods JSONB NOT NULL DEFAULT '[]',
    -- Example: ["bank_transfer", "ewallet", "qris", "credit_card"]
    -- Depends on provider capabilities
    
    -- Invoice settings
    invoice_duration_hours  INTEGER DEFAULT 24,
    success_redirect_url    VARCHAR(500),
    failure_redirect_url    VARCHAR(500),
    
    -- Status
    is_active       BOOLEAN DEFAULT true,
    
    -- Timestamps
    created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT uq_one_active_payment_per_account UNIQUE (account_id, is_active)
);

CREATE INDEX idx_payment_config_account ON payment_configurations(account_id);
CREATE UNIQUE INDEX idx_one_active_payment_per_account ON payment_configurations(account_id) WHERE is_active = true;
```

### 3.8 Migration: Add `account_id` to Existing Tables

```sql
-- 000017_add_account_id_to_existing_tables.sql

-- Products & Catalog
ALTER TABLE categories ADD COLUMN account_id UUID REFERENCES users(id);
ALTER TABLE products ADD COLUMN account_id UUID REFERENCES users(id);
ALTER TABLE product_variants ADD COLUMN account_id UUID REFERENCES users(id);

-- Customers & Cart
ALTER TABLE customers ADD COLUMN account_id UUID REFERENCES users(id);
ALTER TABLE customer_addresses ADD COLUMN account_id UUID REFERENCES users(id);
ALTER TABLE carts ADD COLUMN account_id UUID REFERENCES users(id);
ALTER TABLE cart_items ADD COLUMN account_id UUID REFERENCES users(id);

-- Orders
ALTER TABLE orders ADD COLUMN account_id UUID REFERENCES users(id);
ALTER TABLE order_items ADD COLUMN account_id UUID REFERENCES users(id);

-- Payment
ALTER TABLE invoices ADD COLUMN account_id UUID REFERENCES users(id);
ALTER TABLE payment_events ADD COLUMN account_id UUID REFERENCES users(id);

-- Shipping
ALTER TABLE shipments ADD COLUMN account_id UUID REFERENCES users(id);
ALTER TABLE shipment_events ADD COLUMN account_id UUID REFERENCES users(id);

-- Promo & Audit
ALTER TABLE vouchers ADD COLUMN account_id UUID REFERENCES users(id);
ALTER TABLE audit_logs ADD COLUMN account_id UUID REFERENCES users(id);

-- Create indexes for tenant isolation
CREATE INDEX idx_categories_account ON categories(account_id);
CREATE INDEX idx_products_account ON products(account_id);
CREATE INDEX idx_product_variants_account ON product_variants(account_id);
CREATE INDEX idx_customers_account ON customers(account_id);
CREATE INDEX idx_customer_addresses_account ON customer_addresses(account_id);
CREATE INDEX idx_carts_account ON carts(account_id);
CREATE INDEX idx_cart_items_account ON cart_items(account_id);
CREATE INDEX idx_orders_account ON orders(account_id);
CREATE INDEX idx_order_items_account ON order_items(account_id);
CREATE INDEX idx_invoices_account ON invoices(account_id);
CREATE INDEX idx_payment_events_account ON payment_events(account_id);
CREATE INDEX idx_shipments_account ON shipments(account_id);
CREATE INDEX idx_shipment_events_account ON shipment_events(account_id);
CREATE INDEX idx_vouchers_account ON vouchers(account_id);
CREATE INDEX idx_audit_logs_account ON audit_logs(account_id);

-- Composite indexes for common queries (tenant + filter)
CREATE INDEX idx_products_account_status ON products(account_id, status);
CREATE INDEX idx_orders_account_status ON orders(account_id, status);
CREATE INDEX idx_customers_account_phone ON customers(account_id, phone);
```

### 3.9 Table: `tenant_audit_log` (Enhanced)

```sql
-- 000018_create_tenant_audit_log.sql
-- Extends existing audit_logs with tenant-specific fields
-- If audit_logs already exists, ALTER instead of CREATE

ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS account_id UUID REFERENCES users(id);
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS resource_type VARCHAR(50);
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS before_value JSONB;
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS after_value JSONB;
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS ip_address VARCHAR(50);
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS user_agent TEXT;

CREATE INDEX IF NOT EXISTS idx_audit_logs_account ON audit_logs(account_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_account_action ON audit_logs(account_id, action);
```

---

## 4. Backend Implementation — Go API

### 4.1 New Packages Structure

```
apps/api/internal/
├── tenant/                          # NEW: Tenant configuration domain
│   ├── models.go                    # Data structs for all config tables
│   ├── repository.go                # Interface
│   ├── postgres_repository.go       # Postgres implementation
│   ├── service.go                   # Business logic + validation
│   ├── handler.go                   # HTTP handlers for settings endpoints
│   └── service_test.go              # Unit tests
│
├── credential/                      # NEW: Credential encryption service
│   ├── service.go                   # Encrypt/Decrypt with AES-256-GCM
│   ├── key_manager.go               # Master key management + rotation
│   └── service_test.go
│
├── middleware/
│   ├── tenant.go                    # NEW: Tenant context middleware
│   │                                # Extract account_id from JWT
│   │                                # Validate tenant active status
│   └── tenant_test.go
│
├── container/container.go           # MODIFIED: Wire new services
├── http/router/router.go            # MODIFIED: Add settings routes
├── bootstrap/app.go                 # MODIFIED: Init tenant services
│
├── catalog/                         # MODIFIED: Add account_id filter
├── customer/                        # MODIFIED: Add account_id filter
├── order/                           # MODIFIED: Add account_id filter
├── payment/                         # MODIFIED: Add account_id filter
├── shipping/                        # MODIFIED: Add account_id filter
├── promo/                           # MODIFIED: Add account_id filter
└── audit/                           # MODIFIED: Add account_id filter
```

### 4.2 Tenant Middleware

```go
// apps/api/internal/http/middleware/tenant.go

package middleware

import (
    "net/http"
    
    "github.com/gin-gonic/gin"
    "ai-sales-platform/apps/api/internal/platform/response"
    "ai-sales-platform/apps/api/internal/auth"
)

// TenantContext extracts account_id from JWT and injects into request context.
// Must be applied AFTER Authenticate middleware.
func TenantContext() gin.HandlerFunc {
    return func(c *gin.Context) {
        claims, exists := c.Get("auth_claims")
        if !exists {
            response.Fail(c, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
            c.Abort()
            return
        }
        
        userClaims, ok := claims.(*auth.Claims)
        if !ok {
            response.Fail(c, http.StatusUnauthorized, "UNAUTHORIZED", "invalid claims")
            c.Abort()
            return
        }
        
        // Inject account_id into context
        c.Set("account_id", userClaims.AccountID)
        c.Next()
    }
}

// RequireActiveTenant validates that the tenant's business profile is active.
// Must be applied AFTER TenantContext middleware.
// Only blocks mutation endpoints; read endpoints can still work.
func RequireActiveTenant(tenantService *tenant.Service) gin.HandlerFunc {
    return func(c *gin.Context) {
        accountID, _ := c.Get("account_id")
        
        profile, err := tenantService.GetBusinessProfile(c.Request.Context(), accountID.(string))
        if err != nil {
            // If profile doesn't exist yet, allow (user is setting up)
            c.Next()
            return
        }
        
        if profile.Status == "suspended" {
            response.Fail(c, http.StatusForbidden, "TENANT_SUSPENDED", 
                "account is suspended: "+profile.SuspendedReason)
            c.Abort()
            return
        }
        
        c.Next()
    }
}
```

### 4.3 Credential Encryption Service

```go
// apps/api/internal/credential/service.go

package credential

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "encoding/base64"
    "encoding/json"
    "errors"
    "io"
)

type Service struct {
    masterKey    []byte  // 32 bytes for AES-256
    keyID        string
}

func NewService(masterKeyBase64 string) (*Service, error) {
    key, err := base64.StdEncoding.DecodeString(masterKeyBase64)
    if err != nil {
        return nil, errors.New("invalid master key encoding")
    }
    if len(key) != 32 {
        return nil, errors.New("master key must be 32 bytes (AES-256)")
    }
    return &Service{
        masterKey: key,
        keyID:     "mk-2026-07-v1",
    }, nil
}

// Encrypt serializes data to JSON, then encrypts with AES-256-GCM.
// Returns base64-encoded ciphertext (includes nonce prepended).
func (s *Service) Encrypt(data interface{}) ([]byte, error) {
    plaintext, err := json.Marshal(data)
    if err != nil {
        return nil, err
    }
    
    block, err := aes.NewCipher(s.masterKey)
    if err != nil {
        return nil, err
    }
    
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, err
    }
    
    nonce := make([]byte, gcm.NonceSize())
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
        return nil, err
    }
    
    // GCM seals: nonce || ciphertext || tag
    ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
    return ciphertext, nil
}

// Decrypt reverses Encrypt: decrypts AES-256-GCM, then unmarshals JSON.
func (s *Service) Decrypt(ciphertext []byte, target interface{}) error {
    block, err := aes.NewCipher(s.masterKey)
    if err != nil {
        return err
    }
    
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return err
    }
    
    nonceSize := gcm.NonceSize()
    if len(ciphertext) < nonceSize {
        return errors.New("ciphertext too short")
    }
    
    nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
    plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
    if err != nil {
        return err
    }
    
    return json.Unmarshal(plaintext, target)
}
```

### 4.4 Tenant Configuration Service

```go
// apps/api/internal/tenant/service.go (partial — key methods)

package tenant

type Service struct {
    repo        Repository
    credential  *credential.Service
    // WhatsApp client for validation
    // Biteship client for validation
    // LLM client factory for validation
}

// --- Business Profile ---

func (s *Service) CreateOrUpdateBusinessProfile(ctx context.Context, accountID string, input BusinessProfileInput) (*BusinessProfile, error) {
    // Validate required fields (EBR-011)
    if err := validateBusinessProfile(input); err != nil {
        return nil, err
    }
    
    existing, err := s.repo.GetBusinessProfile(ctx, accountID)
    if err == ErrNotFound {
        // Create new
        profile := input.ToModel(accountID)
        profile.Status = s.determineProfileStatus(profile)
        return s.repo.CreateBusinessProfile(ctx, profile)
    }
    
    // Update existing — audit the change (EBR-014)
    updated := input.ToModel(accountID)
    updated.ID = existing.ID
    updated.Status = s.determineProfileStatus(updated)
    
    return s.repo.UpdateBusinessProfile(ctx, updated)
}

func (s *Service) determineProfileStatus(profile *BusinessProfile) string {
    required := []string{
        profile.BusinessName, profile.BusinessCategory, profile.ContactPhone,
        profile.OriginAddress, profile.OriginCity, profile.OriginDistrict,
        profile.OriginPostalCode, profile.Timezone, profile.DefaultLanguage,
    }
    for _, f := range required {
        if f == "" {
            return "incomplete"
        }
    }
    // If was previously active, stay active; otherwise incomplete → active
    if profile.Status == "active" || profile.Status == "suspended" {
        return profile.Status
    }
    return "active"
}

// --- WhatsApp Channel ---

func (s *Service) CreateWhatsAppChannel(ctx context.Context, accountID string, input WhatsAppChannelInput) (*WhatsAppChannel, error) {
    // EBR-004: Check no other active channel
    active, _ := s.repo.GetActiveWhatsAppChannel(ctx, accountID)
    if active != nil {
        return nil, ErrChannelAlreadyActive
    }
    
    channel := input.ToModel(accountID)
    channel.Status = "draft"
    
    // EBR-023: Validate credential before saving
    if err := s.validateWhatsAppCredential(ctx, input); err != nil {
        channel.Status = "failed"
        channel.VerificationError = err.Error()
    } else {
        channel.Status = "pending_verification"
    }
    
    return s.repo.CreateWhatsAppChannel(ctx, channel)
}

func (s *Service) ActivateWhatsAppChannel(ctx context.Context, accountID, channelID string) error {
    channel, err := s.repo.GetWhatsAppChannel(ctx, channelID)
    if err != nil {
        return err
    }
    if channel.AccountID != accountID {
        return ErrNotOwnedByTenant
    }
    
    // EBR-022: Provider switch flow
    // 1. Validate credential again
    if err := s.validateWhatsAppCredential(ctx, channel); err != nil {
        channel.Status = "failed"
        channel.VerificationError = err.Error()
        s.repo.UpdateWhatsAppChannel(ctx, channel)
        return err
    }
    
    // 2. Deactivate old active channel
    oldActive, _ := s.repo.GetActiveWhatsAppChannel(ctx, accountID)
    if oldActive != nil && oldActive.ID != channelID {
        oldActive.Status = "inactive"
        s.repo.UpdateWhatsAppChannel(ctx, oldActive)
    }
    
    // 3. Activate new channel
    channel.Status = "active"
    channel.VerifiedAt = timePtr(time.Now())
    return s.repo.UpdateWhatsAppChannel(ctx, channel)
}

// --- LLM Configuration ---

func (s *Service) SetLLMConfiguration(ctx context.Context, accountID string, input LLMConfigInput) (*LLMConfiguration, error) {
    // EBR-005: Only one active LLM provider
    active, _ := s.repo.GetActiveLLMConfig(ctx, accountID)
    if active != nil && active.Provider != input.Provider {
        // Switching provider — deactivate old
        active.IsActive = false
        s.repo.UpdateLLMConfig(ctx, active)
    }
    
    // Store credential encrypted (EBR-040)
    credData := map[string]string{"api_key": input.APIKey}
    encrypted, err := s.credential.Encrypt(credData)
    if err != nil {
        return nil, err
    }
    
    // Save credential
    credType := "llm_" + input.Provider
    _, err = s.repo.UpsertCredential(ctx, accountID, credType, encrypted, s.credential.KeyID())
    if err != nil {
        return nil, err
    }
    
    // Save LLM config
    config := input.ToModel(accountID)
    config.IsActive = true
    return s.repo.UpsertLLMConfig(ctx, config)
}

// --- Shipping Configuration ---

func (s *Service) SetShippingConfiguration(ctx context.Context, accountID string, input ShippingConfigInput) (*ShippingConfiguration, error) {
    // Store Biteship API key encrypted
    credData := map[string]string{"api_key": input.APIKey}
    encrypted, err := s.credential.Encrypt(credData)
    if err != nil {
        return nil, err
    }
    
    _, err = s.repo.UpsertCredential(ctx, accountID, "biteship", encrypted, s.credential.KeyID())
    if err != nil {
        return nil, err
    }
    
    config := input.ToModel(accountID)
    config.IsActive = true
    return s.repo.UpsertShippingConfig(ctx, config)
}

// --- Payment Configuration ---

func (s *Service) SetPaymentConfiguration(ctx context.Context, accountID string, input PaymentConfigInput) (*PaymentConfiguration, error) {
    // Store payment credentials encrypted
    credData := map[string]string{
        "server_key": input.ServerKey,
        "client_key": input.ClientKey,
    }
    if input.MerchantID != "" {
        credData["merchant_id"] = input.MerchantID
    }
    
    encrypted, err := s.credential.Encrypt(credData)
    if err != nil {
        return nil, err
    }
    
    credType := "payment_" + input.Provider
    _, err = s.repo.UpsertCredential(ctx, accountID, credType, encrypted, s.credential.KeyID())
    if err != nil {
        return nil, err
    }
    
    config := input.ToModel(accountID)
    config.IsActive = true
    return s.repo.UpsertPaymentConfig(ctx, config)
}
```

### 4.5 Modified: Existing Domain Services (Tenant Isolation)

Every existing repository method that queries or mutates data must add `account_id` filtering:

```go
// Example: catalog/postgres_repository.go — BEFORE
func (r *PostgresRepository) ListProducts(ctx context.Context, filter ProductFilter) ([]Product, int, error) {
    query := `SELECT id, category_id, name, slug, description, status, created_at, updated_at 
              FROM products WHERE status = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`
    // ...
}

// Example: catalog/postgres_repository.go — AFTER
func (r *PostgresRepository) ListProducts(ctx context.Context, accountID string, filter ProductFilter) ([]Product, int, error) {
    query := `SELECT id, category_id, name, slug, description, status, created_at, updated_at 
              FROM products WHERE account_id = $1 AND status = $2 ORDER BY created_at DESC LIMIT $3 OFFSET $4`
    // ...
}
```

**All modified repository methods must accept `accountID string` as first parameter.**

### 4.6 New API Endpoints

```
# Tenant Configuration (NEW)
GET    /api/v1/settings/business          Get business profile
PUT    /api/v1/settings/business          Create or update business profile

GET    /api/v1/settings/channel           List WhatsApp channels
POST   /api/v1/settings/channel           Create WhatsApp channel
GET    /api/v1/settings/channel/:id       Get WhatsApp channel detail
PUT    /api/v1/settings/channel/:id       Update WhatsApp channel
POST   /api/v1/settings/channel/:id/activate   Activate channel
POST   /api/v1/settings/channel/:id/deactivate Deactivate channel
POST   /api/v1/settings/channel/:id/validate   Validate credential

GET    /api/v1/settings/llm               Get LLM configuration
PUT    /api/v1/settings/llm               Set LLM configuration
POST   /api/v1/settings/llm/validate      Validate LLM API key

GET    /api/v1/settings/shipping          Get shipping configuration
PUT    /api/v1/settings/shipping          Set shipping configuration
POST   /api/v1/settings/shipping/validate Validate Biteship API key

GET    /api/v1/settings/payment           Get payment configuration
PUT    /api/v1/settings/payment           Set payment configuration
POST   /api/v1/settings/payment/validate  Validate payment credential

# Internal API — Tenant Context (MODIFIED)
# All existing /internal/v1 endpoints now require X-Account-ID header
# or account_id in request body
```

### 4.7 Router Changes

```go
// apps/api/internal/http/router/router.go

func New(deps Deps) *gin.Engine {
    // ... existing setup ...
    
    // Apply tenant middleware to all /api/v1 routes
    apiV1.Use(middleware.TenantContext())
    
    // Settings routes (require auth + tenant context)
    settings := apiV1.Group("/settings")
    {
        settings.GET("/business", deps.TenantHandler.GetBusinessProfile)
        settings.PUT("/business", deps.TenantHandler.UpdateBusinessProfile)
        
        settings.GET("/channel", deps.TenantHandler.ListChannels)
        settings.POST("/channel", deps.TenantHandler.CreateChannel)
        settings.GET("/channel/:id", deps.TenantHandler.GetChannel)
        settings.PUT("/channel/:id", deps.TenantHandler.UpdateChannel)
        settings.POST("/channel/:id/activate", deps.TenantHandler.ActivateChannel)
        settings.POST("/channel/:id/deactivate", deps.TenantHandler.DeactivateChannel)
        settings.POST("/channel/:id/validate", deps.TenantHandler.ValidateChannelCredential)
        
        settings.GET("/llm", deps.TenantHandler.GetLLMConfig)
        settings.PUT("/llm", deps.TenantHandler.UpdateLLMConfig)
        settings.POST("/llm/validate", deps.TenantHandler.ValidateLLMCredential)
        
        settings.GET("/shipping", deps.TenantHandler.GetShippingConfig)
        settings.PUT("/shipping", deps.TenantHandler.UpdateShippingConfig)
        settings.POST("/shipping/validate", deps.TenantHandler.ValidateShippingCredential)
        
        settings.GET("/payment", deps.TenantHandler.GetPaymentConfig)
        settings.PUT("/payment", deps.TenantHandler.UpdatePaymentConfig)
        settings.POST("/payment/validate", deps.TenantHandler.ValidatePaymentCredential)
    }
    
    // Internal routes — add tenant context
    internalV1.Use(middleware.ServiceTokenAuth(deps.Config.InternalServiceToken))
    // Note: account_id will come from request body or X-Account-ID header
    // since internal calls don't have JWT
    
    // ... rest of routes ...
}
```

### 4.8 Configuration Changes

```go
// apps/api/internal/config/config.go — additions

type Config struct {
    // ... existing fields ...
    
    // Tenant encryption
    CredentialMasterKey  string   `env:"CREDENTIAL_MASTER_KEY" envDefault:""`
    // Base64-encoded 32-byte AES-256 key
    // Generate: openssl rand -base64 32
    
    // Tenant defaults (used when tenant hasn't configured their own)
    DefaultLLMProvider       string `env:"DEFAULT_LLM_PROVIDER" envDefault:"openai"`
    DefaultLLMModel          string `env:"DEFAULT_LLM_MODEL" envDefault:"gpt-4o-mini"`
    DefaultShippingProvider  string `env:"DEFAULT_SHIPPING_PROVIDER" envDefault:"biteship"`
    DefaultPaymentProvider   string `env:"DEFAULT_PAYMENT_PROVIDER" envDefault:"midtrans"`
}
```

---

## 5. AI Service Changes — Python

### 5.1 New/Modified Files

```
apps/ai/app/
├── config.py                        # MODIFIED: Remove hardcoded provider keys
│                                    # Add tenant config cache TTL
│
├── main.py                          # MODIFIED: Wire tenant resolver
│
├── tenant/                          # NEW: Tenant context resolution
│   ├── __init__.py
│   ├── resolver.py                  # Map phone number → account_id
│   └── config_loader.py             # Load tenant config from backend
│
├── whatsapp/
│   ├── service.py                   # MODIFIED: Multi-tenant inbound routing
│   ├── adapters/                    # NEW: Provider abstraction
│   │   ├── __init__.py
│   │   ├── base.py                  # Abstract adapter interface
│   │   ├── meta_cloud.py            # Meta WhatsApp Cloud API adapter
│   │   └── third_party.py           # Generic third-party adapter
│   └── webhook.py                   # MODIFIED: Route webhook to correct tenant
│
├── llm/
│   ├── orchestrator.py              # MODIFIED: Load tenant LLM config
│   ├── providers.py                 # MODIFIED: Accept per-request API key
│   └── prompt.py                    # MODIFIED: Inject tenant business profile
│
├── clients/
│   ├── backend_client.py            # MODIFIED: Pass account_id in all calls
│   └── biteship_client.py           # MODIFIED: Use tenant's API key
│
├── knowledge/
│   └── store.py                     # MODIFIED: Support per-tenant KB files
│
├── conversation/
│   └── manager.py                   # MODIFIED: Store account_id in session
│
└── recovery/
    └── service.py                   # MODIFIED: Per-tenant recovery sweeps
```

### 5.2 Tenant Resolver

```python
# apps/ai/app/tenant/resolver.py

import logging
from typing import Optional
from redis.asyncio import Redis

logger = logging.getLogger(__name__)

class TenantResolver:
    """Maps inbound WhatsApp phone numbers to account IDs.
    
    Mapping is populated when admin activates a WhatsApp channel:
    - Backend registers: wa_account:{phone_number} → account_id in Redis
    - Resolver looks up this mapping on every inbound message.
    """
    
    KEY_PREFIX = "wa_account"
    TTL_SECONDS = 86400 * 7  # 7 days
    
    def __init__(self, redis: Redis):
        self._redis = redis
    
    async def resolve(self, phone_number: str) -> Optional[str]:
        """Return account_id for a WhatsApp phone number, or None."""
        key = f"{self.KEY_PREFIX}:{phone_number}"
        account_id = await self._redis.get(key)
        if account_id:
            return account_id.decode("utf-8") if isinstance(account_id, bytes) else account_id
        
        logger.warning("no_tenant_mapping", extra={"phone": phone_number})
        return None
    
    async def register(self, phone_number: str, account_id: str) -> None:
        """Register a phone number → account_id mapping."""
        key = f"{self.KEY_PREFIX}:{phone_number}"
        await self._redis.set(key, account_id, ex=self.TTL_SECONDS)
        logger.info("tenant_registered", extra={
            "phone": phone_number,
            "account_id": account_id,
        })
    
    async def unregister(self, phone_number: str) -> None:
        """Remove a phone number mapping (when channel deactivated)."""
        key = f"{self.KEY_PREFIX}:{phone_number}"
        await self._redis.delete(key)
        logger.info("tenant_unregistered", extra={"phone": phone_number})
```

### 5.3 Tenant Config Loader

```python
# apps/ai/app/tenant/config_loader.py

import logging
from dataclasses import dataclass
from typing import Optional
from redis.asyncio import Redis
from app.clients.backend_client import BackendClient

logger = logging.getLogger(__name__)

@dataclass
class TenantConfig:
    account_id: str
    business_profile: dict
    llm_provider: str       # "openai" | "anthropic"
    llm_api_key: str
    llm_model: str
    llm_temperature: float
    llm_max_tokens: int
    custom_system_prompt: Optional[str]
    whatsapp_provider: str  # "meta_cloud_api" | "third_party_gateway"
    whatsapp_credentials: dict
    biteship_api_key: Optional[str]
    payment_provider: Optional[str]
    payment_credentials: Optional[dict]
    knowledge_base_path: Optional[str]

class TenantConfigLoader:
    """Loads tenant configuration from backend API, with Redis caching."""
    
    CACHE_TTL = 300  # 5 minutes
    
    def __init__(self, redis: Redis, backend: BackendClient):
        self._redis = redis
        self._backend = backend
    
    async def load(self, account_id: str) -> Optional[TenantConfig]:
        """Load full tenant config, from cache or backend."""
        
        # Try cache first
        cache_key = f"tenant_config:{account_id}"
        cached = await self._redis.get(cache_key)
        if cached:
            import json
            data = json.loads(cached)
            return TenantConfig(**data)
        
        # Load from backend
        try:
            config = await self._backend.get_tenant_config(account_id)
        except Exception as e:
            logger.error("tenant_config_load_failed", extra={
                "account_id": account_id,
                "error": str(e),
            })
            return None
        
        # Cache it
        import json
        await self._redis.setex(
            cache_key,
            self.CACHE_TTL,
            json.dumps(config.__dict__),
        )
        
        return config
    
    async def invalidate_cache(self, account_id: str) -> None:
        """Clear cache when tenant updates configuration."""
        cache_key = f"tenant_config:{account_id}"
        await self._redis.delete(cache_key)
```

### 5.4 WhatsApp Provider Adapter Pattern

```python
# apps/ai/app/whatsapp/adapters/base.py

from abc import ABC, abstractmethod
from dataclasses import dataclass

@dataclass
class InboundMessage:
    from_number: str       # Customer's WA number
    to_number: str         # Business WA number
    message_type: str      # "text" | "image" | "document" | etc.
    text: Optional[str]
    media_id: Optional[str]
    timestamp: str
    raw_payload: dict      # Original webhook payload

@dataclass
class OutboundMessage:
    to_number: str
    text: str
    preview_url: bool = True

class WhatsAppAdapter(ABC):
    """Abstract adapter for WhatsApp providers.
    
    Each provider (Meta Cloud API, WATI, Qontak, etc.) implements this interface.
    """
    
    @abstractmethod
    async def verify_webhook(self, query_params: dict) -> str:
        """Verify webhook challenge. Returns challenge token or raises."""
        ...
    
    @abstractmethod
    async def parse_inbound(self, raw_payload: dict) -> InboundMessage:
        """Parse inbound webhook payload into normalized InboundMessage."""
        ...
    
    @abstractmethod
    async def send_message(self, message: OutboundMessage) -> dict:
        """Send a text message. Returns provider response."""
        ...
    
    @abstractmethod
    async def validate_credential(self, credentials: dict) -> bool:
        """Validate that credentials work (test message or health check)."""
        ...
    
    @abstractmethod
    async def get_health(self) -> dict:
        """Return provider health status."""
        ...
```

```python
# apps/ai/app/whatsapp/adapters/meta_cloud.py

import httpx
from app.whatsapp.adapters.base import WhatsAppAdapter, InboundMessage, OutboundMessage

class MetaCloudAdapter(WhatsAppAdapter):
    """WhatsApp Business Cloud API adapter."""
    
    BASE_URL = "https://graph.facebook.com/v21.0"
    
    def __init__(self, credentials: dict):
        self.access_token = credentials["access_token"]
        self.phone_number_id = credentials["phone_number_id"]
        self.waba_id = credentials.get("waba_id")
        self.verify_token = credentials.get("webhook_verify_token")
    
    async def verify_webhook(self, query_params: dict) -> str:
        mode = query_params.get("hub.mode")
        token = query_params.get("hub.verify_token")
        challenge = query_params.get("hub.challenge")
        
        if mode == "subscribe" and token == self.verify_token:
            return challenge
        raise ValueError("webhook verification failed")
    
    async def parse_inbound(self, raw_payload: dict) -> InboundMessage:
        # Extract from Meta webhook format
        entry = raw_payload["entry"][0]
        change = entry["changes"][0]
        message = change["value"]["messages"][0]
        
        return InboundMessage(
            from_number=message["from"],
            to_number=change["value"]["metadata"]["display_phone_number"],
            message_type=message["type"],
            text=message.get("text", {}).get("body"),
            media_id=message.get("image", {}).get("id"),
            timestamp=message["timestamp"],
            raw_payload=raw_payload,
        )
    
    async def send_message(self, message: OutboundMessage) -> dict:
        url = f"{self.BASE_URL}/{self.phone_number_id}/messages"
        headers = {"Authorization": f"Bearer {self.access_token}"}
        body = {
            "messaging_product": "whatsapp",
            "to": message.to_number,
            "type": "text",
            "text": {"body": message.text, "preview_url": message.preview_url},
        }
        
        async with httpx.AsyncClient() as client:
            resp = await client.post(url, json=body, headers=headers)
            resp.raise_for_status()
            return resp.json()
    
    async def validate_credential(self, credentials: dict) -> bool:
        """Test credential by fetching phone number info."""
        url = f"{self.BASE_URL}/{credentials['phone_number_id']}"
        headers = {"Authorization": f"Bearer {credentials['access_token']}"}
        params = {"fields": "display_phone_number,verified_name"}
        
        async with httpx.AsyncClient() as client:
            resp = await client.get(url, headers=headers, params=params)
            return resp.status_code == 200
    
    async def get_health(self) -> dict:
        return {"provider": "meta_cloud_api", "phone_number_id": self.phone_number_id}
```

### 5.5 Modified: WhatsApp Service (Multi-Tenant)

```python
# apps/ai/app/whatsapp/service.py — key changes

class WhatsAppService:
    def __init__(
        self,
        tenant_resolver: TenantResolver,
        config_loader: TenantConfigLoader,
        # ... other deps ...
    ):
        self._tenant_resolver = tenant_resolver
        self._config_loader = config_loader
        # Adapter cache: account_id → WhatsAppAdapter
        self._adapters: dict[str, WhatsAppAdapter] = {}
    
    async def handle_inbound_webhook(self, raw_payload: dict) -> dict:
        """Route inbound webhook to correct tenant."""
        
        # Step 1: Determine which business number received the message
        # For Meta Cloud API, extract from payload
        to_number = self._extract_to_number(raw_payload)
        
        # Step 2: Resolve tenant
        account_id = await self._tenant_resolver.resolve(to_number)
        if not account_id:
            logger.warning("unrecognized_tenant", extra={"to_number": to_number})
            return {"status": "ignored", "reason": "no_tenant_for_number"}
        
        # Step 3: Load tenant config
        config = await self._config_loader.load(account_id)
        if not config:
            logger.error("tenant_config_unavailable", extra={"account_id": account_id})
            return {"status": "error", "reason": "config_unavailable"}
        
        # Step 4: Get or create adapter for this tenant
        adapter = await self._get_adapter(account_id, config)
        
        # Step 5: Parse inbound message
        message = await adapter.parse_inbound(raw_payload)
        
        # Step 6: Process with tenant context
        return await self._process_message(account_id, config, message)
    
    async def _get_adapter(self, account_id: str, config: TenantConfig) -> WhatsAppAdapter:
        if account_id in self._adapters:
            return self._adapters[account_id]
        
        if config.whatsapp_provider == "meta_cloud_api":
            adapter = MetaCloudAdapter(config.whatsapp_credentials)
        elif config.whatsapp_provider == "third_party_gateway":
            adapter = ThirdPartyAdapter(config.whatsapp_credentials)
        else:
            raise ValueError(f"unknown provider: {config.whatsapp_provider}")
        
        self._adapters[account_id] = adapter
        return adapter
    
    async def _process_message(
        self, account_id: str, config: TenantConfig, message: InboundMessage
    ) -> dict:
        """Process message within tenant context."""
        # Set tenant context on Redis session
        session = await self._conversation_manager.get_or_create_session(
            wa_id=message.from_number,
            account_id=account_id,  # NEW: tenant-scoped session
        )
        
        # LLM orchestration uses tenant's provider + API key
        response = await self._llm_orchestrator.process(
            session=session,
            user_message=message.text,
            tenant_config=config,  # NEW: pass tenant config
        )
        
        # Send response via tenant's WhatsApp adapter
        adapter = self._adapters[account_id]
        await adapter.send_message(OutboundMessage(
            to_number=message.from_number,
            text=response,
        ))
        
        return {"status": "ok", "account_id": account_id}
```

### 5.6 Modified: LLM Orchestrator (Tenant Context)

```python
# apps/ai/app/llm/orchestrator.py — key changes

class LLMOrchestrator:
    async def process(
        self,
        session: ConversationSession,
        user_message: str,
        tenant_config: TenantConfig,  # NEW parameter
    ) -> str:
        # Build system prompt with tenant business profile
        system_prompt = self._build_tenant_prompt(tenant_config)
        
        # Select LLM provider based on tenant config
        provider = self._get_provider_for_tenant(tenant_config)
        
        # Call LLM with tenant's API key
        response = await provider.chat(
            system_prompt=system_prompt,
            messages=session.messages,
            api_key=tenant_config.llm_api_key,  # Per-tenant key
            model=tenant_config.llm_model,
            temperature=tenant_config.llm_temperature,
            max_tokens=tenant_config.llm_max_tokens,
        )
        
        return response
    
    def _build_tenant_prompt(self, config: TenantConfig) -> str:
        """Build system prompt incorporating tenant's business profile."""
        bp = config.business_profile
        
        base = f"""Kamu adalah AI Sales Agent untuk {bp['business_name']}.

Informasi Bisnis:
- Nama: {bp['business_name']}
- Kategori: {bp['business_category']}
- Jam Operasional: {self._format_hours(bp['operating_hours'])}
- Bahasa: {bp['default_language']}

Kebijakan:
- Pengiriman: {bp.get('shipping_policy', 'Standar')}
- Retur: {bp.get('return_policy', 'Standar')}
- Pembayaran: {bp.get('payment_policy', 'Standar')}

Tone of Voice: {bp.get('tone_of_voice', 'friendly_professional')}

{config.custom_system_prompt or ''}

PENTING:
- Jawab hanya berdasarkan informasi di atas dan product catalog tenant.
- Jangan mengarang kebijakan yang tidak ada.
- Jika tidak tahu, arahkan ke human support.
"""
        return base
```

### 5.7 Modified: Backend Client (Pass account_id)

```python
# apps/ai/app/clients/backend_client.py — key changes

class BackendClient:
    async def _request(
        self, method: str, path: str, account_id: str, **kwargs
    ) -> dict:
        """All internal API calls now include X-Account-ID header."""
        headers = kwargs.pop("headers", {})
        headers["X-Service-Token"] = self._service_token
        headers["X-Account-ID"] = account_id  # NEW: tenant context
        headers["Content-Type"] = "application/json"
        
        url = f"{self._base_url}{path}"
        async with httpx.AsyncClient() as client:
            resp = await client.request(method, url, headers=headers, **kwargs)
            resp.raise_for_status()
            return resp.json()
    
    async def get_tenant_config(self, account_id: str) -> TenantConfig:
        """Fetch full tenant configuration from backend."""
        # This is a new internal endpoint we need to create
        resp = await self._request("GET", "/internal/v1/tenant/config", account_id)
        return TenantConfig(**resp["data"])
    
    async def search_products(self, account_id: str, query: str) -> list:
        resp = await self._request(
            "GET", f"/internal/v1/products/search?q={query}", account_id
        )
        return resp["data"]["items"]
    
    async def get_cart(self, account_id: str, customer_id: str) -> dict:
        resp = await self._request(
            "GET", f"/internal/v1/customers/{customer_id}/cart", account_id
        )
        return resp["data"]
    
    # ... all other methods similarly add account_id ...
```

### 5.8 New Internal Endpoint: Tenant Config

```go
// Backend: apps/api/internal/tenant/handler.go

// GetTenantConfig returns full tenant configuration for AI service consumption.
// Internal endpoint: GET /internal/v1/tenant/config
func (h *Handler) GetTenantConfig(c *gin.Context) {
    accountID := c.GetHeader("X-Account-ID")
    if accountID == "" {
        response.Fail(c, http.StatusBadRequest, "MISSING_ACCOUNT_ID", "X-Account-ID header required")
        return
    }
    
    config, err := h.service.GetFullTenantConfig(c.Request.Context(), accountID)
    if err != nil {
        response.Error(c, err)
        return
    }
    
    // NEVER return raw credentials — decrypt and return only what AI needs
    response.OK(c, config)
}
```

---

## 6. Admin Frontend Implementation — Next.js

### 6.1 New/Modified Files

```
apps/admin/
├── app/(dashboard)/
│   ├── settings/                     # NEW: Settings landing
│   │   └── page.tsx
│   ├── settings/
│   │   ├── business/                 # NEW: Business profile
│   │   │   └── page.tsx
│   │   ├── channel/                  # NEW: WhatsApp channel
│   │   │   └── page.tsx
│   │   ├── llm/                      # NEW: LLM configuration
│   │   │   └── page.tsx
│   │   ├── shipping/                 # NEW: Shipping configuration
│   │   │   └── page.tsx
│   │   └── payment/                  # NEW: Payment configuration
│   │       └── page.tsx
│
├── components/
│   ├── settings/                     # NEW: Shared settings components
│   │   ├── settings-layout.tsx       # Shared shell for all settings pages
│   │   ├── provider-card.tsx         # Radio card for provider selection
│   │   ├── credential-input.tsx      # Masked API key input with reveal
│   │   ├── validation-badge.tsx      # Credential validation status badge
│   │   ├── status-badge.tsx          # Channel/profile status badge
│   │   ├── operating-hours-editor.tsx # Time picker for operating hours
│   │   └── save-bar.tsx              # Sticky bottom save bar
│
├── lib/
│   ├── api.ts                        # MODIFIED: Add settings API methods
│   └── settings-api.ts               # NEW: Settings-specific API client
│
└── hooks/
    └── use-tenant-config.ts          # NEW: Hook for tenant config state
```

### 6.2 API Client Extensions

```typescript
// apps/admin/lib/settings-api.ts

import { apiRequest } from "./api";

// --- Business Profile ---

export async function getBusinessProfile(): Promise<BusinessProfile> {
  return apiRequest("/settings/business");
}

export async function updateBusinessProfile(data: BusinessProfileInput): Promise<BusinessProfile> {
  return apiRequest("/settings/business", {
    method: "PUT",
    body: JSON.stringify(data),
  });
}

// --- WhatsApp Channel ---

export async function listChannels(): Promise<WhatsAppChannel[]> {
  return apiRequest("/settings/channel");
}

export async function createChannel(data: WhatsAppChannelInput): Promise<WhatsAppChannel> {
  return apiRequest("/settings/channel", {
    method: "POST",
    body: JSON.stringify(data),
  });
}

export async function activateChannel(channelId: string): Promise<WhatsAppChannel> {
  return apiRequest(`/settings/channel/${channelId}/activate`, {
    method: "POST",
  });
}

export async function validateChannelCredential(channelId: string): Promise<ValidationResult> {
  return apiRequest(`/settings/channel/${channelId}/validate`, {
    method: "POST",
  });
}

// --- LLM Configuration ---

export async function getLLMConfig(): Promise<LLMConfiguration> {
  return apiRequest("/settings/llm");
}

export async function updateLLMConfig(data: LLMConfigInput): Promise<LLMConfiguration> {
  return apiRequest("/settings/llm", {
    method: "PUT",
    body: JSON.stringify(data),
  });
}

export async function validateLLMKey(provider: string, apiKey: string): Promise<ValidationResult> {
  return apiRequest("/settings/llm/validate", {
    method: "POST",
    body: JSON.stringify({ provider, api_key: apiKey }),
  });
}

// --- Shipping Configuration ---

export async function getShippingConfig(): Promise<ShippingConfiguration> {
  return apiRequest("/settings/shipping");
}

export async function updateShippingConfig(data: ShippingConfigInput): Promise<ShippingConfiguration> {
  return apiRequest("/settings/shipping", {
    method: "PUT",
    body: JSON.stringify(data),
  });
}

// --- Payment Configuration ---

export async function getPaymentConfig(): Promise<PaymentConfiguration> {
  return apiRequest("/settings/payment");
}

export async function updatePaymentConfig(data: PaymentConfigInput): Promise<PaymentConfiguration> {
  return apiRequest("/settings/payment", {
    method: "PUT",
    body: JSON.stringify(data),
  });
}
```

### 6.3 Sidebar Navigation Changes

```typescript
// apps/admin/components/nav-main.tsx — add Settings section

// Add to navItems:
{
  title: "Settings",
  url: "/settings",
  icon: Settings,
  items: [
    {
      title: "Business Profile",
      url: "/settings/business",
    },
    {
      title: "WhatsApp Channel",
      url: "/settings/channel",
    },
    {
      title: "AI / LLM",
      url: "/settings/llm",
    },
    {
      title: "Shipping",
      url: "/settings/shipping",
    },
    {
      title: "Payment",
      url: "/settings/payment",
    },
  ],
}
```

### 6.4 Key UI Components

**Provider Selection Card (Radio Card pattern):**

```tsx
// apps/admin/components/settings/provider-card.tsx

interface ProviderCardProps {
  id: string;
  name: string;
  description: string;
  icon: React.ReactNode;
  selected: boolean;
  onSelect: (id: string) => void;
  disabled?: boolean;
  badge?: string;  // e.g., "Recommended", "Active"
}

export function ProviderCard({ id, name, description, icon, selected, onSelect, disabled, badge }: ProviderCardProps) {
  return (
    <button
      type="button"
      role="radio"
      aria-checked={selected}
      disabled={disabled}
      onClick={() => onSelect(id)}
      className={cn(
        "flex items-start gap-4 p-4 rounded-lg border-2 transition-all w-full text-left",
        selected
          ? "border-primary bg-primary/5 ring-1 ring-primary"
          : "border-border hover:border-muted-foreground/30",
        disabled && "opacity-50 cursor-not-allowed"
      )}
    >
      <div className="flex-shrink-0 mt-0.5">
        {selected ? (
          <div className="w-5 h-5 rounded-full bg-primary flex items-center justify-center">
            <Check className="w-3 h-3 text-primary-foreground" />
          </div>
        ) : (
          <div className="w-5 h-5 rounded-full border-2 border-muted-foreground/30" />
        )}
      </div>
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2">
          <span className="font-medium">{name}</span>
          {badge && (
            <span className="text-xs px-1.5 py-0.5 rounded bg-secondary text-secondary-foreground">
              {badge}
            </span>
          )}
        </div>
        <p className="text-sm text-muted-foreground mt-1">{description}</p>
      </div>
      <div className="flex-shrink-0 text-muted-foreground">{icon}</div>
    </button>
  );
}
```

**Credential Input (Masked with reveal):**

```tsx
// apps/admin/components/settings/credential-input.tsx

interface CredentialInputProps {
  label: string;
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  hint?: string;         // e.g., "sk-...abc123" for masked display
  validationStatus?: 'not_validated' | 'valid' | 'invalid' | 'validating';
  validationError?: string;
  onValidate?: () => void;
}

export function CredentialInput({
  label, value, onChange, placeholder, hint,
  validationStatus, validationError, onValidate
}: CredentialInputProps) {
  const [revealed, setRevealed] = useState(false);
  
  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <Label>{label}</Label>
        {validationStatus && (
          <ValidationBadge status={validationStatus} />
        )}
      </div>
      
      <div className="relative">
        <Input
          type={revealed ? "text" : "password"}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          placeholder={placeholder || "••••••••••••••••••••••••••"}
          className="pr-20 font-mono"
        />
        <div className="absolute right-1 top-1/2 -translate-y-1/2 flex items-center gap-1">
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="h-7 w-7"
            onClick={() => setRevealed(!revealed)}
          >
            {revealed ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
          </Button>
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="h-7 w-7"
            onClick={() => {
              navigator.clipboard.writeText(value);
              toast.success("API key copied");
            }}
          >
            <Copy className="h-4 w-4" />
          </Button>
        </div>
      </div>
      
      {hint && !revealed && (
        <p className="text-xs text-muted-foreground">Current: {hint}</p>
      )}
      
      {validationError && (
        <p className="text-sm text-destructive">{validationError}</p>
      )}
      
      {onValidate && (
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={onValidate}
          disabled={!value || validationStatus === 'validating'}
        >
          {validationStatus === 'validating' ? (
            <><Loader2 className="mr-2 h-3 w-3 animate-spin" /> Validating...</>
          ) : (
            'Validate Credential'
          )}
        </Button>
      )}
    </div>
  );
}
```

### 6.5 Settings Page Shell

All settings pages share this layout:

```tsx
// apps/admin/components/settings/settings-layout.tsx

export function SettingsLayout({
  title,
  description,
  children,
  onSave,
  isSaving,
  saveDisabled,
}: {
  title: string;
  description: string;
  children: React.ReactNode;
  onSave: () => void;
  isSaving?: boolean;
  saveDisabled?: boolean;
}) {
  return (
    <div className="max-w-3xl mx-auto">
      {/* Header */}
      <div className="mb-8">
        <h1 className="text-2xl font-bold tracking-tight">{title}</h1>
        <p className="text-muted-foreground mt-2">{description}</p>
      </div>
      
      {/* Form Content */}
      <div className="space-y-8">{children}</div>
      
      {/* Sticky Save Bar */}
      <div className="sticky bottom-0 mt-8 -mx-6 px-6 py-4 bg-background border-t flex items-center justify-end gap-3">
        <Button variant="outline" type="button">
          Cancel
        </Button>
        <Button onClick={onSave} disabled={saveDisabled || isSaving}>
          {isSaving ? (
            <><Loader2 className="mr-2 h-4 w-4 animate-spin" /> Saving...</>
          ) : (
            'Save Changes'
          )}
        </Button>
      </div>
    </div>
  );
}
```

---

## 7. Payment Provider Recommendation

### 7.1 Comparative Analysis

| Criteria | **Midtrans** | Xendit | Stripe |
|----------|-------------|--------|--------|
| **Indonesian Market Fit** | ⭐⭐⭐⭐⭐ Built for Indonesia | ⭐⭐⭐⭐ Strong in Indonesia | ⭐⭐ Limited ID support |
| **Payment Methods** | 40+ methods: bank transfer (BCA, BNI, Mandiri, BRI), QRIS, GoPay, OVO, Dana, LinkAja, ShopeePay, Alfamart, Indomaret, Akulaku, Kredivo | 20+ methods: VA, QRIS, e-wallet, retail outlets | Card-focused; limited ID e-wallets |
| **Bahasa Indonesia UI** | ✅ Full ID language | ✅ Full ID language | ❌ English only |
| **Documentation** | ⭐⭐⭐⭐ Good, ID-focused | ⭐⭐⭐⭐ Good | ⭐⭐⭐⭐⭐ Excellent (global) |
| **API Integration Complexity** | ⭐⭐⭐ Moderate (Snap makes it easy) | ⭐⭐⭐ Moderate | ⭐⭐ Simple but limited in ID |
| **Pricing** | Free setup; per-transaction fee (varies by method) | Free setup; per-transaction fee | 2.9% + $0.30 (expensive for ID) |
| **Fraud Detection** | ✅ Built-in risk analysis | ✅ Basic | ✅ Radar (advanced) |
| **Dashboard/Reporting** | ✅ MAP (Merchant Admin Portal) | ✅ Xendit Dashboard | ✅ Stripe Dashboard |
| **Recurring/Subscription** | ✅ Via Snap + Core API | ✅ Via Invoice API | ✅ First-class subscriptions |
| **Payout/Settlement** | Daily settlement to bank | Daily settlement | Weekly (slower in ID) |
| **Customer Support** | ⭐⭐⭐⭐⭐ ID team, WhatsApp, phone | ⭐⭐⭐⭐ ID team, email | ⭐⭐⭐ English only, email |
| **Market Share in Indonesia** | #1 (majority of e-commerce) | #2 (growing fast) | Minimal |

### 7.2 Recommendation: **Midtrans** as Primary

**Rationale:**

1. **Most complete Indonesian payment coverage.** Midtrans supports virtually every payment method Indonesian customers use: bank transfer (all major banks), QRIS (mandatory for Indonesian merchants), GoPay, OVO, Dana, LinkAja, ShopeePay, and offline channels (Alfamart, Indomaret). This is critical for an AI sales platform targeting Indonesian businesses.

2. **Snap API reduces integration complexity.** Midtrans Snap provides a hosted payment page that handles the entire UI — no need to build custom payment forms for each method. Just redirect to Snap URL, and Midtrans handles the rest.

3. **Best local support.** Indonesian-speaking support team available via WhatsApp and phone. Critical for Indonesian business owners who may need help.

4. **Market leader.** Most Indonesian e-commerce platforms use Midtrans. Your users will likely already be familiar with it.

5. **QRIS mandatory compliance.** Bank Indonesia mandates QRIS for all merchants. Midtrans has first-class QRIS support.

### 7.3 Architecture: Multi-Provider Support

Even though Midtrans is recommended as default, the platform should support multiple payment providers so users can choose:

```
Payment Provider Interface:
├── MidtransAdapter    (recommended default)
├── XenditAdapter      (existing — keep for users who prefer it)
└── StripeAdapter      (for international-facing businesses)
```

Each tenant selects one provider and inputs their own credentials (server key, client key) obtained from their own provider account.

### 7.4 Midtrans Integration Flow

```
1. Tenant registers at midtrans.com → gets Server Key + Client Key + Merchant ID
2. Tenant inputs credentials in /settings/payment
3. Backend validates credential via Midtrans API (GET /v2/balance or similar)
4. On checkout:
   a. Backend creates transaction via Midtrans Snap API
   b. Returns snap_token + redirect_url to AI
   c. AI sends payment link to customer via WhatsApp
5. Customer pays on Midtrans hosted page
6. Midtrans sends webhook (HTTP POST) to backend
7. Backend verifies webhook signature → updates order status
8. Backend emits order.paid domain event → AI sends confirmation
```

### 7.5 Midtrans Configuration Schema

```json
{
  "provider": "midtrans",
  "server_key": "Midtrans-...",
  "client_key": "Midtrans-...",
  "merchant_id": "G...",
  "is_production": false,
  "enabled_methods": ["bank_transfer", "qris", "gopay", "ovo", "dana", "shopee_pay"],
  "snap_preferences": {
    "locale": "id",
    "show_payment_methods": true
  }
}
```

---

## 8. Migration Strategy — From Single-Tenant to Multi-Tenant

### 8.1 Phase Approach

**Phase 1: Schema Migration (Non-Breaking)**
- Add `account_id` columns to all existing tables (nullable initially)
- Create new tenant configuration tables
- Existing data gets `account_id = NULL` (legacy)
- Backend continues to work with NULL account_id (backward compatible)

**Phase 2: Backend Tenant Awareness**
- Add tenant middleware (extracts account_id from JWT)
- Modify repositories to filter by account_id when present
- If account_id is NULL/empty → use legacy behavior (no filter)
- Deploy and verify no regressions

**Phase 3: Admin UI for Tenant Config**
- Deploy settings pages
- Users can now configure their own business profile, channel, LLM, etc.
- New accounts get account_id populated automatically

**Phase 4: AI Service Tenant Awareness**
- Deploy tenant resolver + config loader
- AI routes inbound messages to correct tenant
- Legacy: if no tenant mapping, use default/hardcoded config (backward compatible)

**Phase 5: Enforce Tenant Isolation**
- Make account_id NOT NULL on all tables
- Remove legacy fallback paths
- All requests MUST have tenant context

### 8.2 Backward Compatibility During Migration

```go
// During Phase 2-4 transition:
func getAccountID(c *gin.Context) string {
    // Try from JWT (new flow)
    if claims, exists := c.Get("auth_claims"); exists {
        if userClaims, ok := claims.(*auth.Claims); ok && userClaims.AccountID != "" {
            return userClaims.AccountID
        }
    }
    // Try from header (internal API)
    if header := c.GetHeader("X-Account-ID"); header != "" {
        return header
    }
    // Legacy: empty string → repository uses no filter
    return ""
}

// In repository:
func (r *PostgresRepository) ListProducts(ctx context.Context, accountID string, ...) {
    if accountID != "" {
        query += " AND account_id = $1"
    }
    // If accountID empty, no filter (legacy mode)
}
```

---

## 9. Security Considerations

### 9.1 Credential Storage

| Layer | Protection |
|-------|-----------|
| **At Rest (DB)** | AES-256-GCM encrypted `encrypted_data` column. Master key stored in environment variable only (never in code, never in DB). |
| **In Transit** | HTTPS/TLS for all API communication. Credentials transmitted only during initial save (POST) and when AI service fetches config (internal API, service-token protected). |
| **In Memory (Backend)** | Decrypted only when needed for validation or passing to AI service. Immediately discarded after use. |
| **In Memory (AI Service)** | API keys held in `TenantConfig` dataclass during request processing. Not persisted to disk. |
| **In Logs** | NEVER log raw credentials. Log masked hints only (e.g., `sk-...abc123`). |
| **In Admin UI** | Masked by default (password field). Reveal requires explicit user action. Never transmitted in URL query strings. |

### 9.2 Master Key Management

```
Generation:
  openssl rand -base64 32
  → Store as CREDENTIAL_MASTER_KEY env var

Rotation (future):
  1. Generate new key (key_id: mk-2027-01-v2)
  2. Add to CREDENTIAL_MASTER_KEYS env var (JSON map)
  3. On read: try current key first, fall back to old keys
  4. Background job: re-encrypt all credentials with new key
  5. Remove old key after all re-encrypted
```

### 9.3 Tenant Isolation Enforcement

```go
// Every mutation endpoint MUST verify ownership:
func (s *Service) UpdateProduct(ctx context.Context, accountID, productID string, input ProductInput) error {
    existing, err := s.repo.GetProduct(ctx, productID)
    if err != nil {
        return err
    }
    if existing.AccountID != accountID {
        return ErrNotOwnedByTenant  // 403 Forbidden
    }
    // ... proceed with update ...
}
```

### 9.4 Internal API Security

```
Internal endpoints (/internal/v1/*):
  - Require X-Service-Token header (existing)
  - Require X-Account-ID header (NEW)
  - Backend validates service token BEFORE processing
  - AI service never exposes internal endpoints to internet
  - Network-level: internal endpoints only accessible from AI service container
```

---

## 10. Implementation Phases & Timeline

### Phase E1: Database & Foundation (Week 1-2)

| Task | Effort | Priority |
|------|--------|----------|
| Create 8 migration files (000011-000018) | 2 days | P0 |
| Apply migrations to dev database | 0.5 day | P0 |
| Add `account_id` to existing tables (nullable) | 1 day | P0 |
| Create `tenant/` domain package (models, repository interface) | 1 day | P0 |
| Implement Postgres repository for all new tables | 2 days | P0 |
| Create `credential/` encryption service | 1.5 days | P0 |
| Unit tests for credential service | 1 day | P0 |
| Unit tests for tenant repository | 1 day | P0 |

### Phase E2: Backend Tenant Logic (Week 2-3)

| Task | Effort | Priority |
|------|--------|----------|
| Tenant middleware (`middleware/tenant.go`) | 1 day | P0 |
| Tenant configuration service (validation, activation flows) | 2 days | P0 |
| Tenant configuration HTTP handlers | 1.5 days | P0 |
| Wire tenant services into container + router | 1 day | P0 |
| Modify existing repositories: add account_id parameter | 2 days | P0 |
| Modify existing services: validate tenant ownership | 1.5 days | P0 |
| New internal endpoint: GET /internal/v1/tenant/config | 0.5 day | P0 |
| Integration tests for tenant CRUD | 1.5 days | P1 |
| Integration tests for tenant isolation | 1 day | P1 |

### Phase E3: Admin Frontend Settings (Week 3-4)

| Task | Effort | Priority |
|------|--------|----------|
| Settings API client (`lib/settings-api.ts`) | 0.5 day | P0 |
| Shared settings components (layout, provider-card, credential-input, save-bar) | 1.5 days | P0 |
| Settings landing page (`/settings`) | 1 day | P0 |
| Business Profile page (`/settings/business`) | 1.5 days | P0 |
| WhatsApp Channel page (`/settings/channel`) | 2 days | P0 |
| LLM Configuration page (`/settings/llm`) | 1.5 days | P0 |
| Shipping Configuration page (`/settings/shipping`) | 1 day | P0 |
| Payment Configuration page (`/settings/payment`) | 1.5 days | P0 |
| Sidebar navigation update | 0.5 day | P0 |
| Form validation + error handling UX | 1 day | P1 |
| Responsive + accessibility polish | 1 day | P1 |

### Phase E4: AI Service Multi-Tenant (Week 4-5)

| Task | Effort | Priority |
|------|--------|----------|
| Tenant resolver (`tenant/resolver.py`) | 1 day | P0 |
| Tenant config loader (`tenant/config_loader.py`) | 1 day | P0 |
| WhatsApp adapter interface + Meta Cloud adapter | 2 days | P0 |
| Third-party WhatsApp adapter (generic) | 1.5 days | P1 |
| Modify WhatsApp service: multi-tenant inbound routing | 1.5 days | P0 |
| Modify LLM orchestrator: per-tenant provider + API key | 1 day | P0 |
| Modify LLM prompt builder: inject business profile | 0.5 day | P0 |
| Modify backend client: pass account_id in all calls | 1 day | P0 |
| Modify conversation manager: tenant-scoped sessions | 0.5 day | P0 |
| Modify recovery service: per-tenant sweeps | 1 day | P1 |
| Integration tests: multi-tenant message flow | 1.5 days | P1 |

### Phase E5: Payment Provider — Midtrans (Week 5-6)

| Task | Effort | Priority |
|------|--------|----------|
| Midtrans client (`internal/payment/midtrans/client.go`) | 1.5 days | P0 |
| Midtrans Snap API integration | 1 day | P0 |
| Midtrans webhook handler + signature verification | 1 day | P0 |
| Payment provider abstraction (interface for Midtrans + Xendit) | 1 day | P0 |
| Modify existing payment flow: use tenant's payment config | 1 day | P0 |
| Admin UI: Midtrans configuration fields | 0.5 day | P0 |
| Integration tests: Midtrans end-to-end | 1.5 days | P1 |
| Documentation: Midtrans setup guide for tenants | 0.5 day | P1 |

### Phase E6: Testing, Hardening & Migration (Week 6-7)

| Task | Effort | Priority |
|------|--------|----------|
| Full integration test suite (multi-tenant scenarios) | 2 days | P0 |
| Security audit: credential encryption, tenant isolation | 1 day | P0 |
| Performance test: tenant config caching effectiveness | 0.5 day | P1 |
| Data migration script (populate account_id for existing data) | 1 day | P0 |
| Make account_id NOT NULL on all tables | 0.5 day | P0 |
| Remove legacy fallback paths | 1 day | P0 |
| Production deployment checklist | 0.5 day | P0 |
| User documentation (how to configure your business) | 1 day | P1 |

**Total Estimated Effort: 6-7 weeks (1 developer full-time)**

---

## 11. Testing Strategy

### 11.1 Unit Tests

| Package | Test File | Coverage Target |
|---------|-----------|-----------------|
| `credential/` | `service_test.go` | Encrypt/Decrypt roundtrip, invalid key, tampered ciphertext |
| `tenant/` | `service_test.go` | Business profile validation, channel activation flow, provider switch, credential validation |
| `middleware/` | `tenant_test.go` | JWT extraction, missing claims, suspended tenant blocking |
| `ai/tenant/` | `test_resolver.py` | Phone→account mapping, cache hit/miss, unregister |
| `ai/tenant/` | `test_config_loader.py` | Cache hit, cache miss→backend fetch, invalidation |
| `ai/whatsapp/adapters/` | `test_meta_cloud.py` | Parse inbound, send message, validate credential |

### 11.2 Integration Tests

| Scenario | Description |
|----------|-------------|
| **Tenant A creates product → Tenant B cannot see it** | Verify isolation |
| **Tenant A's WhatsApp message → routed to Tenant A's AI** | Verify routing |
| **Tenant switches LLM provider → AI uses new provider** | Verify config propagation |
| **Tenant suspended → AI stops responding** | Verify suspension enforcement |
| **Credential validation fails → channel stays draft** | Verify validation gate |
| **Two tenants try to activate same phone number → second fails** | Verify uniqueness |

### 11.3 Security Tests

| Test | Description |
|------|-------------|
| **Encrypted credentials in DB** | Direct DB query: `encrypted_data` is not plaintext |
| **API response never includes raw key** | GET /settings/llm returns `display_hint` only |
| **Internal endpoint without service token → 401** | Verify auth |
| **Internal endpoint without X-Account-ID → 400** | Verify tenant context required |
| **Cross-tenant mutation → 403** | Tenant A tries to update Tenant B's product |

---

## 12. Deployment Checklist

### Pre-Deployment

- [ ] All 8 migrations applied to production DB
- [ ] `CREDENTIAL_MASTER_KEY` generated and stored in secrets manager (not in .env file)
- [ ] `account_id` backfilled for all existing data (assign to default/admin account)
- [ ] All `account_id` columns set to NOT NULL
- [ ] Legacy fallback code removed
- [ ] Full test suite passing (unit + integration + security)
- [ ] Midtrans production credentials obtained (if using real payments)

### Deployment Order

1. **Database migrations** (can run while old code is live — columns are nullable initially)
2. **Backend deploy** (new endpoints + tenant middleware)
3. **Admin frontend deploy** (settings pages)
4. **AI service deploy** (tenant resolver + multi-tenant routing)
5. **Verify:** Register a test tenant, configure all settings, send a WhatsApp message → should route correctly

### Rollback Plan

- Database migrations are backward-compatible (nullable columns)
- Backend: old code ignores `account_id` — no functional change for legacy
- AI service: if tenant resolver returns NULL, falls back to default config
- Admin frontend: settings pages are additive (don't modify existing pages)

### Monitoring

- Log metric: `tenant_config_load_duration_ms` — track config loading performance
- Log metric: `tenant_resolver_cache_hit_rate` — track Redis cache effectiveness
- Alert: `tenant_config_load_failed` — tenant can't operate if config unavailable
- Alert: `credential_decrypt_failed` — indicates master key mismatch or corruption
- Alert: `cross_tenant_access_blocked` — possible security issue or bug

---

## Appendix A: Environment Variables (New)

```bash
# apps/api/.env — additions

# Credential encryption master key (REQUIRED — generate with: openssl rand -base64 32)
CREDENTIAL_MASTER_KEY=

# Default provider when tenant hasn't configured their own (optional)
DEFAULT_LLM_PROVIDER=openai
DEFAULT_LLM_MODEL=gpt-4o-mini
DEFAULT_SHIPPING_PROVIDER=biteship
DEFAULT_PAYMENT_PROVIDER=midtrans
```

```bash
# apps/ai/.env — additions

# Tenant config cache TTL (seconds)
TENANT_CONFIG_CACHE_TTL=300

# Default config for legacy/unregistered numbers (optional — remove after migration complete)
DEFAULT_LLM_PROVIDER=openai
DEFAULT_LLM_API_KEY=sk-...
DEFAULT_LLM_MODEL=gpt-4o-mini
```

---

## Appendix B: Key Code Interfaces

### Backend: Repository Interface

```go
// apps/api/internal/tenant/repository.go

type Repository interface {
    // Business Profile
    GetBusinessProfile(ctx context.Context, accountID string) (*BusinessProfile, error)
    CreateBusinessProfile(ctx context.Context, profile *BusinessProfile) (*BusinessProfile, error)
    UpdateBusinessProfile(ctx context.Context, profile *BusinessProfile) (*BusinessProfile, error)
    
    // WhatsApp Channel
    ListWhatsAppChannels(ctx context.Context, accountID string) ([]WhatsAppChannel, error)
    GetWhatsAppChannel(ctx context.Context, channelID string) (*WhatsAppChannel, error)
    CreateWhatsAppChannel(ctx context.Context, channel *WhatsAppChannel) (*WhatsAppChannel, error)
    UpdateWhatsAppChannel(ctx context.Context, channel *WhatsAppChannel) (*WhatsAppChannel, error)
    GetActiveWhatsAppChannel(ctx context.Context, accountID string) (*WhatsAppChannel, error)
    
    // Credentials
    UpsertCredential(ctx context.Context, accountID, credType string, encryptedData []byte, keyID string) (*TenantCredential, error)
    GetCredential(ctx context.Context, accountID, credType string) (*TenantCredential, error)
    GetActiveCredential(ctx context.Context, accountID, credType string) (*TenantCredential, error)
    
    // LLM Configuration
    GetActiveLLMConfig(ctx context.Context, accountID string) (*LLMConfiguration, error)
    UpsertLLMConfig(ctx context.Context, config *LLMConfiguration) (*LLMConfiguration, error)
    
    // Shipping Configuration
    GetActiveShippingConfig(ctx context.Context, accountID string) (*ShippingConfiguration, error)
    UpsertShippingConfig(ctx context.Context, config *ShippingConfiguration) (*ShippingConfiguration, error)
    
    // Payment Configuration
    GetActivePaymentConfig(ctx context.Context, accountID string) (*PaymentConfiguration, error)
    UpsertPaymentConfig(ctx context.Context, config *PaymentConfiguration) (*PaymentConfiguration, error)
    
    // Full tenant config (for AI service)
    GetFullTenantConfig(ctx context.Context, accountID string) (*FullTenantConfig, error)
}
```

### AI Service: WhatsApp Adapter Interface

```python
# apps/ai/app/whatsapp/adapters/base.py

class WhatsAppAdapter(ABC):
    @abstractmethod
    async def verify_webhook(self, query_params: dict) -> str: ...
    
    @abstractmethod
    async def parse_inbound(self, raw_payload: dict) -> InboundMessage: ...
    
    @abstractmethod
    async def send_message(self, message: OutboundMessage) -> dict: ...
    
    @abstractmethod
    async def validate_credential(self, credentials: dict) -> bool: ...
    
    @abstractmethod
    async def get_health(self) -> dict: ...
```

### Admin Frontend: Settings API Types

```typescript
// apps/admin/lib/settings-api.ts — type definitions

interface BusinessProfile {
  id: string;
  business_name: string;
  business_category: string;
  contact_phone: string;
  origin_address: string;
  origin_city: string;
  origin_district: string;
  origin_postal_code: string;
  timezone: string;
  default_language: string;
  operating_hours: OperatingHours;
  tone_of_voice: string;
  status: 'incomplete' | 'active' | 'suspended' | 'inactive';
}

interface WhatsAppChannel {
  id: string;
  provider: 'meta_cloud_api' | 'third_party_gateway';
  phone_number: string;
  status: 'draft' | 'pending_verification' | 'active' | 'inactive' | 'failed' | 'revoked';
  verified_at?: string;
  verification_error?: string;
}

interface LLMConfiguration {
  id: string;
  provider: 'openai' | 'anthropic';
  model: string;
  temperature: number;
  max_tokens: number;
  is_active: boolean;
  display_hint: string;  // "sk-...abc123" — never the full key
}

interface ShippingConfiguration {
  id: string;
  provider: 'biteship';
  origin_postal_code: string;
  default_couriers: string;
  is_active: boolean;
  display_hint: string;
}

interface PaymentConfiguration {
  id: string;
  provider: 'midtrans' | 'xendit' | 'stripe';
  enabled_methods: string[];
  invoice_duration_hours: number;
  is_active: boolean;
  display_hint: string;
}
```

---

## Appendix C: Midtrans Integration Detail

### Snap API Flow

```go
// apps/api/internal/payment/midtrans/client.go

type MidtransClient struct {
    serverKey string
    baseURL   string  // https://app.midtrans.com/snap/v1 or https://app.sandbox.midtrans.com/snap/v1
}

func (c *MidtransClient) CreateTransaction(ctx context.Context, req *SnapRequest) (*SnapResponse, error) {
    // POST /snap/v1/transactions
    // Headers: Authorization: Basic base64(serverKey:)
    // Body: SnapRequest JSON
    
    url := c.baseURL + "/transactions"
    auth := base64.StdEncoding.EncodeToString([]byte(c.serverKey + ":"))
    
    resp, err := http.Post(url, "application/json", req.ToJSON())
    // ...
    return &SnapResponse{
        Token:       resp.Token,
        RedirectURL: resp.RedirectURL,
    }, nil
}

type SnapRequest struct {
    TransactionDetails struct {
        OrderID     string  `json:"order_id"`
        GrossAmount float64 `json:"gross_amount"`
    } `json:"transaction_details"`
    CustomerDetails struct {
        FirstName string `json:"first_name"`
        Phone     string `json:"phone"`
        Email     string `json:"email,omitempty"`
    } `json:"customer_details,omitempty"`
    Callbacks struct {
        Finish string `json:"finish"`  // Redirect after payment
    } `json:"callbacks,omitempty"`
}
```

### Webhook Verification

```go
func (c *MidtransClient) VerifyWebhookSignature(ctx context.Context, payload []byte, headerSignature string) error {
    // Midtrans signature: SHA512(order_id + status_code + gross_amount + server_key)
    // Compare with header "X-Midtrans-Signature"
    
    var webhook MidtransWebhook
    json.Unmarshal(payload, &webhook)
    
    expected := sha512(
        webhook.OrderID + webhook.StatusCode + fmt.Sprintf("%.2f", webhook.GrossAmount) + c.serverKey,
    )
    
    if !hmac.Equal([]byte(expected), []byte(headerSignature)) {
        return ErrInvalidSignature
    }
    return nil
}
```

---

**Document Version:** 1.0  
**Last Updated:** 2026-07-23  
**Author:** Kiro AI Assistant  
**Status:** Draft — Ready for Review

---

*This document covers the complete technical implementation plan for transforming the AI Sales Platform from single-tenant to multi-tenant SaaS with per-account business configuration, WhatsApp channel selection, LLM provider choice, shipping API key, and payment provider configuration.*