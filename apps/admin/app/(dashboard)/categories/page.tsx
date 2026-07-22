"use client";

import { useQuery } from "@tanstack/react-query";
import { Plus } from "lucide-react";
import Link from "next/link";
import api from "@/lib/api";
import { Button } from "@/components/ui/button";

interface Category {
  id: string;
  name: string;
  slug: string;
  description: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export default function CategoriesPage() {
  const { data: categories, isLoading, error } = useQuery<Category[]>({
    queryKey: ["categories"],
    queryFn: async () => {
      const response = await api.get("/api/v1/categories");
      return response.data.data;
    },
  });

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Categories</h1>
          <p className="text-sm text-slate-500">Manage product categories</p>
        </div>
        <Button asChild>
          <Link href="/categories/new">
            <Plus className="mr-2 h-4 w-4" />
            New Category
          </Link>
        </Button>
      </div>

      <div className="rounded-md border bg-white">
        {isLoading ? (
          <div className="p-8 text-center text-sm text-slate-500">Loading categories...</div>
        ) : error ? (
          <div className="p-8 text-center text-sm text-red-500">Failed to load categories</div>
        ) : categories?.length === 0 ? (
          <div className="p-8 text-center">
            <p className="text-sm text-slate-500 mb-4">No categories found</p>
            <Button variant="outline" asChild>
              <Link href="/categories/new">Create your first category</Link>
            </Button>
          </div>
        ) : (
          <div className="relative w-full overflow-auto">
            <table className="w-full caption-bottom text-sm">
              <thead className="[&_tr]:border-b">
                <tr className="border-b transition-colors hover:bg-slate-50/50 data-[state=selected]:bg-slate-50">
                  <th className="h-12 px-4 text-left align-middle font-medium text-slate-500">Name</th>
                  <th className="h-12 px-4 text-left align-middle font-medium text-slate-500">Slug</th>
                  <th className="h-12 px-4 text-left align-middle font-medium text-slate-500">Status</th>
                  <th className="h-12 px-4 text-right align-middle font-medium text-slate-500">Actions</th>
                </tr>
              </thead>
              <tbody className="[&_tr:last-child]:border-0">
                {categories?.map((category) => (
                  <tr
                    key={category.id}
                    className="border-b transition-colors hover:bg-slate-50/50 data-[state=selected]:bg-slate-50"
                  >
                    <td className="p-4 align-middle font-medium">{category.name}</td>
                    <td className="p-4 align-middle text-slate-500">{category.slug}</td>
                    <td className="p-4 align-middle">
                      <span
                        className={`inline-flex items-center rounded-full px-2 py-1 text-xs font-medium ${
                          category.is_active
                            ? "bg-green-50 text-green-700"
                            : "bg-slate-100 text-slate-700"
                        }`}
                      >
                        {category.is_active ? "Active" : "Inactive"}
                      </span>
                    </td>
                    <td className="p-4 align-middle text-right space-x-2">
                      <Button variant="ghost" size="sm" asChild>
                        <Link href={`/categories/${category.id}/edit`}>Edit</Link>
                      </Button>
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