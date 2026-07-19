# Phase 5: Product Catalog — Detailed Plan

**Status:** Starting  
**Phase:** 5 of 10  
**Focus:** Catalog domain (categories, products, variants) + Admin CRUD + Internal Search API  

---

## 📋 Overview

Phase 5 implements the **Product Catalog domain** - the core data model for the BR-004, BR-005, BR-048 business requirements.

**Key Deliverables:**
1. Database migrations (categories, products, product_variants)
2. Catalog domain with full CRUD
3. Admin API endpoints (`/api/v1/...`)
4. Internal search API (`/internal/v1/...`)
5. Service & repository layers
6. Unit tests

---

## 🗄️ Database Schema

### Migration 1: Create Categories Table

**File:** `database/migration/000002_create_categories.sql`

```sql
CREATE TABLE categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(255) NOT NULL UNIQUE,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_categories_slug ON categories(slug);
CREATE INDEX idx_categories_is_active ON categories(is_active);
```

---

### Migration 2: Create Products Table

**File:** `database/migration/000003_create_products.sql`

```sql
CREATE TABLE products (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    category_id UUID NOT NULL REFERENCES categories(id) ON DELETE RESTRICT,
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    status VARCHAR(50) NOT NULL CHECK (status IN ('active', 'inactive', 'draft')) DEFAULT 'draft',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_products_category_id ON products(category_id);
CREATE INDEX idx_products_slug ON products(slug);
CREATE INDEX idx_products_status ON products(status);
CREATE INDEX idx_products_status_created ON products(status, created_at);
```

---

### Migration 3: Create Product Variants Table

**File:** `database/migration/000004_create_product_variants.sql`

```sql
CREATE TABLE product_variants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    sku VARCHAR(255) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    price DECIMAL(10, 2) NOT NULL,
    stock INTEGER NOT NULL DEFAULT 0,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_variants_product_id ON product_variants(product_id);
CREATE INDEX idx_variants_sku ON product_variants(sku);
CREATE INDEX idx_variants_is_active ON product_variants(is_active);
CREATE INDEX idx_variants_stock ON product_variants(stock) WHERE is_active = true;
```

---

## 📦 Domain Structure

```
apps/api/internal/catalog/
├── handler.go              # HTTP handlers
├── service.go              # Business logic
├── repository.go           # Interface
├── postgres_repository.go  # Postgres implementation
├── models.go               # Data models
└── service_test.go         # Unit tests
```

---

## 🏗️ Data Models

### Category Model

```go
type Category struct {
    ID        string    `json:"id"`
    Name      string    `json:"name"`
    Slug      string    `json:"slug"`
    IsActive  bool      `json:"is_active"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}
```

### Product Model

```go
type Product struct {
    ID          string    `json:"id"`
    CategoryID  string    `json:"category_id"`
    Name        string    `json:"name"`
    Slug        string    `json:"slug"`
    Description string    `json:"description"`
    Status      string    `json:"status"` // active, inactive, draft
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
}

