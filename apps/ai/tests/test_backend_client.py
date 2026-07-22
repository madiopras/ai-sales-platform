"""Tests for the backend client: auth header, envelope unwrap, error mapping."""

from __future__ import annotations

import httpx
import pytest
import respx

from app.clients.backend import BackendClient
from app.config import Settings
from app.platform.errors import BackendError

BASE = "http://backend:8081"
INTERNAL = f"{BASE}/internal/v1"


def make_client() -> BackendClient:
    settings = Settings(
        backend_base_url=BASE,
        backend_service_token="secret-token",
        backend_max_retries=1,
    )
    return BackendClient(settings)


@respx.mock
async def test_get_unwraps_data_and_sends_service_token():
    route = respx.get(f"{INTERNAL}/products/search").mock(
        return_value=httpx.Response(200, json={"data": [{"id": "1"}], "request_id": "x"})
    )
    client = make_client()
    try:
        data = await client.get("/products/search", params={"q": "kaos"})
    finally:
        await client.aclose()

    assert data == [{"id": "1"}]
    sent = route.calls.last.request
    assert sent.headers["X-Service-Token"] == "secret-token"


@respx.mock
async def test_client_error_maps_to_backend_error_with_status():
    respx.get(f"{INTERNAL}/products/999").mock(
        return_value=httpx.Response(404, json={"error": {"code": "NOT_FOUND", "message": "nope"}})
    )
    client = make_client()
    try:
        with pytest.raises(BackendError) as exc_info:
            await client.get("/products/999")
    finally:
        await client.aclose()

    err = exc_info.value
    assert err.status_code == 404
    assert err.upstream_code == "NOT_FOUND"


@respx.mock
async def test_server_error_collapses_to_502_after_retries():
    route = respx.get(f"{INTERNAL}/products/search").mock(
        return_value=httpx.Response(500, json={"error": {"code": "BOOM", "message": "x"}})
    )
    client = make_client()
    try:
        with pytest.raises(BackendError) as exc_info:
            await client.get("/products/search")
    finally:
        await client.aclose()

    # max_retries=1 → 2 attempts total for an idempotent GET.
    assert route.call_count == 2
    assert exc_info.value.status_code == 502


@respx.mock
async def test_transport_error_raises_backend_error():
    respx.get(f"{INTERNAL}/products/search").mock(side_effect=httpx.ConnectError("down"))
    client = make_client()
    try:
        with pytest.raises(BackendError):
            await client.get("/products/search")
    finally:
        await client.aclose()


@respx.mock
async def test_ping_true_on_backend_ready():
    respx.get(f"{BASE}/health/ready").mock(return_value=httpx.Response(200, json={"data": {}}))
    client = make_client()
    try:
        assert await client.ping() is True
    finally:
        await client.aclose()


@respx.mock
async def test_ping_false_when_backend_unreachable():
    respx.get(f"{BASE}/health/ready").mock(side_effect=httpx.ConnectError("down"))
    client = make_client()
    try:
        assert await client.ping() is False
    finally:
        await client.aclose()
