"use client";

import { FormEvent, useMemo, useState } from "react";
import { formatDistanceToNow } from "date-fns";
import {
  AlertCircle,
  Check,
  Inbox,
  LoaderCircle,
  MessageSquare,
  RefreshCw,
  Search,
  Send,
} from "lucide-react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { cn } from "@/lib/utils";
import {
  ConversationDetail,
  getInboxConversation,
  getInboxConversations,
  HandoverReason,
  InboxConversation,
  InboxIntegrationUnavailableError,
  resolveInboxConversation,
  sendInboxReply,
} from "@/lib/inbox";

const EMPTY_CONVERSATIONS: InboxConversation[] = [];

const reasonLabels: Record<HandoverReason, string> = {
  customer_request: "Permintaan admin",
  complaint: "Keluhan",
  negotiation: "Negosiasi",
  payment_issue: "Kendala pembayaran",
  shipping_issue: "Kendala pengiriman",
  bulk_order: "Pesanan besar",
  low_confidence: "Butuh bantuan",
};

function formatTime(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "Waktu tidak tersedia";
  return formatDistanceToNow(date, { addSuffix: true });
}

function conversationName(conversation: InboxConversation | ConversationDetail) {
  return conversation.customer_name?.trim() || conversation.customer_phone;
}

function ConversationListItem({
  conversation,
  selected,
  onSelect,
}: {
  conversation: InboxConversation;
  selected: boolean;
  onSelect: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onSelect}
      className={cn(
        "w-full border-b border-slate-200 px-4 py-3 text-left transition-colors",
        selected ? "bg-slate-100" : "bg-white hover:bg-slate-50"
      )}
    >
      <div className="flex items-start justify-between gap-3">
        <p className="min-w-0 truncate text-sm font-semibold text-slate-900">
          {conversationName(conversation)}
        </p>
        <time className="shrink-0 text-xs text-slate-600">
          {formatTime(conversation.last_message_at)}
        </time>
      </div>
      <div className="mt-1 flex items-center gap-2">
        <span className="rounded-md bg-slate-100 px-1.5 py-0.5 text-xs font-medium text-slate-700">
          {reasonLabels[conversation.reason]}
        </span>
        {conversation.assigned_to && (
          <span className="truncate text-xs text-slate-600">
            {conversation.assigned_to}
          </span>
        )}
      </div>
      <p className="mt-2 line-clamp-1 text-sm text-slate-700">
        {conversation.preview || "Tidak ada pratinjau pesan."}
      </p>
    </button>
  );
}

function IntegrationUnavailable() {
  return (
    <div className="flex min-h-[440px] items-center justify-center rounded-lg border border-slate-200 bg-white p-6">
      <div className="max-w-md text-center">
        <div className="mx-auto flex h-10 w-10 items-center justify-center rounded-md bg-slate-100">
          <AlertCircle className="h-5 w-5 text-slate-700" />
        </div>
        <h2 className="mt-4 text-lg font-semibold text-slate-900">
          Inbox belum terhubung
        </h2>
        <p className="mt-2 text-sm leading-6 text-slate-700">
          Service AI belum menyediakan API inbox untuk mengambil handover, riwayat
          percakapan, atau mengirim balasan manual. Tidak ada percakapan contoh yang
          ditampilkan agar data operasional tetap akurat.
        </p>
        <p className="mt-3 text-sm leading-6 text-slate-700">
          Endpoint yang diperlukan: <code className="rounded bg-slate-100 px-1 py-0.5 text-xs">GET /api/v1/admin/inbox/conversations</code>.
        </p>
      </div>
    </div>
  );
}

