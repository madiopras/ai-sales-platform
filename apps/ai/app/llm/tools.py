"""Tool/function-calling framework.

Each tool is a thin, typed wrapper over the Go backend client (``backend.py``).
Tools are the ONLY way the agent obtains or mutates domain data, so tool results
are the source of truth (BR-004): the model must never invent
price/stock/shipping or operate on data it made up.

This module owns:

- :class:`Tool` — name, description, JSON-schema parameters, and an async
  handler ``(args, context) -> JSON-serializable``.
- :class:`ToolContext` — per-turn, server-controlled context (the customer's
  WhatsApp id + the mutable conversation working-memory). Customer-scoped tools
  read identity from here, never from model-supplied arguments, so the model
  cannot act on another customer's cart/data (privacy; sales-rules).
- :class:`ToolRegistry` — registration, schema export, and safe dispatch that
  maps backend errors to structured, non-leaking failures.

Phase A4 registered the read-only product tools. Phase A5 adds the sales-flow
tools — customer upsert, addresses, and cart CRUD (BR-006, BR-007, BR-008,
BR-009) — using the same framework. Later phases add shipping, checkout,
payment, and order tools.

Guardrails baked in:
- Product results are filtered to what a customer may act on: ``active`` products
  and variants with available stock (BR-004, sales-rules "Stock Rules").
- Customer identity is resolved from the session (phone = WhatsApp id), not from
  the model, and cached on the conversation context after the first upsert.
- Backend *client* errors (4xx) with a known code are surfaced as actionable
  results (e.g. ``INSUFFICIENT_STOCK``) so the model can explain the problem;
  server/unreachable errors collapse to ``backend_unavailable`` (BR-044).
"""

from __future__ import annotations

from collections.abc import Awaitable, Callable
from dataclasses import dataclass
from typing import Any

from app.clients.backend import BackendClient
from app.conversation.models import ConversationContext
from app.knowledge import KnowledgeBase, load_knowledge_base
from app.llm.types import ToolCall, ToolResult
from app.platform.errors import BackendError
from app.platform.logging import get_logger


logger = get_logger(__name__)


@dataclass(slots=True)
class ToolContext:
    """Server-controlled context for a single turn's tool calls.

    - ``wa_id``: the customer's WhatsApp id / phone (identity; never taken from
      model arguments).
    - ``session_context``: the mutable conversation working-memory. Tools cache
      resolved backend references here (``customer_id``, ``cart_id``,
      ``address_id``); the caller persists the session after the reply.
    """

    wa_id: str | None = None
    session_context: ConversationContext | None = None


# Handlers receive parsed arguments plus the per-turn context.
ToolHandler = Callable[[dict[str, Any], ToolContext], Awaitable[Any]]

# Backend client-error codes that are safe + useful to expose to the model so it
# can explain the outcome to the customer. Anything else is treated as opaque.
_ACTIONABLE_CODES = {
    # cart / catalog
    "INSUFFICIENT_STOCK",
    "VARIANT_NOT_FOUND",
    "CART_NOT_FOUND",
    "CART_ITEM_NOT_FOUND",
    "CUSTOMER_NOT_FOUND",
    "ADDRESS_NOT_FOUND",
    "VALIDATION_ERROR",
    # checkout (Phase A6)
    "EMPTY_CART",
    "ADDRESS_REQUIRED",
    "ADDRESS_INCOMPLETE",
    "COURIER_REQUIRED",
    "ORDER_NO_CONFLICT",
    # shipping (Phase A6)
    "RATES_UNAVAILABLE",
    "SHIPMENT_NOT_FOUND",
    # payment (Phase A6)
    "ORDER_NOT_FOUND",
    "ORDER_NOT_PENDING_PAYMENT",
    "INVOICE_ALREADY_EXISTS",
    "INVOICE_NOT_FOUND",
    # vouchers (Phase A6 / BR-037..BR-038)
    "VOUCHER_NOT_FOUND",
    "VOUCHER_NOT_USABLE",
    "VOUCHER_INACTIVE",
    "VOUCHER_EXPIRED",
    "VOUCHER_NOT_STARTED",
    "VOUCHER_EXHAUSTED",
    "VOUCHER_MIN_NOT_MET",
    "MIN_ORDER_NOT_MET",
}



@dataclass(slots=True)
class Tool:
    """A callable exposed to the model.

    ``parameters`` is a JSON Schema object (OpenAI function-calling compatible).
    ``handler`` receives the parsed arguments dict + :class:`ToolContext` and
    returns any JSON-serializable value.
    """

    name: str
    description: str
    parameters: dict[str, Any]
    handler: ToolHandler

    def schema(self) -> dict[str, Any]:
        """OpenAI-style function schema for this tool."""
        return {
            "type": "function",
            "function": {
                "name": self.name,
                "description": self.description,
                "parameters": self.parameters,
            },
        }


