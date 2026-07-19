# 🚀 IMMEDIATE ACTIONS — Phase 4 Completion

## ⚠️ CRITICAL: You MUST Run These Commands Now

Phase 4 code is complete, but **Redis dependency is not installed**. This will cause build to fail.

---

## Step 1: Install Redis Dependency

**Run in terminal (PowerShell/WSL):**

```powershell
cd apps/api
go get github.com/redis/go-redis/v9@latest
go mod tidy
```

**Expected output:**
```
go: added github.com/redis/go-redis/v9 v9.x.x
go: added github.com/cespare/xxhash/v2 v2.x.x (indirect)
go: added github.com/dgryski/go-rendezvous v0.0.x (indirect)
```

✅ **Verify:** Check `go.mod` has `github.com/redis/go-redis/v9 vX.X.X`

---

## Step 2: Verify Compilation

**Run:**

```powershell
cd apps/api
go vet ./...
```

**Expected:** No errors or warnings

---

## Step 3: Build Test

**Run:**

```powershell
cd apps/api
go build -o ./bin/api ./cmd/api
```

**Expected:** Binary created at `./bin/api` without errors

---

## Step 4: Unit Tests

**Run:**

```powershell
cd apps/api
go test ./...
```

**Expected:** Tests pass (or skip if none exist)

---

## Step 5: Git Commit

**Run:**

```bash
cd apps/api
git add go.mod go.sum
cd ../..  # back to root
git add apps/api/internal/health/ 
cd apps/api
git add internal/http/middleware/rbac.go
git add internal/http/router/router.go
git add internal/container/container.go
cd ../..  # back to root
git add Makefile
git add docs/PHASE_4_*.md
git add docs/IMMEDIATE_ACTIONS.md

git commit -m "Phase 4: Platform hardening - health ready, RBAC, Redis wire, /api/v1 grouping"
```

---

## ✅ Verification Checklist

- [ ] `go get github.com/redis/go-redis/v9@latest` — completed
- [ ] `go mod tidy` — completed
- [ ] `go vet ./...` — no errors
- [ ] `go build -o ./bin/api ./cmd/api` — binary created
- [ ] `go test ./...` — tests pass
- [ ] Git commit — changes pushed

---

## 🎯 After Completion

Phase 4 is DONE. You're ready for **Phase 5: Product Catalog**.

Phase 5 will include:
- Categories domain
- Products domain  
- Product variants domain
- Admin CRUD API
- Internal search API for AI

---

## 🆘 If You Get Errors

### Error: `cannot find package "github.com/redis/go-redis/v9"`

**Solution:** Run `go get` again:
```bash
go get github.com/redis/go-redis/v9@latest
go mod tidy
```

### Error: Build fails after `go get`

**Solution:** Clean cache and rebuild:
```bash
go clean -modcache
go mod tidy
go build -o ./bin/api ./cmd/api
```

### Error: Port 8081 already in use (when testing)

**Solution:**
```bash
make dev-down
make dev
```

---

**Ready? Start with Step 1 above! ⬆️**
