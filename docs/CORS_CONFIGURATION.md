# CORS Configuration

## Overview

Backend API telah dikonfigurasi dengan CORS (Cross-Origin Resource Sharing) middleware untuk mengizinkan akses dari admin frontend dan aplikasi lainnya.

## Environment Variable

Tambahkan environment variable berikut ke dalam konfigurasi deployment Anda:

```bash
# CORS Allowed Origins (comma-separated list)
CORS_ALLOWED_ORIGINS=http://localhost:3000,http://localhost:3001,https://admin.yourdomain.com
```

## Default Configuration

Jika `CORS_ALLOWED_ORIGINS` tidak diset, default origins adalah:
- `http://localhost:3000` - Admin frontend development
- `http://localhost:3001` - Alternative development port

## Production Configuration

Untuk production, pastikan hanya origins yang valid yang diizinkan:

```bash
# Production example
CORS_ALLOWED_ORIGINS=https://admin.yourdomain.com,https://admin-staging.yourdomain.com
```

## CORS Headers

Middleware CORS mengatur header berikut:

- `Access-Control-Allow-Origin`: Origin yang diizinkan
- `Access-Control-Allow-Credentials`: `true` (untuk cookies/auth)
- `Access-Control-Allow-Headers`: Content-Type, Authorization, X-Request-ID, dll
- `Access-Control-Allow-Methods`: GET, POST, PUT, DELETE, PATCH, OPTIONS
- `Access-Control-Max-Age`: 86400 (24 jam cache untuk preflight)

## Preflight Requests

Middleware menangani OPTIONS preflight requests secara otomatis dengan response 204 No Content.

## Security Notes

1. **Jangan gunakan wildcard (`*`)** di production untuk security
2. **Gunakan HTTPS** di production
3. **List origins spesifik** yang benar-benar diperlukan
4. **Verifikasi origin** di setiap deployment environment

## Testing CORS

Untuk test CORS dari browser console:

```javascript
fetch('http://localhost:8081/api/v1/products', {
  method: 'GET',
  headers: {
    'Authorization': 'Bearer YOUR_TOKEN',
    'Content-Type': 'application/json'
  },
  credentials: 'include'
})
.then(res => res.json())
.then(data => console.log(data))
.catch(err => console.error(err));
```

## Troubleshooting

### Error: "No 'Access-Control-Allow-Origin' header"

**Solusi:**
1. Pastikan `CORS_ALLOWED_ORIGINS` sudah diset dengan origin yang benar
2. Cek origin di browser matches dengan yang di config (termasuk protocol dan port)
3. Restart backend service setelah mengubah env variable

### Error: "CORS policy blocked the request"

**Solusi:**
1. Cek browser console untuk detail error
2. Verifikasi format origin: `protocol://domain:port`
3. Pastikan tidak ada trailing slash di origin
4. Cek preflight response dengan browser DevTools Network tab

### Credentials Issue

Jika menggunakan cookies/authentication:
1. Set `credentials: 'include'` di fetch request
2. Backend sudah set `Access-Control-Allow-Credentials: true`
3. Origin harus spesifik, tidak boleh wildcard

## Examples

### Development (Docker Compose)

```yaml
# docker-compose.dev.yml
services:
  api:
    environment:
      - CORS_ALLOWED_ORIGINS=http://localhost:3000,http://localhost:3001
```

### Production (Kubernetes)

```yaml
# deployment.yaml
env:
  - name: CORS_ALLOWED_ORIGINS
    value: "https://admin.yourdomain.com"
```

### Multiple Environments

```bash
# .env.development
CORS_ALLOWED_ORIGINS=http://localhost:3000,http://localhost:3001

# .env.staging
CORS_ALLOWED_ORIGINS=https://admin-staging.yourdomain.com

# .env.production
CORS_ALLOWED_ORIGINS=https://admin.yourdomain.com
```