# Phase 4 Setup & Completion Guide

## Status Implementasi Phase 4

### ✅ Sudah Dikerjakan

1. **Health Ready Check dengan DB Connection**
   - File: `apps/api/internal/health/service.go`
   - Perubahan: Added `pool *pgxpool.Pool`, `Ready(ctx context.Context)` method dengan `pool.Ping(ctx)`
   - Response: `{"status": "ok", "checks": {"postgres": "ok"}}` atau status `unavailable` jika error

2. **Health Handler Update**
   - File: `apps/api/internal/health/handler.go`
   - Perubahan: `Ready()` sekarang return 503 `SERVICE_UNAVAILABLE` jika status unavailable

3. **RBAC Middleware**
   - File: `apps/api/internal/http/middleware/rbac.go` (NEW)
   - Fungsi: `Authorize(allowedRoles ...string)` untuk validasi role dari JWT claims
   - Contoh: `apiV1.Use(middleware.Authorize("admin", "manager"))`

4. **Redis Client Wiring**
   - File: `apps/api/internal/container/container.go`
   - Perubahan: Added `Redis *redis.Client` field, initialize dengan graceful warning jika koneksi gagal

5. **Route Grouping `/api/v1` & `/internal/v1`**
   - File: `apps/api/internal/http/router/router.go`
   - Perubahan: Added route groups dengan middleware auth yang sudah siap untuk domain baru

6. **Makefile PHONY Updated**
   - File: `Makefile`
   - Perubahan: Added `migrate-up migrate-down migrate-create` ke `.PHONY` list

### ⚠️ Pekerjaan yang Masih Perlu Dikerjakan Oleh Developer

## 1. Install Go Redis Dependency

**Jalankan command di terminal:**

```bash
cd apps/api
go get github.com/redis/go-redis/v9@latest
go mod tidy
```

**Verifikasi:** Cek `go.mod` dan `go.sum` sudah ter-update dengan redis dependency.

## 2. Verify Compilation

**Jalankan:**

```bash
cd apps/api
go vet ./...
go build -o /tmp/test-build ./cmd/api
```

**Expected:** Tidak ada compile error.

## 3. Test Endpoints

Setelah container berjalan (`make dev`), test endpoints:

```bash
# Health Live (always ok)
curl http://localhost:8081/health/live
# Response: {"data":{"status":"ok"}}

# Health Ready (checks DB)
curl http://localhost:8081/health/ready
# Response: {"data":{"status":"ok","checks":{"postgres":"ok"}}}
# atau {"data":{"status":"unavailable","checks":{"postgres":"error: ..."}}} jika DB down
```

## 4. Migration Makefile Targets (Optional untuk Phase 4)

Makefile sudah ter-update dengan:

- `make migrate-create` — Creates new migration file dengan timestamp
- `make migrate-up` — Lists migrations ready to apply (manual apply via psql)
- `make migrate-down` — Shows up-only strategy message

**Contoh penggunaan:**

```bash
make migrate-create
# Input: create_categories
# Output: database/migration/1721378838_create_categories.sql

# Manual apply:
psql $POSTGRES_URL < database/migration/1721378838_create_categories.sql
```

---

## Deliverable Phase 4

✅ API siap terima domain baru dengan:
- Real readiness check via DB ping
- RBAC middleware untuk admin endpoints
- Redis client wired (non-blocking jika fail)
- `/api/v1` + `/internal/v1` route groups dengan auth
- Migration helper di Makefile

**Next Phase:** Phase 5 — Product Catalog
