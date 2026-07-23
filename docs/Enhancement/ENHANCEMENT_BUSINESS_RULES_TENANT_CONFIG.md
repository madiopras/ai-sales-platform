# Enhancement Business Rules — Tenant Business Configuration

> Dokumen ini mendefinisikan business rule enhancement untuk AI Sales Platform agar setiap akun/dashboard dapat mengonfigurasi bisnisnya sendiri, memilih 1 channel WhatsApp, memilih 1 provider LLM, menyimpan credential integrasi, dan menentukan strategi payment yang paling tepat.
>
> Dokumen ini melanjutkan business rules existing di `docs/ai/business-rules.md` dan roadmap yang sebelumnya masih bersifat **single-tenant/global configuration**. Enhancement ini mengarah ke **account-level / tenant-level configuration**.

---

## 1. Tujuan Enhancement

### 1.1 Objective

AI Sales Platform harus mendukung konfigurasi bisnis per akun/dashboard sehingga:

1. Setiap akun memiliki konfigurasi bisnis sendiri.
2. Setiap akun hanya boleh menautkan **1 channel WhatsApp aktif**.
3. Setiap akun dapat memilih salah satu sumber koneksi WhatsApp:
   - WhatsApp Business API resmi / Meta Cloud API, atau
   - Third-party WhatsApp gateway.
4. Setiap akun dapat memilih salah satu provider LLM:
   - Anthropic, atau
   - OpenAI.
5. Setiap akun dapat menginput dan mengelola API key LLM miliknya sendiri.
6. Setiap akun dapat menginput dan mengelola credential Biteship miliknya sendiri.
7. Payment perlu ditentukan dengan strategi yang aman, scalable, dan sesuai kebutuhan multi-account.

### 1.2 Scope

Masuk scope dokumen ini:

- Business profile per akun.
- Channel WhatsApp per akun.
- Provider LLM per akun.
- Credential Biteship per akun.
- Payment recommendation.
- Security rule untuk secret/API key.
- Operational rule untuk validasi, aktivasi, deaktivasi, dan audit.
- Dampak terhadap AI, backend, dan dashboard.

Tidak masuk scope dokumen ini:

- Implementasi kode.
- Desain UI detail.
- Struktur migration final.
- Harga subscription SaaS.
- Legal agreement dengan provider pihak ketiga.

---

## 2. Definisi Entitas Bisnis

### 2.1 Account / Tenant

Account atau tenant adalah pemilik konfigurasi bisnis di platform.

Contoh:

- Toko A
- Brand Fashion B
- UMKM C
- Merchant D

Setiap akun memiliki data, konfigurasi, channel, credential, dan operasional sendiri.

### 2.2 Business Profile

Business Profile adalah informasi bisnis yang digunakan oleh AI dan dashboard.

Minimal data:

- Nama bisnis.
- Deskripsi bisnis.
- Industri/kategori bisnis.
- Alamat origin pengiriman.
- Nomor kontak bisnis.
- Jam operasional.
- Bahasa utama.
- Tone of voice AI.
- Kebijakan toko:
  - Retur.
  - Refund.
  - Pengiriman.
  - Garansi.
  - Wholesale/reseller.
- Status konfigurasi:
  - draft
  - active
  - suspended
  - incomplete

### 2.3 Channel

Channel adalah koneksi komunikasi yang digunakan customer untuk berinteraksi dengan AI Sales Agent.

Pada enhancement ini, channel yang diatur adalah WhatsApp.

### 2.4 Provider Credential

Provider Credential adalah secret/API key/token yang digunakan untuk mengakses layanan eksternal:

- WhatsApp provider.
- LLM provider.
- Biteship.
- Payment provider.

Semua credential wajib dianggap sebagai data sensitif.

---

## 3. Prinsip Utama Business Rule

### EBR-001 — Account Isolation

Setiap akun wajib terisolasi secara data dan konfigurasi.

Akun A tidak boleh:

- Melihat produk akun B.
- Menggunakan credential akun B.
- Menggunakan nomor WhatsApp akun B.
- Mengakses order, customer, cart, voucher, invoice, shipment, atau analytics milik akun B.

### EBR-002 — Tenant Context Required

Setiap proses bisnis yang terkait transaksi atau AI wajib memiliki konteks akun/tenant.

Contoh proses yang wajib punya tenant context:

- Inbound WhatsApp message.
- Product search.
- Cart.
- Checkout.
- Invoice.
- Shipping rate.
- Shipment.
- LLM response.
- Analytics event.
- Knowledge base retrieval.
- Admin dashboard request.

### EBR-003 — Configuration Before Activation

Akun tidak boleh menjalankan AI Sales Agent sebelum konfigurasi minimum lengkap.

Konfigurasi minimum:

