"""Tests for the Phase A5 sales-flow tools: cart, customer, and address.

Focus areas:
- Customer identity is resolved from the session (phone = WhatsApp id), never
  from model arguments, and cached on the conversation context (BR-008).
- Cart operations are scoped to the resolved customer and shaped into a compact
  summary with a subtotal derived from backend unit prices (BR-004).
- Business rejections from the backend (e.g. INSUFFICIENT_STOCK) surface as
  actionable, ``ok=False`` results the model can explain (BR-006).
"""

from __future__ import annotations

from typing import Any

from app.conversation.models import ConversationContext
from app.llm.tools import ToolContext, build_registry
from app.llm.types import ToolCall
from app.platform.errors import BackendError


class RoutingBackend:
    """A configurable fake backend that routes by (method, path prefix).

    ``routes`` maps a ``"METHOD /prefix"`` key to either a value to return or a
    :class:`BackendError` to raise. Records every call for assertions.
    """

    def __init__(self, routes: dict[str, Any] | None = None):
        self.routes = routes or {}
        self.calls: list[tuple[str, str, dict | None]] = []

    def _resolve(self, method: str, path: str, payload: dict | None) -> Any:
        self.calls.append((method, path, payload))
        # Match the most specific (longest) registered prefix for this method so
        # e.g. "/customers/x/cart/items" wins over "/customers".
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


def _ctx(wa_id: str = "628123", **fields: Any) -> ToolContext:
    return ToolContext(wa_id=wa_id, session_context=ConversationContext(**fields))


def _customer(cid: str = "cust-1", name: str = "Budi") -> dict[str, Any]:
    return {"id": cid, "phone": "628123", "name": name}


def _cart(items: list[dict[str, Any]] | None = None) -> dict[str, Any]:
    return {
        "id": "cart-1",
        "customer_id": "cust-1",
        "status": "open",
        "items": items if items is not None else [],
    }


# --- customer identity resolution -----------------------------------------


async def test_get_cart_upserts_customer_from_wa_id_and_caches_id():
    backend = RoutingBackend(
        {
            "POST /customers": _customer(),
            "GET /customers/cust-1/cart": _cart(
                [{"variant_id": "v1", "qty": 2, "unit_price": 50000}]
            ),
        }
    )
    registry = build_registry(backend)  # type: ignore[arg-type]
    context = _ctx()

    result = await registry.dispatch(
        ToolCall(id="c1", name="get_cart", arguments={}), context
    )

    assert result.ok is True
    cart = result.content["cart"]
    assert cart["item_count"] == 2
    assert cart["subtotal"] == 100000
    # Customer was upserted by the session's WhatsApp id (phone), not model args.
    post_call = backend.calls[0]
    assert post_call[0] == "POST" and post_call[1] == "/customers"
    assert post_call[2]["phone"] == "628123"
    # Resolved ids are cached on the conversation context for later calls.
    assert context.session_context.customer_id == "cust-1"
    assert context.session_context.cart_id == "cart-1"


async def test_cached_customer_id_skips_upsert():
    backend = RoutingBackend({"GET /customers/cust-9/cart": _cart()})
    registry = build_registry(backend)  # type: ignore[arg-type]
    context = _ctx(customer_id="cust-9")

    result = await registry.dispatch(
        ToolCall(id="c1", name="get_cart", arguments={}), context
    )
    assert result.ok is True
    # No POST /customers because the id was already known.
    assert all(not (m == "POST" and p == "/customers") for m, p, _ in backend.calls)


async def test_missing_wa_id_returns_no_customer_context():
    backend = RoutingBackend()
    registry = build_registry(backend)  # type: ignore[arg-type]
    context = ToolContext(wa_id=None, session_context=ConversationContext())

    result = await registry.dispatch(
        ToolCall(id="c1", name="get_cart", arguments={}), context
    )
    # ok=True (tool ran) but content signals we can't act without identity.
    assert result.ok is True
    assert result.content == {"error": "no_customer_context"}


# --- cart mutations ---------------------------------------------------------


async def test_add_cart_item_posts_variant_and_qty():
    backend = RoutingBackend(
        {
            "POST /customers": _customer(),
            "POST /customers/cust-1/cart/items": _cart(
                [{"variant_id": "v1", "qty": 3, "unit_price": 25000}]
            ),
        }
    )
    registry = build_registry(backend)  # type: ignore[arg-type]

    result = await registry.dispatch(
        ToolCall(
            id="c1",
            name="add_cart_item",
            arguments={"variant_id": "v1", "qty": 3},
        ),
        _ctx(customer_id="cust-1"),
    )
    assert result.ok is True
    assert result.content["cart"]["subtotal"] == 75000
    add_call = next(c for c in backend.calls if c[1] == "/customers/cust-1/cart/items")
    assert add_call[2] == {"variant_id": "v1", "qty": 3}


