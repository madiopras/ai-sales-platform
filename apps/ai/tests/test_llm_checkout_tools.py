"""Tests for the Phase A6 tools: shipping, checkout, payment, order tracking.

Focus areas:
- Shipping fees come only from a backend rate quote; checkout refuses to price a
  courier the AI never fetched (BR-004 "source of truth").
- Checkout preview/confirm are scoped to the session customer and pass the
  backend's own totals through unchanged (BR-015).
- Invoice creation returns the payment link + status; the AI never asserts
  "paid" (payment rules / Phase A7).
- Order/shipment lookups default to the session's order and treat "no shipment
  yet" as an empty, non-error outcome (BR-032).
"""

from __future__ import annotations

from typing import Any

from app.conversation.models import ConversationContext
from app.llm.tools import ToolContext, build_registry
from app.llm.types import ToolCall
from app.platform.errors import BackendError


class RoutingBackend:
    """Fake backend routing by ``"METHOD /prefix"`` (longest prefix wins)."""

    def __init__(self, routes: dict[str, Any] | None = None):
        self.routes = routes or {}
        self.calls: list[tuple[str, str, dict | None]] = []

    def _resolve(self, method: str, path: str, payload: dict | None) -> Any:
        self.calls.append((method, path, payload))
        best_key: str | None = None
        best_len = -1
        for key in self.routes:
            m, _, prefix = key.partition(" ")
            if m == method and path.startswith(prefix) and len(prefix) > best_len:
                best_key, best_len = key, len(prefix)
        if best_key is None:
            return None
        value = self.routes[best_key]
        if isinstance(value, BackendError):
            raise value
        return value

    async def get(self, path: str, *, params: dict | None = None) -> Any:
        return self._resolve("GET", path, params)

    async def post(self, path: str, *, json: dict | None = None) -> Any:
        return self._resolve("POST", path, json)

    async def put(self, path: str, *, json: dict | None = None) -> Any:
        return self._resolve("PUT", path, json)

    async def delete(self, path: str, *, params: dict | None = None) -> Any:
        return self._resolve("DELETE", path, params)


def _ctx(**fields: Any) -> ToolContext:
    fields.setdefault("customer_id", "cust-1")
    return ToolContext(wa_id="628123", session_context=ConversationContext(**fields))


def _rate() -> dict[str, Any]:
    return {
        "courier_code": "jne",
        "courier_service": "REG",
        "courier_name": "JNE",
        "service_name": "Layanan Reguler",
        "duration": "2-3 hari",
        "price": 18000,
    }


# --- shipping rates ---------------------------------------------------------


async def test_get_shipping_rates_shapes_and_caches_fees():
    backend = RoutingBackend({"POST /shipping/rates": {"rates": [_rate()]}})
    registry = build_registry(backend)  # type: ignore[arg-type]
    context = _ctx()

    result = await registry.dispatch(
        ToolCall(
            id="c1",
            name="get_shipping_rates",
            arguments={"destination_postal_code": 40123},
        ),
        context,
    )
    assert result.ok is True
    assert result.content["rates"][0]["price"] == 18000
    # Rates are cached so checkout can price the chosen courier authoritatively.
    assert context.session_context.extra["shipping_rates"][0]["courier_code"] == "jne"
    # The backend was asked with the numeric destination.
    assert backend.calls[0][2]["destination_postal_code"] == 40123


async def test_get_shipping_rates_rejects_non_numeric_postal():
    backend = RoutingBackend()
    registry = build_registry(backend)  # type: ignore[arg-type]
    result = await registry.dispatch(
        ToolCall(
            id="c1",
            name="get_shipping_rates",
            arguments={"destination_postal_code": "abc"},
        ),
        _ctx(),
    )
    assert result.ok is True
    assert result.content == {"error": "destination_postal_code must be a number"}
    assert backend.calls == []


# --- checkout preview / confirm (BR-004, BR-015, BR-045) --------------------


