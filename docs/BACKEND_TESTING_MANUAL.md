# Backend Testing Manual — Phase 4 Verification

## ✅ Backend Status

Backend sudah dijalankan dengan: `make api-run`

Sekarang test endpoints berikut untuk verify semua Phase 4 components bekerja.

---

## 🧪 Test 1: Health Live Endpoint

**Command:**
```bash
curl -i http://localhost:8081/health/live
```

**Expected Response (200 OK):**
```json
{
  "code": "OK",
  "message": "OK",
  "data": {
    "status": "ok"
  }
}
```

**Status Check:** ✅ If you see 200 OK with `status: ok`

---

## 🧪 Test 2: Health Ready Endpoint (DB Check)

**Command:**
```bash
curl -i http://localhost:8081/health/ready
```

**Expected Response (200 OK):**
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

**Status Check:** ✅ If you see:
- HTTP 200 OK
- `status: "ok"`
- `postgres: "ok"` in checks

**Note:** If postgres check shows error, verify containers:
```bash
make dev-ps | grep postgres
```

---

## 🧪 Test 3: Auth Login Endpoint

**Command:**
```bash
curl -X POST http://localhost:8081/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email": "admin@example.com", "password": "password123"}'
```

**Expected Response (200 OK or 401):**
```json
{
  "code": "OK",
  "message": "OK",
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIs..."
  }
}
```

or

```json
{
  "code": "UNAUTHORIZED",
  "message": "invalid credentials",
  "data": null
}
```

**Status Check:** ✅ If you see HTTP 200 or 401 (not 500)

**Save token:** If login successful, copy the `access_token` value for next tests.

---

## 🧪 Test 4: Protected Auth Me Endpoint (No Token)

**Command:**
```bash
curl -i http://localhost:8081/auth/me
```

**Expected Response (401 Unauthorized):**
```json
{
  "code": "UNAUTHORIZED",
  "message": "authentication is required",
  "data": null
}
```

**Status Check:** ✅ If you see HTTP 401 Unauthorized

---

## 🧪 Test 5: Protected Auth Me Endpoint (With Token)

**Command:**
```bash
curl -i http://localhost:8081/auth/me \
  -H "Authorization: Bearer <TOKEN_FROM_LOGIN>"
```

Replace `<TOKEN_FROM_LOGIN>` with actual token from Test 3.

**Example:**
```bash
curl -i http://localhost:8081/auth/me \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..."
```

**Expected Response (200 OK):**
```json
{
  "code": "OK",
  "message": "OK",
  "data": {
    "id": "...",
    "email": "admin@example.com",
    "name": "...",
    "role": "admin"
  }
}
```

**Status Check:** ✅ If you see HTTP 200 with user data

---

## 🧪 Test 6: API v1 Protected Endpoint (No Token)

**Command:**
```bash
curl -i http://localhost:8081/api/v1/test
```

**Expected Response (401 Unauthorized):**
```json
{
  "code": "UNAUTHORIZED",
  "message": "authentication is required",
  "data": null
}
```

**Status Check:** ✅ If you see HTTP 401 (not 404)

This verifies RBAC middleware is protecting /api/v1 routes.

---

## 🧪 Test 7: API v1 Protected Endpoint (With Token)

**Command:**
```bash
curl -i http://localhost:8081/api/v1/test \
  -H "Authorization: Bearer <TOKEN_FROM_LOGIN>"
```

**Expected Response (404 Not Found):**
```json
{
  "code": "NOT_FOUND",
  "message": "route not found",
  "data": null
}
```

**Status Check:** ✅ If you see HTTP 404 (means auth passed, endpoint just doesn't exist yet)

This verifies:
- Auth middleware working
- Route /api/v1 structure ready for Phase 5

---

## 🧪 Test 8: Internal v1 Endpoint

**Command:**
```bash
curl -i http://localhost:8081/internal/v1/test
```

**Expected Response (404 Not Found):**
```json
{
  "code": "NOT_FOUND",
  "message": "route not found",
  "data": null
}
```

**Status Check:** ✅ If you see HTTP 404

This verifies /internal/v1 route group is created (just no routes yet).

---

## 📊 Test Results Checklist

- [ ] **Test 1** - Health Live: HTTP 200, status ok
- [ ] **Test 2** - Health Ready: HTTP 200, postgres ok
- [ ] **Test 3** - Auth Login: HTTP 200 or 401
- [ ] **Test 4** - Auth Me (no token): HTTP 401
- [ ] **Test 5** - Auth Me (with token): HTTP 200 with user data
- [ ] **Test 6** - /api/v1 (no token): HTTP 401
- [ ] **Test 7** - /api/v1 (with token): HTTP 404
- [ ] **Test 8** - /internal/v1: HTTP 404

---

## ✅ Phase 4 Verification Complete

If ALL 8 tests pass as expected:

✅ Health readiness checks working  
✅ Auth middleware working  
✅ RBAC middleware protecting /api/v1  
✅ Route structure ready for Phase 5  
✅ Redis dependency installed  
✅ All containers healthy  

**Phase 4 is PRODUCTION READY** 🚀

---

## 🐛 Troubleshooting

### Error: Connection refused

**Cause:** Backend not running or wrong port

**Solution:**
```bash
make api-run
# or if already running
make dev-ps  # Check if running
```

---

### Error: 500 Internal Server Error

**Cause:** Configuration or database issue

**Solution:**
1. Check logs: `make dev-logs`
2. Verify env file: `ls -la apps/api/.env`
3. Verify DB: `make dev-ps | grep postgres`
4. Restart: `make dev-restart`

---

### Error: 404 on /health/live

**Cause:** API not running on correct port

**Solution:**
```bash
grep HTTP_PORT apps/api/.env
# Should show 8081

# If different, update curl:
curl http://localhost:<YOUR_PORT>/health/live
```

---

### Error: auth token invalid

**Cause:** Wrong credentials or token expired

**Solution:**
1. Get fresh token from Test 3
2. Use immediately (token may have short TTL)
3. Check .env for AUTH_ACCESS_TOKEN_TTL

---

## 📞 Next Steps

After all tests pass:

1. **Commit Phase 4:**
   ```bash
   git add apps/api/internal/http/router/router.go
   git commit -m "Fix: unused internalV1 variable in router"
   ```

2. **Proceed to Phase 5:**
   - Product Catalog
   - Categories, Products, Variants
   - Admin CRUD API
   - Internal search API

---

## 📋 Test Report Template

Copy and fill this after running tests:

```
=== Phase 4 Backend Verification ===
Date: [TODAY]
Backend Status: [RUNNING/STOPPED]

Test Results:
[ ] Test 1 - Health Live: ✅/❌
[ ] Test 2 - Health Ready: ✅/❌  
[ ] Test 3 - Auth Login: ✅/❌
[ ] Test 4 - Auth Me (no token): ✅/❌
[ ] Test 5 - Auth Me (with token): ✅/❌
[ ] Test 6 - /api/v1 (no token): ✅/❌
[ ] Test 7 - /api/v1 (with token): ✅/❌
[ ] Test 8 - /internal/v1: ✅/❌

Overall Status: [PASS/FAIL]

Notes:
[Your notes here]
```

---

*Created: 2026-07-19*  
*For: Phase 4 Manual Backend Testing*
