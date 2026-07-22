"use client";

import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ChevronLeft } from "lucide-react";
import api from "@/lib/api";
import { VoucherForm } from "@/components/vouchers/voucher-form";
import { Button } from "@/components/ui/button";
import { Voucher, VoucherPayload } from "@/lib/vouchers";

function apiErrorMessage(error: unknown, fallback: string) {
  if (
    typeof error === "object" &&
    error !== null &&
    "response" in error &&
    typeof error.response === "object" &&
    error.response !== null &&
    "data" in error.response &&
    typeof error.response.data === "object" &&
    error.response.data !== null &&
    "message" in error.response.data &&
    typeof error.response.data.message === "string"
  ) {
    return error.response.data.message;
  }

  return fallback;
}

export default function EditVoucherPage() {
  const { id } = useParams<{ id: string }>();
  const router = useRouter();
  const queryClient = useQueryClient();
  const [errorMessage, setErrorMessage] = useState("");

  const {
    data: voucher,
    isLoading,
    isError,
  } = useQuery<Voucher>({
    queryKey: ["vouchers", id],
    queryFn: async () => {
      const response = await api.get(`/api/v1/vouchers/${id}`);
      return response.data.data;
    },
    enabled: Boolean(id),
  });

  const updateVoucher = useMutation({
    mutationFn: async (payload: VoucherPayload) => {
      const updatePayload = Object.fromEntries(
        Object.entries(payload).filter(([key]) => key !== "code")
      );
      const response = await api.put(`/api/v1/vouchers/${id}`, updatePayload);
      return response.data.data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["vouchers"] });
      queryClient.invalidateQueries({ queryKey: ["vouchers", id] });
      router.push("/promos");
    },
    onError: (error: unknown) => {
      setErrorMessage(
        apiErrorMessage(error, "Unable to update voucher. Please try again.")
      );
    },
  });

  if (isLoading) {
    return <p className="text-sm text-slate-600">Loading voucher...</p>;
  }

  if (isError || !voucher) {
    return (
      <div className="space-y-4">
        <p className="text-sm text-red-800">
          Unable to load this voucher. It may have been removed.
        </p>
        <Button variant="outline" asChild>
          <Link href="/promos">Back to promo</Link>
        </Button>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-3xl space-y-6">
      <div className="flex items-center gap-4">
        <Button variant="ghost" size="icon" asChild>
          <Link href="/promos" aria-label="Back to promo">
            <ChevronLeft className="h-4 w-4" />
          </Link>
        </Button>
        <div>
          <h1 className="text-2xl font-semibold tracking-tight text-slate-950">
            Edit voucher
          </h1>
          <p className="mt-1 text-sm text-slate-600">
            Update availability and rules for <span className="font-medium">{voucher.code}</span>.
          </p>
        </div>
      </div>

      <div className="rounded-md border border-slate-200 bg-white p-5 sm:p-6">
        <VoucherForm
          key={voucher.id}
          voucher={voucher}
          submitLabel="Save changes"
          isSubmitting={updateVoucher.isPending}
          errorMessage={errorMessage}
          onSubmit={(payload) => {
            setErrorMessage("");
            updateVoucher.mutate(payload);
          }}
        />
      </div>
    </div>
  );
}