class ToolRegistry:
    """Holds the available tools and dispatches calls safely."""

    def __init__(self) -> None:
        self._tools: dict[str, Tool] = {}

    def register(self, tool: Tool) -> None:
        if tool.name in self._tools:
            raise ValueError(f"tool already registered: {tool.name}")
        self._tools[tool.name] = tool

    def get(self, name: str) -> Tool | None:
        return self._tools.get(name)

    def names(self) -> list[str]:
        return list(self._tools)

    def schemas(self) -> list[dict[str, Any]]:
        """Function schemas for every registered tool (for the provider)."""
        return [tool.schema() for tool in self._tools.values()]

    async def dispatch(
        self, call: ToolCall, context: ToolContext | None = None
    ) -> ToolResult:
        """Run a tool call, never raising: failures become a structured result.

        A missing tool or a backend error is returned as ``ok=False`` so the
        orchestrator can feed it back to the model, which then apologizes /
        explains / offers handover instead of fabricating data.
        """
        context = context or ToolContext()
        tool = self._tools.get(call.name)
        if tool is None:
            logger.warning("tool_unknown", extra={"tool": call.name})
            return ToolResult(call.id, call.name, ok=False, content="unknown tool")

        try:
            data = await tool.handler(call.arguments or {}, context)
            return ToolResult(call.id, call.name, ok=True, content=data)
        except BackendError as exc:
            # Client errors with a known code are actionable: surface the code so
            # the model can explain (e.g. out of stock). Server/unreachable
            # errors stay opaque so we never leak backend internals (BR-044).
            if (
                exc.status_code
                and exc.status_code < 500
                and exc.upstream_code in _ACTIONABLE_CODES
            ):
                content: Any = {"error": "request_rejected", "code": exc.upstream_code}
            else:
                content = {"error": "backend_unavailable"}
            logger.warning(
                "tool_backend_error",
                extra={
                    "tool": call.name,
                    "status": exc.status_code,
                    "upstream_code": exc.upstream_code,
                },
            )
            return ToolResult(call.id, call.name, ok=False, content=content)
        except Exception:  # noqa: BLE001 — dispatch must never crash the turn
            logger.exception("tool_unexpected_error", extra={"tool": call.name})
            return ToolResult(
                call.id, call.name, ok=False, content={"error": "internal_error"}
            )


# --- Product result shaping ------------------------------------------------


def _shape_variant(variant: dict[str, Any]) -> dict[str, Any] | None:
    """Trim a variant to customer-actionable fields; drop unavailable ones.

    Backend variant shapes differ slightly (``stock`` on search results vs.
    ``stock_on_hand``/``stock_reserved`` on management views), so compute an
    ``available`` value defensively and hide variants with none (BR-004).
    """
    if variant.get("is_active") is False:
        return None

    stock = variant.get("stock")
    if stock is None:
        on_hand = variant.get("stock_on_hand", 0) or 0
        reserved = variant.get("stock_reserved", 0) or 0
        stock = on_hand - reserved
    try:
        available = int(stock)
    except (TypeError, ValueError):
        available = 0
    if available <= 0:
        return None

    return {
        "id": variant.get("id"),
        "sku": variant.get("sku"),
        "name": variant.get("name"),
        "price": variant.get("price"),
        "available_stock": available,
    }


def _shape_product(product: dict[str, Any]) -> dict[str, Any] | None:
    """Keep only active products with at least one purchasable variant."""
    if product.get("status") not in (None, "active"):
        return None

    variants_in = product.get("variants") or []
    variants = [v for v in (_shape_variant(x) for x in variants_in) if v is not None]
    # A product with variant data but nothing in stock isn't actionable; hide it
    # so the model offers alternatives instead (sales-rules "Stock Rules").
    if variants_in and not variants:
        return None

    return {
        "id": product.get("id"),
        "name": product.get("name"),
        "slug": product.get("slug"),
        "description": product.get("description"),
        "variants": variants,
    }


def _shape_products(products: Any) -> list[dict[str, Any]]:
    if not isinstance(products, list):
        return []
    shaped = [_shape_product(p) for p in products if isinstance(p, dict)]
    return [p for p in shaped if p is not None]


# --- Cart / address result shaping -----------------------------------------


def _to_number(value: Any) -> float:
    try:
        return float(value)
    except (TypeError, ValueError):
        return 0.0


def _shape_cart(cart: Any) -> dict[str, Any]:
    """Shape a backend cart into a compact, model-friendly summary.

    Line/subtotal figures are derived from the backend's per-item ``unit_price``
    (the source of truth captured when the item was added); the authoritative
    order total incl. shipping/discount still comes from checkout preview in a
    later phase (BR-014, BR-015).
    """
    if not isinstance(cart, dict):
        return {"items": [], "item_count": 0, "subtotal": 0}

    items_in = cart.get("items") or []
    items: list[dict[str, Any]] = []
    subtotal = 0.0
    for item in items_in:
        if not isinstance(item, dict):
            continue
        qty = int(item.get("qty") or 0)
        unit_price = _to_number(item.get("unit_price"))
        line_total = unit_price * qty
        subtotal += line_total
        items.append(
            {
                "variant_id": item.get("variant_id"),
                "qty": qty,
                "unit_price": unit_price,
                "line_total": line_total,
            }
        )

    return {
        "id": cart.get("id"),
        "status": cart.get("status"),
        "items": items,
        "item_count": sum(i["qty"] for i in items),
        "subtotal": subtotal,
    }


