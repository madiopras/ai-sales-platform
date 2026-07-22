export type DiscountType = "percent" | "fixed";

export interface Voucher {
  id: string;
  code: string;
  description: string;
  discount_type: DiscountType;
  discount_value: number;
  min_order_amount: number;
  quota: number;
  used_count: number;
  starts_at: string;
  expires_at: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export interface VoucherPayload {
  code?: string;
  description: string;
  discount_type: DiscountType;
  discount_value: number;
  min_order_amount: number;
  quota: number;
  starts_at?: string;
  expires_at: string;
  is_active: boolean;
}

export function formatCurrency(value: number) {
  return new Intl.NumberFormat("id-ID", {
    style: "currency",
    currency: "IDR",
    maximumFractionDigits: 0,
  }).format(value);
}

export function formatDateTime(value?: string) {
  if (!value) return "—";

  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "—";

  return new Intl.DateTimeFormat("id-ID", {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(date);
}

export function toDateTimeLocal(value?: string) {
  if (!value) return "";

  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";

  const offset = date.getTimezoneOffset() * 60_000;
  return new Date(date.getTime() - offset).toISOString().slice(0, 16);
}

export function voucherState(voucher: Voucher) {
  const now = new Date();
  const startsAt = voucher.starts_at ? new Date(voucher.starts_at) : null;
  const expiresAt = new Date(voucher.expires_at);

  if (!voucher.is_active) return { label: "Inactive", className: "bg-slate-100 text-slate-700" };
  if (startsAt && startsAt > now) return { label: "Scheduled", className: "bg-blue-50 text-blue-700" };
  if (!Number.isNaN(expiresAt.getTime()) && expiresAt < now) {
    return { label: "Expired", className: "bg-amber-50 text-amber-800" };
  }
  if (voucher.quota > 0 && voucher.used_count >= voucher.quota) {
    return { label: "Exhausted", className: "bg-amber-50 text-amber-800" };
  }

  return { label: "Active", className: "bg-emerald-50 text-emerald-700" };
}