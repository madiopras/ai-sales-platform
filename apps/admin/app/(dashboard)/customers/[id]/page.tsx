"use client";

import { useParams } from "next/navigation";
import { useQuery } from "@tanstack/react-query";
import { format } from "date-fns";
import { ChevronLeft, User, MapPin, ShoppingBag, Loader2, Phone, Mail, Calendar } from "lucide-react";
import Link from "next/link";
import api from "@/lib/api";
import { Button } from "@/components/ui/button";

interface CustomerDetail {
  id: string;
  phone: string;
  name: string;
  email: string;
  created_at: string;
  total_orders: number;
  total_spent: number;
}

interface Order {
  id: string;
  order_no: string;
  status: string;
  total: number;
  created_at: string;
}

interface Address {
  id: string;
  recipient_name: string;
  phone: string;
  address_line: string;
  city: string;
  district: string;
  postal_code: string;
  is_default: boolean;
  created_at: string;
}

interface Cart {
  id: string;
  product_id: string;
  product_name: string;
  quantity: number;
  price: number;
  updated_at: string;
}

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
      className={`inline-flex items-center rounded-full border px-2 py-0.5 text-xs font-semibold ${
        styles[status] || "bg-slate-100 text-slate-800 border-slate-200"
      }`}
    >
      {labels[status] || status}
    </span>
  );
}

