# AI Sales Platform Business Rules

> Dokumen ini menjadi referensi utama AI Agent, Backend Service, dan
> Dashboard Admin untuk memahami proses bisnis platform.

AI Sales Platform - Business Rules

## 1. Customer Interaction

### BR-001 - Customer Entry Point

Customer dapat memulai percakapan melalui WhatsApp. AI akan menjadi
first-line customer service. AI harus mampu memahami bahasa Indonesia
formal maupun informal. AI harus mempertahankan context percakapan
selama sesi berlangsung.

### BR-002 - Product Inquiry

AI dapat menjawab:

Informasi produk Harga Promo Stok Varian Ongkir Cara pembayaran Status
pesanan

### BR-003 - Human Handover

AI wajib mengalihkan percakapan kepada Admin apabila:

Customer meminta berbicara dengan admin AI confidence score rendah
Pertanyaan di luar knowledge Komplain Refund Retur Negosiasi harga
khusus

## 2. Product Catalog

### BR-004

AI hanya boleh menawarkan produk yang:

Status Active Stock Available Masih dijual

### BR-005

Apabila stok habis maka AI harus:

Memberitahu stok kosong Menawarkan produk alternatif Menawarkan
notifikasi ketika stok tersedia

## 3. Shopping Cart

### BR-006

Customer dapat membuat cart melalui chat.

**Contoh**

Saya mau beli

2 Kaos Hitam XL 1 Celana Jeans 32

AI akan membuat cart.

### BR-007

Customer dapat:

tambah produk hapus produk ubah qty ubah varian

melalui chat.

## 4. Customer Information

### BR-008

Sebelum checkout AI wajib memastikan data berikut tersedia:

Nama Nomor HP Alamat Kota Kecamatan Kode Pos (opsional) Catatan

### BR-009

Apabila customer pernah berbelanja:

AI mengambil alamat terakhir.

AI bertanya:

Apakah masih menggunakan alamat sebelumnya?

## 5. Shipping (Biteship)

### BR-010

Setelah alamat lengkap tersedia:

AI meminta tarif ongkir ke Biteship.

### BR-011

AI menampilkan beberapa pilihan kurir.

Misal:

JNE REG Rp18.000

J&T Rp20.000

SiCepat Rp17.000

AnterAja Rp16.000

### BR-012

Customer wajib memilih salah satu layanan pengiriman sebelum checkout.

### BR-013

Apabila Biteship gagal memberikan tarif:

AI akan:

meminta customer mencoba kembali atau menghubungkan ke admin

## 6. Checkout

### BR-014

Checkout hanya dapat dilakukan apabila:

-   Cart tidak kosong

-   Alamat lengkap

-   Kurir dipilih

-   Ongkir tersedia

### BR-015

Sebelum membuat order AI wajib mengirim ringkasan.

**Contoh**

Produk

2 Kaos Rp200.000

Ongkir Rp18.000

Total Rp218.000

AI meminta konfirmasi.

## 7. Payment (Xendit)

### BR-016

Setelah customer konfirmasi:

System membuat Invoice.

Status:

`Waiting Payment`

### BR-017

Invoice memiliki:

Nomor Invoice Expired Time Total Payment Channel

### BR-018

Customer memilih metode pembayaran.

Misal:

Virtual Account QRIS Transfer Bank E-Wallet

### BR-019

System meminta Payment URL ke Xendit.

### BR-020

AI mengirimkan:

Link pembayaran QRIS VA Number

### BR-021

Invoice mempunyai masa berlaku.

Misal:

24 Jam

Setelah expired:

Status

`Expired`

### BR-022

Xendit akan mengirim callback.

System melakukan verifikasi signature.

### BR-023

Jika pembayaran berhasil:

Status berubah menjadi:

`Paid`

### BR-024

AI otomatis mengirim pesan.

**Contoh**

Pembayaran berhasil.

Pesanan Anda sedang diproses.

## 8. Order Processing

### BR-025

Setelah status Paid.

Order otomatis masuk ke dashboard Admin.

### BR-026

Admin dapat mengubah status:

Diproses Dikemas Siap Dikirim Dikirim Selesai

### BR-027

Setiap perubahan status akan mengirim notifikasi WhatsApp.

## 9. Shipping Process

### BR-028

Saat order siap dikirim.

System membuat shipment ke Biteship.

### BR-029

Biteship mengembalikan:

Tracking Number Courier Label

### BR-030

Tracking Number disimpan pada Order.

### BR-031

AI mengirim:

