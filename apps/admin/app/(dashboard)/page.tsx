"use client";

import Link from "next/link";
import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  ArrowUpRight,
  BarChart3,
  CalendarDays,
  CircleAlert,
  Package,
  RefreshCw,
  ShoppingBag,
  WalletCards,
} from "lucide-react";
import api from "@/lib/api";
import {
  type AnalyticsOrder,
  type AnalyticsOrderDetail,
  formatCompactCurrency,
  formatCurrency,
  getSalesSeries,
  getTopProducts,
  isRevenueOrder,
  isSameCalendarDay,
} from "@/lib/analytics";
import { Button } from "@/components/ui/button";

type ChartPeriod = "week" | "month";

function MetricCard({
  label,
  value,
  supportingText,
  icon: Icon,
}: {
  label: string;
  value: string;
  supportingText: string;
  icon: typeof WalletCards;
}) {
  return (
    <section className="rounded-lg border border-slate-200 bg-white p-5">
      <div className="flex items-start justify-between gap-4">
        <div>
          <p className="text-sm font-medium text-slate-700">{label}</p>
          <p className="mt-2 text-2xl font-semibold tracking-tight text-slate-950">{value}</p>
          <p className="mt-1.5 text-sm text-slate-600">{supportingText}</p>
        </div>
        <div className="rounded-md bg-slate-100 p-2.5 text-slate-700">
          <Icon className="h-5 w-5" aria-hidden="true" />
        </div>
      </div>
    </section>
  );
}

function SalesChart({
  data,
  period,
}: {
  data: ReturnType<typeof getSalesSeries>;
  period: ChartPeriod;
}) {
  const maximumSales = Math.max(...data.map((point) => point.sales), 1);
  const chartWidth = 720;
  const chartHeight = 230;
  const sidePadding = 26;
  const topPadding = 18;
  const bottomPadding = 35;
  const plotWidth = chartWidth - sidePadding * 2;
  const plotHeight = chartHeight - topPadding - bottomPadding;

  const points = data.map((point, index) => {
    const x = sidePadding + (index * plotWidth) / Math.max(data.length - 1, 1);
    const y = topPadding + plotHeight - (point.sales / maximumSales) * plotHeight;
    return { ...point, x, y };
  });

  const linePath = points.map((point, index) => `${index === 0 ? "M" : "L"} ${point.x} ${point.y}`).join(" ");
  const areaPath = `${linePath} L ${points.at(-1)?.x ?? sidePadding} ${
    topPadding + plotHeight
  } L ${points[0]?.x ?? sidePadding} ${topPadding + plotHeight} Z`;

  return (
    <div className="mt-5">
      <div className="mb-4 flex items-baseline justify-between gap-4">
        <div>
          <p className="text-sm font-medium text-slate-900">Penjualan terkonfirmasi</p>
          <p className="mt-1 text-sm text-slate-600">
            {period === "week" ? "7 hari terakhir" : "6 bulan terakhir"}
          </p>
        </div>
        <p className="text-sm font-medium tabular-nums text-slate-700">
          Tertinggi {formatCompactCurrency(maximumSales)}
        </p>
      </div>

      <div className="overflow-x-auto">
        <svg
          className="min-w-[620px] w-full"
          viewBox={`0 0 ${chartWidth} ${chartHeight}`}
          role="img"
          aria-label={`Grafik penjualan ${period === "week" ? "7 hari" : "6 bulan"} terakhir`}
        >
          {[0, 0.5, 1].map((step) => {
            const y = topPadding + plotHeight - step * plotHeight;
            return (
              <line
                key={step}
                x1={sidePadding}
                x2={chartWidth - sidePadding}
                y1={y}
                y2={y}
                stroke="#e2e8f0"
                strokeWidth="1"
              />
            );
          })}
          <path d={areaPath} fill="#e0f2fe" />
          <path d={linePath} fill="none" stroke="#0369a1" strokeWidth="2.5" />
          {points.map((point) => (
            <g key={point.key}>
              <circle cx={point.x} cy={point.y} r="4" fill="#ffffff" stroke="#0369a1" strokeWidth="2" />
              <text
                x={point.x}
                y={chartHeight - 10}
                textAnchor="middle"
                className="fill-slate-600 text-[11px]"
              >
                {point.label}
              </text>
              <title>{`${point.label}: ${formatCurrency(point.sales)} dari ${point.orders} pesanan`}</title>
            </g>
          ))}
        </svg>
      </div>
    </div>
  );
}