export default function CustomerDetailPage() {
  const params = useParams();
  const customerId = params.id as string;

  const { data: customer, isLoading: customerLoading } = useQuery<CustomerDetail>({
    queryKey: ["customer", customerId],
    queryFn: async () => {
      const response = await api.get(`/api/v1/customers/${customerId}`);
      return response.data.data;
    },
  });

  const { data: orders } = useQuery<Order[]>({
    queryKey: ["customer-orders", customerId],
    queryFn: async () => {
      const response = await api.get(`/api/v1/customers/${customerId}/orders`);
      return response.data.data || [];
    },
    enabled: !!customer,
  });

  const { data: addresses } = useQuery<Address[]>({
    queryKey: ["customer-addresses", customerId],
    queryFn: async () => {
      const response = await api.get(`/api/v1/customers/${customerId}/addresses`);
      return response.data.data || [];
    },
    enabled: !!customer,
  });

  const { data: cart } = useQuery<Cart[]>({
    queryKey: ["customer-cart", customerId],
    queryFn: async () => {
      const response = await api.get(`/api/v1/customers/${customerId}/cart`);
      return response.data.data || [];
    },
    enabled: !!customer,
  });

  if (customerLoading) {
    return (
      <div className="flex h-[50vh] items-center justify-center">
        <Loader2 className="h-8 w-8 animate-spin text-slate-400" />
      </div>
    );
  }

  if (!customer) {
    return (
      <div className="flex h-[50vh] flex-col items-center justify-center space-y-4">
        <p className="text-slate-500">Customer not found.</p>
        <Button variant="outline" asChild>
          <Link href="/customers">Back to Customers</Link>
        </Button>
      </div>
    );
  }

  const cartTotal = cart?.reduce((sum, item) => sum + item.price * item.quantity, 0) || 0;

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center space-x-4">
          <Button variant="ghost" size="icon" asChild>
            <Link href="/customers">
              <ChevronLeft className="h-4 w-4" />
            </Link>
          </Button>
          <div>
            <h1 className="text-2xl font-semibold tracking-tight">{customer.name}</h1>
            <p className="text-sm text-slate-500">Customer since {format(new Date(customer.created_at), "MMMM yyyy")}</p>
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 gap-6 md:grid-cols-3">
        <div className="space-y-6">
          {/* Customer Info */}
          <div className="rounded-md border bg-white">
            <div className="border-b bg-slate-50/50 px-6 py-4 flex items-center gap-2">
              <User className="h-5 w-5 text-slate-500" />
              <h2 className="font-semibold text-slate-900">Customer Information</h2>
            </div>
            <div className="p-6 space-y-4">
              <div className="flex items-start gap-3">
                <Phone className="h-4 w-4 text-slate-400 mt-0.5" />
                <div className="flex-1">
                  <p className="text-sm font-medium text-slate-500">Phone</p>
                  <p className="text-sm text-slate-900 mt-1 font-mono">{customer.phone}</p>
                </div>
              </div>
              <div className="flex items-start gap-3">
                <Mail className="h-4 w-4 text-slate-400 mt-0.5" />
                <div className="flex-1">
                  <p className="text-sm font-medium text-slate-500">Email</p>
                  <p className="text-sm text-slate-900 mt-1">{customer.email || "-"}</p>
                </div>
              </div>
              <div className="flex items-start gap-3">
                <Calendar className="h-4 w-4 text-slate-400 mt-0.5" />
                <div className="flex-1">
                  <p className="text-sm font-medium text-slate-500">Joined</p>
                  <p className="text-sm text-slate-900 mt-1">
                    {format(new Date(customer.created_at), "MMM d, yyyy")}
                  </p>
                </div>
              </div>
            </div>
          </div>

          {/* Stats */}
          <div className="rounded-md border bg-white">
            <div className="border-b bg-slate-50/50 px-6 py-4">
              <h2 className="font-semibold text-slate-900">Statistics</h2>
            </div>
            <div className="p-6 space-y-4">
              <div>
                <p className="text-sm font-medium text-slate-500">Total Orders</p>
                <p className="text-2xl font-semibold text-slate-900 mt-1">{customer.total_orders || 0}</p>
              </div>
              <div>
                <p className="text-sm font-medium text-slate-500">Total Spent</p>
                <p className="text-2xl font-semibold text-slate-900 mt-1">
                  Rp {(customer.total_spent || 0).toLocaleString("id-ID")}
                </p>
              </div>
              {customer.total_orders > 0 && (
                <div>
                  <p className="text-sm font-medium text-slate-500">Average Order Value</p>
                  <p className="text-lg font-medium text-slate-700 mt-1">
                    Rp {Math.round((customer.total_spent || 0) / customer.total_orders).toLocaleString("id-ID")}
                  </p>
                </div>
              )}
            </div>
          </div>

          {/* Current Cart */}
          {cart && cart.length > 0 && (
            <div className="rounded-md border bg-white">
              <div className="border-b bg-slate-50/50 px-6 py-4">
                <h2 className="font-semibold text-slate-900">Current Cart</h2>
              </div>
              <div className="p-6 space-y-3">
                {cart.map((item) => (
                  <div key={item.id} className="flex justify-between text-sm">
                    <div className="flex-1">
                      <p className="font-medium text-slate-900">{item.product_name}</p>
                      <p className="text-xs text-slate-500">Qty: {item.quantity}</p>
                    </div>
                    <p className="font-medium text-slate-900">
                      Rp {(item.price * item.quantity).toLocaleString("id-ID")}
                    </p>
                  </div>
                ))}
                <div className="pt-3 border-t">
                  <div className="flex justify-between">
                    <p className="text-sm font-semibold text-slate-900">Total</p>
                    <p className="text-sm font-semibold text-slate-900">
                      Rp {cartTotal.toLocaleString("id-ID")}
                    </p>
                  </div>
                </div>
              </div>
            </div>
          )}
        </div>

        <div className="md:col-span-2 space-y-6">
          {/* Order History */}
          <div className="rounded-md border bg-white">
            <div className="border-b bg-slate-50/50 px-6 py-4 flex items-center gap-2">
              <ShoppingBag className="h-5 w-5 text-slate-500" />
              <h2 className="font-semibold text-slate-900">Order History</h2>
            </div>
            <div className="p-0">
              {!orders || orders.length === 0 ? (
                <div className="p-8 text-center text-sm text-slate-500">
                  No orders yet
                </div>
              ) : (
                <div className="relative w-full overflow-auto">
                  <table className="w-full text-sm">
                    <thead className="text-xs text-slate-700 uppercase bg-slate-50 border-b">
                      <tr>
                        <th className="px-6 py-3 text-left">Order ID</th>
                        <th className="px-6 py-3 text-left">Date</th>
                        <th className="px-6 py-3 text-left">Status</th>
                        <th className="px-6 py-3 text-right">Total</th>
                        <th className="px-6 py-3 text-right">Actions</th>
                      </tr>
                    </thead>
                    <tbody>
                      {orders.map((order) => (
                        <tr key={order.id} className="border-b last:border-0 hover:bg-slate-50/50">
                          <td className="px-6 py-4 font-medium text-slate-900">{order.order_no}</td>
                          <td className="px-6 py-4 text-slate-500">
                            {format(new Date(order.created_at), "MMM d, yyyy")}
                          </td>
                          <td className="px-6 py-4">
                            <StatusBadge status={order.status} />
                          </td>
                          <td className="px-6 py-4 text-right font-medium text-slate-900">
                            Rp {order.total.toLocaleString("id-ID")}
                          </td>
                          <td className="px-6 py-4 text-right">
                            <Button variant="ghost" size="sm" asChild>
                              <Link href={`/orders/${order.id}`}>View</Link>
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

          {/* Address History */}
          <div className="rounded-md border bg-white">
            <div className="border-b bg-slate-50/50 px-6 py-4 flex items-center gap-2">
              <MapPin className="h-5 w-5 text-slate-500" />
              <h2 className="font-semibold text-slate-900">Saved Addresses</h2>
            </div>
            <div className="p-6">
              {!addresses || addresses.length === 0 ? (
                <p className="text-sm text-slate-500 text-center py-4">No saved addresses</p>
              ) : (
                <div className="space-y-4">
                  {addresses.map((address) => (
                    <div key={address.id} className="p-4 rounded-md border bg-slate-50/50">
                      <div className="flex items-start justify-between mb-2">
                        <div>
                          <p className="font-medium text-slate-900">{address.recipient_name}</p>
                          <p className="text-sm text-slate-600 font-mono">{address.phone}</p>
                        </div>
                        {address.is_default && (
                          <span className="inline-flex items-center rounded-full bg-blue-100 px-2 py-0.5 text-xs font-medium text-blue-800">
                            Default
                          </span>
                        )}
                      </div>
                      <p className="text-sm text-slate-700 leading-relaxed">
                        {address.address_line}<br />
                        {address.district}, {address.city}<br />
                        {address.postal_code}
                      </p>
                      <p className="text-xs text-slate-400 mt-2">
                        Added {format(new Date(address.created_at), "MMM d, yyyy")}
                      </p>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}