"use client";

import Link from "next/link";
import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Edit, Plus, Search, Trash2 } from "lucide-react";
import api from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  formatCurrency,
  formatDateTime,
  Voucher,
  voucherState,
} from "@/lib/vouchers";

export default function PromosPage() {
  const queryClient = useQueryClient();
  const [query, setQuery] = useState("");
  const [deleteError, setDeleteError] = useState("");

  const { data: vouchers = [], isLoading, isError } = useQuery<Voucher[]>({
    queryKey: ["vouchers"],
    queryFn: async () => {
      const response = await api.get("/api/v1/vouchers");
      return response.data.data;
    },
  });

  const deleteVoucher = useMutation({
    mutationFn: async (voucherId: string) => {
      await api.delete(`/api/v1/vouchers/${voucherId}`);
    },
    onSuccess: () => {
      setDeleteError("");
      queryClient.invalidateQueries({ queryKey: ["vouchers"] });
    },
    onError: () => {
      setDeleteError("Unable to delete this voucher. Please try again.");
    },
  });

  const filteredVouchers = useMemo(() => {
    const normalizedQuery = query.trim().toLowerCase();
    if (!normalizedQuery) return vouchers;

    return vouchers.filter((voucher) =>
      [voucher.code, voucher.description]
        .filter(Boolean)
        .some((value) => value.toLowerCase().includes(normalizedQuery))
    );
  }, [query, vouchers]);

  const handleDelete = (voucher: Voucher) => {
    if (
      window.confirm(
        `Delete voucher ${voucher.code}? This action cannot be undone.`
      )
    ) {
      deleteVoucher.mutate(voucher.id);
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex flex-col justify-between gap-4 sm:flex-row sm:items-start">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight text-slate-950">
            Promo
          </h1>
          <p className="mt-1 text-sm text-slate-600">
            Create and manage vouchers available to customers.
          </p>
        </div>
        <Button asChild>
          <Link href="/promos/new">
            <Plus className="mr-2 h-4 w-4" />
            New voucher
          </Link>
        </Button>
      </div>

      <div className="flex max-w-md items-center gap-2">
        <Search className="h-4 w-4 shrink-0 text-slate-600" />
        <Input
          aria-label="Search vouchers"
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          placeholder="Search code or description"
        />
      </div>

      {deleteError ? (
        <div role="alert" className="rounded-md bg-red-50 px-4 py-3 text-sm text-red-800">
          {deleteError}
        </div>
      ) : null}

      <div className="overflow-hidden rounded-md border border-slate-200 bg-white">
        <div className="overflow-x-auto">
          <table className="w-full min-w-[780px] text-left text-sm">
            <thead className="border-b border-slate-200 bg-slate-50 text-xs font-medium text-slate-700">
              <tr>
                <th scope="col" className="px-4 py-3">Code</th>
                <th scope="col" className="px-4 py-3">Discount</th>
                <th scope="col" className="px-4 py-3">Requirements</th>
                <th scope="col" className="px-4 py-3">Usage</th>
                <th scope="col" className="px-4 py-3">Expires</th>
                <th scope="col" className="px-4 py-3">Status</th>
                <th scope="col" className="w-24 px-4 py-3 text-right">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-200">
              {isLoading ? (
                <tr>
                  <td colSpan={7} className="px-4 py-12 text-center text-slate-600">
                    Loading vouchers...
                  </td>
                </tr>
              ) : isError ? (
                <tr>
                  <td colSpan={7} className="px-4 py-12 text-center text-red-800">
                    Unable to load vouchers. Refresh the page to try again.
                  </td>
                </tr>
              ) : filteredVouchers.length === 0 ? (
                <tr>
                  <td colSpan={7} className="px-4 py-12 text-center text-slate-600">
                    {query ? "No vouchers match your search." : "No vouchers have been created."}
                  </td>
                </tr>
              ) : (
                filteredVouchers.map((voucher) => {
                  const state = voucherState(voucher);
                  const discount =
                    voucher.discount_type === "percent"
                      ? `${voucher.discount_value}%`
                      : formatCurrency(voucher.discount_value);

                  return (
                    <tr key={voucher.id} className="hover:bg-slate-50">
                      <td className="px-4 py-3 align-top">
                        <p className="font-semibold text-slate-950">{voucher.code}</p>
                        {voucher.description ? (
                          <p className="mt-0.5 max-w-[260px] truncate text-xs text-slate-600">
                            {voucher.description}
                          </p>
                        ) : null}
                      </td>
                      <td className="px-4 py-3 align-top font-medium text-slate-900">
                        {discount}
                      </td>
                      <td className="px-4 py-3 align-top text-slate-700">
                        {voucher.min_order_amount > 0
                          ? `Min. ${formatCurrency(voucher.min_order_amount)}`
                          : "No minimum"}
                      </td>
                      <td className="px-4 py-3 align-top text-slate-700">
                        {voucher.quota > 0
                          ? `${voucher.used_count} / ${voucher.quota}`
                          : `${voucher.used_count} / ∞`}
                      </td>
                      <td className="px-4 py-3 align-top text-slate-700">
                        {formatDateTime(voucher.expires_at)}
                      </td>
                      <td className="px-4 py-3 align-top">
                        <span className={`inline-flex rounded-md px-2 py-1 text-xs font-medium ${state.className}`}>
                          {state.label}
                        </span>
                      </td>
                      <td className="px-4 py-3 align-top">
                        <div className="flex justify-end gap-1">
                          <Button variant="ghost" size="icon" asChild>
                            <Link href={`/promos/${voucher.id}/edit`} aria-label={`Edit ${voucher.code}`}>
                              <Edit className="h-4 w-4" />
                            </Link>
                          </Button>
                          <Button
                            variant="ghost"
                            size="icon"
                            disabled={deleteVoucher.isPending}
                            onClick={() => handleDelete(voucher)}
                            aria-label={`Delete ${voucher.code}`}
                            className="text-red-700 hover:bg-red-50 hover:text-red-800"
                          >
                            <Trash2 className="h-4 w-4" />
                          </Button>
                        </div>
                      </td>
                    </tr>
                  );
                })
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}