def _shape_address(address: Any) -> dict[str, Any] | None:
    if not isinstance(address, dict) or not address.get("id"):
        return None
    return {
        "id": address.get("id"),
        "label": address.get("label"),
        "recipient_name": address.get("recipient_name"),
        "phone": address.get("phone"),
        "address_line": address.get("address_line"),
        "city": address.get("city"),
        "district": address.get("district"),
        "postal_code": address.get("postal_code"),
        "notes": address.get("notes"),
        "is_default": address.get("is_default"),
    }


# --- Shipping / checkout / payment result shaping (Phase A6) ----------------


def _shape_rate(rate: Any) -> dict[str, Any] | None:
    if not isinstance(rate, dict):
        return None
    return {
        "courier_code": rate.get("courier_code"),
        "courier_service": rate.get("courier_service"),
        "courier_name": rate.get("courier_name"),
        "service_name": rate.get("service_name"),
        "duration": rate.get("duration"),
        "price": _to_number(rate.get("price")),
    }


def _shape_rates(rates: Any) -> list[dict[str, Any]]:
    if not isinstance(rates, list):
        return []
    return [r for r in (_shape_rate(x) for x in rates) if r is not None]


def _shape_order_item(item: Any) -> dict[str, Any] | None:
    if not isinstance(item, dict):
        return None
    return {
        "variant_id": item.get("variant_id"),
        "sku": item.get("sku"),
        "product_name": item.get("product_name"),
        "variant_name": item.get("variant_name"),
        "qty": int(item.get("qty") or 0),
        "unit_price": _to_number(item.get("unit_price")),
        "subtotal": _to_number(item.get("subtotal")),
    }


def _shape_items(items: Any) -> list[dict[str, Any]]:
    if not isinstance(items, list):
        return []
    return [i for i in (_shape_order_item(x) for x in items) if i is not None]


def _shape_address_snap(address: Any) -> dict[str, Any]:
    if not isinstance(address, dict):
        return {}
    return {
        "recipient_name": address.get("recipient_name"),
        "phone": address.get("phone"),
        "address_line": address.get("address_line"),
        "city": address.get("city"),
        "district": address.get("district"),
        "postal_code": address.get("postal_code"),
        "notes": address.get("notes"),
    }


def _shape_courier(courier: Any) -> dict[str, Any]:
    if not isinstance(courier, dict):
        return {}
    return {
        "code": courier.get("code"),
        "service": courier.get("service"),
        "fee": _to_number(courier.get("fee")),
    }


def _shape_preview(preview: Any) -> dict[str, Any]:
    """Shape a checkout preview into a compact summary for the model.

    All monetary figures come straight from the backend — the AI never computes
    totals/shipping (BR-004, BR-015).
    """
    if not isinstance(preview, dict):
        return {}
    return {
        "items": _shape_items(preview.get("items")),
        "address": _shape_address_snap(preview.get("address")),
        "courier": _shape_courier(preview.get("courier")),
        "voucher_code": preview.get("voucher_code"),
        "discount": _to_number(preview.get("discount")),
        "subtotal": _to_number(preview.get("subtotal")),
        "shipping_fee": _to_number(preview.get("shipping_fee")),
        "total": _to_number(preview.get("total")),
    }


def _shape_order(order: Any) -> dict[str, Any]:
    if not isinstance(order, dict):
        return {}
    return {
        "id": order.get("id"),
        "order_no": order.get("order_no"),
        "status": order.get("status"),
        "recipient_name": order.get("recipient_name"),
        "phone": order.get("phone"),
        "address_line": order.get("address_line"),
        "city": order.get("city"),
        "district": order.get("district"),
        "postal_code": order.get("postal_code"),
        "courier_code": order.get("courier_code"),
        "courier_service": order.get("courier_service"),
        "tracking_no": order.get("tracking_no"),
        "voucher_code": order.get("voucher_code"),
        "discount": _to_number(order.get("discount")),
        "subtotal": _to_number(order.get("subtotal")),
        "shipping_fee": _to_number(order.get("shipping_fee")),
        "total": _to_number(order.get("total")),
        "items": _shape_items(order.get("items")),
    }


def _shape_invoice(invoice: Any) -> dict[str, Any]:
    """Shape an invoice: the payment URL + status are what the customer needs.

    Never assert an order is paid from the AI side — status flips to ``paid``
    only via the Xendit webhook the backend verifies (payment rules / Phase A7).
    """
    if not isinstance(invoice, dict):
        return {}
    return {
        "id": invoice.get("id"),
        "invoice_no": invoice.get("invoice_no"),
        "order_id": invoice.get("order_id"),
        "amount": _to_number(invoice.get("amount")),
        "status": invoice.get("status"),
        "payment_url": invoice.get("xendit_payment_url"),
        "payment_channel": invoice.get("payment_channel"),
        "payment_method": invoice.get("xendit_payment_method"),
        "expired_at": invoice.get("expired_at"),
        "paid_at": invoice.get("paid_at"),
    }


