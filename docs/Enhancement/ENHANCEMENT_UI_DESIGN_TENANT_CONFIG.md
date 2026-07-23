# Enhancement UI Design — Tenant Configuration Dashboard

**Status:** Draft — Ready for Review  
**Date:** 2026-07-23  
**Related:** [ENHANCEMENT_BUSINESS_RULES_TENANT_CONFIG.md](./ENHANCEMENT_BUSINESS_RULES_TENANT_CONFIG.md)  
**Design System:** [Admin Design Guidelines](../admin/admin_design_guidelines.md)

---

## Table of Contents

1. [Overview & Goals](#1-overview--goals)
2. [Information Architecture](#2-information-architecture)
3. [Navigation & Sidebar Changes](#3-navigation--sidebar-changes)
4. [Page-by-Page UI Specification](#4-page-by-page-ui-specification)
   - [4.1 Settings Landing Page (`/settings`)](#41-settings-landing-page-settings)
   - [4.2 Business Profile (`/settings/business`)](#42-business-profile-settingsbusiness)
   - [4.3 Channel Configuration (`/settings/channel`)](#43-channel-configuration-settingschannel)
   - [4.4 LLM Configuration (`/settings/llm`)](#44-llm-configuration-settingsllm)
   - [4.5 Shipping Configuration (`/settings/shipping`)](#45-shipping-configuration-settingsshipping)
   - [4.6 Payment Configuration (`/settings/payment`)](#46-payment-configuration-settingspayment)
5. [Component Library — New Components](#5-component-library--new-components)
6. [API Client Extensions](#6-api-client-extensions)
7. [State Management & Data Flow](#7-state-management--data-flow)
8. [Validation & Error Handling UX](#8-validation--error-handling-ux)
9. [Responsive Behavior](#9-responsive-behavior)
10. [Empty States & Edge Cases](#10-empty-states--edge-cases)
11. [Accessibility Requirements](#11-accessibility-requirements)
12. [Implementation Phases](#12-implementation-phases)

---

## 1. Overview & Goals

### Purpose

Enable each registered account (tenant) to self-configure their business operations through a unified **Settings** area in the admin dashboard. This replaces hardcoded environment variables with per-tenant database-backed configuration.

### Design Principles

| Principle | Application |
|-----------|-------------|
| **Progressive Disclosure** | Settings landing page shows summary cards; drill-down into each section for editing |
| **Single Choice Enforcement** | Radio cards (not checkboxes) for exclusive selections (1 channel, 1 LLM provider) |
| **Credential Security** | Masked API keys with reveal/copy toggle; never transmitted in plaintext after initial save |
| **Validation at Rest** | Inline validation on blur + form-level validation on submit; no silent failures |
| **Optimistic UI** | Save button shows loading state; success toast confirms persistence; error toast with retry |
| **Consistency** | All settings pages share the same layout shell (header + description + form + save bar) |

### User Personas

1. **Business Owner** — Configures business profile, channel, and payment. Non-technical. Needs guided setup.
2. **Technical Admin** — Configures LLM API keys, shipping API keys. Understands API concepts. Needs clear credential management.
3. **Support Staff** — View-only access to settings for troubleshooting. No edit permissions.

---

## 2. Information Architecture

### Settings Section Hierarchy

```
/settings                      → Landing (summary cards)
├── /settings/business         → Business Profile
├── /settings/channel          → WhatsApp Channel Configuration
├── /settings/llm              → LLM Provider Configuration
├── /settings/shipping         → Biteship API Configuration
└── /settings/payment          → Payment Gateway Configuration
```

### Why a Dedicated `/settings` Section?

- **Logical grouping**: All configuration lives under one roof
- **Scalable**: Future settings (notification preferences, team members, etc.) slot in naturally
- **Discoverable**: Single sidebar entry point; landing page acts as a directory
- **Secure**: Settings routes can be RBAC-protected independently from operational routes

### Relationship to Existing Pages

| Existing Page | Relationship |
|---------------|--------------|
| `/products` | Products reference `business_name` from settings in receipts/invoices |
| `/orders` | Orders use channel config for WhatsApp notifications |
| `/promos` | Promo vouchers may reference business currency from settings |
| `/` (Dashboard) | Dashboard metrics may show channel status indicator from settings |

---

## 3. Navigation & Sidebar Changes

### Current Sidebar Structure

```
Overview
  └── Dashboard

Operasional
  ├── Pesanan
  ├── Produk
  └── Kategori

Pelanggan
  ├── Pelanggan
  └── Inbox

Sistem
  ├── Promo
  └── Audit log
```

### Proposed Sidebar Structure

```
Overview
  └── Dashboard

Operasional
  ├── Pesanan
  ├── Produk
  └── Kategori

Pelanggan
  ├── Pelanggan
  └── Inbox

Sistem
  ├── Promo
  └── Audit log

Konfigurasi                          ← NEW GROUP
  └── Pengaturan                     ← NEW ITEM (icon: Settings/Sliders)
```

### Sidebar Implementation Detail

```typescript
// In sidebar.tsx navigationGroups array, ADD after "Sistem" group:

{
  label: "Konfigurasi",
  items: [
    { label: "Pengaturan", href: "/settings", icon: Settings }, // lucide-react "Settings" icon
  ],
},
```

### Icon Selection

Use `Settings` from `lucide-react` (gear/cog icon). Rationale:
- Universally recognized for configuration
- Already in the lucide-react package (no new dependency)
- Distinct from existing icons (Dashboard, ReceiptText, Package, Tag, Users, MessageSquare, FileText)

### Active State Behavior

```typescript
// Existing isCurrentPath logic already handles prefix matching:
// href="/settings" → pathname.startsWith("/settings") matches all sub-pages
```

---

## 4. Page-by-Page UI Specification

---

### 4.1 Settings Landing Page (`/settings`)

#### Purpose

Provide a high-level overview of all configuration sections with status indicators, enabling quick navigation to each sub-section.

#### Layout

```
┌──────────────────────────────────────────────────────────────┐
│  ← Sidebar Trigger    │    Pengaturan                        │  ← Header (sticky)
│                       │                                      │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│  Pengaturan Bisnis                                           │
│  Kelola profil bisnis, channel, LLM, pengiriman & pembayaran │  ← Page description
│                                                              │
│  ┌─────────────────────┐  ┌─────────────────────┐            │
│  │ 🏢 Bisnis           │  │ 💬 Channel           │            │  ← Summary Cards
│  │ Toko Berkah Jaya    │  │ WhatsApp Business    │            │     (2-column grid)
│  │ Konfigurasi lengkap │  │ API • Tersambung     │            │
│  │ [Edit]              │  │ [Edit]               │            │
│  └─────────────────────┘  └─────────────────────┘            │
│                                                              │
│  ┌─────────────────────┐  ┌─────────────────────┐            │
│  │ 🧠 LLM              │  │ 🚚 Pengiriman        │            │
│  │ OpenAI • GPT-4o     │  │ Biteship • Aktif     │            │
│  │ API Key: sk-...xxxx │  │ API Key: biteship... │            │
│  │ [Edit]              │  │ [Edit]               │            │
│  └─────────────────────┘  └─────────────────────┘            │
│                                                              │
│  ┌─────────────────────┐                                     │
│  │ 💳 Pembayaran        │                                     │
│  │ Xendit • Production │                                     │
│  │ [Edit]              │                                     │
│  └─────────────────────┘                                     │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

#### Summary Card Component Specification

Each card displays:

| Element | Source | Example |
|---------|--------|---------|
| **Icon** | Static per category | `Building2` (Bisnis), `MessageCircle` (Channel), `Brain` (LLM), `Truck` (Pengiriman), `CreditCard` (Pembayaran) |
| **Label** | Static | "Bisnis", "Channel", "LLM", "Pengiriman", "Pembayaran" |
| **Primary Info** | Dynamic from API | Business name, channel type + connection status, LLM provider + model, shipping provider + status, payment provider + mode |
| **Secondary Info** | Dynamic from API | Completion status, API key masked preview, connection health |
| **Status Indicator** | Computed | Green dot = configured + active; Yellow dot = partially configured; Gray dot = not configured |
| **Action Button** | Static | "Konfigurasi" → links to sub-page |

#### Status Logic

```
Card Status = 
  IF all required fields filled AND connection verified → "active" (green)
  ELSE IF some fields filled → "partial" (yellow)
  ELSE → "empty" (gray)
```

#### Loading State

While fetching settings from API (`GET /api/v1/settings`), show 5 skeleton cards (pulsing gray rectangles matching card dimensions).

#### Empty State (First Visit)

All cards show gray status dot with secondary text "Belum dikonfigurasi". A banner at top:

```
┌──────────────────────────────────────────────────────────────┐
│  🚀 Selamat datang! Lengkapi pengaturan bisnis kamu dulu.    │
│     Mulai dari Profil Bisnis → Channel → LLM → Pengiriman    │
│     [Mulai Konfigurasi]                                      │
└──────────────────────────────────────────────────────────────┘
```

#### Error State

If `GET /api/v1/settings` fails:
- Toast: "Gagal memuat pengaturan. Coba lagi."
- Each card shows "—" for dynamic fields
- Retry button in page header area

---

### 4.2 Business Profile (`/settings/business`)

#### Purpose

Configure the tenant's core business identity: name, description, currency, timezone, and contact information.

#### Layout

```
┌──────────────────────────────────────────────────────────────┐
│  ← Sidebar Trigger    │    Pengaturan > Profil Bisnis        │  ← Header with breadcrumb
│                       │                                      │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│  Profil Bisnis                                               │
│  Informasi dasar bisnis yang akan muncul di struk & invoice  │  ← Section description
│                                                              │
│  ┌──────────────────────────────────────────────────────────┐│
│  │  Nama Bisnis *                                           ││
│  │  ┌──────────────────────────────────────────────────────┐││
│  │  │ Toko Berkah Jaya                                     │││  ← Text input
│  │  └──────────────────────────────────────────────────────┘││
│  │  Nama yang muncul di header invoice & pesan WhatsApp     ││  ← Helper text
│  └──────────────────────────────────────────────────────────┘│
│                                                              │
│  ┌──────────────────────────────────────────────────────────┐│
│  │  Deskripsi Bisnis                                        ││
│  │  ┌──────────────────────────────────────────────────────┐││
│  │  │ Menjual berbagai produk elektronik & aksesoris...    │││  ← Textarea (3 rows)
│  │  └──────────────────────────────────────────────────────┘││
│  │  Deskripsi singkat untuk katalog & AI agent context      ││
│  └──────────────────────────────────────────────────────────┘│
│                                                              │
│  ┌─ Two-column row ─────────────────────────────────────────┐│
│  │  ┌──────────────────────┐  ┌──────────────────────────┐  ││
│  │  │ Mata Uang *          │  │ Zona Waktu *             │  ││
│  │  │ ┌──────────────────┐ │  │ ┌──────────────────────┐ │  ││
│  │  │ │ IDR (Rp)      ▼ │ │  │  │ Asia/Jakarta (WIB) ▼│ │  ││  ← Select dropdowns
│  │  │ └──────────────────┘ │  │ └──────────────────────┘ │  ││
│  │  └──────────────────────┘  └──────────────────────────┘  ││
│  └──────────────────────────────────────────────────────────┘│
│                                                              │
│  ┌──────────────────────────────────────────────────────────┐│
│  │  Informasi Kontak                                        ││
│  │  ┌──────────────────────┐  ┌──────────────────────────┐  ││
│  │  │ Email Bisnis         │  │ Telepon Bisnis           │  ││
│  │  │ ┌──────────────────┐ │  │ ┌──────────────────────┐ │  ││
│  │  │ │ toko@email.com   │ │  │ │ +6281234567890       │ │  ││
│  │  │ └──────────────────┘ │  │ └──────────────────────┘ │  ││
│  │  └──────────────────────┘  └──────────────────────────┘  ││
│  └──────────────────────────────────────────────────────────┘│
│                                                              │
│  ┌─ Save Bar ───────────────────────────────────────────────┐│
│  │  [Reset]                          [💾 Simpan Perubahan]  ││  ← Sticky bottom bar
│  └──────────────────────────────────────────────────────────┘│
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

#### Form Fields Specification

| Field | Type | Required | Validation | Default |
|-------|------|----------|------------|---------|
| `business_name` | `text` (max 100) | ✅ | Non-empty, max 100 chars | `""` |
| `business_description` | `textarea` (max 500) | ❌ | Max 500 chars | `""` |
| `currency` | `select` | ✅ | Must be in allowed list | `"IDR"` |
| `timezone` | `select` | ✅ | Must be valid IANA timezone | `"Asia/Jakarta"` |
| `contact_email` | `email` | ❌ | Valid email format if filled | `""` |
| `contact_phone` | `tel` | ❌ | E.164 format if filled | `""` |

#### Currency Options

```
IDR (Rp)  — Indonesian Rupiah
USD ($)   — US Dollar
SGD (S$)  — Singapore Dollar
MYR (RM)  — Malaysian Ringgit
```

#### Timezone Options (Top 3 + All IANA)

```
Asia/Jakarta (WIB)    — GMT+7
Asia/Makassar (WITA)  — GMT+8
Asia/Jayapura (WIT)   — GMT+9
--- (divider) ---
[All other IANA timezones alphabetically]
```

#### Save Bar Behavior

- **Sticky**: Fixed to bottom of viewport while scrolling form
- **Background**: White with top border `border-slate-200`
- **Reset button**: Secondary variant, resets to last-saved values (not empty)
- **Save button**: Primary variant, shows spinner while saving
- **Unsaved changes indicator**: Small orange dot next to "Profil Bisnis" breadcrumb when form is dirty

#### Validation UX

- **On Blur**: Validate individual field; show red border + error message below field
- **On Submit**: Validate all fields; focus first error field; scroll to it
- **Success**: Green toast "Profil bisnis disimpan" + breadcrumb dot cleared
- **Error**: Red toast "Gagal menyimpan. [error message]. Coba lagi."

#### API Contract

```
GET    /api/v1/settings/business     → Read business profile
PUT    /api/v1/settings/business     → Update business profile
```

---

### 4.3 Channel Configuration (`/settings/channel`)

#### Purpose

Select and configure exactly ONE WhatsApp channel provider: either **WhatsApp Business API** (direct) or a **Third-Party Provider** (e.g., WATI, AiChat, Qiscus). The selection is exclusive — choosing one deselects the other.

#### Layout

```
┌──────────────────────────────────────────────────────────────┐
│  ← Sidebar Trigger    │    Pengaturan > Channel              │  ← Header with breadcrumb
│                       │                                      │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│  Channel WhatsApp                                            │
│  Pilih satu penyedia channel untuk menghubungkan AI agent    │  ← Section description
│  ke WhatsApp. Hanya satu channel yang bisa aktif.            │
│                                                              │
│  ┌─ Provider Selection (Radio Cards) ───────────────────────┐│
│  │                                                          ││
│  │  ┌─────────────────────────────┐  ┌────────────────────┐ ││
│  │  │ ○ WhatsApp Business API     │  │ ○ Third-Party      │ ││  ← Radio cards
│  │  │   (Meta / Cloud API)        │  │   (WATI, AiChat,   │ ││     (mutually exclusive)
│  │  │                             │  │    Qiscus, etc.)   │ ││
│  │  │   ✓ Official Meta API       │  │   ✓ Cepat setup    │ ││
│  │  │   ✓ Full message types      │  │   ✓ Managed infra  │ ││
│  │  │   ✓ Template management     │  │   ✓ Built-in tools │ ││
│  │  │                             │  │                    │ ││
│  │  │   ⚠ Perlu Meta Business     │  │   ⚠ Biaya provider │ ││
│  │  │      Account terverifikasi  │  │      pihak ketiga  │ ││
│  │  └─────────────────────────────┘  └────────────────────┘ ││
│  └──────────────────────────────────────────────────────────┘│
│                                                              │
│  ┌─ Conditional Form (appears based on selection) ──────────┐│
│  │                                                          ││
│  │  [IF WhatsApp Business API selected]                     ││
│  │                                                          ││
│  │  ┌──────────────────────────────────────────────────────┐││
│  │  │  Phone Number ID *                                   │││
│  │  │  ┌──────────────────────────────────────────────────┐│││
│  │  │  │ 12345678901234567890                             ││││
│  │  │  └──────────────────────────────────────────────────┘│││
│  │  │  ID nomor telepon dari WhatsApp Business Account     │││
│  │  └──────────────────────────────────────────────────────┘││
│  │                                                          ││
│  │  ┌──────────────────────────────────────────────────────┐││
│  │  │  Business Account ID *                               │││
│  │  │  ┌──────────────────────────────────────────────────┐│││
│  │  │  │ 98765432109876543210                             ││││
│  │  │  └──────────────────────────────────────────────────┘│││
│  │  │  ID akun bisnis WhatsApp                             │││
│  │  └──────────────────────────────────────────────────────┘││
│  │                                                          ││
│  │  ┌──────────────────────────────────────────────────────┐││
│  │  │  Access Token *                                      │││
│  │  │  ┌──────────────────────────────────────────────────┐│││
│  │  │  │ ●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●● ││││  ← Masked input
│  │  │  │                                       [👁 Reveal]││││  ← Toggle visibility
│  │  │  └──────────────────────────────────────────────────┘│││
│  │  │  System user access token dari Meta Business Suite   │││
│  │  │  [📋 Copy]                                           │││  ← Copy to clipboard
│  │  └──────────────────────────────────────────────────────┘││
│  │                                                          ││
│  │  ┌──────────────────────────────────────────────────────┐││
│  │  │  Webhook Verify Token                                │││
│  │  │  ┌──────────────────────────────────────────────────┐│││
│  │  │  │ ●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●● ││││
│  │  │  └──────────────────────────────────────────────────┘│││
│  │  │  Token untuk verifikasi webhook callback dari Meta   │││
│  │  └──────────────────────────────────────────────────────┘││
│  │                                                          ││
│  │  ┌─ Connection Test ────────────────────────────────────┐││
│  │  │  Status: ◉ Belum diuji                               │││
│  │  │  [🔌 Uji Koneksi]                                    │││  ← Test button
│  │  │  ┌─ Result (appears after test) ───────────────────┐ │││
│  │  │  │ ✅ Koneksi berhasil                             │ │││
│  │  │  │    Phone Number ID: +62 812-3456-7890           │ │││
│  │  │  │    Webhook URL: https://.../webhooks/whatsapp   │ │││
│  │  │  └────────────────────────────────────────────────┘ │││
│  │  └──────────────────────────────────────────────────────┘││
│  │                                                          ││
│  │  ─── OR ───                                              ││
│  │                                                          ││
│  │  [IF Third-Party selected]                               ││
│  │                                                          ││
│  │  ┌──────────────────────────────────────────────────────┐││
│  │  │  Provider *                                          │││
│  │  │  ┌──────────────────────────────────────────────────┐│││
│  │  │  │ Pilih provider...                          ▼     ││││  ← Select dropdown
│  │  │  └──────────────────────────────────────────────────┘│││
│  │  │  Options: WATI, AiChat, Qiscus, WhatsApp Cloud API  │││
│  │  └──────────────────────────────────────────────────────┘││
│  │                                                          ││
│  │  ┌──────────────────────────────────────────────────────┐││
│  │  │  API Key / Access Token *                            │││
│  │  │  ┌──────────────────────────────────────────────────┐│││
│  │  │  │ ●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●● ││││
│  │  │  └──────────────────────────────────────────────────┘│││
│  │  │  API key dari dashboard provider pihak ketiga        │││
│  │  └──────────────────────────────────────────────────────┘││
│  │                                                          ││
│  │  ┌──────────────────────────────────────────────────────┐││
│  │  │  API URL / Base URL                                  │││
│  │  │  ┌──────────────────────────────────────────────────┐│││
│  │  │  │ https://api.wati.io/v1                           ││││
│  │  │  └──────────────────────────────────────────────────┘│││
│  │  │  Base URL endpoint provider (default terisi otomatis)│││
│  │  └──────────────────────────────────────────────────────┘││
│  └──────────────────────────────────────────────────────────┘│
│                                                              │
│  ┌─ Save Bar ───────────────────────────────────────────────┐│
│  │  [Reset]                          [💾 Simpan Konfigurasi]││
│  └──────────────────────────────────────────────────────────┘│
└──────────────────────────────────────────────────────────────┘
```

#### Radio Card Component Specification

- **Visual**: White card with border; selected card has `border-blue-500` + `bg-blue-50` + blue radio dot filled
- **Content**: Provider name (bold), subtitle (gray), 2-3 benefit bullets (green checkmarks), 1 caveat bullet (yellow warning)
- **Interaction**: Click entire card to select; only one selectable at a time
- **Transition**: Border color transitions 200ms ease; background transitions 200ms ease

#### Provider Selection Logic

```
WHEN user selects a different provider:
  1. Show confirmation dialog: "Mengganti provider akan menghapus konfigurasi 
     sebelumnya. Lanjutkan?"
  2. IF confirmed → Clear previous provider fields, switch form
  3. IF cancelled → Keep previous selection
```

#### Third-Party Provider Options

```typescript
const THIRD_PARTY_PROVIDERS = [
  { value: "wati",        label: "WATI",        defaultBaseUrl: "https://api.wati.io/v1" },
  { value: "aichat",      label: "AiChat",      defaultBaseUrl: "https://api.aichat.com/v1" },
  { value: "qiscus",      label: "Qiscus",      defaultBaseUrl: "https://api.qiscus.com/v1" },
  { value: "whatsapp-cloud", label: "WhatsApp Cloud API (Direct)", defaultBaseUrl: "https://graph.facebook.com/v18.0" },
];
```

#### Connection Test UX

1. User clicks "Uji Koneksi"
2. Button shows spinner + text "Menguji..."
3. Backend validates credentials against provider API
4. Result appears below button:
   - **Success**: Green card with checkmark, phone number display name, webhook URL
   - **Failure**: Red card with error details (e.g., "Invalid access token", "Phone Number ID not found")
5. Test result is ephemeral (not saved); disappears on page reload

#### Credential Field UX

- **Masked by default**: Shows `●` characters (same length as actual value for visual consistency)
- **Reveal toggle**: Eye icon button toggles between masked and plaintext
- **Copy button**: Clipboard icon copies plaintext value (with toast "Token disalin")
- **On first entry**: Field is empty plaintext input (not masked) until saved
- **On edit**: Shows masked; clicking reveal shows plaintext; user can edit

#### API Contract

```
GET    /api/v1/settings/channel      → Read channel configuration
PUT    /api/v1/settings/channel      → Update channel configuration
POST   /api/v1/settings/channel/test → Test channel connection
```

---

### 4.4 LLM Configuration (`/settings/llm`)

#### Purpose

Select exactly ONE LLM provider (Anthropic or OpenAI) and configure its API credentials. The AI Sales Agent uses this LLM for all conversations.

#### Layout

```
┌──────────────────────────────────────────────────────────────┐
│  ← Sidebar Trigger    │    Pengaturan > LLM                  │  ← Header with breadcrumb
│                       │                                      │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│  Konfigurasi LLM                                             │
│  Pilih penyedia LLM untuk AI Sales Agent. Hanya satu yang    │  ← Section description
│  bisa aktif. API key disimpan terenkripsi.                   │
│                                                              │
│  ┌─ Provider Selection (Radio Cards) ───────────────────────┐│
│  │                                                          ││
│  │  ┌─────────────────────────────┐  ┌────────────────────┐ ││
│  │  │ ○ OpenAI                    │  │ ○ Anthropic        │ ││  ← Radio cards
│  │  │   (GPT-4o / GPT-4o-mini)    │  │   (Claude 3.5 /    │ ││
│  │  │                             │  │    Claude 3 Opus)  │ ││
│  │  │   ✓ Model terbaru           │  │   ✓ Konteks 200K   │ ││
│  │  │   ✓ Ekosistem luas          │  │   ✓ Safety built-in│ ││
│  │  │   ✓ Multi-modal             │  │   ✓ Reasoning kuat │ ││
│  │  └─────────────────────────────┘  └────────────────────┘ ││
│  └──────────────────────────────────────────────────────────┘│
│                                                              │
│  ┌─ Conditional Form ───────────────────────────────────────┐│
│  │                                                          ││
│  │  ┌──────────────────────────────────────────────────────┐││
│  │  │  Model *                                             │││
│  │  │  ┌──────────────────────────────────────────────────┐│││
│  │  │  │ [IF OpenAI] GPT-4o                        ▼     ││││  ← Dynamic options
│  │  │  │ [IF Anthropic] Claude 3.5 Sonnet          ▼     ││││
│  │  │  └──────────────────────────────────────────────────┘│││
│  │  │  Model yang digunakan AI agent untuk percakapan      │││
│  │  └──────────────────────────────────────────────────────┘││
│  │                                                          ││
│  │  ┌──────────────────────────────────────────────────────┐││
│  │  │  API Key *                                           │││
│  │  │  ┌──────────────────────────────────────────────────┐│││
│  │  │  │ ●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●● ││││
│  │  │  │                                       [👁 Reveal]││││
│  │  │  └──────────────────────────────────────────────────┘│││
│  │  │  [IF OpenAI]     sk-... (dari platform.openai.com)   │││
│  │  │  [IF Anthropic]  sk-ant-... (dari console.anthropic) │││
│  │  │  [📋 Copy]                                           │││
│  │  └──────────────────────────────────────────────────────┘││
│  │                                                          ││
│  │  ┌──────────────────────────────────────────────────────┐││
│  │  │  Organization ID (Optional — OpenAI only)            │││
│  │  │  ┌──────────────────────────────────────────────────┐│││
│  │  │  │ org-xxxxxxxxxxxxxxxx                            ││││
│  │  │  └──────────────────────────────────────────────────┘│││
│  │  │  Hanya muncul jika OpenAI dipilih                    │││
│  │  └──────────────────────────────────────────────────────┘││
│  │                                                          ││
│  │  ┌─ Model Info Card ────────────────────────────────────┐││
│  │  │  📊 GPT-4o                                           │││
│  │  │  Context window: 128K tokens                         │││
│  │  │  Max output: 16,384 tokens                           │││
│  │  │  Pricing: $2.50/1M input | $10.00/1M output          │││
│  │  │  Knowledge cutoff: Oct 2023                          │││
│  │  └──────────────────────────────────────────────────────┘││
│  │                                                          ││
│  │  ┌─ Connection Test ────────────────────────────────────┐││
│  │  │  Status: ◉ Belum diuji                               │││
│  │  │  [🔌 Uji Koneksi]                                    │││
│  │  └──────────────────────────────────────────────────────┘││
│  └──────────────────────────────────────────────────────────┘│
│                                                              │
│  ┌─ Save Bar ───────────────────────────────────────────────┐│
│  │  [Reset]                          [💾 Simpan Konfigurasi]││
│  └──────────────────────────────────────────────────────────┘│
└──────────────────────────────────────────────────────────────┘
```

#### Model Options (Dynamic by Provider)

**OpenAI Models:**
```
gpt-4o            — Fast, intelligent, multi-modal (128K context)
gpt-4o-mini       — Cost-effective for simple tasks (128K context)
gpt-4-turbo       — Legacy high-intelligence (128K context)
```

**Anthropic Models:**
```
claude-3-5-sonnet — Best balance of speed & intelligence (200K context)
claude-3-opus     — Most powerful for complex reasoning (200K context)
claude-3-haiku    — Fastest, most cost-effective (200K context)
```

#### Model Info Card

Dynamic content based on selected model. Shows:
- Context window size
- Max output tokens
- Pricing (input/output per 1M tokens)
- Knowledge cutoff date
- Link to provider docs

#### API Key Validation (Client-Side)

```
[OpenAI]   Must start with "sk-" or "sk-proj-"
[Anthropic] Must start with "sk-ant-"
Show inline hint below field with expected prefix format
```

#### API Contract

```
GET    /api/v1/settings/llm          → Read LLM configuration
PUT    /api/v1/settings/llm          → Update LLM configuration
POST   /api/v1/settings/llm/test     → Test LLM connection (simple completion)
```

---

### 4.5 Shipping Configuration (`/settings/shipping`)

#### Purpose

Configure Biteship API credentials for shipping rate quotes and shipment booking. This is a single-provider configuration (no provider selection — Biteship is the only shipping provider).

#### Layout

```
┌──────────────────────────────────────────────────────────────┐
│  ← Sidebar Trigger    │    Pengaturan > Pengiriman           │  ← Header with breadcrumb
│                       │                                      │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│  Konfigurasi Pengiriman (Biteship)                           │
│  Atur kredensial API Biteship untuk kalkulasi ongkir &       │  ← Section description
│  pembuatan resi otomatis.                                    │
│                                                              │
│  ┌─ Provider Badge ─────────────────────────────────────────┐│
│  │  🚚 Biteship — Logistics API Platform                    ││  ← Fixed provider badge
│  │  docs.biteship.com                                       ││
│  └──────────────────────────────────────────────────────────┘│
│                                                              │
│  ┌──────────────────────────────────────────────────────────┐│
│  │  API Key *                                               ││
│  │  ┌──────────────────────────────────────────────────────┐││
│  │  │ ●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●● │││
│  │  │                                       [👁 Reveal]    │││
│  │  └──────────────────────────────────────────────────────┘││
│  │  API key dari dashboard Biteship (Settings > API Key)    ││
│  │  Format: biteship_xxx... atau biteship_test_xxx...       ││
│  │  [📋 Copy]                                               ││
│  └──────────────────────────────────────────────────────────┘│
│                                                              │
│  ┌──────────────────────────────────────────────────────────┐│
│  │  Webhook Token                                           ││
│  │  ┌──────────────────────────────────────────────────────┐││
│  │  │ ●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●● │││
│  │  └──────────────────────────────────────────────────────┘││
│  │  Token untuk verifikasi callback tracking dari Biteship  ││
│  └──────────────────────────────────────────────────────────┘│
│                                                              │
│  ┌──────────────────────────────────────────────────────────┐│
│  │  Origin Address (Alamat Pengirim) *                      ││
│  │  ┌──────────────────────────────────────────────────────┐││
│  │  │ Alamat Lengkap                                       │││
│  │  │ ┌──────────────────────────────────────────────────┐ │││
│  │  │ │ Jl. Merdeka No. 10, Lt. 2                        │ │││
│  │  │ └──────────────────────────────────────────────────┘ │││
│  │  └──────────────────────────────────────────────────────┘││
│  │  ┌─ Two-column row ─────────────────────────────────────┐││
│  │  │  ┌──────────────────────┐  ┌──────────────────────┐  │││
│  │  │  │ Kota / Kabupaten *   │  │ Kecamatan             │  │││
│  │  │  │ ┌──────────────────┐ │  │ ┌──────────────────┐ │  │││
│  │  │  │ │ Jakarta Pusat    │ │  │ │ Menteng          │ │  │││
│  │  │  │ └──────────────────┘ │  │ └──────────────────┘ │  │││
│  │  │  └──────────────────────┘  └──────────────────────┘  │││
│  │  └──────────────────────────────────────────────────────┘││
│  │  ┌─ Two-column row ─────────────────────────────────────┐││
│  │  │  ┌──────────────────────┐  ┌──────────────────────┐  │││
│  │  │  │ Kode Pos *           │  │ Provinsi              │  │││
│  │  │  │ ┌──────────────────┐ │  │ ┌──────────────────┐ │  │││
│  │  │  │ │ 10310            │ │  │ │ DKI Jakarta      │ │  │││
│  │  │  │ └──────────────────┘ │  │ └──────────────────┘ │  │││
│  │  │  └──────────────────────┘  └──────────────────────┘  │││
│  │  └──────────────────────────────────────────────────────┘││
│  └──────────────────────────────────────────────────────────┘│
│                                                              │
│  ┌──────────────────────────────────────────────────────────┐│
│  │  Origin Contact                                          ││
│  │  ┌──────────────────────┐  ┌──────────────────────────┐  ││
│  │  │ Nama Pengirim        │  │ Telepon Pengirim         │  ││
│  │  │ ┌──────────────────┐ │  │ ┌──────────────────────┐ │  ││
│  │  │ │ Toko Berkah Jaya │ │  │ │ +6281234567890      │ │  ││
│  │  │ └──────────────────┘ │  │ └──────────────────────┘ │  ││
│  │  └──────────────────────┘  └──────────────────────────┘  ││
│  └──────────────────────────────────────────────────────────┘│
│                                                              │
│  ┌─ Connection Test ────────────────────────────────────────┐│
│  │  Status: ◉ Belum diuji                                   ││
│  │  [🔌 Uji Koneksi]                                        ││
│  └──────────────────────────────────────────────────────────┘│
│                                                              │
│  ┌─ Save Bar ───────────────────────────────────────────────┐│
│  │  [Reset]                          [💾 Simpan Konfigurasi]││
│  └──────────────────────────────────────────────────────────┘│
└──────────────────────────────────────────────────────────────┘
```

#### Form Fields Specification

| Field | Type | Required | Validation |
|-------|------|----------|------------|
| `api_key` | `password` (masked) | ✅ | Non-empty; prefix check (`biteship_` or `biteship_test_`) |
| `webhook_token` | `password` (masked) | ❌ | — |
| `origin_address_line` | `text` (max 500) | ✅ | Non-empty |
| `origin_city` | `text` (max 100) | ✅ | Non-empty |
| `origin_district` | `text` (max 100) | ❌ | — |
| `origin_postal_code` | `text` (max 10) | ✅ | Numeric, 5 digits |
| `origin_province` | `text` (max 100) | ❌ | — |
| `origin_name` | `text` (max 100) | ❌ | — |
| `origin_phone` | `tel` | ❌ | E.164 format if filled |

#### API Contract

```
GET    /api/v1/settings/shipping     → Read shipping configuration
PUT    /api/v1/settings/shipping     → Update shipping configuration
POST   /api/v1/settings/shipping/test → Test Biteship connection (rate quote to a known postal code)
```

---

### 4.6 Payment Configuration (`/settings/payment`)

#### Purpose

Configure the payment gateway. Currently **Xendit** is the primary recommendation. This page shows the current provider and allows credential configuration.

> **Note:** Payment provider selection (Xendit vs others) is a Phase 2 enhancement. Phase 1 assumes Xendit as the fixed provider with credential configuration only.

#### Layout

```
┌──────────────────────────────────────────────────────────────┐
│  ← Sidebar Trigger    │    Pengaturan > Pembayaran           │  ← Header with breadcrumb
│                       │                                      │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│  Konfigurasi Pembayaran                                      │
│  Atur gateway pembayaran untuk menerima pembayaran dari      │  ← Section description
│  pelanggan. Saat ini mendukung Xendit.                       │
│                                                              │
│  ┌─ Provider Badge ─────────────────────────────────────────┐│
│  │  💳 Xendit — Payment Gateway Indonesia                   ││  ← Fixed provider badge
│  │  docs.xendit.co                                          ││
│  │  Mode: [Production] / [Sandbox]   ← Toggle switch        ││
│  └──────────────────────────────────────────────────────────┘│
│                                                              │
│  ┌─ Mode Toggle ────────────────────────────────────────────┐│
│  │  ┌──────────────────────────────────────────────────────┐││
│  │  │  Mode:  ○ Sandbox (Test)   ● Production (Live)       │││  ← Segmented control
│  │  │                                                      │││
│  │  │  ⚠ Production mode akan memproses pembayaran nyata.  │││  ← Warning for production
│  │  │     Pastikan API key sudah benar.                    │││
│  │  └──────────────────────────────────────────────────────┘││
│  └──────────────────────────────────────────────────────────┘│
│                                                              │
│  ┌──────────────────────────────────────────────────────────┐│
│  │  Secret Key *                                            ││
│  │  ┌──────────────────────────────────────────────────────┐││
│  │  │ ●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●● │││
│  │  │                                       [👁 Reveal]    │││
│  │  └──────────────────────────────────────────────────────┘││
│  │  [Sandbox]     xnd_development_...                       ││
│  │  [Production]  xnd_production_...                        ││
│  │  Dari dashboard Xendit > Settings > API Keys             ││
│  │  [📋 Copy]                                               ││
│  └──────────────────────────────────────────────────────────┘│
│                                                              │
│  ┌──────────────────────────────────────────────────────────┐│
│  │  Callback Token *                                        ││
│  │  ┌──────────────────────────────────────────────────────┐││
│  │  │ ●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●● │││
│  │  └──────────────────────────────────────────────────────┘││
│  │  Token untuk verifikasi webhook callback dari Xendit     ││
│  │  (Bebas diisi — harus sama dengan setting di Xendit)     ││
│  └──────────────────────────────────────────────────────────┘│
│                                                              │
│  ┌──────────────────────────────────────────────────────────┐│
│  │  Redirect URLs                                           ││
│  │  ┌──────────────────────────────────────────────────────┐││
│  │  │ Success Redirect URL                                 │││
│  │  │ ┌──────────────────────────────────────────────────┐ │││
│  │  │ │ https://tokoberkah.com/order/success             │ │││
│  │  │ └──────────────────────────────────────────────────┘ │││
│  │  └──────────────────────────────────────────────────────┘││
│  │  ┌──────────────────────────────────────────────────────┐││
│  │  │ Failure Redirect URL                                 │││
│  │  │ ┌──────────────────────────────────────────────────┐ │││
│  │  │ │ https://tokoberkah.com/order/failed              │ │││
│  │  │ └──────────────────────────────────────────────────┘ │││
│  │  └──────────────────────────────────────────────────────┘││
│  └──────────────────────────────────────────────────────────┘│
│                                                              │
│  ┌─ Webhook Info ───────────────────────────────────────────┐│
│  │  📡 Webhook URL (beri ke dashboard Xendit):              ││
│  │  ┌──────────────────────────────────────────────────────┐││
│  │  │ https://api.tokoberkah.com/webhooks/xendit           │││  ← Read-only, copyable
│  │  │                                       [📋 Copy]      │││
│  │  └──────────────────────────────────────────────────────┘││
│  └──────────────────────────────────────────────────────────┘││
│                                                              │
│  ┌─ Connection Test ────────────────────────────────────────┐│
│  │  Status: ◉ Belum diuji                                   ││
│  │  [🔌 Uji Koneksi]                                        ││
│  └──────────────────────────────────────────────────────────┘││
│                                                              │
│  ┌─ Save Bar ───────────────────────────────────────────────┐│
│  │  [Reset]                          [💾 Simpan Konfigurasi]││
│  └──────────────────────────────────────────────────────────┘│
└──────────────────────────────────────────────────────────────┘
```

#### Mode Toggle Behavior

```
WHEN user switches from Sandbox → Production:
  1. Show warning dialog: "Anda beralih ke mode Production. Pembayaran nyata 
     akan diproses. Pastikan Secret Key Production sudah benar."
  2. IF confirmed → Switch mode, clear sandbox test results
  3. IF cancelled → Keep sandbox mode

WHEN user switches from Production → Sandbox:
  1. No warning (safe operation)
  2. Switch immediately
```

#### Secret Key Validation (Client-Side)

```
[Sandbox]     Must start with "xnd_development_"
[Production]  Must start with "xnd_production_" or "xnd_live_"
Show inline hint below field with expected prefix based on selected mode
```

#### Webhook URL Display

- Read-only field with copy button
- URL is constructed from `API_BASE_URL` + `/webhooks/xendit`
- Shows helper text: "Daftarkan URL ini di dashboard Xendit > Settings > Webhooks"

#### API Contract

```
GET    /api/v1/settings/payment      → Read payment configuration
PUT    /api/v1/settings/payment      → Update payment configuration
POST   /api/v1/settings/payment/test → Test Xendit connection (create test invoice)
```

---

## 5. Component Library — New Components

### 5.1 `SettingsShell`

**Purpose:** Shared layout wrapper for all settings sub-pages.

```typescript
// Props
interface SettingsShellProps {
  title: string;           // e.g., "Profil Bisnis"
  description: string;     // e.g., "Informasi dasar bisnis..."
  breadcrumb: string;      // e.g., "Pengaturan > Profil Bisnis"
  children: React.ReactNode;
  onSave: () => Promise<void>;
  onReset: () => void;
  isDirty: boolean;
  isSaving: boolean;
}
```

**Structure:**
```
┌─ Breadcrumb + Title + Description ─┐
│                                     │
│  {children}  ← Form fields          │
│                                     │
│  ┌─ Save Bar (sticky bottom) ────┐  │
│  │  [Reset]    [💾 Save Changes] │  │
│  └───────────────────────────────┘  │
└─────────────────────────────────────┘
```

### 5.2 `RadioCardGroup` + `RadioCard`

**Purpose:** Exclusive selection between two or more options with rich content.

```typescript
interface RadioCardOption {
  value: string;
  label: string;
  subtitle: string;
  benefits: string[];    // Green checkmark items
  caveats: string[];     // Yellow warning items (optional)
  icon?: React.ReactNode;
}

interface RadioCardGroupProps {
  options: RadioCardOption[];
  value: string | null;
  onChange: (value: string) => void;
  name: string;          // For radio group accessibility
}
```

**Visual States:**
- **Unselected**: White bg, `border-slate-200`, radio circle empty
- **Selected**: `bg-blue-50`, `border-blue-500 ring-1 ring-blue-500`, radio circle filled blue
- **Hover (unselected)**: `border-slate-300 bg-slate-50`
- **Focus**: `ring-2 ring-blue-500 ring-offset-2`

### 5.3 `MaskedInput`

**Purpose:** Secure input field for API keys/tokens with reveal toggle and copy button.

```typescript
interface MaskedInputProps {
  value: string;
  onChange: (value: string) => void;
  label: string;
  helperText?: string;
  prefixHint?: string;    // e.g., "Starts with sk-"
  required?: boolean;
  error?: string;
}
```

**States:**
- **Empty (first entry)**: Plain text input, type="text" (user needs to see what they paste)
- **Saved (masked)**: Shows `●` characters, type="password", eye icon toggles visibility
- **Revealed**: Plain text visible, eye slash icon, copy button active
- **Error**: Red border + error message below

### 5.4 `ConnectionTestButton`

**Purpose:** Test API credentials against provider and display result.

```typescript
interface ConnectionTestButtonProps {
  onTest: () => Promise<ConnectionTestResult>;
  providerLabel: string;  // e.g., "OpenAI", "Biteship"
}

interface ConnectionTestResult {
  success: boolean;
  message: string;
  details?: Record<string, string>;  // e.g., { phoneNumber: "+62...", webhookUrl: "https://..." }
}
```

**States:**
- **Idle**: "◉ Belum diuji" + blue outline button "🔌 Uji Koneksi"
- **Testing**: Button shows spinner + "Menguji..."
- **Success**: Green card replacing idle state, with checkmark + details
- **Failure**: Red card replacing idle state, with X mark + error message + "Coba Lagi" button

### 5.5 `SettingsSummaryCard`

**Purpose:** Card on `/settings` landing page showing configuration status.

```typescript
interface SettingsSummaryCardProps {
  icon: React.ComponentType<{ className?: string }>;
  label: string;
  primaryInfo: string;      // e.g., "OpenAI • GPT-4o"
  secondaryInfo: string;    // e.g., "API Key: sk-...xxxx"
  status: "active" | "partial" | "empty";
  href: string;             // Link to sub-page
}
```

**Status Indicators:**
- **Active**: Green dot (`bg-green-500`) + "Tersambung" / "Aktif"
- **Partial**: Yellow dot (`bg-yellow-500`) + "Sebagian"
- **Empty**: Gray dot (`bg-slate-300`) + "Belum dikonfigurasi"

### 5.6 `SaveBar`

**Purpose:** Sticky bottom bar with reset and save actions.

```typescript
interface SaveBarProps {
  onReset: () => void;
  onSave: () => Promise<void>;
  isDirty: boolean;
  isSaving: boolean;
  resetLabel?: string;     // Default: "Reset"
  saveLabel?: string;      // Default: "Simpan Perubahan"
}
```

**Behavior:**
- **Sticky**: `position: sticky; bottom: 0;` within settings shell
- **Background**: White with `border-t border-slate-200`
- **Save button disabled**: When `!isDirty` or `isSaving`
- **Save button loading**: Spinner + "Menyimpan..." text
- **Reset button disabled**: When `!isDirty`

---

## 6. API Client Extensions

### New API Module: `lib/settings-api.ts`

```typescript
// lib/settings-api.ts
import api from "./api";

// --- Business Profile ---
export async function getBusinessProfile() {
  const { data } = await api.get("/api/v1/settings/business");
  return data.data;
}

export async function updateBusinessProfile(payload: BusinessProfilePayload) {
  const { data } = await api.put("/api/v1/settings/business", payload);
  return data.data;
}

// --- Channel Configuration ---
export async function getChannelConfig() {
  const { data } = await api.get("/api/v1/settings/channel");
  return data.data;
}

export async function updateChannelConfig(payload: ChannelConfigPayload) {
  const { data } = await api.put("/api/v1/settings/channel", payload);
  return data.data;
}

export async function testChannelConnection() {
  const { data } = await api.post("/api/v1/settings/channel/test");
  return data.data;
}

// --- LLM Configuration ---
export async function getLlmConfig() {
  const { data } = await api.get("/api/v1/settings/llm");
  return data.data;
}

export async function updateLlmConfig(payload: LlmConfigPayload) {
  const { data } = await api.put("/api/v1/settings/llm", payload);
  return data.data;
}

export async function testLlmConnection() {
  const { data } = await api.post("/api/v1/settings/llm/test");
  return data.data;
}

// --- Shipping Configuration ---
export async function getShippingConfig() {
  const { data } = await api.get("/api/v1/settings/shipping");
  return data.data;
}

export async function updateShippingConfig(payload: ShippingConfigPayload) {
  const { data } = await api.put("/api/v1/settings/shipping", payload);
  return data.data;
}

export async function testShippingConnection() {
  const { data } = await api.post("/api/v1/settings/shipping/test");
  return data.data;
}

// --- Payment Configuration ---
export async function getPaymentConfig() {
  const { data } = await api.get("/api/v1/settings/payment");
  return data.data;
}

export async function updatePaymentConfig(payload: PaymentConfigPayload) {
  const { data } = await api.put("/api/v1/settings/payment", payload);
  return data.data;
}

export async function testPaymentConnection() {
  const { data } = await api.post("/api/v1/settings/payment/test");
  return data.data;
}

// --- Settings Summary (for landing page) ---
export async function getSettingsSummary() {
  const { data } = await api.get("/api/v1/settings");
  return data.data; // Returns summary of all sections with status
}
```

### Type Definitions

```typescript
// lib/types/settings.ts

export interface BusinessProfile {
  business_name: string;
  business_description: string | null;
  currency: "IDR" | "USD" | "SGD" | "MYR";
  timezone: string;
  contact_email: string | null;
  contact_phone: string | null;
}

export interface ChannelConfig {
  provider: "whatsapp_business_api" | "third_party";
  // WhatsApp Business API fields
  phone_number_id?: string;
  business_account_id?: string;
  access_token?: string;
  webhook_verify_token?: string;
  // Third-party fields
  third_party_provider?: "wati" | "aichat" | "qiscus" | "whatsapp-cloud";
  third_party_api_key?: string;
  third_party_base_url?: string;
}

export interface LlmConfig {
  provider: "openai" | "anthropic";
  model: string;
  api_key: string;
  org_id?: string; // OpenAI only
}

export interface ShippingConfig {
  api_key: string;
  webhook_token?: string;
  origin_address_line: string;
  origin_city: string;
  origin_district?: string;
  origin_postal_code: string;
  origin_province?: string;
  origin_name?: string;
  origin_phone?: string;
}

export interface PaymentConfig {
  mode: "sandbox" | "production";
  secret_key: string;
  callback_token: string;
  success_redirect_url?: string;
  failure_redirect_url?: string;
}

export interface SettingsSummary {
  business: { status: "active" | "partial" | "empty"; name: string };
  channel: { status: "active" | "partial" | "empty"; provider: string; connected: boolean };
  llm: { status: "active" | "partial" | "empty"; provider: string; model: string; key_preview: string };
  shipping: { status: "active" | "partial" | "empty"; provider: string; key_preview: string };
  payment: { status: "active" | "partial" | "empty"; provider: string; mode: string };
}
```

---

## 7. State Management & Data Flow

### Per-Page State Pattern

Each settings sub-page follows the same state pattern using React Query for server state + local state for form editing:

```typescript
// Pattern for /settings/business (repeated for each sub-page)

function BusinessSettingsPage() {
  // 1. Server state (read)
  const { data, isLoading, isError, refetch } = useQuery({
    queryKey: ["settings", "business"],
    queryFn: getBusinessProfile,
  });

  // 2. Local state (edit buffer)
  const [form, setForm] = useState<BusinessProfile | null>(null);
  const [isDirty, setIsDirty] = useState(false);

  // 3. Sync server → local on first load
  useEffect(() => {
    if (data && !form) {
      setForm(data);
    }
  }, [data]);

  // 4. Mutation (write)
  const mutation = useMutation({
    mutationFn: updateBusinessProfile,
    onSuccess: () => {
      toast.success("Profil bisnis disimpan");
      setIsDirty(false);
      queryClient.invalidateQueries({ queryKey: ["settings"] });
    },
    onError: (error) => {
      toast.error(`Gagal menyimpan: ${error.message}`);
    },
  });

  // 5. Field change handler
  function handleChange(field: keyof BusinessProfile, value: string) {
    setForm((prev) => (prev ? { ...prev, [field]: value } : null));
    setIsDirty(true);
  }

  // 6. Save handler
  async function handleSave() {
    if (!form) return;
    await mutation.mutateAsync(form);
  }

  // 7. Reset handler
  function handleReset() {
    setForm(data ?? null);
    setIsDirty(false);
  }
}
```

### Cross-Page Invalidation

When any settings page saves successfully, invalidate the `["settings"]` query key so the landing page summary cards refresh on next visit:

```typescript
// In every mutation's onSuccess:
queryClient.invalidateQueries({ queryKey: ["settings"] });
```

### Dirty State Tracking

- Track `isDirty` per page
- Breadcrumb shows orange dot when dirty
- `beforeunload` event warns user if navigating away with unsaved changes:

```typescript
useEffect(() => {
  if (!isDirty) return;
  function warn(e: BeforeUnloadEvent) {
    e.preventDefault();
    e.returnValue = "";
  }
  window.addEventListener("beforeunload", warn);
  return () => window.removeEventListener("beforeunload", warn);
}, [isDirty]);
```

---

## 8. Validation & Error Handling UX

### Inline Validation Rules

| Rule | UX |
|------|-----|
| **Required field empty** | Red border + "Wajib diisi" below field on blur |
| **Format mismatch** | Red border + specific message (e.g., "Harus diawali dengan 'sk-'") |
| **Max length exceeded** | Red border + "Maksimal X karakter" + character counter |
| **Invalid email** | Red border + "Format email tidak valid" |
| **Invalid phone** | Red border + "Format telepon tidak valid (contoh: +6281234567890)" |

### Form-Level Validation (on Submit)

1. Validate all fields
2. Collect all errors
3. Scroll to first error field
4. Focus first error field
5. Show error summary toast: "Terdapat X field yang perlu diperbaiki"

### API Error Handling

| HTTP Status | UX Response |
|-------------|-------------|
| 400 | Show field-level errors from response body; scroll to first error |
| 401 | Redirect to login (handled by existing interceptor) |
| 403 | Toast: "Anda tidak memiliki izin untuk mengubah pengaturan." |
| 409 | Toast: "Konflik: [message]. Refresh halaman dan coba lagi." |
| 422 | Show field-level validation errors from backend |
| 500 | Toast: "Gagal menyimpan. Coba lagi dalam beberapa saat." + Retry button |

### Connection Test Error Handling

| Error Type | UX Response |
|------------|-------------|
| **Invalid credentials** | Red card: "Kredensial tidak valid. Periksa API key kamu." |
| **Network timeout** | Red card: "Koneksi timeout. Provider mungkin tidak terjangkau. Coba lagi." |
| **Provider error** | Red card: "Provider error: [message]. Cek dashboard provider kamu." |
| **Unexpected** | Red card: "Error tidak terduga. Coba lagi atau hubungi support." |

---

## 9. Responsive Behavior

### Breakpoints

| Breakpoint | Width | Behavior |
|------------|-------|----------|
| **Mobile** | < 640px | Single column; radio cards stack vertically; save bar full-width |
| **Tablet** | 640px – 1024px | Two-column form rows where applicable; radio cards side-by-side |
| **Desktop** | ≥ 1024px | Full layout as specified; max-width 960px for form content area |

### Mobile-Specific Adjustments

- **Save Bar**: Full-width, buttons stack vertically (Save on top, Reset below)
- **Radio Cards**: Stack vertically, full width
- **Two-Column Rows**: Stack to single column
- **Breadcrumb**: Truncate to "Pengaturan > ..." with ellipsis
- **Sidebar**: Collapsed by default (icon mode)

### Tablet-Specific Adjustments

- **Radio Cards**: Side-by-side, equal width
- **Two-Column Rows**: Maintain two columns
- **Save Bar**: Buttons inline (Reset left, Save right)

---

## 10. Empty States & Edge Cases

### First-Time Setup (All Empty)

**Landing Page (`/settings`):**
- Banner: "🚀 Selamat datang! Lengkapi pengaturan bisnis kamu dulu."
- All 5 cards show gray status dot + "Belum dikonfigurasi"
- "Mulai Konfigurasi" button links to `/settings/business`

**Sub-Pages:**
- Form fields show placeholder text
- Dropdowns show "Pilih..." default option
- Save button enabled (user can save partial config)
- Connection test button disabled until required fields filled

### Partial Configuration

**Landing Page:**
- Cards show yellow dot for partially configured sections
- Primary info shows what's filled (e.g., "OpenAI • (model belum dipilih)")
- Secondary info shows what's missing (e.g., "API Key belum diisi")

**Sub-Pages:**
- Filled fields show saved values
- Empty fields show placeholder
- Save bar active

### Fully Configured

**Landing Page:**
- All cards show green dot
- Primary info shows provider + model/mode
- Secondary info shows masked key preview

### Edge Cases

| Scenario | Handling |
|----------|----------|
| **User switches provider without saving** | Confirmation dialog before clearing form |
| **User pastes invalid API key format** | Inline validation on blur catches prefix mismatch |
| **Connection test fails after save** | Save succeeds independently; test result is ephemeral |
| **Multiple tabs editing same settings** | Last-write-wins (no conflict detection in Phase 1) |
| **User with viewer role accesses /settings** | 403 page or redirect to dashboard with toast |
| **API returns stale data after save** | `invalidateQueries` ensures fresh data on next navigation |
| **Long form scroll** | Save bar is sticky; user always sees save button |

---

## 11. Accessibility Requirements

### Keyboard Navigation

- All form fields focusable via Tab
- Radio cards selectable via Arrow keys (within radio group)
- Save bar buttons focusable
- Connection test result dismissable via Escape
- Masked input reveal toggle operable via Enter/Space

### Screen Reader

- Radio cards: `role="radio"`, `aria-checked`, `aria-labelledby`
- Masked input: `aria-label="API key (masked)"`, reveal button has `aria-label="Tampilkan API key"`
- Connection test: Status announced via `aria-live="polite"` region
- Save confirmation: Toast has `role="status"` `aria-live="polite"`
- Form errors: Error messages linked to fields via `aria-describedby`

### Color & Contrast

- Status dots include text labels (not color-only)
- Error states use both red border AND error icon + text
- All text meets WCAG AA contrast ratio (4.5:1 for normal text, 3:1 for large text)

### Focus Management

- On form validation error: Focus moves to first error field
- On save success: Focus stays on save button (user may want to make more changes)
- On navigation to sub-page: Focus moves to page title (h1)

---

## 12. Implementation Phases

### Phase E1: Foundation (3-4 days)

**Backend:**
- [ ] Create `tenant_settings` database migration (single table or config JSONB column on users table)
- [ ] Create `/api/v1/settings/*` endpoints (GET/PUT for each section + summary GET)
- [ ] Create `/api/v1/settings/*/test` endpoints (connection tests)
- [ ] Add RBAC: `admin` role can write; `manager` can write; `viewer` can only read
- [ ] Add audit logging for settings changes

**Frontend:**
- [ ] Add "Pengaturan" to sidebar navigation
- [ ] Create `/settings` landing page with summary cards
- [ ] Create `SettingsShell` component
- [ ] Create `RadioCardGroup` + `RadioCard` components
- [ ] Create `MaskedInput` component
- [ ] Create `ConnectionTestButton` component
- [ ] Create `SaveBar` component
- [ ] Create `lib/settings-api.ts` API module
- [ ] Create `lib/types/settings.ts` type definitions

### Phase E2: Business + Channel (2-3 days)

- [ ] Implement `/settings/business` page (Business Profile form)
- [ ] Implement `/settings/channel` page (Channel selection + conditional forms)
- [ ] Wire up API calls with React Query
- [ ] Add form validation
- [ ] Add connection test for channel
- [ ] Add dirty state tracking + beforeunload warning

### Phase E3: LLM + Shipping (2-3 days)

- [ ] Implement `/settings/llm` page (LLM provider selection + model picker + API key)
- [ ] Implement `/settings/shipping` page (Biteship credentials + origin address)
- [ ] Wire up API calls
- [ ] Add model info cards (dynamic based on selection)
- [ ] Add connection tests for LLM and shipping

### Phase E4: Payment + Polish (2-3 days)

- [ ] Implement `/settings/payment` page (Xendit credentials + mode toggle)
- [ ] Wire up API calls
- [ ] Add mode toggle with production warning
- [ ] Add webhook URL display
- [ ] Polish: loading skeletons, empty states, error states
- [ ] Accessibility audit
- [ ] Responsive testing (mobile/tablet/desktop)
- [ ] End-to-end integration testing

### Phase E5: Documentation & Deployment (1 day)

- [ ] Update admin frontend roadmap docs
- [ ] Create testing instructions
- [ ] Final review against design guidelines
- [ ] Deploy to staging
- [ ] User acceptance testing

---

## Appendix A: Icon Mapping

| Context | lucide-react Icon |
|---------|-------------------|
| Sidebar — Pengaturan | `Settings` |
| Landing — Bisnis card | `Building2` |
| Landing — Channel card | `MessageCircle` |
| Landing — LLM card | `Brain` |
| Landing — Pengiriman card | `Truck` |
| Landing — Pembayaran card | `CreditCard` |
| Save bar — Reset | `Undo2` (or no icon) |
| Save bar — Save | `Save` (or no icon) |
| Connection test — Idle | `Plug` |
| Connection test — Success | `CheckCircle2` (green) |
| Connection test — Failure | `XCircle` (red) |
| Masked input — Reveal | `Eye` / `EyeOff` |
| Masked input — Copy | `ClipboardCopy` |
| Breadcrumb separator | `ChevronRight` |
| Status dot — Active | `Circle` (fill green-500) |
| Status dot — Partial | `Circle` (fill yellow-500) |
| Status dot — Empty | `Circle` (fill slate-300) |

---

## Appendix B: Color Tokens

| Token | Tailwind Class | Usage |
|-------|---------------|-------|
| Selected card border | `border-blue-500` | Radio card selected state |
| Selected card bg | `bg-blue-50` | Radio card selected state |
| Success green | `text-green-600`, `bg-green-50`, `border-green-200` | Connection test success, status dot |
| Warning yellow | `text-yellow-600`, `bg-yellow-50`, `border-yellow-200` | Partial status, production mode warning |
| Error red | `text-red-600`, `bg-red-50`, `border-red-200` | Validation errors, connection test failure |
| Neutral gray | `text-slate-500`, `bg-slate-50`, `border-slate-200` | Empty states, disabled buttons |
| Primary blue | `bg-blue-600`, `text-white` | Save button, test button |
| Masked chars | `text-slate-400` | Masked input dots |

---

## Appendix C: Toast Messages Reference

| Context | Type | Message |
|---------|------|---------|
| Save success | `success` | "[section] disimpan" (e.g., "Profil bisnis disimpan") |
| Save error | `error` | "Gagal menyimpan. [error]. Coba lagi." |
| Connection test success | `success` | "Koneksi ke [provider] berhasil!" |
| Connection test failure | `error` | "Koneksi gagal: [error]" |
| Token copied | `success` | "Token disalin ke clipboard" |
| Webhook URL copied | `success` | "URL webhook disalin" |
| Provider switch warning | `warning` | Dialog, not toast (see Section 4.3) |
| Production mode warning | `warning` | Dialog, not toast (see Section 4.6) |
| Unsaved changes | `warning` | Browser native `beforeunload` dialog |

---

**Document Version:** 1.0  
**Last Updated:** 2026-07-23  
**Author:** Kiro AI Assistant  
**Review Status:** Pending User Review

---

*This document is part of the AI Sales Platform Enhancement — Tenant Configuration project.*