1. Business Profile aktif.
2. 1 WhatsApp channel aktif.
3. 1 LLM provider aktif dan credential valid.
4. Biteship credential valid jika checkout/shipping diaktifkan.
5. Payment configuration valid jika pembayaran online diaktifkan.
6. Product catalog minimal tersedia jika mode sales aktif.

### EBR-004 — One Active WhatsApp Channel Per Account

Setiap akun hanya boleh memiliki **1 channel WhatsApp aktif** pada satu waktu.

Akun dapat menyimpan riwayat channel lama, tetapi hanya satu yang boleh berstatus `active`.

Status channel:

- draft
- pending_verification
- active
- inactive
- failed
- revoked

### EBR-005 — One Active Provider Per Integration Type

Untuk setiap jenis integrasi, satu akun hanya boleh memiliki 1 provider aktif.

Contoh:

- WhatsApp: hanya 1 provider aktif.
- LLM: hanya 1 provider aktif.
- Biteship: hanya 1 credential aktif.
- Payment: hanya 1 mode payment utama aktif, kecuali payment provider mendukung multi-method di dalam satu provider.

---

## 4. Business Profile Rules

### EBR-010 — Business Profile Ownership

Setiap Business Profile dimiliki oleh satu akun.

Business Profile tidak boleh shared antar akun.

### EBR-011 — Business Profile Required Fields

Business Profile wajib memiliki:

- business_name
- business_category
- contact_phone
- origin_address
- origin_city
- origin_district
- origin_postal_code
- timezone
- default_language
- operating_hours

Jika field wajib belum lengkap, status Business Profile adalah `incomplete`.

### EBR-012 — Business Profile Used by AI

AI wajib menggunakan Business Profile sebagai konteks jawaban.

AI boleh menggunakan informasi berikut untuk menyusun respons:

- Nama bisnis.
- Jam operasional.
- Kebijakan pengiriman.
- Kebijakan refund/retur.
- Tone of voice.
- FAQ yang dipublish.

AI tidak boleh mengarang informasi bisnis yang tidak ada di profile/knowledge base.

### EBR-013 — Business Profile Status

Hanya Business Profile dengan status `active` yang boleh digunakan untuk:

- Menjalankan WhatsApp AI.
- Melakukan checkout.
- Mengirim invoice.
- Membuat shipment.
- Menjalankan abandoned cart recovery.

Jika status berubah menjadi `suspended`, semua automation wajib dihentikan.

### EBR-014 — Business Profile Audit

Setiap perubahan Business Profile wajib dicatat dalam audit log.

Audit log minimal mencatat:

- actor_user_id
- account_id
- action
- before_value
- after_value
- timestamp
- IP/device metadata jika tersedia

---

## 5. WhatsApp Channel Rules

### EBR-020 — WhatsApp Provider Choice

User dapat memilih salah satu koneksi WhatsApp:

1. Meta WhatsApp Business Cloud API.
2. Third-party WhatsApp gateway.

Hanya satu pilihan yang dapat aktif per akun.

### EBR-021 — WhatsApp Provider Cannot Be Active Together

Jika akun sudah memiliki Meta WhatsApp Business API aktif, maka third-party provider tidak boleh diaktifkan bersamaan.

Jika akun ingin mengganti provider, user wajib melakukan proses switch provider.

### EBR-022 — Provider Switch Rule

Switch provider WhatsApp harus mengikuti flow:

1. User memilih provider baru.
2. User menginput credential provider baru.
3. Sistem melakukan validasi credential.
4. Jika valid, channel baru masuk status `pending_activation`.
5. User mengonfirmasi switch.
6. Sistem menonaktifkan provider lama.
7. Sistem mengaktifkan provider baru.
8. Audit log dibuat.

Sistem tidak boleh menonaktifkan provider lama sebelum provider baru valid, kecuali user secara eksplisit memilih disconnect.

### EBR-023 — WhatsApp Credential Validation

Sebelum channel WhatsApp aktif, sistem wajib memvalidasi credential.

Untuk Meta Cloud API, validasi minimal:

- access token valid.
- phone_number_id valid.
- webhook verify token tersedia.
- business account id jika dibutuhkan.
- permission scope cukup untuk send/receive message.

Untuk third-party gateway, validasi minimal:

- API key/token valid.
- sender/instance id valid.
- endpoint reachable.
- provider dapat mengirim test message atau health check.

### EBR-024 — WhatsApp Webhook Routing

Inbound webhook WhatsApp wajib dipetakan ke account yang benar berdasarkan salah satu identifier berikut:

- phone_number_id
- sender instance id
- webhook path token
- provider account id
- dedicated webhook secret

Jika sistem tidak dapat menentukan account, webhook harus ditolak atau diabaikan secara aman.

### EBR-025 — WhatsApp Number Uniqueness

Satu nomor WhatsApp tidak boleh aktif pada lebih dari satu akun.

Jika nomor sudah digunakan oleh akun lain, sistem harus menolak aktivasi dengan error:

`WHATSAPP_NUMBER_ALREADY_LINKED`

### EBR-026 — WhatsApp Channel Disconnect

User dapat disconnect WhatsApp channel.

Saat disconnect:

- status channel menjadi `inactive` atau `revoked`.
- AI tidak boleh membalas inbound message dari channel tersebut.
- outbound notification tidak boleh dikirim.
- existing conversation session dapat ditandai `paused`.
- audit log wajib dibuat.

### EBR-027 — WhatsApp Channel Health

Sistem wajib menyimpan status kesehatan channel.

Contoh status:

- healthy
- degraded
- invalid_credential
- webhook_failed
- rate_limited
- provider_down

Jika channel tidak sehat, dashboard wajib menampilkan peringatan.

### EBR-028 — WhatsApp Sending Rules

AI hanya boleh mengirim pesan dari channel WhatsApp milik akun terkait.

AI tidak boleh menggunakan fallback channel dari akun lain.

### EBR-029 — WhatsApp Provider Abstraction

AI service wajib memperlakukan WhatsApp provider melalui abstraction/interface.

Business rule tidak boleh bergantung pada detail provider tertentu.

---

## 6. LLM Provider Rules

### EBR-040 — LLM Provider Choice

User dapat memilih salah satu provider LLM:

1. Anthropic.
2. OpenAI.

Hanya satu provider LLM aktif per akun.

### EBR-041 — LLM Credential Required

Akun wajib menginput API key provider LLM sebelum AI dapat aktif.

Jika API key belum tersedia atau invalid:

- AI tidak boleh menjawab otomatis.
- Dashboard menampilkan status `LLM_NOT_CONFIGURED`.
- Inbound message dapat diarahkan ke handover/admin fallback jika channel aktif.

### EBR-042 — LLM Credential Validation

Saat user menyimpan API key, sistem wajib melakukan validasi.

Validasi minimal:

- API key tidak kosong.
- Format dasar sesuai provider.
- Test request ringan ke provider berhasil.
- Model yang dipilih tersedia/diizinkan.
- API key tidak expired/revoked.

Jika validasi gagal, credential tidak boleh diaktifkan.

### EBR-043 — LLM Model Selection

Setiap provider dapat memiliki daftar model yang didukung.

Contoh:

Anthropic:

- claude-3-5-sonnet
- claude-3-5-haiku
- claude-3-opus jika tersedia

OpenAI:

- gpt-4o
- gpt-4o-mini
- gpt-4.1 jika tersedia

Platform harus mengizinkan model list dikontrol oleh sistem agar tidak semua model provider otomatis tersedia.

### EBR-044 — One Active LLM Config

Setiap akun hanya boleh memiliki satu LLM config aktif.

LLM config mencakup:

- provider
- model
- encrypted_api_key
- max_tokens
- temperature
- status
- last_validated_at

### EBR-045 — LLM Fallback Policy

Jika provider LLM akun gagal:

- Sistem tidak boleh otomatis memakai API key global tanpa izin eksplisit.
- Sistem tidak boleh memakai API key akun lain.
- Sistem boleh:
  - retry sesuai policy,
  - kirim pesan fallback,
  - handover ke admin,
  - menandai status LLM degraded.

### EBR-046 — LLM Cost Ownership

Biaya LLM yang menggunakan API key akun menjadi tanggung jawab akun tersebut.

Dashboard sebaiknya menampilkan:

- provider aktif.
- model aktif.
- estimasi pemakaian token.
- jumlah percakapan.
- error rate.
- last validation.

### EBR-047 — LLM Safety Rules Still Apply

Walaupun user menggunakan API key sendiri, semua guardrail platform tetap wajib berlaku:

- Tidak bocorkan secret.
- Tidak bocorkan system prompt.
- Tidak mengarang harga/stok/ongkir.
- Tetap menggunakan backend sebagai source of truth.
- Tetap mematuhi handover rules.
- Tetap memblok prompt injection.

### EBR-048 — LLM Credential Rotation

User dapat mengganti API key.

Saat API key diganti:

- key lama dinonaktifkan.
- key baru divalidasi.
- audit log dibuat.
- active session boleh lanjut menggunakan config baru pada request berikutnya.

---

## 7. Biteship Credential Rules

### EBR-060 — Biteship Per Account

Setiap akun dapat menginput credential Biteship sendiri.

Credential Biteship akun digunakan untuk:

- shipping rate.
- shipment booking.
- tracking integration jika applicable.
- webhook validation jika provider mendukung account-specific token.

### EBR-061 — Biteship Credential Required for Shipping

Jika akun mengaktifkan fitur shipping otomatis, credential Biteship wajib valid.

Jika credential tidak valid:

- AI tidak boleh memberikan ongkir final.
- checkout tidak boleh dilanjutkan dengan shipping otomatis.
- AI harus meminta retry atau handover ke admin.