Pesanan Anda telah dikirim.

No Resi:

JP123456789

Kurir: JNE REG

## 10. Order Tracking

### BR-032

Customer dapat bertanya.

Pesanan saya dimana?

AI mengambil status terbaru dari sistem.

### BR-033

Jika status berubah menjadi Delivered.

Order otomatis menjadi:

`Delivered`

### BR-034

Setelah Delivered selama X hari.

Order menjadi:

`Completed`

## 11. Stock Management

### BR-035

Setelah pembayaran berhasil.

Stock otomatis berkurang.

### BR-036

Jika pembayaran expired.

Stock dikembalikan.

## 12. Promotion

### BR-037

AI dapat menawarkan:

Voucher Diskon Bundling Cashback

berdasarkan rule bisnis.

### BR-038

Voucher hanya dapat digunakan apabila:

belum expired memenuhi minimum transaksi masih kuota tersedia

## 13. Customer History

### BR-039

AI dapat melihat:

histori order produk favorit transaksi terakhir

### BR-040

AI dapat memberikan rekomendasi produk berdasarkan histori pembelian.

## 14. Notification

### BR-041

WhatsApp notification dikirim untuk:

Invoice dibuat Pembayaran berhasil Pembayaran gagal Pesanan diproses
Pesanan dikirim Pesanan selesai

## 15. Security

### BR-042

Semua callback dari Xendit wajib diverifikasi menggunakan
Xendit callback token (header `x-callback-token`) dan/atau Signature
Validation sesuai jenis produk (Invoice / Payment Request API).

### BR-043

Semua callback dari Biteship wajib diverifikasi sesuai mekanisme
autentikasi yang disediakan.

### BR-044

AI tidak boleh menampilkan informasi sensitif seperti:

API Key Secret Key Token Password

## 16. AI Conversation Rules

### BR-045

AI tidak boleh melakukan checkout tanpa konfirmasi customer.

### BR-046

AI harus meminta konfirmasi ulang apabila terdapat perubahan:

alamat jumlah barang kurir total pembayaran

### BR-047

AI harus menjaga konteks percakapan hingga transaksi selesai atau sesi
berakhir.

## 17. Admin Dashboard Rules

### BR-048

Admin dapat mengelola:

Produk Kategori Harga Promo Voucher Pelanggan Pesanan Pembayaran
Pengiriman Knowledge Base AI

### BR-049

Semua aktivitas admin dicatat dalam Audit Log.

## 18. Analytics Rules

### BR-050

Platform mencatat metrik seperti:

Total Percakapan Conversion Rate Cart Abandonment Revenue Average Order
Value (AOV) Produk Terlaris Customer Repeat Rate Response Time AI Human
Handover Rate

## End-to-End Business Flow

Customer │ ▼ WhatsApp │ ▼ AI Sales Agent │ ├── Tanya Produk ├──
Rekomendasi Produk ├── Kelola Keranjang ├── Ambil Data Customer ▼
Checkout │ ▼ Biteship (Hitung Ongkir) │ ▼ Konfirmasi Pesanan │ ▼ Xendit
(Pembayaran) │ ▼ Webhook Payment │ ▼ Order Paid │ ▼ Admin Dashboard │ ▼
Biteship (Buat Pengiriman & Resi) │ ▼ WhatsApp Notification │ ▼ Customer
Menerima Barang

## Rekomendasi Pengembangan

Untuk platform AI Sales yang akan dikembangkan secara monorepo (sesuai
roadmap yang telah kita susun), saya juga menyarankan menambahkan
beberapa business rule tingkat enterprise, seperti:

Conversation Session Management (state machine percakapan dan timeout
sesi). Lead Management (membedakan prospek, pelanggan baru, pelanggan
aktif, dan pelanggan loyal). Reservation & Inventory Lock (stok
direservasi selama invoice masih aktif untuk mencegah overselling).
Campaign & Marketing Automation (broadcast terjadwal, abandoned cart
reminder, follow-up otomatis, dan upselling berbasis AI). Multi-Tenant
Rules (satu platform dapat digunakan banyak UMKM dengan katalog, nomor
WhatsApp, dan konfigurasi pembayaran masing-masing). Role & Permission
(RBAC) (Super Admin, Owner, Admin Toko, Customer Service, dan AI Agent
dengan hak akses yang berbeda). Knowledge Base Management (AI hanya
menggunakan data produk, FAQ, SOP, dan kebijakan yang telah
dipublikasikan oleh tenant sebagai sumber jawaban).