function DetailPanel({
  conversation,
  isLoading,
  onResolve,
  isResolving,
}: {
  conversation: ConversationDetail | undefined;
  isLoading: boolean;
  onResolve: () => void;
  isResolving: boolean;
}) {
  const queryClient = useQueryClient();
  const [body, setBody] = useState("");
  const sendMutation = useMutation({
    mutationFn: () => sendInboxReply(conversation!.id, body.trim()),
    onSuccess: () => {
      setBody("");
      void queryClient.invalidateQueries({
        queryKey: ["inbox-conversation", conversation?.id],
      });
      void queryClient.invalidateQueries({ queryKey: ["inbox-conversations"] });
    },
  });

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!body.trim() || sendMutation.isPending) return;
    sendMutation.mutate();
  };

  if (isLoading) {
    return (
      <section className="flex min-h-[440px] items-center justify-center bg-white">
        <LoaderCircle className="h-5 w-5 animate-spin text-slate-600" />
        <span className="ml-2 text-sm text-slate-700">Memuat percakapan…</span>
      </section>
    );
  }

  if (!conversation) {
    return (
      <section className="flex min-h-[440px] items-center justify-center bg-white p-6 text-center">
        <div>
          <MessageSquare className="mx-auto h-6 w-6 text-slate-500" />
          <p className="mt-3 text-sm font-medium text-slate-900">
            Pilih percakapan untuk mulai membantu pelanggan
          </p>
        </div>
      </section>
    );
  }

  return (
    <section className="flex min-h-[520px] flex-col bg-white">
      <header className="flex items-start justify-between gap-4 border-b border-slate-200 px-5 py-4">
        <div className="min-w-0">
          <h2 className="truncate text-base font-semibold text-slate-900">
            {conversationName(conversation)}
          </h2>
          <p className="mt-1 text-sm text-slate-700">{conversation.customer_phone}</p>
          <span className="mt-2 inline-flex rounded-md bg-slate-100 px-1.5 py-0.5 text-xs font-medium text-slate-700">
            {reasonLabels[conversation.reason]}
          </span>
        </div>
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={onResolve}
          disabled={isResolving}
        >
          {isResolving ? (
            <LoaderCircle className="h-4 w-4 animate-spin" />
          ) : (
            <Check className="h-4 w-4" />
          )}
          Selesaikan
        </Button>
      </header>

      <div className="flex-1 space-y-4 overflow-y-auto bg-slate-50 p-5">
        {conversation.messages.length === 0 ? (
          <p className="py-10 text-center text-sm text-slate-700">
            Belum ada riwayat pesan yang tersedia.
          </p>
        ) : (
          conversation.messages.map((message) => {
            const isOutbound = message.direction === "outbound";
            return (
              <div
                key={message.id}
                className={cn("flex", isOutbound ? "justify-end" : "justify-start")}
              >
                <div
                  className={cn(
                    "max-w-[85%] rounded-lg px-3 py-2 text-sm leading-6",
                    isOutbound
                      ? "bg-slate-900 text-white"
                      : "border border-slate-200 bg-white text-slate-900"
                  )}
                >
                  <p>{message.body}</p>
                  <time
                    className={cn(
                      "mt-1 block text-xs",
                      isOutbound ? "text-slate-300" : "text-slate-600"
                    )}
                  >
                    {formatTime(message.sent_at)}
                  </time>
                </div>
              </div>
            );
          })
        )}
      </div>

      <form onSubmit={submit} className="border-t border-slate-200 p-4">
        {sendMutation.isError && (
          <p className="mb-2 text-sm text-red-700">
            Balasan tidak dapat dikirim. Coba lagi.
          </p>
        )}
        <div className="flex items-end gap-2">
          <Textarea
            value={body}
            onChange={(event) => setBody(event.target.value)}
            placeholder="Tulis balasan untuk pelanggan…"
            className="min-h-20 resize-none"
            disabled={sendMutation.isPending}
          />
          <Button
            type="submit"
            size="icon"
            disabled={!body.trim() || sendMutation.isPending}
            aria-label="Kirim balasan"
          >
            {sendMutation.isPending ? (
              <LoaderCircle className="h-4 w-4 animate-spin" />
            ) : (
              <Send className="h-4 w-4" />
            )}
          </Button>
        </div>
      </form>
    </section>
  );
}

