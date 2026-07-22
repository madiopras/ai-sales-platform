"use client";

import { useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ChevronLeft } from "lucide-react";
import Link from "next/link";
import api from "@/lib/api";
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

interface Category {
  id: string;
  name: string;
}

type ProductStatus = "draft" | "active" | "inactive";

interface ProductFormData {
  name: string;
  slug: string;
  category_id: string;
  description: string;
  status: ProductStatus;
}

function slugify(value: string) {
  return value
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
}

function getErrorMessage(error: unknown) {
  if (error && typeof error === "object" && "response" in error) {
    const response = (error as { response?: { data?: { error?: { message?: string } } } }).response;
    return response?.data?.error?.message || "Produk gagal disimpan.";
  }

  return "Produk gagal disimpan.";
}

export default function NewProductPage() {
  const router = useRouter();
  const queryClient = useQueryClient();
  const [formData, setFormData] = useState<ProductFormData>({
    name: "",
    slug: "",
    category_id: "",
    description: "",
    status: "draft",
  });
  const [manualSlug, setManualSlug] = useState(false);

  const { data: categories, isLoading: categoriesLoading } = useQuery<Category[]>({
    queryKey: ["categories"],
    queryFn: async () => {
      const response = await api.get("/api/v1/categories");
      return response.data.data;
    },
  });

  const selectedCategoryName = useMemo(() => {
    return categories?.find((category) => category.id === formData.category_id)?.name;
  }, [categories, formData.category_id]);

  const createProduct = useMutation({
    mutationFn: async (data: ProductFormData) => {
      const payload = {
        category_id: data.category_id,
        name: data.name.trim(),
        slug: slugify(data.slug || data.name),
        description: data.description.trim(),
        status: data.status,
      };

      const response = await api.post("/api/v1/products", payload);
      return response.data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["products"] });
      router.push("/products");
    },
  });

  const handleNameChange = (value: string) => {
    setFormData((current) => ({
      ...current,
      name: value,
      slug: manualSlug ? current.slug : slugify(value),
    }));
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    createProduct.mutate(formData);
  };

  const isSubmitDisabled =
    createProduct.isPending ||
    !formData.name.trim() ||
    !formData.category_id ||
    !slugify(formData.slug || formData.name);

  return (
    <div className="mx-auto max-w-2xl space-y-6">
      <div className="flex items-center space-x-4">
        <Button variant="ghost" size="icon" asChild>
          <Link href="/products">
            <ChevronLeft className="h-4 w-4" />
          </Link>
        </Button>
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">New Product</h1>
          <p className="text-sm text-slate-600">Add catalog metadata first, then manage variants for price and stock.</p>
        </div>
      </div>

      <div className="rounded-md border bg-white p-6">
        <form onSubmit={handleSubmit} className="space-y-6">
          {createProduct.isError ? (
            <div className="rounded-md border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
              {getErrorMessage(createProduct.error)}
            </div>
          ) : null}

          <div className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="name">Name</Label>
              <Input
                id="name"
                required
                value={formData.name}
                onChange={(e) => handleNameChange(e.target.value)}
                placeholder="e.g., Premium Wireless Headphones"
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="slug">Slug</Label>
              <Input
                id="slug"
                required
                value={formData.slug}
                onChange={(e) => {
                  setManualSlug(true);
                  setFormData({ ...formData, slug: slugify(e.target.value) });
                }}
                placeholder="premium-wireless-headphones"
              />
              <p className="text-xs text-slate-600">Used in catalog URLs. Keep it short, lowercase, and unique.</p>
            </div>

            <div className="space-y-2">
              <Label htmlFor="category">Category</Label>
              <Select
                value={formData.category_id}
                onValueChange={(value) => setFormData({ ...formData, category_id: value ?? "" })}
                disabled={categoriesLoading}
              >
                <SelectTrigger id="category">
                  <SelectValue placeholder={categoriesLoading ? "Loading categories..." : "Select a category"} />
                </SelectTrigger>
                <SelectContent>
                  {categories?.map((category) => (
                    <SelectItem key={category.id} value={category.id}>
                      {category.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {selectedCategoryName ? (
                <p className="text-xs text-slate-600">Selected: {selectedCategoryName}</p>
              ) : null}
            </div>

            <div className="space-y-2">
              <Label htmlFor="status">Status</Label>
              <Select
                value={formData.status}
                onValueChange={(value) =>
                  setFormData({ ...formData, status: (value ?? "draft") as ProductStatus })
                }
              >
                <SelectTrigger id="status">
                  <SelectValue placeholder="Select product status" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="draft">Draft</SelectItem>
                  <SelectItem value="active">Active</SelectItem>
                  <SelectItem value="inactive">Inactive</SelectItem>
                </SelectContent>
              </Select>
              <p className="text-xs text-slate-600">Only active products should be visible in customer-facing catalog.</p>
            </div>

            <div className="space-y-2">
              <Label htmlFor="description">Description</Label>
              <Textarea
                id="description"
                value={formData.description}
                onChange={(e) => setFormData({ ...formData, description: e.target.value })}
                placeholder="Detailed description of the product"
                rows={4}
              />
            </div>

            <div className="rounded-md bg-slate-50 px-4 py-3 text-sm text-slate-700">
              Price and stock are managed per variant after this product is created.
            </div>
          </div>

          <div className="flex justify-end space-x-2 pt-4">
            <Button variant="outline" asChild>
              <Link href="/products">Cancel</Link>
            </Button>
            <Button type="submit" disabled={isSubmitDisabled}>
              {createProduct.isPending ? "Creating..." : "Create Product"}
            </Button>
          </div>
        </form>
      </div>
    </div>
  );
}