def _shape_shipment(shipment: Any) -> dict[str, Any]:
    if not isinstance(shipment, dict):
        return {}
    return {
        "order_id": shipment.get("order_id"),
        "courier_code": shipment.get("courier_code"),
        "courier_service": shipment.get("courier_service"),
        "courier_name": shipment.get("courier_name"),
        "status": shipment.get("status"),
        "tracking_no": shipment.get("tracking_no"),
        "waybill_id": shipment.get("waybill_id"),
        "label_url": shipment.get("label_url"),
        "delivered_at": shipment.get("delivered_at"),
    }


def _shape_voucher_result(result: Any) -> dict[str, Any]:
    if not isinstance(result, dict):
        return {}
    return {
        "code": result.get("code"),
        "discount_type": result.get("discount_type"),
        "discount": _to_number(result.get("discount")),
        "subtotal": _to_number(result.get("subtotal")),
    }



# --- Tool builders ---------------------------------------------------------


def build_registry(
    backend: BackendClient, knowledge: KnowledgeBase | None = None
) -> ToolRegistry:
    """Create the registry with product (A4) + sales-flow (A5) + KB (A9) tools.

    ``knowledge`` is the Phase A9 knowledge base used by the ``search_knowledge``
    tool; when omitted it is loaded from the repo default. Later phases extend
    this using the same :class:`Tool` shape.
    """
    registry = ToolRegistry()
    knowledge = knowledge if knowledge is not None else load_knowledge_base()

    async def _resolve_customer_id(context: ToolContext) -> str | None:
        """Return the customer id for this session, upserting by phone if needed.

        Identity is the WhatsApp id (phone); the model never supplies it. The
        resolved id is cached on the conversation context so subsequent tool
        calls in the same session reuse it (BR-008).
        """
        ctx = context.session_context
        if ctx is not None and ctx.customer_id:
            return ctx.customer_id
        if not context.wa_id:
            return None
        customer = await backend.post(
            "/customers",
            json={"phone": context.wa_id, "name": (ctx.name if ctx else None) or ""},
        )
        customer_id = customer.get("id") if isinstance(customer, dict) else None
        if customer_id and ctx is not None:
            ctx.customer_id = customer_id
        return customer_id

    def _no_customer() -> dict[str, Any]:
        return {"error": "no_customer_context"}

    # --- product tools (Phase A4) ---------------------------------------

    async def _search_products(args: dict[str, Any], _: ToolContext) -> Any:
        query = (args.get("q") or args.get("query") or "").strip()
        limit = args.get("limit", 5)
        try:
            limit = max(1, min(int(limit), 20))
        except (TypeError, ValueError):
            limit = 5
        raw = await backend.get("/products/search", params={"q": query, "limit": limit})
        return {"products": _shape_products(raw)}

    async def _get_product(args: dict[str, Any], context: ToolContext) -> Any:
        slug = (args.get("slug") or "").strip()
        product_id = (args.get("id") or "").strip()
        if slug:
            raw = await backend.get(f"/products/slug/{slug}")
        elif product_id:
            raw = await backend.get(f"/products/{product_id}")
        else:
            return {"error": "provide id or slug"}
        shaped = _shape_product(raw) if isinstance(raw, dict) else None
        # Remember the product in context so follow-up cart calls have a hint.
        if shaped and context.session_context is not None:
            context.session_context.product_id = shaped.get("id")
        return {"product": shaped}

    # --- cart tools (Phase A5) ------------------------------------------

    async def _get_cart(_: dict[str, Any], context: ToolContext) -> Any:
        customer_id = await _resolve_customer_id(context)
        if not customer_id:
            return _no_customer()
        raw = await backend.get(f"/customers/{customer_id}/cart")
        cart = _shape_cart(raw)
        if context.session_context is not None:
            context.session_context.cart_id = cart.get("id")
        return {"cart": cart}

    async def _add_cart_item(args: dict[str, Any], context: ToolContext) -> Any:
        variant_id = (args.get("variant_id") or "").strip()
        if not variant_id:
            return {"error": "variant_id is required"}
        try:
            qty = int(args.get("qty", 1))
        except (TypeError, ValueError):
            qty = 0
        if qty <= 0:
            return {"error": "qty must be a positive integer"}
        customer_id = await _resolve_customer_id(context)
        if not customer_id:
            return _no_customer()
        raw = await backend.post(
            f"/customers/{customer_id}/cart/items",
            json={"variant_id": variant_id, "qty": qty},
        )
        cart = _shape_cart(raw)
        if context.session_context is not None:
            context.session_context.cart_id = cart.get("id")
        return {"cart": cart}

    async def _update_cart_item(args: dict[str, Any], context: ToolContext) -> Any:
        variant_id = (args.get("variant_id") or "").strip()
        if not variant_id:
            return {"error": "variant_id is required"}
        try:
            qty = int(args.get("qty"))
        except (TypeError, ValueError):
            qty = 0
        if qty <= 0:
            return {"error": "qty must be a positive integer"}
        customer_id = await _resolve_customer_id(context)
        if not customer_id:
            return _no_customer()
        raw = await backend.put(
            f"/customers/{customer_id}/cart/items/{variant_id}",
            json={"qty": qty},
        )
        return {"cart": _shape_cart(raw)}

    async def _remove_cart_item(args: dict[str, Any], context: ToolContext) -> Any:
        variant_id = (args.get("variant_id") or "").strip()
        if not variant_id:
            return {"error": "variant_id is required"}
        customer_id = await _resolve_customer_id(context)
        if not customer_id:
            return _no_customer()
        raw = await backend.delete(f"/customers/{customer_id}/cart/items/{variant_id}")
        return {"cart": _shape_cart(raw)}

    # --- customer / address tools (Phase A5) ----------------------------

    async def _save_customer(args: dict[str, Any], context: ToolContext) -> Any:
        name = (args.get("name") or "").strip()
        if name and context.session_context is not None:
            context.session_context.name = name
        if not context.wa_id:
            return _no_customer()
        customer = await backend.post(
            "/customers", json={"phone": context.wa_id, "name": name}
        )
        customer_id = customer.get("id") if isinstance(customer, dict) else None
        if customer_id and context.session_context is not None:
            context.session_context.customer_id = customer_id
        return {
            "customer": {
                "id": customer_id,
                "name": customer.get("name") if isinstance(customer, dict) else name,
            }
        }

    async def _get_default_address(_: dict[str, Any], context: ToolContext) -> Any:
        customer_id = await _resolve_customer_id(context)
        if not customer_id:
            return _no_customer()
        try:
            raw = await backend.get(f"/customers/{customer_id}/addresses/default")
        except BackendError as exc:
            # No default address yet is a normal "empty" outcome, not an error.
            if exc.status_code == 404:
                return {"address": None}
            raise
        address = _shape_address(raw)
        if address and context.session_context is not None:
            context.session_context.address_id = address.get("id")
        return {"address": address}

    async def _create_address(args: dict[str, Any], context: ToolContext) -> Any:
        # Required shipping fields per BR-008 / BR-009.
        required = ("recipient_name", "phone", "address_line", "city", "district")
        payload = {key: (args.get(key) or "").strip() for key in required}
        missing = [key for key, value in payload.items() if not value]
        if missing:
            return {"error": "missing_fields", "fields": missing}
        payload["postal_code"] = (args.get("postal_code") or "").strip()
        payload["notes"] = (args.get("notes") or "").strip()
        payload["label"] = (args.get("label") or "").strip()
        payload["is_default"] = bool(args.get("is_default", True))

        customer_id = await _resolve_customer_id(context)
        if not customer_id:
            return _no_customer()
        raw = await backend.post(f"/customers/{customer_id}/addresses", json=payload)
        address = _shape_address(raw)
        if address and context.session_context is not None:
            context.session_context.address_id = address.get("id")
        return {"address": address}

    # --- shipping / checkout / payment tools (Phase A6) -----------------

    def _find_rate_fee(context: ToolContext, code: str, service: str) -> float | None:
        """Look up an authoritative courier fee from previously-fetched rates.

        The AI must never invent a shipping fee (BR-004). A courier can only be
        priced from a rate returned by ``get_shipping_rates`` this session; the
        fee is stashed on the context and matched here by code (+ service).
        """
        ctx = context.session_context
        if ctx is None:
            return None
        rates = ctx.extra.get("shipping_rates") or []
        for rate in rates:
            if rate.get("courier_code") != code:
                continue
            if service and rate.get("courier_service") not in (service, None):
                continue
            return _to_number(rate.get("price"))
        return None

    async def _cart_subtotal(customer_id: str) -> float:
        raw = await backend.get(f"/customers/{customer_id}/cart")
        return _to_number(_shape_cart(raw).get("subtotal"))

    async def _get_shipping_rates(args: dict[str, Any], context: ToolContext) -> Any:
        try:
            destination = int(args.get("destination_postal_code"))
        except (TypeError, ValueError):
            return {"error": "destination_postal_code must be a number"}
        payload: dict[str, Any] = {"destination_postal_code": destination}
        couriers = (args.get("couriers") or "").strip()
        if couriers:
            payload["couriers"] = couriers
        raw = await backend.post("/shipping/rates", json=payload)
        rates = _shape_rates(raw.get("rates") if isinstance(raw, dict) else raw)
        # Cache the authoritative fees so checkout can price the chosen courier.
        if context.session_context is not None:
            context.session_context.extra["shipping_rates"] = rates
        return {"rates": rates}

    async def _validate_voucher(args: dict[str, Any], context: ToolContext) -> Any:
        code = (args.get("code") or "").strip()
        if not code:
            return {"error": "code is required"}
        customer_id = await _resolve_customer_id(context)
        if not customer_id:
            return _no_customer()
        subtotal = await _cart_subtotal(customer_id)
        if subtotal <= 0:
            return {"error": "empty_cart"}
        raw = await backend.post(
            "/vouchers/validate", json={"code": code, "subtotal": subtotal}
        )
        return {"voucher": _shape_voucher_result(raw)}

    async def _build_checkout_payload(
        args: dict[str, Any], context: ToolContext, customer_id: str
    ) -> dict[str, Any] | dict[str, str]:
        code = (args.get("courier_code") or "").strip()
        service = (args.get("courier_service") or "").strip()
        if not code:
            return {"error": "courier_code is required"}
        fee = _find_rate_fee(context, code, service)
        if fee is None:
            # Guardrail: refuse to price a courier we never quoted (BR-004).
            return {"error": "unknown_courier", "hint": "call get_shipping_rates first"}
        payload: dict[str, Any] = {
            "customer_id": customer_id,
            "courier": {"code": code, "service": service, "fee": fee},
        }
        voucher_code = (args.get("voucher_code") or "").strip()
        if voucher_code:
            payload["voucher_code"] = voucher_code
        return payload

    async def _preview_checkout(args: dict[str, Any], context: ToolContext) -> Any:
        customer_id = await _resolve_customer_id(context)
        if not customer_id:
            return _no_customer()
        payload = await _build_checkout_payload(args, context, customer_id)
        if "error" in payload:
            return payload
        raw = await backend.post("/checkout/preview", json=payload)
        preview = _shape_preview(raw)
        if context.session_context is not None:
            context.session_context.courier = payload["courier"]["code"]
        return {"preview": preview}

    async def _confirm_checkout(args: dict[str, Any], context: ToolContext) -> Any:
        customer_id = await _resolve_customer_id(context)
        if not customer_id:
            return _no_customer()
        payload = await _build_checkout_payload(args, context, customer_id)
        if "error" in payload:
            return payload
        raw = await backend.post("/checkout", json=payload)
        order = _shape_order(raw)
        if context.session_context is not None and order.get("id"):
            context.session_context.order_id = order.get("id")
        return {"order": order}

    async def _create_invoice(args: dict[str, Any], context: ToolContext) -> Any:
        ctx = context.session_context
        order_id = (args.get("order_id") or (ctx.order_id if ctx else None) or "").strip()
        if not order_id:
            return {"error": "no_order_context"}
        payload: dict[str, Any] = {"order_id": order_id}
        payer_email = (args.get("payer_email") or "").strip()
        if payer_email:
            payload["payer_email"] = payer_email
        raw = await backend.post("/payments/invoices", json=payload)
        invoice = _shape_invoice(raw)
        if ctx is not None:
            ctx.invoice_id = invoice.get("id")
            ctx.invoice_status = invoice.get("status")
        return {"invoice": invoice}

    async def _get_invoice(args: dict[str, Any], context: ToolContext) -> Any:
        ctx = context.session_context
        order_id = (args.get("order_id") or (ctx.order_id if ctx else None) or "").strip()
        if not order_id:
            return {"error": "no_order_context"}
        raw = await backend.get(f"/orders/{order_id}/invoice")
        invoice = _shape_invoice(raw)
        if ctx is not None:
            ctx.invoice_status = invoice.get("status")
        return {"invoice": invoice}

    async def _get_order(args: dict[str, Any], context: ToolContext) -> Any:
        ctx = context.session_context
        order_id = (args.get("order_id") or (ctx.order_id if ctx else None) or "").strip()
        if not order_id:
            return {"error": "no_order_context"}
        raw = await backend.get(f"/orders/{order_id}")
        return {"order": _shape_order(raw)}

    async def _get_shipment(args: dict[str, Any], context: ToolContext) -> Any:
        ctx = context.session_context
        order_id = (args.get("order_id") or (ctx.order_id if ctx else None) or "").strip()
        if not order_id:
            return {"error": "no_order_context"}
        try:
            raw = await backend.get(f"/orders/{order_id}/shipment")
        except BackendError as exc:
            # No shipment booked yet is a normal "not shipped" outcome (BR-032).
            if exc.status_code == 404:
                return {"shipment": None}
            raise
        return {"shipment": _shape_shipment(raw)}

    # --- knowledge base tool (Phase A9) ---------------------------------

    async def _search_knowledge(args: dict[str, Any], context: ToolContext) -> Any:
        """Answer a non-product policy question from the curated KB (BR-050)."""
        query = (args.get("q") or args.get("query") or "").strip()
        article = knowledge.search(query)
        if article is None:
            return {"answer": None}
        # Stash the hit so the caller can emit a KB-answered analytics event.
        if context.session_context is not None:
            context.session_context.extra["last_kb_article"] = article.id
        return {"answer": article.answer, "topic": article.id}

    _register_product_tools(registry, _search_products, _get_product)
    _register_knowledge_tool(registry, _search_knowledge)

    _register_cart_tools(
        registry, _get_cart, _add_cart_item, _update_cart_item, _remove_cart_item
    )
    _register_customer_tools(
        registry, _save_customer, _get_default_address, _create_address
    )
    _register_checkout_tools(
        registry,
        rates_handler=_get_shipping_rates,
        validate_voucher_handler=_validate_voucher,
        preview_handler=_preview_checkout,
        confirm_handler=_confirm_checkout,
        create_invoice_handler=_create_invoice,
        get_invoice_handler=_get_invoice,
        get_order_handler=_get_order,
        get_shipment_handler=_get_shipment,
    )
    return registry