type ProductWithVariants struct {
    Product   `json:"inline"`
    Variants  []ProductVariant `json:"variants"`
}
```

### ProductVariant Model

```go
type ProductVariant struct {
    ID        string    `json:"id"`
    ProductID string    `json:"product_id"`
    SKU       string    `json:"sku"`
    Name      string    `json:"name"` // e.g., "Red XL", "Blue M"
    Price     float64   `json:"price"`
    Stock     int       `json:"stock"`
    IsActive  bool      `json:"is_active"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}
```

---

## 🔌 API Endpoints

### Admin API (`/api/v1`, JWT + RBAC)

#### Categories

```
POST   /api/v1/categories
GET    /api/v1/categories
GET    /api/v1/categories/:id
PUT    /api/v1/categories/:id
DELETE /api/v1/categories/:id
```

#### Products

```
POST   /api/v1/products
GET    /api/v1/products
GET    /api/v1/products/:id
PUT    /api/v1/products/:id
DELETE /api/v1/products/:id
```

#### Product Variants

```
POST   /api/v1/products/:id/variants
GET    /api/v1/products/:id/variants
GET    /api/v1/products/:id/variants/:variant_id
PUT    /api/v1/products/:id/variants/:variant_id
DELETE /api/v1/products/:id/variants/:variant_id
```

**Auth:** Requires JWT token + role `admin` or `manager`

---

### Internal API (`/internal/v1`, Service Token)

#### Search Products

```
GET /internal/v1/products/search?q=<query>&limit=<limit>&offset=<offset>
```

**Response:**
```json
{
  "data": {
    "items": [
      {
        "id": "...",
        "name": "...",
        "slug": "...",
        "variants": [
          {
            "id": "...",
            "sku": "...",
            "price": 99.99,
            "stock": 10
          }
        ]
      }
    ],
    "total": 100
  }
}
```

**Filter:**
- Only `active` products
- Only variants with `is_active = true` and `stock > 0`
- Search by product name, description

#### Get Product by ID/Slug

```
GET /internal/v1/products/:id
GET /internal/v1/products/slug/:slug
```

**Response:** Full product with active variants only

---

## 🔧 Implementation Order

### Step 1: Create Migrations
- [ ] Create `000002_create_categories.sql`
- [ ] Create `000003_create_products.sql`
- [ ] Create `000004_create_product_variants.sql`
- [ ] Verify migrations can run

### Step 2: Create Domain Package
- [ ] Create `apps/api/internal/catalog/` directory
- [ ] Create `models.go` with data structs
- [ ] Create `repository.go` interface

### Step 3: Implement Repository
- [ ] Create `postgres_repository.go`
- [ ] Implement CRUD operations
- [ ] Implement search queries

### Step 4: Implement Service
- [ ] Create `service.go`
- [ ] Business logic (validation, transactions)
- [ ] Error handling

### Step 5: Implement Handlers
- [ ] Create `handler.go`
- [ ] Admin endpoints (CRUD)
- [ ] Internal endpoints (search)
- [ ] Middleware integration

### Step 6: Wire Routes
- [ ] Register handlers in router
- [ ] Apply RBAC middleware to admin routes
- [ ] Apply service token middleware to internal routes

### Step 7: Tests & Verification
- [ ] Unit tests
- [ ] Integration tests
- [ ] Manual curl tests
- [ ] Load testing (optional)

---

## 📝 Request/Response Examples

### Create Category

**Request:**
```bash
POST /api/v1/categories
Authorization: Bearer <token>
Content-Type: application/json

{
  "name": "Electronics",
  "slug": "electronics",
  "is_active": true
}
```

**Response (201 Created):**
```json
{
  "code": "OK",
  "message": "Category created",
  "data": {
    "id": "uuid-123",
    "name": "Electronics",
    "slug": "electronics",
    "is_active": true,
    "created_at": "2026-07-19T08:33:23Z",
    "updated_at": "2026-07-19T08:33:23Z"
  }
}
```

---

### Create Product

**Request:**
```bash
POST /api/v1/products
Authorization: Bearer <token>
Content-Type: application/json

{
  "category_id": "uuid-123",
  "name": "Laptop Pro",
  "slug": "laptop-pro",
  "description": "High-performance laptop",
  "status": "active"
}
```

**Response (201 Created):**
```json
{
  "code": "OK",
  "message": "Product created",
  "data": {
    "id": "uuid-456",
    "category_id": "uuid-123",
    "name": "Laptop Pro",
    "slug": "laptop-pro",
    "description": "High-performance laptop",
    "status": "active",
    "created_at": "2026-07-19T08:33:23Z",
    "updated_at": "2026-07-19T08:33:23Z"
  }
}
```

---

### Create Product Variant

**Request:**
```bash
POST /api/v1/products/uuid-456/variants
Authorization: Bearer <token>
Content-Type: application/json

{
  "sku": "LAPTOP-PRO-001",
  "name": "Silver 16GB",
  "price": 1299.99,
  "stock": 50,
  "is_active": true
}
```

**Response (201 Created):**
```json
{
  "code": "OK",
  "message": "Variant created",
  "data": {
    "id": "uuid-789",
    "product_id": "uuid-456",
    "sku": "LAPTOP-PRO-001",
    "name": "Silver 16GB",
    "price": 1299.99,
    "stock": 50,
    "is_active": true,
    "created_at": "2026-07-19T08:33:23Z",
    "updated_at": "2026-07-19T08:33:23Z"
  }
}
```

---

### Search Products (Internal API)

**Request:**
```bash
GET /internal/v1/products/search?q=laptop&limit=10&offset=0
X-Service-Token: <service_token>
```

**Response (200 OK):**
```json
{
  "code": "OK",
  "message": "OK",
  "data": {
    "items": [
      {
        "id": "uuid-456",
        "name": "Laptop Pro",
        "slug": "laptop-pro",
        "description": "High-performance laptop",
        "status": "active",
        "variants": [
          {
            "id": "uuid-789",
            "sku": "LAPTOP-PRO-001",
            "name": "Silver 16GB",
            "price": 1299.99,
            "stock": 50,
            "is_active": true
          }
        ]
      }
    ],
    "total": 1
  }
}
```

---

## ✅ Acceptance Criteria

- [ ] All migrations run successfully
- [ ] Category CRUD endpoints work
- [ ] Product CRUD endpoints work
- [ ] Variant CRUD endpoints work
- [ ] Admin can manage all resources
- [ ] Internal search API returns correct results
- [ ] Only active products appear in internal API
- [ ] Only variants with stock > 0 appear in internal API
- [ ] Proper error handling and validation
- [ ] Unit tests covering business logic
- [ ] No compilation errors
- [ ] Backend runs without crashes

---

## 📚 References

- Roadmap: `docs/be/go_backend_roadmap_e471211b.plan.md`
- Phase 4 docs: `docs/PHASE_4_*.md`
- Code patterns: `apps/api/internal/auth/` (reference implementation)

---

## 🎯 Success Criteria

Phase 5 is complete when:

1. ✅ All 3 migrations created and applied
2. ✅ Catalog domain fully implemented
3. ✅ Admin CRUD API endpoints working
4. ✅ Internal search API working
5. ✅ Unit tests written and passing
6. ✅ Manual testing complete
7. ✅ Code review approved
8. ✅ Ready for Phase 6 (Customers & Cart)

---

*Phase 5 Plan Created: 2026-07-19*
