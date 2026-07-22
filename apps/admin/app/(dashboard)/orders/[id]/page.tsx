"use client";

import { useState } from "react";
import { useParams } from "next/navigation";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { format } from "date-fns";
import { ChevronLeft, Package, User, MapPin, Loader2, CreditCard, Truck } from "lucide-react";
import Link from "next/link";
import api from "@/lib/api";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

interface OrderDetail {
  id: string;
  order_no: string;
  customer_id: string;
  status: string;
  total: number;
  recipient_name: string;
  phone: string;
  address_line: string;
  city: string;
  district: string;
  postal_code: string;
  notes: string;
  created_at: string;
  items: OrderItem[];
  invoice?: Invoice;
  shipment?: Shipment;
}

interface OrderItem {
  id: string;
  product_id: string;
  product_name: string;
  quantity: number;
  unit_price: number;
  subtotal: number;
}

interface Invoice {
  id: string;
  invoice_no: string;
  external_id: string;
  payment_method: string;
  status: string;
  amount: number;
  paid_at: string;
  expires_at: string;
}

interface Shipment {
  id: string;
  tracking_no: string;
  courier: string;
  service: string;
  status: string;
  estimated_delivery: string;
  actual_delivery: string;
}

const ORDER_STATUSES = [
  { value: "pending_payment", label: "Pending Payment" },
  { value: "paid", label: "Paid" },
  { value: "processing", label: "Processing" },
  { value: "shipped", label: "Shipped" },
  { value: "delivered", label: "Delivered" },
  { value: "completed", label: "Completed" },
  { value: "cancelled", label: "Cancelled" },
];

function StatusBadge({ status }: { status: string }) {
  const styles: Record<string, string> = {
    pending_payment: "bg-amber-100 text-amber-800 border-amber-200",
    paid: "bg-blue-100 text-blue-800 border-blue-200",
    processing: "bg-indigo-100 text-indigo-800 border-indigo-200",
    shipped: "bg-purple-100 text-purple-800 border-purple-200",
    delivered: "bg-teal-100 text-teal-800 border-teal-200",
    completed: "bg-emerald-100 text-emerald-800 border-emerald-200",
    cancelled: "bg-slate-100 text-slate-800 border-slate-200",
  };

  const labels: Record<string, string> = {
    pending_payment: "Pending Payment",
    paid: "Paid",
    processing: "Processing",
    shipped: "Shipped",
    delivered: "Delivered",
    completed: "Completed",
    cancelled: "Cancelled",
  };

  return (
    <span
      className={`inline-flex items-center rounded-full border px-2.5 py-0.5 text-xs font-semibold transition-colors ${
        styles[status] || "bg-slate-100 text-slate-800 border-slate-200"
      }`}
    >
      {labels[status] || status}
    </span>
  );
}