# --- registration helpers (kept out of build_registry for readability) -----


def _register_product_tools(
    registry: ToolRegistry, search_handler: ToolHandler, get_handler: ToolHandler
) -> None:
    registry.register(
        Tool(
            name="search_products",
            description=(
                "Search the active product catalog by keyword. Returns only "
                "active products that have variants in stock, with price and "
                "available stock. Use this for any product/price/stock question; "
                "never guess product data."
            ),
            parameters={
                "type": "object",
                "properties": {
                    "q": {
                        "type": "string",
                        "description": "Search keywords, e.g. 'kaos hitam'",
                    },
                    "limit": {
                        "type": "integer",
                        "description": "Max results (1-20, default 5)",
                    },
                },
                "required": ["q"],
            },
            handler=search_handler,
        )
    )
    registry.register(
        Tool(
            name="get_product",
            description=(
                "Fetch one active product with its purchasable variants by slug "
                "or id. Use when the customer refers to a specific product."
            ),
            parameters={
                "type": "object",
                "properties": {
                    "id": {"type": "string", "description": "Product id"},
                    "slug": {"type": "string", "description": "Product slug"},
                },
            },
            handler=get_handler,
        )
    )


def _register_knowledge_tool(
    registry: ToolRegistry, search_handler: ToolHandler
) -> None:
    registry.register(
        Tool(
            name="search_knowledge",
            description=(
                "Answer a NON-product policy/FAQ question from the shop's "
                "knowledge base: shipping coverage/estimates, payment methods, "
                "returns, operating hours, size guide, wholesale. Use this for "
                "'how/what/policy' questions that aren't about a specific product "
                "or the customer's own order. Returns null when there's no "
                "matching article — then ask a clarifying question or offer "
                "handover instead of guessing (BR-044). Do not use for prices, "
                "stock, or live order status (use the product/order tools)."
            ),
            parameters={
                "type": "object",
                "properties": {
                    "q": {
                        "type": "string",
                        "description": "The customer's question in their own words",
                    }
                },
                "required": ["q"],
            },
            handler=search_handler,
        )
    )


