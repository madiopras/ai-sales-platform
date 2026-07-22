"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { ChevronLeft } from "lucide-react";
import api from "@/lib/api";
import { VoucherForm } from "@/components/vouchers/voucher-form";
import { Button } from "@/components/ui/button";
import { VoucherPayload } from "@/lib/vouchers";

export default function NewVoucherPage() {
  const router = useRouter();
  const queryClient = useQueryClient();
  const [errorMessage, setErrorMessage] = useState("");

  const createVoucher = useMutation({
    mutationFn: async (payload: VoucherPayload) => {
      const response = await api.post("/api/v1/vouchers", payload);
      return response.data.data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["vouchers"] });
      router.push("/promos");
    },
    onError: (error: unknown) => {
      const message =
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
          ? error.response.data.message
          : "Unable to create voucher. Please review the form and try again.";

      setErrorMessage(message);
    },
  });

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
            New voucher
          </h1>
          <p className="mt-1 text-sm text-slate-600">
            Define a customer-facing promotional voucher.
          </p>
        </div>
      </div>

      <div className="rounded-md border border-slate-200 bg-white p-5 sm:p-6">
        <VoucherForm
          submitLabel="Create voucher"
          isSubmitting={createVoucher.isPending}
          errorMessage={errorMessage}
          onSubmit={(payload) => {
            setErrorMessage("");
            createVoucher.mutate(payload);
          }}
        />
      </div>
    </div>
  );
}