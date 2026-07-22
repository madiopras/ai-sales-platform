export interface AnalyticsOrder {
  id: string;
  order_no: string;
  status: string;
  total: number;
  created_at: string;
}

export interface AnalyticsOrderItem {
  id: string;
  product_id: string;
  product_name: string;
  quantity: number;
  subtotal: number;
}

export interface AnalyticsOrderDetail extends AnalyticsOrder {
  items: AnalyticsOrderItem[];
}

export interface SalesDataPoint {
  key: string;
  label: string;
  sales: number;
  orders: number;
}

export interface TopProduct {
  productId: string;
  name: string;
  units: number;
  revenue: number;
}

const REVENUE_STATUSES = new Set([
  "paid",
  "processing",
  "shipped",
  "delivered",
  "completed",
]);

export function isRevenueOrder(order: AnalyticsOrder) {
  return REVENUE_STATUSES.has(order.status);
}

export function formatCurrency(value: number) {
  return new Intl.NumberFormat("id-ID", {
    style: "currency",
    currency: "IDR",
    maximumFractionDigits: 0,
  }).format(value);
}

export function formatCompactCurrency(value: number) {
  return new Intl.NumberFormat("id-ID", {
    style: "currency",
    currency: "IDR",
    notation: "compact",
    maximumFractionDigits: 1,
  }).format(value);
}

export function isSameCalendarDay(date: Date, reference: Date) {
  return (
    date.getFullYear() === reference.getFullYear() &&
    date.getMonth() === reference.getMonth() &&
    date.getDate() === reference.getDate()
  );
}

export function getSalesSeries(
  orders: AnalyticsOrder[],
  period: "week" | "month",
  referenceDate = new Date()
): SalesDataPoint[] {
  const seriesLength = period === "week" ? 7 : 6;
  const bucketStarts: Date[] = [];

  for (let index = seriesLength - 1; index >= 0; index -= 1) {
    const bucket = new Date(referenceDate);

    if (period === "week") {
      bucket.setHours(0, 0, 0, 0);
      bucket.setDate(bucket.getDate() - index);
    } else {
      bucket.setDate(1);
      bucket.setHours(0, 0, 0, 0);
      bucket.setMonth(bucket.getMonth() - index);
    }

    bucketStarts.push(bucket);
  }

  return bucketStarts.map((bucket) => {
    const nextBucket = new Date(bucket);

    if (period === "week") {
      nextBucket.setDate(nextBucket.getDate() + 1);
    } else {
      nextBucket.setMonth(nextBucket.getMonth() + 1);
    }

    const bucketOrders = orders.filter((order) => {
      if (!isRevenueOrder(order)) return false;

      const createdAt = new Date(order.created_at);
      return createdAt >= bucket && createdAt < nextBucket;
    });

    return {
      key: bucket.toISOString(),
      label:
        period === "week"
          ? new Intl.DateTimeFormat("id-ID", {
              weekday: "short",
              day: "numeric",
            }).format(bucket)
          : new Intl.DateTimeFormat("id-ID", {
              month: "short",
              year: "2-digit",
            }).format(bucket),
      sales: bucketOrders.reduce((total, order) => total + order.total, 0),
      orders: bucketOrders.length,
    };
  });
}

export function getTopProducts(orderDetails: AnalyticsOrderDetail[]) {
  const productMap = new Map<string, TopProduct>();

  for (const order of orderDetails) {
    if (!isRevenueOrder(order)) continue;

    for (const item of order.items ?? []) {
      const current = productMap.get(item.product_id) ?? {
        productId: item.product_id,
        name: item.product_name,
        units: 0,
        revenue: 0,
      };

      current.units += item.quantity;
      current.revenue += item.subtotal;
      productMap.set(item.product_id, current);
    }
  }

  return Array.from(productMap.values())
    .sort((first, second) => second.revenue - first.revenue || second.units - first.units)
    .slice(0, 5);
}