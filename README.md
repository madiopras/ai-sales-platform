# AI Sales Platform

Enterprise AI Sales Platform dengan fondasi monorepo untuk API, dashboard admin, dan layanan AI.

## Struktur proyek

```text
ai-sales-platform/
├── apps/
│   ├── api/              # Backend API (Go)
│   ├── admin/            # Dashboard admin (Next.js)
│   └── ai/               # AI service (FastAPI)
├── packages/
│   └── shared/           # Kode dan kontrak lintas aplikasi
├── database/
│   ├── migration/        # Database migrations
│   ├── seed/             # Data seed
│   └── sql/              # Query atau skrip SQL terkelola
├── configs/              # Konfigurasi aplikasi bersama
├── docs/                 # Dokumentasi arsitektur dan operasional
└── infra/
    ├── compose/          # Docker Compose base, development, production
    ├── docker/           # Konfigurasi image tiap layanan infrastruktur
    ├── k8s/              # Manifest Kubernetes saat deployment diperlukan
    └── scripts/          # Skrip infrastruktur dan operasional
```

## Tech stack

- Go
- Next.js
- FastAPI
- PostgreSQL
- Redis
- RabbitMQ
- MinIO

## Menjalankan infrastructure development

Salin `.env.example` ke `.env.development`, kemudian isi seluruh variabel layanan inti.

```bash
make dev
```

Layanan development tersedia melalui port yang didefinisikan pada `.env.development`: PostgreSQL, Redis, RabbitMQ, MinIO, Nginx, pgAdmin, Mailpit, Jaeger, Prometheus, dan Grafana.

Validasi gabungan konfigurasi tanpa menjalankan container:

```bash
make dev-config
```

## Backend framework (Phase 2)

API Go berada di `apps/api`. Salin `apps/api/.env.example` ke `apps/api/.env` bila memerlukan konfigurasi lokal, lalu jalankan:

```bash
make api-run
```

Endpoint awal tersedia di `GET /health/live` dan `GET /health/ready`. API menggunakan response envelope, request ID, logging Zap, recovery middleware, validasi, serta dependency container sebagai fondasi untuk domain pada fase berikutnya.