### EBR-062 — Biteship Origin Address

Setiap akun wajib memiliki origin address untuk rate dan shipment.

Origin address minimal:

- shipper name
- shipper phone
- origin address line
- origin city
- origin district
- origin postal code

Origin address harus berasal dari Business Profile atau Shipping Setting akun.

### EBR-063 — Biteship Credential Validation

Sistem wajib memvalidasi Biteship API key sebelum aktif.

Validasi minimal:

- API key diterima Biteship.
- rate endpoint dapat dipanggil dengan test payload.
- credential memiliki permission yang cukup.
- account tidak suspended di Biteship.

### EBR-064 — Biteship Failure Handling

Jika Biteship gagal:

- AI tidak boleh mengarang ongkir.
- AI harus menyampaikan bahwa ongkir belum dapat dihitung.
- Sistem dapat menawarkan:
  - coba lagi,
  - pilih pengiriman manual,
  - hubungkan ke admin.

### EBR-065 — Biteship Webhook Tenant Routing

Webhook Biteship harus dipetakan ke akun yang benar berdasarkan:

- biteship_order_id
- shipment_id
- order_id internal
- account_id mapping di shipment record

Jika mapping tidak ditemukan, webhook boleh diabaikan dengan response aman agar tidak terjadi retry storm.

### EBR-066 — Manual Shipping Fallback

Akun dapat mengaktifkan manual shipping fallback.

Jika aktif:

- admin dapat input ongkir manual.
- AI harus menyebut bahwa ongkir dikonfirmasi manual oleh admin.
- checkout dapat dilanjutkan hanya setelah customer menyetujui ongkir manual.

---

## 8. Payment Recommendation & Rules

### 8.1 Rekomendasi Payment

Untuk AI Sales Platform multi-account, pilihan payment terbaik bergantung pada model bisnis yang diinginkan.

Ada 3 opsi utama:

---

### Option A — Platform-Owned Payment Provider

Platform menggunakan satu payment provider utama milik platform, misalnya Xendit account milik platform.

Merchant/akun menerima settlement melalui mekanisme internal platform.

#### Kelebihan

- Onboarding merchant lebih mudah.
- Merchant tidak perlu membuat akun Xendit sendiri.
- Platform dapat mengontrol payment flow.
- Lebih mudah membuat standard invoice/payment page.
- Cocok jika platform ingin menjadi SaaS + payment facilitator.

#### Kekurangan

- Kompleks secara legal dan settlement.
- Platform bertanggung jawab atas dana merchant.
- Perlu rekonsiliasi payout.
- Potensi kebutuhan izin/regulasi lebih tinggi.
- Risiko fraud/chargeback berada di platform.

#### Cocok Jika

- Platform ingin mengelola seluruh pembayaran.
- Merchant kecil tidak mau setup payment gateway sendiri.
- Platform siap menangani settlement dan finance operation.

---

### Option B — Merchant-Owned Payment Credential

Setiap akun menginput credential payment provider sendiri, misalnya Xendit secret key masing-masing merchant.

#### Kelebihan

- Dana langsung masuk ke akun merchant.
- Tanggung jawab payment ada pada merchant.
- Platform tidak memegang dana.
- Lebih sederhana dari sisi settlement.
- Cocok untuk SaaS tool.

#### Kekurangan

- Onboarding lebih berat.
- Merchant harus punya akun payment provider.
- User harus menginput dan menjaga credential.
- Support lebih kompleks karena setiap credential bisa berbeda status.

#### Cocok Jika

- Platform berperan sebagai software/SaaS, bukan payment facilitator.
- Merchant sudah punya payment gateway.
- Ingin mengurangi risiko legal/settlement platform.

---

### Option C — Hybrid Payment Strategy

Platform mendukung dua mode:

1. Platform-managed payment untuk merchant yang belum punya gateway.
2. Merchant-owned payment untuk merchant yang ingin memakai akun sendiri.

#### Kelebihan

- Fleksibel.
- Bisa melayani UMKM pemula dan merchant yang sudah mature.
- Platform bisa mulai dari model sederhana lalu berkembang.
- Bisa menjadi monetization channel tambahan.

#### Kekurangan

- Business rule lebih kompleks.
- UI config lebih kompleks.
- Reconciliation dan reporting harus jelas.
- Harus ada pemisahan mode yang tegas.

#### Cocok Jika

- Platform ingin tumbuh jangka panjang.
- Target user bervariasi.
- Ingin mulai cepat tapi tetap scalable.

---

### 8.2 Rekomendasi Utama

Untuk tahap enhancement ini, rekomendasi terbaik adalah:

## Gunakan Option B terlebih dahulu: Merchant-Owned Payment Credential

Alasan:

1. Sesuai pola enhancement yang user inginkan:
   - user input LLM API key sendiri,
   - user input Biteship API key sendiri,
   - maka payment juga konsisten jika user input payment credential sendiri.