export default function DashboardHomePage() {
  const [period, setPeriod] = useState<ChartPeriod>("week");

  const {
    data: orders = [],
    isLoading: isLoadingOrders,
    isError,
    refetch,
    isFetching,
  } = useQuery({
    queryKey: ["dashboard-orders"],
    queryFn: async () => {
      const response = await api.get("/api/v1/orders");
      return response.data.data as AnalyticsOrder[];
    },
  });

  const revenueOrders = useMemo(() => orders.filter(isRevenueOrder), [orders]);
  const salesSeries = useMemo(() => getSalesSeries(orders, period), [orders, period]);
  const today = useMemo(() => new Date(), []);
  const confirmedRevenue = useMemo(
    () => revenueOrders.reduce((sum, order) => sum + order.total, 0),
    [revenueOrders]
  );
  const todayOrders = useMemo(
    () => orders.filter((order) => isSameCalendarDay(new Date(order.created_at), today)).length,
    [orders, today]
  );

  const revenueOrderIds = useMemo(
    () =>
      [...revenueOrders]
        .sort(
          (first, second) =>
            new Date(second.created_at).getTime() - new Date(first.created_at).getTime()
        )
        .slice(0, 24)
        .map((order) => order.id),
    [revenueOrders]
  );

  const { data: orderDetails = [], isLoading: isLoadingProducts } = useQuery({
    queryKey: ["dashboard-product-performance", revenueOrderIds],
    enabled: revenueOrderIds.length > 0,
    queryFn: async () => {
      const details = await Promise.all(
        revenueOrderIds.map(async (id) => {
          const response = await api.get(`/api/v1/orders/${id}`);
          return response.data.data as AnalyticsOrderDetail;
        })
      );

      return details;
    },
  });

  const topProducts = useMemo(() => getTopProducts(orderDetails), [orderDetails]);
  const maxProductRevenue = Math.max(...topProducts.map((product) => product.revenue), 1);

  return (
    <div className="space-y-7">
      <div className="flex flex-col gap-4 border-b border-slate-200 pb-5 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight text-slate-950">Dashboard</h1>
          <p className="mt-1.5 text-sm text-slate-600">
            Pantau ringkasan transaksi dan aktivitas operasional.
          </p>
        </div>
        <Button variant="outline" size="sm" onClick={() => refetch()} disabled={isFetching}>
          <RefreshCw className={`h-4 w-4 ${isFetching ? "animate-spin" : ""}`} />
          Refresh data
        </Button>
      </div>

      {isError ? (
        <div className="flex gap-3 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-900">
          <CircleAlert className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
          <div>
            <p className="font-medium">Data dashboard tidak dapat dimuat.</p>
            <p className="mt-1 text-red-800">Periksa koneksi backend, lalu muat ulang data.</p>
          </div>
        </div>
      ) : null}

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
        <MetricCard
          label="Total penjualan"
          value={isLoadingOrders ? "—" : formatCurrency(confirmedRevenue)}
          supportingText="Dari pesanan berstatus dibayar atau diproses"
          icon={WalletCards}
        />
        <MetricCard
          label="Total pesanan"
          value={isLoadingOrders ? "—" : orders.length.toLocaleString("id-ID")}
          supportingText={`${revenueOrders.length.toLocaleString("id-ID")} pesanan terkonfirmasi`}
          icon={ShoppingBag}
        />
        <MetricCard
          label="Pesanan hari ini"
          value={isLoadingOrders ? "—" : todayOrders.toLocaleString("id-ID")}
          supportingText="Termasuk semua status pesanan baru"
          icon={CalendarDays}
        />
      </div>

      <div className="grid gap-5 xl:grid-cols-[minmax(0,1.65fr)_minmax(300px,0.85fr)]">
        <section className="rounded-lg border border-slate-200 bg-white p-5">
          <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
            <div>
              <h2 className="text-base font-semibold text-slate-950">Tren penjualan</h2>
              <p className="mt-1 text-sm text-slate-600">
                Nilai pesanan pada status paid hingga completed.
              </p>
            </div>
            <div className="inline-flex w-fit rounded-md border border-slate-200 bg-slate-50 p-1">
              {(["week", "month"] as const).map((option) => (
                <button
                  key={option}
                  type="button"
                  onClick={() => setPeriod(option)}
                  className={`rounded px-3 py-1.5 text-sm font-medium transition-colors ${
                    period === option
                      ? "bg-white text-slate-950 shadow-sm"
                      : "text-slate-700 hover:text-slate-950"
                  }`}
                  aria-pressed={period === option}
                >
                  {option === "week" ? "Mingguan" : "Bulanan"}
                </button>
              ))}
            </div>
          </div>

          {isLoadingOrders ? (
            <div className="flex h-[288px] items-center justify-center text-sm text-slate-600">
              Memuat data penjualan…
            </div>
          ) : (
            <SalesChart data={salesSeries} period={period} />
          )}
        </section>

        <section className="rounded-lg border border-slate-200 bg-white p-5">
          <div className="flex items-start justify-between gap-4">
            <div>
              <h2 className="text-base font-semibold text-slate-950">Produk terlaris</h2>
              <p className="mt-1 text-sm text-slate-600">Berdasarkan nilai penjualan terbaru.</p>
            </div>
            <Package className="h-5 w-5 text-slate-500" aria-hidden="true" />
          </div>

          {isLoadingProducts ? (
            <div className="flex h-56 items-center justify-center text-sm text-slate-600">
              Menghitung performa produk…
            </div>
          ) : topProducts.length > 0 ? (
            <ol className="mt-5 space-y-4">
              {topProducts.map((product, index) => (
                <li key={product.productId}>
                  <div className="flex items-start gap-3">
                    <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-md bg-slate-100 text-xs font-semibold tabular-nums text-slate-700">
                      {index + 1}
                    </span>
                    <div className="min-w-0 flex-1">
                      <div className="flex items-baseline justify-between gap-3">
                        <p className="truncate text-sm font-medium text-slate-900">{product.name}</p>
                        <p className="shrink-0 text-sm font-medium tabular-nums text-slate-900">
                          {formatCompactCurrency(product.revenue)}
                        </p>
                      </div>
                      <div className="mt-1.5 h-1.5 overflow-hidden rounded bg-slate-100">
                        <div
                          className="h-full rounded bg-sky-700"
                          style={{ width: `${(product.revenue / maxProductRevenue) * 100}%` }}
                        />
                      </div>
                      <p className="mt-1.5 text-xs text-slate-600">
                        {product.units.toLocaleString("id-ID")} unit terjual
                      </p>
                    </div>
                  </div>
                </li>
              ))}
            </ol>
          ) : (
            <div className="flex h-56 flex-col items-center justify-center text-center">
              <BarChart3 className="h-7 w-7 text-slate-400" aria-hidden="true" />
              <p className="mt-3 text-sm font-medium text-slate-800">Belum ada data produk terjual</p>
              <p className="mt-1 max-w-xs text-sm leading-5 text-slate-600">
                Peringkat akan tersedia setelah ada pesanan berstatus dibayar atau diproses.
              </p>
            </div>
          )}
        </section>
      </div>

      <section className="flex flex-col gap-4 rounded-lg border border-slate-200 bg-white p-5 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h2 className="text-base font-semibold text-slate-950">Butuh detail operasional?</h2>
          <p className="mt-1 text-sm text-slate-600">
            Buka daftar pesanan untuk meninjau pembayaran, fulfillment, dan status pengiriman.
          </p>
        </div>
        <Button asChild variant="outline" className="w-full sm:w-auto">
          <Link href="/orders">
            Lihat pesanan
            <ArrowUpRight className="h-4 w-4" />
          </Link>
        </Button>
      </section>
    </div>
  );
}