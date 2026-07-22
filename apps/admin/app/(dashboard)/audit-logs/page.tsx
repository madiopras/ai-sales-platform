"use client";

import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { CircleAlert, FileSearch, RefreshCw, Search, X } from "lucide-react";
import api from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { formatDateTime } from "@/lib/vouchers";

interface AuditLog {
  id: string;
  actor_user_id?: string;
  actor_email: string;
  action: string;
  resource_type: string;
  resource_id: string;
  metadata: Record<string, unknown>;
  created_at: string;
}

type DateFilter = "all" | "today" | "week" | "month";

function matchesDateRange(createdAt: string, filter: DateFilter) {
  if (filter === "all") return true;

  const createdDate = new Date(createdAt);
  const now = new Date();
  const start = new Date(now);
  start.setHours(0, 0, 0, 0);

  if (filter === "week") start.setDate(start.getDate() - 6);
  if (filter === "month") start.setMonth(start.getMonth() - 1);

  return createdDate >= start && createdDate <= now;
}

function MetadataPreview({ metadata }: { metadata: Record<string, unknown> }) {
  const content = Object.keys(metadata ?? {}).length ? JSON.stringify(metadata, null, 2) : "—";

  return (
    <pre className="max-h-24 overflow-auto whitespace-pre-wrap break-words rounded-md bg-slate-50 p-2 text-xs leading-5 text-slate-700">
      {content}
    </pre>
  );
}

