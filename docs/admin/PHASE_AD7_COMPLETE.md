# Phase AD7 Complete — Dashboard Analytics & Audit Log

## Status

Phase AD7 untuk frontend Admin selesai di `apps/admin`.

## Implementasi

### Dashboard analytics

Halaman dashboard (`apps/admin/app/(dashboard)/page.tsx`) kini memuat data pesanan aktual dari `GET /api/v1/orders` dan menyediakan:

- Metrik total penjualan dari pesanan dengan status `paid`, `processing`, `shipped`, atau `completed`.
- Metrik total pesanan dan pesanan yang dibuat hari ini.
- Grafik SVG penjualan dengan pilihan:
  - 7 hari terakhir
  - 6 bulan terakhir
- Peringkat produk terlaris berdasarkan nilai pendapatan dan jumlah unit.
- Pengambilan detail untuk maksimum 24 pesanan pendapatan terbaru melalui `GET /api/v1/orders/:id`, sehingga produk terlaris dihitung dari order item nyata.
- State loading, error, refresh manual, dan empty state yang eksplisit.

Utilitas transformasi dan agregasi data berada di:

- `apps/admin/lib/analytics.ts`

### Audit log

Halaman (`apps/admin/app/(dashboard)/audit-logs/page.tsx`) telah diperluas sebagai alat investigasi operasional:

- Pengambilan data dari `GET /api/v1/audit-logs?limit=100`.
- Filter resource di server dan filter lokal berdasarkan:
  - Kata kunci pelaku, aksi, resource ID, atau metadata.
  - Jenis aksi.
  - Rentang waktu: hari ini, 7 hari, atau 30 hari terakhir.
- Tabel desktop dengan horizontal overflow yang aman.
- Tampilan daftar ringkas khusus mobile.
- Dialog detail untuk metadata JSON lengkap, pelaku, resource, aksi, serta identitas log.
- State loading, error, empty state, dan refresh manual.

## Prinsip UI yang diterapkan

- Palet netral/slate dengan aksen sky terbatas.
- Card memakai flat border tanpa kombinasi drop-shadow besar.
- Radius maksimum `rounded-lg`.
- Kontras teks dipertahankan untuk konten operasional.
- Tidak ada gradient, glassmorphism, nested card, maupun side-stripe callout.
- Tabel data-heavy memiliki strategi desktop/mobile terpisah agar tetap terbaca pada layar kecil.

## Kontrak backend dan batasan

Dashboard saat ini mengagregasi endpoint order yang sudah tersedia di client, bukan endpoint analytics khusus. Akibatnya, peringkat produk hanya mengevaluasi 24 pesanan pendapatan terbaru agar jumlah request detail tetap terkendali.

Untuk skala data produksi yang lebih besar, backend sebaiknya menyediakan endpoint agregasi yang terautorisasi, misalnya:

- `GET /api/v1/analytics/dashboard?period=week|month`
- Metrik penjualan dan jumlah pesanan per interval.
- Produk terlaris langsung dari agregasi database.
- Pagination/filter tanggal server-side untuk audit log.

Implementasi kini tetap fungsional tanpa endpoint tambahan dan tidak memasukkan data contoh operasional.

## File utama

- `apps/admin/app/(dashboard)/page.tsx`
- `apps/admin/lib/analytics.ts`
- `apps/admin/app/(dashboard)/audit-logs/page.tsx`
- `docs/admin/admin_frontend_roadmap.plan.md`