async def test_add_cart_item_rejects_non_positive_qty_without_calling_backend():
    backend = RoutingBackend()
    registry = build_registry(backend)  # type: ignore[arg-type]
    result = await registry.dispatch(
        ToolCall(
            id="c1", name="add_cart_item", arguments={"variant_id": "v1", "qty": 0}
        ),
        _ctx(customer_id="cust-1"),
    )
    assert result.ok is True
    assert result.content == {"error": "qty must be a positive integer"}
    assert backend.calls == []


async def test_add_cart_item_insufficient_stock_is_actionable():
    backend = RoutingBackend(
        {
            "POST /customers/cust-1/cart/items": BackendError(
                "insufficient stock",
                status_code=409,
                upstream_status=409,
                upstream_code="INSUFFICIENT_STOCK",
            ),
        }
    )
    registry = build_registry(backend)  # type: ignore[arg-type]
    result = await registry.dispatch(
        ToolCall(
            id="c1", name="add_cart_item", arguments={"variant_id": "v1", "qty": 99}
        ),
        _ctx(customer_id="cust-1"),
    )
    assert result.ok is False
    assert result.content == {"error": "request_rejected", "code": "INSUFFICIENT_STOCK"}


async def test_update_and_remove_cart_item_hit_variant_path():
    backend = RoutingBackend(
        {
            "PUT /customers/cust-1/cart/items/v1": _cart(
                [{"variant_id": "v1", "qty": 5, "unit_price": 10000}]
            ),
            "DELETE /customers/cust-1/cart/items/v1": _cart([]),
        }
    )
    registry = build_registry(backend)  # type: ignore[arg-type]

    updated = await registry.dispatch(
        ToolCall(
            id="c1", name="update_cart_item", arguments={"variant_id": "v1", "qty": 5}
        ),
        _ctx(customer_id="cust-1"),
    )
    assert updated.content["cart"]["subtotal"] == 50000

    removed = await registry.dispatch(
        ToolCall(id="c2", name="remove_cart_item", arguments={"variant_id": "v1"}),
        _ctx(customer_id="cust-1"),
    )
    assert removed.content["cart"]["items"] == []


# --- customer / address -----------------------------------------------------


async def test_save_customer_uses_session_phone_and_caches():
    backend = RoutingBackend({"POST /customers": _customer(name="Siti")})
    registry = build_registry(backend)  # type: ignore[arg-type]
    context = _ctx()

    result = await registry.dispatch(
        ToolCall(id="c1", name="save_customer", arguments={"name": "Siti"}), context
    )
    assert result.ok is True
    assert result.content["customer"]["id"] == "cust-1"
    assert context.session_context.name == "Siti"
    assert context.session_context.customer_id == "cust-1"
    assert backend.calls[0][2] == {"phone": "628123", "name": "Siti"}


async def test_get_default_address_missing_returns_null_not_error():
    backend = RoutingBackend(
        {
            "GET /customers/cust-1/addresses/default": BackendError(
                "not found", status_code=404, upstream_status=404
            ),
        }
    )
    registry = build_registry(backend)  # type: ignore[arg-type]
    result = await registry.dispatch(
        ToolCall(id="c1", name="get_default_address", arguments={}),
        _ctx(customer_id="cust-1"),
    )
    assert result.ok is True
    assert result.content == {"address": None}


async def test_create_address_requires_core_fields():
    backend = RoutingBackend()
    registry = build_registry(backend)  # type: ignore[arg-type]
    result = await registry.dispatch(
        ToolCall(
            id="c1",
            name="create_address",
            arguments={"recipient_name": "Budi", "phone": "628123"},
        ),
        _ctx(customer_id="cust-1"),
    )
    assert result.ok is True
    assert result.content["error"] == "missing_fields"
    assert set(result.content["fields"]) == {"address_line", "city", "district"}
    assert backend.calls == []


async def test_create_address_posts_and_caches_id():
    backend = RoutingBackend(
        {
            "POST /customers/cust-1/addresses": {
                "id": "addr-1",
                "recipient_name": "Budi",
                "city": "Jakarta",
                "district": "Menteng",
                "is_default": True,
            }
        }
    )
    registry = build_registry(backend)  # type: ignore[arg-type]
    context = _ctx(customer_id="cust-1")

    result = await registry.dispatch(
        ToolCall(
            id="c1",
            name="create_address",
            arguments={
                "recipient_name": "Budi",
                "phone": "628123",
                "address_line": "Jl. Merdeka 1",
                "city": "Jakarta",
                "district": "Menteng",
                "postal_code": "10310",
            },
        ),
        context,
    )
    assert result.ok is True
    assert result.content["address"]["id"] == "addr-1"
    assert context.session_context.address_id == "addr-1"
    # is_default defaults to True when unspecified.
    post_call = next(c for c in backend.calls if c[1] == "/customers/cust-1/addresses")
    assert post_call[2]["is_default"] is True