2. Risiko platform lebih rendah karena tidak memegang dana merchant.
3. Lebih cocok untuk SaaS dashboard.
4. Lebih mudah dari sisi akuntansi awal.
5. Settlement langsung dari payment gateway ke merchant.

Namun, platform sebaiknya menyiapkan struktur agar nanti bisa berkembang ke Option C / hybrid.

---

### 8.3 Payment Provider Recommendation

Provider yang disarankan:

## Xendit tetap direkomendasikan sebagai payment provider utama

Alasan:

- Sudah ada implementasi existing Phase 8 menggunakan Xendit.
- Mendukung invoice/payment link.
- Mendukung VA, QRIS, e-wallet, retail outlet, dan payment methods Indonesia.
- Webhook sudah dipahami dalam business rules existing.
- Cocok untuk checkout via WhatsApp karena payment link mudah dikirim AI.
- Dokumentasi dan API relatif matang.

Alternative yang bisa dipertimbangkan nanti:

- Midtrans
- Duitku
- Tripay
- DOKU
- Payment manual transfer bank

Untuk saat ini jangan mengganti Xendit sebelum ada alasan kuat, karena Xendit sudah menjadi bagian dari arsitektur existing.

---

## 9. Payment Business Rules

### EBR-080 — Payment Mode Per Account

Setiap akun wajib memiliki payment mode.

Payment mode:

- disabled
- manual_transfer
- xendit_merchant_owned
- platform_managed_xendit
- hybrid

Untuk tahap awal enhancement, gunakan:

`xendit_merchant_owned`

atau jika belum siap:

`manual_transfer`

### EBR-081 — Payment Credential Required

Jika akun memilih `xendit_merchant_owned`, akun wajib menginput:

- Xendit secret key.
- Xendit callback token.
- success redirect URL jika digunakan.
- failure redirect URL jika digunakan.
- invoice duration.

Credential wajib divalidasi sebelum aktif.

### EBR-082 — Payment Credential Validation

Sistem wajib memvalidasi credential payment sebelum aktif.

Validasi minimal:

- Secret key valid.
- Dapat melakukan test call ke Xendit.
- Callback token tersedia.
- Invoice duration valid.
- Provider account tidak suspended.

### EBR-083 — Payment Link Uses Account Credential

Invoice untuk order akun tertentu wajib dibuat menggunakan credential payment akun tersebut.

Sistem tidak boleh membuat invoice akun A menggunakan credential akun B.

### EBR-084 — Payment Webhook Routing

Webhook payment harus dipetakan ke akun yang benar.

Mapping dapat menggunakan:

- xendit_invoice_id
- external_id
- invoice_id internal
- order_id internal
- account_id pada invoice record

Jika webhook tidak bisa dipetakan, sistem harus mengabaikan secara aman dan mencatat event.

### EBR-085 — Payment Callback Verification

Callback Xendit wajib diverifikasi menggunakan callback token milik akun terkait.

Jika account mapping belum diketahui sebelum validasi, sistem harus:

1. Cari invoice berdasarkan external_id/id.
2. Ambil account_id dari invoice.
3. Ambil callback token account tersebut.
4. Verifikasi callback token.
5. Proses event jika valid.

### EBR-086 — Manual Transfer Mode

Jika akun memilih manual transfer:

- Admin wajib menginput rekening tujuan.
- AI dapat mengirim instruksi transfer.
- Order tidak boleh otomatis paid.
- Admin wajib melakukan verifikasi pembayaran manual.
- Audit log wajib mencatat siapa yang menandai pembayaran sebagai paid.

### EBR-087 — Payment Method Display

AI hanya boleh menampilkan payment method yang aktif pada akun tersebut.

Contoh:

- Jika Xendit aktif: tampilkan payment link.
- Jika manual transfer aktif: tampilkan rekening.
- Jika payment disabled: AI tidak boleh melakukan checkout berbayar.

### EBR-088 — Payment Failure Handling

Jika pembuatan invoice gagal:

- AI tidak boleh menyatakan invoice berhasil.
- AI harus meminta customer menunggu atau menghubungkan ke admin.
- Dashboard harus mencatat error provider.
- Retry boleh dilakukan sesuai policy.

### EBR-089 — Payment Audit

Semua perubahan payment setting dan credential wajib dicatat dalam audit log.

---

## 10. Dashboard Configuration Rules

### EBR-100 — Dashboard Business Settings Menu

Dashboard wajib memiliki area konfigurasi bisnis.

Minimum menu:

1. Business Profile.
2. WhatsApp Channel.
3. AI / LLM Settings.
4. Shipping / Biteship Settings.
5. Payment Settings.
6. Security & Credential Status.
7. Audit Logs.

### EBR-101 — Setup Checklist

Dashboard wajib menampilkan setup checklist.

Contoh checklist:

- Business profile complete.
- WhatsApp connected.
- LLM provider connected.
- Biteship connected.
- Payment connected.
- Product catalog ready.
- AI agent active.

AI Agent hanya boleh aktif jika checklist minimum terpenuhi.

### EBR-102 — Credential Display Rule

Credential/API key tidak boleh ditampilkan secara penuh setelah disimpan.

Dashboard hanya boleh menampilkan:

- masked value, contoh: `sk-...abcd`
- provider name
- status
- last validated time
- last updated by
- rotate/revoke button

### EBR-103 — Test Connection Button

Setiap integration setting wajib menyediakan test connection.

Test connection:

- tidak menyimpan credential jika validasi gagal.
- mencatat hasil test.
- menampilkan error yang aman.
- tidak menampilkan raw response yang mengandung secret.

### EBR-104 — Activation Toggle

Setiap integration dapat memiliki toggle aktif/nonaktif.

Namun toggle aktif hanya boleh tersedia jika credential valid.

### EBR-105 — Prevent Dangerous Deactivation

Jika user ingin menonaktifkan integrasi yang sedang digunakan, sistem wajib memberi warning.

Contoh:

- Menonaktifkan WhatsApp akan menghentikan auto-reply.
- Menonaktifkan LLM akan menghentikan AI.
- Menonaktifkan Biteship akan menghentikan ongkir otomatis.
- Menonaktifkan payment akan membuat checkout online tidak tersedia.

### EBR-106 — Role Permission

Hanya role tertentu yang boleh mengubah setting integrasi.

Rekomendasi permission:

| Action | Owner | Admin | Manager | Sales/CS | Viewer |
|---|---:|---:|---:|---:|---:|
| View business profile | Yes | Yes | Yes | Yes | Yes |
| Edit business profile | Yes | Yes | Yes | No | No |
| View integration status | Yes | Yes | Yes | No | Yes |
| Edit credentials | Yes | Yes | No | No | No |
| Rotate/revoke credentials | Yes | Yes | No | No | No |
| Activate/deactivate AI | Yes | Yes | Manager optional | No | No |
| View audit logs | Yes | Yes | Manager optional | No | No |

---

## 11. Credential Security Rules

### EBR-120 — Encryption at Rest

Semua credential wajib dienkripsi saat disimpan.

Credential yang wajib dienkripsi:

- WhatsApp access token.
- Third-party WhatsApp API key.
- LLM API key.
- Biteship API key.
- Xendit secret key.
- Xendit callback token.
- Webhook secret/token.

### EBR-121 — No Secret in Logs

Sistem tidak boleh mencatat secret ke log.

Jika error provider mengandung secret, sistem wajib melakukan redaction.

### EBR-122 — No Secret in API Response

API dashboard tidak boleh mengembalikan raw secret.

Response hanya boleh berisi:

- masked_secret
- provider
- status
- created_at
- updated_at
- last_validated_at

### EBR-123 — Credential Rotation

User dapat rotate credential.

Rotation flow:

1. Input credential baru.
2. Validate credential baru.
3. Jika valid, aktifkan credential baru.
4. Revoke/deactivate credential lama.
5. Audit log dibuat.

### EBR-124 — Credential Revocation

Jika credential direvoke:

- Integrasi terkait menjadi inactive.
- Automation terkait berhenti.
- Dashboard menampilkan warning.
- Audit log dibuat.

### EBR-125 — Access Control for Credential Management

Credential hanya boleh dikelola oleh role:

- owner
- admin

Role lain tidak boleh melihat, mengubah, atau menghapus credential.

---

## 12. AI Runtime Rules in Multi-Account Mode

### EBR-140 — Runtime Config Resolution

Setiap kali AI menerima pesan inbound, AI wajib resolve:

1. account_id dari WhatsApp channel.
2. business profile.
3. active LLM config.
4. active knowledge base.
5. backend tenant context.
6. active integration config.

Jika account tidak ditemukan, message tidak boleh diproses.

### EBR-141 — AI Uses Account-Specific LLM

AI wajib menggunakan provider/model/API key milik akun terkait.

AI tidak boleh menggunakan global LLM key kecuali mode fallback global diaktifkan secara eksplisit oleh platform dan disetujui dalam policy.

### EBR-142 — AI Uses Account-Specific Knowledge Base

AI hanya boleh menggunakan knowledge base milik akun tersebut.

AI tidak boleh mengambil FAQ/kebijakan dari akun lain.

### EBR-143 — AI Uses Account-Specific Catalog

AI hanya boleh menawarkan produk milik akun tersebut.

### EBR-144 — AI Uses Account-Specific Shipping

AI hanya boleh menghitung ongkir menggunakan Biteship credential dan origin address milik akun tersebut.

### EBR-145 — AI Uses Account-Specific Payment

AI hanya boleh membuat invoice menggunakan payment setting akun tersebut.

### EBR-146 — Missing Config Handling

