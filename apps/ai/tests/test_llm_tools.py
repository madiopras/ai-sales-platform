"""Tests for the tool registry: schemas, product shaping, and safe dispatch."""

from __future__ import annotations

from typing import Any

from app.llm.tools import ToolRegistry, build_registry
from app.llm.types import ToolCall
from app.platform.errors import BackendError


class FakeBackend:
    """Records calls and returns scripted responses or raises BackendError."""

    def __init__(self, responses: dict[str, Any] | None = None, raise_on: str | None = None):
        self.responses = responses or {}
        self.raise_on = raise_on
        self.calls: list[tuple[str, dict | None]] = []

    async def get(self, path: str, *, params: dict | None = None) -> Any:
        self.calls.append((path, params))
        if self.raise_on and self.raise_on in path:
            raise BackendError("boom", status_code=502)
        # Match search vs. slug vs. id by path prefix.
        for key, value in self.responses.items():
            if path.startswith(key):
                return value
        return None

    async def post(self, path: str, *, json: dict | None = None) -> Any:
        self.calls.append((path, json))
        if self.raise_on and self.raise_on in path:
            raise BackendError("boom", status_code=502)
        for key, value in self.responses.items():
            if path.startswith(key):
                return value
        return None



def _product(name="Kaos Hitam", status="active", variants=None):
    return {
        "id": "p1",
        "name": name,
        "slug": "kaos-hitam",
        "description": "Kaos katun",
        "status": status,
        "variants": variants
        if variants is not None
        else [
            {
                "id": "v1",
                "sku": "KH-XL",
                "name": "XL",
                "price": 89000,
                "stock": 10,
                "is_active": True,
            },
        ],

    }


async def test_search_products_filters_and_shapes():
    backend = FakeBackend(responses={"/products/search": [_product()]})
    registry = build_registry(backend)  # type: ignore[arg-type]

    result = await registry.dispatch(
        ToolCall(id="c1", name="search_products", arguments={"q": "kaos hitam"})
    )

    assert result.ok is True
    products = result.content["products"]
    assert len(products) == 1
    variant = products[0]["variants"][0]
    assert variant["available_stock"] == 10
    assert variant["price"] == 89000
    # Backend was called with the query + a clamped limit.
    path, params = backend.calls[0]
    assert path == "/products/search"
    assert params["q"] == "kaos hitam"


async def test_search_drops_out_of_stock_products():
    out = _product(
        variants=[
            {"id": "v1", "sku": "S", "name": "S", "price": 50000, "stock": 0, "is_active": True},
        ]
    )
    backend = FakeBackend(responses={"/products/search": [out]})
    registry = build_registry(backend)  # type: ignore[arg-type]

    result = await registry.dispatch(
        ToolCall(id="c1", name="search_products", arguments={"q": "kaos"})
    )
    assert result.ok is True
    assert result.content["products"] == []


async def test_search_uses_stock_on_hand_minus_reserved():
    prod = _product(
        variants=[
            {
                "id": "v1",
                "sku": "S",
                "name": "S",
                "price": 50000,
                "stock_on_hand": 5,
                "stock_reserved": 5,
                "is_active": True,
            },
            {
                "id": "v2",
                "sku": "M",
                "name": "M",
                "price": 55000,
                "stock_on_hand": 8,
                "stock_reserved": 2,
                "is_active": True,
            },
        ]
    )
    backend = FakeBackend(responses={"/products/search": [prod]})
    registry = build_registry(backend)  # type: ignore[arg-type]
    result = await registry.dispatch(
        ToolCall(id="c1", name="search_products", arguments={"q": "kaos"})
    )
    variants = result.content["products"][0]["variants"]
    # v1 has 0 available (5-5) → dropped; v2 has 6 available → kept.
    assert [v["sku"] for v in variants] == ["M"]
    assert variants[0]["available_stock"] == 6


async def test_get_product_by_slug():
    backend = FakeBackend(responses={"/products/slug/": _product()})
    registry = build_registry(backend)  # type: ignore[arg-type]
    result = await registry.dispatch(
        ToolCall(id="c1", name="get_product", arguments={"slug": "kaos-hitam"})
    )
    assert result.ok is True
    assert result.content["product"]["name"] == "Kaos Hitam"


async def test_dispatch_backend_error_is_structured_not_raised():
    backend = FakeBackend(raise_on="/products/search")
    registry = build_registry(backend)  # type: ignore[arg-type]
    result = await registry.dispatch(
        ToolCall(id="c1", name="search_products", arguments={"q": "x"})
    )
    assert result.ok is False
    assert result.content == {"error": "backend_unavailable"}


async def test_dispatch_unknown_tool():
    registry = ToolRegistry()
    result = await registry.dispatch(ToolCall(id="c1", name="nope", arguments={}))
    assert result.ok is False


def test_schemas_expose_registered_tools():
    backend = FakeBackend()
    registry = build_registry(backend)  # type: ignore[arg-type]
    names = {s["function"]["name"] for s in registry.schemas()}
    assert names == {
        "search_products",
        "get_product",
        "search_knowledge",
        "get_cart",
        "add_cart_item",
        "update_cart_item",
        "remove_cart_item",
        "save_customer",
        "get_default_address",
        "create_address",
        "get_shipping_rates",
        "validate_voucher",
        "preview_checkout",
        "confirm_checkout",
        "create_invoice",
        "get_invoice",
        "get_order",
        "get_shipment",
    }


    # Each schema is OpenAI function-calling shaped.
    for schema in registry.schemas():
        assert schema["type"] == "function"
        assert "parameters" in schema["function"]