export default function InboxPage() {
  const queryClient = useQueryClient();
  const [query, setQuery] = useState("");
  const [selectedId, setSelectedId] = useState<string | null>(null);

  const conversationsQuery = useQuery({
    queryKey: ["inbox-conversations"],
    queryFn: () => getInboxConversations("open"),
    retry: false,
    refetchInterval: 30_000,
  });

  const conversations = conversationsQuery.data ?? EMPTY_CONVERSATIONS;
  const filteredConversations = useMemo(() => {
    const normalizedQuery = query.trim().toLowerCase();
    if (!normalizedQuery) return conversations;

    return conversations.filter((conversation) =>
      [conversation.customer_name, conversation.customer_phone, conversation.preview]
        .filter(Boolean)
        .some((value) => value!.toLowerCase().includes(normalizedQuery))
    );
  }, [conversations, query]);

  const activeSelectedId = conversations.some(
    (conversation) => conversation.id === selectedId
  )
    ? selectedId
    : null;

  const detailQuery = useQuery({
    queryKey: ["inbox-conversation", activeSelectedId],
    queryFn: () => getInboxConversation(activeSelectedId!),
    enabled: Boolean(activeSelectedId),
    retry: false,
  });

  const resolveMutation = useMutation({
    mutationFn: () => resolveInboxConversation(activeSelectedId!),
    onSuccess: () => {
      setSelectedId(null);
      void queryClient.invalidateQueries({ queryKey: ["inbox-conversations"] });
    },
  });

  const isUnavailable =
    conversationsQuery.error instanceof InboxIntegrationUnavailableError;

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight text-slate-950">
            Inbox
          </h1>
          <p className="mt-1 text-sm text-slate-700">
            Tangani percakapan yang telah dialihkan oleh asisten AI.
          </p>
        </div>
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={() => void conversationsQuery.refetch()}
          disabled={conversationsQuery.isFetching}
        >
          <RefreshCw
            className={cn(
              "h-4 w-4",
              conversationsQuery.isFetching && "animate-spin"
            )}
          />
          Perbarui
        </Button>
      </div>

      {isUnavailable ? (
        <IntegrationUnavailable />
      ) : conversationsQuery.isError ? (
        <div className="flex min-h-[280px] items-center justify-center rounded-lg border border-slate-200 bg-white p-6 text-center">
          <div>
            <AlertCircle className="mx-auto h-6 w-6 text-red-700" />
            <p className="mt-3 text-sm font-medium text-slate-900">
              Inbox tidak dapat dimuat
            </p>
            <p className="mt-1 text-sm text-slate-700">
              Periksa koneksi ke AI service, lalu coba lagi.
            </p>
          </div>
        </div>
      ) : (
        <div className="overflow-hidden rounded-lg border border-slate-200 bg-white lg:grid lg:grid-cols-[minmax(280px,0.8fr)_minmax(0,1.7fr)]">
          <section className="border-b border-slate-200 lg:border-b-0 lg:border-r">
            <div className="border-b border-slate-200 p-3">
              <div className="relative">
                <Search className="pointer-events-none absolute left-3 top-2.5 h-4 w-4 text-slate-600" />
                <Input
                  value={query}
                  onChange={(event) => setQuery(event.target.value)}
                  placeholder="Cari nama atau nomor…"
                  className="pl-9"
                />
              </div>
            </div>
            <div className="max-h-[360px] overflow-y-auto lg:max-h-[calc(100vh-240px)]">
              {conversationsQuery.isLoading ? (
                <div className="flex h-44 items-center justify-center text-sm text-slate-700">
                  <LoaderCircle className="mr-2 h-4 w-4 animate-spin" />
                  Memuat handover…
                </div>
              ) : filteredConversations.length === 0 ? (
                <div className="px-5 py-12 text-center">
                  <Inbox className="mx-auto h-5 w-5 text-slate-500" />
                  <p className="mt-3 text-sm font-medium text-slate-900">
                    Tidak ada handover terbuka
                  </p>
                  <p className="mt-1 text-sm text-slate-700">
                    Percakapan yang dialihkan AI akan tampil di sini.
                  </p>
                </div>
              ) : (
                filteredConversations.map((conversation) => (
                  <ConversationListItem
                    key={conversation.id}
                    conversation={conversation}
                    selected={conversation.id === activeSelectedId}
                    onSelect={() => setSelectedId(conversation.id)}
                  />
                ))
              )}
            </div>
          </section>

          <DetailPanel
            conversation={detailQuery.data}
            isLoading={detailQuery.isLoading}
            onResolve={() => resolveMutation.mutate()}
            isResolving={resolveMutation.isPending}
          />
        </div>
      )}
    </div>
  );
}