Jika konfigurasi akun belum lengkap:

- AI tidak boleh menjalankan sales flow penuh.
- AI dapat mengirim pesan fallback yang aman.
- AI dapat mengarahkan ke admin/handover.
- Dashboard harus menunjukkan konfigurasi yang belum lengkap.

---

## 13. Data Model Recommendation

> Ini bukan implementasi final, tetapi rekomendasi entitas agar business rules mudah diterapkan.

### 13.1 Core Tenant Tables

Rekomendasi tabel:

- accounts
- account_users
- business_profiles
- integration_configs
- integration_credentials
- whatsapp_channels
- llm_configs
- shipping_configs
- payment_configs
- audit_logs

### 13.2 Tenant-Aware Existing Tables

Existing domain yang perlu menjadi tenant-aware:

- categories
- products
- product_variants
- customers
- customer_addresses
- carts
- cart_items
- orders
- order_items
- invoices
- payment_events
- shipments
- shipment_events
- vouchers
- audit_logs
- knowledge_base
- analytics_events
- conversation_sessions

Setiap tabel bisnis tersebut sebaiknya memiliki `account_id`.

### 13.3 Unique Constraint Recommendation

Contoh constraint:

- `whatsapp_channels.phone_number` unique ketika status active.
- `llm_configs(account_id, status='active')` hanya satu.
- `shipping_configs(account_id, status='active')` hanya satu.
- `payment_configs(account_id, status='active')` hanya satu.
- `products(account_id, slug)` unique.
- `product_variants(account_id, sku)` unique.
- `vouchers(account_id, code)` unique.

---

## 14. API Behavior Recommendation

### 14.1 Admin API

Admin API `/api/v1` wajib resolve account context dari JWT/session.

Request dashboard tidak boleh mengirim account_id sembarangan kecuali user memiliki akses ke account tersebut.

### 14.2 Internal AI API

Internal API `/internal/v1` wajib menerima account context dari AI service.

Pilihan mekanisme:

1. Header `X-Account-ID` + service token.
2. Service token scoped per account.
3. Internal signed request dengan account id.

Rekomendasi awal:

- gunakan service token global internal,
- tambahkan `X-Account-ID`,
- backend wajib validasi bahwa account aktif.

Untuk production enterprise:

- gunakan service token scoped per account atau signed internal JWT.

### 14.3 Webhook API

Webhook endpoint harus bisa resolve account.

Contoh:

- `/webhooks/whatsapp/:channel_id`
- `/webhooks/whatsapp/meta`
- `/webhooks/xendit`
- `/webhooks/biteship`

Webhook generic harus mencari account dari payload/reference data.

---

## 15. Status & Lifecycle

### 15.1 Account Lifecycle

Status account:

- pending_setup
- active
- suspended
- cancelled

Rule:

- pending_setup: dashboard bisa config, AI belum aktif.
- active: semua fitur sesuai config berjalan.
- suspended: AI dan automation berhenti.
- cancelled: semua channel inactive, data retained sesuai retention policy.

### 15.2 Integration Lifecycle

Status integration:

- draft
- validating
- active
- inactive
- invalid
- revoked
- failed

Rule:

- hanya `active` yang boleh digunakan runtime.
- `invalid` tidak boleh dipakai.
- `revoked` tidak boleh dipakai lagi.
- `failed` perlu test ulang.

---

## 16. Analytics Rules for Tenant Config

### EBR-160 — Analytics Per Account

Analytics wajib dipisahkan per account.

Metric seperti:

- total conversation
- conversion rate
- revenue
- AOV
- product performance
- handover rate
- abandoned cart
- LLM usage
- provider error rate

harus dihitung per account.

### EBR-161 — Integration Health Metrics

Dashboard sebaiknya menampilkan health metric:

- WhatsApp delivery success/failure.
- LLM latency/error/token usage.
- Biteship rate success/failure.
- Payment invoice success/failure.
- Webhook processing success/failure.

### EBR-162 — No Cross-Account Analytics Leak

Akun tidak boleh melihat analytics akun lain.

Super admin platform boleh melihat aggregated metric jika role dan policy mengizinkan.

---

## 17. Recommended Implementation Phases

### Phase E1 — Tenant Foundation

Deliverables:

- accounts
- account_users
- account_id di domain utama
- tenant-aware auth/RBAC
- tenant-aware audit log
- migration strategy dari single-tenant ke tenant-aware

### Phase E2 — Business Profile & Dashboard Settings

Deliverables:

- Business Profile CRUD.
- setup checklist.
- business config status.
- audit log.

### Phase E3 — WhatsApp Channel Configuration

Deliverables:

- WhatsApp channel config per account.
- Meta provider config.
- third-party provider abstraction.
- one active channel rule.
- webhook routing per channel.

### Phase E4 — LLM Provider Configuration

Deliverables:

