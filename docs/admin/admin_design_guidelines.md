# Pedoman Desain Frontend Admin (Anti-AI Slop)

Dokumen ini mendefinisikan standar UI/UX untuk pengembangan Frontend Admin (`apps/admin`) menggunakan Next.js, Tailwind CSS, dan Shadcn UI. 

Berpedoman pada prinsip desain **Impeccable** dan **Hallmark** (agen desain anti-slop), pedoman ini dibuat untuk memastikan antarmuka admin (dashboard, tabel, form) terlihat berkelas, *production-grade*, tingkat keterbacaan tinggi, dan sepenuhnya menghindari *AI-slop* (pola desain generik, dekorasi berlebihan, dan cliché AI).

---

## 1. Filosofi Inti: Design Serves the Product
Admin Dashboard adalah sebuah alat (tool). Fungsi, densitas data, dan kejelasan hierarki informasi jauh lebih penting daripada estetika dekoratif. Setiap elemen harus memiliki tujuan yang jelas.

## 2. Warna & Tema (Color Strategy)
Desain AI generik seringkali terjebak pada palet warna yang mudah ditebak. Kita akan menggunakan strategi **Restrained** (Minimalis).

- **BAN Background "Warm/Beige/Sand":** Jangan gunakan warna dasar krem/pasir/beige (misal: warna kertas/ivory). Ini adalah *tell* (tanda) paling jelas dari desain AI 2024+. Gunakan **Off-white bersih** (Chroma mendekati 0), **Abu-abu netral (Cool/Slate)**, atau rancang mode gelap (Dark Mode) yang terencana (misal latar *near-black* pekat).
- **Kontras Ketat (Accessibility):** Teks *body* (termasuk *placeholder* pada input form) WAJIB memiliki rasio kontras **≥ 4.5:1** terhadap *background*. Teks besar (≥18px) minimal 3:1.
- **Hindari Teks "Muted Gray":** Jangan gunakan teks abu-abu muda di atas background putih tint hanya demi kesan "elegan". Jika teks terlihat pudar, gelapkan (*bump toward ink/black*).
- **Penggunaan Warna Aksen:** Aksen (misal: Primary Button, Status Badge) hanya boleh memakan maksimal ≤ 10% dari luas layar. Sisanya adalah monokrom / netral untuk memprioritaskan data katalog dan pesanan.

## 3. Tipografi
- **Tanpa "Eyebrow Kicker" Berulang:** Hindari menempatkan teks kecil *All-Caps* dengan spasi renggang (misal: `KATEGORI`, `PROSES`, `STATISTIK`) di atas setiap judul halaman/section. Itu adalah pola AI yang usang. Gunakan hierarki heading standar (H1, H2) yang rapi.
- **Letter-Spacing Natural:** Untuk judul besar (H1/H2 Dashboard), jangan buat *letter-spacing* (tracking) terlalu sempit hingga huruf bersentuhan (Batas bawah: `-0.04em`).
- **Data-Heavy Typography:** Gunakan font Sans-Serif yang bersih (misal: Inter, Roboto, atau Geist) dengan variasi *weight* (Medium/Semibold) untuk menonjolkan data angka (seperti metrik penghasilan) tanpa harus memperbesar font secara ekstrem.

## 4. Layout & Komponen (Khusus Shadcn UI)
Meskipun kita menggunakan Shadcn UI, kita perlu menyesuaikan beberapa nilai default-nya agar tidak terlihat seperti *template boilerplate*.

- **Border Radius Wajar:** Sudut (rounded corners) pada Card, Modal, dan Input maksimal **8px hingga 12px** (`rounded-md` atau `rounded-lg`). **DILARANG** menggunakan sudut melengkung ekstrem (misal `rounded-2xl` / `24px-32px`) pada antarmuka admin.
- **NO "Ghost Cards" (Ban):** Jangan menggabungkan border tipis (`border: 1px solid`) DENGAN *drop-shadow* lebar/pudar (blur ≥16px) pada komponen yang sama. Pilih salah satu:
  - **Flat Border:** Hanya border 1px tanpa bayangan (Sangat disarankan untuk Admin Dashboard).
  - **Soft Shadow:** Hanya bayangan tipis tanpa border.
- **Tanpa "Nested Cards":** Jangan meletakkan Card di dalam Card (misal: Card Profil Pelanggan yang di dalamnya terdapat Card Alamat). Gunakan *separator* (garis batas), *layout grid*, atau *background tint* halus untuk mengelompokkan konten anak.
- **Z-Index Semantik:** Jangan gunakan z-index sembarangan seperti `999` atau `9999`. Buat skala berurutan (Dropdown -> Sticky Header -> Modal Backdrop -> Modal -> Toast/Tooltip).

## 5. Larangan Mutlak (Absolute Bans)
Jika pola-pola ini ditemukan selama implementasi, kode harus **ditolak dan ditulis ulang**:

1. **Side-stripe Borders:** Garis tebal di pinggir kiri/kanan pada card notifikasi atau callout. Ganti dengan *background fill* tipis atau icon sebagai penanda.
2. **Glassmorphism Standar:** Latar belakang transparan dengan efek *blur* berat pada sidebar atau card. UI Admin butuh soliditas, hindari *glassmorphism*.
3. **Gradient Text / Background:** Dilarang menggunakan teks dengan `background-clip: text` gradient atau *background* bergaris diagonal (`repeating-linear-gradient`). Ini murni dekorasi AI.
4. **Ilustrasi "Sketchy SVG":** Jangan gunakan ilustrasi SVG kasar seperti coretan tangan/doodle untuk *Empty State* (misal saat keranjang pesanan kosong atau tidak ada pelanggan). Gunakan icon geometris yang profesional dan minimalis (seperti Lucide Icons).
5. **Animasi Berlebihan (Reflex Animations):** Animasi boleh ada, tapi jangan buat elemen (seperti baris tabel) masuk dengan *fade-up* tersendat secara seragam (*uniform reflex*). Gunakan transisi instan atau *crossfade* sangat cepat untuk Dashboard.

## 6. Integrasi dengan Roadmap Frontend
Saat mengerjakan Phase AD1 hingga AD7, pastikan menguji setiap tampilan (terutama List Pesanan dan Katalog) dalam mode responsive (Mobile/Tablet/Desktop) agar kolom tabel tidak berantakan (overflow). Gunakan komponen *Data Table* (Tanstack Table) dengan fitur *horizontal scroll* atau *card-view* untuk layar kecil.