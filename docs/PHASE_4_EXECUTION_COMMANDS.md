# Phase 4 Execution Commands

## 🚀 Langkah-langkah Eksekusi

### Step 1: Install Go Redis Dependency

**Jalankan di terminal (PowerShell/WSL):**

```powershell
cd apps/api
go get github.com/redis/go-redis/v9@latest
go mod tidy
```

**Hasil yang diharapkan:**
```
go: added github.com/redis/go-redis/v9 v9.x.x
go: added github.com/cespare/xxhash/v2 v2.x.x (indirect)
go: added github.com/dgryski/go-rendezvous v0.0.x (indirect)
```

**Verifikasi:** Cek file `apps/api/go.mod` sudah ada baris:
```
require (
    ...
    github.com/redis/go-redis/v9 vX.X.X
    ...
)
```

---

### Step 2: Verify Build & Vet

**Jalankan:**

```powershell
cd apps/api
go vet ./...
```

**Expected output:** Tidak ada error atau warning

**Test compile:**

```powershell
go build -o ./bin/api ./cmd/api
```

**Expected output:** Binary berhasil dibuat di `./bin/api`

---

### Step 3: Run Unit Tests

**Jalankan:**

```powershell
cd apps/api
go test ./...
```

**Expected:** Semua test pass (atau skip jika belum ada)

---

### Step 4: Start Docker Containers

**Jalankan:**

```powershell
make dev
```

**Tunggu ~10 detik untuk services ready:**

```powershell
make dev-ps
```

**Expected:** Semua container status `Up`

---

### Step 5: Test Health Endpoints

#### Test Live (always OK)

```bash
curl -i http://localhost:8081/health/live
```

**Expected response:**
```json
{
  "code": "OK",
  "message": "OK",
  "data": {
    "status": "ok"
  }
}
```

---

#### Test Ready (checks DB connection)

```bash
curl -i http://localhost:8081/health/ready
```

**Expected response (DB OK):**
```json
{
  "code": "OK",
  "message": "OK",
  "data": {
    "status": "ok",
    "checks": {
      "postgres": "ok"
    }
  }
}
```

**Expected response (DB down - 503 error):**
```json
{
  "code": "SERVICE_UNAVAILABLE",
  "message": "service unavailable",
  "data": {
    "status": "unavailable",
    "checks": {
      "postgres": "error: connection refused"
    }
  }
}
```

---

### Step 6: Test RBAC Middleware (Optional)

#### 6a. Test protected route without auth (should 401)

```bash
curl -i http://localhost:8081/api/v1/admin/test
```

**Expected:**
```json
{
  "code": "UNAUTHORIZED",
  "message": "authentication is required",
  "data": null
}
```

---

#### 6b. Get valid token

```bash
curl -X POST http://localhost:8081/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email": "admin@example.com", "password": "password123"}'
```

**Response:**
```json
{
  "code": "OK",
  "message": "OK",
  "data": {
    "access_token": "eyJhbGc..."
  }
}
```

---

#### 6c. Test with token

```bash
curl -i http://localhost:8081/api/v1/admin/test \
  -H "Authorization: Bearer eyJhbGc..."
```

**Expected:** Endpoint found (atau 404 jika endpoint belum ada, tapi auth pass)

---

### Step 7: Test Migration Makefile

#### Create new migration

```bash
make migrate-create
# Input: create_categories
```

**Expected output:**
```
Enter migration name (e.g. create_products): create_categories
Created: database/migration/1234567890_create_categories.sql
Edit the file and add your SQL statements.
```

**Verify file created:**

```bash
ls -la database/migration/
```

---

#### List migrations

```bash
make migrate-up
```

**Expected output:**
```
Running all migrations from database/migration...
Applying: database/migration/000001_create_users.sql
Applying: database/migration/000007_create_vouchers_audit.sql
Applying: database/migration/1234567890_create_categories.sql

Note: Use your database client to run migrations:
  psql $POSTGRES_URL < database/migration/000001_*.sql
  psql $POSTGRES_URL < database/migration/000007_*.sql
```

---

## ✅ Checklist Completion

- [ ] Run `go get github.com/redis/go-redis/v9@latest && go mod tidy`
- [ ] Run `go vet ./...` — no errors
- [ ] Run `go build -o ./bin/api ./cmd/api` — binary created
- [ ] Run `go test ./...` — tests pass
- [ ] Run `make dev` — containers running
- [ ] Test `curl http://localhost:8081/health/live` — returns ok
- [ ] Test `curl http://localhost:8081/health/ready` — checks postgres
- [ ] Test `make migrate-create` — creates migration file
- [ ] Commit changes:
  ```bash
  git add apps/api/go.mod apps/api/go.sum
  git add Makefile
  git add apps/api/internal/health/
  git add apps/api/internal/http/middleware/rbac.go
  git add apps/api/internal/http/router/router.go
  git add apps/api/internal/container/container.go
  git commit -m "Phase 4: Platform hardening - health ready, RBAC, Redis wire, /api/v1 grouping"
  ```

---

## 🛑 Troubleshooting

### Error: `cannot find package "github.com/redis/go-redis/v9"`

**Solusi:** Run `go get github.com/redis/go-redis/v9@latest` dan `go mod tidy`

---

### Error: `connection refused` on health/ready

**Solusi:** Postgres container belum siap. Tunggu:

```bash
make dev-logs
# Cari baris "database system is ready to accept connections"
```

---

### Error: Port 8081 already in use

**Solusi:**

```bash
make dev-down
make dev
```

---

## 📞 Next Steps

Setelah Phase 4 selesai:
1. Commit changes ke git (create PR)
2. Proceed to **Phase 5: Product Catalog**

Phase 5 akan include:
- `categories` domain
- `products` domain
- `product_variants` domain
- Admin CRUD API
- Internal search API untuk AI
