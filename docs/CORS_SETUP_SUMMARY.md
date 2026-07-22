# CORS Configuration - Summary

✅ **CORS telah berhasil dikonfigurasi untuk backend API**

## File yang Dibuat/Diubah

### 1. Middleware CORS
**File:** `apps/api/internal/http/middleware/cors.go`
- Middleware untuk handle CORS headers
- Support preflight OPTIONS requests
- Whitelist based origins (secure)

### 2. Configuration
**File:** `apps/api/internal/config/config.go`
- Menambahkan `CORSConfig` struct
- Parse `CORS_ALLOWED_ORIGINS` dari environment
- Default: `http://localhost:3000,http://localhost:3001`

### 3. Router Integration
**File:** `apps/api/internal/http/router/router.go`
- Menambahkan `AllowedOrigins` ke `Deps`
- Apply CORS middleware secara global

**File:** `apps/api/internal/bootstrap/app.go`
- Pass `AllowedOrigins` dari config ke router

### 4. Dokumentasi
- `docs/CORS_CONFIGURATION.md` - Dokumentasi lengkap
- `docs/QUICK_START_CORS.md` - Quick start guide

## Cara Menggunakan

### Development (Local)

```bash
# Set environment variable
export CORS_ALLOWED_ORIGINS=http://localhost:3000,http://localhost:3001

# Start backend
cd apps/api
go run cmd/api/main.go
```

### Development (Docker)

Tambahkan ke `docker-compose` atau file environment:

```yaml
services:
  api:
    environment:
      - CORS_ALLOWED_ORIGINS=http://localhost:3000,http://localhost:3001
```

### Production

```bash
# Production environment
export CORS_ALLOWED_ORIGINS=https://admin.yourdomain.com
```

## Environment Variable

```bash
CORS_ALLOWED_ORIGINS=http://localhost:3000,http://localhost:3001
```

**Format:**
- Comma-separated list
- Include protocol (http/https)
- Include port jika bukan default (80/443)
- Tanpa trailing slash

**Examples:**
```bash
# Development
CORS_ALLOWED_ORIGINS=http://localhost:3000,http://localhost:3001

# Staging
CORS_ALLOWED_ORIGINS=https://admin-staging.yourdomain.com

# Production
CORS_ALLOWED_ORIGINS=https://admin.yourdomain.com

# Multiple production domains
CORS_ALLOWED_ORIGINS=https://admin.yourdomain.com,https://admin.otherdomain.com
```

## Test CORS

### Quick Test dari Browser Console

```javascript
fetch('http://localhost:8081/health/live', {
  method: 'GET',
  headers: { 'Content-Type': 'application/json' }
})
.then(res => console.log('Status:', res.status, 'CORS OK!'))
.catch(err => console.error('CORS Error:', err));
```

### Test dengan curl

```bash
# Test simple request
curl http://localhost:8081/health/live \
  -H "Origin: http://localhost:3000" \
  -v

# Test preflight
curl -X OPTIONS http://localhost:8081/api/v1/products \
  -H "Origin: http://localhost:3000" \
  -H "Access-Control-Request-Method: GET" \
  -H "Access-Control-Request-Headers: Authorization" \
  -v
```

## CORS Headers yang Diset

```
Access-Control-Allow-Origin: <requested-origin>
Access-Control-Allow-Credentials: true
Access-Control-Allow-Methods: POST, OPTIONS, GET, PUT, DELETE, PATCH
Access-Control-Allow-Headers: Content-Type, Authorization, X-Request-ID, ...
Access-Control-Max-Age: 86400
```

## Admin Frontend Setup

Pastikan admin frontend menggunakan:

```typescript
// lib/api.ts
const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8081';

export async function apiRequest(endpoint: string, options: RequestInit = {}) {
  const response = await fetch(`${API_BASE_URL}${endpoint}`, {
    ...options,
    credentials: 'include', // Important untuk auth!
    headers: {
      'Content-Type': 'application/json',
      ...options.headers,
    },
  });
  return response;
}
```

## Troubleshooting

### CORS Error masih muncul?

1. **Cek environment variable sudah diset:**
   ```bash
   echo $CORS_ALLOWED_ORIGINS
   ```

2. **Restart backend service:**
   ```bash
   # Kill dan restart
   pkill -f "go run cmd/api/main.go"
   go run cmd/api/main.go
   ```

3. **Verifikasi origin match persis:**
   - Browser origin: `http://localhost:3000` (cek di DevTools)
   - Config: `http://localhost:3000` (tanpa trailing slash)
   - Protocol harus sama (http vs https)
   - Port harus sama jika ada

4. **Cek browser DevTools Network tab:**
   - Lihat preflight OPTIONS request
   - Cek response headers
   - Cek status code (should be 204 for OPTIONS)

### Credentials Issue?

Pastikan:
- Frontend: `credentials: 'include'` di fetch
- Backend: CORS middleware sudah set `Access-Control-Allow-Credentials: true` ✅
- Origin harus spesifik (tidak boleh wildcard)

## Security Best Practices

✅ **DO:**
- Gunakan whitelist origins yang spesifik
- Gunakan HTTPS di production
- Review allowed origins secara berkala
- Set minimal origins yang diperlukan

❌ **DON'T:**
- Jangan gunakan wildcard `*` di production
- Jangan allow `http://` di production
- Jangan allow origins yang tidak dikenal
- Jangan commit credentials ke git

## Next Steps

1. ✅ CORS middleware created
2. ✅ Configuration added
3. ✅ Router integrated
4. ✅ Documentation written
5. ⏳ Set environment variable untuk environment Anda
6. ⏳ Test dari admin frontend
7. ⏳ Deploy ke staging/production
8. ⏳ Monitor CORS errors

## Resources

- [CORS Configuration](./CORS_CONFIGURATION.md) - Full documentation
- [Quick Start Guide](./QUICK_START_CORS.md) - Step by step guide
- [MDN CORS](https://developer.mozilla.org/en-US/docs/Web/HTTP/CORS) - CORS reference

---

**Status:** ✅ Ready to use

**Configured by:** Kiro AI

**Date:** 2026-07-21