async def test_preview_checkout_refuses_unquoted_courier():
    # No get_shipping_rates was called this session → we must not invent a fee.
    backend = RoutingBackend({"POST /checkout/preview": {"total": 1}})
    registry = build_registry(backend)  # type: ignore[arg-type]
    result = await registry.dispatch(
        ToolCall(
            id="c1",
            name="preview_checkout",
            arguments={"courier_code": "jne", "courier_service": "REG"},
        ),
        _ctx(),
    )
    assert result.ok is True
    assert result.content["error"] == "unknown_courier"
    # Never hit the checkout endpoint without an authoritative fee.
    assert all(path != "/checkout/preview" for _, path, _ in backend.calls)


async def test_preview_checkout_uses_cached_fee_and_passes_backend_totals():
    backend = RoutingBackend(
        {
            "POST /checkout/preview": {
                "items": [
                    {"variant_id": "v1", "qty": 2, "unit_price": 50000, "subtotal": 100000}
                ],
                "address": {"city": "Bandung"},
                "courier": {"code": "jne", "service": "REG", "fee": 18000},
                "subtotal": 100000,
                "shipping_fee": 18000,
                "discount": 0,
                "total": 118000,
            }
        }
    )
    registry = build_registry(backend)  # type: ignore[arg-type]
    context = _ctx(extra={"shipping_rates": [_rate()]})

    result = await registry.dispatch(
        ToolCall(
            id="c1",
            name="preview_checkout",
            arguments={"courier_code": "jne", "courier_service": "REG"},
        ),
        context,
    )
    assert result.ok is True
    preview = result.content["preview"]
    assert preview["total"] == 118000
    assert preview["shipping_fee"] == 18000
    # The fee sent to the backend came from the cached quote, not the model.
    preview_call = next(c for c in backend.calls if c[1] == "/checkout/preview")
    assert preview_call[2]["courier"]["fee"] == 18000


async def test_confirm_checkout_creates_order_and_caches_id():
    backend = RoutingBackend(
        {
            "POST /checkout": {
                "id": "order-1",
                "order_no": "INV-1",
                "status": "pending_payment",
                "total": 118000,
            }
        }
    )
    registry = build_registry(backend)  # type: ignore[arg-type]
    context = _ctx(extra={"shipping_rates": [_rate()]})

    result = await registry.dispatch(
        ToolCall(
            id="c1",
            name="confirm_checkout",
            arguments={"courier_code": "jne", "courier_service": "REG"},
        ),
        context,
    )
    assert result.ok is True
    assert result.content["order"]["id"] == "order-1"
    assert result.content["order"]["status"] == "pending_payment"
    # Order id is cached so create_invoice can default to it.
    assert context.session_context.order_id == "order-1"


async def test_confirm_checkout_surfaces_insufficient_stock():
    backend = RoutingBackend(
        {
            "POST /checkout": BackendError(
                "insufficient stock",
                status_code=409,
                upstream_status=409,
                upstream_code="INSUFFICIENT_STOCK",
            )
        }
    )
    registry = build_registry(backend)  # type: ignore[arg-type]
    context = _ctx(extra={"shipping_rates": [_rate()]})
    result = await registry.dispatch(
        ToolCall(
            id="c1",
            name="confirm_checkout",
            arguments={"courier_code": "jne", "courier_service": "REG"},
        ),
        context,
    )
    assert result.ok is False
    assert result.content == {"error": "request_rejected", "code": "INSUFFICIENT_STOCK"}


# --- voucher ----------------------------------------------------------------


async def test_validate_voucher_uses_cart_subtotal():
    backend = RoutingBackend(
        {
            "GET /customers/cust-1/cart": {
                "id": "cart-1",
                "items": [{"variant_id": "v1", "qty": 2, "unit_price": 50000}],
            },
            "POST /vouchers/validate": {
                "code": "HEMAT10",
                "discount_type": "percent",
                "discount": 10000,
                "subtotal": 100000,
            },
        }
    )
    registry = build_registry(backend)  # type: ignore[arg-type]
    result = await registry.dispatch(
        ToolCall(id="c1", name="validate_voucher", arguments={"code": "HEMAT10"}),
        _ctx(),
    )
    assert result.ok is True
    assert result.content["voucher"]["discount"] == 10000
    # The subtotal sent to the backend was derived from the cart, not the model.
    validate_call = next(c for c in backend.calls if c[1] == "/vouchers/validate")
    assert validate_call[2]["subtotal"] == 100000


