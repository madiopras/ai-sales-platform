# Phase AD5 — Promo & Voucher Management ✅

**Status:** COMPLETE  
**Tanggal:** 2026-07-22

## Overview

Phase AD5 menyelesaikan manajemen campaign promo melalui voucher. Admin dapat melihat daftar voucher, mencari dan memfilter campaign, membuat voucher baru, memperbarui aturan voucher, serta mengaktifkan atau menonaktifkan voucher tanpa menghapus riwayat campaign.

## Deliverables

### 1. Voucher List (`/promos`)

- Tabel voucher responsif dengan horizontal scroll pada layar kecil.
- Kolom kode, tipe diskon, nilai diskon, minimum pembelian, kuota pemakaian, periode aktif, dan status.
- Pencarian instan berdasarkan kode voucher.
- Filter status untuk seluruh, aktif, dan nonaktif.
- Aksi edit per voucher.
- Tombol cepat untuk mengaktifkan atau menonaktifkan voucher.
- State loading, error, dan empty state yang jelas.

### 2. Create Voucher (`/promos/new`)

- Form pembuatan voucher dengan:
  - Kode voucher.
  - Tipe diskon persen atau nominal.
  - Nilai diskon.
  - Minimum pembelian.
  - Kuota penggunaan.
  - Tanggal mulai dan berakhir.
  - Status aktif awal.
- Validasi sisi klien untuk memastikan:
  - Kode wajib diisi.
  - Nilai diskon bernilai positif.
  - Diskon persen tidak melebihi 100%.
  - Minimum pembelian dan kuota tidak negatif.
  - Masa berlaku berakhir setelah masa mulai.
- Pesan error dari API ditampilkan pada form.

### 3. Edit Voucher (`/promos/[id]/edit`)

- Pengambilan data voucher berdasarkan ID.
- Form yang dapat digunakan kembali dari halaman create.
- Perubahan aturan diskon, kuota, periode, dan status.
- Kembali ke daftar promo setelah penyimpanan berhasil.
- Penanganan status voucher tidak ditemukan dan kegagalan pemuatan data.

### 4. Status Campaign

- Toggle aktif/nonaktif menggunakan endpoint update voucher.
- Cache React Query diinvalidate setelah create, update, atau perubahan status untuk menjaga daftar tetap sinkron.
- Status ditampilkan dengan badge kontras tinggi dan non-dekoratif.

## API Integration

| Feature | Method | Endpoint |
|---|---:|---|
| List voucher | GET | `/api/v1/vouchers` |
| Get voucher | GET | `/api/v1/vouchers/:id` |
| Create voucher | POST | `/api/v1/vouchers` |
| Update voucher / status | PUT | `/api/v1/vouchers/:id` |

Integrasi menggunakan Axios instance admin yang sudah menyematkan JWT, dengan React Query untuk caching, loading state, mutation state, dan refresh data setelah perubahan.

## File Structure

```text
apps/admin/
├── app/(dashboard)/promos/
│   ├── page.tsx
│   ├── new/page.tsx
│   └── [id]/edit/page.tsx
├── components/vouchers/
│   └── voucher-form.tsx
└── lib/
    └── vouchers.ts
```

## Design Compliance

Implementasi mengikuti `admin_design_guidelines.md`:

- Background putih/slate netral; tidak ada warm beige atau gradient.
- Card dan input memakai radius `rounded-md`/`rounded-lg` yang wajar.
- Tabel menggunakan border datar tanpa kombinasi ghost-card/shadow lebar.
- Kontras teks dan placeholder dipertahankan agar tetap terbaca.
- Tidak ada glassmorphism, nested card, side-stripe border, ilustrasi sketsa, atau animasi dekoratif.
- Desktop memakai tabel padat; layar kecil tetap aman melalui overflow horizontal.

## Additional AD7 Foundation

Halaman read-only `/audit-logs` juga telah ditambahkan lebih awal sebagai fondasi Phase AD7. Halaman ini memakai `GET /api/v1/audit-logs`, menyediakan filter resource type, pencarian aktivitas, dan tampilan metadata audit. Status Phase AD7 tetap **pending** karena dashboard analytics belum diimplementasikan.

## Verification

Jalankan dari `apps/admin`:

```bash
npm run lint
npm run build
```

## Next Phase

Phase berikutnya adalah **AD6 — Inbox Handover**, untuk memonitor percakapan yang diteruskan AI kepada tim operasional.

---

**Phase AD5 Status: ✅ COMPLETE**