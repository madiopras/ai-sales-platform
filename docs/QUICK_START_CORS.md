# Quick Start: CORS Setup untuk Admin Frontend

## Langkah 1: Konfigurasi Backend

### Development (Local)

Tambahkan environment variable di terminal atau `.env` file:

```bash
export CORS_ALLOWED_ORIGINS=http://localhost:3000,http://localhost:3001
```

Atau buat file `.env` di `apps/api/`:

```bash
# apps/api/.env
CORS_ALLOWED_ORIGINS=http://localhost:3000,http://localhost:3001
```

### Development (Docker)

Edit `infra/compose/docker-compose.dev.yml`:

```yaml
services:
  api:
    environment:
      - CORS_ALLOWED_ORIGINS=http://localhost:3000,http://localhost:3001
```

### Production

Sesuaikan dengan domain production Anda:

```bash
export CORS_ALLOWED_ORIGINS=https://admin.yourdomain.com
```

## Langkah 2: Restart Backend

### Local Development

```bash
cd apps/api
go run cmd/api/main.go
```

### Docker

```bash
cd infra/compose
docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml restart api
```

## Langkah 3: Verifikasi CORS

### Test dari Browser Console

Buka browser console di admin frontend (`http://localhost:3000`) dan jalankan:

```javascript
fetch('http://localhost:8081/health/live', {
  method: 'GET',
  headers: {
    'Content-Type': 'application/json'
  }
})
.then(res => {
  console.log('CORS working!', res.status);
  return res.json();
})
.then(data => console.log(data))
.catch(err => console.error('CORS error:', err));
```

### Expected Response

Jika CORS berhasil dikonfigurasi, Anda akan melihat:
- Status: 200 OK
- Response headers dengan `Access-Control-Allow-Origin`
- Tidak ada CORS error di console

### Response Headers yang Harus Ada

```
Access-Control-Allow-Origin: http://localhost:3000
Access-Control-Allow-Credentials: true
Access-Control-Allow-Methods: POST, OPTIONS, GET, PUT, DELETE, PATCH
Access-Control-Allow-Headers: Content-Type, Authorization, ...
```

## Langkah 4: Test dengan Admin Frontend

### Update API Base URL di Admin Frontend

File: `apps/admin/lib/api.ts`

```typescript
const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8081';
```

### Test Login

```bash
cd apps/admin
npm run dev
```

Buka browser ke `http://localhost:3000/login` dan coba login.

## Troubleshooting

### Problem: CORS error "No 'Access-Control-Allow-Origin' header"

**Cek:**
1. Backend service sudah running
2. Environment variable `CORS_ALLOWED_ORIGINS` sudah diset
3. Origin di browser matches dengan config (cek protocol, domain, dan port)

**Solusi:**
```bash
# Verifikasi config di backend
curl http://localhost:8081/health/live -H "Origin: http://localhost:3000" -v

# Cek response headers, harus ada Access-Control-Allow-Origin
```

### Problem: Preflight request failed

**Cek:**
```bash
# Test OPTIONS request
curl -X OPTIONS http://localhost:8081/api/v1/products \
  -H "Origin: http://localhost:3000" \
  -H "Access-Control-Request-Method: GET" \
  -H "Access-Control-Request-Headers: Authorization" \
  -v
```

**Expected:** Status 204, dengan CORS headers.

### Problem: Credentials not working

**Admin Frontend - ensure credentials are included:**

```typescript
// lib/api.ts
export async function apiRequest(endpoint: string, options: RequestInit = {}) {
  const response = await fetch(`${API_BASE_URL}${endpoint}`, {
    ...options,
    credentials: 'include', // Important!
    headers: {
      'Content-Type': 'application/json',
      ...options.headers,
    },
  });
  return response;
}
```

## Contoh Lengkap: API Request dari Admin

```typescript
// apps/admin/lib/api.ts
const API_BASE_URL = 'http://localhost:8081';

export async function login(username: string, password: string) {
  const response = await fetch(`${API_BASE_URL}/auth/login`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    credentials: 'include',
    body: JSON.stringify({ username, password }),
  });

  if (!response.ok) {
    throw new Error('Login failed');
  }

  return response.json();
}

export async function getProducts(token: string) {
  const response = await fetch(`${API_BASE_URL}/api/v1/products`, {
    method: 'GET',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${token}`,
    },
    credentials: 'include',
  });

  if (!response.ok) {
    throw new Error('Failed to fetch products');
  }

  return response.json();
}
```

## Production Checklist

- [ ] Set `CORS_ALLOWED_ORIGINS` dengan domain production (HTTPS)
- [ ] Jangan gunakan wildcard `*` di production
- [ ] Verifikasi HTTPS certificate valid
- [ ] Test dari production frontend
- [ ] Monitor CORS errors di production logs
- [ ] Document allowed origins untuk team

## Next Steps

1. Configure production origins
2. Setup environment-specific configs
3. Add monitoring untuk CORS errors
4. Update admin frontend untuk handle CORS properly

Untuk detail lebih lanjut, lihat: [CORS_CONFIGURATION.md](./CORS_CONFIGURATION.md)