export default function OrderDetailPage() {
  const params = useParams();
  const queryClient = useQueryClient();
  const orderId = params.id as string;

  const [selectedStatus, setSelectedStatus] = useState<string>("");

  const { data: order, isLoading } = useQuery<OrderDetail>({
    queryKey: ["order", orderId],
    queryFn: async () => {
      const response = await api.get(`/api/v1/orders/${orderId}`);
      setSelectedStatus(response.data.data.status);
      return response.data.data;
    },
  });

  const updateStatus = useMutation({
    mutationFn: async (newStatus: string) => {
      const response = await api.patch(`/api/v1/orders/${orderId}/status`, {
        status: newStatus,
      });
      return response.data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["order", orderId] });
      queryClient.invalidateQueries({ queryKey: ["orders"] });
    },
  });

  const handleStatusChange = (newStatus: string | null) => {
    if (!newStatus) return;
    setSelectedStatus(newStatus);
    updateStatus.mutate(newStatus);
  };

  if (isLoading) {
    return (
      <div className="flex h-[50vh] items-center justify-center">
        <Loader2 className="h-8 w-8 animate-spin text-slate-400" />
      </div>
    );
  }

  if (!order) {
    return (
      <div className="flex h-[50vh] flex-col items-center justify-center space-y-4">
        <p className="text-slate-500">Order not found.</p>
        <Button variant="outline" asChild>
          <Link href="/orders">Back to Orders</Link>
        </Button>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center space-x-4">
          <Button variant="ghost" size="icon" asChild>
            <Link href="/orders">
              <ChevronLeft className="h-4 w-4" />
            </Link>
          </Button>
          <div>
            <h1 className="text-2xl font-semibold tracking-tight flex items-center gap-3">
              Order #{order.order_no}
              <StatusBadge status={order.status} />
            </h1>
            <p className="text-sm text-slate-500">
              Placed on {format(new Date(order.created_at), "MMMM d, yyyy 'at' h:mm a")}
            </p>
          </div>
        </div>
        <div className="flex items-center space-x-2">
          <p className="text-sm font-medium text-slate-700">Update Status:</p>
          <Select
            value={selectedStatus}
            onValueChange={handleStatusChange}
            disabled={updateStatus.isPending}
          >
            <SelectTrigger className="w-[180px]">
              <SelectValue placeholder="Select status" />
            </SelectTrigger>
            <SelectContent>
              {ORDER_STATUSES.map((status) => (
                <SelectItem key={status.value} value={status.value}>
                  {status.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          {updateStatus.isPending && (
            <Loader2 className="h-4 w-4 animate-spin text-slate-400" />
          )}
        </div>
      </div>

      <div className="grid grid-cols-1 gap-6 md:grid-cols-3">
        <div className="md:col-span-2 space-y-6">
          <div className="rounded-md border bg-white overflow-hidden">
            <div className="border-b bg-slate-50/50 px-6 py-4 flex items-center gap-2">
              <Package className="h-5 w-5 text-slate-500" />
              <h2 className="font-semibold text-slate-900">Order Items</h2>
            </div>
            <div className="p-0">
              <table className="w-full text-sm text-left text-slate-500">
                <thead className="text-xs text-slate-700 uppercase bg-slate-50">
                  <tr>
                    <th className="px-6 py-3">Product</th>
                    <th className="px-6 py-3 text-right">Price</th>
                    <th className="px-6 py-3 text-center">Qty</th>
                    <th className="px-6 py-3 text-right">Total</th>
                  </tr>
                </thead>
                <tbody>
                  {order.items?.map((item) => (
                    <tr key={item.id} className="bg-white border-b last:border-0">
                      <td className="px-6 py-4 font-medium text-slate-900">
                        {item.product_name}
                      </td>
                      <td className="px-6 py-4 text-right">
                        Rp {item.unit_price.toLocaleString("id-ID")}
                      </td>
                      <td className="px-6 py-4 text-center">{item.quantity}</td>
                      <td className="px-6 py-4 text-right font-medium text-slate-900">
                        Rp {item.subtotal.toLocaleString("id-ID")}
                      </td>
                    </tr>
                  ))}
                  {(!order.items || order.items.length === 0) && (
                    <tr className="bg-white">
                       <td colSpan={4} className="px-6 py-8 text-center text-slate-500">No items found for this order.</td>
                    </tr>
                  )}
                </tbody>
                <tfoot className="bg-slate-50 border-t border-slate-200 font-semibold text-slate-900">
                  <tr>
                    <td colSpan={3} className="px-6 py-4 text-right">Order Total</td>
                    <td className="px-6 py-4 text-right text-lg">Rp {order.total.toLocaleString("id-ID")}</td>
                  </tr>
                </tfoot>
              </table>
            </div>
          </div>
        </div>

        <div className="space-y-6">
          <div className="rounded-md border bg-white">
            <div className="border-b bg-slate-50/50 px-6 py-4 flex items-center gap-2">
              <User className="h-5 w-5 text-slate-500" />
              <h2 className="font-semibold text-slate-900">Customer Details</h2>
            </div>
            <div className="p-6 space-y-4">
              <div>
                <p className="text-sm font-medium text-slate-500">Name</p>
                <p className="text-base text-slate-900 mt-1">{order.recipient_name}</p>
              </div>
              <div>
                <p className="text-sm font-medium text-slate-500">Phone</p>
                <p className="text-base text-slate-900 mt-1">{order.phone}</p>
              </div>
              <div>
                <p className="text-sm font-medium text-slate-500">Customer ID</p>
                <p className="text-sm text-slate-600 mt-1 break-all">{order.customer_id}</p>
              </div>
            </div>
          </div>

          <div className="rounded-md border bg-white">
            <div className="border-b bg-slate-50/50 px-6 py-4 flex items-center gap-2">
              <MapPin className="h-5 w-5 text-slate-500" />
              <h2 className="font-semibold text-slate-900">Shipping Address</h2>
            </div>
            <div className="p-6 space-y-4">
              <div>
                <p className="text-sm text-slate-900 leading-relaxed">
                  {order.address_line}<br />
                  {order.district}, {order.city}<br />
                  {order.postal_code}
                </p>
              </div>
              {order.notes && (
                <>
                  <div className="h-px w-full bg-slate-200" />
                  <div>
                    <p className="text-sm font-medium text-slate-500">Notes</p>
                    <p className="text-sm text-slate-700 mt-1 italic">{order.notes}</p>
                  </div>
                </>
              )}
            </div>
          </div>

          {/* Payment Information */}
          {order.invoice && (
            <div className="rounded-md border bg-white">
              <div className="border-b bg-slate-50/50 px-6 py-4 flex items-center gap-2">
                <CreditCard className="h-5 w-5 text-slate-500" />
                <h2 className="font-semibold text-slate-900">Payment Information</h2>
              </div>
              <div className="p-6 space-y-4">
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <p className="text-sm font-medium text-slate-500">Invoice No</p>
                    <p className="text-sm text-slate-900 mt-1">{order.invoice.invoice_no}</p>
                  </div>
                  <div>
                    <p className="text-sm font-medium text-slate-500">Status</p>
                    <span className={`inline-flex items-center rounded-full border px-2 py-0.5 text-xs font-semibold mt-1 ${
                      order.invoice.status === 'paid' 
                        ? 'bg-green-100 text-green-800 border-green-200'
                        : order.invoice.status === 'pending'
                        ? 'bg-amber-100 text-amber-800 border-amber-200'
                        : 'bg-slate-100 text-slate-800 border-slate-200'
                    }`}>
                      {order.invoice.status.charAt(0).toUpperCase() + order.invoice.status.slice(1)}
                    </span>
                  </div>
                </div>
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <p className="text-sm font-medium text-slate-500">Payment Method</p>
                    <p className="text-sm text-slate-900 mt-1">{order.invoice.payment_method || 'N/A'}</p>
                  </div>
                  <div>
                    <p className="text-sm font-medium text-slate-500">Amount</p>
                    <p className="text-sm text-slate-900 mt-1 font-medium">
                      Rp {order.invoice.amount.toLocaleString("id-ID")}
                    </p>
                  </div>
                </div>
                {order.invoice.paid_at && (
                  <div>
                    <p className="text-sm font-medium text-slate-500">Paid At</p>
                    <p className="text-sm text-slate-900 mt-1">
                      {format(new Date(order.invoice.paid_at), "MMM d, yyyy 'at' h:mm a")}
                    </p>
                  </div>
                )}
                {order.invoice.expires_at && order.invoice.status === 'pending' && (
                  <div>
                    <p className="text-sm font-medium text-slate-500">Expires At</p>
                    <p className="text-sm text-amber-700 mt-1">
                      {format(new Date(order.invoice.expires_at), "MMM d, yyyy 'at' h:mm a")}
                    </p>
                  </div>
                )}
              </div>
            </div>
          )}

          {/* Shipping Information */}
          {order.shipment && (
            <div className="rounded-md border bg-white">
              <div className="border-b bg-slate-50/50 px-6 py-4 flex items-center gap-2">
                <Truck className="h-5 w-5 text-slate-500" />
                <h2 className="font-semibold text-slate-900">Shipping Information</h2>
              </div>
              <div className="p-6 space-y-4">
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <p className="text-sm font-medium text-slate-500">Tracking Number</p>
                    <p className="text-sm text-slate-900 mt-1 font-mono">{order.shipment.tracking_no || '-'}</p>
                  </div>
                  <div>
                    <p className="text-sm font-medium text-slate-500">Status</p>
                    <span className={`inline-flex items-center rounded-full border px-2 py-0.5 text-xs font-semibold mt-1 ${
                      order.shipment.status === 'delivered' 
                        ? 'bg-green-100 text-green-800 border-green-200'
                        : order.shipment.status === 'on_delivery'
                        ? 'bg-blue-100 text-blue-800 border-blue-200'
                        : 'bg-slate-100 text-slate-800 border-slate-200'
                    }`}>
                      {order.shipment.status.replace('_', ' ').replace(/\b\w/g, l => l.toUpperCase())}
                    </span>
                  </div>
                </div>
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <p className="text-sm font-medium text-slate-500">Courier</p>
                    <p className="text-sm text-slate-900 mt-1">{order.shipment.courier}</p>
                  </div>
                  <div>
                    <p className="text-sm font-medium text-slate-500">Service</p>
                    <p className="text-sm text-slate-900 mt-1">{order.shipment.service}</p>
                  </div>
                </div>
                {order.shipment.estimated_delivery && (
                  <div>
                    <p className="text-sm font-medium text-slate-500">Estimated Delivery</p>
                    <p className="text-sm text-slate-900 mt-1">
                      {format(new Date(order.shipment.estimated_delivery), "MMM d, yyyy")}
                    </p>
                  </div>
                )}
                {order.shipment.actual_delivery && (
                  <div>
                    <p className="text-sm font-medium text-slate-500">Delivered At</p>
                    <p className="text-sm text-green-700 mt-1 font-medium">
                      {format(new Date(order.shipment.actual_delivery), "MMM d, yyyy 'at' h:mm a")}
                    </p>
                  </div>
                )}
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