- LLM config per account.
- Anthropic/OpenAI selection.
- API key encryption.
- validation/test connection.
- runtime config resolution in AI service.

### Phase E5 — Biteship Per Account

Deliverables:

- Biteship credential per account.
- origin address per account.
- shipping rate uses account credential.
- webhook mapping per account.

### Phase E6 — Payment Per Account

Deliverables:

- payment mode per account.
- Xendit merchant-owned credential.
- invoice uses account credential.
- webhook token resolution per invoice/account.
- manual transfer fallback.

### Phase E7 — Tenant-Aware AI Runtime

Deliverables:

- inbound WA resolve account.
- AI uses account-specific LLM, KB, catalog, shipping, payment.
- account-specific analytics.
- hard isolation tests.

---

## 18. Acceptance Criteria

Enhancement dianggap siap jika:

1. Setiap akun dapat membuat Business Profile.
2. Setiap akun hanya dapat mengaktifkan 1 WhatsApp channel.
3. Akun dapat memilih Meta WhatsApp API atau third-party gateway, tetapi tidak keduanya aktif.
4. Akun dapat memilih Anthropic atau OpenAI, tetapi tidak keduanya aktif.
5. API key LLM tersimpan terenkripsi dan tidak pernah muncul di response/log.
6. Akun dapat menginput Biteship API key sendiri.
7. Shipping rate menggunakan credential Biteship akun terkait.
8. Payment menggunakan configuration akun terkait.
9. AI runtime selalu resolve account sebelum memproses pesan.
10. Data catalog/order/customer/invoice/shipment terisolasi per account.
11. Audit log mencatat semua perubahan konfigurasi.
12. Dashboard menampilkan setup checklist dan status health setiap integrasi.
13. Semua webhook dapat dipetakan ke account yang benar.
14. Tidak ada cross-account data leak.
15. Existing single-tenant flow tetap dapat dimigrasikan ke default account.

---

## 19. Risk & Mitigation

### Risk 1 — Secret Leakage

Mitigation:

- encryption at rest.
- masked response.
- log redaction.
- restricted RBAC.
- audit log.

### Risk 2 — Cross-Account Data Leak

Mitigation:

- account_id wajib di semua query.
- repository/service tenant-aware.
- automated tests untuk tenant isolation.
- policy di middleware.

### Risk 3 — Webhook Misrouting

Mitigation:

- simpan mapping provider reference ke account.
- webhook idempotency per account.
- reject/ignore unknown mapping.

### Risk 4 — Provider Credential Invalid

Mitigation:

- test connection.
- periodic health check.
- dashboard warning.
- graceful degradation.

### Risk 5 — Payment Settlement Complexity

Mitigation:

- mulai dari merchant-owned payment credential.
- hindari platform-managed funds pada tahap awal.
- siapkan hybrid sebagai future roadmap.

---

## 20. Summary Decision

Keputusan business rule untuk enhancement:

1. Platform bergerak dari single-tenant menjadi account/tenant-aware.
2. Setiap akun memiliki konfigurasi bisnis sendiri.
3. Setiap akun hanya boleh memiliki 1 WhatsApp channel aktif.
4. WhatsApp provider dapat berupa Meta Cloud API atau third-party gateway.
5. Setiap akun hanya boleh memiliki 1 LLM provider aktif: Anthropic atau OpenAI.
6. API key LLM, Biteship, WhatsApp, dan Payment wajib encrypted, masked, audited.
7. Biteship credential digunakan per account.
8. Payment disarankan tetap menggunakan Xendit, tetapi dengan model merchant-owned credential terlebih dahulu.
9. AI runtime wajib resolve account dan menggunakan seluruh config milik account tersebut.
10. Semua domain bisnis existing perlu ditambahkan account isolation sebelum enhancement ini production-ready.

---

## 21. Appendix — Mapping ke Business Rules Existing

| Existing BR | Enhancement Impact |
|---|---|
| BR-001 Customer Entry Point | Entry point menjadi WhatsApp channel per account |
| BR-002 Product Inquiry | Product inquiry harus account-specific |
| BR-004 Active Product Only | Active product only per account |
| BR-010 Biteship Rate | Biteship credential per account |
| BR-016 Invoice | Invoice dibuat dengan payment config account |
| BR-022 Xendit Callback | Callback token per account/payment config |
| BR-035 Stock Management | Stock isolation per account |
| BR-041 Notification | Notification dikirim via WhatsApp channel account |
| BR-044 Security | Secret masking/encryption diperluas |
| BR-048 Admin Dashboard | Dashboard ditambah Business/Integration Settings |
| BR-049 Audit Log | Audit wajib untuk semua setting/credential |
| BR-050 Analytics | Analytics wajib per account |

---

**Document Status:** Draft v1  
**Created For:** AI Sales Platform Enhancement  
**Recommended Next Step:** Review business rules, lalu buat technical roadmap dan database design untuk tenant configuration.