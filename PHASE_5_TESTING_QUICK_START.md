# Phase 5 Testing — Quick Start Guide

**Status:** Phase 5 fully implemented and wired  
**Next:** Apply migrations and test  

---

## 🚀 Quick Start (5 minutes)

### Step 1: Apply Database Migrations

```bash
cd /home/prasdios/aiproject/ai-sales-platform

# Apply migrations in order
psql $POSTGRES_URL < database/migration/000002_create_categories.sql
psql $POSTGRES_URL < database/migration/000003_create_products.sql
psql $POSTGRES_URL < database/migration/000004_create_product_variants.sql

# Verify tables created
psql $POSTGRES_URL -c "\dt categories, products, product_variants"
```

**Expected:** 3 tables created successfully

---

### Step 2: Rebuild and Run Backend

```bash
# Stop existing backend if running
# Then run:
make api-run

# Wait for startup message (~5 seconds)
```

**Expected:** Backend starts on http://localhost:8081

---

### Step 3: Get Auth Token

```bash
curl -X POST http://localhost:8081/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email": "admin@example.com", "password": "password123"}'
```

**Response:** Copy the `access_token` value

```json
{
  "code": "OK",
  "data": {
    "access_token": "eyJhbGc..."
  }
}
```

---

### Step 4: Test Category CRUD

#### Create Category
```bash
TOKEN="<PASTE_TOKEN_HERE>"

curl -X POST http://localhost:8081/api/v1/categories \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "Electronics", "slug": "electronics", "is_active": true}'
```

**Save the returned `id`** (you'll need it for products)

#### List Categories
```bash
curl http://localhost:8081/api/v1/categories \
  -H "Authorization: Bearer $TOKEN"
```

**Expected:** Array with electronics category

---

### Step 5: Test Product CRUD

#### Create Product
```bash
CATEGORY_ID="<PASTE_CATEGORY_ID>"

curl -X POST http://localhost:8081/api/v1/products \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"category_id\": \"$CATEGORY_ID\",
    \"name\": \"Laptop Pro\",
    \"slug\": \"laptop-pro\",
    \"description\": \"High-performance laptop\",
    \"status\": \"active\"
  }"
```

**Save the returned `id`**

#### List Products
```bash
curl "http://localhost:8081/api/v1/products?status=active" \
  -H "Authorization: Bearer $TOKEN"
```

---

### Step 6: Test Variant CRUD

#### Create Variant
```bash
PRODUCT_ID="<PASTE_PRODUCT_ID>"

curl -X POST http://localhost:8081/api/v1/products/$PRODUCT_ID/variants \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "sku": "LAPTOP-PRO-001",
    "name": "Silver 16GB",
    "price": 1299.99,
    "stock": 50,
    "is_active": true
  }'
```

#### List Variants
```bash
curl http://localhost:8081/api/v1/products/$PRODUCT_ID/variants \
  -H "Authorization: Bearer $TOKEN"
```

---

### Step 7: Test Internal Search API

#### Search Products
```bash
curl "http://localhost:8081/internal/v1/products/search?q=laptop&limit=10&offset=0"
```

**Expected:** Active product with available variants

#### Get Product by ID
```bash
curl http://localhost:8081/internal/v1/products/$PRODUCT_ID
```

#### Get Product by Slug
```bash
curl http://localhost:8081/internal/v1/products/slug/laptop-pro
```

---

## ✅ Verification Checklist

All tests passed?

- [ ] Migrations applied (3 tables created)
- [ ] Backend runs without errors
- [ ] Auth login returns token
- [ ] Create category: 201 Created
- [ ] List categories: 200 OK
- [ ] Create product: 201 Created
- [ ] List products: 200 OK
- [ ] Create variant: 201 Created
- [ ] List variants: 200 OK
- [ ] Search products: 200 OK, returns active products
- [ ] Get product by ID: 200 OK
- [ ] Get product by slug: 200 OK
- [ ] No auth token: 401 Unauthorized
- [ ] Invalid role: 403 Forbidden

---

## 🐛 Troubleshooting

### Error: "relation categories does not exist"

**Solution:** Migrations not applied. Run:
```bash
psql $POSTGRES_URL < database/migration/000002_*.sql
psql $POSTGRES_URL < database/migration/000003_*.sql
psql $POSTGRES_URL < database/migration/000004_*.sql
```

### Error: "cannot find package catalog"

**Solution:** Rebuild:
```bash
cd apps/api
go mod tidy
go build -o ./bin/api ./cmd/api
```

### Error: "slug already taken"

**Solution:** Use unique slug:
```json
{"slug": "electronics-unique-123"}
```

### Port 8081 already in use

**Solution:**
```bash
make dev-down
make dev
make api-run
```

---

## 📝 Testing Notes

- Save token and IDs in variables for easy reuse
- Each endpoint returns proper HTTP status codes
- Validation errors return 400 Bad Request
- Not found errors return 404 Not Found
- Duplicate unique fields return 409 Conflict
- Unauthorized returns 401
- Forbidden (insufficient role) returns 403

---

## 🎯 Next Phase

After testing Phase 5, commit and move to **Phase 6: Customers & Cart**

```bash
git add .
git commit -m "Phase 5: Product Catalog complete"
```

---

**Phase 5 is READY FOR TESTING!** 🚀
