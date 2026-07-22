"use client";

import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useParams } from "next/navigation";
import { ChevronLeft, Plus, Edit2, Trash2 } from "lucide-react";
import Link from "next/link";
import api from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

interface Product {
  id: string;
  category_id: string;
  name: string;
  slug: string;
  description: string;
  price: number;
  stock: number;
  is_active: boolean;
  category?: {
    id: string;
    name: string;
  };
}

interface Variant {
  id: string;
  product_id: string;
  sku: string;
  name: string;
  price: number;
  stock_on_hand: number;
  stock_reserved: number;
  is_active: boolean;
}

export default function ProductDetailsPage() {
  const params = useParams();
  const queryClient = useQueryClient();
  const productId = params.id as string;
  
  const [isVariantModalOpen, setIsVariantModalOpen] = useState(false);
  const [editingVariant, setEditingVariant] = useState<Variant | null>(null);
  
  const [variantForm, setVariantForm] = useState({
    sku: "",
    name: "",
    price: "",
    stock_on_hand: "0",
    is_active: true,
  });

  const { data: product, isLoading: productLoading } = useQuery<Product>({
    queryKey: ["products", productId],
    queryFn: async () => {
      const response = await api.get(`/api/v1/products/${productId}`);
      return response.data.data;
    },
  });

  const { data: variants, isLoading: variantsLoading } = useQuery<Variant[]>({
    queryKey: ["products", productId, "variants"],
    queryFn: async () => {
      const response = await api.get(`/api/v1/products/${productId}/variants`);
      return response.data.data;
    },
  });

  const saveVariant = useMutation({
    mutationFn: async (data: typeof variantForm) => {
      const payload = {
        ...data,
        price: parseFloat(data.price),
        stock_on_hand: parseInt(data.stock_on_hand, 10),
      };
      
      if (editingVariant) {
        const response = await api.put(`/api/v1/products/${productId}/variants/${editingVariant.id}`, payload);
        return response.data;
      } else {
        const response = await api.post(`/api/v1/products/${productId}/variants`, payload);
        return response.data;
      }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["products", productId, "variants"] });
      closeVariantModal();
    },
  });

  const deleteVariant = useMutation({
    mutationFn: async (variantId: string) => {
      await api.delete(`/api/v1/products/${productId}/variants/${variantId}`);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["products", productId, "variants"] });
    },
  });

  const openVariantModal = (variant?: Variant) => {
    if (variant) {
      setEditingVariant(variant);
      setVariantForm({
        sku: variant.sku,
        name: variant.name,
        price: variant.price.toString(),
        stock_on_hand: variant.stock_on_hand.toString(),
        is_active: variant.is_active,
      });
    } else {
      setEditingVariant(null);
      setVariantForm({
        sku: "",
        name: "",
        price: "",
        stock_on_hand: "0",
        is_active: true,
      });
    }
    setIsVariantModalOpen(true);
  };

  const closeVariantModal = () => {
    setIsVariantModalOpen(false);
    setEditingVariant(null);
  };

  const handleVariantSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    saveVariant.mutate(variantForm);
  };

  if (productLoading) {
    return <div className="p-8 text-center text-sm text-slate-500">Loading product details...</div>;
  }

  if (!product) {
    return <div className="p-8 text-center text-sm text-red-500">Product not found</div>;
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center space-x-4">
        <Button variant="ghost" size="icon" asChild>
          <Link href="/products">
            <ChevronLeft className="h-4 w-4" />
          </Link>
        </Button>
        <div className="flex-1">
          <h1 className="text-2xl font-semibold tracking-tight">{product.name}</h1>
          <p className="text-sm text-slate-500">
            {product.category?.name} &bull; Rp {Number(product.price || 0).toLocaleString("id-ID")} &bull; Base Stock: {product.stock}
          </p>
        </div>
        <Button variant="outline" asChild>
          <Link href={`/products/${product.id}/edit`}>Edit Product</Link>
        </Button>
      </div>

      <div className="space-y-4">
        <div className="flex items-center justify-between">
          <h2 className="text-lg font-medium">Product Variants</h2>
          <Button onClick={() => openVariantModal()}>
            <Plus className="mr-2 h-4 w-4" />
            Add Variant
          </Button>
        </div>

        <div className="rounded-md border bg-white">
          {variantsLoading ? (
            <div className="p-8 text-center text-sm text-slate-500">Loading variants...</div>
          ) : variants?.length === 0 ? (
            <div className="p-8 text-center text-sm text-slate-500">
              No variants configured. Add a variant to manage different options like size or color.
            </div>
          ) : (
            <div className="relative w-full overflow-auto">
              <table className="w-full caption-bottom text-sm">
                <thead className="[&_tr]:border-b">
                  <tr className="border-b transition-colors hover:bg-slate-50/50">
                    <th className="h-12 px-4 text-left align-middle font-medium text-slate-500">SKU</th>
                    <th className="h-12 px-4 text-left align-middle font-medium text-slate-500">Name</th>
                    <th className="h-12 px-4 text-right align-middle font-medium text-slate-500">Price (Rp)</th>
                    <th className="h-12 px-4 text-right align-middle font-medium text-slate-500">On Hand</th>
                    <th className="h-12 px-4 text-right align-middle font-medium text-slate-500">Reserved</th>
                    <th className="h-12 px-4 text-center align-middle font-medium text-slate-500">Status</th>
                    <th className="h-12 px-4 text-right align-middle font-medium text-slate-500">Actions</th>
                  </tr>
                </thead>
                <tbody className="[&_tr:last-child]:border-0">
                  {variants?.map((variant) => (
                    <tr key={variant.id} className="border-b transition-colors hover:bg-slate-50/50">
                      <td className="p-4 align-middle font-medium">{variant.sku}</td>
                      <td className="p-4 align-middle">{variant.name}</td>
                      <td className="p-4 align-middle text-right">{Number(variant.price || 0).toLocaleString("id-ID")}</td>
                      <td className="p-4 align-middle text-right">{variant.stock_on_hand}</td>
                      <td className="p-4 align-middle text-right text-amber-600">{variant.stock_reserved}</td>
                      <td className="p-4 align-middle text-center">
                        <span
                          className={`inline-flex items-center rounded-full px-2 py-1 text-xs font-medium ${
                            variant.is_active
                              ? "bg-green-50 text-green-700"
                              : "bg-slate-100 text-slate-700"
                          }`}
                        >
                          {variant.is_active ? "Active" : "Inactive"}
                        </span>
                      </td>
                      <td className="p-4 align-middle text-right space-x-2">
                        <Button variant="ghost" size="icon" onClick={() => openVariantModal(variant)}>
                          <Edit2 className="h-4 w-4" />
                        </Button>
                        <Button 
                          variant="ghost" 
                          size="icon" 
                          className="text-red-500 hover:text-red-700 hover:bg-red-50"
                          onClick={() => {
                            if (window.confirm("Are you sure you want to delete this variant?")) {
                              deleteVariant.mutate(variant.id);
                            }
                          }}
                        >
                          <Trash2 className="h-4 w-4" />
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

      {isVariantModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
          <div className="w-full max-w-md rounded-lg bg-white p-6 shadow-lg">
            <h2 className="text-lg font-semibold mb-4">
              {editingVariant ? "Edit Variant" : "Add Variant"}
            </h2>
            <form onSubmit={handleVariantSubmit} className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="sku">SKU</Label>
                <Input
                  id="sku"
                  required
                  value={variantForm.sku}
                  onChange={(e) => setVariantForm({ ...variantForm, sku: e.target.value })}
                  placeholder="e.g., PROD-BLK-SM"
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="vname">Variant Name</Label>
                <Input
                  id="vname"
                  required
                  value={variantForm.name}
                  onChange={(e) => setVariantForm({ ...variantForm, name: e.target.value })}
                  placeholder="e.g., Black - Small"
                />
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div className="space-y-2">
                  <Label htmlFor="vprice">Price (Rp)</Label>
                  <Input
                    id="vprice"
                    type="number"
                    min="0"
                    step="0.01"
                    required
                    value={variantForm.price}
                    onChange={(e) => setVariantForm({ ...variantForm, price: e.target.value })}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="vstock">Stock On Hand</Label>
                  <Input
                    id="vstock"
                    type="number"
                    min="0"
                    required
                    value={variantForm.stock_on_hand}
                    onChange={(e) => setVariantForm({ ...variantForm, stock_on_hand: e.target.value })}
                  />
                </div>
              </div>
              <div className="flex items-center space-x-2 pt-2">
                <input
                  type="checkbox"
                  id="vis_active"
                  className="h-4 w-4 rounded border-gray-300 text-slate-900 focus:ring-slate-900"
                  checked={variantForm.is_active}
                  onChange={(e) => setVariantForm({ ...variantForm, is_active: e.target.checked })}
                />
                <Label htmlFor="vis_active" className="font-normal">Variant is active</Label>
              </div>
              <div className="pt-4 flex justify-end space-x-2">
                <Button type="button" variant="outline" onClick={closeVariantModal}>
                  Cancel
                </Button>
                <Button type="submit" disabled={saveVariant.isPending}>
                  {saveVariant.isPending ? "Saving..." : "Save Variant"}
                </Button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}