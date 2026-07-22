# Phase AD6 — Inbox Handover ✅

**Status:** COMPLETE  
**Tanggal:** 2026-07-22

## Overview

Phase AD6 menambahkan inbox operasional untuk percakapan WhatsApp yang dialihkan oleh asisten AI. Tim admin dapat memantau handover terbuka, mencari pelanggan, membaca riwayat percakapan, mengirim balasan manual, dan menandai handover sebagai selesai.

Frontend tidak menampilkan data percakapan contoh. Jika API inbox AI belum tersedia, halaman memberi status integrasi yang eksplisit agar tidak ada data operasional fiktif yang dapat disalahartikan.

## Deliverables

### 1. Inbox Handover (`/inbox`)

- Item navigasi **Inbox** pada sidebar dashboard.
- Daftar handover berstatus `open`.
- Informasi pelanggan, nomor WhatsApp, alasan handover, admin yang ditugaskan, pratinjau pesan, dan waktu aktivitas terakhir.
- Pencarian lokal berdasarkan nama pelanggan, nomor WhatsApp, atau pratinjau pesan.
- Tombol perbarui dan polling daftar percakapan setiap 30 detik.
- Empty state, loading state, dan error state yang jelas.

### 2. Detail Percakapan

- Riwayat pesan inbound dan outbound.
- Penanda visual netral untuk bubble pesan pelanggan dan balasan outbound.
- Identitas pelanggan, nomor WhatsApp, dan alasan handover pada header detail.
- Layout dua panel pada desktop dan susunan vertikal yang aman pada perangkat kecil.

### 3. Operasional Manual

- Form balasan manual dengan status pengiriman.
- Tombol untuk menyelesaikan handover.
- React Query melakukan invalidasi cache detail dan daftar setelah balasan atau penyelesaian berhasil.

### 4. Safety untuk Integrasi yang Belum Ada

- Error HTTP `404`, `405`, dan `501` dipetakan menjadi `InboxIntegrationUnavailableError`.
- Halaman menjelaskan bahwa inbox belum terhubung dan menyebut endpoint minimum yang diperlukan.
- Tidak ada mock conversation atau data handover palsu.

## API Contract

Inbox menggunakan `aiApi` pada konfigurasi admin dan mengharapkan API AI service yang memerlukan autentikasi admin.

| Feature | Method | Endpoint |
|---|---:|---|
| List handover | GET | `/api/v1/admin/inbox/conversations?status=open` |
| Detail percakapan | GET | `/api/v1/admin/inbox/conversations/:id` |
| Kirim balasan manual | POST | `/api/v1/admin/inbox/conversations/:id/messages` |
| Selesaikan handover | POST | `/api/v1/admin/inbox/conversations/:id/resolve` |

Response dapat berbentuk payload langsung atau dibungkus dalam properti `data`.

Kontrak data utama:

- `InboxConversation`: ID, identitas pelanggan, alasan handover, status, preview, waktu pesan terakhir, dan penugasan admin.
- `ConversationDetail`: data percakapan beserta array `messages`.
- `ConversationMessage`: arah pesan (`inbound` atau `outbound`), isi pesan, waktu kirim, dan pengirim.

## File Structure

```text
apps/admin/
├── app/(dashboard)/inbox/
│   └── page.tsx
├── components/layout/
│   └── sidebar.tsx
└── lib/
    └── inbox.ts

docs/admin/
└── PHASE_AD6_COMPLETE.md
```

## Design Compliance

Implementasi mengikuti `admin_design_guidelines.md`:

- Menggunakan latar putih/slate netral tanpa beige, gradient, atau glassmorphism.
- Border datar dan radius `rounded-md`/`rounded-lg`; tidak ada ghost card.
- Kontras teks, placeholder, status, dan pesan dipertahankan untuk keterbacaan operasional.
- Tidak menggunakan nested card, side-stripe, ilustrasi sketsa, atau animasi dekoratif.
- Daftar dan detail percakapan responsif: desktop dua panel, mobile bertumpuk dengan area riwayat yang dapat di-scroll.

## Dependency Follow-up

AI service perlu menyediakan dan melindungi empat endpoint inbox di atas sebelum handover nyata dapat diproses dari admin. Setelah endpoint tersedia, frontend yang telah dibuat akan mengonsumsi data aktual tanpa penggantian UI atau perubahan routing.

## Verification

Jalankan dari `apps/admin`:

```bash
npm run lint
npm run build
```

## Next Phase

Phase berikutnya adalah **AD7 — Dashboard Analytics & Audit Log**, untuk menyelesaikan dashboard metrik penjualan dan operasional AI.

---

**Phase AD6 Status: ✅ COMPLETE**