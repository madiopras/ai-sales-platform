"use client";

import { useMemo } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Plus, Trash2 } from "lucide-react";
import Link from "next/link";
import api from "@/lib/api";
import { Button } from "@/components/ui/button";

type ProductStatus = "draft" | "active" | "inactive";

interface ProductVariant {
  id: string;
  price?: number;
  stock?: number;
  stock_on_hand?: number;
  stock_reserved?: number;
  is_active: boolean;
}

interface Product {
  id: string;
  category_id: string;
  name: string;
  slug: string;
  description: string;
  status: ProductStatus;
  variants?: ProductVariant[];
  created_at: string;
  updated_at: string;
}

interface Category {
  id: string;
  name: string;
}

function getErrorMessage(error: unknown) {
  if (error && typeof error === "object" && "response" in error) {
    const response = (error as { response?: { data?: { error?: { message?: string } } } }).response;
    return response?.data?.error?.message || "Produk gagal dihapus.";
  }

  return "Produk gagal dihapus.";
}

function formatVariantSummary(product: Product) {
  const variants = product.variants ?? [];

  if (variants.length === 0) {
    return "No variants";
  }

  const activeVariants = variants.filter((variant) => variant.is_active).length;
  return `${variants.length} variant${variants.length > 1 ? "s" : ""} · ${activeVariants} active`;
}

function formatPriceSummary(product: Product) {
  const prices = (product.variants ?? [])
    .map((variant) => variant.price)
    .filter((price): price is number => typeof price === "number");

  if (prices.length === 0) {
    return "Set in variants";
  }

  const minPrice = Math.min(...prices);
  const maxPrice = Math.max(...prices);

  if (minPrice === maxPrice) {
    return `Rp ${minPrice.toLocaleString("id-ID")}`;
  }

  return `Rp ${minPrice.toLocaleString("id-ID")}–${maxPrice.toLocaleString("id-ID")}`;
}

function getStatusClass(status: ProductStatus) {
  if (status === "active") {
    return "bg-green-50 text-green-700";
  }

  if (status === "inactive") {
    return "bg-slate-100 text-slate-700";
  }

  return "bg-amber-50 text-amber-800";
}

export default function ProductsPage() {
  const queryClient = useQueryClient();

  const { data: products, isLoading, error } = useQuery<Product[]>({
    queryKey: ["products"],
    queryFn: async () => {
      const response = await api.get("/api/v1/products");
      return response.data.data;
    },
  });

  const { data: categories } = useQuery<Category[]>({
    queryKey: ["categories"],
    queryFn: async () => {
      const response = await api.get("/api/v1/categories");
      return response.data.data;
    },
  });

  const categoryById = useMemo(() => {
    return new Map((categories ?? []).map((category) => [category.id, category.name]));
  }, [categories]);

  const deleteProduct = useMutation({
    mutationFn: async (product: Product) => {
      await api.delete(`/api/v1/products/${product.id}`);
      return product;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["products"] });
    },
  });

  const handleDelete = (product: Product) => {
    if (window.confirm(`Yakin ingin menghapus produk "${product.name}"? Aksi ini tidak dapat dibatalkan.`)) {
      deleteProduct.mutate(product);
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Products</h1>
          <p className="text-sm text-slate-600">Manage catalog metadata, publication status, and variants.</p>
        </div>
        <Button asChild>
          <Link href="/products/new">
            <Plus className="mr-2 h-4 w-4" />
            New Product
          </Link>
        </Button>
      </div>

      {deleteProduct.isError ? (
        <div className="rounded-md border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
          {getErrorMessage(deleteProduct.error)}
        </div>
      ) : null}

      <div className="rounded-md border bg-white">
        {isLoading ? (
          <div className="space-y-3 p-6">
            <div className="h-4 w-48 rounded bg-slate-100" />
            <div className="h-10 rounded bg-slate-100" />
            <div className="h-10 rounded bg-slate-100" />
            <div className="h-10 rounded bg-slate-100" />
          </div>
        ) : error ? (
          <div className="p-8 text-center text-sm text-red-600">Failed to load products</div>
        ) : products?.length === 0 ? (
          <div className="p-8 text-center">
            <h2 className="text-sm font-medium text-slate-900">No products yet</h2>
            <p className="mx-auto mt-1 max-w-sm text-sm text-slate-600">
              Create a product record first, then add variants to define price and stock.
            </p>
            <Button variant="outline" className="mt-4" asChild>
              <Link href="/products/new">Create your first product</Link>
            </Button>
          </div>
        ) : (
          <div className="relative w-full overflow-auto">
            <table className="w-full caption-bottom text-sm">
              <thead className="[&_tr]:border-b">
                <tr className="border-b transition-colors hover:bg-slate-50/50 data-[state=selected]:bg-slate-50">
                  <th className="h-12 px-4 text-left align-middle font-medium text-slate-600">Name</th>
                  <th className="h-12 px-4 text-left align-middle font-medium text-slate-600">Category</th>
                  <th className="h-12 px-4 text-left align-middle font-medium text-slate-600">Variants</th>
                  <th className="h-12 px-4 text-right align-middle font-medium text-slate-600">Price</th>
                  <th className="h-12 px-4 text-center align-middle font-medium text-slate-600">Status</th>
                  <th className="h-12 px-4 text-right align-middle font-medium text-slate-600">Actions</th>
                </tr>
              </thead>
              <tbody className="[&_tr:last-child]:border-0">
                {products?.map((product) => (
                  <tr
                    key={product.id}
                    className="border-b transition-colors hover:bg-slate-50/50 data-[state=selected]:bg-slate-50"
                  >
                    <td className="p-4 align-middle">
                      <div className="font-medium text-slate-900">{product.name}</div>
                      <div className="text-xs text-slate-600">{product.slug}</div>
                    </td>
                    <td className="p-4 align-middle text-slate-700">
                      {categoryById.get(product.category_id) || "Uncategorized"}
                    </td>
                    <td className="p-4 align-middle text-slate-700">{formatVariantSummary(product)}</td>
                    <td className="p-4 align-middle text-right text-slate-900">{formatPriceSummary(product)}</td>
                    <td className="p-4 align-middle text-center">
                      <span
                        className={`inline-flex items-center rounded-full px-2 py-1 text-xs font-medium ${getStatusClass(
                          product.status,
                        )}`}
                      >
                        {product.status}
                      </span>
                    </td>
                    <td className="p-4 align-middle text-right">
                      <div className="flex justify-end gap-2">
                        <Button variant="ghost" size="sm" asChild>
                          <Link href={`/products/${product.id}`}>Variants</Link>
                        </Button>
                        <Button variant="ghost" size="sm" asChild>
                          <Link href={`/products/${product.id}/edit`}>Edit</Link>
                        </Button>
                        <Button
                          type="button"
                          variant="ghost"
                          size="sm"
                          className="text-red-700 hover:bg-red-50 hover:text-red-800"
                          onClick={() => handleDelete(product)}
                          disabled={deleteProduct.isPending}
                          aria-label={`Delete ${product.name}`}
                        >
                          <Trash2 className="h-4 w-4" />
                        </Button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
}