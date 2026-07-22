"use client";

import { useRouter, useParams } from "next/navigation";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ChevronLeft } from "lucide-react";
import Link from "next/link";
import api from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

interface Category {
  id: string;
  name: string;
  slug: string;
  description: string;
  is_active: boolean;
}

interface FormData {
  name: string;
  description: string;
  is_active: boolean;
}

export default function EditCategoryPage() {
  const router = useRouter();
  const params = useParams();
  const queryClient = useQueryClient();
  const categoryId = params.id as string;

  const { data: category, isLoading } = useQuery<Category>({
    queryKey: ["categories", categoryId],
    queryFn: async () => {
      const response = await api.get(`/api/v1/categories/${categoryId}`);
      return response.data.data;
    },
  });

  const formData: FormData = {
    name: category?.name || "",
    description: category?.description || "",
    is_active: category?.is_active ?? true,
  };

  const updateCategory = useMutation({
    mutationFn: async (data: FormData) => {
      const response = await api.put(`/api/v1/categories/${categoryId}`, data);
      return response.data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["categories"] });
      queryClient.invalidateQueries({ queryKey: ["categories", categoryId] });
      router.push("/categories");
    },
  });

  const deleteCategory = useMutation({
    mutationFn: async () => {
      await api.delete(`/api/v1/categories/${categoryId}`);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["categories"] });
      router.push("/categories");
    },
  });

  const handleSubmit = (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    const form = e.currentTarget;
    const data: FormData = {
      name: form.categoryName.value,
      description: form.categoryDescription.value,
      is_active: form.categoryActive.checked,
    };
    updateCategory.mutate(data);
  };

  const handleDelete = () => {
    if (window.confirm(`Yakin ingin menghapus kategori "${category?.name}"? Aksi ini tidak dapat dibatalkan.`)) {
      deleteCategory.mutate();
    }
  };

  if (isLoading) {
    return <div className="p-8 text-center text-sm text-slate-500">Loading category...</div>;
  }

  if (!category) {
    return <div className="p-8 text-center text-sm text-red-500">Category not found</div>;
  }

  return (
    <div className="mx-auto max-w-2xl space-y-6">
      <div className="flex items-center space-x-4">
        <Button variant="ghost" size="icon" asChild>
          <Link href="/categories">
            <ChevronLeft className="h-4 w-4" />
          </Link>
        </Button>
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Edit Category</h1>
          <p className="text-sm text-slate-500">Update category information</p>
        </div>
      </div>

      <div className="rounded-md border bg-white p-6">
        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="name">Name</Label>
            <Input
              id="name"
              name="categoryName"
              required
              defaultValue={formData.name}
              placeholder="e.g., Electronics"
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="description">Description (Optional)</Label>
            <Input
              id="description"
              name="categoryDescription"
              defaultValue={formData.description}
              placeholder="Brief description of the category"
            />
          </div>

          <div className="flex items-center space-x-2">
            <input
              type="checkbox"
              id="is_active"
              name="categoryActive"
              className="h-4 w-4 rounded border-gray-300 text-slate-900 focus:ring-slate-900"
              defaultChecked={formData.is_active}
            />
            <Label htmlFor="is_active" className="font-normal">Category is active</Label>
          </div>

          <div className="pt-4 flex justify-between">
            <Button
              type="button"
              variant="outline"
              className="text-red-600 hover:text-red-700 hover:bg-red-50"
              onClick={handleDelete}
              disabled={deleteCategory.isPending}
            >
              {deleteCategory.isPending ? "Deleting..." : "Delete Category"}
            </Button>
            <div className="flex space-x-2">
              <Button variant="outline" asChild>
                <Link href="/categories">Cancel</Link>
              </Button>
              <Button type="submit" disabled={updateCategory.isPending}>
                {updateCategory.isPending ? "Saving..." : "Save Changes"}
              </Button>
            </div>
          </div>
        </form>
      </div>
    </div>
  );
}