def _register_cart_tools(
    registry: ToolRegistry,
    get_handler: ToolHandler,
    add_handler: ToolHandler,
    update_handler: ToolHandler,
    remove_handler: ToolHandler,
) -> None:
    _variant_prop = {
        "type": "string",
        "description": "The product variant id (from search_products/get_product)",
    }
    _qty_prop = {"type": "integer", "description": "Quantity (positive integer)"}

    registry.register(
        Tool(
            name="get_cart",
            description=(
                "Get the customer's current open cart with items, quantities, "
                "unit prices and subtotal. The customer is identified "
                "automatically; never ask for a phone number."
            ),
            parameters={"type": "object", "properties": {}},
            handler=get_handler,
        )
    )
    registry.register(
        Tool(
            name="add_cart_item",
            description=(
                "Add a product variant to the customer's cart. Resolve the "
                "variant id first via search_products/get_product. Stock is "
                "validated by the backend; if it reports INSUFFICIENT_STOCK, tell "
                "the customer and offer alternatives (do not force it)."
            ),
            parameters={
                "type": "object",
                "properties": {"variant_id": _variant_prop, "qty": _qty_prop},
                "required": ["variant_id", "qty"],
            },
            handler=add_handler,
        )
    )
    registry.register(
        Tool(
            name="update_cart_item",
            description=(
                "Change the quantity of a variant already in the cart. Confirm "
                "changes with the customer (BR-046)."
            ),
            parameters={
                "type": "object",
                "properties": {"variant_id": _variant_prop, "qty": _qty_prop},
                "required": ["variant_id", "qty"],
            },
            handler=update_handler,
        )
    )
    registry.register(
        Tool(
            name="remove_cart_item",
            description="Remove a variant from the customer's cart.",
            parameters={
                "type": "object",
                "properties": {"variant_id": _variant_prop},
                "required": ["variant_id"],
            },
            handler=remove_handler,
        )
    )


