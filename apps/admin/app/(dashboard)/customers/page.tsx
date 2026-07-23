"use client";

import { useMemo, useState } from "react";
import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { format } from "date-fns";
import { Eye, Search, Users } from "lucide-react";
import api from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

interface Customer {
  id: string;
  phone: string;
  name: string;
  email?: string;
  created_at: string;
  total_orders?: number;
  total_spent?: number;
}

interface CustomerListResponse {
  customers?: Customer[];
}

export default function CustomersPage() {
  const [searchQuery, setSearchQuery] = useState("");

  const { data: customers = [], isLoading } = useQuery<Customer[]>({
    queryKey: ["customers"],
    queryFn: async () => {
      const response = await api.get("/api/v1/customers");
      const data = response.data.data as Customer[] | CustomerListResponse;
      return Array.isArray(data) ? data : data.customers ?? [];
    },
  });

  const filteredCustomers = useMemo(() => {
    const query = searchQuery.trim().toLowerCase();

    if (!query) {
      return customers;
    }

    return customers.filter(
      (customer) =>
        customer.phone.toLowerCase().includes(query) ||
        customer.name.toLowerCase().includes(query) ||
        customer.email?.toLowerCase().includes(query),
    );
  }, [customers, searchQuery]);

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Customers</h1>
          <p className="text-sm text-slate-500">
            View and manage customer information
          </p>
        </div>
      </div>

      <div className="flex items-center gap-4">
        <div className="relative w-full max-w-sm">
          <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-slate-500" />
          <Input
            placeholder="Search by phone, name, or email..."
            className="pl-9"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
          />
        </div>
      </div>

      <div className="rounded-md border bg-white">
        {isLoading ? (
          <div className="p-8 text-center text-sm text-slate-500">
            Loading customers...
          </div>
        ) : filteredCustomers?.length === 0 ? (
          <div className="p-8 text-center">
            <Users className="mx-auto h-12 w-12 text-slate-300 mb-4" />
            <p className="text-sm text-slate-500 mb-2">
              {searchQuery ? "No customers found" : "No customers yet"}
            </p>
            {searchQuery && (
              <p className="text-xs text-slate-400">
                Try adjusting your search terms
              </p>
            )}
          </div>
        ) : (
          <div className="relative w-full overflow-auto">
            <table className="w-full caption-bottom text-sm">
              <thead className="[&_tr]:border-b">
                <tr className="border-b transition-colors hover:bg-slate-50/50">
                  <th className="h-12 px-4 text-left align-middle font-medium text-slate-500">
                    Phone
                  </th>
                  <th className="h-12 px-4 text-left align-middle font-medium text-slate-500">
                    Name
                  </th>
                  <th className="h-12 px-4 text-left align-middle font-medium text-slate-500">
                    Email
                  </th>
                  <th className="h-12 px-4 text-center align-middle font-medium text-slate-500">
                    Total Orders
                  </th>
                  <th className="h-12 px-4 text-right align-middle font-medium text-slate-500">
                    Total Spent
                  </th>
                  <th className="h-12 px-4 text-left align-middle font-medium text-slate-500">
                    Joined
                  </th>
                  <th className="h-12 px-4 text-right align-middle font-medium text-slate-500">
                    Actions
                  </th>
                </tr>
              </thead>
              <tbody className="[&_tr:last-child]:border-0">
                {filteredCustomers?.map((customer) => (
                  <tr
                    key={customer.id}
                    className="border-b transition-colors hover:bg-slate-50/50"
                  >
                    <td className="p-4 align-middle">
                      <span className="font-mono text-sm">{customer.phone}</span>
                    </td>
                    <td className="p-4 align-middle">
                      <div className="font-medium">{customer.name}</div>
                    </td>
                    <td className="p-4 align-middle text-slate-500">
                      {customer.email || "-"}
                    </td>
                    <td className="p-4 align-middle text-center">
                      {customer.total_orders || 0}
                    </td>
                    <td className="p-4 align-middle text-right font-medium">
                      {customer.total_spent
                        ? `Rp ${customer.total_spent.toLocaleString("id-ID")}`
                        : "-"}
                    </td>
                    <td className="p-4 align-middle text-slate-500">
                      {format(new Date(customer.created_at), "MMM d, yyyy")}
                    </td>
                    <td className="p-4 align-middle text-right">
                      <Button variant="ghost" size="sm" asChild>
                        <Link href={`/customers/${customer.id}`}>
                          <Eye className="h-4 w-4 mr-1" />
                          View
                        </Link>
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