async def test_validate_voucher_empty_cart_short_circuits():
    backend = RoutingBackend(
        {"GET /customers/cust-1/cart": {"id": "cart-1", "items": []}}
    )
    registry = build_registry(backend)  # type: ignore[arg-type]
    result = await registry.dispatch(
        ToolCall(id="c1", name="validate_voucher", arguments={"code": "HEMAT10"}),
        _ctx(),
    )
    assert result.ok is True
    assert result.content == {"error": "empty_cart"}
    assert all(path != "/vouchers/validate" for _, path, _ in backend.calls)


# --- payment / invoice ------------------------------------------------------


async def test_create_invoice_defaults_to_session_order_and_caches_status():
    backend = RoutingBackend(
        {
            "POST /payments/invoices": {
                "id": "inv-1",
                "invoice_no": "INV-1",
                "order_id": "order-1",
                "amount": 118000,
                "status": "waiting_payment",
                "xendit_payment_url": "https://pay.example/inv-1",
                "expired_at": "2026-01-01T00:00:00Z",
            }
        }
    )
    registry = build_registry(backend)  # type: ignore[arg-type]
    context = _ctx(order_id="order-1")

    result = await registry.dispatch(
        ToolCall(id="c1", name="create_invoice", arguments={}), context
    )
    assert result.ok is True
    invoice = result.content["invoice"]
    assert invoice["payment_url"] == "https://pay.example/inv-1"
    assert invoice["status"] == "waiting_payment"
    # Invoice status/id mirrored onto the session for later checks.
    assert context.session_context.invoice_id == "inv-1"
    assert context.session_context.invoice_status == "waiting_payment"
    # Called the backend with the session's order id.
    assert backend.calls[0][2] == {"order_id": "order-1"}


async def test_create_invoice_without_order_context_errors():
    backend = RoutingBackend()
    registry = build_registry(backend)  # type: ignore[arg-type]
    result = await registry.dispatch(
        ToolCall(id="c1", name="create_invoice", arguments={}), _ctx()
    )
    assert result.ok is True
    assert result.content == {"error": "no_order_context"}
    assert backend.calls == []


# --- order / shipment tracking (BR-032) -------------------------------------


async def test_get_order_defaults_to_session_order():
    backend = RoutingBackend(
        {"GET /orders/order-1": {"id": "order-1", "status": "paid", "total": 118000}}
    )
    registry = build_registry(backend)  # type: ignore[arg-type]
    result = await registry.dispatch(
        ToolCall(id="c1", name="get_order", arguments={}),
        _ctx(order_id="order-1"),
    )
    assert result.ok is True
    assert result.content["order"]["status"] == "paid"


async def test_get_shipment_not_yet_shipped_returns_null():
    backend = RoutingBackend(
        {
            "GET /orders/order-1/shipment": BackendError(
                "not found", status_code=404, upstream_status=404
            )
        }
    )
    registry = build_registry(backend)  # type: ignore[arg-type]
    result = await registry.dispatch(
        ToolCall(id="c1", name="get_shipment", arguments={"order_id": "order-1"}),
        _ctx(),
    )
    assert result.ok is True
    assert result.content == {"shipment": None}


async def test_get_shipment_returns_tracking():
    backend = RoutingBackend(
        {
            "GET /orders/order-1/shipment": {
                "order_id": "order-1",
                "courier_code": "jne",
                "status": "shipped",
                "tracking_no": "JNE123",
            }
        }
    )
    registry = build_registry(backend)  # type: ignore[arg-type]
    result = await registry.dispatch(
        ToolCall(id="c1", name="get_shipment", arguments={"order_id": "order-1"}),
        _ctx(),
    )
    assert result.ok is True
    assert result.content["shipment"]["tracking_no"] == "JNE123"