export default function AuditLogsPage() {
  const [query, setQuery] = useState("");
  const [resourceType, setResourceType] = useState("");
  const [action, setAction] = useState("");
  const [dateFilter, setDateFilter] = useState<DateFilter>("all");
  const [selectedLog, setSelectedLog] = useState<AuditLog | null>(null);

  const {
    data: logs = [],
    isLoading,
    isError,
    refetch,
    isFetching,
  } = useQuery<AuditLog[]>({
    queryKey: ["audit-logs", resourceType],
    queryFn: async () => {
      const response = await api.get("/api/v1/audit-logs", {
        params: { limit: 100, ...(resourceType ? { resource_type: resourceType } : {}) },
      });
      return response.data.data.logs;
    },
  });

  const resourceTypes = useMemo(
    () =>
      Array.from(new Set(logs.map((log) => log.resource_type).filter(Boolean))).sort((first, second) =>
        first.localeCompare(second)
      ),
    [logs]
  );

  const actions = useMemo(
    () =>
      Array.from(new Set(logs.map((log) => log.action).filter(Boolean))).sort((first, second) =>
        first.localeCompare(second)
      ),
    [logs]
  );

  const filteredLogs = useMemo(() => {
    const normalizedQuery = query.trim().toLowerCase();

    return logs.filter((log) => {
      const matchesQuery =
        !normalizedQuery ||
        [
          log.actor_email,
          log.action,
          log.resource_type,
          log.resource_id,
          JSON.stringify(log.metadata ?? {}),
        ]
          .filter(Boolean)
          .some((value) => value.toLowerCase().includes(normalizedQuery));

      return matchesQuery && (!action || log.action === action) && matchesDateRange(log.created_at, dateFilter);
    });
  }, [action, dateFilter, logs, query]);

  const clearFilters = () => {
    setQuery("");
    setResourceType("");
    setAction("");
    setDateFilter("all");
  };

  const hasActiveFilters = Boolean(query || resourceType || action || dateFilter !== "all");

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-4 border-b border-slate-200 pb-5 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight text-slate-950">Audit log</h1>
          <p className="mt-1.5 text-sm text-slate-600">
            Telusuri perubahan administratif dan konteks tindakan yang tercatat.
          </p>
        </div>
        <Button variant="outline" size="sm" onClick={() => refetch()} disabled={isFetching}>
          <RefreshCw className={`h-4 w-4 ${isFetching ? "animate-spin" : ""}`} />
          Refresh data
        </Button>
      </div>

      <section className="rounded-lg border border-slate-200 bg-white p-4">
        <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-[minmax(230px,1.4fr)_repeat(3,minmax(150px,0.7fr))_auto]">
          <div className="relative">
            <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-600" />
            <Input
              aria-label="Cari audit log"
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder="Cari pelaku, aksi, resource, atau metadata"
              className="pl-9"
            />
          </div>
          <select
            aria-label="Filter audit log berdasarkan resource"
            value={resourceType}
            onChange={(event) => setResourceType(event.target.value)}
            className="h-9 rounded-md border border-slate-300 bg-white px-3 text-sm text-slate-900 outline-none focus:border-slate-900 focus:ring-2 focus:ring-slate-200"
          >
            <option value="">Semua resource</option>
            {resourceTypes.map((type) => (
              <option key={type} value={type}>
                {type}
              </option>
            ))}
          </select>
          <select
            aria-label="Filter audit log berdasarkan aksi"
            value={action}
            onChange={(event) => setAction(event.target.value)}
            className="h-9 rounded-md border border-slate-300 bg-white px-3 text-sm text-slate-900 outline-none focus:border-slate-900 focus:ring-2 focus:ring-slate-200"
          >
            <option value="">Semua aksi</option>
            {actions.map((item) => (
              <option key={item} value={item}>
                {item}
              </option>
            ))}
          </select>
          <select
            aria-label="Filter audit log berdasarkan waktu"
            value={dateFilter}
            onChange={(event) => setDateFilter(event.target.value as DateFilter)}
            className="h-9 rounded-md border border-slate-300 bg-white px-3 text-sm text-slate-900 outline-none focus:border-slate-900 focus:ring-2 focus:ring-slate-200"
          >
            <option value="all">Semua waktu</option>
            <option value="today">Hari ini</option>
            <option value="week">7 hari terakhir</option>
            <option value="month">30 hari terakhir</option>
          </select>
          {hasActiveFilters ? (
            <Button variant="ghost" size="sm" onClick={clearFilters}>
              Reset
            </Button>
          ) : null}
        </div>
        <p className="mt-3 text-sm text-slate-600">
          Menampilkan <span className="font-medium tabular-nums text-slate-900">{filteredLogs.length}</span> dari{" "}
          <span className="font-medium tabular-nums text-slate-900">{logs.length}</span> aktivitas terbaru.
        </p>
      </section>

      {isError ? (
        <div className="flex gap-3 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-900">
          <CircleAlert className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
          <div>
            <p className="font-medium">Aktivitas audit tidak dapat dimuat.</p>
            <p className="mt-1 text-red-800">Periksa koneksi backend lalu refresh data.</p>
          </div>
        </div>
      ) : null}

      <div className="overflow-hidden rounded-lg border border-slate-200 bg-white">
        <div className="hidden overflow-x-auto md:block">
          <table className="w-full min-w-[890px] text-left text-sm">
            <thead className="border-b border-slate-200 bg-slate-50 text-xs font-medium text-slate-700">
              <tr>
                <th scope="col" className="px-4 py-3">Waktu</th>
                <th scope="col" className="px-4 py-3">Pelaku</th>
                <th scope="col" className="px-4 py-3">Aksi</th>
                <th scope="col" className="px-4 py-3">Resource</th>
                <th scope="col" className="px-4 py-3">Metadata</th>
                <th scope="col" className="px-4 py-3"><span className="sr-only">Detail</span></th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-200">
              {isLoading ? (
                <tr><td colSpan={6} className="px-4 py-12 text-center text-slate-600">Memuat aktivitas audit…</td></tr>
              ) : filteredLogs.length === 0 ? (
                <tr><td colSpan={6} className="px-4 py-12 text-center text-slate-600">Tidak ada aktivitas yang sesuai dengan filter.</td></tr>
              ) : (
                filteredLogs.map((log) => (
                  <tr key={log.id} className="hover:bg-slate-50">
                    <td className="whitespace-nowrap px-4 py-3 align-top text-slate-700">{formatDateTime(log.created_at)}</td>
                    <td className="px-4 py-3 align-top font-medium text-slate-900">{log.actor_email || "System"}</td>
                    <td className="px-4 py-3 align-top"><span className="rounded-md bg-slate-100 px-2 py-1 text-xs font-medium text-slate-800">{log.action}</span></td>
                    <td className="px-4 py-3 align-top text-slate-700"><p>{log.resource_type}</p><p className="mt-0.5 max-w-48 truncate font-mono text-xs text-slate-600">{log.resource_id}</p></td>
                    <td className="max-w-sm px-4 py-3 align-top"><MetadataPreview metadata={log.metadata} /></td>
                    <td className="px-4 py-3 align-top"><Button variant="ghost" size="sm" onClick={() => setSelectedLog(log)}>Detail</Button></td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>

        <div className="divide-y divide-slate-200 md:hidden">
          {isLoading ? (
            <p className="px-4 py-12 text-center text-sm text-slate-600">Memuat aktivitas audit…</p>
          ) : filteredLogs.length === 0 ? (
            <p className="px-4 py-12 text-center text-sm text-slate-600">Tidak ada aktivitas yang sesuai dengan filter.</p>
          ) : (
            filteredLogs.map((log) => (
              <article key={log.id} className="p-4">
                <div className="flex items-start justify-between gap-3">
                  <span className="rounded-md bg-slate-100 px-2 py-1 text-xs font-medium text-slate-800">{log.action}</span>
                  <p className="shrink-0 text-xs text-slate-600">{formatDateTime(log.created_at)}</p>
                </div>
                <p className="mt-3 text-sm font-medium text-slate-900">{log.actor_email || "System"}</p>
                <p className="mt-1 text-sm text-slate-700">{log.resource_type}</p>
                <p className="mt-0.5 truncate font-mono text-xs text-slate-600">{log.resource_id}</p>
                <Button variant="ghost" size="sm" className="mt-3 -ml-2" onClick={() => setSelectedLog(log)}>
                  Lihat detail
                </Button>
              </article>
            ))
          )}
        </div>
      </div>

      {selectedLog ? (
        <div
          className="fixed inset-0 z-50 flex items-end bg-slate-950/35 p-0 sm:items-center sm:justify-center sm:p-6"
          role="presentation"
          onMouseDown={() => setSelectedLog(null)}
        >
          <section
            role="dialog"
            aria-modal="true"
            aria-labelledby="audit-detail-title"
            className="max-h-[85vh] w-full overflow-auto rounded-t-lg bg-white p-5 sm:max-w-2xl sm:rounded-lg"
            onMouseDown={(event) => event.stopPropagation()}
          >
            <div className="flex items-start justify-between gap-4 border-b border-slate-200 pb-4">
              <div>
                <h2 id="audit-detail-title" className="text-lg font-semibold text-slate-950">Detail aktivitas</h2>
                <p className="mt-1 text-sm text-slate-600">{formatDateTime(selectedLog.created_at)}</p>
              </div>
              <Button variant="ghost" size="icon" aria-label="Tutup detail" onClick={() => setSelectedLog(null)}>
                <X className="h-4 w-4" />
              </Button>
            </div>

            <dl className="mt-5 grid gap-4 sm:grid-cols-2">
              <div><dt className="text-sm text-slate-600">Pelaku</dt><dd className="mt-1 break-words text-sm font-medium text-slate-900">{selectedLog.actor_email || "System"}</dd></div>
              <div><dt className="text-sm text-slate-600">Aksi</dt><dd className="mt-1 text-sm font-medium text-slate-900">{selectedLog.action}</dd></div>
              <div><dt className="text-sm text-slate-600">Jenis resource</dt><dd className="mt-1 text-sm font-medium text-slate-900">{selectedLog.resource_type}</dd></div>
              <div><dt className="text-sm text-slate-600">ID resource</dt><dd className="mt-1 break-all font-mono text-xs text-slate-900">{selectedLog.resource_id}</dd></div>
              <div className="sm:col-span-2"><dt className="text-sm text-slate-600">ID audit</dt><dd className="mt-1 break-all font-mono text-xs text-slate-900">{selectedLog.id}</dd></div>
              <div className="sm:col-span-2"><dt className="text-sm text-slate-600">Metadata</dt><dd className="mt-2"><pre className="max-h-80 overflow-auto whitespace-pre-wrap break-words rounded-md bg-slate-50 p-3 text-xs leading-5 text-slate-800">{Object.keys(selectedLog.metadata ?? {}).length ? JSON.stringify(selectedLog.metadata, null, 2) : "Tidak ada metadata."}</pre></dd></div>
            </dl>
          </section>
        </div>
      ) : null}

      {!isLoading && !isError && logs.length === 0 ? (
        <div className="flex flex-col items-center justify-center rounded-lg border border-dashed border-slate-300 bg-slate-50 px-6 py-12 text-center">
          <FileSearch className="h-7 w-7 text-slate-400" aria-hidden="true" />
          <p className="mt-3 text-sm font-medium text-slate-800">Belum ada aktivitas audit</p>
          <p className="mt-1 max-w-sm text-sm leading-5 text-slate-600">
            Aktivitas administratif yang direkam akan muncul di halaman ini.
          </p>
        </div>
      ) : null}
    </div>
  );
}