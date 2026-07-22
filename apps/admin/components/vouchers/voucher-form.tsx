"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  DiscountType,
  Voucher,
  VoucherPayload,
  toDateTimeLocal,
} from "@/lib/vouchers";

interface VoucherFormProps {
  voucher?: Voucher;
  onSubmit: (payload: VoucherPayload) => void;
  isSubmitting?: boolean;
  submitLabel: string;
  errorMessage?: string;
}

interface VoucherFormData {
  code: string;
  description: string;
  discountType: DiscountType;
  discountValue: string;
  minOrderAmount: string;
  quota: string;
  startsAt: string;
  expiresAt: string;
  isActive: boolean;
}

function buildInitialData(voucher?: Voucher): VoucherFormData {
  return {
    code: voucher?.code ?? "",
    description: voucher?.description ?? "",
    discountType: voucher?.discount_type ?? "percent",
    discountValue: voucher ? String(voucher.discount_value) : "",
    minOrderAmount: voucher ? String(voucher.min_order_amount) : "0",
    quota: voucher ? String(voucher.quota) : "0",
    startsAt: toDateTimeLocal(voucher?.starts_at),
    expiresAt: toDateTimeLocal(voucher?.expires_at),
    isActive: voucher?.is_active ?? true,
  };
}

export function VoucherForm({
  voucher,
  onSubmit,
  isSubmitting = false,
  submitLabel,
  errorMessage,
}: VoucherFormProps) {
  const [formData, setFormData] = useState<VoucherFormData>(() =>
    buildInitialData(voucher)
  );
  const [validationError, setValidationError] = useState("");

  const updateField = <K extends keyof VoucherFormData>(
    field: K,
    value: VoucherFormData[K]
  ) => {
    setFormData((previous) => ({ ...previous, [field]: value }));
  };

  const handleSubmit = (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    const discountValue = Number(formData.discountValue);
    const minOrderAmount = Number(formData.minOrderAmount || "0");
    const quota = Number(formData.quota || "0");
    const expiresAt = formData.expiresAt ? new Date(formData.expiresAt) : null;
    const startsAt = formData.startsAt ? new Date(formData.startsAt) : null;

    if (!formData.code.trim() && !voucher) {
      setValidationError("Voucher code is required.");
      return;
    }
    if (!Number.isFinite(discountValue) || discountValue <= 0) {
      setValidationError("Discount value must be greater than zero.");
      return;
    }
    if (
      formData.discountType === "percent" &&
      (discountValue <= 0 || discountValue > 100)
    ) {
      setValidationError("Percentage discount must be between 0 and 100.");
      return;
    }
    if (!Number.isFinite(minOrderAmount) || minOrderAmount < 0) {
      setValidationError("Minimum order amount cannot be negative.");
      return;
    }
    if (!Number.isInteger(quota) || quota < 0) {
      setValidationError("Quota must be a whole number of zero or more.");
      return;
    }
    if (!expiresAt || Number.isNaN(expiresAt.getTime())) {
      setValidationError("An expiry date and time is required.");
      return;
    }
    if (startsAt && startsAt >= expiresAt) {
      setValidationError("The start time must be before the expiry time.");
      return;
    }

    setValidationError("");
    onSubmit({
      ...(voucher ? {} : { code: formData.code.trim().toUpperCase() }),
      description: formData.description.trim(),
      discount_type: formData.discountType,
      discount_value: discountValue,
      min_order_amount: minOrderAmount,
      quota,
      ...(startsAt ? { starts_at: startsAt.toISOString() } : {}),
      expires_at: expiresAt.toISOString(),
      is_active: formData.isActive,
    });
  };

  const shownError = validationError || errorMessage;

  return (
    <form onSubmit={handleSubmit} className="space-y-7">
      {shownError ? (
        <div
          role="alert"
          className="rounded-md bg-red-50 px-4 py-3 text-sm text-red-800"
        >
          {shownError}
        </div>
      ) : null}

      <div className="grid gap-5 sm:grid-cols-2">
        <div className="space-y-2">
          <Label htmlFor="code">Voucher code</Label>
          <Input
            id="code"
            required={!voucher}
            disabled={Boolean(voucher)}
            value={formData.code}
            onChange={(event) =>
              updateField("code", event.target.value.toUpperCase())
            }
            placeholder="HEMAT10"
            className="font-medium uppercase"
          />
          <p className="text-xs text-slate-600">
            {voucher
              ? "Voucher code cannot be changed after creation."
              : "Use a short code customers can easily enter."}
          </p>
        </div>

        <div className="space-y-2">
          <Label htmlFor="discount-type">Discount type</Label>
          <Select
            value={formData.discountType}
            onValueChange={(value) =>
              updateField("discountType", value as DiscountType)
            }
          >
            <SelectTrigger id="discount-type">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="percent">Percentage (%)</SelectItem>
              <SelectItem value="fixed">Fixed amount (IDR)</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>

      <div className="space-y-2">
        <Label htmlFor="description">Description</Label>
        <Textarea
          id="description"
          value={formData.description}
          onChange={(event) => updateField("description", event.target.value)}
          placeholder="e.g. Launch promotion for new customers"
          rows={3}
        />
      </div>

      <div className="grid gap-5 sm:grid-cols-3">
        <div className="space-y-2">
          <Label htmlFor="discount-value">
            Discount value {formData.discountType === "percent" ? "(%)" : "(Rp)"}
          </Label>
          <Input
            id="discount-value"
            type="number"
            min="0"
            max={formData.discountType === "percent" ? "100" : undefined}
            step={formData.discountType === "percent" ? "0.01" : "1"}
            required
            value={formData.discountValue}
            onChange={(event) =>
              updateField("discountValue", event.target.value)
            }
            placeholder={formData.discountType === "percent" ? "10" : "50000"}
          />
        </div>

        <div className="space-y-2">
          <Label htmlFor="minimum-order">Minimum order (Rp)</Label>
          <Input
            id="minimum-order"
            type="number"
            min="0"
            step="1"
            value={formData.minOrderAmount}
            onChange={(event) =>
              updateField("minOrderAmount", event.target.value)
            }
          />
          <p className="text-xs text-slate-600">Set 0 for no minimum.</p>
        </div>

        <div className="space-y-2">
          <Label htmlFor="quota">Redemption quota</Label>
          <Input
            id="quota"
            type="number"
            min="0"
            step="1"
            value={formData.quota}
            onChange={(event) => updateField("quota", event.target.value)}
          />
          <p className="text-xs text-slate-600">Set 0 for unlimited use.</p>
        </div>
      </div>

      <div className="grid gap-5 sm:grid-cols-2">
        <div className="space-y-2">
          <Label htmlFor="starts-at">Starts at</Label>
          <Input
            id="starts-at"
            type="datetime-local"
            value={formData.startsAt}
            onChange={(event) => updateField("startsAt", event.target.value)}
          />
          <p className="text-xs text-slate-600">Leave empty to start immediately.</p>
        </div>

        <div className="space-y-2">
          <Label htmlFor="expires-at">Expires at</Label>
          <Input
            id="expires-at"
            type="datetime-local"
            required
            value={formData.expiresAt}
            onChange={(event) => updateField("expiresAt", event.target.value)}
          />
        </div>
      </div>

      <label className="flex items-start gap-3 rounded-md bg-slate-50 px-4 py-3">
        <input
          type="checkbox"
          checked={formData.isActive}
          onChange={(event) => updateField("isActive", event.target.checked)}
          className="mt-0.5 h-4 w-4 rounded border-slate-300 text-slate-900 focus:ring-slate-900"
        />
        <span>
          <span className="block text-sm font-medium text-slate-900">
            Voucher is active
          </span>
          <span className="block text-xs text-slate-600">
            An inactive voucher cannot be redeemed, even within its validity period.
          </span>
        </span>
      </label>

      <div className="flex justify-end">
        <Button type="submit" disabled={isSubmitting}>
          {isSubmitting ? "Saving..." : submitLabel}
        </Button>
      </div>
    </form>
  );
}