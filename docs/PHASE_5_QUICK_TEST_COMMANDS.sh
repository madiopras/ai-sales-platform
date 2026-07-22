#!/bin/bash
# Phase 5 Testing - Quick Reference Commands
# Copy and paste these commands to test Phase 5

echo "╔════════════════════════════════════════════════════════════╗"
echo "║         PHASE 5 TESTING — QUICK COMMANDS                   ║"
echo "║              Product Catalog Testing                        ║"
echo "╚════════════════════════════════════════════════════════════╝"

# ============ SETUP ============
echo ""
echo "STEP 1: Apply Migrations"
echo "Run these 3 commands:"
echo "  psql \$POSTGRES_URL < database/migration/000002_create_categories.sql"
echo "  psql \$POSTGRES_URL < database/migration/000003_create_products.sql"
echo "  psql \$POSTGRES_URL < database/migration/000004_create_product_variants.sql"

echo ""
echo "STEP 2: Start Backend"
echo "  make api-run"

echo ""
echo "STEP 3: Get Token (in another terminal)"
echo ""

curl -s -X POST http://localhost:8081/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email": "admin@example.com", "password": "password123"}' | jq .

echo ""
echo "👉 Save token to variable: TOKEN=\"<PASTE_TOKEN>\""
echo ""

# ============ TESTING ============
echo "════════════════════════════════════════════════════════════"
echo "TESTING COMMANDS"
echo "════════════════════════════════════════════════════════════"

echo ""
echo "TEST 1: Create Category"
echo "curl -X POST http://localhost:8081/api/v1/categories \\"
echo "  -H \"Authorization: Bearer \$TOKEN\" \\"
echo "  -H \"Content-Type: application/json\" \\"
echo "  -d '{\"name\": \"Electronics\", \"slug\": \"electronics\", \"is_active\": true}'"

echo ""
echo "TEST 2: List Categories"
echo "curl http://localhost:8081/api/v1/categories \\"
echo "  -H \"Authorization: Bearer \$TOKEN\""

echo ""
echo "TEST 3: Create Product"
echo "curl -X POST http://localhost:8081/api/v1/products \\"
echo "  -H \"Authorization: Bearer \$TOKEN\" \\"
echo "  -H \"Content-Type: application/json\" \\"
echo "  -d '{"
echo "    \"category_id\": \"<CATEGORY_ID>\","
echo "    \"name\": \"Laptop Pro\","
echo "    \"slug\": \"laptop-pro\","
echo "    \"description\": \"High-performance laptop\","
echo "    \"status\": \"active\""
echo "  }'"

echo ""
echo "TEST 4: Create Variant"
echo "curl -X POST http://localhost:8081/api/v1/products/<PRODUCT_ID>/variants \\"
echo "  -H \"Authorization: Bearer \$TOKEN\" \\"
echo "  -H \"Content-Type: application/json\" \\"
echo "  -d '{"
echo "    \"sku\": \"LAPTOP-PRO-001\","
echo "    \"name\": \"Silver 16GB\","
echo "    \"price\": 1299.99,"
echo "    \"stock\": 50,"
echo "    \"is_active\": true"
echo "  }'"

echo ""
echo "TEST 5: Search Products (Internal API)"
echo "curl \"http://localhost:8081/internal/v1/products/search?q=laptop&limit=10&offset=0\""

echo ""
echo "TEST 6: Get Product by Slug (Internal API)"
echo "curl http://localhost:8081/internal/v1/products/slug/laptop-pro"

echo ""
echo "TEST 7: Test Auth (should 401)"
echo "curl http://localhost:8081/api/v1/categories"

echo ""
echo "════════════════════════════════════════════════════════════"
echo "✅ All tests passed? Ready for Phase 6!"
echo "════════════════════════════════════════════════════════════"