def _register_customer_tools(
    registry: ToolRegistry,
    save_handler: ToolHandler,
    default_address_handler: ToolHandler,
    create_address_handler: ToolHandler,
) -> None:
    registry.register(
        Tool(
            name="save_customer",
            description=(
                "Create or update the customer's profile (name). The phone is "
                "taken automatically from WhatsApp; never ask for it. Call this "
                "once you know the customer's name."
            ),
            parameters={
                "type": "object",
                "properties": {
                    "name": {"type": "string", "description": "Customer's name"}
                },
                "required": ["name"],
            },
            handler=save_handler,
        )
    )
    registry.register(
        Tool(
            name="get_default_address",
            description=(
                "Get the customer's default shipping address, or null if none is "
                "saved yet. For returning customers, confirm they still want to "
                "use it (BR-009)."
            ),
            parameters={"type": "object", "properties": {}},
            handler=default_address_handler,
        )
    )
    registry.register(
        Tool(
            name="create_address",
            description=(
                "Save a shipping address for the customer. Collect all required "
                "fields first (BR-008): recipient_name, phone, address_line, "
                "city, district; postal_code and notes are optional. Set as "
                "default unless the customer says otherwise."
            ),
            parameters={
                "type": "object",
                "properties": {
                    "recipient_name": {"type": "string"},
                    "phone": {"type": "string"},
                    "address_line": {"type": "string"},
                    "city": {"type": "string"},
                    "district": {"type": "string"},
                    "postal_code": {"type": "string"},
                    "notes": {"type": "string"},
                    "label": {"type": "string"},
                    "is_default": {"type": "boolean"},
                },
                "required": [
                    "recipient_name",
                    "phone",
                    "address_line",
                    "city",
                    "district",
                ],
            },
            handler=create_address_handler,
        )
    )


