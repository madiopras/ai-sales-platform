import { isAxiosError } from "axios";
import { aiApi } from "@/lib/api";

export type HandoverReason =
  | "customer_request"
  | "complaint"
  | "negotiation"
  | "payment_issue"
  | "shipping_issue"
  | "bulk_order"
  | "low_confidence";

export type InboxStatus = "open" | "resolved";

export interface InboxConversation {
  id: string;
  customer_name: string | null;
  customer_phone: string;
  reason: HandoverReason;
  status: InboxStatus;
  preview: string | null;
  last_message_at: string;
  assigned_to: string | null;
}

export interface ConversationMessage {
  id: string;
  direction: "inbound" | "outbound";
  body: string;
  sent_at: string;
  sender_name: string | null;
}

export interface ConversationDetail extends InboxConversation {
  messages: ConversationMessage[];
}

export class InboxIntegrationUnavailableError extends Error {
  constructor() {
    super("Inbox integration is not available.");
    this.name = "InboxIntegrationUnavailableError";
  }
}

function unwrap<T>(data: unknown): T {
  if (
    typeof data === "object" &&
    data !== null &&
    "data" in data &&
    (data as { data?: unknown }).data !== undefined
  ) {
    return (data as { data: T }).data;
  }

  return data as T;
}

function mapIntegrationError(error: unknown): never {
  if (isAxiosError(error) && [404, 405, 501].includes(error.response?.status ?? 0)) {
    throw new InboxIntegrationUnavailableError();
  }

  throw error;
}

/**
 * Contract reserved for the AI service's protected human-handover inbox API.
 * The service does not expose these endpoints yet; callers handle that state
 * explicitly instead of rendering non-production sample conversations.
 */
export async function getInboxConversations(status: InboxStatus = "open") {
  try {
    const response = await aiApi.get("/api/v1/admin/inbox/conversations", {
      params: { status },
    });
    return unwrap<InboxConversation[]>(response.data);
  } catch (error) {
    mapIntegrationError(error);
  }
}

export async function getInboxConversation(conversationId: string) {
  try {
    const response = await aiApi.get(
      `/api/v1/admin/inbox/conversations/${conversationId}`
    );
    return unwrap<ConversationDetail>(response.data);
  } catch (error) {
    mapIntegrationError(error);
  }
}

export async function sendInboxReply(conversationId: string, body: string) {
  try {
    const response = await aiApi.post(
      `/api/v1/admin/inbox/conversations/${conversationId}/messages`,
      { body }
    );
    return unwrap<ConversationMessage>(response.data);
  } catch (error) {
    mapIntegrationError(error);
  }
}

export async function resolveInboxConversation(conversationId: string) {
  try {
    await aiApi.post(`/api/v1/admin/inbox/conversations/${conversationId}/resolve`);
  } catch (error) {
    mapIntegrationError(error);
  }
}