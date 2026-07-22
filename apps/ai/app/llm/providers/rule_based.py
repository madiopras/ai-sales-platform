"""Rule-based chat model (no external LLM).

A deterministic default that lets the whole orchestration pipeline — prompt,
tool calling, guardrails — run end-to-end without any API key. It is
intentionally simple:

- On a product/price/recommendation intent, it requests a ``search_products``
  tool call (satisfying the Phase A4 deliverable: "ada kaos hitam ukuran XL?" →
  calls the tool → replies with only active, in-stock products).
- After the tool result comes back, it composes a short Bahasa Indonesia reply
  from that data only — never inventing price/stock (BR-004).
- For other intents it returns a friendly, state-appropriate acknowledgement and
  asks a clarifying question, mirroring the sales-rules "understand first" flow.

This is not a substitute for a real model's language ability; it is a safe,
offline stand-in and a reference for how a provider drives the tool loop.
"""

from __future__ import annotations

import json
from typing import Any

from app.llm.intent import Intent, detect_intent
from app.llm.types import ChatMessage, ToolCall

# Intents for which we should consult the catalog before replying.
_SEARCH_INTENTS = {
    Intent.PRODUCT_INQUIRY,
    Intent.PRICE_INQUIRY,
    Intent.RECOMMENDATION,
    Intent.COMPARISON,
}


class RuleBasedChatModel:
    """Deterministic ChatModel used when no LLM provider is configured."""

    name = "rule_based"

    async def complete(
        self,
        messages: list[ChatMessage],
        *,
        tools: list[dict[str, Any]] | None = None,  # noqa: ARG002 — schema unused here
    ) -> ChatMessage:
        last = messages[-1] if messages else None

        # Step 2 of the loop: a tool result is the most recent message → answer.
        if last is not None and last.role == "tool":
            return ChatMessage.assistant(content=self._reply_from_tool(last))

        # Step 1: look at the latest user message to decide whether to search.
        user_text = self._last_user_text(messages)
        intent = detect_intent(user_text)

        if intent in _SEARCH_INTENTS and user_text:
            # Request a catalog lookup; the orchestrator runs it and calls us back.
            call = ToolCall(id="call_search", name="search_products", arguments={"q": user_text})
            return ChatMessage.assistant(tool_calls=[call])

        return ChatMessage.assistant(content=self._acknowledge(intent))

    async def aclose(self) -> None:  # pragma: no cover - nothing to release
        return None

    # --- helpers ---------------------------------------------------------

    @staticmethod
    def _last_user_text(messages: list[ChatMessage]) -> str | None:
        for message in reversed(messages):
            if message.role == "user" and message.content:
                return message.content
        return None

    def _reply_from_tool(self, tool_message: ChatMessage) -> str:
        """Compose a reply strictly from a search_products tool result."""
        payload = self._parse(tool_message.content)
        if not payload.get("ok", False):
            return (
                "Maaf ya, aku lagi kesulitan mengambil datanya. Boleh dicoba "
                "lagi sebentar? Kalau masih gagal, aku bantu hubungkan ke admin."
            )

        data = payload.get("data") or {}
        products = data.get("products") if isinstance(data, dict) else None
        if not products:
            return (
                "Maaf, untuk saat ini aku belum menemukan produk yang cocok dan "
                "tersedia stoknya. Boleh sebutkan detail lain (misalnya warna, "
                "ukuran, atau budget) biar aku carikan alternatifnya?"
            )

        lines = ["Ini beberapa produk yang tersedia:"]
        for product in products[:5]:
            lines.append(self._format_product(product))
        lines.append("\nMau aku bantu masukkan salah satu ke keranjang?")
        return "\n".join(lines)

    @staticmethod
    def _format_product(product: dict[str, Any]) -> str:
        name = product.get("name") or "Produk"
        variants = product.get("variants") or []
        if not variants:
            return f"• {name}"
        # Show the first variant's price as a representative figure; all figures
        # come straight from the backend (BR-004).
        first = variants[0]
        price = first.get("price")
        if isinstance(price, (int, float)):
            price_txt = f"Rp{int(price):,}".replace(",", ".")
        else:
            price_txt = "-"

        variant_names = ", ".join(
            v.get("name") or v.get("sku") or "" for v in variants[:4]
        ).strip(", ")
        detail = f" (varian: {variant_names})" if variant_names else ""
        return f"• {name} — mulai {price_txt}{detail}"

    @staticmethod
    def _acknowledge(intent: Intent) -> str:
        return _ACK.get(intent, _ACK[Intent.GENERAL])

    @staticmethod
    def _parse(content: str | None) -> dict[str, Any]:
        if not content:
            return {}
        try:
            parsed = json.loads(content)
            return parsed if isinstance(parsed, dict) else {}
        except (ValueError, TypeError):
            return {}


# Short, friendly acknowledgements per intent. No domain data here — those come
# from tools. Kept in Bahasa Indonesia per the system prompt default.
_ACK: dict[Intent, str] = {
    Intent.GREETING: "Halo! 😊 Ada yang bisa aku bantu hari ini?",
    Intent.SHIPPING: (
        "Untuk cek ongkir, boleh aku tahu alamat tujuannya (kota/kecamatan) "
        "dan produk yang mau dikirim?"
    ),
    Intent.PAYMENT: (
        "Nanti setelah pesanan siap, aku bantu buatkan info pembayarannya ya. "
        "Sekarang boleh kita pastikan dulu produk dan pengirimannya?"
    ),
    Intent.CHECKOUT: (
        "Siap, sebelum lanjut aku ringkas dulu pesanannya biar tidak ada yang "
        "keliru ya."
    ),
    Intent.ORDER_TRACKING: (
        "Boleh aku dibantu nomor pesanannya? Nanti aku cek status dan "
        "pengirimannya."
    ),
    Intent.COMPLAINT: (
        "Maaf atas ketidaknyamanannya. Aku bantu teruskan ke admin kami ya "
        "supaya ditangani lebih lanjut."
    ),
    Intent.HANDOVER: "Baik, aku hubungkan ke admin kami ya. Mohon tunggu sebentar.",
    Intent.GENERAL: (
        "Boleh cerita sedikit kebutuhan kamu? Misalnya produk seperti apa yang "
        "sedang dicari, biar aku bantu carikan yang paling cocok."
    ),
}