def _register_checkout_tools(
    registry: ToolRegistry,
    *,
    rates_handler: ToolHandler,
    validate_voucher_handler: ToolHandler,
    preview_handler: ToolHandler,
    confirm_handler: ToolHandler,
    create_invoice_handler: ToolHandler,
    get_invoice_handler: ToolHandler,
    get_order_handler: ToolHandler,
    get_shipment_handler: ToolHandler,
) -> None:
    _courier_props = {
        "courier_code": {
            "type": "string",
            "description": "Courier code from get_shipping_rates (e.g. 'jne')",
        },
        "courier_service": {
            "type": "string",
            "description": "Courier service from get_shipping_rates (e.g. 'REG')",
        },
        "voucher_code": {
            "type": "string",
            "description": "Optional voucher code to apply",
        },
    }
    _order_prop = {
        "order_id": {
            "type": "string",
            "description": "Order id; defaults to the current session's order",
        }
    }

    registry.register(
        Tool(
            name="get_shipping_rates",
            description=(
                "Get courier options + prices for a destination postal code "
                "(BR-010, BR-011). Always call this before checkout so the "
                "customer can choose a courier; never guess a shipping fee. "
                "Requires the shipping address first."
            ),
            parameters={
                "type": "object",
                "properties": {
                    "destination_postal_code": {
                        "type": "integer",
                        "description": "Destination postal code",
                    },
                    "couriers": {
                        "type": "string",
                        "description": "Optional comma-separated courier codes",
                    },
                },
                "required": ["destination_postal_code"],
            },
            handler=rates_handler,
        )
    )
    registry.register(
        Tool(
            name="validate_voucher",
            description=(
                "Check whether a voucher code is usable for the current cart "
                "subtotal and return the discount, without consuming it "
                "(BR-037, BR-038). Validate before applying at checkout."
            ),
            parameters={
                "type": "object",
                "properties": {
                    "code": {"type": "string", "description": "Voucher code"}
                },
                "required": ["code"],
            },
            handler=validate_voucher_handler,
        )
    )
    registry.register(
        Tool(
            name="preview_checkout",
            description=(
                "Build a checkout summary (items, shipping fee, discount, total) "
                "for the customer to review BEFORE confirming (BR-015). The "
                "courier must come from a prior get_shipping_rates call. Show the "
                "totals from this result verbatim; never compute them yourself."
            ),
            parameters={
                "type": "object",
                "properties": _courier_props,
                "required": ["courier_code"],
            },
            handler=preview_handler,
        )
    )
    registry.register(
        Tool(
            name="confirm_checkout",
            description=(
                "Confirm the order and reserve stock. ONLY call this after the "
                "customer has explicitly confirmed the preview_checkout summary "
                "(BR-045). This creates a real order; do not call it speculatively."
            ),
            parameters={
                "type": "object",
                "properties": _courier_props,
                "required": ["courier_code"],
            },
            handler=confirm_handler,
        )
    )
    registry.register(
        Tool(
            name="create_invoice",
            description=(
                "Create a payment invoice + link for a confirmed order and return "
                "the payment URL and expiry (BR-016..BR-021). Call after "
                "confirm_checkout. Never tell the customer the order is paid; "
                "payment confirmation arrives separately."
            ),
            parameters={
                "type": "object",
                "properties": {
                    **_order_prop,
                    "payer_email": {
                        "type": "string",
                        "description": "Optional payer email for the receipt",
                    },
                },
            },
            handler=create_invoice_handler,
        )
    )
    registry.register(
        Tool(
            name="get_invoice",
            description=(
                "Get the current invoice + payment status for an order. Use to "
                "check whether a payment has gone through; report the status as-is."
            ),
            parameters={"type": "object", "properties": dict(_order_prop)},
            handler=get_invoice_handler,
        )
    )
    registry.register(
        Tool(
            name="get_order",
            description=(
                "Get an order's current status, items and totals (BR-025, "
                "BR-032..BR-034). Use for 'where is my order' / status questions."
            ),
            parameters={"type": "object", "properties": dict(_order_prop)},
            handler=get_order_handler,
        )
    )
    registry.register(
        Tool(
            name="get_shipment",
            description=(
                "Get shipment tracking for an order — courier, status and "
                "tracking number — or null if not shipped yet (BR-032)."
            ),
            parameters={"type": "object", "properties": dict(_order_prop)},
            handler=get_shipment_handler,
        )
    )

