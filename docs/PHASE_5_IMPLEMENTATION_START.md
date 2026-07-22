# 🚀 Phase 5: Product Catalog — Implementation Start

**Status:** Ready to implement  
**Estimated Time:** 2-3 hours  
**Complexity:** Medium  

---

## 📋 Quick Summary

**Phase 5 will implement:**

1. **3 Database Tables**
   - `categories` — product categories
   - `products` — product information
   - `product_variants` — SKU, price, stock per variant

2. **3 Database Migrations**
   - `000002_create_categories.sql`
   - `000003_create_products.sql`
   - `000004_create_product_variants.sql`

3. **Catalog Domain Package**
   - `apps/api/internal/catalog/` directory
   - Models, Repository, Service, Handlers
   - Full CRUD operations

4. **Admin API Endpoints** (`/api/v1`)
   - Create, Read, Update, Delete categories
   - Create, Read, Update, Delete products
   - Create, Read, Update, Delete variants
   - List with filters (status, category, search)

5. **Internal API Endpoints** (`/internal/v1`)
   - Search products by query
   - Get product by ID or slug
   - Only return active products with available stock

6. **Tests & Verification**
   - Unit tests
   - Integration tests
   - Manual API tests

---

## 📊 Implementation Breakdown

### Part 1: Database Migrations (30 min)

**Files to create:**
- [ ] `database/migration/000002_create_categories.sql`
- [ ] `database/migration/000003_create_products.sql`
- [ ] `database/migration/000004_create_product_variants.sql`

**Tasks:**
- [ ] Create category table with indexes
- [ ] Create product table with foreign key to category
- [ ] Create variants table with foreign key to product
- [ ] Apply migrations to verify they work

---

### Part 2: Catalog Domain Structure (30 min)

**Files to create:**
- [ ] `apps/api/internal/catalog/models.go`
- [ ] `apps/api/internal/catalog/repository.go`
- [ ] `apps/api/internal/catalog/postgres_repository.go`

**Tasks:**
- [ ] Define Category, Product, ProductVariant structs
- [ ] Create repository interface
- [ ] Implement Postgres repository
- [ ] CRUD operations (Create, Read, Update, Delete, List)

---

### Part 3: Service Layer (30 min)

**Files to create:**
- [ ] `apps/api/internal/catalog/service.go`

**Tasks:**
- [ ] Create CatalogService
- [ ] Implement business logic
- [ ] Validation (name, slug, price, etc.)
- [ ] Error handling
- [ ] Transaction management

---

### Part 4: API Handlers (45 min)

**Files to create:**
- [ ] `apps/api/internal/catalog/handler.go`

**Tasks:**
- [ ] Create HTTP handlers for all endpoints
- [ ] Parse requests, validate, call service
- [ ] Return proper responses (200, 201, 400, 404, 500)
- [ ] Admin endpoints with RBAC
- [ ] Internal search endpoints

---

### Part 5: Router Integration (15 min)

**Files to modify:**
- [ ] `apps/api/internal/http/router/router.go`
- [ ] `apps/api/internal/container/container.go`

**Tasks:**
- [ ] Register catalog service in container
- [ ] Register catalog handler in router
- [ ] Add `/api/v1/categories` routes
- [ ] Add `/api/v1/products` routes
- [ ] Add `/internal/v1/products` routes
- [ ] Apply RBAC middleware to admin routes

---

### Part 6: Tests (30 min)

**Files to create:**
- [ ] `apps/api/internal/catalog/service_test.go`

**Tasks:**
- [ ] Unit tests for service layer
- [ ] Mock repository
- [ ] Test business logic
- [ ] Test error cases

---

### Part 7: Manual Testing (30 min)

**Create:** `docs/PHASE_5_TESTING.md`

**Tasks:**
- [ ] Test category CRUD via curl
- [ ] Test product CRUD via curl
- [ ] Test variant CRUD via curl
- [ ] Test search API
- [ ] Verify filters work
- [ ] Verify validation works

---

## 🎯 Success Checklist

### Database Layer
- [ ] All 3 migrations created
- [ ] Migrations apply without errors
- [ ] Tables exist with correct structure
- [ ] Indexes created
- [ ] Foreign keys working

### Domain Layer
- [ ] Models defined
- [ ] Repository interface defined
- [ ] Postgres repository implemented
- [ ] CRUD operations working
- [ ] Search queries working

### Service Layer
- [ ] Service created
- [ ] Business logic implemented
- [ ] Validation working
- [ ] Error handling in place
- [ ] Transactions working

### API Layer
- [ ] Handlers implemented
- [ ] Admin routes registered
- [ ] Internal routes registered
- [ ] RBAC middleware applied
- [ ] All endpoints responding

### Testing
- [ ] Unit tests written
- [ ] Manual tests executed
- [ ] All tests passing
- [ ] No compilation errors
- [ ] Backend runs without crashes

---

## 📝 File Summary

**To Create:**
```
apps/api/internal/catalog/
├── handler.go              # ~300 lines
├── service.go              # ~200 lines
├── repository.go           # ~100 lines (interface)
├── postgres_repository.go  # ~400 lines
├── models.go               # ~80 lines
└── service_test.go         # ~150 lines

database/migration/
├── 000002_create_categories.sql
├── 000003_create_products.sql
└── 000004_create_product_variants.sql

docs/
└── PHASE_5_TESTING.md
```

**To Modify:**
```
apps/api/internal/http/router/router.go
apps/api/internal/container/container.go
```

---

## 📚 Reference Implementation

Use Phase 4 & existing auth domain as reference:
- `apps/api/internal/auth/` — handler, service, repository pattern
- `apps/api/internal/health/` — simple service example
- `database/migration/000001_create_users.sql` — migration example

---

## 🚀 Ready to Start?

Let me know when you're ready and I'll:

1. Create all migration files
2. Create catalog domain structure
3. Implement all layers
4. Wire everything together
5. Create comprehensive tests
6. Provide manual testing guide

**Shall we begin?** 🎯
